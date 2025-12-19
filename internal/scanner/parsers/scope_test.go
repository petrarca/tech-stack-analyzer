package parsers

import (
	"encoding/json"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestScopeJSONMarshaling(t *testing.T) {
	// Maven dep with scope but no source -> 4 elements
	depWithScopeNoSource := types.Dependency{
		Type:    "maven",
		Name:    "junit:junit",
		Version: "4.13.2",
		Scope:   types.ScopeDev,
	}

	// npm dep with scope and source -> 5 elements
	depWithScopeAndSource := types.Dependency{
		Type:       "npm",
		Name:       "lodash",
		Version:    "4.17.21",
		Scope:      types.ScopeProd,
		SourceFile: "package-lock.json",
	}

	// Dep with no scope, no source -> 3 elements
	depNoScopeNoSource := types.Dependency{
		Type:    "golang",
		Name:    "github.com/user/module",
		Version: "v1.2.3",
	}

	// Dep with source but no scope -> 5 elements (scope is empty string)
	depSourceNoScope := types.Dependency{
		Type:       "python",
		Name:       "requests",
		Version:    "2.31.0",
		SourceFile: "requirements.txt",
	}

	// Test 4-element format (scope, no source)
	json4, _ := json.Marshal(depWithScopeNoSource)
	var arr4 []string
	json.Unmarshal(json4, &arr4)
	if len(arr4) != 4 {
		t.Errorf("Expected 4 elements for scope-only dep, got %d: %v", len(arr4), arr4)
	}
	if arr4[3] != types.ScopeDev {
		t.Errorf("Expected scope at index 3, got '%s'", arr4[3])
	}

	// Test 5-element format (scope and source)
	json5, _ := json.Marshal(depWithScopeAndSource)
	var arr5 []string
	json.Unmarshal(json5, &arr5)
	if len(arr5) != 5 {
		t.Errorf("Expected 5 elements for scope+source dep, got %d: %v", len(arr5), arr5)
	}
	if arr5[3] != types.ScopeProd {
		t.Errorf("Expected scope at index 3, got '%s'", arr5[3])
	}
	if arr5[4] != "package-lock.json" {
		t.Errorf("Expected source at index 4, got '%s'", arr5[4])
	}

	// Test 3-element format (no scope, no source)
	json3, _ := json.Marshal(depNoScopeNoSource)
	var arr3 []string
	json.Unmarshal(json3, &arr3)
	if len(arr3) != 3 {
		t.Errorf("Expected 3 elements for no-scope-no-source dep, got %d: %v", len(arr3), arr3)
	}

	// Test 5-element format with empty scope (source but no scope)
	json5empty, _ := json.Marshal(depSourceNoScope)
	var arr5empty []string
	json.Unmarshal(json5empty, &arr5empty)
	if len(arr5empty) != 5 {
		t.Errorf("Expected 5 elements for source-only dep, got %d: %v", len(arr5empty), arr5empty)
	}
	if arr5empty[3] != "" {
		t.Errorf("Expected empty scope at index 3, got '%s'", arr5empty[3])
	}
	if arr5empty[4] != "requirements.txt" {
		t.Errorf("Expected source at index 4, got '%s'", arr5empty[4])
	}
}

func TestEmptyVersionHandling(t *testing.T) {
	// Verify empty version doesn't cause issues with scope/source
	tests := []struct {
		name     string
		dep      types.Dependency
		wantLen  int
		wantIdx2 string // version at index 2
		wantIdx3 string // scope at index 3 (if present)
	}{
		{
			name:     "empty version with scope and source",
			dep:      types.Dependency{Type: "npm", Name: "pkg", Version: "", Scope: "prod", SourceFile: "package.json"},
			wantLen:  5,
			wantIdx2: "",
			wantIdx3: "prod",
		},
		{
			name:     "empty version with scope only",
			dep:      types.Dependency{Type: "maven", Name: "junit:junit", Version: "", Scope: "dev"},
			wantLen:  4,
			wantIdx2: "",
			wantIdx3: "dev",
		},
		{
			name:     "empty version no scope no source",
			dep:      types.Dependency{Type: "delphi", Name: "Vcl", Version: ""},
			wantLen:  3,
			wantIdx2: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.dep)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var arr []string
			if err := json.Unmarshal(jsonBytes, &arr); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if len(arr) != tt.wantLen {
				t.Errorf("Expected %d elements, got %d: %v", tt.wantLen, len(arr), arr)
			}

			if arr[2] != tt.wantIdx2 {
				t.Errorf("Expected version '%s' at index 2, got '%s'", tt.wantIdx2, arr[2])
			}

			if tt.wantLen >= 4 && arr[3] != tt.wantIdx3 {
				t.Errorf("Expected scope '%s' at index 3, got '%s'", tt.wantIdx3, arr[3])
			}
		})
	}
}

func TestMavenScopeDetection(t *testing.T) {
	parser := NewMavenParser()
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-core</artifactId>
            <version>5.3.23</version>
        </dependency>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.13.2</version>
            <scope>test</scope>
        </dependency>
    </dependencies>
</project>`

	deps := parser.ParsePomXML(pomContent)

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	for _, dep := range deps {
		switch dep.Name {
		case "junit:junit":
			if dep.Scope != types.ScopeDev {
				t.Errorf("Expected junit scope '%s', got '%s'", types.ScopeDev, dep.Scope)
			}
		case "org.springframework:spring-core":
			if dep.Scope != types.ScopeProd {
				t.Errorf("Expected spring-core scope '%s', got '%s'", types.ScopeProd, dep.Scope)
			}
		}
	}
}

func TestGradleScopeDetection(t *testing.T) {
	parser := NewGradleParser()
	gradleContent := `dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web:2.7.5'
    testImplementation 'junit:junit:4.13.2'
    compileOnly 'org.projectlombok:lombok:1.18.24'
}`

	deps := parser.ParseGradle(gradleContent)

	if len(deps) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(deps))
	}

	for _, dep := range deps {
		switch dep.Name {
		case "org.springframework.boot:spring-boot-starter-web":
			if dep.Scope != types.ScopeProd {
				t.Errorf("Expected spring-boot-starter-web scope 'prod', got '%s'", dep.Scope)
			}
		case "junit:junit":
			if dep.Scope != types.ScopeDev {
				t.Errorf("Expected junit scope 'dev', got '%s'", dep.Scope)
			}
		case "org.projectlombok:lombok":
			if dep.Scope != types.ScopeBuild {
				t.Errorf("Expected lombok scope 'build', got '%s'", dep.Scope)
			}
		}
	}
}

func TestConanScopeDetection(t *testing.T) {
	parser := NewConanParser()
	conanContent := `from conan import ConanFile

class MyProject(ConanFile):
    requires = [
        "boost/1.75.0",
        "openssl/1.1.1k"
    ]
    
    tool_requires = [
        "cmake/3.21.0",
        "ninja/1.10.2"
    ]
`

	deps := parser.ExtractDependencies(conanContent)

	if len(deps) != 4 {
		t.Errorf("Expected 4 dependencies, got %d", len(deps))
	}

	for _, dep := range deps {
		switch dep.Name {
		case "boost", "openssl":
			if dep.Scope != types.ScopeProd {
				t.Errorf("Expected %s scope 'prod', got '%s'", dep.Name, dep.Scope)
			}
		case "cmake", "ninja":
			if dep.Scope != types.ScopeDev {
				t.Errorf("Expected %s scope 'dev', got '%s'", dep.Name, dep.Scope)
			}
		}
	}
}
