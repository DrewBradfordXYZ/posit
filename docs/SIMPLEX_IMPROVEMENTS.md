# Network Simplex Improvements

Improvements identified from code review comparing Posit's implementation against Graphviz, ELK, and dagre.

**Note:** These are performance optimizations. Defer until after `STACKING_PREVENTION.md` is complete (anti-stacking constraints are the next feature priority).

## Status Overview

| Improvement | Priority | Effort | Status |
|-------------|----------|--------|--------|
| Cut value ok idiom | High | Low | ✅ Done |
| Subtree removal | Medium | Medium | ⬚ Todo |
| Adjacency lists | Medium | Low | ⬚ Todo |
| Incremental cut values | Medium | High | ⬚ Todo |
| Cached sorted lists | Low | Low | ⬚ Todo |
| Generics consolidation | Low | High | ⬚ Todo |

---

## Completed

### ✅ Cut Value Lookup Bug (High Priority)
**Commit:** `a962e17`

Fixed bug where `childCutValue == 0` conflated "not found" with "actual zero value". Now uses Go's `ok` idiom.

---

## Todo

### 1. Subtree Removal (Medium Priority)
**Reference:** ELK `NetworkSimplex.java` lines ~100-150

Remove leaf nodes (degree 1) before running simplex, reattach trivially after. Reduces problem size significantly for graphs with chains.

**Algorithm:**
```
1. Find all leaf nodes (degree 1)
2. Push to stack, remove from graph
3. Repeat until no leaves remain
4. Run network simplex on reduced graph
5. Pop from stack, place at: parent.coord ± edge.delta
```

**ELK details:**
- Threshold: Only applies for graphs ≥40 nodes
- Uses stack for LIFO reattachment order
- Handles both incoming and outgoing single edges

**Files to modify:** `simplex.go`

**Expected impact:** 2-10x speedup on graphs with long chains

---

### 2. Adjacency Lists for Spanning Tree (Medium Priority)
**Reference:** All implementations use some form of adjacency tracking

Current `neighbors()` iterates ALL tree edges to find neighbors - O(edges) per call. With adjacency lists, it's O(degree).

**Current code (lines 81-89, 911-915):**
```go
func (t *spanningTree) neighbors(v string) []string {
    var result []string
    for key := range t.treeEdges {  // O(all edges)
        if key.from == v {
            result = append(result, key.to)
        }
    }
    return result
}
```

**Proposed change:**
```go
type spanningTree struct {
    nodes     map[string]*treeNode
    treeEdges map[edgeKey]bool
    adj       map[string][]string  // NEW: adjacency lists
    cutValues map[edgeKey]int
    root      string
}

func (t *spanningTree) addEdge(key edgeKey) {
    t.treeEdges[key] = true
    t.treeEdges[edgeKey{from: key.to, to: key.from}] = true
    t.adj[key.from] = append(t.adj[key.from], key.to)  // NEW
    t.adj[key.to] = append(t.adj[key.to], key.from)    // NEW
}

func (t *spanningTree) neighbors(v string) []string {
    return t.adj[v]  // O(1) lookup
}
```

**Files to modify:** `simplex.go` (both Y and X sections)

**Expected impact:** ~2-5x speedup for tree traversal operations

---

### 3. Incremental Cut Value Updates (Medium Priority)
**Reference:** Graphviz `ns.c` - only updates path between swapped edges

Currently recomputes ALL cut values after each edge exchange (lines 492, 1170). Graphviz only updates cut values along the path between the leaving and entering edges.

**Current code:**
```go
func (t *spanningTree) exchangeEdges(s *layoutState, leave, enter edgeKey) {
    // ... swap edges ...
    t.initLowLimValues()  // Full recompute
    t.initCutValues(s)    // Full recompute - O(n+m)
}
```

**Graphviz approach:**
1. Identify the path between leave.tail and enter endpoints
2. Only recompute cut values for edges on this path
3. Use `invalidate_path()` to mark affected nodes

**Complexity reduction:** O(n+m) → O(path length) per iteration

**Files to modify:** `simplex.go`

**Expected impact:** 2-10x speedup when many iterations needed

**Note:** ELK has a TODO comment noting they should do this but haven't yet.

---

### 4. Cached Sorted Lists (Low Priority)
**Lines:** 348-352, 398-407, 1044-1048, 1086-1092

Currently sorts node/edge lists on every `leaveEdge()` and `enterEdge()` call.

**Current:**
```go
func (t *spanningTree) leaveEdge() (edgeKey, bool) {
    nodeIDs := make([]string, 0, len(t.nodes))
    for id := range t.nodes {
        nodeIDs = append(nodeIDs, id)
    }
    sort.Strings(nodeIDs)  // Repeated every call
    // ...
}
```

**Proposed:** Sort once during tree construction, maintain sorted order.

**Files to modify:** `simplex.go`

**Expected impact:** Minor (sorting is fast, but unnecessary work)

---

### 5. Generics Consolidation (Low Priority)
**Observation:** Y simplex and X simplex have nearly identical tree operations with different types.

| Component | Y Simplex | X Simplex |
|-----------|-----------|-----------|
| Tree type | `spanningTree` | `xSpanningTree` |
| Node type | `*treeNode` | `*xAuxNode` |
| Cut values | `map[edgeKey]int` | `map[xEdgeKey]float64` |
| Edge key | `edgeKey` | `xEdgeKey` |

**Potential approach:** Use Go generics to create a single parameterized spanning tree type.

```go
type SpanningTree[K comparable, V numeric] struct {
    nodes     map[string]*TreeNode[V]
    treeEdges map[K]bool
    cutValues map[K]V
    root      string
}
```

**Tradeoff:**
- Pro: Reduces code duplication (~400 lines)
- Con: More complex type signatures, harder to read
- Con: High effort for no runtime benefit

**Recommendation:** Low priority unless significant new simplex work planned.

---

## Performance Comparison

From benchmarks (50-node graph):

| Algorithm | Current | With optimizations (estimated) |
|-----------|---------|-------------------------------|
| Brandes-Köpf | ~630μs | N/A |
| Network Simplex | ~26.5ms | ~5-10ms |

The optimizations primarily help Network Simplex, which is 42x slower than BK but provides global optimality needed for anti-stacking constraints.

---

## References

- **Graphviz:** `_ref/graphviz/lib/dotgen/` - `rank.c`, `position.c`, `ns.c`
- **ELK:** `_ref/elk/plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/`
- **dagre:** `_ref/dagre/lib/rank/network-simplex.js`
- **Paper:** Gansner et al. 1993 "A Technique for Drawing Directed Graphs" - `literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`
