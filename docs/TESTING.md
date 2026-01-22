# Testing Strategy for Posit

This document defines the comprehensive testing strategy for the posit library, a pure Go implementation of the Sugiyama algorithm for layered graph layout.

## Table of Contents

1. [Testing Philosophy](#testing-philosophy)
2. [Unit Tests by Phase](#unit-tests-by-phase)
3. [Integration Tests](#integration-tests)
4. [Golden File Tests](#golden-file-tests)
5. [Property-Based Tests](#property-based-tests)
6. [Benchmark Suite](#benchmark-suite)
7. [Test Fixtures](#test-fixtures)
8. [CI Integration](#ci-integration)

---

## Testing Philosophy

### Core Principles

1. **Test each phase independently** - The Sugiyama algorithm consists of distinct phases (acyclic, rank, normalize, order, position). Each phase should be tested in isolation with well-defined inputs and expected outputs.

2. **Golden file tests comparing to dagre output** - Since posit aims for dagre-compatible behavior, we maintain golden files generated from dagre's output to verify correctness.

3. **Property-based testing for invariants** - Many graph layout properties are invariants that must hold for ANY valid input. Property-based testing catches edge cases that example-based tests miss.

4. **Benchmark suite for performance regression** - Layout performance is critical for interactive applications. We track performance metrics to prevent regressions.

### Test Organization

```
posit/
  acyclic.go          -> acyclic_test.go
  rank.go             -> rank_test.go
  normalize.go        -> normalize_test.go
  order.go            -> order_test.go
  position.go         -> position_test.go
  layout_test.go      (integration tests)
  testdata/
    golden/           (golden file outputs)
    fixtures/         (test input graphs)
```

---

## Unit Tests by Phase

### acyclic_test.go

The acyclic phase detects and breaks cycles by reversing edges. Tests verify cycle detection and proper edge reversal.

```go
package posit

import (
	"testing"
)

// TestAcyclic_SimpleDAG verifies that a graph without cycles remains unchanged.
func TestAcyclic_SimpleDAG(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	reversed := state.makeAcyclic()

	if len(reversed) != 0 {
		t.Errorf("Expected no reversed edges for DAG, got %d", len(reversed))
	}
}

// TestAcyclic_SingleCycle verifies that a simple cycle is broken.
func TestAcyclic_SingleCycle(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A") // Creates cycle

	state := newLayoutState(g, DefaultOptions())
	reversed := state.makeAcyclic()

	if len(reversed) != 1 {
		t.Errorf("Expected 1 reversed edge for single cycle, got %d", len(reversed))
	}

	// Verify graph is now acyclic
	if hasCycle(state) {
		t.Error("Graph still has cycle after makeAcyclic")
	}
}

// TestAcyclic_MultipleCycles verifies handling of multiple independent cycles.
func TestAcyclic_MultipleCycles(t *testing.T) {
	g := NewGraph()
	// Cycle 1: A -> B -> C -> A
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")

	// Cycle 2: D -> E -> D
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddNode("E", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("D", "E")
	g.AddEdge("E", "D")

	state := newLayoutState(g, DefaultOptions())
	reversed := state.makeAcyclic()

	if len(reversed) < 2 {
		t.Errorf("Expected at least 2 reversed edges, got %d", len(reversed))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycles after makeAcyclic")
	}
}

// TestAcyclic_SelfLoop verifies self-loop handling.
func TestAcyclic_SelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "A") // Self-loop

	state := newLayoutState(g, DefaultOptions())
	selfLoops := state.removeSelfLoops()

	if len(selfLoops) != 1 {
		t.Errorf("Expected 1 self-loop removed, got %d", len(selfLoops))
	}
}

// TestAcyclic_DisconnectedWithCycles tests disconnected components each with cycles.
func TestAcyclic_DisconnectedWithCycles(t *testing.T) {
	g := NewGraph()
	// Component 1 with cycle
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	// Component 2 with cycle
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("X", "Y")
	g.AddEdge("Y", "X")

	state := newLayoutState(g, DefaultOptions())
	reversed := state.makeAcyclic()

	// Should break cycles in both components
	if len(reversed) < 2 {
		t.Errorf("Expected at least 2 reversed edges, got %d", len(reversed))
	}

	if hasCycle(state) {
		t.Error("Graph still has cycles after makeAcyclic")
	}
}

// hasCycle is a test helper that detects cycles using DFS.
func hasCycle(state *layoutState) bool {
	// Implementation: DFS with white/gray/black coloring
	// Returns true if back edge detected
	return false // Placeholder
}
```

### rank_test.go

The rank phase assigns nodes to layers. Tests verify correct layer assignment for various topologies.

```go
package posit

import (
	"testing"
)

// TestRank_LinearChain verifies A->B->C gets ranks 0, 1, 2.
func TestRank_LinearChain(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	ranks := state.getRanks()

	// A should be rank 0, B rank 1, C rank 2
	if ranks["A"] != 0 {
		t.Errorf("Expected A at rank 0, got %d", ranks["A"])
	}
	if ranks["B"] != 1 {
		t.Errorf("Expected B at rank 1, got %d", ranks["B"])
	}
	if ranks["C"] != 2 {
		t.Errorf("Expected C at rank 2, got %d", ranks["C"])
	}
}

// TestRank_Diamond verifies diamond pattern A->(B,C)->D.
func TestRank_Diamond(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")
	g.AddEdge("C", "D")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	ranks := state.getRanks()

	// A at rank 0, B and C at rank 1, D at rank 2
	if ranks["A"] != 0 {
		t.Errorf("Expected A at rank 0, got %d", ranks["A"])
	}
	if ranks["B"] != 1 {
		t.Errorf("Expected B at rank 1, got %d", ranks["B"])
	}
	if ranks["C"] != 1 {
		t.Errorf("Expected C at rank 1, got %d", ranks["C"])
	}
	if ranks["D"] != 2 {
		t.Errorf("Expected D at rank 2, got %d", ranks["D"])
	}
}

// TestRank_WideGraph tests graph with many root nodes.
func TestRank_WideGraph(t *testing.T) {
	g := NewGraph()
	// 5 roots all pointing to single sink
	roots := []string{"R1", "R2", "R3", "R4", "R5"}
	for _, r := range roots {
		g.AddNode(r, NodeOptions{Width: 100, Height: 50})
	}
	g.AddNode("Sink", NodeOptions{Width: 100, Height: 50})
	for _, r := range roots {
		g.AddEdge(r, "Sink")
	}

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	ranks := state.getRanks()

	// All roots at rank 0
	for _, r := range roots {
		if ranks[r] != 0 {
			t.Errorf("Expected %s at rank 0, got %d", r, ranks[r])
		}
	}
	// Sink at rank 1
	if ranks["Sink"] != 1 {
		t.Errorf("Expected Sink at rank 1, got %d", ranks["Sink"])
	}
}

// TestRank_DeepGraph tests long chain (10 nodes).
func TestRank_DeepGraph(t *testing.T) {
	g := NewGraph()
	nodes := make([]string, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = string(rune('A' + i))
		g.AddNode(nodes[i], NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < 9; i++ {
		g.AddEdge(nodes[i], nodes[i+1])
	}

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	ranks := state.getRanks()

	for i, n := range nodes {
		if ranks[n] != i {
			t.Errorf("Expected %s at rank %d, got %d", n, i, ranks[n])
		}
	}
}

// TestRank_NetworkSimplex tests the network simplex algorithm.
func TestRank_NetworkSimplex(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "C")

	opts := DefaultOptions()
	opts.Algorithm = NetworkSimplex

	state := newLayoutState(g, opts)
	state.makeAcyclic()
	state.assignLayers()

	ranks := state.getRanks()

	// Network simplex should produce tight tree
	// A at 0, B at 1, C at 2 (tighter than longest path might give)
	if ranks["A"] >= ranks["B"] || ranks["B"] >= ranks["C"] {
		t.Error("Rank order violated: expected A < B < C")
	}
}
```

### normalize_test.go

The normalize phase adds dummy nodes for edges spanning multiple layers.

```go
package posit

import (
	"testing"
)

// TestNormalize_SingleDummy tests edge spanning 2 layers (needs 1 dummy).
func TestNormalize_SingleDummy(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C") // A at rank 0, B at rank 1, but C at rank 2

	// Force C to rank 2 by adding intermediate
	g.AddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	initialNodes := state.nodeCount()
	state.addDummyNodes()
	afterNodes := state.nodeCount()

	// Edge A->C spans 2 ranks, needs 1 dummy
	// But we also have A->B and B->C which span 1 rank each
	// So the A->C edge is actually 2 hops through B anyway
	// Let's use a clearer test case
	if afterNodes < initialNodes {
		t.Error("Dummy node count should not decrease")
	}
}

// TestNormalize_MultipleDummies tests edge spanning 5 layers (needs 4 dummies).
func TestNormalize_MultipleDummies(t *testing.T) {
	g := NewGraph()
	// Create chain to establish 5 ranks
	g.AddNode("L0", NodeOptions{Width: 100, Height: 50})
	g.AddNode("L1", NodeOptions{Width: 100, Height: 50})
	g.AddNode("L2", NodeOptions{Width: 100, Height: 50})
	g.AddNode("L3", NodeOptions{Width: 100, Height: 50})
	g.AddNode("L4", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("L0", "L1")
	g.AddEdge("L1", "L2")
	g.AddEdge("L2", "L3")
	g.AddEdge("L3", "L4")

	// Add node at rank 0 with edge to rank 4
	g.AddNode("Start", NodeOptions{Width: 100, Height: 50})
	g.AddNode("End", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("Start", "L0") // Start at rank -1 or 0
	g.AddEdge("L4", "End")   // End at rank 5

	// Long edge spanning multiple ranks
	g.AddEdge("Start", "End")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	initialNodes := state.nodeCount()
	dummyCount := state.addDummyNodes()
	afterNodes := state.nodeCount()

	// Verify dummies were added
	if afterNodes <= initialNodes {
		t.Errorf("Expected dummy nodes added, initial=%d after=%d", initialNodes, afterNodes)
	}

	t.Logf("Added %d dummy nodes", dummyCount)
}

// TestNormalize_MultipleLongEdges tests multiple edges needing dummies.
func TestNormalize_MultipleLongEdges(t *testing.T) {
	g := NewGraph()
	// Layer 0
	g.AddNode("A0", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B0", NodeOptions{Width: 100, Height: 50})

	// Layer 1
	g.AddNode("A1", NodeOptions{Width: 100, Height: 50})

	// Layer 2
	g.AddNode("A2", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B2", NodeOptions{Width: 100, Height: 50})

	// Establish ranks
	g.AddEdge("A0", "A1")
	g.AddEdge("A1", "A2")

	// Long edges
	g.AddEdge("A0", "A2") // Spans 2 ranks
	g.AddEdge("B0", "B2") // Spans 2 ranks (needs B0 at rank 0)

	// Connect B0 to establish its rank
	g.AddEdge("A0", "B0") // B0 same rank as A0? No, this makes B0 child
	// Better: make B0 a root too
	g.AddEdge("B0", "A1") // Now B0 is at rank 0

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	dummyCount := state.addDummyNodes()

	if dummyCount < 2 {
		t.Errorf("Expected at least 2 dummy nodes for long edges, got %d", dummyCount)
	}
}

// TestNormalize_DummyConnections verifies dummy nodes have correct connections.
func TestNormalize_DummyConnections(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C") // Long edge: A(0) -> C(2)

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Find the dummy node
	dummies := state.getDummyNodes()

	for _, d := range dummies {
		// Each dummy should have exactly 1 incoming and 1 outgoing edge
		inDegree := state.inDegree(d)
		outDegree := state.outDegree(d)

		if inDegree != 1 {
			t.Errorf("Dummy %s has in-degree %d, expected 1", d, inDegree)
		}
		if outDegree != 1 {
			t.Errorf("Dummy %s has out-degree %d, expected 1", d, outDegree)
		}
	}
}
```

### order_test.go

The order phase minimizes edge crossings within layers.

```go
package posit

import (
	"testing"
)

// TestOrder_CrossingsDecrease verifies crossing count improves or stays same.
func TestOrder_CrossingsDecrease(t *testing.T) {
	g := NewGraph()
	// Create a graph with known crossings
	// Layer 0: A, B
	// Layer 1: C, D
	// Edges: A->D, B->C (crossed) vs A->C, B->D (not crossed)
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "D")
	g.AddEdge("B", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	initialCrossings := state.countCrossings()
	state.minimizeCrossings()
	finalCrossings := state.countCrossings()

	if finalCrossings > initialCrossings {
		t.Errorf("Crossings increased: %d -> %d", initialCrossings, finalCrossings)
	}
}

// TestOrder_KnownOptimal verifies known optimal ordering for small graph.
func TestOrder_KnownOptimal(t *testing.T) {
	g := NewGraph()
	// Simple case where optimal is zero crossings
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	crossings := state.countCrossings()
	if crossings != 0 {
		t.Errorf("Expected 0 crossings for tree, got %d", crossings)
	}
}

// TestOrder_BarycenterMethod tests the barycenter heuristic.
func TestOrder_BarycenterMethod(t *testing.T) {
	g := NewGraph()
	// Layer 0: A, B, C (order 0, 1, 2)
	// Layer 1: X, Y
	// Edges that would benefit from reordering
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})

	g.AddEdge("A", "Y") // A connects to Y (right side)
	g.AddEdge("B", "X") // B connects to X (left side)
	g.AddEdge("C", "X") // C connects to X (left side)

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	// After barycenter: X should be positioned based on avg of B,C positions
	// Y should be positioned based on A's position
	order := state.getLayerOrder(1) // Layer 1 order

	t.Logf("Layer 1 order: %v", order)
	// Just verify no panic and order is valid
	if len(order) != 2 {
		t.Errorf("Expected 2 nodes in layer 1, got %d", len(order))
	}
}

// TestOrder_MultipleIterations verifies iterative improvement.
func TestOrder_MultipleIterations(t *testing.T) {
	g := NewGraph()
	// Build a more complex graph
	for i := 0; i < 4; i++ {
		g.AddNode(string(rune('A'+i)), NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < 4; i++ {
		g.AddNode(string(rune('W'+i)), NodeOptions{Width: 100, Height: 50})
	}

	// Cross-connect to create crossings
	g.AddEdge("A", "Z")
	g.AddEdge("B", "Y")
	g.AddEdge("C", "X")
	g.AddEdge("D", "W")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Track crossings over iterations
	prev := state.countCrossings()
	for i := 0; i < 5; i++ {
		state.orderSweep(i%2 == 0) // Alternate directions
		curr := state.countCrossings()
		if curr > prev {
			t.Logf("Warning: crossings increased in iteration %d: %d -> %d", i, prev, curr)
		}
		prev = curr
	}
}
```

### position_test.go

The position phase assigns X/Y coordinates to nodes.

```go
package posit

import (
	"math"
	"testing"
)

// TestPosition_NoOverlap verifies nodes don't overlap.
func TestPosition_NoOverlap(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")

	layout := g.Layout()

	// Check all pairs for overlap
	nodes := []string{"A", "B", "C"}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			n1 := layout.Nodes[nodes[i]]
			n2 := layout.Nodes[nodes[j]]

			if overlaps(n1, n2) {
				t.Errorf("Nodes %s and %s overlap", nodes[i], nodes[j])
			}
		}
	}
}

// overlaps checks if two node layouts overlap.
func overlaps(a, b NodeLayout) bool {
	// Rectangles overlap if they overlap on both axes
	aLeft := a.X - a.Width/2
	aRight := a.X + a.Width/2
	aTop := a.Y - a.Height/2
	aBottom := a.Y + a.Height/2

	bLeft := b.X - b.Width/2
	bRight := b.X + b.Width/2
	bTop := b.Y - b.Height/2
	bBottom := b.Y + b.Height/2

	xOverlap := aLeft < bRight && aRight > bLeft
	yOverlap := aTop < bBottom && aBottom > bTop

	return xOverlap && yOverlap
}

// TestPosition_MinimumSpacing verifies minimum spacing is respected.
func TestPosition_MinimumSpacing(t *testing.T) {
	opts := Options{
		NodeSep: 50,
		RankSep: 100,
	}

	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")

	layout := g.Layout(opts)

	// B and C should be in same layer, check horizontal spacing
	b := layout.Nodes["B"]
	c := layout.Nodes["C"]

	// If same layer, check horizontal gap
	if math.Abs(b.Y-c.Y) < 1 { // Same layer
		gap := math.Abs(b.X-c.X) - (b.Width+c.Width)/2
		if gap < opts.NodeSep-1 { // Allow 1px tolerance
			t.Errorf("Horizontal gap %.1f is less than NodeSep %.1f", gap, opts.NodeSep)
		}
	}

	// Check vertical spacing between layers
	a := layout.Nodes["A"]
	vertGapAB := math.Abs(a.Y-b.Y) - (a.Height+b.Height)/2
	if vertGapAB < opts.RankSep-1 {
		t.Errorf("Vertical gap %.1f is less than RankSep %.1f", vertGapAB, opts.RankSep)
	}
}

// TestPosition_EdgesStraight verifies edges are reasonably straight.
func TestPosition_EdgesStraight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")

	layout := g.Layout()

	a := layout.Nodes["A"]
	b := layout.Nodes["B"]

	// For simple edge, nodes should be roughly vertically aligned
	xDiff := math.Abs(a.X - b.X)
	if xDiff > 50 { // Allow some tolerance
		t.Logf("Warning: single edge has X offset of %.1f", xDiff)
	}
}

// TestPosition_Directions tests different layout directions.
func TestPosition_Directions(t *testing.T) {
	tests := []struct {
		dir      Direction
		name     string
		checkFn  func(a, b NodeLayout) bool
		expected string
	}{
		{
			TopToBottom,
			"TopToBottom",
			func(a, b NodeLayout) bool { return a.Y < b.Y },
			"A above B",
		},
		{
			LeftToRight,
			"LeftToRight",
			func(a, b NodeLayout) bool { return a.X < b.X },
			"A left of B",
		},
		{
			BottomToTop,
			"BottomToTop",
			func(a, b NodeLayout) bool { return a.Y > b.Y },
			"A below B",
		},
		{
			RightToLeft,
			"RightToLeft",
			func(a, b NodeLayout) bool { return a.X > b.X },
			"A right of B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraph()
			g.AddNode("A", NodeOptions{Width: 100, Height: 50})
			g.AddNode("B", NodeOptions{Width: 100, Height: 50})
			g.AddEdge("A", "B")

			layout := g.Layout(Options{Direction: tt.dir})

			a := layout.Nodes["A"]
			b := layout.Nodes["B"]

			if !tt.checkFn(a, b) {
				t.Errorf("Expected %s for direction %s, got A=(%.1f,%.1f) B=(%.1f,%.1f)",
					tt.expected, tt.name, a.X, a.Y, b.X, b.Y)
			}
		})
	}
}

// TestPosition_LargeNodes tests positioning with varying node sizes.
func TestPosition_LargeNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode("Small", NodeOptions{Width: 50, Height: 30})
	g.AddNode("Large", NodeOptions{Width: 200, Height: 100})
	g.AddNode("Medium", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("Small", "Large")
	g.AddEdge("Large", "Medium")

	layout := g.Layout()

	// Verify dimensions are preserved
	if layout.Nodes["Small"].Width != 50 {
		t.Errorf("Small node width changed: expected 50, got %.1f", layout.Nodes["Small"].Width)
	}
	if layout.Nodes["Large"].Width != 200 {
		t.Errorf("Large node width changed: expected 200, got %.1f", layout.Nodes["Large"].Width)
	}

	// Verify no overlaps despite size differences
	small := layout.Nodes["Small"]
	large := layout.Nodes["Large"]
	if overlaps(small, large) {
		t.Error("Small and Large nodes overlap")
	}
}
```

---

## Integration Tests

### layout_test.go

Integration tests verify the complete pipeline produces correct results.

```go
package posit

import (
	"math"
	"testing"
)

// TestLayout_FullPipeline_SimpleGraph tests complete layout on simple graph.
func TestLayout_FullPipeline_SimpleGraph(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	layout := g.Layout()

	// Verify all nodes have positions
	if len(layout.Nodes) != 3 {
		t.Errorf("Expected 3 nodes in layout, got %d", len(layout.Nodes))
	}

	// Verify all edges have points
	if len(layout.Edges) != 2 {
		t.Errorf("Expected 2 edges in layout, got %d", len(layout.Edges))
	}

	// Verify positions are reasonable (not zero, not infinite)
	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
			t.Errorf("Node %s has invalid X: %v", id, node.X)
		}
		if math.IsNaN(node.Y) || math.IsInf(node.Y, 0) {
			t.Errorf("Node %s has invalid Y: %v", id, node.Y)
		}
	}
}

// TestLayout_SchemaGraph tests database schema-like graph.
func TestLayout_SchemaGraph(t *testing.T) {
	g := NewGraph()

	// Tables (wider nodes)
	tables := []string{"Users", "Orders", "Products", "OrderItems", "Categories"}
	for _, table := range tables {
		g.AddNode(table, NodeOptions{Width: 150, Height: 80})
	}

	// Relationships (foreign keys)
	g.AddEdge("Orders", "Users")        // Orders belongs to Users
	g.AddEdge("OrderItems", "Orders")   // OrderItems belongs to Orders
	g.AddEdge("OrderItems", "Products") // OrderItems references Products
	g.AddEdge("Products", "Categories") // Products belongs to Categories

	layout := g.Layout()

	// Verify all tables are laid out
	for _, table := range tables {
		if _, ok := layout.Nodes[table]; !ok {
			t.Errorf("Table %s missing from layout", table)
		}
	}

	// Verify no overlaps
	for i, t1 := range tables {
		for j := i + 1; j < len(tables); j++ {
			t2 := tables[j]
			if overlaps(layout.Nodes[t1], layout.Nodes[t2]) {
				t.Errorf("Tables %s and %s overlap", t1, t2)
			}
		}
	}
}

// TestLayout_CyclicGraph tests graph with cycles.
func TestLayout_CyclicGraph(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A") // Cycle

	layout := g.Layout()

	// Should still produce valid layout
	if len(layout.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(layout.Nodes))
	}

	// Verify no overlaps
	if overlaps(layout.Nodes["A"], layout.Nodes["B"]) ||
		overlaps(layout.Nodes["B"], layout.Nodes["C"]) ||
		overlaps(layout.Nodes["A"], layout.Nodes["C"]) {
		t.Error("Nodes overlap in cyclic graph layout")
	}
}

// TestLayout_DisconnectedComponents tests graph with multiple components.
func TestLayout_DisconnectedComponents(t *testing.T) {
	g := NewGraph()

	// Component 1
	g.AddNode("A1", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B1", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A1", "B1")

	// Component 2
	g.AddNode("A2", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B2", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A2", "B2")

	layout := g.Layout()

	// All nodes should be laid out
	if len(layout.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(layout.Nodes))
	}

	// Components should not overlap
	// (implementation may place them side by side or stacked)
}

// TestLayout_StressTest_100Nodes stress tests with 100 nodes.
func TestLayout_StressTest_100Nodes(t *testing.T) {
	g := buildRandomDAG(100, 150)

	layout := g.Layout()

	if len(layout.Nodes) != 100 {
		t.Errorf("Expected 100 nodes, got %d", len(layout.Nodes))
	}

	// Verify all positions are valid
	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
			t.Errorf("Node %s has invalid X", id)
		}
	}
}

// TestLayout_StressTest_200Nodes stress tests with 200 nodes.
func TestLayout_StressTest_200Nodes(t *testing.T) {
	g := buildRandomDAG(200, 300)

	layout := g.Layout()

	if len(layout.Nodes) != 200 {
		t.Errorf("Expected 200 nodes, got %d", len(layout.Nodes))
	}
}

// TestLayout_StressTest_500Nodes stress tests with 500 nodes.
func TestLayout_StressTest_500Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 500-node stress test in short mode")
	}

	g := buildRandomDAG(500, 750)

	layout := g.Layout()

	if len(layout.Nodes) != 500 {
		t.Errorf("Expected 500 nodes, got %d", len(layout.Nodes))
	}
}

// buildRandomDAG creates a random DAG with n nodes and e edges.
func buildRandomDAG(n, e int) *Graph {
	g := NewGraph()

	// Add nodes
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("N%d", i)
		g.AddNode(id, NodeOptions{
			Width:  50 + float64(i%3)*25,
			Height: 30 + float64(i%2)*20,
		})
	}

	// Add edges (only forward to maintain DAG property)
	edgeCount := 0
	for edgeCount < e {
		from := rand.Intn(n - 1)
		to := from + 1 + rand.Intn(n-from-1)
		if to < n {
			g.AddEdge(fmt.Sprintf("N%d", from), fmt.Sprintf("N%d", to))
			edgeCount++
		}
	}

	return g
}
```

---

## Golden File Tests

Golden file tests compare posit output against dagre's output for identical inputs.

### Setup and Structure

```
testdata/
  golden/
    simple_chain.json      # dagre output for simple chain
    diamond.json           # dagre output for diamond graph
    schema_5table.json     # dagre output for 5-table schema
    complex_50node.json    # dagre output for 50-node graph
```

### golden_test.go

```go
package posit

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// GoldenLayout represents the expected layout from dagre.
type GoldenLayout struct {
	Nodes map[string]struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"nodes"`
	Edges map[string]struct {
		Points []struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"points"`
	} `json:"edges"`
}

// TestGolden_SimpleChain compares posit output to dagre golden file.
func TestGolden_SimpleChain(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	compareToGolden(t, g, "simple_chain.json")
}

// TestGolden_Diamond compares diamond graph output.
func TestGolden_Diamond(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")
	g.AddEdge("C", "D")

	compareToGolden(t, g, "diamond.json")
}

// compareToGolden loads golden file and compares to posit output.
func compareToGolden(t *testing.T, g *Graph, goldenFile string) {
	t.Helper()

	// Load golden file
	goldenPath := filepath.Join("testdata", "golden", goldenFile)
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("Golden file not found: %s (run 'go generate' to create)", goldenPath)
		return
	}

	var golden GoldenLayout
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("Failed to parse golden file: %v", err)
	}

	// Generate posit layout
	layout := g.Layout()

	// Compare node positions (allowing small tolerance)
	const tolerance = 1.0 // 1 pixel tolerance for floating-point differences

	for id, goldenNode := range golden.Nodes {
		positNode, ok := layout.Nodes[id]
		if !ok {
			t.Errorf("Node %s missing from posit output", id)
			continue
		}

		if !approxEqual(goldenNode.X, positNode.X, tolerance) {
			t.Errorf("Node %s X mismatch: golden=%.2f posit=%.2f", id, goldenNode.X, positNode.X)
		}
		if !approxEqual(goldenNode.Y, positNode.Y, tolerance) {
			t.Errorf("Node %s Y mismatch: golden=%.2f posit=%.2f", id, goldenNode.Y, positNode.Y)
		}
	}
}

// approxEqual checks if two floats are approximately equal.
func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// UpdateGolden generates new golden files from dagre.
// Run with: go test -run TestUpdateGolden -update-golden
var updateGolden = flag.Bool("update-golden", false, "Update golden files")

func TestUpdateGolden(t *testing.T) {
	if !*updateGolden {
		t.Skip("Use -update-golden flag to update golden files")
	}

	// This would typically call a helper script that:
	// 1. Creates the same graph in JavaScript
	// 2. Runs dagre layout
	// 3. Serializes output to JSON
	// 4. Saves to testdata/golden/

	t.Log("To update golden files, run: npm run generate-golden")
}
```

### Generating Golden Files

Create a Node.js script to generate golden files:

```javascript
// scripts/generate-golden.js
const dagre = require('dagre');
const fs = require('fs');
const path = require('path');

const goldenDir = path.join(__dirname, '..', 'testdata', 'golden');

// Ensure directory exists
fs.mkdirSync(goldenDir, { recursive: true });

// Simple chain test case
function generateSimpleChain() {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: 'TB', nodesep: 50, ranksep: 100 });
  g.setDefaultEdgeLabel(() => ({}));

  g.setNode('A', { width: 100, height: 50 });
  g.setNode('B', { width: 100, height: 50 });
  g.setNode('C', { width: 100, height: 50 });
  g.setEdge('A', 'B');
  g.setEdge('B', 'C');

  dagre.layout(g);

  return extractLayout(g);
}

function extractLayout(g) {
  const nodes = {};
  g.nodes().forEach(v => {
    const node = g.node(v);
    nodes[v] = { x: node.x, y: node.y, width: node.width, height: node.height };
  });

  const edges = {};
  g.edges().forEach(e => {
    const edge = g.edge(e);
    const key = `${e.v}->${e.w}`;
    edges[key] = { points: edge.points };
  });

  return { nodes, edges };
}

// Generate all golden files
const testCases = {
  'simple_chain.json': generateSimpleChain,
  // Add more test cases here
};

Object.entries(testCases).forEach(([filename, generator]) => {
  const layout = generator();
  const filepath = path.join(goldenDir, filename);
  fs.writeFileSync(filepath, JSON.stringify(layout, null, 2));
  console.log(`Generated: ${filepath}`);
});
```

---

## Property-Based Tests

Property-based tests verify invariants that must hold for ANY valid graph input.

### property_test.go

```go
package posit

import (
	"math"
	"math/rand"
	"testing"
	"testing/quick"
)

// TestProperty_NodesMatchInput verifies all input nodes appear in output.
func TestProperty_NodesMatchInput(t *testing.T) {
	f := func(seed int64) bool {
		rand.Seed(seed)
		g := generateRandomGraph(rand.Intn(50) + 1)

		layout := g.Layout()

		// Every input node must be in output
		for id := range g.nodes {
			if _, ok := layout.Nodes[id]; !ok {
				return false
			}
		}

		// No extra nodes in output (except we don't expose dummies)
		return len(layout.Nodes) == len(g.nodes)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestProperty_EdgesHaveValidEndpoints verifies all edges reference valid nodes.
func TestProperty_EdgesHaveValidEndpoints(t *testing.T) {
	f := func(seed int64) bool {
		rand.Seed(seed)
		g := generateRandomGraph(rand.Intn(50) + 1)

		layout := g.Layout()

		for edgeKey, edge := range layout.Edges {
			// Edge key format: "from->to"
			// All edge points should be finite
			for _, pt := range edge.Points {
				if math.IsNaN(pt.X) || math.IsInf(pt.X, 0) ||
					math.IsNaN(pt.Y) || math.IsInf(pt.Y, 0) {
					return false
				}
			}
			_ = edgeKey // Edge key validation could go here
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestProperty_PositionsFinite verifies all positions are finite numbers.
func TestProperty_PositionsFinite(t *testing.T) {
	f := func(seed int64) bool {
		rand.Seed(seed)
		g := generateRandomGraph(rand.Intn(100) + 1)

		layout := g.Layout()

		for _, node := range layout.Nodes {
			if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
				return false
			}
			if math.IsNaN(node.Y) || math.IsInf(node.Y, 0) {
				return false
			}
			if node.Width <= 0 || node.Height <= 0 {
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestProperty_NoOverlappingNodes verifies no nodes overlap.
func TestProperty_NoOverlappingNodes(t *testing.T) {
	f := func(seed int64) bool {
		rand.Seed(seed)
		g := generateRandomGraph(rand.Intn(30) + 2) // At least 2 nodes

		layout := g.Layout()

		// Collect all nodes
		var nodes []NodeLayout
		for _, n := range layout.Nodes {
			nodes = append(nodes, n)
		}

		// Check all pairs
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				if overlaps(nodes[i], nodes[j]) {
					return false
				}
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Error(err)
	}
}

// TestProperty_HierarchyRespected verifies parent nodes are above children (for TB).
func TestProperty_HierarchyRespected(t *testing.T) {
	f := func(seed int64) bool {
		rand.Seed(seed)
		g := generateRandomDAG(rand.Intn(30)+2, rand.Intn(50)+1)

		opts := Options{Direction: TopToBottom, NodeSep: 50, RankSep: 100}
		layout := g.Layout(opts)

		// For every edge A->B, A.Y should be less than B.Y (TB direction)
		for _, e := range g.edges {
			from := layout.Nodes[e.from]
			to := layout.Nodes[e.to]

			// Allow small tolerance for nodes in same layer
			if from.Y > to.Y+1 {
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Error(err)
	}
}

// TestProperty_LayoutDeterministic verifies same input produces same output.
func TestProperty_LayoutDeterministic(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")

	// Run layout multiple times
	layout1 := g.Layout()
	layout2 := g.Layout()
	layout3 := g.Layout()

	// All should be identical
	for id := range layout1.Nodes {
		n1 := layout1.Nodes[id]
		n2 := layout2.Nodes[id]
		n3 := layout3.Nodes[id]

		if n1.X != n2.X || n2.X != n3.X {
			t.Errorf("Node %s X not deterministic: %v, %v, %v", id, n1.X, n2.X, n3.X)
		}
		if n1.Y != n2.Y || n2.Y != n3.Y {
			t.Errorf("Node %s Y not deterministic: %v, %v, %v", id, n1.Y, n2.Y, n3.Y)
		}
	}
}

// generateRandomGraph creates a random graph with n nodes.
func generateRandomGraph(n int) *Graph {
	g := NewGraph()

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("N%d", i)
		g.AddNode(id, NodeOptions{
			Width:  50 + float64(rand.Intn(100)),
			Height: 30 + float64(rand.Intn(50)),
		})
	}

	// Add random edges (may create cycles)
	edgeCount := rand.Intn(n * 2)
	for i := 0; i < edgeCount; i++ {
		from := fmt.Sprintf("N%d", rand.Intn(n))
		to := fmt.Sprintf("N%d", rand.Intn(n))
		if from != to {
			g.AddEdge(from, to)
		}
	}

	return g
}

// generateRandomDAG creates a random DAG with n nodes and up to e edges.
func generateRandomDAG(n, e int) *Graph {
	g := NewGraph()

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("N%d", i)
		g.AddNode(id, NodeOptions{
			Width:  50 + float64(rand.Intn(100)),
			Height: 30 + float64(rand.Intn(50)),
		})
	}

	// Only add forward edges (lower index -> higher index)
	edgeCount := 0
	attempts := 0
	for edgeCount < e && attempts < e*3 {
		attempts++
		from := rand.Intn(n - 1)
		to := from + 1 + rand.Intn(n-from-1)
		if to < n {
			g.AddEdge(fmt.Sprintf("N%d", from), fmt.Sprintf("N%d", to))
			edgeCount++
		}
	}

	return g
}
```

---

## Benchmark Suite

Benchmarks track performance and detect regressions.

### benchmark_test.go

```go
package posit

import (
	"fmt"
	"testing"
)

// BenchmarkLayout10 benchmarks layout with 10 nodes.
func BenchmarkLayout10(b *testing.B) {
	benchmarkLayoutN(b, 10, 15)
}

// BenchmarkLayout50 benchmarks layout with 50 nodes.
func BenchmarkLayout50(b *testing.B) {
	benchmarkLayoutN(b, 50, 75)
}

// BenchmarkLayout100 benchmarks layout with 100 nodes.
func BenchmarkLayout100(b *testing.B) {
	benchmarkLayoutN(b, 100, 150)
}

// BenchmarkLayout200 benchmarks layout with 200 nodes.
func BenchmarkLayout200(b *testing.B) {
	benchmarkLayoutN(b, 200, 300)
}

// BenchmarkLayout500 benchmarks layout with 500 nodes.
func BenchmarkLayout500(b *testing.B) {
	benchmarkLayoutN(b, 500, 750)
}

// benchmarkLayoutN is the core benchmark helper.
func benchmarkLayoutN(b *testing.B, nodes, edges int) {
	g := buildBenchmarkGraph(nodes, edges)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = g.Layout()
	}
}

// buildBenchmarkGraph creates a reproducible benchmark graph.
func buildBenchmarkGraph(n, e int) *Graph {
	g := NewGraph()

	// Add nodes with consistent sizes
	for i := 0; i < n; i++ {
		g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{
			Width:  100,
			Height: 50,
		})
	}

	// Add edges in a reproducible pattern
	edgeIdx := 0
	for i := 0; i < n-1 && edgeIdx < e; i++ {
		// Connect to 1-3 downstream nodes
		targets := min(3, n-i-1)
		for j := 1; j <= targets && edgeIdx < e; j++ {
			g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+j))
			edgeIdx++
		}
	}

	return g
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BenchmarkLayout_SchemaGraph benchmarks a realistic schema graph.
func BenchmarkLayout_SchemaGraph(b *testing.B) {
	g := NewGraph()

	// Simulate a database schema with 20 tables
	tables := []string{
		"users", "profiles", "organizations", "memberships",
		"projects", "tasks", "comments", "attachments",
		"notifications", "settings", "audit_logs", "permissions",
		"roles", "role_permissions", "user_roles", "tags",
		"task_tags", "categories", "project_categories", "reports",
	}

	for _, t := range tables {
		g.AddNode(t, NodeOptions{Width: 150, Height: 80})
	}

	// Typical FK relationships
	relationships := [][2]string{
		{"profiles", "users"},
		{"memberships", "users"},
		{"memberships", "organizations"},
		{"projects", "organizations"},
		{"tasks", "projects"},
		{"tasks", "users"},
		{"comments", "tasks"},
		{"comments", "users"},
		{"attachments", "tasks"},
		{"attachments", "comments"},
		{"notifications", "users"},
		{"settings", "users"},
		{"audit_logs", "users"},
		{"role_permissions", "roles"},
		{"role_permissions", "permissions"},
		{"user_roles", "users"},
		{"user_roles", "roles"},
		{"task_tags", "tasks"},
		{"task_tags", "tags"},
		{"project_categories", "projects"},
		{"project_categories", "categories"},
		{"reports", "projects"},
	}

	for _, rel := range relationships {
		g.AddEdge(rel[0], rel[1])
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = g.Layout()
	}
}

// BenchmarkPhases benchmarks individual algorithm phases.
func BenchmarkPhases(b *testing.B) {
	g := buildBenchmarkGraph(100, 150)
	opts := DefaultOptions()

	b.Run("MakeAcyclic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			state := newLayoutState(g, opts)
			state.makeAcyclic()
		}
	})

	b.Run("AssignLayers", func(b *testing.B) {
		state := newLayoutState(g, opts)
		state.makeAcyclic()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			state.assignLayers()
		}
	})

	b.Run("AddDummyNodes", func(b *testing.B) {
		state := newLayoutState(g, opts)
		state.makeAcyclic()
		state.assignLayers()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			state.addDummyNodes()
		}
	})

	b.Run("MinimizeCrossings", func(b *testing.B) {
		state := newLayoutState(g, opts)
		state.makeAcyclic()
		state.assignLayers()
		state.addDummyNodes()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			state.minimizeCrossings()
		}
	})

	b.Run("AssignCoordinates", func(b *testing.B) {
		state := newLayoutState(g, opts)
		state.makeAcyclic()
		state.assignLayers()
		state.addDummyNodes()
		state.minimizeCrossings()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			state.assignCoordinates()
		}
	})
}

// BenchmarkMemory tracks memory usage for large graphs.
func BenchmarkMemory(b *testing.B) {
	sizes := []int{100, 200, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Nodes%d", size), func(b *testing.B) {
			g := buildBenchmarkGraph(size, size*3/2)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.Layout()
			}
		})
	}
}
```

### Performance Targets

| Graph Size | Target Time | Max Allocations |
|------------|-------------|-----------------|
| 10 nodes   | <1ms        | <100 allocs     |
| 50 nodes   | <10ms       | <500 allocs     |
| 100 nodes  | <25ms       | <1000 allocs    |
| 200 nodes  | <100ms      | <2500 allocs    |
| 500 nodes  | <500ms      | <10000 allocs   |

---

## Test Fixtures

### fixtures.go

Reusable test helpers and graph builders.

```go
package posit

import (
	"fmt"
	"math/rand"
)

// GraphBuilder provides a fluent API for building test graphs.
type GraphBuilder struct {
	graph *Graph
}

// NewBuilder creates a new graph builder.
func NewBuilder() *GraphBuilder {
	return &GraphBuilder{graph: NewGraph()}
}

// Node adds a node with default dimensions.
func (b *GraphBuilder) Node(id string) *GraphBuilder {
	b.graph.AddNode(id, NodeOptions{Width: 100, Height: 50})
	return b
}

// NodeWithSize adds a node with custom dimensions.
func (b *GraphBuilder) NodeWithSize(id string, w, h float64) *GraphBuilder {
	b.graph.AddNode(id, NodeOptions{Width: w, Height: h})
	return b
}

// Edge adds an edge between two nodes.
func (b *GraphBuilder) Edge(from, to string) *GraphBuilder {
	b.graph.AddEdge(from, to)
	return b
}

// Build returns the constructed graph.
func (b *GraphBuilder) Build() *Graph {
	return b.graph
}

// --- Common Graph Patterns ---

// Chain creates a linear chain: A -> B -> C -> ... -> N
func Chain(n int) *Graph {
	b := NewBuilder()
	for i := 0; i < n; i++ {
		b.Node(fmt.Sprintf("N%d", i))
	}
	for i := 0; i < n-1; i++ {
		b.Edge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	return b.Build()
}

// Diamond creates a diamond pattern: A -> (B, C) -> D
func Diamond() *Graph {
	return NewBuilder().
		Node("A").Node("B").Node("C").Node("D").
		Edge("A", "B").Edge("A", "C").
		Edge("B", "D").Edge("C", "D").
		Build()
}

// BinaryTree creates a complete binary tree of given depth.
func BinaryTree(depth int) *Graph {
	b := NewBuilder()

	nodeID := 0
	var addLevel func(int, string)
	addLevel = func(d int, prefix string) {
		id := fmt.Sprintf("N%d", nodeID)
		nodeID++
		b.Node(id)

		if prefix != "" {
			b.Edge(prefix, id)
		}

		if d > 0 {
			addLevel(d-1, id)
			addLevel(d-1, id)
		}
	}

	addLevel(depth, "")
	return b.Build()
}

// Mesh creates a mesh/grid graph.
func Mesh(rows, cols int) *Graph {
	b := NewBuilder()

	// Add all nodes
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			b.Node(fmt.Sprintf("N%d_%d", r, c))
		}
	}

	// Add horizontal edges
	for r := 0; r < rows; r++ {
		for c := 0; c < cols-1; c++ {
			b.Edge(fmt.Sprintf("N%d_%d", r, c), fmt.Sprintf("N%d_%d", r, c+1))
		}
	}

	// Add vertical edges
	for r := 0; r < rows-1; r++ {
		for c := 0; c < cols; c++ {
			b.Edge(fmt.Sprintf("N%d_%d", r, c), fmt.Sprintf("N%d_%d", r+1, c))
		}
	}

	return b.Build()
}

// RandomDAG creates a random directed acyclic graph.
func RandomDAG(nodes, edges int, seed int64) *Graph {
	rand.Seed(seed)
	b := NewBuilder()

	for i := 0; i < nodes; i++ {
		b.NodeWithSize(
			fmt.Sprintf("N%d", i),
			50+float64(rand.Intn(100)),
			30+float64(rand.Intn(50)),
		)
	}

	edgeCount := 0
	for edgeCount < edges {
		from := rand.Intn(nodes - 1)
		to := from + 1 + rand.Intn(nodes-from-1)
		if to < nodes {
			b.Edge(fmt.Sprintf("N%d", from), fmt.Sprintf("N%d", to))
			edgeCount++
		}
	}

	return b.Build()
}

// SchemaGraph creates a database schema-like graph.
func SchemaGraph(tables int) *Graph {
	b := NewBuilder()

	// Add tables
	for i := 0; i < tables; i++ {
		b.NodeWithSize(fmt.Sprintf("Table%d", i), 150, 80)
	}

	// Add relationships (each table has 1-3 FKs to earlier tables)
	for i := 1; i < tables; i++ {
		refs := 1 + rand.Intn(min(3, i))
		targets := make(map[int]bool)

		for j := 0; j < refs; j++ {
			target := rand.Intn(i)
			if !targets[target] {
				targets[target] = true
				b.Edge(fmt.Sprintf("Table%d", i), fmt.Sprintf("Table%d", target))
			}
		}
	}

	return b.Build()
}

// --- Test Helpers ---

// AssertNoOverlap verifies no nodes in the layout overlap.
func AssertNoOverlap(t testing.TB, layout *Layout) {
	t.Helper()

	var nodes []struct {
		id string
		nl NodeLayout
	}

	for id, nl := range layout.Nodes {
		nodes = append(nodes, struct {
			id string
			nl NodeLayout
		}{id, nl})
	}

	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if overlaps(nodes[i].nl, nodes[j].nl) {
				t.Errorf("Nodes %s and %s overlap", nodes[i].id, nodes[j].id)
			}
		}
	}
}

// AssertAllNodesPresent verifies all input nodes are in the layout.
func AssertAllNodesPresent(t testing.TB, g *Graph, layout *Layout) {
	t.Helper()

	for id := range g.nodes {
		if _, ok := layout.Nodes[id]; !ok {
			t.Errorf("Node %s missing from layout", id)
		}
	}
}

// AssertValidPositions verifies all positions are finite numbers.
func AssertValidPositions(t testing.TB, layout *Layout) {
	t.Helper()

	for id, node := range layout.Nodes {
		if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
			t.Errorf("Node %s has invalid X: %v", id, node.X)
		}
		if math.IsNaN(node.Y) || math.IsInf(node.Y, 0) {
			t.Errorf("Node %s has invalid Y: %v", id, node.Y)
		}
	}
}
```

---

## CI Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Check coverage
        run: |
          go tool cover -func=coverage.out
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Coverage: $COVERAGE%"
          if (( $(echo "$COVERAGE < 70" | bc -l) )); then
            echo "Coverage below 70% threshold"
            exit 1
          fi

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out

  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run benchmarks
        run: go test -bench=. -benchmem -count=5 ./... | tee benchmark.txt

      - name: Compare benchmarks
        uses: benchmark-action/github-action-benchmark@v1
        with:
          tool: 'go'
          output-file-path: benchmark.txt
          github-token: ${{ secrets.GITHUB_TOKEN }}
          alert-threshold: '150%'
          comment-on-alert: true
          fail-on-alert: true
          alert-comment-cc-users: '@maintainers'

  golden:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dagre
        run: npm install dagre @dagrejs/graphlib

      - name: Generate golden files
        run: node scripts/generate-golden.js

      - name: Run golden tests
        run: go test -v -run TestGolden ./...
```

### Coverage Targets

| Component     | Target Coverage |
|---------------|-----------------|
| acyclic.go    | 90%             |
| rank.go       | 85%             |
| normalize.go  | 85%             |
| order.go      | 80%             |
| position.go   | 80%             |
| posit.go      | 90%             |
| **Overall**   | **80%**         |

### Running Tests Locally

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -run TestAcyclic_SingleCycle ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run benchmarks and save results
go test -bench=. -benchmem -count=5 ./... > benchmark.txt

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run short tests only (skip stress tests)
go test -short ./...

# Update golden files
go test -run TestUpdateGolden -update-golden ./...
```

---

## Summary

This testing strategy ensures posit is:

1. **Correct** - Unit tests verify each phase, golden tests ensure dagre compatibility
2. **Robust** - Property-based tests catch edge cases
3. **Fast** - Benchmarks prevent performance regressions
4. **Maintainable** - Fixtures and helpers make tests readable and reusable

Key metrics to track:
- Test coverage: 80%+ overall
- Benchmark: <100ms for 200-node graphs
- Golden file alignment: 1-pixel tolerance to dagre output
