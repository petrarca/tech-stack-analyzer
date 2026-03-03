package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGemfileLockParser(t *testing.T) {
	parser := NewGemfileLockParser()
	assert.NotNil(t, parser)
	assert.IsType(t, &GemfileLockParser{}, parser)
}

func TestParseGemfileLock(t *testing.T) {
	parser := NewGemfileLockParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "basic Gemfile.lock with only direct dependencies (default)",
			content: `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
      actioncable (= 7.1.0)
      actionpack (= 7.1.0)
    actioncable (7.1.0)
      actionpack (= 7.1.0)
      nio4r (~> 2.0)
    actionpack (7.1.0)
      rack (~> 2.0)
    nio4r (2.5.9)
    rack (2.2.8)

PLATFORMS
  ruby

DEPENDENCIES
  rails (= 7.1.0)

BUNDLED WITH
   2.4.10
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "7.1.0", Direct: true},
			},
		},
		{
			name: "multiple direct dependencies",
			content: `GEM
  remote: https://rubygems.org/
  specs:
    pg (1.5.4)
    puma (6.4.0)
      nio4r (~> 2.0)
    nio4r (2.5.9)
    rails (7.1.0)

PLATFORMS
  ruby

DEPENDENCIES
  pg (~> 1.5)
  puma (~> 6.4)
  rails (= 7.1.0)

BUNDLED WITH
   2.4.10
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "pg", Version: "1.5.4", Direct: true},
				{Type: "ruby", Name: "puma", Version: "6.4.0", Direct: true},
				{Type: "ruby", Name: "rails", Version: "7.1.0", Direct: true},
			},
		},
		{
			name: "platform-specific gems",
			content: `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.15.5)
      racc (~> 1.4)
    nokogiri (1.15.5-x86_64-linux)
      racc (~> 1.4)
    racc (1.7.3)

PLATFORMS
  ruby
  x86_64-linux

DEPENDENCIES
  nokogiri

BUNDLED WITH
   2.4.10
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "nokogiri", Version: "1.15.5", Direct: true},
				{Type: "ruby", Name: "nokogiri", Version: "1.15.5-x86_64-linux", Direct: true},
			},
		},
		{
			name:         "empty Gemfile.lock",
			content:      "",
			expectedDeps: []types.Dependency{},
		},
		{
			name: "Gemfile.lock with only comments",
			content: `# This is a comment
# Another comment
`,
			expectedDeps: []types.Dependency{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies := parser.ParseGemfileLock(tt.content)

			require.Len(t, dependencies, len(tt.expectedDeps))

			for i, expectedDep := range tt.expectedDeps {
				assert.Equal(t, expectedDep.Type, dependencies[i].Type)
				assert.Equal(t, expectedDep.Name, dependencies[i].Name)
				assert.Equal(t, expectedDep.Version, dependencies[i].Version)
				assert.Equal(t, expectedDep.Direct, dependencies[i].Direct)
				assert.Equal(t, types.ScopeProd, dependencies[i].Scope)
				assert.NotNil(t, dependencies[i].Metadata)
				assert.Equal(t, "Gemfile.lock", dependencies[i].Metadata["source"])
			}
		})
	}
}

func TestParseGemfileLockWithMetadata(t *testing.T) {
	parser := NewGemfileLockParser()

	t.Run("extract platforms and bundler version", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)

PLATFORMS
  ruby
  x86_64-linux
  x86_64-darwin

DEPENDENCIES
  rails (= 7.1.0)

BUNDLED WITH
   2.4.10
`

		deps, metadata := parser.ParseGemfileLockWithMetadata(content)

		assert.Len(t, deps, 1)
		assert.Equal(t, "rails", deps[0].Name)

		platforms, ok := metadata["platforms"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"ruby", "x86_64-linux", "x86_64-darwin"}, platforms)

		bundlerVersion, ok := metadata["bundler_version"].(string)
		require.True(t, ok)
		assert.Equal(t, "2.4.10", bundlerVersion)
	})

	t.Run("no platforms or bundler version", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)

DEPENDENCIES
  rails (= 7.1.0)
`

		_, metadata := parser.ParseGemfileLockWithMetadata(content)

		assert.NotContains(t, metadata, "platforms")
		assert.NotContains(t, metadata, "bundler_version")
	})
}

func TestParseGemfileLockWithOptions(t *testing.T) {
	parser := NewGemfileLockParser()

	content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
      actioncable (= 7.1.0)
      actionpack (= 7.1.0)
    actioncable (7.1.0)
    actionpack (7.1.0)
    pg (1.5.4)

PLATFORMS
  ruby

DEPENDENCIES
  rails (= 7.1.0)
  pg (~> 1.5)

BUNDLED WITH
   2.4.10
`

	t.Run("default excludes transitive dependencies", func(t *testing.T) {
		dependencies := parser.ParseGemfileLock(content)

		assert.Len(t, dependencies, 2)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.True(t, depMap["rails"].Direct)
		assert.True(t, depMap["pg"].Direct)
		assert.NotContains(t, depMap, "actioncable")
		assert.NotContains(t, depMap, "actionpack")
	})

	t.Run("IncludeTransitive includes all dependencies", func(t *testing.T) {
		dependencies := parser.ParseGemfileLockWithOptions(content, ParseGemfileLockOptions{IncludeTransitive: true})

		assert.Len(t, dependencies, 4)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.True(t, depMap["rails"].Direct)
		assert.True(t, depMap["pg"].Direct)
		assert.False(t, depMap["actioncable"].Direct)
		assert.False(t, depMap["actionpack"].Direct)
	})
}

func TestGemfileLockParser_Integration(t *testing.T) {
	parser := NewGemfileLockParser()

	// Realistic Rails application Gemfile.lock
	railsGemfileLock := `GEM
  remote: https://rubygems.org/
  specs:
    actioncable (7.1.0)
      actionpack (= 7.1.0)
      activesupport (= 7.1.0)
      nio4r (~> 2.0)
      websocket-driver (>= 0.6.1)
    actionpack (7.1.0)
      actionview (= 7.1.0)
      activesupport (= 7.1.0)
      rack (~> 2.0, >= 2.2.4)
    actionview (7.1.0)
      activesupport (= 7.1.0)
      builder (~> 3.1)
    activesupport (7.1.0)
      concurrent-ruby (~> 1.0, >= 1.0.2)
      i18n (>= 1.6, < 2)
      minitest (>= 5.1)
      tzinfo (~> 2.0)
    builder (3.2.4)
    concurrent-ruby (1.2.2)
    i18n (1.14.1)
      concurrent-ruby (~> 1.0)
    minitest (5.20.0)
    nio4r (2.5.9)
    pg (1.5.4)
    puma (6.4.0)
      nio4r (~> 2.0)
    rack (2.2.8)
    rails (7.1.0)
      actioncable (= 7.1.0)
      actionpack (= 7.1.0)
      actionview (= 7.1.0)
      activesupport (= 7.1.0)
    rspec-rails (6.1.0)
      rspec-core (~> 3.12)
    rspec-core (3.12.2)
    tzinfo (2.0.6)
      concurrent-ruby (~> 1.0)
    websocket-driver (0.7.6)

PLATFORMS
  x86_64-linux
  ruby

DEPENDENCIES
  pg (~> 1.5)
  puma (~> 6.4)
  rails (= 7.1.0)
  rspec-rails (~> 6.1)

BUNDLED WITH
   2.4.10
`

	dependencies := parser.ParseGemfileLock(railsGemfileLock)

	// Should extract only direct dependencies by default
	assert.Len(t, dependencies, 4)

	// Create dependency map for verification
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Verify all are direct dependencies
	assert.True(t, depMap["rails"].Direct)
	assert.True(t, depMap["pg"].Direct)
	assert.True(t, depMap["puma"].Direct)
	assert.True(t, depMap["rspec-rails"].Direct)

	// Verify exact versions from lockfile
	assert.Equal(t, "7.1.0", depMap["rails"].Version)
	assert.Equal(t, "1.5.4", depMap["pg"].Version)
	assert.Equal(t, "6.4.0", depMap["puma"].Version)
	assert.Equal(t, "6.1.0", depMap["rspec-rails"].Version)

	// Verify transitive dependencies are NOT included by default
	assert.NotContains(t, depMap, "actioncable")
	assert.NotContains(t, depMap, "activesupport")
	assert.NotContains(t, depMap, "nio4r")
	assert.NotContains(t, depMap, "rack")
}

func TestGemfileLockParser_EdgeCases(t *testing.T) {
	parser := NewGemfileLockParser()

	t.Run("gem with complex version", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.15.5-x86_64-linux)
    rails (7.1.0.rc1)
    devise (4.9.3.pre)

PLATFORMS
  x86_64-linux

DEPENDENCIES
  nokogiri
  rails
  devise

BUNDLED WITH
   2.4.10
`

		dependencies := parser.ParseGemfileLock(content)

		assert.Len(t, dependencies, 3)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Equal(t, "1.15.5-x86_64-linux", depMap["nokogiri"].Version)
		assert.Equal(t, "7.1.0.rc1", depMap["rails"].Version)
		assert.Equal(t, "4.9.3.pre", depMap["devise"].Version)
	})

	t.Run("malformed DEPENDENCIES section", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
    pg (1.5.4)

PLATFORMS
  ruby

DEPENDENCIES
  rails (= 7.1.0)
  
  pg

BUNDLED WITH
   2.4.10
`

		dependencies := parser.ParseGemfileLock(content)

		assert.Len(t, dependencies, 2)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.True(t, depMap["rails"].Direct)
		assert.True(t, depMap["pg"].Direct)
	})

	t.Run("missing sections", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
`

		dependencies := parser.ParseGemfileLock(content)

		// No DEPENDENCIES section means all gems are transitive, so default behavior returns nothing
		assert.Len(t, dependencies, 0)
	})

	t.Run("empty GEM section", func(t *testing.T) {
		content := `GEM
  remote: https://rubygems.org/
  specs:

PLATFORMS
  ruby

DEPENDENCIES
  rails

BUNDLED WITH
   2.4.10
`

		dependencies := parser.ParseGemfileLock(content)

		assert.Len(t, dependencies, 0)
	})
}
