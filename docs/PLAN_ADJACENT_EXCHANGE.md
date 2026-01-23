# Adjacent Exchange: Implementation Status & Improvement Opportunities

## Current State (post-implementation)

The msagljs-style ILS (Iterated Local Search) adjacent exchange is implemented in `order.go`.

### What's Implemented:

- **Sorted merge crossing delta** — `countInversions()` uses O(deg) linear merge scan
- **Neighbor cache** — `buildNeighborCache()` precomputes sorted positions once per layer, avoids per-swap allocations
- **First-improvement greedy with propagation** — `greedyExchangeWithCache()` finds first beneficial swap, propagates left/right (up to 5 positions), restarts scan
- **Stochastic disturbance** — `disturbLayerWithCache()` accepts gain=0 with 50% probability (deterministic RNG, seed=42)
- **ILS per-layer** — `adjExchangeLayer()` runs 3 passes, stops after 2 no-improvement
- **Bidirectional layer sweep** — `adjacentExchange()` sweeps forward then backward
- **Best-solution tracking** — per-layer best order saved and restored
- **Reverse ordering** — `TryReverseOrdering` option tries both layer directions, keeps better
- **layerCrossings helper** — counts crossings for a rank against both adjacent layers

### Key Design Decisions (vs msagljs):

| Decision | msagljs | Posit | Rationale |
|----------|---------|-------|-----------|
| ILS passes | 50 | 3 | Posit's AE is called 24× from outer loop |
| No-improvement bound | progress-based | 2 | Compensated by outer loop |
| Greedy strategy | First-improvement + propagation | First-improvement + propagation (capped at 5) | Same approach, bounded for dense graphs |
| Layer sweep | All layers fwd + bwd, up to 50 cycles | Forward + backward, 1 cycle | Outer loop provides additional opportunities |
| Round cap | Unbounded (first-improvement limits work) | 5 per greedy call | Prevents pathological behavior |
| Best tracking | Global (all layers together) | Per-layer | More fine-grained (see Design Notes) |
| RNG | Math.random() (non-deterministic) | Seeded rand (deterministic) | Reproducible layouts |
| Cache | Rebuilt per pass + maintained during swaps | Built once per layer, reused | Valid because neighbors are in adjacent layers |
| Reverse attempt | Always (tryReverse=true) | Optional (TryReverseOrdering) | User controls perf/quality trade-off |

---

## Implemented Improvements

All three improvements from the msagljs code review have been implemented.

### 1. First-Improvement with Propagation ✓

After each beneficial swap at position i, propagates left (up to 5 positions) and right (up to 5 positions) checking for compound improvements. Uses first-improvement strategy: restarts the scan after each swap+propagation round.

**Parameters:** `maxRounds=5`, `maxPropagation=5`

### 2. Bidirectional Layer Sweep ✓

`adjacentExchange` now sweeps all layers forward then backward, allowing improvements in one layer to create opportunities in adjacent layers within the same call.

**Parameters:** `maxCycles=1` (1 forward + 1 backward per call; outer loop provides additional opportunities)

### 3. Reverse Layout Attempt ✓

`TryReverseOrdering` option (default: false) — reverses layer order, re-runs the full ordering pass, and keeps the better result. For asymmetric graphs, one direction may produce fewer crossings.

---

## Performance Characteristics (Measured)

| Test | Time | Limit |
|------|------|-------|
| Large graph (500 nodes, 1000 edges) | 1.4–2.7s | 5s |
| Dense graph (100 nodes, 2033 edges) | 2.8–4.5s | 5s |
| Wide graph (100 nodes/layer × 5 layers) | 50–70ms | 5s |
| All algorithms (6 combinations) | 0.2–0.4s | — |

### Performance Analysis:

- **Neighbor cache** eliminates per-swap allocation overhead (was the dominant cost without caching)
- **First-improvement + propagation** finds compound improvements per round, reducing total rounds needed
- **Propagation cap (5)** prevents pathological scan depth on dense graphs
- **Round cap (5)** with first-improvement strategy is sufficient since each round does more work
- **Bidirectional sweep (1 cycle)** gives each layer 2 visits per call without excessive overhead
- **ILS passes (3)** per layer per visit, balanced for the 24× outer invocation context

---

## Files Modified

- `order.go` — `minimizeCrossings` (refactored into `runOrderingPass`), `adjacentExchange` (bidirectional sweep), `adjExchangeLayer` (per-layer ILS), `greedyExchangeWithCache` (first-improvement + propagation), `disturbLayerWithCache`, `buildNeighborCache`, `swapCrossingDeltaCached`, `countInversions`, `sortedNeighborPositions`, `layerCrossings`, `reverseLayerOrder`, `neighborPos`, `neighborCache` types
- `posit.go` — added `TryReverseOrdering bool` option
- `state.go` — added `rng *rand.Rand` to `layoutState`, initialized with seed 42

---

## Design Notes

### Why Per-Layer Best Tracking (Not Global)

msagljs tracks the best ordering across all layers together — if a perturbation helps one layer but hurts another, the whole thing gets rolled back. Posit tracks best per-layer independently. Per-layer tracking is more fine-grained and preserves local improvements that a global measure would reject. This is better for the nested-call context where each `adjacentExchange` call is one step in a larger optimization.

### Parameter Tuning Rationale

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| maxCycles | 1 | 24× outer calls already provide repeated opportunities |
| maxPasses (ILS) | 3 | Each visit does greedy+propagation which converges faster |
| maxNoImprovement | 2 | Quick exit when stuck; next outer iteration will try again |
| maxRounds (greedy) | 5 | First-improvement + propagation needs fewer rounds |
| maxPropagation | 5 | Bounds worst-case for high-degree nodes |
| RNG seed | 42 | Deterministic, reproducible layouts |
