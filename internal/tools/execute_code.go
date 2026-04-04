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
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "execute_code",
			Description: executeCodeDescription,
		},
		newExecuteCode(mgr),
	)
}

func newExecuteCode(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, executeCodeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input executeCodeInput) (*mcp.CallToolResult, any, error) {
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

		tools := mgr.AllTools()
		fns := make(map[string]monty.ExternalFunction, len(tools))
		for _, t := range tools {
			srv, err := mgr.GetServer(t.ServerName)
			if err != nil {
				continue
			}
			fns[t.ResolvedName] = buildTool(t, srv)
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
	}
}

// buildTool creates a Monty external function for a single downstream tool.
func buildTool(t mcpserver.Tool, srv *mcpserver.Server) monty.ExternalFunction {
	paramByName := make(map[string]mcpserver.ParamInfo, len(t.Params))
	for _, p := range t.Params {
		paramByName[p.Name] = p
	}

	return func(fnCtx context.Context, call monty.Call) (monty.Result, error) {
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

		return monty.Return(extractResult(toolResult)), nil
	}
}

// validateMontyValue checks that a Monty value matches the expected JSON Schema
// types for a parameter. Returns nil if validation passes or types are unknown.
func validateMontyValue(v monty.Value, param mcpserver.ParamInfo) error {
	types := extractSchemaTypes(param.Schema)
	if len(types) == 0 {
		return nil
	}
	kind := v.Kind()
	for _, t := range types {
		if jsonSchemaTypeMatchesMonty(t, kind) {
			return nil
		}
	}
	return fmt.Errorf("parameter %q: expected %s, got %s", param.Name, types[0], kind)
}

// extractSchemaTypes extracts the top-level type(s) from a JSON Schema property.
func extractSchemaTypes(schema any) []string {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	switch t := m["type"].(type) {
	case string:
		return []string{t}
	case []any:
		types := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
		return types
	default:
		return nil
	}
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

// anyToMonty converts a Go value (from JSON unmarshaling) to a Monty Value.
func anyToMonty(v any) monty.Value {
	switch val := v.(type) {
	case string:
		return monty.String(val)
	case float64:
		if val == float64(int64(val)) {
			return monty.Int(int64(val))
		}
		return monty.Float(val)
	case bool:
		return monty.Bool(val)
	case nil:
		return monty.None()
	case map[string]any:
		pairs := make(monty.Dict, 0, len(val))
		for k, v := range val {
			pairs = append(pairs, monty.Pair{Key: monty.String(k), Value: anyToMonty(v)})
		}
		return monty.DictValue(pairs)
	case []any:
		items := make([]monty.Value, len(val))
		for i, item := range val {
			items[i] = anyToMonty(item)
		}
		return monty.List(items...)
	default:
		return monty.String(fmt.Sprintf("%v", val))
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

// extractResult converts a CallToolResult to a Monty value.
// Prefers structured content when available; otherwise extracts first text content.
func extractResult(result *mcp.CallToolResult) monty.Value {
	if result.StructuredContent != nil {
		return anyToMonty(result.StructuredContent)
	}
	return monty.String(extractText(result))
}
