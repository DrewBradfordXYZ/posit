package posit

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"math/rand"
)

var (
	benchSave   = flag.Bool("bench-save", false, "Save benchmark results as baseline")
	benchExport = flag.Bool("bench-export", false, "Export graph profiles as JSON for cross-language benchmarks")
)

// benchProfile defines a graph profile for benchmarking.
type benchProfile struct {
	name  string
	build func() *Graph
}

// benchResult holds metrics for one profile run.
type benchResult struct {
	Name      string  `json:"name"`
	Nodes     int     `json:"nodes"`
	Edges     int     `json:"edges"`
	TimeMs    float64 `json:"time_ms"`
	Crossings int     `json:"crossings"`
	Area      float64 `json:"area"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Layers    int     `json:"layers"`
}

// benchBaseline is the JSON format for saved baselines.
type benchBaseline struct {
	Environment benchEnvironment `json:"environment"`
	Profiles    []benchResult    `json:"profiles"`
}

// benchEnvironment records the system info when baseline was saved.
type benchEnvironment struct {
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	NumCPU    int    `json:"num_cpu"`
}

func currentEnvironment() benchEnvironment {
	return benchEnvironment{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
	}
}

func TestBenchmarkReport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	profiles := []benchProfile{
		{"Large", buildLargeGraph},
		{"Dense", buildDenseGraph},
		{"Wide", buildWideGraph},
		{"Deep", buildDeepGraph},
		{"Medium", buildMediumGraph},
		{"CHDI", buildCHDIGraph},
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
		t.Logf("Baseline environment: %s %s/%s (%d CPUs)",
			baseline.Environment.GoVersion, baseline.Environment.GOOS,
			baseline.Environment.GOARCH, baseline.Environment.NumCPU)
		printBaselineComparison(t, results, baseline.Profiles)
	}

	// Save baseline if requested
	if *benchSave {
		saveBaseline(t, results)
	}

	// Export graph profiles for cross-language benchmarks
	if *benchExport {
		exportProfiles(t, profiles)
	}
}

func runBenchProfile(p benchProfile) benchResult {
	g := p.build()
	nodeCount := len(g.nodes)
	edgeCount := len(g.edges)

	start := time.Now()
	layout := g.Layout()
	elapsed := time.Since(start).Seconds() * 1000

	return benchResult{
		Name:      p.name,
		Nodes:     nodeCount,
		Edges:     edgeCount,
		TimeMs:    math.Round(elapsed*100) / 100, // informational only
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

// buildCHDIGraph builds a graph structure similar to the CHDI QuickBase schema:
// - 109 tables (nodes)
// - Several hub tables with 50-200+ relationships
// - Mix of hub tables and leaf tables
// - Self-referencing relationships
func buildCHDIGraph() *Graph {
	g := NewGraph()

	// Define hub tables (high connectivity) - these mirror the actual CHDI hubs
	hubs := []struct {
		name       string
		childEdges int
		parentEdges int
	}{
		{"AgreementWorkflows", 92, 111},      // 203 total relationships
		{"PaymentWorkflows", 120, 6},          // 126 total
		{"ClinicalStudies", 64, 17},           // 81 total
		{"ProjectWorkflowDocs", 30, 56},       // 86 total
		{"ProjectCodes", 4, 58},               // 62 total
		{"Organizations", 4, 45},              // 49 total
		{"OrgContacts", 4, 34},                // 38 total
		{"Materials", 15, 3},                  // 18 total
		{"MaterialWebSubs", 32, 2},            // 34 total
		{"AnimalLineWebSubs", 22, 2},          // 24 total
		{"AnimalLines", 25, 2},                // 27 total
	}

	// Add hub nodes
	for _, h := range hubs {
		g.AddNode(h.name, NodeOptions{Width: 120, Height: 40})
	}

	// Add regular tables (98 more to reach 109 total)
	regularTables := 98
	for i := 0; i < regularTables; i++ {
		g.AddNode(fmt.Sprintf("Table%d", i), NodeOptions{Width: 80, Height: 30})
	}

	rng := rand.New(rand.NewSource(42))

	// Connect hubs to each other (mimics cross-hub relationships)
	for i := 0; i < len(hubs); i++ {
		for j := i + 1; j < len(hubs); j++ {
			if rng.Float32() < 0.6 {
				g.AddEdge(hubs[i].name, hubs[j].name)
			}
		}
	}

	// Connect regular tables to hubs based on hub connectivity
	regularTableNames := make([]string, regularTables)
	for i := 0; i < regularTables; i++ {
		regularTableNames[i] = fmt.Sprintf("Table%d", i)
	}

	for _, h := range hubs {
		// Add child edges (hub -> regular tables) - use 50% of actual count
		targetCount := h.childEdges / 2
		if targetCount > regularTables {
			targetCount = regularTables / 2
		}
		targets := rng.Perm(regularTables)[:targetCount]
		for _, t := range targets {
			g.AddEdge(h.name, regularTableNames[t])
		}

		// Add parent edges (regular tables -> hub) - use 50% of actual count
		sourceCount := h.parentEdges / 2
		if sourceCount > regularTables {
			sourceCount = regularTables / 2
		}
		sources := rng.Perm(regularTables)[:sourceCount]
		for _, s := range sources {
			g.AddEdge(regularTableNames[s], h.name)
		}
	}

	// Add some inter-table edges (tables referencing other tables)
	for i := 0; i < regularTables; i++ {
		edgeCount := rng.Intn(3) // 0-2 edges per table
		for j := 0; j < edgeCount; j++ {
			target := rng.Intn(regularTables)
			if target != i {
				g.AddEdge(regularTableNames[i], regularTableNames[target])
			}
		}
	}

	return g
}

// --- Standard Go Benchmarks ---

func BenchmarkLayout_Large(b *testing.B) {
	g := buildLargeGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_Dense(b *testing.B) {
	g := buildDenseGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_Wide(b *testing.B) {
	g := buildWideGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_Deep(b *testing.B) {
	g := buildDeepGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_Medium(b *testing.B) {
	g := buildMediumGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_CHDI(b *testing.B) {
	g := buildCHDIGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout()
	}
}

func BenchmarkLayout_CHDI_XSimplex(b *testing.B) {
	g := buildCHDIGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(Options{XCoordAlgorithm: XNetworkSimplex})
	}
}

func BenchmarkLayout_CHDI_XSimplex_AntiStack(b *testing.B) {
	g := buildCHDIGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Layout(Options{XCoordAlgorithm: XNetworkSimplex, PreventStacking: true})
	}
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

// countLayers counts distinct Y positions (layers) in the layout using
// tolerance-based grouping. Positions within 1.0 unit are considered the same layer.
func countLayers(layout *Layout) int {
	if len(layout.Nodes) == 0 {
		return 0
	}

	// Collect all Y center positions
	ys := make([]float64, 0, len(layout.Nodes))
	for _, n := range layout.Nodes {
		ys = append(ys, n.Y)
	}
	sort.Float64s(ys)

	// Count distinct layers: new layer when gap > tolerance
	const tolerance = 1.0
	layers := 1
	prev := ys[0]
	for i := 1; i < len(ys); i++ {
		if ys[i]-prev > tolerance {
			layers++
			prev = ys[i]
		}
	}
	return layers
}

// --- Output Formatting ---

func printResultsTable(t *testing.T, results []benchResult) {
	t.Log("")
	t.Log("Layout Quality Report")
	t.Logf("Environment: %s %s/%s (%d CPUs)",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	t.Log("Note: Use 'go test -bench=BenchmarkLayout' + benchstat for timing")
	t.Log("=====================")
	t.Logf("%-12s | %5s %5s | %9s | %12s | %6s | %10s",
		"Profile", "Nodes", "Edges", "Crossings", "Size (WxH)", "Layers", "~Time")
	t.Logf("%-12s-+-%5s-%5s-+-%9s-+-%12s-+-%6s-+-%10s",
		strings.Repeat("-", 12), "-----", "-----", "---------", "------------", "------", "----------")

	for _, r := range results {
		size := fmt.Sprintf("%.0fx%.0f", r.Width, r.Height)
		timeStr := formatTime(r.TimeMs)
		t.Logf("%-12s | %5d %5d | %9d | %12s | %6d | %10s",
			r.Name, r.Nodes, r.Edges, r.Crossings, size, r.Layers, timeStr)
	}
	t.Log("")
}

func formatTime(ms float64) string {
	if ms < 1 {
		return fmt.Sprintf("%.2fms", ms)
	}
	if ms < 10 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.0fms", ms)
}

func printBaselineComparison(t *testing.T, current, baseline []benchResult) {
	// Build baseline map by name
	baseMap := make(map[string]benchResult)
	for _, b := range baseline {
		baseMap[b.Name] = b
	}

	t.Log("Baseline Comparison (deterministic metrics only)")
	t.Log("================================================")
	t.Logf("%-12s | %11s | %8s | %8s", "Profile", "Crossings Δ", "Area Δ", "Layers Δ")
	t.Logf("%-12s-+-%11s-+-%8s-+-%8s", strings.Repeat("-", 12), "-----------", "--------", "--------")

	for _, r := range current {
		b, ok := baseMap[r.Name]
		if !ok {
			t.Logf("%-12s | %11s | %8s | %8s", r.Name, "new", "new", "new")
			continue
		}

		crossD := deltaStr(float64(b.Crossings), float64(r.Crossings))
		areaD := deltaStr(b.Area, r.Area)
		layerD := deltaStr(float64(b.Layers), float64(r.Layers))
		t.Logf("%-12s | %11s | %8s | %8s", r.Name, crossD, areaD, layerD)
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
	b := benchBaseline{
		Environment: currentEnvironment(),
		Profiles:    results,
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal baseline: %v", err)
	}
	if err := os.WriteFile(baselineFile, data, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}
	t.Logf("Baseline saved to %s", baselineFile)
}

// --- Graph Export for Cross-Language Benchmarks ---

type exportedProfile struct {
	Name  string         `json:"name"`
	Nodes []exportedNode `json:"nodes"`
	Edges []exportedEdge `json:"edges"`
}

type exportedNode struct {
	ID     string  `json:"id"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type exportedEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

const profilesExportFile = "_bench/profiles.json"

func exportProfiles(t *testing.T, profiles []benchProfile) {
	var exported []exportedProfile
	for _, p := range profiles {
		g := p.build()
		ep := exportedProfile{Name: p.name}

		// Export nodes sorted by insertion order
		type nodeEntry struct {
			id    string
			node  *node
			order int
		}
		var entries []nodeEntry
		for id, n := range g.nodes {
			entries = append(entries, nodeEntry{id, n, n.insertOrder})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].order < entries[j].order
		})
		for _, e := range entries {
			ep.Nodes = append(ep.Nodes, exportedNode{
				ID:     e.id,
				Width:  e.node.width,
				Height: e.node.height,
			})
		}

		// Export edges
		for _, e := range g.edges {
			ep.Edges = append(ep.Edges, exportedEdge{
				From: e.from,
				To:   e.to,
			})
		}

		exported = append(exported, ep)
	}

	data, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal profiles: %v", err)
	}
	if err := os.WriteFile(profilesExportFile, data, 0644); err != nil {
		t.Fatalf("Failed to write profiles: %v", err)
	}
	t.Logf("Profiles exported to %s", profilesExportFile)
}
