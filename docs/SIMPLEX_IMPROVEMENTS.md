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
| Critical bug fixes (code review) | High | Low | ✅ Done |
| LCA validation in invalidatePath | Medium | Low | ✅ Done |
| Cached sorted lists | Low | Low | ✅ Done |
| Circular search index | Low | Low | ✅ Done |
| Incremental low/lim recompute | Low | High | ⏸️ Deferred |
| Union-find for tree | Medium | High | ⬚ Todo |
| DFS-based enter edge search | Medium | Medium | ⬚ Todo |
| Generics consolidation | Low | High | ⬚ Todo |

### Implementation Notes

**Adjacency lists (Done):** Both Y and X simplex spanning trees now maintain adjacency lists for O(1) neighbor lookup during tree traversals. This replaces the O(edges) iteration that was previously used.

**Subtree removal (Done):** Leaf nodes (degree 1) are now removed before running network simplex and reattached afterward with trivially computed positions. This follows ELK's approach with a threshold of 40 nodes. Applies to both Y simplex (ranking) and X simplex (coordinate assignment).

**O(1) swap-delete (Done):** The `removeFromSlice()` helper now uses swap-with-last-element pattern for O(1) removal instead of O(n) linear shift. This fixes algorithmic complexity in the `exchangeEdges()` tight loop.

**Leave edge search limit (Done):** Both Y and X simplex `leaveEdge()` functions now stop after finding 30 negative cut value candidates (following Graphviz's SEARCHSIZE heuristic). Returns the most negative found for better convergence.

**Incremental cut values (Done):** Following Graphviz's `invalidate_path()` approach, `treeUpdate()` walks from endpoints to LCA updating cut values along the path, and `invalidatePath()` marks affected nodes for DFS recomputation. The key insight is the direction flip logic: when walking up the tree, the sign of cut value updates depends on whether we're on the "from" or "to" side of each edge. This provides an additional 30.6% speedup, bringing total improvement to 51% faster than baseline.

**LCA validation (Done):** Both `invalidatePath()` and `xInvalidatePath()` now include Graphviz-style defensive checks to detect if we've skipped over the LCA. This prevents infinite loops if postorder values become corrupted.

**Cached sorted lists (Done):** `sortedNodeIDs` cached at end of `feasibleTree()`/`xFeasibleTree()`, and `sortedEdgeKeys` cached before simplex loop. Eliminates allocation+sort on every `leaveEdge()`/`enterEdge()` call.

**Circular search index (Done):** Both Y and X simplex `leaveEdge()` now maintain a `searchIndex` to continue from the previous position, wrapping around for full coverage. Follows Graphviz's `S_i` pattern for better cache locality.

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

## Code Review Findings (January 2026)

Comprehensive code review comparing Posit against Graphviz and ELK identified the following issues:

### Bugs Fixed (Commit `81d7dbd`)

| Issue | Severity | Location | Status |
|-------|----------|----------|--------|
| Missing child check in Y simplex | **Critical** | `assignCutValue()` | ✅ Fixed |
| Missing nil guard in Y simplex DFS | **Critical** | `initLowLimValues()` | ✅ Fixed |
| Empty graph panic risk | High | `feasibleTree()` | ✅ Fixed |

### Remaining Issues

| Issue | Severity | Location | Description |
|-------|----------|----------|-------------|
| Missing LCA validation | Medium | `xInvalidatePath()` | Could silently skip LCA if postorder values corrupt |

### Dead Code

- `initLowLimValuesIncremental()` (lines 553-585) exists but is never called
- Comment at line 640 acknowledges: "full recompute for now - incremental is complex"
- The `invalidatePath()` markers are set but not used for incremental DFS

### Code Consistency (Y vs X Simplex)

| Aspect | Y Simplex | X Simplex |
|--------|-----------|-----------|
| Nil checks in DFS | ✅ Present | ✅ Present |
| Child validation in cut value | ✅ Present | ✅ Present |
| Empty graph handling | ✅ Graceful | ✅ Graceful |
| Cut value type | `int` | `float64` with tolerance |

### What's Working Well

- All 11 contract test topologies pass
- Direction flip logic in `treeUpdate()` is correct
- O(1) swap-delete properly implemented
- Leave edge search limit (30) correct
- Subtree removal matches ELK approach

---

## Todo

### 1. Union-Find for Tree Construction (Medium Priority)

Graphviz uses union-find with path compression for O(α(n)) subtree membership tests during `feasible_tree()`. Posit currently iterates all edges O(N*E).

**Expected impact:** Significant on large graphs (>500 nodes)

**Effort:** High - requires architectural change

---

### 2. DFS-Based Enter Edge Search (Medium Priority)

Graphviz uses iterative DFS within the tail subtree to find entering edges, rather than checking all non-tree edges.

**Current approach:** `enterEdge()` iterates all edges, checking `isDescendant()` for each
**Better approach:** DFS only within relevant subtree

**Expected impact:** Medium (reduces edge iterations)

---

### 3. Generics Consolidation (Low Priority)

Y simplex and X simplex have nearly identical tree operations with different types. Could use Go generics to reduce code duplication (~400 lines).

**Tradeoff:**
- Pro: Reduces code duplication, fixes apply to both
- Con: More complex type signatures, harder to read
- Con: High effort for no runtime benefit

**Recommendation:** Low priority unless significant new simplex work planned.

---

## Deferred

### Incremental Low/Lim Recomputation

Infrastructure exists (`invalidatePath()` sets `low = -1`) but true incremental recomputation is complex:
- After edge exchange, subtree structure changes (nodes move between subtrees)
- Postorder numbering must remain globally consistent
- Need to correctly propagate values up to root

The current `initLowLimValuesIncremental()` doesn't actually implement incremental logic.

**Expected impact:** ~5-10% additional speedup
**Status:** Deferred due to complexity vs. benefit tradeoff

---

## Performance Comparison

### Benchmark Results (All Optimizations)

Total measured improvement: **57% faster overall** (geometric mean across all benchmarks).

| Benchmark | Baseline | Final | Improvement |
|-----------|----------|-------|-------------|
| X_Simplex_Layered20x5 | 288ms | 61ms | **79% faster** |
| X_Simplex_Layered10x5 | 40.5ms | 13.5ms | **67% faster** |
| Y_Layered20x5 | 40.8ms | 17.7ms | **57% faster** |
| AntiStack_Layered10x5 | 34.6ms | 15.0ms | **57% faster** |
| Y_Layered10x5 | 7.6ms | 3.6ms | **53% faster** |
| Y_Chain200 | 2.0ms | 1.65ms | **19% faster** |
| Y_Diamond25 | 4.2ms | 3.5ms | **17% faster** |

**Optimization breakdown:**
- Adjacency lists + subtree removal: ~23% improvement
- O(1) swap-delete + search limit: additional ~8% improvement
- Incremental cut values: additional ~20% improvement
- Cached sorted lists + circular search: additional ~10% improvement

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
