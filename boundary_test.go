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
