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
}

// ParamInfo describes a parameter extracted from a tool's JSON Schema.
type ParamInfo struct {
	Name     string   // property name from JSON Schema
	Types    []string // allowed JSON Schema types, e.g. ["string"] or ["string", "null"]
	Required bool
}

// Signature returns a Python-style function signature.
func (t *Tool) Signature() string {
	var parts []string
	for _, p := range t.Params {
		pyType := jsonSchemaToPython(p.Types)
		part := p.Name + ": " + pyType
		if !p.Required {
			part += " = None"
		}
		parts = append(parts, part)
	}

	sig := fmt.Sprintf("%s(%s) -> str", t.ResolvedName, strings.Join(parts, ", "))
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
					Types:    extractPropertyTypes(props[name]),
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
			Types:    extractPropertyTypes(props[name]),
			Required: false,
		})
	}

	return params, nil
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

// jsonSchemaToPython maps JSON Schema types to Python type annotations.
func jsonSchemaToPython(types []string) string {
	if len(types) == 0 {
		return "any"
	}
	var pyTypes []string
	nullable := false
	for _, t := range types {
		switch t {
		case "string":
			pyTypes = append(pyTypes, "str")
		case "integer":
			pyTypes = append(pyTypes, "int")
		case "number":
			pyTypes = append(pyTypes, "float")
		case "boolean":
			pyTypes = append(pyTypes, "bool")
		case "array":
			pyTypes = append(pyTypes, "list")
		case "object":
			pyTypes = append(pyTypes, "dict")
		case "null":
			nullable = true
		default:
			pyTypes = append(pyTypes, t)
		}
	}
	if len(pyTypes) == 0 {
		return "None"
	}
	result := strings.Join(pyTypes, " | ")
	if nullable {
		result += " | None"
	}
	return result
}
