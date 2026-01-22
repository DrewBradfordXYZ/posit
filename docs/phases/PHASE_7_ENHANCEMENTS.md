# Phase 7: Enhancements

**Files:** `posit.go`, `direction.go`, `validate.go`

## Table of Contents

- [Goal](#goal)
- [Enhancement Overview](#enhancement-overview)
- [7.1 Input Validation](#71-input-validation)
- [7.2 Direction Support](#72-direction-support)
- [7.3 Graph Query API](#73-graph-query-api)
- [7.4 Duplicate Edge Handling](#74-duplicate-edge-handling)
- [Implementation Order](#implementation-order)
- [Testing](#testing)

---

## Goal

Improve correctness, usability, and feature parity with dagre through targeted enhancements to the public API and input handling.

### Non-Goals (Future Work)

- Network simplex ranking algorithm
- Edge label support
- Compound graph (subgraph) support
- Self-loop rendering

---

## Enhancement Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  7.1 INPUT VALIDATION                                           │
│  ─────────────────────                                          │
│  File: posit.go (modify AddEdge)                                │
│  Purpose: Prevent nil panics from invalid edge references       │
│  Scope: AddEdge validates node existence                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  7.2 DIRECTION SUPPORT                                          │
│  ─────────────────────                                          │
│  File: direction.go (new)                                       │
│  Purpose: Support TB, BT, LR, RL layout directions              │
│  Method: Coordinate transforms before/after core layout         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  7.3 GRAPH QUERY API                                            │
│  ───────────────────                                            │
│  File: posit.go (add methods)                                   │
│  Purpose: Allow inspection of graph structure                   │
│  Methods: Nodes(), Edges(), HasNode(), HasEdge()                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  7.4 DUPLICATE EDGE HANDLING                                    │
│  ───────────────────────────                                    │
│  File: state.go (modify newLayoutState)                         │
│  Purpose: Aggregate duplicate edges instead of overwriting      │
│  Method: Sum weights, take max minlen                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 7.1 Input Validation

### Problem

Currently, `AddEdge` accepts any node IDs without validation:

```go
// Current: No validation
func (g *Graph) AddEdge(from, to string) {
    g.edges = append(g.edges, &edge{from: from, to: to})
}

// Usage that causes panic later:
g := NewGraph()
g.AddNode("A", NodeOptions{Width: 100, Height: 50})
g.AddEdge("A", "B")  // "B" doesn't exist!
g.Layout()           // Nil pointer panic in phase processing
```

### Solution

Add validation with clear error reporting:

```go
// AddEdge adds a directed edge from source to target.
// Returns an error if either node does not exist.
func (g *Graph) AddEdge(from, to string) error {
    if _, ok := g.nodes[from]; !ok {
        return fmt.Errorf("posit: source node %q does not exist", from)
    }
    if _, ok := g.nodes[to]; !ok {
        return fmt.Errorf("posit: target node %q does not exist", to)
    }
    g.edges = append(g.edges, &edge{from: from, to: to})
    return nil
}
```

### Alternative: MustAddEdge

For convenience, also provide a panicking version:

```go
// MustAddEdge adds an edge or panics if nodes don't exist.
func (g *Graph) MustAddEdge(from, to string) {
    if err := g.AddEdge(from, to); err != nil {
        panic(err)
    }
}
```

### Migration Path

To maintain backward compatibility initially:

```go
// AddEdge adds a directed edge. Panics if nodes don't exist.
// Deprecated: Use AddEdgeChecked for error handling.
func (g *Graph) AddEdge(from, to string) {
    if err := g.AddEdgeChecked(from, to); err != nil {
        panic(err)
    }
}

// AddEdgeChecked adds an edge with error checking.
func (g *Graph) AddEdgeChecked(from, to string) error {
    // ... validation logic
}
```

---

## 7.2 Direction Support

### Problem

The `Direction` type is defined but not implemented:

```go
// Declared but unused
type Direction int

const (
    TopToBottom Direction = iota
    LeftToRight
    BottomToTop
    RightToLeft
)
```

### Solution: Coordinate Transforms

Following dagre's elegant approach, perform layout in `TopToBottom` orientation, then transform coordinates at the end.

### New File: direction.go

```go
package posit

// adjustForDirection transforms the graph before layout if needed.
// For LR/RL directions, swaps width/height so layout treats X as Y.
func (s *layoutState) adjustForDirection() {
    switch s.opts.Direction {
    case LeftToRight, RightToLeft:
        // Swap width and height for all nodes
        for _, node := range s.nodes {
            node.width, node.height = node.height, node.width
        }
    }
}

// undoDirectionAdjustment transforms coordinates after layout.
func (s *layoutState) undoDirectionAdjustment() {
    switch s.opts.Direction {
    case BottomToTop:
        s.reverseY()
    case LeftToRight:
        s.swapXY()
        s.swapWidthHeight()
    case RightToLeft:
        s.reverseY()
        s.swapXY()
        s.swapWidthHeight()
    }
}

// reverseY flips Y coordinates (for BT and RL).
func (s *layoutState) reverseY() {
    // Find max Y
    maxY := 0.0
    for _, node := range s.nodes {
        bottom := node.y + node.height
        if bottom > maxY {
            maxY = bottom
        }
    }

    // Flip all Y coordinates
    for _, node := range s.nodes {
        node.y = maxY - node.y - node.height
    }

    // Flip edge points
    for _, edge := range s.edges {
        for i := range edge.points {
            edge.points[i].Y = maxY - edge.points[i].Y
        }
    }
}

// swapXY exchanges X and Y coordinates (for LR and RL).
func (s *layoutState) swapXY() {
    for _, node := range s.nodes {
        node.x, node.y = node.y, node.x
    }

    for _, edge := range s.edges {
        for i := range edge.points {
            edge.points[i].X, edge.points[i].Y = edge.points[i].Y, edge.points[i].X
        }
    }
}

// swapWidthHeight exchanges width and height (for LR and RL).
func (s *layoutState) swapWidthHeight() {
    for _, node := range s.nodes {
        node.width, node.height = node.height, node.width
    }
}
```

### Integration in Layout Pipeline

Modify `posit.go`:

```go
func (g *Graph) Layout(opts ...Options) *Layout {
    opt := DefaultOptions()
    if len(opts) > 0 {
        opt = opts[0]
    }

    state := newLayoutState(g, opt)

    // Pre-transform for direction
    state.adjustForDirection()

    // Core layout phases (always in TB orientation)
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()
    state.minimizeCrossings()
    state.assignCoordinates()
    state.routeEdges()

    // Post-transform for direction
    state.undoDirectionAdjustment()

    return state.buildLayout()
}
```

### Visual Examples

```
TopToBottom (default):     LeftToRight:

    ┌───┐                  ┌───┐
    │ A │                  │ A │───┐
    └─┬─┘                  └───┘   │
      │                            ▼
      ▼                          ┌───┐
    ┌───┐                        │ B │
    │ B │                        └───┘
    └───┘


BottomToTop:               RightToLeft:

    ┌───┐                        ┌───┐
    │ B │                  ┌─────│ A │
    └─┬─┘                  │     └───┘
      │                    ▼
      ▼                  ┌───┐
    ┌───┐                │ B │
    │ A │                └───┘
    └───┘
```

---

## 7.3 Graph Query API

### Problem

Users cannot inspect the graph after construction:

```go
g := NewGraph()
g.AddNode("A", NodeOptions{})
g.AddNode("B", NodeOptions{})
g.AddEdge("A", "B")

// Cannot do:
// - List all nodes
// - Check if a node exists
// - List all edges
// - Check if an edge exists
```

### Solution: Add Query Methods

```go
// Nodes returns a slice of all node IDs.
func (g *Graph) Nodes() []string {
    ids := make([]string, 0, len(g.nodes))
    for id := range g.nodes {
        ids = append(ids, id)
    }
    sort.Strings(ids) // Deterministic order
    return ids
}

// Edges returns a slice of all edges as [from, to] pairs.
func (g *Graph) Edges() [][2]string {
    result := make([][2]string, len(g.edges))
    for i, e := range g.edges {
        result[i] = [2]string{e.from, e.to}
    }
    return result
}

// HasNode returns true if a node with the given ID exists.
func (g *Graph) HasNode(id string) bool {
    _, ok := g.nodes[id]
    return ok
}

// HasEdge returns true if an edge from source to target exists.
func (g *Graph) HasEdge(from, to string) bool {
    for _, e := range g.edges {
        if e.from == from && e.to == to {
            return true
        }
    }
    return false
}

// Node returns the dimensions of a node, or false if not found.
func (g *Graph) Node(id string) (NodeOptions, bool) {
    n, ok := g.nodes[id]
    if !ok {
        return NodeOptions{}, false
    }
    return NodeOptions{Width: n.width, Height: n.height}, true
}
```

### Usage Example

```go
g := NewGraph()
g.AddNode("A", NodeOptions{Width: 100, Height: 50})
g.AddNode("B", NodeOptions{Width: 100, Height: 50})
g.AddEdge("A", "B")

fmt.Println(g.Nodes())        // [A B]
fmt.Println(g.Edges())        // [[A B]]
fmt.Println(g.HasNode("A"))   // true
fmt.Println(g.HasNode("C"))   // false
fmt.Println(g.HasEdge("A", "B")) // true
fmt.Println(g.HasEdge("B", "A")) // false
```

---

## 7.4 Duplicate Edge Handling

### Problem

Adding the same edge twice silently overwrites:

```go
g.AddEdge("A", "B")
g.AddEdge("A", "B")  // Silently overwrites, adjacency list has duplicate
```

### Solution: Aggregate Duplicates

Modify `newLayoutState` to aggregate duplicate edges:

```go
func newLayoutState(g *Graph, opts Options) *layoutState {
    s := &layoutState{
        // ... initialization
    }

    // Copy nodes
    for id, n := range g.nodes {
        s.nodes[id] = &layoutNode{
            id:     id,
            width:  n.width,
            height: n.height,
        }
        s.successors[id] = []string{}
        s.predecessors[id] = []string{}
    }

    // Copy edges with aggregation
    edgeSeen := make(map[edgeKey]bool)
    for _, e := range g.edges {
        key := edgeKey{from: e.from, to: e.to}

        if existing := s.edges[key]; existing != nil {
            // Aggregate: sum weights (future), keep edge
            existing.weight += 1
            continue
        }

        s.edges[key] = &layoutEdge{
            key:    key,
            weight: 1,
            minlen: 1,
        }

        // Only add to adjacency lists once
        if !edgeSeen[key] {
            s.successors[e.from] = append(s.successors[e.from], e.to)
            s.predecessors[e.to] = append(s.predecessors[e.to], e.from)
            edgeSeen[key] = true
        }
    }

    return s
}
```

### Behavior

```go
g.AddEdge("A", "B")  // weight = 1
g.AddEdge("A", "B")  // weight = 2 (aggregated)
g.AddEdge("A", "B")  // weight = 3 (aggregated)

// During layout, higher weight edges have more influence
// on barycenter calculations and crossing minimization
```

---

## Implementation Order

### Phase 7.1: Input Validation (Recommended First)

1. Add `HasNode` method to `Graph`
2. Modify `AddEdge` to validate and return error
3. Add `MustAddEdge` convenience method
4. Update tests

**Estimated scope:** ~30 lines, 5 tests

### Phase 7.2: Direction Support

1. Create `direction.go` with transform functions
2. Modify `Layout` to call transforms
3. Add direction tests for all 4 orientations

**Estimated scope:** ~100 lines, 8 tests

### Phase 7.3: Graph Query API

1. Add `Nodes()`, `Edges()` methods
2. Add `HasEdge()`, `Node()` methods
3. Add query tests

**Estimated scope:** ~50 lines, 6 tests

### Phase 7.4: Duplicate Edge Handling

1. Modify `newLayoutState` for aggregation
2. Add duplicate edge tests
3. Verify weight affects layout

**Estimated scope:** ~20 lines, 4 tests

---

## Testing

### 7.1 Validation Tests

```go
func TestAddEdge_MissingSource(t *testing.T) {
    g := NewGraph()
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})

    err := g.AddEdge("A", "B")
    if err == nil {
        t.Error("Expected error for missing source node")
    }
}

func TestAddEdge_MissingTarget(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})

    err := g.AddEdge("A", "B")
    if err == nil {
        t.Error("Expected error for missing target node")
    }
}

func TestAddEdge_Valid(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})

    err := g.AddEdge("A", "B")
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
}
```

### 7.2 Direction Tests

```go
func TestDirection_LeftToRight(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")

    layout := g.Layout(Options{Direction: LeftToRight})

    aNode := layout.Nodes["A"]
    bNode := layout.Nodes["B"]

    // In LR layout, B should be to the right of A
    if bNode.X <= aNode.X {
        t.Errorf("Expected B.X > A.X for LR layout, got A.X=%v, B.X=%v",
            aNode.X, bNode.X)
    }

    // Y coordinates should be similar (same "rank" = same horizontal level)
    if math.Abs(aNode.Y-bNode.Y) > 1 {
        t.Errorf("Expected similar Y for LR layout, got A.Y=%v, B.Y=%v",
            aNode.Y, bNode.Y)
    }
}

func TestDirection_BottomToTop(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")

    layout := g.Layout(Options{Direction: BottomToTop})

    aNode := layout.Nodes["A"]
    bNode := layout.Nodes["B"]

    // In BT layout, A (source) should be below B (target)
    if aNode.Y <= bNode.Y {
        t.Errorf("Expected A.Y > B.Y for BT layout, got A.Y=%v, B.Y=%v",
            aNode.Y, bNode.Y)
    }
}
```

### 7.3 Query API Tests

```go
func TestGraph_Nodes(t *testing.T) {
    g := NewGraph()
    g.AddNode("C", NodeOptions{})
    g.AddNode("A", NodeOptions{})
    g.AddNode("B", NodeOptions{})

    nodes := g.Nodes()
    expected := []string{"A", "B", "C"}

    if !reflect.DeepEqual(nodes, expected) {
        t.Errorf("Nodes() = %v, want %v", nodes, expected)
    }
}

func TestGraph_HasNode(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{})

    if !g.HasNode("A") {
        t.Error("HasNode(A) = false, want true")
    }
    if g.HasNode("B") {
        t.Error("HasNode(B) = true, want false")
    }
}

func TestGraph_HasEdge(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{})
    g.AddNode("B", NodeOptions{})
    g.AddEdge("A", "B")

    if !g.HasEdge("A", "B") {
        t.Error("HasEdge(A,B) = false, want true")
    }
    if g.HasEdge("B", "A") {
        t.Error("HasEdge(B,A) = true, want false")
    }
}
```

### 7.4 Duplicate Edge Tests

```go
func TestDuplicateEdges_Aggregated(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "B")
    g.AddEdge("A", "B")

    // Should not panic
    layout := g.Layout()

    // Should have exactly one edge in output
    if len(layout.Edges) != 1 {
        t.Errorf("Expected 1 edge, got %d", len(layout.Edges))
    }

    if _, ok := layout.Edges["A->B"]; !ok {
        t.Error("Expected edge A->B in output")
    }
}
```

---

## Complexity Analysis

| Enhancement | Time Impact | Space Impact |
|-------------|-------------|--------------|
| Input validation | O(1) per AddEdge | O(1) |
| Direction transforms | O(V + E) | O(1) |
| Query API | O(V) or O(E) | O(V) or O(E) for results |
| Duplicate handling | O(E) | O(E) for seen map |

---

## Summary

Phase 7 focuses on hardening the API and adding expected features:

| Enhancement | Priority | Impact |
|-------------|----------|--------|
| 7.1 Input Validation | High | Prevents crashes |
| 7.2 Direction Support | High | Completes declared API |
| 7.3 Graph Query API | Medium | Improves usability |
| 7.4 Duplicate Edges | Medium | Fixes silent data loss |

After Phase 7, the library will be production-ready for its core use case of hierarchical graph layout.

---

## Future Phases

Potential Phase 8+ enhancements (not in scope):

- **8.1** Network simplex ranking (optimal layer assignment)
- **8.2** Edge label support (labels on edges)
- **8.3** Self-loop rendering (curved return paths)
- **8.4** Compound graphs (nested subgraphs)
- **8.5** Debug timing option

---

## Back to Overview

← [Phase 0: Overview](./PHASE_0_OVERVIEW.md)
