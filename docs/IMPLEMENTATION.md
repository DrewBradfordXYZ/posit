# Posit Implementation Guide

This document provides step-by-step guidance for implementing the Sugiyama graph layout algorithm in Go. It maps the dagre JavaScript reference implementation to idiomatic Go patterns.

## Design Philosophy

Posit is a **general-purpose library** first. The implementation prioritizes:

1. **Sound algorithms** - Follow established theory (Sugiyama, Brandes-Köpf, etc.)
2. **Standard conventions** - Top-left coordinates, screen coordinate system
3. **Clean abstractions** - Phases that work for any directed graph
4. **Extensibility** - Easy to swap algorithms per phase

Project-specific optimizations can be layered on top by consumers. The core library should be useful for any hierarchical graph visualization.

## Table of Contents

1. [Implementation Order](#implementation-order)
2. [state.go - Layout State](#statego---layout-state)
3. [acyclic.go - Cycle Removal](#acyclicgo---cycle-removal)
4. [rank.go - Layer Assignment](#rankgo---layer-assignment)
5. [normalize.go - Dummy Nodes](#normalizego---dummy-nodes)
6. [order.go - Crossing Minimization](#ordergo---crossing-minimization)
7. [position.go - Coordinate Assignment](#positiongo---coordinate-assignment)
8. [route.go - Edge Routing](#routego---edge-routing)
9. [Go-Specific Patterns](#go-specific-patterns)
10. [Debugging Tips](#debugging-tips)

---

## Implementation Order

### Recommended Order

```
1. state.go      - Foundation: internal data structures
2. acyclic.go    - Phase 1: break cycles (simple DFS)
3. rank.go       - Phase 2: assign layers (longest path first)
4. normalize.go  - Phase 3: split long edges (straightforward)
5. order.go      - Phase 4: minimize crossings (most complex)
6. position.go   - Phase 5: assign coordinates
7. route.go      - Phase 6: build final output
```

### What Can Be Stubbed Initially

For early testing, stub these with minimal implementations:

```go
// Stub for order.go - just use initial order
func (s *layoutState) minimizeCrossings() {
    // Initial implementation: keep DFS traversal order
    // Nodes already have order from rank assignment
}

// Stub for position.go - simple grid layout
func (s *layoutState) assignCoordinates() {
    for rank, layer := range s.layers {
        y := float64(rank) * s.opts.RankSep
        x := 0.0
        for _, nodeID := range layer {
            node := s.nodes[nodeID]
            node.x = x + node.width/2
            node.y = y + node.height/2
            x += node.width + s.opts.NodeSep
        }
    }
}
```

### Dependencies Between Phases

```
                    +-----------+
                    | state.go  |  (foundation for all)
                    +-----+-----+
                          |
                    +-----v-----+
                    | acyclic   |  (requires: adjacency lists)
                    +-----+-----+
                          |
                    +-----v-----+
                    | rank      |  (requires: acyclic graph)
                    +-----+-----+
                          |
                    +-----v-----+
                    | normalize |  (requires: rank assignments)
                    +-----+-----+
                          |
                    +-----v-----+
                    | order     |  (requires: layers, dummy nodes)
                    +-----+-----+
                          |
                    +-----v-----+
                    | position  |  (requires: node order within layers)
                    +-----+-----+
                          |
                    +-----v-----+
                    | route     |  (requires: coordinates, dummy positions)
                    +-----+-----+
```

---

## state.go - Layout State

### Core Internal Structures

The internal state tracks everything the algorithm needs across phases:

```go
package posit

// layoutState holds all intermediate data during layout computation.
// This is the "working copy" that gets mutated through each phase.
type layoutState struct {
    opts Options

    // Node data (keyed by node ID)
    nodes map[string]*layoutNode

    // Edge data (keyed by "from->to" or use edgeKey struct)
    edges map[edgeKey]*layoutEdge

    // Adjacency lists for graph traversal
    successors   map[string][]string // node -> outgoing neighbors
    predecessors map[string][]string // node -> incoming neighbors

    // Layer assignments (populated by rank phase)
    // layers[rank] = list of node IDs at that rank
    layers [][]string

    // Tracking for undo operations
    reversedEdges []edgeKey       // edges reversed to break cycles
    dummyChains   []string        // first dummy in each chain

    // ID generation for dummy nodes
    dummyCounter int
}

// layoutNode holds per-node state during layout.
type layoutNode struct {
    id     string
    width  float64
    height float64

    // Assigned during rank phase
    rank int

    // Assigned during order phase
    order int

    // Assigned during position phase
    x, y float64

    // For dummy nodes
    isDummy   bool
    edgeLabel *layoutEdge // if dummy, which edge it belongs to
}

// edgeKey uniquely identifies an edge.
type edgeKey struct {
    from string
    to   string
}

// layoutEdge holds per-edge state during layout.
type layoutEdge struct {
    key      edgeKey
    weight   float64 // for crossing minimization (default: 1)
    minlen   int     // minimum layers to span (default: 1)
    reversed bool    // was this edge reversed to break cycle?

    // Populated during route phase
    points []EdgePoint

    // For tracking dummy chains
    labelRank int // rank where edge label should go (if any)
}
```

### Translating from Public Graph to Internal State

```go
func newLayoutState(g *Graph, opts Options) *layoutState {
    s := &layoutState{
        opts:         opts,
        nodes:        make(map[string]*layoutNode, len(g.nodes)),
        edges:        make(map[edgeKey]*layoutEdge, len(g.edges)),
        successors:   make(map[string][]string, len(g.nodes)),
        predecessors: make(map[string][]string, len(g.nodes)),
    }

    // Copy nodes
    for id, n := range g.nodes {
        s.nodes[id] = &layoutNode{
            id:     id,
            width:  n.width,
            height: n.height,
            rank:   -1, // unassigned
            order:  -1, // unassigned
        }
        // Initialize empty adjacency lists
        s.successors[id] = nil
        s.predecessors[id] = nil
    }

    // Copy edges and build adjacency lists
    for _, e := range g.edges {
        key := edgeKey{from: e.from, to: e.to}
        s.edges[key] = &layoutEdge{
            key:    key,
            weight: 1,
            minlen: 1,
        }
        s.successors[e.from] = append(s.successors[e.from], e.to)
        s.predecessors[e.to] = append(s.predecessors[e.to], e.from)
    }

    return s
}
```

### Building Final Output

```go
func (s *layoutState) buildLayout() *Layout {
    result := &Layout{
        Nodes: make(map[string]NodeLayout, len(s.nodes)),
        Edges: make(map[string]EdgeLayout, len(s.edges)),
    }

    // Copy node positions (skip dummy nodes)
    for id, node := range s.nodes {
        if node.isDummy {
            continue
        }
        result.Nodes[id] = NodeLayout{
            Position: Position{X: node.x, Y: node.y},
            Width:    node.width,
            Height:   node.height,
        }
    }

    // Copy edge routes
    for key, edge := range s.edges {
        // Use original direction, not reversed
        edgeID := key.from + "->" + key.to
        result.Edges[edgeID] = EdgeLayout{
            Points: edge.points,
        }
    }

    return result
}
```

---

## acyclic.go - Cycle Removal

### DFS Implementation

The goal is to find and reverse edges that create cycles. This is a simple DFS that tracks the current path.

```go
// makeAcyclic reverses edges to break cycles in the graph.
// Reversed edges are tracked for later restoration.
func (s *layoutState) makeAcyclic() {
    visited := make(map[string]bool, len(s.nodes))
    onStack := make(map[string]bool) // current DFS path

    var dfs func(v string)
    dfs = func(v string) {
        if visited[v] {
            return
        }
        visited[v] = true
        onStack[v] = true

        // Iterate over successors (copy slice to allow modification)
        for _, w := range s.successors[v] {
            if onStack[w] {
                // Back edge found - reverse it
                s.reverseEdge(edgeKey{from: v, to: w})
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

    // Track for undo
    s.reversedEdges = append(s.reversedEdges, newKey)
}

// Helper to remove a string from a slice
func removeString(slice []string, s string) []string {
    for i, v := range slice {
        if v == s {
            return append(slice[:i], slice[i+1:]...)
        }
    }
    return slice
}
```

### Why DFS for Cycle Detection?

- **Simple**: Easy to implement and understand
- **Fast**: O(V + E) time complexity
- **Sufficient**: Doesn't produce optimal results (minimum feedback arc set is NP-hard) but works well in practice

Dagre also supports a "greedy" acyclicer that tries to minimize the number of reversed edges. For most use cases, the simple DFS is adequate.

---

## rank.go - Layer Assignment

### Longest-Path Algorithm

This is the simplest ranking algorithm. It assigns each node to the lowest possible layer while respecting edge constraints.

```go
// assignLayers assigns each node to a rank (layer) using longest-path algorithm.
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
            if edge != nil {
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

    // Start from source nodes (no predecessors)
    for id := range s.nodes {
        if len(s.predecessors[id]) == 0 {
            dfs(id)
        }
    }

    // Handle any unvisited nodes (disconnected components)
    for id := range s.nodes {
        if !visited[id] {
            dfs(id)
        }
    }
}

// normalizeRanks shifts all ranks so minimum is 0.
func (s *layoutState) normalizeRanks() {
    minRank := 0
    for _, node := range s.nodes {
        if node.rank < minRank {
            minRank = node.rank
        }
    }

    if minRank < 0 {
        for _, node := range s.nodes {
            node.rank -= minRank
        }
    }
}

// buildLayers creates the layers slice from node ranks.
func (s *layoutState) buildLayers() {
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

    // Assign nodes to layers
    for id, node := range s.nodes {
        s.layers[node.rank] = append(s.layers[node.rank], id)
    }
}
```

### Handling Disconnected Components

The longest-path algorithm naturally handles disconnected components by:
1. Starting from all source nodes
2. Then processing any remaining unvisited nodes

Each disconnected component will have its own set of ranks, which is correct behavior.

---

## normalize.go - Dummy Nodes

### When to Create Dummy Nodes

Dummy nodes are created when an edge spans more than one layer. Each dummy represents a "bend point" where the edge passes through a layer.

```
Before normalization:       After normalization:

   A (rank 0)                  A (rank 0)
   |                           |
   |                           D1 (rank 1, dummy)
   |                           |
   |                           D2 (rank 2, dummy)
   |                           |
   B (rank 3)                  B (rank 3)
```

### Implementation

```go
// addDummyNodes splits edges that span multiple layers.
func (s *layoutState) addDummyNodes() {
    // Collect edges to process (iterate over copy to allow modification)
    edgesToProcess := make([]edgeKey, 0, len(s.edges))
    for key := range s.edges {
        edgesToProcess = append(edgesToProcess, key)
    }

    for _, key := range edgesToProcess {
        s.normalizeEdge(key)
    }
}

// normalizeEdge splits a single edge if it spans multiple layers.
func (s *layoutState) normalizeEdge(key edgeKey) {
    edge := s.edges[key]
    if edge == nil {
        return
    }

    vNode := s.nodes[key.from]
    wNode := s.nodes[key.to]

    vRank := vNode.rank
    wRank := wNode.rank

    // If edge spans only one layer, nothing to do
    if wRank == vRank+1 {
        return
    }

    // Remove original edge
    s.removeEdge(key)

    // Create chain of dummy nodes
    v := key.from
    var firstDummy string

    for rank := vRank + 1; rank < wRank; rank++ {
        // Create dummy node
        dummyID := s.newDummyID()
        dummy := &layoutNode{
            id:        dummyID,
            width:     0,
            height:    0,
            rank:      rank,
            isDummy:   true,
            edgeLabel: edge,
        }
        s.nodes[dummyID] = dummy
        s.successors[dummyID] = nil
        s.predecessors[dummyID] = nil

        // Add to layer
        s.layers[rank] = append(s.layers[rank], dummyID)

        // Track first dummy for undo
        if firstDummy == "" {
            firstDummy = dummyID
        }

        // Create edge from previous node to dummy
        s.addEdge(edgeKey{from: v, to: dummyID}, edge.weight)
        v = dummyID
    }

    // Create final edge from last dummy to target
    s.addEdge(edgeKey{from: v, to: key.to}, edge.weight)

    // Track dummy chain for later reconstruction
    if firstDummy != "" {
        s.dummyChains = append(s.dummyChains, firstDummy)
    }
}

// newDummyID generates a unique ID for a dummy node.
func (s *layoutState) newDummyID() string {
    s.dummyCounter++
    return fmt.Sprintf("_d%d", s.dummyCounter)
}

// addEdge adds an edge with the given weight.
func (s *layoutState) addEdge(key edgeKey, weight float64) {
    s.edges[key] = &layoutEdge{
        key:    key,
        weight: weight,
        minlen: 1,
    }
    s.successors[key.from] = append(s.successors[key.from], key.to)
    s.predecessors[key.to] = append(s.predecessors[key.to], key.from)
}

// removeEdge removes an edge from the graph.
func (s *layoutState) removeEdge(key edgeKey) {
    delete(s.edges, key)
    s.successors[key.from] = removeString(s.successors[key.from], key.to)
    s.predecessors[key.to] = removeString(s.predecessors[key.to], key.from)
}
```

### Edge-to-Dummy Mapping

The `edgeLabel` field on each dummy node points back to the original edge. This allows reconstruction during the routing phase:

```go
type layoutNode struct {
    // ...
    edgeLabel *layoutEdge // if dummy, which edge it belongs to
}
```

During `routeEdges()`, we walk each dummy chain, collecting coordinates to build the edge's points array.

---

## order.go - Crossing Minimization

This is the most complex phase. The goal is to reorder nodes within each layer to minimize edge crossings.

### Barycenter Calculation

The barycenter of a node is the average position of its neighbors in the adjacent layer:

```go
// barycenter calculates the weighted average position of a node's neighbors.
// Returns (barycenter, weight, hasNeighbors).
func (s *layoutState) barycenter(nodeID string, neighborFn func(string) []string) (float64, float64, bool) {
    neighbors := neighborFn(nodeID)
    if len(neighbors) == 0 {
        return 0, 0, false
    }

    sum := 0.0
    weight := 0.0

    for _, neighborID := range neighbors {
        neighbor := s.nodes[neighborID]
        // Get edge weight
        var edgeWeight float64 = 1.0
        if edge := s.edges[edgeKey{from: nodeID, to: neighborID}]; edge != nil {
            edgeWeight = edge.weight
        } else if edge := s.edges[edgeKey{from: neighborID, to: nodeID}]; edge != nil {
            edgeWeight = edge.weight
        }

        sum += float64(neighbor.order) * edgeWeight
        weight += edgeWeight
    }

    return sum / weight, weight, true
}

// barycenterEntry holds barycenter data for sorting.
type barycenterEntry struct {
    nodeID     string
    barycenter float64
    weight     float64
    hasValue   bool
}
```

### Layer Sweep Implementation

```go
// minimizeCrossings reorders nodes within layers to reduce edge crossings.
func (s *layoutState) minimizeCrossings() {
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
}

// sweepDown reorders each layer based on predecessors (top-to-bottom).
func (s *layoutState) sweepDown() {
    for rank := 1; rank < len(s.layers); rank++ {
        s.sortLayerByBarycenter(rank, s.predecessors)
    }
}

// sweepUp reorders each layer based on successors (bottom-to-top).
func (s *layoutState) sweepUp() {
    for rank := len(s.layers) - 2; rank >= 0; rank-- {
        s.sortLayerByBarycenter(rank, s.successors)
    }
}

// sortLayerByBarycenter sorts a layer based on neighbor barycenters.
func (s *layoutState) sortLayerByBarycenter(rank int, neighborFn func(string) []string) {
    layer := s.layers[rank]

    // Calculate barycenters
    entries := make([]barycenterEntry, len(layer))
    for i, nodeID := range layer {
        bc, weight, hasValue := s.barycenter(nodeID, neighborFn)
        entries[i] = barycenterEntry{
            nodeID:     nodeID,
            barycenter: bc,
            weight:     weight,
            hasValue:   hasValue,
        }
    }

    // Sort by barycenter (stable sort preserves relative order for equal values)
    sort.SliceStable(entries, func(i, j int) bool {
        if !entries[i].hasValue && !entries[j].hasValue {
            return false // keep original order
        }
        if !entries[i].hasValue {
            return false // nodes without neighbors go to end
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
```

### Counting Crossings

The crossing count uses an efficient accumulator tree algorithm:

```go
// countCrossings counts total edge crossings in the current layout.
func (s *layoutState) countCrossings() int {
    total := 0
    for i := 1; i < len(s.layers); i++ {
        total += s.twoLayerCrossCount(s.layers[i-1], s.layers[i])
    }
    return total
}

// twoLayerCrossCount counts crossings between two adjacent layers.
// Uses the accumulator tree algorithm from Barth et al.
func (s *layoutState) twoLayerCrossCount(northLayer, southLayer []string) int {
    // Build position map for south layer
    southPos := make(map[string]int, len(southLayer))
    for i, id := range southLayer {
        southPos[id] = i
    }

    // Collect south positions for all edges from north
    type entry struct {
        pos    int
        weight int
    }
    var southEntries []entry

    for _, v := range northLayer {
        // Get all edges from v to south layer
        var edges []entry
        for _, w := range s.successors[v] {
            if pos, ok := southPos[w]; ok {
                weight := 1
                if edge := s.edges[edgeKey{from: v, to: w}]; edge != nil {
                    weight = int(edge.weight)
                }
                edges = append(edges, entry{pos: pos, weight: weight})
            }
        }
        // Sort by position in south layer
        sort.Slice(edges, func(i, j int) bool {
            return edges[i].pos < edges[j].pos
        })
        southEntries = append(southEntries, edges...)
    }

    // Build accumulator tree
    n := len(southLayer)
    if n == 0 {
        return 0
    }

    firstIndex := 1
    for firstIndex < n {
        firstIndex <<= 1
    }
    treeSize := 2*firstIndex - 1
    firstIndex -= 1
    tree := make([]int, treeSize)

    // Count crossings
    cc := 0
    for _, e := range southEntries {
        index := e.pos + firstIndex
        tree[index] += e.weight
        weightSum := 0
        for index > 0 {
            if index%2 == 1 {
                weightSum += tree[index+1]
            }
            index = (index - 1) >> 1
            tree[index] += e.weight
        }
        cc += e.weight * weightSum
    }

    return cc
}
```

### Initial Order

```go
// initOrder assigns initial order to nodes using DFS.
func (s *layoutState) initOrder() {
    visited := make(map[string]bool)

    var dfs func(v string)
    dfs = func(v string) {
        if visited[v] {
            return
        }
        visited[v] = true

        node := s.nodes[v]
        layer := s.layers[node.rank]

        // Find current position in layer
        for i, id := range layer {
            if id == v {
                node.order = i
                break
            }
        }

        // Visit successors
        for _, w := range s.successors[v] {
            dfs(w)
        }
    }

    // Start from nodes in first layer, sorted by current order
    if len(s.layers) > 0 {
        for _, id := range s.layers[0] {
            dfs(id)
        }
    }

    // Handle any unvisited nodes
    for id := range s.nodes {
        if !visited[id] {
            dfs(id)
        }
    }

    // Assign orders from layer positions
    s.assignOrderFromLayers()
}

// assignOrderFromLayers sets node.order based on position in layer.
func (s *layoutState) assignOrderFromLayers() {
    for _, layer := range s.layers {
        for i, id := range layer {
            s.nodes[id].order = i
        }
    }
}

// copyLayers creates a deep copy of the layers structure.
func (s *layoutState) copyLayers() [][]string {
    result := make([][]string, len(s.layers))
    for i, layer := range s.layers {
        result[i] = make([]string, len(layer))
        copy(result[i], layer)
    }
    return result
}
```

---

## position.go - Coordinate Assignment

### Coordinate Convention

Posit outputs **top-left coordinates** for nodes. This is the standard convention for:
- Layout algorithms (dagre, ELK, graphviz)
- Graphics APIs (SVG, Canvas, CSS)
- Most UI frameworks

Internally, the algorithm works with top-left positions. The final `Layout` output uses `X, Y` as the top-left corner of each node.

### Simplified Approach

A full Brandes-Kopf implementation is complex. Start with a simpler approach:

```go
// assignCoordinates computes X and Y positions for all nodes.
func (s *layoutState) assignCoordinates() {
    s.assignYCoordinates()
    s.assignXCoordinates()
}

// assignYCoordinates assigns Y based on rank and RankSep.
// Y is the top edge of the node (top-left convention).
func (s *layoutState) assignYCoordinates() {
    y := 0.0
    for rank, layer := range s.layers {
        // Find max height in this layer
        maxHeight := 0.0
        for _, id := range layer {
            node := s.nodes[id]
            if node.height > maxHeight {
                maxHeight = node.height
            }
        }

        // Assign Y to top of node (top-left convention)
        for _, id := range layer {
            s.nodes[id].y = y
        }

        // Move to next layer
        if rank < len(s.layers)-1 {
            y += maxHeight + s.opts.RankSep
        }
    }
}

// assignXCoordinates assigns X using a simple left-to-right placement.
// X is the left edge of the node (top-left convention).
func (s *layoutState) assignXCoordinates() {
    // First pass: simple placement (top-left coordinates)
    for _, layer := range s.layers {
        x := 0.0
        for _, id := range layer {
            node := s.nodes[id]
            node.x = x  // top-left corner
            x += node.width + s.opts.NodeSep
        }
    }

    // Center layers
    s.centerLayers()
}

// centerLayers adjusts X so all layers are centered.
func (s *layoutState) centerLayers() {
    // Find max layer width
    maxWidth := 0.0
    for _, layer := range s.layers {
        if len(layer) == 0 {
            continue
        }
        lastID := layer[len(layer)-1]
        lastNode := s.nodes[lastID]
        width := lastNode.x + lastNode.width/2
        if width > maxWidth {
            maxWidth = width
        }
    }

    // Center each layer
    for _, layer := range s.layers {
        if len(layer) == 0 {
            continue
        }
        lastID := layer[len(layer)-1]
        lastNode := s.nodes[lastID]
        layerWidth := lastNode.x + lastNode.width/2
        offset := (maxWidth - layerWidth) / 2

        for _, id := range layer {
            s.nodes[id].x += offset
        }
    }
}
```

### Full Brandes-Kopf (Advanced)

For optimal results, implement the four-alignment approach from the paper:

```go
// assignXCoordinatesBK uses the Brandes-Kopf algorithm.
func (s *layoutState) assignXCoordinatesBK() {
    // The algorithm computes four alignments:
    // - ul: up-left (predecessors, left-biased)
    // - ur: up-right (predecessors, right-biased)
    // - dl: down-left (successors, left-biased)
    // - dr: down-right (successors, right-biased)

    layering := s.layers

    // Find conflicts (inner segments that shouldn't cross)
    conflicts := s.findType1Conflicts(layering)

    // Compute four alignments
    xss := make(map[string]map[string]float64)

    for _, vert := range []string{"u", "d"} {
        adjustedLayering := layering
        if vert == "d" {
            adjustedLayering = s.reverseLayers(layering)
        }

        for _, horiz := range []string{"l", "r"} {
            if horiz == "r" {
                adjustedLayering = s.reverseLayerOrder(adjustedLayering)
            }

            neighborFn := s.predecessors
            if vert == "d" {
                neighborFn = s.successors
            }

            root, align := s.verticalAlignment(adjustedLayering, conflicts, neighborFn)
            xs := s.horizontalCompaction(adjustedLayering, root, align, horiz == "r")

            if horiz == "r" {
                // Negate X values
                for id := range xs {
                    xs[id] = -xs[id]
                }
            }

            xss[vert+horiz] = xs
        }
    }

    // Find smallest width alignment
    smallestWidth := s.findSmallestWidthAlignment(xss)

    // Align all to smallest
    s.alignCoordinates(xss, smallestWidth)

    // Balance: take median of four alignments
    for id, node := range s.nodes {
        values := []float64{
            xss["ul"][id],
            xss["ur"][id],
            xss["dl"][id],
            xss["dr"][id],
        }
        sort.Float64s(values)
        // Median of four = average of middle two
        node.x = (values[1] + values[2]) / 2
    }
}
```

### Handling Node Dimensions

Key considerations:
1. **Node center**: All coordinates refer to node centers
2. **Spacing**: NodeSep is between node edges, not centers
3. **Dummy nodes**: Have zero width/height

```go
// separation calculates required space between two adjacent nodes.
func (s *layoutState) separation(leftID, rightID string) float64 {
    left := s.nodes[leftID]
    right := s.nodes[rightID]

    sep := s.opts.NodeSep
    if left.isDummy || right.isDummy {
        // Edge separation is typically smaller
        sep = s.opts.NodeSep / 2
    }

    return left.width/2 + sep + right.width/2
}
```

---

## route.go - Edge Routing

### Converting Dummy Nodes to Bend Points

```go
// routeEdges builds the final edge paths and restores reversed edges.
func (s *layoutState) routeEdges() {
    // Process dummy chains to build edge points
    s.buildEdgePaths()

    // Restore reversed edges
    s.restoreReversedEdges()

    // Add endpoint intersections with node boundaries
    s.addNodeIntersections()
}

// buildEdgePaths walks dummy chains to create edge bend points.
func (s *layoutState) buildEdgePaths() {
    for _, firstDummy := range s.dummyChains {
        dummy := s.nodes[firstDummy]
        edge := dummy.edgeLabel
        if edge == nil {
            continue
        }

        // Initialize points array
        edge.points = make([]EdgePoint, 0)

        // Walk the chain
        currentID := firstDummy
        for {
            node := s.nodes[currentID]
            if !node.isDummy {
                break
            }

            // Add this dummy's position as a bend point
            edge.points = append(edge.points, EdgePoint{
                X: node.x,
                Y: node.y,
            })

            // Move to next node in chain
            successors := s.successors[currentID]
            if len(successors) == 0 {
                break
            }
            currentID = successors[0]
        }
    }

    // Handle edges without dummy chains (single-layer span)
    for key, edge := range s.edges {
        if edge.points == nil {
            edge.points = make([]EdgePoint, 0)
        }
    }
}

// restoreReversedEdges flips points for edges that were reversed.
func (s *layoutState) restoreReversedEdges() {
    for _, key := range s.reversedEdges {
        edge := s.edges[key]
        if edge == nil {
            continue
        }

        // Reverse the points array
        for i, j := 0, len(edge.points)-1; i < j; i, j = i+1, j-1 {
            edge.points[i], edge.points[j] = edge.points[j], edge.points[i]
        }

        // Swap the edge key back to original direction
        delete(s.edges, key)
        originalKey := edgeKey{from: key.to, to: key.from}
        edge.key = originalKey
        edge.reversed = false
        s.edges[originalKey] = edge
    }
}

// addNodeIntersections adds start/end points at node boundaries.
func (s *layoutState) addNodeIntersections() {
    for key, edge := range s.edges {
        fromNode := s.nodes[key.from]
        toNode := s.nodes[key.to]

        // Calculate intersection with source node
        var startPoint EdgePoint
        if len(edge.points) > 0 {
            startPoint = s.intersectRect(fromNode, edge.points[0])
        } else {
            startPoint = s.intersectRect(fromNode, EdgePoint{X: toNode.x, Y: toNode.y})
        }

        // Calculate intersection with target node
        var endPoint EdgePoint
        if len(edge.points) > 0 {
            endPoint = s.intersectRect(toNode, edge.points[len(edge.points)-1])
        } else {
            endPoint = s.intersectRect(toNode, EdgePoint{X: fromNode.x, Y: fromNode.y})
        }

        // Prepend start, append end
        edge.points = append([]EdgePoint{startPoint}, edge.points...)
        edge.points = append(edge.points, endPoint)
    }
}

// intersectRect finds where a line from rect center to point crosses rect boundary.
func (s *layoutState) intersectRect(node *layoutNode, point EdgePoint) EdgePoint {
    x := node.x
    y := node.y
    w := node.width / 2
    h := node.height / 2

    dx := point.X - x
    dy := point.Y - y

    if dx == 0 && dy == 0 {
        return EdgePoint{X: x, Y: y}
    }

    var sx, sy float64

    if w == 0 || h == 0 {
        // Zero-dimension node (dummy)
        return EdgePoint{X: x, Y: y}
    }

    if abs(dy)*w > abs(dx)*h {
        // Intersection is top or bottom
        if dy < 0 {
            h = -h
        }
        sx = h * dx / dy
        sy = h
    } else {
        // Intersection is left or right
        if dx < 0 {
            w = -w
        }
        sx = w
        sy = w * dy / dx
    }

    return EdgePoint{X: x + sx, Y: y + sy}
}

func abs(x float64) float64 {
    if x < 0 {
        return -x
    }
    return x
}
```

---

## Go-Specific Patterns

### When to Use Maps vs Slices

**Use maps for:**
- Node lookups by ID: `map[string]*layoutNode`
- Edge lookups by key: `map[edgeKey]*layoutEdge`
- Visited tracking in DFS: `map[string]bool`
- Position lookups: `map[string]int`

**Use slices for:**
- Adjacency lists: `[]string` (order matters for iteration stability)
- Layers: `[][]string` (indexed by rank)
- Points arrays: `[]EdgePoint`
- Iteration order needs to be consistent

```go
// Good: Map for random access
nodes := make(map[string]*layoutNode)

// Good: Slice for ordered iteration
layer := make([]string, 0, expectedSize)

// Good: Slice for adjacency (order matters for determinism)
successors := make(map[string][]string)
```

### Avoiding Allocations in Hot Paths

The crossing minimization loop runs many times. Avoid allocations:

```go
// Bad: Allocates every iteration
func (s *layoutState) countCrossings() int {
    for i := 1; i < len(s.layers); i++ {
        southPos := make(map[string]int) // allocation!
        // ...
    }
}

// Better: Reuse scratch space
type layoutState struct {
    // ... other fields ...
    scratchPosMap map[string]int // reusable position map
    scratchTree   []int          // reusable accumulator tree
}

func (s *layoutState) twoLayerCrossCount(north, south []string) int {
    // Clear and reuse
    for k := range s.scratchPosMap {
        delete(s.scratchPosMap, k)
    }
    for i := range south {
        s.scratchPosMap[south[i]] = i
    }
    // ...
}
```

### Error Handling

This is internal code - errors indicate bugs, not user mistakes:

```go
// Don't do this for internal code:
func (s *layoutState) getNode(id string) (*layoutNode, error) {
    node := s.nodes[id]
    if node == nil {
        return nil, fmt.Errorf("node not found: %s", id)
    }
    return node, nil
}

// Do this instead - panic on programmer error:
func (s *layoutState) getNode(id string) *layoutNode {
    node := s.nodes[id]
    if node == nil {
        panic("internal error: node not found: " + id)
    }
    return node
}

// Or just use direct access when confident:
node := s.nodes[id]
// Only check if there's a reasonable possibility of missing
```

### Testing Each Phase Independently

Structure code to allow phase-by-phase testing:

```go
// In posit_test.go

func TestAcyclic(t *testing.T) {
    g := NewGraph()
    g.AddNode("a", NodeOptions{})
    g.AddNode("b", NodeOptions{})
    g.AddNode("c", NodeOptions{})
    g.AddEdge("a", "b")
    g.AddEdge("b", "c")
    g.AddEdge("c", "a") // creates cycle

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()

    // Verify: should have exactly one reversed edge
    if len(state.reversedEdges) != 1 {
        t.Errorf("expected 1 reversed edge, got %d", len(state.reversedEdges))
    }

    // Verify: graph is now acyclic
    if hasCycle(state) {
        t.Error("graph still has cycle after makeAcyclic")
    }
}

func TestRank(t *testing.T) {
    g := NewGraph()
    g.AddNode("a", NodeOptions{})
    g.AddNode("b", NodeOptions{})
    g.AddNode("c", NodeOptions{})
    g.AddEdge("a", "b")
    g.AddEdge("b", "c")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    // Verify layer assignments
    if state.nodes["a"].rank >= state.nodes["b"].rank {
        t.Error("a should be in earlier layer than b")
    }
    if state.nodes["b"].rank >= state.nodes["c"].rank {
        t.Error("b should be in earlier layer than c")
    }
}

// Helper for testing
func hasCycle(s *layoutState) bool {
    visited := make(map[string]bool)
    onStack := make(map[string]bool)

    var dfs func(v string) bool
    dfs = func(v string) bool {
        if onStack[v] {
            return true // cycle found
        }
        if visited[v] {
            return false
        }
        visited[v] = true
        onStack[v] = true
        for _, w := range s.successors[v] {
            if dfs(w) {
                return true
            }
        }
        delete(onStack, v)
        return false
    }

    for id := range s.nodes {
        if dfs(id) {
            return true
        }
    }
    return false
}
```

---

## Debugging Tips

### Visualizing Intermediate States

Add a debug dump function:

```go
func (s *layoutState) debugDump(phase string) {
    fmt.Printf("\n=== %s ===\n", phase)

    fmt.Println("Nodes:")
    for id, n := range s.nodes {
        fmt.Printf("  %s: rank=%d order=%d pos=(%.1f,%.1f) dummy=%v\n",
            id, n.rank, n.order, n.x, n.y, n.isDummy)
    }

    fmt.Println("Layers:")
    for rank, layer := range s.layers {
        fmt.Printf("  [%d]: %v\n", rank, layer)
    }

    fmt.Println("Edges:")
    for key, e := range s.edges {
        rev := ""
        if e.reversed {
            rev = " (reversed)"
        }
        fmt.Printf("  %s -> %s%s\n", key.from, key.to, rev)
    }
}
```

Call it between phases:

```go
func (g *Graph) Layout(opts ...Options) *Layout {
    // ...
    state := newLayoutState(g, opt)
    state.debugDump("Initial")

    state.makeAcyclic()
    state.debugDump("After makeAcyclic")

    state.assignLayers()
    state.debugDump("After assignLayers")
    // ...
}
```

### Generating DOT Output

For visual debugging, output Graphviz DOT format:

```go
func (s *layoutState) toDOT() string {
    var b strings.Builder
    b.WriteString("digraph G {\n")
    b.WriteString("  rankdir=TB;\n")

    // Output nodes grouped by rank
    for rank, layer := range s.layers {
        b.WriteString(fmt.Sprintf("  { rank=same; /* rank %d */\n", rank))
        for _, id := range layer {
            node := s.nodes[id]
            shape := "box"
            if node.isDummy {
                shape = "point"
            }
            b.WriteString(fmt.Sprintf("    \"%s\" [shape=%s, label=\"%s\\norder=%d\"];\n",
                id, shape, id, node.order))
        }
        b.WriteString("  }\n")
    }

    // Output edges
    for key, e := range s.edges {
        style := ""
        if e.reversed {
            style = " [style=dashed, color=red]"
        }
        b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\"%s;\n", key.from, key.to, style))
    }

    b.WriteString("}\n")
    return b.String()
}
```

Save and visualize:
```bash
go test -run TestMyGraph -v 2>&1 | grep -A1000 "digraph" > debug.dot
dot -Tpng debug.dot -o debug.png
```

### Common Bugs and How to Spot Them

#### 1. Edges Not Reversed Properly

**Symptom**: Ranking assigns wrong layers (child above parent).

**Check**: After `makeAcyclic()`, verify all edges go from lower rank to higher rank:
```go
for key := range s.edges {
    fromRank := s.nodes[key.from].rank
    toRank := s.nodes[key.to].rank
    if fromRank >= toRank {
        t.Errorf("edge %s->%s violates rank order: %d >= %d",
            key.from, key.to, fromRank, toRank)
    }
}
```

#### 2. Dummy Nodes in Wrong Layer

**Symptom**: Edges appear to bend incorrectly.

**Check**: Each dummy should be in exactly one layer between source and target:
```go
for _, firstDummy := range s.dummyChains {
    current := firstDummy
    for {
        node := s.nodes[current]
        if !node.isDummy {
            break
        }
        // Verify node is in correct layer
        layer := s.layers[node.rank]
        found := false
        for _, id := range layer {
            if id == current {
                found = true
                break
            }
        }
        if !found {
            t.Errorf("dummy %s not in layer %d", current, node.rank)
        }
        // Move to next
        if len(s.successors[current]) == 0 {
            break
        }
        current = s.successors[current][0]
    }
}
```

#### 3. Crossing Count Not Decreasing

**Symptom**: Order optimization makes no progress.

**Check**: Log crossing counts each iteration:
```go
for i := 0; i < maxIterations; i++ {
    if i%2 == 0 {
        s.sweepDown()
    } else {
        s.sweepUp()
    }
    cc := s.countCrossings()
    fmt.Printf("Iteration %d: crossings = %d\n", i, cc)
    // ...
}
```

If count never changes, check:
- Barycenter calculation returning correct values
- Sort actually reordering the layer
- Order values being updated after sort

#### 4. Nodes Overlapping

**Symptom**: Nodes at same position or overlapping.

**Check**: After `assignCoordinates()`, verify minimum separation:
```go
for _, layer := range s.layers {
    for i := 1; i < len(layer); i++ {
        left := s.nodes[layer[i-1]]
        right := s.nodes[layer[i]]
        gap := right.x - right.width/2 - (left.x + left.width/2)
        if gap < s.opts.NodeSep-0.01 {
            t.Errorf("nodes %s and %s too close: gap=%.2f < %.2f",
                layer[i-1], layer[i], gap, s.opts.NodeSep)
        }
    }
}
```

### Comparing Output to Dagre

Create a test that compares posit output to dagre:

```go
func TestCompareWithDagre(t *testing.T) {
    // Create same graph in both
    g := NewGraph()
    g.AddNode("a", NodeOptions{Width: 100, Height: 50})
    g.AddNode("b", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("a", "b")

    positResult := g.Layout()

    // Run dagre via node (requires dagre installed)
    dagreResult := runDagre(t, `
        var g = new dagre.graphlib.Graph();
        g.setGraph({});
        g.setNode("a", {width: 100, height: 50});
        g.setNode("b", {width: 100, height: 50});
        g.setEdge("a", "b");
        dagre.layout(g);
        console.log(JSON.stringify({
            a: g.node("a"),
            b: g.node("b")
        }));
    `)

    // Compare (with tolerance for floating point)
    tolerance := 1.0
    if !withinTolerance(positResult.Nodes["a"].X, dagreResult["a"].X, tolerance) {
        t.Errorf("node a X differs: posit=%.2f dagre=%.2f",
            positResult.Nodes["a"].X, dagreResult["a"].X)
    }
    // ... more comparisons
}
```

---

## Summary

Implementation should proceed in this order:

1. **state.go**: Get the data structures right first
2. **acyclic.go**: Simple DFS, easy to test
3. **rank.go**: Longest path is straightforward
4. **normalize.go**: Mechanical edge splitting
5. **order.go**: Most complex - start with simple barycenter, iterate
6. **position.go**: Start simple, add Brandes-Kopf later if needed
7. **route.go**: Straightforward once other phases work

Key principles:
- Test each phase independently
- Use debug output liberally during development
- Compare against dagre for validation
- Prefer maps for lookups, slices for ordered data
- Avoid allocations in the crossing minimization loop
