# Network Simplex Improvements

Improvements identified from code review comparing Posit's implementation against Graphviz, ELK, and dagre.

**Note:** Anti-stacking (`STACKING_PREVENTION.md`) is now COMPLETE. These performance optimizations are the next priority to enable anti-stacking on large graphs (currently limited to ≤100 nodes).

## Status Overview

| Improvement | Priority | Effort | Status |
|-------------|----------|--------|--------|
| Cut value ok idiom | High | Low | ✅ Done |
| Adjacency lists | Medium | Low | ✅ Done |
| Subtree removal | Medium | Medium | ✅ Done |
| O(1) swap-delete for adjacency | High | Low | ✅ Done |
| Leave edge search limit | Medium | Low | ✅ Done |
| Incremental cut values | Medium | High | ✅ Done |
| Cached sorted lists | Low | Low | ⬚ Todo |
| Generics consolidation | Low | High | ⬚ Todo |

### Implementation Notes

**Adjacency lists (Done):** Both Y and X simplex spanning trees now maintain adjacency lists for O(1) neighbor lookup during tree traversals. This replaces the O(edges) iteration that was previously used.

**Subtree removal (Done):** Leaf nodes (degree 1) are now removed before running network simplex and reattached afterward with trivially computed positions. This follows ELK's approach with a threshold of 40 nodes. Applies to both Y simplex (ranking) and X simplex (coordinate assignment).

**O(1) swap-delete (Done):** The `removeFromSlice()` helper now uses swap-with-last-element pattern for O(1) removal instead of O(n) linear shift. This fixes algorithmic complexity in the `exchangeEdges()` tight loop.

**Leave edge search limit (Done):** Both Y and X simplex `leaveEdge()` functions now stop after finding 30 negative cut value candidates (following Graphviz's SEARCHSIZE heuristic). Returns the most negative found for better convergence.

**Incremental cut values (Done):** Following Graphviz's `invalidate_path()` approach, `treeUpdate()` walks from endpoints to LCA updating cut values along the path, and `invalidatePath()` marks affected nodes for DFS recomputation. The key insight is the direction flip logic: when walking up the tree, the sign of cut value updates depends on whether we're on the "from" or "to" side of each edge. This provides an additional 30.6% speedup, bringing total improvement to 51% faster than baseline.

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
| Incremental cut values | ✅ `invalidate_path()` | ❌ Full recompute | ❌ Full recompute | ✅ `treeUpdate()` |
| Subtree removal | ❌ | ✅ (≥40 nodes) | ❌ | ✅ (≥40 nodes) |
| Adjacency lists | ✅ | ✅ | ❌ | ✅ |
| O(1) adjacency removal | ✅ swap-delete | ✅ edge-centric | ❌ | ✅ swap-delete |
| Leave edge search limit | ✅ (30) | ✅ | ❌ | ✅ (30) |
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

### ✅ O(1) Swap-Delete for Adjacency Removal (High Priority)
**Commit:** `68f4197`

The `removeFromSlice()` helper now uses Graphviz's swap-with-last-element pattern for O(1) removal instead of O(n) linear shift. This fixes algorithmic complexity from O(n²) to O(n log n) in the `exchangeEdges()` tight loop.

### ✅ Leave Edge Search Limit (Medium Priority)
**Commit:** `68f4197`

Both Y and X simplex `leaveEdge()` functions now stop after finding 30 negative cut value candidates (following Graphviz's SEARCHSIZE=30 heuristic). Returns the most negative found for better convergence. Prevents pathological cases on large graphs.

### ✅ Incremental Cut Value Updates (Medium Priority)
**Commit:** `bd70239`

Following Graphviz's `invalidate_path()` approach:
- `treeUpdate()` / `xTreeUpdate()`: walks from endpoints to LCA, updating cut values with proper direction handling
- `invalidatePath()` / `xInvalidatePath()`: marks affected nodes for DFS recomputation
- `exchangeEdges()`: uses incremental updates instead of full recompute

The key insight is the direction flip logic: when walking up the tree, the sign of cut value updates depends on whether we're on the "from" or "to" side of each edge. Provides an additional 30.6% speedup.

---

## Todo

### 1. Cached Sorted Lists (Low Priority)

Currently sorts node/edge lists on every `leaveEdge()` and `enterEdge()` call for determinism.

**Proposed:** Sort once during tree construction, maintain sorted order.

**Expected impact:** Minor (sorting is fast, but unnecessary work)

---

### 3. Generics Consolidation (Low Priority)

Y simplex and X simplex have nearly identical tree operations with different types. Could use Go generics to reduce code duplication (~400 lines).

**Tradeoff:**
- Pro: Reduces code duplication
- Con: More complex type signatures, harder to read
- Con: High effort for no runtime benefit

**Recommendation:** Low priority unless significant new simplex work planned.

---

## Performance Comparison

### Benchmark Results (All Optimizations)

Total measured improvement: **51% faster overall** (geometric mean across all benchmarks).

| Benchmark | Baseline | Final | Improvement |
|-----------|----------|-------|-------------|
| X_Simplex_Layered20x5 | 287ms | 69ms | **76% faster** |
| Y_Layered20x5 | 40.8ms | 19.4ms | **52% faster** |
| X_Simplex_Layered10x5 | 40.5ms | 15.0ms | **63% faster** |
| Y_Layered10x5 | 7.6ms | 3.9ms | **49% faster** |
| AntiStack_Layered10x5 | 34.6ms | 17.0ms | **51% faster** |
| Y_Chain200 | 2.0ms | 1.8ms | **13% faster** |
| Y_Diamond25 | 4.2ms | 3.8ms | **11% faster** |

**Optimization breakdown:**
- Adjacency lists + subtree removal: ~23% improvement
- O(1) swap-delete + search limit: additional ~8% improvement
- Incremental cut values: additional ~31% improvement

Layered graphs benefit most from the full optimization suite. X simplex sees the largest gains due to the auxiliary graph having more edges.

### Current Anti-Stacking Limitation

Anti-stacking is currently disabled for graphs >100 nodes due to performance. The constraint:

```go
// simplex.go line ~735
if xs.s.opts.PreventStacking && len(xs.s.nodes) <= 100 {
    xs.addAntiStackingEdges()
}
```

With the optimizations now complete, this limit could potentially be raised. Testing needed to determine the new safe threshold.

---

## References

- **Graphviz:** `_ref/graphviz/lib/dotgen/` - `rank.c`, `position.c`, `ns.c`
- **ELK:** `_ref/elk/plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/`
- **dagre:** `_ref/dagre/lib/rank/network-simplex.js`
- **Paper:** Gansner et al. 1993 "A Technique for Drawing Directed Graphs" - `literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`
