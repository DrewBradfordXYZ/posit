# Constraint-Based Stacking Prevention

**Status: DESIGN NEEDED** - Previous approach was incorrect. Rethinking required.

## The Actual Problem

When two nodes connected by an edge are **vertically aligned** (stacked), we want to move one of them horizontally to produce a cleaner diagonal edge:

```
Current (stacked):              Desired (offset):

   [A]                             [A]
    |                                \
    |   ← Vertical edge               \   ← Diagonal edge
    |      (visually cluttered)        \     (cleaner)
   [C]                                 [C]
```

The current solution in `spread.go` uses **post-hoc nudging**: after coordinate assignment, detect stacking and nudge nodes apart. This works but fights against the layout algorithm.

The goal is to integrate stacking prevention **into** the coordinate assignment algorithm so it finds optimal positions holistically.

## Why `separation()` Cannot Solve This

The previous design proposed modifying `separation()` in the Brandes-Köpf algorithm. This was **incorrect** for the single-edge stacking problem.

### What `separation()` Does

`separation(leftID, rightID)` is called for **adjacent nodes in the same layer** to determine minimum horizontal spacing:

```
Layer 0:   [A]  [B]  [C]   ← separation() controls gaps between A-B, B-C
                |
Layer 1:       [D]
```

It can spread A, B, and C apart within their layer. It **cannot** move A relative to D across layers.

### What It Cannot Do

For a single edge A→C where A and C are in different layers:

```
Layer 0:   [A]         ← separation() has no say here
            |
Layer 1:   [C]         ← A and C are never "adjacent" to separation()
```

There's no mechanism in `separation()` to push A away from C vertically-aligned position.

### The Fan-In Case Was a Distraction

The previous design focused on fan-in (A,B → C) and fan-out (A → B,C) cases:

```
[A]    [B]
  \    /
   \  /
   [C]
```

While spreading A and B apart helps this case, it doesn't address the fundamental problem: a single edge A→C where A is directly above C.

---

## Approaches That Might Work

### Approach 1: Barycenter/Median Bias

During crossing minimization (Phase 4), nodes are positioned based on the barycenter (average position) of their neighbors. We could add a bias term that pushes nodes away from direct vertical alignment.

**Idea**: When computing the target position for A, instead of centering it over C, offset it slightly.

**Challenge**: How much offset? This affects crossing minimization quality.

### Approach 2: Repulsion Term in Coordinate Assignment

Add a "repulsion" force between connected nodes that penalizes vertical alignment during BK coordinate assignment.

**Idea**: Modify the coordinate assignment objective function to include:
- Minimize edge length (existing)
- Minimize crossings (existing)
- **Penalize vertical alignment** (new)

**Challenge**: BK doesn't use an explicit objective function - it's heuristic-based.

### Approach 3: Post-BK Constraint Propagation

After initial BK pass:
1. Detect stacked pairs (A directly above C)
2. Decide which direction to offset A (left or right of C)
3. Propagate this as an **ordering constraint** in the layer
4. Re-run crossing minimization and/or coordinate assignment

**Idea**: If A should be "left of C's center", find a same-layer reference point and constrain A to be left of it.

**Challenge**: Converting cross-layer position goals into same-layer ordering constraints.

### Approach 4: Virtual Alignment Nodes

Insert a virtual "alignment target" node in the same layer as A, positioned where we want A to be. Use this as an alignment anchor during BK.

**Idea**:
1. Detect A is stacked over C
2. Insert virtual node V in layer 0 at position "C.x + offset"
3. Create alignment affinity between A and V
4. BK naturally pulls A toward V

**Challenge**: Managing virtual nodes, ensuring they don't interfere with real layout.

### Approach 5: Iterative Re-layout with Explicit Constraints (MSAGL-style)

MSAGL supports explicit horizontal constraints: "A must be left of B", "A and B must be adjacent".

**Idea**:
1. Run initial layout
2. Detect stacking
3. Create explicit constraint: "A.center must be at least X pixels from C.center"
4. Re-run layout with constraint solver

**Challenge**: Requires significant changes to add constraint solving to BK.

---

## Current State

The post-hoc nudging in `spread.go` works but is inelegant:
- Runs after coordinate assignment
- Uses fixed margins
- May create new conflicts
- Doesn't leverage the optimization algorithm

## Research Needed

Before implementing, we need to understand:

1. **How do other layout engines handle this?**
   - MSAGL uses explicit constraints (user-specified)
   - ELK has various spacing options
   - Graphviz uses spline routing to work around it

2. **What's the right trade-off?**
   - Wider layouts vs. straighter edges
   - Performance cost of re-running phases
   - Complexity of implementation

3. **Can we modify crossing minimization instead?**
   - The stacking happens because crossing minimization optimizes for edge straightness
   - Maybe the fix belongs there, not in coordinate assignment

## MSAGL Research Findings

From analyzing MSAGL.js:

### `DeltaBetweenVertices()` (analogous to our `separation()`)

```typescript
// MSAGL: xCoordsWithAlignment.ts:693-706
DeltaBetweenVertices(u: number, v: number): number {
    return (this.anchors[u].rightAnchor + this.anchors[v].leftAnchor + this.nodeSep) * sign
}
```

This only handles same-layer spacing, same as our `separation()`.

### MSAGL's Constraint System

MSAGL has explicit constraint classes that require **manual specification**:

- `HorizontalConstraintsForSugiyama`: `leftRightConstraints`, `leftRightNeighbors`, `BlockRootToBlock`
- `VerticalConstraintsForSugiyama`: `sameLayerConstraints`, `upDownConstraints`

These let users say "A must be left of B" but don't auto-detect stacking.

### Key Insight

MSAGL doesn't auto-solve the stacking problem either. It provides tools for users to manually constrain layouts. Our `SpreadStackedNodes` feature is actually novel - it **auto-detects** and fixes stacking without user intervention.

The question is: can we do this more elegantly than post-hoc nudging?

---

## Next Steps

1. **Investigate crossing minimization**: Can we bias node ordering during Phase 4 to reduce stacking?

2. **Prototype Approach 3**: Try converting stacking detection into same-layer ordering constraints.

3. **Benchmark current solution**: How bad is the post-hoc nudging really? Maybe it's good enough.

4. **Study ELK's approach**: ELK has extensive layered layout options - what do they do?

## References

- [SPREAD_STACKED_NODES_DESIGN.md](SPREAD_STACKED_NODES_DESIGN.md) - Current nudge-based implementation
- [STACKED_NODE_EDGE_CROSSING_PROBLEM.md](STACKED_NODE_EDGE_CROSSING_PROBLEM.md) - Problem statement
- `position.go:479-515` - The `separation()` function (same-layer only)
- `spread.go` - Current post-hoc nudging solution
- `_ref/msagljs/` - Microsoft MSAGL.js reference implementation
