package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"skillful-mcp/internal/mcpserver"
	"skillful-mcp/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewServer(mgr *mcpserver.Manager) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "skillful-mcp",
		Version: "0.1.0",
	}, nil)

	tools.RegisterListSkills(s, mgr)
	tools.RegisterUseSkill(s, mgr)
	tools.RegisterReadResource(s, mgr)
	tools.RegisterExecuteCode(s, mgr)

	return s
}

func Serve(ctx context.Context, s *mcp.Server, transport, host, port string) error {
	switch transport {
	case "stdio":
		return s.Run(ctx, &mcp.StdioTransport{})
	case "http":
		handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return s
		}, nil)
		addr := net.JoinHostPort(host, port)
		srv := &http.Server{Addr: addr, Handler: handler}
		go func() {
			<-ctx.Done()
			srv.Close()
		}()
		fmt.Fprintf(os.Stderr, "Listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown transport: %q (use 'stdio' or 'http')", transport)
	}
}
