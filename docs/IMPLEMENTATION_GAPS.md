# Implementation Gaps

Issues identified by code review comparing posit against msagljs (MIT, Microsoft).

---

## Critical

### 1. Orthogonal Routing: No Node Avoidance or Edge Spacing

**File:** `route.go` — `routeOrthogonal()`

**Problem:** The current implementation assigns edges to vertical channels between node columns, but doesn't check whether nodes obstruct those channels. Edges can pass directly through intermediate nodes. Additionally, multiple edges sharing a channel are not spaced apart — they overlap.

**What the doc specifies:**
- "Route around nodes that fall within the channel"
- "Assign edges to channels with offset spacing to prevent overlaps"

**What msagljs does:** Full visibility graph construction with obstacle boundaries, shortest-path routing, and a "nudging" pass that spreads overlapping edge segments apart.

**Fix:** After channel assignment, check if any node bounding box intersects the channel corridor. If so, insert additional bends to route around the obstruction. For edge spacing, track which edges occupy each channel segment and offset them by `ChannelGap`.

**Scope:** `route.go` — rewrite `routeOrthogonal()` internals.

---

### 2. Rank Constraints: Post-Hoc Adjustment Breaks Edge Spans

**File:** `rank.go` — `applyRankConstraints()`

**Problem:** Rank constraints are applied after Network Simplex finishes. Moving a node to rank 0 (RankMin) or max rank (RankMax) can create edges that span many layers, but the normalization phase (which inserts dummy nodes for multi-layer edges) has already run by the time constraints are applied.

**Current order:**
1. Network Simplex assigns ranks
2. `applyRankConstraints()` moves nodes
3. `normalize()` inserts dummy nodes

This works only because `normalize()` runs after constraints. But the rank adjustment can violate the tight-tree invariant that Network Simplex established, producing suboptimal rank assignments for unconstrained nodes.

**What msagljs does:** Integrates constraints directly into the Network Simplex solver as additional edges with min/max length constraints.

**Fix:** Encode RankMin as a virtual edge from a synthetic source to the node with `minlen=0, weight=high`. Encode RankMax similarly. Encode RankGroup as zero-length edges between group members. Feed these into Network Simplex so the solver respects constraints natively.

**Scope:** `rank.go` — modify `networkSimplex()` or add constraint edges before calling it.

---

### 3. Component Discovery Includes Dummy Nodes

**File:** `components.go` — `findComponents()`

**Problem:** BFS traversal includes dummy nodes (inserted during normalization). If a long edge is split into dummy nodes that bridge two otherwise-disconnected subgraphs, `findComponents()` will treat them as one component. Conversely, if `findComponents()` is called before normalization, dummy nodes don't exist yet and edges spanning multiple ranks aren't traversable.

**Current call site:** `findComponents()` uses `s.nodes` and `s.successors`/`s.predecessors`, which include dummy nodes after normalization.

**Fix:** Either:
- (a) Run component detection before normalization, using only real nodes and original edges, OR
- (b) Filter dummy nodes from BFS and treat the original edge endpoints as connected regardless of intermediate dummies.

**Scope:** `components.go` — modify `findComponents()` to skip dummy nodes or run it earlier in the pipeline.

---

## High

### 4. Crossing Minimization: No Adjacent Exchange

**File:** `order.go` — `minimizeCrossings()`

**Problem:** The barycenter heuristic can get stuck in local minima. The standard fix (used by dagre and msagljs) is an "adjacent exchange" pass after each sweep: for each pair of adjacent nodes in a layer, swap them if doing so reduces crossings.

**What msagljs does:** After each barycenter sweep, iterates adjacent pairs and swaps when it improves the crossing count. This is O(n²) per layer per sweep but catches cases where barycenter ordering is suboptimal.

**Fix:** Add `adjacentExchange()` called after each `sweepDown()`/`sweepUp()` in the main loop. For each layer, try swapping each adjacent pair and keep the swap if `twoLayerCrossCount` improves.

**Scope:** `order.go` — add `adjacentExchange()` method, call from `minimizeCrossings()`.

---

### 5. Port Coordinates Not Direction-Transformed

**File:** `route.go` — `getPortPosition()`

**Problem:** Port offsets are computed assuming TopToBottom direction (Side=Right means x+width, Side=Left means x=0, etc.). For LeftToRight, BottomToTop, or RightToLeft layouts, the port coordinates are incorrect because node width/height have been swapped but port Side/Offset values haven't been remapped.

**Example:** A port with `Side=Right, Offset=20` on a node in LR layout should produce a point on the bottom of the (rotated) node, not the right side.

**Fix:** In `getPortPosition()`, transform the port's Side through the same rotation used in `adjustForDirection()` / `undoDirectionAdjustment()`. Or: compute port positions in internal TB space (after the direction swap of width/height), then let `undoDirectionAdjustment()` transform coordinates naturally.

**Scope:** `route.go` — modify `getPortPosition()` to account for direction.

---

### 6. Multi-Edge Offset Only Applies to Endpoints

**File:** `route.go` — `offsetParallelEdges()`

**Problem:** The function offsets only the first and last points of parallel edges. For edges with intermediate bend points (from dummy node routing), the offset is only visible at the start/end — the middle segments converge back to the same path.

**Fix:** Offset all points in the edge path perpendicular to the path direction at each segment. For straight edges this is trivial; for edges with bends, compute the perpendicular at each segment and offset accordingly.

**Scope:** `route.go` — rewrite `offsetParallelEdges()` to offset entire polylines.

---

### 7. Cluster Boundaries Not Respected by Edge Routing

**File:** `route.go`, `posit.go`

**Problem:** Cluster bounding boxes are computed in `adjustClusters()` after layout, but edge routing runs before cluster adjustment. Even if reordered, the edge router doesn't know about cluster boundaries and will route edges through cluster interiors.

**What msagljs does:** Clusters are modeled as obstacles in the routing graph. Edges entering/exiting a cluster must cross the cluster boundary at defined points.

**Fix:** After cluster bounds are computed, add cluster rectangles as obstacles to the edge routing phase. Edges not originating from within a cluster must route around its boundary.

**Scope:** `route.go` + `posit.go` — requires cluster computation before routing, and obstacle-awareness in the router.

---

## Medium

### 8. Brandes-Köpf: Only One Alignment

**File:** `position.go` — `assignXCoordinates()`

**Problem:** The Brandes-Köpf algorithm specifies four alignment passes (upper-left, upper-right, lower-left, lower-right) with the final X position being the median of all four. Posit implements only the upper-left alignment, which biases nodes toward the left.

**What msagljs does:** Implements all four alignments and takes the median X for each node.

**Fix:** Implement the remaining three alignments (upper-right, lower-left, lower-right) by reversing layer iteration order and/or reversing the scan direction within layers. Take the median of all four X values for each node.

**Scope:** `position.go` — add three more alignment passes, add median selection.

---

### 9. Barycenter vs Weighted Median

**File:** `order.go` — `calculateBarycenter()`

**Problem:** Barycenter (arithmetic mean of neighbor positions) is sensitive to outliers. If a node has 10 neighbors at position 0 and 1 neighbor at position 100, the barycenter is ~9, pulling the node far from the majority of its connections.

**What msagljs does:** Uses weighted median, which is more robust to outlier positions.

**Fix:** Offer weighted median as an alternative. The median of neighbor positions (weighted by edge weight) resists outlier pull. This could be a configuration option or simply replace barycenter.

**Scope:** `order.go` — add `calculateWeightedMedian()`, use it in `sortLayerByBarycenter()`.

---

### 10. Port-Level Crossing Minimization: O(E²) Edge Scan

**File:** `order.go` — `adjustForPortPositions()`

**Problem:** For each neighbor of each node, the function iterates over all edges in `s.edges` to find matching edges. This is O(E) per neighbor per node, making the whole function O(V × deg × E) which is effectively O(E²) for dense graphs.

**Fix:** Pre-build an edge index by endpoint pair. Use `s.edges[edgeKey{from, to}]` directly (which already exists as a map lookup) instead of iterating all edges. The current code iterates all edges because it needs to check both directions and find port IDs, but this can be restructured.

**Scope:** `order.go` — refactor `adjustForPortPositions()` to use map lookups instead of full iteration.

---

### 11. Incremental Layout: Not Truly Incremental

**File:** `posit.go` — `IncrementalLayout()`

**Problem:** The current implementation pins fixed nodes by overriding their X/Y after a full layout run. This means the entire graph is re-laid-out and then fixed nodes are snapped back, which can produce overlaps and edge routing that assumes positions that no longer hold.

**What the doc specifies:** "Keep same layer assignment, re-run Y coordinate assignment, keep X positions fixed for unchanged nodes, re-route affected edges only."

**What msagljs does:** Uses stress minimization with position constraints for fixed nodes, solving for minimal displacement.

**Fix (matching the doc):**
1. Preserve the original layer assignment (ranks) for all nodes
2. Apply dimension changes
3. Re-run only Y coordinate assignment within existing layers
4. Keep X positions for fixed nodes, run X assignment only for changed nodes
5. Re-route only edges connected to changed nodes

**Scope:** `posit.go` — rewrite `IncrementalLayout()` to selectively re-run phases.

---

### 12. Weight Truncation in Cross Counting

**File:** `order.go` — `twoLayerCrossCount()`

**Problem:** Edge weights are stored as `float64` but cast to `int` for the accumulator tree. Weights between 0 and 1 become 0, and fractional weights are lost.

**Fix:** Either use float64 in the accumulator tree, or scale weights to integers (multiply all by a factor that preserves relative magnitudes).

**Scope:** `order.go` — change `weight` type in `entry` struct and accumulator arithmetic, or add scaling.

---

## Low

### 13. No Edge Label Collision Avoidance

**Problem:** Edge labels are placed at dummy node positions but may overlap with other nodes or labels. No post-processing checks for or resolves label overlaps.

**Fix:** After label positioning, check for bounding box overlaps and nudge labels that collide.

**Scope:** `route.go` or new `labels.go`.

---

### 14. No Port Boundary Curves

**Problem:** Ports are modeled as dimensionless points. msagljs models ports with `ICurve` boundaries for precise edge clipping. This matters when port attachment areas are larger than a single pixel (e.g., a field row in a database node).

**Fix:** Add optional `Width`/`Height` to `PortOptions`. Use port rectangle intersection instead of point when computing edge endpoints.

**Scope:** `posit.go` (API), `route.go` (endpoint computation).

---

### 15. Compound Graph: Only Bounding Box Adjustment

**Problem:** The doc says compound graphs require "changes to every phase of the algorithm — cycle removal, ranking, ordering, and coordinate assignment all need cluster awareness." The current implementation only adjusts bounding boxes after layout, meaning:
- Child nodes aren't constrained to same/adjacent ranks
- Cluster nodes don't participate in ordering as atomic units
- Coordinate assignment doesn't reserve space for cluster padding

**Fix:** This is a large architectural change. For a minimal improvement:
- During ranking, constrain all children of a cluster to consecutive ranks
- During ordering, treat cluster contents as a contiguous block
- During coordinate assignment, add cluster padding to node spacing

**Scope:** All layout phases. Defer until critical/high items are resolved.

---

## Summary

| # | Issue | Severity | File(s) | Effort |
|---|-------|----------|---------|--------|
| 1 | Orthogonal routing: no node avoidance/spacing | Critical | route.go | High |
| 2 | Rank constraints: post-hoc breaks invariants | Critical | rank.go | Medium |
| 3 | Component discovery includes dummies | Critical | components.go | Low |
| 4 | No adjacent exchange in crossing min | High | order.go | Low |
| 5 | Port coords not direction-transformed | High | route.go | Low |
| 6 | Multi-edge offset only at endpoints | High | route.go | Medium |
| 7 | Clusters don't affect routing | High | route.go, posit.go | High |
| 8 | Brandes-Köpf: one alignment only | Medium | position.go | Medium |
| 9 | Barycenter vs weighted median | Medium | order.go | Low |
| 10 | Port crossing min: O(E²) scan | Medium | order.go | Low |
| 11 | Incremental layout: full re-layout | Medium | posit.go | High |
| 12 | Weight truncation in cross count | Medium | order.go | Low |
| 13 | No label collision avoidance | Low | route.go | Medium |
| 14 | No port boundary curves | Low | posit.go, route.go | Low |
| 15 | Compound graphs: bbox only | Low | all phases | Very High |

### Recommended Order

**First pass (low-hanging fruit):** 3, 4, 5, 10, 12 — all low effort, fix correctness bugs and performance issues.

**Second pass (correctness):** 2, 6, 8 — medium effort, fix algorithmic correctness.

**Third pass (features):** 1, 9, 11 — higher effort, bring orthogonal routing and incremental layout to spec.

**Deferred:** 7, 13, 14, 15 — depend on architectural decisions about clusters and labels.
