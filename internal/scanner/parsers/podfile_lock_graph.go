package parsers

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// podfileLock is the YAML view of Podfile.lock. PODS lists each pod as either a
// bare string ("Name (version)", no deps) or a single-key map
// ("Name (version)": ["Dep (constraint)", ...]). DEPENDENCIES lists the direct
// pods ("Name (~> 1.0)").
type podfileLock struct {
	Pods         []any    `yaml:"PODS"`
	Dependencies []string `yaml:"DEPENDENCIES"`
}

// ParsePodfileLockGraph parses Podfile.lock and returns the package-to-package
// edges, honoring the requested graph mode. It implements the GraphProducer
// contract.
//
// Pod names may carry a subspec ("Alamofire/Core"); the node is the root pod
// ("Alamofire") at its locked version. Versions come from the PODS section
// (each pod is single-versioned). DEPENDENCIES gives the direct pods.
func ParsePodfileLockGraph(input GraphInput) LockGraph {
	var result LockGraph
	if input.Mode == types.DependencyGraphOff {
		return result
	}

	var lock podfileLock
	if err := yaml.Unmarshal(input.Lockfile, &lock); err != nil {
		return result
	}

	versionByPod, edges := parsePodfilePods(lock.Pods)
	node := func(podRef string) string {
		name := podRootName(podRef)
		if v, ok := versionByPod[name]; ok {
			return name + "@" + v
		}
		return ""
	}

	switch input.Mode {
	case types.DependencyGraphDirect:
		seen := map[string]bool{}
		for _, dep := range lock.Dependencies {
			to := node(dep)
			if to != "" && !seen[to] {
				seen[to] = true
				result.Edges = append(result.Edges, types.DependencyEdge{From: ".", To: to})
			}
		}
	case types.DependencyGraphFull:
		seen := map[string]bool{}
		for _, e := range edges {
			from, to := node(e[0]), node(e[1])
			if from == "" || to == "" || from == to {
				continue
			}
			key := from + "|" + to
			if seen[key] {
				continue
			}
			seen[key] = true
			result.Edges = append(result.Edges, types.DependencyEdge{From: from, To: to})
		}
	}
	return result
}

// parsePodfilePods walks the PODS section, returning the root-pod -> version map
// and the raw (from, to) name pairs (subspec-qualified) for edge building.
func parsePodfilePods(pods []any) (versionByPod map[string]string, edges [][2]string) {
	versionByPod = make(map[string]string)
	record := func(podRef string) {
		name := podRootName(podRef)
		if v := podVersion(podRef); name != "" && v != "" {
			versionByPod[name] = v
		}
	}
	for _, item := range pods {
		switch v := item.(type) {
		case string:
			record(v)
		case map[string]any:
			for podRef, rawDeps := range v {
				record(podRef)
				deps, _ := rawDeps.([]any)
				for _, d := range deps {
					if ds, ok := d.(string); ok {
						edges = append(edges, [2]string{podRef, ds})
					}
				}
			}
		}
	}
	return versionByPod, edges
}

// podRootName extracts the root pod name from a pod reference, dropping any
// subspec ("Alamofire/Core" -> "Alamofire") and version/constraint suffix
// ("Alamofire (5.8.0)" -> "Alamofire").
func podRootName(ref string) string {
	name := strings.TrimSpace(ref)
	if i := strings.IndexByte(name, '('); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name = name[:i]
	}
	return name
}

// podVersion extracts the locked version from a PODS entry "Name (5.8.0)".
// Returns "" for dependency constraints (e.g. "~> 5.0") which are not exact.
func podVersion(ref string) string {
	open := strings.IndexByte(ref, '(')
	closeIdx := strings.IndexByte(ref, ')')
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return ""
	}
	ver := strings.TrimSpace(ref[open+1 : closeIdx])
	// Exact versions only (PODS section); skip operators like "~>", ">=".
	if strings.ContainsAny(ver, "~><= ") {
		return ""
	}
	return ver
}
