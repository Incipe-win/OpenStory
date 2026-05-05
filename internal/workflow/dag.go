package workflow

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// DAG validation errors.
var (
	ErrCycleDetected    = errors.New("cycle detected in workflow DAG")
	ErrOrphanNode       = errors.New("node referenced by edge does not exist")
	ErrSelfLoop         = errors.New("self-loop detected: edge source equals target")
	ErrInvalidNodeType  = errors.New("invalid node type")
	ErrEmptyWorkflow    = errors.New("workflow has no nodes")
)

// ValidateDAG checks that the workflow graph is a valid DAG:
//   - No cycles
//   - No self-loops
//   - All edge references point to existing nodes
//   - All node types are valid
//
// Returns nil if the DAG is valid, or the first error found.
func ValidateDAG(nodes []Node, edges []Edge) error {
	if len(nodes) == 0 {
		return ErrEmptyWorkflow
	}

	// Validate node types
	nodeSet := make(map[uuid.UUID]bool, len(nodes))
	for _, n := range nodes {
		if !ValidNodeType(string(n.Type)) {
			return fmt.Errorf("%w: %s", ErrInvalidNodeType, n.Type)
		}
		nodeSet[n.ID] = true
	}

	// Validate edges
	for _, e := range edges {
		if e.SourceNodeID == e.TargetNodeID {
			return fmt.Errorf("%w: node %s", ErrSelfLoop, e.SourceNodeID)
		}
		if !nodeSet[e.SourceNodeID] {
			return fmt.Errorf("%w: source %s", ErrOrphanNode, e.SourceNodeID)
		}
		if !nodeSet[e.TargetNodeID] {
			return fmt.Errorf("%w: target %s", ErrOrphanNode, e.TargetNodeID)
		}
	}

	// Build adjacency list
	adj := make(map[uuid.UUID][]uuid.UUID, len(nodes))
	for _, n := range nodes {
		adj[n.ID] = nil // ensure all nodes appear
	}
	for _, e := range edges {
		adj[e.SourceNodeID] = append(adj[e.SourceNodeID], e.TargetNodeID)
	}

	// Cycle detection using DFS with 3-color marking
	// 0=white (unvisited), 1=gray (in progress), 2=black (done)
	color := make(map[uuid.UUID]int, len(nodes))

	var dfs func(u uuid.UUID) bool
	dfs = func(u uuid.UUID) bool {
		color[u] = 1 // gray
		for _, v := range adj[u] {
			if color[v] == 1 {
				return true // back edge → cycle
			}
			if color[v] == 0 && dfs(v) {
				return true
			}
		}
		color[u] = 2 // black
		return false
	}

	for _, n := range nodes {
		if color[n.ID] == 0 {
			if dfs(n.ID) {
				return ErrCycleDetected
			}
		}
	}

	return nil
}

// TopologicalSort returns nodes in topological order using Kahn's algorithm.
// Returns an error if the graph contains a cycle.
func TopologicalSort(nodes []Node, edges []Edge) ([]Node, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeMap := make(map[uuid.UUID]*Node, len(nodes))
	inDegree := make(map[uuid.UUID]int, len(nodes))
	adj := make(map[uuid.UUID][]uuid.UUID, len(nodes))

	for i := range nodes {
		id := nodes[i].ID
		nodeMap[id] = &nodes[i]
		inDegree[id] = 0
		adj[id] = nil
	}

	for _, e := range edges {
		adj[e.SourceNodeID] = append(adj[e.SourceNodeID], e.TargetNodeID)
		inDegree[e.TargetNodeID]++
	}

	// Collect nodes with in-degree 0
	queue := make([]uuid.UUID, 0)
	for _, n := range nodes {
		if inDegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	sorted := make([]Node, 0, len(nodes))
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		sorted = append(sorted, *nodeMap[u])

		for _, v := range adj[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if len(sorted) != len(nodes) {
		return nil, ErrCycleDetected
	}

	return sorted, nil
}
