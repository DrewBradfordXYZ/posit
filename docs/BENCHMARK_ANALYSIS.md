# Benchmark Analysis: Posit vs ELK vs Dagre vs MSAGL

## Test Profiles

All libraries run the same graph structures (exported from Go via `profiles.json`):

| Profile | Nodes | Edges | Structure |
|---------|------:|------:|-----------|
| Large   | 500   | 996   | Random sparse (seed 42) |
| Dense   | 100   | 2033  | Random 20% connectivity |
| Wide    | 500   | 1200  | 100 nodes × 5 layers, 3 edges each |
| Deep    | 200   | 199   | Linear chain |
| Medium  | 100   | 198   | Random sparse |

## Results

### Timing (ms, lower = faster)

| Profile | Posit | Dagre | MSAGL | ELK |
|---------|------:|------:|------:|----:|
| Large   | 2,258 | 8,763 | 62,994 | **1,280** |
| Dense   | **2,363** | 24,051 | 50,195 | 11,690 |
| Wide    | **63** | 39,156 | 509 | 950 |
| Deep    | **0.8** | 26 | 4.7 | 33 |
| Medium  | **81** | 118 | 305 | 137 |

### Crossing Count (lower = better layout quality)

| Profile | Posit | Dagre | ELK |
|---------|------:|------:|----:|
| Large   | 40,284 | **26,359** | 28,603 |
| Dense   | **247,928** | 325,309 | 302,272 |
| Wide    | **39,346** | 43,425 | 50,300 |
| Deep    | 0 | 0 | 0 |
| Medium  | 1,058 | **854** | 986 |

### Layout Dimensions

| Profile | Posit (WxH) | Layers |
|---------|-------------|-------:|
| Large   | 15,625 × 22,390 | 173 |
| Dense   | 19,775 × 12,510 | 97 |
| Wide    | 10,700 × 550 | 5 |
| Deep    | 50 × 25,900 | 200 |
| Medium  | 2,775 × 4,320 | 34 |

## Analysis

### Where Posit Wins

**Wide (15x faster than ELK, 620x faster than Dagre):** Shallow graphs with many nodes per layer are Posit's sweet spot. With only 5 layers, each barycenter sweep is cheap and converges fast. Posit also achieves the best crossing count (39,346 vs ELK's 50,300).

**Dense (5x faster than ELK, 10x faster than Dagre):** High-connectivity graphs with many edges benefit from Posit's efficient Fenwick-tree cross counting. Posit achieves significantly fewer crossings (248K vs 325K for Dagre, 302K for ELK).

**Deep (40x faster than ELK):** Trivial structure with zero crossings. Posit's low overhead dominates — no WASM startup cost, no framework abstraction layers.

**Medium (1.7x faster than ELK):** Moderate graphs where Posit's tight Go implementation outperforms ELK's WASM overhead.

### Where ELK Wins

**Large (1.8x faster, 30% fewer crossings):** Random sparse graphs that produce deep layer structures (173 layers from 500 nodes). ELK handles this topology more efficiently.

## Root Cause: The Large Profile Problem

### Why 173 Layers?

The Large profile is a random graph (500 nodes, 996 edges, average degree ~2). After cycle removal, the resulting DAG has a critical path length of 173. Both LongestPath and NetworkSimplex ranking produce the same depth — the structure genuinely requires it.

With ~2.9 nodes per layer on average, this is an unusually deep, narrow layout.

### Dummy Node Explosion

Edges spanning multiple layers are split into chains of dummy nodes (one per intermediate layer). With 996 edges spanning an average of ~10 layers:

```
~10,000 dummy nodes added to 500 real nodes
= ~10,500 total nodes in crossing minimization
```

### Cost Cascade

Each crossing minimization iteration:
1. **Barycenter sweep**: O(173 layers × nodes_per_layer × edges)
2. **Adjacent exchange**: O(173 layers × layer_size²) per layer
3. **Cross counting**: O(173 layers × E × log(V)) using Fenwick tree

With up to 24 iterations (early exit after 4 no-improvement), this adds up to ~2.3s.

### Why ELK Handles This Better

ELK uses the same Sugiyama pipeline but with decades of implementation optimization:

- More sophisticated crossing minimization (likely sifting or global sifting heuristics)
- Better convergence detection (fewer wasted iterations)
- Optimized data structures for sparse layers with many dummies
- Tighter inner loops from years of profiling

The 1.8x speed difference and 30% fewer crossings suggest constant-factor improvements, not a fundamentally different algorithm.

## Strategic Context

Posit's architectural advantage — constraint vocabulary at the information boundary, server-first single-pass computation — applies to any directed graph, not just schemas. The architecture will attract use cases beyond schema diagrams:

- **Dependency graphs** (package managers, build systems): wide and moderately deep
- **State machines** (protocol diagrams, workflow engines): moderate size, many multi-edges
- **Call graphs** (profiling, debugging): large, deep, random-like connectivity
- **CI/CD pipelines** (deployment flows): wide with parallel branches
- **Organizational charts** (reporting structures): deep trees with cross-links

For the architectural pattern to prove itself across these domains, the layout engine must perform well on diverse graph structures — including large random graphs. The Large profile gap (1.8x slower than ELK, 40% more crossings) represents a real weakness that limits adoption.

**Current standing:**
- Shallow/wide graphs: Posit dominates (5-620x faster than competitors)
- Deep random graphs: ELK is faster with better quality
- Goal: competitive or better on all profiles

## Improvement Priorities

These improvements target the Large profile gap while maintaining Posit's advantages on other structures. Ordered by expected impact:

### 1. Greedy FAS as Default Cycle Removal (high impact, already implemented)

The DFS-based cycle removal creates long directed paths in random graphs, producing deep DAGs. Greedy FAS (Eades/Lin/Smyth) minimizes the number of reversed edges, which can produce a shallower DAG with fewer layers and fewer dummy nodes.

This option already exists in the API:

```go
g.Layout(Options{CycleRemoval: GreedyFAS})
```

Needs benchmarking to verify it reduces layer count on the Large profile. If effective, consider making it the default.

### 2. Post-Ranking Layer Compaction (high impact, new feature)

After ranking, nodes on sparse layers may be movable to adjacent layers without violating edge constraints. Compaction reduces:
- Total layer count (fewer sweeps per iteration)
- Dummy node count (shorter edge spans)
- Total crossings (fewer segment pairs)

Algorithm sketch:
```
for each layer with few nodes:
    for each node in layer:
        if all predecessors are in layer-1 or earlier:
            try moving node to layer-1
            accept if it doesn't increase layer-1 width beyond threshold
```

### 3. Adaptive Crossing Minimization (medium impact, low effort)

Deep graphs with sparse layers get diminishing returns from the fixed 24-iteration limit. Adapt based on graph structure:

```go
func (s *layoutState) adaptiveMaxIterations() int {
    avgNodesPerLayer := float64(len(s.nodes)) / float64(len(s.layers))
    if len(s.layers) > 50 && avgNodesPerLayer < 5 {
        return 12 // fewer iterations, faster convergence check
    }
    return 24
}
```

### 4. Skip Adjacent Exchange on Dummy-Heavy Layers (low-medium impact, low effort)

Interior dummy nodes have order fully determined by their chain neighbors. Adjacent exchange on layers where >90% of nodes are dummies provides minimal benefit:

```go
if dummyRatio > 0.9 && activeCount < 3 {
    return false // skip this layer
}
```

The `activeCount < 2` check already exists; extending it to account for dummy ratio would skip more layers in deep graphs.

### 5. Sifting-Based Crossing Minimization (high impact, significant effort)

The barycenter heuristic places nodes at the average position of their neighbors. Sifting (Matuszewski et al.) instead tries each node in every position within its layer and picks the best. Global sifting extends this across layers. This is what ELK uses for its crossing quality advantage.

Significantly more complex to implement but would close the crossing count gap on all profiles.

## Benchmark Methodology

### Quality Metrics (Deterministic)

Crossings, area, dimensions, and layer count are deterministic — same input always produces the same output. These are compared via baseline regression in `TestBenchmarkReport`.

### Timing (Statistical)

Timing is measured via Go's `testing.B` benchmarks:

```bash
go test -bench=BenchmarkLayout -count=10 > old.txt
# ... make changes ...
go test -bench=BenchmarkLayout -count=10 > new.txt
benchstat old.txt new.txt
```

The `~Time` column in the quality report is informational only — not used for regression detection.

### Cross-Library Comparison

The JS benchmark (`_bench/bench.mjs`) uses graph structures exported from Go (`profiles.json`) to ensure identical graphs across all libraries. Each library runs 2 warm-up + 5 timed iterations, reporting the median.

## Summary

### Current Position

| Scenario | Winner | Margin |
|----------|--------|--------|
| Shallow graphs (5-15 layers) | **Posit** | 5-620x faster |
| Deep random graphs (100+ layers) | **ELK** | 1.8x faster |
| Crossing quality (dense/wide) | **Posit** | 20-30% fewer |
| Crossing quality (sparse random) | **ELK/Dagre** | 30% fewer |

### Target Position (after improvements)

| Scenario | Goal |
|----------|------|
| Shallow graphs | Maintain dominance |
| Deep random graphs | Parity or better than ELK |
| Crossing quality (all profiles) | Best or within 10% |
| API expressiveness | Unique (constraint vocabulary advantage) |

The architectural advantage (single-pass, server-first, complete constraint vocabulary) is the differentiator. Performance parity on general graphs ensures the architecture gets the chance to prove itself across diverse use cases.
