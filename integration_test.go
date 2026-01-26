package posit

import (
	"sync"
	"testing"
)

// Integration tests for cross-cutting features and behaviors.

func TestSelfLoopsPreserved(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "A") // Self-loop

	layout := g.Layout()

	// Check that self-loop is in output
	edge, ok := layout.Edge("A", "A")
	if !ok {
		t.Fatal("Self-loop should be in output")
	}

	// Self-loop should have multiple points forming a curve
	if len(edge.Points) < 3 {
		t.Errorf("Self-loop should have curved path, got %d points", len(edge.Points))
	}
}

func TestDeterministicLayout(t *testing.T) {
	// Run layout 10 times on same graph and verify all outputs are identical
	var layouts []*Layout

	for i := 0; i < 10; i++ {
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
		layouts = append(layouts, layout)
	}

	// Compare all layouts to first
	first := layouts[0]
	for i := 1; i < len(layouts); i++ {
		for nodeID, firstNode := range first.Nodes {
			otherNode := layouts[i].Nodes[nodeID]
			if firstNode.X != otherNode.X || firstNode.Y != otherNode.Y {
				t.Errorf("Layout %d differs from first for node %s: got (%v,%v), want (%v,%v)",
					i, nodeID, otherNode.X, otherNode.Y, firstNode.X, firstNode.Y)
			}
		}
	}
}

func TestEdgeLabelPositioning(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})

	// Add edge with label - needs to span multiple layers to create label dummy
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.AddEdge("A", "C", EdgeOptions{
		LabelWidth:  40,
		LabelHeight: 20,
	})

	layout := g.Layout()
	edge, ok := layout.Edge("A", "C")
	if !ok {
		t.Fatal("Edge A->C not found in layout")
	}

	if edge.Label == nil {
		t.Fatal("Edge label should have coordinates")
	}

	// Label should be positioned between A and C
	nodeA := layout.Nodes["A"]
	nodeC := layout.Nodes["C"]

	if edge.Label.Y <= nodeA.Y || edge.Label.Y >= nodeC.Y {
		t.Errorf("Label Y (%v) should be between node A (%v) and node C (%v)",
			edge.Label.Y, nodeA.Y, nodeC.Y)
	}

	// Label dimensions should be preserved
	if edge.Label.Width != 40 || edge.Label.Height != 20 {
		t.Errorf("Label dimensions wrong: got %vx%v, want 40x20",
			edge.Label.Width, edge.Label.Height)
	}
}

func TestGreedyFAS(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.AddNode("C", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "A") // Creates cycle

	layout := g.Layout(Options{
		NodeSep:   50,
		RankSep:   100,
		Acyclicer: GreedyAcyclicer,
	})

	// Should produce valid layout (no crashes, all nodes positioned)
	if len(layout.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(layout.Nodes))
	}

	// Nodes should have valid coordinates
	for id, node := range layout.Nodes {
		if node.X < 0 || node.Y < 0 {
			t.Errorf("Node %s has invalid coordinates: (%v, %v)", id, node.X, node.Y)
		}
	}
}

func TestTightTreeRanker(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.AddNode("C", NodeOptions{Width: 50, Height: 30})
	g.AddNode("D", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		NodeSep:   50,
		RankSep:   100,
		Algorithm: TightTree,
	})

	// Should produce valid layout
	if len(layout.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(layout.Nodes))
	}

	// D should be below A
	if layout.Nodes["D"].Y <= layout.Nodes["A"].Y {
		t.Error("Node D should be below node A")
	}
}

func TestOptionsValidation(t *testing.T) {
	// Valid options should not error
	_, err := NewGraph().LayoutWithError(Options{
		NodeSep: 50,
		RankSep: 100,
	})
	if err != nil {
		t.Errorf("Valid options should not error: %v", err)
	}

	// Negative NodeSep should error
	_, err = NewGraph().LayoutWithError(Options{
		NodeSep: -10,
		RankSep: 100,
	})
	if err == nil {
		t.Error("Negative NodeSep should error")
	}

	// Negative RankSep should error
	_, err = NewGraph().LayoutWithError(Options{
		NodeSep: 50,
		RankSep: -10,
	})
	if err == nil {
		t.Error("Negative RankSep should error")
	}
}

func TestLayoutEdgeMethod(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	// Use Edge method for lookup
	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	if edge.From != "A" || edge.To != "B" {
		t.Errorf("Edge From/To wrong: got %s->%s, want A->B", edge.From, edge.To)
	}

	// Non-existent edge should return false
	_, ok = layout.Edge("B", "A")
	if ok {
		t.Error("Non-existent edge B->A should not be found")
	}
}

func TestHasEdgeO1(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")

	// Should be O(1) lookup
	if !g.HasEdge("A", "B") {
		t.Error("HasEdge should return true for existing edge")
	}

	if g.HasEdge("B", "A") {
		t.Error("HasEdge should return false for non-existing edge")
	}
}

func TestSmallestWidthAlignment(t *testing.T) {
	// Create a graph where different alignments produce different widths
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B1", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B1")
	g.MustAddEdge("A", "B2")
	g.MustAddEdge("B1", "C")
	g.MustAddEdge("B2", "C")

	layout := g.Layout()

	// Calculate layout width
	var minX, maxX float64 = 1e9, -1e9
	for _, node := range layout.Nodes {
		if node.X < minX {
			minX = node.X
		}
		if node.X+node.Width > maxX {
			maxX = node.X + node.Width
		}
	}
	width := maxX - minX

	// Width should be reasonable (not excessively large)
	// The old broken implementation could produce widths > 300
	if width > 300 {
		t.Errorf("Layout width %v is too large, smallest-width alignment may be broken", width)
	}
}

func TestEdgeLabelPositions(t *testing.T) {
	// Test all three label positions: LabelLeft, LabelCenter, LabelRight
	for _, pos := range []LabelPosition{LabelLeft, LabelCenter, LabelRight} {
		t.Run(string(pos), func(t *testing.T) {
			g := NewGraph()
			g.AddNode("A", NodeOptions{Width: 100, Height: 50})
			g.AddNode("B", NodeOptions{Width: 100, Height: 50})
			g.AddNode("C", NodeOptions{Width: 100, Height: 50})
			g.AddNode("D", NodeOptions{Width: 100, Height: 50})

			// Create a long edge A->D that spans multiple layers
			g.MustAddEdge("A", "B")
			g.MustAddEdge("B", "C")
			g.MustAddEdge("C", "D")
			g.AddEdge("A", "D", EdgeOptions{
				LabelWidth:    40,
				LabelHeight:   20,
				LabelPosition: pos,
			})

			layout := g.Layout()
			edge, ok := layout.Edge("A", "D")
			if !ok {
				t.Fatal("Edge A->D not found in layout")
			}

			if edge.Label == nil {
				t.Fatal("Edge label should have coordinates")
			}

			// Label should be positioned between A and D
			nodeA := layout.Nodes["A"]
			nodeD := layout.Nodes["D"]

			if edge.Label.Y <= nodeA.Y || edge.Label.Y >= nodeD.Y+nodeD.Height {
				t.Errorf("Label Y (%v) should be between node A (%v) and node D (%v)",
					edge.Label.Y, nodeA.Y, nodeD.Y+nodeD.Height)
			}

			// Label dimensions should be preserved
			if edge.Label.Width != 40 || edge.Label.Height != 20 {
				t.Errorf("Label dimensions wrong: got %vx%v, want 40x20",
					edge.Label.Width, edge.Label.Height)
			}
		})
	}
}

func TestConcurrentLayout(t *testing.T) {
	// Build a graph once
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	// Run layout concurrently from multiple goroutines
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			layout := g.Layout()
			if len(layout.Nodes) != 3 {
				errors <- &concurrentError{msg: "Unexpected node count"}
			}
			if len(layout.Edges) != 2 {
				errors <- &concurrentError{msg: "Unexpected edge count"}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

type concurrentError struct {
	msg string
}

func (e *concurrentError) Error() string {
	return e.msg
}

func TestRouteFromPositions_SelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:  200,
		Height: 100,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 40, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "port-7", Offset: 80, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "A", EdgeOptions{ID: "7", SourcePort: "port-3", TargetPort: "port-7"})

	// Route with node at position (100, 50)
	layout := g.RouteFromPositions(map[string]Position{
		"A": {X: 100, Y: 50},
	})

	edge, ok := layout.Edge("A", "A")
	if !ok {
		t.Fatal("Self-loop edge should be in output")
	}

	// Self-loop should have 3 points: start, waypoint, end
	if len(edge.Points) != 3 {
		t.Fatalf("Expected 3 points for self-loop, got %d", len(edge.Points))
	}

	// Start and end should be on right side of node (x = 100 + 200 = 300)
	if edge.Points[0].X != 300 {
		t.Errorf("Start X should be 300 (node right edge), got %.1f", edge.Points[0].X)
	}
	if edge.Points[2].X != 300 {
		t.Errorf("End X should be 300 (node right edge), got %.1f", edge.Points[2].X)
	}

	// Start Y should be at port-3 offset (50 + 40 = 90)
	if edge.Points[0].Y != 90 {
		t.Errorf("Start Y should be 90 (node.Y + port offset), got %.1f", edge.Points[0].Y)
	}

	// End Y should be at port-7 offset (50 + 80 = 130)
	if edge.Points[2].Y != 130 {
		t.Errorf("End Y should be 130 (node.Y + port offset), got %.1f", edge.Points[2].Y)
	}

	// Waypoint X should be to the right of the node (> 300)
	if edge.Points[1].X <= 300 {
		t.Errorf("Waypoint X should be > 300 (right of node), got %.1f", edge.Points[1].X)
	}

	// Both sides should be Right
	if edge.SourceSide != Right {
		t.Errorf("Source side should be Right, got %v", edge.SourceSide)
	}
	if edge.TargetSide != Right {
		t.Errorf("Target side should be Right, got %v", edge.TargetSide)
	}
}

func TestRouteFromPositions_MovedNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:  200,
		Height: 100,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 40, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "port-7", Offset: 80, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "A", EdgeOptions{ID: "7", SourcePort: "port-3", TargetPort: "port-7"})

	// Route at original position
	layout1 := g.RouteFromPositions(map[string]Position{
		"A": {X: 100, Y: 50},
	})
	edge1, _ := layout1.Edge("A", "A")

	// Route at moved position (shifted by +200, +100)
	layout2 := g.RouteFromPositions(map[string]Position{
		"A": {X: 300, Y: 150},
	})
	edge2, _ := layout2.Edge("A", "A")

	// Waypoints should shift by the same delta as the node
	dx := edge2.Points[1].X - edge1.Points[1].X
	dy := edge2.Points[1].Y - edge1.Points[1].Y

	if dx != 200 {
		t.Errorf("Waypoint X should shift by 200, shifted by %.1f", dx)
	}
	if dy != 100 {
		t.Errorf("Waypoint Y should shift by 100, shifted by %.1f", dy)
	}
}

func TestRouteFromPositions_ProportionalArc(t *testing.T) {
	g := NewGraph()
	// Wide port spacing → larger arc
	g.AddNode("A", NodeOptions{
		Width:  200,
		Height: 200,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 20, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "port-7", Offset: 180, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "A", EdgeOptions{ID: "7", SourcePort: "port-3", TargetPort: "port-7"})

	// Narrow port spacing → smaller arc
	g2 := NewGraph()
	g2.AddNode("A", NodeOptions{
		Width:  200,
		Height: 200,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 90, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "port-7", Offset: 110, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g2.MustAddEdge("A", "A", EdgeOptions{ID: "7", SourcePort: "port-3", TargetPort: "port-7"})

	pos := map[string]Position{"A": {X: 0, Y: 0}}
	layout1 := g.RouteFromPositions(pos)
	layout2 := g2.RouteFromPositions(pos)

	edge1, _ := layout1.Edge("A", "A")
	edge2, _ := layout2.Edge("A", "A")

	// Wide spacing should produce a larger arc (waypoint further from node)
	arcWidth1 := edge1.Points[1].X - 200 // distance from right edge
	arcWidth2 := edge2.Points[1].X - 200

	if arcWidth1 <= arcWidth2 {
		t.Errorf("Wider port spacing should produce larger arc: wide=%.1f, narrow=%.1f", arcWidth1, arcWidth2)
	}

	// Verify proportional: portDist=160 → loopDist = 160*0.6 = 96
	expectedArc1 := 160.0 * 0.6
	if arcWidth1 != expectedArc1 {
		t.Errorf("Expected arc width %.1f (portDist*0.6), got %.1f", expectedArc1, arcWidth1)
	}

	// portDist=20 → loopDist = max(20*0.6=12, 30) = 30
	expectedArc2 := 30.0
	if arcWidth2 != expectedArc2 {
		t.Errorf("Expected arc width %.1f (min 30), got %.1f", expectedArc2, arcWidth2)
	}
}

func TestRouteFromPositions_NonSelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:  100,
		Height: 50,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 25, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{
		Width:  100,
		Height: 50,
		Ports: []PortOptions{
			{ID: "port-5", Offset: 25, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "port-3", TargetPort: "port-5"})

	// B is to the right of A
	layout := g.RouteFromPositions(map[string]Position{
		"A": {X: 0, Y: 0},
		"B": {X: 300, Y: 0},
	})

	edgeKey := "A->B"
	edge, ok := layout.Edges[edgeKey]
	if !ok {
		t.Fatal("Edge A->B should be in output")
	}

	// Source should exit right (B is to the right)
	if edge.SourceSide != Right {
		t.Errorf("Source side should be Right (target is to the right), got %v", edge.SourceSide)
	}
	// Target should enter left
	if edge.TargetSide != Left {
		t.Errorf("Target side should be Left, got %v", edge.TargetSide)
	}

	// Start point should be on right side of A (x=100, y=25)
	if edge.Points[0].X != 100 {
		t.Errorf("Start X should be 100 (right of A), got %.1f", edge.Points[0].X)
	}
	// End point should be on left side of B (x=300, y=25)
	if edge.Points[len(edge.Points)-1].X != 300 {
		t.Errorf("End X should be 300 (left of B), got %.1f", edge.Points[len(edge.Points)-1].X)
	}
}

func TestRouteFromPositions_SideFlipsOnMove(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:  100,
		Height: 50,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 25, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{
		Width:  100,
		Height: 50,
		Ports: []PortOptions{
			{ID: "port-5", Offset: 25, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "port-3", TargetPort: "port-5"})

	// B to the right: source exits right
	layout1 := g.RouteFromPositions(map[string]Position{
		"A": {X: 0, Y: 0},
		"B": {X: 300, Y: 0},
	})
	edge1 := layout1.Edges["A->B"]
	if edge1.SourceSide != Right {
		t.Errorf("With B to right, source should exit Right, got %v", edge1.SourceSide)
	}

	// B to the left: source exits left
	layout2 := g.RouteFromPositions(map[string]Position{
		"A": {X: 300, Y: 0},
		"B": {X: 0, Y: 0},
	})
	edge2 := layout2.Edges["A->B"]
	if edge2.SourceSide != Left {
		t.Errorf("With B to left, source should exit Left, got %v", edge2.SourceSide)
	}
}

func TestRouteEdgesForNode_OnlyAffectedEdges(t *testing.T) {
	// Create a graph with multiple nodes and edges
	// A -> B, A -> C, B -> C, C -> D
	// When we move node B, only edges A->B and B->C should be in the result
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B", EdgeOptions{})
	g.MustAddEdge("A", "C", EdgeOptions{})
	g.MustAddEdge("B", "C", EdgeOptions{})
	g.MustAddEdge("C", "D", EdgeOptions{})

	positions := map[string]Position{
		"A": {X: 0, Y: 0},
		"B": {X: 200, Y: 0},
		"C": {X: 100, Y: 100},
		"D": {X: 100, Y: 200},
	}

	// Route only edges connected to B
	layout := g.RouteEdgesForNode("B", positions)

	// Should have edges A->B and B->C
	if _, ok := layout.Edges["A->B"]; !ok {
		t.Error("Expected edge A->B in result")
	}
	if _, ok := layout.Edges["B->C"]; !ok {
		t.Error("Expected edge B->C in result")
	}

	// Should NOT have edges A->C or C->D (not connected to B)
	if _, ok := layout.Edges["A->C"]; ok {
		t.Error("Edge A->C should not be in result (not connected to B)")
	}
	if _, ok := layout.Edges["C->D"]; ok {
		t.Error("Edge C->D should not be in result (not connected to B)")
	}

	// Verify edge count
	if len(layout.Edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(layout.Edges))
	}
}

func TestRouteEdgesForNode_SelfLoop(t *testing.T) {
	// Test that self-loops are included when routing for their node
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width:  100,
		Height: 100,
		Ports: []PortOptions{
			{ID: "p1", Offset: 30, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "p2", Offset: 70, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "A", EdgeOptions{SourcePort: "p1", TargetPort: "p2"}) // self-loop
	g.MustAddEdge("A", "B", EdgeOptions{})

	positions := map[string]Position{
		"A": {X: 0, Y: 0},
		"B": {X: 200, Y: 0},
	}

	layout := g.RouteEdgesForNode("A", positions)

	// Should have both the self-loop and A->B
	if _, ok := layout.Edges["A->A"]; !ok {
		t.Error("Expected self-loop A->A in result")
	}
	if _, ok := layout.Edges["A->B"]; !ok {
		t.Error("Expected edge A->B in result")
	}
	if len(layout.Edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(layout.Edges))
	}
}
