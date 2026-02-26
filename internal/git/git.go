package git

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
)

// GitInfo contains git repository information
type GitInfo struct {
	Branch    string `json:"branch,omitempty"`
	Commit    string `json:"commit,omitempty"`
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

	// PERFORMANCE: Skipping worktree.Status() check - can take 20+ seconds on large repos
	// IsDirty field was removed as it wasn't used in any analysis or output

	// Get remote URL (origin)
	remoteConfig, err := repo.Config()
	if err == nil {
		if origin := remoteConfig.Remotes["origin"]; origin != nil {
			if len(origin.URLs) > 0 {
				gitInfo.RemoteURL = sanitizeRemoteURL(origin.URLs[0])
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

// sanitizeRemoteURL removes credentials (userinfo) from a git remote URL
// to prevent tokens and passwords from leaking into scan output.
// Examples:
//   - https://oauth2:token@git.example.com/repo.git -> https://git.example.com/repo.git
//   - https://user:pass@github.com/org/repo.git    -> https://github.com/org/repo.git
//   - git@github.com:org/repo.git                   -> git@github.com:org/repo.git (SSH, no change)
func sanitizeRemoteURL(rawURL string) string {
	// SSH URLs (git@host:path) don't contain credential tokens
	if strings.HasPrefix(rawURL, "git@") {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Remove userinfo (credentials) from the URL
	if parsed.User != nil {
		parsed.User = nil
	}

	return parsed.String()
}

// GenerateRootIDFromMultiPaths generates a deterministic root ID for a multi-path scan.
// It hashes the common root together with the sorted list of direct subfolder names,
// so different subsets of paths produce different IDs while the same set is stable.
// The subPaths must be the relative subfolder names (not full paths).
// If the common root is a git repo with a remote URL, the git-based identity is used
// as the base instead of the filesystem path, ensuring portability across machines.
func GenerateRootIDFromMultiPaths(commonRoot string, subPaths []string) string {
	// Sort subPaths for deterministic output regardless of argument order
	sorted := make([]string, len(subPaths))
	copy(sorted, subPaths)
	sort.Strings(sorted)

	// Try git-based identity first (portable across machines)
	gitInfo, repoRoot := GetGitInfoWithRoot(commonRoot)
	var base string
	if gitInfo != nil && gitInfo.RemoteURL != "" {
		base = normalizeRemoteURL(gitInfo.RemoteURL)
		// If commonRoot is a subdirectory of the repo, include the relative path
		if repoRoot != "" && repoRoot != commonRoot {
			rel, err := filepath.Rel(repoRoot, commonRoot)
			if err == nil && rel != "." {
				base += ":" + rel
			}
		}
	} else {
		// Fallback to absolute path
		base = filepath.Clean(commonRoot)
	}

	// Append sorted subfolder names
	content := base
	for _, sub := range sorted {
		content += ":" + sub
	}

	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:20]
}

// GenerateRootIDFromPath generates a deterministic root ID from absolute path
// Used when git repository is not available
func GenerateRootIDFromPath(basePath string) string {
	// Ensure we have an absolute, clean path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		absPath = filepath.Clean(basePath)
	}

	hash := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(hash[:])[:20]
}
