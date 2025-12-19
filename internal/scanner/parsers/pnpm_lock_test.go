package parsers

import (
	"testing"
)

func TestParsePnpmLock(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		wantDeps map[string]string
	}{
		{
			name: "basic dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
      lodash:
        specifier: ^4.17.21
        version: 4.17.21
`,
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"lodash":  "4.17.21",
			},
		},
		{
			name: "with dev dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
    devDependencies:
      jest:
        specifier: ^29.0.0
        version: 29.7.0
`,
			expected: 2,
			wantDeps: map[string]string{
				"express": "4.18.2",
				"jest":    "29.7.0",
			},
		},
		{
			name: "only dev dependencies",
			content: `lockfileVersion: '9.0'

importers:
  .:
    devDependencies:
      typescript:
        specifier: ^5.0.0
        version: 5.3.2
`,
			expected: 1,
			wantDeps: map[string]string{
				"typescript": "5.3.2",
			},
		},
		{
			name: "no root importer",
			content: `lockfileVersion: '9.0'

importers:
  packages/sub:
    dependencies:
      express:
        specifier: ^4.18.0
        version: 4.18.2
`,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name:     "empty content",
			content:  ``,
			expected: 0,
			wantDeps: map[string]string{},
		},
		{
			name:     "invalid yaml",
			content:  `{invalid: yaml: content`,
			expected: 0,
			wantDeps: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := ParsePnpmLock([]byte(tt.content))

			if len(deps) != tt.expected {
				t.Errorf("ParsePnpmLock() got %d dependencies, want %d", len(deps), tt.expected)
			}

			for _, dep := range deps {
				if dep.Type != "npm" {
					t.Errorf("ParsePnpmLock() dep.Type = %s, want npm", dep.Type)
				}
				if dep.SourceFile != "pnpm-lock.yaml" {
					t.Errorf("ParsePnpmLock() dep.SourceFile = %s, want pnpm-lock.yaml", dep.SourceFile)
				}
				if expectedVersion, ok := tt.wantDeps[dep.Name]; ok {
					if dep.Version != expectedVersion {
						t.Errorf("ParsePnpmLock() dep %s version = %s, want %s", dep.Name, dep.Version, expectedVersion)
					}
				}
			}
		})
	}
}
