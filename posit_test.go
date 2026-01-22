package posit

import (
	"sort"
	"testing"
)

// ==================== 7.1 Input Validation Tests ====================

func TestAddEdge_MissingSource(t *testing.T) {
	g := NewGraph()
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})

	err := g.AddEdge("A", "B")
	if err == nil {
		t.Error("Expected error for missing source node")
	}
}

func TestAddEdge_MissingTarget(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	err := g.AddEdge("A", "B")
	if err == nil {
		t.Error("Expected error for missing target node")
	}
}

func TestAddEdge_Valid(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})

	err := g.AddEdge("A", "B")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestAddEdge_BothMissing(t *testing.T) {
	g := NewGraph()

	err := g.AddEdge("A", "B")
	if err == nil {
		t.Error("Expected error when both nodes missing")
	}
}

func TestMustAddEdge_Panics(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustAddEdge should panic for missing node")
		}
	}()

	g.MustAddEdge("A", "B") // B doesn't exist
}

func TestMustAddEdge_Valid(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})

	// Should not panic
	g.MustAddEdge("A", "B")

	if g.EdgeCount() != 1 {
		t.Errorf("Expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestHasNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	if !g.HasNode("A") {
		t.Error("HasNode(A) = false, want true")
	}
	if g.HasNode("B") {
		t.Error("HasNode(B) = true, want false")
	}
}

// ==================== 7.3 Graph Query API Tests ====================

func TestGraph_Nodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("C", NodeOptions{})
	g.AddNode("A", NodeOptions{})
	g.AddNode("B", NodeOptions{})

	nodes := g.Nodes()
	expected := []string{"A", "B", "C"}

	if len(nodes) != len(expected) {
		t.Errorf("Nodes() returned %d nodes, want %d", len(nodes), len(expected))
	}

	// Check sorted order
	if !sort.StringsAreSorted(nodes) {
		t.Error("Nodes() should return sorted slice")
	}

	for i, id := range expected {
		if nodes[i] != id {
			t.Errorf("Nodes()[%d] = %s, want %s", i, nodes[i], id)
		}
	}
}

func TestGraph_Nodes_Empty(t *testing.T) {
	g := NewGraph()
	nodes := g.Nodes()

	if len(nodes) != 0 {
		t.Errorf("Nodes() on empty graph = %v, want []", nodes)
	}
}

func TestGraph_Edges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{})
	g.AddNode("B", NodeOptions{})
	g.AddNode("C", NodeOptions{})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	edges := g.Edges()

	if len(edges) != 2 {
		t.Errorf("Edges() returned %d edges, want 2", len(edges))
	}

	// Check edges are present (order matches insertion order)
	if edges[0] != [2]string{"A", "B"} {
		t.Errorf("edges[0] = %v, want [A B]", edges[0])
	}
	if edges[1] != [2]string{"B", "C"} {
		t.Errorf("edges[1] = %v, want [B C]", edges[1])
	}
}

func TestGraph_Edges_Empty(t *testing.T) {
	g := NewGraph()
	edges := g.Edges()

	if len(edges) != 0 {
		t.Errorf("Edges() on empty graph = %v, want []", edges)
	}
}

func TestGraph_HasEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{})
	g.AddNode("B", NodeOptions{})
	g.MustAddEdge("A", "B")

	if !g.HasEdge("A", "B") {
		t.Error("HasEdge(A,B) = false, want true")
	}
	if g.HasEdge("B", "A") {
		t.Error("HasEdge(B,A) = true, want false (edge is directional)")
	}
	if g.HasEdge("A", "C") {
		t.Error("HasEdge(A,C) = true, want false (C doesn't exist)")
	}
}

func TestGraph_Node(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	opts, ok := g.Node("A")
	if !ok {
		t.Error("Node(A) = _, false; want _, true")
	}
	if opts.Width != 100 || opts.Height != 50 {
		t.Errorf("Node(A) = {%v, %v}, want {100, 50}", opts.Width, opts.Height)
	}

	_, ok = g.Node("B")
	if ok {
		t.Error("Node(B) = _, true; want _, false")
	}
}

// ==================== 7.4 Duplicate Edge Handling Tests ====================

func TestDuplicateEdges_Aggregated(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B")

	// Should not panic
	layout := g.Layout()

	// Should have exactly one edge in output
	if len(layout.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(layout.Edges))
	}

	if _, ok := layout.Edges["A->B"]; !ok {
		t.Error("Expected edge A->B in output")
	}
}

func TestDuplicateEdges_WeightAggregation(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B")

	state := newLayoutState(g, DefaultOptions())

	// Check that weight is aggregated
	key := edgeKey{from: "A", to: "B"}
	edge, ok := state.edges[key]
	if !ok {
		t.Fatal("Edge A->B not found in state")
	}

	if edge.weight != 2 {
		t.Errorf("Edge weight = %v, want 2", edge.weight)
	}
}

func TestDuplicateEdges_AdjacencyOnce(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B")

	state := newLayoutState(g, DefaultOptions())

	// Check successors only has B once
	successors := state.successors["A"]
	if len(successors) != 1 {
		t.Errorf("A has %d successors, want 1", len(successors))
	}

	// Check predecessors only has A once
	predecessors := state.predecessors["B"]
	if len(predecessors) != 1 {
		t.Errorf("B has %d predecessors, want 1", len(predecessors))
	}
}
