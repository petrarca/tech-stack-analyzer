package git

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

// GitInfo contains git repository information
type GitInfo struct {
	Branch    string `json:"branch,omitempty"`
	Commit    string `json:"commit,omitempty"`
	IsDirty   bool   `json:"is_dirty"`
	RemoteURL string `json:"remote_url,omitempty"`
}

// GetGitInfo retrieves git repository information for the given path
// Consider using GetGitInfoCached for better performance in recursive scans
func GetGitInfo(path string) *GitInfo {
	info, _ := GetGitInfoWithRoot(path)
	return info
}

// FindRepoRoot finds the git repository root for a given path
// Returns empty string if not in a git repository
// This is a fast operation that doesn't compute status
func FindRepoRoot(path string) string {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return ""
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return ""
	}
	return worktree.Filesystem.Root()
}

// GetGitInfoWithRoot retrieves git info and returns the repository root path
// This allows callers to cache by repo root for better performance
func GetGitInfoWithRoot(path string) (*GitInfo, string) {
	// Open the repository
	repo, err := git.PlainOpen(path)
	if err != nil {
		// Not a git repository or error opening it
		return nil, ""
	}

	// Get worktree to find repo root
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, ""
	}
	repoRoot := worktree.Filesystem.Root()

	gitInfo := &GitInfo{}

	// Get current commit hash
	head, err := repo.Head()
	if err == nil {
		// Use short hash (first 7 characters)
		gitInfo.Commit = head.Hash().String()[:7]
	}

	// Get current branch
	if head != nil {
		// Check if we're on a branch (not detached HEAD)
		if head.Name().IsBranch() {
			gitInfo.Branch = head.Name().Short()
		} else {
			gitInfo.Branch = "HEAD" // Detached HEAD
		}
	}

	// Get worktree status to check if dirty (expensive operation)
	status, err := worktree.Status()
	if err == nil {
		gitInfo.IsDirty = !status.IsClean()
	}

	// Get remote URL (origin)
	remoteConfig, err := repo.Config()
	if err == nil {
		if origin := remoteConfig.Remotes["origin"]; origin != nil {
			if len(origin.URLs) > 0 {
				gitInfo.RemoteURL = origin.URLs[0]
			}
		}
	}

	return gitInfo, repoRoot
}

// GenerateRootIDFromGit generates a deterministic root ID from git remote URL and relative path
// If no remote URL is available, returns empty string
func GenerateRootIDFromGit(path string) string {
	// Get git info to extract remote URL and repo root
	gitInfo, repoRoot := GetGitInfoWithRoot(path)
	if gitInfo == nil || gitInfo.RemoteURL == "" {
		return ""
	}

	// Normalize the remote URL to create a consistent base
	remoteURL := normalizeRemoteURL(gitInfo.RemoteURL)

	// Get relative path from repo root if we're in a subdirectory
	var relativePath string
	if repoRoot != "" && repoRoot != path {
		rel, err := filepath.Rel(repoRoot, path)
		if err == nil {
			relativePath = rel
		}
	}

	// Generate deterministic ID: hash(remoteURL + relativePath)
	content := remoteURL
	if relativePath != "" && relativePath != "." {
		content += ":" + relativePath
	}

	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:20]
}

// normalizeRemoteURL converts various git URL formats to a consistent format
func normalizeRemoteURL(url string) string {
	// Remove protocol variations
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")
	url = strings.TrimPrefix(url, "git://")
	url = strings.TrimSuffix(url, ".git")

	// Convert SSH format git@github.com:user/repo to github.com/user/repo
	if strings.Contains(url, ":") && strings.Contains(url, "@") {
		url = strings.Replace(url, ":", "/", 1)
	}

	// Remove trailing slash
	url = strings.TrimSuffix(url, "/")

	return url
}

// GenerateRootIDFromPath generates a deterministic root ID from absolute path
// Used when git repository is not available
func GenerateRootIDFromPath(basePath string) string {
	// Ensure we have absolute path
	absPath := basePath
	if !filepath.IsAbs(basePath) {
		absPath = filepath.Join(basePath) // This will make it relative to current dir
	}

	// Normalize path for consistency across platforms
	absPath = filepath.Clean(absPath)

	hash := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(hash[:])[:20]
}
