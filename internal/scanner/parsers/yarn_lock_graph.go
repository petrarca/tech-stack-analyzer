package parsers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// yarnEntry is a resolved yarn.lock entry: one or more declared specifiers
// (name@range) that resolve to a single locked version, plus that version's
// own dependency ranges.
type yarnEntry struct {
	specifiers []string          // e.g. ["lodash@^4.0.0", "lodash@^4.17.0"]
	version    string            // resolved version
	deps       map[string]string // name -> range from the dependencies block
}

// ParseYarnLockGraph parses yarn.lock and returns the package-to-package edges,
// honoring the requested graph mode. It implements the GraphProducer contract
// (ParseGraphFunc). Yarn entries declare dependency ranges; each range is
// resolved to its locked entry to form a clean "name@version" node.
//
// The flat dependency list is left empty here: ParseYarnLock requires the
// package.json (for direct-dep scoping) which the graph producer does not
// receive. The detector already populates payload.Dependencies separately.
func ParseYarnLockGraph(input GraphInput) LockGraph {
	var result LockGraph

	if input.Mode == types.DependencyGraphOff {
		return result
	}

	entries := parseYarnEntries(input.Lockfile)
	resolver := buildYarnResolver(entries)

	switch input.Mode {
	case types.DependencyGraphDirect:
		// Prefer package.json-declared direct deps (resolved via the lock's
		// specifier index); fall back to the not-referenced heuristic.
		if edges, ok := yarnDirectEdgesFromManifest(input.Manifest, resolver); ok {
			result.Edges = edges
		} else {
			result.Edges = yarnDirectEdges(entries, resolver)
		}
	case types.DependencyGraphFull:
		result.Edges = yarnFullEdges(entries, resolver)
	}
	return result
}

// yarnDirectEdgesFromManifest builds root -> direct edges from package.json's
// declared dependencies, resolving each (name, range) to its locked version via
// the yarn specifier index. The bool return is true when a manifest was
// successfully parsed (even if it declares no dependencies); false means "no
// manifest, fall back to the heuristic" (F-08: replaces fragile nil-sentinel).
func yarnDirectEdgesFromManifest(manifest []byte, resolver yarnResolver) ([]types.DependencyEdge, bool) {
	if len(manifest) == 0 {
		return nil, false
	}
	var pkg struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(manifest, &pkg); err != nil {
		return nil, false
	}
	var edges []types.DependencyEdge
	add := func(deps map[string]string, scope string) {
		for name, rng := range deps {
			if to := resolver.yarnResolve(name, rng); to != "" {
				edges = append(edges, types.DependencyEdge{From: ".", To: to, Scope: scope})
			}
		}
	}
	add(pkg.Dependencies, types.ScopeProd)
	add(pkg.DevDependencies, types.ScopeDev)
	add(pkg.OptionalDependencies, types.ScopeOptional)
	return edges, true
}

// yarnSpecKeyRe splits a yarn entry header key into name and range, e.g.
// "lodash@^4.17.0" or "@babel/core@^7.0.0".
var yarnSpecKeyRe = regexp.MustCompile(`^((?:@[^/]+/)?[^@]+)@(.+)$`)

// yarnResolver maps a declared specifier "name@range" to its locked version.
type yarnResolver map[string]string

// buildYarnResolver indexes every specifier of every entry to its resolved
// version so dependency ranges can be looked up.
func buildYarnResolver(entries []yarnEntry) yarnResolver {
	r := make(yarnResolver)
	for _, e := range entries {
		for _, spec := range e.specifiers {
			r[spec] = e.version
		}
	}
	return r
}

// yarnResolve resolves a dependency name+range to its locked "name@version"
// node via the resolver. Returns "" when unresolved.
func (r yarnResolver) yarnResolve(name, rng string) string {
	if v, ok := r[name+"@"+rng]; ok {
		return name + "@" + v
	}
	return ""
}

// yarnFullEdges builds every package -> dependency edge stated by yarn.lock.
func yarnFullEdges(entries []yarnEntry, resolver yarnResolver) []types.DependencyEdge {
	var edges []types.DependencyEdge
	for _, e := range entries {
		from := yarnEntryName(e) + "@" + e.version
		if yarnEntryName(e) == "" || e.version == "" {
			continue
		}
		for name, rng := range e.deps {
			if to := resolver.yarnResolve(name, rng); to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			}
		}
	}
	return edges
}

// yarnDirectEdges builds root -> direct-dependency edges. Yarn classic does not
// embed the root manifest, so a root is approximated as any entry not appearing
// as a resolved dependency target of another entry. The synthetic "." marker is
// the from node.
func yarnDirectEdges(entries []yarnEntry, resolver yarnResolver) []types.DependencyEdge {
	referenced := make(map[string]bool)
	for _, e := range entries {
		for name, rng := range e.deps {
			if to := resolver.yarnResolve(name, rng); to != "" {
				referenced[to] = true
			}
		}
	}
	var edges []types.DependencyEdge
	for _, e := range entries {
		node := yarnEntryName(e) + "@" + e.version
		if referenced[node] || yarnEntryName(e) == "" || e.version == "" {
			continue
		}
		edges = append(edges, types.DependencyEdge{From: ".", To: node})
	}
	return edges
}

// yarnEntryName returns the package name shared by an entry's specifiers.
func yarnEntryName(e yarnEntry) string {
	if len(e.specifiers) == 0 {
		return ""
	}
	if m := yarnSpecKeyRe.FindStringSubmatch(e.specifiers[0]); len(m) > 2 {
		return m[1]
	}
	return ""
}

// parseYarnEntries parses yarn.lock (classic format) into resolved entries with
// their specifiers, version, and dependency ranges.
func parseYarnEntries(content []byte) []yarnEntry {
	var entries []yarnEntry
	lines := strings.Split(string(content), "\n")

	var current *yarnEntry
	inDeps := false

	flush := func() {
		if current != nil && current.version != "" {
			entries = append(entries, *current)
		}
		current = nil
		inDeps = false
	}

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}

		// Top-level entry header: not indented, ends with ":".
		if !strings.HasPrefix(raw, " ") && strings.HasSuffix(strings.TrimSpace(raw), ":") {
			flush()
			current = &yarnEntry{deps: map[string]string{}, specifiers: parseYarnHeaderSpecifiers(raw)}
			continue
		}
		if current == nil {
			continue
		}

		trimmed := strings.TrimSpace(raw)

		// The dependencies sub-block.
		if trimmed == "dependencies:" {
			inDeps = true
			continue
		}
		// optionalDependencies and other sub-blocks end the deps block.
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			inDeps = false
		}

		if strings.HasPrefix(trimmed, "version ") || strings.HasPrefix(trimmed, `version:`) {
			current.version = parseYarnVersionField(trimmed)
			continue
		}

		if inDeps {
			if name, rng := parseYarnDepLine(trimmed); name != "" {
				current.deps[name] = rng
			}
		}
	}
	flush()
	return entries
}

// parseYarnHeaderSpecifiers splits an entry header into its name@range
// specifiers. A header may list several comma-separated specifiers, each
// optionally quoted, e.g. `lodash@^4.0.0, lodash@^4.17.0:`.
func parseYarnHeaderSpecifiers(line string) []string {
	line = strings.TrimSuffix(strings.TrimSpace(line), ":")
	var specs []string
	for _, part := range strings.Split(line, ",") {
		spec := strings.Trim(strings.TrimSpace(part), `"`)
		// Drop the "npm:" protocol prefix in the range, if present.
		spec = strings.Replace(spec, "@npm:", "@", 1)
		if spec != "" {
			specs = append(specs, spec)
		}
	}
	return specs
}

// parseYarnVersionField extracts the version value from a `version "x.y.z"` or
// `version: x.y.z` line.
func parseYarnVersionField(line string) string {
	line = strings.TrimPrefix(line, "version")
	line = strings.TrimPrefix(strings.TrimSpace(line), ":")
	return strings.Trim(strings.TrimSpace(line), `"`)
}

// parseYarnDepLine parses a dependency line inside a dependencies block, e.g.
// `lodash "^4.17.0"` or `"@babel/core" "^7.0.0"`.
func parseYarnDepLine(line string) (name, rng string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	// Name may be quoted (scoped packages).
	var rest string
	if strings.HasPrefix(line, `"`) {
		end := strings.Index(line[1:], `"`)
		if end < 0 {
			return "", ""
		}
		name = line[1 : 1+end]
		rest = strings.TrimSpace(line[1+end+1:])
	} else {
		fields := strings.SplitN(line, " ", 2)
		name = fields[0]
		if len(fields) > 1 {
			rest = strings.TrimSpace(fields[1])
		}
	}
	rng = strings.Trim(rest, `"`)
	rng = strings.Replace(rng, "npm:", "", 1)
	return name, rng
}
