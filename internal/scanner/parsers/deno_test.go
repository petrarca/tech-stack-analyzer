package parsers

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDenoParser(t *testing.T) {
	parser := NewDenoParser()
	assert.NotNil(t, parser, "Should create a new DenoParser")
	assert.IsType(t, &DenoParser{}, parser, "Should return correct type")
}

func TestParseDenoLock(t *testing.T) {
	parser := NewDenoParser()

	tests := []struct {
		name            string
		content         string
		expectedVersion string
		expectedDeps    []types.Dependency
	}{
		{
			name: "basic deno.lock with dependencies",
			content: `{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.140.0/fmt/colors.ts": "3d5a9b5e5c5a5d5e5f5a5b5c5d5e5f5a5b5c5d5e",
    "https://deno.land/x/oak@v10.6.0/mod.ts": "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b",
    "https://deno.land/x/redis@v0.25.2/mod.ts": "9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1f0e9"
  }
}`,
			expectedVersion: "2",
			expectedDeps: []types.Dependency{
				{Type: "deno", Name: "https://deno.land/std@0.140.0/fmt/colors.ts", Example: "3d5a9b5e5c5a5d5e5f5a5b5c5d5e5f5a5b5c5d5e"},
				{Type: "deno", Name: "https://deno.land/x/oak@v10.6.0/mod.ts", Example: "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"},
				{Type: "deno", Name: "https://deno.land/x/redis@v0.25.2/mod.ts", Example: "9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1f0e9"},
			},
		},
		{
			name: "deno.lock with no dependencies",
			content: `{
  "version": "3",
  "remote": {}
}`,
			expectedVersion: "3",
			expectedDeps:    []types.Dependency{},
		},
		{
			name: "deno.lock with missing remote field",
			content: `{
  "version": "2"
}`,
			expectedVersion: "2",
			expectedDeps:    []types.Dependency{},
		},
		{
			name: "deno.lock with null remote field",
			content: `{
  "version": "2",
  "remote": null
}`,
			expectedVersion: "2",
			expectedDeps:    []types.Dependency{},
		},
		{
			name: "deno.lock with complex URLs",
			content: `{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.177.0/node/fs.ts": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
    "https://deno.land/x/express@v4.17.1/mod.ts": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1",
    "https://github.com/denoland/deno_std/archive/refs/heads/main.zip": "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
    "https://raw.githubusercontent.com/denoland/deno/main/README.md": "d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3",
    "file:///Users/user/project/local_module.ts": "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4"
  }
}`,
			expectedVersion: "2",
			expectedDeps: []types.Dependency{
				{Type: "deno", Name: "https://deno.land/std@0.177.0/node/fs.ts", Example: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"},
				{Type: "deno", Name: "https://deno.land/x/express@v4.17.1/mod.ts", Example: "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1"},
				{Type: "deno", Name: "https://github.com/denoland/deno_std/archive/refs/heads/main.zip", Example: "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2"},
				{Type: "deno", Name: "https://raw.githubusercontent.com/denoland/deno/main/README.md", Example: "d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3"},
				{Type: "deno", Name: "file:///Users/user/project/local_module.ts", Example: "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4"},
			},
		},
		{
			name:            "empty deno.lock",
			content:         "",
			expectedVersion: "",
			expectedDeps:    []types.Dependency{},
		},
		{
			name:            "invalid JSON",
			content:         "{ invalid json }",
			expectedVersion: "",
			expectedDeps:    []types.Dependency{},
		},
		{
			name: "deno.lock with version 1",
			content: `{
  "version": "1",
  "remote": {
    "https://deno.land/std@0.100.0/mod.ts": "f1e2d3c4b5a6f7e8d9c0b1a2c3d4e5f6a7b8c9d0"
  }
}`,
			expectedVersion: "1",
			expectedDeps: []types.Dependency{
				{Type: "deno", Name: "https://deno.land/std@0.100.0/mod.ts", Example: "f1e2d3c4b5a6f7e8d9c0b1a2c3d4e5f6a7b8c9d0"},
			},
		},
		{
			name: "deno.lock with special characters in URLs",
			content: `{
  "version": "2",
  "remote": {
    "https://deno.land/x/awesome-lib@v1.2.3/mod.ts": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
    "https://example.com/some-path/with-hyphens.ts": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1",
    "https://api.example.com/v1/endpoints": "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2"
  }
}`,
			expectedVersion: "2",
			expectedDeps: []types.Dependency{
				{Type: "deno", Name: "https://deno.land/x/awesome-lib@v1.2.3/mod.ts", Example: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"},
				{Type: "deno", Name: "https://example.com/some-path/with-hyphens.ts", Example: "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1"},
				{Type: "deno", Name: "https://api.example.com/v1/endpoints", Example: "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, dependencies := parser.ParseDenoLock(tt.content)

			assert.Equal(t, tt.expectedVersion, version, "Should return correct version")
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
				assert.Equal(t, expectedDep.Example, actualDep.Example, "Should have correct hash for %s", name)
			}
		})
	}
}

func TestDenoParser_Integration(t *testing.T) {
	parser := NewDenoParser()

	// Test realistic Deno application lock file
	realisticDenoLock := `{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.177.0/fmt/colors.ts": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c",
    "https://deno.land/std@0.177.0/async/mod.ts": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d",
    "https://deno.land/std@0.177.0/http/server.ts": "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e",
    "https://deno.land/x/oak@v12.1.0/mod.ts": "d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f",
    "https://deno.land/x/redis@v0.31.0/mod.ts": "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a",
    "https://deno.land/x/dotenv@v3.2.0/mod.ts": "f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b",
    "https://deno.land/x/zod@v3.21.4/mod.ts": "a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c",
    "https://deno.land/x/eta@v1.12.3/mod.ts": "b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d",
    "https://github.com/denoland/deno_std/archive/refs/heads/main.zip": "c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e",
    "file:///Users/user/project/src/utils.ts": "d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f"
  }
}`

	version, dependencies := parser.ParseDenoLock(realisticDenoLock)

	assert.Equal(t, "2", version)
	assert.Len(t, dependencies, 10)

	// Create dependency map for verification
	depMap := make(map[string]types.Dependency)
	for _, dep := range dependencies {
		depMap[dep.Name] = dep
	}

	// Verify standard library dependencies
	assert.Equal(t, "deno", depMap["https://deno.land/std@0.177.0/fmt/colors.ts"].Type)
	assert.Equal(t, "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c", depMap["https://deno.land/std@0.177.0/fmt/colors.ts"].Example)
	assert.Equal(t, "deno", depMap["https://deno.land/std@0.177.0/async/mod.ts"].Type)
	assert.Equal(t, "deno", depMap["https://deno.land/std@0.177.0/http/server.ts"].Type)

	// Verify third-party dependencies
	assert.Equal(t, "deno", depMap["https://deno.land/x/oak@v12.1.0/mod.ts"].Type)
	assert.Equal(t, "d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f", depMap["https://deno.land/x/oak@v12.1.0/mod.ts"].Example)
	assert.Equal(t, "deno", depMap["https://deno.land/x/redis@v0.31.0/mod.ts"].Type)
	assert.Equal(t, "deno", depMap["https://deno.land/x/dotenv@v3.2.0/mod.ts"].Type)

	// Verify GitHub dependency
	assert.Equal(t, "deno", depMap["https://github.com/denoland/deno_std/archive/refs/heads/main.zip"].Type)
	assert.Equal(t, "c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e", depMap["https://github.com/denoland/deno_std/archive/refs/heads/main.zip"].Example)

	// Verify local file dependency
	assert.Equal(t, "deno", depMap["file:///Users/user/project/src/utils.ts"].Type)
	assert.Equal(t, "d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f", depMap["file:///Users/user/project/src/utils.ts"].Example)
}

func TestDenoParser_EdgeCases(t *testing.T) {
	parser := NewDenoParser()

	// Test with very long URLs and hashes
	t.Run("long URLs and hashes", func(t *testing.T) {
		content := `{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.177.0/very/long/path/to/specific/module/file.ts": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0",
    "https://example.com/with/many/path/segments/and/parameters?query=value&other=param": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0"
  }
}`

		version, dependencies := parser.ParseDenoLock(content)

		assert.Equal(t, "2", version)
		assert.Len(t, dependencies, 2)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Contains(t, depMap, "https://deno.land/std@0.177.0/very/long/path/to/specific/module/file.ts")
		assert.Contains(t, depMap, "https://example.com/with/many/path/segments/and/parameters?query=value&other=param")
	})

	// Test with malformed remote entries
	t.Run("malformed remote entries", func(t *testing.T) {
		content := `{
  "version": "2",
  "remote": {
    "": "empty_url_hash",
    "https://deno.land/std@0.177.0/mod.ts": "",
    "https://example.com/mod.ts": "valid_hash"
  }
}`

		version, dependencies := parser.ParseDenoLock(content)

		assert.Equal(t, "2", version)

		// Should include all entries, even empty ones
		assert.Len(t, dependencies, 3)

		depMap := make(map[string]types.Dependency)
		for _, dep := range dependencies {
			depMap[dep.Name] = dep
		}

		assert.Equal(t, "empty_url_hash", depMap[""].Example)
		assert.Equal(t, "", depMap["https://deno.land/std@0.177.0/mod.ts"].Example)
		assert.Equal(t, "valid_hash", depMap["https://example.com/mod.ts"].Example)
	})

	// Test with different JSON formatting
	t.Run("different JSON formatting", func(t *testing.T) {
		content := `{
  "version": "2",
  "remote": {
    "https://deno.land/std@0.177.0/mod.ts": "hash1",
    "https://deno.land/x/oak@v12.1.0/mod.ts": "hash2"
  }
}`

		// Test with compact JSON
		compactContent := `{"version":"2","remote":{"https://deno.land/std@0.177.0/mod.ts":"hash1","https://deno.land/x/oak@v12.1.0/mod.ts":"hash2"}}`

		version1, deps1 := parser.ParseDenoLock(content)
		version2, deps2 := parser.ParseDenoLock(compactContent)

		assert.Equal(t, version1, version2)
		assert.Len(t, deps1, len(deps2))
	})
}
