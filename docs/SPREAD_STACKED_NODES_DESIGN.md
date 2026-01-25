# Spread Stacked Nodes: A Novel Post-Layout Optimization

**Status: IMPLEMENTED** (see `spread.go`)

## Abstract

This document proposes a new phase for Sugiyama-style graph layout: **spreading stacked nodes** to improve edge routing clarity. Existing coordinate assignment algorithms optimize for edge straightness, which causes nodes to stack vertically. This stacking creates ambiguous port-side selection when nodes are nearly aligned, resulting in chaotic edge crossings. We propose a post-processing phase that detects stacked nodes and spreads them horizontally to create unambiguous port-side selection.

## Problem Statement

### The Stacking Effect

Coordinate assignment algorithms (Brandes-Köpf, Network Simplex) minimize total edge length by aligning connected nodes vertically. This is generally desirable - straight edges are easier to follow.

```
Optimal for edge length:          Problem for port selection:

    [A]                              [A] ← port exits right
     |                                 ↘
     |  (straight, minimal length)       ↘  (crosses with B→C edge)
     |                                     ↘
    [C]                              [B] → [C]
                                      ↗
                                    (port exits left, crosses A→C)
```

### Port-Side Selection Chaos

When nodes are nearly vertically aligned ("stacked"), the port-side selection algorithm faces ambiguity:

```go
// Current side selection (simplified)
func inferSide(source, target Node) Side {
    if target.CenterX > source.RightEdge {
        return Right
    } else if target.CenterX < source.LeftEdge {
        return Left
    }
    // Ambiguous zone: target is "under" source
    // Small X differences flip the decision
}
```

When `target.CenterX` is between `source.LeftEdge` and `source.RightEdge`, the side depends on tiny X differences. Multiple edges from different sources to a stacked target will have inconsistent sides, causing crossings.

### Visual Example

```
Before spreading (stacked):

Layer 0:   [Node A]    [Node B]
              \          /
               \   ✗    /     ← Edges cross because:
                \      /         A→C exits right (C is slightly right of A)
                 \    /          B→C exits left (C is slightly left of B)
                  \  /
Layer 1:        [Node C]


After spreading:

Layer 0:   [Node A]         [Node B]
              \                 /
               \               /     ← No crossing:
                \             /         A→C exits right
                 \           /          B→C exits right
                  \         /           Both enter C from same direction
                   \       /
Layer 1:          [Node C]
```

## Literature Gap

### What Existing Algorithms Optimize

| Algorithm | Optimization Target | Effect on Stacking |
|-----------|--------------------|--------------------|
| Brandes-Köpf | Align with median neighbor | Causes stacking |
| Network Simplex X | Minimize edge length | Causes stacking |
| BK Edge Straightening | Maximize straight edges | Causes more stacking |
| MSAGL StraightenEdgePaths | Straighten via dummy nodes | Doesn't affect real nodes |

### The Missing Optimization

No existing algorithm considers **port-side clarity** as an optimization target. They all assume:
- Straight edges = good
- Short edges = good
- Stacking = acceptable side effect

The missing insight: **Sometimes longer edges with consistent port sides are clearer than short edges with crossing port sides.**

## Proposed Algorithm

### Phase: Spread Stacked Nodes

Runs after coordinate assignment, before edge routing.

```go
// spreadStackedNodes detects vertically stacked nodes and spreads them
// horizontally to create unambiguous port-side selection.
func (s *layoutState) spreadStackedNodes() {
    threshold := s.opts.StackingThreshold // e.g., 0.5 * average node width
    if threshold <= 0 {
        threshold = s.averageNodeWidth() * 0.5
    }

    // Process each pair of adjacent layers
    for i := 0; i < len(s.layers)-1; i++ {
        upper := s.layers[i]
        lower := s.layers[i+1]

        // Find "convergence points" - lower nodes receiving edges from
        // multiple stacked upper nodes
        for _, lowerNode := range lower {
            stackedSources := s.findStackedSources(lowerNode, upper, threshold)
            if len(stackedSources) > 1 {
                s.spreadNodes(stackedSources, lowerNode)
            }
        }
    }
}
```

### Detection: Finding Stacked Sources

```go
// findStackedSources returns upper-layer nodes that:
// 1. Have edges to lowerNode
// 2. Are within 'threshold' X distance of lowerNode's center
func (s *layoutState) findStackedSources(lowerNode *layoutNode, upper []*layoutNode, threshold float64) []*layoutNode {
    var stacked []*layoutNode
    lowerCenterX := lowerNode.x + lowerNode.width/2

    for _, upperNode := range upper {
        if !s.hasEdge(upperNode.id, lowerNode.id) {
            continue
        }

        upperCenterX := upperNode.x + upperNode.width/2
        xDiff := abs(upperCenterX - lowerCenterX)

        // Node is "stacked" if its center is within threshold of lower node
        if xDiff < threshold {
            stacked = append(stacked, upperNode)
        }
    }

    return stacked
}
```

### Resolution: Spreading Stacked Nodes

```go
// spreadNodes moves stacked nodes apart so they're no longer ambiguous
func (s *layoutState) spreadNodes(stacked []*layoutNode, target *layoutNode) {
    if len(stacked) < 2 {
        return
    }

    // Sort by current X position
    sort.Slice(stacked, func(i, j int) bool {
        return stacked[i].x < stacked[j].x
    })

    // Calculate target's center and determine spread direction
    targetCenterX := target.x + target.width/2

    // Strategy: Move nodes to clear left/right of target
    // Left group: nodes that should connect from target's left
    // Right group: nodes that should connect from target's right

    midpoint := len(stacked) / 2
    leftGroup := stacked[:midpoint]
    rightGroup := stacked[midpoint:]

    minClearance := target.width/2 + s.opts.NodeSep/2

    // Move left group to be clearly left of target
    for _, node := range leftGroup {
        maxX := targetCenterX - minClearance - node.width/2
        if node.x + node.width/2 > maxX {
            s.shiftNodeLeft(node, (node.x + node.width/2) - maxX)
        }
    }

    // Move right group to be clearly right of target
    for _, node := range rightGroup {
        minX := targetCenterX + minClearance - node.width/2
        if node.x + node.width/2 < minX {
            s.shiftNodeRight(node, minX - (node.x + node.width/2))
        }
    }
}
```

### Constraint Preservation

When shifting nodes, we must preserve:

1. **Layer order**: Nodes stay in their assigned layer (Y unchanged)
2. **Within-layer order**: Relative X order within a layer is preserved
3. **Minimum separation**: `NodeSep` between same-layer neighbors
4. **No new overlaps**: Don't create overlaps while fixing stacking

```go
func (s *layoutState) shiftNodeLeft(node *layoutNode, amount float64) bool {
    // Find left neighbor in same layer
    leftNeighbor := s.getLeftNeighbor(node)

    if leftNeighbor != nil {
        // Can only shift as far as neighbor allows
        available := node.x - (leftNeighbor.x + leftNeighbor.width + s.opts.NodeSep)
        amount = min(amount, available)
    }

    if amount > 0 {
        node.x -= amount
        return true
    }
    return false
}
```

## Configuration

### New Options

```go
type Options struct {
    // ... existing options ...

    // SpreadStackedNodes enables the stacked-node spreading optimization.
    // When true, nodes that are nearly vertically aligned are spread apart
    // to create unambiguous port-side selection.
    // Default: false (opt-in, as it may increase layout width)
    SpreadStackedNodes bool

    // StackingThreshold is the X-distance within which nodes are considered
    // "stacked". Nodes within this distance of a shared target are spread.
    // Default: 0 (auto-calculate as 50% of average node width)
    StackingThreshold float64
}
```

## Trade-offs

### Benefits

1. **Clearer edge routing**: Edges don't cross due to port-side flipping
2. **Predictable port selection**: Port sides are unambiguous
3. **Better for dense graphs**: Hub nodes with many incoming edges benefit most
4. **Opt-in**: Doesn't affect existing layouts unless enabled

### Costs

1. **Wider layouts**: Spreading nodes increases total width
2. **Longer edges**: Some edges become longer (no longer straight)
3. **Additional phase**: Small performance cost (O(n²) worst case per layer pair)

### When to Use

| Scenario | Recommendation |
|----------|----------------|
| Schema diagrams with hub tables | Enable |
| Sparse graphs (<10 edges) | Disable (not needed) |
| Very wide graphs already | Disable (don't widen further) |
| Graphs with many fan-in/fan-out | Enable |

## Integration

### Pipeline Position

```go
func (s *layoutState) layout() *Layout {
    s.removeCycles()
    s.assignLayers()
    s.insertDummyNodes()
    s.minimizeCrossings()
    s.assignCoordinates()

    // NEW: Spread stacked nodes for edge clarity
    if s.opts.SpreadStackedNodes {
        s.spreadStackedNodes()
    }

    s.resolveCrossLayerOverlaps()
    s.computePortOffsets()
    s.routeEdges()
    return s.buildLayout()
}
```

### Interaction with Other Features

- **NodeNodeBetweenLayers**: Compatible. Spreading happens first, then cross-layer overlap resolution adjusts Y if needed.
- **Orthogonal routing**: Compatible. Spreading improves orthogonal routing by creating cleaner channel assignments.
- **Port constraints**: Compatible. Spreading affects node positions, not port offsets.

## Testing Strategy

### Unit Tests

```go
func TestSpreadStackedNodes_BasicCase(t *testing.T) {
    // Two sources stacked over one target
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "C")
    g.MustAddEdge("B", "C")

    layout := g.Layout(Options{
        SpreadStackedNodes: true,
    })

    // After spreading, A and B should be on opposite sides of C
    cCenter := layout.Nodes["C"].X + layout.Nodes["C"].Width/2
    aCenter := layout.Nodes["A"].X + layout.Nodes["A"].Width/2
    bCenter := layout.Nodes["B"].X + layout.Nodes["B"].Width/2

    // One should be clearly left, one clearly right
    if !((aCenter < cCenter && bCenter > cCenter) ||
         (aCenter > cCenter && bCenter < cCenter)) {
        t.Error("A and B should be on opposite sides of C after spreading")
    }
}

func TestSpreadStackedNodes_PreservesLayerOrder(t *testing.T) {
    // Spreading shouldn't change relative order within layer
}

func TestSpreadStackedNodes_RespectsNodeSep(t *testing.T) {
    // Spreading shouldn't violate minimum node separation
}

func TestSpreadStackedNodes_DisabledByDefault(t *testing.T) {
    // Without opt-in, no spreading occurs
}
```

### Visual Validation

Create test cases with known problematic patterns:
1. Fan-in: Multiple sources → single target
2. Fan-out: Single source → multiple targets
3. Diamond: A→B, A→C, B→D, C→D
4. Hub: Central node with many connections

## Future Enhancements

### Iterative Refinement

The initial algorithm is greedy (single pass). An enhancement could iterate:

```go
for i := 0; i < maxIterations; i++ {
    if !s.spreadStackedNodes() {
        break // No changes, stable
    }
}
```

### Cost Function Optimization

Instead of heuristic spreading, formulate as optimization:

```
Minimize: Σ (edge_length_increase)
Subject to: no_stacked_pairs, preserve_order, respect_separation
```

### Port-Aware Spreading

Consider port positions when deciding spread direction:

```go
// If source has port on right side, prefer moving it left of target
// (so edge exits right toward target)
```

## Implementation Notes

### Phase Timing and Dummy Chains

**Critical insight discovered during implementation:** The spread phase runs AFTER `addDummyNodes()` (Phase 3), which means long edges spanning multiple layers have already been REMOVED from `s.edges` and replaced with dummy chains.

```
Phase order:
  Phase 1: Cycle removal
  Phase 2: Ranking (layer assignment)
  Phase 3: addDummyNodes() ← REMOVES long edges, creates dummy chains
  Phase 4: Crossing minimization
  Phase 5a: Coordinate assignment
  Phase 5b: spreadStackedNodes() ← s.edges doesn't have long edges!
  Phase 6: Edge routing
```

When `nudgeAllStackedEdgePairs()` iterates `s.edges`, it only sees short edges (edges within adjacent layers). Long edges like `DocumentTypes → PaymentWorkflows` (spanning ranks 5 → 8) are NOT in `s.edges` - they've been replaced with a chain of dummy nodes.

**The fix:** Also iterate `s.dummyChains` to find original long edge connections:

```go
// In nudgeAllStackedEdgePairs(), collect pairs from dummy chains
for _, firstDummy := range s.dummyChains {
    dummy := s.nodes[firstDummy]
    if dummy == nil || dummy.edgeLabel == nil {
        continue
    }
    // edgeLabel.key has the original source and target
    origKey := dummy.edgeLabel.key
    pairs = append(pairs, realPair{origKey.from, origKey.to})
}
```

Similarly, `hasEdgeBetween()` must check both `s.edges` and `s.dummyChains`:

```go
func (s *layoutState) hasEdgeBetween(a, b string) bool {
    // Check short edges in s.edges
    for key := range s.edges {
        if (key.from == a && key.to == b) || (key.from == b && key.to == a) {
            return true
        }
    }
    // Check long edges via dummy chains
    for _, firstDummy := range s.dummyChains {
        dummy := s.nodes[firstDummy]
        if dummy == nil || dummy.edgeLabel == nil {
            continue
        }
        origKey := dummy.edgeLabel.key
        if (origKey.from == a && origKey.to == b) || (origKey.from == b && origKey.to == a) {
            return true
        }
    }
    return false
}
```

### Current Limitation: Post-hoc Nudging

**Status: TODO - Replace with constraint-based re-layout**

The current implementation uses a post-hoc "nudge" approach with a fixed 15px margin. This is a band-aid solution that:
- Doesn't leverage Posit's optimization algorithms
- Uses an arbitrary fixed margin
- May create new conflicts by pushing nodes into others
- Fights against the layout rather than working with it

## TODO: Constraint-Based Re-Layout

The elegant solution is to **re-run coordinate assignment with stacking constraints** rather than nudging positions afterward.

### Proposed Approach

1. **Detect stacking pairs** after initial coordinate assignment (keep current detection)
2. **Add minimum-separation constraints** between stacked pairs
3. **Re-run Brandes-Köpf coordinate assignment** with the new constraints
4. The algorithm naturally finds optimal positions respecting both edge-straightness AND separation

### Why This Is Better

| Aspect | Current (Nudge) | Proposed (Re-layout) |
|--------|-----------------|----------------------|
| Uses Posit's optimization | No | Yes |
| Considers full graph | No | Yes |
| Margin calculation | Fixed 15px | Algorithm-determined |
| May create new conflicts | Yes | No (holistic) |
| Performance | O(n) single pass | O(n²) but still fast |

### Design Questions to Resolve

1. **Constraint representation**: How do we express "nodes A and B must have centers at least X apart" in Brandes-Köpf?
   - Option A: Virtual edges with minimum length
   - Option B: Modified block formation (don't merge stacked nodes)
   - Option C: Post-alignment adjustment within BK framework

2. **Which phases to re-run**: Just coordinate assignment, or also crossing minimization?
   - Coordinate assignment only: Faster, preserves node ordering
   - Include crossing minimization: May find better orderings that avoid stacking

3. **Convergence**: What if new layout creates new stacking?
   - Iterate until stable (with max iterations)
   - Or: constraints should guarantee no new stacking

4. **API**: Automatic detection vs. explicit constraints?
   - Current: Automatic (detect and fix)
   - Could also expose: `g.AddSeparationConstraint(nodeA, nodeB, minDistance)`

### Implementation Plan

See plan file for detailed implementation steps. Key phases:
1. Analysis of Brandes-Köpf constraint integration points
2. Design constraint data structure
3. Implement constrained coordinate assignment
4. Replace nudge logic with re-layout trigger
5. Test convergence and performance

## Conclusion

The "spread stacked nodes" optimization addresses a gap in existing Sugiyama-style layout algorithms. While traditional algorithms optimize for edge straightness (causing stacking), this phase optimizes for **edge clarity** (preventing port-side chaos).

This is a novel contribution that could benefit any graph visualization with:
- Hub nodes receiving many edges
- Variable node sizes
- Port-based edge attachment

The implementation is straightforward, the trade-offs are clear, and the feature is opt-in for backwards compatibility.

## References

- Brandes, U., & Köpf, B. (2001). Fast and simple horizontal coordinate assignment.
- Gansner, E. R., et al. (1993). A technique for drawing directed graphs.
- Sugiyama, K., et al. (1981). Methods for visual understanding of hierarchical system structures.
- ELK Layered Algorithm: https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html
- MSAGL Source: layeredLayout.ts (StraightenEdgePaths)
