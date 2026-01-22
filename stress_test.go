package posit

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// TestStress_LargeGraph tests layout performance with 500 nodes and random edges.
func TestStress_LargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const nodeCount = 500
	const edgeCount = 1000

	g := NewGraph()
	for i := 0; i < nodeCount; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}

	// Add random edges (sparse graph)
	rng := rand.New(rand.NewSource(42)) // Deterministic seed
	for i := 0; i < edgeCount; i++ {
		from := fmt.Sprintf("n%d", rng.Intn(nodeCount))
		to := fmt.Sprintf("n%d", rng.Intn(nodeCount))
		if from != to { // Skip self-loops for simplicity
			g.AddEdge(from, to)
		}
	}

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("Layout took too long: %v", elapsed)
	}
	if len(layout.Nodes) != nodeCount {
		t.Errorf("Expected %d nodes, got %d", nodeCount, len(layout.Nodes))
	}

	t.Logf("Large graph (500 nodes, 1000 edges): %v", elapsed)
}

// TestStress_DenseGraph tests layout performance with many edges.
func TestStress_DenseGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const nodeCount = 100

	g := NewGraph()
	for i := 0; i < nodeCount; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}

	// Add many edges (dense graph: ~20% of possible edges)
	rng := rand.New(rand.NewSource(42))
	edgesAdded := 0
	for i := 0; i < nodeCount; i++ {
		for j := 0; j < nodeCount; j++ {
			if i != j && rng.Float32() < 0.20 {
				g.AddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", j))
				edgesAdded++
			}
		}
	}

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("Layout took too long: %v", elapsed)
	}
	if len(layout.Nodes) != nodeCount {
		t.Errorf("Expected %d nodes, got %d", nodeCount, len(layout.Nodes))
	}

	t.Logf("Dense graph (100 nodes, %d edges): %v", edgesAdded, elapsed)
}

// TestStress_DeepGraph tests layout performance with many layers (long chain).
func TestStress_DeepGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const depth = 200

	g := NewGraph()
	for i := 0; i < depth; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}

	// Create a linear chain
	for i := 0; i < depth-1; i++ {
		g.MustAddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1))
	}

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("Layout took too long: %v", elapsed)
	}
	if len(layout.Nodes) != depth {
		t.Errorf("Expected %d nodes, got %d", depth, len(layout.Nodes))
	}

	// Verify layers are sequential
	for i := 0; i < depth-1; i++ {
		curr := layout.Nodes[fmt.Sprintf("n%d", i)]
		next := layout.Nodes[fmt.Sprintf("n%d", i+1)]
		if curr.Y >= next.Y {
			t.Errorf("Node n%d (Y=%f) should be above n%d (Y=%f)", i, curr.Y, i+1, next.Y)
		}
	}

	t.Logf("Deep graph (200 layers): %v", elapsed)
}

// TestStress_WideGraph tests layout performance with many nodes per layer.
func TestStress_WideGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const width = 100
	const depth = 5

	g := NewGraph()

	// Create a wide graph with multiple layers
	for layer := 0; layer < depth; layer++ {
		for i := 0; i < width; i++ {
			g.AddNode(fmt.Sprintf("n%d_%d", layer, i), NodeOptions{Width: 50, Height: 30})
		}
	}

	// Connect adjacent layers
	rng := rand.New(rand.NewSource(42))
	for layer := 0; layer < depth-1; layer++ {
		for i := 0; i < width; i++ {
			// Connect to 2-3 random nodes in the next layer
			targets := rng.Perm(width)[:3]
			for _, j := range targets {
				g.MustAddEdge(
					fmt.Sprintf("n%d_%d", layer, i),
					fmt.Sprintf("n%d_%d", layer+1, j),
				)
			}
		}
	}

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Errorf("Layout took too long: %v", elapsed)
	}
	expectedNodes := width * depth
	if len(layout.Nodes) != expectedNodes {
		t.Errorf("Expected %d nodes, got %d", expectedNodes, len(layout.Nodes))
	}

	t.Logf("Wide graph (%d nodes per layer, %d layers): %v", width, depth, elapsed)
}

// TestStress_AllAlgorithms tests all algorithm combinations on a medium graph.
func TestStress_AllAlgorithms(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const nodeCount = 100
	const edgeCount = 200

	g := NewGraph()
	for i := 0; i < nodeCount; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < edgeCount; i++ {
		from := fmt.Sprintf("n%d", rng.Intn(nodeCount))
		to := fmt.Sprintf("n%d", rng.Intn(nodeCount))
		if from != to {
			g.AddEdge(from, to)
		}
	}

	algorithms := []struct {
		name string
		algo RankAlgorithm
	}{
		{"LongestPath", LongestPath},
		{"TightTree", TightTree},
		{"NetworkSimplex", NetworkSimplex},
	}

	acyclicers := []struct {
		name string
		acyc Acyclicer
	}{
		{"DFS", DFSAcyclicer},
		{"Greedy", GreedyAcyclicer},
	}

	for _, algo := range algorithms {
		for _, acyc := range acyclicers {
			t.Run(fmt.Sprintf("%s_%s", algo.name, acyc.name), func(t *testing.T) {
				start := time.Now()
				layout := g.Layout(Options{
					Algorithm: algo.algo,
					Acyclicer: acyc.acyc,
					NodeSep:   50,
					RankSep:   100,
				})
				elapsed := time.Since(start)

				if len(layout.Nodes) != nodeCount {
					t.Errorf("Expected %d nodes, got %d", nodeCount, len(layout.Nodes))
				}

				t.Logf("%s + %s: %v", algo.name, acyc.name, elapsed)
			})
		}
	}
}
