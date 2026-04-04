package tools

import (
	"context"
	"testing"

	"skillful-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newFakeSession(t *testing.T, ctx context.Context, configure ...func(*mcp.Server)) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "fake"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "fake_tool", Description: "A test tool"}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})
	for _, fn := range configure {
		fn(s)
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

func TestUseSkillListsTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := newFakeSession(t, ctx)
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "fake_tool" {
		t.Errorf("expected [fake_tool], got %v", result.Tools)
	}
}

func TestUseSkillListsResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := newFakeSession(t, ctx, func(s *mcp.Server) {
		s.AddResource(&mcp.Resource{URI: "test://r", Name: "r"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{}, nil
		})
	})

	result, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Resources) != 1 || result.Resources[0].URI != "test://r" {
		t.Errorf("expected [test://r], got %v", result.Resources)
	}
}

func TestGetSessionUnknownSkill(t *testing.T) {
	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	_, err := mgr.GetServer("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}
