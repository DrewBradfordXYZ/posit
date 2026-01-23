package posit

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"math/rand"
)

var benchSave = flag.Bool("bench-save", false, "Save benchmark results as baseline")

// benchProfile defines a graph profile for benchmarking.
type benchProfile struct {
	name  string
	build func() *Graph
}

// benchResult holds metrics for one profile run.
type benchResult struct {
	Name      string  `json:"name"`
	TimeMs    float64 `json:"time_ms"`
	Crossings int     `json:"crossings"`
	Area      float64 `json:"area"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Layers    int     `json:"layers"`
}

// benchBaseline is the JSON format for saved baselines.
type benchBaseline struct {
	Profiles []benchResult `json:"profiles"`
}

func TestBenchmarkReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	profiles := []benchProfile{
		{"Large (500n/1000e)", buildLargeGraph},
		{"Dense (100n/~2000e)", buildDenseGraph},
		{"Wide (100x5)", buildWideGraph},
		{"Deep (200-chain)", buildDeepGraph},
		{"Medium (100n/200e)", buildMediumGraph},
	}

	results := make([]benchResult, len(profiles))
	for i, p := range profiles {
		results[i] = runBenchProfile(p)
	}

	// Print results table
	printResultsTable(t, results)

	// Load and compare baseline if it exists
	baseline, err := loadBaseline()
	if err == nil && baseline != nil {
		printBaselineComparison(t, results, baseline.Profiles)
	}

	// Save baseline if requested
	if *benchSave {
		saveBaseline(t, results)
	}
}

func runBenchProfile(p benchProfile) benchResult {
	g := p.build()

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start)

	return benchResult{
		Name:      p.name,
		TimeMs:    float64(elapsed.Milliseconds()),
		Crossings: countLayoutCrossings(layout),
		Area:      layoutArea(layout),
		Width:     layoutWidth(layout),
		Height:    layoutHeight(layout),
		Layers:    countLayers(layout),
	}
}

// --- Graph Builders ---

func buildLargeGraph() *Graph {
	const nodeCount = 500
	const edgeCount = 1000

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
	return g
}

func buildDenseGraph() *Graph {
	const nodeCount = 100

	g := NewGraph()
	for i := 0; i < nodeCount; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < nodeCount; i++ {
		for j := 0; j < nodeCount; j++ {
			if i != j && rng.Float32() < 0.20 {
				g.AddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", j))
			}
		}
	}
	return g
}

func buildWideGraph() *Graph {
	const width = 100
	const depth = 5

	g := NewGraph()
	for layer := 0; layer < depth; layer++ {
		for i := 0; i < width; i++ {
			g.AddNode(fmt.Sprintf("n%d_%d", layer, i), NodeOptions{Width: 50, Height: 30})
		}
	}

	rng := rand.New(rand.NewSource(42))
	for layer := 0; layer < depth-1; layer++ {
		for i := 0; i < width; i++ {
			targets := rng.Perm(width)[:3]
			for _, j := range targets {
				g.MustAddEdge(
					fmt.Sprintf("n%d_%d", layer, i),
					fmt.Sprintf("n%d_%d", layer+1, j),
				)
			}
		}
	}
	return g
}

func buildDeepGraph() *Graph {
	const depth = 200

	g := NewGraph()
	for i := 0; i < depth; i++ {
		g.AddNode(fmt.Sprintf("n%d", i), NodeOptions{Width: 50, Height: 30})
	}
	for i := 0; i < depth-1; i++ {
		g.MustAddEdge(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1))
	}
	return g
}

func buildMediumGraph() *Graph {
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
	return g
}

// --- Metric Functions ---

// countLayoutCrossings counts edge crossings by testing segments from
// different edges for geometric intersection.
func countLayoutCrossings(layout *Layout) int {
	// Collect segments grouped by edge
	type segment struct {
		x1, y1, x2, y2 float64
		minX, minY      float64
		maxX, maxY      float64
	}
	type edgeSegments struct {
		segs []segment
	}

	var edges []edgeSegments
	for _, e := range layout.Edges {
		pts := e.Points
		if len(pts) < 2 {
			continue
		}
		var segs []segment
		for i := 0; i < len(pts)-1; i++ {
			s := segment{
				x1: pts[i].X, y1: pts[i].Y,
				x2: pts[i+1].X, y2: pts[i+1].Y,
			}
			s.minX = math.Min(s.x1, s.x2)
			s.minY = math.Min(s.y1, s.y2)
			s.maxX = math.Max(s.x1, s.x2)
			s.maxY = math.Max(s.y1, s.y2)
			segs = append(segs, s)
		}
		edges = append(edges, edgeSegments{segs})
	}

	// Test all pairs of edges (not segments within the same edge)
	crossings := 0
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			for _, a := range edges[i].segs {
				for _, b := range edges[j].segs {
					// Bounding box pre-filter
					if a.maxX < b.minX || b.maxX < a.minX ||
						a.maxY < b.minY || b.maxY < a.minY {
						continue
					}
					if segmentsIntersect(a.x1, a.y1, a.x2, a.y2,
						b.x1, b.y1, b.x2, b.y2) {
						crossings++
					}
				}
			}
		}
	}
	return crossings
}

// segmentsIntersect tests if two line segments properly intersect (cross each other).
// Shared endpoints don't count as crossings.
func segmentsIntersect(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2 float64) bool {
	// Skip if segments share an endpoint
	const eps = 1e-9
	if (math.Abs(ax1-bx1) < eps && math.Abs(ay1-by1) < eps) ||
		(math.Abs(ax1-bx2) < eps && math.Abs(ay1-by2) < eps) ||
		(math.Abs(ax2-bx1) < eps && math.Abs(ay2-by1) < eps) ||
		(math.Abs(ax2-bx2) < eps && math.Abs(ay2-by2) < eps) {
		return false
	}

	// Cross product orientation test
	d1 := cross(bx1, by1, bx2, by2, ax1, ay1)
	d2 := cross(bx1, by1, bx2, by2, ax2, ay2)
	d3 := cross(ax1, ay1, ax2, ay2, bx1, by1)
	d4 := cross(ax1, ay1, ax2, ay2, bx2, by2)

	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}

	return false
}

// cross computes the cross product of vectors (bx-ax, by-ay) and (cx-ax, cy-ay).
func cross(ax, ay, bx, by, cx, cy float64) float64 {
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
}

// layoutArea computes the bounding box area of the layout.
func layoutArea(layout *Layout) float64 {
	return layoutWidth(layout) * layoutHeight(layout)
}

func layoutWidth(layout *Layout) float64 {
	if len(layout.Nodes) == 0 {
		return 0
	}
	minX := math.MaxFloat64
	maxX := -math.MaxFloat64
	for _, n := range layout.Nodes {
		left := n.X - n.Width/2
		right := n.X + n.Width/2
		if left < minX {
			minX = left
		}
		if right > maxX {
			maxX = right
		}
	}
	return maxX - minX
}

func layoutHeight(layout *Layout) float64 {
	if len(layout.Nodes) == 0 {
		return 0
	}
	minY := math.MaxFloat64
	maxY := -math.MaxFloat64
	for _, n := range layout.Nodes {
		top := n.Y - n.Height/2
		bottom := n.Y + n.Height/2
		if top < minY {
			minY = top
		}
		if bottom > maxY {
			maxY = bottom
		}
	}
	return maxY - minY
}

// countLayers counts distinct Y positions (layers) in the layout.
func countLayers(layout *Layout) int {
	ySet := make(map[float64]bool)
	for _, n := range layout.Nodes {
		// Round to avoid float precision issues
		y := math.Round(n.Y*100) / 100
		ySet[y] = true
	}
	return len(ySet)
}

// --- Output Formatting ---

func printResultsTable(t *testing.T, results []benchResult) {
	t.Log("")
	t.Log("Benchmark Report")
	t.Log("================")
	t.Logf("%-20s | %8s | %9s | %12s | %6s", "Profile", "Time", "Crossings", "Size (WxH)", "Layers")
	t.Logf("%-20s-+-%8s-+-%9s-+-%12s-+-%6s", strings.Repeat("-", 20), "--------", "---------", "------------", "------")

	for _, r := range results {
		size := fmt.Sprintf("%.0fx%.0f", r.Width, r.Height)
		t.Logf("%-20s | %6.0fms | %9d | %12s | %6d", r.Name, r.TimeMs, r.Crossings, size, r.Layers)
	}
	t.Log("")
}

func printBaselineComparison(t *testing.T, current, baseline []benchResult) {
	// Build baseline map by name
	baseMap := make(map[string]benchResult)
	for _, b := range baseline {
		baseMap[b.Name] = b
	}

	t.Log("Baseline Comparison")
	t.Log("===================")
	t.Logf("%-20s | %8s | %11s | %8s", "Profile", "Time Δ", "Crossings Δ", "Area Δ")
	t.Logf("%-20s-+-%8s-+-%11s-+-%8s", strings.Repeat("-", 20), "--------", "-----------", "--------")

	for _, r := range current {
		b, ok := baseMap[r.Name]
		if !ok {
			t.Logf("%-20s | %8s | %11s | %8s", r.Name, "new", "new", "new")
			continue
		}

		timeD := deltaStr(b.TimeMs, r.TimeMs)
		crossD := deltaStr(float64(b.Crossings), float64(r.Crossings))
		areaD := deltaStr(b.Area, r.Area)
		t.Logf("%-20s | %8s | %11s | %8s", r.Name, timeD, crossD, areaD)
	}
	t.Log("")
}

func deltaStr(old, new float64) string {
	if old == 0 {
		if new == 0 {
			return "="
		}
		return "+∞"
	}
	pct := (new - old) / old * 100
	if math.Abs(pct) < 0.5 {
		return "="
	}
	sign := "+"
	if pct < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.0f%%", sign, pct)
}

// --- Baseline Persistence ---

const baselineFile = "benchmark_baseline.json"

func loadBaseline() (*benchBaseline, error) {
	data, err := os.ReadFile(baselineFile)
	if err != nil {
		return nil, err
	}
	var b benchBaseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func saveBaseline(t *testing.T, results []benchResult) {
	b := benchBaseline{Profiles: results}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal baseline: %v", err)
	}
	if err := os.WriteFile(baselineFile, data, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}
	t.Logf("Baseline saved to %s", baselineFile)
}
