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
// gemLockGraphState holds the mutable parse state for a Gemfile.lock graph scan.
type gemLockGraphState struct {
	versionByName  map[string]string
	depsByName     map[string][]string
	direct         []string
	current        string
	inDependencies bool
}

func parseGemfileLockGraph(content []byte) (versionByName map[string]string, depsByName map[string][]string, direct []string) {
	st := &gemLockGraphState{
		versionByName: make(map[string]string),
		depsByName:    make(map[string][]string),
	}
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "DEPENDENCIES" {
			st.inDependencies = true
			st.current = ""
			continue
		}
		if st.inDependencies && st.consumeDependencyLine(line, trimmed) {
			continue
		}
		st.consumeSpecLine(line, trimmed)
	}
	return st.versionByName, st.depsByName, st.direct
}

// consumeDependencyLine handles a line while in the DEPENDENCIES section. It
// returns true when the line was consumed (a direct dep). A top-level header
// ends the section and is left for the spec parser.
func (st *gemLockGraphState) consumeDependencyLine(line, trimmed string) bool {
	if line != "" && !strings.HasPrefix(line, " ") {
		st.inDependencies = false
		return false
	}
	if trimmed == "" {
		return false
	}
	name := strings.TrimSuffix(strings.Fields(trimmed)[0], "!") // bang = pinned source
	st.direct = append(st.direct, name)
	return true
}

// consumeSpecLine handles the GEM specs: a 4-space "name (version)" line sets
// the current gem and its version; a 6-space line adds a dependency edge.
func (st *gemLockGraphState) consumeSpecLine(line, trimmed string) {
	switch countLeadingSpaces(line) {
	case 4:
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			st.current = ""
			return
		}
		version := strings.Trim(fields[1], "()")
		version = strings.SplitN(version, "-", 2)[0] // drop platform suffix
		st.versionByName[fields[0]] = version
		st.current = fields[0]
	case 6:
		if st.current == "" {
			return
		}
		if fields := strings.Fields(trimmed); len(fields) > 0 {
			st.depsByName[st.current] = append(st.depsByName[st.current], fields[0])
		}
	}
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
