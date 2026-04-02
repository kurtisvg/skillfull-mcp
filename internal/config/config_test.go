package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "valid stdio server",
			json: `{"mcpServers":{"fs":{"command":"npx","args":["-y","server"]}}}`,
		},
		{
			name: "valid http server",
			json: `{"mcpServers":{"api":{"type":"http","url":"https://example.com/mcp","headers":{"Authorization":"Bearer tok"}}}}`,
		},
		{
			name: "valid sse server",
			json: `{"mcpServers":{"api":{"type":"sse","url":"https://example.com/sse"}}}`,
		},
		{
			name: "mixed stdio and http",
			json: `{"mcpServers":{"fs":{"command":"npx","args":[]},"api":{"type":"http","url":"https://example.com"}}}`,
		},
		{
			name: "empty mcpServers",
			json: `{"mcpServers":{}}`,
		},
		{
			name:    "malformed json",
			json:    `{not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.json)
			cfg, err := Load(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
		})
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/mcp.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestTransportType(t *testing.T) {
	tests := []struct {
		name     string
		typeVal  string
		expected TransportType
		wantErr  bool
	}{
		{"empty defaults to stdio", "", TransportSTDIO, false},
		{"explicit stdio", "stdio", TransportSTDIO, false},
		{"explicit http", "http", TransportHTTP, false},
		{"explicit sse", "sse", TransportSSE, false},
		{"unknown type errors", "grpc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ServerConfig{Type: tt.typeVal}
			got, err := s.TransportType()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for type %q", tt.typeVal)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "valid stdio",
			json: `{"mcpServers":{"fs":{"command":"npx"}}}`,
		},
		{
			name: "valid http",
			json: `{"mcpServers":{"api":{"type":"http","url":"https://example.com"}}}`,
		},
		{
			name: "empty servers is valid",
			json: `{"mcpServers":{}}`,
		},
		{
			name:    "stdio missing command",
			json:    `{"mcpServers":{"fs":{}}}`,
			wantErr: true,
		},
		{
			name:    "http missing url",
			json:    `{"mcpServers":{"api":{"type":"http"}}}`,
			wantErr: true,
		},
		{
			name:    "sse missing url",
			json:    `{"mcpServers":{"api":{"type":"sse"}}}`,
			wantErr: true,
		},
		{
			name:    "unknown transport type",
			json:    `{"mcpServers":{"api":{"type":"grpc"}}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.json)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("load error: %v", err)
			}
			err = cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestLoadedFieldValues(t *testing.T) {
	json := `{
		"mcpServers": {
			"fs": {
				"command": "npx",
				"args": ["-y", "server"],
				"env": {"DEBUG": "true"}
			},
			"api": {
				"type": "http",
				"url": "https://example.com/mcp",
				"headers": {"Authorization": "Bearer tok"}
			}
		}
	}`
	path := writeTestConfig(t, json)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Check STDIO server fields
	fs, ok := cfg.MCPServers["fs"]
	if !ok {
		t.Fatal("missing 'fs' server")
	}
	if fs.Command != "npx" {
		t.Errorf("fs.Command = %q, want 'npx'", fs.Command)
	}
	if len(fs.Args) != 2 || fs.Args[0] != "-y" || fs.Args[1] != "server" {
		t.Errorf("fs.Args = %v, want [-y server]", fs.Args)
	}
	if fs.Env["DEBUG"] != "true" {
		t.Errorf("fs.Env[DEBUG] = %q, want 'true'", fs.Env["DEBUG"])
	}
	if tt, err := fs.TransportType(); err != nil || tt != TransportSTDIO {
		t.Errorf("fs.TransportType() = %q, %v; want stdio", tt, err)
	}

	// Check HTTP server fields
	api, ok := cfg.MCPServers["api"]
	if !ok {
		t.Fatal("missing 'api' server")
	}
	if api.URL != "https://example.com/mcp" {
		t.Errorf("api.URL = %q, want 'https://example.com/mcp'", api.URL)
	}
	if api.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("api.Headers[Authorization] = %q, want 'Bearer tok'", api.Headers["Authorization"])
	}
	if tt, err := api.TransportType(); err != nil || tt != TransportHTTP {
		t.Errorf("api.TransportType() = %q, %v; want http", tt, err)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_CMD", "my-server")
	t.Setenv("TEST_ARG", "--verbose")
	t.Setenv("TEST_URL", "https://api.example.com")
	t.Setenv("TEST_TOKEN", "secret123")
	t.Setenv("TEST_ENV_VAL", "debug-mode")

	json := `{
		"mcpServers": {
			"s1": {
				"command": "${TEST_CMD}",
				"args": ["${TEST_ARG}", "literal"],
				"env": {"MODE": "${TEST_ENV_VAL}"}
			},
			"s2": {
				"type": "http",
				"url": "${TEST_URL}/mcp",
				"headers": {"Authorization": "Bearer ${TEST_TOKEN}"}
			}
		}
	}`
	path := writeTestConfig(t, json)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	s1 := cfg.MCPServers["s1"]
	if s1.Command != "my-server" {
		t.Errorf("command = %q, want 'my-server'", s1.Command)
	}
	if s1.Args[0] != "--verbose" {
		t.Errorf("args[0] = %q, want '--verbose'", s1.Args[0])
	}
	if s1.Args[1] != "literal" {
		t.Errorf("args[1] = %q, want 'literal'", s1.Args[1])
	}
	if s1.Env["MODE"] != "debug-mode" {
		t.Errorf("env[MODE] = %q, want 'debug-mode'", s1.Env["MODE"])
	}

	s2 := cfg.MCPServers["s2"]
	if s2.URL != "https://api.example.com/mcp" {
		t.Errorf("url = %q, want 'https://api.example.com/mcp'", s2.URL)
	}
	if s2.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("header = %q, want 'Bearer secret123'", s2.Headers["Authorization"])
	}
}

func TestEnvVarExpansionMissingVar(t *testing.T) {
	json := `{
		"mcpServers": {
			"s1": {
				"type": "http",
				"url": "${THIS_VAR_DOES_NOT_EXIST}/mcp"
			}
		}
	}`
	path := writeTestConfig(t, json)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestEnvVarNoExpansionWithoutBraces(t *testing.T) {
	json := `{
		"mcpServers": {
			"s1": {
				"command": "$NOT_EXPANDED"
			}
		}
	}`
	path := writeTestConfig(t, json)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPServers["s1"].Command != "$NOT_EXPANDED" {
		t.Errorf("bare $VAR should not expand, got %q", cfg.MCPServers["s1"].Command)
	}
}
