# Network Simplex for X Coordinate Assignment

## Goal

Implement Network Simplex for X coordinate assignment following Gansner et al. (1993) Section 4. This replaces the Brandes-Köpf heuristic with a globally optimal algorithm that can express cross-layer constraints (enabling future anti-stacking).

## Why Network Simplex?

| Aspect | Brandes-Köpf (Current) | Network Simplex (New) |
|--------|------------------------|----------------------|
| Optimization scope | Per-layer heuristic | Global optimal |
| Cross-layer constraints | Not possible | Native support |
| Anti-stacking | Cannot express | Add as constraint edges |
| Complexity | O(n) | O(n·m) but fast in practice |

## Algorithm Overview

### Optimization Problem (from Paper Section 4)

```
min Σ Ω(e)ω(e)|x_w - x_v|
subject to: x_b - x_a ≥ ρ(a,b) for adjacent same-rank nodes
```

### Auxiliary Graph Transformation

The absolute value cannot be directly optimized. Transform to standard network simplex:

**For each edge e=(u,v):**
1. Create proxy node `n_e`
2. Add edges `(n_e → u)` and `(n_e → v)` with δ=0, ω=Ω(e)·ω(e)

**For adjacent same-rank nodes (v,w):**
1. Add edge `(v → w)` with δ=ρ(v,w), ω=0

**Key insight:** Proxy node `n_e` gets position `min(x_u, x_v)`, so one edge has length 0, other has `|x_u - x_v|`.

### Ω Weights (Internal Edge Weights)

| Edge Type | Ω Weight |
|-----------|----------|
| real → real | 1 |
| real → dummy | 2 |
| dummy → dummy | 8 |

## Files

| File | Purpose |
|------|---------|
| `simplex.go` | Network simplex for Y ranking and X coordinates (unified) |
| `xsimplex_test.go` | Unit tests for X simplex |
| `posit.go` | `XCoordAlgorithm` option |
| `position.go` | Dispatch in `assignCoordinates()` |

## Implementation

### 1. Add Option to `posit.go`

```go
type XCoordAlgorithm int

const (
    XBrandesKopf XCoordAlgorithm = iota  // Default
    XNetworkSimplex
)

// Add to Options struct:
XCoordAlgorithm XCoordAlgorithm
```

### 2. X Simplex Types (in `simplex.go`)

**Key types:**

```go
type xAuxNode struct {
    id       string
    x        float64      // Current X coordinate
    parent   string       // Spanning tree parent
    low, lim int          // Postorder values for O(1) descendant check
    origNode *layoutNode  // nil for proxy nodes
}

type xAuxEdge struct {
    from, to string
    delta    float64  // Minimum length constraint
    weight   float64  // Ω·ω
}

type xSimplexState struct {
    s        *layoutState
    auxNodes map[string]*xAuxNode
    auxEdges map[xEdgeKey]*xAuxEdge
    tree     *xSpanningTree
}
```

**Key functions:**

```go
func (s *layoutState) assignXCoordinatesNetworkSimplex()
func (xs *xSimplexState) buildAuxiliaryGraph()
func (xs *xSimplexState) xFeasibleTree() *xSpanningTree
func (xs *xSimplexState) xSlack(key xEdgeKey) float64
func (xs *xSimplexState) initCutValues()
func (xs *xSimplexState) leaveEdge() (xEdgeKey, bool)
func (xs *xSimplexState) enterEdge(leave xEdgeKey) xEdgeKey
func (xs *xSimplexState) exchangeEdges(leave, enter xEdgeKey)
func (xs *xSimplexState) extractCoordinates()
```

### 3. Modify `position.go`

```go
func (s *layoutState) assignCoordinates() {
    s.assignYCoordinates()

    switch s.opts.XCoordAlgorithm {
    case XNetworkSimplex:
        s.assignXCoordinatesNetworkSimplex()
    default:
        // Existing BK/simple logic
        if len(s.nodes) > s.opts.BKThreshold {
            s.assignXCoordinatesSimple()
        } else {
            s.assignXCoordinatesBK()
        }
    }
}
```

## Algorithm Flow

```
assignXCoordinatesNetworkSimplex()
    |
    |-- (1) assignXCoordinatesSimple()  // Initial feasible solution
    |
    |-- (2) buildAuxiliaryGraph()
    |       - Add original nodes
    |       - For each edge: create proxy node + 2 edges
    |       - For same-rank pairs: add separation edges
    |
    |-- (3) xFeasibleTree()
    |       - Greedy min-slack edge selection
    |       - Tighten edges as added
    |
    |-- (4) initLowLim() + initCutValues()
    |
    |-- (5) Simplex loop:
    |       while (leave = leaveEdge()) found:
    |           enter = enterEdge(leave)
    |           exchangeEdges(leave, enter)
    |
    |-- (6) extractCoordinates()
    |       - Copy X from aux nodes to layout nodes
    |
    |-- (7) normalizeXCoordinates()
            - Shift so min X = 0
```

## Key Differences: Y vs X Simplex

Both algorithms are in `simplex.go`. Key differences:

| Aspect | Y Simplex | X Simplex |
|--------|-----------|-----------|
| Values | int (ranks) | float64 (coordinates) |
| Constraints | minlen (edge length) | delta (separation) |
| Auxiliary nodes | None | One per original edge |
| slack() | `toRank - fromRank - minlen` | `x[to] - x[from] - delta` |
| Tolerance | Exact integer comparison | Float tolerance (1e-9) |

## Test Strategy

1. **Unit tests** - Each component (aux graph, feasible tree, cut values, etc.)
2. **Contract tests** - Run existing contract fixtures with `XNetworkSimplex`
3. **Comparison tests** - Both algorithms satisfy separation constraints
4. **Benchmarks** - Compare BK vs NetworkSimplex performance

```bash
go test -short ./...
go test -run TestContract -v
go test -bench=BenchmarkXCoord -count=5
```

## Future: Anti-Stacking Extension

Once base implementation works, add cross-layer constraints:

```go
// In buildAuxiliaryGraph, after separation edges:
if s.opts.PreventStacking {
    for stacked pairs across layers:
        addAuxEdge(left, right, minSep, 0, xEdgeAntiStack)
}
```

## References

- **Gansner, Koutsofios, North, Vo (1993)**: "A Technique for Drawing Directed Graphs" - IEEE TSE Vol. 19 No. 3. Section 4 describes the auxiliary graph technique. [`literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`](../literature/TSE93-gansner-technique-drawing-directed-graphs.pdf)
