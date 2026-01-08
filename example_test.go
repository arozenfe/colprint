package colprint_test

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/arozenfe/colprint"
)

// Person is our example domain type.
type Person struct {
	Name   string
	Age    int
	Height float64 // centimeters
	Kids   int
}

// Example_basic demonstrates simple field registration and formatting.
func Example_basic() {
	// Create a registry for Person type
	reg := colprint.NewRegistry[Person]()

	// Register fields using fluent API
	reg.Field("name", "Name", "Person's full name").
		Width(12).
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(4).
		Int(func(p *Person) int { return p.Age }).
		Register()

	// Compile a program
	prog, _ := colprint.Compile(reg, "name,age")

	// Create reusable buffers
	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	// Write header
	prog.WriteHeader(os.Stdout, &line)
	prog.WriteUnderline(os.Stdout, &line)

	// Write rows
	people := []Person{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}

	for i := range people {
		prog.WriteRow(os.Stdout, &people[i], &tmp, &line)
	}

	// Output:
	// Name          Age
	// ----          ---
	// Alice         30
	// Bob           25
}

// Example_widthOverride shows how to override field widths.
func Example_widthOverride() {
	reg := colprint.NewRegistry[Person]()

	reg.Field("name", "Name", "Person's name").
		Width(10).
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(3).
		Int(func(p *Person) int { return p.Age }).
		Register()

	// Override name width to 20
	prog, _ := colprint.Compile(reg, "name:20,age")

	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	prog.WriteHeader(os.Stdout, &line)

	person := Person{Name: "Alexandria", Age: 33}
	prog.WriteRow(os.Stdout, &person, &tmp, &line)

	// Output:
	// Name                  Age
	// Alexandria            33
}

// Example_custom demonstrates custom formatters for complex types.
func Example_custom() {
	reg := colprint.NewRegistry[Person]()

	reg.Field("name", "Name", "Person's name").
		Width(10).
		String(func(p *Person) string { return p.Name }).
		Register()

	// Custom formatter: convert cm to feet/inches
	reg.Field("height", "Height", "Height in imperial units").
		Width(10).
		Custom(func(dst []byte, p *Person) []byte {
			totalIn := p.Height / 2.54
			feet := int(math.Floor(totalIn / 12))
			inches := int(math.Round(totalIn)) % 12
			dst = strconv.AppendInt(dst, int64(feet), 10)
			dst = append(dst, '\'')
			dst = strconv.AppendInt(dst, int64(inches), 10)
			dst = append(dst, '"')
			return dst
		}).
		Register()

	prog, _ := colprint.Compile(reg, "name,height")

	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	prog.WriteHeader(os.Stdout, &line)

	person := Person{Name: "Bob", Height: 175.0}
	prog.WriteRow(os.Stdout, &person, &tmp, &line)

	// Output:
	// Name        Height
	// Bob         5'9"
}

// Example_collections shows how to use field collections.
func Example_collections() {
	reg := colprint.NewRegistry[Person]()

	// Register fields
	reg.Field("name", "Name", "Person's name").
		Width(10).
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(4).
		Int(func(p *Person) int { return p.Age }).
		Register()

	reg.Field("kids", "Kids", "Number of children").
		Width(5).
		Int(func(p *Person) int { return p.Kids }).
		Register()

	reg.Field("height", "Height", "Height in cm").
		Width(8).
		Float(1, func(p *Person) float64 { return p.Height }).
		Register()

	// Define collections
	reg.DefineCollection("basic", "name,age", "name", "age")
	reg.DefineCollection("family", "kids", "kids")

	// Use collections with @ prefix
	prog, _ := colprint.Compile(reg, "@basic,@family")

	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	prog.WriteHeader(os.Stdout, &line)

	person := Person{Name: "Carol", Age: 45, Kids: 2}
	prog.WriteRow(os.Stdout, &person, &tmp, &line)

	// Output:
	// Name        Age   Kids
	// Carol       45    2
}

// Example_csv demonstrates CSV-style output with custom separator.
func Example_csv() {
	reg := colprint.NewRegistry[Person]()

	// For CSV output, width determines max field size
	reg.Field("name", "Name", "Person's name").
		Width(5).
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(3).
		Int(func(p *Person) int { return p.Age }).
		Register()

	// For CSV output, use NoPadding option
	opts := colprint.Options{
		Separator: ",",
		NoPadding: true,
	}
	prog, _ := colprint.CompileWithOptions(reg, "name,age", opts)

	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	prog.WriteHeader(os.Stdout, &line)

	people := []Person{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}

	for i := range people {
		prog.WriteRow(os.Stdout, &people[i], &tmp, &line)
	}

	// Output:
	// Name,Age
	// Alice,30
	// Bob,25
}

// Example_streaming shows handling of streaming data.
func Example_streaming() {
	reg := colprint.NewRegistry[Person]()

	reg.Field("name", "Name", "Person's name").
		Width(10).
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(4).
		Int(func(p *Person) int { return p.Age }).
		Register()

	prog, _ := colprint.Compile(reg, "name,age")

	// Simulate streaming data
	stream := make(chan Person, 3)
	go func() {
		stream <- Person{Name: "Alice", Age: 30}
		stream <- Person{Name: "Bob", Age: 25}
		stream <- Person{Name: "Carol", Age: 35}
		close(stream)
	}()

	// Print header once
	var buf bytes.Buffer
	line := make([]byte, 0, 128)
	tmp := make([]byte, 0, 32)

	prog.WriteHeader(&buf, &line)

	// Process stream
	for person := range stream {
		prog.WriteRow(&buf, &person, &tmp, &line)
	}

	fmt.Print(buf.String())

	// Output:
	// Name        Age
	// Alice       30
	// Bob         25
	// Carol       35
}

// Example_help demonstrates the help functionality.
func Example_help() {
	reg := colprint.NewRegistry[Person]()

	reg.Field("name", "Name", "Person's full name").
		Width(12).
		Category("Basic").
		String(func(p *Person) string { return p.Name }).
		Register()

	reg.Field("age", "Age", "Age in years").
		Width(4).
		Category("Basic").
		Int(func(p *Person) int { return p.Age }).
		Register()

	reg.Field("height", "Height", "Height in centimeters").
		Width(8).
		Category("Physical").
		Float(1, func(p *Person) float64 { return p.Height }).
		Register()

	// Print help
	var buf bytes.Buffer
	reg.PrintHelp(&buf, "")

	// Show output
	fmt.Print(buf.String())

	// Output:
	//
	// Basic:
	//   Field  Display  Description
	//   age    Age      Age in years
	//   name   Name     Person's full name
	//
	// Physical:
	//   Field   Display  Description
	//   height  Height   Height in centimeters
}
