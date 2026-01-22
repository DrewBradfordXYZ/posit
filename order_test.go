package posit

import (
	"math"
	"reflect"
	"testing"
)

func TestOrder_NoCrossings(t *testing.T) {
	// Tree structure has 0 crossings when ordered properly
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

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

func TestOrder_CrossingsDecrease(t *testing.T) {
	// Create graph with known crossings that can be reduced
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "D") // Cross edge
	g.MustAddEdge("B", "C") // Cross edge

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Set initial order to create crossings
	state.layers[0] = []string{"A", "B"}
	state.layers[1] = []string{"C", "D"}
	state.assignOrderFromLayers()

	initialCrossings := state.countCrossings()
	state.minimizeCrossings()
	finalCrossings := state.countCrossings()

	if finalCrossings > initialCrossings {
		t.Errorf("Crossings increased: %d -> %d", initialCrossings, finalCrossings)
	}
}

func TestOrder_Deterministic(t *testing.T) {
	// Same input should produce same output
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")

	var orders [][]string
	for i := 0; i < 3; i++ {
		state := newLayoutState(g, DefaultOptions())
		state.makeAcyclic()
		state.assignLayers()
		state.addDummyNodes()
		state.minimizeCrossings()
		orders = append(orders, state.copyLayers()[1]) // Layer 1 order
	}

	for i := 1; i < len(orders); i++ {
		if !reflect.DeepEqual(orders[0], orders[i]) {
			t.Errorf("Non-deterministic: run 0 = %v, run %d = %v",
				orders[0], i, orders[i])
		}
	}
}

func TestOrder_BarycenterCalculation(t *testing.T) {
	// Manual verification of barycenter
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "X") // A at order 0
	g.MustAddEdge("B", "X") // B at order 1
	g.MustAddEdge("C", "X") // C at order 2

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Set known orders for layer 0
	state.layers[0] = []string{"A", "B", "C"}
	state.nodes["A"].order = 0
	state.nodes["B"].order = 1
	state.nodes["C"].order = 2

	bc, hasValue := state.calculateBarycenter("X", func(id string) []string {
		return state.predecessors[id]
	})

	if !hasValue {
		t.Fatal("Expected barycenter to have a value")
	}

	// Expected: (0 + 1 + 2) / 3 = 1.0
	expected := 1.0
	if math.Abs(bc-expected) > 0.001 {
		t.Errorf("Expected barycenter %v, got %v", expected, bc)
	}
}

func TestOrder_TwoLayerCrossCount(t *testing.T) {
	tests := []struct {
		name     string
		north    []string
		south    []string
		edges    [][2]string
		expected int
	}{
		{
			name:     "no crossings parallel",
			north:    []string{"A", "B"},
			south:    []string{"C", "D"},
			edges:    [][2]string{{"A", "C"}, {"B", "D"}},
			expected: 0,
		},
		{
			name:     "one crossing",
			north:    []string{"A", "B"},
			south:    []string{"C", "D"},
			edges:    [][2]string{{"A", "D"}, {"B", "C"}},
			expected: 1,
		},
		{
			name:     "no edges",
			north:    []string{"A", "B"},
			south:    []string{"C", "D"},
			edges:    [][2]string{},
			expected: 0,
		},
		{
			name:     "complete bipartite K2,2",
			north:    []string{"A", "B"},
			south:    []string{"C", "D"},
			edges:    [][2]string{{"A", "C"}, {"A", "D"}, {"B", "C"}, {"B", "D"}},
			expected: 1, // Only A->D and B->C cross (one pair)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraph()
			for _, n := range tt.north {
				g.AddNode(n, NodeOptions{Width: 100, Height: 50})
			}
			for _, n := range tt.south {
				g.AddNode(n, NodeOptions{Width: 100, Height: 50})
			}
			for _, e := range tt.edges {
				g.MustAddEdge(e[0], e[1])
			}

			state := newLayoutState(g, DefaultOptions())

			// Manually set up layers and orders
			state.layers = [][]string{tt.north, tt.south}
			for i, id := range tt.north {
				state.nodes[id].rank = 0
				state.nodes[id].order = i
			}
			for i, id := range tt.south {
				state.nodes[id].rank = 1
				state.nodes[id].order = i
			}

			crossings := state.twoLayerCrossCount(tt.north, tt.south)
			if crossings != tt.expected {
				t.Errorf("Expected %d crossings, got %d", tt.expected, crossings)
			}
		})
	}
}

func TestOrder_EmptyGraph(t *testing.T) {
	g := NewGraph()
	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	// Should not panic
	if len(state.layers) != 0 {
		t.Errorf("Expected 0 layers for empty graph, got %d", len(state.layers))
	}
}

func TestOrder_SingleNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	if state.nodes["A"].order != 0 {
		t.Errorf("Expected order 0 for single node, got %d", state.nodes["A"].order)
	}

	crossings := state.countCrossings()
	if crossings != 0 {
		t.Errorf("Expected 0 crossings for single node, got %d", crossings)
	}
}

func TestOrder_DisconnectedComponents(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B") // Component 1
	g.MustAddEdge("C", "D") // Component 2

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	// Should handle disconnected components without errors
	crossings := state.countCrossings()
	if crossings != 0 {
		t.Errorf("Expected 0 crossings for disconnected simple edges, got %d", crossings)
	}
}

func TestOrder_LongChain(t *testing.T) {
	// A -> B -> C -> D -> E
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.AddNode("E", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")
	g.MustAddEdge("C", "D")
	g.MustAddEdge("D", "E")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	// Linear chain should have 0 crossings
	crossings := state.countCrossings()
	if crossings != 0 {
		t.Errorf("Expected 0 crossings for linear chain, got %d", crossings)
	}
}

func TestOrder_DiamondGraph(t *testing.T) {
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("C", NodeOptions{Width: 100, Height: 50})
	g.AddNode("D", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "C")
	g.MustAddEdge("B", "D")
	g.MustAddEdge("C", "D")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.minimizeCrossings()

	// Diamond should have 0 crossings when ordered properly
	crossings := state.countCrossings()
	if crossings != 0 {
		t.Errorf("Expected 0 crossings for diamond, got %d", crossings)
	}
}

func TestOrder_CopyLayers(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "B")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()

	original := state.copyLayers()
	copy := state.copyLayers()

	// Modify copy
	copy[0][0] = "modified"

	// Original should be unchanged
	if state.layers[0][0] == "modified" {
		t.Error("copyLayers did not create a deep copy - modification affected original layers")
	}
	if original[0][0] == "modified" {
		t.Error("copyLayers did not create a deep copy - modification affected first copy")
	}
}

func TestOrder_SweepDown(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "Y") // A connects to Y (crosses if Y is left of X)
	g.MustAddEdge("B", "X") // B connects to X

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Set initial order with crossings
	state.layers[0] = []string{"A", "B"}
	state.layers[1] = []string{"X", "Y"}
	state.assignOrderFromLayers()

	initialCrossings := state.countCrossings()
	state.sweepDown()
	finalCrossings := state.countCrossings()

	if finalCrossings > initialCrossings {
		t.Errorf("sweepDown made crossings worse: %d -> %d", initialCrossings, finalCrossings)
	}
}

func TestOrder_SweepUp(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 100, Height: 50})
	g.AddNode("B", NodeOptions{Width: 100, Height: 50})
	g.AddNode("X", NodeOptions{Width: 100, Height: 50})
	g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
	g.MustAddEdge("A", "Y")
	g.MustAddEdge("B", "X")

	state := newLayoutState(g, DefaultOptions())
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()

	// Set initial order with crossings
	state.layers[0] = []string{"A", "B"}
	state.layers[1] = []string{"X", "Y"}
	state.assignOrderFromLayers()

	initialCrossings := state.countCrossings()
	state.sweepUp()
	finalCrossings := state.countCrossings()

	if finalCrossings > initialCrossings {
		t.Errorf("sweepUp made crossings worse: %d -> %d", initialCrossings, finalCrossings)
	}
}
