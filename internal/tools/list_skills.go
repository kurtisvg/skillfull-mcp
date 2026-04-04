package tools

import (
	"context"
	"fmt"
	"strings"

	"skillful-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listSkillsInput struct{}

func RegisterListSkills(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "list_skills",
			Description: "List all available skills with their descriptions",
		},
		newListSkills(mgr),
	)
}

func newListSkills(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, listSkillsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input listSkillsInput) (*mcp.CallToolResult, any, error) {
		var lines []string
		for _, name := range mgr.ListServerNames() {
			srv, err := mgr.GetServer(name)
			if err != nil {
				continue
			}
			if instr := srv.Instructions(); instr != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", name, instr))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	}
}
