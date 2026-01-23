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
	"math"
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

// RouteStyle specifies the edge routing algorithm.
type RouteStyle int

const (
	// RoutePolyline uses the current behavior: polyline paths through dummy nodes (default).
	RoutePolyline RouteStyle = iota
	// RouteOrthogonal uses channel-routed horizontal/vertical segments.
	RouteOrthogonal
)

// ComponentPacking specifies how disconnected components are arranged.
type ComponentPacking int

const (
	// PackHorizontal arranges components side by side (default).
	PackHorizontal ComponentPacking = iota
	// PackVertical stacks components vertically.
	PackVertical
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

	// BKThreshold is the node count above which coordinate assignment
	// switches from Brandes-Köpf (optimal but slower) to simple centering.
	// Default: 100. Set higher if you want better alignment for larger graphs.
	BKThreshold int

	// AdjacentExchangeLimit is the maximum layer size for which adjacent
	// exchange optimization is performed during crossing minimization.
	// Adjacent exchange swaps neighboring nodes to escape local minima.
	// Default: 0 (no limit — uses efficient O(deg²) incremental algorithm).
	// Set to a positive value to skip adjacent exchange for layers larger than this.
	AdjacentExchangeLimit int

	// TryReverseOrdering runs crossing minimization in both layer directions
	// and keeps the better result. For asymmetric graphs, one direction may
	// naturally produce fewer crossings. Roughly doubles ordering time.
	// Default: false.
	TryReverseOrdering bool

	// RouteStyle selects the edge routing algorithm (default: RoutePolyline).
	RouteStyle RouteStyle

	// ChannelGap is the spacing between parallel edges in orthogonal routing channels.
	// Only used when RouteStyle is RouteOrthogonal. Default: 10.
	ChannelGap float64

	// ComponentPacking controls how disconnected components are arranged (default: PackHorizontal).
	ComponentPacking ComponentPacking

	// ComponentGap is the spacing between disconnected components.
	// Default: NodeSep * 2.
	ComponentGap float64
}

// DefaultOptions returns sensible defaults for layout.
func DefaultOptions() Options {
	return Options{
		Direction:   TopToBottom,
		NodeSep:     50,
		RankSep:     100,
		Algorithm:   LongestPath,
		BKThreshold: 100,
	}
}

// RankConstraint specifies how a node's layer is constrained.
type RankConstraint int

const (
	// RankUnconstrained allows normal layer assignment (default).
	RankUnconstrained RankConstraint = iota
	// RankMin forces the node to the first (top) layer.
	RankMin
	// RankMax forces the node to the last (bottom) layer.
	RankMax
)

// Side specifies which side of a node an edge connects to.
type Side int

const (
	// Top is the top side of a node.
	Top Side = iota
	// Bottom is the bottom side of a node.
	Bottom
	// Left is the left side of a node.
	Left
	// Right is the right side of a node.
	Right
)

// PortConstraint specifies how a port's position is determined.
type PortConstraint int

const (
	// PortFixedPos uses the exact Offset specified (current behavior, default).
	PortFixedPos PortConstraint = iota
	// PortFixedSide keeps the port on its declared Side but computes the optimal
	// offset and order to minimize edge crossings.
	PortFixedSide
	// PortFixedOrder keeps the port on its declared Side in declared relative order
	// but computes evenly-distributed offsets.
	PortFixedOrder
	// PortFree allows the algorithm to choose both the side and offset.
	// The side is selected based on the position of the connected node.
	// Use the Axis field to restrict which sides are considered.
	PortFree
)

// PortAxis constrains which sides PortFree considers.
type PortAxis int

const (
	// PortAxisAny allows any side (default).
	PortAxisAny PortAxis = iota
	// PortAxisHorizontal restricts to Left or Right.
	PortAxisHorizontal
	// PortAxisVertical restricts to Top or Bottom.
	PortAxisVertical
)

// PortOptions specifies a connection point on a node.
type PortOptions struct {
	// ID uniquely identifies this port within the node (e.g., "in-1", "out-2").
	ID string
	// Side specifies which side of the node this port is on.
	Side Side
	// Offset is the distance from the node origin along the side.
	// For Top/Bottom sides: offset from left edge.
	// For Left/Right sides: offset from top edge.
	// Used only for PortFixedPos (default constraint).
	Offset float64
	// Order specifies relative ordering for PortFixedOrder constraint.
	// Lower values are placed first (top for Left/Right sides, left for Top/Bottom sides).
	Order int
	// Constraint controls how the port position is determined (default: PortFixedPos).
	Constraint PortConstraint
	// Axis restricts which sides PortFree considers (default: PortAxisAny).
	// Only used when Constraint is PortFree.
	Axis PortAxis
	// Width is the optional width of the port attachment area (default: 0, point).
	// When non-zero, edge endpoints are clipped to the port rectangle boundary.
	Width float64
	// Height is the optional height of the port attachment area (default: 0, point).
	Height float64
}

// NodeOptions specifies dimensions and constraints for a node.
type NodeOptions struct {
	Width  float64
	Height float64

	// RankConstraint pins the node to the min or max layer.
	RankConstraint RankConstraint

	// RankGroup groups nodes to share the same layer.
	// Nodes with the same non-empty RankGroup are placed on the same layer.
	RankGroup string

	// OrderGroup groups nodes to be placed adjacently within their layer.
	// Nodes with the same non-empty OrderGroup cluster together.
	OrderGroup string

	// OrderPriority controls ordering within an OrderGroup.
	// Lower priority = further left (or top for vertical layouts).
	OrderPriority int

	// Ports specifies fixed connection points on this node.
	Ports []PortOptions

	// IsCluster marks this node as a compound graph cluster container.
	// Cluster nodes contain child nodes set via Graph.SetParent().
	IsCluster bool

	// Padding is the internal padding for cluster nodes (default: 20).
	// Only used when IsCluster is true.
	Padding float64
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
	// ID distinguishes multiple edges between the same node pair.
	// When specified, the edge key becomes "from->to:id" instead of "from->to".
	ID string

	// Weight influences layout priority (default: 1.0, higher = more important).
	// Heavier edges are less likely to be reversed during cycle removal and
	// crossing minimization favors keeping them uncrossed.
	Weight float64

	// SourcePort connects this edge to a specific port on the source node.
	SourcePort string
	// TargetPort connects this edge to a specific port on the target node.
	TargetPort string

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

// PortLayout contains the computed position for a port after layout.
type PortLayout struct {
	ID     string
	Side   Side
	Offset float64 // Computed offset along the side
}

// NodeLayout contains position and dimensions for a laid-out node.
type NodeLayout struct {
	Position
	Width  float64
	Height float64
	// Ports contains computed port positions for nodes with PortFixedSide or
	// PortFixedOrder constraints. Nil for nodes with only PortFixedPos ports.
	Ports map[string]PortLayout
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

	// SourcePort is the port ID the edge exits from (if specified)
	SourcePort string
	// TargetPort is the port ID the edge enters (if specified)
	TargetPort string
	// SourceSide is the computed attachment side on the source node
	SourceSide Side
	// TargetSide is the computed attachment side on the target node
	TargetSide Side
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
	nodes     map[string]*node
	edges     []*edge
	edgeSet   map[[2]string]bool // for O(1) edge lookup
	nextOrder int               // tracks insertion order for deterministic layout
	parents   map[string]string  // child -> parent for compound graphs
	clusters  map[string]float64 // cluster ID -> padding
}

type node struct {
	id             string
	width          float64
	height         float64
	insertOrder    int // preserves AddNode() call order for initial layer ordering
	rankConstraint RankConstraint
	rankGroup      string
	orderGroup     string
	orderPriority  int
	ports          []PortOptions
	isCluster      bool
	padding        float64
}

type edge struct {
	from        string
	to          string
	id          string // for multi-edge support
	weight      float64
	sourcePort  string
	targetPort  string
	labelWidth  float64
	labelHeight float64
	labelPos    LabelPosition
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes:    make(map[string]*node),
		edges:    make([]*edge, 0),
		edgeSet:  make(map[[2]string]bool),
		parents:  make(map[string]string),
		clusters: make(map[string]float64),
	}
}

// SetParent sets a node's parent cluster.
// The parent must be a node added with IsCluster: true in NodeOptions.
func (g *Graph) SetParent(child, parent string) {
	g.parents[child] = parent
}

// Parent returns the parent cluster of a node, or empty string if none.
func (g *Graph) Parent(child string) string {
	return g.parents[child]
}

// Children returns all direct children of a cluster node.
func (g *Graph) Children(parent string) []string {
	var children []string
	for child, p := range g.parents {
		if p == parent {
			children = append(children, child)
		}
	}
	sort.Strings(children)
	return children
}

// AddNode adds a node with the given ID and dimensions.
// If a node with the same ID exists, it is replaced (preserving its insertion order).
func (g *Graph) AddNode(id string, opts NodeOptions) {
	order := g.nextOrder
	if existing, ok := g.nodes[id]; ok {
		order = existing.insertOrder // preserve original order on replace
	} else {
		g.nextOrder++
	}
	padding := opts.Padding
	if opts.IsCluster && padding == 0 {
		padding = 20 // default cluster padding
	}
	g.nodes[id] = &node{
		id:             id,
		width:          opts.Width,
		height:         opts.Height,
		insertOrder:    order,
		rankConstraint: opts.RankConstraint,
		rankGroup:      opts.RankGroup,
		orderGroup:     opts.OrderGroup,
		orderPriority:  opts.OrderPriority,
		ports:          opts.Ports,
		isCluster:      opts.IsCluster,
		padding:        padding,
	}
	if opts.IsCluster {
		g.clusters[id] = padding
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
// If an edge from source to target already exists (and no ID is specified),
// the edges are aggregated: their weights are summed for crossing minimization
// and ranking calculations. The first edge's label options are preserved.
//
// When EdgeOptions.ID is specified, multiple distinct edges between the same
// node pair are preserved as separate paths.
//
// Optional EdgeOptions can be provided to specify label dimensions, weight, and ports.
func (g *Graph) AddEdge(from, to string, opts ...EdgeOptions) error {
	if _, ok := g.nodes[from]; !ok {
		return fmt.Errorf("posit: source node %q does not exist", from)
	}
	if _, ok := g.nodes[to]; !ok {
		return fmt.Errorf("posit: target node %q does not exist", to)
	}
	e := &edge{from: from, to: to}
	if len(opts) > 0 {
		e.id = opts[0].ID
		e.weight = opts[0].Weight
		e.sourcePort = opts[0].SourcePort
		e.targetPort = opts[0].TargetPort
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

// Node returns the options of a node, or false if not found.
func (g *Graph) Node(id string) (NodeOptions, bool) {
	n, ok := g.nodes[id]
	if !ok {
		return NodeOptions{}, false
	}
	return NodeOptions{
		Width:          n.width,
		Height:         n.height,
		RankConstraint: n.rankConstraint,
		RankGroup:      n.rankGroup,
		OrderGroup:     n.orderGroup,
		OrderPriority:  n.orderPriority,
		Ports:          n.ports,
	}, true
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
	state.markInteriorDummies()

	// Phase 4: Order nodes within layers to minimize crossings
	state.minimizeCrossings()

	// Phase 5: Assign X/Y coordinates
	state.assignCoordinates()

	// Phase 5b: Compute auto port offsets (PortFixedSide, PortFixedOrder)
	state.computePortOffsets()

	// Phase 5c: Pack disconnected components
	state.packComponents()

	// Phase 6: Route edges and restore reversed edges
	state.routeEdges()

	// Post-transform for direction (convert coordinates)
	state.undoDirectionAdjustment()

	// Compound graph: size cluster nodes to contain their children
	layout := state.buildLayout()
	g.adjustClusters(layout)

	return layout, nil
}

// IncrementalOptions configures an incremental layout update.
type IncrementalOptions struct {
	// Fixed lists node IDs that should not move.
	Fixed map[string]bool
	// Changes maps node IDs to their new dimensions.
	Changes map[string]NodeOptions
}

// IncrementalLayout produces a minimal layout adjustment from an existing layout.
// It preserves layer assignments and X positions for unchanged nodes, only
// re-running Y coordinate assignment for affected layers and re-routing edges
// connected to changed nodes.
func (g *Graph) IncrementalLayout(base *Layout, changes IncrementalOptions, opts ...Options) *Layout {
	// Apply dimension changes to the graph
	for id, newOpts := range changes.Changes {
		if _, ok := g.nodes[id]; ok {
			g.nodes[id].width = newOpts.Width
			g.nodes[id].height = newOpts.Height
		}
	}

	opt := DefaultOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	state := newLayoutState(g, opt)
	state.adjustForDirection()
	state.makeAcyclic()
	state.assignLayers()
	state.addDummyNodes()
	state.markInteriorDummies()

	// When we have a base layout and only dimensions changed, restore ordering
	// from base positions instead of re-running crossing minimization.
	if base != nil {
		state.restoreOrderFromBase(base)
	} else {
		state.minimizeCrossings()
	}

	// Assign coordinates: Y from scratch (layer heights may have changed),
	// but preserve X positions for fixed/unchanged nodes
	state.assignYCoordinates()

	// For X: use base positions for fixed nodes, recompute only for changed nodes
	if base != nil {
		// Start with full X assignment
		threshold := opt.BKThreshold
		if threshold <= 0 {
			threshold = 100
		}
		if len(state.nodes) > threshold {
			state.assignXCoordinatesSimple()
		} else {
			state.assignXCoordinatesBK()
		}

		// Pin fixed node X positions from base layout
		if changes.Fixed != nil {
			for id := range changes.Fixed {
				if baseNode, ok := base.Nodes[id]; ok {
					if layoutNode, ok := state.nodes[id]; ok {
						layoutNode.x = baseNode.X
					}
				}
			}
		}

		// Also pin unchanged nodes (not in changes.Changes and not fixed)
		for id := range state.nodes {
			if state.nodes[id].isDummy {
				continue
			}
			if _, isChanged := changes.Changes[id]; isChanged {
				continue
			}
			if changes.Fixed != nil {
				if _, isFixed := changes.Fixed[id]; isFixed {
					continue // already pinned above
				}
			}
			// Unchanged node: preserve base X if available
			if baseNode, ok := base.Nodes[id]; ok {
				state.nodes[id].x = baseNode.X
			}
		}
	} else {
		state.assignCoordinates()
	}

	state.computePortOffsets()
	state.packComponents()
	state.routeEdges()
	state.undoDirectionAdjustment()

	layout := state.buildLayout()
	g.adjustClusters(layout)
	return layout
}

// adjustClusters sizes cluster nodes to contain their children.
func (g *Graph) adjustClusters(layout *Layout) {
	if len(g.clusters) == 0 {
		return
	}

	// Process clusters (may need multiple passes for nested clusters)
	for clusterID, padding := range g.clusters {
		children := g.Children(clusterID)
		if len(children) == 0 {
			continue
		}

		// Find bounding box of all children
		minX := math.Inf(1)
		minY := math.Inf(1)
		maxX := math.Inf(-1)
		maxY := math.Inf(-1)

		hasChild := false
		for _, childID := range children {
			childLayout, ok := layout.Nodes[childID]
			if !ok {
				continue
			}
			hasChild = true
			if childLayout.X < minX {
				minX = childLayout.X
			}
			if childLayout.Y < minY {
				minY = childLayout.Y
			}
			if childLayout.X+childLayout.Width > maxX {
				maxX = childLayout.X + childLayout.Width
			}
			if childLayout.Y+childLayout.Height > maxY {
				maxY = childLayout.Y + childLayout.Height
			}
		}

		if !hasChild {
			continue
		}

		// Set cluster position and size to contain children with padding
		clusterLayout := NodeLayout{
			Position: Position{
				X: minX - padding,
				Y: minY - padding,
			},
			Width:  (maxX - minX) + 2*padding,
			Height: (maxY - minY) + 2*padding,
		}
		layout.Nodes[clusterID] = clusterLayout
	}
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
