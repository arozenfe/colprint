# colprint

High-performance, zero-allocation column formatting library for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/arozenfe/colprint.svg)](https://pkg.go.dev/github.com/arozenfe/colprint)

## Features

- **Zero allocations** in formatting hot path
- **Type-safe** using Go generics
- **Fast** - single syscall per row with line buffering
- **Flexible** field selection and collections
- **Custom formatters** for complex types
- Suitable for streaming millions of rows

## Installation

```bash
go get github.com/arozenfe/colprint
```

## Quick Start

```go
package main

import (
    "os"
    "github.com/arozenfe/colprint"
)

type Person struct {
    Name string
    Age  int
    City string
}

func main() {
    // 1. Create registry and register fields
    reg := colprint.NewRegistry[Person]()
    
    reg.Field("name", "Name", "Person's name").
        Width(15).
        String(func(p *Person) string { return p.Name }).
        Register()
    
    reg.Field("age", "Age", "Age in years").
        Width(5).
        Int(func(p *Person) int { return p.Age }).
        Register()
    
    reg.Field("city", "City", "City of residence").
        Width(12).
        String(func(p *Person) string { return p.City }).
        Register()
    
    // 2. Compile a field specification
    prog, _ := colprint.Compile(reg, "name,age,city")
    
    // 3. Format rows (zero allocations in this loop)
    line := make([]byte, 0, 256)
    tmp := make([]byte, 0, 64)
    
    people := []Person{
        {"Alice", 30, "New York"},
        {"Bob", 25, "Los Angeles"},
        {"Charlie", 35, "Chicago"},
    }
    
    prog.WriteHeader(os.Stdout, &line)
    prog.WriteUnderline(os.Stdout, &line)
    
    for i := range people {
        prog.WriteRow(os.Stdout, &people[i], &tmp, &line)
    }
}
```

Output:
```
Name           Age   City        
-------------- ----- ------------
Alice          30    New York    
Bob            25    Los Angeles 
Charlie        35    Chicago     
```

## Field Collections

Group related fields into named collections:

```go
reg.DefineCollection("basic", "Basic info", "name", "age")
reg.DefineCollection("location", "Location info", "city", "country")
reg.SetDefaults("basic", "name,age")

// Use @collection syntax in specs
prog, _ := colprint.Compile(reg, "@basic,city")
prog, _ := colprint.Compile(reg, "@default")  // Uses default fields
```

## Custom Separators

```go
// No spacing between columns
prog, _ := colprint.CompileWithOptions(reg, "name,age,city", colprint.Options{
    Separator: "",
})

// CSV output
prog, _ := colprint.CompileWithOptions(reg, "name,age,city", colprint.Options{
    Separator: ",",
    NoHeader:  false,
})
```

## Field Width Override

```go
// Override width in specification
prog, _ := colprint.Compile(reg, "name:20,age:8,city:15")
```

## Custom Formatters

```go
reg.Field("elapsed", "Elapsed", "Time elapsed").
    Width(10).
    Custom(func(buf []byte, p *Person) []byte {
        // Custom formatting logic
        elapsed := time.Since(p.StartTime)
        return append(buf, elapsed.String()...)
    }).
    Register()
```

## Performance

Designed for maximum performance:
- All field resolution happens once during `Compile()`
- Formatting uses typed closures (no interface dispatch)
- Buffers are reused across rows (caller-provided)
- No reflection in hot path

Expected performance: **1M+ rows/sec** for typical workloads.

## Options

```go
type Options struct {
    Separator      string  // Column separator (default: "  ")
    NoHeader       bool    // Skip header line
    NoUnderline    bool    // Skip header underline
    NoPadding      bool    // No padding on any column
    PadLastColumn  bool    // Pad last column to width (default: false)
}
```

## Documentation

Full documentation available at [pkg.go.dev](https://pkg.go.dev/github.com/arozenfe/colprint).

See [.github/DESIGN.md](.github/DESIGN.md) for design rationale and implementation details.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please see [.github/DEVELOPMENT.md](.github/DEVELOPMENT.md) for development status and roadmap.
