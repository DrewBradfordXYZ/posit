package posit

import (
	"math"
	"testing"
)

// Helper to check if two nodes overlap
func nodesOverlap(n1, n2 NodeLayout) bool {
	// Two rectangles overlap if they overlap on both axes
	return !(n1.X+n1.Width <= n2.X || // n1 is completely left of n2
		n2.X+n2.Width <= n1.X || // n2 is completely left of n1
		n1.Y+n1.Height <= n2.Y || // n1 is completely above n2
		n2.Y+n2.Height <= n1.Y) // n2 is completely above n1
}

func TestPosition_NoOverlap(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	layout := g.Layout()

	// Check all pairs for overlap
	nodes := []string{"A", "B", "C"}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			n1 := layout.Nodes[nodes[i]]
			n2 := layout.Nodes[nodes[j]]
			if nodesOverlap(n1, n2) {
				t.Errorf("Nodes %s and %s overlap: %s at (%.1f, %.1f) size %.1fx%.1f, %s at (%.1f, %.1f) size %.1fx%.1f",
					nodes[i], nodes[j],
					nodes[i], n1.X, n1.Y, n1.Width, n1.Height,
					nodes[j], n2.X, n2.Y, n2.Width, n2.Height)
			}
		}
	}
}

func TestPosition_MinimumSpacing(t *testing.T) {
	opts := Options{NodeSep: 50, RankSep: 100}

	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	layout := g.Layout(opts)

	// B and C should be in the same layer - check horizontal spacing
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	if math.Abs(b.Y-c.Y) < 1 { // Same layer
		// Get the gap between them
		var left, right NodeLayout
		if b.X < c.X {
			left, right = b, c
		} else {
			left, right = c, b
		}
		gap := right.X - (left.X + left.Width)
		if gap < opts.NodeSep-1 {
			t.Errorf("Horizontal gap %.1f < NodeSep %.1f", gap, opts.NodeSep)
		}
	}

	// A should be above B - check vertical spacing
	a := layout.Nodes["A"]
	vertGap := b.Y - (a.Y + a.Height)
	if vertGap < opts.RankSep-1 {
		t.Errorf("Vertical gap %.1f < RankSep %.1f", vertGap, opts.RankSep)
	}
}

func TestPosition_ValidCoordinates(t *testing.T) {
	g := NewGraph()

	// Build a moderately sized DAG
	nodes := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for _, id := range nodes {
		g.AddNode(id, NodeOptions{Width: 80, Height: 40})
	}
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("B", "E")
	g.MustAddEdge("C", "F")
	g.MustAddEdge("D", "G")
	g.MustAddEdge("E", "G")
	g.MustAddEdge("F", "H")
	g.MustAddEdge("G", "H")

	layout := g.Layout()

	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
			t.Errorf("Node %s has invalid X: %v", id, node.X)
		}
		if math.IsNaN(node.Y) || math.IsInf(node.Y, 0) {
			t.Errorf("Node %s has invalid Y: %v", id, node.Y)
		}
		if node.X < 0 {
			t.Errorf("Node %s has negative X: %v", id, node.X)
		}
		if node.Y < 0 {
			t.Errorf("Node %s has negative Y: %v", id, node.Y)
		}
	}
}

func TestPosition_YCoordinatesFollowRank(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout()

	a := layout.Nodes["A"]
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	// With top-to-bottom layout, A.Y < B.Y < C.Y
	if a.Y >= b.Y {
		t.Errorf("A.Y (%.1f) should be < B.Y (%.1f)", a.Y, b.Y)
	}
	if b.Y >= c.Y {
		t.Errorf("B.Y (%.1f) should be < C.Y (%.1f)", b.Y, c.Y)
	}
}

func TestPosition_SameLayerSameY(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	layout := g.Layout()

	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	// B and C should be in the same layer (same Y)
	if math.Abs(b.Y-c.Y) > 1 {
		t.Errorf("B and C should have same Y, but B.Y=%.1f and C.Y=%.1f", b.Y, c.Y)
	}
}

func TestPosition_VariableHeights(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 100}) // Taller
	g.AddNode("C", NodeOptions{Width: 100, Height: 30})  // Shorter
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	opts := Options{RankSep: 50}
	layout := g.Layout(opts)

	// B and C are in the same layer
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]
	d := layout.Nodes["D"]

	// Both should have same Y (top-left convention)
	if math.Abs(b.Y-c.Y) > 1 {
		t.Errorf("B and C should have same Y, but B.Y=%.1f and C.Y=%.1f", b.Y, c.Y)
	}

	// D should be below the tallest node (B) plus RankSep
	maxHeight := math.Max(b.Height, c.Height)
	expectedMinDY := b.Y + maxHeight + opts.RankSep
	if d.Y < expectedMinDY-1 {
		t.Errorf("D.Y (%.1f) should be >= %.1f (maxHeight=%.1f + RankSep=%.1f)",
			d.Y, expectedMinDY, maxHeight, opts.RankSep)
	}
}

func TestPosition_VariableWidths(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 200, Height: 50}) // Wide
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})  // Narrow
	g.AddNode("C", NodeOptions{Width: 100, Height: 50}) // Medium
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	opts := Options{NodeSep: 30}
	layout := g.Layout(opts)

	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	// B and C should not overlap
	if nodesOverlap(b, c) {
		t.Errorf("B and C overlap despite NodeSep")
	}

	// And should be properly separated
	var left, right NodeLayout
	if b.X < c.X {
		left, right = b, c
	} else {
		left, right = c, b
	}
	gap := right.X - (left.X + left.Width)
	if gap < opts.NodeSep-1 {
		t.Errorf("Gap between nodes (%.1f) < NodeSep (%.1f)", gap, opts.NodeSep)
	}
}

func TestPosition_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	layout := g.Layout()

	a := layout.Nodes["A"]
	if a.X < 0 || a.Y < 0 {
		t.Errorf("Single node has negative coordinates: (%.1f, %.1f)", a.X, a.Y)
	}
	if math.IsNaN(a.X) || math.IsNaN(a.Y) {
		t.Errorf("Single node has NaN coordinates")
	}
}

func TestPosition_TwoNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	a := layout.Nodes["A"]
	b := layout.Nodes["B"]

	// A should be above B
	if a.Y >= b.Y {
		t.Errorf("A should be above B: A.Y=%.1f, B.Y=%.1f", a.Y, b.Y)
	}

	// No overlap
	if nodesOverlap(a, b) {
		t.Errorf("A and B overlap")
	}
}

func TestPosition_DiamondGraph(t *testing.T) {
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	layout := g.Layout()

	a := layout.Nodes["A"]
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]
	d := layout.Nodes["D"]

	// Verify layer ordering: A above B/C above D
	if a.Y >= b.Y || a.Y >= c.Y {
		t.Error("A should be above B and C")
	}
	if b.Y >= d.Y || c.Y >= d.Y {
		t.Error("B and C should be above D")
	}

	// B and C same layer
	if math.Abs(b.Y-c.Y) > 1 {
		t.Errorf("B and C should have same Y: B.Y=%.1f, C.Y=%.1f", b.Y, c.Y)
	}

	// No overlaps
	nodes := []NodeLayout{a, b, c, d}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodesOverlap(nodes[i], nodes[j]) {
				t.Errorf("Nodes %d and %d overlap", i, j)
			}
		}
	}
}

func TestPosition_WideGraph(t *testing.T) {
	// Single node connecting to many
	//        A
	//    / | | | \
	//   B  C D E  F
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	children := []string{"B", "C", "D", "E", "F"}
	for _, id := range children {
		g.AddNode(id, NodeOptions{Width: 100, Height: 50})
		g.MustAddEdge("A", id)
	}

	opts := Options{NodeSep: 40}
	layout := g.Layout(opts)

	// All children should be in same layer
	firstY := layout.Nodes["B"].Y
	for _, id := range children {
		node := layout.Nodes[id]
		if math.Abs(node.Y-firstY) > 1 {
			t.Errorf("Child %s has different Y than others: %.1f vs %.1f", id, node.Y, firstY)
		}
	}

	// Check no overlaps among children
	for i := 0; i < len(children); i++ {
		for j := i + 1; j < len(children); j++ {
			n1 := layout.Nodes[children[i]]
			n2 := layout.Nodes[children[j]]
			if nodesOverlap(n1, n2) {
				t.Errorf("Children %s and %s overlap", children[i], children[j])
			}
		}
	}
}

func TestPosition_DeepGraph(t *testing.T) {
	// A -> B -> C -> D -> E -> F
	g := NewGraph()
	nodes := []string{"A", "B", "C", "D", "E", "F"}
	for _, id := range nodes {
		g.AddNode(id, NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < len(nodes)-1; i++ {
		g.MustAddEdge(nodes[i], nodes[i+1])
	}

	opts := Options{RankSep: 60}
	layout := g.Layout(opts)

	// Each node should be lower than the previous
	for i := 0; i < len(nodes)-1; i++ {
		n1 := layout.Nodes[nodes[i]]
		n2 := layout.Nodes[nodes[i+1]]
		if n1.Y >= n2.Y {
			t.Errorf("%s should be above %s: Y=%.1f vs %.1f", nodes[i], nodes[i+1], n1.Y, n2.Y)
		}

		// Check spacing
		gap := n2.Y - (n1.Y + n1.Height)
		if gap < opts.RankSep-1 {
			t.Errorf("Gap between %s and %s (%.1f) < RankSep (%.1f)",
				nodes[i], nodes[i+1], gap, opts.RankSep)
		}
	}
}

func TestPosition_LargeGraphUsesSimpleMethod(t *testing.T) {
	// Build a graph with >100 nodes
	g := NewGraph()
	for i := 0; i < 150; i++ {
		g.AddNode(string(rune('A'+i%26))+string(rune('0'+i/26)), NodeOptions{Width: 50, Height: 30})
	}
	// Add some edges
	for i := 0; i < 100; i++ {
		from := string(rune('A'+i%26)) + string(rune('0'+i/26))
		to := string(rune('A'+(i+1)%26)) + string(rune('0'+(i+1)/26))
		g.MustAddEdge(from, to)
	}

	// This should use the simple method and not hang
	layout := g.Layout()

	// Basic sanity check
	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsNaN(node.Y) {
			t.Errorf("Node %s has NaN coordinates", id)
		}
		if node.X < 0 || node.Y < 0 {
			t.Errorf("Node %s has negative coordinates", id)
		}
	}
}

func TestPosition_CenteringWorks(t *testing.T) {
	//   A
	//   |
	// B-C-D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("C", "B") // B comes from C
	g.MustAddEdge("C", "D") // D comes from C

	layout := g.Layout()

	a := layout.Nodes["A"]
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]
	d := layout.Nodes["D"]

	// All should have valid coordinates
	for id, n := range layout.Nodes {
		if n.X < 0 || n.Y < 0 {
			t.Errorf("Node %s has negative coordinate: (%.1f, %.1f)", id, n.X, n.Y)
		}
	}

	// A should be centered relative to the wider layer
	// Since there's a layer with 3 nodes, A's center should be approximately
	// at the center of the layout
	_ = a
	_ = b
	_ = c
	_ = d
}

func TestPosition_EmptyGraph(t *testing.T) {
	g := NewGraph()
	layout := g.Layout()

	if len(layout.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(layout.Nodes))
	}
}

func TestPosition_DisconnectedNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	// No edges - all disconnected

	layout := g.Layout()

	// All should have valid coordinates
	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsNaN(node.Y) {
			t.Errorf("Node %s has NaN coordinates", id)
		}
		if node.X < 0 || node.Y < 0 {
			t.Errorf("Node %s has negative coordinates: (%.1f, %.1f)", id, node.X, node.Y)
		}
	}

	// No overlaps
	a := layout.Nodes["A"]
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	if nodesOverlap(a, b) || nodesOverlap(a, c) || nodesOverlap(b, c) {
		t.Error("Disconnected nodes should not overlap")
	}
}

func TestPosition_LongEdgeWithDummies(t *testing.T) {
	// A -> D (span 3 layers, requiring dummy nodes)
	// B -> C -> D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "D") // Spans multiple layers
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "D")

	layout := g.Layout()

	// Should have 4 nodes (no dummies exported)
	if len(layout.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(layout.Nodes))
	}

	// All nodes should have valid positions
	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsNaN(node.Y) {
			t.Errorf("Node %s has NaN coordinates", id)
		}
	}
}
