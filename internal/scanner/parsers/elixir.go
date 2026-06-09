package parsers

import (
	"regexp"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ElixirParser handles Elixir mix.lock parsing. mix.lock is an Elixir map
// literal; entries look like:
//
//	"name": {:hex, :name, "version", "hash", [:mix], [{:dep, "~> 1.0", [..]}, ...], "hexpm", "hash2"},
//
// or for git deps:
//
//	"name": {:git, "https://...", "ref", [..]},
type ElixirParser struct{}

// NewElixirParser creates a new Elixir parser.
func NewElixirParser() *ElixirParser {
	return &ElixirParser{}
}

// mixEntryRe matches a top-level mix.lock entry: the quoted package name and
// the opening of its tuple. Used to locate entries; the tuple body is then
// scanned for the version and dependency block.
var mixEntryRe = regexp.MustCompile(`(?m)^\s*"([^"]+)":\s*\{`)

// mixHexVersionRe extracts the version from a :hex entry: the first quoted
// string after ":hex, :name,".
var mixHexVersionRe = regexp.MustCompile(`:hex,\s*:[A-Za-z0-9_]+,\s*"([^"]+)"`)

// mixDepRe matches a single dependency tuple inside the deps block:
// {:depname, "constraint", [hex: :depname, ...]}.
var mixDepRe = regexp.MustCompile(`\{:([A-Za-z0-9_]+),\s*"([^"]*)"`)

// mixLockEntry is a parsed package: name, resolved version (empty for git), and
// the names of its declared dependencies.
type mixLockEntry struct {
	Name         string
	Version      string
	Dependencies []string
}

// ParseMixLock parses mix.lock into resolved dependencies.
func (p *ElixirParser) ParseMixLock(content string) []types.Dependency {
	var deps []types.Dependency
	for _, e := range parseMixLockEntries(content) {
		if e.Version == "" {
			continue
		}
		deps = append(deps, types.Dependency{
			Type:       DependencyTypeElixir,
			Name:       e.Name,
			Version:    e.Version,
			SourceFile: "mix.lock",
		})
	}
	return deps
}

// parseMixLockEntries extracts every package entry with its version and
// dependency names from mix.lock.
func parseMixLockEntries(content string) []mixLockEntry {
	var entries []mixLockEntry
	locs := mixEntryRe.FindAllStringSubmatchIndex(content, -1)
	for i, loc := range locs {
		name := content[loc[2]:loc[3]]
		// The tuple body runs from the entry's '{' to the start of the next
		// entry (or end of file). Top-level entries are comma-separated.
		bodyStart := loc[1] - 1 // position of '{'
		bodyEnd := len(content)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		body := content[bodyStart:bodyEnd]

		entry := mixLockEntry{Name: name}
		if m := mixHexVersionRe.FindStringSubmatch(body); m != nil {
			entry.Version = m[1]
		}
		entry.Dependencies = parseMixDeps(body)
		entries = append(entries, entry)
	}
	return entries
}

// parseMixDeps extracts dependency names from the inner deps list of an entry.
// It scans the bracketed deps block (the list of {:dep, "constraint", [..]}
// tuples) to avoid matching the entry's own atoms.
func parseMixDeps(body string) []string {
	// The deps block is the list following the build-tools list, e.g.
	// "[:mix], [{:hpax, "~> 1.0", [...]}, ...]". Find the second top-level
	// bracket group; matching {:name, "..."} across the whole body is robust
	// enough because only dependency tuples take that shape.
	var deps []string
	seen := map[string]bool{}
	for _, m := range mixDepRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		// Skip the build-tool / source atoms that are not dependencies.
		switch name {
		case "hex", "git", "mix", "rebar", "rebar3", "make", "hexpm":
			continue
		}
		if !seen[name] {
			seen[name] = true
			deps = append(deps, name)
		}
	}
	return deps
}
