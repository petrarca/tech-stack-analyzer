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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "devise", Example: "4.8.0"},
				{Type: "ruby", Name: "pg", Example: "1.2.3"},
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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "devise", Example: "4.8.0"},
				{Type: "ruby", Name: "pg", Example: "1.2.3"},
				{Type: "ruby", Name: "redis", Example: "4.0.0"},
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
				{Type: "ruby", Name: "rails", Example: "latest"},
				{Type: "ruby", Name: "devise", Example: "latest"},
				{Type: "ruby", Name: "pg", Example: "latest"},
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
				{Type: "ruby", Name: "rails", Example: "~> 6.1.4"},
				{Type: "ruby", Name: "devise", Example: "~> 4.8"},
				{Type: "ruby", Name: "pg", Example: "~> 1.0"},
				{Type: "ruby", Name: "redis", Example: "~> 4.0"},
				{Type: "ruby", Name: "sidekiq", Example: "~> 6.0"},
				{Type: "ruby", Name: "sass-rails", Example: "~> 6.0"},
				{Type: "ruby", Name: "webpacker", Example: "~> 5.0"},
				{Type: "ruby", Name: "jbuilder", Example: "~> 2.7"},
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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "my_gem", Example: "latest"},
				{Type: "ruby", Name: "local_gem", Example: "latest"},
				{Type: "ruby", Name: "another_gem", Example: "1.0.0"},
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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "rspec-rails", Example: "5.0.0"},
				{Type: "ruby", Name: "factory_bot_rails", Example: "6.1.0"},
				{Type: "ruby", Name: "pry-rails", Example: "latest"},
				{Type: "ruby", Name: "puma", Example: "5.5.0"},
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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "pg", Example: "~> 1.0"},
				{Type: "ruby", Name: "pry-rails", Example: "latest"},
				{Type: "ruby", Name: "rubocop", Example: "latest"},
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
				{Type: "ruby", Name: "rails", Example: "6.1.4"},
				{Type: "ruby", Name: "invalid_gem", Example: "latest"},
				{Type: "ruby", Name: "another_gem", Example: "1.0.0"},
				{Type: "ruby", Name: "incomplete", Example: "latest"},
				// Empty gem name should be filtered out
				{Type: "ruby", Name: "valid_gem", Example: "2.0.0"},
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
				assert.Equal(t, expectedDep.Example, actualDep.Example, "Should have correct version for %s", name)
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
	assert.Len(t, dependencies, 15) // 9 production + 2 dev/test + 4 test

	// Create dependency map for verification
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Verify key production gems
	assert.Equal(t, "ruby", depMap["rails"].Type)
	assert.Equal(t, "~> 6.1.4", depMap["rails"].Example)
	assert.Equal(t, "ruby", depMap["pg"].Type)
	assert.Equal(t, "~> 1.1", depMap["pg"].Example)
	assert.Equal(t, "ruby", depMap["puma"].Type)
	assert.Equal(t, "~> 5.0", depMap["puma"].Example)

	// Verify development gems
	assert.Equal(t, "ruby", depMap["byebug"].Type)
	assert.Equal(t, "ruby", depMap["web-console"].Type)
	assert.Equal(t, ">= 4.1.0", depMap["web-console"].Example)

	// Verify test gems
	assert.Equal(t, "ruby", depMap["capybara"].Type)
	assert.Equal(t, ">= 3.26", depMap["capybara"].Example)
	assert.Equal(t, "ruby", depMap["selenium-webdriver"].Type)
	assert.Equal(t, ">= 4.0.0.rc1", depMap["selenium-webdriver"].Example)
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
