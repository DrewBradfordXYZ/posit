package posit

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestRoute_SimpleEdge(t *testing.T) {
	// A → B with no dummies
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	edge, ok := layout.Edges["A->B"]
	if !ok {
		t.Fatal("Edge A->B not found")
	}

	// Should have at least start and end points
	if len(edge.Points) < 2 {
		t.Errorf("Expected at least 2 points, got %d", len(edge.Points))
	}

	// Start point should be at or below bottom of A
	aNode := layout.Nodes["A"]
	startY := edge.Points[0].Y
	if startY < aNode.Y {
		t.Errorf("Start point Y=%v should be >= top of A (%v)", startY, aNode.Y)
	}

	// End point should be at or above top of B
	bNode := layout.Nodes["B"]
	endY := edge.Points[len(edge.Points)-1].Y
	if endY > bNode.Y+bNode.Height {
		t.Errorf("End point Y=%v should be <= bottom of B (%v)",
			endY, bNode.Y+bNode.Height)
	}
}

func TestRoute_LongEdgeWithDummies(t *testing.T) {
	// A → B → C → D with long edge A → D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "D")
	g.MustAddEdge("A", "D") // Long edge

	layout := g.Layout()

	edge, ok := layout.Edges["A->D"]
	if !ok {
		t.Fatal("Edge A->D not found")
	}

	// Long edge should have bend points (2 dummies + start + end = 4+ points)
	if len(edge.Points) < 4 {
		t.Errorf("Expected at least 4 points for long edge, got %d",
			len(edge.Points))
	}

	// Points should be ordered by Y (increasing for top-to-bottom layout)
	for i := 1; i < len(edge.Points); i++ {
		if edge.Points[i].Y < edge.Points[i-1].Y {
			t.Errorf("Points not ordered by Y: point %d (Y=%v) < point %d (Y=%v)",
				i, edge.Points[i].Y, i-1, edge.Points[i-1].Y)
		}
	}
}

func TestRoute_ReversedEdge(t *testing.T) {
	// A → B → C → A (cycle, one edge reversed)
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "A") // Will be reversed

	layout := g.Layout()

	// Original edge C→A should be present (restored)
	_, ok := layout.Edges["C->A"]
	if !ok {
		t.Error("Edge C->A not found (should be restored)")
	}

	// Verify all edges exist
	expectedEdges := []string{"A->B", "B->C", "C->A"}
	for _, edgeID := range expectedEdges {
		if _, ok := layout.Edges[edgeID]; !ok {
			t.Errorf("Expected edge %s not found", edgeID)
		}
	}
}

func TestRoute_NoDummiesInOutput(t *testing.T) {
	// Long edge creates dummies, but they shouldn't be in output
	g := NewGraph()
	for i := 0; i < 5; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < 4; i++ {
		g.MustAddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	g.MustAddEdge("N0", "N4") // Long edge, creates dummies

	layout := g.Layout()

	// Only real nodes should be in output
	if len(layout.Nodes) != 5 {
		t.Errorf("Expected 5 nodes, got %d", len(layout.Nodes))
	}

	// Check no dummy IDs
	for id := range layout.Nodes {
		if strings.HasPrefix(id, "_dummy") {
			t.Errorf("Dummy node %s found in output", id)
		}
	}
}

func TestRoute_EdgePointsValid(t *testing.T) {
	// Build a graph with multiple paths
	g := NewGraph()
	for i := 0; i < 10; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	// Create various edge patterns
	g.MustAddEdge("N0", "N1")
	g.MustAddEdge("N0", "N2")
	g.MustAddEdge("N1", "N3")
	g.MustAddEdge("N2", "N3")
	g.MustAddEdge("N3", "N4")
	g.MustAddEdge("N0", "N5") // Skip some layers
	g.MustAddEdge("N5", "N6")
	g.MustAddEdge("N6", "N7")
	g.MustAddEdge("N7", "N8")
	g.MustAddEdge("N8", "N9")

	layout := g.Layout()

	for edgeID, edge := range layout.Edges {
		for i, pt := range edge.Points {
			if math.IsNaN(pt.X) || math.IsInf(pt.X, 0) {
				t.Errorf("Edge %s point %d has invalid X: %v",
					edgeID, i, pt.X)
			}
			if math.IsNaN(pt.Y) || math.IsInf(pt.Y, 0) {
				t.Errorf("Edge %s point %d has invalid Y: %v",
					edgeID, i, pt.Y)
			}
		}
	}
}

func TestRoute_AllEdgesHavePoints(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("A", "C") // Long edge

	layout := g.Layout()

	for edgeID, edge := range layout.Edges {
		if edge.Points == nil {
			t.Errorf("Edge %s has nil Points", edgeID)
		}
		if len(edge.Points) < 2 {
			t.Errorf("Edge %s has fewer than 2 points: %d", edgeID, len(edge.Points))
		}
	}
}

func TestRoute_BoundaryIntersections(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]
	edge := layout.Edges["A->B"]

	// Start point should be on A's boundary
	start := edge.Points[0]
	if !isOnBoundary(start, aNode) {
		t.Errorf("Start point (%v, %v) not on A's boundary", start.X, start.Y)
	}

	// End point should be on B's boundary
	end := edge.Points[len(edge.Points)-1]
	if !isOnBoundary(end, bNode) {
		t.Errorf("End point (%v, %v) not on B's boundary", end.X, end.Y)
	}
}

// isOnBoundary checks if a point is on the boundary of a node (with tolerance).
func isOnBoundary(pt EdgePoint, node NodeLayout) bool {
	const tol = 0.001

	// Check if on left edge
	if math.Abs(pt.X-node.X) < tol && pt.Y >= node.Y-tol && pt.Y <= node.Y+node.Height+tol {
		return true
	}
	// Check if on right edge
	if math.Abs(pt.X-(node.X+node.Width)) < tol && pt.Y >= node.Y-tol && pt.Y <= node.Y+node.Height+tol {
		return true
	}
	// Check if on top edge
	if math.Abs(pt.Y-node.Y) < tol && pt.X >= node.X-tol && pt.X <= node.X+node.Width+tol {
		return true
	}
	// Check if on bottom edge
	if math.Abs(pt.Y-(node.Y+node.Height)) < tol && pt.X >= node.X-tol && pt.X <= node.X+node.Width+tol {
		return true
	}

	return false
}

func TestRoute_IntersectRect(t *testing.T) {
	s := &layoutState{nodes: make(map[string]*layoutNode)}

	node := &layoutNode{
		x:      0,
		y:      0,
		width:  100,
		height: 50,
	}

	tests := []struct {
		name     string
		point    EdgePoint
		expected EdgePoint
	}{
		{
			name:     "directly below",
			point:    EdgePoint{X: 50, Y: 100},
			expected: EdgePoint{X: 50, Y: 50}, // Bottom center
		},
		{
			name:     "directly above",
			point:    EdgePoint{X: 50, Y: -100},
			expected: EdgePoint{X: 50, Y: 0}, // Top center
		},
		{
			name:     "directly right",
			point:    EdgePoint{X: 200, Y: 25},
			expected: EdgePoint{X: 100, Y: 25}, // Right center
		},
		{
			name:     "directly left",
			point:    EdgePoint{X: -100, Y: 25},
			expected: EdgePoint{X: 0, Y: 25}, // Left center
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.intersectRect(node, tt.point)
			if math.Abs(result.X-tt.expected.X) > 0.001 || math.Abs(result.Y-tt.expected.Y) > 0.001 {
				t.Errorf("intersectRect() = (%v, %v), want (%v, %v)",
					result.X, result.Y, tt.expected.X, tt.expected.Y)
			}
		})
	}
}

func TestRoute_ZeroSizeNode(t *testing.T) {
	s := &layoutState{nodes: make(map[string]*layoutNode)}

	// Dummy nodes have zero size
	node := &layoutNode{
		x:      50,
		y:      100,
		width:  0,
		height: 0,
	}

	point := EdgePoint{X: 100, Y: 200}
	result := s.intersectRect(node, point)

	// For zero-size nodes, should return center
	if result.X != 50 || result.Y != 100 {
		t.Errorf("intersectRect for zero-size node = (%v, %v), want (50, 100)",
			result.X, result.Y)
	}
}

func TestRoute_SamePointIntersection(t *testing.T) {
	s := &layoutState{nodes: make(map[string]*layoutNode)}

	node := &layoutNode{
		x:      0,
		y:      0,
		width:  100,
		height: 50,
	}

	// Point at center
	point := EdgePoint{X: 50, Y: 25}
	result := s.intersectRect(node, point)

	// Should return center when point is at center
	if result.X != 50 || result.Y != 25 {
		t.Errorf("intersectRect for same point = (%v, %v), want (50, 25)",
			result.X, result.Y)
	}
}

func TestRoute_MultiLayerLongEdge(t *testing.T) {
	// Create a chain with a very long edge
	g := NewGraph()
	for i := 0; i < 6; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
	}
	// Create chain
	for i := 0; i < 5; i++ {
		g.MustAddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	// Add long edge spanning 5 layers
	g.MustAddEdge("N0", "N5")

	layout := g.Layout()

	edge, ok := layout.Edges["N0->N5"]
	if !ok {
		t.Fatal("Edge N0->N5 not found")
	}

	// Should have 4 bend points (layers 1-4) + start + end = 6 points
	if len(edge.Points) != 6 {
		t.Errorf("Expected 6 points for edge spanning 5 layers, got %d", len(edge.Points))
	}
}

func TestRoute_EdgeDirection(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	layout := g.Layout()

	aNode := layout.Nodes["A"]
	bNode := layout.Nodes["B"]
	edge := layout.Edges["A->B"]

	// First point should be closer to A
	start := edge.Points[0]
	end := edge.Points[len(edge.Points)-1]

	distStartToA := math.Hypot(start.X-(aNode.X+aNode.Width/2), start.Y-(aNode.Y+aNode.Height/2))
	distStartToB := math.Hypot(start.X-(bNode.X+bNode.Width/2), start.Y-(bNode.Y+bNode.Height/2))

	if distStartToA > distStartToB {
		t.Error("Start point should be closer to A than B")
	}

	distEndToA := math.Hypot(end.X-(aNode.X+aNode.Width/2), end.Y-(aNode.Y+aNode.Height/2))
	distEndToB := math.Hypot(end.X-(bNode.X+bNode.Width/2), end.Y-(bNode.Y+bNode.Height/2))

	if distEndToB > distEndToA {
		t.Error("End point should be closer to B than A")
	}
}
