# Phase 8: Network Simplex Ranking

**Files:** `rank.go`, `simplex.go` (new)

## Table of Contents

- [Goal](#goal)
- [Why Network Simplex](#why-network-simplex)
- [Algorithm Overview](#algorithm-overview)
- [Key Data Structures](#key-data-structures)
- [Implementation Plan](#implementation-plan)
- [8.1 Feasible Tree Construction](#81-feasible-tree-construction)
- [8.2 Low/Lim Values](#82-lowlim-values)
- [8.3 Cut Values](#83-cut-values)
- [8.4 Pivot Operations](#84-pivot-operations)
- [8.5 Integration](#85-integration)
- [Testing](#testing)
- [Complexity Analysis](#complexity-analysis)
- [Visual Examples](#visual-examples)
- [References](#references)

---

## Goal

Implement the **network simplex algorithm** for optimal layer assignment. This minimizes total edge length, producing tighter, more balanced layouts than the current longest-path algorithm.

### Current State

```go
// rank.go - Current implementation
func (s *layoutState) assignLayers() {
    switch s.opts.Algorithm {
    case NetworkSimplex:
        // TODO: implement network simplex for optimal ranking
        // For now, fall back to longest path
        s.assignLayersLongestPath()
    default:
        s.assignLayersLongestPath()
    }
}
```

### Target State

```go
func (s *layoutState) assignLayers() {
    switch s.opts.Algorithm {
    case NetworkSimplex:
        s.assignLayersNetworkSimplex()  // Optimal ranking
    default:
        s.assignLayersLongestPath()      // Fast ranking
    }
}
```

---

## Why Network Simplex

### Problem with Longest Path

Longest-path ranking pushes nodes toward sinks, creating **wide bottom layers**:

```
Longest Path Result:        Network Simplex Result:

     A                           A
     |                          / \
     B                         B   C
    / \                        |   |
   C   D                       D   E
   |   |                        \ /
   E   F                         F
    \ /
     G

Layers: 5                   Layers: 4
Total edge length: 7        Total edge length: 6
```

### Benefits of Network Simplex

| Metric | Longest Path | Network Simplex |
|--------|--------------|-----------------|
| Layer count | Often excessive | Minimal |
| Edge length | Not optimized | Minimized |
| Balance | Bottom-heavy | Balanced |
| Speed | O(V + E) | O(V × E) practical |

---

## Algorithm Overview

Network simplex treats layer assignment as a **minimum cost flow problem**:

1. **Initialize**: Use longest-path to get valid (but suboptimal) ranks
2. **Build tree**: Construct a spanning tree where all edges are "tight" (slack = 0)
3. **Iterate**: Find edges with negative cut values and swap them
4. **Terminate**: When no negative cut values exist, solution is optimal

```
┌─────────────────────────────────────────────────────────────┐
│  NETWORK SIMPLEX ALGORITHM                                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. longestPath(G)           // Initial feasible ranking    │
│                        ↓                                     │
│  2. feasibleTree(G)          // Build tight spanning tree   │
│                        ↓                                     │
│  3. initLowLimValues(tree)   // For descendant queries      │
│                        ↓                                     │
│  4. initCutValues(tree, G)   // Compute edge cut values     │
│                        ↓                                     │
│  5. while leaveEdge exists:  // Main optimization loop      │
│       ├── e = leaveEdge(tree)                               │
│       ├── f = enterEdge(tree, G, e)                         │
│       ├── exchangeEdges(tree, G, e, f)                      │
│       └── updateRanks(tree, G)                              │
│                        ↓                                     │
│  6. Return optimized ranks                                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Key Data Structures

### Slack

For edge (u → v), slack measures how much longer the edge is than required:

```go
slack = rank[v] - rank[u] - minlen
```

- **slack = 0**: Edge is "tight" (exactly minlen apart)
- **slack > 0**: Edge is longer than required (can be shortened)

### Spanning Tree

A tree connecting all nodes where every tree edge has slack = 0:

```go
type spanningTree struct {
    nodes    map[string]*treeNode
    edges    map[edgeKey]bool  // Tree edges
    root     string
}

type treeNode struct {
    id     string
    parent string      // Parent in tree (empty for root)
    low    int         // Minimum postorder in subtree
    lim    int         // This node's postorder (max in subtree)
}
```

### Cut Value

For each tree edge, the cut value indicates whether removing it would improve the solution:

```
Cut value of tree edge (u, v):
  = weight(u, v)
  + Σ weight(edges pointing same direction as u→v)
  - Σ weight(edges pointing opposite direction)

Negative cut value → Edge should be removed (pivoted out)
```

---

## Implementation Plan

| Step | Description | Lines (est.) |
|------|-------------|--------------|
| 8.1 | Feasible tree construction | ~60 |
| 8.2 | Low/Lim value computation | ~40 |
| 8.3 | Cut value computation | ~50 |
| 8.4 | Pivot operations (leave/enter/exchange) | ~80 |
| 8.5 | Integration with rank.go | ~20 |
| **Total** | | **~250** |

---

## 8.1 Feasible Tree Construction

Build a spanning tree where all tree edges are tight (slack = 0).

### Algorithm

```
Algorithm feasibleTree(G):
    tree = new SpanningTree()
    tree.addNode(arbitrary root)

    while tree.nodeCount < G.nodeCount:
        // Find edge with minimum slack connecting tree to non-tree node
        minSlack = infinity
        bestEdge = null

        for each edge (u, v) in G:
            if (u in tree) XOR (v in tree):  // Exactly one endpoint in tree
                s = slack(u, v)
                if s < minSlack:
                    minSlack = s
                    bestEdge = (u, v)

        // Tighten: adjust ranks to make edge tight
        if minSlack > 0:
            // If u in tree, increase ranks of tree nodes
            // If v in tree, decrease ranks of tree nodes
            adjustRanks(tree, minSlack, bestEdge)

        tree.addEdge(bestEdge)
        tree.addNode(other endpoint)

    return tree
```

### Implementation

```go
// feasibleTree builds a tight spanning tree.
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

        for key, edge := range s.edges {
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
            }
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
```

---

## 8.2 Low/Lim Values

Enable O(1) descendant queries using postorder numbering.

### Concept

```
Tree structure:          Postorder numbering:

       A                  low=1, lim=7 (A)
      /|\                      |
     B C D                low=1,lim=3 (B)  low=4,lim=4 (C)  low=5,lim=6 (D)
    /|   |                    |                                   |
   E F   G               low=1,lim=1 (E)  low=2,lim=2 (F)    low=5,lim=5 (G)

Node X is descendant of Y iff: low[Y] <= lim[X] <= lim[Y]

Examples:
  Is E descendant of A?  1 <= 1 <= 7  → YES
  Is G descendant of B?  1 <= 5 <= 3  → NO (5 > 3)
  Is F descendant of B?  1 <= 2 <= 3  → YES
```

### Implementation

```go
// initLowLimValues computes low/lim for descendant queries.
func (tree *spanningTree) initLowLimValues() {
    visited := make(map[string]bool)
    counter := 1

    var dfs func(v, parent string) int
    dfs = func(v, parent string) int {
        node := tree.nodes[v]
        node.parent = parent
        low := counter

        visited[v] = true

        // Visit children (neighbors except parent)
        for _, neighbor := range tree.neighbors(v) {
            if !visited[neighbor] {
                counter = dfs(neighbor, v)
            }
        }

        node.low = low
        node.lim = counter
        counter++
        return counter
    }

    dfs(tree.root, "")
}

// isDescendant returns true if v is in the subtree rooted at u.
func (tree *spanningTree) isDescendant(v, u string) bool {
    uNode := tree.nodes[u]
    vNode := tree.nodes[v]
    return uNode.low <= vNode.lim && vNode.lim <= uNode.lim
}
```

---

## 8.3 Cut Values

Compute the cost of removing each tree edge.

### Formula

For tree edge (u → v) where v is the child:

```
cutValue = weight(u, v)
         + Σ w(e) for non-tree edges where:
             e goes from descendant of v to non-descendant (same direction)
         - Σ w(e) for non-tree edges where:
             e goes from non-descendant to descendant of v (opposite direction)
```

### Implementation

```go
// initCutValues computes cut values for all tree edges.
func (tree *spanningTree) initCutValues(s *layoutState) {
    // Process in postorder (children before parents)
    postorder := tree.postorderNodes()

    for _, v := range postorder {
        if v == tree.root {
            continue
        }
        tree.assignCutValue(s, v)
    }
}

// assignCutValue computes the cut value for edge (v, parent[v]).
func (tree *spanningTree) assignCutValue(s *layoutState, v string) {
    parent := tree.nodes[v].parent
    treeEdgeKey := edgeKey{from: v, to: parent}

    // Check if edge direction is reversed in graph
    graphEdge := s.edges[treeEdgeKey]
    if graphEdge == nil {
        treeEdgeKey = edgeKey{from: parent, to: v}
        graphEdge = s.edges[treeEdgeKey]
    }

    cutValue := graphEdge.weight

    // Add/subtract weights of non-tree edges based on direction
    for key, edge := range s.edges {
        if tree.isTreeEdge(key) {
            continue
        }

        fromIsDesc := tree.isDescendant(key.from, v)
        toIsDesc := tree.isDescendant(key.to, v)

        if fromIsDesc && !toIsDesc {
            // Same direction as tree edge
            cutValue += edge.weight
        } else if !fromIsDesc && toIsDesc {
            // Opposite direction
            cutValue -= edge.weight
        }
    }

    tree.setCutValue(v, parent, cutValue)
}
```

---

## 8.4 Pivot Operations

### Leave Edge

Find a tree edge with negative cut value:

```go
// leaveEdge finds a tree edge with negative cut value.
func (tree *spanningTree) leaveEdge() (edgeKey, bool) {
    for key, cutValue := range tree.cutValues {
        if cutValue < 0 {
            return key, true
        }
    }
    return edgeKey{}, false
}
```

### Enter Edge

Find a non-tree edge to replace the leaving edge:

```go
// enterEdge finds the best non-tree edge to add.
func (tree *spanningTree) enterEdge(s *layoutState, leave edgeKey) edgeKey {
    // Determine which subtree is "tail" vs "head"
    v, w := leave.from, leave.to
    vNode := tree.nodes[v]
    wNode := tree.nodes[w]

    tailNode := vNode
    flip := false
    if vNode.lim > wNode.lim {
        tailNode = wNode
        flip = true
    }

    // Find non-tree edges crossing the cut
    var best edgeKey
    bestSlack := math.MaxInt

    for key := range s.edges {
        if tree.isTreeEdge(key) {
            continue
        }

        fromIsDesc := tree.isDescendant(key.from, tailNode.id)
        toIsDesc := tree.isDescendant(key.to, tailNode.id)

        // Edge must cross the cut
        crosses := (flip == fromIsDesc) && (flip != toIsDesc)
        if !crosses {
            continue
        }

        slack := s.slack(key)
        if slack < bestSlack {
            bestSlack = slack
            best = key
        }
    }

    return best
}
```

### Exchange Edges

Swap edges and update the tree structure:

```go
// exchangeEdges swaps leave edge with enter edge.
func (tree *spanningTree) exchangeEdges(s *layoutState, leave, enter edgeKey) {
    // Remove leaving edge
    tree.removeEdge(leave)

    // Add entering edge
    tree.addEdge(enter)

    // Recompute tree structure
    tree.initLowLimValues()
    tree.initCutValues(s)

    // Update ranks based on new tree
    tree.updateRanks(s)
}

// updateRanks adjusts node ranks based on the spanning tree.
func (tree *spanningTree) updateRanks(s *layoutState) {
    // BFS from root, setting ranks based on tree edge minlens
    visited := make(map[string]bool)
    queue := []string{tree.root}
    s.nodes[tree.root].rank = 0
    visited[tree.root] = true

    for len(queue) > 0 {
        v := queue[0]
        queue = queue[1:]

        for _, neighbor := range tree.neighbors(v) {
            if visited[neighbor] {
                continue
            }
            visited[neighbor] = true

            // Determine edge direction and set rank
            edge := s.edges[edgeKey{from: v, to: neighbor}]
            if edge != nil {
                s.nodes[neighbor].rank = s.nodes[v].rank + edge.minlen
            } else {
                edge = s.edges[edgeKey{from: neighbor, to: v}]
                s.nodes[neighbor].rank = s.nodes[v].rank - edge.minlen
            }

            queue = append(queue, neighbor)
        }
    }
}
```

---

## 8.5 Integration

### Main Entry Point

```go
// simplex.go

// assignLayersNetworkSimplex uses the network simplex algorithm.
func (s *layoutState) assignLayersNetworkSimplex() {
    // Step 1: Initial feasible ranking
    s.assignLayersLongestPath()

    // Step 2: Build feasible spanning tree
    tree := s.feasibleTree()

    // Step 3: Initialize tree values
    tree.initLowLimValues()
    tree.initCutValues(s)

    // Step 4: Iterate until optimal
    maxIterations := len(s.nodes) * len(s.edges) // Safety limit
    for i := 0; i < maxIterations; i++ {
        leave, found := tree.leaveEdge()
        if !found {
            break // Optimal!
        }

        enter := tree.enterEdge(s, leave)
        tree.exchangeEdges(s, leave, enter)
    }
}
```

### Options Update

```go
// posit.go - Already exists, just needs implementation

type RankAlgorithm int

const (
    LongestPath    RankAlgorithm = iota  // Fast, O(V+E)
    NetworkSimplex                        // Optimal, O(V×E)
)

// Usage:
layout := g.Layout(posit.Options{
    Algorithm: posit.NetworkSimplex,
})
```

---

## Testing

### Unit Tests

```go
func TestNetworkSimplex_Linear(t *testing.T) {
    // A → B → C should still be 0, 1, 2
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B")
    g.MustAddEdge("B", "C")

    layout := g.Layout(Options{Algorithm: NetworkSimplex})

    // Verify ranks
    if layout.Nodes["A"].Y >= layout.Nodes["B"].Y {
        t.Error("A should be above B")
    }
}

func TestNetworkSimplex_ReducesLayers(t *testing.T) {
    // Graph where longest-path produces more layers than necessary
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("D", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B")
    g.MustAddEdge("A", "C")
    g.MustAddEdge("B", "D")
    g.MustAddEdge("C", "D")

    state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
    state.makeAcyclic()
    state.assignLayers()

    // Network simplex should produce optimal layers
    maxRank := 0
    for _, node := range state.nodes {
        if node.rank > maxRank {
            maxRank = node.rank
        }
    }

    // Diamond should have exactly 3 layers (0, 1, 2)
    if maxRank != 2 {
        t.Errorf("Expected max rank 2, got %d", maxRank)
    }
}

func TestNetworkSimplex_MinimizesEdgeLength(t *testing.T) {
    // Compare total edge length
    g := buildTestGraph()

    lpState := newLayoutState(g, Options{Algorithm: LongestPath})
    lpState.makeAcyclic()
    lpState.assignLayers()
    lpLength := totalEdgeLength(lpState)

    nsState := newLayoutState(g, Options{Algorithm: NetworkSimplex})
    nsState.makeAcyclic()
    nsState.assignLayers()
    nsLength := totalEdgeLength(nsState)

    if nsLength > lpLength {
        t.Errorf("NetworkSimplex should not produce longer edges: NS=%d, LP=%d",
            nsLength, lpLength)
    }
}

func TestNetworkSimplex_RespectsMinlen(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B") // Default minlen = 1

    state := newLayoutState(g, Options{Algorithm: NetworkSimplex})
    // Manually set minlen = 3
    for _, edge := range state.edges {
        edge.minlen = 3
    }

    state.makeAcyclic()
    state.assignLayers()

    rankDiff := state.nodes["B"].rank - state.nodes["A"].rank
    if rankDiff < 3 {
        t.Errorf("Edge should span at least 3 layers, got %d", rankDiff)
    }
}
```

### Benchmark Tests

```go
func BenchmarkNetworkSimplex_Small(b *testing.B) {
    g := buildRandomDAG(20, 30)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        g.Layout(Options{Algorithm: NetworkSimplex})
    }
}

func BenchmarkNetworkSimplex_Medium(b *testing.B) {
    g := buildRandomDAG(100, 200)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        g.Layout(Options{Algorithm: NetworkSimplex})
    }
}

func BenchmarkNetworkSimplex_Large(b *testing.B) {
    g := buildRandomDAG(500, 1000)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        g.Layout(Options{Algorithm: NetworkSimplex})
    }
}

func BenchmarkComparison(b *testing.B) {
    g := buildRandomDAG(100, 200)

    b.Run("LongestPath", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            g.Layout(Options{Algorithm: LongestPath})
        }
    })

    b.Run("NetworkSimplex", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            g.Layout(Options{Algorithm: NetworkSimplex})
        }
    })
}
```

---

## Complexity Analysis

| Operation | Time | Space |
|-----------|------|-------|
| Initial longest-path | O(V + E) | O(V) |
| Feasible tree | O(V × E) | O(V) |
| Low/Lim values | O(V) | O(V) |
| Cut values | O(V × E) | O(E) |
| Each pivot | O(V + E) | O(V) |
| **Practical total** | **O(V × E)** | **O(V + E)** |
| **Worst case** | O(V² × E) | O(V + E) |

### Notes

- In practice, convergence is fast (typically < 10 iterations)
- Worst case is rare; most graphs converge in O(V) iterations
- The algorithm is still efficient for graphs with thousands of nodes

---

## Visual Examples

### Example 1: Longest Path vs Network Simplex

```
Input graph:
    A ──→ B
    │     │
    ▼     ▼
    C ──→ D
    │
    ▼
    E

Longest Path result:         Network Simplex result:

Rank 0:    A                 Rank 0:    A    B
           │                           │    │
Rank 1:    B    C                      ▼    ▼
           │    │            Rank 1:    C    D
           ▼    ▼                      │
Rank 2:    D    E            Rank 2:    E
           │
Rank 3: (empty)

Layers: 3-4                  Layers: 3
Edge length: 5               Edge length: 4
```

### Example 2: Pivot Operation

```
Before pivot:                After pivot:

Tree edges: A-B, B-C         Tree edges: A-B, A-C
Non-tree: A-C (slack=1)      Non-tree: B-C (slack=0)

    A (rank 0)                   A (rank 0)
    │                           / \
    B (rank 1)                 B   C (rank 1)
    │                          |
    C (rank 2)                (nothing)

Cut value of B-C: -1         Cut value of A-B: 0
(negative = should leave)    (all non-negative = optimal)
```

---

## References

1. Gansner, E.R., et al. "A Technique for Drawing Directed Graphs." IEEE Transactions on Software Engineering, 1993.
   - Original paper describing network simplex for graph layout

2. dagre source code: `lib/rank/network-simplex.js`
   - Reference implementation (~250 lines)

3. Graphviz documentation: "Dot Layout Algorithm"
   - Practical implementation notes

---

## Summary

Phase 8 implements network simplex ranking, the **single most impactful improvement** for layout quality. It:

1. **Minimizes total edge length** - Produces tighter layouts
2. **Reduces layer count** - More balanced vertical distribution
3. **Respects edge weights** - Higher-weight edges are kept shorter
4. **Maintains compatibility** - Longest-path remains the default for speed

After Phase 8, posit will achieve **~95% feature parity** with dagre.

---

## Implementation Order

| Step | Task | Depends On |
|------|------|------------|
| 1 | Create `simplex.go` with `spanningTree` type | - |
| 2 | Implement `feasibleTree()` | Step 1 |
| 3 | Implement `initLowLimValues()` | Step 1 |
| 4 | Implement `initCutValues()` | Steps 2, 3 |
| 5 | Implement `leaveEdge()`, `enterEdge()` | Step 4 |
| 6 | Implement `exchangeEdges()` | Steps 4, 5 |
| 7 | Wire up `assignLayersNetworkSimplex()` | All above |
| 8 | Add tests | Step 7 |

Estimated effort: **~250 lines of code**, **~100 lines of tests**
