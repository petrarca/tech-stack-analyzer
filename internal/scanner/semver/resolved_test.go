package semver

import "testing"

func TestResolvedVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"^1.2.3", ""},
		{"~1.2", ""},
		{">=1.0", ""},
		{"latest", ""},
		{"git", ""},
		{"", ""},
		{"git:https://example.com/repo.git", ""},
		{"path:../local", ""},
		{"  2.0.0  ", "2.0.0"},
		// Real-world unresolved tokens observed across scans.
		{"${project.parent.version}", ""},
		{"${ktor_version}", ""},
		{"$ktor_version", ""},
		{"4.10.*", ""},
		{"8.*", ""},
		{"RELEASE", ""},
		{"LATEST", ""},
		{"workspace", ""},
		// Maven range expressions are not concrete.
		{"[1.0,2.0)", ""},
		{"(,1.0]", ""},
		{"31.1-jre", "31.1-jre"},
		{"7.14.0", "7.14.0"},
		{"2.2.8", "2.2.8"},
	}
	for _, tt := range tests {
		if got := ResolvedVersion(tt.in); got != tt.want {
			t.Errorf("ResolvedVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsResolved(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1.2.3", true},
		{"v1.2.3", true},
		{"31.1-jre", true},
		{"", false},
		{"latest", false},
		{"^1.2.3", false},
		{"${x}", false},
		{"[1.0,2.0)", false},
	}
	for _, tt := range tests {
		if got := IsResolved(tt.in); got != tt.want {
			t.Errorf("IsResolved(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
