package posit

// edgeKey uniquely identifies an edge by its endpoints.
type edgeKey struct {
	from string
	to   string
}

// layoutNode holds internal state for a node during layout.
type layoutNode struct {
	id     string
	width  float64
	height float64

	// Phase 2 output
	rank int

	// Phase 4 output
	order int

	// Phase 5 output
	x, y float64

	// Dummy node tracking
	isDummy   bool
	edgeLabel *layoutEdge // original edge for dummies
}

// layoutEdge holds internal state for an edge during layout.
type layoutEdge struct {
	key      edgeKey
	weight   float64
	minlen   int
	reversed bool

	// Phase 6 output
	points []EdgePoint
}

// layoutState holds all internal state during the layout process.
type layoutState struct {
	// Configuration
	opts Options

	// Node data (includes dummies during processing)
	nodes map[string]*layoutNode

	// Edge data
	edges map[edgeKey]*layoutEdge

	// Adjacency lists for fast traversal
	successors   map[string][]string
	predecessors map[string][]string

	// Layer structure (built in Phase 2, refined in Phase 3-4)
	layers [][]string // layers[rank] = ordered node IDs

	// Tracking for cleanup
	reversedEdges []edgeKey // edges to flip back
	dummyChains   []string  // first dummy in each chain
}

// newLayoutState initializes internal state from a Graph.
func newLayoutState(g *Graph, opts Options) *layoutState {
	s := &layoutState{
		opts:         opts,
		nodes:        make(map[string]*layoutNode, len(g.nodes)),
		edges:        make(map[edgeKey]*layoutEdge, len(g.edges)),
		successors:   make(map[string][]string, len(g.nodes)),
		predecessors: make(map[string][]string, len(g.nodes)),
	}

	// Copy nodes
	for id, n := range g.nodes {
		s.nodes[id] = &layoutNode{
			id:     id,
			width:  n.width,
			height: n.height,
		}
		// Initialize empty adjacency lists
		s.successors[id] = []string{}
		s.predecessors[id] = []string{}
	}

	// Copy edges and build adjacency lists
	for _, e := range g.edges {
		key := edgeKey{from: e.from, to: e.to}
		s.edges[key] = &layoutEdge{
			key:    key,
			weight: 1,
			minlen: 1,
		}
		s.successors[e.from] = append(s.successors[e.from], e.to)
		s.predecessors[e.to] = append(s.predecessors[e.to], e.from)
	}

	return s
}

// buildLayout converts internal state to the final Layout output.
func (s *layoutState) buildLayout() *Layout {
	layout := &Layout{
		Nodes: make(map[string]NodeLayout, len(s.nodes)),
		Edges: make(map[string]EdgeLayout, len(s.edges)),
	}

	// Export non-dummy nodes
	for id, n := range s.nodes {
		if n.isDummy {
			continue
		}
		layout.Nodes[id] = NodeLayout{
			Position: Position{X: n.x, Y: n.y},
			Width:    n.width,
			Height:   n.height,
		}
	}

	// Export edges
	for key, e := range s.edges {
		edgeID := key.from + "->" + key.to
		layout.Edges[edgeID] = EdgeLayout{
			Points: e.points,
		}
	}

	return layout
}
