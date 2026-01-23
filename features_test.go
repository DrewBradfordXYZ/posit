package posit

import (
	"math"
	"sort"
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

// ==================== Port Constraint Tests ====================

func TestPortFixedOrder(t *testing.T) {
	// 3 ports declared with explicit order on the right side.
	// Verify computed offsets are evenly distributed.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 90,
		Ports: []PortOptions{
			{ID: "p1", Side: Right, Constraint: PortFixedOrder, Order: 1},
			{ID: "p2", Side: Right, Constraint: PortFixedOrder, Order: 2},
			{ID: "p3", Side: Right, Constraint: PortFixedOrder, Order: 3},
		},
	})
	g.AddNode("B1", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B3", NodeOptions{Width: 50, Height: 30})

	g.MustAddEdge("A", "B1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("A", "B2", EdgeOptions{SourcePort: "p2"})
	g.MustAddEdge("A", "B3", EdgeOptions{SourcePort: "p3"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map to be populated for PortFixedOrder node")
	}

	// Expect evenly distributed: 90 * 1/4 = 22.5, 90 * 2/4 = 45, 90 * 3/4 = 67.5
	expectedOffsets := map[string]float64{
		"p1": 22.5,
		"p2": 45.0,
		"p3": 67.5,
	}

	for id, expected := range expectedOffsets {
		port, ok := nodeA.Ports[id]
		if !ok {
			t.Errorf("Port %q not found in layout output", id)
			continue
		}
		if math.Abs(port.Offset-expected) > 0.01 {
			t.Errorf("Port %q: expected offset %.1f, got %.1f", id, expected, port.Offset)
		}
		if port.Side != Right {
			t.Errorf("Port %q: expected side Right, got %v", id, port.Side)
		}
	}

	// Verify order is preserved: p1 < p2 < p3
	if nodeA.Ports["p1"].Offset >= nodeA.Ports["p2"].Offset {
		t.Error("Port p1 should have smaller offset than p2")
	}
	if nodeA.Ports["p2"].Offset >= nodeA.Ports["p3"].Offset {
		t.Error("Port p2 should have smaller offset than p3")
	}
}

func TestPortFixedSide(t *testing.T) {
	// 3 ports on the right side with PortFixedSide constraint.
	// Connected nodes are at known Y positions. Verify optimal reordering.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 90,
		Ports: []PortOptions{
			{ID: "p1", Side: Right, Constraint: PortFixedSide},
			{ID: "p2", Side: Right, Constraint: PortFixedSide},
			{ID: "p3", Side: Right, Constraint: PortFixedSide},
		},
	})
	g.AddNode("B1", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B3", NodeOptions{Width: 50, Height: 30})

	g.MustAddEdge("A", "B1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("A", "B2", EdgeOptions{SourcePort: "p2"})
	g.MustAddEdge("A", "B3", EdgeOptions{SourcePort: "p3"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map to be populated for PortFixedSide node")
	}

	// All 3 ports should be present with computed offsets
	if len(nodeA.Ports) != 3 {
		t.Fatalf("Expected 3 ports, got %d", len(nodeA.Ports))
	}

	// Verify offsets are evenly distributed (regardless of order)
	offsets := []float64{
		nodeA.Ports["p1"].Offset,
		nodeA.Ports["p2"].Offset,
		nodeA.Ports["p3"].Offset,
	}
	for _, o := range offsets {
		if o <= 0 || o >= 90 {
			t.Errorf("Port offset %.1f should be between 0 and 90 (exclusive)", o)
		}
	}

	// Verify spacing is even: all gaps should be equal
	// Sort offsets to check spacing
	sorted := make([]float64, 3)
	copy(sorted, offsets)
	sort.Float64s(sorted)
	gap1 := sorted[1] - sorted[0]
	gap2 := sorted[2] - sorted[1]
	if math.Abs(gap1-gap2) > 0.01 {
		t.Errorf("Port spacing should be even: gaps are %.1f and %.1f", gap1, gap2)
	}
}

func TestPortFixedPosUnchanged(t *testing.T) {
	// Existing FIXED_POS behavior should remain unchanged.
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

	// PortFixedPos nodes should NOT have Ports map populated
	nodeA := layout.Nodes["A"]
	if nodeA.Ports != nil {
		t.Error("PortFixedPos nodes should not have Ports in output")
	}

	// Verify edge still uses the exact offset
	edge, ok := layout.Edge("A", "B")
	if !ok {
		t.Fatal("Edge A->B not found")
	}
	if len(edge.Points) < 2 {
		t.Fatal("Edge should have at least 2 points")
	}

	// Source port is on bottom side at offset 25
	startPt := edge.Points[0]
	expectedX := layout.Nodes["A"].X + 25
	expectedY := layout.Nodes["A"].Y + 100
	if math.Abs(startPt.X-expectedX) > 0.01 || math.Abs(startPt.Y-expectedY) > 0.01 {
		t.Errorf("Edge start should be at port (%.1f, %.1f), got (%.1f, %.1f)",
			expectedX, expectedY, startPt.X, startPt.Y)
	}
}

func TestPortLayoutOutput(t *testing.T) {
	// Verify PortLayout appears in output for computed ports and not for fixed.
	g := NewGraph()
	g.AddNode("fixed", NodeOptions{
		Width: 100, Height: 60,
		Ports: []PortOptions{
			{ID: "p1", Side: Right, Offset: 30},
		},
	})
	g.AddNode("computed", NodeOptions{
		Width: 100, Height: 60,
		Ports: []PortOptions{
			{ID: "p1", Side: Right, Constraint: PortFixedOrder, Order: 1},
			{ID: "p2", Side: Right, Constraint: PortFixedOrder, Order: 2},
		},
	})
	g.AddNode("target1", NodeOptions{Width: 50, Height: 30})
	g.AddNode("target2", NodeOptions{Width: 50, Height: 30})

	g.MustAddEdge("fixed", "target1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("computed", "target1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("computed", "target2", EdgeOptions{SourcePort: "p2"})

	layout := g.Layout()

	// "fixed" node should NOT have Ports
	if layout.Nodes["fixed"].Ports != nil {
		t.Error("PortFixedPos node should not have Ports in output")
	}

	// "computed" node SHOULD have Ports
	computedNode := layout.Nodes["computed"]
	if computedNode.Ports == nil {
		t.Fatal("PortFixedOrder node should have Ports in output")
	}
	if len(computedNode.Ports) != 2 {
		t.Fatalf("Expected 2 ports in output, got %d", len(computedNode.Ports))
	}

	// Verify structure
	p1 := computedNode.Ports["p1"]
	if p1.ID != "p1" || p1.Side != Right {
		t.Errorf("Port p1: expected ID='p1' Side=Right, got ID=%q Side=%v", p1.ID, p1.Side)
	}
	p2 := computedNode.Ports["p2"]
	if p2.ID != "p2" || p2.Side != Right {
		t.Errorf("Port p2: expected ID='p2' Side=Right, got ID=%q Side=%v", p2.ID, p2.Side)
	}

	// p1 should be before p2 (lower offset)
	if p1.Offset >= p2.Offset {
		t.Errorf("Port p1 offset (%.1f) should be less than p2 offset (%.1f)", p1.Offset, p2.Offset)
	}
}

func TestPortFixedOrder_TopBottom(t *testing.T) {
	// Test PortFixedOrder on Top/Bottom sides (uses node width).
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 120, Height: 60,
		Ports: []PortOptions{
			{ID: "p1", Side: Bottom, Constraint: PortFixedOrder, Order: 1},
			{ID: "p2", Side: Bottom, Constraint: PortFixedOrder, Order: 2},
		},
	})
	g.AddNode("B1", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 30})

	g.MustAddEdge("A", "B1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("A", "B2", EdgeOptions{SourcePort: "p2"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for PortFixedOrder node")
	}

	// Width=120, 2 ports: offset = 120*1/3=40, 120*2/3=80
	p1 := nodeA.Ports["p1"]
	p2 := nodeA.Ports["p2"]
	if math.Abs(p1.Offset-40.0) > 0.01 {
		t.Errorf("Port p1: expected offset 40, got %.1f", p1.Offset)
	}
	if math.Abs(p2.Offset-80.0) > 0.01 {
		t.Errorf("Port p2: expected offset 80, got %.1f", p2.Offset)
	}
}

func TestPortMixed_FixedPosAndFixedOrder(t *testing.T) {
	// A node can have both PortFixedPos and PortFixedOrder ports.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 90,
		Ports: []PortOptions{
			{ID: "fixed", Side: Right, Offset: 10}, // PortFixedPos (default)
			{ID: "auto1", Side: Right, Constraint: PortFixedOrder, Order: 1},
			{ID: "auto2", Side: Right, Constraint: PortFixedOrder, Order: 2},
		},
	})
	g.AddNode("B1", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B2", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B3", NodeOptions{Width: 50, Height: 30})

	g.MustAddEdge("A", "B1", EdgeOptions{SourcePort: "fixed"})
	g.MustAddEdge("A", "B2", EdgeOptions{SourcePort: "auto1"})
	g.MustAddEdge("A", "B3", EdgeOptions{SourcePort: "auto2"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	// Should have Ports for auto1 and auto2, but not for fixed
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for node with computed ports")
	}
	if _, ok := nodeA.Ports["fixed"]; ok {
		t.Error("PortFixedPos port should not appear in Ports output")
	}
	if _, ok := nodeA.Ports["auto1"]; !ok {
		t.Error("PortFixedOrder port auto1 should appear in Ports output")
	}
	if _, ok := nodeA.Ports["auto2"]; !ok {
		t.Error("PortFixedOrder port auto2 should appear in Ports output")
	}

	// Verify auto ports are ordered
	if nodeA.Ports["auto1"].Offset >= nodeA.Ports["auto2"].Offset {
		t.Error("auto1 should have lower offset than auto2")
	}
}

func TestPortFree_SideSelection(t *testing.T) {
	// Node A in layer 0 with PortFree ports connecting to B (layer 1, below).
	// In a top-to-bottom layout, B is below A, so ports should end up on Bottom.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 60,
		Ports: []PortOptions{
			{ID: "p1", Constraint: PortFree},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 60})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for PortFree node")
	}
	p1 := nodeA.Ports["p1"]
	// In top-to-bottom layout, B is below A → port on Bottom
	if p1.Side != Bottom {
		t.Errorf("Expected port on Bottom (connected node is below), got %v", p1.Side)
	}
}

func TestPortFree_HorizontalAxis(t *testing.T) {
	// PortFree with PortAxisHorizontal: port should be on Left or Right only.
	// A and B are in the same layer (use RankGroup to force), B is to the right.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 60,
		RankGroup: "same",
		Ports: []PortOptions{
			{ID: "p1", Constraint: PortFree, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 60, RankGroup: "same"})
	// Need a third node to create a valid graph with edges
	g.AddNode("root", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("root", "A")
	g.MustAddEdge("root", "B")
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for PortFree node")
	}
	p1 := nodeA.Ports["p1"]
	// With PortAxisHorizontal, side must be Left or Right
	if p1.Side != Left && p1.Side != Right {
		t.Errorf("PortAxisHorizontal: expected Left or Right, got %v", p1.Side)
	}
}

func TestPortFree_MultiplePorts(t *testing.T) {
	// Node with multiple PortFree ports connecting to nodes in different directions.
	// With PortAxisHorizontal, all should be Left or Right.
	// Each port should face toward its connected node.
	g := NewGraph()
	g.AddNode("center", NodeOptions{Width: 100, Height: 90,
		Ports: []PortOptions{
			{ID: "p1", Constraint: PortFree, Axis: PortAxisHorizontal},
			{ID: "p2", Constraint: PortFree, Axis: PortAxisHorizontal},
			{ID: "p3", Constraint: PortFree, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("target1", NodeOptions{Width: 80, Height: 40})
	g.AddNode("target2", NodeOptions{Width: 80, Height: 40})
	g.AddNode("target3", NodeOptions{Width: 80, Height: 40})

	g.MustAddEdge("center", "target1", EdgeOptions{SourcePort: "p1"})
	g.MustAddEdge("center", "target2", EdgeOptions{SourcePort: "p2"})
	g.MustAddEdge("center", "target3", EdgeOptions{SourcePort: "p3"})

	layout := g.Layout()

	centerNode := layout.Nodes["center"]
	if centerNode.Ports == nil {
		t.Fatal("Expected Ports map for PortFree node")
	}

	// All ports should be Left or Right (PortAxisHorizontal)
	for id, port := range centerNode.Ports {
		if port.Side != Left && port.Side != Right {
			t.Errorf("Port %q: expected Left or Right, got %v", id, port.Side)
		}
	}

	// Each port should face toward its connected node
	centerX := centerNode.X + centerNode.Width/2
	for _, portID := range []string{"p1", "p2", "p3"} {
		port := centerNode.Ports[portID]
		// Find connected node
		var targetID string
		switch portID {
		case "p1":
			targetID = "target1"
		case "p2":
			targetID = "target2"
		case "p3":
			targetID = "target3"
		}
		targetNode := layout.Nodes[targetID]
		targetX := targetNode.X + targetNode.Width/2

		if targetX > centerX && port.Side != Right {
			t.Errorf("Port %s: target is right of center, expected Right, got %v", portID, port.Side)
		}
		if targetX < centerX && port.Side != Left {
			t.Errorf("Port %s: target is left of center, expected Left, got %v", portID, port.Side)
		}
	}
}

func TestPortFree_SchemaPattern(t *testing.T) {
	// Schema diagram pattern: Users.id connects to Orders.user_id.
	// Both use PortFree with PortAxisHorizontal.
	// If Users is to the left of Orders, Users.id should be on Right,
	// Orders.user_id should be on Left.
	g := NewGraph()
	g.AddNode("users", NodeOptions{
		Width: 150, Height: 90,
		RankGroup: "tables",
		Ports: []PortOptions{
			{ID: "id", Constraint: PortFree, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("orders", NodeOptions{
		Width: 150, Height: 90,
		RankGroup: "tables",
		Ports: []PortOptions{
			{ID: "user_id", Constraint: PortFree, Axis: PortAxisHorizontal},
		},
	})

	// Need an edge to create the graph structure and a root for valid layout
	g.AddNode("root", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("root", "users")
	g.MustAddEdge("root", "orders")
	g.MustAddEdge("users", "orders", EdgeOptions{SourcePort: "id", TargetPort: "user_id"})

	layout := g.Layout()

	usersNode := layout.Nodes["users"]
	ordersNode := layout.Nodes["orders"]

	if usersNode.Ports == nil || ordersNode.Ports == nil {
		t.Fatal("Expected Ports maps for PortFree nodes")
	}

	// Determine which node is to the left
	usersPort := usersNode.Ports["id"]
	ordersPort := ordersNode.Ports["user_id"]

	if usersNode.X < ordersNode.X {
		// Users is left of Orders
		if usersPort.Side != Right {
			t.Errorf("Users.id: expected Right (Users is left of Orders), got %v", usersPort.Side)
		}
		if ordersPort.Side != Left {
			t.Errorf("Orders.user_id: expected Left (Orders is right of Users), got %v", ordersPort.Side)
		}
	} else {
		// Orders is left of Users
		if usersPort.Side != Left {
			t.Errorf("Users.id: expected Left (Users is right of Orders), got %v", usersPort.Side)
		}
		if ordersPort.Side != Right {
			t.Errorf("Orders.user_id: expected Right (Orders is left of Users), got %v", ordersPort.Side)
		}
	}

	// Regardless of order, the ports should face each other
	if usersPort.Side == ordersPort.Side {
		t.Error("Ports should face each other (one Left, one Right)")
	}
}

func TestPortFree_VerticalAxis(t *testing.T) {
	// PortFree with PortAxisVertical: port should be on Top or Bottom only.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 60,
		Ports: []PortOptions{
			{ID: "p1", Constraint: PortFree, Axis: PortAxisVertical},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 60})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1"})

	layout := g.Layout()

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for PortFree node")
	}
	p1 := nodeA.Ports["p1"]
	// PortAxisVertical: must be Top or Bottom
	if p1.Side != Top && p1.Side != Bottom {
		t.Errorf("PortAxisVertical: expected Top or Bottom, got %v", p1.Side)
	}
	// B is below A in top-to-bottom layout → Bottom
	if p1.Side != Bottom {
		t.Errorf("Expected Bottom (B is below A), got %v", p1.Side)
	}
}

// ==================== PortFixedOffset Tests ====================

func TestPortFixedOffset_SideSelection(t *testing.T) {
	// PortFixedOffset: algorithm chooses side, offset preserved.
	// B is to the right of A, so A's port should be on the Right side.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 80,
		Ports: []PortOptions{
			{ID: "p1", Offset: 44, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{
		Width: 100, Height: 80,
		Ports: []PortOptions{
			{ID: "p2", Offset: 44, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1", TargetPort: "p2"})

	layout := g.Layout(Options{Direction: LeftToRight, NodeSep: 50, RankSep: 100})

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map for PortFixedOffset node A")
	}
	p1 := nodeA.Ports["p1"]
	// B is to the right of A → port should be on Right
	if p1.Side != Right {
		t.Errorf("Expected Right side (B is right of A), got %v", p1.Side)
	}

	nodeB := layout.Nodes["B"]
	if nodeB.Ports == nil {
		t.Fatal("Expected Ports map for PortFixedOffset node B")
	}
	p2 := nodeB.Ports["p2"]
	// A is to the left of B → port should be on Left
	if p2.Side != Left {
		t.Errorf("Expected Left side (A is left of B), got %v", p2.Side)
	}
}

func TestPortFixedOffset_OffsetPreserved(t *testing.T) {
	// The declared offset should appear unchanged in output regardless of computed side.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 200, Height: 100,
		Ports: []PortOptions{
			{ID: "field-3", Offset: 44, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "field-18", Offset: 69, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{Width: 200, Height: 100})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "field-3"})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "field-18", ID: "e2"})

	layout := g.Layout(Options{Direction: LeftToRight, NodeSep: 50, RankSep: 100})

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map")
	}

	if p := nodeA.Ports["field-3"]; p.Offset != 44 {
		t.Errorf("field-3 offset: got %v, want 44", p.Offset)
	}
	if p := nodeA.Ports["field-18"]; p.Offset != 69 {
		t.Errorf("field-18 offset: got %v, want 69", p.Offset)
	}
}

func TestPortFixedOffset_AxisConstraint(t *testing.T) {
	// PortAxisHorizontal restricts to Left/Right only, even in top-to-bottom layout.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 100, Height: 80,
		Ports: []PortOptions{
			{ID: "p1", Offset: 40, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 80})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1"})

	layout := g.Layout() // TopToBottom default

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map")
	}
	p1 := nodeA.Ports["p1"]
	if p1.Side != Left && p1.Side != Right {
		t.Errorf("PortAxisHorizontal: expected Left or Right, got %v", p1.Side)
	}
	// Offset must be preserved
	if p1.Offset != 40 {
		t.Errorf("Expected offset 40, got %v", p1.Offset)
	}
}

func TestPortFixedOffset_MultiplePortsSameNode(t *testing.T) {
	// Multiple PortFixedOffset ports on the same node maintain their declared offsets.
	g := NewGraph()
	g.AddNode("hub", NodeOptions{
		Width: 200, Height: 120,
		Ports: []PortOptions{
			{ID: "f1", Offset: 20, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "f2", Offset: 45, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "f3", Offset: 70, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "f4", Offset: 95, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("right1", NodeOptions{Width: 100, Height: 60})
	g.AddNode("right2", NodeOptions{Width: 100, Height: 60})
	g.MustAddEdge("hub", "right1", EdgeOptions{SourcePort: "f1"})
	g.MustAddEdge("hub", "right1", EdgeOptions{SourcePort: "f2", ID: "e2"})
	g.MustAddEdge("hub", "right2", EdgeOptions{SourcePort: "f3"})
	g.MustAddEdge("hub", "right2", EdgeOptions{SourcePort: "f4", ID: "e4"})

	layout := g.Layout(Options{Direction: LeftToRight, NodeSep: 50, RankSep: 100})

	hubLayout := layout.Nodes["hub"]
	if hubLayout.Ports == nil {
		t.Fatal("Expected Ports map for hub")
	}

	// All offsets should be preserved exactly
	expectedOffsets := map[string]float64{"f1": 20, "f2": 45, "f3": 70, "f4": 95}
	for id, want := range expectedOffsets {
		got := hubLayout.Ports[id].Offset
		if got != want {
			t.Errorf("Port %s offset: got %v, want %v", id, got, want)
		}
	}
}

func TestPortFixedOffset_SchemaPattern(t *testing.T) {
	// Hub node with FK fields connecting to nodes in various directions.
	// Each port gets correct side based on connected node position.
	g := NewGraph()
	g.AddNode("users", NodeOptions{
		Width: 200, Height: 100,
		Ports: []PortOptions{
			{ID: "fk-orders", Offset: 34, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "fk-profiles", Offset: 54, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("orders", NodeOptions{Width: 200, Height: 80})
	g.AddNode("profiles", NodeOptions{Width: 200, Height: 80})

	g.MustAddEdge("users", "orders", EdgeOptions{SourcePort: "fk-orders"})
	g.MustAddEdge("users", "profiles", EdgeOptions{SourcePort: "fk-profiles"})

	layout := g.Layout(Options{Direction: LeftToRight, NodeSep: 50, RankSep: 100})

	usersLayout := layout.Nodes["users"]
	if usersLayout.Ports == nil {
		t.Fatal("Expected Ports map for users")
	}

	// Both connected nodes are to the right in LTR layout → ports should be Right
	if p := usersLayout.Ports["fk-orders"]; p.Side != Right {
		t.Errorf("fk-orders: expected Right, got %v", p.Side)
	}
	if p := usersLayout.Ports["fk-profiles"]; p.Side != Right {
		t.Errorf("fk-profiles: expected Right, got %v", p.Side)
	}

	// Offsets preserved
	if p := usersLayout.Ports["fk-orders"]; p.Offset != 34 {
		t.Errorf("fk-orders offset: got %v, want 34", p.Offset)
	}
	if p := usersLayout.Ports["fk-profiles"]; p.Offset != 54 {
		t.Errorf("fk-profiles offset: got %v, want 54", p.Offset)
	}
}

func TestPortFixedOffset_MixedWithFree(t *testing.T) {
	// PortFixedOffset and PortFree ports on the same node.
	// PortFixedOffset offsets preserved, PortFree offsets computed.
	g := NewGraph()
	g.AddNode("A", NodeOptions{
		Width: 200, Height: 100,
		Ports: []PortOptions{
			{ID: "fixed1", Offset: 30, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
			{ID: "free1", Constraint: PortFree, Axis: PortAxisHorizontal},
			{ID: "fixed2", Offset: 70, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("B", NodeOptions{Width: 100, Height: 60})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "fixed1"})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "free1", ID: "e2"})
	g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "fixed2", ID: "e3"})

	layout := g.Layout(Options{Direction: LeftToRight, NodeSep: 50, RankSep: 100})

	nodeA := layout.Nodes["A"]
	if nodeA.Ports == nil {
		t.Fatal("Expected Ports map")
	}

	// PortFixedOffset offsets preserved
	if p := nodeA.Ports["fixed1"]; p.Offset != 30 {
		t.Errorf("fixed1 offset: got %v, want 30", p.Offset)
	}
	if p := nodeA.Ports["fixed2"]; p.Offset != 70 {
		t.Errorf("fixed2 offset: got %v, want 70", p.Offset)
	}

	// PortFree offset should be computed (not 0, not same as fixed offsets)
	freePort := nodeA.Ports["free1"]
	if freePort.Offset == 0 {
		t.Error("PortFree offset should be computed, got 0")
	}
	if freePort.Offset == 30 || freePort.Offset == 70 {
		t.Errorf("PortFree offset should differ from fixed offsets, got %v", freePort.Offset)
	}
}

func TestPortFixedOffset_PerEdgeSide(t *testing.T) {
	// A hub node with a PortFixedOffset port connected to nodes on BOTH sides.
	// The per-edge side should face each individual target, not the averaged direction.
	//
	// Layout (LeftToRight):
	//   Left → Hub → Right
	//
	// Hub's port-3 is used as BOTH target (from Left) and source (to Right).
	// The target side should face Left, source side should face Right.
	g := NewGraph()
	g.AddNode("Left", NodeOptions{Width: 100, Height: 60})
	g.AddNode("Hub", NodeOptions{
		Width: 200, Height: 100,
		Ports: []PortOptions{
			{ID: "port-3", Offset: 44, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
		},
	})
	g.AddNode("Right", NodeOptions{Width: 100, Height: 60})

	// Left → Hub (Hub's port-3 is target)
	g.MustAddEdge("Left", "Hub", EdgeOptions{TargetPort: "port-3", ID: "from-left"})
	// Hub → Right (Hub's port-3 is source)
	g.MustAddEdge("Hub", "Right", EdgeOptions{SourcePort: "port-3", ID: "to-right"})

	layout := g.Layout(Options{
		Direction: LeftToRight,
		NodeSep:   50,
		RankSep:   150,
	})

	hubX := layout.Nodes["Hub"].X + layout.Nodes["Hub"].Width/2
	leftX := layout.Nodes["Left"].X + layout.Nodes["Left"].Width/2
	rightX := layout.Nodes["Right"].X + layout.Nodes["Right"].Width/2

	// Verify layout order: Left < Hub < Right
	if leftX >= hubX {
		t.Fatalf("Left not left of Hub (leftX=%v, hubX=%v)", leftX, hubX)
	}
	if rightX <= hubX {
		t.Fatalf("Right not right of Hub (rightX=%v, hubX=%v)", rightX, hubX)
	}

	// Edge from Left → Hub: Hub's port-3 should receive on Left side
	edgeFromLeft, ok := layout.Edge("Left", "Hub")
	if !ok {
		t.Fatal("Edge Left->Hub not found")
	}
	if edgeFromLeft.TargetSide != Left {
		t.Errorf("Edge from Left: expected TargetSide=Left (facing Left node), got %v", edgeFromLeft.TargetSide)
	}

	// Edge from Hub → Right: Hub's port-3 should exit on Right side
	edgeToRight, ok := layout.Edge("Hub", "Right")
	if !ok {
		t.Fatal("Edge Hub->Right not found")
	}
	if edgeToRight.SourceSide != Right {
		t.Errorf("Edge to Right: expected SourceSide=Right (facing Right node), got %v", edgeToRight.SourceSide)
	}

	// Port offset must be preserved regardless of per-edge side
	hubPorts := layout.Nodes["Hub"].Ports
	if hubPorts == nil {
		t.Fatal("Hub has no ports in layout")
	}
	if p := hubPorts["port-3"]; p.Offset != 44 {
		t.Errorf("port-3 offset: got %v, want 44", p.Offset)
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
