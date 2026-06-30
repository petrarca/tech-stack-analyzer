package python

import "testing"

// TestExtractVersion characterizes extractVersion across all of its branches.
// Written before refactoring to lock in current behavior.
func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"gte operator", `">=1.2.3"`, "1.2.3"},
		{"lte operator", `"<=1.2.3"`, "1.2.3"},
		{"eq operator", `"==1.2.3"`, "1.2.3"},
		{"neq operator", `"!=1.2.3"`, "1.2.3"},
		{"compatible release", `"~=1.2"`, "1.2"},
		{"caret", `"^1.0.0"`, "1.0.0"},
		{"tilde", `"~1.0"`, "1.0"},
		{"gt operator", `">1"`, "1"},
		{"lt operator", `"<2"`, "2"},
		{"plain version", `"1.2.3"`, "1.2.3"},
		{"version field object", `{version = "2.0.0", optional = true}`, "2.0.0"},
		{"git url with ref", `"git+https://github.com/x/y@abc#egg=z"`, "git: abc@egg=z"},
		{"git object", `{git = "https://..."}`, "git"},
		{"local path", `{path = "../local"}`, "local"},
		// Bare operators with no version are malformed input and resolve to
		// "latest" (previously they leaked the operator or a stray "=").
		{"bare gt only", `">"`, "latest"},
		{"bare tilde only", `"~"`, "latest"},
		{"bare compatible only", `"~="`, "latest"},
		{"unknown object falls back to latest", `{markers = "x"}`, "latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractVersion(tt.in); got != tt.want {
				t.Errorf("extractVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
