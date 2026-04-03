package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"skillful-mcp/internal/clientmanager"

	monty "github.com/ewhauser/gomonty"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type executeCodeInput struct {
	Code string `json:"code" jsonschema:"python code that calls downstream tools by name and returns a computed result"`
}

const executeCodeDescription = `Execute Python code in a secure sandbox to orchestrate multiple tool calls and return a computed result.

All downstream tools are available as functions, called by name:
  result = tool_name(arg1, arg2, key=value) -> str

Positional and keyword arguments are both supported.

IMPORTANT: Only call tools that were returned by use_skill or described in resources. Do not guess tool names or schemas — first call use_skill to discover the available tools and their input schemas for a given skill, then write code that calls those tools.`

func RegisterExecuteCode(s *mcp.Server, mgr *clientmanager.Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "execute_code",
		Description: executeCodeDescription,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input executeCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Code == "" {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("code must not be empty"))
			return result, nil, nil
		}

		runner, err := monty.New(input.Code, monty.CompileOptions{ScriptName: "script.py"})
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("compile error: %w", err))
			return result, nil, nil
		}

		fns, err := buildToolFunctions(ctx, mgr)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		value, err := runner.Run(ctx, monty.RunOptions{
			Functions: fns,
		})
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("runtime error: %w", err))
			return result, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: value.String()}},
		}, nil, nil
	})
}

// buildToolFunctions creates a Monty external function for each downstream tool,
// using resolved names (prefixed only on conflict).
func buildToolFunctions(ctx context.Context, mgr *clientmanager.Manager) (map[string]monty.ExternalFunction, error) {
	resolved, err := ResolveToolNames(ctx, mgr)
	if err != nil {
		return nil, err
	}

	fns := make(map[string]monty.ExternalFunction, len(resolved))
	for _, rt := range resolved {
		rt := rt
		session, err := mgr.GetSession(rt.SkillName)
		if err != nil {
			continue
		}
		paramNames := extractParamNames(rt.Tool.InputSchema)

		fns[rt.ResolvedName] = func(_ context.Context, call monty.Call) (monty.Result, error) {
			args := make(map[string]any)

			// Map positional args to parameter names from the schema.
			for i, val := range call.Args {
				if i < len(paramNames) {
					args[paramNames[i]] = montyValueToAny(val)
				}
			}

			// Keyword args override positional.
			for _, pair := range call.Kwargs {
				key, ok := pair.Key.Raw().(string)
				if !ok {
					continue
				}
				args[key] = montyValueToAny(pair.Value)
			}

			toolResult, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      rt.OriginalName,
				Arguments: args,
			})
			if err != nil {
				return monty.Return(monty.String(fmt.Sprintf("error: %v", err))), nil
			}

			if toolResult.IsError {
				text := extractText(toolResult)
				return monty.Return(monty.String(fmt.Sprintf("error: %s", text))), nil
			}

			return monty.Return(monty.String(extractText(toolResult))), nil
		}
	}

	return fns, nil
}

// extractParamNames extracts ordered property names from a JSON Schema input schema.
// The InputSchema from the client is typically a map[string]any.
func extractParamNames(schema any) []string {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		return nil
	}
	// Check for explicit ordering via "required" array (common convention).
	if required, ok := m["required"].([]any); ok {
		names := make([]string, 0, len(required))
		for _, r := range required {
			if s, ok := r.(string); ok {
				names = append(names, s)
			}
		}
		// Append non-required properties.
		for name := range props {
			found := false
			for _, n := range names {
				if n == name {
					found = true
					break
				}
			}
			if !found {
				names = append(names, name)
			}
		}
		return names
	}
	// No required field — just return property names.
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	return names
}

// montyValueToAny converts a Monty Value to a Go value suitable for JSON tool arguments.
func montyValueToAny(v monty.Value) any {
	raw := v.Raw()
	switch val := raw.(type) {
	case int64:
		return val
	case float64:
		return val
	case string:
		return val
	case bool:
		return val
	case monty.Dict:
		m := make(map[string]any, len(val))
		for _, pair := range val {
			if key, ok := pair.Key.Raw().(string); ok {
				m[key] = montyValueToAny(pair.Value)
			}
		}
		return m
	case []monty.Value:
		list := make([]any, len(val))
		for i, item := range val {
			list[i] = montyValueToAny(item)
		}
		return list
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return v.String()
		}
		return json.RawMessage(data)
	}
}

// extractText pulls the first text content from a CallToolResult.
func extractText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
