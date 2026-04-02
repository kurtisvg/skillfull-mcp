package config

import (
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

type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

type ServerConfig struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (s *ServerConfig) TransportType() (TransportType, error) {
	switch s.Type {
	case "", "stdio":
		return TransportSTDIO, nil
	case "http":
		return TransportHTTP, nil
	case "sse":
		return TransportSSE, nil
	default:
		return "", fmt.Errorf("unknown transport type: %q", s.Type)
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.expandEnvVars(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// envVarPattern matches ${VAR} references in config values.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

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

// expandStringMap expands env vars in all values of a map.
func expandStringMap(m map[string]string) error {
	for k, v := range m {
		expanded, err := expandString(v)
		if err != nil {
			return err
		}
		m[k] = expanded
	}
	return nil
}

// expandEnvVars expands ${VAR} references across all string fields in the config.
func (c *Config) expandEnvVars() error {
	for name, srv := range c.MCPServers {
		var err error
		if srv.Command, err = expandString(srv.Command); err != nil {
			return fmt.Errorf("server %q command: %w", name, err)
		}
		if srv.URL, err = expandString(srv.URL); err != nil {
			return fmt.Errorf("server %q url: %w", name, err)
		}
		for i, arg := range srv.Args {
			if srv.Args[i], err = expandString(arg); err != nil {
				return fmt.Errorf("server %q args[%d]: %w", name, i, err)
			}
		}
		if err := expandStringMap(srv.Env); err != nil {
			return fmt.Errorf("server %q env: %w", name, err)
		}
		if err := expandStringMap(srv.Headers); err != nil {
			return fmt.Errorf("server %q headers: %w", name, err)
		}
		c.MCPServers[name] = srv
	}
	return nil
}

func (c *Config) Validate() error {
	for name, srv := range c.MCPServers {
		tt, err := srv.TransportType()
		if err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
		switch tt {
		case TransportSTDIO:
			if srv.Command == "" {
				return fmt.Errorf("server %q: STDIO transport requires 'command'", name)
			}
		case TransportHTTP, TransportSSE:
			if srv.URL == "" {
				return fmt.Errorf("server %q: %s transport requires 'url'", name, tt)
			}
		}
	}
	return nil
}
