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

	// Input order (from Graph.AddNode calls) for deterministic initial ordering.
	// Used by buildLayers() to preserve user-defined node sequence.
	insertOrder int

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

	// Edge label info
	labelWidth   float64
	labelHeight  float64
	labelPos     LabelPosition
	labelDummyID string  // ID of dummy node representing label (for position extraction)
	labelX       float64 // computed label X coordinate
	labelY       float64 // computed label Y coordinate

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
	dummyCounter  int       // for generating unique dummy IDs

	// Self-loops (edges where source == target)
	selfLoops []*layoutEdge
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
			id:          id,
			width:       n.width,
			height:      n.height,
			insertOrder: n.insertOrder,
		}
		// Initialize empty adjacency lists
		s.successors[id] = []string{}
		s.predecessors[id] = []string{}
	}

	// Copy edges with aggregation and build adjacency lists
	edgeSeen := make(map[edgeKey]bool)
	for _, e := range g.edges {
		key := edgeKey{from: e.from, to: e.to}

		if existing := s.edges[key]; existing != nil {
			// Aggregate: sum weights for duplicate edges
			existing.weight += 1
			// If this edge has label info and existing doesn't, copy it
			if existing.labelWidth == 0 && e.labelWidth > 0 {
				existing.labelWidth = e.labelWidth
				existing.labelHeight = e.labelHeight
				existing.labelPos = e.labelPos
			}
			continue
		}

		s.edges[key] = &layoutEdge{
			key:         key,
			weight:      1,
			minlen:      1,
			labelWidth:  e.labelWidth,
			labelHeight: e.labelHeight,
			labelPos:    e.labelPos,
		}

		// Only add to adjacency lists once
		if !edgeSeen[key] {
			s.successors[e.from] = append(s.successors[e.from], e.to)
			s.predecessors[e.to] = append(s.predecessors[e.to], e.from)
			edgeSeen[key] = true
		}
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

	// Export edges with structured keys
	// Note: The string key format "from->to" is used for backward compatibility.
	// For unambiguous edge lookup, use Layout.Edge(from, to) method.
	for key, e := range s.edges {
		edgeID := key.from + "->" + key.to
		edgeLayout := EdgeLayout{
			From:   key.from,
			To:     key.to,
			Points: e.points,
		}
		// Include label position if edge has a label
		if e.labelWidth > 0 || e.labelHeight > 0 {
			edgeLayout.Label = &LabelLayout{
				X:      e.labelX,
				Y:      e.labelY,
				Width:  e.labelWidth,
				Height: e.labelHeight,
			}
		}
		layout.Edges[edgeID] = edgeLayout
	}

	return layout
}
