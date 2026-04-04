package e2e_test

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

// dedent strips the common leading whitespace from all non-empty lines.
func dedent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")

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
