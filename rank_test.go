package posit

import (
	"math/rand"
	"testing"
)

func TestRank_LinearChain(t *testing.T) {
	// A → B → C should have ranks 0, 1, 2
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if state.nodes["B"].rank != 1 {
		t.Errorf("Expected B at rank 1, got %d", state.nodes["B"].rank)
	}
	if state.nodes["C"].rank != 2 {
		t.Errorf("Expected C at rank 2, got %d", state.nodes["C"].rank)
	}
}

func TestRank_Diamond(t *testing.T) {
	// A → (B, C) → D should have A:0, B:1, C:1, D:2
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if state.nodes["B"].rank != 1 {
		t.Errorf("Expected B at rank 1, got %d", state.nodes["B"].rank)
	}
	if state.nodes["C"].rank != 1 {
		t.Errorf("Expected C at rank 1, got %d", state.nodes["C"].rank)
	}
	if state.nodes["D"].rank != 2 {
		t.Errorf("Expected D at rank 2, got %d", state.nodes["D"].rank)
	}
}

func TestRank_DisconnectedComponents(t *testing.T) {
	// Two separate chains should both start at rank 0
	g := NewGraph()
	// Chain 1
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	// Chain 2
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("X", "Y")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// Both roots should be at rank 0
	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if state.nodes["X"].rank != 0 {
		t.Errorf("Expected X at rank 0, got %d", state.nodes["X"].rank)
	}
	// Sinks should be at rank 1
	if state.nodes["B"].rank != 1 {
		t.Errorf("Expected B at rank 1, got %d", state.nodes["B"].rank)
	}
	if state.nodes["Y"].rank != 1 {
		t.Errorf("Expected Y at rank 1, got %d", state.nodes["Y"].rank)
	}
}

func TestRank_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if len(state.layers) != 1 {
		t.Errorf("Expected 1 layer, got %d", len(state.layers))
	}
}

func TestRank_EmptyGraph(t *testing.T) {
	g := NewGraph()

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	if len(state.layers) != 0 {
		t.Errorf("Expected 0 layers for empty graph, got %d", len(state.layers))
	}
}

func TestRank_LayersStructure(t *testing.T) {
	// Verify layers array is correctly populated
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// Should have 3 layers
	if len(state.layers) != 3 {
		t.Errorf("Expected 3 layers, got %d", len(state.layers))
	}

	// Layer 0 should have A
	if len(state.layers[0]) != 1 || state.layers[0][0] != "A" {
		t.Errorf("Expected layer 0 to have [A], got %v", state.layers[0])
	}

	// Layer 1 should have B and C (sorted)
	if len(state.layers[1]) != 2 {
		t.Errorf("Expected layer 1 to have 2 nodes, got %d", len(state.layers[1]))
	}

	// Layer 2 should have D
	if len(state.layers[2]) != 1 || state.layers[2][0] != "D" {
		t.Errorf("Expected layer 2 to have [D], got %v", state.layers[2])
	}
}

func TestRank_EdgeConstraintViolation(t *testing.T) {
	// Verify all edges satisfy rank constraints
	g := buildRandomDAG(50, 75)

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	for key, edge := range state.edges {
		fromRank := state.nodes[key.from].rank
		toRank := state.nodes[key.to].rank
		minlen := edge.minlen
		if minlen == 0 {
			minlen = 1
		}

		if toRank-fromRank < minlen {
			t.Errorf("Edge %s->%s violates constraint: rank diff %d < minlen %d",
				key.from, key.to, toRank-fromRank, minlen)
		}
	}
}

func TestRank_WideGraph(t *testing.T) {
	// A → (B, C, D, E, F) → G
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("G", NodeOptions{Width: 100, Height: 50})

	middleNodes := []string{"B", "C", "D", "E", "F"}
	for _, id := range middleNodes {
		g.AddNode(id, NodeOptions{Width: 100, Height: 50})
		g.MustAddEdge("A", id)
		g.MustAddEdge(id, "G")
	}

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// A should be at rank 0
	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}

	// All middle nodes should be at rank 1
	for _, id := range middleNodes {
		if state.nodes[id].rank != 1 {
			t.Errorf("Expected %s at rank 1, got %d", id, state.nodes[id].rank)
		}
	}

	// G should be at rank 2
	if state.nodes["G"].rank != 2 {
		t.Errorf("Expected G at rank 2, got %d", state.nodes["G"].rank)
	}
}

func TestRank_LongChain(t *testing.T) {
	// A → B → C → D → E → F
	g := NewGraph()
	nodes := []string{"A", "B", "C", "D", "E", "F"}

	for _, id := range nodes {
		g.AddNode(id, NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < len(nodes)-1; i++ {
		g.MustAddEdge(nodes[i], nodes[i+1])
	}

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// Each node should be at its index rank
	for i, id := range nodes {
		if state.nodes[id].rank != i {
			t.Errorf("Expected %s at rank %d, got %d", id, i, state.nodes[id].rank)
		}
	}
}

func TestRank_MultipleRoots(t *testing.T) {
	// A → C, B → C (both A and B are roots)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// Both roots should be at rank 0
	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if state.nodes["B"].rank != 0 {
		t.Errorf("Expected B at rank 0, got %d", state.nodes["B"].rank)
	}
	// C should be at rank 1
	if state.nodes["C"].rank != 1 {
		t.Errorf("Expected C at rank 1, got %d", state.nodes["C"].rank)
	}
}

func TestRank_AfterCycleRemoval(t *testing.T) {
	// A → B → C → A (cycle) should still get valid ranks after cycle removal
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "A") // Creates cycle

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	// All ranks should be non-negative
	for id, node := range state.nodes {
		if node.rank < 0 {
			t.Errorf("Node %s has negative rank: %d", id, node.rank)
		}
	}

	// Edge constraints should be satisfied (for edges in current direction)
	for key, edge := range state.edges {
		fromRank := state.nodes[key.from].rank
		toRank := state.nodes[key.to].rank
		minlen := edge.minlen
		if minlen == 0 {
			minlen = 1
		}

		if toRank-fromRank < minlen {
			t.Errorf("Edge %s->%s violates constraint: rank diff %d < minlen %d",
				key.from, key.to, toRank-fromRank, minlen)
		}
	}
}

// buildRandomDAG creates a random DAG for testing.
func buildRandomDAG(numNodes, numEdges int) *Graph {
	g := NewGraph()

	// Create nodes
	for i := 0; i < numNodes; i++ {
		id := string(rune('A' + (i % 26)))
		if i >= 26 {
			id = id + string(rune('0'+(i/26)))
		}
		g.AddNode(id, NodeOptions{Width: 100, Height: 50})
	}

	// Get node IDs in a slice for random access
	nodeIDs := make([]string, 0, numNodes)
	for id := range g.nodes {
		nodeIDs = append(nodeIDs, id)
	}

	// Create random edges (only forward edges to ensure DAG)
	edgeCount := 0
	for edgeCount < numEdges && edgeCount < numNodes*(numNodes-1)/2 {
		i := rand.Intn(numNodes)
		j := rand.Intn(numNodes)
		if i < j { // Only forward edges
			from := nodeIDs[i]
			to := nodeIDs[j]
			// Check if edge already exists
			exists := false
			for _, e := range g.edges {
				if e.from == from && e.to == to {
					exists = true
					break
				}
			}
			if !exists {
				g.MustAddEdge(from, to)
				edgeCount++
			}
		}
	}

	return g
}
