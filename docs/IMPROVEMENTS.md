# Posit Improvements Roadmap

**Context:** Posit runs server-side in Go as part of the basetypes graph visualization. This gives it access to schema metadata, field-level handle information, and persistent state that client-side layout engines (ELK.js) cannot leverage.

## Improvement 1: Port-Aware Edge Routing

**Priority:** High
**Impact:** Eliminates ~200 lines of client-side handle calculation code and 1,150 DOM queries

### Problem

When nodes are expanded, edges connect to specific field rows (handles) at known Y positions. Currently:

1. Server sends `source-handle="field-3"` and `target-handle="field-42"` as HTML attributes
2. Client calls `getBoundingClientRect()` on each handle element to find its pixel Y position
3. This causes layout thrashing (the original 2,463ms bottleneck that "fast edge mode" worked around)

The client reinvents what the server already knows: the Y position of each field row.

### Solution

Add port support to Posit. Ports are fixed connection points at specific positions on a node.

```go
type PortOptions struct {
    ID       string  // e.g., "field-3", "field-42"
    Side     Side    // Left, Right, Top, Bottom
    Offset   float64 // Y offset from node top (or X offset for top/bottom ports)
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

Edge routing would use port positions instead of `intersectRect()`:

```go
func (s *layoutState) getEdgeEndpoint(node *layoutNode, portID string) EdgePoint {
    if port, ok := node.ports[portID]; ok {
        // Use exact port position
        switch port.Side {
        case Right:
            return EdgePoint{X: node.x + node.width, Y: node.y + port.Offset}
        case Left:
            return EdgePoint{X: node.x, Y: node.y + port.Offset}
        }
    }
    // Fallback to intersectRect for nodes without ports
    return s.intersectRect(node, target)
}
```

### Server Integration

The server knows field order and can compute port offsets:

```go
// In layout.go
const fieldRowHeight = 20.0
const headerHeight = 32.0

func computePorts(table data.TableGraphNode, fields []data.Field, expanded bool) []posit.PortOptions {
    if !expanded {
        return nil // Collapsed nodes use boundary intersection
    }

    var ports []posit.PortOptions
    for i, field := range fields {
        if field.ID == 3 || field.IsFK {
            offset := headerHeight + float64(i)*fieldRowHeight + fieldRowHeight/2
            side := posit.Right
            if field.IsFK {
                side = posit.Left
            }
            ports = append(ports, posit.PortOptions{
                ID:     fmt.Sprintf("field-%d", field.ID),
                Side:   side,
                Offset: offset,
            })
        }
    }
    return ports
}
```

### What This Eliminates Client-Side

| Current Client Code | Purpose | With Port Support |
|---------------------|---------|-------------------|
| `getHandleWorldPosition()` | DOM query for handle Y | **Eliminated** - server pre-computed |
| `batchPopulateHandleCache()` | Batch DOM reads to avoid thrashing | **Eliminated** |
| `handleLocalOffsetCache` | Cache handle positions after first query | **Eliminated** |
| `invalidateHandleCacheForNode()` | Clear cache on node move | **Eliminated** |
| `enablePreciseEdges()` | Switch from fast mode to DOM mode | **Eliminated** |
| `fastEdgeMode` flag | Skip DOM queries on initial render | **Eliminated** - only mode |

### Edge Output with Ports

```go
type EdgeLayout struct {
    From       string
    To         string
    Points     []EdgePoint
    SourcePort string // Which port the edge exits from
    TargetPort string // Which port the edge enters
    SourceSide string // "left", "right", "top", "bottom"
    TargetSide string // "left", "right", "top", "bottom"
}
```

The server sets `source-position` and `target-position` attributes directly on `<flow-edge>`:

```html
<flow-edge source="nodeA" target="nodeB"
           source-handle="field-3" target-handle="field-42"
           source-position="right" target-position="left"
           waypoints='[{"x":250,"y":180}]' />
```

Client renders with no computation - just uses the provided positions and waypoints.

---

## Improvement 2: Port-Level Crossing Minimization

**Priority:** Medium
**Impact:** Better visual quality for expanded nodes with many FK fields

### Problem

When a node has multiple FK fields (handles), edges to those fields may cross each other unnecessarily. The current barycenter heuristic operates on nodes, not ports.

### Solution

Extend crossing minimization to consider port ordering. When computing barycenters for nodes with ports, use the port Y positions as sub-ordering:

```go
// When computing crossings between edges to the same node's ports,
// optimize port ordering or route edges to minimize visual crossings
func (s *layoutState) minimizePortCrossings() {
    for _, node := range s.nodes {
        if len(node.ports) <= 1 {
            continue
        }
        // Sort edges by their source/target positions to minimize crossings
        // This is a separate optimization pass after node ordering
    }
}
```

Alternatively, use port Y positions as weights in the barycenter calculation so nodes with connected ports at similar heights are placed near each other.

---

## Improvement 3: Handle Side Inference (Server-Side)

**Priority:** Medium
**Impact:** Eliminates client-side `inferPosition()` and `axis` constraints

### Problem

Currently the client infers which side of a node an edge should connect to based on relative node positions:

```javascript
// Client-side inference
function inferPosition(from, to, axis) {
    const dx = to.x - from.x
    if (axis === 'horizontal') {
        return dx > 0 ? 'right' : 'left'
    }
    // ...
}
```

This is recalculated every frame during drag.

### Solution

Posit already knows the relative positions of all nodes after layout. It can compute the optimal side for each edge endpoint:

```go
func inferSide(fromNode, toNode *layoutNode) (sourceSide, targetSide string) {
    dx := (toNode.x + toNode.width/2) - (fromNode.x + fromNode.width/2)
    dy := (toNode.y + toNode.height/2) - (fromNode.y + fromNode.height/2)

    if math.Abs(dx) > math.Abs(dy) {
        if dx > 0 {
            return "right", "left"
        }
        return "left", "right"
    }
    if dy > 0 {
        return "bottom", "top"
    }
    return "top", "bottom"
}
```

The `SourceSide` and `TargetSide` fields in `EdgeLayout` provide this to the client, which renders directly without inference.

**During drag:** The client still uses `inferPosition()` for transient states. On drag-end, the server recomputes authoritative sides.

---

## Improvement 4: Schema-Aware Edge Weighting

**Priority:** Low
**Impact:** Better layout quality for complex schemas

### Problem

All edges are treated equally during crossing minimization. But in QuickBase schemas, some relationships are more important:

- FK relationships (direct parent-child) are primary
- Lookup fields reference data across tables
- Summary fields aggregate data

### Solution

Add edge weight support to the integration layer (Posit already supports weighted edges internally):

```go
func computeEdgeWeight(edge data.TableGraphEdge) float64 {
    // FK relationships are the primary structural edges
    if edge.RelType == "FK" {
        return 2.0
    }
    // Lookups are secondary
    if edge.RelType == "Lookup" {
        return 1.0
    }
    // Summaries are tertiary
    return 0.5
}

// When adding edges to graph
g.AddEdge(edge.From, edge.To, posit.EdgeOptions{
    Weight: computeEdgeWeight(edge),
})
```

Heavier edges are prioritized during crossing minimization, keeping primary relationships visually clearer.

**Note:** This requires adding a `Weight` field to `EdgeOptions` in Posit's API.

---

## Improvement 5: Incremental Layout on Expand/Collapse

**Priority:** Medium
**Impact:** Faster layout updates, less visual disruption

### Problem

When a node expands (height changes from 70px to 450px+), the layout should adjust:
- Nodes below may need to shift down
- Edge routing changes (ports vs boundary)
- But nodes far away shouldn't jump

Currently: client handles expand locally (CSS transition), no layout recompute.

### Solution

Add incremental layout to Posit - given an existing layout and a set of changed nodes, produce a minimal adjustment:

```go
type IncrementalOptions struct {
    // Fixed nodes that should not move (or move minimally)
    Fixed map[string]bool
    // Changed nodes with new dimensions
    Changes map[string]NodeOptions
}

func (g *Graph) IncrementalLayout(base *Layout, changes IncrementalOptions) *Layout {
    // 1. Apply dimension changes
    // 2. Re-run coordinate assignment with constraints
    // 3. Keep fixed nodes at their current positions (or shift minimally)
    // 4. Re-route affected edges
}
```

This is a larger architectural addition. The simple version:
- Keep same layer assignment
- Re-run Y coordinate assignment (layers shift to accommodate taller nodes)
- Keep X positions fixed
- Re-route edges for changed nodes only

---

## Improvement 6: Layout Caching and Pre-computation

**Priority:** Low
**Impact:** Instant graph load for repeated views

### Problem

Layout is computed on first view and saved to SQLite. But if the schema changes (new table, new relationship), positions are stale.

### Solution

Cache layouts by schema hash. Recompute only when graph structure changes:

```go
type LayoutCache struct {
    Hash      string     // SHA256 of graph structure (node IDs + edge pairs)
    Layout    *Layout    // Cached result
    ComputeAt time.Time
}

func (h *Handlers) getOrComputeLayout(graphData *data.TableGraph, expanded map[string]bool) *posit.Layout {
    hash := computeGraphHash(graphData, expanded)

    if cached := h.cache.Get(hash); cached != nil {
        return cached
    }

    layout := ComputeLayout(graphData.Nodes, graphData.Edges, expanded)
    h.cache.Set(hash, layout)
    return layout
}
```

**Pre-computation:** Could run layout at `download_schema` time, so graphs are instantly available.

---

## Improvement 7: Cluster/Grouping Support

**Priority:** Low
**Impact:** Better organization for large schemas

### Problem

Large apps have natural table groupings (e.g., "Project" tables, "Finance" tables). Currently all tables are laid out independently.

### Solution

Add compound node (cluster) support to Posit:

```go
g.AddNode("cluster-projects", posit.NodeOptions{
    IsCluster: true,
    Padding:   20,
})
g.SetParent("projects-table", "cluster-projects")
g.SetParent("tasks-table", "cluster-projects")
```

Clusters are laid out as atomic units first, then internal nodes are positioned within.

**Note:** This is a significant architectural addition (listed as a current limitation in Posit's docs). Defer until basic integration is proven.

---

## Improvement 8: SVG/Image Export

**Priority:** Low
**Impact:** Documentation, sharing, offline viewing

### Problem

Graph can only be viewed in the browser. No way to export for documentation or share with non-users.

### Solution

Since Posit computes complete edge paths server-side, generating SVG is straightforward:

```go
func ExportSVG(layout *posit.Layout, nodes []data.TableGraphNode) []byte {
    var buf bytes.Buffer
    // Write SVG header with viewBox from layout bounds
    // Render nodes as rects with labels
    // Render edges as polylines/paths from EdgeLayout.Points
    // Return complete SVG
}
```

CLI integration:
```bash
basetypes graph export --app=xyz --format=svg > schema.svg
basetypes graph export --app=xyz --format=png > schema.png
```

---

## Implementation Order

| # | Improvement | Effort | Impact | Dependencies |
|---|-------------|--------|--------|--------------|
| 1 | Port-aware routing | Medium | Very High | None |
| 3 | Handle side inference | Low | Medium | Improvement 1 |
| 2 | Port-level crossing minimization | Medium | Medium | Improvement 1 |
| 5 | Incremental layout | High | Medium | None |
| 4 | Schema-aware edge weighting | Low | Low | Weight in EdgeOptions |
| 6 | Layout caching | Low | Low | None |
| 7 | Cluster support | Very High | Medium | Architecture change |
| 8 | SVG export | Medium | Low | None |

**Recommended first step:** Improvement 1 (ports) + Improvement 3 (side inference) together, as they eliminate the most client-side complexity and directly address the performance bottleneck identified in the graph performance analysis.

---

## What This Means for datastar-flow

With improvements 1 and 3 implemented, the following client-side code becomes unnecessary:

```
flow-container.html:
  - getHandleWorldPosition()        (~40 lines)
  - batchPopulateHandleCache()      (~30 lines)
  - handleLocalOffsetCache          (~20 lines)
  - getCachedHandleOffset()         (~15 lines)
  - invalidateHandleCacheForNode()  (~10 lines)
  - enablePreciseEdges()            (~15 lines)
  - fastEdgeMode flag + logic       (~20 lines)
  - getHandlePositionFallback()     (~25 lines)

paths.js:
  - inferPosition()                 (~10 lines)

Total: ~185 lines of performance-critical code eliminated
```

The edge rendering path simplifies to:
1. Read `source-position`, `target-position` from attributes (server-computed)
2. Read `waypoints` from attributes (server-computed)
3. Calculate bezier/smoothstep path from waypoints
4. Update SVG `<path>` element

No DOM queries. No caching. No invalidation. No fast/precise mode switching.
