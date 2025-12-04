package git

import (
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
func GetGitInfo(path string) *GitInfo {
	// Open the repository
	repo, err := git.PlainOpen(path)
	if err != nil {
		// Not a git repository or error opening it
		return nil
	}

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

	// Get worktree status to check if dirty
	worktree, err := repo.Worktree()
	if err == nil {
		status, err := worktree.Status()
		if err == nil {
			gitInfo.IsDirty = !status.IsClean()
		}
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

	return gitInfo
}
