package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"skillful-mcp/internal/app"
	"skillful-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startFakeServer creates a fake MCP server with the given instructions, tools,
// and resources, and returns a connected client session.
func startFakeServer(t *testing.T, ctx context.Context, instructions string, tools []mcp.Tool, resources []mcp.Resource) *mcp.ClientSession {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Name: "downstream"}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	for _, tool := range tools {
		tool := tool
		mcp.AddTool(s, &tool, func(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
			// Echo back the tool name and arguments for verification.
			resp := map[string]any{"tool": tool.Name, "args": input}
			data, _ := json.Marshal(resp)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
			}, nil, nil
		})
	}
	for _, r := range resources {
		r := r
		s.AddResource(&r, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{URI: r.URI, MIMEType: "text/plain", Text: "content of " + r.Name},
				},
			}, nil
		})
	}

	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = s.Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// connectTestClient creates an app server backed by the given manager, connects
// a test client, and returns the session.
func connectTestClient(t *testing.T, ctx context.Context, mgr *mcpserver.Manager) *mcp.ClientSession {
	t.Helper()

	upstream := app.NewServer(mgr)
	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = upstream.Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestE2EMultipleSkills(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	dbSession := startFakeServer(t, ctx, "Query and inspect a SQL database",
		[]mcp.Tool{
			{Name: "execute_sql", Description: "Run a SQL query"},
			{Name: "list_tables", Description: "List database tables"},
		},
		nil,
	)
	fsSession := startFakeServer(t, ctx, "Read files from the local filesystem",
		[]mcp.Tool{
			{Name: "read_file", Description: "Read a file"},
		},
		[]mcp.Resource{
			{URI: "file:///tmp/test.txt", Name: "test.txt", Description: "A test file"},
		},
	)
	// Skill with no instructions.
	plainSession := startFakeServer(t, ctx, "",
		[]mcp.Tool{{Name: "ping", Description: "Ping"}},
		nil,
	)

	dbServer, err := mcpserver.NewServerFromSession(ctx, dbSession)
	if err != nil {
		t.Fatal(err)
	}
	fsServer, err := mcpserver.NewServerFromSession(ctx, fsSession)
	if err != nil {
		t.Fatal(err)
	}
	plainServer, err := mcpserver.NewServerFromSession(ctx, plainSession)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"database":   dbServer,
		"filesystem": fsServer,
		"plain":      plainServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	session := connectTestClient(t, ctx, mgr)

	t.Run("list_skills", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_skills"})
		if err != nil {
			t.Fatal(err)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if !strings.Contains(tc.Text, "- database: Query and inspect a SQL database") {
			t.Errorf("expected database with instructions, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "- filesystem: Read files from the local filesystem") {
			t.Errorf("expected filesystem with instructions, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "- plain\n") && !strings.HasSuffix(tc.Text, "- plain") {
			t.Errorf("expected '- plain' without instructions, got %q", tc.Text)
		}
	})

	t.Run("use_skill_database", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "use_skill",
			Arguments: map[string]any{"skill_name": "database"},
		})
		if err != nil {
			t.Fatal(err)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if !strings.Contains(tc.Text, "execute_sql(") {
			t.Errorf("expected execute_sql signature, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "list_tables(") {
			t.Errorf("expected list_tables signature, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "Run a SQL query") {
			t.Errorf("expected tool description, got %q", tc.Text)
		}
		if strings.Contains(tc.Text, "Resources:") {
			t.Errorf("expected no resources section, got %q", tc.Text)
		}
	})

	t.Run("use_skill_filesystem", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "use_skill",
			Arguments: map[string]any{"skill_name": "filesystem"},
		})
		if err != nil {
			t.Fatal(err)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if !strings.Contains(tc.Text, "read_file(") {
			t.Errorf("expected read_file signature, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "Resources:") {
			t.Errorf("expected resources section, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "file:///tmp/test.txt: A test file") {
			t.Errorf("expected resource URI and description, got %q", tc.Text)
		}
	})

	t.Run("read_resource", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "read_resource",
			Arguments: map[string]any{
				"skill_name":   "filesystem",
				"resource_uri": "file:///tmp/test.txt",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			t.Fatal("tool returned error")
		}
		er := result.Content[0].(*mcp.EmbeddedResource)
		if er.Resource.Text != "content of test.txt" {
			t.Errorf("resource text = %q, want 'content of test.txt'", er.Resource.Text)
		}
	})

	t.Run("execute_code_math", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": "1 + 2 + 3"},
		})
		if err != nil {
			t.Fatal(err)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if tc.Text != "6" {
			t.Errorf("result = %q, want '6'", tc.Text)
		}
	})

	t.Run("execute_code_call_tool", func(t *testing.T) {
		code := dedent(`
			execute_sql(sql="SELECT 1")
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("execute_code returned error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
			t.Fatalf("failed to parse response %q: %v", tc.Text, err)
		}
		if resp["tool"] != "execute_sql" {
			t.Errorf("tool = %v, want 'execute_sql'", resp["tool"])
		}
		args := resp["args"].(map[string]any)
		if args["sql"] != "SELECT 1" {
			t.Errorf("args.sql = %v, want 'SELECT 1'", args["sql"])
		}
	})

	t.Run("execute_code_multi_tool", func(t *testing.T) {
		code := dedent(`
			a = execute_sql(sql="SELECT 1")
			b = read_file(path="/tmp/test.txt")
			a + " | " + b
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("execute_code returned error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp1, resp2 map[string]any
		parts := splitOnce(tc.Text, " | ")
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts separated by ' | ', got %q", tc.Text)
		}
		if err := json.Unmarshal([]byte(parts[0]), &resp1); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(parts[1]), &resp2); err != nil {
			t.Fatal(err)
		}
		if resp1["tool"] != "execute_sql" {
			t.Errorf("first tool = %v, want 'execute_sql'", resp1["tool"])
		}
		if resp2["tool"] != "read_file" {
			t.Errorf("second tool = %v, want 'read_file'", resp2["tool"])
		}
	})

	t.Run("use_skill_unknown", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "use_skill",
			Arguments: map[string]any{"skill_name": "nonexistent"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Error("expected error for unknown skill")
		}
	})
}

func TestE2EPositionalArgs(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	type QueryInput struct {
		SQL string `json:"sql" jsonschema:"the SQL query"`
	}
	ds := mcp.NewServer(&mcp.Implementation{Name: "typed-downstream"}, &mcp.ServerOptions{
		Instructions: "Run SQL queries with typed parameters",
	})
	mcp.AddTool(ds, &mcp.Tool{Name: "execute_sql", Description: "Run a SQL query"}, func(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
		resp := map[string]any{"tool": "execute_sql", "sql": input.SQL}
		data, _ := json.Marshal(resp)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	dsServerT, dsClientT := mcp.NewInMemoryTransports()
	go func() { _ = ds.Run(ctx, dsServerT) }()
	dsClient := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	dsSession, err := dsClient.Connect(ctx, dsClientT, nil)
	if err != nil {
		t.Fatal(err)
	}

	dbServer, err := mcpserver.NewServerFromSession(ctx, dsSession)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"db": dbServer})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	session := connectTestClient(t, ctx, mgr)

	t.Run("positional_arg", func(t *testing.T) {
		code := dedent(`
			execute_sql("SELECT 1")
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
			t.Fatalf("failed to parse %q: %v", tc.Text, err)
		}
		if resp["sql"] != "SELECT 1" {
			t.Errorf("sql = %v, want 'SELECT 1'", resp["sql"])
		}
	})

	t.Run("keyword_arg", func(t *testing.T) {
		code := dedent(`
			execute_sql(sql="SELECT 2")
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
			t.Fatalf("failed to parse %q: %v", tc.Text, err)
		}
		if resp["sql"] != "SELECT 2" {
			t.Errorf("sql = %v, want 'SELECT 2'", resp["sql"])
		}
	})
}

func TestE2EToolNameConflict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Both skills have a tool named "search", plus alpha has "unique_tool".
	alpha := startFakeServer(t, ctx, "Alpha search service",
		[]mcp.Tool{
			{Name: "search", Description: "Search alpha"},
			{Name: "unique_tool", Description: "Only in alpha"},
		},
		nil,
	)
	beta := startFakeServer(t, ctx, "Beta search service",
		[]mcp.Tool{{Name: "search", Description: "Search beta"}},
		nil,
	)

	alphaServer, err := mcpserver.NewServerFromSession(ctx, alpha)
	if err != nil {
		t.Fatal(err)
	}
	betaServer, err := mcpserver.NewServerFromSession(ctx, beta)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"alpha": alphaServer,
		"beta":  betaServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	session := connectTestClient(t, ctx, mgr)

	t.Run("use_skill_shows_resolved_names", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "use_skill",
			Arguments: map[string]any{"skill_name": "alpha"},
		})
		if err != nil {
			t.Fatal(err)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if !strings.Contains(tc.Text, "alpha_search(") {
			t.Errorf("expected prefixed 'alpha_search(' signature, got %q", tc.Text)
		}
		if !strings.Contains(tc.Text, "unique_tool(") {
			t.Errorf("expected 'unique_tool(' signature, got %q", tc.Text)
		}
	})

	t.Run("execute_code_prefixed_name", func(t *testing.T) {
		code := dedent(`
			alpha_search(q="test")
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
			t.Fatalf("failed to parse %q: %v", tc.Text, err)
		}
		if resp["tool"] != "search" {
			t.Errorf("tool = %v, want 'search' (original name sent downstream)", resp["tool"])
		}
	})

	t.Run("execute_code_unique_name", func(t *testing.T) {
		code := dedent(`
			unique_tool()
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		var resp map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
			t.Fatalf("failed to parse %q: %v", tc.Text, err)
		}
		if resp["tool"] != "unique_tool" {
			t.Errorf("tool = %v, want 'unique_tool'", resp["tool"])
		}
	})
}

func TestE2EStructuredOutput(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	type WeatherInput struct {
		Location string `json:"location" jsonschema:"city name"`
	}
	type WeatherOutput struct {
		Temperature float64 `json:"temperature"`
		Conditions  string  `json:"conditions"`
	}

	ds := mcp.NewServer(&mcp.Implementation{Name: "weather-server"}, nil)
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "get_weather", Description: "Get weather data"},
		func(ctx context.Context, req *mcp.CallToolRequest, input WeatherInput) (*mcp.CallToolResult, WeatherOutput, error) {
			return &mcp.CallToolResult{}, WeatherOutput{
				Temperature: 22.5,
				Conditions:  "Partly cloudy",
			}, nil
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
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"weather": srv})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	session := connectTestClient(t, ctx, mgr)

	t.Run("access_string_field", func(t *testing.T) {
		code := dedent(`
			w = get_weather(location="NYC")
			w["conditions"]
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if tc.Text != "Partly cloudy" {
			t.Errorf("expected 'Partly cloudy', got %q", tc.Text)
		}
	})

	t.Run("access_numeric_field", func(t *testing.T) {
		code := dedent(`
			w = get_weather(location="NYC")
			w["temperature"]
		`)
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": code},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			tc := result.Content[0].(*mcp.TextContent)
			t.Fatalf("error: %s", tc.Text)
		}
		tc := result.Content[0].(*mcp.TextContent)
		if tc.Text != "22.5" {
			t.Errorf("expected '22.5', got %q", tc.Text)
		}
	})
}

// dedent strips the common leading whitespace from all non-empty lines.
func dedent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")

	// Find minimum indentation across non-empty lines.
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(trimmed)
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent <= 0 {
		return strings.TrimSpace(s)
	}

	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func splitOnce(s, sep string) []string {
	i := strings.Index(s, sep)
	if i < 0 {
		return []string{s}
	}
	return []string{s[:i], s[i+len(sep):]}
}
