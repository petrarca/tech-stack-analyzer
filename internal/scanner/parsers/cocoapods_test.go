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
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
			},
		},
		{
			name: "parses pods without versions",
			content: `target 'TestApp' do
  pod 'AFNetworking'
  pod 'Alamofire'
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AFNetworking", Version: "latest"},
				{Type: "cocoapods", Name: "Alamofire", Version: "latest"},
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
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
				{Type: "cocoapods", Name: "SnapKit", Version: "latest"},
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
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
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
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
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
				{Type: "cocoapods", Name: "MySDK", Version: "1.0.0"},
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
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
				{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"},
				{Type: "cocoapods", Name: "Alamofire", Version: "5.6.0"},
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

func TestCocoaPodsParser_ParsePodspec(t *testing.T) {
	parser := NewCocoaPodsParser()

	tests := []struct {
		name     string
		content  string
		expected []types.Dependency
	}{
		{
			name: "parses dependencies with version constraints",
			content: `Pod::Specification.new do |s|
  s.name        = "MySDK"
  s.version     = "25.11.0"
  s.dependency "CommonLib", ">= 24.5.0"
  s.dependency "RestClient", "~> 2.1.0"
  s.dependency "JsonParser", "~> 2.3.0"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "CommonLib", Version: ">= 24.5.0"},
				{Type: "cocoapods", Name: "RestClient", Version: "~> 2.1.0"},
				{Type: "cocoapods", Name: "JsonParser", Version: "~> 2.3.0"},
			},
		},
		{
			name: "parses dependencies without version",
			content: `Pod::Specification.new do |spec|
  spec.dependency "SomeLib"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "SomeLib", Version: "latest"},
			},
		},
		{
			name: "handles double and single quotes",
			content: `Pod::Specification.new do |s|
  s.dependency 'CommonLib', '>= 1.0.0'
  s.dependency "RestClient", "~> 2.0"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "CommonLib", Version: ">= 1.0.0"},
				{Type: "cocoapods", Name: "RestClient", Version: "~> 2.0"},
			},
		},
		{
			name: "ignores comments and non-dependency lines",
			content: `# Generated file
Pod::Specification.new do |s|
  s.name        = "MySDK"
  s.version     = "1.0.0"
  s.license     = "MIT"
  # A dependency comment
  s.dependency "SomeLib", "1.0.0"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "SomeLib", Version: "1.0.0"},
			},
		},
		{
			name: "handles SNAPSHOT versions",
			content: `Pod::Specification.new do |s|
  s.dependency "AuditLog", ">= 24.12.0-SNAPSHOT"
  s.dependency "ClientLib", ">= 25.11.0"
end`,
			expected: []types.Dependency{
				{Type: "cocoapods", Name: "AuditLog", Version: ">= 24.12.0-SNAPSHOT"},
				{Type: "cocoapods", Name: "ClientLib", Version: ">= 25.11.0"},
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []types.Dependency{},
		},
		{
			name: "no dependencies",
			content: `Pod::Specification.new do |s|
  s.name = "MySDK"
  s.version = "1.0.0"
end`,
			expected: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParsePodspec(tt.content)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParsePodspec() = %v, want %v", got, tt.expected)
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
			expected: []types.Dependency{{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"}},
		},
		{
			name:     "extracts from Podfile.lock",
			content:  podfileLockContent,
			filename: "Podfile.lock",
			expected: []types.Dependency{{Type: "cocoapods", Name: "AFNetworking", Version: "4.0.1"}},
		},
		{
			name: "extracts from .podspec",
			content: `Pod::Specification.new do |s|
  s.dependency "AFNetworking", "~> 4.0.1"
end`,
			filename: "MySDK.podspec",
			expected: []types.Dependency{{Type: "cocoapods", Name: "AFNetworking", Version: "~> 4.0.1"}},
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
