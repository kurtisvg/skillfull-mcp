package tools

import (
	"context"
	"strings"
	"testing"

	"skillful-mcp/internal/mcpserver"

	monty "github.com/ewhauser/gomonty"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecuteCodeDescriptionRefersToUseSkill(t *testing.T) {
	t.Parallel()
	if !strings.Contains(executeCodeDescription, "use_skill") {
		t.Error("description should refer to use_skill for tool discovery")
	}
	if !strings.Contains(executeCodeDescription, "tool_name") {
		t.Error("description should show tools are called by name")
	}
	if !strings.Contains(executeCodeDescription, "resources") {
		t.Error("description should mention resources")
	}
}

func TestExecuteCodeBasicMath(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("40 + 2", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "42" {
		t.Errorf("result = %q, want '42'", value.String())
	}
}

func TestExecuteCodeStringExpression(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("'hello' + ' ' + 'world'", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "hello world" {
		t.Errorf("result = %q, want 'hello world'", value.String())
	}
}

func TestExecuteCodeSyntaxError(t *testing.T) {
	t.Parallel()
	_, err := monty.New("def (invalid syntax", monty.CompileOptions{ScriptName: "script.py"})
	if err == nil {
		t.Fatal("expected compile error for invalid syntax")
	}
}

// --- validateMontyValue tests ---

func TestValidateMontyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   monty.Value
		param   mcpserver.ParamInfo
		wantErr bool
	}{
		{"string_match", monty.String("hello"), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "string"}}, false},
		{"integer_match", monty.Int(42), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "integer"}}, false},
		{"number_accepts_int", monty.Int(42), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}}, false},
		{"number_accepts_float", monty.Float(3.14), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}}, false},
		{"boolean_match", monty.Bool(true), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "boolean"}}, false},
		{"array_match", monty.List(monty.Int(1)), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "array"}}, false},
		{"object_match", monty.DictValue(monty.Dict{{Key: monty.String("k"), Value: monty.String("v")}}), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "object"}}, false},
		{"type_mismatch", monty.Int(42), mcpserver.ParamInfo{Name: "sql", Schema: map[string]any{"type": "string"}}, true},
		{"nullable_none", monty.None(), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}}, false},
		{"nullable_string", monty.String("hi"), mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}}, false},
		{"no_schema", monty.Int(42), mcpserver.ParamInfo{Name: "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateMontyValue(tt.value, tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMontyValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMontyValueErrorMentionsParam(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "sql", Schema: map[string]any{"type": "string"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sql") {
		t.Errorf("error should mention parameter name, got: %v", err)
	}
}

// --- Integration: type validation via Monty runner ---

func TestExecuteCodeTypeValidation(t *testing.T) {
	t.Parallel()

	// Build tool functions from a manager with a typed tool.
	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "typed"}, nil)
	type GreetInput struct {
		Name string `json:"name" jsonschema:"the name to greet"`
	}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "greet", Description: "Greet someone"},
		func(ctx context.Context, req *mcp.CallToolRequest, input GreetInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "hello " + input.Name}},
			}, nil, nil
		},
	)

	dsServerT, dsClientT := mcp.NewInMemoryTransports()
	go func() { _ = ds.Run(ctx, dsServerT) }()
	dsClient := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	dsSession, err := dsClient.Connect(ctx, dsClientT, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv, err := mcpserver.NewServerFromSession(ctx, dsSession)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	fns := buildToolFunctions(mgr)

	t.Run("valid_string_arg", func(t *testing.T) {
		t.Parallel()
		runner, err := monty.New(`greet("world")`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		value, err := runner.Run(t.Context(), monty.RunOptions{Functions: fns})
		if err != nil {
			t.Fatalf("runtime error: %v", err)
		}
		if value.String() != "hello world" {
			t.Errorf("result = %q, want 'hello world'", value.String())
		}
	})

	t.Run("invalid_int_for_string", func(t *testing.T) {
		t.Parallel()
		runner, err := monty.New(`greet(42)`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.Run(t.Context(), monty.RunOptions{Functions: fns})
		if err == nil {
			t.Fatal("expected runtime error for type mismatch")
		}
		if !strings.Contains(err.Error(), "TypeError") {
			t.Errorf("expected TypeError, got: %v", err)
		}
	})
}

// --- extractResult / contentToMonty tests ---

func TestExtractResultTextContent(t *testing.T) {
	t.Parallel()
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "hello"}},
	}
	got := extractResult(result)
	if got.String() != "hello" {
		t.Errorf("got %q, want %q", got.String(), "hello")
	}
}

func TestExtractResultStructuredContent(t *testing.T) {
	t.Parallel()
	result := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: `{"temp": 22}`}},
		StructuredContent: map[string]any{"temp": float64(22)},
	}
	got := extractResult(result)
	// Should prefer structured content over text.
	if got.Kind() != "dict" {
		t.Fatalf("expected dict, got %s", got.Kind())
	}
}

// --- anyToMonty tests ---

func TestAnyToMontyString(t *testing.T) {
	t.Parallel()
	got := anyToMonty("hello")
	if got.String() != "hello" {
		t.Errorf("got %q, want %q", got.String(), "hello")
	}
}

func TestAnyToMontyFloat(t *testing.T) {
	t.Parallel()
	got := anyToMonty(3.14)
	if got.Kind() != "float" {
		t.Errorf("expected float, got %s", got.Kind())
	}
}

func TestAnyToMontyIntFromFloat(t *testing.T) {
	t.Parallel()
	// JSON numbers without decimals unmarshal as float64 but represent ints.
	got := anyToMonty(float64(42))
	if got.Kind() != "int" {
		t.Errorf("expected int for whole number, got %s", got.Kind())
	}
}

func TestAnyToMontyBool(t *testing.T) {
	t.Parallel()
	got := anyToMonty(true)
	if got.Kind() != "bool" {
		t.Errorf("expected bool, got %s", got.Kind())
	}
}

func TestAnyToMontyNil(t *testing.T) {
	t.Parallel()
	got := anyToMonty(nil)
	if got.Kind() != "none" {
		t.Errorf("expected none, got %s", got.Kind())
	}
}

func TestAnyToMontyMap(t *testing.T) {
	t.Parallel()
	got := anyToMonty(map[string]any{"key": "val"})
	if got.Kind() != "dict" {
		t.Errorf("expected dict, got %s", got.Kind())
	}
}

func TestAnyToMontySlice(t *testing.T) {
	t.Parallel()
	got := anyToMonty([]any{"a", "b"})
	if got.Kind() != "list" {
		t.Errorf("expected list, got %s", got.Kind())
	}
}
