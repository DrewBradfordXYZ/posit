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

	// Initialize order within each layer
	s.initOrder()

	// Track best solution
	bestLayers := s.copyLayers()
	bestCrossings := s.countCrossings()

	// Iterate until no improvement
	maxIterations := 24
	noImprovement := 0

	for i := 0; i < maxIterations && noImprovement < 4; i++ {
		// Alternate between sweeping down and up
		if i%2 == 0 {
			s.sweepDown()
		} else {
			s.sweepUp()
		}

		// Adjacent exchange: try swapping adjacent nodes to escape local minima
		s.adjacentExchange()

		// Count crossings
		crossings := s.countCrossings()

		if crossings < bestCrossings {
			bestCrossings = crossings
			bestLayers = s.copyLayers()
			noImprovement = 0
		} else {
			noImprovement++
		}
	}

	// Restore best solution
	s.layers = bestLayers
	s.assignOrderFromLayers()

	// Ensure cluster children are adjacent within layers
	s.enforceClusterAdjacency()

	// Port-level crossing minimization pass
	s.minimizePortCrossings()
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

// adjacentExchange tries swapping each pair of adjacent nodes in each layer,
// keeping the swap if it reduces the total crossing count.
// Skipped for large layers (>50 nodes) where the O(n²) cost is prohibitive.
func (s *layoutState) adjacentExchange() {
	for rank := 0; rank < len(s.layers); rank++ {
		layer := s.layers[rank]
		if len(layer) <= 1 || len(layer) > 50 {
			continue
		}

		// Limit passes to prevent excessive computation
		for pass := 0; pass < 2; pass++ {
			improved := false
			for i := 0; i < len(layer)-1; i++ {
				// Count crossings before swap
				before := s.crossingsInvolving(rank)

				// Swap adjacent pair
				layer[i], layer[i+1] = layer[i+1], layer[i]
				s.nodes[layer[i]].order = i
				s.nodes[layer[i+1]].order = i + 1

				// Count crossings after swap
				after := s.crossingsInvolving(rank)

				if after < before {
					improved = true
				} else {
					// Revert swap
					layer[i], layer[i+1] = layer[i+1], layer[i]
					s.nodes[layer[i]].order = i
					s.nodes[layer[i+1]].order = i + 1
				}
			}
			if !improved {
				break
			}
		}
	}
}

// crossingsInvolving counts crossings in layers adjacent to the given rank.
func (s *layoutState) crossingsInvolving(rank int) float64 {
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
	var positions []weightedPos
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

	// Sort by position
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].position < positions[j].position
	})

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

	// Build position map for south layer
	southPos := make(map[string]int, len(southLayer))
	for i, id := range southLayer {
		southPos[id] = i
	}

	// Collect south positions for all edges from north, ordered by north position
	type entry struct {
		pos    int
		weight float64
	}
	var southEntries []entry

	for _, v := range northLayer {
		var edges []entry
		for _, w := range s.successors[v] {
			pos, ok := southPos[w]
			if !ok {
				continue // Edge to different layer
			}
			weight := 1.0
			if edge := s.edges[edgeKey{from: v, to: w}]; edge != nil {
				weight = edge.weight
				if weight < 1 {
					weight = 1
				}
			}
			edges = append(edges, entry{pos: pos, weight: weight})
		}
		// Sort by position within this node's edges
		sort.Slice(edges, func(i, j int) bool {
			return edges[i].pos < edges[j].pos
		})
		southEntries = append(southEntries, edges...)
	}

	if len(southEntries) == 0 {
		return 0.0
	}

	// Build accumulator tree
	n := len(southLayer)
	firstIndex := 1
	for firstIndex < n {
		firstIndex <<= 1
	}
	treeSize := 2*firstIndex - 1
	firstIndex--
	tree := make([]float64, treeSize)

	// Count crossings using the accumulator tree
	cc := 0.0
	for _, e := range southEntries {
		index := e.pos + firstIndex
		tree[index] += e.weight

		weightSum := 0.0
		for index > 0 {
			if index%2 == 1 { // Left child
				weightSum += tree[index+1] // Add right sibling
			}
			index = (index - 1) >> 1
			tree[index] += e.weight
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
