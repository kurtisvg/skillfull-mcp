package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startFakeServer creates an in-memory MCP server with one registered tool,
// connects a client to it, and returns the client session.
func startFakeServer(t *testing.T, ctx context.Context, toolName string, opts ...*mcp.ServerOptions) *mcp.ClientSession {
	t.Helper()

	var serverOpts *mcp.ServerOptions
	if len(opts) > 0 {
		serverOpts = opts[0]
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-server"}, serverOpts)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        toolName,
		Description: "A test tool",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil, nil
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect test client: %v", err)
	}
	return session
}

func TestServerCallTool(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "echo"))
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

func TestServerListResources(t *testing.T) {
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

	result, err := s.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources error: %v", err)
	}
	if len(result.Resources) != 1 || result.Resources[0].URI != "test://r" {
		t.Errorf("expected [test://r], got %v", result.Resources)
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
		s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool", &mcp.ServerOptions{
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
		s, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool"))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if s.Instructions() != "" {
			t.Errorf("Instructions() = %q, want empty", s.Instructions())
		}
	})
}
