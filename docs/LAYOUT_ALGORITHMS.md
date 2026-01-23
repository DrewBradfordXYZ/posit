# Graph Layout Algorithms

A reference guide to the major graph layout algorithm families, when to use each, and how they work.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Graph Layout Algorithms                       │
├────────────────┬────────────────┬───────────────────────────────┤
│  Hierarchical  │   Geometric    │      Specialized              │
├────────────────┼────────────────┼───────────────────────────────┤
│ Layered        │ Force-directed │ Tree (Reingold-Tilford)       │
│ (Sugiyama)     │ Stress/MDS     │ Radial                        │
│                │                │ Circular                       │
│                │                │ Orthogonal                     │
│                │                │ Box/Packing                    │
└────────────────┴────────────────┴───────────────────────────────┘
```

---

## 1. Layered (Sugiyama)

**What Posit implements.** Arranges nodes in horizontal layers with edges flowing in one direction. The standard algorithm for visualizing directed graphs with hierarchy.

### When to Use

- Dependency graphs (packages, modules, imports)
- Flowcharts and state machines
- Database schema / ER diagrams
- Org charts
- Git commit history
- Any DAG where directional flow matters

### How It Works

```
Input graph:         After layout:

A → B                    ┌───┐
A → C                    │ A │
B → D                    └─┬─┘
C → D                   ┌──┴──┐
                       ┌─┴─┐┌─┴─┐
                       │ B ││ C │
                       └─┬─┘└─┬─┘
                         └──┬──┘
                          ┌─┴─┐
                          │ D │
                          └───┘
```

Five phases:

1. **Cycle removal** — Temporarily reverse edges to make the graph acyclic
2. **Layer assignment** — Assign each node to a horizontal rank
3. **Dummy nodes** — Insert virtual nodes where edges span multiple layers
4. **Crossing minimization** — Reorder nodes within layers to reduce edge crossings
5. **Coordinate assignment** — Compute final X/Y positions

### Complexity

- O(V + E) for longest-path ranking
- O(V × E) for network simplex ranking
- O(iterations × V × E) for crossing minimization

### Characteristics

- Produces clear directional flow (top-to-bottom, left-to-right, etc.)
- Edges mostly flow in one direction — reversed edges indicate cycles
- Layout width grows with parallelism, height with longest path
- Dummy nodes can make long edges appear less direct

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| **Posit** | Go | Zero deps, ports, order groups |
| ELK (layered) | Java/WASM | Most feature-rich |
| dagre | JavaScript | Simple, widely used |
| MSAGL | TypeScript | Microsoft, Sugiyama variant |
| Graphviz (dot) | C | The original, extremely mature |

---

## 2. Force-Directed

Treats the graph as a physical system: nodes are charged particles that repel each other, edges are springs that attract connected nodes. Iterates until the system reaches equilibrium.

### When to Use

- Social networks
- Knowledge graphs
- Clustering visualization (groups naturally emerge)
- Any undirected graph where structure matters more than flow
- Exploratory visualization where you don't know the shape

### How It Works

```
Input:                  After layout:

A — B                        ┌───┐
A — C                   ┌────│ B │
B — C                   │    └───┘
B — D                 ┌─┴─┐
C — E                 │ A │────┐
                      └─┬─┘    │
                        │    ┌─┴─┐
                      ┌─┴─┐  │ D │
                      │ C │  └───┘
                      └─┬─┘
                      ┌─┴─┐
                      │ E │
                      └───┘
```

Each iteration:
1. Compute repulsive force between all node pairs (O(V²) naive, O(V log V) with Barnes-Hut)
2. Compute attractive force along each edge (O(E))
3. Apply forces to update positions
4. Reduce temperature (step size) for convergence

### Complexity

- O(V² + E) per iteration (naive)
- O(V log V + E) per iteration (Barnes-Hut approximation)
- Typically 100-1000 iterations to converge

### Characteristics

- No predetermined structure — layout emerges from the graph topology
- Symmetric graphs produce symmetric layouts
- Clusters naturally separate (dense subgraphs clump together)
- Non-deterministic (depends on initial positions, unless seeded)
- No directional flow — edges point in all directions
- Can get stuck in local minima for complex graphs

### Variants

| Algorithm | Innovation |
|-----------|-----------|
| Fruchterman-Reingold (1991) | Temperature-based cooling, grid optimization |
| Kamada-Kawai (1989) | Energy minimization, spring model based on graph distance |
| ForceAtlas2 (2014) | Designed for large networks, adaptive speed |
| d3-force | Velocity Verlet integration, configurable forces |

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| d3-force | JavaScript | Most popular for web |
| Gephi | Java | ForceAtlas2, large graphs |
| ELK (force) | Java/WASM | Less commonly used than ELK layered |
| igraph | C/R/Python | Research-grade |

---

## 3. Stress / MDS (Multidimensional Scaling)

Minimizes the difference between graph-theoretic distances (shortest path lengths) and geometric distances (Euclidean distance in the layout). Produces layouts where node proximity reflects connectivity.

### When to Use

- Graphs where distance/proximity has meaning
- Network topology visualization
- Dimensionality reduction of high-dimensional data
- When you want the "true shape" of a graph (faithful distance representation)

### How It Works

```
Graph distances:         Layout (distances preserved):

    A-B: 1                    ┌───┐
    A-C: 1                    │ B │
    A-D: 2               ┌───┘   └──┐
    B-D: 1              ┌┴──┐     ┌──┴┐
    C-D: 2              │ A │     │ D │
                        └┬──┘     └───┘
                         │   ┌───┐
                         └───│ C │
                             └───┘

    A,B close (dist 1) ✓
    A,D far (dist 2)   ✓
    C,D far (dist 2)   ✓
```

Algorithm:
1. Compute all-pairs shortest paths (Floyd-Warshall or BFS)
2. Initialize positions (random or PCA)
3. Iteratively adjust positions to minimize stress:
   `stress = Σ wᵢⱼ (||pᵢ - pⱼ|| - dᵢⱼ)²`
4. Converge when stress stops decreasing

### Complexity

- O(V²) for all-pairs distances (BFS on unweighted graphs)
- O(V²) per iteration for stress computation
- Typically 50-200 iterations

### Characteristics

- Faithful representation of graph distances
- Produces compact, aesthetically pleasing layouts
- Better than force-directed for sparse graphs (no "hairball" effect)
- Deterministic given same initial positions
- No directional bias — purely structural

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| ELK (stress) | Java/WASM | Full implementation |
| OGDF | C++ | Research-grade |
| Graphviz (neato) | C | Classic stress majorization |

---

## 4. Tree (Reingold-Tilford)

Specialized layout for tree structures (each node has exactly one parent). Produces compact, aesthetically pleasing results that are impossible with general-purpose algorithms.

### When to Use

- File system trees
- Parse trees / ASTs
- Organization hierarchies (strict)
- Decision trees
- XML/JSON structure visualization
- Any graph that is actually a tree

### How It Works

```
           ┌─────┐
           │root │
           └──┬──┘
        ┌─────┼─────┐
      ┌─┴─┐ ┌┴──┐ ┌─┴─┐
      │ A │ │ B │ │ C │
      └─┬─┘ └┬──┘ └───┘
      ┌─┴─┐  │
    ┌─┴┐┌─┴┐┌┴─┐
    │D ││E ││F │
    └──┘└──┘└──┘

Properties:
- Subtrees don't overlap
- Parent centered over children
- Symmetric subtrees produce symmetric layouts
- Compact (minimal width for depth)
```

Algorithm (bottom-up):
1. Lay out each subtree independently
2. Push subtrees together as close as possible (contour merging)
3. Center parent over children
4. Thread nodes for O(n) contour computation

### Complexity

- O(V) — linear in the number of nodes (the key advantage over Sugiyama)

### Characteristics

- Much faster than Sugiyama for trees (O(V) vs O(V × E))
- Guaranteed no node overlap
- Subtree preservation (moving a subtree doesn't affect others)
- Only works for trees — fails on DAGs or general graphs
- Parent is always centered over children

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| d3-tree | JavaScript | Web standard |
| ELK (mrtree) | Java/WASM | Extended with ports |
| Graphviz (dot) | C | Handles trees as special case |

---

## 5. Radial

Arranges nodes in concentric rings around a central focus node. Distance from center represents graph distance from the focus.

### When to Use

- Ego networks (one central entity)
- Dependency depth visualization
- BFS/shortest-path exploration
- File/directory size visualization (sunburst)
- When a single node is the "focus"

### How It Works

```
                    ╭───╮
              ╭─────│ D │─────╮
              │     ╰───╯     │
           ╭──┴──╮         ╭──┴──╮
           │  B  │         │  E  │
           ╰──┬──╯         ╰─────╯
              │    ╭───╮
              ╰────│ A │────╮    ← center (focus)
                   ╰─┬─╯    │
                   ╭─┴──╮ ╭─┴──╮
                   │ C  │ │ F  │
                   ╰────╯ ╰────╯

Ring 0: A (focus)
Ring 1: B, C, F (distance 1 from A)
Ring 2: D, E (distance 2 from A)
```

Algorithm:
1. BFS from focus node to determine ring assignments
2. Divide angular space among children proportional to subtree size
3. Place nodes at (ring × radius, angle) in polar coordinates
4. Convert to Cartesian for rendering

### Complexity

- O(V + E) for BFS assignment
- O(V) for position computation

### Characteristics

- Intuitive depth representation
- Compact for balanced trees
- Can waste space for unbalanced structures
- Angular resolution decreases with depth (outer rings get crowded)
- Works poorly for graphs with many cycles

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| ELK (radial) | Java/WASM | Full implementation |
| d3-cluster | JavaScript | Radial variant of tree layout |
| Graphviz (twopi) | C | Classic implementation |

---

## 6. Orthogonal

All edges are drawn using only horizontal and vertical segments. Optimizes for minimum bends and crossings. Distinct from orthogonal *routing* (which just affects edge paths in a layered layout).

### When to Use

- Circuit/chip diagrams (VLSI)
- UML class diagrams
- Entity-relationship diagrams
- Floor plans
- Any diagram requiring clean right-angle aesthetics

### How It Works

```
    ┌─────┐          ┌─────┐
    │  A  │──────────│  B  │
    └──┬──┘          └──┬──┘
       │                │
       │    ┌─────┐     │
       └────│  C  │─────┘
            └──┬──┘
               │
            ┌──┴──┐
            │  D  │
            └─────┘

All edges: horizontal or vertical only
Bends: minimized (each bend costs aesthetic quality)
```

Algorithm (topology-shape-metrics approach):
1. **Planarization** — make graph planar by replacing crossings with dummy nodes
2. **Orthogonalization** — assign edge directions (H/V) and bends
3. **Compaction** — minimize total edge length while preserving topology

### Complexity

- O(V²) for simple approaches
- NP-hard for optimal bend minimization (heuristics used in practice)

### Characteristics

- Clean, professional appearance for technical diagrams
- Limited to degree ≤ 4 per node (only 4 sides available) without ports
- Higher computational cost than layered or force-directed
- Not suitable for large graphs (100+ nodes becomes slow and cluttered)
- Bend minimization is the key quality metric

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| OGDF | C++ | Research-grade, multiple algorithms |
| yFiles | Java | Commercial, high quality |
| ELK | Java/WASM | Basic support |

---

## 7. Circular

Places nodes on a circle. Optimizes edge crossing within the circle. Variants include groups of circles (compound circular).

### When to Use

- Network protocols (ring topologies)
- Showing all-to-all connectivity
- Small dense graphs where hierarchy doesn't exist
- Visualization of cyclic structures

### How It Works

```
            ┌───┐
        ╭───│ A │───╮
        │   └───┘   │
      ┌─┴─┐       ┌─┴─┐
      │ F │       │ B │
      └─┬─┘       └─┬─┘
        │             │
      ┌─┴─┐       ┌─┴─┐
      │ E │       │ C │
      └─┬─┘       └─┬─┘
        │   ┌───┐   │
        ╰───│ D │───╯
            └───┘

Node order on circle determines crossing count.
Optimizing order ≈ Traveling Salesman Problem.
```

Algorithm:
1. Determine node ordering on circle (heuristic for minimum crossings)
2. Compute angular positions: `angle_i = 2π × i / n`
3. Place at `(r × cos(angle), r × sin(angle))`
4. Route edges as straight lines or arcs

### Complexity

- O(V²) for ordering heuristics
- O(V) for position assignment

### Characteristics

- All nodes equally prominent (no hierarchy)
- Works well for small graphs (< 30 nodes)
- Crossing minimization on circles is well-studied
- Poor for sparse graphs (lots of empty space)
- Can combine with clustering (each cluster on its own circle)

### Implementations

| Library | Language | Notes |
|---------|----------|-------|
| Graphviz (circo) | C | Classic |
| ELK | Java/WASM | Basic support |
| Cytoscape.js | JavaScript | With extensions |

---

## 8. Box/Packing

Not a layout algorithm per se — arranges pre-laid-out subgraphs into a compact rectangular area. Used for disconnected components or composite layouts.

### When to Use

- Disconnected graph components
- Dashboard layouts
- Composite visualizations
- Arranging multiple independent diagrams on one canvas

### How It Works

```
┌──────────────┬─────────────────┐
│  ┌───┐       │   ┌───┐         │
│  │ A │──┐    │   │ D │───┐     │
│  └───┘  │    │   └───┘   │     │
│       ┌─┴─┐  │         ┌─┴─┐   │
│       │ B │  │         │ E │   │
│       └───┘  │         └───┘   │
├──────────────┼─────────────────┤
│   ┌───┐      │                  │
│   │ F │      │                  │
│   └───┘      │                  │
└──────────────┴─────────────────┘

Each component laid out independently,
then packed into minimal bounding area.
```

Algorithm (strip packing / bin packing heuristic):
1. Sort components by height (or area) descending
2. Place each component in first available position (bottom-left)
3. Optionally rotate components for better fit

### Complexity

- O(n log n) for sorting + O(n²) for placement
- Usually negligible compared to layout of individual components

### Implementations

All major layout libraries include packing as a post-processing step. Posit uses horizontal or vertical packing for disconnected components.

---

## Choosing an Algorithm

```
Is your graph a tree?
  ├── Yes → Tree (Reingold-Tilford)
  └── No
        Does direction/flow matter?
          ├── Yes → Layered (Sugiyama) ← Posit
          └── No
                Is there a focal node?
                  ├── Yes → Radial
                  └── No
                        Do you need right-angle edges?
                          ├── Yes → Orthogonal
                          └── No
                                Is distance/proximity meaningful?
                                  ├── Yes → Stress/MDS
                                  └── No → Force-directed
```

## Algorithm Comparison

| Algorithm | Directed? | Deterministic? | Speed | Best graph size |
|-----------|-----------|----------------|-------|-----------------|
| Layered | Yes | Yes | Medium | 10-1000 nodes |
| Force-directed | No | No* | Slow | 50-10000 nodes |
| Stress/MDS | No | Yes | Medium | 10-5000 nodes |
| Tree | Yes | Yes | Fast | 10-100000 nodes |
| Radial | Yes | Yes | Fast | 10-500 nodes |
| Orthogonal | No | Yes | Slow | 10-100 nodes |
| Circular | No | Yes | Fast | 5-30 nodes |

\* Force-directed can be made deterministic with fixed seed/initial positions.

---

## Further Reading

- Sugiyama, Tagawa, Toda (1981). "Methods for Visual Understanding of Hierarchical System Structures" — the original layered algorithm
- Fruchterman, Reingold (1991). "Graph Drawing by Force-directed Placement" — foundational force-directed
- Reingold, Tilford (1981). "Tidier Drawings of Trees" — the tree algorithm
- Eiglsperger, Siebenhaller, Kaufmann (2005). "An Efficient Implementation of Sugiyama's Algorithm for Layered Graph Drawing" — inner segment optimization (used in Posit)
- Brandes, Kopf (2002). "Fast and Simple Horizontal Coordinate Assignment" — BK coordinate assignment (used in Posit)
- Barth, Junger, Mutzel (2002). "Simple and Efficient Bilayer Cross Counting" — accumulator tree crossing count (used in Posit)
