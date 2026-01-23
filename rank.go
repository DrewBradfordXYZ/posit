package posit

import "sort"

// assignLayers assigns each node to a rank using the configured algorithm.
func (s *layoutState) assignLayers() {
	switch s.opts.Algorithm {
	case NetworkSimplex:
		s.assignLayersNetworkSimplex()
	case TightTree:
		s.assignLayersTightTree()
	default:
		s.assignLayersLongestPath()
	}

	// Apply rank constraints after initial assignment
	s.applyRankConstraints()

	// Normalize ranks to start at 0
	s.normalizeRanks()

	// Build layers array
	s.buildLayers()
}

// applyRankConstraints adjusts node ranks based on user-specified constraints.
// This runs after initial rank assignment and before normalization.
func (s *layoutState) applyRankConstraints() {
	if len(s.nodes) == 0 {
		return
	}

	// Step 1: Apply RankGroup constraints.
	// Nodes in the same group get the same rank (maximum rank among group members
	// to satisfy edge constraints).
	groups := make(map[string][]string)
	for id, node := range s.nodes {
		if node.rankGroup != "" && !node.isDummy {
			groups[node.rankGroup] = append(groups[node.rankGroup], id)
		}
	}
	for _, members := range groups {
		// Find maximum rank in group
		maxRank := s.nodes[members[0]].rank
		for _, id := range members[1:] {
			if s.nodes[id].rank > maxRank {
				maxRank = s.nodes[id].rank
			}
		}
		// Assign all members to the max rank
		for _, id := range members {
			s.nodes[id].rank = maxRank
		}
	}

	// Step 2: Apply RankMin/RankMax constraints.
	// Find current min and max ranks.
	minRank := 0
	maxRank := 0
	first := true
	for _, node := range s.nodes {
		if first {
			minRank = node.rank
			maxRank = node.rank
			first = false
		} else {
			if node.rank < minRank {
				minRank = node.rank
			}
			if node.rank > maxRank {
				maxRank = node.rank
			}
		}
	}

	for _, node := range s.nodes {
		if node.isDummy {
			continue
		}
		switch node.rankConstraint {
		case RankMin:
			node.rank = minRank
		case RankMax:
			node.rank = maxRank
		}
	}
}

// assignLayersTightTree uses longest-path followed by tight tree construction.
// This is a middle-ground between longest-path (fast but suboptimal) and
// network simplex (optimal but slower). It produces tighter ranks without
// the full optimization loop of network simplex.
func (s *layoutState) assignLayersTightTree() {
	if len(s.nodes) == 0 {
		return
	}

	// Step 1: Initial feasible ranking using longest path
	s.assignLayersLongestPath()

	// Step 2: Build feasible spanning tree (this tightens edges)
	// The feasibleTree function already adjusts ranks to make edges tight
	_ = s.feasibleTree()

	// Note: Unlike NetworkSimplex, we don't run the pivot loop.
	// This is faster but may not produce the optimal result.
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

	// Sort by insertion order (preserves AddNode() call sequence).
	// This gives users control over initial layer ordering, which the
	// barycenter crossing minimization uses as a tiebreaker via stable sort.
	nodeIDs := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Slice(nodeIDs, func(i, j int) bool {
		return s.nodes[nodeIDs[i]].insertOrder < s.nodes[nodeIDs[j]].insertOrder
	})

	// Assign nodes to layers
	for _, id := range nodeIDs {
		node := s.nodes[id]
		s.layers[node.rank] = append(s.layers[node.rank], id)
	}
}
