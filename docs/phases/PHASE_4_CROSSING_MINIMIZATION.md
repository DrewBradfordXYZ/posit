# Phase 4: Crossing Minimization (Ordering)

**File:** `order.go`

## Table of Contents

- [Goal](#goal)
- [Why This Phase is Necessary](#why-this-phase-is-necessary)
- [NP-Completeness](#np-completeness)
- [The Layer Sweep Approach](#the-layer-sweep-approach)
- [Barycenter Heuristic](#barycenter-heuristic)
- [Counting Crossings](#counting-crossings)
- [Implementation](#implementation)
- [Optimization Details](#optimization-details)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Determine the **horizontal order** of nodes within each layer to minimize the number of edge crossings. This dramatically improves readability.

### Input
- Normalized layered graph (from Phase 3)
- All edges span exactly one layer

### Output
- Each node has an `order` attribute (position within its layer, 0-indexed)
- `layers[rank]` arrays are sorted by order

---

## Why This Phase is Necessary

Edge crossings are the primary source of visual clutter:

```
MANY CROSSINGS (hard to read):    FEW CROSSINGS (easy to read):

Rank 0:  A     B     C            Rank 0:  A     B     C
          \   /|\   /                       \    |    /
           \ / | \ /                         \   |   /
            X  |  X                           \  |  /
           / \ | / \                           \ | /
          /   \|/   \                           \|/
Rank 1:  D     E     F            Rank 1:  D     E     F
```

By reordering nodes within layers, we can reduce crossings from many to zero.

---

## NP-Completeness

**Important:** Minimizing crossings is **NP-complete**, even for just two layers.

This means:
- No polynomial-time algorithm finds the optimal solution (unless P=NP)
- We use heuristics that produce **good** but not guaranteed **optimal** results
- The heuristics work well in practice for typical graphs

---

## The Layer Sweep Approach

Instead of optimizing all layers simultaneously (intractable), we:

1. **Fix one layer** (don't change its order)
2. **Optimize the adjacent layer** based on the fixed layer
3. **Repeat**, sweeping up and down through the layers
4. **Track the best solution** seen across all sweeps

### Pseudocode

```
Algorithm LayerSweep(G):
    // Initialize order (e.g., DFS discovery order)
    initOrder(G)

    bestCC = countCrossings(G)
    bestOrder = copyLayerOrder(G)
    noImprovement = 0

    for i = 0; noImprovement < 4; i++:
        // Alternate sweep direction
        if i % 2 == 0:
            sweepDown(G)   // Fix rank 0, optimize 1, then 2, ...
        else:
            sweepUp(G)     // Fix max rank, optimize down

        // Count crossings
        cc = countCrossings(G)

        if cc < bestCC:
            bestCC = cc
            bestOrder = copyLayerOrder(G)
            noImprovement = 0
        else:
            noImprovement++

    // Restore best solution
    restoreLayerOrder(G, bestOrder)
```

---

## Barycenter Heuristic

The **barycenter** (center of mass) heuristic is the core ordering method. For each node, compute the weighted average position of its neighbors in the adjacent layer, then sort by barycenter.

### Definition

For a node `v` in layer `i`, with neighbors in layer `i-1`:

```
barycenter(v) = Σ(weight(u,v) × order(u)) / Σ(weight(u,v))
                for all neighbors u in adjacent layer
```

### Intuition

If a node's neighbors are positioned to the left, the node should be positioned to the left. If neighbors are spread across the layer, the node should be in the middle.

### Example

```
Fixed Layer (rank 0):    A(0)    B(1)    C(2)    D(3)
                          \      |       |      /
                           \     |       |     /
                            \    |       |    /
                             \   |       |   /
Movable Layer (rank 1):       ?X?       ?Y?

Edges: A→X, B→X, C→Y, D→Y

Barycenter(X) = (0×1 + 1×1) / (1 + 1) = 0.5
Barycenter(Y) = (2×1 + 3×1) / (1 + 1) = 2.5

After sorting by barycenter: X(order=0), Y(order=1)
Result: 0 crossings!
```

---

## Counting Crossings

### Naive Approach: O(E²)

Compare every pair of edges and check if they cross:

```
for each edge e1 = (u1, v1):
    for each edge e2 = (u2, v2):
        if order(u1) < order(u2) and order(v1) > order(v2):
            crossings++  // They cross
```

### Efficient Approach: Accumulator Tree O(E log V)

Uses a binary indexed tree (Fenwick tree) to count inversions:

```
Algorithm TwoLayerCrossCount(northLayer, southLayer):
    // Map south nodes to positions
    southPos = {v: i for i, v in enumerate(southLayer)}

    // Collect edges sorted by north position
    edges = []
    for v in northLayer (in order):
        for w in successors(v) in southLayer:
            edges.append({northPos: order(v), southPos: southPos[w]})

    // Build accumulator tree and count inversions
    treeSize = nextPowerOf2(|southLayer|)
    tree = zeros(2 * treeSize)
    firstIndex = treeSize - 1

    crossings = 0
    for edge in edges (sorted by northPos, then southPos):
        index = edge.southPos + firstIndex
        tree[index] += edge.weight

        // Count items to the right that came before
        weightSum = 0
        while index > 0:
            if index is odd (left child):
                weightSum += tree[index + 1]  // Add right sibling
            index = (index - 1) / 2
            tree[index] += edge.weight

        crossings += edge.weight * weightSum

    return crossings
```

**Reference:** Barth, Mutzel, Junger. "Simple and Efficient Bilayer Cross Counting." *Journal of Graph Algorithms and Applications*, 2004.

---

## Implementation

### Main Entry Point

```go
// minimizeCrossings reorders nodes within layers to reduce edge crossings.
func (s *layoutState) minimizeCrossings() {
    if len(s.layers) <= 1 {
        return  // Nothing to optimize
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
}
```

### Sweep Functions

```go
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
```

### Barycenter Calculation

```go
// barycenterEntry holds barycenter data for sorting.
type barycenterEntry struct {
    nodeID     string
    barycenter float64
    weight     float64
    hasValue   bool
}

// sortLayerByBarycenter sorts a layer based on neighbor barycenters.
func (s *layoutState) sortLayerByBarycenter(rank int, neighborFn func(string) []string) {
    layer := s.layers[rank]
    if len(layer) <= 1 {
        return
    }

    // Calculate barycenters
    entries := make([]barycenterEntry, len(layer))
    for i, nodeID := range layer {
        bc, weight, hasValue := s.calculateBarycenter(nodeID, neighborFn)
        entries[i] = barycenterEntry{
            nodeID:     nodeID,
            barycenter: bc,
            weight:     weight,
            hasValue:   hasValue,
        }
    }

    // Sort by barycenter (stable sort preserves order for equal values)
    sort.SliceStable(entries, func(i, j int) bool {
        if !entries[i].hasValue && !entries[j].hasValue {
            return false  // Keep original order
        }
        if !entries[i].hasValue {
            return false  // Nodes without neighbors go to end
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
func (s *layoutState) calculateBarycenter(nodeID string, neighborFn func(string) []string) (float64, float64, bool) {
    neighbors := neighborFn(nodeID)
    if len(neighbors) == 0 {
        return 0, 0, false
    }

    sum := 0.0
    weight := 0.0

    for _, neighborID := range neighbors {
        neighbor := s.nodes[neighborID]

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

    return sum / weight, weight, true
}
```

### Crossing Counter

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
func (s *layoutState) twoLayerCrossCount(northLayer, southLayer []string) int {
    if len(northLayer) == 0 || len(southLayer) == 0 {
        return 0
    }

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
        var edges []entry
        for _, w := range s.successors[v] {
            pos, ok := southPos[w]
            if !ok {
                continue  // Edge to different layer
            }
            weight := 1
            if edge := s.edges[edgeKey{from: v, to: w}]; edge != nil {
                weight = int(edge.weight)
            }
            edges = append(edges, entry{pos: pos, weight: weight})
        }
        // Sort by position
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

    // Count crossings
    cc := 0
    for _, e := range southEntries {
        index := e.pos + firstIndex
        tree[index] += e.weight

        weightSum := 0
        for index > 0 {
            if index%2 == 1 {  // Left child
                weightSum += tree[index+1]  // Add right sibling
            }
            index = (index - 1) >> 1
            tree[index] += e.weight
        }
        cc += e.weight * weightSum
    }

    return cc
}
```

---

## Optimization Details

### Initial Order

The initial order significantly affects results. Options include:

1. **DFS Order** (default): Order of discovery in depth-first search
2. **BFS Order**: Order of discovery in breadth-first search
3. **Degree-based**: Higher-degree nodes first or last

```go
// initOrder assigns initial order using DFS traversal.
func (s *layoutState) initOrder() {
    visited := make(map[string]bool)

    var dfs func(v string)
    dfs = func(v string) {
        if visited[v] {
            return
        }
        visited[v] = true

        for _, w := range s.successors[v] {
            dfs(w)
        }
    }

    // Start from nodes in first layer
    if len(s.layers) > 0 {
        for _, id := range s.layers[0] {
            dfs(id)
        }
    }

    // Process any unvisited nodes
    for id := range s.nodes {
        dfs(id)
    }

    // Assign orders from current layer positions
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
```

### Handling Nodes Without Neighbors

Nodes with no neighbors in the adjacent layer (e.g., isolated nodes, or dummies at edges of the graph) are handled specially:

```go
if !hasValue {
    // Keep original position or push to end
    return false  // In sort comparison
}
```

### Preserving Relative Order

Using `sort.SliceStable` ensures that nodes with equal barycenters keep their original relative order, which aids determinism.

---

## Complexity Analysis

| Operation | Time | Space |
|-----------|------|-------|
| Barycenter calculation | O(E) per layer | O(1) |
| Sorting by barycenter | O(n log n) per layer | O(n) |
| Cross count (two layers) | O(E log V) | O(V) |
| Full cross count | O(L × E log V) | O(V) |
| One sweep iteration | O(L × (E + n log n)) | O(V) |
| **Total (k iterations)** | **O(k × L × (E + n log n + E log V))** | **O(V + E)** |

Where: V = vertices, E = edges, L = layers, k = iterations (typically 4-8)

---

## Testing

### Test Cases

```go
func TestOrder_NoCrossings(t *testing.T) {
    // Tree structure has 0 crossings when ordered properly
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()
    state.minimizeCrossings()

    crossings := state.countCrossings()
    if crossings != 0 {
        t.Errorf("Expected 0 crossings for tree, got %d", crossings)
    }
}

func TestOrder_CrossingsDecrease(t *testing.T) {
    // Create graph with known crossings that can be reduced
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("D", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "D")  // Cross edge
    g.AddEdge("B", "C")  // Cross edge

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()

    initialCrossings := state.countCrossings()
    state.minimizeCrossings()
    finalCrossings := state.countCrossings()

    if finalCrossings > initialCrossings {
        t.Errorf("Crossings increased: %d -> %d", initialCrossings, finalCrossings)
    }
}

func TestOrder_Deterministic(t *testing.T) {
    // Same input should produce same output
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")

    var orders [][]string
    for i := 0; i < 3; i++ {
        state := newLayoutState(g, DefaultOptions())
        state.makeAcyclic()
        state.assignLayers()
        state.addDummyNodes()
        state.minimizeCrossings()
        orders = append(orders, state.copyLayers()[1])  // Layer 1 order
    }

    for i := 1; i < len(orders); i++ {
        if !reflect.DeepEqual(orders[0], orders[i]) {
            t.Errorf("Non-deterministic: run 0 = %v, run %d = %v",
                orders[0], i, orders[i])
        }
    }
}

func TestOrder_BarycenterCalculation(t *testing.T) {
    // Manual verification of barycenter
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("X", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "X")  // A at order 0
    g.AddEdge("B", "X")  // B at order 1
    g.AddEdge("C", "X")  // C at order 2

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()

    // Set known orders for layer 0
    state.nodes["A"].order = 0
    state.nodes["B"].order = 1
    state.nodes["C"].order = 2

    bc, _, _ := state.calculateBarycenter("X", func(id string) []string {
        return state.predecessors[id]
    })

    // Expected: (0 + 1 + 2) / 3 = 1.0
    expected := 1.0
    if math.Abs(bc-expected) > 0.001 {
        t.Errorf("Expected barycenter %v, got %v", expected, bc)
    }
}
```

---

## Visual Examples

### Example 1: Barycenter Ordering

```
BEFORE (with 2 crossings):

Rank 0:  A(0)    B(1)    C(2)
          │       │       │
          │       │       │
          │       │       │
Rank 1:  X(0)    Y(1)    Z(2)

Edges: A→Y, B→Z, C→X

Crossings:
  A→Y and C→X cross (A:0 < C:2, but Y:1 > X:0)
  B→Z and C→X cross (B:1 < C:2, but Z:2 > X:0)


BARYCENTER CALCULATION (for rank 1):

  X's predecessors: C (order 2)
    barycenter(X) = 2

  Y's predecessors: A (order 0)
    barycenter(Y) = 0

  Z's predecessors: B (order 1)
    barycenter(Z) = 1


AFTER (sorted by barycenter, 0 crossings):

Rank 0:  A(0)    B(1)    C(2)
          │       │       │
          │       │       │
          │       │       │
Rank 1:  Y(0)    Z(1)    X(2)

New order: Y, Z, X (sorted by barycenter 0, 1, 2)
All edges now go straight down!
```

### Example 2: Multiple Sweep Iterations

```
INITIAL STATE:

Rank 0:  A       B       C       D
Rank 1:  E       F       G       H
Rank 2:  I       J       K       L

Edges create crossings between all layers.


SWEEP DOWN (iteration 0):
  - Fix rank 0, optimize rank 1 based on rank 0
  - Fix rank 1, optimize rank 2 based on rank 1

SWEEP UP (iteration 1):
  - Fix rank 2, optimize rank 1 based on rank 2
  - Fix rank 1, optimize rank 0 based on rank 1

... repeat until no improvement for 4 iterations ...


FINAL STATE:

Rank 0:  A       C       B       D
Rank 1:  E       G       F       H
Rank 2:  I       K       J       L

Crossings minimized (may not be globally optimal)
```

### Example 3: Weighted Edges

```
Edges with different weights affect barycenter:

Rank 0:  A(0)    B(1)    C(2)
          │w=3    │w=1    │w=1
          └───────┼───────┘
                  │
Rank 1:          X

Barycenter(X) = (0×3 + 1×1 + 2×1) / (3+1+1)
              = (0 + 1 + 2) / 5
              = 0.6

The high-weight edge to A pulls X toward the left.
```

---

## Post-Conditions

After this phase completes:

1. ✅ Every node has a valid `order` >= 0
2. ✅ Within each layer, orders are unique and contiguous (0, 1, 2, ...)
3. ✅ `s.layers[rank]` arrays are sorted by order
4. ✅ Crossing count is locally minimal (may not be globally optimal)

---

## Next Phase

→ [Phase 5: Coordinate Assignment](./PHASE_5_COORDINATE_ASSIGNMENT.md)
