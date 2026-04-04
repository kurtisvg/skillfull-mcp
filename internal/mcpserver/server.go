package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"skillful-mcp/internal/config"
	"skillful-mcp/internal/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps a connected MCP client session to a downstream server.
type Server struct {
	session      *mcp.ClientSession
	instructions string
	tools        []*mcp.Tool
	resources    []*mcp.Resource
}

// NewServer connects to a downstream MCP server and applies config options
// (description override, tool/resource allowlists).
func NewServer(ctx context.Context, srv config.Server) (*Server, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "skillful-mcp",
		Version: version.Version,
	}, nil)

	var transport mcp.Transport
	switch s := srv.(type) {
	case *config.StdioServer:
		cmd := exec.Command(s.Command, s.Args...)
		cmd.Env = toEnv(s.Env)
		transport = &mcp.CommandTransport{Command: cmd}
	case *config.HTTPServer:
		httpClient := &http.Client{Transport: &headerTransport{base: http.DefaultTransport, headers: s.Headers}}
		transport = &mcp.StreamableClientTransport{Endpoint: s.URL, HTTPClient: httpClient}
	case *config.SSEServer:
		httpClient := &http.Client{Transport: &headerTransport{base: http.DefaultTransport, headers: s.Headers}}
		transport = &mcp.SSEClientTransport{Endpoint: s.URL, HTTPClient: httpClient}
	default:
		return nil, fmt.Errorf("unsupported server config type: %T", srv)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	s, err := NewServerFromSession(ctx, session, srv.Options())
	if err != nil {
		session.Close()
		return nil, err
	}
	return s, nil
}

func (s *Server) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	return s.session.CallTool(ctx, params)
}

func (s *Server) Resources() []*mcp.Resource {
	return s.resources
}

func (s *Server) ReadResource(ctx context.Context, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	return s.session.ReadResource(ctx, params)
}

func (s *Server) Instructions() string {
	return s.instructions
}

func (s *Server) Close() error {
	return s.session.Close()
}

// NewServerFromSession creates a Server from a pre-built session and applies
// optional configuration (description override, tool/resource allowlists).
func NewServerFromSession(ctx context.Context, session *mcp.ClientSession, opts ...config.ServerOptions) (*Server, error) {
	s := &Server{session: session}
	if session != nil {
		if res := session.InitializeResult(); res != nil {
			s.instructions = res.Instructions
		}
		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				return nil, err
			}
			s.tools = append(s.tools, tool)
		}
		// Resources are optional — some servers don't support them.
		for r, err := range session.Resources(ctx, nil) {
			if err != nil {
				break
			}
			s.resources = append(s.resources, r)
		}
	}

	if len(opts) > 0 {
		o := opts[0]
		if o.Description != "" {
			s.instructions = o.Description
		}
		if len(o.AllowedTools) > 0 {
			s.tools = filterTools(s.tools, o.AllowedTools)
		}
		if len(o.AllowedResources) > 0 {
			s.resources = filterResources(s.resources, o.AllowedResources)
		}
	}

	return s, nil
}

func filterTools(tools []*mcp.Tool, allowed []string) []*mcp.Tool {
	set := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		set[name] = true
	}
	filtered := make([]*mcp.Tool, 0, len(allowed))
	for _, t := range tools {
		if set[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterResources(resources []*mcp.Resource, allowed []string) []*mcp.Resource {
	set := make(map[string]bool, len(allowed))
	for _, uri := range allowed {
		set[uri] = true
	}
	filtered := make([]*mcp.Resource, 0, len(allowed))
	for _, r := range resources {
		if set[r.URI] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// toEnv converts the configured env map to a slice for exec.Cmd.
// Only the explicitly specified vars are passed to the child process.
// Returns an empty slice (not nil) when no vars are configured, so
// the child gets a clean environment rather than inheriting the parent's.
func toEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// headerTransport injects custom HTTP headers into every request.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}
