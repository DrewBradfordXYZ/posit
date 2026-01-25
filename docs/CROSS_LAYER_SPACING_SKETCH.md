# Cross-Layer Spacing Design Sketch

## Problem

Currently, posit uses fixed `RankSep` for vertical layer separation. Two nodes in adjacent layers can have overlapping X ranges, and if their combined heights exceed RankSep, they visually overlap.

```
Layer 0:    [  Node A (tall)  ]
                    ↓ RankSep = 100
Layer 1:        [  Node B  ]    ← X ranges overlap, could visually collide
```

## Goal

Add ELK-style `NodeNodeBetweenLayers` spacing that ensures minimum clearance between node **boundaries** across adjacent layers, not just layer centerlines.

## Proposed API

```go
type Options struct {
    // ... existing fields ...

    // NodeNodeBetweenLayers is the minimum spacing between node boundaries
    // in adjacent layers. If two nodes in layers i and i+1 have overlapping
    // X ranges, this spacing is enforced between their closest edges.
    // Default: 0 (disabled, use fixed RankSep only)
    NodeNodeBetweenLayers float64
}
```

## Algorithm

### Phase: Cross-Layer Overlap Resolution

Called after `assignCoordinates()`, before `routeEdges()`.

```go
// resolveCrossLayerOverlaps adjusts node positions to ensure minimum
// spacing between node boundaries in adjacent layers.
func (s *layoutState) resolveCrossLayerOverlaps() {
    if s.opts.NodeNodeBetweenLayers <= 0 {
        return // Disabled
    }

    // For each pair of adjacent layers
    for i := 0; i < len(s.layers)-1; i++ {
        upperLayer := s.layers[i]
        lowerLayer := s.layers[i+1]

        // Find overlapping pairs (nodes whose X ranges intersect)
        overlaps := s.findCrossLayerOverlaps(upperLayer, lowerLayer)

        // Resolve each overlap
        for _, overlap := range overlaps {
            s.resolveOverlap(overlap)
        }
    }
}
```

### Overlap Detection

Two nodes "overlap" in X if their horizontal ranges intersect:

```go
type crossLayerOverlap struct {
    upper    *layoutNode  // Node in upper layer
    lower    *layoutNode  // Node in lower layer
    xOverlap float64      // Amount of X-range overlap
    yGap     float64      // Current vertical gap between boundaries
    required float64      // Required gap (NodeNodeBetweenLayers)
}

func (s *layoutState) findCrossLayerOverlaps(upper, lower []*layoutNode) []crossLayerOverlap {
    var overlaps []crossLayerOverlap

    for _, u := range upper {
        uLeft := u.x
        uRight := u.x + u.width
        uBottom := u.y + u.height

        for _, l := range lower {
            lLeft := l.x
            lRight := l.x + l.width
            lTop := l.y

            // Check X-range intersection
            xOverlap := min(uRight, lRight) - max(uLeft, lLeft)
            if xOverlap <= 0 {
                continue // No horizontal overlap
            }

            // Calculate current vertical gap
            yGap := lTop - uBottom
            required := s.opts.NodeNodeBetweenLayers

            if yGap < required {
                overlaps = append(overlaps, crossLayerOverlap{
                    upper:    u,
                    lower:    l,
                    xOverlap: xOverlap,
                    yGap:     yGap,
                    required: required,
                })
            }
        }
    }

    return overlaps
}
```

### Overlap Resolution Strategies

When an overlap is detected, we have three options:

#### Strategy 1: Horizontal Shift (Preferred)

Shift one node horizontally to eliminate X overlap:

```go
func (s *layoutState) resolveByHorizontalShift(o crossLayerOverlap) bool {
    // Calculate minimum shift to eliminate X overlap
    // Try shifting the node with fewer connections first

    upperConnections := s.countConnections(o.upper)
    lowerConnections := s.countConnections(o.lower)

    if upperConnections <= lowerConnections {
        return s.tryShiftNode(o.upper, o.xOverlap)
    }
    return s.tryShiftNode(o.lower, o.xOverlap)
}

func (s *layoutState) tryShiftNode(n *layoutNode, minShift float64) bool {
    // Check if shifting right is possible (doesn't overlap same-layer neighbor)
    rightNeighbor := s.getRightNeighbor(n)
    if rightNeighbor != nil {
        availableRight := rightNeighbor.x - (n.x + n.width) - s.opts.NodeSep
        if availableRight >= minShift {
            n.x += minShift + 1 // +1 for clearance
            return true
        }
    }

    // Check if shifting left is possible
    leftNeighbor := s.getLeftNeighbor(n)
    if leftNeighbor != nil {
        availableLeft := n.x - (leftNeighbor.x + leftNeighbor.width) - s.opts.NodeSep
        if availableLeft >= minShift {
            n.x -= minShift + 1
            return true
        }
    }

    return false // Can't resolve horizontally
}
```

#### Strategy 2: Increase Layer Gap (Fallback)

If horizontal shift isn't possible, increase the gap between these two layers:

```go
func (s *layoutState) resolveByLayerGap(o crossLayerOverlap) {
    deficit := o.required - o.yGap

    // Shift all nodes in lower layer (and below) down
    lowerLayerIdx := o.lower.layer
    for _, node := range s.nodes {
        if node.layer >= lowerLayerIdx {
            node.y += deficit
        }
    }
}
```

#### Strategy 3: Hybrid Resolution

```go
func (s *layoutState) resolveOverlap(o crossLayerOverlap) {
    // First, try horizontal shift (cheaper, local)
    if s.resolveByHorizontalShift(o) {
        return
    }

    // Fallback: increase layer gap (affects more nodes)
    s.resolveByLayerGap(o)
}
```

## Integration Point

In `state.go`, add the call after coordinate assignment:

```go
func (s *layoutState) layout() *Layout {
    s.removeCycles()
    s.assignLayers()
    s.insertDummyNodes()
    s.minimizeCrossings()
    s.assignCoordinates()

    // NEW: Resolve cross-layer overlaps
    s.resolveCrossLayerOverlaps()

    s.computePortOffsets()
    s.routeEdges()
    return s.buildLayout()
}
```

## Edge Cases

### 1. Cascading Shifts

Shifting a node might create a new overlap with a different cross-layer neighbor. Solution: iterate until stable or max iterations.

```go
func (s *layoutState) resolveCrossLayerOverlaps() {
    for iteration := 0; iteration < 10; iteration++ {
        overlaps := s.findAllCrossLayerOverlaps()
        if len(overlaps) == 0 {
            break
        }
        for _, o := range overlaps {
            s.resolveOverlap(o)
        }
    }
}
```

### 2. Dummy Nodes

Dummy nodes (for long edges) should be excluded from cross-layer overlap checks - they're just edge routing points.

```go
if u.isDummy || l.isDummy {
    continue
}
```

### 3. Same-Side Ports

If two overlapping nodes have ports facing each other (like in schema diagrams), the edge needs that overlap. In this case, don't resolve - the edge routing handles it.

```go
// Check if there's a direct edge between these nodes
if s.hasDirectEdge(o.upper.id, o.lower.id) {
    continue // Let edge routing handle it
}
```

## Performance

- **O(n*m)** where n = nodes in layer i, m = nodes in layer i+1
- For sparse graphs: O(n) per layer pair
- Total: O(L * n²/L²) = O(n²/L) where L = number of layers
- For typical graphs with good layer distribution: effectively O(n log n)

Can be optimized with spatial indexing (interval trees) if needed.

## Testing

```go
func TestCrossLayerOverlapResolution(t *testing.T) {
    // Create two tall nodes that would overlap with default RankSep
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 150}) // Tall
    g.AddNode("B", NodeOptions{Width: 100, Height: 150}) // Tall
    g.AddEdge("A", "B")

    layout := g.Layout(Options{
        RankSep:               100,
        NodeNodeBetweenLayers: 20,
    })

    // Verify no overlap
    aBottom := layout.Nodes["A"].Y + layout.Nodes["A"].Height
    bTop := layout.Nodes["B"].Y

    gap := bTop - aBottom
    if gap < 20 {
        t.Errorf("Cross-layer gap = %.1f, want >= 20", gap)
    }
}

func TestCrossLayerNoOverlapWhenXSeparated(t *testing.T) {
    // Nodes with no X overlap shouldn't be affected
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 50, Height: 150})
    g.AddNode("B", NodeOptions{Width: 50, Height: 150})
    g.AddNode("C", NodeOptions{Width: 50, Height: 150})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")

    // B and C are in same layer, won't overlap with A if X-separated
    layout := g.Layout(Options{
        NodeSep:               100, // Wide separation
        RankSep:               50,  // Small vertical gap
        NodeNodeBetweenLayers: 20,
    })

    // B and C should remain at original Y (no adjustment needed)
    // because they're X-separated from A
}
```

## Open Questions

1. **Should this affect edge routing?** If we shift nodes, edges might need re-routing. Current plan: run before edge routing, so routing sees final positions.

2. **Priority for shifting:** Which node to shift when both could move? Current: node with fewer connections.

3. **Interaction with ports:** PortFixedOffset ports have specific Y positions. If we shift a node vertically (layer gap increase), port positions remain correct relative to node.

4. **Contract implications:** Does this affect any CONTRACT.md guarantees? Probably not - we're just adjusting coordinates, maintaining all invariants.
