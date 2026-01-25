# Stacking Prevention

**Status: IMPLEMENTED** (post-hoc nudging) | **Planned: Auto-generated constraints**

## Problem

When connected nodes are vertically aligned ("stacked"), edges become vertical lines that look cluttered:

```
Stacked (current problem):         Offset (desired):

      [A]                              [A]
       |                                 \
       |   ← vertical edge                \   ← diagonal edge
       |      (cluttered)                  \     (cleaner)
      [C]                                 [C]
```

This affects single edges (A→C) and fan-in/fan-out patterns (A,B→C).

## Why Standard Algorithms Cause This

All coordinate assignment algorithms optimize for **vertical alignment**:

| Algorithm | Optimization Target | Effect |
|-----------|--------------------| -------|
| Brandes-Kopf | Align with median neighbor | Causes stacking |
| Network Simplex | Minimize edge length | Causes stacking |
| Graphviz | Minimize weighted edge length | Causes stacking |

Stacking is the *goal* of these algorithms, not a bug. They assume straight edges are always better.

## Current Solution: Post-hoc Nudging

`spread.go` implements a post-processing phase that:

1. **Detects stacking** - finds connected node pairs within threshold X distance
2. **Nudges apart** - shifts nodes horizontally by a fixed margin (15px)

```go
// Usage
layout := g.Layout(posit.Options{
    SpreadStackedNodes: true,   // Enable the feature
    StackingThreshold:  50,     // Optional: pixels within which nodes are "stacked"
})
```

### How It Works

```go
func (s *layoutState) spreadStackedNodes() {
    // Collect all connected pairs (short edges + long edges via dummy chains)
    pairs := s.collectRealPairs()

    for _, pair := range pairs {
        if s.areStacked(pair.a, pair.b, threshold) {
            s.nudgeApart(pair.a, pair.b, margin)
        }
    }
}
```

### Limitations

| Issue | Impact |
|-------|--------|
| Fixed 15px margin | May be too much or too little |
| Doesn't use optimizer | Fights against BK instead of working with it |
| May create new conflicts | Nudging one pair can stack another |
| Single pass | Doesn't iterate to convergence |

## Planned Improvement: Auto-Generated Constraints

Replace post-hoc nudging with **constraint-based re-layout**:

```
Standard Approach (MSAGL):     Posit Approach:
User specifies constraints  →  Algorithm auto-detects stacking
Single pass with constraints → Multi-pass: layout → analyze → constrain → re-layout
"Do what I say"             →  "Do what looks good"
```

### Algorithm

```go
func (s *layoutState) assignXCoordinatesWithConstraints() {
    // Pass 1: Standard BK
    s.assignXCoordinatesBK()

    for iteration := 0; iteration < maxIterations; iteration++ {
        // Detect stacking
        stackedPairs := s.detectStackedPairs()
        if len(stackedPairs) == 0 {
            break // Converged
        }

        // Auto-generate separation constraints
        for _, pair := range stackedPairs {
            s.addSeparationConstraint(pair.a, pair.b, minSeparation)
        }

        // Re-run BK - separation() now enforces constraints
        s.assignXCoordinatesBK()
    }
}
```

### Why This Is Better

| Aspect | Post-hoc Nudge | Constraint Re-layout |
|--------|----------------|----------------------|
| Uses BK optimization | No | Yes |
| Considers full graph | No | Yes |
| Margin calculation | Fixed 15px | Algorithm-determined |
| Creates new conflicts | Possible | No (holistic) |
| Convergence | Single pass | Iterates until stable |

### Integration Point

The `separation()` function (`position.go:479-515`) is called for every pair of adjacent same-layer nodes during BK compaction. We add constraint checking there:

```go
func (s *layoutState) separation(leftID, rightID string) float64 {
    // ... existing NodeSep, dummy, cluster logic ...

    // Check for auto-generated separation constraints
    for _, c := range s.separationConstraints {
        if c.matches(leftID, rightID) {
            sep = max(sep, c.minGap)
        }
    }

    return left.width/2 + sep + right.width/2
}
```

### Key Insight: Same-Layer Conversion

`separation()` only controls same-layer spacing. For cross-layer stacking (A in layer 0, C in layer 1), we need to convert to same-layer constraints:

```
Cross-layer stacking:           Same-layer constraint:

Layer 0:  [A]  [B]              Constraint: "A and B need more separation"
           \  /                  (so their edges to C don't cross)
Layer 1:   [C]
```

When A and B are both stacked over C, spreading A and B apart in layer 0 creates diagonal edges that don't cross.

## Implementation Details

### Phase Timing

The spread phase runs AFTER `addDummyNodes()`:

```
Phase 1: Cycle removal
Phase 2: Ranking (layer assignment)
Phase 3: addDummyNodes() ← REMOVES long edges, creates dummy chains
Phase 4: Crossing minimization
Phase 5a: Coordinate assignment (BK)
Phase 5b: spreadStackedNodes() ← s.edges doesn't have long edges!
Phase 6: Edge routing
```

**Important**: Long edges are removed from `s.edges` and replaced with dummy chains. Detection must check both `s.edges` (short edges) and `s.dummyChains` (long edges):

```go
// Also collect real-to-real pairs from dummy chains (long edges)
for _, firstDummy := range s.dummyChains {
    dummy := s.nodes[firstDummy]
    if dummy == nil || dummy.edgeLabel == nil {
        continue
    }
    origKey := dummy.edgeLabel.key  // Has original source and target
    pairs = append(pairs, realPair{origKey.from, origKey.to})
}
```

### Decision: Which Node to Move

When A is stacked over C, we could move either. Factors:

| Factor | Prefer Moving |
|--------|---------------|
| Lower degree | Move node with fewer connections |
| More space available | Move node with room to shift |
| Not in alignment block | Move node that's block root |
| Minimize cascade | Move node affecting fewer others |

## Files

| File | Purpose |
|------|---------|
| `spread.go` | Current post-hoc nudging implementation |
| `position.go` | BK coordinate assignment, `separation()` function |
| `state.go` | Layout pipeline, will hold constraint storage |

## References

- Brandes & Kopf (2001): Fast and simple horizontal coordinate assignment
- Gansner et al. (1993): A technique for drawing directed graphs
- ELK Layered: https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html
