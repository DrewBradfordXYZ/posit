# Plan: Boundary-Based Side Selection (MSAGL Approach)

Add boundary-based side selection using MSAGL's geometric intersection approach. Instead of checking gaps between nodes to decide sides, compute where the edge path intersects each node's boundary rectangle. The intersection point implicitly determines both side and offset.

## Concept

**Center-based (current default):** Uses center-to-center direction to determine sides. Simple, works for most cases, but can produce odd results when nodes overlap.

**Boundary-based (opt-in):** Uses line-rectangle intersection:
1. Cast a line from source attachment area toward target attachment area
2. Find where line exits source node boundary → source side + offset
3. Find where line enters target node boundary → target side + offset
4. Side selection is implicit in the geometry — no gap/overlap heuristics

This is how MSAGL handles edge attachment: the `trimSplineAndCalculateArrowheads` function finds where the edge curve intersects the node's `boundaryCurve`, and that intersection determines the attachment point.

## Why MSAGL Approach

| Aspect | Gap-Based (Previous Plan) | Intersection-Based (MSAGL) |
|--------|---------------------------|---------------------------|
| Side selection | Explicit heuristics | Implicit in geometry |
| Overlap handling | Special case (same-side) | Natural (intersection picks correct side) |
| Edge direction | Uses node centers | Uses actual edge path |
| Asymmetric nodes | Treats all sides equally | Can use actual boundary shape |
| Port offsets | Computed separately | Derived from intersection Y/X |

## API Design

### Input: New Option

```go
// posit.go
type SideSelection int

const (
    SideFromCenter   SideSelection = iota  // Default (current behavior)
    SideFromBoundary                        // Use boundary intersection
)

type Options struct {
    // ...existing fields...
    SideSelection SideSelection
}
```

### Internal: Boundary Rectangle

Nodes already have X, Y, Width, Height. We derive the boundary rectangle:

```go
// boundary.go (new file)
type Rect struct {
    Left, Right, Top, Bottom float64
}

func nodeRect(n *layoutNode) Rect {
    return Rect{
        Left:   n.x,
        Right:  n.x + n.width,
        Top:    n.y,
        Bottom: n.y + n.height,
    }
}
```

### Core Algorithm: Line-Rectangle Intersection

```go
// boundary.go

// IntersectResult contains the intersection point and which side it's on.
type IntersectResult struct {
    Point  Point   // Intersection coordinates
    Side   Side    // Which side of the rectangle
    Offset float64 // Distance along that side from its start
}

// IntersectLineRect finds where a ray from `from` toward `to` exits rectangle `r`.
// Returns the intersection point, which side, and the offset along that side.
// If `from` is outside the rectangle, returns the entry point instead.
func IntersectLineRect(from, to Point, r Rect) IntersectResult {
    // Direction vector
    dx := to.X - from.X
    dy := to.Y - from.Y

    // Parametric line: P = from + t * (to - from)
    // Find t where line crosses each edge
    var tMin, tMax float64 = 0, 1
    var exitSide Side
    var exitT float64 = 1

    // Check each edge
    edges := []struct {
        side  Side
        coord float64  // edge position
        isMin bool     // is this the min or max edge for that axis
        axis  bool     // true = vertical edge (left/right), false = horizontal (top/bottom)
    }{
        {Left, r.Left, true, true},
        {Right, r.Right, false, true},
        {Top, r.Top, true, false},
        {Bottom, r.Bottom, false, false},
    }

    for _, edge := range edges {
        var t float64
        if edge.axis { // vertical edge
            if dx == 0 {
                continue // parallel to edge
            }
            t = (edge.coord - from.X) / dx
        } else { // horizontal edge
            if dy == 0 {
                continue
            }
            t = (edge.coord - from.Y) / dy
        }

        if t > 0 && t < exitT {
            // Check if intersection is within edge bounds
            p := Point{from.X + t*dx, from.Y + t*dy}
            if edge.axis {
                if p.Y >= r.Top && p.Y <= r.Bottom {
                    exitT = t
                    exitSide = edge.side
                }
            } else {
                if p.X >= r.Left && p.X <= r.Right {
                    exitT = t
                    exitSide = edge.side
                }
            }
        }
    }

    // Compute intersection point and offset
    point := Point{from.X + exitT*dx, from.Y + exitT*dy}
    var offset float64
    switch exitSide {
    case Left, Right:
        offset = point.Y - r.Top
    case Top, Bottom:
        offset = point.X - r.Left
    }

    return IntersectResult{Point: point, Side: exitSide, Offset: offset}
}
```

## Algorithm Integration

### For Edges with Ports (PortFixedOffset)

When `SideSelection == SideFromBoundary`:

```go
func (s *layoutState) edgePortSideBoundary(port *PortOptions, thisNode, connNode *layoutNode) Side {
    // Source point: port center on this node (using existing offset)
    var srcPoint Point
    switch port.Axis {
    case PortAxisHorizontal:
        // Port could be on left or right; use center Y
        srcPoint = Point{
            X: thisNode.x + thisNode.width/2,
            Y: thisNode.y + port.Offset,
        }
    case PortAxisVertical:
        srcPoint = Point{
            X: thisNode.x + port.Offset,
            Y: thisNode.y + thisNode.height/2,
        }
    default:
        srcPoint = Point{
            X: thisNode.x + thisNode.width/2,
            Y: thisNode.y + port.Offset,
        }
    }

    // Target point: connected node center
    tgtPoint := Point{
        X: connNode.x + connNode.width/2,
        Y: connNode.y + connNode.height/2,
    }

    // Find exit intersection
    result := IntersectLineRect(srcPoint, tgtPoint, nodeRect(thisNode))

    // Apply axis constraint
    switch port.Axis {
    case PortAxisHorizontal:
        if result.Side == Top || result.Side == Bottom {
            // Force to nearest horizontal side
            if srcPoint.X < tgtPoint.X {
                return Right
            }
            return Left
        }
    case PortAxisVertical:
        if result.Side == Left || result.Side == Right {
            if srcPoint.Y < tgtPoint.Y {
                return Bottom
            }
            return Top
        }
    }

    return result.Side
}
```

### For Edges without Ports

```go
func (s *layoutState) inferSideFromBoundary(fromNode, toNode *layoutNode) (sourceSide, targetSide Side) {
    fromCenter := Point{
        X: fromNode.x + fromNode.width/2,
        Y: fromNode.y + fromNode.height/2,
    }
    toCenter := Point{
        X: toNode.x + toNode.width/2,
        Y: toNode.y + toNode.height/2,
    }

    // Find where line from source center exits source boundary
    srcResult := IntersectLineRect(fromCenter, toCenter, nodeRect(fromNode))

    // Find where line from target center exits target boundary (toward source)
    tgtResult := IntersectLineRect(toCenter, fromCenter, nodeRect(toNode))

    return srcResult.Side, tgtResult.Side
}
```

### Port Assignment (assignFreeSides)

For `PortFree` and `PortFixedOffset` ports during initial assignment:

```go
func (s *layoutState) assignFreeSides(node *layoutNode) {
    nodeCX := node.x + node.width/2
    nodeCY := node.y + node.height/2

    for i := range node.ports {
        port := &node.ports[i]
        if port.Constraint != PortFree && port.Constraint != PortFixedOffset {
            continue
        }

        if s.opts.SideSelection == SideFromBoundary {
            // Use boundary intersection for each connected node, then pick majority
            port.Side = s.assignPortSideFromBoundary(node, port)
        } else {
            // Existing center-based logic
            dx, dy := s.portConnectedDirection(node.id, port.ID, nodeCX, nodeCY)
            dx, dy = s.internalToUserDirection(dx, dy)
            port.Side = s.bestSide(dx, dy, port.Axis)
        }
    }
}

func (s *layoutState) assignPortSideFromBoundary(node *layoutNode, port *PortOptions) Side {
    // Find all nodes connected to this port
    connectedNodes := s.getNodesConnectedToPort(node.id, port.ID)
    if len(connectedNodes) == 0 {
        return s.defaultSide(port.Axis)
    }

    // Vote: for each connected node, compute boundary intersection side
    votes := make(map[Side]int)
    portPoint := Point{
        X: node.x + node.width/2,
        Y: node.y + port.Offset, // For horizontal axis
    }
    if port.Axis == PortAxisVertical {
        portPoint = Point{X: node.x + port.Offset, Y: node.y + node.height/2}
    }

    for _, conn := range connectedNodes {
        connCenter := Point{X: conn.x + conn.width/2, Y: conn.y + conn.height/2}
        result := IntersectLineRect(portPoint, connCenter, nodeRect(node))

        // Apply axis constraint
        side := result.Side
        if port.Axis == PortAxisHorizontal && (side == Top || side == Bottom) {
            if portPoint.X < connCenter.X {
                side = Right
            } else {
                side = Left
            }
        } else if port.Axis == PortAxisVertical && (side == Left || side == Right) {
            if portPoint.Y < connCenter.Y {
                side = Bottom
            } else {
                side = Top
            }
        }
        votes[side]++
    }

    // Return side with most votes
    var bestSide Side
    var maxVotes int
    for side, count := range votes {
        if count > maxVotes {
            maxVotes = count
            bestSide = side
        }
    }
    return bestSide
}
```

## Files Modified

| File | Change |
|------|--------|
| `posit.go` | Add `SideSelection` type and Options field |
| `boundary.go` | **NEW**: `Rect`, `IntersectLineRect`, `IntersectResult` |
| `route.go` | Add `edgePortSideBoundary()`, modify `edgePortSide()` to check option |
| `route.go` | Add `inferSideFromBoundary()`, modify `inferSide()` wrapper |
| `port.go` | Add `assignPortSideFromBoundary()`, modify `assignFreeSides()` |
| `boundary_test.go` | **NEW**: Unit tests for intersection algorithm |
| `features_test.go` | Integration tests for boundary option |

## Tests

### Unit Tests (boundary_test.go)

```go
func TestIntersectLineRect_ExitRight(t *testing.T) {
    r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
    from := Point{50, 25}  // center
    to := Point{200, 25}   // directly right
    result := IntersectLineRect(from, to, r)
    assert(result.Side == Right)
    assert(result.Point.X == 100)
    assert(result.Offset == 25) // Y offset from top
}

func TestIntersectLineRect_ExitBottom(t *testing.T) {
    r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
    from := Point{50, 25}
    to := Point{50, 100}   // directly down
    result := IntersectLineRect(from, to, r)
    assert(result.Side == Bottom)
    assert(result.Point.Y == 50)
    assert(result.Offset == 50) // X offset from left
}

func TestIntersectLineRect_Diagonal(t *testing.T) {
    r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 50}
    from := Point{50, 25}
    to := Point{200, 100}  // diagonal right+down
    result := IntersectLineRect(from, to, r)
    // Should exit through right side (closer than bottom for this angle)
    assert(result.Side == Right)
}

func TestIntersectLineRect_OverlappingNodes(t *testing.T) {
    // When target is inside source bounds, intersection still works
    r := Rect{Left: 0, Right: 100, Top: 0, Bottom: 100}
    from := Point{50, 50}  // center of source
    to := Point{75, 50}    // inside source but to the right
    result := IntersectLineRect(from, to, r)
    assert(result.Side == Right)
}
```

### Integration Tests (features_test.go)

```go
func TestSideSelection_DefaultIsCenterBased(t *testing.T) {
    // Verify default behavior unchanged
}

func TestSideSelection_BoundaryOption_ClearGap(t *testing.T) {
    // Nodes with clear horizontal gap → opposite sides
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B")

    layout := g.Layout(Options{SideSelection: SideFromBoundary})
    edge := layout.Edges["A->B"]
    // A is left of B → A exits right, B enters left
    assert(edge.SourceSide == Right)
    assert(edge.TargetSide == Left)
}

func TestSideSelection_BoundaryOption_VerticalStack(t *testing.T) {
    // Nodes stacked vertically → top/bottom sides
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B")

    layout := g.Layout(Options{
        Direction:     TopToBottom,
        SideSelection: SideFromBoundary,
    })
    edge := layout.Edges["A->B"]
    assert(edge.SourceSide == Bottom)
    assert(edge.TargetSide == Top)
}

func TestSideSelection_BoundaryOption_Overlap(t *testing.T) {
    // When nodes overlap, intersection still produces valid sides
    // (geometry handles it naturally, no special case needed)
}

func TestSideSelection_BoundaryOption_WithPorts(t *testing.T) {
    // PortFixedOffset + boundary selection
    g := NewGraph()
    g.AddNode("A", NodeOptions{
        Width: 200, Height: 100,
        Ports: []PortOptions{
            {ID: "p1", Offset: 30, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
        },
    })
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "B", EdgeOptions{SourcePort: "p1"})

    layout := g.Layout(Options{SideSelection: SideFromBoundary})
    // Verify port side is determined by boundary intersection toward B
}
```

## Implementation Order

1. **Add boundary.go** with `Rect`, `Point`, `IntersectLineRect`
2. **Add unit tests** for intersection algorithm (verify geometry math)
3. **Add SideSelection option** to posit.go (zero-value = center-based)
4. **Modify inferSide()** to use `inferSideFromBoundary` when option set
5. **Modify edgePortSide()** to use `edgePortSideBoundary` when option set
6. **Modify assignFreeSides()** to use boundary-based assignment
7. **Add integration tests** for all scenarios
8. **Run existing tests** to verify backward compatibility

## Verification

1. `go test ./...` — all existing tests pass (backward compat)
2. `go test -run TestIntersect -v` — intersection algorithm correct
3. `go test -run TestSideSelection -v` — option works
4. Test with basetypes schema graph:
   - Nodes with clear gaps → opposite sides
   - Nodes vertically stacked → top/bottom
   - Dragged nodes → sides update correctly
5. Verify self-loops still work (from/to same node)

## Future Extensions

The intersection-based approach opens the door for:

1. **Non-rectangular boundaries**: Could support ellipses, rounded rectangles, or arbitrary polygons by parameterizing `IntersectLineRect` to `IntersectLineBoundary`

2. **Asymmetric anchors**: Like MSAGL's `leftAnchor != rightAnchor`, useful for nodes with labels on one side

3. **Edge trimming**: Instead of just computing sides, could trim edge paths at the actual boundary intersection (matching MSAGL's `trimEdgeSplineWithNodeBoundaries`)

4. **Port areas**: When ports have Width/Height, find intersection with the port rectangle instead of a point
