package posit

// Stub implementations for Phase 6.
// These will be replaced with full implementations in later phases.

// routeEdges generates edge paths and restores reversed edges (Phase 6).
// Stub: straight lines between node centers.
func (s *layoutState) routeEdges() {
	for key, edge := range s.edges {
		fromNode := s.nodes[key.from]
		toNode := s.nodes[key.to]

		if fromNode == nil || toNode == nil {
			continue
		}

		// Simple straight line from source center to target center
		edge.points = []EdgePoint{
			{X: fromNode.x + fromNode.width/2, Y: fromNode.y + fromNode.height/2},
			{X: toNode.x + toNode.width/2, Y: toNode.y + toNode.height/2},
		}
	}

	// Restore reversed edges to original direction
	s.undoReversals()
}

// undoReversals restores all reversed edges to original direction.
func (s *layoutState) undoReversals() {
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
}
