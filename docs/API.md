# Posit API Reference

Posit is a pure Go library for computing hierarchical graph layouts using the Sugiyama algorithm. It takes a directed graph and produces X/Y coordinates for all nodes, arranging them in layers with minimal edge crossings.

Posit is designed as a **general-purpose layout engine**, suitable for any directed graph visualization: database schemas, dependency trees, organizational charts, state machines, data pipelines, and more. The API is intentionally minimal and unopinionated, outputting standard coordinates that work with any rendering approach.

## Table of Contents

- [Quick Start](#quick-start)
- [Graph Construction](#graph-construction)
- [Layout Options](#layout-options)
- [Layout Results](#layout-results)
- [Usage Patterns](#usage-patterns)
- [Integration Examples](#integration-examples)
- [Edge Cases](#edge-cases)
- [Thread Safety](#thread-safety)

---

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/drew/posit"
)

func main() {
    // Create a new graph
    g := posit.NewGraph()

    // Add nodes with dimensions
    g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
    g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
    g.AddNode("c", posit.NodeOptions{Width: 100, Height: 40})

    // Add directed edges
    g.AddEdge("a", "b")
    g.AddEdge("a", "c")
    g.AddEdge("b", "c")

    // Compute layout with default options
    layout := g.Layout()

    // Access node positions
    for id, node := range layout.Nodes {
        fmt.Printf("Node %s: position=(%v, %v) size=(%v x %v)\n",
            id, node.X, node.Y, node.Width, node.Height)
    }

    // Access edge routing points
    for id, edge := range layout.Edges {
        fmt.Printf("Edge %s: %d points\n", id, len(edge.Points))
    }
}
```

---

## Graph Construction

### NewGraph

Creates a new empty graph ready for nodes and edges.

```go
func NewGraph() *Graph
```

**Example:**
```go
g := posit.NewGraph()
```

### AddNode

Adds a node with the given ID and dimensions. If a node with the same ID already exists, it is replaced with the new dimensions.

```go
func (g *Graph) AddNode(id string, opts NodeOptions)
```

**Parameters:**
- `id` - Unique string identifier for the node
- `opts` - Node options specifying dimensions

**NodeOptions struct:**
```go
type NodeOptions struct {
    Width  float64  // Width of the node in pixels
    Height float64  // Height of the node in pixels
}
```

**Examples:**
```go
// Simple rectangular nodes
g.AddNode("user", posit.NodeOptions{Width: 120, Height: 60})
g.AddNode("database", posit.NodeOptions{Width: 80, Height: 80})

// Nodes with varying sizes
g.AddNode("header", posit.NodeOptions{Width: 200, Height: 30})
g.AddNode("content", posit.NodeOptions{Width: 200, Height: 150})

// Replace an existing node with new dimensions
g.AddNode("user", posit.NodeOptions{Width: 150, Height: 80})  // replaces previous
```

### AddEdge

Adds a directed edge from source node to target node.

```go
func (g *Graph) AddEdge(from, to string)
```

**Parameters:**
- `from` - ID of the source node
- `to` - ID of the target node

**Examples:**
```go
// Simple edge
g.AddEdge("parent", "child")

// Multiple edges from one node
g.AddEdge("root", "left")
g.AddEdge("root", "right")

// Chain of edges
g.AddEdge("a", "b")
g.AddEdge("b", "c")
g.AddEdge("c", "d")
```

**Note:** Edges to non-existent nodes are allowed during construction. The layout algorithm will handle them appropriately.

### NodeCount

Returns the number of nodes in the graph.

```go
func (g *Graph) NodeCount() int
```

**Example:**
```go
g := posit.NewGraph()
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
fmt.Println(g.NodeCount())  // Output: 2
```

### EdgeCount

Returns the number of edges in the graph.

```go
func (g *Graph) EdgeCount() int
```

**Example:**
```go
g := posit.NewGraph()
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("a", "b")
g.AddEdge("a", "b")  // Duplicate edges are counted
fmt.Println(g.EdgeCount())  // Output: 2
```

---

## Layout Options

### Options Struct

```go
type Options struct {
    Direction Direction       // Flow direction of the layout
    NodeSep   float64         // Horizontal spacing between nodes
    RankSep   float64         // Vertical spacing between layers
    Algorithm RankAlgorithm   // Layer assignment algorithm
}
```

### Direction

Controls the primary direction of graph flow.

```go
type Direction int

const (
    TopToBottom Direction = iota  // Root at top, children below (default)
    LeftToRight                   // Root at left, children to right
    BottomToTop                   // Root at bottom, children above
    RightToLeft                   // Root at right, children to left
)
```

**Examples:**
```go
// Vertical flowchart (most common)
layout := g.Layout(posit.Options{
    Direction: posit.TopToBottom,
})

// Horizontal dependency graph
layout := g.Layout(posit.Options{
    Direction: posit.LeftToRight,
})

// Org chart with executives at bottom
layout := g.Layout(posit.Options{
    Direction: posit.BottomToTop,
})
```

### NodeSep

Minimum horizontal spacing between adjacent nodes on the same layer.

- **Type:** `float64`
- **Default:** `50`
- **Unit:** pixels

**Examples:**
```go
// Tight spacing for compact layouts
layout := g.Layout(posit.Options{
    NodeSep: 20,
})

// Wide spacing for readability
layout := g.Layout(posit.Options{
    NodeSep: 100,
})
```

### RankSep

Minimum vertical spacing between layers (ranks) of nodes.

- **Type:** `float64`
- **Default:** `100`
- **Unit:** pixels

**Examples:**
```go
// Compact vertical layout
layout := g.Layout(posit.Options{
    RankSep: 50,
})

// Spacious layout for annotations
layout := g.Layout(posit.Options{
    RankSep: 200,
})
```

### Algorithm

Selects the algorithm used for layer assignment.

```go
type RankAlgorithm int

const (
    LongestPath    RankAlgorithm = iota  // Fast, may produce more layers
    NetworkSimplex                        // Optimal, more computation
)
```

**LongestPath:**
- Simple and fast O(V + E) algorithm
- May produce more layers than necessary
- Good for interactive use or large graphs

**NetworkSimplex:**
- Produces optimal (minimum) number of layers
- More complex computation
- Better for final/print-quality layouts

**Examples:**
```go
// Fast layout for real-time updates
layout := g.Layout(posit.Options{
    Algorithm: posit.LongestPath,
})

// Optimal layout for export
layout := g.Layout(posit.Options{
    Algorithm: posit.NetworkSimplex,
})
```

### DefaultOptions

Returns sensible defaults for common use cases.

```go
func DefaultOptions() Options
```

**Returns:**
```go
Options{
    Direction: TopToBottom,
    NodeSep:   50,
    RankSep:   100,
    Algorithm: LongestPath,
}
```

**Example:**
```go
// Start with defaults and customize
opts := posit.DefaultOptions()
opts.Direction = posit.LeftToRight
opts.NodeSep = 75
layout := g.Layout(opts)
```

### Layout Method Signature

```go
func (g *Graph) Layout(opts ...Options) *Layout
```

The `Layout` method accepts zero or one Options argument:

```go
// Use default options
layout := g.Layout()

// Use custom options
layout := g.Layout(posit.Options{
    Direction: posit.LeftToRight,
    NodeSep:   75,
    RankSep:   120,
    Algorithm: posit.NetworkSimplex,
})
```

---

## Layout Results

### Layout Struct

```go
type Layout struct {
    Nodes map[string]NodeLayout  // Node positions keyed by node ID
    Edges map[string]EdgeLayout  // Edge paths keyed by edge ID
}
```

### NodeLayout

Contains the computed position and dimensions for a node.

```go
type NodeLayout struct {
    Position          // Embedded X, Y coordinates
    Width   float64   // Original width from NodeOptions
    Height  float64   // Original height from NodeOptions
}

type Position struct {
    X float64  // Horizontal position (top-left corner of node)
    Y float64  // Vertical position (top-left corner of node)
}
```

**Accessing node data:**
```go
layout := g.Layout()

// Access specific node
node := layout.Nodes["myNode"]
fmt.Printf("Top-left: (%v, %v)\n", node.X, node.Y)
fmt.Printf("Size: %v x %v\n", node.Width, node.Height)

// Calculate bounding box
left := node.X
top := node.Y
right := node.X + node.Width
bottom := node.Y + node.Height

// Calculate center (if needed for rendering)
centerX := node.X + node.Width/2
centerY := node.Y + node.Height/2
```

### EdgeLayout

Contains the routed path for an edge as a series of points.

```go
type EdgeLayout struct {
    Points []EdgePoint  // Ordered points defining the edge path
}

type EdgePoint struct {
    X float64
    Y float64
}
```

**Accessing edge data:**
```go
layout := g.Layout()

// Edge IDs are formatted as "from->to"
edge := layout.Edges["a->b"]

// Draw edge as polyline
for i, point := range edge.Points {
    fmt.Printf("Point %d: (%v, %v)\n", i, point.X, point.Y)
}

// First point is typically at source node
// Last point is typically at target node
start := edge.Points[0]
end := edge.Points[len(edge.Points)-1]
```

### Coordinate System

Posit uses a standard screen coordinate system with **top-left positioning**:

- **Origin (0, 0):** Top-left corner of the layout
- **X axis:** Increases to the right
- **Y axis:** Increases downward
- **Node X, Y:** Top-left corner of the node (not center)
- **Units:** Same as input dimensions (typically pixels)

```
(0,0) -----> X+
  |
  |    ┌─────────┐
  |    │ node    │  ← node.X, node.Y points here (top-left)
  v    │         │
  Y+   └─────────┘
```

This convention matches standard layout algorithms (dagre, ELK) and graphics APIs. Libraries that use center-based positioning (like React Flow with `origin: [0.5, 0.5]`) can easily convert:

```go
// Convert top-left to center
centerX := node.X + node.Width/2
centerY := node.Y + node.Height/2
```

**Example: Calculate canvas bounds**
```go
layout := g.Layout()

var minX, minY float64 = math.MaxFloat64, math.MaxFloat64
var maxX, maxY float64 = -math.MaxFloat64, -math.MaxFloat64

for _, node := range layout.Nodes {
    left := node.X - node.Width/2
    right := node.X + node.Width/2
    top := node.Y - node.Height/2
    bottom := node.Y + node.Height/2

    minX = min(minX, left)
    maxX = max(maxX, right)
    minY = min(minY, top)
    maxY = max(maxY, bottom)
}

canvasWidth := maxX - minX
canvasHeight := maxY - minY
```

---

## Usage Patterns

### Building Graphs from Database Schemas

```go
func buildSchemaGraph(tables []Table) *posit.Layout {
    g := posit.NewGraph()

    // Add table nodes
    for _, table := range tables {
        // Size based on number of columns
        height := 30 + float64(len(table.Columns)) * 20
        g.AddNode(table.Name, posit.NodeOptions{
            Width:  150,
            Height: height,
        })
    }

    // Add foreign key relationships
    for _, table := range tables {
        for _, fk := range table.ForeignKeys {
            g.AddEdge(table.Name, fk.ReferencedTable)
        }
    }

    // Layout with horizontal flow for ER diagrams
    return g.Layout(posit.Options{
        Direction: posit.LeftToRight,
        NodeSep:   80,
        RankSep:   150,
    })
}
```

### Building Graphs from Dependency Trees

```go
func buildDependencyGraph(root *Package) *posit.Layout {
    g := posit.NewGraph()
    visited := make(map[string]bool)

    var addPackage func(pkg *Package)
    addPackage = func(pkg *Package) {
        if visited[pkg.Name] {
            return
        }
        visited[pkg.Name] = true

        g.AddNode(pkg.Name, posit.NodeOptions{
            Width:  float64(len(pkg.Name)*8 + 20),
            Height: 30,
        })

        for _, dep := range pkg.Dependencies {
            g.AddEdge(pkg.Name, dep.Name)
            addPackage(dep)
        }
    }

    addPackage(root)

    return g.Layout(posit.Options{
        Direction: posit.TopToBottom,
        Algorithm: posit.NetworkSimplex,  // Optimal depth
    })
}
```

### Handling Disconnected Components

Posit automatically handles graphs with multiple disconnected components.

```go
g := posit.NewGraph()

// Component 1
g.AddNode("a1", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("a2", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("a1", "a2")

// Component 2 (not connected to Component 1)
g.AddNode("b1", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b2", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("b1", "b2")

// Both components are laid out
layout := g.Layout()
// Components will be positioned side by side
```

### Large Graphs - Performance Considerations

```go
// For large graphs (1000+ nodes), use LongestPath algorithm
func layoutLargeGraph(nodeCount int) *posit.Layout {
    g := posit.NewGraph()

    // Add many nodes
    for i := 0; i < nodeCount; i++ {
        g.AddNode(fmt.Sprintf("node_%d", i), posit.NodeOptions{
            Width:  60,
            Height: 30,
        })
    }

    // Add edges...

    // Use fast algorithm for large graphs
    return g.Layout(posit.Options{
        Algorithm: posit.LongestPath,  // O(V+E) vs O(V*E) for NetworkSimplex
        NodeSep:   30,                 // Tighter spacing to fit more
        RankSep:   60,
    })
}
```

**Performance tips:**
- Use `LongestPath` algorithm for graphs with 500+ nodes
- Reduce `NodeSep` and `RankSep` for very large graphs
- Consider pre-filtering to show only relevant subgraphs
- Layout computation is CPU-bound; consider caching results

---

## Integration Examples

### SVG Rendering

```go
func renderSVG(layout *posit.Layout) string {
    var svg strings.Builder

    // Calculate bounds
    var maxX, maxY float64
    for _, node := range layout.Nodes {
        maxX = max(maxX, node.X+node.Width/2)
        maxY = max(maxY, node.Y+node.Height/2)
    }

    svg.WriteString(fmt.Sprintf(
        `<svg xmlns="http://www.w3.org/2000/svg" width="%v" height="%v">`,
        maxX+20, maxY+20,
    ))

    // Render edges first (behind nodes)
    for _, edge := range layout.Edges {
        if len(edge.Points) < 2 {
            continue
        }

        svg.WriteString(`<polyline fill="none" stroke="#666" stroke-width="2" points="`)
        for i, pt := range edge.Points {
            if i > 0 {
                svg.WriteString(" ")
            }
            svg.WriteString(fmt.Sprintf("%v,%v", pt.X, pt.Y))
        }
        svg.WriteString(`"/>`)
    }

    // Render nodes
    for id, node := range layout.Nodes {
        x := node.X - node.Width/2
        y := node.Y - node.Height/2

        svg.WriteString(fmt.Sprintf(
            `<rect x="%v" y="%v" width="%v" height="%v" fill="#fff" stroke="#333" rx="4"/>`,
            x, y, node.Width, node.Height,
        ))
        svg.WriteString(fmt.Sprintf(
            `<text x="%v" y="%v" text-anchor="middle" dominant-baseline="middle">%s</text>`,
            node.X, node.Y, id,
        ))
    }

    svg.WriteString(`</svg>`)
    return svg.String()
}
```

### Canvas Rendering (with go-canvas or similar)

```go
func renderToCanvas(ctx *canvas.Context, layout *posit.Layout) {
    // Draw edges
    ctx.SetStrokeStyle("#666666")
    ctx.SetLineWidth(2)

    for _, edge := range layout.Edges {
        if len(edge.Points) == 0 {
            continue
        }

        ctx.BeginPath()
        ctx.MoveTo(edge.Points[0].X, edge.Points[0].Y)
        for _, pt := range edge.Points[1:] {
            ctx.LineTo(pt.X, pt.Y)
        }
        ctx.Stroke()
    }

    // Draw nodes
    ctx.SetFillStyle("#ffffff")
    ctx.SetStrokeStyle("#333333")

    for id, node := range layout.Nodes {
        x := node.X - node.Width/2
        y := node.Y - node.Height/2

        // Draw rectangle
        ctx.FillRect(x, y, node.Width, node.Height)
        ctx.StrokeRect(x, y, node.Width, node.Height)

        // Draw label
        ctx.SetFillStyle("#000000")
        ctx.FillText(id, node.X, node.Y)
        ctx.SetFillStyle("#ffffff")
    }
}
```

### Exporting to JSON for Frontend Consumption

```go
type LayoutJSON struct {
    Nodes []NodeJSON `json:"nodes"`
    Edges []EdgeJSON `json:"edges"`
}

type NodeJSON struct {
    ID     string  `json:"id"`
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

type EdgeJSON struct {
    ID     string      `json:"id"`
    From   string      `json:"from"`
    To     string      `json:"to"`
    Points []PointJSON `json:"points"`
}

type PointJSON struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

func exportToJSON(layout *posit.Layout) ([]byte, error) {
    result := LayoutJSON{
        Nodes: make([]NodeJSON, 0, len(layout.Nodes)),
        Edges: make([]EdgeJSON, 0, len(layout.Edges)),
    }

    for id, node := range layout.Nodes {
        result.Nodes = append(result.Nodes, NodeJSON{
            ID:     id,
            X:      node.X,
            Y:      node.Y,
            Width:  node.Width,
            Height: node.Height,
        })
    }

    for id, edge := range layout.Edges {
        parts := strings.Split(id, "->")
        points := make([]PointJSON, len(edge.Points))
        for i, pt := range edge.Points {
            points[i] = PointJSON{X: pt.X, Y: pt.Y}
        }
        result.Edges = append(result.Edges, EdgeJSON{
            ID:     id,
            From:   parts[0],
            To:     parts[1],
            Points: points,
        })
    }

    return json.Marshal(result)
}
```

**Frontend usage (JavaScript):**
```javascript
fetch('/api/graph/layout')
    .then(res => res.json())
    .then(layout => {
        layout.nodes.forEach(node => {
            drawRect(node.x - node.width/2, node.y - node.height/2,
                     node.width, node.height);
            drawText(node.id, node.x, node.y);
        });

        layout.edges.forEach(edge => {
            drawPolyline(edge.points);
        });
    });
```

---

## Edge Cases

### Empty Graph

An empty graph produces an empty layout.

```go
g := posit.NewGraph()
layout := g.Layout()

fmt.Println(len(layout.Nodes))  // 0
fmt.Println(len(layout.Edges))  // 0
```

### Single Node

A graph with one node and no edges works correctly.

```go
g := posit.NewGraph()
g.AddNode("lonely", posit.NodeOptions{Width: 100, Height: 50})

layout := g.Layout()
node := layout.Nodes["lonely"]
// Node is positioned, typically at origin
fmt.Printf("Position: (%v, %v)\n", node.X, node.Y)
```

### Cycles

Posit automatically detects and handles cycles by temporarily reversing edges. The final layout preserves the original edge direction in the `Edges` result.

```go
g := posit.NewGraph()
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("c", posit.NodeOptions{Width: 100, Height: 40})

// Create a cycle: a -> b -> c -> a
g.AddEdge("a", "b")
g.AddEdge("b", "c")
g.AddEdge("c", "a")  // This creates a cycle

layout := g.Layout()  // Works correctly
// All three edges are present in layout.Edges
```

### Disconnected Components

Multiple disconnected subgraphs are laid out together.

```go
g := posit.NewGraph()

// Island 1
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("a", "b")

// Island 2 (no connection to Island 1)
g.AddNode("x", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("y", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("x", "y")

layout := g.Layout()
// Both islands are positioned without overlap
```

### Self-Loops

Edges where source equals target are handled gracefully.

```go
g := posit.NewGraph()
g.AddNode("self", posit.NodeOptions{Width: 100, Height: 40})
g.AddEdge("self", "self")  // Self-loop

layout := g.Layout()
// Self-loop edge is included in layout.Edges
```

### Duplicate Edges

Multiple edges between the same pair of nodes are preserved.

```go
g := posit.NewGraph()
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})

g.AddEdge("a", "b")
g.AddEdge("a", "b")  // Second edge between same nodes

fmt.Println(g.EdgeCount())  // 2
```

---

## Thread Safety

### Graph Building

**Graph construction is NOT thread-safe.** All `AddNode` and `AddEdge` calls must happen from a single goroutine or be externally synchronized.

```go
// WRONG - data race
g := posit.NewGraph()
go func() { g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40}) }()
go func() { g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40}) }()

// CORRECT - build sequentially
g := posit.NewGraph()
g.AddNode("a", posit.NodeOptions{Width: 100, Height: 40})
g.AddNode("b", posit.NodeOptions{Width: 100, Height: 40})
```

### Layout Computation

**Layout computation can be called concurrently on different graphs.** Each `Layout()` call operates on its own internal state.

```go
// CORRECT - different graphs can be laid out concurrently
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        g := posit.NewGraph()
        // ... build graph ...
        layout := g.Layout()  // Safe
        // ... use layout ...
    }(i)
}

wg.Wait()
```

### Layout Results

**Layout results are immutable.** The `*Layout` returned by `Layout()` is safe to read from multiple goroutines. However, modifying the maps directly is not safe.

```go
layout := g.Layout()

// CORRECT - concurrent reads
go func() { _ = layout.Nodes["a"] }()
go func() { _ = layout.Nodes["b"] }()

// WRONG - don't modify the layout
layout.Nodes["a"] = posit.NodeLayout{}  // Unsafe if concurrent
```

### Recommended Pattern

```go
// Build graph (single goroutine)
g := posit.NewGraph()
for _, item := range items {
    g.AddNode(item.ID, posit.NodeOptions{Width: item.Width, Height: item.Height})
}
for _, link := range links {
    g.AddEdge(link.From, link.To)
}

// Compute layout (can be done in goroutine)
layout := g.Layout()

// Use layout (safe for concurrent reads)
http.HandleFunc("/layout", func(w http.ResponseWriter, r *http.Request) {
    nodeID := r.URL.Query().Get("node")
    if node, ok := layout.Nodes[nodeID]; ok {
        json.NewEncoder(w).Encode(node)
    }
})
```

---

## Version Compatibility

This documentation covers posit v0.x. The API is subject to change before v1.0.

## See Also

- [README.md](../README.md) - Project overview and installation
- [Sugiyama Algorithm](https://en.wikipedia.org/wiki/Layered_graph_drawing) - Background on the algorithm
