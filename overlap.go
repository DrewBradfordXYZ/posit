package posit

// crossLayerOverlap represents two nodes in adjacent layers whose X ranges
// intersect and whose vertical gap is less than the required spacing.
type crossLayerOverlap struct {
	upper    *layoutNode // Node in upper layer (smaller Y)
	lower    *layoutNode // Node in lower layer (larger Y)
	xOverlap float64     // Amount of X-range overlap
	yGap     float64     // Current vertical gap between boundaries
	required float64     // Required gap (NodeNodeBetweenLayers)
}

// resolveCrossLayerOverlaps adjusts node positions to ensure minimum
// spacing between node boundaries in adjacent layers.
func (s *layoutState) resolveCrossLayerOverlaps() {
	if s.opts.NodeNodeBetweenLayers <= 0 {
		return // Feature disabled
	}

	// Iterate until stable or max iterations (to handle cascading shifts)
	for iteration := 0; iteration < 10; iteration++ {
		overlaps := s.findAllCrossLayerOverlaps()
		if len(overlaps) == 0 {
			break
		}

		resolved := false
		for _, o := range overlaps {
			if s.resolveOverlap(o) {
				resolved = true
			}
		}

		if !resolved {
			break // No progress, stop iterating
		}
	}
}

// findAllCrossLayerOverlaps finds all node pairs in adjacent layers that
// have overlapping X ranges and insufficient vertical spacing.
func (s *layoutState) findAllCrossLayerOverlaps() []crossLayerOverlap {
	var overlaps []crossLayerOverlap

	// Group non-dummy nodes by rank (layer)
	rankNodes := make(map[int][]*layoutNode)
	maxRank := 0
	for _, node := range s.nodes {
		if node.isDummy {
			continue // Skip dummy nodes
		}
		rankNodes[node.rank] = append(rankNodes[node.rank], node)
		if node.rank > maxRank {
			maxRank = node.rank
		}
	}

	// Check each pair of adjacent layers
	for rank := 0; rank < maxRank; rank++ {
		upperLayer := rankNodes[rank]
		lowerLayer := rankNodes[rank+1]

		for _, upper := range upperLayer {
			for _, lower := range lowerLayer {
				if o := s.checkCrossLayerOverlap(upper, lower); o != nil {
					overlaps = append(overlaps, *o)
				}
			}
		}
	}

	return overlaps
}

// checkCrossLayerOverlap checks if two nodes have an overlap that needs resolution.
// Returns nil if no overlap or if it's a direct edge (handled by routing).
func (s *layoutState) checkCrossLayerOverlap(upper, lower *layoutNode) *crossLayerOverlap {
	// Check X-range intersection
	uLeft := upper.x
	uRight := upper.x + upper.width
	lLeft := lower.x
	lRight := lower.x + lower.width

	xOverlap := min(uRight, lRight) - max(uLeft, lLeft)
	if xOverlap <= 0 {
		return nil // No horizontal overlap
	}

	// Calculate current vertical gap (bottom of upper to top of lower)
	uBottom := upper.y + upper.height
	lTop := lower.y
	yGap := lTop - uBottom

	required := s.opts.NodeNodeBetweenLayers
	if yGap >= required {
		return nil // Sufficient spacing already
	}

	// Skip if there's a direct edge between these nodes
	// (the edge routing will handle the connection)
	if s.hasDirectEdge(upper.id, lower.id) {
		return nil
	}

	return &crossLayerOverlap{
		upper:    upper,
		lower:    lower,
		xOverlap: xOverlap,
		yGap:     yGap,
		required: required,
	}
}

// hasDirectEdge checks if there's a direct edge between two nodes.
func (s *layoutState) hasDirectEdge(a, b string) bool {
	for _, edge := range s.edges {
		if (edge.key.from == a && edge.key.to == b) ||
			(edge.key.from == b && edge.key.to == a) {
			return true
		}
	}
	return false
}

// resolveOverlap attempts to resolve a cross-layer overlap.
// Returns true if progress was made.
func (s *layoutState) resolveOverlap(o crossLayerOverlap) bool {
	// Strategy 1: Try horizontal shift (preferred, local impact)
	if s.resolveByHorizontalShift(o) {
		return true
	}

	// Strategy 2: Increase layer gap (fallback, affects more nodes)
	s.resolveByLayerGap(o)
	return true
}

// resolveByHorizontalShift tries to shift one node horizontally to eliminate
// the X overlap. Prefers shifting the node with fewer connections.
func (s *layoutState) resolveByHorizontalShift(o crossLayerOverlap) bool {
	// Calculate minimum shift needed to eliminate X overlap
	minShift := o.xOverlap + 1 // +1 for clearance

	// Try shifting the node with fewer connections first
	upperConns := s.countConnections(o.upper)
	lowerConns := s.countConnections(o.lower)

	if upperConns <= lowerConns {
		if s.tryShiftNode(o.upper, minShift) {
			return true
		}
		return s.tryShiftNode(o.lower, minShift)
	}
	if s.tryShiftNode(o.lower, minShift) {
		return true
	}
	return s.tryShiftNode(o.upper, minShift)
}

// countConnections returns the number of edges connected to a node.
func (s *layoutState) countConnections(n *layoutNode) int {
	count := 0
	for _, edge := range s.edges {
		if edge.key.from == n.id || edge.key.to == n.id {
			count++
		}
	}
	return count
}

// tryShiftNode attempts to shift a node horizontally by at least minShift.
// Returns true if successful.
func (s *layoutState) tryShiftNode(n *layoutNode, minShift float64) bool {
	// Get same-layer neighbors
	var leftNeighbor, rightNeighbor *layoutNode
	for _, other := range s.nodes {
		if other.isDummy || other.rank != n.rank || other.id == n.id {
			continue
		}
		if other.x+other.width <= n.x {
			// other is to the left
			if leftNeighbor == nil || other.x > leftNeighbor.x {
				leftNeighbor = other
			}
		}
		if other.x >= n.x+n.width {
			// other is to the right
			if rightNeighbor == nil || other.x < rightNeighbor.x {
				rightNeighbor = other
			}
		}
	}

	nodeSep := s.opts.NodeSep
	if nodeSep <= 0 {
		nodeSep = 50 // default
	}

	// Try shifting right
	if rightNeighbor != nil {
		availableRight := rightNeighbor.x - (n.x + n.width) - nodeSep
		if availableRight >= minShift {
			n.x += minShift
			return true
		}
	} else {
		// No right neighbor, can shift freely
		n.x += minShift
		return true
	}

	// Try shifting left
	if leftNeighbor != nil {
		availableLeft := n.x - (leftNeighbor.x + leftNeighbor.width) - nodeSep
		if availableLeft >= minShift {
			n.x -= minShift
			return true
		}
	} else {
		// No left neighbor, can shift freely
		n.x -= minShift
		return true
	}

	return false // Can't shift without violating same-layer constraints
}

// resolveByLayerGap increases the gap between layers to resolve the overlap.
func (s *layoutState) resolveByLayerGap(o crossLayerOverlap) {
	deficit := o.required - o.yGap
	if deficit <= 0 {
		return
	}

	// Shift all nodes in the lower layer (and below) down
	lowerRank := o.lower.rank
	for _, node := range s.nodes {
		if node.rank >= lowerRank {
			node.y += deficit
		}
	}
}
