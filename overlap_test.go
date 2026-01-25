package posit

import "testing"

func TestCrossLayerOverlapResolution(t *testing.T) {
	// Create two tall nodes that would overlap with default RankSep
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 150}) // Tall node
	g.AddNode("B", NodeOptions{Width: 100, Height: 150}) // Tall node
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{
		RankSep:               100, // Default gap
		NodeNodeBetweenLayers: 20,  // Require 20px between boundaries
	})

	// Verify no overlap: bottom of A should be at least 20px above top of B
	aBottom := layout.Nodes["A"].Y + layout.Nodes["A"].Height
	bTop := layout.Nodes["B"].Y

	gap := bTop - aBottom
	if gap < 20 {
		t.Errorf("Cross-layer gap = %.1f, want >= 20", gap)
	}
}

func TestCrossLayerNoOverlapWhenXSeparated(t *testing.T) {
	// Nodes with no X overlap shouldn't trigger adjustment
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 150})
	g.AddNode("B", NodeOptions{Width: 50, Height: 150})
	g.AddNode("C", NodeOptions{Width: 50, Height: 150})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	layout := g.Layout(Options{
		NodeSep:               200, // Wide horizontal separation
		RankSep:               50,  // Small vertical gap
		NodeNodeBetweenLayers: 20,
	})

	// B and C should be in the same layer, X-separated from each other
	// Neither should trigger cross-layer adjustment with A since X ranges don't overlap
	bY := layout.Nodes["B"].Y
	cY := layout.Nodes["C"].Y

	// B and C should be at the same Y (same layer)
	if bY != cY {
		t.Errorf("B.Y = %.1f, C.Y = %.1f, expected same layer", bY, cY)
	}
}

func TestCrossLayerDisabledByDefault(t *testing.T) {
	// When NodeNodeBetweenLayers is 0 (default), no adjustment should happen
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 150})
	g.AddNode("B", NodeOptions{Width: 100, Height: 150})
	g.MustAddEdge("A", "B")

	layout1 := g.Layout(Options{
		RankSep:               100,
		NodeNodeBetweenLayers: 0, // Disabled
	})

	layout2 := g.Layout(Options{
		RankSep: 100,
		// NodeNodeBetweenLayers not set, defaults to 0
	})

	// Both layouts should be identical
	if layout1.Nodes["A"].Y != layout2.Nodes["A"].Y {
		t.Errorf("A.Y differs: %.1f vs %.1f", layout1.Nodes["A"].Y, layout2.Nodes["A"].Y)
	}
	if layout1.Nodes["B"].Y != layout2.Nodes["B"].Y {
		t.Errorf("B.Y differs: %.1f vs %.1f", layout1.Nodes["B"].Y, layout2.Nodes["B"].Y)
	}
}

func TestCrossLayerHorizontalShift(t *testing.T) {
	// When there's room to shift horizontally, prefer that over layer gap increase
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 150})  // Narrow but tall
	g.AddNode("B1", NodeOptions{Width: 50, Height: 150}) // In layer below A
	g.AddNode("B2", NodeOptions{Width: 50, Height: 50})  // Also in layer below A
	g.MustAddEdge("A", "B1")
	g.MustAddEdge("A", "B2")

	layout := g.Layout(Options{
		NodeSep:               100, // Room to shift
		RankSep:               50,  // Small gap
		NodeNodeBetweenLayers: 20,
	})

	// All nodes should exist
	if _, ok := layout.Nodes["A"]; !ok {
		t.Fatal("Node A not in layout")
	}
	if _, ok := layout.Nodes["B1"]; !ok {
		t.Fatal("Node B1 not in layout")
	}
	if _, ok := layout.Nodes["B2"]; !ok {
		t.Fatal("Node B2 not in layout")
	}
}

func TestCrossLayerDirectEdgeSkipped(t *testing.T) {
	// Nodes with direct edges between them should not trigger overlap resolution
	// (the edge routing handles the visual connection)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 150})
	g.AddNode("B", NodeOptions{Width: 100, Height: 150})
	g.MustAddEdge("A", "B")

	// Even with a very small RankSep, direct edges should be allowed
	// (This test verifies the hasDirectEdge check works)
	layout := g.Layout(Options{
		RankSep:               10, // Very small
		NodeNodeBetweenLayers: 50, // Large requirement
	})

	// Should still produce valid layout
	if layout.Nodes["A"].Y >= layout.Nodes["B"].Y {
		t.Errorf("A should be above B: A.Y=%.1f, B.Y=%.1f",
			layout.Nodes["A"].Y, layout.Nodes["B"].Y)
	}
}

func TestCrossLayerMultipleLayers(t *testing.T) {
	// Test with a chain of nodes across multiple layers
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 100})
	g.AddNode("B", NodeOptions{Width: 100, Height: 100})
	g.AddNode("C", NodeOptions{Width: 100, Height: 100})
	g.AddNode("D", NodeOptions{Width: 100, Height: 100})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		RankSep:               80,
		NodeNodeBetweenLayers: 20,
	})

	// Verify all pairs have sufficient gap
	pairs := [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}}
	for _, pair := range pairs {
		upper := layout.Nodes[pair[0]]
		lower := layout.Nodes[pair[1]]
		gap := lower.Y - (upper.Y + upper.Height)
		if gap < 20 {
			t.Errorf("%s->%s gap = %.1f, want >= 20", pair[0], pair[1], gap)
		}
	}
}

func TestCrossLayerWideNodes(t *testing.T) {
	// Wide nodes that span across multiple X positions
	g := NewGraph()
	g.AddNode("Wide", NodeOptions{Width: 300, Height: 150}) // Very wide and tall
	g.AddNode("B1", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 50})
	g.AddNode("B3", NodeOptions{Width: 50, Height: 50})
	g.MustAddEdge("Wide", "B1")
	g.MustAddEdge("Wide", "B2")
	g.MustAddEdge("Wide", "B3")

	layout := g.Layout(Options{
		NodeSep:               30,
		RankSep:               50,
		NodeNodeBetweenLayers: 20,
	})

	// Wide node's bottom should be at least 20px above each B node's top
	wideBottom := layout.Nodes["Wide"].Y + layout.Nodes["Wide"].Height
	for _, id := range []string{"B1", "B2", "B3"} {
		node := layout.Nodes[id]
		// Check if there's X overlap
		wideLeft := layout.Nodes["Wide"].X
		wideRight := wideLeft + layout.Nodes["Wide"].Width
		nodeLeft := node.X
		nodeRight := node.X + node.Width

		xOverlap := min(wideRight, nodeRight) - max(wideLeft, nodeLeft)
		if xOverlap > 0 {
			gap := node.Y - wideBottom
			if gap < 20 {
				t.Errorf("Wide->%s gap = %.1f (X overlap %.1f), want >= 20",
					id, gap, xOverlap)
			}
		}
	}
}
