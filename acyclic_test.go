package posit

import "testing"

func TestAcyclic_SimpleDAG(t *testing.T) {
	// A → B → C (no cycles)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if len(state.reversedEdges) != 0 {
		t.Errorf("Expected no reversed edges for DAG, got %d", len(state.reversedEdges))
	}

	if hasCycle(state) {
		t.Error("Graph should remain acyclic")
	}
}

func TestAcyclic_SingleCycle(t *testing.T) {
	// A → B → C → A (one cycle)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "A") // Creates cycle

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if len(state.reversedEdges) != 1 {
		t.Errorf("Expected 1 reversed edge, got %d", len(state.reversedEdges))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}
}

func TestAcyclic_TwoNodeCycle(t *testing.T) {
	// A → B → A (simple two-node cycle)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "A")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if len(state.reversedEdges) != 1 {
		t.Errorf("Expected 1 reversed edge, got %d", len(state.reversedEdges))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}
}

func TestAcyclic_MultipleCycles(t *testing.T) {
	// Two independent cycles
	g := NewGraph()
	// Cycle 1: A → B → A
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "A")

	// Cycle 2: X → Y → X
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("X", "Y")
	g.MustAddEdge("Y", "X")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if len(state.reversedEdges) < 2 {
		t.Errorf("Expected at least 2 reversed edges, got %d", len(state.reversedEdges))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}
}

func TestAcyclic_SelfLoop(t *testing.T) {
	// A → A (self-loop)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "A")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	// Self-loop should be removed, not reversed
	if len(state.edges) != 0 {
		t.Errorf("Expected self-loop to be removed, got %d edges", len(state.edges))
	}
}

func TestAcyclic_DisconnectedComponents(t *testing.T) {
	// Component 1: A → B (no cycle)
	// Component 2: X → Y → X (cycle)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("X", "Y")
	g.MustAddEdge("Y", "X")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if len(state.reversedEdges) != 1 {
		t.Errorf("Expected 1 reversed edge, got %d", len(state.reversedEdges))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}
}

func TestAcyclic_ComplexGraph(t *testing.T) {
	// A → B → C
	// ↓   ↓   ↓
	// D ← E → F
	// ↓       ↑
	// └───────┘
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddNode("E", NodeOptions{Width: 100, Height: 50})
	g.AddNode("F", NodeOptions{Width: 100, Height: 50})

	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("A", "D")
	g.MustAddEdge("B", "E")
	g.MustAddEdge("C", "F")
	g.MustAddEdge("E", "D")
	g.MustAddEdge("E", "F")
	g.MustAddEdge("D", "F") // D → F → (back to something creating cycle)
	g.MustAddEdge("F", "A") // Creates cycle: A → B → C → F → A

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}

	// At least one edge should be reversed to break the cycle
	if len(state.reversedEdges) < 1 {
		t.Errorf("Expected at least 1 reversed edge, got %d", len(state.reversedEdges))
	}
}

func TestAcyclic_ReversedEdgeMarked(t *testing.T) {
	// A → B → A
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "A")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	// Find the reversed edge
	var foundReversed bool
	for _, edge := range state.edges {
		if edge.reversed {
			foundReversed = true
			break
		}
	}

	if !foundReversed {
		t.Error("Expected one edge to be marked as reversed")
	}
}

func TestAcyclic_AdjacencyListsConsistent(t *testing.T) {
	// A → B → C → A
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "A")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()

	// Verify adjacency lists match edges
	for key := range state.edges {
		// Check successor list contains the target
		found := false
		for _, succ := range state.successors[key.from] {
			if succ == key.to {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Edge %s→%s not in successors list", key.from, key.to)
		}

		// Check predecessor list contains the source
		found = false
		for _, pred := range state.predecessors[key.to] {
			if pred == key.from {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Edge %s→%s not in predecessors list", key.from, key.to)
		}
	}
}

// hasCycle checks if the graph contains any cycles using DFS.
func hasCycle(s *layoutState) bool {
	visited := make(map[string]bool)
	onStack := make(map[string]bool)

	var dfs func(v string) bool
	dfs = func(v string) bool {
		if onStack[v] {
			return true // cycle found
		}
		if visited[v] {
			return false
		}

		visited[v] = true
		onStack[v] = true

		for _, w := range s.successors[v] {
			if dfs(w) {
				return true
			}
		}

		delete(onStack, v)
		return false
	}

	for id := range s.nodes {
		if dfs(id) {
			return true
		}
	}
	return false
}
