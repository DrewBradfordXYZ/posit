# Stacking Prevention

**Status: COMPLETE**

Anti-stacking is implemented as post-processing in `simplex.go`, running after the X coordinate network simplex completes.

## Usage

```go
layout := g.Layout(posit.Options{
    XCoordAlgorithm: posit.XNetworkSimplex,
    PreventStacking: true,
    // Optional: customize minimum edge-to-edge gap (default: 120)
    StackingMinSep:  120,
})
```

**Options:**
- `XCoordAlgorithm: XNetworkSimplex` - Required (anti-stacking runs after X simplex)
- `PreventStacking: true` - Enables post-processing nudge
- `StackingMinSep: float64` - Minimum edge-to-edge gap between connected nodes (default: 120)

## Problem

When connected nodes are vertically aligned ("stacked"), edges become vertical lines that look cluttered:

```
Stacked (problem):              Offset (desired):

      [A]                              [A]
       |                                 \
       |   <- vertical edge                \   <- diagonal edge
       |      (cluttered)                  \     (cleaner)
      [C]                                 [C]
```

This affects single edges (A->C) and fan-in/fan-out patterns (A,B->C).

## Why Standard Algorithms Cause This

All coordinate assignment algorithms optimize for **short, straight edges**:

| Algorithm | Optimization Target | Effect |
|-----------|--------------------| -------|
| Brandes-Kopf | Align with median neighbor | Causes stacking |
| Network Simplex | Minimize weighted edge length | Causes stacking |
| Graphviz dot | Minimize weighted edge length | Causes stacking |

Stacking is the *goal* of these algorithms, not a bug.

## Implementation: Post-Processing Nudge

Anti-stacking runs **after** the simplex completes, not as hard constraints within it. This is the right approach because anti-stacking is a cosmetic preference, not a structural requirement. The simplex handles the hard constraints (node ordering, separation, alignment), then `enforceAntiStacking()` adjusts spacing where needed.

### How It Works

1. **Compute center of mass** of the graph (average X of all nodes)
2. **Iterate through `realEdges`** (original edge endpoints, not dummy segments)
3. For each connected pair with insufficient edge-to-edge gap:
   - Push the node **further from center of mass** outward by the needed delta
   - Propagate the nudge to same-layer neighbors to maintain ordering constraints
4. **Repeat** until no violations remain (up to 50 iterations)

### Why Not Simplex Constraints?

We initially tried adding anti-stacking as hard constraint edges in the auxiliary graph. This worked for small graphs but failed for complex ones: the constraints created **infeasible positive-weight cycles** when combined with existing separation and alignment edges. The simplex couldn't find a solution satisfying all constraints simultaneously.

Post-processing avoids this entirely — it always finds a feasible result because it adjusts positions incrementally rather than requiring global constraint satisfaction.

### Long Edges

Edges spanning multiple layers get split into dummy segments. Anti-stacking uses `realEdges` (stored before dummy insertion) to constrain the actual source/target nodes, not dummy-to-dummy pairs.

## Detection: Edge-to-Edge Gap

Detection uses edge-to-edge gap (not center distance) because the client's `computeOptimalSides()` function uses the same metric for deciding edge routing sides:

```
Node A: [----200px----]
Node B:          [----200px----]
         <- 50px gap (stacked, routes same-side)

Node A: [----200px----]
Node B:                    [----200px----]
                  <- 150px gap (offset, routes opposing sides)
```

The default `StackingMinSep` of 120px matches the client's threshold.

## Files

| File | Purpose |
|------|---------|
| `simplex.go` | `enforceAntiStacking()` and `nudgeNodeAndNeighbors()` |
| `state.go` | `realEdges` field (original endpoints before dummy insertion) |
| `posit.go` | `PreventStacking` and `StackingMinSep` options |

## References

- **Gansner, Koutsofios, North, Vo (1993)**: "A Technique for Drawing Directed Graphs" - IEEE TSE Vol. 19 No. 3. Section 4 describes the auxiliary graph technique for X coordinates. [`literature/TSE93-gansner-technique-drawing-directed-graphs.pdf`](../literature/TSE93-gansner-technique-drawing-directed-graphs.pdf)
