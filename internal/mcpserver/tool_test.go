package mcpserver

import (
	"testing"
)

// --- extractParamSchema tests ---

func TestExtractParamSchemaRequiredAndOptional(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{"type": "integer"},
			"sql":   map[string]any{"type": "string"},
		},
		"required": []any{"sql"},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	// Required first, then optional sorted.
	if params[0].Name != "sql" || params[0].Types[0] != "string" || !params[0].Required {
		t.Errorf("params[0] = %+v, want sql/string/required", params[0])
	}
	if params[1].Name != "limit" || params[1].Types[0] != "integer" || params[1].Required {
		t.Errorf("params[1] = %+v, want limit/integer/optional", params[1])
	}
}

func TestExtractParamSchemaNoRequired(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"gamma": map[string]any{"type": "string"},
			"alpha": map[string]any{"type": "string"},
			"beta":  map[string]any{"type": "number"},
		},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(params))
	}
	// All sorted lexicographically, all optional.
	expected := []string{"alpha", "beta", "gamma"}
	for i, name := range expected {
		if params[i].Name != name {
			t.Errorf("params[%d].Name = %q, want %q", i, params[i].Name, name)
		}
		if params[i].Required {
			t.Errorf("params[%d].Required = true, want false", i)
		}
	}
}

func TestExtractParamSchemaRequiredWithNonRequiredSorted(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"z": map[string]any{"type": "string"},
			"c": map[string]any{"type": "string"},
			"a": map[string]any{"type": "string"},
		},
		"required": []any{"z"},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(params))
	}
	// z first (required), then a, c (sorted).
	expected := []string{"z", "a", "c"}
	for i, name := range expected {
		if params[i].Name != name {
			t.Errorf("params[%d].Name = %q, want %q", i, params[i].Name, name)
		}
	}
}

func TestExtractParamSchemaNilSchema(t *testing.T) {
	t.Parallel()
	params, err := extractParamSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	if params != nil {
		t.Errorf("expected nil, got %v", params)
	}
}

func TestExtractParamSchemaEmptyProperties(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Errorf("expected empty, got %v", params)
	}
}

func TestExtractParamSchemaArrayType(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": []any{"string", "null"}},
		},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if len(params[0].Types) != 2 || params[0].Types[0] != "string" || params[0].Types[1] != "null" {
		t.Errorf("types = %v, want [string null]", params[0].Types)
	}
}

func TestExtractParamSchemaNoTypeField(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"description": "no type here"},
		},
	}
	params, err := extractParamSchema(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if params[0].Types != nil {
		t.Errorf("types = %v, want nil", params[0].Types)
	}
}

func TestExtractParamSchemaInvalidSchemaType(t *testing.T) {
	t.Parallel()
	_, err := extractParamSchema("not a map")
	if err == nil {
		t.Fatal("expected error for non-map schema")
	}
}

func TestExtractParamSchemaInvalidPropertiesType(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type":       "object",
		"properties": "not a map",
	}
	_, err := extractParamSchema(schema)
	if err == nil {
		t.Fatal("expected error for non-map properties")
	}
}

// --- Tool.Signature tests ---

func TestToolSignatureWithParams(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "execute_sql",
		Description:  "Run a SQL query",
		Params: []ParamInfo{
			{Name: "sql", Types: []string{"string"}, Required: true},
			{Name: "limit", Types: []string{"integer"}, Required: false},
		},
	}
	want := "execute_sql(sql: str, limit: int = None) -> str\n  Run a SQL query"
	if got := tool.Signature(); got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

func TestToolSignatureNoParams(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "list_tables",
		Description:  "List database tables",
	}
	want := "list_tables() -> str\n  List database tables"
	if got := tool.Signature(); got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

func TestToolSignatureNoDescription(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "ping",
		Params: []ParamInfo{
			{Name: "host", Types: []string{"string"}, Required: true},
		},
	}
	want := "ping(host: str) -> str"
	if got := tool.Signature(); got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

// --- resolveTools tests ---

func TestResolveToolsNoConflict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_a"))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_b"))
	if err != nil {
		t.Fatal(err)
	}

	tools, err := resolveTools(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.ResolvedName] = true
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("expected tool_a and tool_b, got %v", names)
	}
}

func TestResolveToolsWithConflict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Both servers have a tool named "my_test_tool".
	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "my_test_tool"))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "my_test_tool"))
	if err != nil {
		t.Fatal(err)
	}

	tools, err := resolveTools(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.ResolvedName] = true
	}
	if !names["alpha_my_test_tool"] || !names["beta_my_test_tool"] {
		t.Errorf("expected prefixed names, got %v", names)
	}
	if names["my_test_tool"] {
		t.Error("should not have unprefixed name when conflicting")
	}
}

func TestResolveToolsEmpty(t *testing.T) {
	t.Parallel()
	tools, err := resolveTools(map[string]*Server{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 0 {
		t.Errorf("expected empty, got %v", tools)
	}
}
