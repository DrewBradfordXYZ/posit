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

	// Apply RankGroup constraints (post-hoc: move group members to same rank)
	s.applyRankGroups()

	// Apply RankMin/RankMax constraints
	s.applyRankMinMax()

	// Constrain cluster children to consecutive ranks
	s.constrainClusterRanks()

	// Normalize ranks to start at 0
	s.normalizeRanks()

	// Build layers array
	s.buildLayers()
}

// applyRankGroups moves all members of a RankGroup to the minimum rank
// among the group members. This is applied post-hoc after ranking so it
// works regardless of the graph structure (even when group members are
// connected through paths that would make same-rank infeasible as edge constraints).
func (s *layoutState) applyRankGroups() {
	groups := make(map[string][]string)
	for id, node := range s.nodes {
		if node.rankGroup != "" && !node.isDummy {
			groups[node.rankGroup] = append(groups[node.rankGroup], id)
		}
	}

	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		// Find minimum rank in this group
		minRank := s.nodes[members[0]].rank
		for _, id := range members[1:] {
			if s.nodes[id].rank < minRank {
				minRank = s.nodes[id].rank
			}
		}
		// Move all members to the minimum rank
		for _, id := range members {
			s.nodes[id].rank = minRank
		}
	}
}

// applyRankMinMax adjusts ranks for nodes with RankMin/RankMax constraints.
func (s *layoutState) applyRankMinMax() {
	if len(s.nodes) == 0 {
		return
	}

	// Find current min and max ranks
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

// constrainClusterRanks ensures children of each cluster occupy consecutive ranks.
// If children are spread across non-consecutive ranks, intermediate nodes are
// shifted to make the cluster's rank range contiguous.
func (s *layoutState) constrainClusterRanks() {
	if len(s.clusters) == 0 {
		return
	}

	for clusterID := range s.clusters {
		// Find all children of this cluster
		var children []string
		for childID, parentID := range s.parents {
			if parentID == clusterID {
				if _, ok := s.nodes[childID]; ok {
					children = append(children, childID)
				}
			}
		}
		if len(children) < 2 {
			continue
		}

		// Find min and max rank of children
		minRank := s.nodes[children[0]].rank
		maxRank := s.nodes[children[0]].rank
		for _, id := range children[1:] {
			r := s.nodes[id].rank
			if r < minRank {
				minRank = r
			}
			if r > maxRank {
				maxRank = r
			}
		}

		// Children already on consecutive ranks if maxRank - minRank + 1 == len(distinct ranks)
		childRanks := make(map[int]bool)
		for _, id := range children {
			childRanks[s.nodes[id].rank] = true
		}
		if len(childRanks) == maxRank-minRank+1 {
			continue // Already contiguous
		}

		// Compact: move all children to fill the range [minRank, minRank+len(childRanks)-1]
		// Sort unique ranks and assign children accordingly
		sortedRanks := make([]int, 0, len(childRanks))
		for r := range childRanks {
			sortedRanks = append(sortedRanks, r)
		}
		sort.Ints(sortedRanks)

		rankMap := make(map[int]int)
		for i, r := range sortedRanks {
			rankMap[r] = minRank + i
		}

		for _, id := range children {
			s.nodes[id].rank = rankMap[s.nodes[id].rank]
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
