package posit

import "sort"

// assignLayers assigns each node to a rank using the configured algorithm.
func (s *layoutState) assignLayers() {
	switch s.opts.Algorithm {
	case NetworkSimplex:
		s.assignLayersNetworkSimplex()
	default:
		s.assignLayersLongestPath()
	}

	// Normalize ranks to start at 0
	s.normalizeRanks()

	// Build layers array
	s.buildLayers()
}

// assignLayersLongestPath implements the fast longest-path ranking.
// It assigns ranks by working backwards from sink nodes, ensuring each node
// is placed as close to the bottom as possible while respecting edge constraints.
func (s *layoutState) assignLayersLongestPath() {
	visited := make(map[string]bool, len(s.nodes))

	var dfs func(v string) int
	dfs = func(v string) int {
		node := s.nodes[v]
		if visited[v] {
			return node.rank
		}
		visited[v] = true

		// Find minimum rank based on successors
		minRank := 0
		hasSuccessor := false

		for _, w := range s.successors[v] {
			hasSuccessor = true
			edge := s.edges[edgeKey{from: v, to: w}]
			minlen := 1
			if edge != nil && edge.minlen > 0 {
				minlen = edge.minlen
			}
			wRank := dfs(w)
			candidate := wRank - minlen
			if candidate < minRank {
				minRank = candidate
			}
		}

		if !hasSuccessor {
			// Sink node - assign rank 0 (will be normalized later)
			node.rank = 0
		} else {
			node.rank = minRank
		}

		return node.rank
	}

	// Process all nodes
	for id := range s.nodes {
		dfs(id)
	}
}

// normalizeRanks shifts all ranks so minimum is 0.
func (s *layoutState) normalizeRanks() {
	if len(s.nodes) == 0 {
		return
	}

	// Find minimum rank
	minRank := 0
	first := true
	for _, node := range s.nodes {
		if first || node.rank < minRank {
			minRank = node.rank
			first = false
		}
	}

	// Shift all ranks if needed
	if minRank != 0 {
		for _, node := range s.nodes {
			node.rank -= minRank
		}
	}
}

// buildLayers creates the layers slice from node ranks.
func (s *layoutState) buildLayers() {
	if len(s.nodes) == 0 {
		s.layers = [][]string{}
		return
	}

	// Find max rank
	maxRank := 0
	for _, node := range s.nodes {
		if node.rank > maxRank {
			maxRank = node.rank
		}
	}

	// Create layer arrays
	s.layers = make([][]string, maxRank+1)
	for i := range s.layers {
		s.layers[i] = make([]string, 0)
	}

	// Sort node IDs for deterministic ordering
	nodeIDs := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Assign nodes to layers
	for _, id := range nodeIDs {
		node := s.nodes[id]
		s.layers[node.rank] = append(s.layers[node.rank], id)
	}
}
