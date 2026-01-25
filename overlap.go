package posit

import "sort"

// crossLayerOverlap represents two nodes in adjacent layers whose X ranges
// intersect and whose vertical gap is less than the required spacing.
type crossLayerOverlap struct {
	upper    *layoutNode // Node in upper layer (smaller Y)
	lower    *layoutNode // Node in lower layer (larger Y)
	xOverlap float64     // Amount of X-range overlap
	yGap     float64     // Current vertical gap between boundaries
	required float64     // Required gap (NodeNodeBetweenLayers)
}

// overlapContext holds pre-computed data for efficient overlap resolution.
// Built once per resolution pass to avoid O(n²×m) edge lookups.
type overlapContext struct {
	directEdges map[[2]string]bool // O(1) edge lookup
	connCounts  map[string]int     // O(1) connection count lookup
}

// resolveCrossLayerOverlaps adjusts node positions to ensure minimum
// spacing between node boundaries in adjacent layers.
func (s *layoutState) resolveCrossLayerOverlaps() {
	if s.opts.NodeNodeBetweenLayers <= 0 {
		return // Feature disabled
	}

	// Pre-compute edge lookups for O(1) access
	ctx := s.buildOverlapContext()

	// Iterate until stable or max iterations (handles cascading shifts).
	// Most graphs converge in 1-2 iterations; 10 is a safety limit.
	for iteration := 0; iteration < 10; iteration++ {
		overlaps := s.findAllCrossLayerOverlaps(ctx)
		if len(overlaps) == 0 {
			break
		}

		resolved := false
		for _, o := range overlaps {
			if s.resolveOverlap(o, ctx) {
				resolved = true
			}
		}

		if !resolved {
			break // No progress, stop iterating
		}
	}
}

// buildOverlapContext pre-computes edge and connection data for efficient lookup.
func (s *layoutState) buildOverlapContext() *overlapContext {
	ctx := &overlapContext{
		directEdges: make(map[[2]string]bool),
		connCounts:  make(map[string]int),
	}

	for _, edge := range s.edges {
		// Store both directions for O(1) lookup
		ctx.directEdges[[2]string{edge.key.from, edge.key.to}] = true
		ctx.directEdges[[2]string{edge.key.to, edge.key.from}] = true

		// Count connections per node
		ctx.connCounts[edge.key.from]++
		ctx.connCounts[edge.key.to]++
	}

	return ctx
}

// findAllCrossLayerOverlaps finds all node pairs in adjacent layers that
// have overlapping X ranges and insufficient vertical spacing.
func (s *layoutState) findAllCrossLayerOverlaps(ctx *overlapContext) []crossLayerOverlap {
	var overlaps []crossLayerOverlap

	// Group non-dummy, non-cluster nodes by rank
	rankNodes := make(map[int][]*layoutNode)
	for _, node := range s.nodes {
		if node.isDummy {
			continue // Skip dummy nodes (edge routing points)
		}
		if _, isCluster := s.clusters[node.id]; isCluster {
			continue // Skip clusters (resized later by adjustClusters)
		}
		rankNodes[node.rank] = append(rankNodes[node.rank], node)
	}

	// Get sorted list of ranks (handles non-consecutive ranks)
	ranks := make([]int, 0, len(rankNodes))
	for r := range rankNodes {
		ranks = append(ranks, r)
	}
	sort.Ints(ranks)

	// Check each pair of adjacent ranks
	for i := 0; i < len(ranks)-1; i++ {
		upperRank := ranks[i]
		lowerRank := ranks[i+1]
		upperLayer := rankNodes[upperRank]
		lowerLayer := rankNodes[lowerRank]

		for _, upper := range upperLayer {
			for _, lower := range lowerLayer {
				if o := s.checkCrossLayerOverlap(upper, lower, ctx); o != nil {
					overlaps = append(overlaps, *o)
				}
			}
		}
	}

	return overlaps
}

// checkCrossLayerOverlap checks if two nodes have an overlap that needs resolution.
// Returns nil if no overlap or if it's a direct edge (handled by routing).
func (s *layoutState) checkCrossLayerOverlap(upper, lower *layoutNode, ctx *overlapContext) *crossLayerOverlap {
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
	// (edge routing will handle the visual connection)
	if ctx.directEdges[[2]string{upper.id, lower.id}] {
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

// resolveOverlap attempts to resolve a cross-layer overlap.
// Returns true if progress was made.
func (s *layoutState) resolveOverlap(o crossLayerOverlap, ctx *overlapContext) bool {
	// Strategy 1: Try horizontal shift (preferred, local impact)
	if s.resolveByHorizontalShift(o, ctx) {
		return true
	}

	// Strategy 2: Increase layer gap (fallback, affects more nodes)
	s.resolveByLayerGap(o)
	return true
}

// resolveByHorizontalShift tries to shift one node horizontally to eliminate
// the X overlap. Prefers shifting the node with fewer connections.
func (s *layoutState) resolveByHorizontalShift(o crossLayerOverlap, ctx *overlapContext) bool {
	// Calculate minimum shift needed to eliminate X overlap
	minShift := o.xOverlap + 1 // +1 for clearance

	// Prefer shifting the node with fewer connections (less impact on layout)
	upperConns := ctx.connCounts[o.upper.id]
	lowerConns := ctx.connCounts[o.lower.id]

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

// tryShiftNode attempts to shift a node horizontally by at least minShift.
// Returns true if successful.
func (s *layoutState) tryShiftNode(n *layoutNode, minShift float64) bool {
	// Get same-layer neighbors (non-dummy, non-cluster)
	var leftNeighbor, rightNeighbor *layoutNode
	for _, other := range s.nodes {
		if other.isDummy || other.rank != n.rank || other.id == n.id {
			continue
		}
		if _, isCluster := s.clusters[other.id]; isCluster {
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
