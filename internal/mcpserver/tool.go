package mcpserver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool represents a downstream tool with its resolved (potentially prefixed) name.
type Tool struct {
	ResolvedName string
	OriginalName string
	SkillName    string
	Description  string
	Params       []ParamInfo
	OutputSchema any // raw JSON Schema for output, nil = unstructured (str)
}

// ParamInfo describes a parameter extracted from a tool's JSON Schema.
type ParamInfo struct {
	Name     string
	Schema   any // raw JSON Schema for this property
	Required bool
}

// Signature returns a Python-style function signature.
func (t *Tool) Signature() string {
	var parts []string
	for _, p := range t.Params {
		pyType := formatSchema(p.Schema)
		part := p.Name + ": " + pyType
		if !p.Required {
			part += " = None"
		}
		parts = append(parts, part)
	}

	returnType := "str"
	if t.OutputSchema != nil {
		returnType = formatSchema(t.OutputSchema)
	}

	sig := fmt.Sprintf("%s(%s) -> %s", t.ResolvedName, strings.Join(parts, ", "), returnType)
	if t.Description != "" {
		sig += "\n  " + t.Description
	}
	return sig
}

func newTool(resolvedName, originalName, skillName string, tool *mcp.Tool) (Tool, error) {
	params, err := extractParamSchema(tool.InputSchema)
	if err != nil {
		return Tool{}, fmt.Errorf("tool %q: %w", originalName, err)
	}
	return Tool{
		ResolvedName: resolvedName,
		OriginalName: originalName,
		SkillName:    skillName,
		Description:  tool.Description,
		Params:       params,
		OutputSchema: tool.OutputSchema,
	}, nil
}

// extractParamSchema extracts ordered parameter definitions from a JSON Schema.
// Ordering: required params first (in JSON array order), then non-required sorted
// lexicographically. This ensures deterministic positional argument mapping.
// Returns nil with no error for nil schemas (parameterless tools).
func extractParamSchema(schema any) ([]ParamInfo, error) {
	if schema == nil {
		return nil, nil
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object schema, got %T", schema)
	}
	rawProps, exists := m["properties"]
	if !exists {
		return nil, nil
	}
	props, ok := rawProps.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected properties to be object, got %T", rawProps)
	}

	requiredSet := make(map[string]bool)
	var params []ParamInfo

	// Required params come first, in the order declared by the JSON array.
	if required, ok := m["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				requiredSet[name] = true
				params = append(params, ParamInfo{
					Name:     name,
					Schema:   props[name],
					Required: true,
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
		params = append(params, ParamInfo{
			Name:     name,
			Schema:   props[name],
			Required: false,
		})
	}

	return params, nil
}

// formatSchema converts a JSON Schema to a Python type annotation string.
// Handles nested types: objects with properties, arrays with items, unions.
func formatSchema(schema any) string {
	if schema == nil {
		return "any"
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return "any"
	}

	// Handle type field — can be string or array (union).
	switch t := m["type"].(type) {
	case string:
		return formatSingleType(t, m)
	case []any:
		var pyTypes []string
		nullable := false
		for _, item := range t {
			if s, ok := item.(string); ok {
				if s == "null" {
					nullable = true
				} else {
					pyTypes = append(pyTypes, formatSingleType(s, m))
				}
			}
		}
		if len(pyTypes) == 0 {
			if nullable {
				return "None"
			}
			return "any"
		}
		result := strings.Join(pyTypes, " | ")
		if nullable {
			result += " | None"
		}
		return result
	default:
		return "any"
	}
}

// formatSingleType formats a single JSON Schema type with nested detail.
func formatSingleType(typeName string, schema map[string]any) string {
	switch typeName {
	case "string":
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "null":
		return "None"
	case "array":
		if items, ok := schema["items"]; ok {
			return "list[" + formatSchema(items) + "]"
		}
		return "list"
	case "object":
		if props, ok := schema["properties"].(map[string]any); ok && len(props) > 0 {
			return formatObjectProps(props)
		}
		return "dict"
	default:
		return typeName
	}
}

// formatObjectProps formats an object schema's properties as {"key": type, ...}.
func formatObjectProps(props map[string]any) string {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%q: %s", k, formatSchema(props[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
