package posit

import (
	"math"
	"sort"
)

// spreadStackedNodes detects nodes that are nearly vertically aligned ("stacked")
// and spreads them horizontally to create unambiguous port-side selection.
//
// The problem: Coordinate assignment algorithms (Brandes-Köpf, Network Simplex)
// optimize for edge straightness, causing nodes to stack vertically. When a
// target node's center falls within a source node's horizontal bounds, port-side
// selection becomes ambiguous — small X differences flip the decision, causing
// chaotic edge crossings.
//
// The solution: Detect stacked configurations and spread nodes apart so that
// port-side selection is unambiguous (target center is clearly left or right
// of source bounds).
//
// This phase runs after coordinate assignment, before edge routing.
func (s *layoutState) spreadStackedNodes() {
	if !s.opts.SpreadStackedNodes {
		return // Feature disabled
	}

	threshold := s.opts.StackingThreshold
	if threshold <= 0 {
		threshold = s.averageNodeWidth() * 0.5
	}

	// Process each pair of adjacent layers.
	// We look for "convergence points" — nodes in the lower layer that receive
	// edges from multiple stacked nodes in the upper layer.
	for rank := 0; rank < len(s.layers)-1; rank++ {
		upperLayer := s.layers[rank]
		lowerLayer := s.layers[rank+1]

		// For each node in the lower layer, find stacked sources
		for _, lowerID := range lowerLayer {
			lowerNode := s.nodes[lowerID]
			if lowerNode == nil || lowerNode.isDummy {
				continue
			}
			if _, isCluster := s.clusters[lowerID]; isCluster {
				continue
			}

			stackedSources := s.findStackedSources(lowerNode, upperLayer, threshold)
			if len(stackedSources) > 1 {
				s.spreadNodes(stackedSources, lowerNode)
			}
		}

		// Also check for fan-out: a single upper node with multiple stacked targets
		for _, upperID := range upperLayer {
			upperNode := s.nodes[upperID]
			if upperNode == nil || upperNode.isDummy {
				continue
			}
			if _, isCluster := s.clusters[upperID]; isCluster {
				continue
			}

			stackedTargets := s.findStackedTargets(upperNode, lowerLayer, threshold)
			if len(stackedTargets) > 1 {
				s.spreadTargets(upperNode, stackedTargets)
			}
		}

		// Nudge stacked pairs: when exactly one source connects to one target
		// and they're stacked, nudge them apart for clean edge routing.
		// This handles cases like Document Types → Payment Workflows with many
		// edges between the same two stacked nodes.
		s.nudgeStackedPairs(upperLayer, lowerLayer, threshold)
	}

	// Final pass: check ALL real node pairs connected by edges, even across
	// multiple layers. Long edges have dummy nodes in between, so the adjacent
	// layer check above misses them.
	s.nudgeAllStackedEdgePairs(threshold)
}

// nudgeStackedPairs finds pairs of connected nodes that are stacked and nudges
// one of them horizontally so edges can route cleanly on one side.
func (s *layoutState) nudgeStackedPairs(upperLayer, lowerLayer []string, threshold float64) {
	// Track which nodes we've already nudged to avoid double-processing
	nudged := make(map[string]bool)

	for _, upperID := range upperLayer {
		if nudged[upperID] {
			continue
		}
		upperNode := s.nodes[upperID]
		if upperNode == nil || upperNode.isDummy {
			continue
		}
		if _, isCluster := s.clusters[upperID]; isCluster {
			continue
		}

		for _, lowerID := range lowerLayer {
			if nudged[lowerID] {
				continue
			}
			lowerNode := s.nodes[lowerID]
			if lowerNode == nil || lowerNode.isDummy {
				continue
			}
			if _, isCluster := s.clusters[lowerID]; isCluster {
				continue
			}

			// Check if there's an edge between them
			hasEdge := s.hasEdgeBetween(upperID, lowerID)
			if !hasEdge {
				continue
			}

			// Check if they're stacked (centers within threshold)
			upperCenterX := upperNode.x + upperNode.width/2
			lowerCenterX := lowerNode.x + lowerNode.width/2
			xDiff := math.Abs(upperCenterX - lowerCenterX)

			if xDiff >= threshold {
				continue // Not stacked
			}

			// They're stacked - nudge the upper node so its center is outside
			// the lower node's bounds (or vice versa, pick the smaller shift)
			s.nudgeApart(upperNode, lowerNode)
			nudged[upperID] = true
			nudged[lowerID] = true
		}
	}
}

// nudgeApart moves two stacked nodes apart so one's center is outside the other's bounds.
// Nudges the node that requires the smaller shift.
func (s *layoutState) nudgeApart(upper, lower *layoutNode) {
	// Calculate how much to shift to get upper's center outside lower's bounds
	// or lower's center outside upper's bounds
	margin := 15.0 // Small margin for clarity
	upperCenterX := upper.x + upper.width/2
	lowerCenterX := lower.x + lower.width/2

	// Option 1: Move upper so its center is left of lower's left edge
	shiftUpperLeft := upperCenterX - (lower.x - margin)
	// Option 2: Move upper so its center is right of lower's right edge
	shiftUpperRight := (lower.x + lower.width + margin) - upperCenterX
	// Option 3: Move lower so its center is left of upper's left edge
	shiftLowerLeft := lowerCenterX - (upper.x - margin)
	// Option 4: Move lower so its center is right of upper's right edge
	shiftLowerRight := (upper.x + upper.width + margin) - lowerCenterX

	// Find the minimum positive shift
	type shiftOption struct {
		node      *layoutNode
		amount    float64
		direction int // -1 for left, +1 for right
		name      string
	}
	options := []shiftOption{
		{upper, shiftUpperLeft, -1, "upper-left"},
		{upper, shiftUpperRight, +1, "upper-right"},
		{lower, shiftLowerLeft, -1, "lower-left"},
		{lower, shiftLowerRight, +1, "lower-right"},
	}

	var best *shiftOption
	for i := range options {
		opt := &options[i]
		if opt.amount > 0 && (best == nil || opt.amount < best.amount) {
			best = opt
		}
	}

	if best == nil || best.amount <= 0 {
		return // Already not stacked
	}

	// Apply the shift using cascade to push neighbors if needed
	if best.direction < 0 {
		s.cascadeShiftLeft(best.node, best.amount)
	} else {
		s.cascadeShiftRight(best.node, best.amount)
	}
}

// averageNodeWidth computes the average width of non-dummy nodes.
func (s *layoutState) averageNodeWidth() float64 {
	var sum float64
	var count int
	for _, node := range s.nodes {
		if node.isDummy {
			continue
		}
		if _, isCluster := s.clusters[node.id]; isCluster {
			continue
		}
		sum += node.width
		count++
	}
	if count == 0 {
		return 100 // fallback default
	}
	return sum / float64(count)
}

// findStackedSources returns upper-layer nodes that:
// 1. Have edges to lowerNode
// 2. Are within 'threshold' X distance of lowerNode's center
func (s *layoutState) findStackedSources(lowerNode *layoutNode, upperLayer []string, threshold float64) []*layoutNode {
	var stacked []*layoutNode
	lowerCenterX := lowerNode.x + lowerNode.width/2

	for _, upperID := range upperLayer {
		upperNode := s.nodes[upperID]
		if upperNode == nil || upperNode.isDummy {
			continue
		}
		if _, isCluster := s.clusters[upperID]; isCluster {
			continue
		}

		// Check if there's an edge from upperNode to lowerNode
		if !s.hasEdgeBetween(upperNode.id, lowerNode.id) {
			continue
		}

		upperCenterX := upperNode.x + upperNode.width/2
		xDiff := math.Abs(upperCenterX - lowerCenterX)

		// Node is "stacked" if its center is within threshold of lower node's center
		if xDiff < threshold {
			stacked = append(stacked, upperNode)
		}
	}

	return stacked
}

// findStackedTargets returns lower-layer nodes that:
// 1. Have edges from upperNode
// 2. Are within 'threshold' X distance of upperNode's center
func (s *layoutState) findStackedTargets(upperNode *layoutNode, lowerLayer []string, threshold float64) []*layoutNode {
	var stacked []*layoutNode
	upperCenterX := upperNode.x + upperNode.width/2

	for _, lowerID := range lowerLayer {
		lowerNode := s.nodes[lowerID]
		if lowerNode == nil || lowerNode.isDummy {
			continue
		}
		if _, isCluster := s.clusters[lowerID]; isCluster {
			continue
		}

		// Check if there's an edge from upperNode to lowerNode
		if !s.hasEdgeBetween(upperNode.id, lowerNode.id) {
			continue
		}

		lowerCenterX := lowerNode.x + lowerNode.width/2
		xDiff := math.Abs(upperCenterX - lowerCenterX)

		// Node is "stacked" if its center is within threshold of upper node's center
		if xDiff < threshold {
			stacked = append(stacked, lowerNode)
		}
	}

	return stacked
}

// hasEdgeBetween checks if there's an edge between two nodes (in either direction).
// This includes both short edges (in s.edges) and long edges (replaced with dummy chains).
func (s *layoutState) hasEdgeBetween(a, b string) bool {
	// Check short edges in s.edges
	for key := range s.edges {
		if (key.from == a && key.to == b) || (key.from == b && key.to == a) {
			return true
		}
	}
	// Check long edges via dummy chains
	for _, firstDummy := range s.dummyChains {
		dummy := s.nodes[firstDummy]
		if dummy == nil || dummy.edgeLabel == nil {
			continue
		}
		origKey := dummy.edgeLabel.key
		if (origKey.from == a && origKey.to == b) || (origKey.from == b && origKey.to == a) {
			return true
		}
	}
	return false
}

// countOutgoingEdges counts edges from upperNode to nodes in lowerLayer.
func (s *layoutState) countOutgoingEdges(upperNode *layoutNode, lowerLayer []string) int {
	count := 0
	for _, lowerID := range lowerLayer {
		if s.hasEdgeBetween(upperNode.id, lowerID) {
			count++
		}
	}
	return count
}

// spreadNodes moves stacked source nodes apart so they're no longer ambiguous
// relative to the target node.
func (s *layoutState) spreadNodes(stacked []*layoutNode, target *layoutNode) {
	if len(stacked) < 2 {
		return
	}

	// Sort by current X position (left to right)
	sort.Slice(stacked, func(i, j int) bool {
		return stacked[i].x < stacked[j].x
	})

	// Calculate target's bounds
	targetLeftEdge := target.x
	targetRightEdge := target.x + target.width

	// Strategy: Move nodes so their CENTER is clearly outside target's bounds.
	// This ensures unambiguous port-side selection.
	// Split into left and right groups based on current relative position.
	var leftGroup, rightGroup []*layoutNode
	targetCenterX := target.x + target.width/2
	for _, node := range stacked {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX <= targetCenterX {
			leftGroup = append(leftGroup, node)
		} else {
			rightGroup = append(rightGroup, node)
		}
	}

	// If all nodes are on one side, split them in half
	if len(leftGroup) == 0 || len(rightGroup) == 0 {
		midpoint := len(stacked) / 2
		leftGroup = stacked[:midpoint]
		rightGroup = stacked[midpoint:]
	}

	// Small margin for clarity (port-side selection has some tolerance)
	margin := 10.0

	// Move left group: source CENTER should be left of target's LEFT edge
	// Process from INNERMOST to OUTERMOST so inner nodes push outer nodes
	// Sort leftGroup by X descending (rightmost/innermost first)
	sort.Slice(leftGroup, func(i, j int) bool {
		return leftGroup[i].x > leftGroup[j].x
	})

	limit := targetLeftEdge - margin
	for _, node := range leftGroup {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX > limit {
			shift := nodeCenterX - limit
			s.cascadeShiftLeft(node, shift)
		}
		// Update limit for next node - it needs to be left of this one
		limit = node.x - margin
	}

	// Move right group: source CENTER should be right of target's RIGHT edge
	// Process from INNERMOST to OUTERMOST (leftmost/innermost first)
	sort.Slice(rightGroup, func(i, j int) bool {
		return rightGroup[i].x < rightGroup[j].x
	})

	limit = targetRightEdge + margin
	for _, node := range rightGroup {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX < limit {
			shift := limit - nodeCenterX
			s.cascadeShiftRight(node, shift)
		}
		// Update limit for next node - it needs to be right of this one
		limit = node.x + node.width + margin
	}

}

// spreadTargets spreads target nodes that are stacked under a single source.
func (s *layoutState) spreadTargets(source *layoutNode, targets []*layoutNode) {
	if len(targets) < 2 {
		return
	}

	// Sort by current X position (left to right)
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].x < targets[j].x
	})

	// Calculate source's bounds
	sourceLeftEdge := source.x
	sourceRightEdge := source.x + source.width

	// Strategy: Move nodes so their CENTER is clearly outside source's bounds.
	var leftGroup, rightGroup []*layoutNode
	sourceCenterX := source.x + source.width/2
	for _, node := range targets {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX <= sourceCenterX {
			leftGroup = append(leftGroup, node)
		} else {
			rightGroup = append(rightGroup, node)
		}
	}

	// If all nodes are on one side, split them in half
	if len(leftGroup) == 0 || len(rightGroup) == 0 {
		midpoint := len(targets) / 2
		leftGroup = targets[:midpoint]
		rightGroup = targets[midpoint:]
	}

	// Small margin for clarity
	margin := 10.0

	// Move left group: target CENTER should be left of source's LEFT edge
	// Process from INNERMOST to OUTERMOST (rightmost first)
	sort.Slice(leftGroup, func(i, j int) bool {
		return leftGroup[i].x > leftGroup[j].x
	})

	limit := sourceLeftEdge - margin
	for _, node := range leftGroup {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX > limit {
			shift := nodeCenterX - limit
			s.cascadeShiftLeft(node, shift)
		}
		limit = node.x - margin
	}

	// Move right group: target CENTER should be right of source's RIGHT edge
	// Process from INNERMOST to OUTERMOST (leftmost first)
	sort.Slice(rightGroup, func(i, j int) bool {
		return rightGroup[i].x < rightGroup[j].x
	})

	limit = sourceRightEdge + margin
	for _, node := range rightGroup {
		nodeCenterX := node.x + node.width/2
		if nodeCenterX < limit {
			shift := limit - nodeCenterX
			s.cascadeShiftRight(node, shift)
		}
		limit = node.x + node.width + margin
	}
}

// cascadeShiftLeft shifts a node left, pushing any blocking neighbors recursively.
func (s *layoutState) cascadeShiftLeft(node *layoutNode, amount float64) {
	if amount <= 0 {
		return
	}

	// Find left neighbor in same layer
	var leftNeighbor *layoutNode
	for _, other := range s.nodes {
		if other.isDummy || other.rank != node.rank || other.id == node.id {
			continue
		}
		if _, isCluster := s.clusters[other.id]; isCluster {
			continue
		}
		if other.x+other.width <= node.x {
			if leftNeighbor == nil || other.x > leftNeighbor.x {
				leftNeighbor = other
			}
		}
	}

	nodeSep := s.opts.NodeSep
	if nodeSep <= 0 {
		nodeSep = 50
	}

	if leftNeighbor != nil {
		// Check if neighbor is blocking
		available := node.x - (leftNeighbor.x + leftNeighbor.width + nodeSep)
		if available < amount {
			// Push the neighbor first
			pushAmount := amount - available
			s.cascadeShiftLeft(leftNeighbor, pushAmount)
		}
	}

	// Now shift this node
	node.x -= amount
}

// cascadeShiftRight shifts a node right, pushing any blocking neighbors recursively.
func (s *layoutState) cascadeShiftRight(node *layoutNode, amount float64) {
	if amount <= 0 {
		return
	}

	// Find right neighbor in same layer
	var rightNeighbor *layoutNode
	for _, other := range s.nodes {
		if other.isDummy || other.rank != node.rank || other.id == node.id {
			continue
		}
		if _, isCluster := s.clusters[other.id]; isCluster {
			continue
		}
		if other.x >= node.x+node.width {
			if rightNeighbor == nil || other.x < rightNeighbor.x {
				rightNeighbor = other
			}
		}
	}

	nodeSep := s.opts.NodeSep
	if nodeSep <= 0 {
		nodeSep = 50
	}

	if rightNeighbor != nil {
		// Check if neighbor is blocking
		available := rightNeighbor.x - (node.x + node.width + nodeSep)
		if available < amount {
			// Push the neighbor first
			pushAmount := amount - available
			s.cascadeShiftRight(rightNeighbor, pushAmount)
		}
	}

	// Now shift this node
	node.x += amount
}

// nudgeAllStackedEdgePairs checks ALL edges in the graph for stacked real node
// pairs, regardless of layer distance. This catches long edges that span multiple
// layers (with dummy nodes in between) which the adjacent-layer check misses.
func (s *layoutState) nudgeAllStackedEdgePairs(threshold float64) {
	// Track which node pairs we've already processed
	type nodePair struct{ a, b string }
	processed := make(map[nodePair]bool)

	// Collect all real-to-real node pairs from short edges (in s.edges)
	type realPair struct {
		from, to string
	}
	var pairs []realPair

	for key := range s.edges {
		fromNode := s.nodes[key.from]
		toNode := s.nodes[key.to]

		// Skip dummy nodes - we only want short edges between real nodes
		if fromNode == nil || toNode == nil {
			continue
		}
		if fromNode.isDummy || toNode.isDummy {
			continue
		}
		if _, isCluster := s.clusters[key.from]; isCluster {
			continue
		}
		if _, isCluster := s.clusters[key.to]; isCluster {
			continue
		}

		pairs = append(pairs, realPair{key.from, key.to})
	}

	// Also collect real-to-real pairs from dummy chains (long edges)
	// During Phase 5b, long edges have been replaced with dummy chains,
	// so they don't appear in s.edges. But we can find them via dummyChains.
	for _, firstDummy := range s.dummyChains {
		dummy := s.nodes[firstDummy]
		if dummy == nil || dummy.edgeLabel == nil {
			continue
		}
		// edgeLabel.key has the original source and target
		origKey := dummy.edgeLabel.key
		pairs = append(pairs, realPair{origKey.from, origKey.to})
	}

	// Process all pairs
	for _, p := range pairs {
		fromNode := s.nodes[p.from]
		toNode := s.nodes[p.to]

		if fromNode == nil || toNode == nil {
			continue
		}

		// Normalize pair to avoid processing both directions
		pair := nodePair{p.from, p.to}
		if p.from > p.to {
			pair = nodePair{p.to, p.from}
		}
		if processed[pair] {
			continue
		}
		processed[pair] = true

		// Check if they're stacked
		fromCenterX := fromNode.x + fromNode.width/2
		toCenterX := toNode.x + toNode.width/2
		xDiff := math.Abs(fromCenterX - toCenterX)

		if xDiff >= threshold {
			continue // Not stacked
		}

		// They're stacked! Determine which is "upper" (lower rank = earlier layer)
		var upper, lower *layoutNode
		if fromNode.rank < toNode.rank {
			upper, lower = fromNode, toNode
		} else {
			upper, lower = toNode, fromNode
		}

		s.nudgeApart(upper, lower)
	}
}
