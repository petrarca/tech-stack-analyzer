package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRubyParser(t *testing.T) {
	parser := NewRubyParser()
	assert.NotNil(t, parser, "Should create a new RubyParser")
	assert.IsType(t, &RubyParser{}, parser, "Should return correct type")
}

func TestParseGemfile(t *testing.T) {
	parser := NewRubyParser()

	tests := []struct {
		name         string
		content      string
		expectedDeps []types.Dependency
	}{
		{
			name: "basic Gemfile with versions",
			content: `source 'https://rubygems.org'

gem 'rails', '6.1.4'
gem 'devise', '4.8.0'
gem 'pg', '1.2.3'
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "devise", Version: "4.8.0"},
				{Type: "ruby", Name: "pg", Version: "1.2.3"},
			},
		},
		{
			name: "Gemfile with mixed quotes",
			content: `gem "rails", "6.1.4"
gem 'devise', '4.8.0'
gem "pg", '1.2.3'
gem 'redis', "4.0.0"
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "devise", Version: "4.8.0"},
				{Type: "ruby", Name: "pg", Version: "1.2.3"},
				{Type: "ruby", Name: "redis", Version: "4.0.0"},
			},
		},
		{
			name: "Gemfile without versions",
			content: `source 'https://rubygems.org'

gem 'rails'
gem "devise"
gem 'pg'
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "latest"},
				{Type: "ruby", Name: "devise", Version: "latest"},
				{Type: "ruby", Name: "pg", Version: "latest"},
			},
		},
		{
			name: "Gemfile with complex dependencies",
			content: `source 'https://rubygems.org'
git_source(:github) { |repo| "https://github.com/#{repo}.git" }

gem 'rails', '~> 6.1.4'
gem 'devise', '~> 4.8'
gem 'pg', '~> 1.0'
gem 'redis', '~> 4.0'
gem 'sidekiq', '~> 6.0'
gem 'sass-rails', '~> 6.0'
gem 'webpacker', '~> 5.0'
gem 'jbuilder', '~> 2.7'
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "~> 6.1.4"},
				{Type: "ruby", Name: "devise", Version: "~> 4.8"},
				{Type: "ruby", Name: "pg", Version: "~> 1.0"},
				{Type: "ruby", Name: "redis", Version: "~> 4.0"},
				{Type: "ruby", Name: "sidekiq", Version: "~> 6.0"},
				{Type: "ruby", Name: "sass-rails", Version: "~> 6.0"},
				{Type: "ruby", Name: "webpacker", Version: "~> 5.0"},
				{Type: "ruby", Name: "jbuilder", Version: "~> 2.7"},
			},
		},
		{
			name: "Gemfile with git and path dependencies",
			content: `gem 'rails', '6.1.4'
gem 'my_gem', git: 'https://github.com/user/my_gem.git'
gem 'local_gem', path: '../local_gem'
gem 'another_gem', '1.0.0'
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "my_gem", Version: "latest"},
				{Type: "ruby", Name: "local_gem", Version: "latest"},
				{Type: "ruby", Name: "another_gem", Version: "1.0.0"},
			},
		},
		{
			name: "Gemfile with groups",
			content: `gem 'rails', '6.1.4'

group :development, :test do
  gem 'rspec-rails', '5.0.0'
  gem 'factory_bot_rails', '6.1.0'
  gem 'pry-rails'
end

group :production do
  gem 'puma', '5.5.0'
end
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "rspec-rails", Version: "5.0.0"},
				{Type: "ruby", Name: "factory_bot_rails", Version: "6.1.0"},
				{Type: "ruby", Name: "pry-rails", Version: "latest"},
				{Type: "ruby", Name: "puma", Version: "5.5.0"},
			},
		},
		{
			name: "Gemfile with comments and empty lines",
			content: `# A sample Gemfile
source 'https://rubygems.org'

# Rails framework
gem 'rails', '6.1.4'

# Database
gem 'pg', '~> 1.0'

# Development tools
group :development do
  gem 'pry-rails'  # Debugging
  gem 'rubocop'   # Linting
end

# Empty line above and below

`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "pg", Version: "~> 1.0"},
				{Type: "ruby", Name: "pry-rails", Version: "latest"},
				{Type: "ruby", Name: "rubocop", Version: "latest"},
			},
		},
		{
			name:         "empty Gemfile",
			content:      "",
			expectedDeps: []types.Dependency{},
		},
		{
			name: "Gemfile with only comments",
			content: `# This is just a comment
# Another comment
# No gems here`,
			expectedDeps: []types.Dependency{},
		},
		{
			name: "Gemfile with malformed gem declarations",
			content: `gem 'rails', '6.1.4'
gem 'invalid_gem'
gem 'another_gem', '1.0.0'
gem 'incomplete'
gem ""  # Empty gem name
gem 'valid_gem', '2.0.0'
`,
			expectedDeps: []types.Dependency{
				{Type: "ruby", Name: "rails", Version: "6.1.4"},
				{Type: "ruby", Name: "invalid_gem", Version: "latest"},
				{Type: "ruby", Name: "another_gem", Version: "1.0.0"},
				{Type: "ruby", Name: "incomplete", Version: "latest"},
				// Empty gem name should be filtered out
				{Type: "ruby", Name: "valid_gem", Version: "2.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies := parser.ParseGemfile(tt.content)

			require.Len(t, dependencies, len(tt.expectedDeps), "Should return correct number of dependencies")

			// Create dependency maps for order-independent comparison
			expectedDepMap := make(map[string]types.Dependency)
			actualDepMap := make(map[string]types.Dependency)

			for _, dep := range tt.expectedDeps {
				expectedDepMap[dep.Name] = dep
			}

			for _, dep := range dependencies {
				actualDepMap[dep.Name] = dep
			}

			// Verify all expected dependencies are present
			for name, expectedDep := range expectedDepMap {
				actualDep, exists := actualDepMap[name]
				require.True(t, exists, "Expected dependency %s not found", name)
				assert.Equal(t, expectedDep.Type, actualDep.Type, "Should have correct type for %s", name)
				assert.Equal(t, expectedDep.Version, actualDep.Version, "Should have correct version for %s", name)
			}
		})
	}
}

func TestRubyParser_Integration(t *testing.T) {
	parser := NewRubyParser()

	// Test realistic Rails application Gemfile
	railsGemfile := `source 'https://rubygems.org'
git_source(:github) { |repo| "https://github.com/#{repo}.git" }

ruby '3.0.0'

# Bundle edge Rails instead: gem 'rails', github: 'rails/rails', branch: 'main'
gem 'rails', '~> 6.1.4'
# Use postgresql as the database for Active Record
gem 'pg', '~> 1.1'
# Use the Puma web server [https://github.com/puma/puma]
gem 'puma', '~> 5.0'
# Build JSON APIs with ease [https://github.com/rails/jbuilder]
gem 'jbuilder', '~> 2.7'
# Use Redis adapter to run Action Cable in production
gem 'redis', '~> 5.0'
# Use Active Model has_secure_password [https://guides.rubyonrails.org/active_model_basics.html#securepassword]
gem 'bcrypt', '~> 3.1.7'

# Use Active Storage variant [https://guides.rubyonrails.org/active_storage_overview.html#transforming-images]
gem 'image_processing', '~> 1.2'

# Reduces boot times through caching; required in config/boot.rb
gem 'bootsnap', '>= 1.4.4', require: false

group :development, :test do
  # Call 'byebug' anywhere in the code to stop execution and get a debugger console
  gem 'byebug', platforms: [:mri, :mingw, :x64_mingw]
end

group :development do
  # Access an interactive console on exception pages or by calling 'console' anywhere in the code.
  gem 'web-console', '>= 4.1.0'
  # Display performance information such as SQL queries and inject variables into view templates.
  gem 'rack-mini-profiler', '~> 2.0'
end

group :test do
  # Adds support for Capybara system testing and selenium driver
  gem 'capybara', '>= 3.26'
  gem 'selenium-webdriver', '>= 4.0.0.rc1'
  # Easy installation and use of web drivers to run system tests with browsers
  gem 'webdrivers', '~> 5.0'
end
`

	dependencies := parser.ParseGemfile(railsGemfile)

	// Should extract all gems with versions
	assert.Len(t, dependencies, 14) // 8 production + 2 dev/test + 4 test

	// Create dependency map for verification
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Verify key production gems
	assert.Equal(t, "ruby", depMap["rails"].Type)
	assert.Equal(t, "~> 6.1.4", depMap["rails"].Version)
	assert.Equal(t, "ruby", depMap["pg"].Type)
	assert.Equal(t, "~> 1.1", depMap["pg"].Version)
	assert.Equal(t, "ruby", depMap["puma"].Type)
	assert.Equal(t, "~> 5.0", depMap["puma"].Version)

	// Verify development gems
	assert.Equal(t, "ruby", depMap["byebug"].Type)
	assert.Equal(t, "ruby", depMap["web-console"].Type)
	assert.Equal(t, ">= 4.1.0", depMap["web-console"].Version)

	// Verify test gems
	assert.Equal(t, "ruby", depMap["capybara"].Type)
	assert.Equal(t, ">= 3.26", depMap["capybara"].Version)
	assert.Equal(t, "ruby", depMap["selenium-webdriver"].Type)
	assert.Equal(t, ">= 4.0.0.rc1", depMap["selenium-webdriver"].Version)
}

func TestRubyParser_EdgeCases(t *testing.T) {
	parser := NewRubyParser()

	// Test gem names with hyphens and underscores
	t.Run("complex gem names", func(t *testing.T) {
		content := `gem 'sass-rails', '~> 6.0'
gem 'webpacker', '~> 5.0'
gem 'factory_bot_rails', '6.1.0'
gem 'devise-token-auth', '1.1.0'
gem 'active_model_serializers', '0.10.0'
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 5)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Equal(t, "sass-rails", depMap["sass-rails"].Name)
		assert.Equal(t, "webpacker", depMap["webpacker"].Name)
		assert.Equal(t, "factory_bot_rails", depMap["factory_bot_rails"].Name)
		assert.Equal(t, "devise-token-auth", depMap["devise-token-auth"].Name)
		assert.Equal(t, "active_model_serializers", depMap["active_model_serializers"].Name)
	})

	// Test version constraints
	t.Run("version constraints", func(t *testing.T) {
		content := `gem 'rails', '>= 6.0'
gem 'devise', '~> 4.8'
gem 'pg', '~> 1.0', '>= 1.0.0'
gem 'redis', '!= 4.0'
gem 'sidekiq', '~> 6.0', '>= 6.0.0'
`

		dependencies := parser.ParseGemfile(content)

		// Current regex handles simple version constraints but not complex ones
		assert.Len(t, dependencies, 5) // All simple constraints will match
	})

	// Test gems with special characters in names
	t.Run("special characters", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'awesome_print', '1.9.0'
gem 'will_paginate', '3.3.0'
gem 'paperclip', '6.1.0'
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 4)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Equal(t, "awesome_print", depMap["awesome_print"].Name)
		assert.Equal(t, "will_paginate", depMap["will_paginate"].Name)
	})
}

func TestRubyParser_MetadataEdgeCases(t *testing.T) {
	parser := NewRubyParser()

	// Test empty groups
	t.Run("empty groups", func(t *testing.T) {
		content := `group do
  gem 'rails', '6.1.4'
end`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 1)
		assert.Equal(t, types.ScopeProd, dependencies[0].Scope)   // Should default to prod when no groups
		assert.NotContains(t, dependencies[0].Metadata, "groups") // Should not add empty groups
	})

	// Test malformed git URLs
	t.Run("malformed git sources", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'bad_git', git: 'not-a-url'
gem 'valid_git', git: 'https://github.com/user/repo.git'
gem 'partial_git', git: ''
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 4) // partial_git is still extracted even with empty git

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		// Should extract malformed git URL as-is (parser doesn't validate URL format)
		assert.Equal(t, "not-a-url", depMap["bad_git"].Metadata["git"])
		assert.Equal(t, "https://github.com/user/repo.git", depMap["valid_git"].Metadata["git"])
		assert.NotContains(t, depMap["partial_git"].Metadata, "git") // Empty git should not be added
	})

	// Test malformed paths
	t.Run("malformed paths", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'empty_path', path: ''
gem 'valid_path', path: '../local_gem'
gem 'relative_path', path: '/absolute/path'
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 4) // empty_path is still extracted even with empty path

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.NotContains(t, depMap["empty_path"].Metadata, "path") // Empty path should not be added
		assert.Equal(t, "../local_gem", depMap["valid_path"].Metadata["path"])
		assert.Equal(t, "/absolute/path", depMap["relative_path"].Metadata["path"])
	})

	// Test malformed platforms
	t.Run("malformed platforms", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'empty_platforms', platforms: []
gem 'valid_platforms', platforms: [:mri, :mingw]
gem 'mixed_platforms', platforms: [:ruby, "jruby", :truffleruby]
gem 'single_platform', platforms: [:x64_mingw]
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 5) // empty_platforms is still extracted even with empty platforms

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.NotContains(t, depMap["empty_platforms"].Metadata, "platforms") // Empty platforms should not be added

		validPlatforms := depMap["valid_platforms"].Metadata["platforms"].([]string)
		assert.Equal(t, []string{"mri", "mingw"}, validPlatforms)

		mixedPlatforms := depMap["mixed_platforms"].Metadata["platforms"].([]string)
		assert.Equal(t, []string{"ruby", "jruby", "truffleruby"}, mixedPlatforms)

		singlePlatform := depMap["single_platform"].Metadata["platforms"].([]string)
		assert.Equal(t, []string{"x64_mingw"}, singlePlatform)
	})

	// Test nested groups
	t.Run("nested groups", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'

group :development do
  gem 'rubocop'
  group :test do
    gem 'rspec'
  end
  gem 'pry'
end

group :test do
  gem 'capybara'
end`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 5)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		// Production gem
		assert.Equal(t, types.ScopeProd, depMap["rails"].Scope)

		// Development gems
		assert.Equal(t, types.ScopeDev, depMap["rubocop"].Scope)
		assert.Equal(t, types.ScopeDev, depMap["pry"].Scope)

		// Test gems (including nested)
		assert.Equal(t, types.ScopeDev, depMap["rspec"].Scope)
		assert.Equal(t, types.ScopeDev, depMap["capybara"].Scope)
	})

	// Test malformed group syntax
	t.Run("malformed group syntax", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'

group :development
  gem 'rubocop'
end

group :test do
  gem 'rspec'
end

group "" do
  gem 'invalid_group'
end`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 4) // All gems are extracted, rubocop defaults to prod due to malformed group

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		// Should handle malformed group syntax gracefully
		assert.Equal(t, types.ScopeProd, depMap["rails"].Scope)
		assert.Equal(t, types.ScopeDev, depMap["rspec"].Scope)
		assert.Equal(t, types.ScopeProd, depMap["invalid_group"].Scope) // Empty group should default to prod
	})

	// Test require flag variations
	t.Run("require flag variations", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'no_require', require: false
gem 'no_require_compact', require:false
gem 'require_true', require: true
gem 'require_string', require: 'custom'
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 5)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.NotContains(t, depMap["rails"].Metadata, "require")
		assert.Equal(t, false, depMap["no_require"].Metadata["require"])
		assert.Equal(t, false, depMap["no_require_compact"].Metadata["require"])
		assert.NotContains(t, depMap["require_true"].Metadata, "require")   // Only false is captured
		assert.NotContains(t, depMap["require_string"].Metadata, "require") // Only false is captured
	})

	// Test branch extraction
	t.Run("branch extraction", func(t *testing.T) {
		content := `gem 'rails', '6.1.4'
gem 'main_branch', git: 'https://github.com/user/repo.git', branch: 'main'
gem 'develop_branch', git: 'https://github.com/user/repo2.git', branch: 'develop'
gem 'no_branch', git: 'https://github.com/user/repo3.git'
gem 'empty_branch', git: 'https://github.com/user/repo4.git', branch: ''
`

		dependencies := parser.ParseGemfile(content)

		assert.Len(t, dependencies, 5)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.NotContains(t, depMap["rails"].Metadata, "branch")
		assert.Equal(t, "main", depMap["main_branch"].Metadata["branch"])
		assert.Equal(t, "develop", depMap["develop_branch"].Metadata["branch"])
		assert.NotContains(t, depMap["no_branch"].Metadata, "branch")
		assert.NotContains(t, depMap["empty_branch"].Metadata, "branch")
	})
}
