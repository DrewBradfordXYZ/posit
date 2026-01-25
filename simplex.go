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
	nodes     map[string]*treeNode
	treeEdges map[edgeKey]bool // Which edges are in the tree
	cutValues map[edgeKey]int  // Cut value for each tree edge
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
		if t.isTreeEdge(key) {
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
func (t *spanningTree) leaveEdge() (edgeKey, bool) {
	// Get nodes in sorted order for deterministic iteration
	nodeIDs := make([]string, 0, len(t.nodes))
	for id := range t.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Look for edges where child->parent has negative cut value
	for _, v := range nodeIDs {
		node := t.nodes[v]
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

	// Get edges in sorted order for deterministic iteration
	edgeKeys := make([]edgeKey, 0, len(s.edges))
	for key := range s.edges {
		edgeKeys = append(edgeKeys, key)
	}
	sort.Slice(edgeKeys, func(i, j int) bool {
		if edgeKeys[i].from != edgeKeys[j].from {
			return edgeKeys[i].from < edgeKeys[j].from
		}
		return edgeKeys[i].to < edgeKeys[j].to
	})

	// Find non-tree edge with minimum slack that crosses the cut correctly.
	// The entering edge must go from the tail side to the head side.
	var best edgeKey
	bestSlack := math.MaxInt
	found := false

	for _, key := range edgeKeys {
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
	nodes     map[string]*xAuxNode
	treeEdges map[xEdgeKey]bool
	cutValues map[xEdgeKey]float64
	root      string
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

	// Build initial feasible tree
	xs.tree = xs.xFeasibleTree()

	// Initialize tree values
	xs.tree.initLowLim()
	xs.initCutValues()

	// Iterate until optimal
	maxIterations := max(len(xs.auxNodes)*len(xs.auxEdges), 1000)
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
	edgeNum := 0
	for key, edge := range xs.s.edges {
		proxyID := fmt.Sprintf("_e%d", edgeNum)
		edgeNum++

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

// xFeasibleTree builds a tight spanning tree from current positions.
func (xs *xSimplexState) xFeasibleTree() *xSpanningTree {
	tree := &xSpanningTree{
		nodes:     make(map[string]*xAuxNode),
		treeEdges: make(map[xEdgeKey]bool),
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

	// Grow tree using minimum slack edges
	for len(tree.nodes) < len(xs.auxNodes) {
		var bestEdge xEdgeKey
		minSlack := math.Inf(1)
		treeToNonTree := true
		found := false

		// Get sorted edge keys for determinism
		edgeKeys := make([]xEdgeKey, 0, len(xs.auxEdges))
		for key := range xs.auxEdges {
			edgeKeys = append(edgeKeys, key)
		}
		sort.Slice(edgeKeys, func(i, j int) bool {
			return xEdgeKeyLess(edgeKeys[i], edgeKeys[j])
		})

		for _, key := range edgeKeys {
			inTreeFrom := tree.nodes[key.from] != nil
			inTreeTo := tree.nodes[key.to] != nil

			if inTreeFrom == inTreeTo {
				continue // Both in or both out
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
					break
				}
			}
			continue
		}

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
	}

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

		// Visit children (neighbors except parent)
		for key := range t.treeEdges {
			if key.from == v && !visited[key.to] {
				counter = dfs(key.to, v)
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
		for key := range t.treeEdges {
			if key.from == v && !visited[key.to] {
				dfs(key.to)
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

	// Process all edges incident to v (except the edge to parent)
	for key, edge := range xs.auxEdges {
		if key.from != v && key.to != v {
			continue
		}

		isOutEdge := key.from == v
		var other string
		if isOutEdge {
			other = key.to
		} else {
			other = key.from
		}

		if other == parent {
			continue
		}

		// pointsToHead: does this edge point in the same direction as the tree edge?
		pointsToHead := isOutEdge == childIsTail

		if pointsToHead {
			cutValue += edge.weight
		} else {
			cutValue -= edge.weight
		}

		// If this is a tree edge to a child, propagate its cut value
		if xs.tree.isTreeEdge(key) && xs.tree.nodes[other] != nil && xs.tree.nodes[other].parent == v {
			// Use ok idiom since cut value of 0 is valid (balanced edge)
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
	// Get nodes in sorted order for deterministic iteration
	nodeIDs := make([]string, 0, len(xs.tree.nodes))
	for id := range xs.tree.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, v := range nodeIDs {
		node := xs.tree.nodes[v]
		if node == nil || node.parent == "" {
			continue
		}
		key := xEdgeKey{from: v, to: node.parent}
		if cut := xs.tree.cutValues[key]; cut < -xSimplexTolerance {
			return key, true
		}
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

	// Get edges in sorted order for determinism
	edgeKeys := make([]xEdgeKey, 0, len(xs.auxEdges))
	for key := range xs.auxEdges {
		edgeKeys = append(edgeKeys, key)
	}
	sort.Slice(edgeKeys, func(i, j int) bool {
		return xEdgeKeyLess(edgeKeys[i], edgeKeys[j])
	})

	// Find min-slack non-tree edge crossing the cut correctly
	var best xEdgeKey
	bestSlack := math.Inf(1)
	found := false

	for _, key := range edgeKeys {
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

// exchangeEdges swaps leave with enter and updates tree (X simplex).
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

	// Update tree structure: remove leave edge
	delete(xs.tree.treeEdges, leave)
	delete(xs.tree.treeEdges, xEdgeKey{from: leave.to, to: leave.from})
	delete(xs.tree.cutValues, leave)
	delete(xs.tree.cutValues, xEdgeKey{from: leave.to, to: leave.from})

	// Add enter edge
	xs.tree.treeEdges[enter] = true
	xs.tree.treeEdges[xEdgeKey{from: enter.to, to: enter.from}] = true

	// Recompute tree values
	xs.tree.initLowLim()
	xs.initCutValues()
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
