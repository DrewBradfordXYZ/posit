package posit

// makeAcyclic reverses edges to break cycles in the graph.
// It uses DFS to find back edges (edges pointing to ancestors in the DFS tree)
// and reverses them to create a DAG.
func (s *layoutState) makeAcyclic() {
	// Handle self-loops first
	s.removeSelfLoops()

	visited := make(map[string]bool, len(s.nodes))
	onStack := make(map[string]bool) // currently in DFS path

	var reversed []edgeKey

	var dfs func(v string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		onStack[v] = true

		// Iterate over copy to allow modification during iteration
		successors := make([]string, len(s.successors[v]))
		copy(successors, s.successors[v])

		for _, w := range successors {
			if onStack[w] {
				// Back edge found - reverse it
				key := edgeKey{from: v, to: w}
				s.reverseEdge(key)
				reversed = append(reversed, key)
			} else if !visited[w] {
				dfs(w)
			}
		}

		delete(onStack, v)
	}

	// Start DFS from all nodes (handles disconnected components)
	for id := range s.nodes {
		dfs(id)
	}

	s.reversedEdges = reversed
}

// removeSelfLoops removes edges where source equals target.
// Self-loops are always cycles and cannot be laid out meaningfully.
func (s *layoutState) removeSelfLoops() {
	for key := range s.edges {
		if key.from == key.to {
			s.removeEdge(key)
		}
	}
}

// reverseEdge flips an edge's direction and updates adjacency lists.
func (s *layoutState) reverseEdge(key edgeKey) {
	edge := s.edges[key]
	if edge == nil {
		return
	}

	// Remove from adjacency lists
	s.successors[key.from] = removeString(s.successors[key.from], key.to)
	s.predecessors[key.to] = removeString(s.predecessors[key.to], key.from)

	// Create reversed edge
	newKey := edgeKey{from: key.to, to: key.from}
	delete(s.edges, key)

	edge.key = newKey
	edge.reversed = true
	s.edges[newKey] = edge

	// Add to adjacency lists in new direction
	s.successors[newKey.from] = append(s.successors[newKey.from], newKey.to)
	s.predecessors[newKey.to] = append(s.predecessors[newKey.to], newKey.from)
}

// removeEdge removes an edge from the graph and updates adjacency lists.
func (s *layoutState) removeEdge(key edgeKey) {
	if s.edges[key] == nil {
		return
	}

	delete(s.edges, key)
	s.successors[key.from] = removeString(s.successors[key.from], key.to)
	s.predecessors[key.to] = removeString(s.predecessors[key.to], key.from)
}

// removeString removes first occurrence of target from slice.
func removeString(slice []string, target string) []string {
	for i, v := range slice {
		if v == target {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
