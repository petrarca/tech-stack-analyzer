package parsers

import "testing"

// TestIsProfileActive characterizes Maven profile activation across JDK, OS,
// property and file conditions, using the package default JDK (11.0.8) and OS
// (linux/unix/amd64). Written before refactoring.
func TestIsProfileActive(t *testing.T) {
	p := &MavenParser{}

	tests := []struct {
		name string
		act  MavenActivation
		want bool
	}{
		{"empty activation is inactive", MavenActivation{}, false},
		{"jdk prefix match active", MavenActivation{JDK: "11"}, true},
		{"jdk exact match active", MavenActivation{JDK: DefaultJDKVersion}, true},
		{"jdk mismatch inactive", MavenActivation{JDK: "17"}, false},
		{"os name match active", MavenActivation{OS: MavenActivationOS{Name: "linux"}}, true},
		{"os name mismatch inactive", MavenActivation{OS: MavenActivationOS{Name: "windows"}}, false},
		{"os family match active", MavenActivation{OS: MavenActivationOS{Family: "unix"}}, true},
		{"os arch match active", MavenActivation{OS: MavenActivationOS{Arch: "amd64"}}, true},
		{"os negated name active", MavenActivation{OS: MavenActivationOS{Name: "!windows"}}, true},
		{"os all conditions match active", MavenActivation{OS: MavenActivationOS{Name: "linux", Family: "unix", Arch: "amd64", Version: "5.10.0"}}, true},
		{"os one condition mismatch inactive", MavenActivation{OS: MavenActivationOS{Name: "linux", Arch: "arm64"}}, false},
		{"jdk and os both match active", MavenActivation{JDK: "11", OS: MavenActivationOS{Name: "linux"}}, true},
		{"jdk match but os mismatch inactive", MavenActivation{JDK: "11", OS: MavenActivationOS{Name: "windows"}}, false},
		{"property activation conservatively inactive", MavenActivation{Property: MavenActivationProperty{Name: "env"}}, false},
		{"file exists activation conservatively inactive", MavenActivation{File: MavenActivationFile{Exists: "x.txt"}}, false},
		{"file missing activation conservatively inactive", MavenActivation{File: MavenActivationFile{Missing: "x.txt"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isProfileActive(tt.act); got != tt.want {
				t.Errorf("isProfileActive(%+v) = %v, want %v", tt.act, got, tt.want)
			}
		})
	}
}
