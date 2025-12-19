package parsers

import (
	"testing"
)

func TestParseCargoLock(t *testing.T) {
	tests := []struct {
		name             string
		lockContent      string
		cargoTomlContent string
		expected         int
		wantDeps         map[string]string
	}{
		{
			name: "basic Cargo.lock with direct dependencies",
			lockContent: `version = 3

[[package]]
name = "serde"
version = "1.0.193"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "serde_json"
version = "1.0.108"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "itoa"
version = "1.0.10"
source = "registry+https://github.com/rust-lang/crates.io-index"
`,
			cargoTomlContent: `[package]
name = "my-project"
version = "0.1.0"

[dependencies]
serde = "1.0"
serde_json = "1.0"
`,
			expected: 2,
			wantDeps: map[string]string{
				"serde":      "1.0.193",
				"serde_json": "1.0.108",
			},
		},
		{
			name: "Cargo.lock filters transitive dependencies",
			lockContent: `version = 3

[[package]]
name = "tokio"
version = "1.35.0"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "mio"
version = "0.8.10"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "socket2"
version = "0.5.5"
source = "registry+https://github.com/rust-lang/crates.io-index"
`,
			cargoTomlContent: `[dependencies]
tokio = { version = "1.35", features = ["full"] }
`,
			expected: 1,
			wantDeps: map[string]string{
				"tokio": "1.35.0",
			},
		},
		{
			name: "Cargo.lock with dev-dependencies",
			lockContent: `version = 3

[[package]]
name = "serde"
version = "1.0.193"

[[package]]
name = "criterion"
version = "0.5.1"
`,
			cargoTomlContent: `[dependencies]
serde = "1.0"

[dev-dependencies]
criterion = "0.5"
`,
			expected: 2,
			wantDeps: map[string]string{
				"serde":     "1.0.193",
				"criterion": "0.5.1",
			},
		},
		{
			name:        "empty lock content",
			lockContent: ``,
			cargoTomlContent: `[dependencies]
serde = "1.0"
`,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name: "empty Cargo.toml content",
			lockContent: `[[package]]
name = "serde"
version = "1.0.193"
`,
			cargoTomlContent: ``,
			expected:         0,
			wantDeps:         map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParseCargoLock([]byte(tt.lockContent), tt.cargoTomlContent)

			if len(deps) != tt.expected {
				t.Errorf("ParseCargoLock() got %d dependencies, want %d", len(deps), tt.expected)
				for _, d := range deps {
					t.Logf("  got: %s = %s", d.Name, d.Version)
				}
			}

			for _, dep := range deps {
				if dep.Type != "cargo" {
					t.Errorf("ParseCargoLock() dep.Type = %s, want cargo", dep.Type)
				}
				if dep.SourceFile != "Cargo.lock" {
					t.Errorf("ParseCargoLock() dep.SourceFile = %s, want Cargo.lock", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParseCargoLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}
