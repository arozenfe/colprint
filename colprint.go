// Package colprint provides high-performance, zero-allocation column formatting
// for tabular data output in Go.
//
// # Overview
//
// colprint allows you to define fields for your struct types, compile field
// specifications into optimized write programs, and format millions of rows
// with zero allocations in the hot path.
//
// # Key Features
//
//   - Type-safe using Go generics
//   - Zero allocations in formatting hot path
//   - Single syscall per row with line buffering
//   - Support for collections and default field sets
//   - Custom formatters for complex types
//   - Suitable for streaming and batch processing
//
// # Basic Usage
//
//	// 1. Define your type
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//
//	// 2. Create registry and register fields
//	reg := colprint.NewRegistry[Person]()
//	reg.Field("name", "Name", "Person's name").
//	    Width(15).
//	    String(func(p *Person) string { return p.Name }).
//	    Register()
//
//	reg.Field("age", "Age", "Age in years").
//	    Width(5).
//	    Int(func(p *Person) int { return p.Age }).
//	    Register()
//
//	// 3. Compile a field specification
//	prog, _ := colprint.Compile(reg, "name,age")
//
//	// 4. Format rows (zero allocations)
//	line := make([]byte, 0, 256)
//	tmp := make([]byte, 0, 64)
//
//	prog.WriteHeader(os.Stdout, &line)
//	for i := range people {
//	    prog.WriteRow(os.Stdout, &people[i], &tmp, &line)
//	}
//
// # Collections
//
// Group related fields into named collections:
//
//	reg.DefineCollection("basic", "name,age", "name", "age", "email")
//	reg.SetDefaults("basic", "name,age")
//
//	// Use @collection syntax in specs
//	prog, _ := colprint.Compile(reg, "@basic,custom_field")
//
// # Performance
//
// The library is designed for maximum performance:
//
//   - All field resolution happens once during Compile()
//   - Formatting uses typed closures (no interface dispatch)
//   - Buffers are reused across rows (caller-provided)
//   - No reflection in hot path
//
// Expected performance: 1M+ rows/sec for typical workloads.
//
// # Current Limitations
//
// This initial version supports left-alignment only. Future versions will
// add right-alignment support for numeric fields.
package colprint

import (
	"io"
)

// Kind represents the data type of a field.
type Kind int

const (
	// KindString indicates a string field.
	KindString Kind = iota + 1
	// KindInt indicates an integer field.
	KindInt
	// KindFloat indicates a floating-point field.
	KindFloat
	// KindCustom indicates a custom formatter function.
	KindCustom
)

// Field describes how to extract and format a field from type T.
//
// Fields are created using the Registry.Field() builder pattern, not
// by constructing this struct directly.
type Field[T any] struct {
	// Name is the unique identifier for this field
	Name string

	// Display is the text shown in the column header
	Display string

	// Description provides help text for this field
	Description string

	// Width is the column width in characters
	Width int

	// Kind indicates the data type (String, Int, Float, Custom)
	Kind Kind

	// Precision specifies decimal places for Float fields
	Precision int

	// Value extractors - only one should be set based on Kind
	GetString func(*T) string
	GetInt    func(*T) int
	GetFloat  func(*T) float64
	GetCustom func(dst []byte, v *T) []byte

	// Future: Align field will be added here for Phase 2
	// Align Align  // Left or Right alignment
}

// Options configures program compilation.
type Options struct {
	// Separator is inserted between columns (default: "  ")
	Separator string

	// NoPadding disables all column padding (useful for CSV)
	NoPadding bool

	// PadLastColumn pads the last column to its width (default: false)
	// Setting to false avoids trailing spaces
	PadLastColumn bool

	// NoHeader skips header line generation
	NoHeader bool

	// NoUnderline skips underline generation
	NoUnderline bool
}

// compiledCol is an optimized, type-specialized column writer.
type compiledCol[T any] struct {
	width int
	// Future: align field will be added here
	write func(line *[]byte, v *T, tmp *[]byte)
}

// Program is a compiled, optimized formatting plan for type T.
//
// Programs are created by Compile() and can be reused for formatting
// millions of rows with zero allocations.
type Program[T any] struct {
	header    []byte
	underline []byte
	separator []byte
	columns   []compiledCol[T]
}

// WriteHeader writes the column headers to w.
//
// The line buffer is used for temporary storage and reused across calls.
// It should have adequate capacity (typically 256 bytes).
func (p *Program[T]) WriteHeader(w io.Writer, line *[]byte) error {
	*line = append((*line)[:0], p.header...)
	*line = append(*line, '\n')
	_, err := w.Write(*line)
	return err
}

// WriteUnderline writes the header underline to w.
//
// The underline uses dashes under text and spaces elsewhere.
func (p *Program[T]) WriteUnderline(w io.Writer, line *[]byte) error {
	*line = append((*line)[:0], p.underline...)
	*line = append(*line, '\n')
	_, err := w.Write(*line)
	return err
}

// WriteRow formats and writes a single row to w.
//
// This is the hot path - designed for zero allocations and maximum speed.
// Buffers tmp and line are reused across calls and should have adequate
// capacity (typically 64 and 256 bytes respectively).
//
// The tmp buffer is used for formatting individual values. The line buffer
// accumulates the complete row before writing.
func (p *Program[T]) WriteRow(w io.Writer, v *T, tmp, line *[]byte) error {
	*line = (*line)[:0]
	for i := range p.columns {
		if i > 0 {
			*line = append(*line, p.separator...)
		}
		p.columns[i].write(line, v, tmp)
	}
	*line = append(*line, '\n')
	_, err := w.Write(*line)
	return err
}

// HeaderString returns the header as a string.
func (p *Program[T]) HeaderString() string {
	return string(p.header)
}

// FormatRow formats a row and returns it as a string.
//
// This is less efficient than WriteRow as it allocates a string.
// Prefer WriteRow for high-volume output.
func (p *Program[T]) FormatRow(v *T, tmp, line *[]byte) string {
	*line = (*line)[:0]
	for i := range p.columns {
		if i > 0 {
			*line = append(*line, p.separator...)
		}
		p.columns[i].write(line, v, tmp)
	}
	return string(*line)
}
