package mcpserver

import (
	"context"
	"net/http"
	"os/exec"

	"skillful-mcp/internal/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps a connected MCP client session to a downstream server.
type Server struct {
	session *mcp.ClientSession
}

// NewServer connects to a downstream MCP server and returns a Server.
func NewServer(ctx context.Context, srv *config.ServerConfig) (*Server, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "skillful-mcp",
		Version: "0.1.0",
	}, nil)

	tt, err := srv.TransportType()
	if err != nil {
		return nil, err
	}

	var transport mcp.Transport
	switch tt {
	case config.TransportSTDIO:
		cmd := exec.Command(srv.Command, srv.Args...)
		cmd.Env = toEnv(srv.Env)
		transport = &mcp.CommandTransport{Command: cmd}
	case config.TransportHTTP:
		httpClient := &http.Client{Transport: &headerTransport{base: http.DefaultTransport, headers: srv.Headers}}
		transport = &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: httpClient}
	case config.TransportSSE:
		httpClient := &http.Client{Transport: &headerTransport{base: http.DefaultTransport, headers: srv.Headers}}
		transport = &mcp.SSEClientTransport{Endpoint: srv.URL, HTTPClient: httpClient}
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return &Server{session: session}, nil
}

func (s *Server) ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	return s.session.ListTools(ctx, params)
}

func (s *Server) CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	return s.session.CallTool(ctx, params)
}

func (s *Server) ListResources(ctx context.Context, params *mcp.ListResourcesParams) (*mcp.ListResourcesResult, error) {
	return s.session.ListResources(ctx, params)
}

func (s *Server) ReadResource(ctx context.Context, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	return s.session.ReadResource(ctx, params)
}

func (s *Server) Close() error {
	return s.session.Close()
}

// NewServerFromSession creates a Server from a pre-built session (useful for testing).
func NewServerFromSession(session *mcp.ClientSession) *Server {
	return &Server{session: session}
}

// toEnv converts the configured env map to a slice for exec.Cmd.
// Only the explicitly specified vars are passed to the child process.
// If no env vars are configured, returns nil (child inherits nothing).
func toEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
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
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}
