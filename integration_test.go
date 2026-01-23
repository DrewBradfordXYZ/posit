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
