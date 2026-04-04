package mcpserver

import (
	"testing"
)

func TestGetServer(t *testing.T) {
	t.Parallel()

	m, err := NewManagerFromServers(map[string]*Server{
		"alpha": {},
		"bravo": {},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("existing server", func(t *testing.T) {
		t.Parallel()
		s, err := m.GetServer("alpha")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil server")
		}
	})

	t.Run("unknown server", func(t *testing.T) {
		t.Parallel()
		_, err := m.GetServer("nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown server")
		}
	})
}

func TestListServerNames(t *testing.T) {
	t.Parallel()

	t.Run("returns all names", func(t *testing.T) {
		t.Parallel()
		m, err := NewManagerFromServers(map[string]*Server{
			"charlie": {},
			"alpha":   {},
			"bravo":   {},
		})
		if err != nil {
			t.Fatal(err)
		}

		names := m.ListServerNames()
		if len(names) != 3 {
			t.Fatalf("got %d names, want 3", len(names))
		}
		nameSet := map[string]bool{}
		for _, n := range names {
			nameSet[n] = true
		}
		for _, expected := range []string{"alpha", "bravo", "charlie"} {
			if !nameSet[expected] {
				t.Errorf("missing expected name %q", expected)
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		m, err := NewManagerFromServers(map[string]*Server{})
		if err != nil {
			t.Fatal(err)
		}
		names := m.ListServerNames()
		if len(names) != 0 {
			t.Errorf("expected empty, got %v", names)
		}
	})
}

func TestAllTools(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_a"))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_b"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}

	tools := m.AllTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestManagerServerTools(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_a"))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, "tool_b"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}

	tools := m.ServerTools("alpha")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool for alpha, got %d", len(tools))
	}
	if tools[0].ResolvedName != "tool_a" {
		t.Errorf("expected tool_a, got %q", tools[0].ResolvedName)
	}

	tools = m.ServerTools("nonexistent")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for nonexistent, got %d", len(tools))
	}
}

func TestManagerClose(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	session := startFakeServer(t, ctx, "tool")
	srv, err := NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic on multiple closes.
	m.Close()
	m.Close()
}
