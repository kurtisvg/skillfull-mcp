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
	t.Parallel()

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
			name: "valid explicit stdio server",
			json: `{"mcpServers":{"fs":{"type":"stdio","command":"npx"}}}`,
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
			name:    "nonexistent file",
			json:    "", // handled specially below
			wantErr: true,
		},
		{
			name:    "malformed json",
			json:    `{not json`,
			wantErr: true,
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
		{
			name:    "unknown field on stdio",
			json:    `{"mcpServers":{"fs":{"command":"npx","url":"https://bad"}}}`,
			wantErr: true,
		},
		{
			name:    "unknown field on http",
			json:    `{"mcpServers":{"api":{"type":"http","url":"https://x","command":"bad"}}}`,
			wantErr: true,
		},
		{
			name:    "unknown field on sse",
			json:    `{"mcpServers":{"api":{"type":"sse","url":"https://x","command":"bad"}}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var path string
			if tt.name == "nonexistent file" {
				path = "/nonexistent/path/mcp.json"
			} else {
				path = writeTestConfig(t, tt.json)
			}

			servers, err := Load(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if servers == nil {
				t.Fatal("expected non-nil result")
			}
		})
	}
}

func TestLoadedFieldValues(t *testing.T) {
	t.Parallel()

	jsonStr := `{
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
			},
			"events": {
				"type": "sse",
				"url": "https://example.com/sse",
				"headers": {"X-Key": "abc"}
			}
		}
	}`
	path := writeTestConfig(t, jsonStr)
	servers, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// STDIO
	fs, ok := servers["fs"].(*StdioServer)
	if !ok {
		t.Fatalf("expected *StdioServer, got %T", servers["fs"])
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
	if fs.TransportType() != TransportSTDIO {
		t.Errorf("fs.TransportType() = %q, want stdio", fs.TransportType())
	}

	// HTTP
	api, ok := servers["api"].(*HTTPServer)
	if !ok {
		t.Fatalf("expected *HTTPServer, got %T", servers["api"])
	}
	if api.URL != "https://example.com/mcp" {
		t.Errorf("api.URL = %q, want 'https://example.com/mcp'", api.URL)
	}
	if api.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("api.Headers[Authorization] = %q, want 'Bearer tok'", api.Headers["Authorization"])
	}
	if api.TransportType() != TransportHTTP {
		t.Errorf("api.TransportType() = %q, want http", api.TransportType())
	}

	// SSE
	events, ok := servers["events"].(*SSEServer)
	if !ok {
		t.Fatalf("expected *SSEServer, got %T", servers["events"])
	}
	if events.URL != "https://example.com/sse" {
		t.Errorf("events.URL = %q, want 'https://example.com/sse'", events.URL)
	}
	if events.Headers["X-Key"] != "abc" {
		t.Errorf("events.Headers[X-Key] = %q, want 'abc'", events.Headers["X-Key"])
	}
	if events.TransportType() != TransportSSE {
		t.Errorf("events.TransportType() = %q, want sse", events.TransportType())
	}
}

func TestServerOptions(t *testing.T) {
	t.Parallel()

	jsonStr := `{
		"mcpServers": {
			"db": {
				"command": "db-server",
				"description": "SQL database for analytics",
				"allowedTools": ["execute_sql", "list_tables"],
				"allowedResources": ["schema://tables"]
			},
			"plain": {
				"command": "plain-server"
			}
		}
	}`
	path := writeTestConfig(t, jsonStr)
	servers, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	db := servers["db"].(*StdioServer)
	opts := db.Options()
	if opts.Description != "SQL database for analytics" {
		t.Errorf("Description = %q, want 'SQL database for analytics'", opts.Description)
	}
	if len(opts.AllowedTools) != 2 || opts.AllowedTools[0] != "execute_sql" || opts.AllowedTools[1] != "list_tables" {
		t.Errorf("AllowedTools = %v, want [execute_sql list_tables]", opts.AllowedTools)
	}
	if len(opts.AllowedResources) != 1 || opts.AllowedResources[0] != "schema://tables" {
		t.Errorf("AllowedResources = %v, want [schema://tables]", opts.AllowedResources)
	}

	// Server with no options should have zero values.
	plain := servers["plain"].(*StdioServer)
	plainOpts := plain.Options()
	if plainOpts.Description != "" {
		t.Errorf("expected empty description, got %q", plainOpts.Description)
	}
	if len(plainOpts.AllowedTools) != 0 {
		t.Errorf("expected no allowed tools, got %v", plainOpts.AllowedTools)
	}
	if len(plainOpts.AllowedResources) != 0 {
		t.Errorf("expected no allowed resources, got %v", plainOpts.AllowedResources)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_CMD", "my-server")
	t.Setenv("TEST_ARG", "--verbose")
	t.Setenv("TEST_URL", "https://api.example.com")
	t.Setenv("TEST_TOKEN", "secret123")
	t.Setenv("TEST_ENV_VAL", "debug-mode")
	t.Setenv("TEST_SSE_URL", "https://sse.example.com")

	jsonStr := `{
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
			},
			"s3": {
				"type": "sse",
				"url": "${TEST_SSE_URL}/events"
			}
		}
	}`
	path := writeTestConfig(t, jsonStr)
	servers, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	s1 := servers["s1"].(*StdioServer)
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

	s2 := servers["s2"].(*HTTPServer)
	if s2.URL != "https://api.example.com/mcp" {
		t.Errorf("url = %q, want 'https://api.example.com/mcp'", s2.URL)
	}
	if s2.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("header = %q, want 'Bearer secret123'", s2.Headers["Authorization"])
	}

	s3 := servers["s3"].(*SSEServer)
	if s3.URL != "https://sse.example.com/events" {
		t.Errorf("url = %q, want 'https://sse.example.com/events'", s3.URL)
	}
}

func TestEnvVarExpansionMissingVar(t *testing.T) {
	t.Setenv("SKILLFUL_TEST_MISSING", "")
	os.Unsetenv("SKILLFUL_TEST_MISSING")

	jsonStr := `{
		"mcpServers": {
			"s1": {
				"type": "http",
				"url": "${SKILLFUL_TEST_MISSING}/mcp"
			}
		}
	}`
	path := writeTestConfig(t, jsonStr)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestEnvVarNoExpansionWithoutBraces(t *testing.T) {
	t.Setenv("NOT_EXPANDED", "")
	os.Unsetenv("NOT_EXPANDED")

	jsonStr := `{
		"mcpServers": {
			"s1": {
				"command": "$NOT_EXPANDED"
			}
		}
	}`
	path := writeTestConfig(t, jsonStr)
	servers, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s1 := servers["s1"].(*StdioServer)
	if s1.Command != "$NOT_EXPANDED" {
		t.Errorf("bare $VAR should not expand, got %q", s1.Command)
	}
}
