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
├── posit.go          # Public API: Graph, Layout, Options, types, constants
├── state.go          # Internal layoutState struct and edge/node tracking
├── acyclic.go        # Phase 1: Cycle removal dispatcher (DFS/Greedy)
├── greedy_fas.go     # Phase 1: Greedy feedback arc set algorithm
├── rank.go           # Phase 2: Layer assignment dispatcher and constraints
├── simplex.go        # Phase 2: Network simplex algorithm for ranking
├── normalize.go      # Phase 3: Dummy node insertion for long edges
├── order.go          # Phase 4: Crossing minimization (barycenter heuristic)
├── position.go       # Phase 5: X/Y coordinate assignment (Brandes-Kopf, Network Simplex)
├── boundary.go       # Phase 5: Node boundary intersection geometry
├── overlap.go        # Phase 5: Cross-layer overlap resolution
├── port.go           # Phase 5: Port offset computation and assignment
├── route.go          # Phase 6: Edge routing, self-loops, label collisions
├── components.go     # Component detection and packing
└── direction.go      # Layout direction transforms (LR, RL, BT, TB)
```

### File Responsibilities

| File | Lines | Purpose |
|------|------:|---------:|
| `posit.go` | 990 | Public types (Graph, Layout, Options), builder methods, entry point |
| `state.go` | 276 | `layoutState` struct, `edgeKey`, `layoutNode` with phase outputs |
| `acyclic.go` | 134 | DFS-based cycle detection, edge reversal, self-loop handling |
| `greedy_fas.go` | 172 | Eades/Lin/Smyth greedy heuristic for feedback arc set minimization |
| `rank.go` | 292 | Ranking dispatch, rank constraints (RankGroup, RankMin/Max), cluster ranks |
| `simplex.go` | 2,082 | Network simplex algorithm: spanning tree, cut values, pivoting |
| `normalize.go` | 171 | Dummy node insertion/removal for multi-layer edge splitting |
| `order.go` | 1,235 | Layer sweeps, barycenter/median calculation, crossing count, cluster adjacency |
| `position.go` | 591 | Brandes-Kopf algorithm, Y stacking, network simplex X placement |
| `boundary.go` | 433 | `Rect` type, line-rectangle intersection, edge attachment geometry |
| `overlap.go` | 264 | Cross-layer node overlap detection and resolution |
| `port.go` | 192 | Port constraint modes (FixedPos, FixedSide, FixedOrder, Free), offset computation |
| `route.go` | 1,174 | Edge path construction, orthogonal routing, self-loops, label collision resolution |
| `components.go` | 153 | Disconnected component detection via union-find, bounding box packing |
| `direction.go` | 142 | Coordinate transforms for TopToBottom, LeftToRight, BottomToTop, RightToLeft |

**Total: ~8,300 lines** (excluding tests)

---

## Data Flow

The algorithm transforms data through ten phases, with pre- and post-processing for direction handling and compound graphs:

```
┌─────────────────────┐
│    Input Graph      │  User-provided nodes and edges
│    (posit.Graph)    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   layoutState       │  Internal representation with adjacency lists
│   initialization    │  newLayoutState()
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Pre-transform      │  adjustForDirection()
│  Direction Setup    │  Swap W/H for LR/RL layouts
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 1: Acyclic    │  makeAcyclic()
│                     │  Remove cycles, extract self-loops
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 2: Rank       │  assignLayers()
│                     │  Assign node.rank, apply constraints
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 3: Normalize  │  addDummyNodes(), markInteriorDummies()
│                     │  Split long edges with dummy nodes
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 4: Order      │  minimizeCrossings()
│                     │  Minimize crossings, cluster adjacency
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 5: Position   │  assignCoordinates()
│                     │  Compute X/Y for all nodes
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 6: Overlap    │  resolveCrossLayerOverlaps()
│  Resolution         │  Fix node overlaps across layers
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 7: Ports      │  computePortOffsets()
│                     │  Compute auto port positions
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 8: Components │  packComponents()
│                     │  Arrange disconnected subgraphs
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Phase 9: Route      │  routeEdges()
│                     │  Build edge paths, restore reversals
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Post-transform     │  undoDirectionAdjustment()
│  Direction Finalize │  Convert to user coordinate space
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Cluster Sizing     │  adjustClusters()
│                     │  Size compound nodes to contain children
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Output Layout     │  Final positions for rendering
│   (posit.Layout)    │
└─────────────────────┘
```

### Phase Details

#### Pre-transform: Direction Setup

**File**: `direction.go` | **Method**: `adjustForDirection()`

Prepares the graph for layout in non-default directions. For `LeftToRight` and `RightToLeft` layouts, swaps node width and height so the core algorithm (which always works top-to-bottom internally) produces correct results.

```go
// For LR/RL directions, the layout treats the "rank" axis as horizontal
// by swapping dimensions before processing
node.width, node.height = node.height, node.width
```

#### Phase 1: Make Acyclic

**File**: `acyclic.go` | **Method**: `makeAcyclic()`

**Input**: Directed graph (may contain cycles and self-loops)
**Output**: Directed acyclic graph (DAG)

Two algorithms available:

1. **DFS Acyclicer** (default): Uses depth-first search to identify back-edges (edges pointing to ancestors in the DFS tree). These edges are reversed by swapping direction and marking for later restoration.

2. **Greedy Acyclicer**: Uses the Eades/Lin/Smyth heuristic for weighted feedback arc sets. Produces better results for graphs with edge weights where minimizing reversed edge weight matters.

Self-loops (edges where source equals target) are extracted and stored separately for curved path rendering in Phase 9.

```
Before:  A -> B -> C -> A (cycle), D -> D (self-loop)
After:   A -> B -> C <- A (reversed), self-loops tracked separately
```

#### Phase 2: Assign Layers (Ranking)

**File**: `rank.go`, `simplex.go` | **Method**: `assignLayers()`

**Input**: DAG
**Output**: Each node has a `rank` (layer number, 0-indexed)

Three ranking algorithms available:

1. **Longest Path** (default): Works backwards from sink nodes, assigning each node the minimum rank that satisfies edge constraints. Fast O(V+E) but may create more layers than necessary.

2. **Tight Tree**: Uses longest-path as initial solution, then builds a feasible spanning tree to tighten edge lengths. Middle ground between speed and compactness.

3. **Network Simplex**: Optimal layer assignment minimizing total edge length. Uses the simplex algorithm on the graph's spanning tree to iteratively improve ranks. Most compact results but slower for large graphs.

After initial ranking, several post-processing steps apply:

- **RankGroup constraints**: Moves group members to the minimum rank among them
- **RankMin/RankMax constraints**: Pins nodes to first or last layer
- **Cluster rank constraints**: Ensures cluster children occupy consecutive ranks
- **Rank normalization**: Shifts all ranks so minimum is 0

```
Rank 0:  [A]           <- RankMin nodes forced here
Rank 1:  [B, C]        <- Grouped nodes share same rank
Rank 2:  [D, E]
Rank 3:  [F]           <- RankMax nodes forced here
```

#### Phase 3: Normalize (Dummy Nodes)

**File**: `normalize.go` | **Methods**: `addDummyNodes()`, `markInteriorDummies()`

**Input**: DAG with ranks
**Output**: DAG where all edges span exactly 1 layer

Long edges (spanning multiple layers) are split by inserting "dummy" nodes. This ensures every edge connects adjacent layers, which is required for crossing minimization and coordinate assignment.

```
Before:  A (rank 0) ────────────────→ D (rank 3)

After:   A (rank 0) → _dummy_1 (rank 1) → _dummy_2 (rank 2) → D (rank 3)
```

For edges with labels, a "label dummy" is created at the appropriate position (center, near source, or near target) with dimensions matching the label size. This reserves space for the label in crossing minimization and coordinate assignment.

After dummy insertion, `markInteriorDummies()` identifies dummy nodes whose both neighbors are also dummies. These "interior dummies" can skip adjacent exchange optimization since their position is fully determined by their single incoming and outgoing edges.

#### Phase 4: Minimize Crossings (Ordering)

**File**: `order.go` | **Method**: `minimizeCrossings()`

**Input**: Layered graph with dummy nodes
**Output**: Each node has an `order` (position within its layer)

Uses a multi-pass approach combining several techniques:

1. **Barycenter Heuristic with Layer Sweeps**:
   - Sweep down: Position each node at the barycenter (weighted average) of its predecessors
   - Sweep up: Position each node at the barycenter of its successors
   - Repeat up to 24 iterations, stopping when no improvement for 4 consecutive passes

2. **Adjacent Exchange**:
   - After each sweep, attempt local improvements by swapping adjacent node pairs
   - Uses efficient O(degree^2) incremental crossing count (not O(E^2) full recount)
   - Includes stochastic perturbation to escape local minima

3. **Order Constraints**:
   - OrderGroup: Groups nodes to be placed adjacently within their layer
   - OrderPriority: Controls ordering within an OrderGroup

4. **Optional Reverse Ordering**:
   - When `TryReverseOrdering` is enabled, runs minimization in both layer directions
   - Keeps the result with fewer crossings (roughly doubles ordering time)

5. **Cluster Adjacency**:
   - `enforceClusterAdjacency()` ensures children of each cluster are adjacent within layers

6. **Port-Level Crossing Minimization**:
   - `minimizePortCrossings()` pre-assigns sides for PortFree ports and computes optimal ordering for ports on each side

```
Before (4 crossings):     After (0 crossings):
Layer 0:  A   B           Layer 0:  A   B
           ╲ ╱                       │   │
            ╳                        │   │
           ╱ ╲                       │   │
Layer 1:  C   D           Layer 1:  C   D
```

#### Phase 5: Assign Coordinates

**File**: `position.go`, `simplex.go` | **Method**: `assignCoordinates()`

**Input**: Ordered layers
**Output**: X/Y coordinates for each node

**Y coordinates**: `assignYCoordinates()` stacks layers with `RankSep` spacing. Each layer's Y is the previous layer's bottom plus the configured separation.

**X coordinates**: Three algorithms available based on `XCoordAlgorithm` option:

1. **Brandes-Kopf** (default for small graphs, < BKThreshold nodes):
   - Identifies "type 1" conflicts (inner segments that would cause crossings)
   - Performs four alignment passes: up-left, up-right, down-left, down-right
   - Each pass aligns nodes vertically with their "median" neighbor
   - Final position is the median of the four pass results
   - Produces compact, balanced layouts with straight vertical edges where possible

2. **Simple Centering** (default for large graphs, >= BKThreshold nodes):
   - Places nodes left-to-right within each layer with `NodeSep` spacing
   - Centers all layers relative to the widest layer
   - Fast but less optimal alignment

3. **Network Simplex** (when `XCoordAlgorithm: XNetworkSimplex`):
   - Builds auxiliary graph with separation constraints as edges
   - Runs network simplex to find globally optimal X positions
   - Supports `PreventStacking` option to add cross-layer separation constraints
   - Produces the most balanced results but slower for large graphs

#### Phase 6: Cross-Layer Overlap Resolution

**File**: `overlap.go` | **Method**: `resolveCrossLayerOverlaps()`

**Input**: Positioned nodes
**Output**: Adjusted positions with no cross-layer node overlaps

When `NodeNodeBetweenLayers` option is set (default: 0, disabled), this phase ensures minimum spacing between node boundaries in adjacent layers. This prevents tall nodes from visually overlapping nodes in the next layer.

Resolution strategies (in order of preference):

1. **Horizontal Shift**: Move the node with fewer connections horizontally to eliminate X-range overlap
2. **Layer Gap Increase**: If horizontal shift fails, increase the gap between the two layers

Skips node pairs that have a direct edge between them (edge routing handles their visual connection).

Iterates until stable or maximum 10 iterations (handles cascading shifts).

#### Phase 7: Port Position Computation

**File**: `port.go` | **Method**: `computePortOffsets()`

**Input**: Positioned nodes with port declarations
**Output**: Computed offsets (and sides for PortFree) written to node.ports

For ports with computed positions (PortFixedSide, PortFixedOrder, PortFree, PortFixedOffset):

1. **Side Assignment** (PortFree, PortFixedOffset):
   - Uses geometric line-rectangle intersection voting
   - Examines all connected nodes to determine the best side for the port
   - Respects PortAxis constraints (horizontal-only, vertical-only, or any)

2. **Offset Computation**:
   - Groups ports by side
   - For PortFixedOrder: sorts by declared Order value
   - For PortFixedSide/PortFree: sorts by barycenter of connected nodes along the side
   - Distributes ports evenly along the side length
   - PortFixedOffset ports keep their declared offset

#### Phase 8: Component Packing

**File**: `components.go` | **Method**: `packComponents()`

**Input**: Positioned nodes (may include disconnected subgraphs)
**Output**: Components arranged according to packing options

For graphs with multiple disconnected components, this phase arranges them according to `ComponentPacking` option:

1. **PackHorizontal** (default): Places components side by side, aligned to top edge
2. **PackVertical**: Stacks components vertically, aligned to left edge

Components are identified using BFS traversal through all edges (including through dummy nodes). The gap between components is controlled by `ComponentGap` (default: `NodeSep * 2`).

```
PackHorizontal:                PackVertical:
┌─────┐  ┌─────┐              ┌─────┐
│ C1  │  │ C2  │              │ C1  │
└─────┘  └─────┘              └─────┘
                              ┌─────┐
                              │ C2  │
                              └─────┘
```

#### Phase 9: Edge Routing

**File**: `route.go` | **Method**: `routeEdges()`

**Input**: Positioned nodes (including dummies), reversed edges, self-loops
**Output**: Edge paths as arrays of points, attachment sides, label positions

This phase builds the final edge paths through eight sub-steps:

1. **Build Dummy Chain Paths** (`buildEdgePaths()`):
   - Walks each dummy chain to collect bend points
   - Computes label positions from label dummy coordinates
   - Parallelizes for graphs with 50+ chains

2. **Initialize Short Edges** (`initializeShortEdges()`):
   - Creates straight paths for edges without dummy nodes

3. **Restore Reversed Edges** (`restoreReversedEdges()`):
   - Swaps source/target for edges reversed in Phase 1
   - Reverses point arrays so paths flow correctly

4. **Route by Style**:
   - **RoutePolyline** (default): Adds node boundary intersection points (`addNodeIntersections()`)
   - **RouteOrthogonal**: Creates channel-routed horizontal/vertical segments (`routeOrthogonal()`)

5. **Offset Parallel Edges** (`offsetParallelEdges()`):
   - Separates multi-edges between the same node pair
   - Uses `ChannelGap` spacing

6. **Infer Attachment Sides** (`inferEdgeSides()`):
   - Computes `SourceSide` and `TargetSide` for each edge
   - Considers port constraints, stacked nodes, and relative positions

7. **Resolve Label Collisions** (`resolveEdgeLabelCollisions()`):
   - Shifts overlapping edge labels apart

8. **Restore Self-Loops** (`restoreSelfLoops()`):
   - Creates curved arc paths for self-loop edges
   - Positions arcs on the appropriate side based on port constraints

#### Post-transform: Direction Finalize

**File**: `direction.go` | **Method**: `undoDirectionAdjustment()`

Converts coordinates from internal layout space to user coordinate space:

| Direction | Transformation |
|-----------|----------------|
| TopToBottom | No change (internal = user space) |
| BottomToTop | Flip Y coordinates, swap Top/Bottom sides |
| LeftToRight | Swap X/Y coordinates, swap Width/Height, rotate sides |
| RightToLeft | Flip Y, swap X/Y, swap Width/Height, rotate sides |

#### Final: Cluster Sizing

**File**: `posit.go` | **Method**: `adjustClusters()`

For compound graphs with cluster nodes (`IsCluster: true`), sizes each cluster to contain all its children with the configured padding:

1. Finds bounding box of all child nodes
2. Sets cluster position and size to contain children with padding on all sides
3. Handles nested clusters (may require multiple passes)

```
┌────────────────────┐
│ Cluster (padding)  │
│  ┌────┐   ┌────┐   │
│  │ A  │   │ B  │   │
│  └────┘   └────┘   │
└────────────────────┘
```

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

Posit extends beyond dagre's feature set with:

- Compound graph support (nested clusters)
- Edge label positioning (left/center/right)
- Self-loop rendering (curved paths)
- Port constraints (5 modes: FixedPos, FixedSide, FixedOrder, Free, FixedOffset)
- Orthogonal edge routing
- Multi-edge support
- Incremental layout
- Rank and order constraints
- Disconnected component packing

While using simpler data structures (maps vs graphlib).

---

## Future Considerations

### Potential Enhancements

1. **Spline routing**: Curved edges instead of polylines (consumers currently post-process polyline points)

### Non-Goals

- General force-directed layout (different algorithm family)
- 3D layout
- Interactive/animated layout
- Graph editing or manipulation beyond layout
- Spline/bézier curve generation (consumers handle this with their rendering library)

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
