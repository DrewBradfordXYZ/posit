# The Sugiyama Algorithm: A Comprehensive Reference

This document provides a detailed explanation of the Sugiyama algorithm for hierarchical graph layout, as implemented in the **posit** library. It serves as a definitive reference for understanding, maintaining, and extending the implementation.

---

## Table of Contents

1. [Overview and History](#1-overview-and-history)
2. [Algorithm Structure](#2-algorithm-structure)
3. [Phase 1: Cycle Removal](#3-phase-1-cycle-removal)
4. [Phase 2: Layer Assignment (Ranking)](#4-phase-2-layer-assignment-ranking)
5. [Phase 3: Dummy Node Insertion (Normalization)](#5-phase-3-dummy-node-insertion-normalization)
6. [Phase 4: Crossing Minimization (Ordering)](#6-phase-4-crossing-minimization-ordering)
7. [Phase 5: Coordinate Assignment (Positioning)](#7-phase-5-coordinate-assignment-positioning)
8. [Phase 6: Edge Routing](#8-phase-6-edge-routing)
9. [Complexity Summary](#9-complexity-summary)
10. [References](#10-references)

---

## 1. Overview and History

### 1.1 Origins

The Sugiyama algorithm was introduced in the seminal 1981 paper:

> Kozo Sugiyama, Shojiro Tagawa, and Mitsuhiko Toda. "Methods for Visual Understanding of Hierarchical System Structures." *IEEE Transactions on Systems, Man, and Cybernetics*, 11(2):109-125, 1981.

This algorithm revolutionized the visualization of directed acyclic graphs (DAGs) and remains the foundation for most hierarchical graph layout systems today, including:

- **Graphviz/dot** - The most widely used graph visualization tool
- **dagre** - JavaScript graph layout library (our reference implementation)
- **D3-dag** - D3-based DAG visualization
- **ELK** - Eclipse Layout Kernel

### 1.2 Why Sugiyama?

The Sugiyama algorithm is the standard for DAG visualization because it produces layouts that:

1. **Respect hierarchy** - Edges flow in a consistent direction (typically top-to-bottom)
2. **Minimize crossings** - Reduces visual clutter and improves readability
3. **Maintain proximity** - Connected nodes are placed close together
4. **Handle complexity** - Scales to graphs with thousands of nodes

### 1.3 Core Insight

The key insight of Sugiyama is to decompose the complex 2D layout problem into a series of simpler 1D problems:

```
                    2D Layout Problem
                           |
        +------------------+------------------+
        |                  |                  |
   Y-coordinates      X-coordinates      Edge routing
   (layer assignment)  (node ordering)   (path drawing)
```

Each dimension is solved independently, with constraints flowing between phases.

---

## 2. Algorithm Structure

### 2.1 The 4+2 Phase Structure

The algorithm consists of **four core phases** plus **two bookkeeping phases**:

```
    +-------------------+
    | INPUT: Digraph G  |
    +-------------------+
             |
             v
    +-------------------+
    | Phase 0: CYCLE    |  <- Bookkeeping (pre-processing)
    | REMOVAL           |     Make graph acyclic
    +-------------------+
             |
             v
    +-------------------+
    | Phase 1: LAYER    |  <- Core Phase
    | ASSIGNMENT        |     Assign Y-coordinates
    +-------------------+
             |
             v
    +-------------------+
    | Phase 2: DUMMY    |  <- Core Phase
    | NODES             |     Normalize long edges
    +-------------------+
             |
             v
    +-------------------+
    | Phase 3: CROSSING |  <- Core Phase
    | MINIMIZATION      |     Order nodes per layer
    +-------------------+
             |
             v
    +-------------------+
    | Phase 4: POSITION |  <- Core Phase
    | ASSIGNMENT        |     Assign X-coordinates
    +-------------------+
             |
             v
    +-------------------+
    | Phase 5: EDGE     |  <- Bookkeeping (post-processing)
    | ROUTING           |     Generate edge paths
    +-------------------+
             |
             v
    +-------------------+
    | OUTPUT: Positioned|
    | Graph with Paths  |
    +-------------------+
```

### 2.2 Data Flow Between Phases

```
Phase 0 (Cycle Removal):
  Input:  Possibly cyclic directed graph
  Output: DAG (with some edges marked as "reversed")

Phase 1 (Layer Assignment):
  Input:  DAG
  Output: Each node has a "rank" attribute (layer number)

Phase 2 (Dummy Nodes):
  Input:  DAG with ranks
  Output: Graph where every edge spans exactly one layer

Phase 3 (Crossing Minimization):
  Input:  Normalized graph with ranks
  Output: Each node has an "order" attribute (position within layer)

Phase 4 (Coordinate Assignment):
  Input:  Graph with ranks and orders
  Output: Each node has (x, y) coordinates

Phase 5 (Edge Routing):
  Input:  Positioned graph with dummy nodes
  Output: Edge paths as polylines, dummy nodes removed
```

---

## 3. Phase 1: Cycle Removal

**File:** `acyclic.go`

### 3.1 Goal

Transform a potentially cyclic directed graph into a DAG by identifying and reversing "back edges" - edges that create cycles.

### 3.2 Why This Matters

The Sugiyama algorithm requires a DAG because:
- Layer assignment assumes edges flow in one direction
- Cycles would create contradictory layer constraints (A before B, B before A)

### 3.3 Algorithm: DFS-Based Cycle Detection

The implementation uses depth-first search to find a **Feedback Arc Set (FAS)** - a set of edges whose removal makes the graph acyclic.

```
Algorithm DFS-FAS(G):
    fas = []           // Feedback Arc Set
    visited = {}       // Nodes we've finished processing
    stack = {}         // Nodes currently in DFS path (gray nodes)

    function dfs(v):
        if v in visited:
            return

        visited[v] = true
        stack[v] = true    // v is now on current path

        for each edge (v, w) in outEdges(v):
            if w in stack:
                // Back edge found! w is ancestor of v
                fas.append((v, w))
            else:
                dfs(w)

        delete stack[v]    // v is no longer on current path

    for each node v in G.nodes():
        dfs(v)

    return fas
```

**Key insight:** An edge (v, w) creates a cycle if and only if w is already on the current DFS path (in the "stack"). These are called **back edges**.

### 3.4 Reversing Edges

Once we identify back edges, we reverse them:

```
for each edge e in fas:
    label = G.edge(e)
    G.removeEdge(e)

    // Store original direction for later restoration
    label.forwardName = e.name
    label.reversed = true

    // Add reversed edge
    G.setEdge(e.w, e.v, label, uniqueId("rev"))
```

### 3.5 Undoing the Reversal

After layout is complete, reversed edges are flipped back:

```
function undo(G):
    for each edge e in G.edges():
        if e.label.reversed:
            G.removeEdge(e)
            // Restore original direction
            G.setEdge(e.w, e.v, label, label.forwardName)
```

### 3.6 Alternative: Greedy FAS

For weighted graphs, dagre supports a greedy heuristic based on:

> P. Eades, X. Lin, and W. F. Smyth. "A fast and effective heuristic for the feedback arc set problem."

This algorithm prioritizes keeping high-weight edges in their original direction.

### 3.7 Complexity

| Operation | Time Complexity |
|-----------|-----------------|
| DFS-FAS   | O(V + E)       |
| Greedy FAS| O(V + E)       |
| Undo      | O(E)           |

### 3.8 Edge Cases

1. **Self-loops:** An edge (v, v) is always a back edge
2. **Disconnected components:** DFS naturally handles these by iterating over all nodes
3. **Already acyclic:** No edges are reversed; algorithm still runs in O(V + E)

### 3.9 Visual Example

```
Before (with cycle):          After (cycle broken):

    A -----> B                    A -----> B
    ^        |                             |
    |        v                             v
    D <----- C                    D -----X C
                                  (reversed)

The edge C->D becomes D->C (marked as reversed)
```

---

## 4. Phase 2: Layer Assignment (Ranking)

**File:** `rank.go`

### 4.1 Goal

Assign each node to a discrete layer (rank) such that for every edge (u, v), rank(u) < rank(v). This determines the Y-coordinate of each node.

### 4.2 Constraint: Minimum Edge Length

Edges can have a `minlen` attribute specifying minimum layer span:
- `minlen = 1`: Normal edge (adjacent layers)
- `minlen = 2`: Skip one layer between source and target
- `minlen = 0`: Same layer allowed (rare)

### 4.3 Algorithm 1: Longest Path (Simple)

The simplest approach - used in our MVP implementation.

```
Algorithm LongestPath(G):
    visited = {}

    function dfs(v):
        if v in visited:
            return G.node(v).rank

        visited[v] = true

        // Find minimum rank allowed by successors
        minRank = +infinity
        for each edge (v, w) in outEdges(v):
            successorRank = dfs(w) - edge.minlen
            minRank = min(minRank, successorRank)

        if minRank == +infinity:
            minRank = 0  // Sink node

        G.node(v).rank = minRank
        return minRank

    // Start from all source nodes
    for each v in G.sources():
        dfs(v)
```

**Characteristics:**
- Time: O(V + E)
- Space: O(V) for recursion
- Quality: Produces valid but not optimal rankings
- Tendency: Pushes nodes to lowest possible rank, creating wide bottom layers

### 4.4 Visual Example of Longest Path

```
Input DAG:                  Longest Path Result:

    A -----> B              Layer 0:  A
             |              Layer 1:  B, C
    C -----> D              Layer 2:  D
         \   ^
          \__|

Note: C could be at Layer 0, but longest path puts it at Layer 1
```

### 4.5 Algorithm 2: Network Simplex (Optimal)

The network simplex algorithm finds the optimal ranking that minimizes total edge length. This is the default in dagre.

**Reference:** Gansner, Koutsofios, North, Vo. "A Technique for Drawing Directed Graphs." *IEEE Trans. Software Engineering*, 1993.

```
Algorithm NetworkSimplex(G):
    // Step 1: Initial ranking (use Longest Path)
    longestPath(G)

    // Step 2: Build feasible spanning tree
    // A "tight" tree where all tree edges have slack = 0
    tree = feasibleTree(G)

    // Step 3: Initialize tree structure
    initLowLimValues(tree)   // For O(1) descendant queries
    initCutValues(tree, G)   // Edge cut values for pivoting

    // Step 4: Iterate until optimal
    while true:
        // Find edge with negative cut value (leaving edge)
        e = findLeavingEdge(tree)
        if e == null:
            break  // Optimal!

        // Find replacement edge (entering edge)
        f = findEnteringEdge(tree, G, e)

        // Pivot: swap edges in spanning tree
        exchangeEdges(tree, G, e, f)
```

**Key Concepts:**

1. **Slack:** For edge (u, v): `slack = rank(v) - rank(u) - minlen`
   - Slack = 0: Edge is "tight"
   - Slack > 0: Edge is longer than required

2. **Feasible Tree:** A spanning tree where all edges are tight

3. **Cut Value:** For tree edge e, measures benefit of removing e
   - Negative cut value = removing e can improve solution

4. **Low/Lim Values:** Enable O(1) descendant queries in tree
   - `low[v]`: Minimum DFS number in subtree
   - `lim[v]`: Maximum DFS number in subtree
   - Node u is descendant of v iff `low[v] <= lim[u] <= lim[v]`

### 4.6 Network Simplex Complexity

| Operation | Complexity |
|-----------|------------|
| Initial ranking | O(V + E) |
| Build feasible tree | O(V * E) |
| Each pivot | O(V) |
| Total (practical) | O(V * E) |
| Total (worst case) | O(V^2 * E) |

In practice, network simplex converges quickly (usually < 10 iterations).

### 4.7 Choosing a Ranker

| Ranker | Pros | Cons |
|--------|------|------|
| Longest Path | Fast, simple | Poor quality (wide bottom) |
| Tight Tree | Moderate quality | Still suboptimal |
| Network Simplex | Optimal | More complex, slower |

---

## 5. Phase 3: Dummy Node Insertion (Normalization)

**File:** `normalize.go`

### 5.1 Goal

Transform the graph so that every edge spans exactly one layer. This simplifies crossing minimization and coordinate assignment.

### 5.2 Why Normalize?

Without normalization:
- Crossing counting is complex (edges skip layers)
- Coordinate assignment must handle varying edge lengths
- Edge routing requires special cases

With normalization:
- All algorithms work on uniform "layer graphs"
- Crossings are counted between adjacent layers only
- Coordinates can be assigned layer by layer

### 5.3 Algorithm

```
Algorithm Normalize(G):
    G.graph().dummyChains = []  // Track for later removal

    for each edge e = (v, w) in G.edges():
        vRank = G.node(v).rank
        wRank = G.node(w).rank

        if wRank == vRank + 1:
            continue  // Already spans one layer

        // Remove long edge
        edgeLabel = G.edge(e)
        G.removeEdge(e)

        // Insert chain of dummy nodes
        current = v
        for rank = vRank + 1 to wRank - 1:
            dummy = addDummyNode(G, {
                width: 0,
                height: 0,
                rank: rank,
                edgeLabel: edgeLabel,
                edgeObj: e
            })

            G.setEdge(current, dummy)

            if rank == vRank + 1:
                G.graph().dummyChains.push(dummy)  // First in chain

            current = dummy

        // Connect last dummy to target
        G.setEdge(current, w)
```

### 5.4 Visual Example

```
Before normalization:           After normalization:

Layer 0:    A                   Layer 0:    A
            |                               |
            | (long edge)                   v
            |                   Layer 1:   [D1] <- dummy
            |                               |
            v                               v
Layer 3:    B                   Layer 2:   [D2] <- dummy
                                            |
                                            v
                                Layer 3:    B
```

### 5.5 Dummy Node Properties

Dummy nodes have special properties:
- `width: 0` and `height: 0` (invisible)
- `dummy: "edge"` marker
- `edgeLabel`: Original edge data
- `edgeObj`: Original edge identifier

For edge labels at specific positions:
- `dummy: "edge-label"` marker
- `width/height`: Label dimensions
- `labelpos`: Label position (l/c/r)

### 5.6 Denormalization (Undo)

After coordinate assignment, dummy nodes are removed and replaced with bend points:

```
Algorithm Denormalize(G):
    for each dummyChain in G.graph().dummyChains:
        v = dummyChain
        node = G.node(v)
        originalLabel = node.edgeLabel

        // Restore original edge
        G.setEdge(node.edgeObj, originalLabel)

        // Collect bend points from dummy chain
        while node.dummy:
            w = G.successors(v)[0]
            G.removeNode(v)

            // Store dummy position as bend point
            originalLabel.points.push({x: node.x, y: node.y})

            v = w
            node = G.node(v)
```

### 5.7 Complexity

| Operation | Complexity |
|-----------|------------|
| Normalize | O(E * maxSpan) where maxSpan = max edge length |
| Denormalize | O(D) where D = number of dummy nodes |

Note: Number of dummy nodes can be O(V * E) in worst case.

---

## 6. Phase 4: Crossing Minimization (Ordering)

**File:** `order.go`

### 6.1 Goal

Determine the horizontal order of nodes within each layer to minimize the number of edge crossings.

### 6.2 Why This Matters

Edge crossings are the primary source of visual clutter in graph layouts:

```
Many crossings (hard to read):    Few crossings (easy to read):

Layer 0:  A     B     C           Layer 0:  A     B     C
           \   /|\   /                       \    |    /
            \ / | \ /                         \   |   /
             X  |  X                           \  |  /
            / \ | / \                           \ | /
           /   \|/   \                           \|/
Layer 1:  D     E     F           Layer 1:  D     E     F
```

### 6.3 NP-Completeness

**Important:** Minimizing crossings is NP-complete, even for just two layers. Therefore, we use heuristics that produce good (but not guaranteed optimal) results.

### 6.4 The Layer Sweep Approach

Instead of optimizing all layers simultaneously, we:
1. Fix one layer
2. Optimize the adjacent layer
3. Repeat, sweeping up and down

```
Algorithm LayerSweep(G):
    // Build layer graphs for up/down sweeps
    downGraphs = buildLayerGraphs(G, [1..maxRank], "inEdges")
    upGraphs = buildLayerGraphs(G, [maxRank-1..0], "outEdges")

    // Initial ordering (by DFS order)
    layering = initOrder(G)
    assignOrder(G, layering)

    bestCC = infinity
    best = null
    lastBest = 0

    for i = 0; lastBest < 4; i++:
        // Alternate sweep direction
        if i % 2 == 0:
            sweep(downGraphs)
        else:
            sweep(upGraphs)

        // Alternate bias direction every 2 iterations
        biasRight = (i % 4 >= 2)

        // Count crossings
        layering = buildLayerMatrix(G)
        cc = crossCount(G, layering)

        if cc < bestCC:
            lastBest = 0
            best = copy(layering)
            bestCC = cc
        else:
            lastBest++

    assignOrder(G, best)
```

### 6.5 The Barycenter Heuristic

The core ordering heuristic. For each node, compute the "barycenter" (weighted average position) of its neighbors, then sort by barycenter.

```
Algorithm Barycenter(G, movable):
    results = []

    for each node v in movable:
        edges = G.inEdges(v)

        if edges is empty:
            results.append({v: v})  // No constraint
            continue

        sum = 0
        weight = 0

        for each edge e in edges:
            edgeWeight = G.edge(e).weight
            neighborOrder = G.node(e.v).order

            sum += edgeWeight * neighborOrder
            weight += edgeWeight

        results.append({
            v: v,
            barycenter: sum / weight,
            weight: weight
        })

    return results
```

### 6.6 Visual Example of Barycenter

```
Fixed Layer (Layer 0):    A(0)    B(1)    C(2)    D(3)
                            \      |       |      /
                             \     |       |     /
                              \    |       |    /
                               \   |       |   /
Movable Layer (Layer 1):        X         Y

Edges: A->X, B->X, C->Y, D->Y

Barycenter(X) = (0 + 1) / 2 = 0.5
Barycenter(Y) = (2 + 3) / 2 = 2.5

After sorting by barycenter: X, Y
This ordering has 0 crossings!
```

### 6.7 Counting Crossings Efficiently

Counting crossings between two layers uses an accumulator tree (similar to merge-sort counting inversions):

```
Algorithm TwoLayerCrossCount(G, northLayer, southLayer):
    // Map south nodes to positions
    southPos = {v: i for i, v in enumerate(southLayer)}

    // Collect all edges, sorted by north then south position
    southEntries = []
    for each v in northLayer:
        for each edge (v, w) in G.outEdges(v):
            southEntries.append({
                pos: southPos[w],
                weight: G.edge(v, w).weight
            })
        sort by pos

    // Build accumulator tree (binary indexed tree)
    firstIndex = nextPowerOf2(len(southLayer))
    tree = array of zeros, size 2 * firstIndex - 1

    // Count weighted crossings
    cc = 0
    for each entry in southEntries:
        index = entry.pos + firstIndex - 1
        tree[index] += entry.weight

        // Count items to the right that came before
        weightSum = 0
        while index > 0:
            if index is odd:  // Left child
                weightSum += tree[index + 1]  // Add right sibling
            index = (index - 1) / 2
            tree[index] += entry.weight

        cc += entry.weight * weightSum

    return cc
```

**Reference:** Barth, Mutzel, Junger. "Simple and Efficient Bilayer Cross Counting." *Journal of Graph Algorithms and Applications*, 2004.

### 6.8 Complexity

| Operation | Complexity |
|-----------|------------|
| Barycenter calculation | O(E) per layer |
| Sorting by barycenter | O(n log n) per layer |
| Cross count (two layers) | O(E log V) |
| Full cross count | O(L * E log V) |
| Layer sweep (one iteration) | O(L * (E + n log n)) |
| Total (k iterations) | O(k * L * (E + n log n + E log V)) |

Where: V = vertices, E = edges, L = layers, k = iterations (typically 4-8)

---

## 7. Phase 5: Coordinate Assignment (Positioning)

**File:** `position.go`

### 7.1 Goal

Assign actual X and Y coordinates to each node, respecting:
- Layer assignments (Y coordinates)
- Node ordering within layers (X coordinates)
- Minimum spacing constraints (nodeSep, rankSep)
- Edge straightness (prefer vertical edges)

### 7.2 The Brandes-Kopf Algorithm

The implementation uses the Brandes-Kopf algorithm for X coordinate assignment:

> Ulrik Brandes and Boris Kopf. "Fast and Simple Horizontal Coordinate Assignment." *Graph Drawing 2001*.

**Key Idea:** Compute four different alignments (combinations of up/down and left/right), then take the median or best one.

### 7.3 Algorithm Overview

```
Algorithm PositionX(G):
    layering = buildLayerMatrix(G)

    // Find conflicts (edges that would cross inner segments)
    conflicts = findType1Conflicts(G, layering)
    conflicts += findType2Conflicts(G, layering)

    // Compute four alignments
    xss = {}
    for vert in ["u", "d"]:  // up, down
        adjustedLayering = reverse(layering) if vert == "d"

        for horiz in ["l", "r"]:  // left, right
            if horiz == "r":
                adjustedLayering = reverseEachLayer(adjustedLayering)

            neighborFn = predecessors if vert == "u" else successors
            align = verticalAlignment(G, adjustedLayering, conflicts, neighborFn)
            xs = horizontalCompaction(G, adjustedLayering, align)

            if horiz == "r":
                xs = negate(xs)

            xss[vert + horiz] = xs

    // Align all four to same reference point
    smallestWidth = findSmallestWidthAlignment(xss)
    alignCoordinates(xss, smallestWidth)

    // Balance: take median of four alignments
    return balance(xss)
```

### 7.4 Vertical Alignment

Creates "blocks" of nodes that should be vertically aligned:

```
Algorithm VerticalAlignment(G, layering, conflicts, neighborFn):
    root = {}   // root[v] = root node of v's block
    align = {}  // align[v] = next node in v's block
    pos = {}    // pos[v] = position of v in its layer

    // Initialize: each node is its own block
    for each layer in layering:
        for i, v in enumerate(layer):
            root[v] = v
            align[v] = v
            pos[v] = i

    // Process layers to form blocks
    for each layer in layering:
        prevIdx = -1

        for each v in layer:
            neighbors = neighborFn(v)
            if neighbors is empty:
                continue

            // Sort neighbors by position, find median
            neighbors.sort(by pos)
            medianIdx = (len(neighbors) - 1) / 2

            for i = floor(medianIdx) to ceil(medianIdx):
                w = neighbors[i]

                // Try to align v with w
                if align[v] == v and prevIdx < pos[w]:
                    if not hasConflict(conflicts, v, w):
                        align[w] = v
                        align[v] = root[v] = root[w]
                        prevIdx = pos[w]

    return {root, align}
```

### 7.5 Type-1 and Type-2 Conflicts

**Type-1 Conflict:** A non-inner segment crosses an inner segment.
- Inner segment: Edge between two dummy nodes
- We prioritize keeping inner segments straight

**Type-2 Conflict:** Occurs at subgraph borders (compound graphs).

```
Inner segment (dummy-to-dummy):

Layer i:     [D1]          [D2]
               |              |
               |   <- inner   |
               |   segment    |
Layer i+1:   [D3]          [D4]

Type-1 conflict:

Layer i:     [D1]    A      [D2]
               | \  / |        |
               |  \/  |        |
               |  /\  |        |
               | /  \ |        |
Layer i+1:   [D3]    B      [D4]

Edge A->D3 crosses inner segment D1->D3: CONFLICT!
```

### 7.6 Horizontal Compaction

Assigns actual X coordinates while respecting separation constraints:

```
Algorithm HorizontalCompaction(G, layering, root, align):
    xs = {}
    blockGraph = buildBlockGraph(G, layering, root)

    // Pass 1: Assign minimum coordinates (left to right)
    for each block in topological order:
        xs[block] = max(xs[pred] + separation for pred in predecessors)

    // Pass 2: Compact (right to left)
    for each block in reverse topological order:
        minSucc = min(xs[succ] - separation for succ in successors)
        xs[block] = max(xs[block], minSucc)

    // Propagate to all nodes in each block
    for each v in G.nodes():
        xs[v] = xs[root[v]]

    return xs
```

### 7.7 Separation Function

Calculates minimum horizontal distance between adjacent nodes:

```
function sep(G, v, w):
    vLabel = G.node(v)
    wLabel = G.node(w)

    sum = 0
    sum += vLabel.width / 2
    sum += (vLabel.dummy ? edgeSep : nodeSep) / 2
    sum += (wLabel.dummy ? edgeSep : nodeSep) / 2
    sum += wLabel.width / 2

    return sum
```

### 7.8 Y Coordinate Assignment

Y coordinates are simpler - determined by rank:

```
function positionY(G):
    layering = buildLayerMatrix(G)
    rankSep = G.graph().rankSep

    y = 0
    for rank = 0 to maxRank:
        layer = layering[rank]
        maxHeight = max(G.node(v).height for v in layer)

        for each v in layer:
            G.node(v).y = y + maxHeight / 2

        y += maxHeight + rankSep
```

### 7.9 Complexity

| Operation | Complexity |
|-----------|------------|
| Find conflicts | O(L * E) |
| Vertical alignment | O(V + E) |
| Horizontal compaction | O(V + E) |
| Total (4 alignments) | O(V + E + L * E) |

---

## 8. Phase 6: Edge Routing

**File:** `route.go`

### 8.1 Goal

Generate the final edge paths as polylines, converting dummy node chains back into bend points.

### 8.2 Algorithm

```
Algorithm RouteEdges(G):
    // Process each dummy chain
    for each chain in G.graph().dummyChains:
        v = chain
        node = G.node(v)
        originalEdge = node.edgeObj
        label = node.edgeLabel
        label.points = []

        // Walk the chain, collecting bend points
        while node.dummy:
            successor = G.successors(v)[0]

            // Add dummy's position as bend point
            label.points.push({x: node.x, y: node.y})

            // Handle edge labels
            if node.dummy == "edge-label":
                label.x = node.x
                label.y = node.y
                label.width = node.width
                label.height = node.height

            // Remove dummy node
            G.removeNode(v)

            v = successor
            node = G.node(v)

        // Restore original edge with collected points
        G.setEdge(originalEdge, label)

    // Handle reversed edges
    for each edge e in G.edges():
        if e.label.reversed:
            // Flip points order
            e.label.points.reverse()

            // Swap source and target
            G.removeEdge(e)
            G.setEdge(e.w, e.v, e.label)
```

### 8.3 Edge Path Structure

The final edge has:
- `points`: Array of {x, y} bend points
- `x`, `y`: Label position (if labeled)
- Start/end points are the source/target node positions

```
Complete edge path:

    Source Node (A)
         |
         * (A.x, A.y + A.height/2)  <- start point
         |
         * points[0]                 <- first bend
         |
         * points[1]                 <- second bend
         |
         * (B.x, B.y - B.height/2)  <- end point
         |
    Target Node (B)
```

### 8.4 Complexity

| Operation | Complexity |
|-----------|------------|
| Route edges | O(D) where D = total dummy nodes |

---

## 9. Complexity Summary

### 9.1 Per-Phase Complexity

| Phase | Time Complexity | Space Complexity |
|-------|-----------------|------------------|
| Cycle Removal | O(V + E) | O(V) |
| Layer Assignment (Longest Path) | O(V + E) | O(V) |
| Layer Assignment (Network Simplex) | O(V * E) typical | O(V + E) |
| Dummy Node Insertion | O(E * L) | O(E * L) |
| Crossing Minimization | O(k * L * E log V) | O(V + E) |
| Coordinate Assignment | O(V + E + L * E) | O(V) |
| Edge Routing | O(E * L) | O(E * L) |

Where:
- V = number of vertices
- E = number of edges
- L = number of layers (ranks)
- k = number of crossing minimization iterations

### 9.2 Overall Complexity

**Time:** O(V * E) for network simplex, or O(k * L * E log V) for crossing minimization - whichever dominates.

**Space:** O(V + E * L) due to dummy nodes.

### 9.3 Practical Performance

For typical graphs (< 1000 nodes):
- Layout completes in milliseconds
- Memory usage is modest

For large graphs (10,000+ nodes):
- Consider using longest path instead of network simplex
- Reduce crossing minimization iterations
- Graph may need partitioning

---

## 10. References

### 10.1 Primary Sources

1. **Sugiyama, K., Tagawa, S., and Toda, M.** (1981). "Methods for Visual Understanding of Hierarchical System Structures." *IEEE Transactions on Systems, Man, and Cybernetics*, 11(2):109-125.
   - The original paper introducing the algorithm

2. **Gansner, E.R., Koutsofios, E., North, S.C., and Vo, K.P.** (1993). "A Technique for Drawing Directed Graphs." *IEEE Transactions on Software Engineering*, 19(3):214-230.
   - Network simplex algorithm for optimal ranking
   - Foundation of Graphviz/dot

3. **Brandes, U. and Kopf, B.** (2001). "Fast and Simple Horizontal Coordinate Assignment." *Proceedings of Graph Drawing 2001*, LNCS 2265:31-44.
   - The coordinate assignment algorithm used in Phase 5

### 10.2 Crossing Minimization

4. **Barth, W., Junger, M., and Mutzel, P.** (2004). "Simple and Efficient Bilayer Cross Counting." *Journal of Graph Algorithms and Applications*, 8(2):179-194.
   - Efficient O(E log V) crossing count algorithm

5. **Eades, P., Lin, X., and Smyth, W.F.** (1993). "A Fast and Effective Heuristic for the Feedback Arc Set Problem." *Information Processing Letters*, 47(6):319-323.
   - Greedy FAS algorithm for cycle removal

### 10.3 Books

6. **Di Battista, G., Eades, P., Tamassia, R., and Tollis, I.G.** (1999). *Graph Drawing: Algorithms for the Visualization of Graphs*. Prentice Hall.
   - Comprehensive textbook covering Sugiyama and other algorithms

7. **Kaufmann, M. and Wagner, D.** (2001). *Drawing Graphs: Methods and Models*. LNCS 2025, Springer.
   - Collection of graph drawing techniques

### 10.4 Implementation References

8. **dagre** - JavaScript graph layout library
   - https://github.com/dagrejs/dagre
   - Primary reference implementation for this library

9. **Graphviz** - Graph visualization software
   - https://graphviz.org/
   - Reference implementation of network simplex

---

## Appendix A: ASCII Art Diagrams

### A.1 Complete Algorithm Flow

```
INPUT GRAPH (possibly cyclic):

      A -----> B
      ^        |
      |        v
      E <----- C -----> D


PHASE 0 - CYCLE REMOVAL:

      A -----> B
      :        |          : = reversed edge (was E->A)
      :        v
      E <----- C -----> D


PHASE 1 - LAYER ASSIGNMENT:

  Rank 0:    A
  Rank 1:    B    E
  Rank 2:    C
  Rank 3:    D


PHASE 2 - DUMMY NODES (if C->D spanned multiple layers, not needed here):

  (No change - all edges span 1 layer)


PHASE 3 - CROSSING MINIMIZATION:

  Rank 0:        A
  Rank 1:    E       B      (E moved left to reduce crossings)
  Rank 2:        C
  Rank 3:        D


PHASE 4 - COORDINATE ASSIGNMENT:

     0    50   100
     |    |    |
  0  +----A----+
     |    |    |
 50  E----+----B
     |    |    |
100  +----C----+
     |    |    |
150  +----D----+


PHASE 5 - EDGE ROUTING:

  Final edges with paths:
  - A -> B: straight line
  - B -> C: straight line
  - C -> D: straight line
  - C -> E: bend at rank 1
  - A -> E: reversed (originally E -> A)
```

### A.2 Dummy Node Example

```
BEFORE (edge A->D spans 3 layers):

  Rank 0:    A ----------+
  Rank 1:    B           |
  Rank 2:    C           |
  Rank 3:    D <---------+


AFTER NORMALIZATION:

  Rank 0:    A -----> d1
  Rank 1:    B        |
                      d2
  Rank 2:    C        |
                      d3
  Rank 3:    D <------+

  d1, d2, d3 are dummy nodes (width=0, height=0)


AFTER DENORMALIZATION (bend points):

  Rank 0:    A --+
  Rank 1:    B   * <- bend point (was d1)
  Rank 2:    C   * <- bend point (was d2)
  Rank 3:    D <-+
```

### A.3 Barycenter Calculation

```
FIXED LAYER (rank 0):

  pos:    0      1      2      3
          A      B      C      D


EDGES TO MOVABLE LAYER (rank 1):

  A -> X (weight 1)
  B -> X (weight 1)
  C -> Y (weight 2)
  D -> Y (weight 1)


BARYCENTER CALCULATION:

  X: sum = 0*1 + 1*1 = 1, weight = 2
     barycenter = 1/2 = 0.5

  Y: sum = 2*2 + 3*1 = 7, weight = 3
     barycenter = 7/3 = 2.33


RESULT:

  Sort by barycenter: X (0.5), Y (2.33)
  Final order: X, Y
```

---

## Appendix B: Glossary

| Term | Definition |
|------|------------|
| **Back Edge** | An edge from a node to one of its ancestors in DFS tree |
| **Barycenter** | Weighted average position of a node's neighbors |
| **Crossing** | Visual intersection of two edges |
| **DAG** | Directed Acyclic Graph |
| **Dummy Node** | Invisible node inserted to normalize edge lengths |
| **FAS** | Feedback Arc Set - edges to remove to make graph acyclic |
| **Inner Segment** | Edge between two dummy nodes |
| **Layer/Rank** | Horizontal level in the layout |
| **minlen** | Minimum number of layers an edge must span |
| **Network Simplex** | Algorithm for optimal layer assignment |
| **nodeSep** | Minimum horizontal separation between nodes |
| **Order** | Position of a node within its layer |
| **rankSep** | Minimum vertical separation between layers |
| **Slack** | Difference between edge length and minlen |
| **Tight Edge** | Edge where slack = 0 |

---

*Document version: 1.0*
*Last updated: January 2026*
*Based on dagre v0.8.5 implementation*
