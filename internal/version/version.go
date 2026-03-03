// Package version provides build-time version information injected via ldflags.
package version

import "fmt"

// These variables are set at build time via ldflags:
//
//	go build -ldflags "-X github.com/petrarca/tech-stack-analyzer/internal/version.Version=v1.0.0 ..."
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Full returns a formatted version string including commit and build date.
func Full() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}
