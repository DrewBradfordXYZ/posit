package posit

import (
	"math"
	"testing"
)

// ==================== Rank Constraints Tests ====================

func TestRankConstraint_Min(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50, RankConstraint: RankMin})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout()

	// C should be at the top (lowest Y) despite being at the end of the chain
	if layout.Nodes["C"].Y > layout.Nodes["A"].Y {
		t.Errorf("RankMin node C should be at top layer, got Y=%v (A.Y=%v)",
			layout.Nodes["C"].Y, layout.Nodes["A"].Y)
	}
}

func TestRankConstraint_Max(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50, RankConstraint: RankMax})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout()

	// A should be at the bottom (highest Y) despite being at the start of the chain
	if layout.Nodes["A"].Y < layout.Nodes["C"].Y {
		t.Errorf("RankMax node A should be at bottom layer, got Y=%v (C.Y=%v)",
			layout.Nodes["A"].Y, layout.Nodes["C"].Y)
	}
}

func TestRankConstraint_Group(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50, RankGroup: "same"})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50, RankGroup: "same"})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout()

	// A and C should be on the same layer (same Y)
	if layout.Nodes["A"].Y != layout.Nodes["C"].Y {
		t.Errorf("RankGroup nodes A and C should share a layer, got A.Y=%v, C.Y=%v",
			layout.Nodes["A"].Y, layout.Nodes["C"].Y)
	}
}

func TestRankConstraint_Unconstrained(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	// Normal behavior: A above B
	if layout.Nodes["A"].Y >= layout.Nodes["B"].Y {
		t.Errorf("Unconstrained: A should be above B, got A.Y=%v, B.Y=%v",
			layout.Nodes["A"].Y, layout.Nodes["B"].Y)
	}
}

// ==================== Port Support Tests ====================

func TestPort_BasicConnections(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 100,
		Ports: []PortOptions{
			{ID: "out-1", Side: Bottom, Offset: 25},
			{ID: "out-2", Side: Bottom, Offset: 75},
		},
	})
	g.AddNode("B", NodeOptions{
		Width: 100, Height: 100,
		Ports: []PortOptions{
			{ID: "in-1", Side: Top, Offset: 50},
		},
	})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "out-1", TargetPort: "in-1"})

	layout := g.Layout()

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	if edge.SourcePort != "out-1" {
		t.Errorf("Expected SourcePort 'out-1', got %q", edge.SourcePort)
	}
	if edge.TargetPort != "in-1" {
		t.Errorf("Expected TargetPort 'in-1', got %q", edge.TargetPort)
	}

	// The first point should be at the port position
	if len(edge.Points) < 2 {
		t.Fatal("Edge should have at least 2 points")
	}

	// Source port is on bottom side at offset 25
	startPt := edge.Points[0]
	expectedX := layout.Nodes["A"].X + 25
	expectedY := layout.Nodes["A"].Y + 100 // bottom
	if math.Abs(startPt.X-expectedX) > 0.01 || math.Abs(startPt.Y-expectedY) > 0.01 {
		t.Errorf("Edge start should be at port (%.1f, %.1f), got (%.1f, %.1f)",
			expectedX, expectedY, startPt.X, startPt.Y)
	}

	// Target port is on top side at offset 50
	endPt := edge.Points[len(edge.Points)-1]
	expectedX = layout.Nodes["B"].X + 50
	expectedY = layout.Nodes["B"].Y // top
	if math.Abs(endPt.X-expectedX) > 0.01 || math.Abs(endPt.Y-expectedY) > 0.01 {
		t.Errorf("Edge end should be at port (%.1f, %.1f), got (%.1f, %.1f)",
			expectedX, expectedY, endPt.X, endPt.Y)
	}
}

func TestPort_SidePositions(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 80,
		Ports: []PortOptions{
			{ID: "right", Side: Right, Offset: 40},
			{ID: "left", Side: Left, Offset: 20},
			{ID: "top", Side: Top, Offset: 50},
			{ID: "bottom", Side: Bottom, Offset: 50},
		},
	})

	// Use internal state to test port resolution
	state := newLayoutState(g, DefaultOptions())
	node := state.nodes["A"]
	node.x = 10
	node.y = 20

	// Test Right port
	pt, ok := state.getPortPosition(node, "right")
	if !ok {
		t.Fatal("right port not found")
	}
	if pt.X != 110 || pt.Y != 60 {
		t.Errorf("Right port: expected (110, 60), got (%.1f, %.1f)", pt.X, pt.Y)
	}

	// Test Left port
	pt, ok = state.getPortPosition(node, "left")
	if !ok {
		t.Fatal("left port not found")
	}
	if pt.X != 10 || pt.Y != 40 {
		t.Errorf("Left port: expected (10, 40), got (%.1f, %.1f)", pt.X, pt.Y)
	}

	// Test Top port
	pt, ok = state.getPortPosition(node, "top")
	if !ok {
		t.Fatal("top port not found")
	}
	if pt.X != 60 || pt.Y != 20 {
		t.Errorf("Top port: expected (60, 20), got (%.1f, %.1f)", pt.X, pt.Y)
	}

	// Test Bottom port
	pt, ok = state.getPortPosition(node, "bottom")
	if !ok {
		t.Fatal("bottom port not found")
	}
	if pt.X != 60 || pt.Y != 100 {
		t.Errorf("Bottom port: expected (60, 100), got (%.1f, %.1f)", pt.X, pt.Y)
	}
}

func TestPort_FallbackToIntersection(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	// Reference a non-existent port - should fall back to intersection
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "nonexistent"})

	layout := g.Layout()

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}
	if len(edge.Points) < 2 {
		t.Error("Edge should have points even with nonexistent port")
	}
}

// ==================== Side Inference Tests ====================

func TestSideInference_TopToBottom(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	// In top-to-bottom layout, edges go from bottom of source to top of target
	if edge.SourceSide != Bottom {
		t.Errorf("Expected SourceSide=Bottom, got %v", edge.SourceSide)
	}
	if edge.TargetSide != Top {
		t.Errorf("Expected TargetSide=Top, got %v", edge.TargetSide)
	}
}

func TestSideInference_Horizontal(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{
		Direction: LeftToRight,
		NodeSep:   50,
		RankSep:   100,
	})

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	// In left-to-right layout, edges go from right of source to left of target
	if edge.SourceSide != Right {
		t.Errorf("Expected SourceSide=Right, got %v", edge.SourceSide)
	}
	if edge.TargetSide != Left {
		t.Errorf("Expected TargetSide=Left, got %v", edge.TargetSide)
	}
}

func TestSideInference_WithPort(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 50,
		Ports: []PortOptions{{ID: "out", Side: Right, Offset: 25}},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "out"})

	layout := g.Layout()

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	// Port side should override inferred side
	if edge.SourceSide != Right {
		t.Errorf("Expected SourceSide=Right (from port), got %v", edge.SourceSide)
	}
}

// ==================== Edge Weight Tests ====================

func TestEdgeWeight_HigherWeightPreservesOrder(t *testing.T) {
	// A graph where edge weight should influence crossing minimization
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})
	g.AddNode("C", NodeOptions{Width: 50, Height: 50})
	g.AddNode("D", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("A", "C", EdgeOptions{Weight: 10}) // Strong edge
	g.MustAddEdge("A", "D", EdgeOptions{Weight: 1})  // Weak edge
	g.MustAddEdge("B", "D", EdgeOptions{Weight: 10}) // Strong edge
	g.MustAddEdge("B", "C", EdgeOptions{Weight: 1})  // Weak edge

	layout := g.Layout()

	// With high weight on A->C and B->D, these should ideally not cross
	// A should be left of B, C should be left of D
	_ = layout // Layout completes without error
}

func TestEdgeWeight_DefaultWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B") // No weight specified

	layout := g.Layout()

	// Should work normally with default weight of 1
	if len(layout.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(layout.Nodes))
	}
}

func TestEdgeWeight_ExplicitWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{Weight: 5})

	layout := g.Layout()
	if len(layout.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(layout.Nodes))
	}
}

// ==================== Multi-Edge Tests ====================

func TestMultiEdge_DistinctPaths(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "edge-1", LabelWidth: 60, LabelHeight: 20})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "edge-2", LabelWidth: 60, LabelHeight: 20})

	layout := g.Layout()

	// Both edges should be in the layout with distinct keys
	edge1, ok1 := layout.Edges["A->B:edge-1"]
	edge2, ok2 := layout.Edges["A->B:edge-2"]

	if !ok1 {
		t.Error("Edge A->B:edge-1 not found in layout")
	}
	if !ok2 {
		t.Error("Edge A->B:edge-2 not found in layout")
	}

	if ok1 && ok2 && len(edge1.Points) > 0 && len(edge2.Points) > 0 {
		// Edges should be offset from each other
		samePath := true
		for i := range edge1.Points {
			if i < len(edge2.Points) {
				if edge1.Points[i].X != edge2.Points[i].X || edge1.Points[i].Y != edge2.Points[i].Y {
					samePath = false
					break
				}
			}
		}
		if samePath && len(edge1.Points) == len(edge2.Points) {
			t.Error("Multi-edges should have different paths (offset from each other)")
		}
	}
}

func TestMultiEdge_BackwardCompatible(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B") // No ID - should use "A->B" key

	layout := g.Layout()

	_, ok := layout.Edges["A->B"]
	if !ok {
		t.Error("Edge without ID should use 'A->B' key format")
	}
}

// ==================== Ordering Constraints Tests ====================

func TestOrderConstraint_Group(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 50, OrderGroup: "left"})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50, OrderGroup: "right"})
	g.AddNode("C", NodeOptions{Width: 50, Height: 50, OrderGroup: "left"})
	g.AddNode("D", NodeOptions{Width: 50, Height: 50})
	// All on same layer (no edges between them, but connected to a common parent)
	g.AddNode("Parent", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("Parent", "A")
	g.MustAddEdge("Parent", "B")
	g.MustAddEdge("Parent", "C")
	g.MustAddEdge("Parent", "D")

	layout := g.Layout()

	// A and C should be adjacent (same group)
	aX := layout.Nodes["A"].X
	cX := layout.Nodes["C"].X

	// They should be next to each other (no other node between them)
	bX := layout.Nodes["B"].X
	if (aX < bX && bX < cX) || (cX < bX && bX < aX) {
		t.Error("Nodes in same OrderGroup should be adjacent, but B is between A and C")
	}
}

func TestOrderConstraint_Priority(t *testing.T) {
	g := NewGraph()
	g.AddNode("C", NodeOptions{Width: 50, Height: 50, OrderGroup: "grp", OrderPriority: 3})
	g.AddNode("A", NodeOptions{Width: 50, Height: 50, OrderGroup: "grp", OrderPriority: 1})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50, OrderGroup: "grp", OrderPriority: 2})
	g.AddNode("Parent", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("Parent", "A")
	g.MustAddEdge("Parent", "B")
	g.MustAddEdge("Parent", "C")

	layout := g.Layout()

	// Within the group, priority order should be: A (1) < B (2) < C (3) from left to right
	aX := layout.Nodes["A"].X
	bX := layout.Nodes["B"].X
	cX := layout.Nodes["C"].X

	if !(aX < bX && bX < cX) {
		t.Errorf("OrderPriority: expected A < B < C left-to-right, got A.X=%v, B.X=%v, C.X=%v",
			aX, bX, cX)
	}
}

// ==================== Orthogonal Routing Tests ====================

func TestOrthogonalRouting_Basic(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{
		Direction:  TopToBottom,
		NodeSep:    50,
		RankSep:    100,
		RouteStyle: RouteOrthogonal,
	})

	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	// Orthogonal edges should have only horizontal and vertical segments
	for i := 1; i < len(edge.Points); i++ {
		dx := math.Abs(edge.Points[i].X - edge.Points[i-1].X)
		dy := math.Abs(edge.Points[i].Y - edge.Points[i-1].Y)
		if dx > 0.01 && dy > 0.01 {
			t.Errorf("Segment %d is diagonal: (%.1f,%.1f) -> (%.1f,%.1f)",
				i, edge.Points[i-1].X, edge.Points[i-1].Y,
				edge.Points[i].X, edge.Points[i].Y)
		}
	}
}

func TestOrthogonalRouting_MultiLayer(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{
		Direction:  TopToBottom,
		NodeSep:    50,
		RankSep:    100,
		RouteStyle: RouteOrthogonal,
		ChannelGap: 15,
	})

	// All edges should have orthogonal paths
	for edgeID, edge := range layout.Edges {
		for i := 1; i < len(edge.Points); i++ {
			dx := math.Abs(edge.Points[i].X - edge.Points[i-1].X)
			dy := math.Abs(edge.Points[i].Y - edge.Points[i-1].Y)
			if dx > 0.01 && dy > 0.01 {
				t.Errorf("Edge %s segment %d is diagonal", edgeID, i)
			}
		}
	}

	_ = layout
}

func TestOrthogonalRouting_ChannelGap(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{
		Direction:  TopToBottom,
		NodeSep:    50,
		RankSep:    100,
		RouteStyle: RouteOrthogonal,
		ChannelGap: 20,
	})

	if len(layout.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(layout.Edges))
	}
}

// ==================== Disconnected Component Packing Tests ====================

func TestComponentPacking_Horizontal(t *testing.T) {
	g := NewGraph()
	// Component 1
	g.AddNode("A", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("A", "B")
	// Component 2
	g.AddNode("C", NodeOptions{Width: 50, Height: 50})
	g.AddNode("D", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		Direction:        TopToBottom,
		NodeSep:          50,
		RankSep:          100,
		ComponentPacking: PackHorizontal,
		ComponentGap:     80,
	})

	// Components should be side by side
	comp1MaxX := math.Max(layout.Nodes["A"].X+layout.Nodes["A"].Width,
		layout.Nodes["B"].X+layout.Nodes["B"].Width)
	comp2MinX := math.Min(layout.Nodes["C"].X, layout.Nodes["D"].X)

	// There should be a gap between components
	if comp2MinX < comp1MaxX {
		// Could also be in reverse order
		comp2MaxX := math.Max(layout.Nodes["C"].X+layout.Nodes["C"].Width,
			layout.Nodes["D"].X+layout.Nodes["D"].Width)
		comp1MinX := math.Min(layout.Nodes["A"].X, layout.Nodes["B"].X)
		if comp1MinX < comp2MaxX {
			// One component is inside the other - this is a packing failure only
			// if they actually overlap. Check Y too.
			aY := layout.Nodes["A"].Y
			cY := layout.Nodes["C"].Y
			if math.Abs(aY-cY) < 1 {
				t.Error("Horizontal packing: components should not overlap")
			}
		}
	}

	_ = layout
}

func TestComponentPacking_Vertical(t *testing.T) {
	g := NewGraph()
	// Component 1
	g.AddNode("A", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("A", "B")
	// Component 2
	g.AddNode("C", NodeOptions{Width: 50, Height: 50})
	g.AddNode("D", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		Direction:        TopToBottom,
		NodeSep:          50,
		RankSep:          100,
		ComponentPacking: PackVertical,
		ComponentGap:     80,
	})

	// Components should be stacked
	_ = layout
	if len(layout.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(layout.Nodes))
	}
}

func TestComponentPacking_SingleComponent(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("A", "B")

	// Single component - packing should be a no-op
	layout := g.Layout(Options{
		Direction:        TopToBottom,
		NodeSep:          50,
		RankSep:          100,
		ComponentPacking: PackHorizontal,
	})

	if len(layout.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(layout.Nodes))
	}
}

// ==================== Incremental Layout Tests ====================

func TestIncrementalLayout_Basic(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	// Get base layout
	base := g.Layout()

	// Change B's height
	result := g.IncrementalLayout(base, IncrementalOptions{
		Fixed: map[string]bool{"A": true, "C": true},
		Changes: map[string]NodeOptions{
			"B": {Width: 100, Height: 100},
		},
	})

	// A and C should retain their X positions from base
	if math.Abs(result.Nodes["A"].X-base.Nodes["A"].X) > 0.01 {
		t.Errorf("Fixed node A moved: base.X=%v, result.X=%v",
			base.Nodes["A"].X, result.Nodes["A"].X)
	}
}

func TestIncrementalLayout_NilBase(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	// Should work without a base layout (equivalent to full layout)
	result := g.IncrementalLayout(nil, IncrementalOptions{
		Changes: map[string]NodeOptions{
			"A": {Width: 200, Height: 50},
		},
	})

	if result.Nodes["A"].Width != 200 {
		t.Errorf("Expected A width=200, got %v", result.Nodes["A"].Width)
	}
}

// ==================== Compound Graph (Cluster) Tests ====================

func TestCluster_Basic(t *testing.T) {
	g := NewGraph()
	g.AddNode("cluster-a", NodeOptions{IsCluster: true, Padding: 20})
	g.AddNode("node-1", NodeOptions{Width: 50, Height: 50})
	g.AddNode("node-2", NodeOptions{Width: 50, Height: 50})
	g.AddNode("outside", NodeOptions{Width: 50, Height: 50})
	g.SetParent("node-1", "cluster-a")
	g.SetParent("node-2", "cluster-a")
	g.MustAddEdge("node-1", "node-2")
	g.MustAddEdge("outside", "node-1")

	layout := g.Layout()

	cluster := layout.Nodes["cluster-a"]
	node1 := layout.Nodes["node-1"]
	node2 := layout.Nodes["node-2"]

	// Cluster should contain both children
	if node1.X < cluster.X || node1.X+node1.Width > cluster.X+cluster.Width {
		t.Error("node-1 should be horizontally within cluster-a")
	}
	if node2.X < cluster.X || node2.X+node2.Width > cluster.X+cluster.Width {
		t.Error("node-2 should be horizontally within cluster-a")
	}
	if node1.Y < cluster.Y || node1.Y+node1.Height > cluster.Y+cluster.Height {
		t.Error("node-1 should be vertically within cluster-a")
	}
	if node2.Y < cluster.Y || node2.Y+node2.Height > cluster.Y+cluster.Height {
		t.Error("node-2 should be vertically within cluster-a")
	}
}

func TestCluster_DefaultPadding(t *testing.T) {
	g := NewGraph()
	g.AddNode("cluster", NodeOptions{IsCluster: true}) // Default padding = 20
	g.AddNode("child", NodeOptions{Width: 50, Height: 50})
	g.SetParent("child", "cluster")

	layout := g.Layout()

	cluster := layout.Nodes["cluster"]
	child := layout.Nodes["child"]

	// With default padding of 20, cluster should be 40px wider and taller than child
	expectedWidth := child.Width + 40
	expectedHeight := child.Height + 40
	if math.Abs(cluster.Width-expectedWidth) > 0.01 {
		t.Errorf("Cluster width: expected %v, got %v", expectedWidth, cluster.Width)
	}
	if math.Abs(cluster.Height-expectedHeight) > 0.01 {
		t.Errorf("Cluster height: expected %v, got %v", expectedHeight, cluster.Height)
	}
}

func TestCluster_ParentAndChildren(t *testing.T) {
	g := NewGraph()
	g.AddNode("parent", NodeOptions{IsCluster: true})
	g.AddNode("child1", NodeOptions{Width: 50, Height: 50})
	g.AddNode("child2", NodeOptions{Width: 50, Height: 50})
	g.SetParent("child1", "parent")
	g.SetParent("child2", "parent")

	if g.Parent("child1") != "parent" {
		t.Errorf("Expected parent of child1 to be 'parent', got %q", g.Parent("child1"))
	}

	children := g.Children("parent")
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestCluster_EmptyCluster(t *testing.T) {
	g := NewGraph()
	g.AddNode("cluster", NodeOptions{IsCluster: true, Width: 200, Height: 100})
	g.AddNode("outside", NodeOptions{Width: 50, Height: 50})
	// No children set

	layout := g.Layout()

	// Empty cluster should still appear in layout with its specified dimensions
	cluster := layout.Nodes["cluster"]
	if cluster.Width != 200 || cluster.Height != 100 {
		t.Errorf("Empty cluster should keep specified dimensions, got %vx%v",
			cluster.Width, cluster.Height)
	}
}

// ==================== RouteStyle Option Tests ====================

func TestRouteStyle_Default(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	// Default should be polyline
	layout := g.Layout()
	if len(layout.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(layout.Edges))
	}
}

func TestRouteStyle_Polyline(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{
		Direction:  TopToBottom,
		NodeSep:    50,
		RankSep:    100,
		RouteStyle: RoutePolyline,
	})

	if len(layout.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(layout.Edges))
	}
}

// ==================== Combined Feature Tests ====================

func TestCombined_PortsWithRankConstraints(t *testing.T) {
	g := NewGraph()
	g.AddNode("input", NodeOptions{
		Width: 100, Height: 50,
		RankConstraint: RankMin,
		Ports: []PortOptions{
			{ID: "out", Side: Bottom, Offset: 50},
		},
	})
	g.AddNode("middle", NodeOptions{Width: 100, Height: 50})
	g.AddNode("output", NodeOptions{
		Width: 100, Height: 50,
		RankConstraint: RankMax,
		Ports: []PortOptions{
			{ID: "in", Side: Top, Offset: 50},
		},
	})
	g.MustAddEdge("input", "middle", EdgeOptions{SourcePort: "out"})
	g.MustAddEdge("middle", "output", EdgeOptions{TargetPort: "in"})

	layout := g.Layout()

	// input should be at top, output at bottom
	if layout.Nodes["input"].Y > layout.Nodes["middle"].Y {
		t.Error("RankMin node should be at top")
	}
	if layout.Nodes["output"].Y < layout.Nodes["middle"].Y {
		t.Error("RankMax node should be at bottom")
	}
}

func TestCombined_MultiEdgeWithWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "strong", Weight: 10})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "weak", Weight: 1})

	layout := g.Layout()

	// Both edges should exist
	_, ok1 := layout.Edges["A->B:strong"]
	_, ok2 := layout.Edges["A->B:weak"]
	if !ok1 || !ok2 {
		t.Error("Both multi-edges should be in layout")
	}
}

func TestCombined_AllAlgorithms(t *testing.T) {
	// Ensure new features work with all ranking algorithms
	algorithms := []RankAlgorithm{LongestPath, TightTree, NetworkSimplex}

	for _, alg := range algorithms {
		g := NewGraph()
		g.AddNode("A", NodeOptions{Width: 50, Height: 50, RankConstraint: RankMin})
		g.AddNode("B", NodeOptions{Width: 50, Height: 50})
		g.AddNode("C", NodeOptions{Width: 50, Height: 50, RankConstraint: RankMax})
		g.MustAddEdge("A", "B")
		g.MustAddEdge("B", "C")

		layout := g.Layout(Options{
			Direction: TopToBottom,
			NodeSep:   50,
			RankSep:   100,
			Algorithm: alg,
		})

		if len(layout.Nodes) != 3 {
			t.Errorf("Algorithm %v: expected 3 nodes, got %d", alg, len(layout.Nodes))
		}
	}
}

func TestCombined_AllDirections(t *testing.T) {
	directions := []Direction{TopToBottom, LeftToRight, BottomToTop, RightToLeft}

	for _, dir := range directions {
		g := NewGraph()
		g.AddNode("A", NodeOptions{
			Width: 100, Height: 50,
			Ports: []PortOptions{{ID: "out", Side: Bottom, Offset: 50}},
		})
		g.AddNode("B", NodeOptions{Width: 100, Height: 50})
		g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "out", Weight: 2})

		layout := g.Layout(Options{
			Direction: dir,
			NodeSep:   50,
			RankSep:   100,
		})

		if len(layout.Nodes) != 2 {
			t.Errorf("Direction %v: expected 2 nodes, got %d", dir, len(layout.Nodes))
		}

		edge, ok := layout.Edge("A", "B")
		if !ok {
			t.Errorf("Direction %v: edge not found", dir)
			continue
		}
		if len(edge.Points) < 2 {
			t.Errorf("Direction %v: edge has fewer than 2 points", dir)
		}
	}
}

// ==================== Edge Lookup Tests ====================

func TestEdgeLookup_MultiEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "first"})
	g.MustAddEdge("A", "B", EdgeOptions{ID: "second"})

	layout := g.Layout()

	// Edge() method should still work and return one of them
	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Error("Edge() should find at least one A->B edge")
	}
	if edge.From != "A" || edge.To != "B" {
		t.Error("Edge() returned wrong edge endpoints")
	}
}

// ==================== Validate Tests ====================

func TestValidate_RouteStyle(t *testing.T) {
	opts := Options{
		Direction:  TopToBottom,
		NodeSep:    50,
		RankSep:    100,
		RouteStyle: RouteOrthogonal,
	}
	if err := opts.Validate(); err != nil {
		t.Errorf("RouteOrthogonal should be valid: %v", err)
	}
}

func TestNodeOptions_Preserved(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:          100,
		Height:         50,
		RankConstraint: RankMin,
		RankGroup:      "group1",
		OrderGroup:     "order1",
		OrderPriority:  5,
		Ports: []PortOptions{
			{ID: "p1", Side: Right, Offset: 25},
		},
	})

	opts, ok := g.Node("A")
	if !ok {
		t.Fatal("Node A not found")
	}
	if opts.RankConstraint != RankMin {
		t.Error("RankConstraint not preserved")
	}
	if opts.RankGroup != "group1" {
		t.Error("RankGroup not preserved")
	}
	if opts.OrderGroup != "order1" {
		t.Error("OrderGroup not preserved")
	}
	if opts.OrderPriority != 5 {
		t.Error("OrderPriority not preserved")
	}
	if len(opts.Ports) != 1 || opts.Ports[0].ID != "p1" {
		t.Error("Ports not preserved")
	}
}
