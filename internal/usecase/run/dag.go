package run

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// ResolveSetupDAG calculates the transitive closure of required setups for a scenario,
// validates that all referenced setups exist, detects circular dependencies (cycles),
// and returns a topologically sorted list of setup names (dependencies before dependees).
func ResolveSetupDAG(scenarioSetups []string, allSetups map[string]domain.Setup) ([]string, error) {
	if len(scenarioSetups) == 0 {
		return nil, nil
	}

	// 1. Validate that all directly requested setups exist in suite
	for _, sName := range scenarioSetups {
		if _, ok := allSetups[sName]; !ok {
			return nil, fmt.Errorf("setup %q required by scenario is not defined in suite.setup", sName)
		}
	}

	// 2. Discover all reachable setups (Transitive Closure)
	visited := make(map[string]bool)
	var discoverDeps func(name string) error
	discoverDeps = func(name string) error {
		setup, ok := allSetups[name]
		if !ok {
			return fmt.Errorf("dependency setup %q is not defined in suite.setup", name)
		}
		visited[name] = true
		for _, dep := range setup.Use {
			if !visited[dep] {
				if err := discoverDeps(dep); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, sName := range scenarioSetups {
		if !visited[sName] {
			if err := discoverDeps(sName); err != nil {
				return nil, err
			}
		}
	}

	// 3. Cycle Detection and Topological Sort using DFS
	// 0: Unvisited (White), 1: Visiting (Gray), 2: Visited (Black)
	state := make(map[string]int)
	var sorted []string
	var path []string

	var visit func(node string) error
	visit = func(node string) error {
		state[node] = 1 // Visiting
		path = append(path, node)

		setup := allSetups[node]
		// Sort dependencies alphabetically for deterministic ordering
		deps := make([]string, len(setup.Use))
		copy(deps, setup.Use)
		sort.Strings(deps)

		for _, dep := range deps {
			if state[dep] == 1 {
				// Cycle detected
				cycleStart := 0
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}
				cycleNodes := append(path[cycleStart:], dep)
				return fmt.Errorf("cyclic dependency detected in setup: %s", strings.Join(cycleNodes, " -> "))
			}
			if state[dep] == 0 {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		path = path[:len(path)-1]
		state[node] = 2 // Visited
		sorted = append(sorted, node)
		return nil
	}

	// Get all reachable nodes and sort them for deterministic iteration
	reachable := make([]string, 0, len(visited))
	for name := range visited {
		reachable = append(reachable, name)
	}
	sort.Strings(reachable)

	for _, node := range reachable {
		if state[node] == 0 {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	return sorted, nil
}
