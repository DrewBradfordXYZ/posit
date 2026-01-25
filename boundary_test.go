package posit

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestIntersectLineRect_ExitRight(t *testing.T) {
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25} // center
	to := EdgePoint{X: 200, Y: 25}  // directly right
	result := IntersectLineRect(from, to, r)

	if result.Side != Right {
		t.Errorf("expected Side=Right, got %v", result.Side)
	}
	if result.Point.X != 100 {
		t.Errorf("expected Point.X=100, got %v", result.Point.X)
	}
	if result.Point.Y != 25 {
		t.Errorf("expected Point.Y=25, got %v", result.Point.Y)
	}
	if result.Offset != 25 {
		t.Errorf("expected Offset=25 (Y from top), got %v", result.Offset)
	}
}

func TestIntersectLineRect_ExitLeft(t *testing.T) {
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25} // center
	to := EdgePoint{X: -100, Y: 25} // directly left
	result := IntersectLineRect(from, to, r)

	if result.Side != Left {
		t.Errorf("expected Side=Left, got %v", result.Side)
	}
	if !floatEq(result.Point.X, 0) {
		t.Errorf("expected Point.X≈0, got %v", result.Point.X)
	}
	if !floatEq(result.Offset, 25) {
		t.Errorf("expected Offset≈25 (Y from top), got %v", result.Offset)
	}
}

func TestIntersectLineRect_ExitBottom(t *testing.T) {
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25} // center
	to := EdgePoint{X: 50, Y: 100}  // directly down
	result := IntersectLineRect(from, to, r)

	if result.Side != Bottom {
		t.Errorf("expected Side=Bottom, got %v", result.Side)
	}
	if result.Point.Y != 50 {
		t.Errorf("expected Point.Y=50, got %v", result.Point.Y)
	}
	if result.Point.X != 50 {
		t.Errorf("expected Point.X=50, got %v", result.Point.X)
	}
	if result.Offset != 50 {
		t.Errorf("expected Offset=50 (X from left), got %v", result.Offset)
	}
}

func TestIntersectLineRect_ExitTop(t *testing.T) {
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25} // center
	to := EdgePoint{X: 50, Y: -50}  // directly up
	result := IntersectLineRect(from, to, r)

	if result.Side != Top {
		t.Errorf("expected Side=Top, got %v", result.Side)
	}
	if !floatEq(result.Point.Y, 0) {
		t.Errorf("expected Point.Y≈0, got %v", result.Point.Y)
	}
	if !floatEq(result.Offset, 50) {
		t.Errorf("expected Offset≈50 (X from left), got %v", result.Offset)
	}
}

func TestIntersectLineRect_Diagonal_ExitRight(t *testing.T) {
	// Wide rectangle: diagonal should exit through right side
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25}  // center
	to := EdgePoint{X: 200, Y: 100}  // diagonal right+down
	result := IntersectLineRect(from, to, r)

	// For a 100x50 rect, moving 150 right and 75 down from center at (50,25):
	// To reach right edge (x=100): need t where 50 + t*150 = 100 → t = 50/150 = 1/3
	// At t=1/3: y = 25 + (1/3)*75 = 50
	// To reach bottom edge (y=50): need t where 25 + t*75 = 50 → t = 25/75 = 1/3
	// Both edges are reached at the same time! This is the corner case.
	// The algorithm should pick one - let's verify it's consistent.
	if result.Side != Right && result.Side != Bottom {
		t.Errorf("expected Side=Right or Bottom (corner), got %v", result.Side)
	}
}

func TestIntersectLineRect_Diagonal_ExitBottom(t *testing.T) {
	// Tall rectangle: diagonal should exit through bottom side
	r := Rect{Left: 0, Right: 50, Top: 0, Bottom: 100}
	from := EdgePoint{X: 25, Y: 50}   // center
	to := EdgePoint{X: 100, Y: 200}   // diagonal right+down
	result := IntersectLineRect(from, to, r)

	// For a 50x100 rect, moving 75 right and 150 down from center at (25,50):
	// To reach right edge (x=50): t = 25/75 = 1/3
	// At t=1/3: y = 50 + 50 = 100 → exactly at bottom
	// To reach bottom edge (y=100): t = 50/150 = 1/3
	// Again a corner case - both sides at same t
	if result.Side != Right && result.Side != Bottom {
		t.Errorf("expected Side=Right or Bottom (corner), got %v", result.Side)
	}
}

func TestIntersectLineRect_Diagonal_ClearRight(t *testing.T) {
	// Wide rectangle, shallow diagonal → clearly exits right
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25}  // center
	to := EdgePoint{X: 200, Y: 30}   // mostly horizontal, slight down
	result := IntersectLineRect(from, to, r)

	if result.Side != Right {
		t.Errorf("expected Side=Right for shallow diagonal, got %v", result.Side)
	}
}

func TestIntersectLineRect_Diagonal_ClearBottom(t *testing.T) {
	// Flat rectangle, steep diagonal → clearly exits bottom
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25}  // center
	to := EdgePoint{X: 55, Y: 200}   // mostly vertical, slight right
	result := IntersectLineRect(from, to, r)

	if result.Side != Bottom {
		t.Errorf("expected Side=Bottom for steep diagonal, got %v", result.Side)
	}
}

func TestIntersectLineRect_ZeroMovement(t *testing.T) {
	// When from == to, should return a sensible default
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
	from := EdgePoint{X: 50, Y: 25}
	to := EdgePoint{X: 50, Y: 25}
	result := IntersectLineRect(from, to, r)

	// Should get a default (right side, center)
	if result.Side != Right {
		t.Errorf("expected default Side=Right for zero movement, got %v", result.Side)
	}
}

func TestIntersectLineRect_OverlappingNodes(t *testing.T) {
	// When target is inside source bounds, intersection still works
	// The ray direction determines exit point
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 100}
	from := EdgePoint{X: 50, Y: 50}  // center of source
	to := EdgePoint{X: 75, Y: 50}    // inside source but to the right
	result := IntersectLineRect(from, to, r)

	if result.Side != Right {
		t.Errorf("expected Side=Right for target inside but to right, got %v", result.Side)
	}
}

func TestIntersectLineRect_OffsetAccuracy(t *testing.T) {
	// Verify offset calculation is accurate
	r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 100}
	from := EdgePoint{X: 50, Y: 50}  // center
	to := EdgePoint{X: 200, Y: 75}   // right and slightly down
	result := IntersectLineRect(from, to, r)

	if result.Side != Right {
		t.Errorf("expected Side=Right, got %v", result.Side)
	}

	// Calculate expected Y at x=100:
	// t = (100 - 50) / (200 - 50) = 50/150 = 1/3
	// y = 50 + (1/3)*(75-50) = 50 + 25/3 ≈ 58.33
	expectedY := 50.0 + (1.0/3.0)*25.0
	expectedOffset := expectedY - 0 // offset from top

	if math.Abs(result.Offset-expectedOffset) > 0.001 {
		t.Errorf("expected Offset≈%.3f, got %.3f", expectedOffset, result.Offset)
	}
}

func TestInferSideFromBoundary_HorizontalGap(t *testing.T) {
	// Two nodes with clear horizontal gap
	fromNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	toNode := &layoutNode{x: 200, y: 0, width: 100, height: 50}

	srcSide, tgtSide := inferSideFromBoundary(fromNode, toNode)

	if srcSide != Right {
		t.Errorf("expected srcSide=Right, got %v", srcSide)
	}
	if tgtSide != Left {
		t.Errorf("expected tgtSide=Left, got %v", tgtSide)
	}
}

func TestInferSideFromBoundary_VerticalGap(t *testing.T) {
	// Two nodes with clear vertical gap
	fromNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	toNode := &layoutNode{x: 0, y: 150, width: 100, height: 50}

	srcSide, tgtSide := inferSideFromBoundary(fromNode, toNode)

	if srcSide != Bottom {
		t.Errorf("expected srcSide=Bottom, got %v", srcSide)
	}
	if tgtSide != Top {
		t.Errorf("expected tgtSide=Top, got %v", tgtSide)
	}
}

func TestInferSideFromBoundary_Diagonal(t *testing.T) {
	// Two nodes diagonal from each other
	fromNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	toNode := &layoutNode{x: 200, y: 150, width: 100, height: 50}

	srcSide, tgtSide := inferSideFromBoundary(fromNode, toNode)

	// For diagonal, the side depends on angle and aspect ratio
	// From (50,25) to (250,175): dx=200, dy=150
	// Source aspect ratio: 100x50 (wide, but shallow)
	//
	// Ray from source center (50,25) toward target center (250,175):
	// - Right edge at x=100: t = 50/200 = 0.25, y = 25 + 0.25*150 = 62.5 (outside bottom=50)
	// - Bottom edge at y=50: t = 25/150 = 0.167, x = 50 + 0.167*200 = 83.33 (inside right=100)
	// Bottom edge is reached first! Source exits through bottom.
	//
	// Ray from target center (250,175) toward source center (50,25):
	// - Left edge at x=200: t = 50/(-200) = -0.25 (negative, wrong direction)
	// - Top edge at y=150: t = 25/(-150) = -0.167 (negative, wrong direction)
	// Wait, for toNode going toward fromNode: dx=-200, dy=-150
	// - Left edge at x=200: t = (200-250)/(-200) = 0.25, y = 175 + 0.25*(-150) = 137.5 (inside 150-200)
	// - Top edge at y=150: t = (150-175)/(-150) = 0.167, x = 250 + 0.167*(-200) = 216.67 (inside 200-300)
	// Top edge is reached first! Target exits through top.
	if srcSide != Bottom {
		t.Errorf("expected srcSide=Bottom for this diagonal, got %v", srcSide)
	}
	if tgtSide != Top {
		t.Errorf("expected tgtSide=Top for this diagonal, got %v", tgtSide)
	}
}

func TestNodeRect(t *testing.T) {
	node := &layoutNode{x: 10, y: 20, width: 100, height: 50}
	r := nodeRect(node)

	if r.Left != 10 {
		t.Errorf("expected Left=10, got %v", r.Left)
	}
	if r.Right != 110 {
		t.Errorf("expected Right=110, got %v", r.Right)
	}
	if r.Top != 20 {
		t.Errorf("expected Top=20, got %v", r.Top)
	}
	if r.Bottom != 70 {
		t.Errorf("expected Bottom=70, got %v", r.Bottom)
	}
}

// ==================== Port Boundary Selection Tests ====================

func TestEdgePortSideBoundary_HorizontalAxis_RightTarget(t *testing.T) {
	// Port on node A with PortAxisHorizontal, target B is to the right
	s := &layoutState{
		opts: Options{Direction: TopToBottom},
	}
	thisNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	connNode := &layoutNode{x: 200, y: 0, width: 100, height: 50}
	port := &PortOptions{
		Axis:   PortAxisHorizontal,
		Offset: 25, // Middle of height
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// Target is to the right, so port should be on Right side
	if side != Right {
		t.Errorf("expected Right (target to right), got %v", side)
	}
}

func TestEdgePortSideBoundary_HorizontalAxis_LeftTarget(t *testing.T) {
	// Port on node A with PortAxisHorizontal, target B is to the left
	s := &layoutState{
		opts: Options{Direction: TopToBottom},
	}
	thisNode := &layoutNode{x: 200, y: 0, width: 100, height: 50}
	connNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	port := &PortOptions{
		Axis:   PortAxisHorizontal,
		Offset: 25,
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// Target is to the left, so port should be on Left side
	if side != Left {
		t.Errorf("expected Left (target to left), got %v", side)
	}
}

func TestEdgePortSideBoundary_VerticalAxis_BottomTarget(t *testing.T) {
	// Port on node A with PortAxisVertical, target B is below
	s := &layoutState{
		opts: Options{Direction: TopToBottom},
	}
	thisNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	connNode := &layoutNode{x: 0, y: 150, width: 100, height: 50}
	port := &PortOptions{
		Axis:   PortAxisVertical,
		Offset: 50, // Middle of width
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// Target is below, so port should be on Bottom side
	if side != Bottom {
		t.Errorf("expected Bottom (target below), got %v", side)
	}
}

func TestEdgePortSideBoundary_VerticalAxis_TopTarget(t *testing.T) {
	// Port on node A with PortAxisVertical, target B is above
	s := &layoutState{
		opts: Options{Direction: TopToBottom},
	}
	thisNode := &layoutNode{x: 0, y: 150, width: 100, height: 50}
	connNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	port := &PortOptions{
		Axis:   PortAxisVertical,
		Offset: 50,
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// Target is above, so port should be on Top side
	if side != Top {
		t.Errorf("expected Top (target above), got %v", side)
	}
}

func TestEdgePortSideBoundary_AxisAny_Diagonal(t *testing.T) {
	// Port with no axis constraint, target is diagonal
	s := &layoutState{
		opts: Options{Direction: TopToBottom},
	}
	thisNode := &layoutNode{x: 0, y: 0, width: 100, height: 50}
	connNode := &layoutNode{x: 200, y: 150, width: 100, height: 50}
	port := &PortOptions{
		Axis: PortAxisAny,
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// Diagonal target - should pick the side based on boundary intersection
	// For a 100x50 rectangle, target at (250, 175), the ray will likely exit Right or Bottom
	if side != Right && side != Bottom {
		t.Errorf("expected Right or Bottom (diagonal target), got %v", side)
	}
}

func TestEdgePortSideBoundary_LeftToRight_Direction(t *testing.T) {
	// Test with LeftToRight direction - coordinate transforms should work
	s := &layoutState{
		opts: Options{Direction: LeftToRight},
	}
	// In LTR internal space, "below" means "to the right" in user space
	thisNode := &layoutNode{x: 0, y: 0, width: 50, height: 100} // Note: swapped for LTR
	connNode := &layoutNode{x: 0, y: 200, width: 50, height: 100}
	port := &PortOptions{
		Axis:   PortAxisHorizontal, // Horizontal in user space = vertical sides
		Offset: 50,
	}

	side := s.edgePortSideBoundary(port, thisNode, connNode)

	// In user space (LTR), the port should be on Right side
	if side != Right {
		t.Errorf("expected Right in LTR direction, got %v", side)
	}
}

func TestAssignPortSideFromBoundary_SingleConnection(t *testing.T) {
	// Test voting with single connected node
	nodeA := &layoutNode{id: "A", x: 0, y: 0, width: 100, height: 50}
	nodeB := &layoutNode{id: "B", x: 200, y: 0, width: 100, height: 50}

	edgeAB := &layoutEdge{key: edgeKey{from: "A", to: "B"}, sourcePort: "p1"}
	s := &layoutState{
		opts:  Options{Direction: TopToBottom},
		nodes: map[string]*layoutNode{"A": nodeA, "B": nodeB},
		edges: map[edgeKey]*layoutEdge{edgeAB.key: edgeAB},
	}

	port := &PortOptions{
		ID:   "p1",
		Axis: PortAxisHorizontal,
	}

	side := s.assignPortSideFromBoundary(nodeA, port)

	// B is to the right, should vote for Right
	if side != Right {
		t.Errorf("expected Right (single vote for right target), got %v", side)
	}
}

func TestAssignPortSideFromBoundary_MultipleConnections_Voting(t *testing.T) {
	// Test voting with multiple connected nodes - majority wins
	nodeA := &layoutNode{id: "A", x: 100, y: 100, width: 100, height: 50}
	nodeB := &layoutNode{id: "B", x: 300, y: 100, width: 100, height: 50} // Right
	nodeC := &layoutNode{id: "C", x: 300, y: 200, width: 100, height: 50} // Right-below
	nodeD := &layoutNode{id: "D", x: 0, y: 100, width: 50, height: 50}    // Left

	edgeAB := &layoutEdge{key: edgeKey{from: "A", to: "B"}, sourcePort: "p1"}
	edgeAC := &layoutEdge{key: edgeKey{from: "A", to: "C"}, sourcePort: "p1"}
	edgeAD := &layoutEdge{key: edgeKey{from: "A", to: "D"}, sourcePort: "p1"}
	s := &layoutState{
		opts:  Options{Direction: TopToBottom},
		nodes: map[string]*layoutNode{"A": nodeA, "B": nodeB, "C": nodeC, "D": nodeD},
		edges: map[edgeKey]*layoutEdge{
			edgeAB.key: edgeAB,
			edgeAC.key: edgeAC,
			edgeAD.key: edgeAD,
		},
	}

	port := &PortOptions{
		ID:   "p1",
		Axis: PortAxisHorizontal,
	}

	side := s.assignPortSideFromBoundary(nodeA, port)

	// B and C vote Right, D votes Left → Right wins
	if side != Right {
		t.Errorf("expected Right (2 votes right, 1 vote left), got %v", side)
	}
}

func TestAssignPortSideFromBoundary_NoConnections_DefaultSide(t *testing.T) {
	// Test that empty connections returns default side
	nodeA := &layoutNode{id: "A", x: 0, y: 0, width: 100, height: 50}

	s := &layoutState{
		opts:  Options{Direction: TopToBottom},
		nodes: map[string]*layoutNode{"A": nodeA},
		edges: map[edgeKey]*layoutEdge{}, // No edges
	}

	// Test horizontal axis - should default to Right
	portH := &PortOptions{
		ID:   "p1",
		Axis: PortAxisHorizontal,
	}
	sideH := s.assignPortSideFromBoundary(nodeA, portH)
	if sideH != Right {
		t.Errorf("expected Right (default for horizontal), got %v", sideH)
	}

	// Test vertical axis - should default to Bottom
	portV := &PortOptions{
		ID:   "p2",
		Axis: PortAxisVertical,
	}
	sideV := s.assignPortSideFromBoundary(nodeA, portV)
	if sideV != Bottom {
		t.Errorf("expected Bottom (default for vertical), got %v", sideV)
	}
}

func TestAssignPortSideFromBoundary_TargetPort(t *testing.T) {
	// Test that target ports also work (edge.targetPort matches)
	nodeA := &layoutNode{id: "A", x: 200, y: 0, width: 100, height: 50}
	nodeB := &layoutNode{id: "B", x: 0, y: 0, width: 100, height: 50}

	edgeBA := &layoutEdge{key: edgeKey{from: "B", to: "A"}, targetPort: "p1"}
	s := &layoutState{
		opts:  Options{Direction: TopToBottom},
		nodes: map[string]*layoutNode{"A": nodeA, "B": nodeB},
		edges: map[edgeKey]*layoutEdge{edgeBA.key: edgeBA},
	}

	port := &PortOptions{
		ID:   "p1",
		Axis: PortAxisHorizontal,
	}

	side := s.assignPortSideFromBoundary(nodeA, port)

	// B is to the left of A, so port on A should be on Left
	if side != Left {
		t.Errorf("expected Left (source B is to the left), got %v", side)
	}
}
