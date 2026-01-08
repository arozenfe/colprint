# Column Printing Library - Design Document

**Version:** 1.0  
**Date:** January 8, 2026  
**Status:** Proposed Design

## Table of Contents

1. [Overview](#overview)
2. [Requirements](#requirements)
3. [Design Goals](#design-goals)
4. [Architecture](#architecture)
5. [API Design](#api-design)
6. [Implementation Details](#implementation-details)
7. [Performance Considerations](#performance-considerations)
8. [Usage Examples](#usage-examples)
9. [Trade-offs and Decisions](#trade-offs-and-decisions)
10. [Future Enhancements](#future-enhancements)

---

## Overview

This document describes the design for `colprint`, a high-performance, zero-allocation column formatting library for Go. The library enables fast tabular output formatting with support for millions of rows with minimal overhead.

### Purpose

- Format structured data (structs) as aligned columns
- Support streaming data (infinite rows) and batch processing (sorted lists)
- Provide maximum performance with zero allocations in hot path
- Be generic enough to use across multiple projects
- Eventually be published as a public package

### Key Features

- **Zero-allocation hot path:** No memory allocations when printing rows
- **Type-safe:** Compile-time type checking with generics
- **Fast:** Single syscall per row with line buffering
- **Flexible:** Support multiple output formats (fixed-width, CSV, custom separators)
- **No reflection in hot path:** All reflection done during initialization
- **Generic:** Works with any struct type

---

## Requirements

### Functional Requirements

#### FR1: Field Definition and Registration
- Each field must have:
  - **Name:** Unique identifier for field selection
  - **Display:** Column header text
  - **Width:** Default column width (overridable)
  - **Alignment:** Left or right alignment
  - **Type:** String, Int, Float, or Custom formatter
  - **Extractor:** Function to get value from struct

#### FR2: Field Collections
- Group related fields into named collections
- Support multiple collections per struct type (e.g., "general", "memory", "cpu", "io")
- Define default field set per collection
- Collections should support categorization for help/documentation

#### FR3: Field Selection
- Parse comma-separated field specification strings
- Support field width override: `"field:width"`
- Support default field set keyword: `"@default"` or `"field1,@default,field2"`
- Support collection references: `"@general,@cpu"` or `"field1,@memory"`
- Case-insensitive field matching
- `@` prefix clearly distinguishes collections from field names

#### FR4: Output Modes
- **Fixed-width:** Space-separated, aligned columns
- **Delimited:** Custom separator (CSV, TSV, etc.)
- **Header control:** Caller decides when/if to print headers
- **Underline:** Optional header underlines for readability

#### FR5: Value Formatting
- **String:** Direct string output, padded/truncated
- **Int:** Base-10 integer formatting
- **Float:** Configurable precision (e.g., 2 decimal places)
- **Custom:** User-provided formatter functions
- Support for common formatters (timestamps, byte counts, percentages)

#### FR6: Performance
- Zero allocations in row printing hot path
- Single syscall per row (line buffering)
- Pre-compiled write plan (no runtime decisions)
- Support for millions of rows

### Non-Functional Requirements

#### NFR1: Output Control
- Caller provides `io.Writer` for output destination
- Support `os.File`, `bytes.Buffer`, network connections, etc.
- Caller provides reusable buffers for zero-allocation guarantee

#### NFR2: Application Separation
- **Library responsibility:** Format and write rows only
- **Application responsibility:** 
  - Data acquisition and filtering
  - Sorting (if needed)
  - Header printing decisions
  - Buffer management

#### NFR3: API Design
- Simple common case: minimal boilerplate
- Type-safe: compile-time checking where possible
- Self-documenting: clear method names
- No interface overhead in hot path

#### NFR4: Extensibility
- Easy to add new field types
- Support custom formatters
- Pluggable value extractors

---

## Design Goals

### Primary Goals

1. **Maximum Performance**
   - Zero allocations in hot path
   - Minimal CPU overhead
   - Cache-friendly data structures
   - No reflection during row formatting

2. **Type Safety**
   - Generic programming with Go 1.18+ generics
   - Compile-time type checking
   - No `interface{}` or `any` in hot path

3. **Simplicity**
   - Intuitive API for common cases
   - Advanced features available but not required
   - Clear separation of concerns

4. **Flexibility**
   - Support various output formats
   - Customizable formatting
   - Works with any struct type

### Secondary Goals

5. **Reusability**
   - Package-level design for use across projects
   - Eventually publish as public library
   - No project-specific dependencies

6. **Maintainability**
   - Clear code structure
   - Well-documented decisions
   - Testable components

---

## Architecture

### Component Overview

```
┌────────────────────────────────────────────────────────────┐
│                    Application Code                        │
│  (Data loading, filtering, sorting, header decisions)      │
└────────────────────────────────────────────────────────────┘
                            ↓
┌────────────────────────────────────────────────────────────┐
│                    Registry[T]                             │
│  - Field definitions for type T                            │
│  - Named collections (groups of fields)                    │
│  - Default field sets                                      │
│  - Case-insensitive lookup                                 │
└────────────────────────────────────────────────────────────┘
                            ↓
┌────────────────────────────────────────────────────────────┐
│                    Compiler                                │
│  - Parses field specification string                       │
│  - Resolves field names and widths                         │
│  - Expands "default" and collection keywords               │
│  - Validates field existence                               │
└────────────────────────────────────────────────────────────┘
                            ↓
┌────────────────────────────────────────────────────────────┐
│                    Program[T]                              │
│  - Pre-compiled write plan                                 │
│  - Typed closures (no interfaces)                          │
│  - Pre-built header bytes                                  │
│  - Fast row formatting methods                             │
└────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Setup Phase** (once per field specification):
   ```
   Register fields → Parse spec → Compile program
   ```

2. **Execution Phase** (per row):
   ```
   Call WriteRow → Execute closures → Write to buffer → Flush to io.Writer
   ```

### Key Design Decisions

#### D1: Generic Types
Use Go generics to avoid `interface{}` overhead:
```go
type Registry[T any]
type Program[T any]  
type Field[T any]
```

#### D2: Closure-Based Dispatch
Instead of interface dispatch, use typed closures:
```go
type compiledCol[T any] struct {
    write func(line *[]byte, v *T, tmp *[]byte)  // Typed, no vtable lookup
}
```

#### D3: Pre-compilation
All decisions made once during `Compile()`, not per-row:
- Field resolution
- Width calculations
- Format string building
- Function pointer binding

#### D4: Caller-Managed Buffers
Buffers passed by reference from caller:
- Zero allocations
- Thread-safe (each thread has own buffers)
- Complete control over buffer sizes

---

## API Design

### Core Types

```go
package colprint

// Alignment for column output
type Align int
const (
    AlignLeft Align = iota
    AlignRight
)

// Kind represents the data type of a field
type Kind int
const (
    KindString Kind = iota + 1
    KindInt
    KindFloat
    KindCustom
)

// Field describes how to extract and format a field from type T
type Field[T any] struct {
    Name        string              // Unique identifier
    Display     string              // Column header text
    Description string              // For help/documentation
    Width       int                 // Default column width
    Align       Align               // Left or right alignment
    Kind        Kind                // Data type
    Precision   int                 // For floats (decimal places)
    
    // Value extractors (only one should be set based on Kind)
    GetString   func(*T) string
    GetInt      func(*T) int
    GetFloat    func(*T) float64
    GetCustom   func(dst []byte, v *T) []byte
}

// Registry stores all fields for a type T
type Registry[T any] struct {
    fields      map[string]Field[T]           // canonical name -> field
    index       map[string]string             // lowercase name -> canonical
    collections map[string][]string           // collection name -> field names
    defaults    map[string]string             // collection name -> default spec
    categories  map[string][]string           // category name -> field names
}

// Program is a compiled, optimized write plan
type Program[T any] struct {
    header    []byte                // Pre-built header line
    underline []byte                // Pre-built underline
    separator []byte                // Column separator
    columns   []compiledCol[T]      // Per-column writers
}

// compiledCol is an optimized column writer (private)
type compiledCol[T any] struct {
    width int
    align Align
    write func(line *[]byte, v *T, tmp *[]byte)  // Typed closure
}
```

### Registry API

```go
// Create a new registry for type T
func NewRegistry[T any]() *Registry[T]

// Register a field with builder pattern
func (r *Registry[T]) Field(name, display, description string) *FieldBuilder[T]

// FieldBuilder provides type-safe field construction
type FieldBuilder[T any] struct {
    registry *Registry[T]
    field    Field[T]
}

// Type-specific methods (each sets Kind internally)
func (b *FieldBuilder[T]) String(fn func(*T) string) *FieldBuilder[T]
func (b *FieldBuilder[T]) Int(fn func(*T) int) *FieldBuilder[T]
func (b *FieldBuilder[T]) Float(precision int, fn func(*T) float64) *FieldBuilder[T]
func (b *FieldBuilder[T]) Custom(fn func([]byte, *T) []byte) *FieldBuilder[T]

// Configuration methods
func (b *FieldBuilder[T]) Width(w int) *FieldBuilder[T]
func (b *FieldBuilder[T]) Align(a Align) *FieldBuilder[T]
func (b *FieldBuilder[T]) Category(name string) *FieldBuilder[T]

// Finalize the field
func (b *FieldBuilder[T]) Register()

// Collection management
func (r *Registry[T]) DefineCollection(name string, defaultSpec string, fields ...string)
func (r *Registry[T]) SetDefaults(collectionName, spec string)

// Help and introspection
func (r *Registry[T]) ListFields() []string
func (r *Registry[T]) ListCollections() []string
func (r *Registry[T]) PrintHelp(w io.Writer, collection string)
```

### Program API

```go
// Compile a field specification into an optimized program
func Compile[T any](reg *Registry[T], spec string) (*Program[T], error)

// Compile with options
func CompileWithOptions[T any](reg *Registry[T], spec string, opts Options) (*Program[T], error)

type Options struct {
    Separator   string  // Column separator (default: "  ")
    NoHeader    bool    // Skip header generation
    NoUnderline bool    // Skip underline generation
}

// Write methods (fast path - caller provides buffers)
func (p *Program[T]) WriteHeader(w io.Writer, line *[]byte) error
func (p *Program[T]) WriteUnderline(w io.Writer, line *[]byte) error
func (p *Program[T]) WriteRow(w io.Writer, v *T, tmp, line *[]byte) error

// Convenience methods (slower - allocates buffers)
func (p *Program[T]) WriteRowSimple(w io.Writer, v *T) error

// Get formatted string (no I/O)
func (p *Program[T]) FormatRow(v *T, tmp, line *[]byte) string
func (p *Program[T]) HeaderString() string
```

---

## Implementation Details

### Field Registration

```go
reg := colprint.NewRegistry[Person]()

// Simple field registration
reg.Field("name", "Name", "Person's full name").
    Width(15).
    String((*Person).GetName).
    Register()

// Numeric field with alignment
reg.Field("age", "Age", "Age in years").
    Width(3).
    Align(colprint.AlignRight).
    Int((*Person).GetAge).
    Register()

// Float with precision
reg.Field("height", "Height", "Height in cm").
    Width(6).
    Align(colprint.AlignRight).
    Float(1, (*Person).GetHeight).
    Register()

// Custom formatter
reg.Field("height_imperial", "Height", "Height in ft/in").
    Width(8).
    Align(colprint.AlignRight).
    Custom(formatImperial).
    Register()

// Categories for help organization
reg.Field("cpu", "CPU%", "CPU usage percentage").
    Width(6).
    Align(colprint.AlignRight).
    Category("Performance").
    Float(1, (*Process).GetCPU).
    Register()
```

### Collections

```go
// Define collections of fields
reg.DefineCollection("general", "pid,name,user", "pid", "name", "user", "state")
reg.DefineCollection("memory", "vmem,rmem", "vmem", "rmem", "vgrow", "rgrow")
reg.DefineCollection("cpu", "cpu,ucpu,scpu", "cpu", "ucpu", "scpu", "utime", "stime")
reg.DefineCollection("io", "ioread,iowrite", "ioread", "iowrite", "rio", "wio")

// Set default for the main collection
reg.SetDefaults("general", "pid,name,cpu,rmem")

// Usage in specs:
// @general  → expands to default: pid,name,cpu,rmem
// @cpu      → expands to default: cpu,ucpu,scpu
// @default  → expands to current collection's default
```

### Compilation

```go
// Simple spec
prog, err := colprint.Compile(reg, "name,age,height")

// With width overrides
prog, err := colprint.Compile(reg, "name:20,age:5,height")

// Using defaults
prog, err := colprint.Compile(reg, "@default,state")  // Expands default

// Using collections
prog, err := colprint.Compile(reg, "@general,@memory")  // Expands both

// Mixed
prog, err := colprint.Compile(reg, "name,@default,custom_field:10")

// Collection with width override
prog, err := colprint.Compile(reg, "@cpu,memory:15")

// With options
opts := colprint.Options{Separator: ",", NoUnderline: true}
prog, err := colprint.CompileWithOptions(reg, "name,age", opts)
```

### Writing Rows

```go
// Fast path - zero allocations
line := make([]byte, 0, 256)
tmp := make([]byte, 0, 64)

// Print header once
prog.WriteHeader(os.Stdout, &line)
prog.WriteUnderline(os.Stdout, &line)

// Print rows (millions possible)
for i := range people {
    prog.WriteRow(os.Stdout, &people[i], &tmp, &line)
}

// Or with streaming
for person := range personChannel {
    prog.WriteRow(os.Stdout, &person, &tmp, &line)
}
```

### Spec Parsing

The spec parser handles:

1. **Split by comma:** `"field1,field2,field3"`
2. **Width override:** `"field:10"` → field with width 10
3. **Default expansion:** `"@default"` → replaced with default spec for current collection
4. **Collection expansion:** `"@cpu"` → replaced with collection fields
5. **Case-insensitive matching:** `"NAME"` matches `"name"`
6. **Validation:** Unknown fields return error
7. **`@` prefix:** Clearly identifies collection/default references vs field names

Algorithm:
```
For each token in comma-separated spec:
    1. Trim whitespace
    2. Check for width override (":N")
    3. If token starts with '@':
        a. Remove '@' prefix
        b. If token == "default", expand to registry.defaults[currentCollection]
        c. Else if token matches collection name, expand to collection fields
        d. Else return error (unknown collection)
    4. Otherwise, lookup field in registry (case-insensitive)
    5. Apply width override if present
    6. Add to compiled program
```

### Closure Generation

The magic happens in `makeWriter()`:

```go
func makeWriter[T any](f Field[T]) compiledCol[T] {
    switch f.Kind {
    case KindString:
        // Generate closure for string fields
        if f.Align == AlignLeft {
            return compiledCol[T]{
                width: f.Width,
                align: f.Align,
                write: func(line *[]byte, v *T, _ *[]byte) {
                    s := f.GetString(v)
                    *line = padLeft(*line, s, f.Width)
                },
            }
        } else {
            return compiledCol[T]{
                width: f.Width,
                align: f.Align,
                write: func(line *[]byte, v *T, _ *[]byte) {
                    s := f.GetString(v)
                    *line = padRight(*line, s, f.Width)
                },
            }
        }
    
    case KindInt:
        // Generate closure for int fields
        if f.Align == AlignLeft {
            return compiledCol[T]{
                width: f.Width,
                align: f.Align,
                write: func(line *[]byte, v *T, tmp *[]byte) {
                    *tmp = (*tmp)[:0]
                    *tmp = strconv.AppendInt(*tmp, int64(f.GetInt(v)), 10)
                    *line = padLeft(*line, *tmp, f.Width)
                },
            }
        } else {
            return compiledCol[T]{
                width: f.Width,
                align: f.Align,
                write: func(line *[]byte, v *T, tmp *[]byte) {
                    *tmp = (*tmp)[:0]
                    *tmp = strconv.AppendInt(*tmp, int64(f.GetInt(v)), 10)
                    *line = padRight(*line, *tmp, f.Width)
                },
            }
        }
    
    case KindFloat:
        // Similar for float...
    
    case KindCustom:
        // User-provided formatter
        return compiledCol[T]{
            width: f.Width,
            align: f.Align,
            write: func(line *[]byte, v *T, tmp *[]byte) {
                *tmp = (*tmp)[:0]
                *tmp = f.GetCustom(*tmp, v)
                *line = padAlign(*line, *tmp, f.Width, f.Align)
            },
        }
    }
}
```

**Key points:**
- Each field type gets its own specialized closure
- Alignment decision baked into closure (no runtime branching)
- Value extractor (`f.GetString`, `f.GetInt`) captured in closure
- Width and alignment captured in closure
- No interface dispatch, no reflection

---

## Performance Considerations

### Zero-Allocation Guarantee

**What we avoid:**
- ❌ No `interface{}` boxing/unboxing
- ❌ No reflection in hot path
- ❌ No temporary strings
- ❌ No slice growth (pre-sized buffers)
- ❌ No map lookups per row

**How we achieve it:**
- ✅ Caller provides buffers (reused)
- ✅ Typed closures (no vtable)
- ✅ Pre-compiled write plan
- ✅ Buffer capacity planning
- ✅ Single syscall per row

### Buffer Sizing

Recommended buffer sizes:

```go
// For line buffer:
// max_line_width = sum(field_widths) + (num_fields-1) * len(separator) + 1
line := make([]byte, 0, 256)  // Typical: ~200 chars per line

// For tmp buffer (largest individual value):
// max_tmp = max(max_int_digits, max_float_chars, max_custom_output)
tmp := make([]byte, 0, 64)   // Typical: ~50 chars for formatted numbers
```

### Syscall Optimization

Line buffering ensures one syscall per row:

```go
// One syscall per WriteRow call
for _, row := range data {
    prog.WriteRow(os.Stdout, &row, &tmp, &line)
    // Internal: builds line buffer, writes once to os.Stdout
}
```

For even better performance, use `bufio.Writer`:

```go
buf := bufio.NewWriterSize(os.Stdout, 64*1024)  // 64KB buffer
for _, row := range data {
    prog.WriteRow(buf, &row, &tmp, &line)
}
buf.Flush()  // One syscall for entire batch
```

### Benchmark Targets

Expected performance (on modern hardware):

- **1 million rows/sec:** Simple fields (3-5 columns)
- **500k rows/sec:** Complex fields (10+ columns with formatters)
- **< 10ns overhead:** Per field (beyond value extraction)
- **Zero allocations:** In hot path (verified with profiler)

---

## Usage Examples

### Example 1: Simple Process Listing

```go
type Process struct {
    PID  int
    Name string
    CPU  float64
}

func (p *Process) GetPID() int        { return p.PID }
func (p *Process) GetName() string    { return p.Name }
func (p *Process) GetCPU() float64    { return p.CPU }

func main() {
    // Setup
    reg := colprint.NewRegistry[Process]()
    
    reg.Field("pid", "PID", "Process ID").
        Width(7).
        Align(colprint.AlignRight).
        Int((*Process).GetPID).
        Register()
    
    reg.Field("name", "Name", "Process name").
        Width(15).
        String((*Process).GetName).
        Register()
    
    reg.Field("cpu", "CPU%", "CPU usage").
        Width(6).
        Align(colprint.AlignRight).
        Float(1, (*Process).GetCPU).
        Register()
    
    // Compile
    prog, _ := colprint.Compile(reg, "pid,name,cpu")
    
    // Print
    line := make([]byte, 0, 128)
    tmp := make([]byte, 0, 32)
    
    prog.WriteHeader(os.Stdout, &line)
    prog.WriteUnderline(os.Stdout, &line)
    
    processes := []Process{
        {PID: 1234, Name: "systemd", CPU: 0.1},
        {PID: 5678, Name: "atop", CPU: 5.2},
    }
    
    for i := range processes {
        prog.WriteRow(os.Stdout, &processes[i], &tmp, &line)
    }
}
```

Output:
```
    PID Name              CPU%
   ---- -----------       ----
   1234 systemd            0.1
   5678 atop               5.2
```

### Example 2: Custom Formatters

```go
// Custom formatter for byte counts
func formatBytes(dst []byte, bytes int64) []byte {
    const unit = 1024
    if bytes < unit {
        dst = strconv.AppendInt(dst, bytes, 10)
        dst = append(dst, 'B')
        return dst
    }
    
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    
    val := float64(bytes) / float64(div)
    dst = strconv.AppendFloat(dst, val, 'f', 1, 64)
    dst = append(dst, " KMGTPE"[exp])
    dst = append(dst, 'B')
    return dst
}

// Register with custom formatter
reg.Field("memory", "Memory", "Memory usage").
    Width(10).
    Align(colprint.AlignRight).
    Custom(func(dst []byte, p *Process) []byte {
        return formatBytes(dst, p.MemoryBytes)
    }).
    Register()
```

### Example 3: Collections and Defaults

```go
// Define collections
reg.DefineCollection("basic", "pid,name,state", "pid", "name", "state", "ppid", "user")
reg.DefineCollection("perf", "cpu,mem", "cpu", "ucpu", "scpu", "mem", "rmem", "vmem")
reg.DefineCollection("io", "ioread,iowrite", "ioread", "iowrite", "rio", "wio")

reg.SetDefaults("basic", "pid,name,cpu,mem")

// Use collection with @prefix
prog, _ := colprint.Compile(reg, "@basic")        // Uses default: pid,name,cpu,mem

// Mix and match
prog, _ := colprint.Compile(reg, "@perf,state")   // Expands perf + adds state

// Use @default keyword
prog, _ := colprint.Compile(reg, "name,@default,ioread")  // name + (pid,name,cpu,mem) + ioread

// Multiple collections
prog, _ := colprint.Compile(reg, "@basic,@io")    // Expands both collections

// Collection with individual fields
prog, _ := colprint.Compile(reg, "pid,@perf,user:20")  // pid + perf fields + user with width 20
```

### Example 4: Streaming Data

```go
// Infinite stream processing
prog, _ := colprint.Compile(reg, "timestamp,level,message")

line := make([]byte, 0, 256)
tmp := make([]byte, 0, 64)

buf := bufio.NewWriterSize(os.Stdout, 64*1024)
prog.WriteHeader(buf, &line)

for logEntry := range logStream {  // Infinite channel
    prog.WriteRow(buf, &logEntry, &tmp, &line)
    
    // Periodic flush
    if counter++; counter%1000 == 0 {
        buf.Flush()
    }
}
```

---

## Trade-offs and Decisions

### Decision 1: Generics vs Reflection

**Choice:** Use Go generics for type safety

**Pros:**
- Compile-time type checking
- No runtime reflection overhead
- Better performance
- Clear type signatures

**Cons:**
- Requires Go 1.18+
- More code generation at compile time
- Cannot mix types in same registry

**Rationale:** Performance and type safety are critical. Reflection in hot path would add 10-50% overhead.

---

### Decision 2: Caller-Managed Buffers

**Choice:** Require caller to provide buffers

**Pros:**
- Zero allocations guarantee
- Maximum performance
- Thread-safe by design
- Predictable memory usage

**Cons:**
- More verbose API
- Caller responsibility
- Potential for errors

**Rationale:** Performance is primary goal. Can add convenience wrappers later for simpler use cases.

---

### Decision 3: Closure-Based Dispatch

**Choice:** Use typed closures instead of interfaces

**Pros:**
- No interface overhead
- Inlined by compiler
- Type-specialized code
- No vtable lookup

**Cons:**
- More complex implementation
- Larger binary size

**Rationale:** Interface dispatch adds 5-10ns per call. With millions of cells, this adds up to significant overhead.

---

### Decision 4: Pre-compilation

**Choice:** Compile spec once, execute many times

**Pros:**
- All decisions made upfront
- Hot path is trivial loop
- Easy to optimize
- Clear separation of phases

**Cons:**
- Two-phase API (compile + execute)
- Cannot change spec dynamically

**Rationale:** Parsing and resolution per-row would be catastrophically slow. This is the standard approach (like prepared statements in SQL).

---

### Decision 5: No Sorting in Library

**Choice:** Sorting is application responsibility

**Pros:**
- Separation of concerns
- Works with streaming
- Different sort needs per application
- Simpler library

**Cons:**
- Less "batteries included"
- Users must implement sorting

**Rationale:** Formatting library should format, not manipulate data. Sorting prevents streaming use cases.

---

### Decision 6: Header Control

**Choice:** Caller decides when to print headers

**Pros:**
- Supports all use cases (once, per-page, never)
- Works with streaming
- Application knows context

**Cons:**
- Must remember to call WriteHeader
- Not automatic

**Rationale:** Different use cases need different header behavior. Library can't guess correctly.

---

### Decision 7: Not Using fmt.Formatter/fmt.State

**Choice:** Use typed closures instead of fmt.Formatter interface

**Context:** Go's `fmt` package provides `fmt.Formatter` interface for custom formatting:
```go
type Formatter interface {
    Format(f State, verb rune)
}
```

**Pros of fmt.Formatter:**
- Standard Go interface
- Familiar to developers
- Types can control their own formatting

**Cons of fmt.Formatter:**
- ❌ Interface dispatch overhead (5-10ns per call)
- ❌ Not zero-allocation (fmt.Fprintf allocates)
- ❌ Works with strings, not []byte buffers
- ❌ Value receivers, we need pointer receivers
- ❌ Cannot be inlined by compiler
- ❌ Adds complexity to our simple model

**Performance Impact:**
```go
// Our approach (zero allocation, inlined):
write: func(line *[]byte, v *T, tmp *[]byte) {
    *tmp = (*tmp)[:0]
    *tmp = strconv.AppendInt(*tmp, int64(v.GetAge()), 10)
    *line = padRight(*line, *tmp, 6)
}
// ~10ns per cell

// With fmt.Formatter (allocates, interface dispatch):
write: func(line *[]byte, v *T, tmp *[]byte) {
    s := fmt.Sprintf("%d", v)  // allocates string
    *line = padRight(*line, s, 6)
}
// ~100ns per cell + allocations
```

With 10 million cells: 100ms vs 1000ms = **10x slowdown**

**Rationale:** Performance is critical. Interface overhead and allocations are unacceptable for our use case.

**However:** We could provide an **optional adapter** for convenience:

```go
// For types that implement fmt.Formatter (slower, but convenient)
reg.Field("formatted", "Custom", "Uses fmt.Formatter").
    Width(15).
    Formatter('v').  // Uses type's Format method
    Register()
```

This would be implemented as a Custom formatter internally:
```go
Custom(func(dst []byte, v *T) []byte {
    // Slow path - allocates, but convenient
    s := fmt.Sprintf("%v", v.GetFormattedField())
    return append(dst, s...)
})
```

Use only when:
- Performance is not critical
- Type already implements fmt.Formatter
- Convenience over performance

---

## Future Enhancements

### Phase 2 Enhancements (Post-Initial Release)

1. **Optional fmt.Formatter Support**
   - Convenience adapter for types implementing fmt.Formatter
   - Clearly documented as slower path
   - For non-performance-critical use cases

2. **Color Support**
   - ANSI color codes for terminal output
   - Conditional coloring based on values
   - Configurable color schemes

3. **Unicode Support**
   - UTF-8 aware padding
   - Wide character handling
   - Combining characters

4. **Numeric Formatting**
   - Thousand separators (1,000,000)
   - Different bases (hex, octal)
   - Scientific notation

5. **Additional Output Formats**
   - JSON Lines
   - HTML table
   - Markdown table

6. **Computed Fields**
   - Fields derived from multiple struct fields
   - Aggregations (sum, average)

7. **Convenience Wrappers**
   - `WriteRowSimple()` with internal buffers
   - `WriteRowPooled()` with sync.Pool
   - Auto-sizing buffers

8. **Validation**
   - Check buffer sizes are adequate
   - Warn about truncation
   - Detect alignment issues

9. **Performance Monitoring**
   - Built-in statistics (rows/sec)
   - Allocation tracking
   - Performance regression tests

### Phase 3 Enhancements (Future)

1. **Dynamic Width Calculation**
   - Auto-size columns based on data
   - Terminal width awareness
   - Responsive layout

2. **Paging Support**
   - Integration with pagers (less, more)
   - Page break handling
   - Repeat headers per page

3. **Filtering Integration**
   - Simple filter DSL
   - Field-based predicates
   - Combine with sorting

---

## Appendix A: Comparison with Existing Solutions

### vs. Standard fmt Package

| Feature | fmt | colprint |
|---------|-----|----------|
| Type safety | Limited | Full (generics) |
| Performance | Moderate | High |
| Alignment | Manual | Automatic |
| Reusability | Low | High |
| Learning curve | Low | Medium |

### vs. text/tabwriter

| Feature | tabwriter | colprint |
|---------|-----------|----------|
| Allocations | Many | Zero |
| Type safety | None | Full |
| Performance | Low | High |
| Streaming | Poor | Excellent |
| Control | Limited | Complete |

### vs. Example.go.txt Approach

| Feature | example.go.txt | colprint |
|---------|----------------|----------|
| Performance | Excellent | Excellent |
| API ergonomics | Good | Better (builder) |
| Collections | No | Yes |
| Defaults | No | Yes |
| Help/Docs | No | Yes |
| Extensibility | Good | Better |

---

## Appendix B: Testing Strategy

### Unit Tests

1. **Registry Tests**
   - Field registration
   - Duplicate detection
   - Case-insensitive lookup
   - Collection management

2. **Compilation Tests**
   - Spec parsing
   - Width overrides
   - Default expansion
   - Collection expansion
   - Error cases

3. **Formatting Tests**
   - String padding
   - Number formatting
   - Alignment
   - Custom formatters

### Integration Tests

1. **End-to-End**
   - Complete workflow
   - Multiple output formats
   - Large data sets

### Performance Tests

1. **Benchmarks**
   - Rows per second
   - Allocation counts
   - CPU profiling
   - Memory profiling

2. **Regression Tests**
   - Track performance over time
   - Prevent degradation

---

## Appendix C: API Examples Reference

### Quick Reference Card

```go
// 1. Create registry
reg := colprint.NewRegistry[MyType]()

// 2. Register fields
reg.Field("name", "Display", "Description").
    Width(10).String(MyType.GetName).Register()

// 3. Define collections
reg.DefineCollection("basic", "field1,field2", "field1", "field2", "field3")
reg.SetDefaults("basic", "field1,field2")

// 4. Compile program
prog, err := colprint.Compile(reg, "field1:15,@default,field3")
// Or with collections:
prog, err := colprint.Compile(reg, "@basic,field4")

// 5. Print data
line := make([]byte, 0, 256)
tmp := make([]byte, 0, 64)
prog.WriteHeader(w, &line)
for i := range data {
    prog.WriteRow(w, &data[i], &tmp, &line)
}
```

---

## Future Enhancements

### Phase 4: Advanced Features (Future Releases)

**Header Underlines**
- Add optional underline support for headers and help output
- Configurable underline character (default: '-')
- Enable/disable via Options
- Example: `Options{ShowUnderlines: true}`

**Additional Features:**
- ANSI color support for highlighting
- Unicode box-drawing characters for table borders
- Multi-line cell content
- Column grouping and sub-headers
- Conditional formatting (e.g., color by value)
- Export to other formats (JSON, XML, etc.)

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-08 | Design Team | Initial design document |

---

## Approval

This design document represents the proposed architecture for the `colprint` library and serves as the reference for implementation.

**Status:** ✅ Approved for implementation

**Next Steps:**
1. Review and feedback period
2. Prototype core functionality
3. Benchmark against requirements
4. Implement full feature set
5. Write comprehensive tests
6. Documentation and examples
7. Public release preparation
