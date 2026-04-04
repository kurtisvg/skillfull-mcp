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

	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"database":   mcpserver.NewServerFromSession(dbSession),
		"filesystem": mcpserver.NewServerFromSession(fsSession),
		"plain":      mcpserver.NewServerFromSession(plainSession),
	})
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
		var info struct {
			Skill     string                  `json:"skill"`
			Tools     []struct{ Name string } `json:"tools"`
			Resources []any                   `json:"resources"`
		}
		if err := json.Unmarshal([]byte(tc.Text), &info); err != nil {
			t.Fatal(err)
		}

		if info.Skill != "database" {
			t.Errorf("skill = %q, want database", info.Skill)
		}
		if len(info.Tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(info.Tools))
		}
		toolNames := map[string]bool{}
		for _, tool := range info.Tools {
			toolNames[tool.Name] = true
		}
		if !toolNames["execute_sql"] || !toolNames["list_tables"] {
			t.Errorf("expected execute_sql and list_tables, got %v", info.Tools)
		}
		if len(info.Resources) != 0 {
			t.Errorf("expected 0 resources, got %d", len(info.Resources))
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
		var info struct {
			Tools     []struct{ Name string } `json:"tools"`
			Resources []struct{ URI string }  `json:"resources"`
		}
		if err := json.Unmarshal([]byte(tc.Text), &info); err != nil {
			t.Fatal(err)
		}

		if len(info.Tools) != 1 || info.Tools[0].Name != "read_file" {
			t.Errorf("expected [read_file], got %v", info.Tools)
		}
		if len(info.Resources) != 1 || info.Resources[0].URI != "file:///tmp/test.txt" {
			t.Errorf("expected [file:///tmp/test.txt], got %v", info.Resources)
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
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": `execute_sql(sql="SELECT 1")`},
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
		code := `
a = execute_sql(sql="SELECT 1")
b = read_file(path="/tmp/test.txt")
a + " | " + b
`
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

	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"db": mcpserver.NewServerFromSession(dsSession)})
	defer mgr.Close()

	session := connectTestClient(t, ctx, mgr)

	t.Run("positional_arg", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": `execute_sql("SELECT 1")`},
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
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": `execute_sql(sql="SELECT 2")`},
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

	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"alpha": mcpserver.NewServerFromSession(alpha),
		"beta":  mcpserver.NewServerFromSession(beta),
	})
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
		var info struct {
			Tools []struct{ Name string } `json:"tools"`
		}
		if err := json.Unmarshal([]byte(tc.Text), &info); err != nil {
			t.Fatal(err)
		}

		names := map[string]bool{}
		for _, tool := range info.Tools {
			names[tool.Name] = true
		}
		if !names["alpha_search"] {
			t.Errorf("expected 'alpha_search' (prefixed due to conflict), got %v", names)
		}
		if !names["unique_tool"] {
			t.Errorf("expected 'unique_tool' (no prefix, no conflict), got %v", names)
		}
		if names["search"] {
			t.Error("'search' should be prefixed due to conflict")
		}
	})

	t.Run("execute_code_prefixed_name", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": `alpha_search(q="test")`},
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
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute_code",
			Arguments: map[string]any{"code": `unique_tool()`},
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

func splitOnce(s, sep string) []string {
	i := strings.Index(s, sep)
	if i < 0 {
		return []string{s}
	}
	return []string{s[:i], s[i+len(sep):]}
}
