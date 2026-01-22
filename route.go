package posit

import "math"

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
		for {
			node := s.nodes[current]
			if node == nil || !node.isDummy {
				// Current is the target node (not a dummy)
				targetID = current
				break
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

// initializeShortEdges ensures edges without dummies have points arrays.
func (s *layoutState) initializeShortEdges() {
	for _, edge := range s.edges {
		if edge.points == nil {
			edge.points = make([]EdgePoint, 0)
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
