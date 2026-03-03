package parsers

import (
	"testing"
)

func TestParseYarnLock(t *testing.T) {
	tests := []struct {
		name        string
		lockContent string
		packageJSON *PackageJSON
		expected    int
		wantDeps    map[string]string
	}{
		{
			name: "basic dependencies",
			lockContent: `# yarn lockfile v1

"express@npm:^4.18.0":
  version: 4.18.2
  resolution: "express@npm:4.18.2"

"lodash@npm:^4.17.21":
  version: 4.17.21
  resolution: "lodash@npm:4.17.21"
`,
			packageJSON: &PackageJSON{
				Name: "test-project",
				Dependencies: map[string]string{
					"express": "^4.18.0",
					"lodash":  "^4.17.21",
				},
			},
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"lodash":  "4.17.21",
			},
		},
		{
			name: "filters transitive dependencies",
			lockContent: `# yarn lockfile v1

"express@npm:^4.18.0":
  version: 4.18.2
  resolution: "express@npm:4.18.2"

"accepts@npm:~1.3.8":
  version: 1.3.8
  resolution: "accepts@npm:1.3.8"

"body-parser@npm:~1.20.3":
  version: 1.20.4
  resolution: "body-parser@npm:1.20.4"
`,
			packageJSON: &PackageJSON{
				Name: "test-project",
				Dependencies: map[string]string{
					"express": "^4.18.0",
				},
			},
			expected: 1,
			wantDeps: map[string]string{
				"express": "4.18.2",
			},
		},
		{
			name: "scoped packages",
			lockContent: `# yarn lockfile v1

"@babel/core@npm:^7.23.0":
  version: 7.23.5
  resolution: "@babel/core@npm:7.23.5"

"@types/node@npm:^20.10.0":
  version: 20.10.4
  resolution: "@types/node@npm:20.10.4"
`,
			packageJSON: &PackageJSON{
				Name: "test-project",
				Dependencies: map[string]string{
					"@babel/core": "^7.23.0",
				},
				DevDependencies: map[string]string{
					"@types/node": "^20.10.0",
				},
			},
			expected: 2,
			wantDeps: map[string]string{
				"@babel/core": "7.23.5",
				"@types/node": "20.10.4",
			},
		},
		{
			name: "dev dependencies",
			lockContent: `# yarn lockfile v1

"jest@npm:^29.0.0":
  version: 29.7.0
  resolution: "jest@npm:29.7.0"
`,
			packageJSON: &PackageJSON{
				Name: "test-project",
				DevDependencies: map[string]string{
					"jest": "^29.0.0",
				},
			},
			expected: 1,
			wantDeps: map[string]string{
				"jest": "29.7.0",
			},
		},
		{
			name:        "nil package.json",
			lockContent: `"express@npm:^4.18.0": version: 4.18.2`,
			packageJSON: nil,
			expected:    0,
			wantDeps:    map[string]string{},
		},
		{
			name:        "empty lock content",
			lockContent: ``,
			packageJSON: &PackageJSON{
				Name:         "test-project",
				Dependencies: map[string]string{"express": "^4.18.0"},
			},
			expected: 0,
			wantDeps: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParseYarnLock([]byte(tt.lockContent), tt.packageJSON)

			if len(deps) != tt.expected {
				t.Errorf("ParseYarnLock() got %d dependencies, want %d", len(deps), tt.expected)
			}

			for _, dep := range deps {
				if dep.Type != "npm" {
					t.Errorf("ParseYarnLock() dep.Type = %s, want npm", dep.Type)
				}
				if dep.SourceFile != "yarn.lock" {
					t.Errorf("ParseYarnLock() dep.SourceFile = %s, want yarn.lock", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParseYarnLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}
