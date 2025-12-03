package parsers

import (
	"reflect"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func TestCocoaPodsParser_ParsePodfile(t *testing.T) {
	parser := NewCocoaPodsParser()

	tests := []struct {
		name     string
		content  string
		expected []types.Dependency
	}{
		{
			name: "parses pods with versions",
			content: `target 'TestApp' do
  pod 'AFNetworking', '4.0.1'
  pod 'Alamofire', '5.6.0'
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
			},
		},
		{
			name: "parses pods without versions",
			content: `target 'TestApp' do
  pod 'AFNetworking'
  pod 'Alamofire'
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "latest"},
				{Type: "cocoapods", Name: "Alamofire", Example: "latest"},
			},
		},
		{
			name: "handles mixed quotes and spaces",
			content: `target 'TestApp' do
  pod "AFNetworking", '4.0.1'
  pod 'Alamofire', "5.6.0"
  pod "SnapKit"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
				{Type: "cocoapods", Name: "SnapKit", Example: "latest"},
			},
		},
		{
			name: "ignores comments and empty lines",
			content: `# This is a comment
target 'TestApp' do
  # Another comment
  pod 'AFNetworking', '4.0.1'
  
  pod 'Alamofire', '5.6.0'
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []types.Dependency{},
		},
		{
			name: "only comments",
			content: `# This is a comment
# Another comment`,
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParsePodfile(tt.content)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParsePodfile() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCocoaPodsParser_ParsePodfileLock(t *testing.T) {
	parser := NewCocoaPodsParser()

	tests := []struct {
		name     string
		content  string
		expected []types.Dependency
	}{
		{
			name: "parses simple PODS section",
			content: `PODS:
  - AFNetworking (4.0.1)
  - Alamofire (5.6.0)

DEPENDENCIES:
  - AFNetworking (~> 4.0.1)
  - Alamofire (~> 5.6.0)`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
			},
		},
		{
			name: "handles pods with dependencies",
			content: `PODS:
  - MySDK (1.0.0):
    - AFNetworking (= 4.0.1)
    - Alamofire (~> 5.6.0)
  - AFNetworking (4.0.1)
  - Alamofire (5.6.0)

DEPENDENCIES:
  - MySDK (= 1.0.0)`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "MySDK", Example: "1.0.0"},
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
			},
		},
		{
			name: "ignores sections after PODS",
			content: `PODS:
  - AFNetworking (4.0.1)
  - Alamofire (5.6.0)

DEPENDENCIES:
  - AFNetworking (~> 4.0.1)
  - Alamofire (~> 5.6.0)

SPEC REPOS:
  trunk:
    - AFNetworking

CHECKSUMS:
  AFNetworking: somechecksum`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Example: "5.6.0"},
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []types.Dependency{},
		},
		{
			name: "no PODS section",
			content: `DEPENDENCIES:
  - AFNetworking (~> 4.0.1)

SPEC REPOS:
  trunk:
    - AFNetworking`,
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParsePodfileLock(tt.content)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParsePodfileLock() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCocoaPodsParser_ExtractDependencies(t *testing.T) {
	parser := NewCocoaPodsParser()

	podfileContent := `target 'TestApp' do
  pod 'AFNetworking', '4.0.1'
end`

	podfileLockContent := `PODS:
  - AFNetworking (4.0.1)
DEPENDENCIES:
  - AFNetworking (~> 4.0.1)`

	tests := []struct {
		name     string
		content  string
		filename string
		expected []types.Dependency
	}{
		{
			name:     "extracts from Podfile",
			content:  podfileContent,
			filename: "Podfile",
			expected: []types.Dependency{{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"}},
		},
		{
			name:     "extracts from Podfile.lock",
			content:  podfileLockContent,
			filename: "Podfile.lock",
			expected: []types.Dependency{{Type: "cocoapods", Name: "AFNetworking", Example: "4.0.1"}},
		},
		{
			name:     "handles unknown filename",
			content:  podfileContent,
			filename: "unknown.txt",
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ExtractDependencies(tt.content, tt.filename)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractDependencies() = %v, want %v", got, tt.expected)
			}
		})
	}
}
