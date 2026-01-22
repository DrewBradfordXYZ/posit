package posit

import "math"

// spanningTree holds the tree structure for the network simplex algorithm.
type spanningTree struct {
	nodes     map[string]*treeNode
	treeEdges map[edgeKey]bool   // Which edges are in the tree
	cutValues map[edgeKey]int    // Cut value for each tree edge
	root      string
}

// treeNode holds per-node data for the spanning tree.
type treeNode struct {
	id     string
	parent string // Parent in tree (empty for root)
	low    int    // Minimum postorder in subtree
	lim    int    // This node's postorder (max in subtree)
}

// newSpanningTree creates a new empty spanning tree.
func newSpanningTree() *spanningTree {
	return &spanningTree{
		nodes:     make(map[string]*treeNode),
		treeEdges: make(map[edgeKey]bool),
		cutValues: make(map[edgeKey]int),
	}
}

// addNode adds a node to the tree.
func (t *spanningTree) addNode(id string) {
	if _, exists := t.nodes[id]; exists {
		return
	}
	t.nodes[id] = &treeNode{id: id}
	if t.root == "" {
		t.root = id
	}
}

// hasNode returns true if the node is in the tree.
func (t *spanningTree) hasNode(id string) bool {
	_, ok := t.nodes[id]
	return ok
}

// nodeCount returns the number of nodes in the tree.
func (t *spanningTree) nodeCount() int {
	return len(t.nodes)
}

// addEdge adds an edge to the tree (both directions for traversal).
func (t *spanningTree) addEdge(key edgeKey) {
	t.treeEdges[key] = true
	// Also store reverse for undirected tree traversal
	t.treeEdges[edgeKey{from: key.to, to: key.from}] = true
}

// removeEdge removes an edge from the tree.
func (t *spanningTree) removeEdge(key edgeKey) {
	delete(t.treeEdges, key)
	delete(t.treeEdges, edgeKey{from: key.to, to: key.from})
	delete(t.cutValues, key)
	delete(t.cutValues, edgeKey{from: key.to, to: key.from})
}

// isTreeEdge returns true if the edge is in the spanning tree.
func (t *spanningTree) isTreeEdge(key edgeKey) bool {
	return t.treeEdges[key] || t.treeEdges[edgeKey{from: key.to, to: key.from}]
}

// neighbors returns adjacent nodes in the tree.
func (t *spanningTree) neighbors(v string) []string {
	var result []string
	for key := range t.treeEdges {
		if key.from == v {
			result = append(result, key.to)
		}
	}
	return result
}

// adjustRanks adjusts ranks of all tree nodes by delta.
func (t *spanningTree) adjustRanks(s *layoutState, delta int) {
	for id := range t.nodes {
		s.nodes[id].rank += delta
	}
}

// setCutValue sets the cut value for a tree edge.
func (t *spanningTree) setCutValue(v, w string, value int) {
	key := edgeKey{from: v, to: w}
	t.cutValues[key] = value
	// Store in both directions
	t.cutValues[edgeKey{from: w, to: v}] = value
}

// slack calculates how much longer an edge is than required.
func (s *layoutState) slack(key edgeKey) int {
	fromRank := s.nodes[key.from].rank
	toRank := s.nodes[key.to].rank
	minlen := 1
	if edge := s.edges[key]; edge != nil && edge.minlen > 0 {
		minlen = edge.minlen
	}
	return toRank - fromRank - minlen
}

// feasibleTree builds a tight spanning tree from the current ranking.
func (s *layoutState) feasibleTree() *spanningTree {
	tree := newSpanningTree()

	// Start with arbitrary node
	var root string
	for id := range s.nodes {
		root = id
		break
	}
	tree.addNode(root)

	// Grow tree until all nodes included
	for tree.nodeCount() < len(s.nodes) {
		// Find minimum slack edge connecting tree to non-tree
		var bestEdge edgeKey
		minSlack := math.MaxInt
		treeToNonTree := true
		found := false

		for key := range s.edges {
			inTreeFrom := tree.hasNode(key.from)
			inTreeTo := tree.hasNode(key.to)

			if inTreeFrom == inTreeTo {
				continue // Both in or both out
			}

			slack := s.slack(key)
			if slack < minSlack {
				minSlack = slack
				bestEdge = key
				treeToNonTree = inTreeFrom
				found = true
			}
		}

		if !found {
			// Graph might be disconnected - find any non-tree node
			for id := range s.nodes {
				if !tree.hasNode(id) {
					tree.addNode(id)
					break
				}
			}
			continue
		}

		// Tighten edge by adjusting tree ranks
		if minSlack > 0 {
			delta := minSlack
			if !treeToNonTree {
				delta = -delta
			}
			tree.adjustRanks(s, delta)
		}

		// Add edge and new node to tree
		tree.addEdge(bestEdge)
		if treeToNonTree {
			tree.addNode(bestEdge.to)
		} else {
			tree.addNode(bestEdge.from)
		}
	}

	return tree
}

// initLowLimValues computes low/lim values for O(1) descendant queries.
func (t *spanningTree) initLowLimValues() {
	visited := make(map[string]bool)
	counter := 1

	var dfs func(v, parent string) int
	dfs = func(v, parent string) int {
		node := t.nodes[v]
		node.parent = parent
		low := counter

		visited[v] = true

		// Visit children (neighbors except parent)
		for _, neighbor := range t.neighbors(v) {
			if !visited[neighbor] {
				counter = dfs(neighbor, v)
			}
		}

		node.low = low
		node.lim = counter
		counter++
		return counter
	}

	dfs(t.root, "")
}

// isDescendant returns true if v is in the subtree rooted at u.
func (t *spanningTree) isDescendant(v, u string) bool {
	uNode := t.nodes[u]
	vNode := t.nodes[v]
	if uNode == nil || vNode == nil {
		return false
	}
	return uNode.low <= vNode.lim && vNode.lim <= uNode.lim
}

// postorderNodes returns nodes in postorder (children before parents).
func (t *spanningTree) postorderNodes() []string {
	result := make([]string, 0, len(t.nodes))
	visited := make(map[string]bool)

	var dfs func(v string)
	dfs = func(v string) {
		visited[v] = true
		for _, neighbor := range t.neighbors(v) {
			if !visited[neighbor] {
				dfs(neighbor)
			}
		}
		result = append(result, v)
	}

	dfs(t.root)
	return result
}

// initCutValues computes cut values for all tree edges.
func (t *spanningTree) initCutValues(s *layoutState) {
	// Process in postorder (children before parents)
	postorder := t.postorderNodes()

	for _, v := range postorder {
		if v == t.root {
			continue
		}
		t.assignCutValue(s, v)
	}
}

// assignCutValue computes the cut value for edge (v, parent[v]).
func (t *spanningTree) assignCutValue(s *layoutState, v string) {
	parent := t.nodes[v].parent
	if parent == "" {
		return
	}

	// Find the graph edge weight
	var graphWeight float64 = 1
	graphEdge := s.edges[edgeKey{from: v, to: parent}]
	if graphEdge != nil {
		graphWeight = graphEdge.weight
	} else {
		graphEdge = s.edges[edgeKey{from: parent, to: v}]
		if graphEdge != nil {
			graphWeight = graphEdge.weight
		}
	}

	cutValue := int(graphWeight)

	// Add/subtract weights of non-tree edges based on direction
	for key, edge := range s.edges {
		if t.isTreeEdge(key) {
			continue
		}

		fromIsDesc := t.isDescendant(key.from, v)
		toIsDesc := t.isDescendant(key.to, v)

		if fromIsDesc && !toIsDesc {
			// Same direction as tree edge (exiting subtree)
			cutValue += int(edge.weight)
		} else if !fromIsDesc && toIsDesc {
			// Opposite direction (entering subtree)
			cutValue -= int(edge.weight)
		}
	}

	t.setCutValue(v, parent, cutValue)
}

// leaveEdge finds a tree edge with negative cut value.
func (t *spanningTree) leaveEdge() (edgeKey, bool) {
	// Look for edges where child->parent has negative cut value
	for v, node := range t.nodes {
		if node.parent == "" {
			continue
		}
		key := edgeKey{from: v, to: node.parent}
		if cutVal, ok := t.cutValues[key]; ok && cutVal < 0 {
			return key, true
		}
	}
	return edgeKey{}, false
}

// enterEdge finds the best non-tree edge to add when removing leave.
func (t *spanningTree) enterEdge(s *layoutState, leave edgeKey) edgeKey {
	// Determine which end is the "tail" (subtree being disconnected)
	v, w := leave.from, leave.to
	vNode := t.nodes[v]
	wNode := t.nodes[w]

	// The tail is the one with smaller lim (it's the descendant)
	var tailID string
	if vNode.lim < wNode.lim {
		tailID = v
	} else {
		tailID = w
	}

	// Find non-tree edge with minimum slack that crosses the cut
	var best edgeKey
	bestSlack := math.MaxInt
	found := false

	for key := range s.edges {
		if t.isTreeEdge(key) {
			continue
		}

		fromInTail := t.isDescendant(key.from, tailID)
		toInTail := t.isDescendant(key.to, tailID)

		// Edge must cross the cut: one end in tail, one end outside
		if fromInTail == toInTail {
			continue
		}

		// For a valid entering edge, we need the direction to tighten the tree
		// The edge should go from tail to head (or opposite direction of what we need)
		slack := s.slack(key)
		if slack < bestSlack {
			bestSlack = slack
			best = key
			found = true
		}
	}

	if !found {
		return edgeKey{}
	}
	return best
}

// exchangeEdges swaps leave edge with enter edge and updates the tree.
func (t *spanningTree) exchangeEdges(s *layoutState, leave, enter edgeKey) {
	// Calculate slack of entering edge BEFORE modifying anything
	slack := s.slack(enter)

	// Identify which subtree will be disconnected by removing leave edge.
	// The node with smaller lim value is the subtree root (child in tree).
	tailNode := leave.from
	if t.nodes[leave.to].lim < t.nodes[leave.from].lim {
		tailNode = leave.to
	}

	// Determine if entering edge goes "into" or "out of" the tail subtree.
	// This determines whether we add or subtract slack.
	enterFromInTail := t.isDescendant(enter.from, tailNode)
	enterToInTail := t.isDescendant(enter.to, tailNode)

	// Calculate delta to apply to tail subtree to make enter edge tight.
	// slack = rank[to] - rank[from] - minlen
	// If enter.from is in tail (we'll add delta to it): new_slack = rank[to] - (rank[from] + delta) - minlen = slack - delta = 0 => delta = slack
	// If enter.to is in tail (we'll add delta to it): new_slack = (rank[to] + delta) - rank[from] - minlen = slack + delta = 0 => delta = -slack
	var delta int
	if enterFromInTail && !enterToInTail {
		delta = slack
	} else if enterToInTail && !enterFromInTail {
		delta = -slack
	}

	// Apply delta to all nodes in the tail subtree
	if delta != 0 {
		for id := range t.nodes {
			if t.isDescendant(id, tailNode) {
				s.nodes[id].rank += delta
			}
		}
	}

	// Now modify the tree structure
	t.removeEdge(leave)
	t.addEdge(enter)

	// Recompute low/lim values for the new tree
	t.initLowLimValues()

	// Recompute cut values
	t.initCutValues(s)
}

// assignLayersNetworkSimplex uses the network simplex algorithm for optimal ranking.
func (s *layoutState) assignLayersNetworkSimplex() {
	if len(s.nodes) == 0 {
		return
	}

	// Step 1: Initial feasible ranking using longest path
	s.assignLayersLongestPath()

	// Save initial ranks in case we need to fallback
	initialRanks := make(map[string]int, len(s.nodes))
	for id, node := range s.nodes {
		initialRanks[id] = node.rank
	}

	// Step 2: Build feasible spanning tree
	tree := s.feasibleTree()

	// Step 3: Initialize tree values
	tree.initLowLimValues()
	tree.initCutValues(s)

	// Step 4: Iterate until optimal (no negative cut values)
	maxIterations := max(len(s.nodes)*len(s.edges), 100)

	for i := 0; i < maxIterations; i++ {
		leave, found := tree.leaveEdge()
		if !found {
			break // Optimal!
		}

		enter := tree.enterEdge(s, leave)
		if enter.from == "" {
			break // No valid entering edge
		}

		tree.exchangeEdges(s, leave, enter)
	}

	// Validate result - ensure all edge constraints are satisfied
	if !s.validateRanks() {
		// Fallback to initial longest-path ranking
		for id, rank := range initialRanks {
			s.nodes[id].rank = rank
		}
	}
}

// validateRanks checks that all edge constraints are satisfied.
func (s *layoutState) validateRanks() bool {
	for key, edge := range s.edges {
		fromRank := s.nodes[key.from].rank
		toRank := s.nodes[key.to].rank
		minlen := edge.minlen
		if minlen <= 0 {
			minlen = 1
		}
		if toRank-fromRank < minlen {
			return false
		}
	}
	return true
}
