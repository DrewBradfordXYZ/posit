# Phase 5: Coordinate Assignment (Positioning)

**File:** `position.go`

## Table of Contents

- [Goal](#goal)
- [Coordinate Convention](#coordinate-convention)
- [Y Coordinate Assignment](#y-coordinate-assignment)
- [X Coordinate Assignment](#x-coordinate-assignment)
- [Simple X Assignment](#simple-x-assignment)
- [Brandes-Kopf Algorithm](#brandes-kopf-algorithm)
- [Implementation](#implementation)
- [Handling Node Dimensions](#handling-node-dimensions)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Assign actual **X and Y pixel coordinates** to each node, respecting:
- Layer assignments (determines Y)
- Node ordering within layers (determines X)
- Minimum spacing constraints (`nodeSep`, `rankSep`)
- Edge straightness (prefer vertical edges where possible)

### Input
- Ordered layered graph (from Phase 4)
- Each node has `rank` and `order`

### Output
- Each node has `x` and `y` coordinates

---

## Coordinate Convention

Posit uses a **top-left coordinate convention**:

```
(0,0) ────────────────────────→ X+
  │
  │    ┌─────────────┐
  │    │             │  ← node.X, node.Y = top-left corner
  │    │    Node     │
  │    │             │
  │    └─────────────┘
  ▼
  Y+
```

| Property | Convention |
|----------|------------|
| Origin | Top-left of layout |
| X axis | Increases to the right |
| Y axis | Increases downward |
| Node position | Top-left corner of node |
| Units | Same as input dimensions (typically pixels) |

This matches standard graphics APIs (SVG, Canvas, CSS) and layout algorithms (dagre, ELK).

---

## Y Coordinate Assignment

Y coordinates are straightforward — determined by rank and `rankSep`:

### Algorithm

```
Algorithm AssignY(G):
    y = 0

    for rank = 0 to maxRank:
        layer = layers[rank]

        // Find max height in this layer
        maxHeight = max(node.height for node in layer)

        // Assign Y to top of node (top-left convention)
        for each node in layer:
            node.y = y

        // Move to next layer
        y += maxHeight + rankSep
```

### Implementation

```go
// assignYCoordinates assigns Y based on rank and RankSep.
func (s *layoutState) assignYCoordinates() {
    y := 0.0

    for rank := 0; rank < len(s.layers); rank++ {
        layer := s.layers[rank]
        if len(layer) == 0 {
            continue
        }

        // Find max height in this layer
        maxHeight := 0.0
        for _, id := range layer {
            node := s.nodes[id]
            if node.height > maxHeight {
                maxHeight = node.height
            }
        }

        // Assign Y to top of node
        for _, id := range layer {
            s.nodes[id].y = y
        }

        // Move to next layer
        if rank < len(s.layers)-1 {
            y += maxHeight + s.opts.RankSep
        }
    }
}
```

---

## X Coordinate Assignment

X assignment is more complex because we want:
1. Nodes spaced by at least `nodeSep`
2. Edges as vertical as possible
3. Balanced layout (not all pushed to one side)

Two approaches are available:
1. **Simple placement** — Fast, left-to-right with centering
2. **Brandes-Kopf** — Optimal, considers edge alignment

---

## Simple X Assignment

A straightforward approach suitable for most cases:

### Algorithm

```
Algorithm SimpleAssignX(G):
    // Pass 1: Place nodes left-to-right
    for each layer:
        x = 0
        for each node in layer (by order):
            node.x = x
            x += node.width + nodeSep

    // Pass 2: Center all layers
    maxWidth = max layer width
    for each layer:
        layerWidth = rightmost node edge
        offset = (maxWidth - layerWidth) / 2
        for each node in layer:
            node.x += offset
```

### Implementation

```go
// assignXCoordinatesSimple uses simple left-to-right placement.
func (s *layoutState) assignXCoordinatesSimple() {
    // Pass 1: Simple left-to-right placement
    for _, layer := range s.layers {
        x := 0.0
        for _, id := range layer {
            node := s.nodes[id]
            node.x = x
            x += node.width + s.opts.NodeSep
        }
    }

    // Pass 2: Center layers
    s.centerLayers()
}

// centerLayers adjusts X so all layers are centered.
func (s *layoutState) centerLayers() {
    // Find max layer width
    maxWidth := 0.0
    for _, layer := range s.layers {
        if len(layer) == 0 {
            continue
        }
        lastID := layer[len(layer)-1]
        lastNode := s.nodes[lastID]
        width := lastNode.x + lastNode.width
        if width > maxWidth {
            maxWidth = width
        }
    }

    // Center each layer
    for _, layer := range s.layers {
        if len(layer) == 0 {
            continue
        }
        lastID := layer[len(layer)-1]
        lastNode := s.nodes[lastID]
        layerWidth := lastNode.x + lastNode.width
        offset := (maxWidth - layerWidth) / 2

        for _, id := range layer {
            s.nodes[id].x += offset
        }
    }
}
```

---

## Brandes-Kopf Algorithm

The Brandes-Kopf algorithm produces optimal X coordinates by considering edge alignment. It computes four different alignments and takes their median.

### Reference

> Ulrik Brandes and Boris Kopf. "Fast and Simple Horizontal Coordinate Assignment." *Graph Drawing 2001*.

### Key Concepts

1. **Vertical Alignment**: Group nodes that should be vertically aligned into "blocks"
2. **Four Alignments**: Compute X for each direction combination:
   - Up-Left (ul)
   - Up-Right (ur)
   - Down-Left (dl)
   - Down-Right (dr)
3. **Median**: Final X is the median of the four alignments

### Type-1 Conflicts

A **Type-1 conflict** occurs when aligning a non-inner segment would cross an inner segment (edge between two dummies).

```
Inner segment (between dummies):

Rank i:     [D1]          [D2]
              │              │
              │  ← inner     │
              │  segment     │
Rank i+1:   [D3]          [D4]

Type-1 conflict:

Rank i:     [D1]    A      [D2]
              │ \  / │        │
              │  \/  │        │
              │  /\  │        │
              │ /  \ │        │
Rank i+1:   [D3]    B      [D4]

If we try to align A with D3, it would cross D1→D3.
We mark this as a conflict and don't align them.
```

### Pseudocode

```
Algorithm BrandesKopf(G):
    // Find conflicts
    conflicts = findType1Conflicts(G)

    // Compute four alignments
    xss = {}
    for vert in ["up", "down"]:
        for horiz in ["left", "right"]:
            root, align = verticalAlignment(G, conflicts, vert, horiz)
            xs = horizontalCompaction(G, root, align, horiz)
            xss[vert + horiz] = xs

    // Align all to smallest width
    alignToSmallestWidth(xss)

    // Final X = median of four
    for each node v:
        values = [xss["ul"][v], xss["ur"][v], xss["dl"][v], xss["dr"][v]]
        sort(values)
        v.x = (values[1] + values[2]) / 2  // Median of 4 = avg of middle 2
```

---

## Implementation

### Main Entry Point

```go
// assignCoordinates computes X and Y positions for all nodes.
func (s *layoutState) assignCoordinates() {
    s.assignYCoordinates()

    // Choose X assignment method
    if len(s.nodes) > 100 {
        s.assignXCoordinatesSimple()  // Fast for large graphs
    } else {
        s.assignXCoordinatesBK()      // Optimal for smaller graphs
    }
}
```

### Brandes-Kopf Implementation

```go
// assignXCoordinatesBK uses the Brandes-Kopf algorithm.
func (s *layoutState) assignXCoordinatesBK() {
    // Find type-1 conflicts
    conflicts := s.findType1Conflicts()

    // Compute four alignments
    xss := make(map[string]map[string]float64)

    for _, vert := range []string{"u", "d"} {
        layering := s.layers
        if vert == "d" {
            layering = s.reverseLayers()
        }

        for _, horiz := range []string{"l", "r"} {
            adjustedLayering := layering
            if horiz == "r" {
                adjustedLayering = s.reverseLayerOrders(layering)
            }

            neighborFn := s.predecessors
            if vert == "d" {
                neighborFn = s.successors
            }

            root, align := s.verticalAlignment(adjustedLayering, conflicts, neighborFn)
            xs := s.horizontalCompaction(adjustedLayering, root, align)

            if horiz == "r" {
                // Negate X values
                for id := range xs {
                    xs[id] = -xs[id]
                }
            }

            xss[vert+horiz] = xs
        }
    }

    // Align to smallest width
    s.alignCoordinatesToSmallest(xss)

    // Take median of four alignments
    for id, node := range s.nodes {
        values := []float64{
            xss["ul"][id],
            xss["ur"][id],
            xss["dl"][id],
            xss["dr"][id],
        }
        sort.Float64s(values)
        // Median of 4 = average of middle 2
        node.x = (values[1] + values[2]) / 2
    }
}
```

### Vertical Alignment

```go
// verticalAlignment creates blocks of vertically aligned nodes.
func (s *layoutState) verticalAlignment(
    layering [][]string,
    conflicts map[conflictKey]bool,
    neighborFn func(string) []string,
) (root map[string]string, align map[string]string) {

    root = make(map[string]string)
    align = make(map[string]string)

    // Initialize: each node is its own block
    for id := range s.nodes {
        root[id] = id
        align[id] = id
    }

    // Process layers
    for _, layer := range layering {
        prevIdx := -1

        for _, v := range layer {
            neighbors := neighborFn(v)
            if len(neighbors) == 0 {
                continue
            }

            // Sort neighbors by order
            sort.Slice(neighbors, func(i, j int) bool {
                return s.nodes[neighbors[i]].order < s.nodes[neighbors[j]].order
            })

            // Find median neighbor(s)
            mid := (len(neighbors) - 1) / 2
            for m := mid; m <= mid+(len(neighbors)%2); m++ {
                if align[v] != v {
                    continue  // Already aligned
                }

                w := neighbors[m]
                wOrder := s.nodes[w].order

                // Check for conflict
                key := conflictKey{v, w}
                if conflicts[key] {
                    continue
                }

                // Align if no crossing with previous alignments
                if prevIdx < wOrder {
                    align[w] = v
                    root[v] = root[w]
                    align[v] = root[v]
                    prevIdx = wOrder
                }
            }
        }
    }

    return root, align
}
```

### Horizontal Compaction

```go
// horizontalCompaction assigns X coordinates respecting alignment blocks.
func (s *layoutState) horizontalCompaction(
    layering [][]string,
    root, align map[string]string,
) map[string]float64 {

    xs := make(map[string]float64)
    sink := make(map[string]string)
    shift := make(map[string]float64)

    // Initialize
    for id := range s.nodes {
        sink[id] = id
        shift[id] = math.Inf(1)
    }

    // Compute X for roots
    for _, layer := range layering {
        for _, v := range layer {
            if root[v] == v {
                s.placeBlock(v, xs, sink, shift, root, align, layering)
            }
        }
    }

    // Apply shifts
    for _, layer := range layering {
        for _, v := range layer {
            xs[v] = xs[root[v]]
            if s := shift[sink[root[v]]]; s < math.Inf(1) {
                xs[v] += s
            }
        }
    }

    return xs
}

// placeBlock positions a block of aligned nodes.
func (s *layoutState) placeBlock(
    v string,
    xs map[string]float64,
    sink, shift map[string]string,
    root, align map[string]string,
    layering [][]string,
) {
    if _, ok := xs[v]; ok {
        return  // Already placed
    }

    xs[v] = 0

    w := v
    for {
        // Find predecessor in same layer
        order := s.nodes[w].order
        if order > 0 {
            layer := layering[s.nodes[w].rank]
            pred := layer[order-1]
            predRoot := root[pred]

            s.placeBlock(predRoot, xs, sink, shift, root, align, layering)

            if sink[v] == v {
                sink[v] = sink[predRoot]
            }

            sep := s.separation(pred, w)
            if sink[v] != sink[predRoot] {
                shift[sink[predRoot]] = math.Min(
                    shift[sink[predRoot]],
                    xs[v]-xs[predRoot]-sep,
                )
            } else {
                xs[v] = math.Max(xs[v], xs[predRoot]+sep)
            }
        }

        w = align[w]
        if w == v {
            break
        }
    }
}
```

---

## Handling Node Dimensions

### Separation Calculation

The minimum distance between two adjacent nodes:

```go
// separation calculates required horizontal gap between adjacent nodes.
func (s *layoutState) separation(leftID, rightID string) float64 {
    left := s.nodes[leftID]
    right := s.nodes[rightID]

    // For dummy nodes, use smaller separation (edge separation)
    sep := s.opts.NodeSep
    if left.isDummy || right.isDummy {
        sep = s.opts.NodeSep / 2  // Or use EdgeSep if available
    }

    return left.width/2 + sep + right.width/2
}
```

### Why Width/2?

With top-left coordinates, we need to account for the full width:
```
Node A                Node B
┌─────────┐          ┌─────────┐
│    *────┼──sep────┼──*      │
└─────────┘          └─────────┘
     │                    │
     └─── A.width ────────┘
                     └─ B.width ─┘

Distance between left edges = A.width + sep + B.width
But if we store center positions during calculation,
separation = A.width/2 + sep + B.width/2
```

---

## Complexity Analysis

### Simple Assignment

| Operation | Time | Space |
|-----------|------|-------|
| Y assignment | O(V) | O(1) |
| X placement | O(V) | O(1) |
| Centering | O(L × n) | O(1) |
| **Total** | **O(V)** | **O(1)** |

### Brandes-Kopf

| Operation | Time | Space |
|-----------|------|-------|
| Find conflicts | O(L × E) | O(E) |
| Vertical alignment | O(V + E) per alignment | O(V) |
| Horizontal compaction | O(V) per alignment | O(V) |
| Four alignments | O(4 × (V + E)) | O(V) |
| **Total** | **O(V + E + L × E)** | **O(V)** |

---

## Testing

### Test Cases

```go
func TestPosition_NoOverlap(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")

    layout := g.Layout()

    // Check all pairs for overlap
    for id1, n1 := range layout.Nodes {
        for id2, n2 := range layout.Nodes {
            if id1 >= id2 {
                continue
            }
            if overlaps(n1, n2) {
                t.Errorf("Nodes %s and %s overlap", id1, id2)
            }
        }
    }
}

func TestPosition_MinimumSpacing(t *testing.T) {
    opts := Options{NodeSep: 50, RankSep: 100}

    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("A", "C")

    layout := g.Layout(opts)

    // B and C in same layer - check horizontal spacing
    b := layout.Nodes["B"]
    c := layout.Nodes["C"]

    if math.Abs(b.Y-c.Y) < 1 {  // Same layer
        gap := math.Abs(b.X-c.X) - b.Width
        if gap < opts.NodeSep-1 {
            t.Errorf("Horizontal gap %.1f < NodeSep %.1f", gap, opts.NodeSep)
        }
    }

    // A and B in different layers - check vertical spacing
    a := layout.Nodes["A"]
    vertGap := b.Y - (a.Y + a.Height)
    if vertGap < opts.RankSep-1 {
        t.Errorf("Vertical gap %.1f < RankSep %.1f", vertGap, opts.RankSep)
    }
}

func TestPosition_DifferentDirections(t *testing.T) {
    tests := []struct {
        dir      Direction
        checkFn  func(a, b NodeLayout) bool
    }{
        {TopToBottom, func(a, b NodeLayout) bool { return a.Y < b.Y }},
        {LeftToRight, func(a, b NodeLayout) bool { return a.X < b.X }},
        {BottomToTop, func(a, b NodeLayout) bool { return a.Y > b.Y }},
        {RightToLeft, func(a, b NodeLayout) bool { return a.X > b.X }},
    }

    for _, tt := range tests {
        t.Run(tt.dir.String(), func(t *testing.T) {
            g := NewGraph()
            g.AddNode("A", NodeOptions{Width: 100, Height: 50})
            g.AddNode("B", NodeOptions{Width: 100, Height: 50})
            g.AddEdge("A", "B")

            layout := g.Layout(Options{Direction: tt.dir})

            if !tt.checkFn(layout.Nodes["A"], layout.Nodes["B"]) {
                t.Errorf("Direction %v not respected", tt.dir)
            }
        })
    }
}

func TestPosition_ValidCoordinates(t *testing.T) {
    g := buildRandomDAG(50, 75)
    layout := g.Layout()

    for id, node := range layout.Nodes {
        if math.IsNaN(node.X) || math.IsInf(node.X, 0) {
            t.Errorf("Node %s has invalid X: %v", id, node.X)
        }
        if math.IsNaN(node.Y) || math.IsInf(node.Y, 0) {
            t.Errorf("Node %s has invalid Y: %v", id, node.Y)
        }
        if node.X < 0 || node.Y < 0 {
            t.Errorf("Node %s has negative coordinates: (%v, %v)",
                id, node.X, node.Y)
        }
    }
}
```

---

## Visual Examples

### Example 1: Simple Layout

```
INPUT (after ordering):

Rank 0: [A]
Rank 1: [B, C]
Rank 2: [D]

Node dimensions: all 100×50
NodeSep: 50, RankSep: 100


Y ASSIGNMENT:

Rank 0: y = 0
  A.y = 0

Rank 1: y = 50 + 100 = 150
  B.y = 150
  C.y = 150

Rank 2: y = 150 + 50 + 100 = 300
  D.y = 300


X ASSIGNMENT (simple):

Rank 0:
  A.x = 0

Rank 1:
  B.x = 0
  C.x = 100 + 50 = 150

Rank 2:
  D.x = 0


CENTERING:

Layer widths:
  Rank 0: 100
  Rank 1: 250 (0 to 250)
  Rank 2: 100

Max width: 250

Adjusted:
  A.x = 0 + (250-100)/2 = 75
  B.x = 0 + 0 = 0
  C.x = 150 + 0 = 150
  D.x = 0 + (250-100)/2 = 75


FINAL POSITIONS:

    ┌─────────┐
    │ A (75,0)│
    └─────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌─────────┐ ┌─────────┐
│B (0,150)│ │C(150,150)
└─────────┘ └─────────┘
    │         │
    └────┬────┘
         ▼
    ┌─────────┐
    │D (75,300)
    └─────────┘
```

### Example 2: Brandes-Kopf Alignment

```
Consider edge alignment:

Rank 0:    A          B
            \        /
             \      /
              \    /
               \  /
Rank 1:         C

With simple assignment, C might not align with either A or B.
With Brandes-Kopf, C will be positioned at the median of A and B,
creating more visually balanced edges.

Before BK:        After BK:
A    B    C       A    C    B
 \  / \  /         \   |   /
  \/   \/           \  |  /
  /\   /\            \ | /
 /  \ /  \            \|/
D    E    F            E
```

---

## Post-Conditions

After this phase completes:

1. ✅ Every node has valid `x` and `y` coordinates (non-negative, finite)
2. ✅ No nodes overlap (respecting NodeSep and RankSep)
3. ✅ Dummy nodes have coordinates (needed for edge routing)
4. ✅ Coordinates use top-left convention

---

## Next Phase

→ [Phase 6: Edge Routing](./PHASE_6_EDGE_ROUTING.md)
