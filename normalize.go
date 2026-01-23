package posit

import (
	"fmt"
	"sort"
)

// addDummyNodes splits edges that span multiple layers.
// Returns the number of dummy nodes created.
func (s *layoutState) addDummyNodes() int {
	dummyCount := 0

	// Collect edges to process (iterate over copy to allow modification).
	// Sort for deterministic dummy node IDs across runs.
	edgesToProcess := make([]edgeKey, 0, len(s.edges))
	for key := range s.edges {
		edgesToProcess = append(edgesToProcess, key)
	}
	sort.Slice(edgesToProcess, func(i, j int) bool {
		return edgeKeyLess(edgesToProcess[i], edgesToProcess[j])
	})

	for _, key := range edgesToProcess {
		count := s.normalizeEdge(key)
		dummyCount += count
	}

	return dummyCount
}

// normalizeEdge splits a single edge if it spans multiple layers.
// Returns the number of dummy nodes created.
func (s *layoutState) normalizeEdge(key edgeKey) int {
	edge := s.edges[key]
	if edge == nil {
		return 0
	}

	vNode := s.nodes[key.from]
	wNode := s.nodes[key.to]

	vRank := vNode.rank
	wRank := wNode.rank

	// If edge spans only one layer, nothing to do
	if wRank == vRank+1 {
		return 0
	}

	// Edge spans multiple layers - needs dummy nodes
	dummyCount := wRank - vRank - 1
	if dummyCount <= 0 {
		return 0 // Shouldn't happen with valid ranks
	}

	// Determine which rank should have the label dummy (if edge has label)
	labelRank := -1
	if edge.labelWidth > 0 || edge.labelHeight > 0 {
		// Determine label position based on labelPos
		switch edge.labelPos {
		case LabelLeft:
			labelRank = vRank + 1 // Near source
		case LabelRight:
			labelRank = wRank - 1 // Near target
		default: // LabelCenter or empty
			labelRank = vRank + (wRank-vRank)/2 // Middle
		}
	}

	// Remove original edge
	s.removeEdge(key)

	// Create chain of dummy nodes
	v := key.from
	var firstDummy string
	var labelDummyID string

	for rank := vRank + 1; rank < wRank; rank++ {
		// Create dummy node
		dummyID := s.newDummyID()

		// Determine dummy size - label dummy gets label dimensions
		dummyWidth := 0.0
		dummyHeight := 0.0
		isLabelDummy := rank == labelRank
		if isLabelDummy {
			dummyWidth = edge.labelWidth
			dummyHeight = edge.labelHeight
			labelDummyID = dummyID
		}

		dummy := &layoutNode{
			id:        dummyID,
			width:     dummyWidth,
			height:    dummyHeight,
			rank:      rank,
			order:     -1, // Will be set in Phase 4
			isDummy:   true,
			edgeLabel: edge,
		}

		s.nodes[dummyID] = dummy
		s.successors[dummyID] = nil
		s.predecessors[dummyID] = nil

		// Add to layer
		s.layers[rank] = append(s.layers[rank], dummyID)

		// Track first dummy for later removal
		if firstDummy == "" {
			firstDummy = dummyID
		}

		// Create edge from previous node to dummy
		s.addEdge(edgeKey{from: v, to: dummyID}, edge.weight)
		v = dummyID
	}

	// Create final edge from last dummy to target
	s.addEdge(edgeKey{from: v, to: key.to}, edge.weight)

	// Track dummy chain for reconstruction
	if firstDummy != "" {
		s.dummyChains = append(s.dummyChains, firstDummy)
	}

	// Track label dummy for position extraction later
	if labelDummyID != "" {
		edge.labelDummyID = labelDummyID
	}

	return dummyCount
}

// markInteriorDummies identifies dummy nodes that are in the interior of a
// long-edge chain (both predecessor and successor are also dummies). These
// nodes have their ordering fully determined by barycenter from their single
// neighbor, making adjacent exchange redundant for them.
// Reference: Eiglsperger, Siebenhaller, Kaufmann (2005) "An Efficient
// Implementation of Sugiyama's Algorithm for Layered Graph Drawing"
func (s *layoutState) markInteriorDummies() {
	for id, node := range s.nodes {
		if !node.isDummy {
			continue
		}
		preds := s.predecessors[id]
		succs := s.successors[id]
		if len(preds) == 1 && len(succs) == 1 &&
			s.nodes[preds[0]].isDummy && s.nodes[succs[0]].isDummy {
			node.isInteriorDummy = true
		}
	}
}

// newDummyID generates a unique ID for a dummy node.
func (s *layoutState) newDummyID() string {
	s.dummyCounter++
	return fmt.Sprintf("_dummy_%d", s.dummyCounter)
}

// addEdge adds a new edge with specified weight.
func (s *layoutState) addEdge(key edgeKey, weight float64) {
	s.edges[key] = &layoutEdge{
		key:    key,
		weight: weight,
		minlen: 1,
	}
	s.successors[key.from] = append(s.successors[key.from], key.to)
	s.predecessors[key.to] = append(s.predecessors[key.to], key.from)
}

