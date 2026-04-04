package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"skillful-mcp/internal/config"
)

type Manager struct {
	servers map[string]*Server
}

// NewManager creates a Manager by connecting to all servers in the config.
func NewManager(ctx context.Context, cfg *config.Config) (*Manager, error) {
	m := &Manager{servers: make(map[string]*Server)}

	for name, srv := range cfg.MCPServers {
		s, err := NewServer(ctx, &srv)
		if err != nil {
			// Close any servers we already opened before returning.
			m.Close()
			return nil, fmt.Errorf("connecting to %q: %w", name, err)
		}
		m.servers[name] = s
		tt, _ := srv.TransportType() // already validated
		slog.Info("connected to server", "skill", name, "transport", tt)
	}

	return m, nil
}

// NewManagerFromServers creates a Manager from pre-built Servers (useful for testing).
func NewManagerFromServers(servers map[string]*Server) *Manager {
	return &Manager{servers: servers}
}

func (m *Manager) GetServer(name string) (*Server, error) {
	s, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("unknown skill: %q", name)
	}
	return s, nil
}

func (m *Manager) ListServerNames() []string {
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *Manager) Close() {
	for name, s := range m.servers {
		if err := s.Close(); err != nil {
			slog.Warn("error closing server", "skill", name, "error", err)
		}
	}
}
