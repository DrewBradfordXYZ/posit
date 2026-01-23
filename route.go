package posit

import (
	"math"
	"sort"
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

	// Step 7: Restore self-loops with curved paths
	s.restoreSelfLoops()
}

// buildEdgePaths walks dummy chains to create edge bend points.
// It also restores the original edges that were split and removes dummy edges.
func (s *layoutState) buildEdgePaths() {
	// Collect dummy edges to remove
	dummyEdges := make([]edgeKey, 0)

	for _, firstDummy := range s.dummyChains {
		dummy := s.nodes[firstDummy]
		if dummy == nil || dummy.edgeLabel == nil {
			continue
		}

		edge := dummy.edgeLabel
		edge.points = make([]EdgePoint, 0)

		// Find the source node (predecessor of first dummy)
		preds := s.predecessors[firstDummy]
		if len(preds) == 0 {
			continue
		}
		sourceID := preds[0]

		// Mark edge from source to first dummy for removal
		dummyEdges = append(dummyEdges, edgeKey{from: sourceID, to: firstDummy})

		// Walk the chain
		current := firstDummy
		var targetID string
		iterations := 0
		for {
			if iterations >= maxDummyChainIterations {
				break // Prevent infinite loop from corrupted chain structure
			}
			iterations++

			node := s.nodes[current]
			if node == nil || !node.isDummy {
				// Current is the target node (not a dummy)
				targetID = current
				break
			}

			// Check if this is the label dummy - extract its position
			if edge.labelDummyID == current {
				edge.labelX = node.x + node.width/2
				edge.labelY = node.y + node.height/2
			}

			// Add dummy's position as bend point
			// Use center of node (even though dummies have 0 size)
			edge.points = append(edge.points, EdgePoint{
				X: node.x + node.width/2,
				Y: node.y + node.height/2,
			})

			// Move to next in chain
			successors := s.successors[current]
			if len(successors) == 0 {
				break
			}
			nextNode := successors[0]

			// Mark edge from current dummy to next for removal
			dummyEdges = append(dummyEdges, edgeKey{from: current, to: nextNode})

			current = nextNode
		}

		// Restore the original edge to s.edges
		// The edge's key should be source -> target
		originalKey := edgeKey{from: sourceID, to: targetID}
		edge.key = originalKey
		s.edges[originalKey] = edge
	}

	// Remove dummy edges from s.edges
	for _, key := range dummyEdges {
		delete(s.edges, key)
	}
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
		reversedKey := edgeKey{from: key.to, to: key.from}
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
func (s *layoutState) getPortPosition(node *layoutNode, portID string) (EdgePoint, bool) {
	for _, port := range node.ports {
		if port.ID == portID {
			switch port.Side {
			case Right:
				return EdgePoint{X: node.x + node.width, Y: node.y + port.Offset}, true
			case Left:
				return EdgePoint{X: node.x, Y: node.y + port.Offset}, true
			case Bottom:
				return EdgePoint{X: node.x + port.Offset, Y: node.y + node.height}, true
			case Top:
				return EdgePoint{X: node.x + port.Offset, Y: node.y}, true
			}
		}
	}
	return EdgePoint{}, false
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
// When ports are specified, the port's Side takes precedence.
func (s *layoutState) inferEdgeSides() {
	for _, edge := range s.edges {
		fromNode := s.nodes[edge.key.from]
		toNode := s.nodes[edge.key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Source side
		if edge.sourcePort != "" {
			if side, ok := s.getPortSide(fromNode, edge.sourcePort); ok {
				edge.sourceSide = side
			} else {
				edge.sourceSide, _ = inferSide(fromNode, toNode)
			}
		} else {
			edge.sourceSide, _ = inferSide(fromNode, toNode)
		}

		// Target side
		if edge.targetPort != "" {
			if side, ok := s.getPortSide(toNode, edge.targetPort); ok {
				edge.targetSide = side
			} else {
				_, edge.targetSide = inferSide(fromNode, toNode)
			}
		} else {
			_, edge.targetSide = inferSide(fromNode, toNode)
		}
	}
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

// getPortSide returns the side of a specific port on a node.
func (s *layoutState) getPortSide(node *layoutNode, portID string) (Side, bool) {
	for _, port := range node.ports {
		if port.ID == portID {
			return port.Side, true
		}
	}
	return Top, false
}

// offsetParallelEdges offsets multiple edges between the same node pair.
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

		fromNode := s.nodes[edges[0].key.from]
		toNode := s.nodes[edges[0].key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Compute perpendicular direction
		dx := (toNode.x + toNode.width/2) - (fromNode.x + fromNode.width/2)
		dy := (toNode.y + toNode.height/2) - (fromNode.y + fromNode.height/2)
		length := math.Sqrt(dx*dx + dy*dy)
		if length == 0 {
			continue
		}

		// Perpendicular unit vector
		px := -dy / length
		py := dx / length

		for i, edge := range edges {
			offset := (float64(i) - float64(n-1)/2) * parallelEdgeSpacing
			if offset == 0 {
				continue
			}
			// Offset all points perpendicular to the edge direction
			for j := range edge.points {
				edge.points[j].X += px * offset
				edge.points[j].Y += py * offset
			}
		}
	}
}

// routeOrthogonal implements channel-based orthogonal edge routing.
// Edges are routed using horizontal and vertical segments through channels
// between node columns and layers.
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

	// Track channel usage for spacing: map from X corridor to count of edges using it
	channelUsage := make(map[int]int)

	for _, entry := range edgesToRoute {
		edge := entry.edge
		fromNode := s.nodes[entry.key.from]
		toNode := s.nodes[entry.key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Determine start and end points (port or center)
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

		// Simple orthogonal routing: exit bottom, go to a horizontal channel,
		// then vertical channel, then horizontal to target top.
		if len(edge.points) > 0 {
			// Edge already has waypoints from dummy nodes.
			// Convert to orthogonal segments.
			orthoPoints := make([]EdgePoint, 0, len(edge.points)*2+2)
			orthoPoints = append(orthoPoints, startPt)

			prevPt := startPt
			for _, pt := range edge.points {
				// Add horizontal then vertical segment
				midY := (prevPt.Y + pt.Y) / 2
				channelIdx := int(midY / channelGap)
				usage := channelUsage[channelIdx]
				channelUsage[channelIdx]++
				offset := float64(usage) * channelGap

				orthoPoints = append(orthoPoints, EdgePoint{X: prevPt.X, Y: midY + offset})
				orthoPoints = append(orthoPoints, EdgePoint{X: pt.X, Y: midY + offset})
				prevPt = pt
			}

			// Final segment to target
			midY := (prevPt.Y + endPt.Y) / 2
			orthoPoints = append(orthoPoints, EdgePoint{X: prevPt.X, Y: midY})
			orthoPoints = append(orthoPoints, EdgePoint{X: endPt.X, Y: midY})
			orthoPoints = append(orthoPoints, endPt)

			edge.points = orthoPoints
		} else {
			// Simple one-layer edge: route with one bend
			midY := (startPt.Y + endPt.Y) / 2

			edge.points = []EdgePoint{
				startPt,
				{X: startPt.X, Y: midY},
				{X: endPt.X, Y: midY},
				endPt,
			}
		}
	}
}

// restoreSelfLoops generates curved paths for self-referential edges.
// Self-loops are rendered as a curved path that exits the right side of
// the node, goes up and around, and re-enters from the right.
func (s *layoutState) restoreSelfLoops() {
	for _, edge := range s.selfLoops {
		node := s.nodes[edge.key.from]
		if node == nil {
			continue
		}

		// Calculate loop dimensions based on node size and separation
		loopOffset := s.opts.NodeSep * 0.75
		loopHeight := node.height * 0.5

		// Node center and bounds
		cx := node.x + node.width/2
		cy := node.y + node.height/2
		right := node.x + node.width
		top := node.y

		// Generate a 5-point curved path:
		// 1. Exit point (right side of node, upper half)
		// 2. Control point out (to the right)
		// 3. Top of loop (above node)
		// 4. Control point back (to the right)
		// 5. Entry point (right side of node, lower half)
		edge.points = []EdgePoint{
			{X: right, Y: cy - loopHeight/3},            // Exit upper right
			{X: right + loopOffset, Y: cy - loopHeight}, // Control right-up
			{X: cx, Y: top - loopOffset},                // Top of loop
			{X: right + loopOffset, Y: cy + loopHeight}, // Control right-down
			{X: right, Y: cy + loopHeight/3},            // Entry lower right
		}

		// Add back to edges map
		s.edges[edge.key] = edge
	}
}
