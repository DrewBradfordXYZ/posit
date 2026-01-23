# Performance: Go Optimization Status

## Current State

The codebase uses Go concurrency for independent parallel phases (BK alignment, edge routing) and field-level buffer reuse for hot-path allocations. Memory allocation discipline is good (~90% of hot-path slices pre-allocated with capacity).

### Implemented Optimizations

| Technique | Where | Impact |
|-----------|-------|--------|
| Map capacity hints | `state.go`, `order.go` | Eliminates rehashing |
| Neighbor cache | `order.go:buildNeighborCache()` | Eliminates per-swap allocations |
| Accumulator tree | `order.go:twoLayerCrossCount()` | O(E log V) crossing count |
| Tree buffer reuse | `order.go:twoLayerCrossCount()` via `state.go:treeBuf` | Eliminates per-call tree allocation |
| Adaptive BK threshold | `position.go` | Skips O(V²) BK for large graphs |
| Parallel BK alignments | `position.go:assignXCoordinatesBK()` | 4 goroutines for independent passes |
| Parallel edge path building | `route.go:buildChainPathsParallel()` | Per-chain goroutines (≥50 chains) |
| Direct order field access | `order.go:twoLayerCrossCount()` | Eliminates per-call southPos map |
| Edges buffer reuse | `order.go:twoLayerCrossCount()` | Eliminates per-node slice allocation |
| Small-sort specialization | `order.go` (3 call sites) | Avoids sort.Slice overhead for ≤3 elements |
| First-improvement search | `order.go:greedyExchangeWithCache()` | Reduces scan iterations |
| Deterministic RNG | `state.go` | Reproducible without syscalls |
| Slice pre-allocation | `order.go`, `route.go` | Eliminates grow-from-zero appends |

---

## Implementation Details

### Parallel Brandes-Köpf Alignments ✓

**Files:** `position.go`

The four BK alignment passes (ul, ur, dl, dr) run concurrently via `sync.WaitGroup`. Each pass calls `verticalAlignment` and `horizontalCompaction` which read shared state (nodes, predecessors, successors) but produce independent `map[string]float64` outputs. Results are collected into a fixed-size array indexed by pass, ensuring deterministic merge order.

**Gate:** BK is only used for graphs under the BK threshold (default 100 nodes including dummies).

### Parallel Edge Path Building ✓

**Files:** `route.go`

Dummy chain path building is split into `buildChainPath` (per-chain, reads from shared immutable state) and a sequential apply step (mutates the edge map). For graphs with ≥50 chains, path building runs in parallel goroutines. Results are applied deterministically by chain index.

**Gate:** Only parallelized for ≥50 dummy chains. Below this threshold, sequential execution avoids goroutine spawn overhead.

### Tree Buffer Reuse ✓

**Files:** `state.go`, `order.go`

The accumulator tree slice in `twoLayerCrossCount()` is stored as `treeBuf []float64` on `layoutState`. This avoids allocation on every call (hundreds of times per layout) without the overhead of `sync.Pool` (which adds per-P pinning, type assertions, and atomic operations that exceeded the allocation savings for these sizes).

**Note:** `sync.Pool` was tested and rejected — its internal locking overhead made the hot-path ~5% slower than simple `make()`. Field-level reuse has zero overhead since `layoutState` is single-owner during the ordering phase.

### Direct Order Field Access ✓

**Files:** `order.go`

`twoLayerCrossCount()` previously rebuilt a `southPos map[string]int` on every call (3,500–7,000 times per layout), mapping each south-layer node to its index. Since `node.order` is kept current by all swap/sort operations, the map is replaced with a direct rank check and `wNode.order` access. This eliminates ~1.4M map insertions and lookups per layout.

### Edges Buffer Reuse ✓

**Files:** `order.go`

In `twoLayerCrossCount()`, the per-north-node `edges` slice was re-allocated via `var edges []entry` on each iteration. A single `edges` buffer is now allocated once before the loop and reset with `edges[:0]` each iteration.

### Small-Sort Specialization ✓

**Files:** `order.go`

Three sort call sites use manual comparison swaps for slices of 2–3 elements instead of `sort.Slice()`. This avoids the function-call overhead, interface boxing, and reflection that `sort.Slice` introduces for the common case (most nodes have 2–5 neighbors).

| Call site | Context |
|-----------|---------|
| `twoLayerCrossCount` | Per-node edges sorted by south position |
| `sortedNeighborPositions` | Neighbor positions for swap delta cache |
| `calculateBarycenter` | Weighted positions for median computation |

### Slice Pre-allocation ✓

**Files:** `order.go`, `route.go`

| Location | Before | After |
|----------|--------|-------|
| `order.go:twoLayerCrossCount` | `var southEntries []entry` | `make([]entry, 0, len(northLayer)*2)` |
| `order.go:calculateBarycenter` | `var positions []weightedPos` | `make([]weightedPos, 0, len(neighbors))` |
| `route.go:findClearVerticalChannel` | `var boundaries []float64` | `make([]float64, 0, 2*len(obstacles)+1)` |

---

## Remaining Opportunities

### Component-Parallel Layout

**Impact:** 10-50% speedup (depends on graph structure)
**Complexity:** Medium
**Files:** `posit.go`

Disconnected components are laid out sequentially in `packComponents()`. Each component's full layout pipeline is independent. However, most real-world graphs are connected (1 component), making this situational.

---

## Benchmarking

Use `task bench` to measure before/after. Save a baseline first:

```bash
task bench:save   # save current metrics
# ... make changes ...
task bench        # compare against baseline
```

The benchmark reports timing, edge crossings, layout area, and layer count for 5 graph profiles (large, dense, wide, deep, medium).

---

## Design Constraints

- **Determinism required:** All parallel paths produce identical results regardless of goroutine scheduling. Fixed-index result arrays ensure deterministic merge order.
- **Size gating:** Goroutines are only spawned when the work exceeds overhead thresholds (≥50 chains for routing, BK threshold for alignment).
- **No external dependencies:** Posit is zero-dependency. Uses only `sync` from stdlib.
- **Backward compatible:** All optimizations are internal. Public API unchanged.
