// Package posit provides a pure Go implementation of the Sugiyama algorithm
// for layered graph layout. It computes X/Y positions for nodes in directed
// graphs, arranging them in hierarchical layers with minimal edge crossings.
//
// # Features
//
//   - Zero external dependencies (standard library only)
//   - Deterministic output (same input always produces same output)
//   - Support for edge labels with automatic positioning
//   - Multiple ranking algorithms (LongestPath, TightTree, NetworkSimplex)
//   - Multiple cycle removal algorithms (DFS, Greedy FAS)
//   - Four layout directions (TopToBottom, LeftToRight, BottomToTop, RightToLeft)
//   - Self-loop support with curved path rendering
//
// # Basic Usage
//
//	g := posit.NewGraph()
//	g.AddNode("A", posit.NodeOptions{Width: 100, Height: 50})
//	g.AddNode("B", posit.NodeOptions{Width: 100, Height: 50})
//	g.MustAddEdge("A", "B")
//
//	layout := g.Layout()
//	fmt.Printf("Node A: %+v\n", layout.Nodes["A"])
//	fmt.Printf("Edge A->B: %+v\n", layout.Edges["A->B"])
//
// # Coordinate System
//
// Coordinates use a top-left origin with Y increasing downward (standard
// screen coordinates). Node positions (X, Y) represent the top-left corner
// of the node. To get the center, add Width/2 and Height/2.
//
// # Algorithm Selection
//
// The Algorithm option controls layer assignment:
//
//   - LongestPath (default): Fastest, O(V+E). Best for interactive use or
//     when layout speed matters more than compactness. May produce more
//     layers than necessary.
//
//   - TightTree: Middle ground, O(V*E). Produces tighter layouts than
//     LongestPath without the full optimization cost of NetworkSimplex.
//     Good default for most graphs under 500 nodes.
//
//   - NetworkSimplex: Optimal edge length minimization, O(V*E) typical.
//     Produces the most compact layouts but slower for large graphs.
//     Best when layout quality is paramount.
//
// The Acyclicer option controls cycle removal:
//
//   - DFSAcyclicer (default): Simple DFS-based back edge detection.
//     Works well for most graphs.
//
//   - GreedyAcyclicer: Eades/Lin/Smyth heuristic. Better results for
//     graphs with weighted edges where minimizing reversed edge weight
//     matters.
//
// # Thread Safety
//
// A Graph instance is NOT safe for concurrent modification. Do not call
// AddNode or AddEdge while Layout is executing or from multiple goroutines.
//
// However, calling Layout() on the same Graph from multiple goroutines is
// safe, as each call creates independent internal state. The returned
// Layout objects are also safe for concurrent read access.
//
// For concurrent graph building, use external synchronization or build
// separate Graph instances per goroutine.
//
// # Performance Tuning
//
// For graphs over 100 nodes, the coordinate assignment automatically
// switches from Brandes-Köpf to a simpler algorithm for speed.
//
// Layout options affect both appearance and performance:
//
//   - NodeSep: Horizontal spacing between nodes. Larger values produce
//     wider layouts but don't significantly impact performance.
//
//   - RankSep: Vertical spacing between layers. Larger values produce
//     taller layouts. No performance impact.
//
// For very large graphs (500+ nodes), consider:
//   - Using LongestPath algorithm (fastest)
//   - Simplifying the graph by collapsing clusters
//   - Running layout in a background goroutine
//
// # Algorithm Complexity
//
//   - LongestPath ranking: O(V + E)
//   - TightTree ranking: O(V * E)
//   - NetworkSimplex ranking: O(V * E) typical, O(V^2 * E) worst case
//   - Crossing minimization: O(iterations * V * E)
//   - Coordinate assignment (Brandes-Köpf): O(V + E)
//
// Target performance: < 10ms for 50 nodes, < 100ms for 200 nodes.
//
// # Limitations
//
//   - Compound graphs (nested subgraphs/clusters) are not supported
//   - Type-2 conflict detection in coordinate assignment is not implemented
//   - Maximum recommended graph size is ~1000 nodes for interactive use
package posit

import (
	"fmt"
	"sort"
)

// Direction specifies the primary direction of the layout.
type Direction int

const (
	TopToBottom Direction = iota
	LeftToRight
	BottomToTop
	RightToLeft
)

// RankAlgorithm specifies which algorithm to use for layer assignment.
type RankAlgorithm int

const (
	// LongestPath is simple and fast, but may produce more layers.
	LongestPath RankAlgorithm = iota
	// NetworkSimplex produces optimal results but is more complex.
	NetworkSimplex
	// TightTree uses longest-path followed by tight tree construction
	// (faster than NetworkSimplex but better than LongestPath).
	TightTree
)

// Acyclicer specifies which algorithm to use for cycle removal.
type Acyclicer int

const (
	// DFSAcyclicer uses DFS-based back edge detection (default).
	DFSAcyclicer Acyclicer = iota
	// GreedyAcyclicer uses the Eades/Lin/Smyth greedy heuristic for
	// weighted feedback arc sets. Produces better results for graphs
	// with edge weights.
	GreedyAcyclicer
)

// Options configures the layout algorithm.
type Options struct {
	// Direction of the layout (default: TopToBottom)
	Direction Direction

	// NodeSep is the minimum horizontal spacing between nodes (default: 50)
	NodeSep float64

	// RankSep is the minimum vertical spacing between layers (default: 100)
	RankSep float64

	// Algorithm for layer assignment (default: LongestPath)
	Algorithm RankAlgorithm

	// Acyclicer for cycle removal (default: DFSAcyclicer)
	Acyclicer Acyclicer
}

// DefaultOptions returns sensible defaults for layout.
func DefaultOptions() Options {
	return Options{
		Direction: TopToBottom,
		NodeSep:   50,
		RankSep:   100,
		Algorithm: LongestPath,
	}
}

// NodeOptions specifies dimensions for a node.
type NodeOptions struct {
	Width  float64
	Height float64
}

// LabelPosition specifies where an edge label is positioned along the edge.
type LabelPosition string

const (
	// LabelCenter positions the label at the center of the edge (default)
	LabelCenter LabelPosition = "c"
	// LabelLeft positions the label toward the source node
	LabelLeft LabelPosition = "l"
	// LabelRight positions the label toward the target node
	LabelRight LabelPosition = "r"
)

// EdgeOptions specifies options for an edge including optional label dimensions.
type EdgeOptions struct {
	// LabelWidth is the width of the edge label (0 for no label)
	LabelWidth float64
	// LabelHeight is the height of the edge label (0 for no label)
	LabelHeight float64
	// LabelPosition specifies where to position the label along the edge
	LabelPosition LabelPosition
}

// Position represents computed X/Y coordinates.
type Position struct {
	X float64
	Y float64
}

// NodeLayout contains position and dimensions for a laid-out node.
type NodeLayout struct {
	Position
	Width  float64
	Height float64
}

// EdgePoint represents a point along an edge path.
type EdgePoint struct {
	X float64
	Y float64
}

// LabelLayout contains the computed position for an edge label.
type LabelLayout struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// EdgeLayout contains the routed path for an edge.
type EdgeLayout struct {
	// From is the source node ID
	From string
	// To is the target node ID
	To string
	// Points is the path from source to target
	Points []EdgePoint
	// Label contains the computed label position, if the edge has a label
	Label *LabelLayout
}

// Layout contains the computed positions for all nodes and edges.
type Layout struct {
	Nodes map[string]NodeLayout
	Edges map[string]EdgeLayout
}

// Edge returns the layout for an edge by its source and target node IDs.
// This method provides unambiguous edge lookup regardless of node ID contents.
// Returns the EdgeLayout and true if found, or zero value and false if not.
func (l *Layout) Edge(from, to string) (EdgeLayout, bool) {
	for _, e := range l.Edges {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return EdgeLayout{}, false
}

// Graph represents a directed graph to be laid out.
type Graph struct {
	nodes   map[string]*node
	edges   []*edge
	edgeSet map[[2]string]bool // for O(1) edge lookup
}

type node struct {
	id     string
	width  float64
	height float64
}

type edge struct {
	from        string
	to          string
	labelWidth  float64
	labelHeight float64
	labelPos    LabelPosition
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes:   make(map[string]*node),
		edges:   make([]*edge, 0),
		edgeSet: make(map[[2]string]bool),
	}
}

// AddNode adds a node with the given ID and dimensions.
// If a node with the same ID exists, it is replaced.
func (g *Graph) AddNode(id string, opts NodeOptions) {
	g.nodes[id] = &node{
		id:     id,
		width:  opts.Width,
		height: opts.Height,
	}
}

// HasNode returns true if a node with the given ID exists.
func (g *Graph) HasNode(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

// AddEdge adds a directed edge from source to target.
// Returns an error if either node does not exist.
//
// If an edge from source to target already exists, the edges are
// aggregated: their weights are summed for crossing minimization
// and ranking calculations. The first edge's label options are preserved.
//
// Optional EdgeOptions can be provided to specify label dimensions.
func (g *Graph) AddEdge(from, to string, opts ...EdgeOptions) error {
	if _, ok := g.nodes[from]; !ok {
		return fmt.Errorf("posit: source node %q does not exist", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return fmt.Errorf("posit: target node %q does not exist", to)
	}
	e := &edge{from: from, to: to}
	if len(opts) > 0 {
		e.labelWidth = opts[0].LabelWidth
		e.labelHeight = opts[0].LabelHeight
		e.labelPos = opts[0].LabelPosition
	}
	g.edges = append(g.edges, e)
	g.edgeSet[[2]string{from, to}] = true
	return nil
}

// MustAddEdge adds an edge or panics if nodes don't exist.
// Optional EdgeOptions can be provided to specify label dimensions.
func (g *Graph) MustAddEdge(from, to string, opts ...EdgeOptions) {
	if err := g.AddEdge(from, to, opts...); err != nil {
		panic(err)
	}
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	return len(g.edges)
}

// Nodes returns a slice of all node IDs in sorted order.
func (g *Graph) Nodes() []string {
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Edges returns a slice of all edges as [from, to] pairs.
func (g *Graph) Edges() [][2]string {
	result := make([][2]string, len(g.edges))
	for i, e := range g.edges {
		result[i] = [2]string{e.from, e.to}
	}
	return result
}

// HasEdge returns true if an edge from source to target exists.
// This is an O(1) operation.
func (g *Graph) HasEdge(from, to string) bool {
	return g.edgeSet[[2]string{from, to}]
}

// Node returns the dimensions of a node, or false if not found.
func (g *Graph) Node(id string) (NodeOptions, bool) {
	n, ok := g.nodes[id]
	if !ok {
		return NodeOptions{}, false
	}
	return NodeOptions{Width: n.width, Height: n.height}, true
}

// Layout computes positions for all nodes and edges.
// For error handling, use LayoutWithError instead.
func (g *Graph) Layout(opts ...Options) *Layout {
	layout, _ := g.LayoutWithError(opts...)
	return layout
}

// LayoutWithError computes positions for all nodes and edges,
// returning an error if options are invalid.
func (g *Graph) LayoutWithError(opts ...Options) (*Layout, error) {
	opt := DefaultOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Validate options
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	// Handle empty graph
	if len(g.nodes) == 0 {
		return &Layout{
			Nodes: make(map[string]NodeLayout),
			Edges: make(map[string]EdgeLayout),
		}, nil
	}

	// Create internal layout state
	state := newLayoutState(g, opt)

	// Pre-transform for direction (swap dimensions for LR/RL)
	state.adjustForDirection()

	// Phase 1: Make acyclic (reverse edges that create cycles)
	state.makeAcyclic()

	// Phase 2: Assign nodes to layers
	state.assignLayers()

	// Phase 3: Add dummy nodes for edges spanning multiple layers
	state.addDummyNodes()

	// Phase 4: Order nodes within layers to minimize crossings
	state.minimizeCrossings()

	// Phase 5: Assign X/Y coordinates
	state.assignCoordinates()

	// Phase 6: Route edges and restore reversed edges
	state.routeEdges()

	// Post-transform for direction (convert coordinates)
	state.undoDirectionAdjustment()

	return state.buildLayout(), nil
}

// Validate checks that the options are valid.
func (o Options) Validate() error {
	if o.NodeSep < 0 {
		return fmt.Errorf("posit: NodeSep cannot be negative (got %v)", o.NodeSep)
	}
	if o.RankSep < 0 {
		return fmt.Errorf("posit: RankSep cannot be negative (got %v)", o.RankSep)
	}
	if o.Direction < TopToBottom || o.Direction > RightToLeft {
		return fmt.Errorf("posit: invalid Direction value %d", o.Direction)
	}
	if o.Algorithm < LongestPath || o.Algorithm > TightTree {
		return fmt.Errorf("posit: invalid Algorithm value %d", o.Algorithm)
	}
	if o.Acyclicer < DFSAcyclicer || o.Acyclicer > GreedyAcyclicer {
		return fmt.Errorf("posit: invalid Acyclicer value %d", o.Acyclicer)
	}
	return nil
}
