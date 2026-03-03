package parsers

import (
	"encoding/json"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestScopeJSONMarshaling(t *testing.T) {
	// Maven dep with scope, direct, no metadata -> 6 elements
	depMaven := types.Dependency{
		Type:    "maven",
		Name:    "junit:junit",
		Version: "4.13.2",
		Scope:   types.ScopeDev,
		Direct:  true,
	}

	// npm dep with scope, direct, and metadata -> 6 elements
	depWithMetadata := types.Dependency{
		Type:    "npm",
		Name:    "lodash",
		Version: "4.17.21",
		Scope:   types.ScopeProd,
		Direct:  true,
		Metadata: map[string]interface{}{
			"optional": true,
		},
	}

	// Go dep with no scope, direct -> 6 elements
	depGo := types.Dependency{
		Type:    "golang",
		Name:    "github.com/user/module",
		Version: "v1.2.3",
		Direct:  true,
	}

	// Python dep with source file -> 6 elements
	depPython := types.Dependency{
		Type:       "python",
		Name:       "requests",
		Version:    "2.31.0",
		SourceFile: "requirements.txt",
		Direct:     true,
	}

	// Test Maven (6 elements with empty metadata)
	jsonMaven, _ := json.Marshal(depMaven)
	var arrMaven []interface{}
	json.Unmarshal(jsonMaven, &arrMaven)
	if len(arrMaven) != 6 {
		t.Errorf("Expected 6 elements for Maven dep, got %d: %v", len(arrMaven), arrMaven)
	}
	if arrMaven[3] != types.ScopeDev {
		t.Errorf("Expected scope 'dev' at index 3, got '%v'", arrMaven[3])
	}
	if arrMaven[4] != true {
		t.Errorf("Expected direct=true at index 4, got '%v'", arrMaven[4])
	}

	// Test NPM with metadata (6 elements)
	jsonNPM, _ := json.Marshal(depWithMetadata)
	var arrNPM []interface{}
	json.Unmarshal(jsonNPM, &arrNPM)
	if len(arrNPM) != 6 {
		t.Errorf("Expected 6 elements for NPM dep, got %d: %v", len(arrNPM), arrNPM)
	}
	if arrNPM[3] != types.ScopeProd {
		t.Errorf("Expected scope 'prod' at index 3, got '%v'", arrNPM[3])
	}
	if arrNPM[4] != true {
		t.Errorf("Expected direct=true at index 4, got '%v'", arrNPM[4])
	}
	if metadata, ok := arrNPM[5].(map[string]interface{}); !ok {
		t.Errorf("Expected metadata object at index 5, got %T", arrNPM[5])
	} else if metadata["optional"] != true {
		t.Errorf("Expected optional=true in metadata, got %v", metadata)
	}

	// Test Go (6 elements with empty metadata)
	jsonGo, _ := json.Marshal(depGo)
	var arrGo []interface{}
	json.Unmarshal(jsonGo, &arrGo)
	if len(arrGo) != 6 {
		t.Errorf("Expected 6 elements for Go dep, got %d: %v", len(arrGo), arrGo)
	}

	// Test Python with source file (6 elements with source in metadata)
	jsonPython, _ := json.Marshal(depPython)
	var arrPython []interface{}
	json.Unmarshal(jsonPython, &arrPython)
	if len(arrPython) != 6 {
		t.Errorf("Expected 6 elements for Python dep, got %d: %v", len(arrPython), arrPython)
	}
	if metadata, ok := arrPython[5].(map[string]interface{}); !ok {
		t.Errorf("Expected metadata object at index 5, got %T", arrPython[5])
	} else if metadata["source"] != "requirements.txt" {
		t.Errorf("Expected source='requirements.txt' in metadata, got %v", metadata)
	}
}

func TestEmptyVersionHandling(t *testing.T) {
	// Verify empty version doesn't cause issues with 6-element format
	tests := []struct {
		name     string
		dep      types.Dependency
		wantIdx2 string // version at index 2
		wantIdx3 string // scope at index 3
	}{
		{
			name:     "empty version with scope and source",
			dep:      types.Dependency{Type: "npm", Name: "pkg", Version: "", Scope: "prod", SourceFile: "package.json", Direct: true},
			wantIdx2: "",
			wantIdx3: "prod",
		},
		{
			name:     "empty version with scope only",
			dep:      types.Dependency{Type: "maven", Name: "junit:junit", Version: "", Scope: "dev", Direct: true},
			wantIdx2: "",
			wantIdx3: "dev",
		},
		{
			name:     "empty version no scope",
			dep:      types.Dependency{Type: "delphi", Name: "Vcl", Version: "", Direct: false},
			wantIdx2: "",
			wantIdx3: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.dep)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var arr []interface{}
			if err := json.Unmarshal(jsonBytes, &arr); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// All dependencies should now be 6 elements
			if len(arr) != 6 {
				t.Errorf("Expected 6 elements, got %d: %v", len(arr), arr)
			}

			if arr[2] != tt.wantIdx2 {
				t.Errorf("Expected version '%s' at index 2, got '%v'", tt.wantIdx2, arr[2])
			}

			if arr[3] != tt.wantIdx3 {
				t.Errorf("Expected scope '%s' at index 3, got '%v'", tt.wantIdx3, arr[3])
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
