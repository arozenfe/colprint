package colprint

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Registry stores field definitions for type T.
//
// Create a registry with NewRegistry, then use the Field() method to
// register fields via a fluent builder API.
//
// Registries can be hierarchical - use AddRegistry to nest sub-registries
// with their own names for organized help output.
type Registry[T any] struct {
	name          string
	fields        map[string]Field[T]
	fieldOrder    []string // preserves insertion order
	index         map[string]string // lowercase -> canonical name
	collections   map[string][]string
	defaults      map[string]string
	subRegistries []*Registry[T]
}

// NewRegistry creates a new unnamed field registry for type T.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		name:        "",
		fields:      make(map[string]Field[T]),
		fieldOrder:  make([]string, 0),
		index:       make(map[string]string),
		collections: make(map[string][]string),
		defaults:    make(map[string]string),
	}
}

// NewRegistryWithName creates a named field registry for type T.
//
// Named registries are useful for organizing related fields into sections
// when building hierarchical registries with AddRegistry().
func NewRegistryWithName[T any](name string) *Registry[T] {
	reg := NewRegistry[T]()
	reg.name = name
	return reg
}

// Field starts building a new field definition.
//
// Use the returned FieldBuilder to configure the field, then call Register()
// to add it to the registry.
//
// Example:
//
//	reg.Field("age", "Age", "Age in years").
//	    Width(5).
//	    Int(func(p *Person) int { return p.Age }).
//	    Register()
func (r *Registry[T]) Field(name, display, description string) *FieldBuilder[T] {
	return &FieldBuilder[T]{
		registry: r,
		field: Field[T]{
			Name:        name,
			Display:     display,
			Description: description,
			Width:       10, // default width
		},
	}
}

// DefineCollection creates a named collection of fields.
//
// The defaultSpec is the comma-separated list of fields used when this
// collection is referenced with @collection_name. The fields list contains
// all fields that belong to this collection (for help display).
//
// Example:
//
//	reg.DefineCollection("basic", "name,age", "name", "age", "email", "phone")
func (r *Registry[T]) DefineCollection(name, defaultSpec string, fields ...string) {
	r.collections[name] = fields
	r.defaults[name] = defaultSpec
}

// SetDefaults sets the default field specification for a collection.
//
// When @default is used in a spec, it expands to this value.
func (r *Registry[T]) SetDefaults(collectionName, spec string) {
	r.defaults[collectionName] = spec
}

// AddRegistry adds a sub-registry to this registry.
//
// This enables hierarchical organization of fields. Sub-registries with
// names will appear as separate sections in help output.
//
// Example:
//
//	procFields := colprint.NewRegistryWithName[TreeNode]("Process Fields")
//	colprint.InheritFieldsFrom(procFields, procReg, mapper)
//	reg.AddRegistry(procFields)
func (r *Registry[T]) AddRegistry(sub *Registry[T]) {
	if r.subRegistries == nil {
		r.subRegistries = make([]*Registry[T], 0)
	}
	r.subRegistries = append(r.subRegistries, sub)
}

// InheritFieldsFrom copies all fields from a source registry to the destination registry,
// transforming the field accessors using the provided mapper function.
//
// This is useful when type T contains or embeds type S, and you want to reuse
// all field definitions from S's registry for T.
//
// Example:
//
//	type TreeNode struct {
//	    Proc  // embedded
//	    cutime float32
//	}
//
//	procReg := CreateProcRegistry()
//	treeReg := colprint.NewRegistry[TreeNode]()
//	colprint.InheritFieldsFrom(treeReg, procReg, func(n *TreeNode) *Proc { return &n.Proc })
//	// Now treeReg has all Proc fields, add TreeNode-specific fields
//	treeReg.Field("cutime", "CUtime", "Cumulative user time").Width(10)...
func InheritFieldsFrom[T any, S any](dest *Registry[T], source *Registry[S], mapper func(*T) *S) {
	for _, name := range source.fieldOrder {
		srcField := source.fields[name]
		// Create a new field with the same metadata (except Category which is registry-level now)
		field := Field[T]{
			Name:        srcField.Name,
			Display:     srcField.Display,
			Description: srcField.Description,
			Width:       srcField.Width,
			Kind:        srcField.Kind,
			Precision:   srcField.Precision,
		}

		// Wrap the source field's getter with the mapper
		switch srcField.Kind {
		case KindString:
			field.GetString = func(t *T) string {
				return srcField.GetString(mapper(t))
			}
		case KindInt:
			field.GetInt = func(t *T) int {
				return srcField.GetInt(mapper(t))
			}
		case KindFloat:
			field.GetFloat = func(t *T) float64 {
				return srcField.GetFloat(mapper(t))
			}
		case KindCustom:
			field.GetCustom = func(buf []byte, t *T) []byte {
				return srcField.GetCustom(buf, mapper(t))
			}
		}

		dest.fields[name] = field
		dest.index[strings.ToLower(name)] = name
		dest.fieldOrder = append(dest.fieldOrder, name)
	}
}

// ListFields returns all registered field names.
//
// By default, fields are returned in insertion order. If sorted is true,
// they are returned in alphabetical order instead.
func (r *Registry[T]) ListFields(sorted bool) []string {
	if !sorted {
		// Return in insertion order
		names := make([]string, len(r.fieldOrder))
		copy(names, r.fieldOrder)
		return names
	}

	// Return sorted
	names := make([]string, len(r.fieldOrder))
	copy(names, r.fieldOrder)
	sort.Strings(names)
	return names
}

// ListCollections returns all collection names in alphabetical order.
func (r *Registry[T]) ListCollections() []string {
	names := make([]string, 0, len(r.collections))
	for name := range r.collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PrintHelp writes formatted help for all fields to w.
//
// If collection is non-empty, only fields in that collection are shown.
// For hierarchical registries, sub-registries are shown as separate sections.
func (r *Registry[T]) PrintHelp(w io.Writer, collection string) {
	var fieldsToShow []string

	if collection != "" {
		if fields, ok := r.collections[collection]; ok {
			fieldsToShow = fields
		} else {
			fmt.Fprintf(w, "Unknown collection: %s\n", collection)
			return
		}
	} else {
		fieldsToShow = r.ListFields(false) // preserve insertion order
	}

	// Print fields from this registry
	if len(fieldsToShow) > 0 {
		sectionName := r.name
		if sectionName == "" {
			sectionName = "General"
		}
		fmt.Fprintf(w, "\n%s:\n", sectionName)

		// Collect fields to display
		fields := make([]Field[T], 0, len(fieldsToShow))
		for _, name := range fieldsToShow {
			if f, ok := r.fields[name]; ok {
				fields = append(fields, f)
			}
		}

		// Calculate column widths
		maxName := len("Field")
		maxDisplay := len("Display")
		for _, f := range fields {
			if len(f.Name) > maxName {
				maxName = len(f.Name)
			}
			if len(f.Display) > maxDisplay {
				maxDisplay = len(f.Display)
			}
		}

		// Print header
		fmt.Fprintf(w, "  %-*s  %-*s  %s\n", maxName, "Field", maxDisplay, "Display", "Description")

		// Print fields in order
		for _, f := range fields {
			fmt.Fprintf(w, "  %-*s  %-*s  %s\n", maxName, f.Name, maxDisplay, f.Display, f.Description)
		}
	}

	// Print sub-registries
	for _, sub := range r.subRegistries {
		sub.PrintHelp(w, collection)
	}

	// Show collections if no specific collection requested and this is root registry
	if collection == "" && len(r.collections) > 0 && r.name == "" {
		fmt.Fprintf(w, "\nCollections:\n")
		for _, name := range r.ListCollections() {
			def := r.defaults[name]
			if def == "" {
				def = "(no default)"
			}
			fmt.Fprintf(w, "  @%-15s  Default: %s\n", name, def)
		}
	}
}

// get retrieves a field by name (case-insensitive).
// Searches this registry and all sub-registries.
func (r *Registry[T]) get(name string) (Field[T], bool) {
	// Try exact match first
	if f, ok := r.fields[name]; ok {
		return f, true
	}

	// Try case-insensitive
	if canonical, ok := r.index[strings.ToLower(name)]; ok {
		return r.fields[canonical], true
	}

	// Search sub-registries
	for _, sub := range r.subRegistries {
		if f, ok := sub.get(name); ok {
			return f, ok
		}
	}

	var zero Field[T]
	return zero, false
}

// FieldBuilder provides a fluent API for field construction.
type FieldBuilder[T any] struct {
	registry *Registry[T]
	field    Field[T]
}

// Width sets the column width in characters.
func (b *FieldBuilder[T]) Width(w int) *FieldBuilder[T] {
	b.field.Width = w
	return b
}

// String configures this field as a string type.
//
// The provided function extracts the string value from the object.
func (b *FieldBuilder[T]) String(fn func(*T) string) *FieldBuilder[T] {
	b.field.Kind = KindString
	b.field.GetString = fn
	return b
}

// Int configures this field as an integer type.
//
// The provided function extracts the int value from the object.
func (b *FieldBuilder[T]) Int(fn func(*T) int) *FieldBuilder[T] {
	b.field.Kind = KindInt
	b.field.GetInt = fn
	return b
}

// Float configures this field as a floating-point type.
//
// The precision parameter specifies the number of decimal places (e.g., 2 for "3.14").
func (b *FieldBuilder[T]) Float(precision int, fn func(*T) float64) *FieldBuilder[T] {
	b.field.Kind = KindFloat
	b.field.Precision = precision
	b.field.GetFloat = fn
	return b
}

// Custom configures this field with a custom formatter.
//
// The formatter appends formatted bytes to dst and returns the result.
// This allows for complex formatting logic (e.g., timestamps, byte counts).
//
// Example:
//
//	Custom(func(dst []byte, p *Person) []byte {
//	    return append(dst, formatTimestamp(p.Created)...)
//	})
func (b *FieldBuilder[T]) Custom(fn func(dst []byte, v *T) []byte) *FieldBuilder[T] {
	b.field.Kind = KindCustom
	b.field.GetCustom = fn
	return b
}

// Register adds this field to the registry.
//
// This is the final step in the builder chain.
func (b *FieldBuilder[T]) Register() {
	name := b.field.Name
	b.registry.fields[name] = b.field
	b.registry.index[strings.ToLower(name)] = name

	// Track insertion order
	b.registry.fieldOrder = append(b.registry.fieldOrder, name)
}
