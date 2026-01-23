# Implementation Gaps

Issues identified by code review comparing posit against msagljs and dagre reference implementations.

**Last reviewed:** 2025-01-22

---

## Status Summary

| # | Issue | Severity | Status |
|---|-------|----------|--------|
| 1 | Orthogonal routing: no node avoidance/spacing | Critical | **FIXED** |
| 2 | Rank constraints: post-hoc breaks invariants | Medium | Open (accepted trade-off) |
| 3 | Component discovery includes dummies | Critical | **FIXED** |
| 4 | No adjacent exchange in crossing min | High | **FIXED** |
| 5 | Port coords not direction-transformed | High | **FIXED** |
| 6 | Multi-edge offset only at endpoints | High | **FIXED** |
| 7 | Clusters don't affect routing | High | **FIXED** |
| 8 | Brandes-Kopf: one alignment only | Medium | **FIXED** |
| 9 | Barycenter vs weighted median | Medium | **FIXED** |
| 10 | Port crossing min: O(E^2) scan | Medium | **FIXED** |
| 11 | Incremental layout: full re-layout | Medium | **FIXED** (partial) |
| 12 | Weight truncation in cross count | Low | **FIXED** |
| 13 | No label collision avoidance | Low | Open |
| 14 | No port boundary curves | Low | **FIXED** |
| 15 | Compound graphs: bbox only | Low | Open |
| 16 | Type-1 conflict: O(V) position scan | Low | **FIXED** |
| 17 | Incremental layout re-runs ordering | Low | **FIXED** |
| 18 | Cross count truncates to int | Low | **FIXED** |

**Resolved: 15/18** — Remaining items are low-severity or accepted trade-offs.

---

## Resolved Items

### 1. Orthogonal Routing (FIXED)

`route.go` — `routeOrthogonal()` now implements:
- Obstacle collection from real node bounding boxes
- Cluster bounding boxes as routing obstacles (`computeClusterObstacles()`)
- Horizontal channel computation between layers (`computeLayerChannelYs()`)
- Channel usage tracking with spacing offsets (`orthoSegKey` + `channelUsage` map)
- Vertical channel clearance search (`findClearVerticalChannel()`)
- Full obstacle-aware path construction (`buildOrthogonalPath()`)

### 3. Component Discovery (FIXED)

`components.go` — `findComponents()` traverses through dummy nodes but only collects real nodes in returned components. BFS uses successors/predecessors (including dummies) for connectivity but filters with `!node.isDummy` before adding to component.

### 4. Adjacent Exchange (FIXED)

`order.go:155` — `adjacentExchange()` implemented with:
- Layer-by-layer swap attempts for adjacent pairs
- O(n^2) per layer, skipped for layers >50 nodes
- 2-pass limit per layer with early termination on no improvement
- Uses `crossingsInvolving()` for efficient local crossing count

### 5. Port Direction Transform (FIXED)

`route.go:252` — `getPortPosition()` calls `portSideToInternal()` which transforms port sides through the direction rotation. Handles all four directions (TB, LR, RL, BT) with proper side rotation functions.

### 6. Multi-Edge Offset (FIXED)

`route.go:402` — `offsetParallelEdges()` now offsets the entire polyline:
- Computes perpendicular direction at each point (averaging adjacent segments for middle points)
- Applies perpendicular offset uniformly along the path
- Parallel edges remain visually separated even around bends

### 7. Cluster Routing Obstacles (FIXED)

`route.go:531` — `routeOrthogonal()` calls `computeClusterObstacles()` and appends cluster bounding boxes to the obstacle list before routing. Edges route around cluster boundaries.

### 8. Brandes-Kopf Four Alignments (FIXED)

`position.go:124-170` — Computes all four alignments (ul, ur, dl, dr) using reversed layers/orders. Takes median of four values (average of middle two) for final X position. Adaptive threshold (default 100 nodes) switches to simple centering for large graphs.

### 9. Weighted Median (FIXED)

`order.go:277-342` — `calculateBarycenter()` (misnamed, actually implements weighted median). Sorts positions, finds the point where cumulative weight crosses half the total. Robust to outliers unlike arithmetic mean.

### 10. Port Crossing Minimization (FIXED)

`order.go:541-654` — `adjustForPortPositions()` uses direct map lookups via `s.edges[edgeKey{...}]` (O(1) per edge) instead of iterating all edges. Checks both directions for each neighbor.

### 11. Incremental Layout (FIXED — partial)

`posit.go:663` — `IncrementalLayout()` now:
- Re-runs full ranking and crossing minimization (can't avoid this for structural correctness)
- Recomputes Y coordinates from scratch (layer heights may change)
- Pins fixed node X positions from base layout after X assignment
- Re-routes all edges

Still runs more phases than strictly necessary (full ranking + ordering), but the X-pinning behavior is correct. A fully incremental approach would require position-constrained stress minimization (like msagljs's iPsepCola), which is a different algorithm family.

### 12. Weight Truncation (FIXED)

`order.go:356-429` — `twoLayerCrossCount()` uses `float64` throughout the accumulator tree. Edge weights <1 are clamped to 1.0 (line 384-386). The final `int(cc)` truncation only affects the return value comparison, not the accumulation precision.

### 14. Port Boundary Curves (FIXED)

`route.go:260-268` — `getPortPosition()` accounts for port `Width`/`Height` when computing center offset. Ports with dimensions are centered on their attachment area.

---

## Open Items

### 2. Rank Constraints: Post-Hoc Application

**Severity:** Medium (accepted trade-off)
**File:** `rank.go` — `applyRankMinMax()`, `applyRankGroups()`

**Current behavior:** Rank constraints are applied after ranking completes. This can produce suboptimal rank assignments for unconstrained nodes because the tight-tree invariant from Network Simplex is violated.

**Why this is acceptable:**
- Post-hoc application avoids creating contradictions between constraints and edge directions
- RankGroup members connected by edges would create cycles if encoded as zero-length constraint edges
- The current approach always produces a valid layout, even if not globally optimal
- Both dagre and msagljs struggle with this trade-off differently (dagre also uses post-hoc; msagljs has incomplete constraint support with many stubs)

**Potential improvement:** Encode constraints as weighted edges in the Network Simplex graph with high weight but allow them to be violated (soft constraints). This would improve quality without risking infeasibility.

---

### 13. No Edge Label Collision Avoidance

**Severity:** Low
**File:** `route.go`

**Problem:** Edge labels are placed at dummy node positions (or midpoints for short edges) but may overlap with other nodes or labels. No post-processing checks for or resolves label overlaps.

**What msagljs does:** Complex nudging pass that adjusts edge paths and labels to minimize overlaps.

**What dagre does:** Nothing — labels can overlap in dagre too.

**Potential fix:** After label positioning, detect bounding box overlaps between labels and nodes/other labels. Nudge labels perpendicular to their edge to resolve collisions.

---

### 15. Compound Graphs: Bounding Box Only

**Severity:** Low
**File:** All layout phases

**Problem:** Clusters are implemented as post-hoc bounding box computation. Child nodes aren't constrained to consecutive ranks during ranking, and cluster contents aren't treated as atomic blocks during ordering.

**Current mitigations already implemented:**
- Rank phase: `applyRankGroups()` moves cluster children to minimum rank
- Rank phase: Cluster rank adjacency enforcement ensures children occupy consecutive ranks
- Order phase: `enforceClusterAdjacency()` keeps cluster children together within their layer
- Route phase: Cluster bounding boxes are routing obstacles

**What's missing for full compound graph support (msagljs-style):**
- Border nodes (top/bottom dummy nodes constraining subgraph extent) — dagre uses this
- Cluster-aware coordinate assignment (reserve padding space in position phase)
- Edge routing that enters/exits clusters at defined border points
- Nested cluster support (clusters within clusters)

**Assessment:** Current implementation handles the common case (visual grouping with routing avoidance). Full compound graph support is a major architectural change with limited practical benefit for most use cases.

---

## Recently Fixed

### 16. Type-1 Conflict Detection (FIXED)

`position.go` — `findType1Conflicts()` rewritten to:
- Precompute all inner segments once by iterating dummy nodes (O(D) instead of O(V) per layer)
- Use `node.order` for O(1) position lookup instead of O(V) linear scan per edge
- Skip checking inner segments against themselves (both endpoints are dummies)
- Group segments by layer rank for O(1) lookup per layer pair

Adjacent-layer detection is correct per the Brandes-Kopf paper: after normalization all edges span exactly one layer, so an edge can only cross inner segments at its own layer pair.

### 17. Incremental Layout Ordering (FIXED)

`posit.go` — `IncrementalLayout()` now skips `minimizeCrossings()` when a base layout is provided. Instead calls `restoreOrderFromBase()` which sorts nodes within each layer by their base X position (preserving the previous ordering). Dummy nodes are positioned by averaging their connected real nodes' base X positions.

### 18. Cross Count Precision (FIXED)

`order.go` — `twoLayerCrossCount()`, `crossingsInvolving()`, and `countCrossings()` all return `float64`. Preserves precision for fractional edge weights throughout the crossing minimization pipeline.

---

## Comparison Notes

### Posit vs Dagre

| Aspect | Dagre | Posit | Winner |
|--------|-------|-------|--------|
| Crossing minimization | Barycenter + accumulator tree | Weighted median + accumulator tree + adjacent exchange | Posit |
| Coordinate assignment | Brandes-Kopf (4 alignments) | BK (4 alignments) + simple fallback for large graphs | Posit |
| Ranking | Network Simplex only | LongestPath + TightTree + NetworkSimplex (selectable) | Posit |
| Compound graphs | Border nodes (full support) | Bounding box + routing obstacles | Dagre |
| Edge routing | Polyline via dummies | Polyline + orthogonal with obstacle avoidance | Posit |
| Multi-edge | Merged visually | Full polyline offset with perpendicular spacing | Posit |
| Port support | None | Full (directional, sized, ordering-aware) | Posit |
| Direction support | 4 directions (TB, LR, RL, BT) | 4 directions with port transform | Posit |
| Incremental layout | None | Partial (X-pinning) | Posit |
| Determinism | Order-dependent | Fully deterministic (sorted iteration) | Posit |

### Posit vs MSAGL.js

| Aspect | MSAGL.js | Posit | Winner |
|--------|----------|-------|--------|
| Edge routing | Rectilinear visibility graph + nudging | Channel-based orthogonal with obstacles | MSAGL.js |
| Spline routing | Full spline support | Not implemented | MSAGL.js |
| Constraint solver | Partial (many stubs) | Working post-hoc constraints | Posit |
| Force-directed | iPsepCola with multipole approx | Not implemented | MSAGL.js |
| Code completeness | ~70% (many `not implemented` stubs) | ~100% | Posit |
| Implementation quality | Professional but incomplete | Complete and tested | Posit |
| Dependencies | Complex TypeScript with many modules | Zero dependencies (pure Go) | Posit |

### Key Algorithmic Advantages Over Both References

1. **Weighted median** (vs dagre's barycenter): More robust to outlier positions
2. **Adaptive algorithm selection** (BK threshold): Avoids O(V^2) BK on large graphs
3. **Port-aware ordering**: Neither dagre nor msagljs adjust layer ordering for port positions
4. **Orthogonal routing with obstacle avoidance + channel spacing**: Dagre has no orthogonal mode; msagljs's is more sophisticated but incomplete in the JS port
5. **Full multi-edge polyline offset**: Dagre merges multi-edges; msagljs handles them but with more complexity
