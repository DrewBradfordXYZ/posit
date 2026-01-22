package posit

import (
	"math"
	"testing"
)

func TestDirection_TopToBottom(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{Direction: TopToBottom})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	// In TB layout, B should be below A
	if bNode.Y <= aNode.Y {
		t.Errorf("Expected B.Y > A.Y for TB layout, got A.Y=%v, B.Y=%v",
			aNode.Y, bNode.Y)
	}
}

func TestDirection_LeftToRight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{Direction: LeftToRight})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	// In LR layout, B should be to the right of A
	if bNode.X <= aNode.X {
		t.Errorf("Expected B.X > A.X for LR layout, got A.X=%v, B.X=%v",
			aNode.X, bNode.X)
	}

	// Y coordinates should be similar (same "rank" = same vertical level)
	if math.Abs(aNode.Y-bNode.Y) > 1 {
		t.Errorf("Expected similar Y for LR layout, got A.Y=%v, B.Y=%v",
			aNode.Y, bNode.Y)
	}
}

func TestDirection_BottomToTop(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{Direction: BottomToTop})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	// In BT layout, A (source) should be below B (target)
	if aNode.Y <= bNode.Y {
		t.Errorf("Expected A.Y > B.Y for BT layout, got A.Y=%v, B.Y=%v",
			aNode.Y, bNode.Y)
	}
}

func TestDirection_RightToLeft(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{Direction: RightToLeft})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	// In RL layout, A (source) should be to the right of B (target)
	if aNode.X <= bNode.X {
		t.Errorf("Expected A.X > B.X for RL layout, got A.X=%v, B.X=%v",
			aNode.X, bNode.X)
	}

	// Y coordinates should be similar
	if math.Abs(aNode.Y-bNode.Y) > 1 {
		t.Errorf("Expected similar Y for RL layout, got A.Y=%v, B.Y=%v",
			aNode.Y, bNode.Y)
	}
}

func TestDirection_LR_DimensionsSwapped(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "A") // Self-loop (will be removed but node still exists)

	// Remove self-loop for cleaner test
	g = NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	layout := g.Layout(Options{Direction: LeftToRight})

	aNode := layout.Nodes["A"]

	// For LR, the original width becomes height and vice versa
	// But the output should reflect the original dimensions
	if aNode.Width != 100 || aNode.Height != 50 {
		t.Errorf("LR layout changed dimensions: Width=%v, Height=%v, want 100, 50",
			aNode.Width, aNode.Height)
	}
}

func TestDirection_TB_NoCoordinateChanges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout(Options{Direction: TopToBottom})

	// All coordinates should be non-negative
	for id, node := range layout.Nodes {
		if node.X < 0 || node.Y < 0 {
			t.Errorf("Node %s has negative coordinates: (%v, %v)", id, node.X, node.Y)
		}
	}
}

func TestDirection_LR_ThreeNodeChain(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{Direction: LeftToRight})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]
	cNode := layout.Nodes["C"]

	// A should be left of B, B should be left of C
	if aNode.X >= bNode.X {
		t.Errorf("Expected A.X < B.X, got A.X=%v, B.X=%v", aNode.X, bNode.X)
	}
	if bNode.X >= cNode.X {
		t.Errorf("Expected B.X < C.X, got B.X=%v, C.X=%v", bNode.X, cNode.X)
	}
}

func TestDirection_BT_ThreeNodeChain(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{Direction: BottomToTop})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]
	cNode := layout.Nodes["C"]

	// In BT: A is source, so A should be at bottom (highest Y)
	// C is sink, so C should be at top (lowest Y)
	if aNode.Y <= bNode.Y {
		t.Errorf("Expected A.Y > B.Y for BT, got A.Y=%v, B.Y=%v", aNode.Y, bNode.Y)
	}
	if bNode.Y <= cNode.Y {
		t.Errorf("Expected B.Y > C.Y for BT, got B.Y=%v, C.Y=%v", bNode.Y, cNode.Y)
	}
}

func TestDirection_EdgePointsTransformed(t *testing.T) {
	// Use a 3-node chain to ensure there's a meaningful flow
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{Direction: LeftToRight})

	// Check the first edge (A->B)
	edge := layout.Edges["A->B"]
	if len(edge.Points) < 2 {
		t.Fatal("Expected at least 2 edge points")
	}

	// In LR layout, edge points should flow left to right (or be equal for adjacent nodes)
	startX := edge.Points[0].X
	endX := edge.Points[len(edge.Points)-1].X

	// For adjacent nodes, start and end X can be equal (at the shared boundary)
	// For the overall layout, A should be left of B
	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	if bNode.X <= aNode.X {
		t.Errorf("Expected B to be right of A, got A.X=%v, B.X=%v", aNode.X, bNode.X)
	}

	// Edge should flow from left (near A) to right (near B)
	// Start should be near A's right side, end near B's left side
	if endX < startX {
		t.Errorf("Expected edge to flow left to right, got start.X=%v, end.X=%v",
			startX, endX)
	}
}

func TestDirection_NoOverlapsInAllDirections(t *testing.T) {
	directions := []Direction{TopToBottom, BottomToTop, LeftToRight, RightToLeft}
	dirNames := []string{"TB", "BT", "LR", "RL"}

	for i, dir := range directions {
		t.Run(dirNames[i], func(t *testing.T) {
			g := NewGraph()
			g.AddNode("A", NodeOptions{Width: 100, Height: 50})
			g.AddNode("B", NodeOptions{Width: 100, Height: 50})
			g.AddNode("C", NodeOptions{Width: 100, Height: 50})
			g.MustAddEdge("A", "B")
			g.MustAddEdge("A", "C")

			layout := g.Layout(Options{Direction: dir})

			// Check no overlaps
			nodes := []string{"A", "B", "C"}
			for j := 0; j < len(nodes); j++ {
				for k := j + 1; k < len(nodes); k++ {
					n1 := layout.Nodes[nodes[j]]
					n2 := layout.Nodes[nodes[k]]
					if nodesOverlapDir(n1, n2) {
						t.Errorf("Nodes %s and %s overlap in %s direction",
							nodes[j], nodes[k], dirNames[i])
					}
				}
			}
		})
	}
}

// nodesOverlapDir checks if two nodes overlap
func nodesOverlapDir(n1, n2 NodeLayout) bool {
	return !(n1.X+n1.Width <= n2.X ||
		n2.X+n2.Width <= n1.X ||
		n1.Y+n1.Height <= n2.Y ||
		n2.Y+n2.Height <= n1.Y)
}
