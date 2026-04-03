package tools

import (
	"context"

	"skillful-mcp/internal/clientmanager"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ResolvedTool maps a resolved (potentially prefixed) function name to
// the skill and original tool name on the downstream server.
type ResolvedTool struct {
	ResolvedName string
	SkillName    string
	OriginalName string
	Tool         *mcp.Tool
}

// ResolveToolNames builds a mapping of all downstream tools, prefixing with
// the skill name only when multiple skills define a tool with the same name.
func ResolveToolNames(ctx context.Context, mgr *clientmanager.Manager) ([]ResolvedTool, error) {
	type entry struct {
		skillName string
		tool      *mcp.Tool
	}

	// Collect all tools grouped by original name.
	byName := make(map[string][]entry)
	for _, skillName := range mgr.ListServerNames() {
		session, err := mgr.GetSession(skillName)
		if err != nil {
			continue
		}
		result, err := session.ListTools(ctx, nil)
		if err != nil {
			continue
		}
		for _, tool := range result.Tools {
			byName[tool.Name] = append(byName[tool.Name], entry{skillName, tool})
		}
	}

	// Build resolved list: prefix only on conflict.
	var resolved []ResolvedTool
	for name, entries := range byName {
		if len(entries) == 1 {
			resolved = append(resolved, ResolvedTool{
				ResolvedName: name,
				SkillName:    entries[0].skillName,
				OriginalName: name,
				Tool:         entries[0].tool,
			})
		} else {
			for _, e := range entries {
				resolved = append(resolved, ResolvedTool{
					ResolvedName: e.skillName + "_" + name,
					SkillName:    e.skillName,
					OriginalName: name,
					Tool:         e.tool,
				})
			}
		}
	}

	return resolved, nil
}
