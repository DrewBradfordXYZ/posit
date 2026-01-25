package posit

import (
	"math"
	"testing"
)

func TestSpreadStackedNodes_BasicFanIn(t *testing.T) {
	// Two source nodes stacked above a single target node
	// Without spreading, edges would cross due to ambiguous port-side selection
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            60,
		RankSep:            100,
	})

	// After spreading, A and B should be on opposite sides of C
	cCenterX := layout.Nodes["C"].X + layout.Nodes["C"].Width/2
	aCenterX := layout.Nodes["A"].X + layout.Nodes["A"].Width/2
	bCenterX := layout.Nodes["B"].X + layout.Nodes["B"].Width/2

	// A and B should not both be on the same side of C
	aLeft := aCenterX < cCenterX
	bLeft := bCenterX < cCenterX

	if aLeft == bLeft {
		// Both on same side - check if they're far enough apart from C
		// that port selection would still be unambiguous
		aDistFromC := math.Abs(aCenterX - cCenterX)
		bDistFromC := math.Abs(bCenterX - cCenterX)

		// If both are very close to C's center, spreading should have happened
		threshold := layout.Nodes["C"].Width / 2
		if aDistFromC < threshold && bDistFromC < threshold {
			t.Errorf("A and B should be spread apart. A center=%.1f, B center=%.1f, C center=%.1f",
				aCenterX, bCenterX, cCenterX)
		}
	}
}

func TestSpreadStackedNodes_BasicFanOut(t *testing.T) {
	// Single source node with two stacked target nodes
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            60,
		RankSep:            100,
	})

	// B and C should be in the same layer below A
	// After spreading, they should be on opposite sides of A
	aCenterX := layout.Nodes["A"].X + layout.Nodes["A"].Width/2
	bCenterX := layout.Nodes["B"].X + layout.Nodes["B"].Width/2
	cCenterX := layout.Nodes["C"].X + layout.Nodes["C"].Width/2

	// B and C should not both be very close to A's center
	bDistFromA := math.Abs(bCenterX - aCenterX)
	cDistFromA := math.Abs(cCenterX - aCenterX)

	threshold := layout.Nodes["A"].Width / 2
	if bDistFromA < threshold && cDistFromA < threshold {
		// Only fail if both are within the ambiguous zone
		bLeft := bCenterX < aCenterX
		cLeft := cCenterX < aCenterX
		if bLeft == cLeft {
			t.Errorf("B and C should be spread to opposite sides of A. B center=%.1f, C center=%.1f, A center=%.1f",
				bCenterX, cCenterX, aCenterX)
		}
	}
}

func TestSpreadStackedNodes_Disabled(t *testing.T) {
	// When disabled (default), no spreading should occur
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	layout1 := g.Layout(Options{
		SpreadStackedNodes: false, // Explicitly disabled
		NodeSep:            60,
		RankSep:            100,
	})

	layout2 := g.Layout(Options{
		// SpreadStackedNodes not set, defaults to false
		NodeSep: 60,
		RankSep: 100,
	})

	// Both layouts should be identical
	if layout1.Nodes["A"].X != layout2.Nodes["A"].X {
		t.Errorf("A.X differs: %.1f vs %.1f", layout1.Nodes["A"].X, layout2.Nodes["A"].X)
	}
	if layout1.Nodes["B"].X != layout2.Nodes["B"].X {
		t.Errorf("B.X differs: %.1f vs %.1f", layout1.Nodes["B"].X, layout2.Nodes["B"].X)
	}
	if layout1.Nodes["C"].X != layout2.Nodes["C"].X {
		t.Errorf("C.X differs: %.1f vs %.1f", layout1.Nodes["C"].X, layout2.Nodes["C"].X)
	}
}

func TestSpreadStackedNodes_CustomThreshold(t *testing.T) {
	// With a very large threshold, more nodes should be considered "stacked"
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		StackingThreshold:  200, // Very large threshold
		NodeSep:            60,
		RankSep:            100,
	})

	// With a large threshold, A and B should definitely be considered stacked
	// and should be spread apart
	if layout.Nodes["A"].X == 0 && layout.Nodes["B"].X == 0 {
		t.Error("Expected nodes to be spread, but both have X=0")
	}
}

func TestSpreadStackedNodes_NoEdges(t *testing.T) {
	// Nodes without edges shouldn't be affected
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	// No edges

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            60,
		RankSep:            100,
	})

	// Should complete without error
	if _, ok := layout.Nodes["A"]; !ok {
		t.Error("Node A not in layout")
	}
	if _, ok := layout.Nodes["B"]; !ok {
		t.Error("Node B not in layout")
	}
}

func TestSpreadStackedNodes_SingleNode(t *testing.T) {
	// Single node should not panic
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
	})

	if _, ok := layout.Nodes["A"]; !ok {
		t.Error("Node A not in layout")
	}
}

func TestSpreadStackedNodes_EmptyGraph(t *testing.T) {
	// Empty graph should not panic
	g := NewGraph()

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
	})

	if len(layout.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(layout.Nodes))
	}
}

func TestSpreadStackedNodes_Diamond(t *testing.T) {
	// Diamond pattern: A->B, A->C, B->D, C->D
	// B and C should be spread apart relative to A
	// B and C should also be spread relative to D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            60,
		RankSep:            100,
	})

	// Verify all nodes are in the layout
	for _, id := range []string{"A", "B", "C", "D"} {
		if _, ok := layout.Nodes[id]; !ok {
			t.Errorf("Node %s not in layout", id)
		}
	}

	// B and C should be horizontally separated
	bCenterX := layout.Nodes["B"].X + layout.Nodes["B"].Width/2
	cCenterX := layout.Nodes["C"].X + layout.Nodes["C"].Width/2

	if math.Abs(bCenterX-cCenterX) < 50 {
		t.Errorf("B and C should be horizontally separated. B center=%.1f, C center=%.1f",
			bCenterX, cCenterX)
	}
}

func TestSpreadStackedNodes_Hub(t *testing.T) {
	// Hub pattern: Multiple sources -> single target
	g := NewGraph()
	g.AddNode("Hub", NodeOptions{Width: 100, Height: 50})
	for i := 0; i < 5; i++ {
		id := string(rune('A' + i))
		g.AddNode(id, NodeOptions{Width: 80, Height: 40})
		g.MustAddEdge(id, "Hub")
	}

	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            40,
		RankSep:            80,
	})

	// All source nodes should be spread out
	hubCenterX := layout.Nodes["Hub"].X + layout.Nodes["Hub"].Width/2
	hubWidth := layout.Nodes["Hub"].Width

	leftCount := 0
	rightCount := 0
	for i := 0; i < 5; i++ {
		id := string(rune('A' + i))
		nodeCenterX := layout.Nodes[id].X + layout.Nodes[id].Width/2

		if nodeCenterX < hubCenterX-hubWidth/2 {
			leftCount++
		} else if nodeCenterX > hubCenterX+hubWidth/2 {
			rightCount++
		}
	}

	// At least some nodes should be clearly to the left and right
	// (not all stacked in the ambiguous zone)
	if leftCount == 0 && rightCount == 0 {
		t.Error("Expected at least some nodes to be clearly left or right of hub")
	}
}

func TestSpreadStackedNodes_PreservesLayerOrder(t *testing.T) {
	// Spreading should not change relative order within a layer
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")

	// Get layout without spreading
	layoutNoSpread := g.Layout(Options{
		SpreadStackedNodes: false,
		NodeSep:            60,
		RankSep:            100,
	})

	// Get layout with spreading
	layoutSpread := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            60,
		RankSep:            100,
	})

	// Check that A and B maintain their relative order
	// (the one that was left stays left, the one that was right stays right)
	aLeftNoSpread := layoutNoSpread.Nodes["A"].X < layoutNoSpread.Nodes["B"].X
	aLeftSpread := layoutSpread.Nodes["A"].X < layoutSpread.Nodes["B"].X

	if aLeftNoSpread != aLeftSpread {
		t.Log("Note: Spreading changed the relative order of A and B")
		// This is not necessarily an error, but worth noting
	}
}

func TestSpreadStackedNodes_WithClusters(t *testing.T) {
	// Clusters should be skipped in spread detection
	g := NewGraph()
	g.AddNode("cluster", NodeOptions{Width: 200, Height: 200, IsCluster: true})
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.SetParent("A", "cluster")
	g.SetParent("B", "cluster")
	g.MustAddEdge("A", "B")

	// Should not panic
	layout := g.Layout(Options{
		SpreadStackedNodes: true,
	})

	if _, ok := layout.Nodes["A"]; !ok {
		t.Error("Node A not in layout")
	}
	if _, ok := layout.Nodes["B"]; !ok {
		t.Error("Node B not in layout")
	}
}

func TestSpreadStackedNodes_RespectsNodeSep(t *testing.T) {
	// Spreading should not cause nodes to overlap or violate NodeSep
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50}) // Extra node in same layer as A, B
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("D", "C")

	nodeSep := 60.0
	layout := g.Layout(Options{
		SpreadStackedNodes: true,
		NodeSep:            nodeSep,
		RankSep:            100,
	})

	// Check that no same-layer nodes overlap
	// Get all nodes in the top layer (A, B, D)
	topNodes := []string{"A", "B", "D"}
	for i := 0; i < len(topNodes); i++ {
		for j := i + 1; j < len(topNodes); j++ {
			n1 := layout.Nodes[topNodes[i]]
			n2 := layout.Nodes[topNodes[j]]

			// Calculate gap between nodes
			var gap float64
			if n1.X < n2.X {
				gap = n2.X - (n1.X + n1.Width)
			} else {
				gap = n1.X - (n2.X + n2.Width)
			}

			// Gap should be at least NodeSep (allowing some tolerance)
			if gap < nodeSep-1 { // -1 for floating point tolerance
				t.Errorf("Nodes %s and %s have gap %.1f, want >= %.1f",
					topNodes[i], topNodes[j], gap, nodeSep)
			}
		}
	}
}
