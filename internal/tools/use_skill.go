package tools

import (
	"context"
	"fmt"
	"strings"

	"skillful-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type useSkillInput struct {
	SkillName string `json:"skill_name" jsonschema:"name of the skill to inspect"`
}

func RegisterUseSkill(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "use_skill",
			Description: "List tools and resources available in a specific skill. Tool names match the function names available in execute_code.",
		},
		newUseSkill(mgr),
	)
}

func newUseSkill(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, useSkillInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input useSkillInput) (*mcp.CallToolResult, any, error) {
		srv, err := mgr.GetServer(input.SkillName)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		var lines []string
		for _, t := range mgr.ServerTools(input.SkillName) {
			lines = append(lines, t.Signature())
		}

		// Resources are optional.
		resourcesResult, err := srv.ListResources(ctx, nil)
		if err == nil && resourcesResult != nil && len(resourcesResult.Resources) > 0 {
			lines = append(lines, "\nResources:")
			for _, r := range resourcesResult.Resources {
				if r.Description != "" {
					lines = append(lines, fmt.Sprintf("- %s: %s", r.URI, r.Description))
				} else {
					lines = append(lines, fmt.Sprintf("- %s", r.URI))
				}
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	}
}
