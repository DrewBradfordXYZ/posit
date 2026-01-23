# posit

Pure Go implementation of the Sugiyama algorithm for layered graph layout.

Posit computes X/Y positions for nodes in a directed graph, arranging them in hierarchical layers with minimal edge crossings. Zero dependencies, deterministic output.

## Features

- Zero external dependencies (standard library only)
- Deterministic layout (same input always produces same output)
- Four layout directions (TopToBottom, LeftToRight, BottomToTop, RightToLeft)
- Ports with flexible constraints (FixedPos, FixedSide, FixedOrder, Free, FixedOffset)
- Port axis constraints (horizontal-only, vertical-only, any)
- Edge labels with automatic positioning
- Compound graphs (nested clusters)
- Orthogonal and polyline edge routing
- Incremental layout (minimal update from prior layout)
- Rank constraints (pin to first/last layer, group to same layer)
- Order constraints (group nodes adjacently, control priority)
- Ranking algorithms: LongestPath, TightTree, NetworkSimplex
- Cycle removal: DFS, Greedy FAS
- Self-loop support
- Disconnected component packing
- Multi-edge support

## Installation

```bash
go get github.com/DrewBradfordXYZ/posit
```

## Usage

```go
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
```

## License

MIT
