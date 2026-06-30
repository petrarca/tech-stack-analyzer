package scanner

import "testing"

// TestResolveSubsystemKey characterizes both resolution modes (group-path map
// and depth) and the file-path fallback, before refactoring.
func TestResolveSubsystemKey(t *testing.T) {
	t.Run("group mode: longest matching prefix wins", func(t *testing.T) {
		s := &Scanner{
			subsystemPathMap:  map[string]string{"/services": "platform", "/services/api": "api-team"},
			subsystemMaxDepth: 2,
		}
		if got := s.resolveSubsystemKey("/services/api/users", ""); got != "api-team" {
			t.Errorf("expected longest-prefix match api-team, got %q", got)
		}
		if got := s.resolveSubsystemKey("/services/web", ""); got != "platform" {
			t.Errorf("expected shallow match platform, got %q", got)
		}
	})

	t.Run("group mode: unmapped path returns empty", func(t *testing.T) {
		s := &Scanner{
			subsystemPathMap:  map[string]string{"/services": "platform"},
			subsystemMaxDepth: 1,
		}
		if got := s.resolveSubsystemKey("/other/thing", ""); got != "" {
			t.Errorf("expected empty for unmapped path, got %q", got)
		}
	})

	t.Run("group mode: file path fallback when no component path", func(t *testing.T) {
		s := &Scanner{
			subsystemPathMap:  map[string]string{"/services": "platform"},
			subsystemMaxDepth: 1,
			cachedBasePath:    "/repo",
		}
		if got := s.resolveSubsystemKey("", "/repo/services/api/main.go"); got != "platform" {
			t.Errorf("expected file-path fallback to resolve platform, got %q", got)
		}
	})

	t.Run("group mode: no path at all returns empty", func(t *testing.T) {
		s := &Scanner{
			subsystemPathMap:  map[string]string{"/services": "platform"},
			subsystemMaxDepth: 1,
		}
		if got := s.resolveSubsystemKey("", ""); got != "" {
			t.Errorf("expected empty when no path provided, got %q", got)
		}
	})

	t.Run("depth mode: extracts depth-N prefix from component path", func(t *testing.T) {
		s := &Scanner{subsystemDepth: 2}
		if got := s.resolveSubsystemKey("/a/b/c/d", ""); got != "/a/b" {
			t.Errorf("expected depth-2 prefix /a/b, got %q", got)
		}
	})

	t.Run("depth mode: empty component path returns empty", func(t *testing.T) {
		s := &Scanner{subsystemDepth: 2}
		if got := s.resolveSubsystemKey("", "/repo/x.go"); got != "" {
			t.Errorf("expected empty for depth mode without component path, got %q", got)
		}
	})
}
