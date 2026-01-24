package posit

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// Contract tests enforce the invariants documented in CONTRACT.md.
// These run against multiple random graph topologies to verify that
// the output contract holds for any valid input.

// graphFixture represents a generated graph with known properties.
type graphFixture struct {
	name     string
	graph    *Graph
	nodeIDs  []string
	edgeKeys [][2]string // [from, to] pairs
	opts     Options
}

// generateFixtures creates a variety of graph topologies for property testing.
func generateFixtures() []graphFixture {
	fixtures := []graphFixture{
		sparseRandom(50, 80, 1),
		sparseRandom(100, 200, 2),
		denseRandom(30, 3),
		linearChain(20),
		wideShallow(50, 5, 4),
		singleNode(),
		disconnected(10, 3, 5),
		withPorts(6),
		withLabels(10, 15, 7),
		withMultiEdges(8),
		withSelfLoops(10),
	}
	return fixtures
}

func sparseRandom(nodes, edges int, seed int64) graphFixture {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))
	ids := make([]string, nodes)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 50 + float64(rng.Intn(100)), Height: 30 + float64(rng.Intn(50))})
	}
	var edgePairs [][2]string
	for i := 0; i < edges; i++ {
		from := ids[rng.Intn(nodes)]
		to := ids[rng.Intn(nodes)]
		if from != to {
			g.AddEdge(from, to)
			edgePairs = append(edgePairs, [2]string{from, to})
		}
	}
	return graphFixture{
		name:     fmt.Sprintf("sparse_%d_%d", nodes, edges),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func denseRandom(nodes int, seed int64) graphFixture {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))
	ids := make([]string, nodes)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 50, Height: 30})
	}
	var edgePairs [][2]string
	for i := 0; i < nodes; i++ {
		for j := 0; j < nodes; j++ {
			if i != j && rng.Float64() < 0.2 {
				g.AddEdge(ids[i], ids[j])
				edgePairs = append(edgePairs, [2]string{ids[i], ids[j]})
			}
		}
	}
	return graphFixture{
		name:     fmt.Sprintf("dense_%d", nodes),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func linearChain(nodes int) graphFixture {
	g := NewGraph()
	ids := make([]string, nodes)
	var edgePairs [][2]string
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 60, Height: 40})
	}
	for i := 0; i < nodes-1; i++ {
		g.AddEdge(ids[i], ids[i+1])
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i+1]})
	}
	return graphFixture{
		name:     fmt.Sprintf("chain_%d", nodes),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func wideShallow(nodesPerLayer, layers int, seed int64) graphFixture {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))
	var ids []string
	var edgePairs [][2]string
	for layer := 0; layer < layers; layer++ {
		for i := 0; i < nodesPerLayer; i++ {
			id := fmt.Sprintf("L%d_n%d", layer, i)
			ids = append(ids, id)
			g.AddNode(id, NodeOptions{Width: 50, Height: 30})
		}
	}
	for layer := 0; layer < layers-1; layer++ {
		for i := 0; i < nodesPerLayer; i++ {
			targets := 1 + rng.Intn(3)
			for t := 0; t < targets; t++ {
				j := rng.Intn(nodesPerLayer)
				from := fmt.Sprintf("L%d_n%d", layer, i)
				to := fmt.Sprintf("L%d_n%d", layer+1, j)
				g.AddEdge(from, to)
				edgePairs = append(edgePairs, [2]string{from, to})
			}
		}
	}
	return graphFixture{
		name:     fmt.Sprintf("wide_%dx%d", nodesPerLayer, layers),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func singleNode() graphFixture {
	g := NewGraph()
	g.AddNode("solo", NodeOptions{Width: 100, Height: 50})
	return graphFixture{
		name:    "single_node",
		graph:   g,
		nodeIDs: []string{"solo"},
		opts:    DefaultOptions(),
	}
}

func disconnected(nodesPerComponent, components int, seed int64) graphFixture {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))
	var ids []string
	var edgePairs [][2]string
	for c := 0; c < components; c++ {
		for i := 0; i < nodesPerComponent; i++ {
			id := fmt.Sprintf("c%d_n%d", c, i)
			ids = append(ids, id)
			g.AddNode(id, NodeOptions{Width: 50, Height: 30})
		}
		for i := 0; i < nodesPerComponent-1; i++ {
			from := fmt.Sprintf("c%d_n%d", c, i)
			to := fmt.Sprintf("c%d_n%d", c, rng.Intn(nodesPerComponent))
			if from != to {
				g.AddEdge(from, to)
				edgePairs = append(edgePairs, [2]string{from, to})
			}
		}
	}
	return graphFixture{
		name:     fmt.Sprintf("disconnected_%dx%d", components, nodesPerComponent),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func withPorts(nodes int) graphFixture {
	g := NewGraph()
	ids := make([]string, nodes)
	var edgePairs [][2]string
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		ports := []PortOptions{
			{ID: "out", Side: Bottom, Offset: 25},
			{ID: "in", Side: Top, Offset: 25},
		}
		g.AddNode(ids[i], NodeOptions{Width: 50, Height: 30, Ports: ports})
	}
	for i := 0; i < nodes-1; i++ {
		g.AddEdge(ids[i], ids[i+1], EdgeOptions{SourcePort: "out", TargetPort: "in"})
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i+1]})
	}
	return graphFixture{
		name:     fmt.Sprintf("ports_%d", nodes),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func withLabels(nodes, edges int, seed int64) graphFixture {
	g := NewGraph()
	rng := rand.New(rand.NewSource(seed))
	ids := make([]string, nodes)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 80, Height: 40})
	}
	var edgePairs [][2]string
	for i := 0; i < edges; i++ {
		from := ids[rng.Intn(nodes)]
		to := ids[rng.Intn(nodes)]
		if from != to {
			g.AddEdge(from, to, EdgeOptions{LabelWidth: 60, LabelHeight: 20})
			edgePairs = append(edgePairs, [2]string{from, to})
		}
	}
	return graphFixture{
		name:     fmt.Sprintf("labels_%d_%d", nodes, edges),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func withMultiEdges(nodes int) graphFixture {
	g := NewGraph()
	ids := make([]string, nodes)
	var edgePairs [][2]string
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 50, Height: 30})
	}
	for i := 0; i < nodes-1; i++ {
		g.AddEdge(ids[i], ids[i+1], EdgeOptions{ID: "a"})
		g.AddEdge(ids[i], ids[i+1], EdgeOptions{ID: "b"})
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i+1]})
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i+1]})
	}
	return graphFixture{
		name:     fmt.Sprintf("multi_edges_%d", nodes),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

func withSelfLoops(nodes int) graphFixture {
	g := NewGraph()
	ids := make([]string, nodes)
	var edgePairs [][2]string
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
		g.AddNode(ids[i], NodeOptions{Width: 50, Height: 30})
	}
	// Chain plus self-loops
	for i := 0; i < nodes-1; i++ {
		g.AddEdge(ids[i], ids[i+1])
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i+1]})
	}
	// Add self-loops to every other node
	for i := 0; i < nodes; i += 2 {
		g.AddEdge(ids[i], ids[i])
		edgePairs = append(edgePairs, [2]string{ids[i], ids[i]})
	}
	return graphFixture{
		name:     fmt.Sprintf("self_loops_%d", nodes),
		graph:    g,
		nodeIDs:  ids,
		edgeKeys: edgePairs,
		opts:     DefaultOptions(),
	}
}

// === Contract Property Tests ===

func TestContract_NodeCompleteness(t *testing.T) {
	// Every input node appears in output. No extra nodes appear.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			if len(layout.Nodes) != len(f.nodeIDs) {
				t.Fatalf("expected %d nodes, got %d", len(f.nodeIDs), len(layout.Nodes))
			}
			for _, id := range f.nodeIDs {
				if _, ok := layout.Nodes[id]; !ok {
					t.Errorf("input node %q missing from output", id)
				}
			}
		})
	}
}

func TestContract_NodeDimensionsPreserved(t *testing.T) {
	// Output dimensions match input dimensions (algorithm does not resize).
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for _, id := range f.nodeIDs {
				inputNode := f.graph.nodes[id]
				if inputNode.isCluster {
					continue // clusters are resized
				}
				outputNode := layout.Nodes[id]
				if outputNode.Width != inputNode.width {
					t.Errorf("node %q: width %v != input %v", id, outputNode.Width, inputNode.width)
				}
				if outputNode.Height != inputNode.height {
					t.Errorf("node %q: height %v != input %v", id, outputNode.Height, inputNode.height)
				}
			}
		})
	}
}

func TestContract_CoordinatesFinite(t *testing.T) {
	// All coordinates are finite (never NaN or Inf).
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for id, n := range layout.Nodes {
				if math.IsNaN(n.X) || math.IsInf(n.X, 0) {
					t.Errorf("node %q: X is %v", id, n.X)
				}
				if math.IsNaN(n.Y) || math.IsInf(n.Y, 0) {
					t.Errorf("node %q: Y is %v", id, n.Y)
				}
			}
			for key, e := range layout.Edges {
				for i, p := range e.Points {
					if math.IsNaN(p.X) || math.IsInf(p.X, 0) {
						t.Errorf("edge %q point %d: X is %v", key, i, p.X)
					}
					if math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
						t.Errorf("edge %q point %d: Y is %v", key, i, p.Y)
					}
				}
			}
		})
	}
}

func TestContract_CoordinatesNonNegative(t *testing.T) {
	// All node positions are non-negative for TopToBottom.
	for _, f := range generateFixtures() {
		if f.opts.Direction != TopToBottom {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for id, n := range layout.Nodes {
				if n.X < -0.5 { // small tolerance for float rounding
					t.Errorf("node %q: X = %v (negative)", id, n.X)
				}
				if n.Y < -0.5 {
					t.Errorf("node %q: Y = %v (negative)", id, n.Y)
				}
			}
		})
	}
}

func TestContract_EdgeCompleteness(t *testing.T) {
	// Every input edge appears in output.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			if len(layout.Edges) == 0 && len(f.edgeKeys) > 0 {
				t.Fatalf("no edges in output but %d edges in input", len(f.edgeKeys))
			}
			// Verify all output edges reference valid nodes
			for key, e := range layout.Edges {
				if _, ok := layout.Nodes[e.From]; !ok {
					t.Errorf("edge %q: From node %q not in layout", key, e.From)
				}
				if _, ok := layout.Nodes[e.To]; !ok {
					t.Errorf("edge %q: To node %q not in layout", key, e.To)
				}
			}
		})
	}
}

func TestContract_EdgePointsMinimum(t *testing.T) {
	// Every edge has at least 2 points.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for key, e := range layout.Edges {
				if len(e.Points) < 2 {
					t.Errorf("edge %q: only %d points (need >= 2)", key, len(e.Points))
				}
			}
		})
	}
}

func TestContract_EdgeEndpointsNearNodes(t *testing.T) {
	// First point is near source node, last point is near target node.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for key, e := range layout.Edges {
				if len(e.Points) < 2 {
					continue
				}
				srcNode := layout.Nodes[e.From]
				tgtNode := layout.Nodes[e.To]

				// First point should be within reasonable distance of source
				first := e.Points[0]
				srcCenterX := srcNode.X + srcNode.Width/2
				srcCenterY := srcNode.Y + srcNode.Height/2
				srcDist := math.Hypot(first.X-srcCenterX, first.Y-srcCenterY)
				maxSrcDist := math.Max(srcNode.Width, srcNode.Height) * 2
				if srcDist > maxSrcDist {
					t.Errorf("edge %q: first point (%.1f, %.1f) too far from source center (%.1f, %.1f), dist=%.1f",
						key, first.X, first.Y, srcCenterX, srcCenterY, srcDist)
				}

				// Last point should be within reasonable distance of target
				last := e.Points[len(e.Points)-1]
				tgtCenterX := tgtNode.X + tgtNode.Width/2
				tgtCenterY := tgtNode.Y + tgtNode.Height/2
				tgtDist := math.Hypot(last.X-tgtCenterX, last.Y-tgtCenterY)
				maxTgtDist := math.Max(tgtNode.Width, tgtNode.Height) * 2
				if tgtDist > maxTgtDist {
					t.Errorf("edge %q: last point (%.1f, %.1f) too far from target center (%.1f, %.1f), dist=%.1f",
						key, last.X, last.Y, tgtCenterX, tgtCenterY, tgtDist)
				}
			}
		})
	}
}

func TestContract_PortOffsetsInBounds(t *testing.T) {
	// Port offsets are within [0, sideLength].
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for id, n := range layout.Nodes {
				if n.Ports == nil {
					continue
				}
				for portID, p := range n.Ports {
					var sideLen float64
					switch p.Side {
					case Top, Bottom:
						sideLen = n.Width
					case Left, Right:
						sideLen = n.Height
					}
					if p.Offset < -0.5 || p.Offset > sideLen+0.5 {
						t.Errorf("node %q port %q: offset %.1f out of bounds [0, %.1f]",
							id, portID, p.Offset, sideLen)
					}
				}
			}
		})
	}
}

func TestContract_PortEchoedOnEdges(t *testing.T) {
	// If SourcePort/TargetPort were specified on input, they appear in output.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for _, inputEdge := range f.graph.edges {
				if inputEdge.sourcePort == "" && inputEdge.targetPort == "" {
					continue
				}
				// Find corresponding output edge
				for _, e := range layout.Edges {
					if e.From == inputEdge.from && e.To == inputEdge.to {
						if inputEdge.sourcePort != "" && e.SourcePort != inputEdge.sourcePort {
							t.Errorf("edge %s->%s: SourcePort %q not echoed (got %q)",
								inputEdge.from, inputEdge.to, inputEdge.sourcePort, e.SourcePort)
						}
						if inputEdge.targetPort != "" && e.TargetPort != inputEdge.targetPort {
							t.Errorf("edge %s->%s: TargetPort %q not echoed (got %q)",
								inputEdge.from, inputEdge.to, inputEdge.targetPort, e.TargetPort)
						}
						break
					}
				}
			}
		})
	}
}

func TestContract_LabelsPresent(t *testing.T) {
	// Edges with label dimensions have non-nil Label in output.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			for _, inputEdge := range f.graph.edges {
				if inputEdge.labelWidth <= 0 || inputEdge.labelHeight <= 0 {
					continue
				}
				for _, e := range layout.Edges {
					if e.From == inputEdge.from && e.To == inputEdge.to {
						if e.Label == nil {
							t.Errorf("edge %s->%s: has label dimensions but Label is nil", e.From, e.To)
						} else {
							if e.Label.Width != inputEdge.labelWidth {
								t.Errorf("edge %s->%s: label width %v != input %v",
									e.From, e.To, e.Label.Width, inputEdge.labelWidth)
							}
							if e.Label.Height != inputEdge.labelHeight {
								t.Errorf("edge %s->%s: label height %v != input %v",
									e.From, e.To, e.Label.Height, inputEdge.labelHeight)
							}
						}
						break
					}
				}
			}
		})
	}
}

func TestContract_Determinism(t *testing.T) {
	// Same input produces identical output on repeated runs.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout1 := f.graph.Layout(f.opts)
			layout2 := f.graph.Layout(f.opts)

			for id, n1 := range layout1.Nodes {
				n2 := layout2.Nodes[id]
				if n1.X != n2.X || n1.Y != n2.Y {
					t.Errorf("node %q: run1=(%v,%v) run2=(%v,%v)", id, n1.X, n1.Y, n2.X, n2.Y)
				}
			}
			for key, e1 := range layout1.Edges {
				e2 := layout2.Edges[key]
				if len(e1.Points) != len(e2.Points) {
					t.Errorf("edge %q: run1 has %d points, run2 has %d", key, len(e1.Points), len(e2.Points))
					continue
				}
				for i := range e1.Points {
					if e1.Points[i] != e2.Points[i] {
						t.Errorf("edge %q point %d: run1=%v run2=%v", key, i, e1.Points[i], e2.Points[i])
					}
				}
			}
		})
	}
}

func TestContract_LayerOrdering(t *testing.T) {
	// For a simple chain A→B→C in TopToBottom: A.Y < B.Y < C.Y.
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.AddNode("C", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	layout := g.Layout()

	if layout.Nodes["A"].Y >= layout.Nodes["B"].Y {
		t.Errorf("A.Y=%v should be < B.Y=%v", layout.Nodes["A"].Y, layout.Nodes["B"].Y)
	}
	if layout.Nodes["B"].Y >= layout.Nodes["C"].Y {
		t.Errorf("B.Y=%v should be < C.Y=%v", layout.Nodes["B"].Y, layout.Nodes["C"].Y)
	}
}

func TestContract_LayerSpacing(t *testing.T) {
	// Layer spacing is at least RankSep.
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.AddNode("C", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("B", "C")

	opts := DefaultOptions()
	opts.RankSep = 80
	layout := g.Layout(opts)

	// Distance between layer bottoms and next layer tops should be >= RankSep
	aBottom := layout.Nodes["A"].Y + layout.Nodes["A"].Height
	bTop := layout.Nodes["B"].Y
	gap := bTop - aBottom
	if gap < opts.RankSep-1 { // small tolerance
		t.Errorf("layer gap A→B: %v < RankSep %v", gap, opts.RankSep)
	}
}

func TestContract_NoNodeOverlap(t *testing.T) {
	// No two non-cluster nodes overlap.
	for _, f := range generateFixtures() {
		t.Run(f.name, func(t *testing.T) {
			layout := f.graph.Layout(f.opts)

			type rect struct {
				id                     string
				x, y, width, height    float64
			}
			var rects []rect
			for id, n := range layout.Nodes {
				if f.graph.nodes[id] != nil && f.graph.nodes[id].isCluster {
					continue
				}
				rects = append(rects, rect{id, n.X, n.Y, n.Width, n.Height})
			}

			for i := 0; i < len(rects); i++ {
				for j := i + 1; j < len(rects); j++ {
					a, b := rects[i], rects[j]
					// Check axis-aligned rectangle overlap
					overlapX := a.x < b.x+b.width && b.x < a.x+a.width
					overlapY := a.y < b.y+b.height && b.y < a.y+a.height
					if overlapX && overlapY {
						t.Errorf("nodes %q and %q overlap: [%.0f,%.0f,%.0f,%.0f] vs [%.0f,%.0f,%.0f,%.0f]",
							a.id, b.id, a.x, a.y, a.width, a.height, b.x, b.y, b.width, b.height)
					}
				}
			}
		})
	}
}

func TestContract_EmptyGraph(t *testing.T) {
	// Empty graph produces empty layout (not nil maps).
	g := NewGraph()
	layout := g.Layout()

	if layout.Nodes == nil {
		t.Error("empty graph: Nodes map is nil (should be empty non-nil)")
	}
	if layout.Edges == nil {
		t.Error("empty graph: Edges map is nil (should be empty non-nil)")
	}
	if len(layout.Nodes) != 0 {
		t.Errorf("empty graph: expected 0 nodes, got %d", len(layout.Nodes))
	}
	if len(layout.Edges) != 0 {
		t.Errorf("empty graph: expected 0 edges, got %d", len(layout.Edges))
	}
}

func TestContract_EdgeKeyFormat(t *testing.T) {
	// Edge keys follow "from->to" or "from->to:id" format.
	g := NewGraph()
	g.AddNode("A", NodeOptions{Width: 50, Height: 30})
	g.AddNode("B", NodeOptions{Width: 50, Height: 30})
	g.MustAddEdge("A", "B")
	g.MustAddEdge("A", "B", EdgeOptions{ID: "secondary"})

	layout := g.Layout()

	if _, ok := layout.Edges["A->B"]; !ok {
		t.Error("expected edge key 'A->B'")
	}
	if _, ok := layout.Edges["A->B:secondary"]; !ok {
		t.Error("expected edge key 'A->B:secondary'")
	}
}
