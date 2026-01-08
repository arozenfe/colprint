package colprint

import (
	"fmt"
	"strconv"
	"strings"
)

// Compile creates an optimized formatting program from a field specification.
//
// The spec is a comma-separated list of field names, with optional features:
//   - Field width override: "name:20" sets width to 20
//   - Default expansion: "@default" expands to collection's default fields
//   - Collection expansion: "@collection_name" expands to collection fields
//
// Examples:
//
//	Compile(reg, "name,age,email")
//	Compile(reg, "name:20,age:5,email:30")
//	Compile(reg, "@default,extra_field")
//	Compile(reg, "@basic,@perf")
//
// Returns an error if any field name is invalid or a collection doesn't exist.
func Compile[T any](reg *Registry[T], spec string) (*Program[T], error) {
	return CompileWithOptions(reg, spec, Options{
		Separator: "  ", // Default: two spaces between columns
	})
}

// CompileWithOptions creates a program with custom options.
func CompileWithOptions[T any](reg *Registry[T], spec string, opts Options) (*Program[T], error) {
	if spec == "" {
		return nil, fmt.Errorf("empty field specification")
	}

	// Parse spec into field list
	fields, err := parseSpec(reg, spec)
	if err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields specified")
	}

	// Set defaults - separator can be empty string (no spacing)
	sep := opts.Separator

	p := &Program[T]{
		separator: []byte(sep),
	}

	// Build header and underline
	if !opts.NoHeader {
		p.header = buildHeader(fields, sep, opts.NoPadding, opts.PadLastColumn)
	}
	if !opts.NoUnderline {
		p.underline = buildUnderline(p.header)
	}

	// Build optimized column writers
	p.columns = make([]compiledCol[T], len(fields))
	lastIdx := len(fields) - 1
	for i, f := range fields {
		isLast := i == lastIdx
		noPad := opts.NoPadding || (isLast && !opts.PadLastColumn)
		p.columns[i] = makeWriter(f, noPad)
	}

	return p, nil
}

// parseSpec parses a field specification string.
func parseSpec[T any](reg *Registry[T], spec string) ([]Field[T], error) {
	tokens := strings.Split(spec, ",")
	var fields []Field[T]

	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}

		// Check for @ prefix (collection or @default)
		if strings.HasPrefix(tok, "@") {
			name := tok[1:]

			// Handle @default
			if name == "default" {
				// Find the default spec
				// For now, use first defined default
				for _, defSpec := range reg.defaults {
					expanded, err := parseSpec(reg, defSpec)
					if err != nil {
						return nil, fmt.Errorf("expanding @default: %w", err)
					}
					fields = append(fields, expanded...)
					break
				}
				continue
			}

			// Handle @collection
			if defSpec, ok := reg.defaults[name]; ok {
				expanded, err := parseSpec(reg, defSpec)
				if err != nil {
					return nil, fmt.Errorf("expanding @%s: %w", name, err)
				}
				fields = append(fields, expanded...)
				continue
			}

			return nil, fmt.Errorf("unknown collection: @%s", name)
		}

		// Parse field name and optional width override
		fieldName, width, hasWidth, err := parseFieldSpec(tok)
		if err != nil {
			return nil, err
		}

		// Look up field
		field, ok := reg.get(fieldName)
		if !ok {
			return nil, fmt.Errorf("unknown field: %q", fieldName)
		}

		// Apply width override
		if hasWidth {
			if width <= 0 {
				return nil, fmt.Errorf("invalid width %d for field %q", width, fieldName)
			}
			field.Width = width
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// parseFieldSpec parses a single field token (name or name:width).
func parseFieldSpec(tok string) (name string, width int, hasWidth bool, err error) {
	idx := strings.IndexByte(tok, ':')
	if idx < 0 {
		return strings.TrimSpace(tok), 0, false, nil
	}

	name = strings.TrimSpace(tok[:idx])
	widthStr := strings.TrimSpace(tok[idx+1:])

	if widthStr == "" {
		return "", 0, false, fmt.Errorf("empty width in %q", tok)
	}

	width, err = strconv.Atoi(widthStr)
	if err != nil {
		return "", 0, false, fmt.Errorf("invalid width %q in %q", widthStr, tok)
	}

	return name, width, true, nil
}

// buildHeader constructs the header line.
func buildHeader[T any](fields []Field[T], sep string, noPadding, padLast bool) []byte {
	var buf []byte
	lastIdx := len(fields) - 1
	for i, f := range fields {
		if i > 0 {
			buf = append(buf, sep...)
		}
		isLast := i == lastIdx
		if noPadding || (isLast && !padLast) {
			// No padding for this column
			display := f.Display
			if len(display) > f.Width {
				display = display[:f.Width]
			}
			buf = append(buf, display...)
		} else {
			// Pad column to width
			buf = padLeft(buf, f.Display, f.Width)
		}
	}
	return buf
}

// buildUnderline creates an underline matching the header.
func buildUnderline(header []byte) []byte {
	underline := make([]byte, len(header))
	for i, ch := range header {
		if ch == ' ' {
			underline[i] = ' '
		} else {
			underline[i] = '-'
		}
	}
	return underline
}

// makeWriter creates an optimized writer closure for a field.
func makeWriter[T any](f Field[T], noPad bool) compiledCol[T] {
	switch f.Kind {
	case KindString:
		// String field - left-aligned
		// Always pad all columns (removed isLast check)
		return compiledCol[T]{
			width: f.Width,
			write: func(line *[]byte, v *T, _ *[]byte) {
				s := f.GetString(v)
				*line = padLeft(*line, s, f.Width)
			},
		}

	case KindInt:
		// Int field - left-aligned (Phase 2 will add right-align)
		// Always pad all columns
		return compiledCol[T]{
			width: f.Width,
			write: func(line *[]byte, v *T, tmp *[]byte) {
				*tmp = (*tmp)[:0]
				*tmp = strconv.AppendInt(*tmp, int64(f.GetInt(v)), 10)
				*line = padBytesLeft(*line, *tmp, f.Width)
			},
		}

	case KindFloat:
		// Float field - left-aligned (Phase 2 will add right-align)
		// Always pad all columns
		prec := f.Precision
		if prec < 0 {
			prec = 2
		}
		return compiledCol[T]{
			width: f.Width,
			write: func(line *[]byte, v *T, tmp *[]byte) {
				*tmp = (*tmp)[:0]
				*tmp = strconv.AppendFloat(*tmp, f.GetFloat(v), 'f', prec, 64)
				*line = padBytesLeft(*line, *tmp, f.Width)
			},
		}

	case KindCustom:
		// Custom formatter - left-aligned
		// Always pad all columns
		return compiledCol[T]{
			width: f.Width,
			write: func(line *[]byte, v *T, tmp *[]byte) {
				*tmp = (*tmp)[:0]
				*tmp = f.GetCustom(*tmp, v)
				*line = padBytesLeft(*line, *tmp, f.Width)
			},
		}

	default:
		// Unknown kind - emit spaces
		return compiledCol[T]{
			width: f.Width,
			write: func(line *[]byte, _ *T, _ *[]byte) {
				for i := 0; i < f.Width; i++ {
					*line = append(*line, ' ')
				}
			},
		}
	}
}
