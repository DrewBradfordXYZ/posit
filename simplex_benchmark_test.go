package posit

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Simplex Performance Benchmarks
//
// These benchmarks are designed to measure network simplex performance
// as we implement optimizations from SIMPLEX_IMPROVEMENTS.md.
//
// Run with: go test -bench=BenchmarkSimplex -benchtime=3s -count=5 | tee bench.txt
// Compare with: benchstat old.txt new.txt

// --- Graph Profiles for Simplex Testing ---

// buildChainGraph creates a linear chain: A → B → C → ... → N
// This profile benefits most from subtree removal optimization.
func buildChainGraph(n int) *Graph {
	g := NewGraph()
	for i := 0; i < n; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 100, Height: 50})
	}
	for i := 0; i < n-1; i++ {
		g.MustAddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1))
	}
	return g
}

// buildFanOutGraph creates a fan-out pattern: root → many children → many grandchildren
// Tests crossing minimization and X coordinate assignment.
func buildFanOutGraph(fanout, depth int) *Graph {
	g := NewGraph()
	g.AddNode("root", NodeOptions{Width: 100, Height: 50})

	id := 1
	parents := []string{"root"}
	for layer := 0; layer < depth; layer++ {
		var children []string
		for _, p := range parents {
			for i := 0; i < fanout; i++ {
				child := fmt.Sprintf("n%d", id)
				id++
				g.AddNode(child, NodeOptions{Width: 100, Height: 50})
				g.MustAddEdge(p, child)
				children = append(children, child)
			}
		}
		parents = children
	}
	return g
}

// buildDiamondGraph creates diamond patterns that stress X coordinate assignment.
// Pattern: A → B,C → D (many such diamonds connected)
func buildDiamondGraph(n int) *Graph {
	g := NewGraph()
	for i := 0; i < n; i++ {
		base := i * 4
		g.AddNode(fmt.Sprintf("n%d", base), NodeOptions{Width: 100, Height: 50})
		g.AddNode(fmt.Sprintf("n%d", base+1), NodeOptions{Width: 100, Height: 50})
		g.AddNode(fmt.Sprintf("n%d", base+2), NodeOptions{Width: 100, Height: 50})
		g.AddNode(fmt.Sprintf("n%d", base+3), NodeOptions{Width: 100, Height: 50})

		g.MustAddEdge(fmt.Sprintf("n%d", base), fmt.Sprintf("n%d", base+1))
		g.MustAddEdge(fmt.Sprintf("n%d", base), fmt.Sprintf("n%d", base+2))
		g.MustAddEdge(fmt.Sprintf("n%d", base+1), fmt.Sprintf("n%d", base+3))
		g.MustAddEdge(fmt.Sprintf("n%d", base+2), fmt.Sprintf("n%d", base+3))

		// Connect diamonds
		if i > 0 {
			prevBase := (i - 1) * 4
			g.MustAddEdge(fmt.Sprintf("n%d", prevBase+3), fmt.Sprintf("n%d", base))
		}
	}
	return g
}

// buildLayeredGraph creates a regular layered graph with specified width and depth.
// Each node connects to 1-3 nodes in the next layer.
func buildLayeredGraph(width, depth int, seed int64) *Graph {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))

	for layer := 0; layer < depth; layer++ {
		for i := 0; i < width; i++ {
			g.AddNode(fmt.Sprintf("L%d_%d", layer, i), NodeOptions{Width: 100, Height: 50})
		}
	}

	for layer := 0; layer < depth-1; layer++ {
		for i := 0; i < width; i++ {
			// Connect to 1-3 nodes in next layer
			numEdges := 1 + rng.Intn(3)
			targets := rng.Perm(width)[:numEdges]
			for _, j := range targets {
				g.MustAddEdge(
					fmt.Sprintf("L%d_%d", layer, i),
					fmt.Sprintf("L%d_%d", layer+1, j),
				)
			}
		}
	}
	return g
}

// --- Y Ranking (Network Simplex) Benchmarks ---

func BenchmarkSimplex_Y_Chain50(b *testing.B) {
	g := buildChainGraph(50)
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_Chain100(b *testing.B) {
	g := buildChainGraph(100)
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_Chain200(b *testing.B) {
	g := buildChainGraph(200)
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_FanOut3x3(b *testing.B) {
	g := buildFanOutGraph(3, 3) // 1 + 3 + 9 + 27 = 40 nodes
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_FanOut4x3(b *testing.B) {
	g := buildFanOutGraph(4, 3) // 1 + 4 + 16 + 64 = 85 nodes
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_Diamond25(b *testing.B) {
	g := buildDiamondGraph(25) // 100 nodes
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_Layered10x5(b *testing.B) {
	g := buildLayeredGraph(10, 5, 42) // 50 nodes
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_Y_Layered20x5(b *testing.B) {
	g := buildLayeredGraph(20, 5, 42) // 100 nodes
	opts := Options{Algorithm: NetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

// --- X Coordinate (Network Simplex vs BK) Benchmarks ---

func BenchmarkSimplex_X_BK_Layered10x5(b *testing.B) {
	g := buildLayeredGraph(10, 5, 42)
	opts := Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XBrandesKopf}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_X_Simplex_Layered10x5(b *testing.B) {
	g := buildLayeredGraph(10, 5, 42)
	opts := Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XNetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_X_BK_Layered20x5(b *testing.B) {
	g := buildLayeredGraph(20, 5, 42)
	opts := Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XBrandesKopf}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_X_Simplex_Layered20x5(b *testing.B) {
	g := buildLayeredGraph(20, 5, 42)
	opts := Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XNetworkSimplex}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

// --- Anti-Stacking Benchmarks ---

func BenchmarkSimplex_AntiStack_Chain50(b *testing.B) {
	g := buildChainGraph(50)
	opts := Options{
		Algorithm:       NetworkSimplex,
		XCoordAlgorithm: XNetworkSimplex,
		PreventStacking: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_AntiStack_Diamond25(b *testing.B) {
	g := buildDiamondGraph(25)
	opts := Options{
		Algorithm:       NetworkSimplex,
		XCoordAlgorithm: XNetworkSimplex,
		PreventStacking: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

func BenchmarkSimplex_AntiStack_Layered10x5(b *testing.B) {
	g := buildLayeredGraph(10, 5, 42)
	opts := Options{
		Algorithm:       NetworkSimplex,
		XCoordAlgorithm: XNetworkSimplex,
		PreventStacking: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(opts)
	}
}

// --- Scaling Test ---
// Run with: go test -run TestSimplexScaling -v

func TestSimplexScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scaling test in short mode")
	}

	sizes := []int{20, 40, 60, 80, 100}

	t.Log("")
	t.Log("Network Simplex Scaling Analysis")
	t.Log("=================================")
	t.Logf("%-6s | %-8s | %-10s | %-10s | %-10s | %-10s",
		"Nodes", "Edges", "Y-Simplex", "X-BK", "X-Simplex", "X+AntiStack")
	t.Log("-------|----------|------------|------------|------------|------------")

	for _, n := range sizes {
		g := buildLayeredGraph(n/5, 5, 42)
		nodeCount := len(g.nodes)
		edgeCount := len(g.edges)

		// Y-Simplex only (BK for X)
		yTime := benchOnce(func() { g.Layout(Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XBrandesKopf}) })

		// X-BK (full layout)
		bkTime := benchOnce(func() { g.Layout(Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XBrandesKopf}) })

		// X-Simplex (full layout)
		xsTime := benchOnce(func() { g.Layout(Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XNetworkSimplex}) })

		// X-Simplex + AntiStacking
		asTime := benchOnce(func() {
			g.Layout(Options{
				Algorithm:       NetworkSimplex,
				XCoordAlgorithm: XNetworkSimplex,
				PreventStacking: true,
			})
		})

		t.Logf("%-6d | %-8d | %10s | %10s | %10s | %10s",
			nodeCount, edgeCount, formatDuration(yTime), formatDuration(bkTime),
			formatDuration(xsTime), formatDuration(asTime))
	}
	t.Log("")
}

func benchOnce(f func()) time.Duration {
	// Warmup
	f()

	// Measure (average of 3)
	var total time.Duration
	for i := 0; i < 3; i++ {
		start := time.Now()
		f()
		total += time.Since(start)
	}
	return total / 3
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// --- Component-Level Profiling Test ---
// This test helps identify which simplex operations are slowest.

func TestSimplexProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping profile test in short mode")
	}

	// Build a graph that exercises simplex
	g := buildLayeredGraph(20, 5, 42) // 100 nodes

	t.Log("")
	t.Log("Simplex Component Profile (100-node layered graph)")
	t.Log("===================================================")
	t.Log("Note: These are full layout times; internal phases cannot be isolated")
	t.Log("without modifying the code. Use go tool pprof for detailed profiling:")
	t.Log("")
	t.Log("  go test -cpuprofile=cpu.prof -bench=BenchmarkSimplex_X_Simplex_Layered20x5")
	t.Log("  go tool pprof cpu.prof")
	t.Log("")

	configs := []struct {
		name string
		opts Options
	}{
		{"LongestPath + BK", Options{Algorithm: LongestPath, XCoordAlgorithm: XBrandesKopf}},
		{"Simplex + BK", Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XBrandesKopf}},
		{"Simplex + X-Simplex", Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XNetworkSimplex}},
		{"Simplex + X-Simplex + AntiStack", Options{Algorithm: NetworkSimplex, XCoordAlgorithm: XNetworkSimplex, PreventStacking: true}},
	}

	for _, c := range configs {
		d := benchOnce(func() { g.Layout(c.opts) })
		t.Logf("%-35s: %s", c.name, formatDuration(d))
	}
	t.Log("")
}
