package parsers

import (
	"bufio"
	"strings"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// GradleTreeFileName is the pre-generated `gradle dependencies` output. Gradle
// build scripts do not state a resolved dependency graph, so -- like Maven's
// dependency-tree.json -- the operator/CI generates this file and the analyzer
// reads it without ever running gradle.
//
//	gradle dependencies --configuration runtimeClasspath > gradle-dependencies.txt
const GradleTreeFileName = "gradle-dependencies.txt"

// ParseGradleTreeGraph parses the pre-generated `gradle dependencies` tree and
// returns the package-to-package edges. It implements the GraphProducer
// contract.
//
// The output is an ASCII tree:
//
//	+--- org.springframework:spring-core:6.1.0
//	|    \--- org.springframework:spring-jcl:6.1.0
//	+--- com.google.guava:guava:32.1.3-jre -> 32.1.3-android
//
// Nodes are "group:artifact@version". Conflict-resolved versions ("a:b:1 -> 2")
// use the resolved version. Markers ((*), (c), (n)) are stripped. Indentation
// depth (each level is a 5-char connector) gives parent/child; top-level entries
// are direct dependencies of the configuration (from the synthetic "." node).
func ParseGradleTreeGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	seen := make(map[string]bool)
	// parents[depth] holds the node id at that tree depth.
	parents := map[int]string{-1: "."}

	scanner := bufio.NewScanner(strings.NewReader(string(input.Lockfile)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		depth, coord, ok := parseGradleTreeLine(line)
		if !ok {
			continue
		}
		node := gradleCoordNode(coord)
		if node == "" {
			continue
		}
		parents[depth] = node
		from := parents[depth-1]
		if from == "" {
			continue
		}

		switch input.Mode {
		case types.DependencyGraphDirect:
			if depth == 0 {
				addGradleEdge(&result.Edges, seen, ".", node)
			}
		case types.DependencyGraphFull:
			addGradleEdge(&result.Edges, seen, from, node)
		}
	}
	return result
}

// parseGradleTreeLine returns the tree depth (0-based) and the raw coordinate
// for a dependency line, or ok=false for non-dependency lines (headers, blank,
// "No dependencies", configuration names).
func parseGradleTreeLine(line string) (depth int, coord string, ok bool) {
	idx := strings.Index(line, "--- ")
	if idx < 0 {
		return 0, "", false
	}
	// The connector is "+--- " or "\--- "; the prefix before it is made of
	// "|    " / "     " segments, each 5 chars wide. Depth = prefix/5.
	// idx points at the start of "--- "; the marker char is at idx-1.
	marker := idx - 1
	if marker < 0 || (line[marker] != '+' && line[marker] != '\\') {
		return 0, "", false
	}
	depth = marker / 5
	coord = strings.TrimSpace(line[idx+4:])
	if coord == "" {
		return 0, "", false
	}
	return depth, coord, true
}

// gradleCoordNode converts a gradle coordinate line into a "group:artifact@version"
// node id, applying conflict resolution ("-> version") and stripping markers.
func gradleCoordNode(coord string) string {
	// Strip trailing markers like "(*)", "(c)", "(n)".
	if i := strings.Index(coord, " ("); i >= 0 {
		coord = coord[:i]
	}
	// Conflict resolution: "group:artifact:requested -> resolved".
	resolved := ""
	if i := strings.Index(coord, " -> "); i >= 0 {
		resolved = strings.TrimSpace(coord[i+4:])
		coord = strings.TrimSpace(coord[:i])
	}
	parts := strings.Split(coord, ":")
	switch len(parts) {
	case 3:
		group, artifact, version := parts[0], parts[1], parts[2]
		if resolved != "" {
			version = resolved
		}
		return group + ":" + artifact + "@" + version
	case 2:
		// "group:artifact -> resolved" (version only via resolution)
		if resolved != "" {
			return parts[0] + ":" + parts[1] + "@" + resolved
		}
	}
	return ""
}

// addGradleEdge appends an edge once (deduped).
func addGradleEdge(edges *[]types.DependencyEdge, seen map[string]bool, from, to string) {
	if from == to {
		return
	}
	key := from + "|" + to
	if seen[key] {
		return
	}
	seen[key] = true
	*edges = append(*edges, types.DependencyEdge{From: from, To: to})
}
