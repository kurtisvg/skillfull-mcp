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
	if params[0].Name != "sql" || formatSchema(params[0].Schema) != "str" || !params[0].Required {
		t.Errorf("params[0]: name=%q type=%s required=%v", params[0].Name, formatSchema(params[0].Schema), params[0].Required)
	}
	if params[1].Name != "limit" || formatSchema(params[1].Schema) != "int" || params[1].Required {
		t.Errorf("params[1]: name=%q type=%s required=%v", params[1].Name, formatSchema(params[1].Schema), params[1].Required)
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

func TestExtractParamSchemaUnionType(t *testing.T) {
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
	if got := formatSchema(params[0].Schema); got != "str | None" {
		t.Errorf("formatSchema = %q, want %q", got, "str | None")
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
	if got := formatSchema(params[0].Schema); got != "any" {
		t.Errorf("formatSchema = %q, want %q", got, "any")
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

// --- formatSchema tests ---

func TestFormatSchemaSimpleTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		schema any
		want   string
	}{
		{
			name:   "nil",
			schema: nil,
			want:   "any",
		},
		{
			name:   "string",
			schema: map[string]any{"type": "string"},
			want:   "str",
		},
		{
			name:   "integer",
			schema: map[string]any{"type": "integer"},
			want:   "int",
		},
		{
			name:   "number",
			schema: map[string]any{"type": "number"},
			want:   "float",
		},
		{
			name:   "boolean",
			schema: map[string]any{"type": "boolean"},
			want:   "bool",
		},
		{
			name:   "null",
			schema: map[string]any{"type": "null"},
			want:   "None",
		},
		{
			name:   "array",
			schema: map[string]any{"type": "array"},
			want:   "list",
		},
		{
			name:   "object",
			schema: map[string]any{"type": "object"},
			want:   "dict",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatSchema(tt.schema); got != tt.want {
				t.Errorf("formatSchema() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSchemaArrayWithItems(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
	if got := formatSchema(schema); got != "list[str]" {
		t.Errorf("got %q, want %q", got, "list[str]")
	}
}

func TestFormatSchemaNestedArray(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "integer"},
		},
	}
	if got := formatSchema(schema); got != "list[list[int]]" {
		t.Errorf("got %q, want %q", got, "list[list[int]]")
	}
}

func TestFormatSchemaObjectWithProperties(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"temperature": map[string]any{"type": "number"},
			"conditions":  map[string]any{"type": "string"},
			"humidity":    map[string]any{"type": "number"},
		},
	}
	want := `{"conditions": str, "humidity": float, "temperature": float}`
	if got := formatSchema(schema); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSchemaUnionType(t *testing.T) {
	t.Parallel()
	schema := map[string]any{"type": []any{"string", "null"}}
	if got := formatSchema(schema); got != "str | None" {
		t.Errorf("got %q, want %q", got, "str | None")
	}
}

func TestFormatSchemaArrayOfObjects(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"id":   map[string]any{"type": "integer"},
			},
		},
	}
	want := `list[{"id": int, "name": str}]`
	if got := formatSchema(schema); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- Tool.Signature tests ---

func TestToolSignatureWithParams(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "execute_sql",
		Description:  "Run a SQL query",
		Params: []ParamInfo{
			{Name: "sql", Schema: map[string]any{"type": "string"}, Required: true},
			{Name: "limit", Schema: map[string]any{"type": "integer"}, Required: false},
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
			{Name: "host", Schema: map[string]any{"type": "string"}, Required: true},
		},
	}
	want := "ping(host: str) -> str"
	if got := tool.Signature(); got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

func TestToolSignatureStructuredOutput(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "get_weather",
		Description:  "Get weather data",
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"temperature": map[string]any{"type": "number"},
				"conditions":  map[string]any{"type": "string"},
				"humidity":    map[string]any{"type": "number"},
			},
		},
		Params: []ParamInfo{
			{Name: "location", Schema: map[string]any{"type": "string"}, Required: true},
		},
	}
	want := `get_weather(location: str) -> {"conditions": str, "humidity": float, "temperature": float}` + "\n  Get weather data"
	if got := tool.Signature(); got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

func TestToolSignatureListOutput(t *testing.T) {
	t.Parallel()
	tool := Tool{
		ResolvedName: "list_users",
		Description:  "List all users",
		OutputSchema: map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	}
	want := "list_users() -> list[str]\n  List all users"
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
