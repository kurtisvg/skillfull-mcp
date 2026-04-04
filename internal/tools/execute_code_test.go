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
		{
			name:  "string_match",
			value: monty.String("hello"),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "string"}},
		},
		{
			name:  "integer_match",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "integer"}},
		},
		{
			name:  "number_accepts_int",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}},
		},
		{
			name:  "number_accepts_float",
			value: monty.Float(3.14),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}},
		},
		{
			name:  "boolean_match",
			value: monty.Bool(true),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "boolean"}},
		},
		{
			name:  "array_match",
			value: monty.List(monty.Int(1)),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "array"}},
		},
		{
			name:  "object_match",
			value: monty.DictValue(monty.Dict{{Key: monty.String("k"), Value: monty.String("v")}}),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "object"}},
		},
		{
			name:    "type_mismatch",
			value:   monty.Int(42),
			param:   mcpserver.ParamInfo{Name: "sql", Schema: map[string]any{"type": "string"}},
			wantErr: true,
		},
		{
			name:  "nullable_none",
			value: monty.None(),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}},
		},
		{
			name:  "nullable_string",
			value: monty.String("hi"),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}},
		},
		{
			name:  "no_schema",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x"},
		},
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

	tools := mgr.AllTools()
	fns := make(map[string]monty.ExternalFunction, len(tools))
	for _, tool := range tools {
		srv, err := mgr.GetServer(tool.ServerName)
		if err != nil {
			t.Fatal(err)
		}
		fns[tool.ResolvedName] = buildTool(tool, srv)
	}

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

// --- extractResult tests ---

func TestExtractResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *mcp.CallToolResult
		wantKind monty.ValueKind
	}{
		{
			name:     "text_content",
			result:   &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}},
			wantKind: "string",
		},
		{
			name: "structured_content_preferred",
			result: &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: `{"temp": 22}`}},
				StructuredContent: map[string]any{"temp": float64(22)},
			},
			wantKind: "dict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractResult(tt.result)
			if got.Kind() != tt.wantKind {
				t.Errorf("Kind() = %s, want %s", got.Kind(), tt.wantKind)
			}
		})
	}
}

// --- anyToMonty tests ---

func TestAnyToMonty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		wantKind monty.ValueKind
	}{
		{
			name:     "string",
			input:    "hello",
			wantKind: "string",
		},
		{
			name:     "float",
			input:    3.14,
			wantKind: "float",
		},
		{
			name:     "int_from_float",
			input:    float64(42),
			wantKind: "int",
		},
		{
			name:     "bool",
			input:    true,
			wantKind: "bool",
		},
		{
			name:     "nil",
			input:    nil,
			wantKind: "none",
		},
		{
			name:     "map",
			input:    map[string]any{"key": "val"},
			wantKind: "dict",
		},
		{
			name:     "slice",
			input:    []any{"a", "b"},
			wantKind: "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := anyToMonty(tt.input)
			if got.Kind() != tt.wantKind {
				t.Errorf("Kind() = %s, want %s", got.Kind(), tt.wantKind)
			}
		})
	}
}
