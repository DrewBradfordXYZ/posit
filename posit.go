// Package posit provides pure Go implementation of the Sugiyama algorithm
// for layered graph layout. It computes X/Y positions for nodes in directed
// graphs, arranging them in hierarchical layers with minimal edge crossings.
package posit

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

// EdgeLayout contains the routed path for an edge.
type EdgeLayout struct {
	Points []EdgePoint
}

// Layout contains the computed positions for all nodes and edges.
type Layout struct {
	Nodes map[string]NodeLayout
	Edges map[string]EdgeLayout
}

// Graph represents a directed graph to be laid out.
type Graph struct {
	nodes map[string]*node
	edges []*edge
}

type node struct {
	id     string
	width  float64
	height float64
}

type edge struct {
	from string
	to   string
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*node),
		edges: make([]*edge, 0),
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

// AddEdge adds a directed edge from source to target.
func (g *Graph) AddEdge(from, to string) {
	g.edges = append(g.edges, &edge{from: from, to: to})
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	return len(g.edges)
}

// Layout computes positions for all nodes and edges.
func (g *Graph) Layout(opts ...Options) *Layout {
	opt := DefaultOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Create internal layout state
	state := newLayoutState(g, opt)

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

	return state.buildLayout()
}
