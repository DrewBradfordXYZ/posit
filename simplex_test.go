package posit

import (
	"testing"
)

func TestNetworkSimplex_Linear(t *testing.T) {
	// A → B → C should still be ranks 0, 1, 2
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
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

func TestNetworkSimplex_Diamond(t *testing.T) {
	// Diamond: A → (B, C) → D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
	state.makeAcyclic()
	state.assignLayers()

	// Network simplex should produce optimal 3 layers (0, 1, 2)
	maxRank := 0
	for _, node := range state.nodes {
		if node.rank > maxRank {
			maxRank = node.rank
		}
	}

	if maxRank != 2 {
		t.Errorf("Expected max rank 2, got %d", maxRank)
	}

	// Verify edge constraints
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

func TestNetworkSimplex_EdgeConstraints(t *testing.T) {
	// Test that all edges satisfy rank constraints
	g := buildRandomDAG(30, 50)

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
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

func TestNetworkSimplex_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
	state.makeAcyclic()
	state.assignLayers()

	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
}

func TestNetworkSimplex_TwoNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
	state.makeAcyclic()
	state.assignLayers()

	if state.nodes["A"].rank != 0 {
		t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
	}
	if state.nodes["B"].rank != 1 {
		t.Errorf("Expected B at rank 1, got %d", state.nodes["B"].rank)
	}
}

func TestNetworkSimplex_EmptyGraph(t *testing.T) {
	g := NewGraph()

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
	state.makeAcyclic()
	state.assignLayers()

	if len(state.layers) != 0 {
		t.Errorf("Expected 0 layers for empty graph, got %d", len(state.layers))
	}
}

func TestNetworkSimplex_DisconnectedComponents(t *testing.T) {
	// Two separate chains
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("X", "Y")

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
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

func TestNetworkSimplex_WideGraph(t *testing.T) {
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

	state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
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

func TestNetworkSimplex_MinimizesEdgeLength(t *testing.T) {
	// Compare total edge length between algorithms
	// Network simplex with fallback should produce results at least as good as LP

	// Build graph once and use for both
	g := buildRandomDAG(20, 35)

	lpState := newLayoutState(g, Options{Algorithm: LongestPath})
	lpState.makeAcyclic()
	lpState.assignLayers()
	lpLength := totalEdgeLength(lpState)

	// Use same graph for NS
	nsState := newLayoutState(g, Options{Algorithm: NetworkSimplex})
	nsState.makeAcyclic()
	nsState.assignLayers()
	nsLength := totalEdgeLength(nsState)

	// Network simplex should produce equal or shorter total edge length
	// With fallback to LP, worst case should be equal
	if nsLength > lpLength {
		t.Errorf("NetworkSimplex produced longer edges: NS=%d, LP=%d", nsLength, lpLength)
	}
}

func totalEdgeLength(s *layoutState) int {
	total := 0
	for key := range s.edges {
		fromRank := s.nodes[key.from].rank
		toRank := s.nodes[key.to].rank
		total += toRank - fromRank
	}
	return total
}

// Tests for internal spanning tree operations

func TestSpanningTree_LowLimValues(t *testing.T) {
	// Build a simple tree: A - B - C
	tree := newSpanningTree()
	tree.addNode("A")
	tree.addNode("B")
	tree.addNode("C")
	tree.addEdge(edgeKey{from: "A", to: "B"})
	tree.addEdge(edgeKey{from: "B", to: "C"})

	tree.initLowLimValues()

	// C should have low=1, lim=1 (leaf)
	// B should have low=1, lim=2 (contains C)
	// A should have low=1, lim=3 (contains all)

	if tree.nodes["A"].low != 1 || tree.nodes["A"].lim != 3 {
		t.Errorf("A: expected low=1, lim=3, got low=%d, lim=%d",
			tree.nodes["A"].low, tree.nodes["A"].lim)
	}
}

func TestSpanningTree_IsDescendant(t *testing.T) {
	// Build a tree: A - (B, C) where B is parent of C
	tree := newSpanningTree()
	tree.addNode("A")
	tree.addNode("B")
	tree.addNode("C")
	tree.addEdge(edgeKey{from: "A", to: "B"})
	tree.addEdge(edgeKey{from: "B", to: "C"})

	tree.initLowLimValues()

	// C is descendant of B
	if !tree.isDescendant("C", "B") {
		t.Error("C should be descendant of B")
	}

	// C is descendant of A
	if !tree.isDescendant("C", "A") {
		t.Error("C should be descendant of A")
	}

	// B is not descendant of C
	if tree.isDescendant("B", "C") {
		t.Error("B should not be descendant of C")
	}

	// A is not descendant of B
	if tree.isDescendant("A", "B") {
		t.Error("A should not be descendant of B")
	}
}

func TestSpanningTree_PostorderNodes(t *testing.T) {
	tree := newSpanningTree()
	tree.addNode("A")
	tree.addNode("B")
	tree.addNode("C")
	tree.addEdge(edgeKey{from: "A", to: "B"})
	tree.addEdge(edgeKey{from: "A", to: "C"})

	tree.initLowLimValues()
	postorder := tree.postorderNodes()

	// Root should be last
	if postorder[len(postorder)-1] != "A" {
		t.Errorf("Root A should be last in postorder, got %v", postorder)
	}
}

func TestSlack(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	state := newLayoutState(g, DefaultOptions())
	state.nodes["A"].rank = 0
	state.nodes["B"].rank = 3

	// slack = toRank - fromRank - minlen = 3 - 0 - 1 = 2
	key := edgeKey{from: "A", to: "B"}
	if slack := state.slack(key); slack != 2 {
		t.Errorf("Expected slack 2, got %d", slack)
	}
}

func TestFeasibleTree(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.assignLayersLongestPath()

	tree := state.feasibleTree()

	// Tree should have all nodes
	if tree.nodeCount() != 3 {
		t.Errorf("Expected 3 nodes in tree, got %d", tree.nodeCount())
	}

	// Tree should have 2 edges (n-1 for spanning tree)
	edgeCount := 0
	seen := make(map[edgeKey]bool)
	for key := range tree.treeEdges {
		canonical := key
		if key.to < key.from {
			canonical = edgeKey{from: key.to, to: key.from}
		}
		if !seen[canonical] {
			edgeCount++
			seen[canonical] = true
		}
	}
	if edgeCount != 2 {
		t.Errorf("Expected 2 tree edges, got %d", edgeCount)
	}
}

// Benchmark tests

func BenchmarkNetworkSimplex_Small(b *testing.B) {
	g := buildRandomDAG(20, 30)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(Options{Algorithm: NetworkSimplex})
	}
}

func BenchmarkNetworkSimplex_Medium(b *testing.B) {
	g := buildRandomDAG(100, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(Options{Algorithm: NetworkSimplex})
	}
}

func BenchmarkComparison(b *testing.B) {
	g := buildRandomDAG(50, 100)

	b.Run("LongestPath", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			g.Layout(Options{Algorithm: LongestPath})
		}
	})

	b.Run("NetworkSimplex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			g.Layout(Options{Algorithm: NetworkSimplex})
		}
	})
}
