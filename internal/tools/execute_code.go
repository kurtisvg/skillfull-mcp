package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"skillful-mcp/internal/mcpserver"

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

func RegisterExecuteCode(s *mcp.Server, mgr *mcpserver.Manager) {
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

		fns := buildToolFunctions(mgr)

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
func buildToolFunctions(mgr *mcpserver.Manager) map[string]monty.ExternalFunction {
	tools := mgr.AllTools()
	fns := make(map[string]monty.ExternalFunction, len(tools))
	for _, t := range tools {
		srv, err := mgr.GetServer(t.SkillName)
		if err != nil {
			continue
		}
		paramByName := make(map[string]mcpserver.ParamInfo, len(t.Params))
		for _, p := range t.Params {
			paramByName[p.Name] = p
		}

		fns[t.ResolvedName] = func(fnCtx context.Context, call monty.Call) (monty.Result, error) {
			args := make(map[string]any)

			// Map positional args to parameter names from the schema, with type validation.
			for i, val := range call.Args {
				if i < len(t.Params) {
					if err := validateMontyValue(val, t.Params[i]); err != nil {
						msg := err.Error()
						return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
					}
					args[t.Params[i].Name] = montyValueToAny(val)
				}
			}

			// Keyword args override positional, with type validation.
			for _, pair := range call.Kwargs {
				key, ok := pair.Key.Raw().(string)
				if !ok {
					continue
				}
				if pi, ok := paramByName[key]; ok {
					if err := validateMontyValue(pair.Value, pi); err != nil {
						msg := err.Error()
						return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
					}
				}
				args[key] = montyValueToAny(pair.Value)
			}

			toolResult, err := srv.CallTool(fnCtx, &mcp.CallToolParams{
				Name:      t.OriginalName,
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

	return fns
}

// validateMontyValue checks that a Monty value matches the expected JSON Schema
// types for a parameter. Returns nil if validation passes or types are unknown.
func validateMontyValue(v monty.Value, param mcpserver.ParamInfo) error {
	if len(param.Types) == 0 {
		return nil
	}
	kind := v.Kind()
	for _, t := range param.Types {
		if jsonSchemaTypeMatchesMonty(t, kind) {
			return nil
		}
	}
	return fmt.Errorf("parameter %q: expected %s, got %s", param.Name, param.Types[0], kind)
}

// jsonSchemaTypeMatchesMonty checks if a JSON Schema type is compatible with a Monty ValueKind.
func jsonSchemaTypeMatchesMonty(schemaType string, kind monty.ValueKind) bool {
	switch schemaType {
	case "string":
		return kind == "string"
	case "integer":
		return kind == "int" || kind == "big_int"
	case "number":
		return kind == "int" || kind == "big_int" || kind == "float"
	case "boolean":
		return kind == "bool"
	case "array":
		return kind == "list" || kind == "tuple"
	case "object":
		return kind == "dict"
	case "null":
		return kind == "none"
	default:
		return false
	}
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
