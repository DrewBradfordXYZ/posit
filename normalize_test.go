package posit

import (
	"fmt"
	"testing"
)

func TestNormalize_NoLongEdges(t *testing.T) {
	// A → B → C (all edges span 1 layer)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	initialNodes := len(state.nodes)
	state.addDummyNodes()

	// No dummies should be added
	if len(state.nodes) != initialNodes {
		t.Errorf("Expected no dummies, got %d extra nodes",
			len(state.nodes)-initialNodes)
	}
}

func TestNormalize_SingleLongEdge(t *testing.T) {
	// A → B → C with long edge A → C
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C") // Long edge spans 2 layers

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	dummyCount := state.addDummyNodes()

	// A→C spans ranks 0→2, needs 1 dummy at rank 1
	if dummyCount != 1 {
		t.Errorf("Expected 1 dummy, got %d", dummyCount)
	}
}

func TestNormalize_VeryLongEdge(t *testing.T) {
	// Chain of 5 nodes, plus edge from first to last
	g := NewGraph()
	for i := 0; i < 5; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < 4; i++ {
		g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	g.AddEdge("N0", "N4") // Long edge spans 4 layers

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	dummyCount := state.addDummyNodes()

	// N0→N4 spans ranks 0→4, needs 3 dummies
	if dummyCount != 3 {
		t.Errorf("Expected 3 dummies, got %d", dummyCount)
	}
}

func TestNormalize_DummyConnections(t *testing.T) {
	// Verify each dummy has exactly 1 in-edge and 1 out-edge
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C") // Creates 1 dummy

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	for id, node := range state.nodes {
		if !node.isDummy {
			continue
		}

		inDegree := len(state.predecessors[id])
		outDegree := len(state.successors[id])

		if inDegree != 1 {
			t.Errorf("Dummy %s has in-degree %d, expected 1", id, inDegree)
		}
		if outDegree != 1 {
			t.Errorf("Dummy %s has out-degree %d, expected 1", id, outDegree)
		}
	}
}

func TestNormalize_DummyInCorrectLayer(t *testing.T) {
	// Verify dummies are in the correct layers
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")
	g.AddEdge("A", "D") // Spans 3 layers, needs 2 dummies

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	for id, node := range state.nodes {
		if !node.isDummy {
			continue
		}

		// Verify node is in the layer array at its rank
		found := false
		for _, nodeID := range state.layers[node.rank] {
			if nodeID == id {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Dummy %s (rank %d) not found in layers[%d]",
				id, node.rank, node.rank)
		}
	}
}

func TestNormalize_DummyProperties(t *testing.T) {
	// Verify dummy nodes have correct properties
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	for id, node := range state.nodes {
		if !node.isDummy {
			continue
		}

		if node.width != 0 {
			t.Errorf("Dummy %s has width %f, expected 0", id, node.width)
		}
		if node.height != 0 {
			t.Errorf("Dummy %s has height %f, expected 0", id, node.height)
		}
		if node.order != -1 {
			t.Errorf("Dummy %s has order %d, expected -1", id, node.order)
		}
		if node.edgeLabel == nil {
			t.Errorf("Dummy %s has nil edgeLabel", id)
		}
	}
}

func TestNormalize_DummyChainTracking(t *testing.T) {
	// Verify dummy chains are tracked
	g := NewGraph()
	for i := 0; i < 5; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < 4; i++ {
		g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	g.AddEdge("N0", "N4") // Creates a chain of 3 dummies

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Should have 1 dummy chain
	if len(state.dummyChains) != 1 {
		t.Errorf("Expected 1 dummy chain, got %d", len(state.dummyChains))
	}

	// Walk the chain to verify
	if len(state.dummyChains) > 0 {
		current := state.dummyChains[0]
		chainLength := 0
		for {
			node := state.nodes[current]
			if !node.isDummy {
				break
			}
			chainLength++
			successors := state.successors[current]
			if len(successors) == 0 {
				break
			}
			current = successors[0]
		}

		if chainLength != 3 {
			t.Errorf("Expected chain length 3, got %d", chainLength)
		}
	}
}

func TestNormalize_AllEdgesSpanOneLayer(t *testing.T) {
	// After normalization, all edges should span exactly one layer
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")
	g.AddEdge("A", "C") // Span 2
	g.AddEdge("A", "D") // Span 3

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	for key := range state.edges {
		fromRank := state.nodes[key.from].rank
		toRank := state.nodes[key.to].rank
		span := toRank - fromRank

		if span != 1 {
			t.Errorf("Edge %s->%s spans %d layers, expected 1",
				key.from, key.to, span)
		}
	}
}

func TestNormalize_MultipleLongEdges(t *testing.T) {
	// Multiple long edges should each get their own dummy chain
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")
	g.AddEdge("A", "C") // Span 2, needs 1 dummy
	g.AddEdge("A", "D") // Span 3, needs 2 dummies
	g.AddEdge("B", "D") // Span 2, needs 1 dummy

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	dummyCount := state.addDummyNodes()

	// Total: 1 + 2 + 1 = 4 dummies
	if dummyCount != 4 {
		t.Errorf("Expected 4 dummies, got %d", dummyCount)
	}

	// 3 long edges means 3 dummy chains
	if len(state.dummyChains) != 3 {
		t.Errorf("Expected 3 dummy chains, got %d", len(state.dummyChains))
	}
}

func TestNormalize_EmptyGraph(t *testing.T) {
	g := NewGraph()

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	dummyCount := state.addDummyNodes()

	if dummyCount != 0 {
		t.Errorf("Expected 0 dummies for empty graph, got %d", dummyCount)
	}
}

func TestNormalize_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	dummyCount := state.addDummyNodes()

	if dummyCount != 0 {
		t.Errorf("Expected 0 dummies for single node, got %d", dummyCount)
	}
}

func TestNormalize_TwoNodesOneEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	dummyCount := state.addDummyNodes()

	// Edge A→B spans 1 layer, no dummies needed
	if dummyCount != 0 {
		t.Errorf("Expected 0 dummies, got %d", dummyCount)
	}
}

func TestNormalize_OriginalEdgeCount(t *testing.T) {
	// Total edges should be: original short edges + dummy edges
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B") // Short edge
	g.AddEdge("B", "C") // Short edge
	g.AddEdge("A", "C") // Long edge, becomes 2 edges

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// 2 short edges + 2 edges from split long edge = 4 edges
	if len(state.edges) != 4 {
		t.Errorf("Expected 4 edges, got %d", len(state.edges))
	}
}

func TestNormalize_EdgeLabelPreserved(t *testing.T) {
	// Verify the edgeLabel on dummies points to a valid edge
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	for id, node := range state.nodes {
		if !node.isDummy {
			continue
		}

		if node.edgeLabel == nil {
			t.Errorf("Dummy %s has nil edgeLabel", id)
			continue
		}

		// The edgeLabel should have a valid weight
		if node.edgeLabel.weight <= 0 {
			t.Errorf("Dummy %s edgeLabel has invalid weight %f",
				id, node.edgeLabel.weight)
		}
	}
}

func TestNormalize_DummyUniqueIDs(t *testing.T) {
	// Verify all dummy IDs are unique
	g := NewGraph()
	for i := 0; i < 10; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	// Create many long edges
	for i := 0; i < 5; i++ {
		g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	g.AddEdge("N0", "N5")  // Span 5
	g.AddEdge("N0", "N9")  // Span 9
	g.AddEdge("N1", "N8")  // Span 7

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	seen := make(map[string]bool)
	for id, node := range state.nodes {
		if !node.isDummy {
			continue
		}
		if seen[id] {
			t.Errorf("Duplicate dummy ID: %s", id)
		}
		seen[id] = true
	}
}
