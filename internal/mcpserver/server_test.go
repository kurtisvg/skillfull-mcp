package mcpserver

import (
	"context"
	"testing"

	"skillful-mcp/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startFakeServer creates an in-memory MCP server with the given tools and
// resources, connects a client to it, and returns the client session.
func startFakeServer(t *testing.T, ctx context.Context, toolNames []string, resourceURIs []string, opts ...*mcp.ServerOptions) *mcp.ClientSession {
	t.Helper()

	var serverOpts *mcp.ServerOptions
	if len(opts) > 0 {
		serverOpts = opts[0]
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-server"}, serverOpts)
	for _, name := range toolNames {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        name,
			Description: "tool " + name,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		})
	}
	for _, uri := range resourceURIs {
		srv.AddResource(&mcp.Resource{URI: uri, Name: uri}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: uri, Text: "content"}},
			}, nil
		})
	}

	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestServerCallTool(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"echo"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "echo"})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "ok" {
		t.Errorf("text = %q, want %q", tc.Text, "ok")
	}
}

func TestServerResources(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-server"}, nil)
	srv.AddResource(&mcp.Resource{URI: "test://r", Name: "r"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "test://r", Text: "hello"}},
		}, nil
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resources := s.Resources()
	if len(resources) != 1 || resources[0].URI != "test://r" {
		t.Errorf("expected [test://r], got %v", resources)
	}
}

func TestServerReadResource(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-server"}, nil)
	srv.AddResource(&mcp.Resource{URI: "test://r", Name: "r"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "test://r", Text: "hello"}},
		}, nil
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result, err := s.ReadResource(ctx, &mcp.ReadResourceParams{URI: "test://r"})
	if err != nil {
		t.Fatalf("ReadResource error: %v", err)
	}
	if len(result.Contents) != 1 || result.Contents[0].Text != "hello" {
		t.Errorf("expected 'hello', got %v", result.Contents)
	}
}

func TestServerInstructions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("with instructions", func(t *testing.T) {
		t.Parallel()
		s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool"}, nil, &mcp.ServerOptions{
			Instructions: "Use this server for testing",
		}))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if s.Instructions() != "Use this server for testing" {
			t.Errorf("Instructions() = %q, want %q", s.Instructions(), "Use this server for testing")
		}
	})

	t.Run("without instructions", func(t *testing.T) {
		t.Parallel()
		s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool"}, nil))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if s.Instructions() != "" {
			t.Errorf("Instructions() = %q, want empty", s.Instructions())
		}
	})
}

func TestServerOptions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("description override", func(t *testing.T) {
		t.Parallel()
		session := startFakeServer(t, ctx, []string{"tool_a"}, nil)
		s, err := NewServerFromSession(ctx, session, config.ServerOptions{
			Description: "custom description",
		})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if s.Instructions() != "custom description" {
			t.Errorf("Instructions() = %q, want 'custom description'", s.Instructions())
		}
	})

	t.Run("allowed tools", func(t *testing.T) {
		t.Parallel()
		session := startFakeServer(t, ctx, []string{"tool_a", "tool_b", "tool_c"}, nil)
		s, err := NewServerFromSession(ctx, session, config.ServerOptions{
			AllowedTools: []string{"tool_a", "tool_c"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if len(s.tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(s.tools))
		}
		names := map[string]bool{}
		for _, tool := range s.tools {
			names[tool.Name] = true
		}
		if !names["tool_a"] || !names["tool_c"] {
			t.Errorf("expected tool_a and tool_c, got %v", names)
		}
		if names["tool_b"] {
			t.Error("tool_b should be filtered out")
		}
	})

	t.Run("allowed resources", func(t *testing.T) {
		t.Parallel()
		session := startFakeServer(t, ctx, []string{"tool_a"}, []string{"test://a", "test://b", "test://c"})
		s, err := NewServerFromSession(ctx, session, config.ServerOptions{
			AllowedResources: []string{"test://a", "test://c"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		resources := s.Resources()
		if len(resources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(resources))
		}
		uris := map[string]bool{}
		for _, r := range resources {
			uris[r.URI] = true
		}
		if !uris["test://a"] || !uris["test://c"] {
			t.Errorf("expected test://a and test://c, got %v", uris)
		}
		if uris["test://b"] {
			t.Error("test://b should be filtered out")
		}
	})

	t.Run("no filter", func(t *testing.T) {
		t.Parallel()
		session := startFakeServer(t, ctx, []string{"tool_a", "tool_b"}, []string{"test://r"})
		s, err := NewServerFromSession(ctx, session)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if len(s.tools) != 2 {
			t.Errorf("expected 2 tools, got %d", len(s.tools))
		}
		if len(s.Resources()) != 1 {
			t.Errorf("expected 1 resource, got %d", len(s.Resources()))
		}
	})
}
