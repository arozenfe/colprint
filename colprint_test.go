package colprint

import (
	"bytes"
	"strings"
	"testing"
)

type testPerson struct {
	Name string
	Age  int
	Temp float64
}

func (p *testPerson) GetName() string  { return p.Name }
func (p *testPerson) GetAge() int      { return p.Age }
func (p *testPerson) GetTemp() float64 { return p.Temp }

func TestRegistryBasic(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test name").
		Width(10).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test age").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	fields := reg.ListFields()
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}

	if fields[0] != "age" || fields[1] != "name" {
		t.Errorf("unexpected field order: %v", fields)
	}
}

func TestRegistryCaseInsensitive(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	// Test case-insensitive lookup
	f, ok := reg.get("NAME")
	if !ok {
		t.Error("case-insensitive lookup failed for 'NAME'")
	}
	if f.Name != "name" {
		t.Errorf("expected canonical name 'name', got '%s'", f.Name)
	}

	f, ok = reg.get("Name")
	if !ok {
		t.Error("case-insensitive lookup failed for 'Name'")
	}
}

func TestCompileBasic(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	prog, err := Compile(reg, "name,age")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if len(prog.columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(prog.columns))
	}
}

func TestCompileWidthOverride(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	prog, err := Compile(reg, "name:20")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if prog.columns[0].width != 20 {
		t.Errorf("expected width 20, got %d", prog.columns[0].width)
	}
}

func TestCompileUnknownField(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	_, err := Compile(reg, "unknown")
	if err == nil {
		t.Error("expected error for unknown field")
	}

	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCompileCollection(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	reg.DefineCollection("basic", "name,age", "name", "age")

	prog, err := Compile(reg, "@basic")
	if err != nil {
		t.Fatalf("compile with collection failed: %v", err)
	}

	if len(prog.columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(prog.columns))
	}
}

func TestCompileDefault(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	reg.SetDefaults("main", "name,age")

	prog, err := Compile(reg, "@default")
	if err != nil {
		t.Fatalf("compile with @default failed: %v", err)
	}

	if len(prog.columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(prog.columns))
	}
}

func TestFormatString(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	prog, _ := Compile(reg, "name")

	person := testPerson{Name: "Alice"}
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "Alice" // No padding on last column
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatInt(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	prog, _ := Compile(reg, "age")

	person := testPerson{Age: 42}
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "42" // No padding on last column
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatFloat(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("temp", "Temp", "Test").
		Width(8).
		Float(2, (*testPerson).GetTemp).
		Register()

	prog, _ := Compile(reg, "temp")

	person := testPerson{Temp: 98.6}
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "98.60" // No padding on last column
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatCustom(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("custom", "Custom", "Test").
		Width(10).
		Custom(func(dst []byte, p *testPerson) []byte {
			return append(dst, "CUSTOM"...)
		}).
		Register()

	prog, _ := Compile(reg, "custom")

	person := testPerson{}
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "CUSTOM" // No padding on last column
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestHeaderAndUnderline(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(10).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	prog, _ := Compile(reg, "name,age")

	var buf bytes.Buffer
	line := make([]byte, 0, 64)

	prog.WriteHeader(&buf, &line)
	header := buf.String()

	expectedHeader := "Name        Age\n"
	if header != expectedHeader {
		t.Errorf("expected header %q, got %q", expectedHeader, header)
	}

	buf.Reset()
	prog.WriteUnderline(&buf, &line)
	underline := buf.String()

	expectedUnderline := "----        ---\n"
	if underline != expectedUnderline {
		t.Errorf("expected underline %q, got %q", expectedUnderline, underline)
	}
}

func TestCustomSeparator(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(5).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(3).
		Int((*testPerson).GetAge).
		Register()

	opts := Options{Separator: ","}
	prog, _ := CompileWithOptions(reg, "name,age", opts)

	person := testPerson{Name: "Bob", Age: 25}
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "Bob  ,25"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTruncation(t *testing.T) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(5). // Only 5 chars
		String((*testPerson).GetName).
		Register()

	prog, _ := Compile(reg, "name")

	person := testPerson{Name: "Alexander"} // 9 chars
	line := make([]byte, 0, 64)
	tmp := make([]byte, 0, 32)

	result := prog.FormatRow(&person, &tmp, &line)

	expected := "Alexa" // Truncated to 5
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// Benchmark the hot path - formatting rows
func BenchmarkWriteRow(b *testing.B) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(15).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	reg.Field("temp", "Temp", "Test").
		Width(8).
		Float(2, (*testPerson).GetTemp).
		Register()

	prog, _ := Compile(reg, "name,age,temp")

	person := testPerson{Name: "Alice", Age: 30, Temp: 98.6}
	line := make([]byte, 0, 256)
	tmp := make([]byte, 0, 64)

	var buf bytes.Buffer

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		prog.WriteRow(&buf, &person, &tmp, &line)
	}
}

// Benchmark formatting 1 million rows
func BenchmarkMillionRows(b *testing.B) {
	reg := NewRegistry[testPerson]()

	reg.Field("name", "Name", "Test").
		Width(15).
		String((*testPerson).GetName).
		Register()

	reg.Field("age", "Age", "Test").
		Width(5).
		Int((*testPerson).GetAge).
		Register()

	prog, _ := Compile(reg, "name,age")

	people := make([]testPerson, 1000000)
	for i := range people {
		people[i] = testPerson{Name: "Person", Age: i % 100}
	}

	line := make([]byte, 0, 256)
	tmp := make([]byte, 0, 64)

	var buf bytes.Buffer

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		for j := range people {
			prog.WriteRow(&buf, &people[j], &tmp, &line)
		}
	}
}
