package posit

import (
	"math"
	"sort"
)

// assignCoordinates computes X and Y positions for all nodes.
func (s *layoutState) assignCoordinates() {
	s.assignYCoordinates()

	// Choose X assignment method based on graph size
	if len(s.nodes) > 100 {
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

// assignXCoordinatesBK uses the Brandes-Kopf algorithm.
func (s *layoutState) assignXCoordinatesBK() {
	if len(s.layers) == 0 {
		return
	}

	// Find type-1 conflicts
	conflicts := s.findType1Conflicts()

	// Compute four alignments
	xss := make(map[string]map[string]float64)

	for _, vert := range []string{"u", "d"} {
		layering := s.layers
		if vert == "d" {
			layering = s.reverseLayers()
		}

		for _, horiz := range []string{"l", "r"} {
			adjustedLayering := layering
			if horiz == "r" {
				adjustedLayering = s.reverseLayerOrders(layering)
			}

			neighborFn := s.getPredecessors
			if vert == "d" {
				neighborFn = s.getSuccessors
			}

			root, align := s.verticalAlignment(adjustedLayering, conflicts, neighborFn)
			xs := s.horizontalCompaction(adjustedLayering, root, align)

			if horiz == "r" {
				// Negate X values
				for id := range xs {
					xs[id] = -xs[id]
				}
			}

			xss[vert+horiz] = xs
		}
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
func (s *layoutState) findType1Conflicts() map[conflictKey]bool {
	conflicts := make(map[conflictKey]bool)

	for rank := 1; rank < len(s.layers); rank++ {
		prevLayer := s.layers[rank-1]
		layer := s.layers[rank]

		// Find inner segment ranges (segments between dummy nodes)
		innerSegments := s.findInnerSegments(prevLayer, layer)

		// Mark conflicts
		for _, seg := range innerSegments {
			// Any edge crossing this inner segment creates a conflict
			for _, v := range layer {
				for _, u := range s.predecessors[v] {
					uNode := s.nodes[u]
					if uNode == nil {
						continue
					}
					// Check if edge (u,v) crosses the inner segment
					if s.edgeCrossesSegment(u, v, seg, prevLayer, layer) {
						conflicts[conflictKey{v, u}] = true
						conflicts[conflictKey{u, v}] = true
					}
				}
			}
		}
	}

	return conflicts
}

// innerSegment represents an edge between two dummy nodes.
type innerSegment struct {
	upperPos int // position in upper layer
	lowerPos int // position in lower layer
}

// findInnerSegments finds edges between dummy nodes (inner segments).
func (s *layoutState) findInnerSegments(upperLayer, lowerLayer []string) []innerSegment {
	var segments []innerSegment

	// Build position maps
	upperPos := make(map[string]int)
	for i, id := range upperLayer {
		upperPos[id] = i
	}
	lowerPos := make(map[string]int)
	for i, id := range lowerLayer {
		lowerPos[id] = i
	}

	// Find inner segments
	for _, upperID := range upperLayer {
		upperNode := s.nodes[upperID]
		if upperNode == nil || !upperNode.isDummy {
			continue
		}

		for _, lowerID := range s.successors[upperID] {
			lowerNode := s.nodes[lowerID]
			if lowerNode == nil || !lowerNode.isDummy {
				continue
			}

			// Check if lower node is in this layer
			if pos, ok := lowerPos[lowerID]; ok {
				segments = append(segments, innerSegment{
					upperPos: upperPos[upperID],
					lowerPos: pos,
				})
			}
		}
	}

	return segments
}

// edgeCrossesSegment checks if an edge would cross an inner segment.
func (s *layoutState) edgeCrossesSegment(u, v string, seg innerSegment, upperLayer, lowerLayer []string) bool {
	// Get positions
	uPos := -1
	for i, id := range upperLayer {
		if id == u {
			uPos = i
			break
		}
	}
	vPos := -1
	for i, id := range lowerLayer {
		if id == v {
			vPos = i
			break
		}
	}

	if uPos == -1 || vPos == -1 {
		return false
	}

	// Check for crossing: edges cross if their positions are interleaved
	// Edge (uPos, vPos) crosses segment (seg.upperPos, seg.lowerPos) if:
	// (uPos < seg.upperPos && vPos > seg.lowerPos) || (uPos > seg.upperPos && vPos < seg.lowerPos)
	return (uPos < seg.upperPos && vPos > seg.lowerPos) ||
		(uPos > seg.upperPos && vPos < seg.lowerPos)
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
	for {
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

	return left.width/2 + sep + right.width/2
}

// alignCoordinatesToSmallest aligns all four coordinate sets to have the same min.
func (s *layoutState) alignCoordinatesToSmallest(xss map[string]map[string]float64) {
	// Find minimum X for each alignment
	alignments := []string{"ul", "ur", "dl", "dr"}
	mins := make(map[string]float64)

	for _, align := range alignments {
		xs := xss[align]
		minX := math.Inf(1)
		for _, x := range xs {
			if x < minX {
				minX = x
			}
		}
		mins[align] = minX
	}

	// Find overall minimum
	globalMin := math.Inf(1)
	for _, min := range mins {
		if min < globalMin {
			globalMin = min
		}
	}

	// Shift each alignment so its minimum equals the global minimum
	for _, align := range alignments {
		xs := xss[align]
		shift := globalMin - mins[align]
		for id := range xs {
			xs[id] += shift
		}
	}
}
