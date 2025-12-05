package config

import (
	"os"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSettings(t *testing.T) {
	settings := DefaultSettings()

	assert.Equal(t, "stack-analysis.json", settings.OutputFile, "OutputFile should be stack-analysis.json by default")
	assert.True(t, settings.PrettyPrint, "PrettyPrint should be true by default")
	assert.Empty(t, settings.ExcludePatterns, "ExcludePatterns should be empty by default")
	assert.Equal(t, "", settings.Aggregate, "Aggregate should be empty by default")
	assert.Equal(t, slog.LevelError, settings.LogLevel, "LogLevel should be Error by default")
	assert.Equal(t, "text", settings.LogFormat, "LogFormat should be text by default")
}

func TestLoadSettings_WithDefaults(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	settings := LoadSettings()

	// Should match default settings
	defaultSettings := DefaultSettings()
	assert.Equal(t, defaultSettings.OutputFile, settings.OutputFile)
	assert.Equal(t, defaultSettings.PrettyPrint, settings.PrettyPrint)
	assert.Equal(t, defaultSettings.ExcludePatterns, settings.ExcludePatterns)
	assert.Equal(t, defaultSettings.Aggregate, settings.Aggregate)
	assert.Equal(t, defaultSettings.LogLevel, settings.LogLevel)
	assert.Equal(t, defaultSettings.LogFormat, settings.LogFormat)
}

func TestLoadSettings_WithEnvironmentVariables(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set environment variables
	os.Setenv("STACK_ANALYZER_OUTPUT", "/tmp/test.json")
	os.Setenv("STACK_ANALYZER_PRETTY", "false")
	os.Setenv("STACK_ANALYZER_EXCLUDE_DIRS", "vendor,node_modules,build")
	os.Setenv("STACK_ANALYZER_AGGREGATE", "tech,techs")
	os.Setenv("STACK_ANALYZER_LOG_LEVEL", "debug")
	os.Setenv("STACK_ANALYZER_LOG_FORMAT", "json")

	defer clearEnvVars()

	settings := LoadSettings()

	assert.Equal(t, "/tmp/test.json", settings.OutputFile)
	assert.False(t, settings.PrettyPrint)
	assert.Equal(t, []string{"vendor", "node_modules", "build"}, settings.ExcludePatterns)
	assert.Equal(t, "tech,techs", settings.Aggregate)
	assert.Equal(t, slog.LevelDebug, settings.LogLevel)
	assert.Equal(t, "json", settings.LogFormat)
}

func TestLoadSettings_WithPartialEnvironmentVariables(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set only some environment variables
	os.Setenv("STACK_ANALYZER_PRETTY", "false")
	os.Setenv("STACK_ANALYZER_LOG_LEVEL", "error")

	defer clearEnvVars()

	settings := LoadSettings()

	// Should have defaults for unset variables
	assert.Equal(t, "stack-analysis.json", settings.OutputFile)
	assert.False(t, settings.PrettyPrint) // From environment
	assert.Empty(t, settings.ExcludePatterns)
	assert.Equal(t, "", settings.Aggregate)
	assert.Equal(t, slog.LevelError, settings.LogLevel) // From environment
	assert.Equal(t, "text", settings.LogFormat)
}

func TestLoadSettings_InvalidLogLevel(t *testing.T) {
	// Clear any existing environment variables
	clearEnvVars()

	// Set invalid log level
	os.Setenv("STACK_ANALYZER_LOG_LEVEL", "invalid")

	defer clearEnvVars()

	settings := LoadSettings()

	// Should fall back to default for invalid log level
	assert.Equal(t, slog.LevelError, settings.LogLevel, "Should use default log level for invalid input")
}

func TestLoadSettings_BooleanParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"true uppercase", "TRUE", true},
		{"false lowercase", "false", false},
		{"false uppercase", "FALSE", false},
		{"invalid value", "maybe", false}, // Should default to false
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()
			os.Setenv("STACK_ANALYZER_PRETTY", tt.envValue)
			defer clearEnvVars()

			settings := LoadSettings()
			assert.Equal(t, tt.expected, settings.PrettyPrint)
		})
	}
}

func TestConfigureLogger_TextFormat(t *testing.T) {
	settings := &Settings{
		LogLevel:  slog.LevelDebug,
		LogFormat: "text",
	}

	logger := settings.ConfigureLogger()

	// slog doesn't expose level in the same way, just test that we get a logger
	assert.NotNil(t, logger)
}

func TestConfigureLogger_JSONFormat(t *testing.T) {
	settings := &Settings{
		LogLevel:  slog.LevelWarn,
		LogFormat: "json",
	}

	logger := settings.ConfigureLogger()

	// slog doesn't expose level in the same way, just test that we get a logger
	assert.NotNil(t, logger)
}

func TestConfigureLogger_InvalidFormat(t *testing.T) {
	settings := &Settings{
		LogLevel:  slog.LevelInfo,
		LogFormat: "invalid",
	}

	logger := settings.ConfigureLogger()

	// slog doesn't expose formatter, just test that we get a logger
	assert.NotNil(t, logger)
}

func TestValidate_AlwaysReturnsNil(t *testing.T) {
	settings := DefaultSettings()

	err := settings.Validate()
	assert.NoError(t, err, "Validate should always return nil for now")
}

// Helper function to clear environment variables
func clearEnvVars() {
	envVars := []string{
		"STACK_ANALYZER_OUTPUT",
		"STACK_ANALYZER_PRETTY",
		"STACK_ANALYZER_EXCLUDE_DIRS",
		"STACK_ANALYZER_AGGREGATE",
		"STACK_ANALYZER_LOG_LEVEL",
		"STACK_ANALYZER_LOG_FORMAT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

// Test table for exclude patterns parsing
func TestLoadSettings_ExcludePatternsParsing(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{"single dir", "vendor", []string{"vendor"}},
		{"multiple dirs", "vendor,node_modules", []string{"vendor", "node_modules"}},
		{"with spaces", "vendor , node_modules , build", []string{"vendor", "node_modules", "build"}},
		{"empty string", "", []string{}},
		{"only commas", ",,,", []string{"", "", "", ""}}, // This might need improvement
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()
			os.Setenv("STACK_ANALYZER_EXCLUDE_DIRS", tt.envValue)
			defer clearEnvVars()

			settings := LoadSettings()
			assert.Equal(t, tt.expected, settings.ExcludePatterns)
		})
	}
}

// Test that LoadSettings doesn't modify the default settings
func TestLoadSettings_DoesNotModifyDefaults(t *testing.T) {
	// Get default settings first
	defaultSettings := DefaultSettings()

	// Set environment variables
	os.Setenv("STACK_ANALYZER_PRETTY", "false")
	defer clearEnvVars()

	// Load settings with environment overrides
	settings := LoadSettings()

	// Default settings should remain unchanged
	assert.True(t, defaultSettings.PrettyPrint, "Default settings should not be modified")

	// Loaded settings should have the override
	assert.False(t, settings.PrettyPrint, "Loaded settings should have environment override")
}
