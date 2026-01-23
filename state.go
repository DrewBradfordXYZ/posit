package posit

import "math/rand"

// edgeKey uniquely identifies an edge by its endpoints and optional ID.
type edgeKey struct {
	from string
	to   string
	id   string // for multi-edge support
}

// edgeKeyLess provides deterministic ordering for edgeKeys.
func edgeKeyLess(a, b edgeKey) bool {
	if a.from != b.from {
		return a.from < b.from
	}
	if a.to != b.to {
		return a.to < b.to
	}
	return a.id < b.id
}

// layoutNode holds internal state for a node during layout.
type layoutNode struct {
	id     string
	width  float64
	height float64

	// Input order (from Graph.AddNode calls) for deterministic initial ordering.
	// Used by buildLayers() to preserve user-defined node sequence.
	insertOrder int

	// Rank constraints
	rankConstraint RankConstraint
	rankGroup      string

	// Order constraints
	orderGroup    string
	orderPriority int

	// Ports
	ports []PortOptions

	// Phase 2 output
	rank int

	// Phase 4 output
	order int

	// Phase 5 output
	x, y float64

	// Dummy node tracking
	isDummy         bool
	isInteriorDummy bool        // interior of long-edge chain (both neighbors are dummies)
	edgeLabel       *layoutEdge // original edge for dummies
}

// layoutEdge holds internal state for an edge during layout.
type layoutEdge struct {
	key      edgeKey
	weight   float64
	minlen   int
	reversed bool

	// Multi-edge ID
	id string

	// Port connections
	sourcePort string
	targetPort string

	// Edge label info
	labelWidth   float64
	labelHeight  float64
	labelPos     LabelPosition
	labelDummyID string  // ID of dummy node representing label (for position extraction)
	labelX       float64 // computed label X coordinate
	labelY       float64 // computed label Y coordinate

	// Phase 6 output
	points     []EdgePoint
	sourceSide Side
	targetSide Side
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

	// Compound graph (clusters)
	parents  map[string]string  // child -> parent cluster ID
	clusters map[string]float64 // cluster ID -> padding

	// Deterministic RNG for stochastic disturbance in adjacent exchange
	rng *rand.Rand

	// Reusable buffer for accumulator tree in twoLayerCrossCount.
	// Avoids repeated allocation in the hot crossing-count loop.
	treeBuf []float64
}

// newLayoutState initializes internal state from a Graph.
func newLayoutState(g *Graph, opts Options) *layoutState {
	s := &layoutState{
		opts:         opts,
		nodes:        make(map[string]*layoutNode, len(g.nodes)),
		edges:        make(map[edgeKey]*layoutEdge, len(g.edges)),
		successors:   make(map[string][]string, len(g.nodes)),
		predecessors: make(map[string][]string, len(g.nodes)),
		parents:      g.parents,
		clusters:     g.clusters,
		rng:          rand.New(rand.NewSource(42)),
	}

	// Copy nodes
	for id, n := range g.nodes {
		s.nodes[id] = &layoutNode{
			id:             id,
			width:          n.width,
			height:         n.height,
			insertOrder:    n.insertOrder,
			rankConstraint: n.rankConstraint,
			rankGroup:      n.rankGroup,
			orderGroup:     n.orderGroup,
			orderPriority:  n.orderPriority,
			ports:          n.ports,
		}
		// Initialize empty adjacency lists
		s.successors[id] = []string{}
		s.predecessors[id] = []string{}
	}

	// Copy edges with aggregation and build adjacency lists.
	// Edges with an ID are kept separate (multi-edge support).
	// Edges without an ID are aggregated as before.
	adjacencySeen := make(map[[2]string]bool)
	for _, e := range g.edges {
		key := edgeKey{from: e.from, to: e.to, id: e.id}

		// For edges without an ID, aggregate duplicates
		if e.id == "" {
			if existing := s.edges[key]; existing != nil {
				// Aggregate: sum weights for duplicate edges
				w := e.weight
				if w <= 0 {
					w = 1
				}
				existing.weight += w
				// If this edge has label info and existing doesn't, copy it
				if existing.labelWidth == 0 && e.labelWidth > 0 {
					existing.labelWidth = e.labelWidth
					existing.labelHeight = e.labelHeight
					existing.labelPos = e.labelPos
				}
				continue
			}
		}

		w := e.weight
		if w <= 0 {
			w = 1
		}

		s.edges[key] = &layoutEdge{
			key:         key,
			weight:      w,
			minlen:      1,
			id:          e.id,
			sourcePort:  e.sourcePort,
			targetPort:  e.targetPort,
			labelWidth:  e.labelWidth,
			labelHeight: e.labelHeight,
			labelPos:    e.labelPos,
		}

		// Only add to adjacency lists once per (from, to) pair
		adjKey := [2]string{e.from, e.to}
		if !adjacencySeen[adjKey] {
			s.successors[e.from] = append(s.successors[e.from], e.to)
			s.predecessors[e.to] = append(s.predecessors[e.to], e.from)
			adjacencySeen[adjKey] = true
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
		nodeLayout := NodeLayout{
			Position: Position{X: n.x, Y: n.y},
			Width:    n.width,
			Height:   n.height,
		}

		// Export computed port positions for auto-positioned ports
		for _, port := range n.ports {
			if port.Constraint == PortFixedSide || port.Constraint == PortFixedOrder || port.Constraint == PortFree {
				if nodeLayout.Ports == nil {
					nodeLayout.Ports = make(map[string]PortLayout)
				}
				nodeLayout.Ports[port.ID] = PortLayout{
					ID:     port.ID,
					Side:   port.Side,
					Offset: port.Offset,
				}
			}
		}

		layout.Nodes[id] = nodeLayout
	}

	// Export edges with structured keys
	// Format: "from->to" for simple edges, "from->to:id" for multi-edges.
	// For unambiguous edge lookup, use Layout.Edge(from, to) method.
	for key, e := range s.edges {
		edgeID := key.from + "->" + key.to
		if key.id != "" {
			edgeID += ":" + key.id
		}
		edgeLayout := EdgeLayout{
			From:       key.from,
			To:         key.to,
			Points:     e.points,
			SourcePort: e.sourcePort,
			TargetPort: e.targetPort,
			SourceSide: e.sourceSide,
			TargetSide: e.targetSide,
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
