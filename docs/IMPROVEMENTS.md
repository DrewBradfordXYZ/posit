# Posit Roadmap

Planned improvements to Posit as a general-purpose layered graph layout library. These are standard graph layout concepts that benefit any consumer — schema diagrams, dependency graphs, org charts, state machines, etc.

---

## 1. Port Support

**Priority:** High
**Status:** Planned

### Concept

Ports are fixed connection points at specific positions on a node. Without ports, edges connect to node boundaries using rectangle intersection. With ports, edges connect to precise, named locations.

This is a standard feature in ELK and Graphviz but absent from dagre (Posit's reference implementation).

### Use Cases

- Database schema diagrams: edges connect to specific field rows
- Circuit diagrams: components have labeled input/output pins
- UML class diagrams: associations connect to specific attributes
- Data flow diagrams: nodes have named input/output channels

### API Design

```go
type Side int

const (
    Left Side = iota
    Right
    Top
    Bottom
)

type PortOptions struct {
    ID     string  // Unique within the node (e.g., "in-1", "out-2")
    Side   Side    // Which side of the node
    Offset float64 // Distance from node origin along the side
}

type NodeOptions struct {
    Width  float64
    Height float64
    Ports  []PortOptions // Optional: fixed connection points
}

type EdgeOptions struct {
    SourcePort string // Connect to this port on source node
    TargetPort string // Connect to this port on target node
    // ... existing label options ...
}
```

### Edge Routing with Ports

When an edge specifies ports, routing uses the port's absolute position instead of boundary intersection:

```go
func (s *layoutState) getEdgeEndpoint(node *layoutNode, portID string) EdgePoint {
    if port, ok := node.ports[portID]; ok {
        switch port.Side {
        case Right:
            return EdgePoint{X: node.x + node.width, Y: node.y + port.Offset}
        case Left:
            return EdgePoint{X: node.x, Y: node.y + port.Offset}
        case Bottom:
            return EdgePoint{X: node.x + port.Offset, Y: node.y + node.height}
        case Top:
            return EdgePoint{X: node.x + port.Offset, Y: node.y}
        }
    }
    // Fallback: boundary intersection for nodes without ports
    return s.intersectRect(node, target)
}
```

### Output

```go
type EdgeLayout struct {
    From       string
    To         string
    Points     []EdgePoint
    SourcePort string // Which port the edge exits from (if specified)
    TargetPort string // Which port the edge enters (if specified)
    SourceSide Side   // Computed attachment side
    TargetSide Side   // Computed attachment side
}
```

Consumers receive fully-resolved positions — no client-side inference needed.

---

## 2. Port-Level Crossing Minimization

**Priority:** Medium
**Depends on:** Port Support

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
        // check if reordering reduces crossings
        // Use port Y positions as weights in barycenter calculation
    }
}
```

Alternatively, port positions influence the barycenter calculation so nodes with connected ports at similar offsets are placed near each other.

---

## 3. Edge Attachment Side Inference

**Priority:** Medium
**Depends on:** Port Support (optional, works without ports too)

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

## 4. Edge Weight in Public API

**Priority:** Low

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

## 5. Incremental Layout

**Priority:** Medium
**Effort:** High

### Concept

Given an existing layout and a set of changed nodes (e.g., one node changed height), produce a minimal adjustment that preserves the mental map. Nodes far from the change shouldn't move.

This is listed in ARCHITECTURE.md as a planned enhancement.

### API Design

```go
type IncrementalOptions struct {
    // Nodes that should not move (or move minimally)
    Fixed map[string]bool
    // Nodes with new dimensions
    Changes map[string]NodeOptions
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

## 6. Compound Graphs (Clusters)

**Priority:** Low
**Effort:** Very High

### Concept

Compound graphs allow nodes to contain other nodes (subgraphs/clusters). Clusters are laid out as atomic units first, then internal nodes are positioned within. This is listed in ARCHITECTURE.md as a known limitation.

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

## Implementation Order

| # | Improvement | Effort | Dependencies |
|---|-------------|--------|--------------|
| 1 | Port support | Medium | None |
| 3 | Side inference | Low | None (enhanced by ports) |
| 4 | Edge weight API | Low | None |
| 2 | Port crossing minimization | Medium | Port support |
| 5 | Incremental layout | High | None |
| 6 | Compound graphs | Very High | Architecture change |

Ports and side inference are the foundational additions. They provide consumers with fully-resolved edge endpoints, eliminating the most common reason for client-side position computation.
