package tools

import (
	"testing"

	"skillful-mcp/internal/mcpserver"
)

func TestListServerNamesSorted(t *testing.T) {
	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"charlie": mcpserver.NewServerFromSession(nil),
		"alpha":   mcpserver.NewServerFromSession(nil),
		"bravo":   mcpserver.NewServerFromSession(nil),
	})

	names := mgr.ListServerNames()
	expected := []string{"alpha", "bravo", "charlie"}
	if len(names) != len(expected) {
		t.Fatalf("got %d names, want %d", len(names), len(expected))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestListServerNamesEmpty(t *testing.T) {
	mgr := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	names := mgr.ListServerNames()
	if len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}
