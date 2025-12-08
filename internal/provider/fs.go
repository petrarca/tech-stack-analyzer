package provider

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// FSProvider implements the Provider interface for local file systems
type FSProvider struct {
	rootPath string
}

// NewFSProvider creates a new file system provider
func NewFSProvider(rootPath string) *FSProvider {
	return &FSProvider{
		rootPath: strings.TrimSuffix(rootPath, "/"),
	}
}

// ListDir returns the contents of a directory
func (p *FSProvider) ListDir(path string) ([]types.File, error) {
	fullPath := p.getFullPath(path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	files := make([]types.File, 0, len(entries))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't get info for
		}

		fileType := "file"
		if entry.IsDir() {
			fileType = "dir"
		}

		files = append(files, types.File{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			Type:     fileType,
			Size:     info.Size(),
			Modified: info.ModTime().Unix(),
		})
	}

	return files, nil
}

// Open returns the content of a file
func (p *FSProvider) Open(path string) (string, error) {
	fullPath := p.getFullPath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ReadFile reads file content as bytes
func (p *FSProvider) ReadFile(path string) ([]byte, error) {
	fullPath := p.getFullPath(path)
	return os.ReadFile(fullPath)
}

// Exists checks if a file or directory exists
func (p *FSProvider) Exists(path string) (bool, error) {
	fullPath := p.getFullPath(path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsDir checks if a path is a directory
func (p *FSProvider) IsDir(path string) (bool, error) {
	fullPath := p.getFullPath(path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// getFullPath converts a relative path to an absolute path
func (p *FSProvider) getFullPath(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}

	if path == "." || path == "" {
		return p.rootPath
	}

	return filepath.Join(p.rootPath, path)
}

// GetBasePath returns the base path for this provider
func (p *FSProvider) GetBasePath() string {
	return p.rootPath
}
