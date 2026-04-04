package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

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
func buildToolFunctions(ctx context.Context, mgr *mcpserver.Manager) (map[string]monty.ExternalFunction, error) {
	resolved, err := ResolveToolNames(ctx, mgr)
	if err != nil {
		return nil, err
	}

	fns := make(map[string]monty.ExternalFunction, len(resolved))
	for _, rt := range resolved {
		session, err := mgr.GetServer(rt.SkillName)
		if err != nil {
			continue
		}
		params := extractParamSchema(rt.Tool.InputSchema)
		paramByName := make(map[string]paramInfo, len(params))
		for _, p := range params {
			paramByName[p.Name] = p
		}

		fns[rt.ResolvedName] = func(fnCtx context.Context, call monty.Call) (monty.Result, error) {
			args := make(map[string]any)

			// Map positional args to parameter names from the schema, with type validation.
			for i, val := range call.Args {
				if i < len(params) {
					if err := validateMontyValue(val, params[i]); err != nil {
						msg := err.Error()
						return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
					}
					args[params[i].Name] = montyValueToAny(val)
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

			toolResult, err := session.CallTool(fnCtx, &mcp.CallToolParams{
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

// paramInfo describes a single parameter extracted from a tool's JSON Schema.
type paramInfo struct {
	Name  string   // property name from JSON Schema
	Types []string // allowed JSON Schema types, e.g. ["string"] or ["string", "null"]
}

// extractParamSchema extracts ordered parameter definitions from a JSON Schema.
// Ordering: required params first (in JSON array order), then non-required sorted
// lexicographically. This ensures deterministic positional argument mapping.
func extractParamSchema(schema any) []paramInfo {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		return nil
	}

	requiredSet := make(map[string]bool)
	var params []paramInfo

	// Required params come first, in the order declared by the JSON array.
	if required, ok := m["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				requiredSet[name] = true
				params = append(params, paramInfo{
					Name:  name,
					Types: extractPropertyTypes(props[name]),
				})
			}
		}
	}

	// Non-required params sorted lexicographically for deterministic ordering.
	var optional []string
	for name := range props {
		if !requiredSet[name] {
			optional = append(optional, name)
		}
	}
	sort.Strings(optional)

	for _, name := range optional {
		params = append(params, paramInfo{
			Name:  name,
			Types: extractPropertyTypes(props[name]),
		})
	}

	return params
}

// extractPropertyTypes extracts the "type" field from a JSON Schema property.
// Handles both string ("type": "string") and array ("type": ["string", "null"]) forms.
func extractPropertyTypes(propSchema any) []string {
	pm, ok := propSchema.(map[string]any)
	if !ok {
		return nil
	}
	switch t := pm["type"].(type) {
	case string:
		return []string{t}
	case []any:
		types := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
		if len(types) == 0 {
			return nil
		}
		return types
	default:
		return nil
	}
}

// validateMontyValue checks that a Monty value matches the expected JSON Schema
// types for a parameter. Returns nil if validation passes or types are unknown.
func validateMontyValue(v monty.Value, param paramInfo) error {
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
