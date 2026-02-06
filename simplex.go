package posit

import (
	"fmt"
	"math"
	"sort"
)

// ============================================================================
// Y Coordinate Network Simplex (Layer/Rank Assignment)
// ============================================================================

// spanningTree holds the tree structure for the network simplex algorithm.
type spanningTree struct {
	nodes          map[string]*treeNode
	treeEdges      map[edgeKey]bool    // Which edges are in the tree
	adj            map[string][]string // Adjacency lists for O(1) neighbor lookup
	cutValues      map[edgeKey]int     // Cut value for each tree edge
	root           string
	sortedNodeIDs  []string   // Cached sorted node IDs for leaveEdge()
	sortedEdgeKeys []edgeKey  // Cached sorted graph edge keys for enterEdge()
	searchIndex    int        // Circular search index for leaveEdge() (Graphviz S_i)
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
		adj:       make(map[string][]string),
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
	// Update adjacency lists for O(1) neighbor lookup
	t.adj[key.from] = append(t.adj[key.from], key.to)
	t.adj[key.to] = append(t.adj[key.to], key.from)
}

// removeEdge removes an edge from the tree.
func (t *spanningTree) removeEdge(key edgeKey) {
	delete(t.treeEdges, key)
	delete(t.treeEdges, edgeKey{from: key.to, to: key.from})
	delete(t.cutValues, key)
	delete(t.cutValues, edgeKey{from: key.to, to: key.from})
	// Update adjacency lists
	t.adj[key.from] = removeFromSlice(t.adj[key.from], key.to)
	t.adj[key.to] = removeFromSlice(t.adj[key.to], key.from)
}

// removeFromSlice removes the first occurrence of val from slice.
// removeFromSlice removes val from slice using O(1) swap-delete.
// Order is not preserved, but that's fine since we sort for determinism elsewhere.
func removeFromSlice(slice []string, val string) []string {
	for i, v := range slice {
		if v == val {
			slice[i] = slice[len(slice)-1] // Swap with last element
			return slice[:len(slice)-1]    // Shrink slice (no shift needed)
		}
	}
	return slice
}

// isTreeEdge returns true if the edge is in the spanning tree.
func (t *spanningTree) isTreeEdge(key edgeKey) bool {
	return t.treeEdges[key] || t.treeEdges[edgeKey{from: key.to, to: key.from}]
}

// neighbors returns adjacent nodes in the tree.
// Uses adjacency lists for O(1) lookup instead of O(edges) iteration.
func (t *spanningTree) neighbors(v string) []string {
	return t.adj[v]
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
	if edge := s.edges[key]; edge != nil {
		if edge.minlen > 0 {
			minlen = edge.minlen
		} else if edge.minlen == 0 {
			minlen = 0 // explicitly set to 0 (e.g., rank group constraints)
		}
	}
	return toRank - fromRank - minlen
}

// feasibleTree builds a tight spanning tree from the current ranking.
func (s *layoutState) feasibleTree() *spanningTree {
	tree := newSpanningTree()

	// Start with deterministic root (first node in sorted order)
	nodeIDs := make([]string, 0, len(s.nodes))
	for id := range s.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Handle empty graph
	if len(nodeIDs) == 0 {
		return tree
	}

	root := nodeIDs[0]
	tree.addNode(root)

	// Grow tree until all nodes included
	for tree.nodeCount() < len(s.nodes) {
		// Find minimum slack edge connecting tree to non-tree.
		// Break ties deterministically by comparing edgeKey fields.
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
			if slack < minSlack || (slack == minSlack && edgeKeyLess(key, bestEdge)) {
				minSlack = slack
				bestEdge = key
				treeToNonTree = inTreeFrom
				found = true
			}
		}

		if !found {
			// Graph might be disconnected - find any non-tree node
			// Use the pre-sorted nodeIDs for deterministic selection.
			for _, id := range nodeIDs {
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

	// Cache sorted node IDs for deterministic iteration in leaveEdge()
	tree.sortedNodeIDs = make([]string, 0, len(tree.nodes))
	for id := range tree.nodes {
		tree.sortedNodeIDs = append(tree.sortedNodeIDs, id)
	}
	sort.Strings(tree.sortedNodeIDs)

	return tree
}

// initLowLimValues computes low/lim values for O(1) descendant queries.
func (t *spanningTree) initLowLimValues() {
	visited := make(map[string]bool)
	counter := 1

	var dfs func(v, parent string) int
	dfs = func(v, parent string) int {
		node := t.nodes[v]
		if node == nil {
			return counter
		}
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
// This follows dagre's calcCutValue algorithm which considers:
// 1. The weight of the tree edge itself
// 2. Non-tree edges crossing the cut
// 3. Cut values of child tree edges (propagation)
func (t *spanningTree) assignCutValue(s *layoutState, v string) {
	parent := t.nodes[v].parent
	if parent == "" {
		return
	}

	// Determine if child (v) is on the tail end of the edge in the directed graph
	childIsTail := true
	graphEdge := s.edges[edgeKey{from: v, to: parent}]
	if graphEdge == nil {
		childIsTail = false
		graphEdge = s.edges[edgeKey{from: parent, to: v}]
	}

	// Start with the weight of the tree edge
	var cutValue int
	if graphEdge != nil {
		cutValue = int(graphEdge.weight)
	} else {
		cutValue = 1 // Default weight
	}

	// Process all edges incident to v (except the edge to parent)
	for key, edge := range s.edges {
		// Skip if neither endpoint is v
		if key.from != v && key.to != v {
			continue
		}

		// Determine the "other" node and whether this is an out-edge from v
		isOutEdge := key.from == v
		var other string
		if isOutEdge {
			other = key.to
		} else {
			other = key.from
		}

		// Skip the edge to parent
		if other == parent {
			continue
		}

		// pointsToHead: does this edge point in the same direction as the tree edge?
		// If childIsTail (v->parent in graph), then out-edges from v point same direction
		// If !childIsTail (parent->v in graph), then in-edges to v point same direction
		pointsToHead := isOutEdge == childIsTail

		// Add or subtract the edge weight
		if pointsToHead {
			cutValue += int(edge.weight)
		} else {
			cutValue -= int(edge.weight)
		}

		// If this is a tree edge to a child, propagate its cut value
		// Must verify other is actually a child (parent == v), not just any tree edge
		if t.isTreeEdge(key) && t.nodes[other] != nil && t.nodes[other].parent == v {
			// Get the cut value of the child tree edge
			// Use ok idiom since cut value of 0 is valid (balanced edge)
			childCutValue, ok := t.cutValues[edgeKey{from: other, to: v}]
			if !ok {
				childCutValue = t.cutValues[edgeKey{from: v, to: other}]
			}

			// Propagate: opposite sign of pointsToHead
			if pointsToHead {
				cutValue -= childCutValue
			} else {
				cutValue += childCutValue
			}
		}
	}

	t.setCutValue(v, parent, cutValue)
}

// leaveEdge finds a tree edge with negative cut value.
// leaveEdgeSearchLimit caps how many negative cut value edges we examine.
// After finding this many candidates, we return the best found so far.
// This follows Graphviz's SEARCHSIZE heuristic to prevent pathological cases.
const leaveEdgeSearchLimit = 30

func (t *spanningTree) leaveEdge() (edgeKey, bool) {
	// Use cached sorted node IDs with circular search (Graphviz S_i pattern)
	// Look for the edge with most negative cut value, but limit search
	n := len(t.sortedNodeIDs)
	if n == 0 {
		return edgeKey{}, false
	}

	var best edgeKey
	bestCut := 0
	found := 0
	startIdx := t.searchIndex

	// Search from startIdx to end
	for i := startIdx; i < n && found < leaveEdgeSearchLimit; i++ {
		v := t.sortedNodeIDs[i]
		node := t.nodes[v]
		if node.parent == "" {
			continue
		}
		key := edgeKey{from: v, to: node.parent}
		if cutVal, ok := t.cutValues[key]; ok && cutVal < 0 {
			if cutVal < bestCut {
				best = key
				bestCut = cutVal
			}
			found++
			t.searchIndex = i + 1 // Continue from next position
		}
	}

	// Wrap around: search from 0 to startIdx
	if found < leaveEdgeSearchLimit && startIdx > 0 {
		for i := 0; i < startIdx && found < leaveEdgeSearchLimit; i++ {
			v := t.sortedNodeIDs[i]
			node := t.nodes[v]
			if node.parent == "" {
				continue
			}
			key := edgeKey{from: v, to: node.parent}
			if cutVal, ok := t.cutValues[key]; ok && cutVal < 0 {
				if cutVal < bestCut {
					best = key
					bestCut = cutVal
				}
				found++
				t.searchIndex = i + 1
			}
		}
	}

	// Reset search index if we wrapped around completely
	if t.searchIndex >= n {
		t.searchIndex = 0
	}

	if found > 0 {
		return best, true
	}
	return edgeKey{}, false
}

// enterEdge finds the best non-tree edge to add when removing leave.
// This follows dagre's enterEdge algorithm with proper flip logic.
func (t *spanningTree) enterEdge(s *layoutState, leave edgeKey) edgeKey {
	v, w := leave.from, leave.to

	// For the rest of this function we assume that v is the tail and w is the
	// head in the graph direction. If we don't have this edge in the graph,
	// flip it to match the correct orientation.
	if s.edges[edgeKey{from: v, to: w}] == nil {
		v, w = w, v
	}

	vNode := t.nodes[v]
	wNode := t.nodes[w]
	if vNode == nil || wNode == nil {
		return edgeKey{} // Invalid state - node not in tree
	}

	// tailLabel is used to determine which side of the cut a node is on
	tailLabel := vNode
	flip := false

	// If the root is in the tail of the edge then we need to flip the logic
	// that checks for the head and tail nodes in the candidates filter below.
	if vNode.lim > wNode.lim {
		tailLabel = wNode
		flip = true
	}

	// Use cached sorted edge keys for deterministic iteration (built before simplex loop)
	// Find non-tree edge with minimum slack that crosses the cut correctly.
	// The entering edge must go from the tail side to the head side.
	var best edgeKey
	bestSlack := math.MaxInt
	found := false

	for _, key := range t.sortedEdgeKeys {
		if t.isTreeEdge(key) {
			continue
		}

		// Check if edge.v (from) is in tail subtree
		fromInTail := t.isDescendant(key.from, tailLabel.id)
		// Check if edge.w (to) is in tail subtree
		toInTail := t.isDescendant(key.to, tailLabel.id)

		// The edge must cross the cut in the right direction:
		// flip == fromInTail means "from" is on the expected side
		// flip != toInTail means "to" is on the opposite side
		// This ensures the edge goes from tail to head (or vice versa with flip)
		if flip != fromInTail || flip == toInTail {
			continue
		}

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

// getParentEdgeKey returns the edge key connecting v to its parent in the tree.
// Returns the key and whether v is the "from" node in that key.
func (t *spanningTree) getParentEdgeKey(v string) (edgeKey, bool) {
	tnode := t.nodes[v]
	if tnode == nil {
		return edgeKey{}, false
	}
	parent := tnode.parent
	if parent == "" {
		return edgeKey{}, false
	}
	// Check both directions since tree edges are stored bidirectionally
	key := edgeKey{from: v, to: parent}
	if t.treeEdges[key] {
		return key, true // v is "from"
	}
	return edgeKey{from: parent, to: v}, false // v is "to"
}

// treeUpdate walks from v toward w, updating cut values along the path.
// Returns the LCA (lowest common ancestor) of v and w.
// The dir parameter tracks the direction of propagation for sign handling.
func (t *spanningTree) treeUpdate(s *layoutState, v, w string, cutvalue int, dir bool) string {
	// Walk up from v until we reach an ancestor of w
	// SEQ(low, lim, ulim) checks if lim is in range [low, ulim]
	for !(t.nodes[v].low <= t.nodes[w].lim && t.nodes[w].lim <= t.nodes[v].lim) {
		parentEdge, vIsFrom := t.getParentEdgeKey(v)
		if parentEdge == (edgeKey{}) {
			break // Reached root
		}

		// Direction flip: if v is on the "from" side of edge, keep direction; otherwise flip
		d := dir
		if !vIsFrom {
			d = !d
		}

		// Update cut value based on direction
		if d {
			t.cutValues[parentEdge] += cutvalue
			// Also update reverse direction
			rev := edgeKey{from: parentEdge.to, to: parentEdge.from}
			t.cutValues[rev] += cutvalue
		} else {
			t.cutValues[parentEdge] -= cutvalue
			rev := edgeKey{from: parentEdge.to, to: parentEdge.from}
			t.cutValues[rev] -= cutvalue
		}

		// Move up to parent
		v = t.nodes[v].parent
	}
	return v // This is the LCA
}

// invalidatePath marks nodes on the path from toNode to lca as needing DFS recomputation.
// Nodes are marked by setting low = -1.
func (t *spanningTree) invalidatePath(lca, toNode string) {
	lcaNode := t.nodes[lca]
	for toNode != lca {
		node := t.nodes[toNode]
		if node == nil || node.low == -1 {
			break // Already invalidated or doesn't exist
		}

		// Validate we haven't skipped the LCA (would indicate corrupted postorder values)
		// Following Graphviz's defensive check in invalidate_path()
		if lcaNode != nil && node.lim >= lcaNode.lim {
			break
		}

		node.low = -1 // Mark as needing recomputation

		// Move up to parent
		if node.parent == "" {
			break
		}
		toNode = node.parent
	}
}

// initLowLimValuesIncremental recomputes low/lim values starting from root,
// but skips subtrees where values haven't been invalidated (low != -1).
func (t *spanningTree) initLowLimValuesIncremental() {
	counter := 0
	visited := make(map[string]bool)

	var dfs func(v, parent string) int
	dfs = func(v, parent string) int {
		node := t.nodes[v]
		if node == nil {
			return counter
		}

		// Set parent relationship
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

// exchangeEdges swaps leave edge with enter edge and updates the tree.
// Uses incremental cut value updates following Graphviz's approach.
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
	var delta int
	if enterFromInTail && !enterToInTail {
		delta = slack
	} else if enterToInTail && !enterFromInTail {
		delta = -slack
	}

	// Apply delta to all nodes in the tail subtree (rerank)
	if delta != 0 {
		for id := range t.nodes {
			if t.isDescendant(id, tailNode) {
				s.nodes[id].rank += delta
			}
		}
	}

	// Get cut value of leaving edge BEFORE modifying tree
	leaveCutValue := t.cutValues[leave]

	// Modify the tree structure
	t.removeEdge(leave)
	t.addEdge(enter)

	// Incremental cut value update: walk from both endpoints to LCA
	lca := t.treeUpdate(s, enter.from, enter.to, leaveCutValue, true)
	t.treeUpdate(s, enter.to, enter.from, leaveCutValue, false)

	// Set entering edge cut value to negative of leaving edge
	t.setCutValue(enter.from, enter.to, -leaveCutValue)

	// Invalidate DFS attributes on paths that need recomputation
	t.invalidatePath(lca, enter.from)
	t.invalidatePath(lca, enter.to)

	// Recompute low/lim values (full recompute for now - incremental is complex)
	t.initLowLimValues()
}

// subtreeRemovalThreshold is the minimum node count for subtree removal optimization.
// Based on ELK's empirically determined threshold of 40 nodes.
const subtreeRemovalThreshold = 40

// removedLeaf represents a leaf node removed for subtree optimization.
type removedLeaf struct {
	nodeID    string  // The removed node
	edgeKey   edgeKey // The single edge connecting it
	isOutEdge bool    // True if edge goes from this node (nodeID->other)
	minlen    int     // Edge minlen for position calculation
}

// removeSubtreeLeaves removes leaf nodes (degree 1) from the graph.
// Returns the removed leaves in stack order (LIFO for reattachment).
// This reduces problem size significantly for chain-heavy graphs.
func (s *layoutState) removeSubtreeLeaves() []removedLeaf {
	if len(s.nodes) < subtreeRemovalThreshold {
		return nil
	}

	var stack []removedLeaf

	// Build degree counts
	degree := make(map[string]int)
	for id := range s.nodes {
		degree[id] = 0
	}
	for key := range s.edges {
		degree[key.from]++
		degree[key.to]++
	}

	// Find initial leaves
	queue := make([]string, 0)
	for id, d := range degree {
		if d == 1 {
			queue = append(queue, id)
		}
	}

	// Process leaves until none remain
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		// Find the single edge connected to this node
		var foundEdge edgeKey
		var isOutEdge bool
		found := false
		for key := range s.edges {
			if key.from == nodeID {
				foundEdge = key
				isOutEdge = true
				found = true
				break
			}
			if key.to == nodeID {
				foundEdge = key
				isOutEdge = false
				found = true
				break
			}
		}

		if !found {
			continue // Node already removed or isolated
		}

		// Get the other node
		other := foundEdge.to
		if !isOutEdge {
			other = foundEdge.from
		}

		// Get minlen for reattachment
		minlen := 1
		if edge := s.edges[foundEdge]; edge != nil && edge.minlen > 0 {
			minlen = edge.minlen
		}

		// Push to stack for LIFO reattachment
		stack = append(stack, removedLeaf{
			nodeID:    nodeID,
			edgeKey:   foundEdge,
			isOutEdge: isOutEdge,
			minlen:    minlen,
		})

		// Remove from graph
		delete(s.edges, foundEdge)
		delete(s.nodes, nodeID)

		// Check if other node became a leaf
		degree[other]--
		if degree[other] == 1 {
			queue = append(queue, other)
		}
	}

	return stack
}

// reattachSubtreeLeaves reattaches previously removed leaf nodes.
// Nodes are reattached in reverse removal order (LIFO).
func (s *layoutState) reattachSubtreeLeaves(stack []removedLeaf, savedNodes map[string]*layoutNode, savedEdges map[edgeKey]*layoutEdge) {
	// Process in reverse order (LIFO)
	for i := len(stack) - 1; i >= 0; i-- {
		leaf := stack[i]

		// Restore node
		s.nodes[leaf.nodeID] = savedNodes[leaf.nodeID]

		// Restore edge
		s.edges[leaf.edgeKey] = savedEdges[leaf.edgeKey]

		// Compute rank based on parent position
		var parentID string
		if leaf.isOutEdge {
			parentID = leaf.edgeKey.to
		} else {
			parentID = leaf.edgeKey.from
		}

		parentNode := s.nodes[parentID]
		if parentNode == nil {
			continue
		}

		if leaf.isOutEdge {
			// Edge: leaf -> parent, so leaf.rank + minlen = parent.rank
			s.nodes[leaf.nodeID].rank = parentNode.rank - leaf.minlen
		} else {
			// Edge: parent -> leaf, so parent.rank + minlen = leaf.rank
			s.nodes[leaf.nodeID].rank = parentNode.rank + leaf.minlen
		}
	}
}

// assignLayersNetworkSimplex uses the network simplex algorithm for optimal ranking.
func (s *layoutState) assignLayersNetworkSimplex() {
	if len(s.nodes) == 0 {
		return
	}

	// Step 1: Initial feasible ranking using longest path
	s.assignLayersLongestPath()

	// Save initial state for subtree removal and fallback
	initialRanks := make(map[string]int, len(s.nodes))
	savedNodes := make(map[string]*layoutNode, len(s.nodes))
	savedEdges := make(map[edgeKey]*layoutEdge, len(s.edges))
	for id, node := range s.nodes {
		initialRanks[id] = node.rank
		savedNodes[id] = node
	}
	for key, edge := range s.edges {
		savedEdges[key] = edge
	}

	// Step 2: Remove leaf subtrees for performance (ELK optimization)
	removedLeaves := s.removeSubtreeLeaves()

	// If graph became empty or trivial after removal, just reattach
	if len(s.nodes) <= 1 {
		s.reattachSubtreeLeaves(removedLeaves, savedNodes, savedEdges)
		return
	}

	// Step 3: Build feasible spanning tree
	tree := s.feasibleTree()

	// Step 4: Initialize tree values
	tree.initLowLimValues()
	tree.initCutValues(s)

	// Cache sorted edge keys for enterEdge() (graph edges don't change during simplex)
	tree.sortedEdgeKeys = make([]edgeKey, 0, len(s.edges))
	for key := range s.edges {
		tree.sortedEdgeKeys = append(tree.sortedEdgeKeys, key)
	}
	sort.Slice(tree.sortedEdgeKeys, func(i, j int) bool {
		if tree.sortedEdgeKeys[i].from != tree.sortedEdgeKeys[j].from {
			return tree.sortedEdgeKeys[i].from < tree.sortedEdgeKeys[j].from
		}
		return tree.sortedEdgeKeys[i].to < tree.sortedEdgeKeys[j].to
	})

	// Step 5: Iterate until optimal (no negative cut values).
	// Cap at 50000 to prevent excessive runtime on large graphs.
	maxIterations := min(max(len(s.nodes)*len(s.edges), 100), 50000)

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

	// Step 6: Reattach removed subtrees
	s.reattachSubtreeLeaves(removedLeaves, savedNodes, savedEdges)

	// Validate result - ensure all edge constraints are satisfied
	if !s.validateRanks() {
		// Fallback to initial longest-path ranking
		for id, rank := range initialRanks {
			if s.nodes[id] != nil {
				s.nodes[id].rank = rank
			}
		}
	}
}

// validateRanks checks that all edge constraints are satisfied.
func (s *layoutState) validateRanks() bool {
	for key, edge := range s.edges {
		fromRank := s.nodes[key.from].rank
		toRank := s.nodes[key.to].rank
		minlen := edge.minlen
		if minlen < 0 {
			minlen = 1
		}
		// minlen=0 is valid (e.g., rank group constraints)
		if toRank-fromRank < minlen {
			return false
		}
	}
	return true
}

// ============================================================================
// X Coordinate Network Simplex (Horizontal Position Assignment)
// Implements Gansner et al. 1993 Section 4
// ============================================================================

// xSimplexTolerance is the floating-point tolerance for comparisons.
const xSimplexTolerance = 1e-9

// xEdgeKey uniquely identifies an edge in the auxiliary graph.
type xEdgeKey struct {
	from, to string
}

// xEdgeKeyLess provides deterministic ordering for xEdgeKeys.
func xEdgeKeyLess(a, b xEdgeKey) bool {
	if a.from != b.from {
		return a.from < b.from
	}
	return a.to < b.to
}

// xAuxNode represents a node in the auxiliary graph for X coordinate simplex.
type xAuxNode struct {
	id string  // Node identifier
	x  float64 // Current X coordinate

	// Spanning tree data
	parent string // Parent in tree (empty for root)
	low    int    // Minimum postorder in subtree
	lim    int    // This node's postorder number

	// Original node reference (nil for edge-proxy nodes)
	origNode *layoutNode
}

// xAuxEdge represents an edge in the auxiliary graph.
type xAuxEdge struct {
	from   string  // Source node ID
	to     string  // Target node ID
	delta  float64 // Minimum length constraint (δ)
	weight float64 // Edge weight (Ω·ω)
}

// xSpanningTree holds the spanning tree for X simplex.
type xSpanningTree struct {
	nodes         map[string]*xAuxNode
	treeEdges     map[xEdgeKey]bool
	adj           map[string][]string // Adjacency lists for O(1) neighbor lookup
	cutValues     map[xEdgeKey]float64
	root          string
	sortedNodeIDs []string // Cached sorted node IDs for leaveEdge()
	searchIndex   int      // Circular search index for leaveEdge() (Graphviz S_i)
}

// xSimplexState holds state for X coordinate network simplex.
type xSimplexState struct {
	s *layoutState // Reference to layout state

	// Auxiliary graph
	auxNodes map[string]*xAuxNode
	auxEdges map[xEdgeKey]*xAuxEdge

	// Adjacency lists for auxiliary graph
	auxSucc map[string][]string
	auxPred map[string][]string

	// Spanning tree
	tree *xSpanningTree

	// Subtree removal state
	removedAuxLeaves []xRemovedLeaf

	// Cached sorted edge keys for enterEdge()
	sortedEdgeKeys []xEdgeKey
}

// xRemovedLeaf represents a leaf node removed from the auxiliary graph.
type xRemovedLeaf struct {
	nodeID    string    // The removed node
	edgeKey   xEdgeKey  // The single edge connecting it
	isOutEdge bool      // True if edge goes from this node
	delta     float64   // Edge delta for position calculation
	origNode  *layoutNode // Original layout node (nil for proxy nodes)
}

// assignXCoordinatesNetworkSimplex uses network simplex for X coordinates.
// Implements Gansner et al. 1993 Section 4.
func (s *layoutState) assignXCoordinatesNetworkSimplex() {
	if len(s.nodes) == 0 {
		return
	}

	// Initialize with simple placement (provides initial X values)
	s.assignXCoordinatesSimple()

	// Create simplex state
	xs := &xSimplexState{
		s:        s,
		auxNodes: make(map[string]*xAuxNode),
		auxEdges: make(map[xEdgeKey]*xAuxEdge),
		auxSucc:  make(map[string][]string),
		auxPred:  make(map[string][]string),
	}

	// Build auxiliary graph
	xs.buildAuxiliaryGraph()

	// Remove leaf subtrees for performance (ELK optimization)
	xs.removeAuxSubtreeLeaves()

	// If graph became empty or trivial, just reattach and exit
	if len(xs.auxNodes) <= 1 {
		xs.reattachAuxSubtreeLeaves()
		xs.extractCoordinates()
		s.normalizeXCoordinates()
		return
	}

	// Build initial feasible tree
	xs.tree = xs.xFeasibleTree()

	// Initialize tree values
	xs.tree.initLowLim()
	xs.initCutValues()

	// Cache sorted edge keys for enterEdge() (aux edges don't change during simplex)
	xs.sortedEdgeKeys = make([]xEdgeKey, 0, len(xs.auxEdges))
	for key := range xs.auxEdges {
		xs.sortedEdgeKeys = append(xs.sortedEdgeKeys, key)
	}
	sort.Slice(xs.sortedEdgeKeys, func(i, j int) bool {
		return xEdgeKeyLess(xs.sortedEdgeKeys[i], xs.sortedEdgeKeys[j])
	})

	// Iterate until optimal. Cap at 50000 to prevent excessive runtime.
	maxIterations := min(max(len(xs.auxNodes)*len(xs.auxEdges), 1000), 50000)
	for i := 0; i < maxIterations; i++ {
		leave, found := xs.leaveEdge()
		if !found {
			break // Optimal!
		}

		enter := xs.enterEdge(leave)
		if enter.from == "" {
			break // No valid entering edge
		}

		xs.exchangeEdges(leave, enter)
	}

	// Reattach removed subtrees
	xs.reattachAuxSubtreeLeaves()

	// Extract X coordinates back to layout nodes
	xs.extractCoordinates()

	// Normalize: shift so minimum X is 0
	s.normalizeXCoordinates()
}

// buildAuxiliaryGraph constructs the auxiliary graph per Gansner 1993 Section 4.
func (xs *xSimplexState) buildAuxiliaryGraph() {
	// Step 1: Add all original nodes (including dummies)
	for id, node := range xs.s.nodes {
		xs.auxNodes[id] = &xAuxNode{
			id:       id,
			x:        node.x,
			origNode: node,
		}
	}

	// Step 2: For each original edge, create edge-proxy node
	// The proxy node will be positioned at min(x_u, x_v)
	// Sort edges for deterministic proxy node IDs
	edgeKeys := make([]edgeKey, 0, len(xs.s.edges))
	for key := range xs.s.edges {
		edgeKeys = append(edgeKeys, key)
	}
	sort.Slice(edgeKeys, func(i, j int) bool {
		return edgeKeyLess(edgeKeys[i], edgeKeys[j])
	})

	for edgeNum, key := range edgeKeys {
		edge := xs.s.edges[key]
		proxyID := fmt.Sprintf("_e%d", edgeNum)

		// Calculate Omega weight based on endpoint types
		omega := xs.calcOmega(key)
		weight := omega * edge.weight

		// Create proxy node with initial position at min of endpoints
		fromX := xs.auxNodes[key.from].x
		toX := xs.auxNodes[key.to].x
		proxyX := math.Min(fromX, toX)

		xs.auxNodes[proxyID] = &xAuxNode{
			id: proxyID,
			x:  proxyX,
		}

		// Add edges (proxy -> from) and (proxy -> to) with delta=0
		// These edges point FROM the proxy TO the original nodes
		xs.addAuxEdge(proxyID, key.from, 0, weight)
		xs.addAuxEdge(proxyID, key.to, 0, weight)
	}

	// Step 3: Add same-rank separation edges
	// For each pair of adjacent nodes in a layer, add constraint edge
	for _, layer := range xs.s.layers {
		for i := 0; i < len(layer)-1; i++ {
			leftID := layer[i]
			rightID := layer[i+1]

			// delta = minimum separation between nodes
			// separation() returns center-to-center distance including half-widths
			delta := xs.s.separation(leftID, rightID)

			// Weight = 0 for separation constraints (they don't affect cost)
			xs.addAuxEdge(leftID, rightID, delta, 0)
		}
	}

	// Step 4: Add cross-layer anti-stacking edges (if enabled)
	// Skip for very large graphs (>2000 nodes including dummies) as it becomes too slow
	if xs.s.opts.PreventStacking && len(xs.s.nodes) <= 2000 {
		xs.addAntiStackingEdges()
	}
}

// addAntiStackingEdges adds separation constraints between connected nodes
// to ensure minimum horizontal gap (default 120px) for proper edge routing.
// This ensures the client-side same-side routing threshold works correctly:
// layout-positioned nodes get opposing sides, user-dragged overlaps get same-side.
func (xs *xSimplexState) addAntiStackingEdges() {
	// Get minimum separation (default to gap threshold for same-side routing)
	minSep := xs.s.opts.StackingMinSep
	if minSep <= 0 {
		minSep = defaultOverlapThreshold // 120px - matches client-side threshold
	}

	// For each edge in the original graph, add separation constraint
	// to ensure connected nodes have at least minSep horizontal gap
	for key := range xs.s.edges {
		fromNode := xs.s.nodes[key.from]
		toNode := xs.s.nodes[key.to]
		if fromNode == nil || toNode == nil {
			continue
		}

		// Skip self-loops
		if key.from == key.to {
			continue
		}

		// Add separation constraint unconditionally to ensure minSep gap.
		// Determine which node is on the left (smaller X center)
		fromCenter := fromNode.x + fromNode.width/2
		toCenter := toNode.x + toNode.width/2

		var leftID, rightID string
		var leftNode *layoutNode
		if fromCenter <= toCenter {
			leftID, rightID = key.from, key.to
			leftNode = fromNode
		} else {
			leftID, rightID = key.to, key.from
			leftNode = toNode
		}

		// Delta = left node width + minimum separation
		// This ensures: rightNode.x >= leftNode.x + leftNode.width + minSep
		// i.e., the gap between rightEdge(leftNode) and leftEdge(rightNode) >= minSep
		delta := leftNode.width + minSep

		// Add constraint edge: rightNode.x >= leftNode.x + delta
		// Weight = 0 (constraint only, doesn't affect objective)
		xs.addAuxEdge(leftID, rightID, delta, 0)
	}
}

// calcOmega computes the internal edge weight Ω per the paper.
// - 8 for virtual-virtual (dummy-dummy) edges
// - 2 for real-virtual (real-dummy) edges
// - 1 for real-real edges
func (xs *xSimplexState) calcOmega(key edgeKey) float64 {
	fromNode := xs.s.nodes[key.from]
	toNode := xs.s.nodes[key.to]

	fromDummy := fromNode != nil && fromNode.isDummy
	toDummy := toNode != nil && toNode.isDummy

	if fromDummy && toDummy {
		return 8.0 // virtual-virtual
	}
	if fromDummy || toDummy {
		return 2.0 // real-virtual
	}
	return 1.0 // real-real
}

// addAuxEdge adds an edge to the auxiliary graph.
func (xs *xSimplexState) addAuxEdge(from, to string, delta, weight float64) {
	key := xEdgeKey{from: from, to: to}
	xs.auxEdges[key] = &xAuxEdge{
		from:   from,
		to:     to,
		delta:  delta,
		weight: weight,
	}
	xs.auxSucc[from] = append(xs.auxSucc[from], to)
	xs.auxPred[to] = append(xs.auxPred[to], from)
}

// removeAuxSubtreeLeaves removes leaf nodes from the auxiliary graph.
// Returns the removed leaves in stack order (LIFO for reattachment).
func (xs *xSimplexState) removeAuxSubtreeLeaves() {
	if len(xs.auxNodes) < subtreeRemovalThreshold {
		return
	}

	// Build degree counts (both directions since aux graph is directed)
	degree := make(map[string]int)
	for id := range xs.auxNodes {
		degree[id] = len(xs.auxSucc[id]) + len(xs.auxPred[id])
	}

	// Find initial leaves (sorted for determinism)
	queue := make([]string, 0)
	for id, d := range degree {
		if d == 1 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	// Process leaves until none remain
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		// Skip if already removed
		if xs.auxNodes[nodeID] == nil {
			continue
		}

		// Recalculate degree in case neighbors were removed
		currentDegree := len(xs.auxSucc[nodeID]) + len(xs.auxPred[nodeID])
		if currentDegree != 1 {
			continue
		}

		// Find the single edge connected to this node (sorted for determinism)
		var foundKey xEdgeKey
		var isOutEdge bool
		found := false

		// Check outgoing edges (sorted)
		outgoing := make([]string, len(xs.auxSucc[nodeID]))
		copy(outgoing, xs.auxSucc[nodeID])
		sort.Strings(outgoing)
		for _, to := range outgoing {
			foundKey = xEdgeKey{from: nodeID, to: to}
			if xs.auxEdges[foundKey] != nil {
				isOutEdge = true
				found = true
				break
			}
		}

		// Check incoming edges if no outgoing found (sorted)
		if !found {
			incoming := make([]string, len(xs.auxPred[nodeID]))
			copy(incoming, xs.auxPred[nodeID])
			sort.Strings(incoming)
			for _, from := range incoming {
				foundKey = xEdgeKey{from: from, to: nodeID}
				if xs.auxEdges[foundKey] != nil {
					isOutEdge = false
					found = true
					break
				}
			}
		}

		if !found {
			continue
		}

		// Get the other node
		var other string
		if isOutEdge {
			other = foundKey.to
		} else {
			other = foundKey.from
		}

		// Get delta for reattachment
		delta := xs.auxEdges[foundKey].delta

		// Save original node reference if it exists
		var origNode *layoutNode
		if xs.auxNodes[nodeID] != nil {
			origNode = xs.auxNodes[nodeID].origNode
		}

		// Push to stack for LIFO reattachment
		xs.removedAuxLeaves = append(xs.removedAuxLeaves, xRemovedLeaf{
			nodeID:    nodeID,
			edgeKey:   foundKey,
			isOutEdge: isOutEdge,
			delta:     delta,
			origNode:  origNode,
		})

		// Remove edge from graph
		delete(xs.auxEdges, foundKey)

		// Update adjacency lists
		if isOutEdge {
			xs.auxSucc[nodeID] = removeFromSlice(xs.auxSucc[nodeID], other)
			xs.auxPred[other] = removeFromSlice(xs.auxPred[other], nodeID)
		} else {
			xs.auxPred[nodeID] = removeFromSlice(xs.auxPred[nodeID], other)
			xs.auxSucc[other] = removeFromSlice(xs.auxSucc[other], nodeID)
		}

		// Remove node from graph
		delete(xs.auxNodes, nodeID)

		// Check if other node became a leaf
		newDegree := len(xs.auxSucc[other]) + len(xs.auxPred[other])
		if newDegree == 1 {
			queue = append(queue, other)
		}
	}
}

// reattachAuxSubtreeLeaves reattaches previously removed leaf nodes.
func (xs *xSimplexState) reattachAuxSubtreeLeaves() {
	// Process in reverse order (LIFO)
	for i := len(xs.removedAuxLeaves) - 1; i >= 0; i-- {
		leaf := xs.removedAuxLeaves[i]

		// Get parent position
		var parentID string
		if leaf.isOutEdge {
			parentID = leaf.edgeKey.to
		} else {
			parentID = leaf.edgeKey.from
		}

		parentNode := xs.auxNodes[parentID]
		if parentNode == nil {
			continue
		}

		// Compute X based on parent position and delta
		var x float64
		if leaf.isOutEdge {
			// Edge: leaf -> parent, constraint: parent.x >= leaf.x + delta
			// So: leaf.x = parent.x - delta
			x = parentNode.x - leaf.delta
		} else {
			// Edge: parent -> leaf, constraint: leaf.x >= parent.x + delta
			// So: leaf.x = parent.x + delta
			x = parentNode.x + leaf.delta
		}

		// Restore node
		xs.auxNodes[leaf.nodeID] = &xAuxNode{
			id:       leaf.nodeID,
			x:        x,
			origNode: leaf.origNode,
		}

		// Restore edge
		xs.auxEdges[leaf.edgeKey] = &xAuxEdge{
			from:  leaf.edgeKey.from,
			to:    leaf.edgeKey.to,
			delta: leaf.delta,
		}

		// Restore adjacency
		if leaf.isOutEdge {
			xs.auxSucc[leaf.nodeID] = append(xs.auxSucc[leaf.nodeID], parentID)
			xs.auxPred[parentID] = append(xs.auxPred[parentID], leaf.nodeID)
		} else {
			xs.auxPred[leaf.nodeID] = append(xs.auxPred[leaf.nodeID], parentID)
			xs.auxSucc[parentID] = append(xs.auxSucc[parentID], leaf.nodeID)
		}
	}
}

// xSlack computes the slack of an auxiliary edge.
// slack = x[to] - x[from] - delta
// Positive slack means the edge is longer than required.
// Zero slack means the edge is "tight".
// Negative slack means constraint is violated.
func (xs *xSimplexState) xSlack(key xEdgeKey) float64 {
	edge := xs.auxEdges[key]
	if edge == nil {
		return 0
	}
	fromNode := xs.auxNodes[key.from]
	toNode := xs.auxNodes[key.to]
	if fromNode == nil || toNode == nil {
		return 0
	}
	return toNode.x - fromNode.x - edge.delta
}

// xFeasibleTree builds a tight spanning tree using frontier-based edge selection.
// Optimization: maintain frontier edges (edges with exactly one endpoint in tree)
// instead of scanning all edges. This reduces O(V*E) to O(V*F) where F is frontier size.
func (xs *xSimplexState) xFeasibleTree() *xSpanningTree {
	tree := &xSpanningTree{
		nodes:     make(map[string]*xAuxNode),
		treeEdges: make(map[xEdgeKey]bool),
		adj:       make(map[string][]string),
		cutValues: make(map[xEdgeKey]float64),
	}

	// Get sorted node IDs for determinism
	nodeIDs := make([]string, 0, len(xs.auxNodes))
	for id := range xs.auxNodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	if len(nodeIDs) == 0 {
		return tree
	}

	// Start with first node as root
	root := nodeIDs[0]
	tree.root = root
	tree.nodes[root] = &xAuxNode{
		id: root,
		x:  xs.auxNodes[root].x,
	}

	// Initialize frontier with edges incident to root
	// frontier[key] = true means this edge has exactly one endpoint in tree
	frontier := make(map[xEdgeKey]bool)
	for _, neighbor := range xs.auxSucc[root] {
		frontier[xEdgeKey{from: root, to: neighbor}] = true
	}
	for _, neighbor := range xs.auxPred[root] {
		frontier[xEdgeKey{from: neighbor, to: root}] = true
	}

	// Grow tree until all nodes included
	for len(tree.nodes) < len(xs.auxNodes) {
		// Find minimum slack edge in frontier
		var bestEdge xEdgeKey
		minSlack := math.Inf(1)
		treeToNonTree := true
		found := false

		// Sort frontier keys for determinism
		frontierKeys := make([]xEdgeKey, 0, len(frontier))
		for key := range frontier {
			frontierKeys = append(frontierKeys, key)
		}
		sort.Slice(frontierKeys, func(i, j int) bool {
			return xEdgeKeyLess(frontierKeys[i], frontierKeys[j])
		})

		for _, key := range frontierKeys {
			inTreeFrom := tree.nodes[key.from] != nil
			inTreeTo := tree.nodes[key.to] != nil

			// Skip if both endpoints now in tree (edge is no longer frontier)
			if inTreeFrom && inTreeTo {
				delete(frontier, key)
				continue
			}

			slack := xs.xSlack(key)
			if slack < minSlack || (slack == minSlack && xEdgeKeyLess(key, bestEdge)) {
				minSlack = slack
				bestEdge = key
				treeToNonTree = inTreeFrom
				found = true
			}
		}

		if !found {
			// Handle disconnected components - add any non-tree node
			for _, id := range nodeIDs {
				if tree.nodes[id] == nil {
					tree.nodes[id] = &xAuxNode{
						id: id,
						x:  xs.auxNodes[id].x,
					}
					// Add edges incident to new node to frontier
					for _, neighbor := range xs.auxSucc[id] {
						if tree.nodes[neighbor] != nil {
							continue // Both in tree
						}
						frontier[xEdgeKey{from: id, to: neighbor}] = true
					}
					for _, neighbor := range xs.auxPred[id] {
						if tree.nodes[neighbor] != nil {
							continue // Both in tree
						}
						frontier[xEdgeKey{from: neighbor, to: id}] = true
					}
					break
				}
			}
			continue
		}

		// Remove best edge from frontier
		delete(frontier, bestEdge)

		// Tighten edge if needed by adjusting tree node positions
		if math.Abs(minSlack) > xSimplexTolerance {
			delta := minSlack
			if !treeToNonTree {
				delta = -delta
			}
			for id := range tree.nodes {
				xs.auxNodes[id].x += delta
			}
		}

		// Add edge to tree
		tree.treeEdges[bestEdge] = true
		tree.treeEdges[xEdgeKey{from: bestEdge.to, to: bestEdge.from}] = true
		tree.adj[bestEdge.from] = append(tree.adj[bestEdge.from], bestEdge.to)
		tree.adj[bestEdge.to] = append(tree.adj[bestEdge.to], bestEdge.from)

		// Add new node to tree
		var newNodeID string
		if treeToNonTree {
			newNodeID = bestEdge.to
		} else {
			newNodeID = bestEdge.from
		}
		tree.nodes[newNodeID] = &xAuxNode{
			id: newNodeID,
			x:  xs.auxNodes[newNodeID].x,
		}

		// Add edges incident to new node to frontier (if other endpoint not in tree)
		for _, neighbor := range xs.auxSucc[newNodeID] {
			if tree.nodes[neighbor] != nil {
				// Other endpoint in tree - remove from frontier if present
				delete(frontier, xEdgeKey{from: newNodeID, to: neighbor})
				continue
			}
			frontier[xEdgeKey{from: newNodeID, to: neighbor}] = true
		}
		for _, neighbor := range xs.auxPred[newNodeID] {
			if tree.nodes[neighbor] != nil {
				// Other endpoint in tree - remove from frontier if present
				delete(frontier, xEdgeKey{from: neighbor, to: newNodeID})
				continue
			}
			frontier[xEdgeKey{from: neighbor, to: newNodeID}] = true
		}
	}

	// Cache sorted node IDs for deterministic iteration in leaveEdge()
	tree.sortedNodeIDs = make([]string, 0, len(tree.nodes))
	for id := range tree.nodes {
		tree.sortedNodeIDs = append(tree.sortedNodeIDs, id)
	}
	sort.Strings(tree.sortedNodeIDs)

	return tree
}

// initLowLim computes low/lim values for O(1) descendant queries.
func (t *xSpanningTree) initLowLim() {
	if len(t.nodes) == 0 {
		return
	}

	visited := make(map[string]bool)
	counter := 1

	var dfs func(v, parent string) int
	dfs = func(v, parent string) int {
		node := t.nodes[v]
		if node == nil {
			return counter
		}
		node.parent = parent
		low := counter

		visited[v] = true

		// Visit children (neighbors except parent) using adjacency list
		// Sort for deterministic DFS order
		neighbors := make([]string, len(t.adj[v]))
		copy(neighbors, t.adj[v])
		sort.Strings(neighbors)
		for _, neighbor := range neighbors {
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
func (t *xSpanningTree) isDescendant(v, u string) bool {
	uNode := t.nodes[u]
	vNode := t.nodes[v]
	if uNode == nil || vNode == nil {
		return false
	}
	return uNode.low <= vNode.lim && vNode.lim <= uNode.lim
}

// isTreeEdge returns true if the edge is in the spanning tree.
func (t *xSpanningTree) isTreeEdge(key xEdgeKey) bool {
	return t.treeEdges[key] || t.treeEdges[xEdgeKey{from: key.to, to: key.from}]
}

// postorderNodes returns nodes in postorder (children before parents).
func (t *xSpanningTree) postorderNodes() []string {
	result := make([]string, 0, len(t.nodes))
	visited := make(map[string]bool)

	var dfs func(v string)
	dfs = func(v string) {
		visited[v] = true
		// Use adjacency list, sorted for deterministic DFS order
		neighbors := make([]string, len(t.adj[v]))
		copy(neighbors, t.adj[v])
		sort.Strings(neighbors)
		for _, neighbor := range neighbors {
			if !visited[neighbor] {
				dfs(neighbor)
			}
		}
		result = append(result, v)
	}

	if t.root != "" {
		dfs(t.root)
	}
	return result
}

// initCutValues computes cut values for all tree edges (X simplex).
func (xs *xSimplexState) initCutValues() {
	postorder := xs.tree.postorderNodes()
	for _, v := range postorder {
		if v == xs.tree.root {
			continue
		}
		xs.assignCutValue(v)
	}
}

// assignCutValue computes the cut value for edge (v, parent[v]).
func (xs *xSimplexState) assignCutValue(v string) {
	treeNode := xs.tree.nodes[v]
	if treeNode == nil {
		return
	}
	parent := treeNode.parent
	if parent == "" {
		return
	}

	// Determine if v is the tail of the edge in the directed aux graph
	childIsTail := xs.auxEdges[xEdgeKey{from: v, to: parent}] != nil

	// Start with the weight of the tree edge itself
	var cutValue float64
	if edge := xs.auxEdges[xEdgeKey{from: v, to: parent}]; edge != nil {
		cutValue = edge.weight
	} else if edge := xs.auxEdges[xEdgeKey{from: parent, to: v}]; edge != nil {
		cutValue = edge.weight
	}

	// Process outgoing edges from v (using adjacency list for O(degree) instead of O(E))
	// Sort for determinism since adjacency lists are built from map iteration
	outgoing := make([]string, len(xs.auxSucc[v]))
	copy(outgoing, xs.auxSucc[v])
	sort.Strings(outgoing)
	for _, other := range outgoing {
		if other == parent {
			continue
		}
		key := xEdgeKey{from: v, to: other}
		edge := xs.auxEdges[key]
		if edge == nil {
			continue
		}

		// isOutEdge=true, pointsToHead = true == childIsTail
		pointsToHead := childIsTail

		if pointsToHead {
			cutValue += edge.weight
		} else {
			cutValue -= edge.weight
		}

		// If this is a tree edge to a child, propagate its cut value
		if xs.tree.isTreeEdge(key) && xs.tree.nodes[other] != nil && xs.tree.nodes[other].parent == v {
			childCut, ok := xs.tree.cutValues[xEdgeKey{from: other, to: v}]
			if !ok {
				childCut = xs.tree.cutValues[xEdgeKey{from: v, to: other}]
			}
			if pointsToHead {
				cutValue -= childCut
			} else {
				cutValue += childCut
			}
		}
	}

	// Process incoming edges to v (using adjacency list for O(degree) instead of O(E))
	// Sort for determinism since adjacency lists are built from map iteration
	incoming := make([]string, len(xs.auxPred[v]))
	copy(incoming, xs.auxPred[v])
	sort.Strings(incoming)
	for _, other := range incoming {
		if other == parent {
			continue
		}
		key := xEdgeKey{from: other, to: v}
		edge := xs.auxEdges[key]
		if edge == nil {
			continue
		}

		// isOutEdge=false, pointsToHead = false == childIsTail
		pointsToHead := !childIsTail

		if pointsToHead {
			cutValue += edge.weight
		} else {
			cutValue -= edge.weight
		}

		// If this is a tree edge to a child, propagate its cut value
		if xs.tree.isTreeEdge(key) && xs.tree.nodes[other] != nil && xs.tree.nodes[other].parent == v {
			childCut, ok := xs.tree.cutValues[xEdgeKey{from: other, to: v}]
			if !ok {
				childCut = xs.tree.cutValues[xEdgeKey{from: v, to: other}]
			}
			if pointsToHead {
				cutValue -= childCut
			} else {
				cutValue += childCut
			}
		}
	}

	xs.tree.cutValues[xEdgeKey{from: v, to: parent}] = cutValue
	xs.tree.cutValues[xEdgeKey{from: parent, to: v}] = cutValue
}

// leaveEdge finds a tree edge with negative cut value (X simplex).
func (xs *xSimplexState) leaveEdge() (xEdgeKey, bool) {
	// Use cached sorted node IDs with circular search (Graphviz S_i pattern)
	n := len(xs.tree.sortedNodeIDs)
	if n == 0 {
		return xEdgeKey{}, false
	}

	var best xEdgeKey
	bestCut := 0.0
	found := 0
	startIdx := xs.tree.searchIndex

	// Search from startIdx to end
	for i := startIdx; i < n && found < leaveEdgeSearchLimit; i++ {
		v := xs.tree.sortedNodeIDs[i]
		node := xs.tree.nodes[v]
		if node == nil || node.parent == "" {
			continue
		}
		key := xEdgeKey{from: v, to: node.parent}
		if cut := xs.tree.cutValues[key]; cut < -xSimplexTolerance {
			if cut < bestCut {
				best = key
				bestCut = cut
			}
			found++
			xs.tree.searchIndex = i + 1
		}
	}

	// Wrap around: search from 0 to startIdx
	if found < leaveEdgeSearchLimit && startIdx > 0 {
		for i := 0; i < startIdx && found < leaveEdgeSearchLimit; i++ {
			v := xs.tree.sortedNodeIDs[i]
			node := xs.tree.nodes[v]
			if node == nil || node.parent == "" {
				continue
			}
			key := xEdgeKey{from: v, to: node.parent}
			if cut := xs.tree.cutValues[key]; cut < -xSimplexTolerance {
				if cut < bestCut {
					best = key
					bestCut = cut
				}
				found++
				xs.tree.searchIndex = i + 1
			}
		}
	}

	// Reset search index if we wrapped around completely
	if xs.tree.searchIndex >= n {
		xs.tree.searchIndex = 0
	}

	if found > 0 {
		return best, true
	}
	return xEdgeKey{}, false
}

// enterEdge finds the best non-tree edge to add when removing leave (X simplex).
func (xs *xSimplexState) enterEdge(leave xEdgeKey) xEdgeKey {
	v, w := leave.from, leave.to

	// Ensure v->w matches graph direction
	if xs.auxEdges[xEdgeKey{from: v, to: w}] == nil {
		v, w = w, v
	}

	vNode := xs.tree.nodes[v]
	wNode := xs.tree.nodes[w]
	if vNode == nil || wNode == nil {
		return xEdgeKey{}
	}

	tailLabel := vNode
	flip := false
	if vNode.lim > wNode.lim {
		tailLabel = wNode
		flip = true
	}

	// Use cached sorted edge keys for determinism (built before simplex loop)
	// Find min-slack non-tree edge crossing the cut correctly
	var best xEdgeKey
	bestSlack := math.Inf(1)
	found := false

	for _, key := range xs.sortedEdgeKeys {
		if xs.tree.isTreeEdge(key) {
			continue
		}

		fromInTail := xs.tree.isDescendant(key.from, tailLabel.id)
		toInTail := xs.tree.isDescendant(key.to, tailLabel.id)

		// Edge must cross the cut in the right direction
		if flip != fromInTail || flip == toInTail {
			continue
		}

		slack := xs.xSlack(key)
		if slack < bestSlack {
			bestSlack = slack
			best = key
			found = true
		}
	}

	if !found {
		return xEdgeKey{}
	}
	return best
}

// getParentEdgeKey returns the edge key connecting v to its parent in the X simplex tree.
func (t *xSpanningTree) getParentEdgeKey(v string) (xEdgeKey, bool) {
	node := t.nodes[v]
	if node == nil || node.parent == "" {
		return xEdgeKey{}, false
	}
	parent := node.parent
	// Check both directions since tree edges are stored bidirectionally
	key := xEdgeKey{from: v, to: parent}
	if t.treeEdges[key] {
		return key, true // v is "from"
	}
	return xEdgeKey{from: parent, to: v}, false // v is "to"
}

// xTreeUpdate walks from v toward w, updating cut values along the path (X simplex).
// Returns the LCA (lowest common ancestor) of v and w.
func (t *xSpanningTree) xTreeUpdate(v, w string, cutvalue float64, dir bool) string {
	// Walk up from v until we reach an ancestor of w
	for !(t.nodes[v].low <= t.nodes[w].lim && t.nodes[w].lim <= t.nodes[v].lim) {
		parentEdge, vIsFrom := t.getParentEdgeKey(v)
		if parentEdge == (xEdgeKey{}) {
			break // Reached root
		}

		// Direction flip: if v is on the "from" side of edge, keep direction; otherwise flip
		d := dir
		if !vIsFrom {
			d = !d
		}

		// Update cut value based on direction
		if d {
			t.cutValues[parentEdge] += cutvalue
			rev := xEdgeKey{from: parentEdge.to, to: parentEdge.from}
			t.cutValues[rev] += cutvalue
		} else {
			t.cutValues[parentEdge] -= cutvalue
			rev := xEdgeKey{from: parentEdge.to, to: parentEdge.from}
			t.cutValues[rev] -= cutvalue
		}

		// Move up to parent
		v = t.nodes[v].parent
	}
	return v // This is the LCA
}

// xInvalidatePath marks nodes on the path from toNode to lca as needing DFS recomputation.
func (t *xSpanningTree) xInvalidatePath(lca, toNode string) {
	lcaNode := t.nodes[lca]
	for toNode != lca {
		node := t.nodes[toNode]
		if node == nil || node.low == -1 {
			break // Already invalidated or doesn't exist
		}

		// Validate we haven't skipped the LCA (would indicate corrupted postorder values)
		// Following Graphviz's defensive check in invalidate_path()
		if lcaNode != nil && node.lim >= lcaNode.lim {
			break
		}

		node.low = -1 // Mark as needing recomputation

		if node.parent == "" {
			break
		}
		toNode = node.parent
	}
}

// exchangeEdges swaps leave with enter and updates tree (X simplex).
// Uses incremental cut value updates following Graphviz's approach.
func (xs *xSimplexState) exchangeEdges(leave, enter xEdgeKey) {
	slack := xs.xSlack(enter)

	// Identify subtree to shift
	tailNode := leave.from
	if xs.tree.nodes[leave.to] != nil && xs.tree.nodes[leave.from] != nil {
		if xs.tree.nodes[leave.to].lim < xs.tree.nodes[leave.from].lim {
			tailNode = leave.to
		}
	}

	// Determine shift direction
	enterFromInTail := xs.tree.isDescendant(enter.from, tailNode)
	enterToInTail := xs.tree.isDescendant(enter.to, tailNode)

	var delta float64
	if enterFromInTail && !enterToInTail {
		delta = slack
	} else if enterToInTail && !enterFromInTail {
		delta = -slack
	}

	// Apply shift to tail subtree
	if math.Abs(delta) > xSimplexTolerance {
		for id := range xs.tree.nodes {
			if xs.tree.isDescendant(id, tailNode) {
				xs.auxNodes[id].x += delta
			}
		}
	}

	// Get cut value of leaving edge BEFORE modifying tree
	leaveCutValue := xs.tree.cutValues[leave]

	// Update tree structure: remove leave edge
	delete(xs.tree.treeEdges, leave)
	delete(xs.tree.treeEdges, xEdgeKey{from: leave.to, to: leave.from})
	delete(xs.tree.cutValues, leave)
	delete(xs.tree.cutValues, xEdgeKey{from: leave.to, to: leave.from})
	xs.tree.adj[leave.from] = removeFromSlice(xs.tree.adj[leave.from], leave.to)
	xs.tree.adj[leave.to] = removeFromSlice(xs.tree.adj[leave.to], leave.from)

	// Add enter edge
	xs.tree.treeEdges[enter] = true
	xs.tree.treeEdges[xEdgeKey{from: enter.to, to: enter.from}] = true
	xs.tree.adj[enter.from] = append(xs.tree.adj[enter.from], enter.to)
	xs.tree.adj[enter.to] = append(xs.tree.adj[enter.to], enter.from)

	// Incremental cut value update: walk from both endpoints to LCA
	lca := xs.tree.xTreeUpdate(enter.from, enter.to, leaveCutValue, true)
	xs.tree.xTreeUpdate(enter.to, enter.from, leaveCutValue, false)

	// Set entering edge cut value to negative of leaving edge
	xs.tree.cutValues[enter] = -leaveCutValue
	xs.tree.cutValues[xEdgeKey{from: enter.to, to: enter.from}] = -leaveCutValue

	// Invalidate DFS attributes on paths that need recomputation
	xs.tree.xInvalidatePath(lca, enter.from)
	xs.tree.xInvalidatePath(lca, enter.to)

	// Recompute low/lim values
	xs.tree.initLowLim()
}

// extractCoordinates copies X values from auxiliary nodes to layout nodes.
func (xs *xSimplexState) extractCoordinates() {
	for _, auxNode := range xs.auxNodes {
		if auxNode.origNode != nil {
			auxNode.origNode.x = auxNode.x
		}
	}
}

// normalizeXCoordinates shifts all X so minimum is 0.
func (s *layoutState) normalizeXCoordinates() {
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
