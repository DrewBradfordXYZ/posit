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

// calculateBarycenter computes the weighted average position of neighbors.
func (s *layoutState) calculateBarycenter(nodeID string, neighborFn func(string) []string) (float64, bool) {
	neighbors := neighborFn(nodeID)
	if len(neighbors) == 0 {
		return 0, false
	}

	sum := 0.0
	weight := 0.0

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

		sum += float64(neighbor.order) * edgeWeight
		weight += edgeWeight
	}

	if weight == 0 {
		return 0, false
	}

	return sum / weight, true
}

// countCrossings counts total edge crossings in the current layout.
func (s *layoutState) countCrossings() int {
	total := 0
	for i := 1; i < len(s.layers); i++ {
		total += s.twoLayerCrossCount(s.layers[i-1], s.layers[i])
	}
	return total
}

// twoLayerCrossCount counts crossings between two adjacent layers.
// Uses an accumulator tree (similar to Fenwick tree) for O(E log V) performance.
// Reference: Barth, Mutzel, Junger. "Simple and Efficient Bilayer Cross Counting."
func (s *layoutState) twoLayerCrossCount(northLayer, southLayer []string) int {
	if len(northLayer) == 0 || len(southLayer) == 0 {
		return 0
	}

	// Build position map for south layer
	southPos := make(map[string]int, len(southLayer))
	for i, id := range southLayer {
		southPos[id] = i
	}

	// Collect south positions for all edges from north, ordered by north position
	type entry struct {
		pos    int
		weight int
	}
	var southEntries []entry

	for _, v := range northLayer {
		var edges []entry
		for _, w := range s.successors[v] {
			pos, ok := southPos[w]
			if !ok {
				continue // Edge to different layer
			}
			weight := 1
			if edge := s.edges[edgeKey{from: v, to: w}]; edge != nil {
				weight = int(edge.weight)
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
		return 0
	}

	// Build accumulator tree
	n := len(southLayer)
	firstIndex := 1
	for firstIndex < n {
		firstIndex <<= 1
	}
	treeSize := 2*firstIndex - 1
	firstIndex--
	tree := make([]int, treeSize)

	// Count crossings using the accumulator tree
	cc := 0
	for _, e := range southEntries {
		index := e.pos + firstIndex
		tree[index] += e.weight

		weightSum := 0
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
				// Check if the edge uses a port on the neighbor
				portOffset := 0.0
				hasPort := false
				for _, edge := range s.edges {
					if (edge.key.from == nodeID && edge.key.to == neighborID) ||
						(edge.key.from == neighborID && edge.key.to == nodeID) {
						portID := ""
						if edge.key.from == neighborID {
							portID = edge.sourcePort
						} else {
							portID = edge.targetPort
						}
						if portID != "" {
							for _, port := range neighbor.ports {
								if port.ID == portID {
									portOffset = port.Offset
									hasPort = true
									break
								}
							}
						}
						break
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
