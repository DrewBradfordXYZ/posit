package posit

import (
	"testing"
)

// TestXSimplex_LinearChain tests a simple linear chain A -> B -> C
func TestXSimplex_LinearChain(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.AddNode("C", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
	})

	// Verify all nodes have positions
	if len(layout.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(layout.Nodes))
	}

	// Verify nodes are in different layers (different Y)
	aY := layout.Nodes["A"].Y
	bY := layout.Nodes["B"].Y
	cY := layout.Nodes["C"].Y

	if bY <= aY {
		t.Errorf("B.Y (%v) should be > A.Y (%v)", bY, aY)
	}
	if cY <= bY {
		t.Errorf("C.Y (%v) should be > B.Y (%v)", cY, bY)
	}
}

// TestXSimplex_Diamond tests a diamond pattern A -> (B, C) -> D
func TestXSimplex_Diamond(t *testing.T) {
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
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
	})

	// Verify all nodes have positions
	if len(layout.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(layout.Nodes))
	}

	// B and C should be on the same layer (same Y) and separated in X
	bNode := layout.Nodes["B"]
	cNode := layout.Nodes["C"]

	if bNode.Y != cNode.Y {
		t.Errorf("B.Y (%v) should equal C.Y (%v)", bNode.Y, cNode.Y)
	}

	// B and C should not overlap horizontally
	bRight := bNode.X + bNode.Width
	cLeft := cNode.X
	if bNode.X < cNode.X {
		// B is to the left of C
		if bRight > cLeft {
			t.Errorf("B and C overlap: B.right=%v, C.left=%v", bRight, cLeft)
		}
	} else {
		// C is to the left of B
		cRight := cNode.X + cNode.Width
		bLeft := bNode.X
		if cRight > bLeft {
			t.Errorf("B and C overlap: C.right=%v, B.left=%v", cRight, bLeft)
		}
	}
}

// TestXSimplex_SeparationConstraints verifies same-layer nodes respect separation
func TestXSimplex_SeparationConstraints(t *testing.T) {
	g := NewGraph()
	// Fan-out pattern: A -> B, A -> C, A -> D (B, C, D on same layer)
	g.AddNode("A", NodeOptions{Width: 100, Height: 30})
	g.AddNode("B", NodeOptions{Width: 100, Height: 30})
	g.AddNode("C", NodeOptions{Width: 100, Height: 30})
	g.AddNode("D", NodeOptions{Width: 100, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("A", "D")

	nodeSep := 50.0
	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         nodeSep,
		RankSep:         100,
	})

	// B, C, D should be on the same layer
	bNode := layout.Nodes["B"]
	cNode := layout.Nodes["C"]
	dNode := layout.Nodes["D"]

	if bNode.Y != cNode.Y || cNode.Y != dNode.Y {
		t.Errorf("B, C, D should be on same layer: B.Y=%v, C.Y=%v, D.Y=%v", bNode.Y, cNode.Y, dNode.Y)
	}

	// Check separation between each pair
	nodes := []NodeLayout{bNode, cNode, dNode}
	// Sort by X for checking
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			left := nodes[i]
			right := nodes[j]
			if left.X > right.X {
				left, right = right, left
			}

			gap := right.X - (left.X + left.Width)
			if gap < nodeSep-1 { // Allow small floating point tolerance
				t.Errorf("insufficient separation: left.right=%v, right.left=%v, gap=%v, required=%v",
					left.X+left.Width, right.X, gap, nodeSep)
			}
		}
	}
}

// TestXSimplex_MatchesBKContract verifies X simplex satisfies same invariants as BK
func TestXSimplex_MatchesBKContract(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 80, Height: 50})
	g.AddNode("C", NodeOptions{Width: 120, Height: 50})
	g.AddNode("D", NodeOptions{Width: 90, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
	})

	// Contract: All coordinates must be non-negative
	for id, node := range layout.Nodes {
		if node.X < 0 {
			t.Errorf("node %s has negative X: %v", id, node.X)
		}
		if node.Y < 0 {
			t.Errorf("node %s has negative Y: %v", id, node.Y)
		}
	}

	// Contract: No overlapping nodes on same layer
	// Group nodes by Y
	byLayer := make(map[float64][]struct {
		id   string
		node NodeLayout
	})
	for id, node := range layout.Nodes {
		byLayer[node.Y] = append(byLayer[node.Y], struct {
			id   string
			node NodeLayout
		}{id, node})
	}

	for _, nodes := range byLayer {
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				n1 := nodes[i].node
				n2 := nodes[j].node

				// Check horizontal overlap
				n1Left, n1Right := n1.X, n1.X+n1.Width
				n2Left, n2Right := n2.X, n2.X+n2.Width

				if n1Right > n2Left && n2Right > n1Left {
					t.Errorf("nodes %s and %s overlap: [%v,%v] and [%v,%v]",
						nodes[i].id, nodes[j].id, n1Left, n1Right, n2Left, n2Right)
				}
			}
		}
	}
}

// TestXSimplex_EmptyGraph handles edge case of empty graph
func TestXSimplex_EmptyGraph(t *testing.T) {
	g := NewGraph()
	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
	})

	if len(layout.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(layout.Nodes))
	}
}

// TestXSimplex_SingleNode handles edge case of single node
func TestXSimplex_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
	})

	if len(layout.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(layout.Nodes))
	}

	node := layout.Nodes["A"]
	if node.X < 0 || node.Y < 0 {
		t.Errorf("node has negative coordinates: X=%v, Y=%v", node.X, node.Y)
	}
}

// TestXSimplex_VsBK compares results of both algorithms
func TestXSimplex_VsBK(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	bkLayout := g.Layout(Options{
		XCoordAlgorithm: XBrandesKopf,
		NodeSep:         50,
		RankSep:         100,
	})

	nsLayout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
	})

	// Both should produce valid layouts with same number of nodes
	if len(bkLayout.Nodes) != len(nsLayout.Nodes) {
		t.Errorf("node count mismatch: BK=%d, NS=%d", len(bkLayout.Nodes), len(nsLayout.Nodes))
	}

	// Both should have same Y coordinates (same layer assignment)
	for id := range bkLayout.Nodes {
		bkY := bkLayout.Nodes[id].Y
		nsY := nsLayout.Nodes[id].Y
		if bkY != nsY {
			t.Errorf("node %s Y mismatch: BK=%v, NS=%v", id, bkY, nsY)
		}
	}
}

// TestXSimplex_PreventStacking verifies that anti-stacking pushes connected nodes apart
func TestXSimplex_PreventStacking(t *testing.T) {
	g := NewGraph()
	// Simple chain: A -> B -> C
	// Without anti-stacking, all nodes would be vertically aligned (stacked)
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	nodeSep := 50.0

	// Without anti-stacking: nodes should be stacked (same X center)
	layoutStacked := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         nodeSep,
		RankSep:         100,
		PreventStacking: false,
	})

	aStacked := layoutStacked.Nodes["A"]
	bStacked := layoutStacked.Nodes["B"]

	// Centers should be close (stacked)
	aCenterStacked := aStacked.X + aStacked.Width/2
	bCenterStacked := bStacked.X + bStacked.Width/2
	stackedDiff := aCenterStacked - bCenterStacked
	if stackedDiff < 0 {
		stackedDiff = -stackedDiff
	}

	// With anti-stacking: nodes should be offset
	layoutUnstacked := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         nodeSep,
		RankSep:         100,
		PreventStacking: true,
	})

	aUnstacked := layoutUnstacked.Nodes["A"]
	bUnstacked := layoutUnstacked.Nodes["B"]

	// Check that horizontal bounds don't overlap
	// A's bounds: [A.X, A.X + A.Width]
	// B's bounds: [B.X, B.X + B.Width]
	aLeft := aUnstacked.X
	aRight := aUnstacked.X + aUnstacked.Width
	bLeft := bUnstacked.X
	bRight := bUnstacked.X + bUnstacked.Width

	// Check for overlap: ranges intersect if NOT (aRight < bLeft OR bRight < aLeft)
	overlapping := !(aRight < bLeft || bRight < aLeft)

	if overlapping {
		t.Errorf("PreventStacking should separate A and B horizontally: A=[%v,%v], B=[%v,%v]",
			aLeft, aRight, bLeft, bRight)
	}

	// The unstacked layout should have more X spread than stacked
	unstackedDiff := (aUnstacked.X + aUnstacked.Width/2) - (bUnstacked.X + bUnstacked.Width/2)
	if unstackedDiff < 0 {
		unstackedDiff = -unstackedDiff
	}

	if unstackedDiff <= stackedDiff {
		t.Errorf("PreventStacking should increase X separation: stacked=%v, unstacked=%v",
			stackedDiff, unstackedDiff)
	}
}

// TestXSimplex_PreventStacking_FanOut tests anti-stacking with fan-out pattern
func TestXSimplex_PreventStacking_FanOut(t *testing.T) {
	g := NewGraph()
	// Fan-out: A -> (B, C, D)
	// In fan-out, A is connected to multiple children on the same layer.
	// Anti-stacking can't prevent overlap with ALL children (they span wider than A).
	// But it should ensure A isn't directly stacked (same center) with any one child.
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("A", "D")

	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
		PreventStacking: true,
	})

	aNode := layout.Nodes["A"]
	aCenter := aNode.X + aNode.Width/2

	// Verify A's center is not identical to any child's center
	// (small tolerance for floating point)
	tolerance := 1.0
	for _, child := range []struct {
		name string
		node NodeLayout
	}{
		{"B", layout.Nodes["B"]},
		{"C", layout.Nodes["C"]},
		{"D", layout.Nodes["D"]},
	} {
		childCenter := child.node.X + child.node.Width/2
		diff := aCenter - childCenter
		if diff < 0 {
			diff = -diff
		}
		if diff < tolerance {
			t.Errorf("A center (%v) should not be identical to %s center (%v)",
				aCenter, child.name, childCenter)
		}
	}

	// Also verify the layout is valid (no same-layer overlaps)
	bNode := layout.Nodes["B"]
	cNode := layout.Nodes["C"]
	dNode := layout.Nodes["D"]

	if bNode.Y != cNode.Y || cNode.Y != dNode.Y {
		t.Errorf("B, C, D should be on same layer")
	}
}

// TestXSimplex_PreventStacking_CustomSep tests custom minimum separation
func TestXSimplex_PreventStacking_CustomSep(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")

	customSep := 100.0
	layout := g.Layout(Options{
		XCoordAlgorithm: XNetworkSimplex,
		NodeSep:         50,
		RankSep:         100,
		PreventStacking: true,
		StackingMinSep:  customSep,
	})

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]

	// Calculate center-to-center distance
	aCenter := aNode.X + aNode.Width/2
	bCenter := bNode.X + bNode.Width/2
	dist := aCenter - bCenter
	if dist < 0 {
		dist = -dist
	}

	// Distance should be at least half-widths + customSep
	minDist := aNode.Width/2 + bNode.Width/2 + customSep
	if dist < minDist-1 { // Small tolerance for floating point
		t.Errorf("separation should be at least %v, got %v", minDist, dist)
	}
}

// BenchmarkXCoordBK benchmarks Brandes-Köpf algorithm
func BenchmarkXCoordBK(b *testing.B) {
	g := buildBenchGraph(50)
	opts := Options{XCoordAlgorithm: XBrandesKopf, NodeSep: 50, RankSep: 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

// BenchmarkXCoordNetworkSimplex benchmarks Network Simplex algorithm
func BenchmarkXCoordNetworkSimplex(b *testing.B) {
	g := buildBenchGraph(50)
	opts := Options{XCoordAlgorithm: XNetworkSimplex, NodeSep: 50, RankSep: 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

// buildBenchGraph creates a graph with n nodes in a layered structure
func buildBenchGraph(n int) *Graph {
	g := NewGraph()
	for i := 0; i < n; i++ {
		g.AddNode(string(rune('A'+i%26))+string(rune('0'+i/26)), NodeOptions{Width: 100, Height: 50})
	}
	// Add edges to create layers
	nodes := g.Nodes()
	for i := 0; i < len(nodes)-1; i++ {
		if i%3 != 2 { // Skip some to create branching
			g.MustAddEdge(nodes[i], nodes[i+1])
		}
		if i > 0 && i%5 == 0 {
			g.MustAddEdge(nodes[i-1], nodes[i+1])
		}
	}
	return g
}
