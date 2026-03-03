package provider

import (
	"path/filepath"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// FakeProvider implements the Provider interface for testing
type FakeProvider struct {
	files   map[string][]types.File
	content map[string]string
}

// NewFakeProvider creates a new fake provider
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		files:   make(map[string][]types.File),
		content: make(map[string]string),
	}
}

// AddFile adds a file to the fake provider
func (p *FakeProvider) AddFile(path, content string) {
	dir := filepath.Dir(path)
	if dir == "." {
		dir = "/"
	}

	if p.files[dir] == nil {
		p.files[dir] = make([]types.File, 0)
	}

	filename := filepath.Base(path)
	p.files[dir] = append(p.files[dir], types.File{
		Name: filename,
		Path: path,
		Type: "file",
		Size: int64(len(content)),
	})

	p.content[path] = content
}

// AddDir adds a directory to the fake provider
func (p *FakeProvider) AddDir(path string) {
	if p.files[path] == nil {
		p.files[path] = make([]types.File, 0)
	}
}

// ListDir returns the contents of a directory
func (p *FakeProvider) ListDir(path string) ([]types.File, error) {
	files, exists := p.files[path]
	if !exists {
		return nil, nil // Directory doesn't exist
	}
	return files, nil
}

// Open returns the content of a file
func (p *FakeProvider) Open(path string) (string, error) {
	content, exists := p.content[path]
	if !exists {
		return "", nil // File doesn't exist
	}
	return content, nil
}

// ReadFile reads file content as bytes
func (p *FakeProvider) ReadFile(path string) ([]byte, error) {
	content, err := p.Open(path)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

// Exists checks if a file or directory exists
func (p *FakeProvider) Exists(path string) (bool, error) {
	_, fileExists := p.content[path]
	_, dirExists := p.files[path]
	return fileExists || dirExists, nil
}

// IsDir checks if a path is a directory
func (p *FakeProvider) IsDir(path string) (bool, error) {
	_, exists := p.files[path]
	return exists, nil
}
