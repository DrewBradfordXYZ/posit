# Posit Roadmap

This roadmap is **complete** — all planned improvements have been implemented. The sections below document the design rationale and API for each feature.

## Design Advantage

Posit runs as a single computation over the entire graph. Unlike client-side renderers that process each edge independently, Posit sees **all nodes and all edges simultaneously**. This global knowledge enables optimizations that per-edge rendering cannot achieve:

- **Edge routing that avoids nodes** — knows where every node is
- **Edge spacing that prevents overlaps** — knows where every other edge is
- **Port positions resolved to coordinates** — knows node dimensions and field order
- **Optimal attachment sides** — knows relative positions of all connected nodes
- **Crossing-aware channel assignment** — knows the full routing topology

Each improvement below leverages this global knowledge. The library computes what requires seeing the full picture; consumers handle aesthetics (curve rendering, colors, interaction).

## Implemented Features

- **Cycle removal:** DFS-based and Greedy FAS (Eades/Lin/Smyth)
- **Ranking:** LongestPath, TightTree, NetworkSimplex
- **Rank constraints:** Pin nodes to first/last layer, group to same layer
- **Crossing minimization:** Barycenter heuristic with layer sweeps, adjacent exchange, port-level optimization
- **Order constraints:** Group nodes adjacently, control priority within groups
- **Coordinate assignment:** Brandes-Köpf (optimal) with simple fallback for large graphs
- **Edge routing:** Polyline and orthogonal (channel-routed) with parallel edge spacing
- **Ports:** Five constraint modes (FixedPos, FixedSide, FixedOrder, Free, FixedOffset) with axis constraints
- **Edge labels:** Automatic positioning via label dummy nodes (left/center/right placement)
- **Edge weight:** Priority-based layout (crossing minimization, cycle removal)
- **Side inference:** Computed SourceSide/TargetSide on all edges
- **Directions:** TopToBottom, LeftToRight, BottomToTop, RightToLeft
- **Multi-edge:** Distinct parallel edges with perpendicular offset spacing
- **Self-loops:** Curved path rendering
- **Disconnected components:** Horizontal/vertical packing with configurable gap
- **Incremental layout:** Minimal update from prior layout (preserves mental map)
- **Compound graphs:** Nested clusters with padding, sized to contain children

## Non-Goals

Posit is a layered (Sugiyama) layout library. The following are out of scope:

- Force-directed / spring-embedded layouts
- Radial or circular layouts
- Treemaps or space-filling layouts
- 3D layout
- Interactive/animated layout (consumers handle this)
- Graph editing or manipulation beyond layout
- Rendering (SVG, Canvas, HTML) — consumers choose their rendering stack
- Spline/bézier curve generation — consumers post-process polyline points into curves as needed for their rendering library

---

## 1. Rank Constraints ✅

### Concept

Allow consumers to constrain which layer a node is assigned to. This is one of the most commonly needed features in layered graph layout (dagre's `rank: "min"`, `rank: "max"`, `rank: "same"`).

### Use Cases

- Force all "input" nodes to the top layer
- Force all "output" nodes to the bottom layer
- Keep related nodes on the same layer regardless of edge structure
- Pin a specific node to a rank for visual anchoring

### API Design

```go
type RankConstraint int

const (
    RankUnconstrained RankConstraint = iota
    RankMin  // Force to first (top) layer
    RankMax  // Force to last (bottom) layer
)

type NodeOptions struct {
    Width          float64
    Height         float64
    RankConstraint RankConstraint // Pin to min/max layer
    RankGroup      string         // Nodes with same group share a layer
}
```

### Implementation

Rank constraints modify the rank assignment phase:

```go
func (s *layoutState) applyRankConstraints() {
    // 1. Nodes with RankMin get rank 0
    // 2. Nodes with RankMax get the maximum rank
    // 3. Nodes in the same RankGroup are assigned the same rank
    //    (use the maximum rank among group members to satisfy edge constraints)
}
```

This runs after initial rank assignment and before normalization. For NetworkSimplex, constraints are encoded as zero-length edges between group members.

---

## 2. Port Support ✅

### Concept

Ports are fixed connection points at specific positions on a node. Without ports, edges connect to node boundaries using rectangle intersection. With ports, edges connect to precise, named locations.

### Port Constraint Modes

| Constraint | Side | Offset | Use Case |
|---|---|---|---|
| `PortFixedPos` | Fixed | Fixed | Exact pixel positions (default) |
| `PortFixedSide` | Fixed | Computed | Keep on declared side, optimize offset |
| `PortFixedOrder` | Fixed | Computed (ordered) | Preserve relative order, even spacing |
| `PortFree` | Computed | Computed | Algorithm chooses both side and offset |
| `PortFixedOffset` | Computed | Fixed | Fixed offset, algorithm chooses side |

### Axis Constraints

`PortFree` and `PortFixedOffset` support axis constraints to restrict which sides are considered:

- `PortAxisAny` — any side (default)
- `PortAxisHorizontal` — Left or Right only
- `PortAxisVertical` — Top or Bottom only

### API

```go
type PortConstraint int

const (
    PortFixedPos    PortConstraint = iota
    PortFixedSide
    PortFixedOrder
    PortFree
    PortFixedOffset
)

type PortAxis int

const (
    PortAxisAny PortAxis = iota
    PortAxisHorizontal
    PortAxisVertical
)

type PortOptions struct {
    ID         string
    Side       Side
    Offset     float64
    Order      int            // For PortFixedOrder
    Constraint PortConstraint // Default: PortFixedPos
    Axis       PortAxis       // For PortFree/PortFixedOffset
    Width      float64
    Height     float64
}
```

### Output

Nodes with computed ports include a `Ports` map in the layout output:

```go
type PortLayout struct {
    ID     string
    Side   Side
    Offset float64
}

type NodeLayout struct {
    Position
    Width  float64
    Height float64
    Ports  map[string]PortLayout // Non-nil for nodes with computed ports
}
```

### Edge Routing with Ports

When an edge specifies ports, routing uses the port's absolute position on the node border:

```go
// Right side: (node.x + node.width, node.y + offset)
// Left side:  (node.x, node.y + offset)
// Bottom:     (node.x + offset, node.y + node.height)
// Top:        (node.x + offset, node.y)
```

Without ports, edges fall back to boundary intersection.

### Schema Diagram Pattern

The primary use case for `PortFixedOffset` is schema diagrams where table nodes have field rows at fixed Y positions:

```go
g.AddNode("users", posit.NodeOptions{
    Width: 200, Height: 100,
    Ports: []posit.PortOptions{
        {ID: "fk-orders", Offset: 34, Constraint: posit.PortFixedOffset, Axis: posit.PortAxisHorizontal},
        {ID: "fk-profiles", Offset: 54, Constraint: posit.PortFixedOffset, Axis: posit.PortAxisHorizontal},
    },
})
```

Each port uses its declared Y offset for layout decisions (approximate row position) while Posit selects Left or Right based on where the connected node is positioned. The client measures actual handle positions from the DOM for pixel-precise edge rendering.

---

## 3. Edge Attachment Side Inference ✅

### Concept

After layout, Posit knows the relative positions of all nodes. It can compute the optimal side (left, right, top, bottom) for each edge endpoint without the consumer needing to infer this at render time.

### Approach

```go
func inferSide(fromNode, toNode *layoutNode) (sourceSide, targetSide Side) {
    dx := (toNode.x + toNode.width/2) - (fromNode.x + fromNode.width/2)
    dy := (toNode.y + toNode.height/2) - (fromNode.y + fromNode.height/2)

    if math.Abs(dx) > math.Abs(dy) {
        if dx > 0 {
            return Right, Left
        }
        return Left, Right
    }
    if dy > 0 {
        return Bottom, Top
    }
    return Top, Bottom
}
```

The `SourceSide` and `TargetSide` fields in `EdgeLayout` provide this directly. Consumers render edges without position inference logic.

### Port Interaction

When ports are specified, the port's `Side` field takes precedence over inferred sides. Side inference is the fallback for edges without port constraints.

---

## 4. Orthogonal Edge Routing ✅

### Concept

Orthogonal routing assigns edges to horizontal and vertical channels between node columns/rows. The primary value is not aesthetic ("right angles look cleaner") — it's algorithmic:

- **Edge-edge overlap prevention** — edges in the same channel are spaced apart (requires knowing all edges)
- **Node avoidance** — edges route around intermediate nodes (requires knowing all node positions)
- **Bend minimization** — choose paths with fewest turns (requires global path optimization)

A client rendering edges independently cannot achieve this. It doesn't know what other edges exist or where intermediate nodes are. This is a global optimization that leverages Posit's full-graph knowledge.

### API Design

```go
type RouteStyle int

const (
    RoutePolyline   RouteStyle = iota // Current behavior (default)
    RouteOrthogonal                   // Channel-routed H/V segments
)

type Options struct {
    // ... existing options ...
    RouteStyle  RouteStyle
    ChannelGap  float64 // Spacing between parallel edges in a channel (default: 10)
}
```

`RouteStyle` is an algorithm choice (like `Algorithm: NetworkSimplex`), not an aesthetic opinion. Both options produce waypoints; the consumer renders them however they want (straight lines, curves, smoothstep).

### Implementation Strategy

1. Determine exit direction from source port/side
2. Assign each edge to a routing channel (vertical corridor between node columns)
3. Space edges within the same channel using `ChannelGap`
4. Route: horizontal → channel → vertical → channel → horizontal to target
5. Minimize bends by choosing the shortest-path channel assignment

### Algorithm Choice

Tamassia's algorithm (1987) solves optimal bend minimization for general orthogonal drawing via min-cost network flow. It's not appropriate here — it assumes a planar embedding and ignores layer structure.

For layered graphs, **channel routing** is the correct algorithm. Posit already knows node layers and X positions, providing natural routing channels between node columns. Channel routing exploits this structure directly:

- Use spaces between node columns as vertical channels
- Use spaces between layers as horizontal channels
- Assign edges to channels with offset spacing to prevent overlaps
- Route around nodes that fall within the channel

The channel assignment is the key server-side advantage — it requires seeing all edges simultaneously to space them correctly.

---

## 5. Multi-Edge Support ✅

### Concept

Currently Posit aggregates duplicate edges between the same node pair (summing weights). State machines, protocol diagrams, and multi-relationship schemas need multiple distinct edges rendered as separate paths with slight offsets.

### API Design

```go
type EdgeOptions struct {
    // ... existing options ...
    ID string // Optional: distinguish multiple edges between same nodes
}

// Multiple edges between same pair
g.AddEdge("A", "B", posit.EdgeOptions{ID: "transition-1", LabelWidth: 60, LabelHeight: 20})
g.AddEdge("A", "B", posit.EdgeOptions{ID: "transition-2", LabelWidth: 60, LabelHeight: 20})
```

### Rendering

Parallel edges are offset perpendicular to their direction:

```go
const parallelEdgeSpacing = 10.0

func (s *layoutState) offsetParallelEdges() {
    // Group edges by (from, to) pair
    // For each group with n > 1 edges:
    //   Offset each edge by (i - (n-1)/2) * parallelEdgeSpacing
    //   perpendicular to the edge direction
}
```

### Output

Edge keys become `"from->to:id"` when ID is specified, otherwise `"from->to"` (backward compatible).

---

## 6. Ordering Constraints ✅

### Concept

Force specific nodes to appear in a given order within their layer, regardless of what barycenter ordering would produce. This is simpler than compound graphs but achieves similar visual grouping.

### API Design

```go
type NodeOptions struct {
    // ... existing options ...
    OrderGroup    string // Nodes in same group are placed adjacently
    OrderPriority int    // Within a group, lower priority = further left
}
```

### Implementation

Add constraints to the barycenter sort in Phase 4:

```go
func (s *layoutState) orderLayerWithConstraints(layer []string) {
    // 1. Compute barycenters as normal
    // 2. Sort by: (OrderGroup, OrderPriority, barycenter)
    // Nodes in the same group cluster together,
    // ordered by priority within the group
}
```

### Use Cases

- Keep related tables adjacent without full cluster support
- Logical grouping in dependency graphs ("all auth nodes together")
- Force specific left-to-right ordering for readability

---

## 7. Port-Level Crossing Minimization ✅

### Concept

When a node has multiple ports on the same side, edges connecting to those ports may cross unnecessarily. Standard crossing minimization operates on nodes; this extends it to consider port ordering within a node.

### Approach

After the standard barycenter node ordering (Phase 4), run an additional pass that considers port positions as sub-ordering weights:

```go
func (s *layoutState) minimizePortCrossings() {
    for _, node := range s.nodes {
        if len(node.ports) <= 1 {
            continue
        }
        // For edges connecting to ports on the same side,
        // use port offsets as weights in the barycenter calculation
        // so connected nodes are placed at heights matching port positions
    }
}
```

---

## 8. Edge Weight in Public API ✅

### Concept

Posit already uses edge weights internally for crossing minimization and ranking. Exposing weight in the public `EdgeOptions` API lets consumers influence layout priority.

Heavier edges are prioritized: they're less likely to be reversed during cycle removal, and crossing minimization favors keeping them uncrossed.

### API Addition

```go
type EdgeOptions struct {
    Weight float64 // Layout priority (default: 1.0, higher = more important)
    // ... existing label options ...
}
```

### Use Cases

- Primary relationships weighted higher than secondary ones
- Critical path edges in dependency graphs
- "Strong" vs "weak" associations in domain models

---

## 9. Disconnected Component Packing ✅

### Concept

Posit already handles disconnected components correctly (no overlaps, valid positions in all phases). However, the arrangement strategy is implicit. Adding an explicit packing option gives consumers control over how separate components are positioned relative to each other.

### API Design

```go
type ComponentPacking int

const (
    PackHorizontal ComponentPacking = iota // Side by side (default)
    PackVertical                           // Stacked
)

type Options struct {
    // ... existing options ...
    ComponentPacking ComponentPacking
    ComponentGap     float64 // Spacing between components (default: NodeSep * 2)
}
```

---

## 10. Incremental Layout ✅

### Concept

Given an existing layout and a set of changed nodes (e.g., one node changed height), produce a minimal adjustment that preserves the mental map. Nodes far from the change shouldn't move.

### API Design

```go
type IncrementalOptions struct {
    Fixed   map[string]bool       // Nodes that should not move
    Changes map[string]NodeOptions // Nodes with new dimensions
}

func (g *Graph) IncrementalLayout(base *Layout, changes IncrementalOptions) *Layout {
    // 1. Apply dimension changes
    // 2. Keep same layer assignment
    // 3. Re-run Y coordinate assignment (layers shift for taller nodes)
    // 4. Keep X positions fixed for unchanged nodes
    // 5. Re-route affected edges only
}
```

### Constraints

The simple version preserves layer assignment and X positions, only adjusting Y coordinates and edge routes. A full version could re-run crossing minimization locally.

---

## 11. Compound Graphs (Clusters) ✅

### Concept

Compound graphs allow nodes to contain other nodes (subgraphs/clusters). Clusters are laid out as atomic units first, then internal nodes are positioned within.

### API Design

```go
g.AddNode("cluster-a", posit.NodeOptions{
    IsCluster: true,
    Padding:   20,
})
g.SetParent("node-1", "cluster-a")
g.SetParent("node-2", "cluster-a")
```

### Use Cases

- Package grouping in dependency graphs
- Organizational units in org charts
- Swimlanes in process diagrams
- Module boundaries in architecture diagrams

### Architecture Impact

This requires changes to every phase of the algorithm — cycle removal, ranking, ordering, and coordinate assignment all need cluster awareness. Defer until core features are stable.

---

## Implementation Summary

All planned improvements have been implemented:

| # | Feature | Status | Key Files |
|---|---------|--------|-----------|
| 1 | Rank constraints | ✅ | `rank.go` |
| 2 | Port support (5 constraint modes) | ✅ | `port.go`, `posit.go` |
| 3 | Side inference | ✅ | `route.go` |
| 4 | Orthogonal routing | ✅ | `route.go` |
| 5 | Multi-edge support | ✅ | `route.go` |
| 6 | Ordering constraints | ✅ | `order.go` |
| 7 | Port crossing minimization | ✅ | `order.go` |
| 8 | Edge weight API | ✅ | `posit.go` |
| 9 | Disconnected component packing | ✅ | `posit.go` |
| 10 | Incremental layout | ✅ | `posit.go` |
| 11 | Compound graphs (clusters) | ✅ | `posit.go` |
