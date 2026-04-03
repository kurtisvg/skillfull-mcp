package tools

import (
	"context"
	"encoding/json"

	"skillful-mcp/internal/clientmanager"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type useSkillInput struct {
	SkillName string `json:"skill_name" jsonschema:"name of the skill to inspect"`
}

type skillInfo struct {
	Skill     string          `json:"skill"`
	Tools     []resolvedInfo  `json:"tools"`
	Resources []*mcp.Resource `json:"resources"`
}

type resolvedInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

func RegisterUseSkill(s *mcp.Server, mgr *clientmanager.Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "use_skill",
		Description: "List tools and resources available in a specific skill. Tool names match the function names available in execute_code.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input useSkillInput) (*mcp.CallToolResult, any, error) {
		_, err := mgr.GetSession(input.SkillName)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		// Resolve names across all skills to determine prefixing.
		resolved, err := ResolveToolNames(ctx, mgr)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		var tools []resolvedInfo
		for _, rt := range resolved {
			if rt.SkillName == input.SkillName {
				tools = append(tools, resolvedInfo{
					Name:        rt.ResolvedName,
					Description: rt.Tool.Description,
					InputSchema: rt.Tool.InputSchema,
				})
			}
		}

		info := skillInfo{
			Skill:     input.SkillName,
			Tools:     tools,
			Resources: []*mcp.Resource{},
		}

		// Resources are optional — some servers don't support them.
		session, _ := mgr.GetSession(input.SkillName)
		resourcesResult, err := session.ListResources(ctx, nil)
		if err == nil && resourcesResult != nil {
			info.Resources = resourcesResult.Resources
		}

		data, err := json.Marshal(info)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})
}
