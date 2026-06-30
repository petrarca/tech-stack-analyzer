package parsers

import (
	"bufio"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// ParseGemfileLockGraph parses Gemfile.lock and returns the package-to-package
// edges, honoring the requested graph mode. It implements the GraphProducer
// contract (ParseGraphFunc).
//
// Gemfile.lock GEM section lists each gem at 4-space indent ("name (version)")
// and its dependencies at 6-space indent ("depname (constraint)"). The
// DEPENDENCIES section lists the direct dependencies. Every gem is locked at a
// single version, so dep names resolve cleanly to "name@version".
func ParseGemfileLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	versionByName, depsByName, directNames := parseGemfileLockGraph(input.Lockfile)

	node := func(name string) string {
		if v, ok := versionByName[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		for _, name := range directNames {
			if to := node(name); to != "" {
				result.Edges = append(result.Edges, types.DependencyEdge{From: ".", To: to})
			}
		}
	case types.DependencyGraphFull:
		result.Edges, result.Unresolved = gemfileLockFullEdges(depsByName, node)
	}
	return result
}

// gemfileLockFullEdges builds the transitive gem-to-gem edges, reporting
// dependencies that resolve to no known node.
func gemfileLockFullEdges(depsByName map[string][]string, node func(string) string) (edges []types.DependencyEdge, unresolved []string) {
	for name, deps := range depsByName {
		from := node(name)
		if from == "" {
			continue
		}
		for _, dep := range deps {
			if to := node(dep); to != "" {
				edges = append(edges, types.DependencyEdge{From: from, To: to})
			} else {
				unresolved = append(unresolved, from+" -> "+dep)
			}
		}
	}
	return edges, unresolved
}

// parseGemfileLockGraph extracts the gem -> version map, the per-gem dependency
// names, and the direct dependency names from Gemfile.lock.
func parseGemfileLockGraph(content []byte) (versionByName map[string]string, depsByName map[string][]string, direct []string) {
	versionByName = make(map[string]string)
	depsByName = make(map[string][]string)

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var current string
	inDependencies := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// The DEPENDENCIES section lists direct deps (2-space indent).
		if trimmed == "DEPENDENCIES" {
			inDependencies = true
			current = ""
			continue
		}
		if inDependencies {
			// A new top-level section header ends DEPENDENCIES.
			if line != "" && !strings.HasPrefix(line, " ") {
				inDependencies = false
			} else if trimmed != "" {
				name := strings.Fields(trimmed)[0]
				name = strings.TrimSuffix(name, "!") // bang = pinned source
				direct = append(direct, name)
				continue
			}
		}

		switch countLeadingSpaces(line) {
		case 4:
			// "name (version)"
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				current = ""
				continue
			}
			name := fields[0]
			version := strings.Trim(fields[1], "()")
			version = strings.SplitN(version, "-", 2)[0] // drop platform suffix
			versionByName[name] = version
			current = name
		case 6:
			// "depname (constraint)" under the current gem
			if current == "" {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) == 0 {
				continue
			}
			depsByName[current] = append(depsByName[current], fields[0])
		}
	}
	return versionByName, depsByName, direct
}

// countLeadingSpaces returns the number of leading space characters.
func countLeadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}
