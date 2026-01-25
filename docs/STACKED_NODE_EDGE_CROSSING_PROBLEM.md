# Stacked Node Edge Crossing Problem

**Status: SOLVED** - See `spread.go` for implementation of Solution 4.

## Problem Statement

When nodes in adjacent layers are nearly vertically aligned ("stacked"), edge routing produces unintuitive results with excessive crossings. The port side selection flips between left/right inconsistently, causing edges to cross over each other.

```
Problematic scenario:

Layer 0:    [  Node A  ]      [  Node B  ]
                 ↓                  ↓
                  \                /
                   \    crosses   /
                    \            /
                     ↘        ↙
Layer 1:           [  Node C  ]

Edges A→C and B→C cross because:
- A is slightly left of C's center → edge exits A's right, enters C's left
- B is slightly right of C's center → edge exits B's left, enters C's right
- The edges cross in the middle
```

## Observed Behavior

In basetypes schema graphs with hub tables (many incoming relationships), we see:
- Multiple parent tables at top layers
- Hub table (e.g., "Orders") at bottom receiving many edges
- Edges fan out and cross each other chaotically
- Port sides flip based on small X differences, creating a "twisted" appearance

## Root Cause Analysis

### Current Side Selection Algorithm

Posit uses **boundary-based side selection** (`route.go:inferEdgeSides`):

```go
// Simplified logic
func inferSide(sourceNode, targetNode) Side {
    // Ray from source center toward target center
    // Return the side where ray exits source boundary
    if targetCenter.X > sourceRight {
        return Right
    } else if targetCenter.X < sourceLeft {
        return Left
    }
    // ... top/bottom for vertical cases
}
```

This works well when nodes are clearly separated horizontally. But when nodes are nearly stacked:
- Small X differences (even 1-2 pixels) determine left vs right
- No consideration of other edges from the same port
- No global optimization to minimize crossings

### Why NodeNodeBetweenLayers Doesn't Help

`NodeNodeBetweenLayers` prevents **vertical boundary overlap** between nodes. It ensures tall nodes don't visually collide across layers. But it doesn't:
- Spread nodes horizontally
- Influence port side selection
- Reduce edge crossings

## Potential Solutions

### Solution 1: Enable Orthogonal Routing

**What:** Set `RouteStyle: posit.RouteOrthogonal` in basetypes

**How it helps:**
- Edges use horizontal/vertical segments instead of diagonal lines
- Channel routing algorithm spaces parallel edges
- May produce cleaner visual result even with same node positions

**Effort:** Minimal (one-line change)

**Limitations:**
- Doesn't fix the underlying node position/side selection issue
- May just make the crossings more visible (90° angles)

### Solution 2: Increase Horizontal Spreading

**What:** Increase `NodeSep` significantly (e.g., 60 → 150)

**How it helps:**
- Pushes nodes further apart horizontally
- Reduces "near-vertical alignment" scenarios
- More clearance for edges to route without crossing

**Effort:** Minimal (one parameter change)

**Limitations:**
- Makes layout wider (may not fit viewport)
- Doesn't solve the problem, just reduces frequency
- Hub tables will still have crossing issues

### Solution 3: Side Consistency Heuristic

**What:** New phase in posit that detects problematic configurations and adjusts

**Algorithm sketch:**
```go
func (s *layoutState) enforceSideConsistency() {
    // For each node with multiple outgoing edges to the same layer
    for _, node := range s.nodes {
        edgesToSameLayer := s.getEdgesToLayer(node, node.rank + 1)
        if len(edgesToSameLayer) < 2 {
            continue
        }

        // Check if targets are "stacked" (within threshold X range)
        if s.targetsAreStacked(edgesToSameLayer, threshold: 50px) {
            // Force all edges to use same side (left or right)
            // Pick side based on majority or barycenter
            side := s.chooseDominantSide(edgesToSameLayer)
            for _, edge := range edgesToSameLayer {
                edge.sourceSide = side
            }
        }
    }
}
```

**Effort:** Medium (new phase, testing)

**Benefits:**
- Addresses root cause
- Edges from same source to stacked targets won't cross
- Could also detect incoming edges to hub nodes

### Solution 4: Position Adjustment for Stacked Nodes

**What:** During coordinate assignment, detect near-vertical stacking and spread nodes

**Algorithm sketch:**
```go
func (s *layoutState) spreadStackedNodes() {
    // For each pair of adjacent layers
    for i := 0; i < len(s.layers)-1; i++ {
        upper := s.layers[i]
        lower := s.layers[i+1]

        // Find nodes in lower layer that are "under" multiple upper nodes
        for _, lowerNode := range lower {
            stackedAbove := s.findNodesAbove(lowerNode, threshold: 50px)
            if len(stackedAbove) > 1 {
                // Spread the upper nodes apart
                s.spreadNodes(stackedAbove, minSeparation: 100px)
            }
        }
    }
}
```

**Effort:** Medium-High (affects coordinate assignment)

**Benefits:**
- Prevents stacking in the first place
- Side selection becomes unambiguous

## Experiments to Try First

Before implementing Solution 3 or 4, try these quick experiments:

### Experiment A: Orthogonal Routing

```go
// In basetypes layout.go
return g.Layout(posit.Options{
    Direction:             posit.TopToBottom,
    NodeSep:               60,
    RankSep:               100,
    Algorithm:             posit.NetworkSimplex,
    NodeNodeBetweenLayers: 10,
    RouteStyle:            posit.RouteOrthogonal,  // ADD THIS
})
```

**Observe:** Do the H/V segments make the layout clearer, or just different?

### Experiment B: Wider NodeSep

```go
// In basetypes layout.go
return g.Layout(posit.Options{
    Direction:             posit.TopToBottom,
    NodeSep:               150,  // INCREASE from 60
    RankSep:               100,
    Algorithm:             posit.NetworkSimplex,
    NodeNodeBetweenLayers: 10,
})
```

**Observe:** Does wider spacing reduce the crossing problem?

### Experiment C: Combined

```go
return g.Layout(posit.Options{
    Direction:             posit.TopToBottom,
    NodeSep:               120,
    RankSep:               120,
    Algorithm:             posit.NetworkSimplex,
    NodeNodeBetweenLayers: 20,
    RouteStyle:            posit.RouteOrthogonal,
})
```

**Observe:** Combined effect of spacing + orthogonal routing.

## Decision Framework

After experiments:

| Result | Next Step |
|--------|-----------|
| Orthogonal routing looks good | Ship it, close issue |
| Wider spacing helps but too wide | Consider Solution 3 (side consistency) |
| Neither helps significantly | Implement Solution 4 (position adjustment) |
| Problem is rare in practice | Document as known limitation |

## Related Work

- ELK has `edgeRouting.polyline.sloppyRouting` for relaxed routing
- Graphviz uses splines with iterative refinement
- dagre uses simple center-to-center with bend points

## Solution: Implemented

After experiments showed that quick fixes (orthogonal routing, wider spacing) don't solve the fundamental problem, we designed and implemented a novel solution.

**Implementation:** `spread.go`
**Design:** [SPREAD_STACKED_NODES_DESIGN.md](SPREAD_STACKED_NODES_DESIGN.md)

The `spreadStackedNodes()` phase:
1. Detects nodes that are nearly vertically aligned
2. Spreads them horizontally to create unambiguous port-side selection
3. Trades slightly wider layouts for dramatically clearer edge routing

**Usage:**
```go
layout := g.Layout(posit.Options{
    SpreadStackedNodes: true,  // Enable the feature
    StackingThreshold:  50,    // Optional: custom threshold in pixels
})
```

This is a novel contribution to the Sugiyama algorithm family - existing algorithms optimize for edge straightness (causing stacking), while this algorithm optimizes for **edge clarity** by spreading stacked nodes.

### Bug Fix: Long Edges Not Detected (2025-01)

**Problem:** The initial implementation only detected stacked nodes connected by short edges (edges within adjacent layers). Long edges spanning multiple layers (e.g., ranks 5 → 8) were not detected.

**Root cause:** The spread phase runs AFTER `addDummyNodes()`, which REMOVES long edges from `s.edges` and replaces them with dummy chains. The original `nudgeAllStackedEdgePairs()` only iterated `s.edges`.

**Fix:** Updated `spread.go` to also iterate `s.dummyChains`:

```go
// Also collect real-to-real pairs from dummy chains (long edges)
for _, firstDummy := range s.dummyChains {
    dummy := s.nodes[firstDummy]
    if dummy == nil || dummy.edgeLabel == nil {
        continue
    }
    origKey := dummy.edgeLabel.key
    pairs = append(pairs, realPair{origKey.from, origKey.to})
}
```

Also updated `hasEdgeBetween()` to check both `s.edges` and `s.dummyChains`.

**Status:** Fixed. Long edges like `DocumentTypes → PaymentWorkflows` are now detected as stacked pairs.

### Known Limitation: Nudge Margin

The current nudge margin (15px) may be too small for some use cases. The algorithm picks the minimum shift to separate stacked nodes, but doesn't account for:
- Node width variation
- Edge density at hub nodes
- User preference for tighter vs. looser layouts

Future work could make this configurable via `Options.NudgeMargin`.

## References

- `posit/route.go` - Current edge routing implementation
- `posit/position.go` - Coordinate assignment (Brandes-Köpf)
- `basetypes/internal/web/graph/layout.go` - Integration point
- `SPREAD_STACKED_NODES_DESIGN.md` - Proposed solution
