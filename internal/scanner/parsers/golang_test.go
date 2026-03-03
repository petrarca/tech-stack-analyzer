package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGolangParser_ParseGoModWithInfo(t *testing.T) {
	parser := NewGolangParser()

	t.Run("parses simple require", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21

require github.com/gin-gonic/gin v1.9.0`

		deps, info := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 1)
		assert.Equal(t, "golang", deps[0].Type)
		assert.Equal(t, "github.com/gin-gonic/gin", deps[0].Name)
		assert.Equal(t, "v1.9.0", deps[0].Version)
		assert.Equal(t, "prod", deps[0].Scope)
		assert.True(t, deps[0].Direct)
		assert.Equal(t, "github.com/example/test", info.ModulePath)
		assert.Equal(t, "1.21", info.GoVersion)
	})

	t.Run("parses multiple requires", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/stretchr/testify v1.8.0
)`

		deps, _ := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 2)
		assert.Equal(t, "github.com/gin-gonic/gin", deps[0].Name)
		assert.Equal(t, "v1.9.0", deps[0].Version)
		assert.Equal(t, "github.com/stretchr/testify", deps[1].Name)
		assert.Equal(t, "v1.8.0", deps[1].Version)
	})

	t.Run("handles indented requires", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21

	require github.com/gin-gonic/gin v1.9.0
    require github.com/stretchr/testify v1.8.0`

		deps, _ := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 2)
	})

	t.Run("skips indirect dependencies", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21

require github.com/gin-gonic/gin v1.9.0 // indirect`

		deps, _ := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 0)
	})

	t.Run("handles empty content", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21`

		deps, _ := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 0)
	})

	t.Run("handles multi-line require block with indirect", func(t *testing.T) {
		content := `module github.com/example/test

go 1.21

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/stretchr/testify v1.8.0
	github.com/spf13/cobra v1.10.1 // indirect
)`

		deps, _ := parser.ParseGoModWithInfo(content)
		assert.Len(t, deps, 2) // Should skip indirect
		assert.Equal(t, "github.com/gin-gonic/gin", deps[0].Name)
		assert.Equal(t, "v1.9.0", deps[0].Version)
		assert.Equal(t, "github.com/stretchr/testify", deps[1].Name)
		assert.Equal(t, "v1.8.0", deps[1].Version)
	})
}
