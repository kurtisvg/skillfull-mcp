package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

type TransportType string

const (
	TransportSTDIO TransportType = "stdio"
	TransportHTTP  TransportType = "http"
	TransportSSE   TransportType = "sse"
)

// Server is the common interface for all server transport configurations.
type Server interface {
	TransportType() TransportType
	Options() ServerOptions
}

func Load(path string) (map[string]Server, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var raw struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	servers := make(map[string]Server, len(raw.MCPServers))
	for name, rawSrv := range raw.MCPServers {
		expanded, err := expandRawJSON(rawSrv)
		if err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}

		srv, err := unmarshalServer(expanded)
		if err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
		servers[name] = srv
	}
	return servers, nil
}

func unmarshalServer(data []byte) (Server, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}

	switch probe.Type {
	case "", "stdio":
		var s StdioServer
		if err := unmarshalStrict(data, &s); err != nil {
			return nil, err
		}
		if s.Command == "" {
			return nil, fmt.Errorf("stdio transport requires 'command'")
		}
		return &s, nil
	case "http":
		var h HTTPServer
		if err := unmarshalStrict(data, &h); err != nil {
			return nil, err
		}
		if h.URL == "" {
			return nil, fmt.Errorf("http transport requires 'url'")
		}
		return &h, nil
	case "sse":
		var s SSEServer
		if err := unmarshalStrict(data, &s); err != nil {
			return nil, err
		}
		if s.URL == "" {
			return nil, fmt.Errorf("sse transport requires 'url'")
		}
		return &s, nil
	default:
		return nil, fmt.Errorf("unknown transport type: %q", probe.Type)
	}
}

func unmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// ServerOptions contains optional fields shared by all server types.
type ServerOptions struct {
	Description      string   `json:"description,omitempty"`
	AllowedTools     []string `json:"allowedTools,omitempty"`
	AllowedResources []string `json:"allowedResources,omitempty"`
}

func (o ServerOptions) Options() ServerOptions { return o }

// Server types

type StdioServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	ServerOptions
}

func (*StdioServer) TransportType() TransportType { return TransportSTDIO }

type HTTPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	ServerOptions
}

func (*HTTPServer) TransportType() TransportType { return TransportHTTP }

type SSEServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	ServerOptions
}

func (*SSEServer) TransportType() TransportType { return TransportSSE }

// Env var expansion

// envVarPattern matches ${VAR} references in config values.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandRawJSON expands ${VAR} references in all string values of a raw JSON
// object. It unmarshals to map[string]any, walks all strings, and re-marshals
// to preserve JSON validity (handling special characters in expanded values).
func expandRawJSON(data json.RawMessage) (json.RawMessage, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if err := expandMap(raw); err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

// expandMap recursively expands ${VAR} references in all string values.
func expandMap(m map[string]any) error {
	for k, val := range m {
		switch v := val.(type) {
		case string:
			expanded, err := expandString(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			m[k] = expanded
		case map[string]any:
			if err := expandMap(v); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		case []any:
			for i, item := range v {
				if s, ok := item.(string); ok {
					expanded, err := expandString(s)
					if err != nil {
						return fmt.Errorf("%s[%d]: %w", k, i, err)
					}
					v[i] = expanded
				}
			}
		}
	}
	return nil
}

// expandString replaces all ${VAR} references with their environment variable
// values. Returns an error if a referenced variable is not set.
func expandString(s string) (string, error) {
	var expandErr error
	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(varName)
		if !ok {
			expandErr = fmt.Errorf("environment variable %q is not set", varName)
			return match
		}
		return val
	})
	if expandErr != nil {
		return "", expandErr
	}
	return result, nil
}
