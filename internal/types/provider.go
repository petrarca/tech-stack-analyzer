package types

import "io"

// Provider defines the interface for file system operations
type Provider interface {
	// ListDir returns the contents of a directory
	ListDir(path string) ([]File, error)

	// Open returns the content of a file
	Open(path string) (string, error)

	// Exists checks if a file or directory exists
	Exists(path string) (bool, error)

	// IsDir checks if a path is a directory
	IsDir(path string) (bool, error)

	// ReadFile reads file content as bytes
	ReadFile(path string) ([]byte, error)

	// GetBasePath returns the base path for this provider
	GetBasePath() string
}

// File represents a file or directory entry
type File struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // "file" or "dir"
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
}

// FileReader defines an interface for reading file content
type FileReader interface {
	io.Reader
	io.Seeker
	io.Closer
}
