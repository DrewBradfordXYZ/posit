# Phase 2: Layer Assignment (Ranking)

**File:** `rank.go`

## Table of Contents

- [Goal](#goal)
- [Why This Phase is Necessary](#why-this-phase-is-necessary)
- [Edge Constraints](#edge-constraints)
- [Algorithm 1: Longest Path](#algorithm-1-longest-path)
- [Algorithm 2: Network Simplex](#algorithm-2-network-simplex)
- [Implementation](#implementation)
- [Building the Layers Structure](#building-the-layers-structure)
- [Choosing a Ranker](#choosing-a-ranker)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Assign each node to a discrete **layer** (also called **rank**) such that for every edge (u, v), rank(u) < rank(v). This determines the Y-coordinate of each node in the final layout.

### Input
- Directed Acyclic Graph (from Phase 1)

### Output
- Each node has a `rank` attribute (integer layer number, 0-indexed)
- `layers` array: `layers[rank]` contains all node IDs at that rank

---

## Why This Phase is Necessary

Layer assignment converts the graph into a **layered graph** where:

1. All edges point "downward" (from lower ranks to higher ranks)
2. Each layer becomes a row in the final visualization
3. Y-coordinates can be derived directly from rank numbers

```
After layer assignment:

Rank 0:  ┌───┐
         │ A │
         └───┘
           │
           ▼
Rank 1:  ┌───┐  ┌───┐
         │ B │  │ C │
         └───┘  └───┘
           │      │
           ▼      ▼
Rank 2:  ┌───┐
         │ D │
         └───┘
```

---

## Edge Constraints

Edges can have a `minlen` attribute specifying the minimum layer span:

| minlen | Meaning |
|--------|---------|
| 1 (default) | Edge spans exactly one layer (adjacent) |
| 2 | At least one layer gap between source and target |
| 0 | Same layer allowed (rare, creates horizontal edges) |

The constraint is: `rank(target) - rank(source) >= minlen`

---

## Algorithm 1: Longest Path

The simplest ranking algorithm. Fast and easy to implement, but may produce more layers than necessary.

### How It Works

1. Start from sink nodes (no outgoing edges) and assign rank 0
2. Work backwards: each node's rank is one less than its lowest successor
3. Normalize so minimum rank is 0

### Pseudocode

```
Algorithm LongestPath(G):
    visited = {}

    function dfs(v):
        if v in visited:
            return G.node(v).rank

        visited[v] = true

        // Find minimum allowed rank based on successors
        minRank = +infinity
        for each edge (v, w) in outEdges(v):
            successorRank = dfs(w)
            minlen = edge.minlen (default 1)
            candidate = successorRank - minlen
            minRank = min(minRank, candidate)

        if minRank == +infinity:
            minRank = 0  // Sink node (no successors)

        G.node(v).rank = minRank
        return minRank

    // Start from all nodes (DFS will naturally work from sinks)
    for each v in G.nodes():
        dfs(v)

    // Normalize: shift all ranks so minimum is 0
    minRank = min(rank for all nodes)
    for each v in G.nodes():
        v.rank -= minRank
```

### Characteristics

| Aspect | Description |
|--------|-------------|
| Time | O(V + E) |
| Space | O(V) for recursion |
| Quality | Valid but not optimal |
| Tendency | Pushes nodes toward sink, creating wide bottom layers |

---

## Algorithm 2: Network Simplex

The optimal ranking algorithm that minimizes total edge length. More complex but produces tighter layouts.

### Key Concepts

1. **Slack**: For edge (u, v): `slack = rank(v) - rank(u) - minlen`
   - Slack = 0: Edge is "tight" (exactly minlen apart)
   - Slack > 0: Edge is longer than required

2. **Feasible Spanning Tree**: A spanning tree where all tree edges are tight (slack = 0)

3. **Cut Value**: For each tree edge, measures the benefit of removing it
   - Negative cut value = removing the edge can improve the solution

4. **Low/Lim Values**: Enable O(1) descendant queries in the tree

### How It Works

```
Algorithm NetworkSimplex(G):
    // Step 1: Initial ranking (use Longest Path)
    longestPath(G)

    // Step 2: Build feasible spanning tree (all edges tight)
    tree = feasibleTree(G)

    // Step 3: Initialize tree structure for efficient queries
    initLowLimValues(tree)   // For O(1) descendant queries
    initCutValues(tree, G)   // Edge cut values for pivoting

    // Step 4: Iterate until optimal
    while true:
        // Find tree edge with negative cut value (leaving edge)
        e = findLeavingEdge(tree)
        if e == null:
            break  // Optimal!

        // Find non-tree edge to replace it (entering edge)
        f = findEnteringEdge(tree, G, e)

        // Pivot: swap edges in spanning tree
        exchangeEdges(tree, G, e, f)

        // Update cut values
        updateCutValues(tree, G)
```

### Low/Lim Values

These enable O(1) queries for "is node A a descendant of node B?":

```
         1
        /|\
       2 3 4
      /|   |
     5 6   7

For each node, compute:
- low[v]: Minimum postorder number in subtree
- lim[v]: Node's postorder number (max in subtree)

Node u is descendant of v iff: low[v] <= lim[u] <= lim[v]
```

---

## Implementation

### Main Entry Point

```go
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
```

### Longest Path Implementation

```go
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
```

### Rank Normalization

```go
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
```

---

## Building the Layers Structure

After ranking, we organize nodes into layers:

```go
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

    // Assign nodes to layers (sorted for determinism)
    nodeIDs := make([]string, 0, len(s.nodes))
    for id := range s.nodes {
        nodeIDs = append(nodeIDs, id)
    }
    sort.Strings(nodeIDs)

    for _, id := range nodeIDs {
        node := s.nodes[id]
        s.layers[node.rank] = append(s.layers[node.rank], id)
    }
}
```

### Data Structure

```go
// After buildLayers():
s.layers = [][]string{
    {"A"},           // Rank 0
    {"B", "C"},      // Rank 1
    {"D"},           // Rank 2
}

// Each node knows its rank:
s.nodes["A"].rank = 0
s.nodes["B"].rank = 1
s.nodes["C"].rank = 1
s.nodes["D"].rank = 2
```

---

## Choosing a Ranker

| Algorithm | Pros | Cons | Best For |
|-----------|------|------|----------|
| Longest Path | Fast O(V+E), simple | May produce more layers | Interactive use, large graphs |
| Network Simplex | Optimal, minimal layers | More complex, slower | Final layouts, print quality |

### Configuration

```go
type RankAlgorithm int

const (
    LongestPath    RankAlgorithm = iota  // Default
    NetworkSimplex
)

// Usage:
layout := g.Layout(posit.Options{
    Algorithm: posit.NetworkSimplex,  // Use optimal ranking
})
```

---

## Complexity Analysis

### Longest Path

| Operation | Time | Space |
|-----------|------|-------|
| DFS traversal | O(V + E) | O(V) recursion |
| Normalization | O(V) | O(1) |
| Build layers | O(V log V) | O(V) |
| **Total** | **O(V + E)** | **O(V)** |

### Network Simplex

| Operation | Time | Space |
|-----------|------|-------|
| Initial ranking | O(V + E) | O(V) |
| Build feasible tree | O(V × E) | O(V + E) |
| Each pivot | O(V) | O(1) |
| Total (practical) | O(V × E) | O(V + E) |
| Total (worst case) | O(V² × E) | O(V + E) |

In practice, network simplex converges quickly (usually < 10 iterations).

---

## Testing

### Test Cases

```go
func TestRank_LinearChain(t *testing.T) {
    // A → B → C should have ranks 0, 1, 2
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    if state.nodes["A"].rank != 0 {
        t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
    }
    if state.nodes["B"].rank != 1 {
        t.Errorf("Expected B at rank 1, got %d", state.nodes["B"].rank)
    }
    if state.nodes["C"].rank != 2 {
        t.Errorf("Expected C at rank 2, got %d", state.nodes["C"].rank)
    }
}

func TestRank_Diamond(t *testing.T) {
    // A → (B, C) → D should have A:0, B:1, C:1, D:2
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("D", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")
    g.AddEdge("B", "D")
    g.AddEdge("C", "D")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    if state.nodes["A"].rank != 0 {
        t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
    }
    if state.nodes["B"].rank != 1 || state.nodes["C"].rank != 1 {
        t.Errorf("Expected B and C at rank 1")
    }
    if state.nodes["D"].rank != 2 {
        t.Errorf("Expected D at rank 2, got %d", state.nodes["D"].rank)
    }
}

func TestRank_DisconnectedComponents(t *testing.T) {
    // Two separate chains should both start at rank 0
    g := NewGraph()
    // Chain 1
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    // Chain 2
    g.AddNode("X", NodeOptions{Width: 100, Height: 50})
    g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("X", "Y")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    // Both roots should be at rank 0
    if state.nodes["A"].rank != 0 {
        t.Errorf("Expected A at rank 0, got %d", state.nodes["A"].rank)
    }
    if state.nodes["X"].rank != 0 {
        t.Errorf("Expected X at rank 0, got %d", state.nodes["X"].rank)
    }
}

func TestRank_EdgeConstraintViolation(t *testing.T) {
    // Verify all edges satisfy rank constraints
    g := buildRandomDAG(50, 75)

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    for key, edge := range state.edges {
        fromRank := state.nodes[key.from].rank
        toRank := state.nodes[key.to].rank
        minlen := edge.minlen
        if minlen == 0 {
            minlen = 1
        }

        if toRank-fromRank < minlen {
            t.Errorf("Edge %s->%s violates constraint: rank diff %d < minlen %d",
                key.from, key.to, toRank-fromRank, minlen)
        }
    }
}
```

---

## Visual Examples

### Example 1: Linear Chain

```
Input:  A → B → C → D

DFS from A:
  A calls dfs(B)
    B calls dfs(C)
      C calls dfs(D)
        D is sink: rank = 0
      return 0
      C.rank = 0 - 1 = -1
    return -1
    B.rank = -1 - 1 = -2
  return -2
  A.rank = -2 - 1 = -3

After normalization (shift by +3):
  A.rank = 0
  B.rank = 1
  C.rank = 2
  D.rank = 3

Layers:
  [0]: A
  [1]: B
  [2]: C
  [3]: D
```

### Example 2: Diamond Pattern

```
Input:  A → B → D
        A → C → D

DFS from A:
  A calls dfs(B)
    B calls dfs(D)
      D is sink: rank = 0
    return 0
    B.rank = 0 - 1 = -1
  return -1

  A calls dfs(C)
    C calls dfs(D)
      D already visited: return 0
    C.rank = 0 - 1 = -1
  return -1

  A.rank = min(-1, -1) - 1 = -2

After normalization (shift by +2):
  A.rank = 0
  B.rank = 1
  C.rank = 1
  D.rank = 2

Layers:
  [0]: A
  [1]: B, C
  [2]: D
```

### Example 3: Long Edge with minlen

```
Input:  A → B (minlen=1)
        A → C (minlen=3)
        C → D (minlen=1)

With longest path:
  D is sink: rank = 0
  C.rank = 0 - 1 = -1
  B is sink: rank = 0
  A.rank = min(B - 1, C - 3) = min(-1, -4) = -4

After normalization:
  A.rank = 0
  B.rank = 3 (was 0, minus -4)
  C.rank = 3 (was -1, minus -4)
  D.rank = 4 (was 0, minus -4)

Wait, that's not right. Let me recalculate:
  If C.rank = -1 and A.rank = C.rank - 3 = -4
  Then B.rank = A.rank + 1 = -3

Normalized (shift +4):
  A.rank = 0
  B.rank = 1
  C.rank = 3
  D.rank = 4

This satisfies:
  A → B: 1 - 0 = 1 >= 1 ✓
  A → C: 3 - 0 = 3 >= 3 ✓
  C → D: 4 - 3 = 1 >= 1 ✓
```

---

## Post-Conditions

After this phase completes:

1. ✅ Every node has a valid `rank` >= 0
2. ✅ For every edge (u, v): `rank(v) - rank(u) >= minlen`
3. ✅ `s.layers` array is populated with node IDs per layer
4. ✅ Minimum rank is 0 (normalization applied)

---

## Next Phase

→ [Phase 3: Dummy Nodes](./PHASE_3_DUMMY_NODES.md)
