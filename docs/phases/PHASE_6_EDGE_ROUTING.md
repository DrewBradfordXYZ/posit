# Phase 6: Edge Routing

**File:** `route.go`

## Table of Contents

- [Goal](#goal)
- [Process Overview](#process-overview)
- [Building Edge Paths](#building-edge-paths)
- [Restoring Reversed Edges](#restoring-reversed-edges)
- [Node Boundary Intersections](#node-boundary-intersections)
- [Implementation](#implementation)
- [Final Output Structure](#final-output-structure)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Generate the **final edge paths** as polylines, converting dummy node chains back into bend points and preparing the output for rendering.

### Input
- Positioned graph with coordinates (from Phase 5)
- Dummy nodes with their positions
- List of reversed edges and dummy chains

### Output
- `edge.points` arrays containing path coordinates
- Dummy nodes removed from output
- Reversed edges restored to original direction

---

## Process Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  STEP 1: Build Edge Paths from Dummy Chains                     │
│  ─────────────────────────────────────────                      │
│  Walk each dummy chain, collect (x, y) coordinates              │
│  Store as edge.points array                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 2: Handle Short Edges                                     │
│  ─────────────────────────                                      │
│  Edges without dummies get empty points array                   │
│  (straight line from source to target)                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 3: Restore Reversed Edges                                 │
│  ────────────────────────────                                   │
│  Flip points array for reversed edges                           │
│  Swap edge direction back to original                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 4: Add Node Boundary Points (Optional)                    │
│  ──────────────────────────────────────────                     │
│  Calculate where edge intersects source/target node boundaries  │
│  Prepend start point, append end point                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  STEP 5: Build Final Output                                     │
│  ──────────────────────────                                     │
│  Remove dummy nodes from node list                              │
│  Return Layout with Nodes and Edges maps                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Building Edge Paths

### Dummy Chain to Bend Points

Each dummy node becomes a bend point in the edge path:

```
BEFORE (with dummies):               AFTER (edge with points):

A ──→ [D1] ──→ [D2] ──→ B           A ────────────────→ B
                                         ╲    │    ╱
Node positions:                           ╲   │   ╱
  A: (100, 0)                              ╲  │  ╱
  D1: (120, 100)                         edge.points = [
  D2: (110, 200)                           {120, 100},  // D1 position
  B: (100, 300)                            {110, 200},  // D2 position
                                         ]
```

### Walking Dummy Chains

```go
for each dummyChain:
    firstDummy = dummyChains[i]
    edge = firstDummy.edgeLabel  // Original edge

    edge.points = []

    current = firstDummy
    while current is dummy:
        edge.points.append({x: current.x, y: current.y})
        current = successors[current][0]  // Next in chain

    // current is now the target node
```

---

## Restoring Reversed Edges

Edges reversed in Phase 1 must be flipped back:

### Before Restoration

```
Original: A → B (but was reversed to B → A)
Current state:
  edge.key = {from: "B", to: "A"}
  edge.reversed = true
  edge.points = [{x1, y1}, {x2, y2}]  // Points from B toward A
```

### After Restoration

```
Restored:
  edge.key = {from: "A", to: "B"}
  edge.reversed = false
  edge.points = [{x2, y2}, {x1, y1}]  // Points reversed, now A toward B
```

---

## Node Boundary Intersections

For a complete edge path, we need start and end points at the node boundaries:

```
Without boundary points:     With boundary points:

    ┌───────┐                   ┌───────┐
    │   A   │                   │   A   │
    │   *   │ ← edge starts     │   │   │
    └───────┘   from center     └───┼───┘ ← edge starts at boundary
        │                           │
        │                           │
        │                           │
    ┌───┼───┐                   ┌───┼───┐
    │   *   │ ← edge ends       │   │   │
    │   B   │   at center       │   B   │ ← edge ends at boundary
    └───────┘                   └───────┘
```

### Rectangle Intersection Algorithm

```go
// intersectRect finds where a line from rect center to external point
// crosses the rect boundary.
func intersectRect(node *layoutNode, point EdgePoint) EdgePoint {
    cx := node.x + node.width/2   // Center X
    cy := node.y + node.height/2  // Center Y
    w := node.width / 2
    h := node.height / 2

    dx := point.X - cx
    dy := point.Y - cy

    if dx == 0 && dy == 0 {
        return EdgePoint{X: cx, Y: cy}
    }

    var sx, sy float64

    if abs(dy)*w > abs(dx)*h {
        // Intersection at top or bottom
        if dy < 0 {
            h = -h
        }
        sx = h * dx / dy
        sy = h
    } else {
        // Intersection at left or right
        if dx < 0 {
            w = -w
        }
        sx = w
        sy = w * dy / dx
    }

    return EdgePoint{X: cx + sx, Y: cy + sy}
}
```

---

## Implementation

### Main Entry Point

```go
// routeEdges builds final edge paths and prepares output.
func (s *layoutState) routeEdges() {
    // Step 1: Build paths from dummy chains
    s.buildEdgePaths()

    // Step 2: Handle edges without dummies
    s.initializeShortEdges()

    // Step 3: Restore reversed edges
    s.restoreReversedEdges()

    // Step 4: Add node boundary points
    s.addNodeIntersections()
}
```

### Building Edge Paths

```go
// buildEdgePaths walks dummy chains to create edge bend points.
func (s *layoutState) buildEdgePaths() {
    for _, firstDummy := range s.dummyChains {
        dummy := s.nodes[firstDummy]
        if dummy == nil || dummy.edgeLabel == nil {
            continue
        }

        edge := dummy.edgeLabel
        edge.points = make([]EdgePoint, 0)

        // Walk the chain
        current := firstDummy
        for {
            node := s.nodes[current]
            if node == nil || !node.isDummy {
                break
            }

            // Add dummy's position as bend point
            // Use center of node (even though dummies have 0 size)
            edge.points = append(edge.points, EdgePoint{
                X: node.x + node.width/2,
                Y: node.y + node.height/2,
            })

            // Move to next in chain
            successors := s.successors[current]
            if len(successors) == 0 {
                break
            }
            current = successors[0]
        }
    }
}
```

### Initialize Short Edges

```go
// initializeShortEdges ensures edges without dummies have points arrays.
func (s *layoutState) initializeShortEdges() {
    for _, edge := range s.edges {
        if edge.points == nil {
            edge.points = make([]EdgePoint, 0)
        }
    }
}
```

### Restore Reversed Edges

```go
// restoreReversedEdges flips reversed edges back to original direction.
func (s *layoutState) restoreReversedEdges() {
    for _, key := range s.reversedEdges {
        edge := s.edges[key]
        if edge == nil {
            continue
        }

        // Reverse the points array
        for i, j := 0, len(edge.points)-1; i < j; i, j = i+1, j-1 {
            edge.points[i], edge.points[j] = edge.points[j], edge.points[i]
        }

        // Swap edge key back to original direction
        delete(s.edges, key)
        originalKey := edgeKey{from: key.to, to: key.from}
        edge.key = originalKey
        edge.reversed = false
        s.edges[originalKey] = edge
    }

    // Clear reversed edges list
    s.reversedEdges = nil
}
```

### Add Node Intersections

```go
// addNodeIntersections adds start/end points at node boundaries.
func (s *layoutState) addNodeIntersections() {
    for key, edge := range s.edges {
        fromNode := s.nodes[key.from]
        toNode := s.nodes[key.to]

        if fromNode == nil || toNode == nil {
            continue
        }

        // Calculate start point (intersection with source node)
        var firstTarget EdgePoint
        if len(edge.points) > 0 {
            firstTarget = edge.points[0]
        } else {
            // Straight edge: target is the destination node center
            firstTarget = EdgePoint{
                X: toNode.x + toNode.width/2,
                Y: toNode.y + toNode.height/2,
            }
        }
        startPoint := s.intersectRect(fromNode, firstTarget)

        // Calculate end point (intersection with target node)
        var lastSource EdgePoint
        if len(edge.points) > 0 {
            lastSource = edge.points[len(edge.points)-1]
        } else {
            // Straight edge: source is the source node center
            lastSource = EdgePoint{
                X: fromNode.x + fromNode.width/2,
                Y: fromNode.y + fromNode.height/2,
            }
        }
        endPoint := s.intersectRect(toNode, lastSource)

        // Build final points array
        finalPoints := make([]EdgePoint, 0, len(edge.points)+2)
        finalPoints = append(finalPoints, startPoint)
        finalPoints = append(finalPoints, edge.points...)
        finalPoints = append(finalPoints, endPoint)
        edge.points = finalPoints
    }
}

// intersectRect finds intersection of line from node center to point.
func (s *layoutState) intersectRect(node *layoutNode, point EdgePoint) EdgePoint {
    cx := node.x + node.width/2
    cy := node.y + node.height/2
    w := node.width / 2
    h := node.height / 2

    // Handle zero-size nodes (dummies)
    if w == 0 || h == 0 {
        return EdgePoint{X: cx, Y: cy}
    }

    dx := point.X - cx
    dy := point.Y - cy

    if dx == 0 && dy == 0 {
        return EdgePoint{X: cx, Y: cy}
    }

    var sx, sy float64

    if math.Abs(dy)*w > math.Abs(dx)*h {
        // Top or bottom intersection
        if dy < 0 {
            h = -h
        }
        sx = h * dx / dy
        sy = h
    } else {
        // Left or right intersection
        if dx < 0 {
            w = -w
        }
        sx = w
        sy = w * dy / dx
    }

    return EdgePoint{X: cx + sx, Y: cy + sy}
}
```

---

## Final Output Structure

### Building the Layout

```go
// buildLayout creates the final Layout output.
func (s *layoutState) buildLayout() *Layout {
    result := &Layout{
        Nodes: make(map[string]NodeLayout, len(s.nodes)),
        Edges: make(map[string]EdgeLayout, len(s.edges)),
    }

    // Copy node positions (skip dummy nodes)
    for id, node := range s.nodes {
        if node.isDummy {
            continue  // Don't expose dummies in output
        }

        result.Nodes[id] = NodeLayout{
            Position: Position{X: node.x, Y: node.y},
            Width:    node.width,
            Height:   node.height,
        }
    }

    // Copy edge routes
    for key, edge := range s.edges {
        edgeID := key.from + "->" + key.to
        result.Edges[edgeID] = EdgeLayout{
            Points: edge.points,
        }
    }

    return result
}
```

### Output Structures

```go
type Layout struct {
    Nodes map[string]NodeLayout  // Node ID -> position/size
    Edges map[string]EdgeLayout  // "from->to" -> path
}

type NodeLayout struct {
    Position          // X, Y (top-left corner)
    Width   float64
    Height  float64
}

type EdgeLayout struct {
    Points []EdgePoint  // Ordered bend points
}

type EdgePoint struct {
    X float64
    Y float64
}
```

### Example Output

```go
layout := g.Layout()

// Nodes:
// layout.Nodes["A"] = {X: 100, Y: 0, Width: 100, Height: 50}
// layout.Nodes["B"] = {X: 100, Y: 150, Width: 100, Height: 50}

// Edges:
// layout.Edges["A->B"] = {
//     Points: [
//         {X: 150, Y: 50},    // Start: bottom of A
//         {X: 150, Y: 150},   // End: top of B
//     ]
// }
```

---

## Complexity Analysis

| Operation | Time | Space |
|-----------|------|-------|
| Build paths from dummies | O(D) | O(D) |
| Initialize short edges | O(E) | O(1) |
| Restore reversed edges | O(R) | O(1) |
| Add intersections | O(E) | O(E) |
| Build output | O(V + E) | O(V + E) |
| **Total** | **O(V + E + D)** | **O(V + E)** |

Where: V = vertices, E = edges, D = dummy nodes, R = reversed edges

---

## Testing

### Test Cases

```go
func TestRoute_SimpleEdge(t *testing.T) {
    // A → B with no dummies
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")

    layout := g.Layout()

    edge, ok := layout.Edges["A->B"]
    if !ok {
        t.Fatal("Edge A->B not found")
    }

    // Should have at least start and end points
    if len(edge.Points) < 2 {
        t.Errorf("Expected at least 2 points, got %d", len(edge.Points))
    }

    // Start point should be below A
    aNode := layout.Nodes["A"]
    startY := edge.Points[0].Y
    if startY < aNode.Y+aNode.Height {
        t.Errorf("Start point Y=%v should be >= bottom of A (%v)",
            startY, aNode.Y+aNode.Height)
    }
}

func TestRoute_LongEdgeWithDummies(t *testing.T) {
    // A → B → C → D with long edge A → D
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("D", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("C", "D")
    g.AddEdge("A", "D")  // Long edge

    layout := g.Layout()

    edge, ok := layout.Edges["A->D"]
    if !ok {
        t.Fatal("Edge A->D not found")
    }

    // Long edge should have bend points (2 dummies + start + end = 4+ points)
    if len(edge.Points) < 4 {
        t.Errorf("Expected at least 4 points for long edge, got %d",
            len(edge.Points))
    }
}

func TestRoute_ReversedEdge(t *testing.T) {
    // A → B → C → A (cycle, one edge reversed)
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("C", "A")  // Will be reversed

    layout := g.Layout()

    // Original edge C→A should be present (restored)
    _, ok := layout.Edges["C->A"]
    if !ok {
        t.Error("Edge C->A not found (should be restored)")
    }
}

func TestRoute_NoDummiesInOutput(t *testing.T) {
    // Long edge creates dummies, but they shouldn't be in output
    g := NewGraph()
    for i := 0; i < 5; i++ {
        g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
    }
    for i := 0; i < 4; i++ {
        g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
    }
    g.AddEdge("N0", "N4")  // Long edge, creates dummies

    layout := g.Layout()

    // Only real nodes should be in output
    if len(layout.Nodes) != 5 {
        t.Errorf("Expected 5 nodes, got %d", len(layout.Nodes))
    }

    // Check no dummy IDs
    for id := range layout.Nodes {
        if strings.HasPrefix(id, "_dummy") {
            t.Errorf("Dummy node %s found in output", id)
        }
    }
}

func TestRoute_EdgePointsValid(t *testing.T) {
    g := buildRandomDAG(20, 30)
    layout := g.Layout()

    for edgeID, edge := range layout.Edges {
        for i, pt := range edge.Points {
            if math.IsNaN(pt.X) || math.IsInf(pt.X, 0) {
                t.Errorf("Edge %s point %d has invalid X: %v",
                    edgeID, i, pt.X)
            }
            if math.IsNaN(pt.Y) || math.IsInf(pt.Y, 0) {
                t.Errorf("Edge %s point %d has invalid Y: %v",
                    edgeID, i, pt.Y)
            }
        }
    }
}
```

---

## Visual Examples

### Example 1: Simple Edge (No Dummies)

```
INPUT:
  A (100, 0, 100×50)
  B (100, 150, 100×50)
  Edge: A → B

AFTER ROUTING:

    ┌─────────────┐
    │      A      │ (100, 0)
    │             │
    └──────┬──────┘
           │ ← Start point: (150, 50)
           │
           │
           │ ← End point: (150, 150)
    ┌──────┴──────┐
    │      B      │ (100, 150)
    │             │
    └─────────────┘

edge.Points = [
    {150, 50},   // Bottom center of A
    {150, 150},  // Top center of B
]
```

### Example 2: Long Edge with Dummies

```
INPUT (after Phase 5):

A (100, 0)
│
├── [D1] (120, 100)  ← dummy
│
├── [D2] (110, 200)  ← dummy
│
D (100, 300)

Original edge: A → D


AFTER ROUTING:

    ┌─────────────┐
    │      A      │
    └──────┬──────┘
           │ (150, 50) ← Start point
            \
             * (120, 100) ← Bend point (was D1)
            /
           * (110, 200) ← Bend point (was D2)
            \
             │ (150, 300) ← End point
    ┌────────┴────┐
    │      D      │
    └─────────────┘

edge.Points = [
    {150, 50},   // Start (A boundary)
    {120, 100},  // Bend (D1 position)
    {110, 200},  // Bend (D2 position)
    {150, 300},  // End (D boundary)
]
```

### Example 3: Reversed Edge

```
BEFORE (in Phase 1):
  Original: C → A
  Reversed to: A → C (to break cycle)

DURING LAYOUT:
  A at rank 0, C at rank 2
  Edge stored as A → C
  Dummies created for A → C

AFTER ROUTING:
  Edge restored to C → A
  Points array reversed

Original order:  A → D1 → D2 → C
                 [p1, p2, p3, p4]

Restored order:  C → D2 → D1 → A
                 [p4, p3, p2, p1]
```

---

## Post-Conditions

After this phase completes:

1. ✅ Every edge has a `points` array (may be empty for self-loops)
2. ✅ Points arrays include start and end boundary intersections
3. ✅ No dummy nodes in the output `Layout.Nodes`
4. ✅ All reversed edges restored to original direction
5. ✅ Edge IDs in output use format "from->to"

---

## Summary

This completes the 6-phase Sugiyama algorithm:

| Phase | File | Purpose | Output |
|-------|------|---------|--------|
| 1 | acyclic.go | Break cycles | DAG |
| 2 | rank.go | Assign layers | node.rank |
| 3 | normalize.go | Insert dummies | normalized graph |
| 4 | order.go | Minimize crossings | node.order |
| 5 | position.go | Assign coordinates | node.x, node.y |
| **6** | **route.go** | **Build edge paths** | **Layout output** |

The final `Layout` object contains all information needed to render the graph:
- Node positions and dimensions
- Edge paths as polylines

---

## Back to Overview

← [Phase 0: Overview](./PHASE_0_OVERVIEW.md)
