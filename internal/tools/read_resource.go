package tools

import (
	"context"

	"skillful-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type readResourceInput struct {
	SkillName   string `json:"skill_name" jsonschema:"name of the skill that owns the resource"`
	ResourceURI string `json:"resource_uri" jsonschema:"URI of the resource to read"`
}

func RegisterReadResource(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "read_resource",
			Description: "Read a resource from a specific skill",
		},
		newReadResource(mgr),
	)
}

func newReadResource(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, readResourceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input readResourceInput) (*mcp.CallToolResult, any, error) {
		srv, err := mgr.GetServer(input.SkillName)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		readResult, err := srv.ReadResource(ctx, &mcp.ReadResourceParams{
			URI: input.ResourceURI,
		})
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		var content []mcp.Content
		for _, rc := range readResult.Contents {
			content = append(content, &mcp.EmbeddedResource{Resource: rc})
		}

		return &mcp.CallToolResult{Content: content}, nil, nil
	}
}
