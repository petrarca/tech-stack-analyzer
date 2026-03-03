package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestParseDependencyList(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		wantErr  bool
	}{
		{
			name: "basic dependency list",
			content: `
The following files have been resolved:
   org.springframework.boot:spring-boot-starter-web:jar:4.0.1:compile -- module spring.boot.starter.web [auto]
   com.fasterxml.jackson.core:jackson-databind:jar:2.15.3:compile -- module com.fasterxml.jackson.databind
   junit:junit:jar:4.13.2:test -- module junit [auto]
`,
			expected: 3,
			wantErr:  false,
		},
		{
			name: "with ANSI color codes",
			content: `
The following files have been resolved:
   org.springframework.boot:spring-boot-starter-web:jar:4.0.1:compile[36m -- module spring.boot.starter.web[0;1m [auto][m
   junit:junit:jar:4.13.2:test[36m -- module junit[0;1m [auto][m
`,
			expected: 2,
			wantErr:  false,
		},
		{
			name: "different scopes",
			content: `
The following files have been resolved:
   org.example:compile-dep:jar:1.0.0:compile
   org.example:test-dep:jar:1.0.0:test
   org.example:provided-dep:jar:1.0.0:provided
   org.example:runtime-dep:jar:1.0.0:runtime
`,
			expected: 4,
			wantErr:  false,
		},
		{
			name: "non-jar types",
			content: `
The following files have been resolved:
   org.example:war-dep:war:1.0.0:compile
   org.example:pom-dep:pom:1.0.0:import
`,
			expected: 2,
			wantErr:  false,
		},
		{
			name:     "empty content",
			content:  "",
			expected: 0,
			wantErr:  false,
		},
		{
			name: "only header",
			content: `
The following files have been resolved:
`,
			expected: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewMavenDependencyListParser()
			deps := parser.ParseDependencyList(tt.content, true)

			if tt.wantErr {
				if deps != nil {
					t.Errorf("Expected error, got %d dependencies", len(deps))
				}
				return
			}

			if len(deps) != tt.expected {
				t.Errorf("Expected %d dependencies, got %d", tt.expected, len(deps))
			}
		})
	}
}

func TestMavenDependencyListScopes(t *testing.T) {
	tests := []struct {
		mavenScope string
		expected   string
	}{
		{"compile", types.ScopeProd},
		{"test", types.ScopeDev},
		{"provided", types.ScopeProd},
		{"runtime", types.ScopeProd},
		{"system", types.ScopeSystem},
		{"import", types.ScopeImport},
	}

	for _, tt := range tests {
		t.Run(tt.mavenScope, func(t *testing.T) {
			content := `
The following files have been resolved:
   org.example:some-lib:jar:1.0.0:` + tt.mavenScope + `
`

			parser := NewMavenDependencyListParser()
			deps := parser.ParseDependencyList(content, false)

			if len(deps) != 1 {
				t.Fatalf("Expected 1 dependency, got %d", len(deps))
			}

			if deps[0].Scope != tt.expected {
				t.Errorf("Expected scope %s, got %s", tt.expected, deps[0].Scope)
			}
		})
	}
}

func TestMavenDependencyListMetadata(t *testing.T) {
	content := `
The following files have been resolved:
   org.example:war-artifact:war:1.0.0:compile
   org.example:jar-artifact:jar:1.0.0:compile
`

	parser := NewMavenDependencyListParser()
	deps := parser.ParseDependencyList(content, false)

	if len(deps) != 2 {
		t.Fatalf("Expected 2 dependencies, got %d", len(deps))
	}

	// First dependency (war) should have type metadata
	if deps[0].Metadata == nil {
		t.Fatal("Expected metadata for war artifact, got nil")
	}
	if deps[0].Metadata["type"] != "war" {
		t.Errorf("Expected type=war, got %v", deps[0].Metadata["type"])
	}
	if deps[0].Metadata["source"] != "dependency-list" {
		t.Errorf("Expected source=dependency-list, got %v", deps[0].Metadata["source"])
	}

	// Second dependency (jar) should not have type metadata (jar is default)
	if deps[1].Metadata == nil {
		t.Fatal("Expected metadata with source, got nil")
	}
	if _, exists := deps[1].Metadata["type"]; exists {
		t.Error("Should not have type metadata for default jar")
	}
	if deps[1].Metadata["source"] != "dependency-list" {
		t.Errorf("Expected source=dependency-list, got %v", deps[1].Metadata["source"])
	}
}

func TestMavenDependencyListRealWorld(t *testing.T) {
	// Real output from mvn dependency:list
	content := `
The following files have been resolved:
   org.springframework.boot:spring-boot-starter-web:jar:4.0.1:compile[36m -- module spring.boot.starter.web[0;1m [auto][m
   org.springframework.boot:spring-boot-starter:jar:4.0.1:compile[36m -- module spring.boot.starter[0;1m [auto][m
   ch.qos.logback:logback-classic:jar:1.5.22:compile[36m -- module ch.qos.logback.classic[m
   org.slf4j:slf4j-api:jar:2.0.17:compile[36m -- module org.slf4j[m
   junit:junit:jar:4.13.2:test[36m -- module junit[0;1m [auto][m
`

	parser := NewMavenDependencyListParser()
	deps := parser.ParseDependencyList(content, true)

	if len(deps) != 5 {
		t.Fatalf("Expected 5 dependencies, got %d", len(deps))
	}

	// Verify first dependency
	if deps[0].Name != "org.springframework.boot:spring-boot-starter-web" {
		t.Errorf("Expected spring-boot-starter-web, got %s", deps[0].Name)
	}
	if deps[0].Version != "4.0.1" {
		t.Errorf("Expected version 4.0.1, got %s", deps[0].Version)
	}
	if deps[0].Scope != types.ScopeProd {
		t.Errorf("Expected scope prod, got %s", deps[0].Scope)
	}

	// Verify test dependency
	if deps[4].Name != "junit:junit" {
		t.Errorf("Expected junit, got %s", deps[4].Name)
	}
	if deps[4].Scope != types.ScopeDev {
		t.Errorf("Expected scope dev for test dependency, got %s", deps[4].Scope)
	}
}
