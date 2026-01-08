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
type Registry[T any] struct {
	fields      map[string]Field[T]
	index       map[string]string // lowercase -> canonical name
	collections map[string][]string
	defaults    map[string]string
	categories  map[string][]string
}

// NewRegistry creates a new field registry for type T.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		fields:      make(map[string]Field[T]),
		index:       make(map[string]string),
		collections: make(map[string][]string),
		defaults:    make(map[string]string),
		categories:  make(map[string][]string),
	}
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

// ListFields returns all registered field names in alphabetical order.
func (r *Registry[T]) ListFields() []string {
	names := make([]string, 0, len(r.fields))
	for name := range r.fields {
		names = append(names, name)
	}
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
		fieldsToShow = r.ListFields()
	}

	// Group by category
	byCat := make(map[string][]Field[T])
	for _, name := range fieldsToShow {
		if f, ok := r.fields[name]; ok {
			cat := f.Category
			if cat == "" {
				cat = "General"
			}
			byCat[cat] = append(byCat[cat], f)
		}
	}

	// Sort categories
	cats := make([]string, 0, len(byCat))
	for cat := range byCat {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	// Print each category
	for _, cat := range cats {
		fmt.Fprintf(w, "\n%s:\n", cat)
		fields := byCat[cat]

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

		// Print fields
		for _, f := range fields {
			fmt.Fprintf(w, "  %-*s  %-*s  %s\n", maxName, f.Name, maxDisplay, f.Display, f.Description)
		}
	}

	// Show collections if no specific collection requested
	if collection == "" && len(r.collections) > 0 {
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
func (r *Registry[T]) get(name string) (Field[T], bool) {
	// Try exact match first
	if f, ok := r.fields[name]; ok {
		return f, true
	}

	// Try case-insensitive
	if canonical, ok := r.index[strings.ToLower(name)]; ok {
		return r.fields[canonical], true
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

// Category sets the category for help organization.
func (b *FieldBuilder[T]) Category(cat string) *FieldBuilder[T] {
	b.field.Category = cat
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

	// Add to category
	cat := b.field.Category
	if cat == "" {
		cat = "General"
	}
	b.registry.categories[cat] = append(b.registry.categories[cat], name)
}
