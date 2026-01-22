# posit

Pure Go implementation of the Sugiyama algorithm for layered graph layout.

Posit computes X/Y positions for nodes in a directed graph, arranging them in hierarchical layers with minimal edge crossings.

## Features

- Pure Go, no CGO or external dependencies
- Sugiyama/layered layout algorithm
- Optimized for DAGs (directed acyclic graphs)
- Suitable for schema diagrams, flowcharts, dependency graphs

## Installation

```bash
go get github.com/DrewBradfordXYZ/posit
```

## Usage

```go
package main

import "github.com/DrewBradfordXYZ/posit"

func main() {
    g := posit.NewGraph()

    // Add nodes
    g.AddNode("a", posit.NodeOptions{Width: 100, Height: 50})
    g.AddNode("b", posit.NodeOptions{Width: 100, Height: 50})
    g.AddNode("c", posit.NodeOptions{Width: 100, Height: 50})

    // Add edges (parent -> child)
    g.AddEdge("a", "b")
    g.AddEdge("a", "c")

    // Compute layout
    layout := g.Layout()

    // Access positions
    for id, pos := range layout.Nodes {
        fmt.Printf("%s: (%.0f, %.0f)\n", id, pos.X, pos.Y)
    }
}
```

## Algorithm

Posit implements the Sugiyama framework:

1. **Cycle Removal** - Reverse edges to make the graph acyclic
2. **Layer Assignment** - Assign nodes to horizontal layers
3. **Crossing Minimization** - Reorder nodes within layers to reduce edge crossings
4. **Coordinate Assignment** - Compute final X/Y positions
5. **Edge Routing** - Route edges through dummy nodes

## Configuration

```go
layout := g.Layout(posit.Options{
    RankDir:   posit.TopToBottom, // or LeftToRight, BottomToTop, RightToLeft
    NodeSep:   50,                // Horizontal spacing between nodes
    RankSep:   100,               // Vertical spacing between layers
    Algorithm: posit.LongestPath, // or NetworkSimplex
})
```

## License

MIT
