package posit

import "math"

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

	// Step 4: Add node boundary points
	s.addNodeIntersections()

	// Step 5: Restore self-loops with curved paths
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

// addNodeIntersections adds start/end points at node boundaries.
func (s *layoutState) addNodeIntersections() {
	for key, edge := range s.edges {
		fromNode := s.nodes[key.from]
		toNode := s.nodes[key.to]

		if fromNode == nil || toNode == nil {
			continue
		}

		// Calculate start point (intersection with source node)
		var firstTarget EdgePoint
		if len(edge.points) > 0 {
			firstTarget = edge.points[0]
		} else {
			// Straight edge: target is the destination node center
			firstTarget = EdgePoint{
				X: toNode.x + toNode.width/2,
				Y: toNode.y + toNode.height/2,
			}
		}
		startPoint := s.intersectRect(fromNode, firstTarget)

		// Calculate end point (intersection with target node)
		var lastSource EdgePoint
		if len(edge.points) > 0 {
			lastSource = edge.points[len(edge.points)-1]
		} else {
			// Straight edge: source is the source node center
			lastSource = EdgePoint{
				X: fromNode.x + fromNode.width/2,
				Y: fromNode.y + fromNode.height/2,
			}
		}
		endPoint := s.intersectRect(toNode, lastSource)

		// Build final points array
		finalPoints := make([]EdgePoint, 0, len(edge.points)+2)
		finalPoints = append(finalPoints, startPoint)
		finalPoints = append(finalPoints, edge.points...)
		finalPoints = append(finalPoints, endPoint)
		edge.points = finalPoints
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
