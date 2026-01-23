package posit

import "sort"

// minimizeCrossings reorders nodes within layers to reduce edge crossings.
// Uses the barycenter heuristic with alternating up/down sweeps.
func (s *layoutState) minimizeCrossings() {
	if len(s.layers) <= 1 {
		// Nothing to optimize with 0 or 1 layers
		s.assignOrderFromLayers()
		return
	}

	// Run ordering in normal direction
	s.runOrderingPass()
	bestLayers := s.copyLayers()
	bestCrossings := s.countCrossings()

	// Optionally try reversed layer direction and keep the better result
	if s.opts.TryReverseOrdering && len(s.layers) > 2 {
		// Reverse layer order
		s.reverseLayerOrder()
		s.assignOrderFromLayers()

		// Re-run ordering from the reversed starting point
		s.runOrderingPass()
		revCrossings := s.countCrossings()

		if revCrossings < bestCrossings {
			// Reversed result is better — un-reverse the layers to restore correct rank order
			s.reverseLayerOrder()
			bestLayers = s.copyLayers()
		} else {
			// Original was better — restore it
			s.layers = bestLayers
		}
		s.assignOrderFromLayers()
	}

	// Ensure cluster children are adjacent within layers
	s.enforceClusterAdjacency()

	// Port-level crossing minimization pass
	s.minimizePortCrossings()
}

// reverseLayerOrder reverses the order of layers in-place.
func (s *layoutState) reverseLayerOrder() {
	for i, j := 0, len(s.layers)-1; i < j; i, j = i+1, j-1 {
		s.layers[i], s.layers[j] = s.layers[j], s.layers[i]
	}
}

// runOrderingPass executes one full ordering optimization (barycenter sweeps + adjacent exchange).
func (s *layoutState) runOrderingPass() {
	s.initOrder()

	bestLayers := s.copyLayers()
	bestCrossings := s.countCrossings()

	maxIterations := 24
	noImprovement := 0

	for i := 0; i < maxIterations && noImprovement < 4; i++ {
		if i%2 == 0 {
			s.sweepDown()
		} else {
			s.sweepUp()
		}

		s.adjacentExchange()

		crossings := s.countCrossings()
		if crossings < bestCrossings {
			bestCrossings = crossings
			bestLayers = s.copyLayers()
			noImprovement = 0
		} else {
			noImprovement++
		}
	}

	s.layers = bestLayers
	s.assignOrderFromLayers()
}

// initOrder assigns initial order using the current layer positions,
// then applies ordering constraints.
func (s *layoutState) initOrder() {
	s.applyOrderConstraints()
	s.assignOrderFromLayers()
}

// applyOrderConstraints reorders nodes within each layer to respect
// OrderGroup and OrderPriority constraints before crossing minimization.
func (s *layoutState) applyOrderConstraints() {
	for rank := range s.layers {
		layer := s.layers[rank]
		if len(layer) <= 1 {
			continue
		}

		// Check if any node has constraints
		hasConstraints := false
		for _, id := range layer {
			if s.nodes[id].orderGroup != "" {
				hasConstraints = true
				break
			}
		}
		if !hasConstraints {
			continue
		}

		// Sort by (OrderGroup, OrderPriority, insertOrder)
		sort.SliceStable(layer, func(i, j int) bool {
			ni := s.nodes[layer[i]]
			nj := s.nodes[layer[j]]

			gi := ni.orderGroup
			gj := nj.orderGroup

			if gi != "" || gj != "" {
				if gi != gj {
					if gi == "" {
						return false
					}
					if gj == "" {
						return true
					}
					return gi < gj
				}
				// Same group: sort by priority
				if ni.orderPriority != nj.orderPriority {
					return ni.orderPriority < nj.orderPriority
				}
			}
			return ni.insertOrder < nj.insertOrder
		})
	}
}

// assignOrderFromLayers sets node.order based on position in layer.
func (s *layoutState) assignOrderFromLayers() {
	for _, layer := range s.layers {
		for i, id := range layer {
			s.nodes[id].order = i
		}
	}
}

// restoreOrderFromBase sets node ordering within each layer based on X positions
// from a previous layout. This avoids re-running crossing minimization when only
// node dimensions have changed (topology is the same).
func (s *layoutState) restoreOrderFromBase(base *Layout) {
	// Build X position lookup from base layout
	baseX := make(map[string]float64, len(base.Nodes))
	for id, node := range base.Nodes {
		baseX[id] = node.X
	}

	// For dummy nodes, estimate X by averaging the X positions of their
	// edge endpoints (source and target of the original edge they represent)
	for id, node := range s.nodes {
		if !node.isDummy {
			continue
		}
		// Find connected real nodes to estimate position
		sum := 0.0
		count := 0
		for _, pred := range s.predecessors[id] {
			if x, ok := baseX[pred]; ok {
				sum += x
				count++
			}
		}
		for _, succ := range s.successors[id] {
			if x, ok := baseX[succ]; ok {
				sum += x
				count++
			}
		}
		if count > 0 {
			baseX[id] = sum / float64(count)
		}
	}

	// Sort each layer by base X position
	for _, layer := range s.layers {
		sort.SliceStable(layer, func(i, j int) bool {
			xi := baseX[layer[i]]
			xj := baseX[layer[j]]
			return xi < xj
		})
	}

	s.assignOrderFromLayers()
}

// copyLayers creates a deep copy of the current layer structure.
func (s *layoutState) copyLayers() [][]string {
	result := make([][]string, len(s.layers))
	for i, layer := range s.layers {
		result[i] = make([]string, len(layer))
		copy(result[i], layer)
	}
	return result
}

// sweepDown reorders each layer based on predecessors (top-to-bottom).
func (s *layoutState) sweepDown() {
	for rank := 1; rank < len(s.layers); rank++ {
		s.sortLayerByBarycenter(rank, func(id string) []string {
			return s.predecessors[id]
		})
	}
}

// sweepUp reorders each layer based on successors (bottom-to-top).
func (s *layoutState) sweepUp() {
	for rank := len(s.layers) - 2; rank >= 0; rank-- {
		s.sortLayerByBarycenter(rank, func(id string) []string {
			return s.successors[id]
		})
	}
}

// adjacentExchange uses Iterated Local Search with bidirectional layer sweeps
// to minimize crossings. Sweeps all layers forward then backward, allowing
// improvements in one layer to create opportunities in adjacent layers.
// Stops when a full forward+backward cycle produces no improvement.
func (s *layoutState) adjacentExchange() {
	const maxCycles = 1 // forward+backward sweep cycles; outer loop (24×) provides additional opportunities

	for cycle := 0; cycle < maxCycles; cycle++ {
		progress := false

		// Forward sweep
		for rank := 0; rank < len(s.layers); rank++ {
			if s.adjExchangeLayer(rank) {
				progress = true
			}
		}

		// Backward sweep
		for rank := len(s.layers) - 2; rank >= 0; rank-- {
			if s.adjExchangeLayer(rank) {
				progress = true
			}
		}

		if !progress {
			break
		}
	}
}

// adjExchangeLayer runs ILS on a single layer: greedy exchange until stable,
// then stochastic disturbance to escape local minima. Tracks the best ordering
// found and restores it at the end. Returns true if any improvement was made.
func (s *layoutState) adjExchangeLayer(rank int) bool {
	limit := s.opts.AdjacentExchangeLimit
	const maxPasses = 3
	const maxNoImprovement = 2

	layer := s.layers[rank]
	if len(layer) <= 1 {
		return false
	}
	if limit > 0 && len(layer) > limit {
		return false
	}

	// Build neighbor cache once — valid for all passes since neighbors
	// are in adjacent layers and their positions don't change.
	cache := s.buildNeighborCache(rank)

	bestOrder := make([]string, len(layer))
	copy(bestOrder, layer)
	bestCrossings := s.layerCrossings(rank)
	improved := false
	noImprovement := 0

	for pass := 0; pass < maxPasses && noImprovement < maxNoImprovement; pass++ {
		gained := s.greedyExchangeWithCache(rank, cache)

		if !gained {
			// Perturbation: randomly accept gain=0 swaps to explore new neighborhoods
			s.disturbLayerWithCache(rank, cache)
			gained = s.greedyExchangeWithCache(rank, cache)
		}

		// Track best solution found
		currentCrossings := s.layerCrossings(rank)
		if currentCrossings < bestCrossings {
			copy(bestOrder, layer)
			bestCrossings = currentCrossings
			noImprovement = 0
			improved = true
		} else {
			noImprovement++
		}
	}

	// Restore best ordering found
	copy(layer, bestOrder)
	for i, id := range layer {
		s.nodes[id].order = i
	}
	return improved
}

// neighborCache holds precomputed sorted neighbor positions for all nodes
// in a layer. Since neighbors are in adjacent layers, their positions don't
// change when we swap nodes within the current layer — so the cache stays
// valid for an entire exchange pass.
type neighborCache struct {
	preds map[string][]neighborPos
	succs map[string][]neighborPos
}

// buildNeighborCache precomputes sorted neighbor positions for all nodes in a layer.
func (s *layoutState) buildNeighborCache(rank int) *neighborCache {
	layer := s.layers[rank]
	cache := &neighborCache{
		preds: make(map[string][]neighborPos, len(layer)),
		succs: make(map[string][]neighborPos, len(layer)),
	}
	for _, id := range layer {
		if rank > 0 {
			cache.preds[id] = s.sortedNeighborPositions(id, true)
		}
		if rank < len(s.layers)-1 {
			cache.succs[id] = s.sortedNeighborPositions(id, false)
		}
	}
	return cache
}

// swapCrossingDeltaCached computes crossing delta using precomputed neighbor positions.
func (s *layoutState) swapCrossingDeltaCached(rank, i int, cache *neighborCache) float64 {
	u := s.layers[rank][i]
	v := s.layers[rank][i+1]

	delta := 0.0

	// Check crossings with layer above (predecessors)
	if uPreds, vPreds := cache.preds[u], cache.preds[v]; len(uPreds) > 0 && len(vPreds) > 0 {
		delta += countInversions(uPreds, vPreds) - countInversions(vPreds, uPreds)
	}

	// Check crossings with layer below (successors)
	if uSuccs, vSuccs := cache.succs[u], cache.succs[v]; len(uSuccs) > 0 && len(vSuccs) > 0 {
		delta += countInversions(uSuccs, vSuccs) - countInversions(vSuccs, uSuccs)
	}

	return delta
}

// greedyExchangeWithCache uses first-improvement with propagation: finds the
// first beneficial swap, then propagates left and right to exploit the new
// neighborhood. Repeats until no beneficial swaps remain or the maximum
// number of rounds is reached.
func (s *layoutState) greedyExchangeWithCache(rank int, cache *neighborCache) bool {
	layer := s.layers[rank]
	anyImproved := false
	maxRounds := len(layer)
	if maxRounds > 5 {
		maxRounds = 5
	}

	const maxPropagation = 5 // max positions to propagate in each direction

	for round := 0; round < maxRounds; round++ {
		found := false
		for i := 0; i < len(layer)-1; i++ {
			delta := s.swapCrossingDeltaCached(rank, i, cache)
			if delta > 0 {
				layer[i], layer[i+1] = layer[i+1], layer[i]
				s.nodes[layer[i]].order = i
				s.nodes[layer[i+1]].order = i + 1

				// Propagate left: the node that moved left may benefit from moving further
				leftBound := i - maxPropagation
				if leftBound < 0 {
					leftBound = 0
				}
				for j := i - 1; j >= leftBound; j-- {
					d := s.swapCrossingDeltaCached(rank, j, cache)
					if d <= 0 {
						break
					}
					layer[j], layer[j+1] = layer[j+1], layer[j]
					s.nodes[layer[j]].order = j
					s.nodes[layer[j+1]].order = j + 1
				}

				// Propagate right: the node that moved right may benefit from moving further
				rightBound := i + 1 + maxPropagation
				if rightBound > len(layer)-1 {
					rightBound = len(layer) - 1
				}
				for j := i + 1; j < rightBound; j++ {
					d := s.swapCrossingDeltaCached(rank, j, cache)
					if d <= 0 {
						break
					}
					layer[j], layer[j+1] = layer[j+1], layer[j]
					s.nodes[layer[j]].order = j
					s.nodes[layer[j+1]].order = j + 1
				}

				found = true
				anyImproved = true
				break // first-improvement: restart scan after propagation
			}
		}
		if !found {
			break
		}
	}
	return anyImproved
}

// disturbLayerWithCache randomly perturbs the ordering by accepting swaps with
// gain >= 0. Swaps with gain > 0 are always accepted; swaps with gain == 0
// are accepted with 50% probability. This is an O(n) perturbation that moves
// the solution to a new neighborhood without degrading quality significantly.
func (s *layoutState) disturbLayerWithCache(rank int, cache *neighborCache) {
	layer := s.layers[rank]
	for i := 0; i < len(layer)-1; i++ {
		delta := s.swapCrossingDeltaCached(rank, i, cache)
		if delta > 0 || (delta == 0 && s.rng.Intn(2) == 0) {
			layer[i], layer[i+1] = layer[i+1], layer[i]
			s.nodes[layer[i]].order = i
			s.nodes[layer[i+1]].order = i + 1
		}
	}
}

// neighborPos holds a neighbor's position and the weight of the edge to it.
type neighborPos struct {
	pos    int
	weight float64
}

// sortedNeighborPositions returns the positions and edge weights of a node's
// neighbors in an adjacent layer, sorted by position ascending.
func (s *layoutState) sortedNeighborPositions(nodeID string, usePredecessors bool) []neighborPos {
	var neighbors []string
	if usePredecessors {
		neighbors = s.predecessors[nodeID]
	} else {
		neighbors = s.successors[nodeID]
	}

	if len(neighbors) == 0 {
		return nil
	}

	positions := make([]neighborPos, 0, len(neighbors))
	for _, nID := range neighbors {
		n := s.nodes[nID]
		if n == nil {
			continue
		}
		w := s.edgeWeightBetween(nodeID, nID)
		positions = append(positions, neighborPos{pos: n.order, weight: w})
	}

	// Manual swaps for common small sizes (most nodes have 2-5 neighbors)
	switch len(positions) {
	case 0, 1:
		// Already sorted
	case 2:
		if positions[0].pos > positions[1].pos {
			positions[0], positions[1] = positions[1], positions[0]
		}
	case 3:
		if positions[0].pos > positions[1].pos {
			positions[0], positions[1] = positions[1], positions[0]
		}
		if positions[1].pos > positions[2].pos {
			positions[1], positions[2] = positions[2], positions[1]
			if positions[0].pos > positions[1].pos {
				positions[0], positions[1] = positions[1], positions[0]
			}
		}
	default:
		sort.Slice(positions, func(i, j int) bool {
			return positions[i].pos < positions[j].pos
		})
	}

	return positions
}

// countInversions counts weighted pairs where an element of `a` has a position
// greater than an element of `b`. Both slices must be sorted by pos ascending.
// This is O(len(a) + len(b)) via a merge scan.
func countInversions(a, b []neighborPos) float64 {
	count := 0.0
	j := 0
	bWeightSoFar := 0.0 // cumulative weight of b elements with pos < current a[i].pos

	for i := 0; i < len(a); i++ {
		// Advance j past all b elements with position < a[i].pos
		for j < len(b) && b[j].pos < a[i].pos {
			bWeightSoFar += b[j].weight
			j++
		}
		// Each b element with pos < a[i].pos is an inversion (a > b)
		count += a[i].weight * bWeightSoFar
	}
	return count
}

// edgeWeightBetween returns the weight of the edge between two nodes.
func (s *layoutState) edgeWeightBetween(a, b string) float64 {
	if edge := s.edges[edgeKey{from: a, to: b}]; edge != nil {
		if edge.weight >= 1 {
			return edge.weight
		}
		return 1
	}
	if edge := s.edges[edgeKey{from: b, to: a}]; edge != nil {
		if edge.weight >= 1 {
			return edge.weight
		}
		return 1
	}
	return 1
}

// layerCrossings counts crossings involving the given rank
// (between it and both adjacent layers).
func (s *layoutState) layerCrossings(rank int) float64 {
	total := 0.0
	if rank > 0 {
		total += s.twoLayerCrossCount(s.layers[rank-1], s.layers[rank])
	}
	if rank < len(s.layers)-1 {
		total += s.twoLayerCrossCount(s.layers[rank], s.layers[rank+1])
	}
	return total
}

// barycenterEntry holds barycenter data for sorting.
type barycenterEntry struct {
	nodeID     string
	barycenter float64
	hasValue   bool
}

// sortLayerByBarycenter sorts a layer based on neighbor barycenters,
// respecting ordering constraints (OrderGroup, OrderPriority).
func (s *layoutState) sortLayerByBarycenter(rank int, neighborFn func(string) []string) {
	layer := s.layers[rank]
	if len(layer) <= 1 {
		return
	}

	// Calculate barycenters
	entries := make([]barycenterEntry, len(layer))
	for i, nodeID := range layer {
		bc, hasValue := s.calculateBarycenter(nodeID, neighborFn)
		entries[i] = barycenterEntry{
			nodeID:     nodeID,
			barycenter: bc,
			hasValue:   hasValue,
		}
	}

	// Sort by: (OrderGroup, OrderPriority, barycenter)
	sort.SliceStable(entries, func(i, j int) bool {
		ni := s.nodes[entries[i].nodeID]
		nj := s.nodes[entries[j].nodeID]

		// If both have order groups, group takes precedence
		gi := ni.orderGroup
		gj := nj.orderGroup

		if gi != "" || gj != "" {
			if gi != gj {
				// Nodes in a group sort before ungrouped nodes
				if gi == "" {
					return false
				}
				if gj == "" {
					return true
				}
				return gi < gj
			}
			// Same group: sort by priority
			if ni.orderPriority != nj.orderPriority {
				return ni.orderPriority < nj.orderPriority
			}
		}

		// Fallback to barycenter
		if !entries[i].hasValue && !entries[j].hasValue {
			return false
		}
		if !entries[i].hasValue {
			return false
		}
		if !entries[j].hasValue {
			return true
		}
		return entries[i].barycenter < entries[j].barycenter
	})

	// Update layer and order values
	for i, entry := range entries {
		s.layers[rank][i] = entry.nodeID
		s.nodes[entry.nodeID].order = i
	}
}

// calculateBarycenter computes the weighted median position of neighbors.
// This is more robust than arithmetic mean (barycenter) because it resists
// outlier positions: a single far-away neighbor won't pull the node
// away from the majority of its connections.
func (s *layoutState) calculateBarycenter(nodeID string, neighborFn func(string) []string) (float64, bool) {
	neighbors := neighborFn(nodeID)
	if len(neighbors) == 0 {
		return 0, false
	}

	// Collect weighted positions
	type weightedPos struct {
		position float64
		weight   float64
	}
	positions := make([]weightedPos, 0, len(neighbors))
	totalWeight := 0.0

	for _, neighborID := range neighbors {
		neighbor := s.nodes[neighborID]
		if neighbor == nil {
			continue
		}

		// Get edge weight
		edgeWeight := 1.0
		if edge := s.edges[edgeKey{from: nodeID, to: neighborID}]; edge != nil {
			edgeWeight = edge.weight
		} else if edge := s.edges[edgeKey{from: neighborID, to: nodeID}]; edge != nil {
			edgeWeight = edge.weight
		}

		positions = append(positions, weightedPos{float64(neighbor.order), edgeWeight})
		totalWeight += edgeWeight
	}

	if totalWeight == 0 || len(positions) == 0 {
		return 0, false
	}

	// For single neighbor, just return its position
	if len(positions) == 1 {
		return positions[0].position, true
	}

	// Sort by position — use manual swaps for small slices (common case: 2-5 neighbors)
	switch len(positions) {
	case 2:
		if positions[0].position > positions[1].position {
			positions[0], positions[1] = positions[1], positions[0]
		}
	case 3:
		if positions[0].position > positions[1].position {
			positions[0], positions[1] = positions[1], positions[0]
		}
		if positions[1].position > positions[2].position {
			positions[1], positions[2] = positions[2], positions[1]
			if positions[0].position > positions[1].position {
				positions[0], positions[1] = positions[1], positions[0]
			}
		}
	default:
		sort.Slice(positions, func(i, j int) bool {
			return positions[i].position < positions[j].position
		})
	}

	// Find weighted median: the position where cumulative weight crosses half
	halfWeight := totalWeight / 2
	cumWeight := 0.0
	for i, wp := range positions {
		cumWeight += wp.weight
		if cumWeight >= halfWeight {
			// If we're exactly at the boundary, average with next position
			if cumWeight == halfWeight && i+1 < len(positions) {
				return (wp.position + positions[i+1].position) / 2, true
			}
			return wp.position, true
		}
	}

	return positions[len(positions)-1].position, true
}

// countCrossings counts total edge crossings in the current layout.
func (s *layoutState) countCrossings() float64 {
	total := 0.0
	for i := 1; i < len(s.layers); i++ {
		total += s.twoLayerCrossCount(s.layers[i-1], s.layers[i])
	}
	return total
}

// twoLayerCrossCount counts crossings between two adjacent layers.
// Uses an accumulator tree (similar to Fenwick tree) for O(E log V) performance.
// Returns float64 to preserve precision when edge weights are fractional.
// Reference: Barth, Mutzel, Junger. "Simple and Efficient Bilayer Cross Counting."
func (s *layoutState) twoLayerCrossCount(northLayer, southLayer []string) float64 {
	if len(northLayer) == 0 || len(southLayer) == 0 {
		return 0.0
	}

	// Use node.order directly instead of building a position map.
	// The order field is kept current by all swap/sort operations.
	southRank := s.nodes[southLayer[0]].rank

	// Collect south positions for all edges from north, ordered by north position
	type entry struct {
		pos    int
		weight float64
	}
	southEntries := make([]entry, 0, len(northLayer)*2)
	edges := make([]entry, 0, 8) // reusable buffer for per-node edges

	for _, v := range northLayer {
		edges = edges[:0]
		for _, w := range s.successors[v] {
			wNode := s.nodes[w]
			if wNode == nil || wNode.rank != southRank {
				continue // Edge to different layer
			}
			weight := 1.0
			if edge := s.edges[edgeKey{from: v, to: w}]; edge != nil {
				weight = edge.weight
				if weight < 1 {
					weight = 1
				}
			}
			edges = append(edges, entry{pos: wNode.order, weight: weight})
		}
		// Sort by position within this node's edges (manual for small sizes)
		switch len(edges) {
		case 0, 1:
			// Already sorted
		case 2:
			if edges[0].pos > edges[1].pos {
				edges[0], edges[1] = edges[1], edges[0]
			}
		case 3:
			if edges[0].pos > edges[1].pos {
				edges[0], edges[1] = edges[1], edges[0]
			}
			if edges[1].pos > edges[2].pos {
				edges[1], edges[2] = edges[2], edges[1]
				if edges[0].pos > edges[1].pos {
					edges[0], edges[1] = edges[1], edges[0]
				}
			}
		default:
			sort.Slice(edges, func(i, j int) bool {
				return edges[i].pos < edges[j].pos
			})
		}
		southEntries = append(southEntries, edges...)
	}

	if len(southEntries) == 0 {
		return 0.0
	}

	// Build accumulator tree, reusing s.treeBuf to avoid repeated allocations
	n := len(southLayer)
	firstIndex := 1
	for firstIndex < n {
		firstIndex <<= 1
	}
	treeSize := 2*firstIndex - 1
	firstIndex--

	if cap(s.treeBuf) < treeSize {
		s.treeBuf = make([]float64, treeSize)
	} else {
		s.treeBuf = s.treeBuf[:treeSize]
		clear(s.treeBuf)
	}

	// Count crossings using the accumulator tree
	cc := 0.0
	for _, e := range southEntries {
		index := e.pos + firstIndex
		s.treeBuf[index] += e.weight

		weightSum := 0.0
		for index > 0 {
			if index%2 == 1 { // Left child
				weightSum += s.treeBuf[index+1] // Add right sibling
			}
			index = (index - 1) >> 1
			s.treeBuf[index] += e.weight
		}
		cc += e.weight * weightSum
	}

	return cc
}

// enforceClusterAdjacency ensures children of each cluster are placed
// adjacently within their layer. Non-cluster nodes between cluster children
// are moved to either side of the cluster block.
func (s *layoutState) enforceClusterAdjacency() {
	if len(s.clusters) == 0 {
		return
	}

	for rank := range s.layers {
		layer := s.layers[rank]
		if len(layer) <= 1 {
			continue
		}

		// Find which nodes belong to which cluster in this layer
		clusterMembers := make(map[string][]int) // cluster ID -> positions in layer
		for i, id := range layer {
			if parent, ok := s.parents[id]; ok && parent != "" {
				if _, isCluster := s.clusters[parent]; isCluster {
					clusterMembers[parent] = append(clusterMembers[parent], i)
				}
			}
		}

		// For each cluster with members in this layer, check if they're adjacent
		for _, positions := range clusterMembers {
			if len(positions) <= 1 {
				continue
			}

			// Check if positions are contiguous
			minPos := positions[0]
			maxPos := positions[0]
			posSet := make(map[int]bool)
			for _, p := range positions {
				posSet[p] = true
				if p < minPos {
					minPos = p
				}
				if p > maxPos {
					maxPos = p
				}
			}

			if maxPos-minPos+1 == len(positions) {
				continue // Already contiguous
			}

			// Reorder: collect cluster members and non-members, then reassemble
			var clusterNodes []string
			var beforeNodes []string
			var afterNodes []string

			clusterStart := minPos
			for i, id := range layer {
				if posSet[i] {
					clusterNodes = append(clusterNodes, id)
				} else if i < clusterStart {
					beforeNodes = append(beforeNodes, id)
				} else {
					afterNodes = append(afterNodes, id)
				}
			}

			// Reassemble: before + cluster + after
			newLayer := make([]string, 0, len(layer))
			newLayer = append(newLayer, beforeNodes...)
			newLayer = append(newLayer, clusterNodes...)
			newLayer = append(newLayer, afterNodes...)
			s.layers[rank] = newLayer

			// Update order values
			for i, id := range s.layers[rank] {
				s.nodes[id].order = i
			}
		}
	}
}

// minimizePortCrossings adjusts node ordering to account for port positions.
// When nodes have multiple ports on the same side, this considers port offsets
// as sub-ordering weights so connected nodes are placed at heights matching
// port positions.
func (s *layoutState) minimizePortCrossings() {
	// For each layer, check if any nodes have ports that could benefit
	// from reordering based on port positions.
	for rank := 0; rank < len(s.layers); rank++ {
		layer := s.layers[rank]
		if len(layer) <= 1 {
			continue
		}

		// Check if any node in this layer has ports
		hasPorts := false
		for _, id := range layer {
			if len(s.nodes[id].ports) > 0 {
				hasPorts = true
				break
			}
		}
		if !hasPorts {
			continue
		}

		// For nodes connected to ported nodes, adjust barycenters
		// using port offsets as weights
		s.adjustForPortPositions(rank)
	}
}

// adjustForPortPositions refines ordering of a layer considering port offsets.
func (s *layoutState) adjustForPortPositions(rank int) {
	_ = s.layers[rank] // validate rank is in bounds

	// For each node in adjacent layers that connects to ports in this layer,
	// compute a port-weighted barycenter
	for adjRank := rank - 1; adjRank <= rank+1; adjRank += 2 {
		if adjRank < 0 || adjRank >= len(s.layers) {
			continue
		}

		adjLayer := s.layers[adjRank]
		type portBarycenter struct {
			nodeID string
			value  float64
			valid  bool
		}

		barycenters := make([]portBarycenter, len(adjLayer))
		changed := false

		for i, nodeID := range adjLayer {
			node := s.nodes[nodeID]
			if node == nil {
				barycenters[i] = portBarycenter{nodeID: nodeID}
				continue
			}

			// Find edges connecting to ports in the target layer
			var neighbors []string
			if adjRank < rank {
				neighbors = s.successors[nodeID]
			} else {
				neighbors = s.predecessors[nodeID]
			}

			sum := 0.0
			weight := 0.0
			for _, neighborID := range neighbors {
				neighbor := s.nodes[neighborID]
				if neighbor == nil || neighbor.rank != rank {
					continue
				}

				// Look up edge directly via map (O(1) per lookup)
				portOffset := 0.0
				hasPort := false

				// Check both directions
				if edge := s.edges[edgeKey{from: nodeID, to: neighborID}]; edge != nil {
					portID := edge.targetPort
					if portID != "" {
						for _, port := range neighbor.ports {
							if port.ID == portID {
								portOffset = port.Offset
								hasPort = true
								break
							}
						}
					}
				} else if edge := s.edges[edgeKey{from: neighborID, to: nodeID}]; edge != nil {
					portID := edge.sourcePort
					if portID != "" {
						for _, port := range neighbor.ports {
							if port.ID == portID {
								portOffset = port.Offset
								hasPort = true
								break
							}
						}
					}
				}

				if hasPort {
					sum += (float64(neighbor.order) + portOffset/100.0)
					weight += 1.0
					changed = true
				} else {
					sum += float64(neighbor.order)
					weight += 1.0
				}
			}

			if weight > 0 {
				barycenters[i] = portBarycenter{nodeID: nodeID, value: sum / weight, valid: true}
			} else {
				barycenters[i] = portBarycenter{nodeID: nodeID}
			}
		}

		if !changed {
			continue
		}

		// Re-sort the adjacent layer by port-adjusted barycenters
		sort.SliceStable(barycenters, func(i, j int) bool {
			if !barycenters[i].valid && !barycenters[j].valid {
				return false
			}
			if !barycenters[i].valid {
				return false
			}
			if !barycenters[j].valid {
				return true
			}
			return barycenters[i].value < barycenters[j].value
		})

		for i, bc := range barycenters {
			s.layers[adjRank][i] = bc.nodeID
			s.nodes[bc.nodeID].order = i
		}
	}
}
