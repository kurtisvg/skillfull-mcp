package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"skillful-mcp/internal/clientmanager"
	"skillful-mcp/internal/config"
	"skillful-mcp/internal/server"
)

var (
	configPath string
	transport  string
	host       string
	port       string
)

func init() {
	flag.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	flag.StringVar(&transport, "transport", "stdio", "Upstream transport: stdio or http")
	flag.StringVar(&host, "host", "localhost", "HTTP host (when transport=http)")
	flag.StringVar(&port, "port", "8080", "HTTP port (when transport=http)")
}

func Execute() {
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d server(s):\n", len(cfg.MCPServers))
	for name, srv := range cfg.MCPServers {
		tt, _ := srv.TransportType() // already validated
		switch tt {
		case config.TransportSTDIO:
			fmt.Fprintf(os.Stderr, "  [%s] %s → %s %v\n", name, tt, srv.Command, srv.Args)
		case config.TransportHTTP, config.TransportSSE:
			fmt.Fprintf(os.Stderr, "  [%s] %s → %s\n", name, tt, srv.URL)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mgr, err := clientmanager.ConnectAll(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to servers: %v", err)
	}
	defer mgr.Close()

	fmt.Fprintf(os.Stderr, "Connected to %d skill(s): %v\n", len(mgr.ListServerNames()), mgr.ListServerNames())

	s := server.NewServer(mgr)
	if err := server.Serve(ctx, s, transport, host, port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
