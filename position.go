package posit

import (
	"math"
	"sort"
	"sync"
)

// maxBlockIterations is a safeguard against infinite loops in placeBlock.
// This should never be reached in practice, but prevents hangs if the
// align map were corrupted.
const maxBlockIterations = 10000

// assignCoordinates computes X and Y positions for all nodes.
func (s *layoutState) assignCoordinates() {
	s.assignYCoordinates()

	// Choose X assignment method based on graph size.
	// BKThreshold controls when to switch from Brandes-Köpf (optimal)
	// to simple centering (faster for large graphs).
	threshold := s.opts.BKThreshold
	if threshold <= 0 {
		threshold = 100 // fallback default
	}
	if len(s.nodes) > threshold {
		s.assignXCoordinatesSimple() // Fast for large graphs
	} else {
		s.assignXCoordinatesBK() // Optimal for smaller graphs
	}
}

// assignYCoordinates assigns Y based on rank and RankSep.
func (s *layoutState) assignYCoordinates() {
	y := 0.0

	for rank := 0; rank < len(s.layers); rank++ {
		layer := s.layers[rank]
		if len(layer) == 0 {
			continue
		}

		// Find max height in this layer
		maxHeight := 0.0
		for _, id := range layer {
			node := s.nodes[id]
			if node.height > maxHeight {
				maxHeight = node.height
			}
		}

		// Assign Y to top of node
		for _, id := range layer {
			s.nodes[id].y = y
		}

		// Move to next layer
		if rank < len(s.layers)-1 {
			y += maxHeight + s.opts.RankSep
		}
	}
}

// assignXCoordinatesSimple uses simple left-to-right placement with centering.
func (s *layoutState) assignXCoordinatesSimple() {
	// Pass 1: Simple left-to-right placement
	for _, layer := range s.layers {
		x := 0.0
		for _, id := range layer {
			node := s.nodes[id]
			node.x = x
			x += node.width + s.opts.NodeSep
		}
	}

	// Pass 2: Center layers
	s.centerLayers()
}

// centerLayers adjusts X so all layers are centered.
func (s *layoutState) centerLayers() {
	// Find max layer width
	maxWidth := 0.0
	for _, layer := range s.layers {
		if len(layer) == 0 {
			continue
		}
		lastID := layer[len(layer)-1]
		lastNode := s.nodes[lastID]
		width := lastNode.x + lastNode.width
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Center each layer
	for _, layer := range s.layers {
		if len(layer) == 0 {
			continue
		}
		lastID := layer[len(layer)-1]
		lastNode := s.nodes[lastID]
		layerWidth := lastNode.x + lastNode.width
		offset := (maxWidth - layerWidth) / 2

		for _, id := range layer {
			s.nodes[id].x += offset
		}
	}
}

// conflictKey identifies a pair of nodes that conflict for alignment.
type conflictKey struct {
	v, w string
}

// assignXCoordinatesBK uses the Brandes-Kopf algorithm with optional
// iterative stacking prevention. When SpreadStackedNodes is enabled,
// it detects stacked pairs after the initial pass and adds separation
// constraints, then re-runs BK until no new stacking is found.
func (s *layoutState) assignXCoordinatesBK() {
	if len(s.layers) == 0 {
		return
	}

	// Maximum iterations for stacking prevention (prevents infinite loops)
	maxIterations := 3
	if !s.opts.SpreadStackedNodes {
		maxIterations = 1 // Single pass when feature is disabled
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		s.runBKPass()

		// Skip constraint detection on final iteration or if feature disabled
		if !s.opts.SpreadStackedNodes || iteration == maxIterations-1 {
			break
		}

		// Detect stacking and generate constraints
		newConstraints := s.detectStackingConstraints()
		if len(newConstraints) == 0 {
			break // No stacking found, converged
		}

		// Add new constraints (avoid duplicates)
		for _, c := range newConstraints {
			if !s.hasConstraint(c) {
				s.separationConstraints = append(s.separationConstraints, c)
			}
		}
	}
}

// runBKPass executes one pass of the Brandes-Köpf algorithm.
func (s *layoutState) runBKPass() {
	// Find type-1 conflicts (shared across all 4 passes)
	conflicts := s.findType1Conflicts()

	// Precompute layerings for each direction
	forwardLayers := s.layers
	reversedLayers := s.reverseLayers()

	// Define the 4 alignment pass configurations
	type bkPass struct {
		key      string
		layering [][]string
		horiz    string
		neighbor func(string) []string
	}
	passes := [4]bkPass{
		{"ul", forwardLayers, "l", s.getPredecessors},
		{"ur", s.reverseLayerOrders(forwardLayers), "r", s.getPredecessors},
		{"dl", reversedLayers, "l", s.getSuccessors},
		{"dr", s.reverseLayerOrders(reversedLayers), "r", s.getSuccessors},
	}

	// Run all 4 alignment passes in parallel
	var results [4]map[string]float64
	var wg sync.WaitGroup
	wg.Add(4)
	for i := range passes {
		go func(idx int) {
			defer wg.Done()
			p := passes[idx]
			root, align := s.verticalAlignment(p.layering, conflicts, p.neighbor)
			xs := s.horizontalCompaction(p.layering, root, align)
			if p.horiz == "r" {
				for id := range xs {
					xs[id] = -xs[id]
				}
			}
			results[idx] = xs
		}(i)
	}
	wg.Wait()

	xss := make(map[string]map[string]float64, 4)
	for i, p := range passes {
		xss[p.key] = results[i]
	}

	// Align to smallest width
	s.alignCoordinatesToSmallest(xss)

	// Take median of four alignments
	for id, node := range s.nodes {
		values := []float64{
			xss["ul"][id],
			xss["ur"][id],
			xss["dl"][id],
			xss["dr"][id],
		}
		sort.Float64s(values)
		// Median of 4 = average of middle 2
		node.x = (values[1] + values[2]) / 2
	}

	// Shift all X coordinates so minimum is 0 (non-negative coordinates)
	minX := math.Inf(1)
	for _, node := range s.nodes {
		if node.x < minX {
			minX = node.x
		}
	}
	if minX != 0 && !math.IsInf(minX, 0) {
		for _, node := range s.nodes {
			node.x -= minX
		}
	}
}

// detectStackingConstraints finds stacked node pairs and returns separation
// constraints that should be added to prevent stacking.
//
// Strategy: For each "convergence point" (node receiving edges from multiple
// same-layer sources), if those sources have overlapping horizontal bounds,
// add a constraint to spread them apart. Similarly for "divergence points".
//
// We check horizontal bound overlap (not center distance) because edges connect
// at node boundaries (ports), so overlapping bounds cause edge crossings.
func (s *layoutState) detectStackingConstraints() []separationConstraint {
	var constraints []separationConstraint

	// Margin for "near overlap" - nodes that are close enough to cause issues
	margin := s.opts.StackingThreshold
	if margin <= 0 {
		margin = 10.0 // Small margin for near-misses
	}

	// Check each layer for convergence/divergence points
	for rank := 0; rank < len(s.layers)-1; rank++ {
		upperLayer := s.layers[rank]
		lowerLayer := s.layers[rank+1]

		// Find convergence points: nodes in lower layer receiving from multiple upper nodes
		for _, lowerID := range lowerLayer {
			lowerNode := s.nodes[lowerID]
			if lowerNode == nil || lowerNode.isDummy {
				continue
			}

			// Find upper nodes that connect to this lower node
			var sources []*layoutNode
			for _, upperID := range upperLayer {
				upperNode := s.nodes[upperID]
				if upperNode == nil || upperNode.isDummy {
					continue
				}
				if s.hasEdgeBetween(upperID, lowerID) {
					sources = append(sources, upperNode)
				}
			}

			// Check if any pair of sources has overlapping horizontal bounds
			for i := 0; i < len(sources); i++ {
				for j := i + 1; j < len(sources); j++ {
					a, b := sources[i], sources[j]

					if s.horizontalBoundsOverlap(a, b, margin) {
						// These sources overlap - need separation
						// Order by current position (left node first)
						left, right := a, b
						if a.x > b.x {
							left, right = b, a
						}
						// Required gap: ensure no overlap with margin
						requiredGap := margin + s.opts.NodeSep/2
						constraints = append(constraints, separationConstraint{
							leftID:  left.id,
							rightID: right.id,
							minGap:  requiredGap,
						})
					}
				}
			}
		}

		// Find divergence points: nodes in upper layer sending to multiple lower nodes
		for _, upperID := range upperLayer {
			upperNode := s.nodes[upperID]
			if upperNode == nil || upperNode.isDummy {
				continue
			}

			// Find lower nodes that this upper node connects to
			var targets []*layoutNode
			for _, lowerID := range lowerLayer {
				lowerNode := s.nodes[lowerID]
				if lowerNode == nil || lowerNode.isDummy {
					continue
				}
				if s.hasEdgeBetween(upperID, lowerID) {
					targets = append(targets, lowerNode)
				}
			}

			// Check if any pair of targets has overlapping horizontal bounds
			for i := 0; i < len(targets); i++ {
				for j := i + 1; j < len(targets); j++ {
					a, b := targets[i], targets[j]

					if s.horizontalBoundsOverlap(a, b, margin) {
						// These targets overlap - need separation
						left, right := a, b
						if a.x > b.x {
							left, right = b, a
						}
						requiredGap := margin + s.opts.NodeSep/2
						constraints = append(constraints, separationConstraint{
							leftID:  left.id,
							rightID: right.id,
							minGap:  requiredGap,
						})
					}
				}
			}
		}
	}

	return constraints
}

// horizontalBoundsOverlap checks if two nodes' horizontal bounds overlap
// (or are within margin of overlapping). This is the correct test for
// stacking because edges connect at node boundaries, not centers.
func (s *layoutState) horizontalBoundsOverlap(a, b *layoutNode, margin float64) bool {
	aLeft := a.x - margin
	aRight := a.x + a.width + margin
	bLeft := b.x - margin
	bRight := b.x + b.width + margin

	// Overlap if one starts before the other ends
	return aLeft < bRight && bLeft < aRight
}

// hasConstraint checks if an equivalent constraint already exists.
func (s *layoutState) hasConstraint(c separationConstraint) bool {
	for _, existing := range s.separationConstraints {
		if (existing.leftID == c.leftID && existing.rightID == c.rightID) ||
			(existing.leftID == c.rightID && existing.rightID == c.leftID) {
			return true
		}
	}
	return false
}


// getPredecessors returns predecessors as a slice.
func (s *layoutState) getPredecessors(id string) []string {
	return s.predecessors[id]
}

// getSuccessors returns successors as a slice.
func (s *layoutState) getSuccessors(id string) []string {
	return s.successors[id]
}

// reverseLayers returns layers in reverse order.
func (s *layoutState) reverseLayers() [][]string {
	result := make([][]string, len(s.layers))
	for i, layer := range s.layers {
		result[len(s.layers)-1-i] = layer
	}
	return result
}

// reverseLayerOrders returns layers with nodes in reverse order.
func (s *layoutState) reverseLayerOrders(layers [][]string) [][]string {
	result := make([][]string, len(layers))
	for i, layer := range layers {
		result[i] = make([]string, len(layer))
		for j, id := range layer {
			result[i][len(layer)-1-j] = id
		}
	}
	return result
}

// findType1Conflicts identifies conflicts where aligning would cross inner segments.
// Adjacent-layer detection is sufficient because after normalization all edges span
// exactly one layer — an edge between layers k-1 and k can only cross inner segments
// at that same layer pair. This matches the Brandes-Köpf paper specification.
func (s *layoutState) findType1Conflicts() map[conflictKey]bool {
	conflicts := make(map[conflictKey]bool)

	// Precompute all inner segments (dummy→dummy edges) grouped by layer pair.
	// Each segment is stored as the order positions of its upper and lower dummy nodes.
	type innerSegment struct {
		upperPos int
		lowerPos int
	}
	segmentsByRank := make(map[int][]innerSegment) // key = lower layer rank

	for id, node := range s.nodes {
		if !node.isDummy {
			continue
		}
		for _, succID := range s.successors[id] {
			succNode := s.nodes[succID]
			if succNode == nil || !succNode.isDummy {
				continue
			}
			if succNode.rank == node.rank+1 {
				segmentsByRank[succNode.rank] = append(segmentsByRank[succNode.rank], innerSegment{
					upperPos: node.order,
					lowerPos: succNode.order,
				})
			}
		}
	}

	// For each layer pair, check if non-inner edges cross any inner segment
	for rank := 1; rank < len(s.layers); rank++ {
		segments := segmentsByRank[rank]
		if len(segments) == 0 {
			continue
		}

		layer := s.layers[rank]
		for _, v := range layer {
			vNode := s.nodes[v]
			if vNode == nil {
				continue
			}
			vPos := vNode.order

			for _, u := range s.predecessors[v] {
				uNode := s.nodes[u]
				if uNode == nil || uNode.rank != rank-1 {
					continue
				}
				uPos := uNode.order

				// Skip if this edge IS an inner segment (both endpoints are dummies)
				if uNode.isDummy && vNode.isDummy {
					continue
				}

				// Check against all inner segments at this layer pair
				for _, seg := range segments {
					if (uPos < seg.upperPos && vPos > seg.lowerPos) ||
						(uPos > seg.upperPos && vPos < seg.lowerPos) {
						conflicts[conflictKey{v, u}] = true
						conflicts[conflictKey{u, v}] = true
						break
					}
				}
			}
		}
	}

	return conflicts
}

// verticalAlignment creates blocks of vertically aligned nodes.
func (s *layoutState) verticalAlignment(
	layering [][]string,
	conflicts map[conflictKey]bool,
	neighborFn func(string) []string,
) (root map[string]string, align map[string]string) {
	root = make(map[string]string)
	align = make(map[string]string)

	// Initialize: each node is its own block
	for id := range s.nodes {
		root[id] = id
		align[id] = id
	}

	// Process layers
	for _, layer := range layering {
		prevIdx := -1

		for _, v := range layer {
			neighbors := neighborFn(v)
			if len(neighbors) == 0 {
				continue
			}

			// Sort neighbors by order
			sortedNeighbors := make([]string, len(neighbors))
			copy(sortedNeighbors, neighbors)
			sort.Slice(sortedNeighbors, func(i, j int) bool {
				return s.nodes[sortedNeighbors[i]].order < s.nodes[sortedNeighbors[j]].order
			})

			// Find median neighbor(s)
			mid := (len(sortedNeighbors) - 1) / 2
			endMid := mid
			if len(sortedNeighbors)%2 == 0 {
				endMid = mid + 1
			}

			for m := mid; m <= endMid && m < len(sortedNeighbors); m++ {
				if align[v] != v {
					continue // Already aligned
				}

				w := sortedNeighbors[m]
				wOrder := s.nodes[w].order

				// Check for conflict
				key := conflictKey{v, w}
				if conflicts[key] {
					continue
				}

				// Align if no crossing with previous alignments
				if prevIdx < wOrder {
					align[w] = v
					root[v] = root[w]
					align[v] = root[v]
					prevIdx = wOrder
				}
			}
		}
	}

	return root, align
}

// horizontalCompaction assigns X coordinates respecting alignment blocks.
func (s *layoutState) horizontalCompaction(
	layering [][]string,
	root, align map[string]string,
) map[string]float64 {
	xs := make(map[string]float64)
	sink := make(map[string]string)
	shiftMap := make(map[string]float64)

	// Build position lookup from layering
	nodeRank := make(map[string]int)
	nodeOrder := make(map[string]int)
	for rank, layer := range layering {
		for order, id := range layer {
			nodeRank[id] = rank
			nodeOrder[id] = order
		}
	}

	// Initialize
	for id := range s.nodes {
		sink[id] = id
		shiftMap[id] = math.Inf(1)
	}

	// Compute X for roots
	for _, layer := range layering {
		for _, v := range layer {
			if root[v] == v {
				s.placeBlock(v, xs, sink, shiftMap, root, align, layering, nodeRank, nodeOrder)
			}
		}
	}

	// Apply shifts
	for _, layer := range layering {
		for _, v := range layer {
			xs[v] = xs[root[v]]
			if sh := shiftMap[sink[root[v]]]; sh < math.Inf(1) {
				xs[v] += sh
			}
		}
	}

	return xs
}

// placeBlock positions a block of aligned nodes.
func (s *layoutState) placeBlock(
	v string,
	xs map[string]float64,
	sink map[string]string,
	shiftMap map[string]float64,
	root, align map[string]string,
	layering [][]string,
	nodeRank, nodeOrder map[string]int,
) {
	if _, ok := xs[v]; ok {
		return // Already placed
	}

	xs[v] = 0

	w := v
	iterations := 0
	for {
		if iterations >= maxBlockIterations {
			return // Prevent infinite loop from corrupted align map
		}
		iterations++

		// Find predecessor in same layer
		order := nodeOrder[w]
		if order > 0 {
			rank := nodeRank[w]
			if rank >= 0 && rank < len(layering) {
				layer := layering[rank]
				if order < len(layer) {
					pred := layer[order-1]
					predRoot := root[pred]

					s.placeBlock(predRoot, xs, sink, shiftMap, root, align, layering, nodeRank, nodeOrder)

					if sink[v] == v {
						sink[v] = sink[predRoot]
					}

					sep := s.separation(pred, w)
					if sink[v] != sink[predRoot] {
						shiftMap[sink[predRoot]] = math.Min(
							shiftMap[sink[predRoot]],
							xs[v]-xs[predRoot]-sep,
						)
					} else {
						xs[v] = math.Max(xs[v], xs[predRoot]+sep)
					}
				}
			}
		}

		w = align[w]
		if w == v {
			break
		}
	}
}

// separation calculates required horizontal gap between adjacent nodes.
func (s *layoutState) separation(leftID, rightID string) float64 {
	left := s.nodes[leftID]
	right := s.nodes[rightID]

	if left == nil || right == nil {
		return s.opts.NodeSep
	}

	// For dummy nodes, use smaller separation
	sep := s.opts.NodeSep
	if left.isDummy || right.isDummy {
		sep = s.opts.NodeSep / 2
	}

	// Add cluster padding when nodes are at a cluster boundary
	// (one inside a cluster, one outside or in a different cluster)
	if len(s.clusters) > 0 {
		leftParent := s.parents[leftID]
		rightParent := s.parents[rightID]
		if leftParent != rightParent {
			// Add padding from the cluster boundary
			if leftParent != "" {
				if padding, ok := s.clusters[leftParent]; ok {
					sep += padding
				}
			}
			if rightParent != "" {
				if padding, ok := s.clusters[rightParent]; ok {
					sep += padding
				}
			}
		}
	}

	// Check for auto-generated separation constraints (stacking prevention).
	// These are added during iterative coordinate assignment when stacked
	// pairs are detected.
	for _, c := range s.separationConstraints {
		if (c.leftID == leftID && c.rightID == rightID) ||
			(c.leftID == rightID && c.rightID == leftID) {
			if c.minGap > sep {
				sep = c.minGap
			}
		}
	}

	return left.width/2 + sep + right.width/2
}

// alignCoordinatesToSmallest finds the alignment with smallest width and
// aligns all others to it. This follows dagre's algorithm which computes
// actual width (max - min considering node widths) for each alignment.
func (s *layoutState) alignCoordinatesToSmallest(xss map[string]map[string]float64) {
	alignments := []string{"ul", "ur", "dl", "dr"}

	// Compute min, max, and width for each alignment
	type alignmentMetrics struct {
		minX  float64
		maxX  float64
		width float64
	}
	metrics := make(map[string]alignmentMetrics)

	for _, align := range alignments {
		xs := xss[align]
		minX := math.Inf(1)
		maxX := math.Inf(-1)

		for id, x := range xs {
			node := s.nodes[id]
			if node == nil {
				continue
			}
			// Consider node width when computing bounds
			left := x - node.width/2
			right := x + node.width/2

			if left < minX {
				minX = left
			}
			if right > maxX {
				maxX = right
			}
		}

		metrics[align] = alignmentMetrics{
			minX:  minX,
			maxX:  maxX,
			width: maxX - minX,
		}
	}

	// Find alignment with smallest width
	smallestAlign := alignments[0]
	smallestWidth := metrics[smallestAlign].width
	for _, align := range alignments[1:] {
		if metrics[align].width < smallestWidth {
			smallestWidth = metrics[align].width
			smallestAlign = align
		}
	}

	// Get the bounds of the smallest alignment
	smallestMin := metrics[smallestAlign].minX

	// Shift each alignment so its minimum matches the smallest alignment's minimum
	for _, align := range alignments {
		xs := xss[align]
		shift := smallestMin - metrics[align].minX
		for id := range xs {
			xs[id] += shift
		}
	}
}
