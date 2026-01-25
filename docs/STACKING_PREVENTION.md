# Stacking Prevention

**Status: FOUNDATION IMPLEMENTED**

Network Simplex for X coordinate assignment is now implemented (`xsimplex.go`). This provides the foundation for anti-stacking constraints. The next step is adding cross-layer separation edges to the auxiliary graph.

See `X_COORDINATE_SIMPLEX.md` for implementation details.

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

### Why This Enables Anti-Stacking

The auxiliary graph approach allows us to:

1. **Express cross-layer constraints**: Add edges between nodes on different layers with minimum separation requirements

2. **Penalize vertical alignment**: Add negative-weight edges or penalty terms for stacked configurations

3. **Global optimization**: Network simplex finds the globally optimal solution considering all constraints simultaneously

### Comparison

| Aspect | Brandes-Köpf (Current) | Network Simplex (Planned) |
|--------|------------------------|---------------------------|
| Cross-layer constraints | Not possible | Native support |
| Anti-stacking constraints | Not possible | Add as penalty edges |
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

### Implementation Path

1. **Reuse existing simplex** (`simplex.go`) - already implements network simplex for ranking
2. **Build auxiliary graph** from the positioned graph after ordering
3. **Add separation edges** for same-rank node pairs with `δ=ρ(a,b)`, `ω=0`
4. **Add edge-cost edges** for each original edge with the Ω/ω weights
5. **Run network simplex** to find optimal X coordinates
6. **Extract positions** from the auxiliary graph solution

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
| `position.go` | Brandes-Köpf coordinate assignment (current) |
| `simplex.go` | Network Simplex (currently ranking only, future: X coordinates) |

## References

- **Gansner, Koutsofios, North, Vo (1993)**: "A Technique for Drawing Directed Graphs" - IEEE TSE Vol. 19 No. 3. Section 4 describes the auxiliary graph technique for X coordinates. [`literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`](../literature/TSE93-gansner-technique-drawing-directed-graphs.pdf)
- **Brandes & Köpf (2001)**: "Fast and Simple Horizontal Coordinate Assignment" - Current algorithm for X coordinates
- **ELK Layered**: https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html
