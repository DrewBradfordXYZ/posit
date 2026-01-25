# Network Simplex Improvements

Improvements identified from code review comparing Posit's implementation against Graphviz, ELK, and dagre.

**Note:** Anti-stacking (`STACKING_PREVENTION.md`) is now COMPLETE. These performance optimizations are the next priority to enable anti-stacking on large graphs (currently limited to ≤100 nodes).

## Status Overview

| Improvement | Priority | Effort | Status |
|-------------|----------|--------|--------|
| Cut value ok idiom | High | Low | ✅ Done |
| Adjacency lists | Medium | Low | ✅ Done |
| Subtree removal | Medium | Medium | ✅ Done |
| Incremental cut values | Medium | High | ⏸️ Deferred |
| Cached sorted lists | Low | Low | ⬚ Todo |
| Generics consolidation | Low | High | ⬚ Todo |

### Implementation Notes

**Adjacency lists (Done):** Both Y and X simplex spanning trees now maintain adjacency lists for O(1) neighbor lookup during tree traversals. This replaces the O(edges) iteration that was previously used.

**Subtree removal (Done):** Leaf nodes (degree 1) are now removed before running network simplex and reattached afterward with trivially computed positions. This follows ELK's approach with a threshold of 40 nodes. Applies to both Y simplex (ranking) and X simplex (coordinate assignment).

**Incremental cut values (Deferred):** Initial implementation following Graphviz's `invalidate_path()` approach was attempted but produced incorrect results due to complex edge direction tracking during cut value propagation. The ELK implementation also uses full recompute with a TODO noting this optimization. Deferred for future work - the current full recompute is correct and benefits from adjacency list speedup.

---

## Reference Implementation Comparison

Detailed analysis of how Graphviz, ELK, and dagre implement network simplex optimizations.

### Graphviz (Most Mature)

**Location:** `_ref/graphviz/lib/dotgen/ns.c`

**Key optimizations:**
1. **Incremental cut value updates via `invalidate_path()`** - Only recomputes cut values for nodes on the path between exchanged edges, not the entire tree. This is the single biggest optimization.
2. **Union-find with path compression** - Uses union-find for O(α(n)) subtree membership tests during feasible tree construction
3. **Search limit of 999 iterations** - Caps leave edge search to prevent pathological cases
4. **Immediate tree edge lookup** - Each node stores its parent tree edge directly, avoiding repeated searches

**Architecture:**
- `init_rank()` - Initial feasible ranking using longest path
- `feasible_tree()` - Builds initial spanning tree with tight edges
- `rank()` - Main loop calling `leave_edge()` / `enter_edge()` / `update()` / `rerank()`
- `cut_value()` + `x_cutval()` - Cut value computation with incremental updates
- `invalidate_path()` - Marks only affected path for recomputation

### ELK (Clean Java Implementation)

**Location:** `_ref/elk/plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/`

**Key optimizations:**
1. **Subtree removal** - For graphs ≥40 nodes, removes leaf nodes (degree 1) before simplex, reattaches after. Can reduce problem size by 50%+ on chain-heavy graphs.
2. **Full cut value recompute** - Unlike Graphviz, ELK recomputes all cut values after each exchange. Has a TODO comment noting they should implement incremental updates.
3. **Postorder values (low/lim)** - Standard technique for O(1) subtree membership tests

**Architecture:**
- `NetworkSimplex.java` - Main algorithm with `process()` entry point
- `NEdge.java` / `NNode.java` - Edge/node wrappers with tree metadata
- Threshold-based subtree removal (40 nodes)
- Bounded iteration count

### dagre (Simplest Implementation)

**Location:** `_ref/dagre/lib/rank/network-simplex.js`

**Key characteristics:**
1. **No optimizations** - Simplest implementation, good for understanding the algorithm
2. **Full recompute every iteration** - No incremental cut values
3. **No subtree removal** - Processes full graph always
4. **Unbounded iterations** - No search limits (can hang on pathological inputs)

**Architecture:**
- `networkSimplex()` - Main entry point
- `feasibleTree()` - Initial tree construction
- `initLowLimValues()` / `initCutValues()` - Full recomputation
- `leaveEdge()` / `enterEdge()` / `exchangeEdges()` - Standard simplex operations

### Comparison Summary

| Feature | Graphviz | ELK | dagre | Posit (current) |
|---------|----------|-----|-------|-----------------|
| Incremental cut values | ✅ `invalidate_path()` | ❌ Full recompute | ❌ Full recompute | ❌ Full recompute |
| Subtree removal | ❌ | ✅ (≥40 nodes) | ❌ | ✅ (≥40 nodes) |
| Adjacency lists | ✅ | ✅ | ❌ | ✅ |
| Iteration limit | ✅ (999) | ✅ | ❌ | ✅ (n×m) |
| Union-find for tree | ✅ | ❌ | ❌ | ❌ |
| Postorder (low/lim) | ✅ | ✅ | ✅ | ✅ |

### Implementation Priority (Recommended Order)

Based on impact/effort ratio:

1. **Subtree removal** - Medium effort, 2-10x speedup on chain-heavy graphs
2. **Incremental cut values** - High effort, 2-10x speedup on iteration-heavy graphs
3. **Adjacency lists** - Low effort, 2-5x speedup on tree traversals
4. **Leave edge search limit** - Low effort, prevents pathological hangs
5. **Better iteration cap** - Low effort, scales with graph structure not n×m

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

Currently recomputes ALL cut values after each edge exchange. Graphviz only updates cut values along the path between the leaving and entering edges.

**Graphviz Algorithm (from ns.c):**

```
exchangeEdges(leave, enter):
    1. slack = slack(enter)
    2. rerank(tailSubtree, delta)           // Adjust ranks to tighten enter edge
    3. cutvalue = cutValue(leave)
    4. lca = treeupdate(enter.from, enter.to, cutvalue, true)   // Update path from->LCA
    5. treeupdate(enter.to, enter.from, cutvalue, false)        // Update path to->LCA
    6. invalidate_path(lca, enter.from)     // Mark low=-1 for nodes needing DFS update
    7. invalidate_path(lca, enter.to)
    8. cutValue(enter) = -cutvalue
    9. cutValue(leave) = 0
    10. swap edges in tree
    11. dfs_range(lca, parent(lca), low(lca))  // Incremental DFS from LCA only
```

**Key functions:**

1. **treeupdate(v, w, cutvalue, dir)** - Walk from v toward w, adding/subtracting cutvalue:
   ```go
   func (t *spanningTree) treeupdate(s *layoutState, v, w string, cutvalue int, dir bool) string {
       for !t.isDescendant(w, v) {
           e := parentEdge(v)
           if v == tail(e) == dir {
               cutValues[e] += cutvalue
           } else {
               cutValues[e] -= cutvalue
           }
           v = parent(v)
       }
       return v  // LCA
   }
   ```

2. **invalidate_path(lca, node)** - Mark nodes for DFS recomputation:
   ```go
   func (t *spanningTree) invalidatePath(lca, node string) {
       for node != lca && low[node] != -1 {
           low[node] = -1  // Mark as needing recomputation
           node = parent(node)
       }
   }
   ```

3. **dfs_range(v, parent, low)** - Incremental DFS that skips unchanged subtrees:
   ```go
   func (t *spanningTree) dfsRange(v, parent string, low int) int {
       if t.nodes[v].parent == parent && t.nodes[v].low == low {
           return t.nodes[v].lim + 1  // Already correct, skip subtree
       }
       t.nodes[v].parent = parent
       t.nodes[v].low = low
       // DFS children, incrementing counter
       // Only recurse into children with low == -1 (invalidated)
       t.nodes[v].lim = counter
       return counter + 1
   }
   ```

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

### Benchmark Results (After Adjacency Lists + Subtree Removal)

Measured improvement: **23.5% faster overall** (geometric mean across all benchmarks).

| Benchmark | Before | After | Improvement |
|-----------|--------|-------|-------------|
| Y_Chain200 (200 nodes) | 6.35ms | 2.07ms | **67% faster** |
| Y_Chain100 (100 nodes) | 2.41ms | 1.45ms | **40% faster** |
| X_Simplex_Layered20x5 | 555ms | 287ms | **48% faster** |
| X_Simplex_Layered10x5 | 62.3ms | 40.5ms | **35% faster** |
| Y_Layered20x5 | 57.6ms | 40.1ms | **30% faster** |
| Y_Layered10x5 | 10.2ms | 7.7ms | **24% faster** |

Chain graphs benefit most from subtree removal since leaf nodes are iteratively removed before simplex runs.

### Current Anti-Stacking Limitation

Anti-stacking is currently disabled for graphs >100 nodes due to performance. The constraint:

```go
// simplex.go line ~735
if xs.s.opts.PreventStacking && len(xs.s.nodes) <= 100 {
    xs.addAntiStackingEdges()
}
```

A 107-node, 575-edge graph caused the iteration count to explode (`maxIterations = nodes × edges` = 1.36M). Implementing the optimizations above (especially subtree removal and incremental cut values) would allow raising or removing this limit.

---

## References

- **Graphviz:** `_ref/graphviz/lib/dotgen/` - `rank.c`, `position.c`, `ns.c`
- **ELK:** `_ref/elk/plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/`
- **dagre:** `_ref/dagre/lib/rank/network-simplex.js`
- **Paper:** Gansner et al. 1993 "A Technique for Drawing Directed Graphs" - `literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`
