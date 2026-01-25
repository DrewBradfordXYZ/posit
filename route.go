package posit

import (
	"math"
	"runtime"
	"sort"
	"sync"
)

// maxDummyChainIterations is a safeguard against infinite loops when
// traversing dummy chains. This should never be reached in practice,
// but prevents hangs if the chain structure were corrupted.
const maxDummyChainIterations = 10000

// routeEdges builds final edge paths and prepares output.
// This is Phase 6 of the Sugiyama algorithm.
func (s *layoutState) routeEdges() {
	// Step 1: Build paths from dummy chains
	s.buildEdgePaths()

	// Step 2: Handle edges without dummies
	s.initializeShortEdges()

	// Step 3: Restore reversed edges
	s.restoreReversedEdges()

	// Step 4: Route based on style
	if s.opts.RouteStyle == RouteOrthogonal {
		s.routeOrthogonal()
	} else {
		// Step 4a: Add node boundary points (polyline mode)
		s.addNodeIntersections()
	}

	// Step 5: Offset parallel multi-edges
	s.offsetParallelEdges()

	// Step 6: Infer attachment sides for all edges
	s.inferEdgeSides()

	// Step 7: Resolve label collisions
	s.resolveEdgeLabelCollisions()

	// Step 8: Restore self-loops with curved paths
	s.restoreSelfLoops()
}

// chainPathResult holds the computed path for one dummy chain.
type chainPathResult struct {
	sourceID   string
	targetID   string
	edge       *layoutEdge
	points     []EdgePoint
	labelX     float64
	labelY     float64
	hasLabel   bool
	dummyEdges []edgeKey
}

// buildEdgePaths walks dummy chains to create edge bend points.
// It also restores the original edges that were split and removes dummy edges.
// For graphs with many chains (≥50), path building runs in parallel.
func (s *layoutState) buildEdgePaths() {
	// Filter valid chains
	validChains := make([]string, 0, len(s.dummyChains))
	for _, firstDummy := range s.dummyChains {
		dummy := s.nodes[firstDummy]
		if dummy != nil && dummy.edgeLabel != nil && len(s.predecessors[firstDummy]) > 0 {
			validChains = append(validChains, firstDummy)
		}
	}

	if len(validChains) == 0 {
		return
	}

	// Build paths — parallel for large graphs, sequential for small ones
	var results []chainPathResult
	if len(validChains) >= 50 {
		results = s.buildChainPathsParallel(validChains)
	} else {
		results = s.buildChainPathsSequential(validChains)
	}

	// Apply results to edge map (sequential — shared map mutations)
	for i := range results {
		r := &results[i]
		originalKey := edgeKey{from: r.sourceID, to: r.targetID, id: r.edge.id}
		r.edge.key = originalKey
		r.edge.points = r.points
		if r.hasLabel {
			r.edge.labelX = r.labelX
			r.edge.labelY = r.labelY
		}
		s.edges[originalKey] = r.edge

		for _, dk := range r.dummyEdges {
			delete(s.edges, dk)
		}
	}
}

// buildChainPathsSequential builds paths for all chains sequentially.
func (s *layoutState) buildChainPathsSequential(chains []string) []chainPathResult {
	results := make([]chainPathResult, len(chains))
	for i, firstDummy := range chains {
		results[i] = s.buildChainPath(firstDummy)
	}
	return results
}

// buildChainPathsParallel builds paths using a fixed number of worker goroutines.
// Uses runtime.NumCPU() workers to avoid spawning thousands of goroutines for
// graphs with many short chains (where per-goroutine overhead would dominate).
func (s *layoutState) buildChainPathsParallel(chains []string) []chainPathResult {
	results := make([]chainPathResult, len(chains))
	workers := runtime.NumCPU()
	if workers > len(chains) {
		workers = len(chains)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	chunkSize := (len(chains) + workers - 1) / workers

	for w := 0; w < workers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(chains) {
			end = len(chains)
		}
		go func(from, to int) {
			defer wg.Done()
			for i := from; i < to; i++ {
				results[i] = s.buildChainPath(chains[i])
			}
		}(start, end)
	}
	wg.Wait()
	return results
}

// buildChainPath walks a single dummy chain to compute its path points.
// Reads only from s.nodes, s.predecessors, s.successors (all read-only during routing).
func (s *layoutState) buildChainPath(firstDummy string) chainPathResult {
	edge := s.nodes[firstDummy].edgeLabel
	sourceID := s.predecessors[firstDummy][0]

	result := chainPathResult{
		sourceID:   sourceID,
		edge:       edge,
		points:     make([]EdgePoint, 0),
		dummyEdges: []edgeKey{{from: sourceID, to: firstDummy}},
	}

	current := firstDummy
	iterations := 0
	for {
		if iterations >= maxDummyChainIterations {
			break
		}
		iterations++

		node := s.nodes[current]
		if node == nil || !node.isDummy {
			result.targetID = current
			break
		}

		if edge.labelDummyID == current {
			result.labelX = node.x + node.width/2
			result.labelY = node.y + node.height/2
			result.hasLabel = true
		}

		result.points = append(result.points, EdgePoint{
			X: node.x + node.width/2,
			Y: node.y + node.height/2,
		})

		successors := s.successors[current]
		if len(successors) == 0 {
			break
		}
		nextNode := successors[0]
		result.dummyEdges = append(result.dummyEdges, edgeKey{from: current, to: nextNode})
		current = nextNode
	}

	return result
}

// initializeShortEdges ensures edges without dummies have points arrays
// and calculates label positions for edges that span only one layer.
func (s *layoutState) initializeShortEdges() {
	for key, edge := range s.edges {
		if edge.points == nil {
			edge.points = make([]EdgePoint, 0)
		}

		// Calculate label position for short edges (those without dummy nodes)
		// These edges have no labelDummyID because they span only one layer
		if edge.labelDummyID == "" && (edge.labelWidth > 0 || edge.labelHeight > 0) {
			fromNode := s.nodes[key.from]
			toNode := s.nodes[key.to]
			if fromNode != nil && toNode != nil {
				// Place label at midpoint between source and target centers
				edge.labelX = (fromNode.x + fromNode.width/2 + toNode.x + toNode.width/2) / 2
				edge.labelY = (fromNode.y + fromNode.height/2 + toNode.y + toNode.height/2) / 2
			}
		}
	}
}

// restoreReversedEdges flips reversed edges back to original direction.
func (s *layoutState) restoreReversedEdges() {
	for _, key := range s.reversedEdges {
		// The key stored is the ORIGINAL direction (before reversal).
		// After reversal, the edge is stored under the reversed key.
		reversedKey := edgeKey{from: key.to, to: key.from, id: key.id}
		edge := s.edges[reversedKey]
		if edge == nil {
			continue
		}

		// Reverse the points array so edge flows in original direction
		for i, j := 0, len(edge.points)-1; i < j; i, j = i+1, j-1 {
			edge.points[i], edge.points[j] = edge.points[j], edge.points[i]
		}

		// Move edge back to original key
		delete(s.edges, reversedKey)
		edge.key = key
		edge.reversed = false
		s.edges[key] = edge
	}

	// Clear reversed edges list
	s.reversedEdges = nil
}

// addNodeIntersections adds start/end points at node boundaries or port positions.
func (s *layoutState) addNodeIntersections() {
	for key, edge := range s.edges {
		fromNode := s.nodes[key.from]
		toNode := s.nodes[key.to]

		if fromNode == nil || toNode == nil {
			continue
		}

		// Calculate start point
		var startPoint EdgePoint
		if edge.sourcePort != "" {
			if pt, ok := s.getPortPosition(fromNode, edge.sourcePort); ok {
				startPoint = pt
			} else {
				startPoint = s.getIntersectionStart(fromNode, toNode, edge)
			}
		} else {
			startPoint = s.getIntersectionStart(fromNode, toNode, edge)
		}

		// Calculate end point
		var endPoint EdgePoint
		if edge.targetPort != "" {
			if pt, ok := s.getPortPosition(toNode, edge.targetPort); ok {
				endPoint = pt
			} else {
				endPoint = s.getIntersectionEnd(fromNode, toNode, edge)
			}
		} else {
			endPoint = s.getIntersectionEnd(fromNode, toNode, edge)
		}

		// Build final points array
		finalPoints := make([]EdgePoint, 0, len(edge.points)+2)
		finalPoints = append(finalPoints, startPoint)
		finalPoints = append(finalPoints, edge.points...)
		finalPoints = append(finalPoints, endPoint)
		edge.points = finalPoints
	}
}

// getIntersectionStart computes the edge start point via boundary intersection.
func (s *layoutState) getIntersectionStart(fromNode, toNode *layoutNode, edge *layoutEdge) EdgePoint {
	var firstTarget EdgePoint
	if len(edge.points) > 0 {
		firstTarget = edge.points[0]
	} else {
		firstTarget = EdgePoint{
			X: toNode.x + toNode.width/2,
			Y: toNode.y + toNode.height/2,
		}
	}
	return s.intersectRect(fromNode, firstTarget)
}

// getIntersectionEnd computes the edge end point via boundary intersection.
func (s *layoutState) getIntersectionEnd(fromNode, toNode *layoutNode, edge *layoutEdge) EdgePoint {
	var lastSource EdgePoint
	if len(edge.points) > 0 {
		lastSource = edge.points[len(edge.points)-1]
	} else {
		lastSource = EdgePoint{
			X: fromNode.x + fromNode.width/2,
			Y: fromNode.y + fromNode.height/2,
		}
	}
	return s.intersectRect(toNode, lastSource)
}

// getPortPosition resolves a port ID to an absolute position on a node.
// Port sides are specified in user coordinate space and transformed to internal
// layout space based on the current direction.
// When port Width/Height are specified, returns the center of the port's
// attachment area on the node boundary.
func (s *layoutState) getPortPosition(node *layoutNode, portID string) (EdgePoint, bool) {
	for _, port := range node.ports {
		if port.ID == portID {
			// Transform port side from user space to internal layout space
			side := s.portSideToInternal(port.Side)

			// Port center offset (accounting for port dimensions)
			portCenterOffset := port.Offset
			if port.Width > 0 || port.Height > 0 {
				// Center the port on its offset
				switch side {
				case Right, Left:
					portCenterOffset = port.Offset + port.Height/2
				case Top, Bottom:
					portCenterOffset = port.Offset + port.Width/2
				}
			}

			switch side {
			case Right:
				return EdgePoint{X: node.x + node.width, Y: node.y + portCenterOffset}, true
			case Left:
				return EdgePoint{X: node.x, Y: node.y + portCenterOffset}, true
			case Bottom:
				return EdgePoint{X: node.x + portCenterOffset, Y: node.y + node.height}, true
			case Top:
				return EdgePoint{X: node.x + portCenterOffset, Y: node.y}, true
			}
		}
	}
	return EdgePoint{}, false
}

// portSideToInternal transforms a port side from user coordinate space to
// internal layout space. This is the inverse of the side rotation applied
// in undoDirectionAdjustment.
func (s *layoutState) portSideToInternal(side Side) Side {
	switch s.opts.Direction {
	case LeftToRight:
		return rotateLR(side)
	case RightToLeft:
		return rotateRL(side)
	case BottomToTop:
		return flipVertical(side)
	default:
		return side
	}
}

// intersectRect finds intersection of line from node center to external point.
func (s *layoutState) intersectRect(node *layoutNode, point EdgePoint) EdgePoint {
	cx := node.x + node.width/2
	cy := node.y + node.height/2
	w := node.width / 2
	h := node.height / 2

	// Handle zero-size nodes (dummies)
	if w == 0 || h == 0 {
		return EdgePoint{X: cx, Y: cy}
	}

	dx := point.X - cx
	dy := point.Y - cy

	if dx == 0 && dy == 0 {
		return EdgePoint{X: cx, Y: cy}
	}

	var sx, sy float64

	if math.Abs(dy)*w > math.Abs(dx)*h {
		// Top or bottom intersection
		if dy < 0 {
			h = -h
		}
		sx = h * dx / dy
		sy = h
	} else {
		// Left or right intersection
		if dx < 0 {
			w = -w
		}
		sx = w
		sy = w * dy / dx
	}

	return EdgePoint{X: cx + sx, Y: cy + sy}
}

// inferEdgeSides computes the optimal attachment side for each edge endpoint.
// For PortFixedOffset ports, the side is computed per-edge based on the direction
// to the specific connected node (not averaged across all connections).
// For other port types, the port's pre-computed Side takes precedence.
func (s *layoutState) inferEdgeSides() {
	for _, edge := range s.edges {
		fromNode := s.nodes[edge.key.from]
		toNode := s.nodes[edge.key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Source side
		if edge.sourcePort != "" {
			if port := s.getPort(fromNode, edge.sourcePort); port != nil {
				edge.sourceSide = s.edgePortSide(port, fromNode, toNode)
			} else {
				edge.sourceSide, _ = inferSide(fromNode, toNode)
			}
		} else {
			edge.sourceSide, _ = inferSide(fromNode, toNode)
		}

		// Target side
		if edge.targetPort != "" {
			if port := s.getPort(toNode, edge.targetPort); port != nil {
				edge.targetSide = s.edgePortSide(port, toNode, fromNode)
			} else {
				_, edge.targetSide = inferSide(fromNode, toNode)
			}
		} else {
			_, edge.targetSide = inferSide(fromNode, toNode)
		}
	}
}

// edgePortSide computes the attachment side for a port on thisNode facing connNode.
// For PortFixedOffset: per-edge side based on direction to the specific connected node,
// constrained by the port's axis. This ensures each edge exits toward its target
// rather than using an averaged direction across all connections.
//
// When nodes have significant overlap on the constrained axis (>50% of smaller node),
// same-side routing is used to avoid edges crossing through overlapping node areas.
// This handles the common case of vertically-stacked nodes with slight horizontal offset.
//
// For other constraints: uses the port's pre-computed side (user space, converted to internal).
//
// Returns an internal-space side that undoDirectionAdjustment will transform to user space.
func (s *layoutState) edgePortSide(port *PortOptions, thisNode, connNode *layoutNode) Side {
	if port.Constraint == PortFixedOffset {
		// Check for significant overlap - if nodes are nearly stacked,
		// use same-side routing to avoid edges crossing through node areas.
		if side, ok := s.overlapAwareSide(thisNode, connNode, port.Axis); ok {
			return s.portSideToInternal(side)
		}

		// No significant overlap: use direction to connected node
		dx := (connNode.x + connNode.width/2) - (thisNode.x + thisNode.width/2)
		dy := (connNode.y + connNode.height/2) - (thisNode.y + thisNode.height/2)
		// Transform to user space (axis constraints are defined in user space)
		dx, dy = s.internalToUserDirection(dx, dy)
		// bestSide returns a user-space side; convert to internal so
		// undoDirectionAdjustment can correctly transform it back to user space.
		return s.portSideToInternal(s.bestSide(dx, dy, port.Axis))
	}
	// port.Side is in user space (set by assignFreeSides); convert to internal.
	return s.portSideToInternal(port.Side)
}

// overlapAwareSide uses node boundaries to determine edge attachment side.
// When nodes have a clear horizontal gap, edges face each other (opposite sides).
// When nodes overlap horizontally, edges use same-side routing to avoid crossing.
//
// This is more precise than center-to-center direction because it uses actual
// node boundaries rather than a heuristic based on center positions.
func (s *layoutState) overlapAwareSide(thisNode, connNode *layoutNode, axis PortAxis) (Side, bool) {
	// Only apply for horizontal axis ports in vertical flow directions.
	if axis != PortAxisHorizontal {
		return Top, false
	}
	if s.opts.Direction != TopToBottom && s.opts.Direction != BottomToTop {
		return Top, false
	}

	// Node boundaries in internal coordinates
	thisLeft := thisNode.x
	thisRight := thisNode.x + thisNode.width
	connLeft := connNode.x
	connRight := connNode.x + connNode.width

	// Check for clear horizontal gap (no overlap)
	if thisRight < connLeft {
		// Connected node is clearly to the right → use opposite sides (Right→Left)
		return Top, false // Let default direction-based logic handle it
	}
	if connRight < thisLeft {
		// Connected node is clearly to the left → use opposite sides (Left→Right)
		return Top, false // Let default direction-based logic handle it
	}

	// Nodes overlap horizontally → use same-side routing
	// Both source and target exit/enter from Right, creating a detour arc
	return Right, true
}

// inferSide determines the optimal source and target sides based on relative positions.
func inferSide(fromNode, toNode *layoutNode) (sourceSide, targetSide Side) {
	dx := (toNode.x + toNode.width/2) - (fromNode.x + fromNode.width/2)
	dy := (toNode.y + toNode.height/2) - (fromNode.y + fromNode.height/2)

	if math.Abs(dx) > math.Abs(dy) {
		if dx > 0 {
			return Right, Left
		}
		return Left, Right
	}
	if dy > 0 {
		return Bottom, Top
	}
	return Top, Bottom
}

// getPort returns the port with the given ID on a node, or nil if not found.
func (s *layoutState) getPort(node *layoutNode, portID string) *PortOptions {
	for i := range node.ports {
		if node.ports[i].ID == portID {
			return &node.ports[i]
		}
	}
	return nil
}

// getPortSide returns the side of a specific port on a node.
func (s *layoutState) getPortSide(node *layoutNode, portID string) (Side, bool) {
	if port := s.getPort(node, portID); port != nil {
		return port.Side, true
	}
	return Top, false
}

// offsetParallelEdges offsets multiple edges between the same node pair.
// Each edge's entire polyline is offset perpendicular to each segment direction,
// so parallel edges remain visually separated even around bends.
func (s *layoutState) offsetParallelEdges() {
	const parallelEdgeSpacing = 10.0

	// Group edges by (from, to) pair
	type pairKey struct{ from, to string }
	groups := make(map[pairKey][]*layoutEdge)

	for _, edge := range s.edges {
		pk := pairKey{edge.key.from, edge.key.to}
		groups[pk] = append(groups[pk], edge)
	}

	for _, edges := range groups {
		n := len(edges)
		if n <= 1 {
			continue
		}

		// Sort by ID for deterministic ordering
		sort.Slice(edges, func(i, j int) bool {
			return edges[i].id < edges[j].id
		})

		for i, edge := range edges {
			offset := (float64(i) - float64(n-1)/2) * parallelEdgeSpacing
			if offset == 0 {
				continue
			}

			pts := edge.points
			if len(pts) < 2 {
				continue
			}

			// Offset each point based on the perpendicular of its adjacent segments
			newPts := make([]EdgePoint, len(pts))
			for j := range pts {
				// Compute average direction at this point from adjacent segments
				var dx, dy float64
				if j == 0 {
					// First point: use direction of first segment
					dx = pts[1].X - pts[0].X
					dy = pts[1].Y - pts[0].Y
				} else if j == len(pts)-1 {
					// Last point: use direction of last segment
					dx = pts[j].X - pts[j-1].X
					dy = pts[j].Y - pts[j-1].Y
				} else {
					// Middle point: average of incoming and outgoing segments
					dx = pts[j+1].X - pts[j-1].X
					dy = pts[j+1].Y - pts[j-1].Y
				}

				length := math.Sqrt(dx*dx + dy*dy)
				if length == 0 {
					newPts[j] = pts[j]
					continue
				}

				// Perpendicular unit vector
				px := -dy / length
				py := dx / length

				newPts[j] = EdgePoint{
					X: pts[j].X + px*offset,
					Y: pts[j].Y + py*offset,
				}
			}
			edge.points = newPts
		}
	}
}

// orthoRect represents a node bounding box for obstacle avoidance.
type orthoRect struct {
	x, y, w, h float64
	id          string
}

// orthoSegKey identifies a channel segment for spacing.
type orthoSegKey struct {
	axis     byte // 'h' for horizontal, 'v' for vertical
	position int  // quantized primary coordinate
}

// routeOrthogonal implements channel-based orthogonal edge routing.
// Edges are routed using horizontal and vertical segments through channels
// between node columns and layers. It avoids routing through nodes and
// spaces parallel edges in shared channels.
func (s *layoutState) routeOrthogonal() {
	channelGap := s.opts.ChannelGap
	if channelGap <= 0 {
		channelGap = 10
	}

	// Collect all edges that need routing
	type edgeEntry struct {
		key  edgeKey
		edge *layoutEdge
	}
	var edgesToRoute []edgeEntry
	for key, edge := range s.edges {
		edgesToRoute = append(edgesToRoute, edgeEntry{key, edge})
	}

	// Sort for deterministic channel assignment
	sort.Slice(edgesToRoute, func(i, j int) bool {
		if edgesToRoute[i].key.from != edgesToRoute[j].key.from {
			return edgesToRoute[i].key.from < edgesToRoute[j].key.from
		}
		if edgesToRoute[i].key.to != edgesToRoute[j].key.to {
			return edgesToRoute[i].key.to < edgesToRoute[j].key.to
		}
		return edgesToRoute[i].key.id < edgesToRoute[j].key.id
	})

	// Build node bounding boxes for obstacle avoidance (exclude edge endpoints)
	var obstacles []orthoRect
	for id, node := range s.nodes {
		if node.isDummy {
			continue
		}
		obstacles = append(obstacles, orthoRect{node.x, node.y, node.width, node.height, id})
	}

	// Add cluster bounding boxes as obstacles
	clusterObstacles := s.computeClusterObstacles()
	obstacles = append(obstacles, clusterObstacles...)

	// Compute horizontal channel Y positions (midpoints between layers)
	layerYs := s.computeLayerChannelYs()

	// Track channel segment usage for edge spacing
	channelUsage := make(map[orthoSegKey]int)

	for _, entry := range edgesToRoute {
		edge := entry.edge
		fromNode := s.nodes[entry.key.from]
		toNode := s.nodes[entry.key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Determine start and end points (port or node center-bottom/center-top)
		var startPt, endPt EdgePoint
		if edge.sourcePort != "" {
			if pt, ok := s.getPortPosition(fromNode, edge.sourcePort); ok {
				startPt = pt
			} else {
				startPt = EdgePoint{X: fromNode.x + fromNode.width/2, Y: fromNode.y + fromNode.height}
			}
		} else {
			startPt = EdgePoint{X: fromNode.x + fromNode.width/2, Y: fromNode.y + fromNode.height}
		}

		if edge.targetPort != "" {
			if pt, ok := s.getPortPosition(toNode, edge.targetPort); ok {
				endPt = pt
			} else {
				endPt = EdgePoint{X: toNode.x + toNode.width/2, Y: toNode.y}
			}
		} else {
			endPt = EdgePoint{X: toNode.x + toNode.width/2, Y: toNode.y}
		}

		// Route the edge through waypoints with node avoidance
		var waypoints []EdgePoint
		if len(edge.points) > 0 {
			waypoints = edge.points
		}

		orthoPoints := s.buildOrthogonalPath(startPt, endPt, waypoints, fromNode, toNode, obstacles, layerYs, channelGap, channelUsage)
		edge.points = orthoPoints
	}
}

// computeLayerChannelYs returns Y coordinates of horizontal routing channels
// (the midpoints between each pair of adjacent layers).
func (s *layoutState) computeLayerChannelYs() []float64 {
	if len(s.layers) <= 1 {
		return nil
	}

	channels := make([]float64, len(s.layers)-1)
	for i := 0; i < len(s.layers)-1; i++ {
		// Find bottom of upper layer
		upperBottom := 0.0
		for _, id := range s.layers[i] {
			node := s.nodes[id]
			if bottom := node.y + node.height; bottom > upperBottom {
				upperBottom = bottom
			}
		}
		// Find top of lower layer
		lowerTop := math.Inf(1)
		for _, id := range s.layers[i+1] {
			node := s.nodes[id]
			if node.y < lowerTop {
				lowerTop = node.y
			}
		}
		channels[i] = (upperBottom + lowerTop) / 2
	}
	return channels
}

// buildOrthogonalPath creates an orthogonal path from start to end,
// routing through waypoints while avoiding obstacle nodes.
func (s *layoutState) buildOrthogonalPath(
	start, end EdgePoint,
	waypoints []EdgePoint,
	fromNode, toNode *layoutNode,
	obstacles []orthoRect,
	layerYs []float64,
	channelGap float64,
	channelUsage map[orthoSegKey]int,
) []EdgePoint {

	// Build sequence of points to connect: start → waypoints → end
	allPts := make([]EdgePoint, 0, len(waypoints)+2)
	allPts = append(allPts, start)
	allPts = append(allPts, waypoints...)
	allPts = append(allPts, end)

	result := make([]EdgePoint, 0, len(allPts)*3)
	result = append(result, start)

	for i := 1; i < len(allPts); i++ {
		prev := allPts[i-1]
		next := allPts[i]

		if prev.X == next.X && prev.Y == next.Y {
			continue
		}

		// Find the horizontal channel Y for this segment
		channelY := (prev.Y + next.Y) / 2
		// Find nearest layer channel if available
		if len(layerYs) > 0 {
			bestDist := math.Inf(1)
			for _, ly := range layerYs {
				if ly > prev.Y && ly < next.Y {
					dist := math.Abs(ly - channelY)
					if dist < bestDist {
						bestDist = dist
						channelY = ly
					}
				}
			}
		}

		// Apply channel spacing offset
		sk := orthoSegKey{axis: 'h', position: int(channelY / channelGap)}
		usage := channelUsage[sk]
		channelUsage[sk]++
		channelY += float64(usage) * channelGap

		// Determine the X channel for the vertical segment
		channelX := next.X
		if prev.X != next.X {
			// Check if vertical path at channelX intersects any obstacle
			channelX = s.findClearVerticalChannel(prev.X, next.X, channelY, next.Y, fromNode, toNode, obstacles, channelGap, channelUsage)
		}

		// Route: vertical from prev to channelY, horizontal to channelX, vertical to next
		if prev.Y != channelY {
			result = append(result, EdgePoint{X: prev.X, Y: channelY})
		}
		if prev.X != channelX {
			result = append(result, EdgePoint{X: channelX, Y: channelY})
		}
		if channelY != next.Y && channelX != next.X {
			result = append(result, EdgePoint{X: channelX, Y: next.Y})
		}
	}

	// Deduplicate consecutive identical points
	if len(result) == 0 || result[len(result)-1] != end {
		result = append(result, end)
	}

	return s.deduplicatePoints(result)
}

// findClearVerticalChannel finds a vertical X position that avoids obstacles
// between the horizontal channel and the target Y.
func (s *layoutState) findClearVerticalChannel(
	fromX, toX, fromY, toY float64,
	fromNode, toNode *layoutNode,
	obstacles []orthoRect,
	channelGap float64,
	channelUsage map[orthoSegKey]int,
) float64 {
	// Preferred X is the target X (shortest path)
	preferredX := toX

	// Check if preferred X intersects any obstacle in the Y range
	minY := math.Min(fromY, toY)
	maxY := math.Max(fromY, toY)

	clear := true
	for _, obs := range obstacles {
		// Skip source and target nodes
		if obs.id == fromNode.id || obs.id == toNode.id {
			continue
		}
		// Skip clusters that contain either endpoint
		if s.isNodeInCluster(fromNode.id, obs.id) || s.isNodeInCluster(toNode.id, obs.id) {
			continue
		}
		// Check if obstacle overlaps with the vertical line at preferredX
		if preferredX >= obs.x && preferredX <= obs.x+obs.w {
			// Check Y overlap
			if obs.y < maxY && obs.y+obs.h > minY {
				clear = false
				break
			}
		}
	}

	if clear {
		// Apply vertical channel spacing
		sk := orthoSegKey{axis: 'v', position: int(preferredX / channelGap)}
		usage := channelUsage[sk]
		channelUsage[sk]++
		return preferredX + float64(usage)*channelGap
	}

	// Find a clear channel: try positions to the left and right of the obstacle
	bestX := preferredX
	bestDist := math.Inf(1)

	// Collect all X boundaries of obstacles in the Y range
	boundaries := make([]float64, 0, 2*len(obstacles)+1)
	for _, obs := range obstacles {
		if obs.id == fromNode.id || obs.id == toNode.id {
			continue
		}
		if s.isNodeInCluster(fromNode.id, obs.id) || s.isNodeInCluster(toNode.id, obs.id) {
			continue
		}
		if obs.y < maxY && obs.y+obs.h > minY {
			boundaries = append(boundaries, obs.x-channelGap)
			boundaries = append(boundaries, obs.x+obs.w+channelGap)
		}
	}
	// Also consider fromX as a candidate
	boundaries = append(boundaries, fromX)

	for _, candidateX := range boundaries {
		// Check if this X is clear
		candidateClear := true
		for _, obs := range obstacles {
			if obs.id == fromNode.id || obs.id == toNode.id {
				continue
			}
			if s.isNodeInCluster(fromNode.id, obs.id) || s.isNodeInCluster(toNode.id, obs.id) {
				continue
			}
			if candidateX >= obs.x && candidateX <= obs.x+obs.w {
				if obs.y < maxY && obs.y+obs.h > minY {
					candidateClear = false
					break
				}
			}
		}
		if candidateClear {
			dist := math.Abs(candidateX - preferredX)
			if dist < bestDist {
				bestDist = dist
				bestX = candidateX
			}
		}
	}

	sk := orthoSegKey{axis: 'v', position: int(bestX / channelGap)}
	usage := channelUsage[sk]
	channelUsage[sk]++
	return bestX + float64(usage)*channelGap
}

// computeClusterObstacles returns bounding boxes for cluster nodes.
// Edges not originating from within a cluster must route around its boundary.
func (s *layoutState) computeClusterObstacles() []orthoRect {
	if len(s.clusters) == 0 {
		return nil
	}

	var clusterRects []orthoRect
	for clusterID, padding := range s.clusters {
		// Find all children of this cluster
		var children []string
		for childID, parentID := range s.parents {
			if parentID == clusterID {
				children = append(children, childID)
			}
		}
		if len(children) == 0 {
			continue
		}

		// Compute bounding box of children
		minX := math.Inf(1)
		minY := math.Inf(1)
		maxX := math.Inf(-1)
		maxY := math.Inf(-1)

		for _, childID := range children {
			node := s.nodes[childID]
			if node == nil {
				continue
			}
			if node.x < minX {
				minX = node.x
			}
			if node.y < minY {
				minY = node.y
			}
			if node.x+node.width > maxX {
				maxX = node.x + node.width
			}
			if node.y+node.height > maxY {
				maxY = node.y + node.height
			}
		}

		if math.IsInf(minX, 1) {
			continue
		}

		// Add padding to form cluster boundary
		clusterRects = append(clusterRects, orthoRect{
			x:  minX - padding,
			y:  minY - padding,
			w:  (maxX - minX) + 2*padding,
			h:  (maxY - minY) + 2*padding,
			id: clusterID,
		})
	}

	return clusterRects
}

// isNodeInCluster returns true if the given node is inside the specified cluster.
func (s *layoutState) isNodeInCluster(nodeID, clusterID string) bool {
	current := nodeID
	for {
		parent, ok := s.parents[current]
		if !ok || parent == "" {
			return false
		}
		if parent == clusterID {
			return true
		}
		current = parent
	}
}

// deduplicatePoints removes consecutive identical points and collinear points.
func (s *layoutState) deduplicatePoints(points []EdgePoint) []EdgePoint {
	if len(points) <= 2 {
		return points
	}

	result := []EdgePoint{points[0]}
	for i := 1; i < len(points)-1; i++ {
		prev := result[len(result)-1]
		curr := points[i]
		next := points[i+1]

		// Skip if same as previous
		if curr.X == prev.X && curr.Y == prev.Y {
			continue
		}
		// Skip if collinear (all on same horizontal or vertical line)
		if (prev.X == curr.X && curr.X == next.X) ||
			(prev.Y == curr.Y && curr.Y == next.Y) {
			continue
		}
		result = append(result, curr)
	}
	result = append(result, points[len(points)-1])
	return result
}

// resolveEdgeLabelCollisions nudges edge labels that overlap with nodes or other labels.
func (s *layoutState) resolveEdgeLabelCollisions() {
	// Collect all labels with bounding boxes
	type labelRect struct {
		edge *layoutEdge
		x, y float64
		w, h float64
	}
	var labels []labelRect
	for _, edge := range s.edges {
		if edge.labelWidth > 0 || edge.labelHeight > 0 {
			labels = append(labels, labelRect{
				edge: edge,
				x:    edge.labelX - edge.labelWidth/2,
				y:    edge.labelY - edge.labelHeight/2,
				w:    edge.labelWidth,
				h:    edge.labelHeight,
			})
		}
	}

	if len(labels) <= 1 {
		return
	}

	// Check label-label overlaps and nudge
	nudgeStep := s.opts.NodeSep / 4
	if nudgeStep < 5 {
		nudgeStep = 5
	}

	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			li := labels[i]
			lj := labels[j]

			// Check overlap
			if li.x < lj.x+lj.w && li.x+li.w > lj.x &&
				li.y < lj.y+lj.h && li.y+li.h > lj.y {
				// Nudge the second label away
				// Determine nudge direction (perpendicular to edge direction)
				dy := lj.y - li.y
				if dy >= 0 {
					labels[j].y += nudgeStep
					labels[j].edge.labelY += nudgeStep
				} else {
					labels[j].y -= nudgeStep
					labels[j].edge.labelY -= nudgeStep
				}
			}
		}
	}

	// Check label-node overlaps
	for i := range labels {
		for _, node := range s.nodes {
			if node.isDummy || node.width == 0 {
				continue
			}
			nx, ny := node.x, node.y
			nw, nh := node.width, node.height

			li := labels[i]
			if li.x < nx+nw && li.x+li.w > nx &&
				li.y < ny+nh && li.y+li.h > ny {
				// Nudge label below the node
				labels[i].y = ny + nh + nudgeStep/2
				labels[i].edge.labelY = labels[i].y + labels[i].h/2
			}
		}
	}
}

// restoreSelfLoops generates paths for self-referential edges.
// For edges with PortFixedOffset ports, both endpoints attach on the same side
// (determined by port axis) and arc outward via a waypoint. Multiple self-loops
// on the same node are staggered so arcs don't overlap.
// For edges without port info, a default right-side arc is generated.
func (s *layoutState) restoreSelfLoops() {
	// Group by node for staggering
	nodeLoops := make(map[string][]*layoutEdge)
	for _, edge := range s.selfLoops {
		nodeLoops[edge.key.from] = append(nodeLoops[edge.key.from], edge)
	}

	for nodeID, loops := range nodeLoops {
		node := s.nodes[nodeID]
		if node == nil {
			continue
		}

		for i, edge := range loops {
			side := s.selfLoopArcSide(node, edge)
			srcOffset := s.selfLoopPortOffset(node, edge.sourcePort, side)
			tgtOffset := s.selfLoopPortOffset(node, edge.targetPort, side)

			// Arc width scales with port distance for perpendicular entry/exit.
			// Stagger multiple self-loops so arcs don't overlap.
			portDist := tgtOffset - srcOffset
			if portDist < 0 {
				portDist = -portDist
			}
			loopDist := portDist * 0.6
			if loopDist < 30.0 {
				loopDist = 30.0
			}
			loopDist += float64(i) * 10.0
			midOffset := (srcOffset + tgtOffset) / 2

			var start, waypoint, end EdgePoint
			switch side {
			case Right:
				x := node.x + node.width
				start = EdgePoint{X: x, Y: node.y + srcOffset}
				waypoint = EdgePoint{X: x + loopDist, Y: node.y + midOffset}
				end = EdgePoint{X: x, Y: node.y + tgtOffset}
			case Left:
				x := node.x
				start = EdgePoint{X: x, Y: node.y + srcOffset}
				waypoint = EdgePoint{X: x - loopDist, Y: node.y + midOffset}
				end = EdgePoint{X: x, Y: node.y + tgtOffset}
			case Bottom:
				y := node.y + node.height
				start = EdgePoint{X: node.x + srcOffset, Y: y}
				waypoint = EdgePoint{X: node.x + midOffset, Y: y + loopDist}
				end = EdgePoint{X: node.x + tgtOffset, Y: y}
			default: // Top
				y := node.y
				start = EdgePoint{X: node.x + srcOffset, Y: y}
				waypoint = EdgePoint{X: node.x + midOffset, Y: y - loopDist}
				end = EdgePoint{X: node.x + tgtOffset, Y: y}
			}

			edge.points = []EdgePoint{start, waypoint, end}
			edge.sourceSide = side
			edge.targetSide = side

			s.edges[edge.key] = edge
		}
	}
}

// selfLoopArcSide determines which side self-loop arcs should use.
// For PortFixedOffset with PortAxisHorizontal: Right (internal space).
// For PortFixedOffset with PortAxisVertical: Bottom (internal space).
// Falls back to Right if no port info available.
func (s *layoutState) selfLoopArcSide(node *layoutNode, edge *layoutEdge) Side {
	// Check source port axis preference
	if edge.sourcePort != "" {
		if port := s.getPort(node, edge.sourcePort); port != nil {
			if port.Constraint == PortFixedOffset {
				switch port.Axis {
				case PortAxisVertical:
					return s.portSideToInternal(Bottom)
				default:
					return s.portSideToInternal(Right)
				}
			}
		}
	}
	return s.portSideToInternal(Right)
}

// selfLoopPortOffset returns the offset along the arc side for a port.
// For PortFixedOffset ports, uses the declared offset.
// Falls back to node center along that side.
func (s *layoutState) selfLoopPortOffset(node *layoutNode, portID string, side Side) float64 {
	if portID != "" {
		if port := s.getPort(node, portID); port != nil {
			return port.Offset
		}
	}
	// Fallback: center of the side
	switch side {
	case Right, Left:
		return node.height / 2
	default:
		return node.width / 2
	}
}
