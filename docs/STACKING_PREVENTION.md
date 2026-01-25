# Stacking Prevention

**Status: COMPLETE**

Network Simplex for X coordinate assignment with anti-stacking constraints is implemented in `simplex.go`.

## Usage

```go
layout := g.Layout(posit.Options{
    XCoordAlgorithm: posit.XNetworkSimplex,
    PreventStacking: true,
    // Optional: customize minimum separation (default: NodeSep/2)
    StackingMinSep:  25,
})
```

**Options:**
- `XCoordAlgorithm: XNetworkSimplex` - Required to enable constraint-based X positioning
- `PreventStacking: true` - Adds cross-layer separation constraints
- `StackingMinSep: float64` - Minimum horizontal separation between connected nodes on adjacent layers (default: `NodeSep/2`)

## Problem

When connected nodes are vertically aligned ("stacked"), edges become vertical lines that look cluttered:

```
Stacked (problem):              Offset (desired):

      [A]                              [A]
       |                                 \
       |   ← vertical edge                \   ← diagonal edge
       |      (cluttered)                  \     (cleaner)
      [C]                                 [C]
```

This affects single edges (A→C) and fan-in/fan-out patterns (A,B→C).

## Why Standard Algorithms Cause This

All coordinate assignment algorithms optimize for **short, straight edges**:

| Algorithm | Optimization Target | Effect |
|-----------|--------------------| -------|
| Brandes-Köpf | Align with median neighbor | Causes stacking |
| Network Simplex | Minimize Σ ω(e)|x_w - x_v| | Causes stacking |
| Graphviz dot | Minimize weighted edge length | Causes stacking |

Stacking is the *goal* of these algorithms, not a bug. The optimization function `min Σ ω(e)|x_w - x_v|` is minimized when connected nodes have the same X coordinate.

## Current State

Posit uses Brandes-Köpf (BK) for X coordinate assignment. BK is a fast O(n) algorithm that produces good layouts but has no mechanism for preventing stacking or expressing cross-layer constraints.

## Planned Solution: Network Simplex for X Coordinates

The Graphviz `dot` algorithm (Gansner et al. 1993, Section 4) uses Network Simplex for **both** ranking (Y) and coordinate assignment (X). Posit currently uses Network Simplex only for ranking.

### The Optimization Problem

The X coordinate assignment problem from the paper:

```
min   Σ      Ω(e) ω(e) |x_w - x_v|
    e=(v,w)

subject to: x_b - x_a ≥ ρ(a,b)
```

Where:
- `ρ(a,b)` = minimum separation between adjacent nodes a and b on the same rank
- `Ω(e)` = internal weight favoring straight long edges (1 for real-real, 2 for real-virtual, 8 for virtual-virtual)
- `ω(e)` = user-specified edge weight

### The Auxiliary Graph Transformation

The absolute value `|x_w - x_v|` cannot be directly optimized. The paper describes an **auxiliary graph** transformation that converts this into a standard network simplex problem.

**Original graph G:**
```
    u ----e---- v
```

**Auxiliary graph G':**
```
         u
        /
    n_e     (δ=0, ω=Ω(e)ω(e))
        \
         v

    v ---f--- w   (same-rank separation: δ=ρ(v,w), ω=0)
```

For every edge `e = (u,v)` in G:
1. Create a new node `n_e`
2. Add edges `(n_e, u)` and `(n_e, v)` with `δ=0` and `ω=Ω(e)ω(e)`

For every pair of adjacent same-rank nodes `(v,w)`:
1. Add edge `(v,w)` with `δ=ρ(v,w)` and `ω=0`

**Key insight:** In the optimal solution, one of `(n_e, u)` or `(n_e, v)` has length 0, and the other has length `|x_u - x_v|`. This means `n_e` is assigned `min(x_u, x_v)`, and the cost equals the original absolute value cost.

### How Anti-Stacking Works

The auxiliary graph approach enables anti-stacking by:

1. **Detecting stacked pairs**: For each edge connecting nodes on adjacent layers, check if their horizontal bounds overlap
2. **Adding separation edges**: For overlapping pairs, add a constraint edge requiring minimum horizontal separation
3. **Global optimization**: Network simplex finds the optimal solution that satisfies all constraints

The `addAntiStackingEdges()` function in `simplex.go` implements this by iterating over graph edges and adding separation constraints for adjacent-layer pairs that are currently stacked.

### Comparison

| Aspect | Brandes-Köpf | Network Simplex |
|--------|--------------|-----------------|
| Cross-layer constraints | Not possible | Native support |
| Anti-stacking constraints | Not possible | `PreventStacking: true` |
| Optimization scope | Per-layer, heuristic | Global, optimal |
| Complexity | O(n) | O(n·m) but fast in practice |
| Node ports | Not native | Supported via δ offsets |

### Node Ports

The paper also describes how to handle "node ports" (edge endpoints offset from node center). For an edge `e = (u,v)` with port offsets `Δu` and `Δv`:

```
d_e = Δv - Δu  (assuming Δu ≤ Δv)

Cost becomes: Ω(e) ω(e) |x_v - x_u + d_e|

In auxiliary graph: δ(e_u) = d_e, δ(e_v) = 0
```

This is directly applicable to Posit's port system.

### Implementation (Complete)

The X simplex implementation in `simplex.go`:

1. **`assignXCoordinatesNetworkSimplex()`** - Entry point, orchestrates the algorithm
2. **`buildAuxiliaryGraph()`** - Constructs the constraint graph:
   - Step 1: Add original nodes
   - Step 2: Add proxy nodes and edge-cost edges (for weighted edge length optimization)
   - Step 3: Add same-rank separation edges (adjacent nodes on same layer)
   - Step 4: Add anti-stacking edges (if `PreventStacking` enabled)
3. **`addAntiStackingEdges()`** - Detects stacked pairs and adds separation constraints
4. **`xFeasibleTree()`** / **simplex loop** / **`extractCoordinates()`** - Standard network simplex

### Performance Considerations

From the paper (Section 4.3):

> "The auxiliary graph is considerably larger than the original one. If the original graph has V nodes, E edges, and R ranks, the graph with 'virtual' nodes added has V+D nodes and E+D edges, where D is the number of 'virtual nodes.' The auxiliary graph then has V+E+2D nodes and V+2E+3D-R edges."

The paper describes several optimizations:
- Construct initial feasible tree using graph structure (not from scratch)
- Use all same-rank edges as tree edges initially
- Compute cut values incrementally from leaves inward

These optimizations made the network simplex approach "as fast or faster than the heuristic implementation."

## Detection: Horizontal Bound Overlap

When implementing anti-stacking, detection should use horizontal bound overlap (not center distance) because edges connect at node **boundaries** (ports):

```
Correct detection (horizontal bounds overlap):

  Node A: [----100px----]
  Node B:      [----100px----]
               ↑ overlap = stacked

Wrong detection (center distance):

  centerA ----60px---- centerB
  But nodes still overlap! Centers ignore width.
```

## Files

| File | Purpose |
|------|---------|
| `position.go` | Coordinate dispatch (BK or Network Simplex) |
| `simplex.go` | Network Simplex for Y ranking and X coordinates |

## References

- **Gansner, Koutsofios, North, Vo (1993)**: "A Technique for Drawing Directed Graphs" - IEEE TSE Vol. 19 No. 3. Section 4 describes the auxiliary graph technique for X coordinates. [`literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`](../literature/TSE93-gansner-technique-drawing-directed-graphs.pdf)
- **Brandes & Köpf (2001)**: "Fast and Simple Horizontal Coordinate Assignment" - Current algorithm for X coordinates
- **ELK Layered**: https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html
