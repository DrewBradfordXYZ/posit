# Posit Architecture

**Posit** is a pure Go implementation of the Sugiyama algorithm for layered graph layout. It computes X/Y positions for nodes in directed graphs, arranging them in hierarchical layers with minimal edge crossings.

## Overview

### Why Posit Exists

The Go ecosystem lacks a native, dependency-free solution for hierarchical graph layout. While JavaScript has dagre and Python has graphviz bindings, Go developers often resort to:

- Shelling out to external tools (graphviz)
- Using CGO bindings with native dependencies
- Implementing ad-hoc layouts that don't handle edge crossings

Posit fills this gap with a pure Go implementation that:

- Requires **zero external dependencies**
- Compiles to a single binary with no runtime requirements
- Performs well for graphs up to 200+ nodes
- Produces publication-quality hierarchical layouts

### General-Purpose Design

Posit is designed as a **general-purpose graph layout library**, suitable for any hierarchical directed graph:

- **Database schemas** - tables as nodes, foreign keys as edges
- **Dependency graphs** - packages, modules, build targets
- **Organizational charts** - reporting hierarchies
- **State machines** - states and transitions
- **Data pipelines** - ETL stages and flows
- **Class hierarchies** - inheritance relationships

The library prioritizes **sound algorithms and good theory** over narrow optimizations. Features that benefit the general case are preferred; project-specific tweaks can be layered on top by consumers.

### Design Influences

The following projects provided real-world requirements that shaped posit's design:

| Project | Influence |
|---------|-----------|
| **dagre** (JS) | Reference implementation; algorithm structure and phase organization |
| **React Flow / xyflow** | Coordinate conventions (top-left origin), API patterns |
| **ELK** | Algorithm options (network-simplex placement), spacing defaults |

These influences are documented to provide context, not to limit scope. Posit aims to be useful **beyond** these specific use cases.

### The Sugiyama Algorithm

The Sugiyama algorithm (also called the "layered graph drawing" algorithm) is the standard approach for drawing hierarchical directed graphs. It was introduced by Kozo Sugiyama in 1981 and consists of several distinct phases that transform an arbitrary directed graph into a readable hierarchical layout.

---

## Design Philosophy

### Zero Dependencies

Posit uses only the Go standard library. This ensures:

- Simple vendoring and reproducible builds
- No transitive dependency vulnerabilities
- Predictable compilation times
- Works in restricted environments (air-gapped, minimal containers)

### Simple API, Complex Internals

The public API surface is intentionally minimal:

```go
// This is the entire public interface
g := posit.NewGraph()
g.AddNode("users", posit.NodeOptions{Width: 120, Height: 40})
g.AddNode("orders", posit.NodeOptions{Width: 120, Height: 40})
g.AddEdge("users", "orders")

layout := g.Layout(posit.Options{
    Direction: posit.TopToBottom,
    NodeSep:   50,
    RankSep:   100,
})

// layout.Nodes["users"].X, layout.Nodes["users"].Y, etc.
```

All algorithmic complexity is hidden inside the package. Users never need to understand layers, dummy nodes, or crossing minimization.

### Modular Phases

Each algorithm phase:

- Has a single responsibility
- Can be tested in isolation
- Has clear pre-conditions and post-conditions
- Operates on a shared internal state structure

This modularity enables:

- Targeted unit tests for each phase
- Easy debugging (inspect state between phases)
- Future extensibility (swap algorithms per phase)

### Performance Target

The algorithm is optimized for interactive use:

- **Small graphs (< 50 nodes)**: < 10ms
- **Medium graphs (50-200 nodes)**: < 100ms
- **Large graphs (200+ nodes)**: < 500ms

These targets assume typical database schemas where most nodes have 2-5 edges.

---

## Module Structure

```
posit/
├── posit.go          # Public API: Graph, Layout, Options, types
├── state.go          # Internal layoutState orchestrator
├── acyclic.go        # Phase 1: Cycle removal via DFS
├── rank.go           # Phase 2: Layer assignment (longest-path, network-simplex)
├── normalize.go      # Phase 3: Dummy node insertion for long edges
├── order.go          # Phase 4: Crossing minimization (barycenter heuristic)
├── position.go       # Phase 5: X/Y coordinate assignment (Brandes-Kopf)
├── route.go          # Phase 6: Edge routing and cleanup
├── util.go           # Shared helpers (adjacency, iteration, math)
└── posit_test.go     # Comprehensive test suite
```

### File Responsibilities

| File | Lines (est.) | Purpose |
|------|--------------|---------|
| `posit.go` | ~150 | Public types, Graph builder, Layout entry point |
| `state.go` | ~100 | `layoutState` struct, phase orchestration |
| `acyclic.go` | ~80 | DFS-based cycle detection, edge reversal |
| `rank.go` | ~200 | Longest-path and network-simplex ranking |
| `normalize.go` | ~100 | Dummy node insertion/removal for multi-layer edges |
| `order.go` | ~250 | Layer sweeps, barycenter calculation, crossing count |
| `position.go` | ~300 | Brandes-Kopf algorithm for X coordinates, Y stacking |
| `route.go` | ~100 | Edge path construction, reversed edge restoration |
| `util.go` | ~100 | Adjacency helpers, topological sort, min/max |

**Total: ~1,400 lines**

---

## Data Flow

The algorithm transforms data through six phases, with each phase building on the previous:

```
┌─────────────────┐
│   Input Graph   │  User-provided nodes and edges
│   (posit.Graph) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  layoutState    │  Internal representation with adjacency lists
│  initialization │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 1: Acyclic│  Remove cycles by reversing back-edges
│                 │  Enables topological processing
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 2: Rank   │  Assign each node to a layer (integer rank)
│                 │  Output: node.rank for all nodes
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 3: Norm.  │  Insert dummy nodes for edges spanning 2+ layers
│                 │  All edges now span exactly 1 layer
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 4: Order  │  Minimize edge crossings within layers
│                 │  Output: node.order (position within layer)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 5: Pos.   │  Assign X/Y coordinates
│                 │  Output: node.x, node.y for all nodes
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Phase 6: Route  │  Build edge paths through dummy nodes
│                 │  Restore reversed edges, cleanup
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Output Layout  │  Final positions for rendering
│  (posit.Layout) │
└─────────────────┘
```

### Phase Details

#### Phase 1: Make Acyclic

**Input**: Directed graph (may contain cycles)
**Output**: Directed acyclic graph (DAG)

Uses depth-first search to identify back-edges (edges that point to an ancestor in the DFS tree). These edges are "reversed" by marking them and swapping their direction. The reversal is undone in Phase 6.

```
Before:  A → B → C → A (cycle)
After:   A → B → C ← A (reversed edge marked)
```

#### Phase 2: Assign Layers (Ranking)

**Input**: DAG
**Output**: Each node has a `rank` (layer number, 0-indexed)

Two algorithms available:

1. **Longest Path** (default): Simple BFS from sources. Fast but may create more layers than necessary.

2. **Network Simplex**: Optimal layer assignment minimizing total edge length. More complex but produces tighter layouts.

```
Rank 0:  [A]
Rank 1:  [B, C]
Rank 2:  [D]
```

#### Phase 3: Normalize (Dummy Nodes)

**Input**: DAG with ranks
**Output**: DAG where all edges span exactly 1 layer

Long edges (spanning multiple layers) are split by inserting "dummy" nodes:

```
Before:  A (rank 0) ────────→ D (rank 2)

After:   A (rank 0) → dummy (rank 1) → D (rank 2)
```

Dummy nodes have zero width/height and exist only to provide anchor points for edge routing.

#### Phase 4: Minimize Crossings (Ordering)

**Input**: Layered graph with dummy nodes
**Output**: Each node has an `order` (position within its layer)

Uses the barycenter heuristic with layer sweeps:

1. Start with initial ordering (e.g., by DFS discovery)
2. Sweep down: For each layer, position nodes at the barycenter (average) of their predecessors
3. Sweep up: Position nodes at the barycenter of their successors
4. Repeat until crossings stop decreasing (typically 4-8 iterations)

```
Before (4 crossings):     After (0 crossings):
Layer 0:  A   B           Layer 0:  A   B
           ╲ ╱                       │   │
            ╳                        │   │
           ╱ ╲                       │   │
Layer 1:  C   D           Layer 1:  C   D
```

#### Phase 5: Assign Coordinates

**Input**: Ordered layers
**Output**: X/Y coordinates for each node

**Y coordinates**: Simple stacking with `rankSep` spacing between layers.

**X coordinates**: Uses the Brandes-Kopf algorithm which:

1. Identifies "type 1" conflicts (inner segments that would cause edge crossings)
2. Performs four passes (up-left, up-right, down-left, down-right)
3. Computes median of the four placements for each node

This produces compact, balanced layouts with straight vertical edge segments where possible.

#### Phase 6: Route Edges

**Input**: Positioned nodes (including dummies)
**Output**: Edge paths as arrays of points

1. Collect coordinates from dummy nodes to form edge polylines
2. Remove dummy nodes from output
3. Restore reversed edges (swap source/target, reverse point order)
4. Optionally apply edge bundling or smoothing

---

## Key Data Structures

### layoutState

The central orchestrator that holds all intermediate state:

```go
type layoutState struct {
    // Original graph reference
    graph *Graph
    opts  Options

    // Working node set (includes dummies)
    nodes map[string]*layoutNode

    // Adjacency lists for fast traversal
    successors   map[string][]string  // outgoing edges
    predecessors map[string][]string  // incoming edges

    // Edge tracking
    edges         []*layoutEdge
    reversedEdges map[string]bool     // edges reversed for acyclicity

    // Layer structure (built in Phase 2-4)
    layers [][]string                 // layers[rank] = ordered node IDs

    // For dummy node generation
    dummyCounter int
}
```

### layoutNode

Extended node representation used internally:

```go
type layoutNode struct {
    id     string
    width  float64
    height float64

    // Assigned during layout
    rank  int      // layer number (Phase 2)
    order int      // position in layer (Phase 4)
    x     float64  // final X coordinate (Phase 5)
    y     float64  // final Y coordinate (Phase 5)

    // Dummy node tracking
    isDummy   bool
    edgeRef   *layoutEdge  // for dummies: the edge this dummy belongs to
}
```

### layoutEdge

Edge with layout metadata:

```go
type layoutEdge struct {
    from   string
    to     string

    // Set during layout
    reversed bool        // true if reversed for acyclicity
    points   []EdgePoint // final routed path

    // For edges split by dummies
    dummies []string     // ordered list of dummy node IDs
}
```

### Adjacency Representation

For fast graph traversal, we maintain both forward and reverse adjacency:

```go
// Building adjacency from edges
for _, e := range edges {
    successors[e.from] = append(successors[e.from], e.to)
    predecessors[e.to] = append(predecessors[e.to], e.from)
}

// Usage in algorithms
for _, succ := range state.successors[nodeID] {
    // process each successor
}
```

This provides O(1) access to neighbors, critical for the repeated graph traversals in crossing minimization.

---

## Algorithm Details

### Cycle Removal (DFS-based)

```go
func (s *layoutState) makeAcyclic() {
    visited := make(map[string]bool)
    inStack := make(map[string]bool)  // currently in DFS path

    var dfs func(id string)
    dfs = func(id string) {
        if visited[id] {
            return
        }
        visited[id] = true
        inStack[id] = true

        for _, succ := range s.successors[id] {
            if inStack[succ] {
                // Back edge found - reverse it
                s.reverseEdge(id, succ)
            } else {
                dfs(succ)
            }
        }

        inStack[id] = false
    }

    for id := range s.nodes {
        dfs(id)
    }
}
```

### Longest Path Ranking

```go
func (s *layoutState) longestPathRank() {
    // Find nodes with no predecessors (sources)
    sources := s.findSources()

    // BFS from sources, assigning ranks
    for _, src := range sources {
        s.nodes[src].rank = 0
    }

    // Topological order traversal
    for _, id := range s.topologicalSort() {
        node := s.nodes[id]
        for _, succ := range s.successors[id] {
            succNode := s.nodes[succ]
            // Successor must be at least one rank below
            if succNode.rank <= node.rank {
                succNode.rank = node.rank + 1
            }
        }
    }
}
```

### Barycenter Crossing Minimization

```go
func (s *layoutState) orderLayer(layerIdx int, moveDown bool) {
    layer := s.layers[layerIdx]

    // Calculate barycenter for each node
    barycenters := make(map[string]float64)
    for _, id := range layer {
        var neighbors []string
        if moveDown {
            neighbors = s.predecessors[id]  // look at layer above
        } else {
            neighbors = s.successors[id]    // look at layer below
        }

        if len(neighbors) == 0 {
            barycenters[id] = float64(s.nodes[id].order)
            continue
        }

        sum := 0.0
        for _, n := range neighbors {
            sum += float64(s.nodes[n].order)
        }
        barycenters[id] = sum / float64(len(neighbors))
    }

    // Sort layer by barycenter
    sort.Slice(layer, func(i, j int) bool {
        return barycenters[layer[i]] < barycenters[layer[j]]
    })

    // Update order values
    for i, id := range layer {
        s.nodes[id].order = i
    }
}
```

### Crossing Count

Used to evaluate layer orderings:

```go
func (s *layoutState) countCrossings() int {
    crossings := 0

    for layerIdx := 0; layerIdx < len(s.layers)-1; layerIdx++ {
        upper := s.layers[layerIdx]
        lower := s.layers[layerIdx+1]

        // Collect all edges between adjacent layers
        var edges [][2]int  // [upperOrder, lowerOrder]
        for _, uid := range upper {
            uOrder := s.nodes[uid].order
            for _, lid := range s.successors[uid] {
                if s.nodes[lid].rank == layerIdx+1 {
                    edges = append(edges, [2]int{uOrder, s.nodes[lid].order})
                }
            }
        }

        // Count inversions (crossings)
        for i := 0; i < len(edges); i++ {
            for j := i + 1; j < len(edges); j++ {
                // Crossing if edges swap order
                if (edges[i][0] < edges[j][0]) != (edges[i][1] < edges[j][1]) {
                    crossings++
                }
            }
        }
    }

    return crossings
}
```

---

## Extension Points

The modular design allows for algorithm substitution at each phase.

### Alternative Ranking Algorithms

To add a new ranking algorithm:

1. Implement the ranking interface in `rank.go`:

```go
type ranker interface {
    rank(s *layoutState)
}

// Example: Network Simplex
type networkSimplexRanker struct{}

func (r *networkSimplexRanker) rank(s *layoutState) {
    // 1. Build feasible spanning tree
    // 2. Calculate edge slacks
    // 3. Iterate: find negative slack edge, pivot
    // 4. Assign ranks from tree
}
```

2. Add to the `RankAlgorithm` enum and switch in `assignLayers()`:

```go
const (
    LongestPath RankAlgorithm = iota
    NetworkSimplex
    TightTree  // new algorithm
)
```

### Different Crossing Minimization Heuristics

The barycenter method can be swapped for alternatives:

```go
type orderingHeuristic interface {
    // Returns optimal order for nodes in a layer
    orderLayer(layer []string, neighbors func(string) []string) []string
}

// Median heuristic (often better than barycenter)
type medianHeuristic struct{}

func (h *medianHeuristic) orderLayer(layer []string, neighbors func(string) []string) []string {
    // Use median of neighbor positions instead of mean
}

// Sifting heuristic (more expensive but better results)
type siftingHeuristic struct{}
```

### Custom Coordinate Assignment

The Brandes-Kopf algorithm can be replaced:

```go
type positioner interface {
    assignX(s *layoutState)
    assignY(s *layoutState)
}

// Simpler left-to-right assignment
type simplePositioner struct{}

func (p *simplePositioner) assignX(s *layoutState) {
    for _, layer := range s.layers {
        x := 0.0
        for _, id := range layer {
            node := s.nodes[id]
            node.x = x + node.width/2
            x += node.width + s.opts.NodeSep
        }
    }
}

// Quadratic programming for optimal balance
type qpPositioner struct{}
```

### Adding New Options

The `Options` struct is designed for extension:

```go
type Options struct {
    // Existing
    Direction   Direction
    NodeSep     float64
    RankSep     float64
    Algorithm   RankAlgorithm

    // Future extensions
    EdgeSep     float64        // minimum spacing between edges
    Align       Alignment      // how to align nodes within layers
    Acyclicer   Acyclicer      // cycle removal strategy
    Ranker      Ranker         // custom ranking function
    Orderer     Orderer        // custom ordering function
}
```

---

## Testing Strategy

### Unit Tests by Phase

Each phase has isolated tests:

```go
// acyclic_test.go
func TestMakeAcyclic_SimpleLoop(t *testing.T) {
    // A → B → A should reverse one edge
}

func TestMakeAcyclic_DiamondWithCycle(t *testing.T) {
    // More complex cycle detection
}

// rank_test.go
func TestLongestPath_Linear(t *testing.T) {
    // A → B → C should have ranks 0, 1, 2
}

func TestLongestPath_Diamond(t *testing.T) {
    // A → B, A → C, B → D, C → D
    // D should be rank 2, not 3
}
```

### Integration Tests

Full pipeline tests with known-good outputs:

```go
func TestLayout_ThreeNodeChain(t *testing.T) {
    g := NewGraph()
    g.AddNode("a", NodeOptions{Width: 50, Height: 30})
    g.AddNode("b", NodeOptions{Width: 50, Height: 30})
    g.AddNode("c", NodeOptions{Width: 50, Height: 30})
    g.AddEdge("a", "b")
    g.AddEdge("b", "c")

    layout := g.Layout()

    // Verify top-to-bottom ordering
    assert(layout.Nodes["a"].Y < layout.Nodes["b"].Y)
    assert(layout.Nodes["b"].Y < layout.Nodes["c"].Y)

    // Verify vertical alignment
    assert(layout.Nodes["a"].X == layout.Nodes["b"].X)
    assert(layout.Nodes["b"].X == layout.Nodes["c"].X)
}
```

### Benchmark Tests

Performance regression tests:

```go
func BenchmarkLayout_50Nodes(b *testing.B) {
    g := generateRandomDAG(50, 100)  // 50 nodes, ~100 edges
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        g.Layout()
    }
}

func BenchmarkLayout_200Nodes(b *testing.B) {
    g := generateRandomDAG(200, 500)
    // ...
}
```

---

## Reference Implementation

Posit is informed by **dagre**, the JavaScript graph layout library. Key reference files in `_refs/dagre/lib/`:

| Dagre File | Posit Equivalent | Notes |
|------------|------------------|-------|
| `layout.js` | `state.go` | Phase orchestration |
| `acyclic.js` | `acyclic.go` | DFS cycle removal |
| `rank/index.js` | `rank.go` | Ranking dispatch |
| `rank/longest-path.js` | `rank.go` | Longest path impl |
| `normalize.js` | `normalize.go` | Dummy node handling |
| `order/index.js` | `order.go` | Crossing minimization |
| `position/index.js` | `position.go` | Coordinate assignment |
| `position/bk.js` | `position.go` | Brandes-Kopf algorithm |

Posit intentionally simplifies dagre's implementation by:

- Removing compound graph support (subgraphs)
- Removing edge label positioning
- Removing self-edge handling (treated as no-ops)
- Using simpler data structures (maps vs graphlib)

---

## Future Considerations

### Potential Enhancements

1. **Edge labels**: Support for labeling edges at midpoints
2. **Clustering**: Group related nodes visually
3. **Incremental layout**: Update layout when graph changes
4. **Spline routing**: Curved edges instead of polylines
5. **Port constraints**: Control which side of a node edges connect

### Non-Goals

- General force-directed layout (different algorithm family)
- 3D layout
- Interactive/animated layout
- Graph editing or manipulation beyond layout

---

## Quick Reference

### Minimum Viable Example

```go
package main

import (
    "fmt"
    "github.com/user/posit"
)

func main() {
    g := posit.NewGraph()

    // Add nodes with dimensions
    g.AddNode("users", posit.NodeOptions{Width: 100, Height: 40})
    g.AddNode("orders", posit.NodeOptions{Width: 100, Height: 40})
    g.AddNode("products", posit.NodeOptions{Width: 100, Height: 40})

    // Add directed edges (foreign key relationships)
    g.AddEdge("users", "orders")
    g.AddEdge("products", "orders")

    // Compute layout
    layout := g.Layout(posit.Options{
        Direction: posit.TopToBottom,
        NodeSep:   50,
        RankSep:   80,
    })

    // Use positions for rendering
    for id, node := range layout.Nodes {
        fmt.Printf("%s: (%.1f, %.1f)\n", id, node.X, node.Y)
    }

    for id, edge := range layout.Edges {
        fmt.Printf("Edge %s: %d points\n", id, len(edge.Points))
    }
}
```

### Key Invariants

After each phase, these invariants hold:

| Phase | Invariant |
|-------|-----------|
| 1. Acyclic | No back-edges in successor traversal |
| 2. Rank | All edges go from lower rank to higher rank |
| 3. Normalize | All edges span exactly 1 rank |
| 4. Order | Crossing count is locally minimal |
| 5. Position | No node overlaps, respects NodeSep/RankSep |
| 6. Route | All edges have valid point arrays |
