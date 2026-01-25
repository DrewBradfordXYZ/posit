# Network Simplex Improvements

Improvements identified from code review comparing Posit's implementation against Graphviz, ELK, and dagre.

**Note:** Anti-stacking (`STACKING_PREVENTION.md`) is now COMPLETE. These performance optimizations are the next priority to enable anti-stacking on large graphs (currently limited to ≤100 nodes).

## Status Overview

| Improvement | Priority | Effort | Status |
|-------------|----------|--------|--------|
| Cut value ok idiom | High | Low | ✅ Done |
| Adjacency lists | Medium | Low | ✅ Done |
| Subtree removal | Medium | Medium | ✅ Done |
| **O(1) swap-delete for adjacency** | **High** | **Low** | ⬚ Todo |
| Leave edge search limit | Medium | Low | ⬚ Todo |
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
2. **O(1) swap-delete for adjacency removal** - Uses array swap with last element instead of linear search and shift
3. **Search limit of 30 edges** - Caps leave edge search to prevent pathological cases (SEARCHSIZE constant)
4. **Union-find with path compression** - Uses union-find for O(α(n)) subtree membership tests during feasible tree construction
5. **Immediate tree edge lookup** - Each node stores its parent tree edge directly, avoiding repeated searches

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
| O(1) adjacency removal | ✅ swap-delete | ✅ edge-centric | ❌ | ❌ O(n) linear |
| Leave edge search limit | ✅ (30) | ✅ | ❌ | ❌ |
| Iteration limit | ✅ (999) | ✅ | ❌ | ✅ (n×m) |
| Union-find for tree | ✅ | ❌ | ❌ | ❌ |
| Postorder (low/lim) | ✅ | ✅ | ✅ | ✅ |

---

## Completed

### ✅ Cut Value Lookup Bug (High Priority)
**Commit:** `a962e17`

Fixed bug where `childCutValue == 0` conflated "not found" with "actual zero value". Now uses Go's `ok` idiom.

### ✅ Adjacency Lists (Medium Priority)
**Commit:** `83c6228`

Both Y and X simplex spanning trees now maintain `adj map[string][]string` for O(1) neighbor lookup. The `neighbors()` function returns `t.adj[v]` directly instead of iterating all tree edges.

### ✅ Subtree Removal (Medium Priority)
**Commit:** `83c6228`

Leaf nodes (degree 1) are removed before running network simplex and reattached afterward. Uses ELK's threshold of 40 nodes. Implemented for both Y simplex (`removeSubtreeLeaves`/`reattachSubtreeLeaves`) and X simplex (`removeAuxSubtreeLeaves`/`reattachAuxSubtreeLeaves`).

---

## Todo

### 1. O(1) Swap-Delete for Adjacency Removal (High Priority)
**Reference:** Graphviz `ns.c` lines 122-128

**Problem:** Current `removeFromSlice()` is O(n) per call - it searches linearly then shifts all elements:

```go
// Current - O(n) per call, allocates new slice
func removeFromSlice(slice []string, val string) []string {
    for i, v := range slice {
        if v == val {
            return append(slice[:i], slice[i+1:]...)  // Shift all elements
        }
    }
    return slice
}
```

This is called in:
- `removeEdge()` - 2 calls per edge removal
- `removeAuxSubtreeLeaves()` - 2-4 calls per node removal
- `exchangeEdges()` - 2 calls per tree rebalance (tight inner loop!)

**Impact:** Creates O(n²) complexity in tight loops instead of intended O(n log n).

**Fix:** Use Graphviz's swap-delete pattern:

```go
// Better - O(1) per call, no allocation
func removeFromSlice(slice []string, val string) []string {
    for i, v := range slice {
        if v == val {
            slice[i] = slice[len(slice)-1]  // Swap with last
            return slice[:len(slice)-1]      // Shrink (no shift)
        }
    }
    return slice
}
```

**Note:** This changes iteration order (adjacency list becomes unordered), but that's fine since we sort for determinism elsewhere.

**Files to modify:** `simplex.go` line 84-91

**Expected impact:** Fixes algorithmic complexity from O(n²) to O(n log n) in edge exchange loops

---

### 2. Leave Edge Search Limit (Medium Priority)
**Reference:** Graphviz `ns.c` line 55: `SEARCHSIZE = 30`

**Problem:** Current `leaveEdge()` always scans all tree edges to find one with negative cut value. On pathological graphs, this can be slow.

**Graphviz approach:** After finding 30 negative cut value edges, return the best found so far instead of continuing. This provides:
- Early termination on large graphs
- Load balancing (circular search from last position)

**Current code:**
```go
func (t *spanningTree) leaveEdge(s *layoutState) (edgeKey, bool) {
    // ... iterates ALL edges, no limit
}
```

**Proposed change:**
```go
const leaveEdgeSearchLimit = 30

func (t *spanningTree) leaveEdge(s *layoutState) (edgeKey, bool) {
    found := 0
    var best edgeKey
    bestCut := 0
    // ... iterate edges ...
    if cutVal < bestCut {
        best = key
        bestCut = cutVal
        found++
        if found >= leaveEdgeSearchLimit {
            return best, true
        }
    }
}
```

**Files to modify:** `simplex.go`

**Expected impact:** 5-10% speedup on pathological graphs, prevents hangs

---

### 3. Incremental Cut Value Updates (Medium Priority - Deferred)
**Reference:** Graphviz `ns.c` lines 85-112, 631-703

Currently recomputes ALL cut values after each edge exchange. Graphviz only updates cut values along the path between the leaving and entering edges.

**Status:** Attempted implementation produced incorrect results due to complex edge direction tracking. ELK also uses full recompute with a TODO noting this optimization. Deferred until correctness issues can be resolved.

**Complexity reduction:** O(n+m) → O(path length) per iteration

**Expected impact:** 2-10x speedup when many iterations needed

---

### 4. Cached Sorted Lists (Low Priority)

Currently sorts node/edge lists on every `leaveEdge()` and `enterEdge()` call for determinism.

**Proposed:** Sort once during tree construction, maintain sorted order.

**Expected impact:** Minor (sorting is fast, but unnecessary work)

---

### 5. Generics Consolidation (Low Priority)

Y simplex and X simplex have nearly identical tree operations with different types. Could use Go generics to reduce code duplication (~400 lines).

**Tradeoff:**
- Pro: Reduces code duplication
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

A 107-node, 575-edge graph caused the iteration count to explode (`maxIterations = nodes × edges` = 1.36M). Implementing the O(1) swap-delete fix and leave edge search limit would help enable anti-stacking on larger graphs.

---

## References

- **Graphviz:** `_ref/graphviz/lib/dotgen/` - `rank.c`, `position.c`, `ns.c`
- **ELK:** `_ref/elk/plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/`
- **dagre:** `_ref/dagre/lib/rank/network-simplex.js`
- **Paper:** Gansner et al. 1993 "A Technique for Drawing Directed Graphs" - `literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`
