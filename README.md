# posit

Pure Go implementation of the Sugiyama algorithm for layered graph layout.

Posit computes X/Y positions for nodes in a directed graph, arranging them in hierarchical layers with minimal edge crossings. Zero dependencies, deterministic output.

## Features

- Zero external dependencies (standard library only)
- Deterministic layout (same input always produces same output)
- Edge labels with automatic positioning
- Ports (fixed connection points on nodes)
- Compound graphs (nested clusters)
- Self-loop support with curved paths
- Orthogonal edge routing
- Incremental layout (minimal update from prior layout)
- Rank constraints (pin nodes to first/last layer, group to same layer)
- Order constraints (group nodes adjacently, control priority)
- Multiple ranking algorithms (LongestPath, TightTree, NetworkSimplex)
- Multiple cycle removal algorithms (DFS, Greedy FAS)
- Four layout directions (TopToBottom, LeftToRight, BottomToTop, RightToLeft)
- Disconnected component packing (horizontal or vertical)

## Installation

```bash
go get github.com/DrewBradfordXYZ/posit
```

## Usage

```go
package main

import (
    "fmt"
    "github.com/DrewBradfordXYZ/posit"
)

func main() {
    g := posit.NewGraph()

    g.AddNode("a", posit.NodeOptions{Width: 100, Height: 50})
    g.AddNode("b", posit.NodeOptions{Width: 100, Height: 50})
    g.AddNode("c", posit.NodeOptions{Width: 100, Height: 50})

    g.MustAddEdge("a", "b")
    g.MustAddEdge("a", "c")

    layout := g.Layout()

    for id, node := range layout.Nodes {
        fmt.Printf("%s: (%.0f, %.0f)\n", id, node.X, node.Y)
    }
}
```

## Configuration

```go
layout := g.Layout(posit.Options{
    Direction: posit.TopToBottom, // or LeftToRight, BottomToTop, RightToLeft
    NodeSep:   50,               // Horizontal spacing between nodes
    RankSep:   100,              // Vertical spacing between layers
    Algorithm: posit.LongestPath, // or TightTree, NetworkSimplex
})
```

## Performance

| Profile | Time |
|---------|------|
| Large (500n/1000e) | 970ms |
| Dense (100n/2000e) | 1335ms |
| Wide (100x5) | 30ms |
| Deep (200-chain) | <1ms |
| Medium (100n/200e) | 32ms |

Run `task bench` for the full report.

## Algorithm

Posit implements the Sugiyama framework:

1. **Cycle Removal** — DFS or greedy feedback arc set
2. **Layer Assignment** — Longest path, tight tree, or network simplex
3. **Crossing Minimization** — Barycenter heuristic with adjacent exchange (ILS), inner segment optimization
4. **Coordinate Assignment** — Brandes-Kopf (small graphs) or simple centering (large graphs)
5. **Edge Routing** — Polyline through dummy nodes or orthogonal channel routing

## License

MIT
