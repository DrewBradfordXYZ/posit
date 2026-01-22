# Phase 3: Dummy Node Insertion (Normalization)

**File:** `normalize.go`

## Table of Contents

- [Goal](#goal)
- [Why This Phase is Necessary](#why-this-phase-is-necessary)
- [What Are Dummy Nodes](#what-are-dummy-nodes)
- [Algorithm](#algorithm)
- [Implementation](#implementation)
- [Dummy Node Properties](#dummy-node-properties)
- [Tracking Dummy Chains](#tracking-dummy-chains)
- [Denormalization](#denormalization)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Transform the graph so that **every edge spans exactly one layer**. This simplifies crossing minimization and coordinate assignment by ensuring uniform edge handling.

### Input
- DAG with ranks assigned (from Phase 2)
- Some edges may span multiple layers

### Output
- Normalized graph where all edges connect adjacent layers
- Dummy nodes inserted for long edges
- Tracking information for later dummy removal

---

## Why This Phase is Necessary

Without normalization, subsequent phases must handle varying edge lengths:

| Problem | Without Normalization | With Normalization |
|---------|----------------------|-------------------|
| Crossing counting | Complex (edges skip layers) | Simple (adjacent layers only) |
| Coordinate assignment | Must handle varying spans | Layer-by-layer processing |
| Edge routing | Special cases needed | Uniform handling |

### The Core Insight

A "long edge" spanning multiple layers is replaced by a **chain** of dummy nodes and short edges:

```
Before:                    After:
  A (rank 0)                 A (rank 0)
  │                          │
  │ (spans 3 layers)         ▼
  │                        [D1] (rank 1, dummy)
  │                          │
  │                          ▼
  │                        [D2] (rank 2, dummy)
  │                          │
  ▼                          ▼
  B (rank 3)                 B (rank 3)
```

---

## What Are Dummy Nodes

Dummy nodes are **invisible placeholder nodes** with special properties:

| Property | Value | Reason |
|----------|-------|--------|
| `width` | 0 | Takes no horizontal space |
| `height` | 0 | Takes no vertical space |
| `isDummy` | true | Marks node for special handling |
| `edgeLabel` | pointer to original edge | Links dummy to its edge |

Dummy nodes exist only during layout computation. They are removed in Phase 6 (Edge Routing) and converted to bend points in the final edge paths.

---

## Algorithm

### Pseudocode

```
Algorithm Normalize(G):
    dummyChains = []  // Track first dummy of each chain

    for each edge e = (v, w) in G.edges():
        vRank = G.node(v).rank
        wRank = G.node(w).rank

        if wRank == vRank + 1:
            continue  // Already spans exactly one layer

        // Remove the long edge
        edgeLabel = G.edge(e)
        G.removeEdge(e)

        // Insert chain of dummy nodes
        current = v
        firstDummy = null

        for rank = vRank + 1 to wRank - 1:
            dummy = createDummyNode({
                width: 0,
                height: 0,
                rank: rank,
                isDummy: true,
                edgeLabel: edgeLabel
            })

            G.addNode(dummy.id)
            G.addEdge(current, dummy.id)

            // Add dummy to its layer
            layers[rank].append(dummy.id)

            if firstDummy == null:
                firstDummy = dummy.id

            current = dummy.id

        // Connect last dummy to target
        G.addEdge(current, w)

        // Track the chain for later removal
        if firstDummy != null:
            dummyChains.append(firstDummy)

    return dummyChains
```

---

## Implementation

### Main Function

```go
// addDummyNodes splits edges that span multiple layers.
// Returns the number of dummy nodes created.
func (s *layoutState) addDummyNodes() int {
    dummyCount := 0

    // Collect edges to process (iterate over copy to allow modification)
    edgesToProcess := make([]edgeKey, 0, len(s.edges))
    for key := range s.edges {
        edgesToProcess = append(edgesToProcess, key)
    }

    for _, key := range edgesToProcess {
        count := s.normalizeEdge(key)
        dummyCount += count
    }

    return dummyCount
}

// normalizeEdge splits a single edge if it spans multiple layers.
// Returns the number of dummy nodes created.
func (s *layoutState) normalizeEdge(key edgeKey) int {
    edge := s.edges[key]
    if edge == nil {
        return 0
    }

    vNode := s.nodes[key.from]
    wNode := s.nodes[key.to]

    vRank := vNode.rank
    wRank := wNode.rank

    // If edge spans only one layer, nothing to do
    if wRank == vRank+1 {
        return 0
    }

    // Edge spans multiple layers - needs dummy nodes
    dummyCount := wRank - vRank - 1
    if dummyCount <= 0 {
        return 0  // Shouldn't happen with valid ranks
    }

    // Remove original edge
    s.removeEdge(key)

    // Create chain of dummy nodes
    v := key.from
    var firstDummy string

    for rank := vRank + 1; rank < wRank; rank++ {
        // Create dummy node
        dummyID := s.newDummyID()
        dummy := &layoutNode{
            id:        dummyID,
            width:     0,
            height:    0,
            rank:      rank,
            order:     -1,  // Will be set in Phase 4
            isDummy:   true,
            edgeLabel: edge,
        }

        s.nodes[dummyID] = dummy
        s.successors[dummyID] = nil
        s.predecessors[dummyID] = nil

        // Add to layer
        s.layers[rank] = append(s.layers[rank], dummyID)

        // Track first dummy for later removal
        if firstDummy == "" {
            firstDummy = dummyID
        }

        // Create edge from previous node to dummy
        s.addEdge(edgeKey{from: v, to: dummyID}, edge.weight)
        v = dummyID
    }

    // Create final edge from last dummy to target
    s.addEdge(edgeKey{from: v, to: key.to}, edge.weight)

    // Track dummy chain for reconstruction
    if firstDummy != "" {
        s.dummyChains = append(s.dummyChains, firstDummy)
    }

    return dummyCount
}
```

### Helper Functions

```go
// newDummyID generates a unique ID for a dummy node.
func (s *layoutState) newDummyID() string {
    s.dummyCounter++
    return fmt.Sprintf("_dummy_%d", s.dummyCounter)
}

// addEdge adds a new edge with specified weight.
func (s *layoutState) addEdge(key edgeKey, weight float64) {
    s.edges[key] = &layoutEdge{
        key:    key,
        weight: weight,
        minlen: 1,
    }
    s.successors[key.from] = append(s.successors[key.from], key.to)
    s.predecessors[key.to] = append(s.predecessors[key.to], key.from)
}

// removeEdge removes an edge and updates adjacency lists.
func (s *layoutState) removeEdge(key edgeKey) {
    delete(s.edges, key)
    s.successors[key.from] = removeString(s.successors[key.from], key.to)
    s.predecessors[key.to] = removeString(s.predecessors[key.to], key.from)
}
```

---

## Dummy Node Properties

### In the Graph

```go
// A dummy node in s.nodes:
dummyNode := &layoutNode{
    id:        "_dummy_1",
    width:     0,
    height:    0,
    rank:      2,           // The layer this dummy occupies
    order:     -1,          // Set in Phase 4
    x:         0,           // Set in Phase 5
    y:         0,           // Set in Phase 5
    isDummy:   true,
    edgeLabel: originalEdge, // Points to the original edge
}
```

### In the Layers

```go
// After normalization, layers include dummies:
s.layers = [][]string{
    {"A"},                    // Rank 0: real node
    {"B", "_dummy_1"},        // Rank 1: real + dummy
    {"_dummy_2", "_dummy_3"}, // Rank 2: all dummies
    {"C", "D"},               // Rank 3: real nodes
}
```

---

## Tracking Dummy Chains

Each original long edge becomes a chain of dummy nodes. We track the **first dummy** in each chain:

```go
s.dummyChains = []string{"_dummy_1", "_dummy_5", "_dummy_8"}
```

### Why Track Chains?

In Phase 6, we need to:
1. Walk each chain to collect bend points
2. Restore the original edge with those points
3. Remove all dummy nodes from the output

The chain is followed via successors:
```go
current := s.dummyChains[i]
for s.nodes[current].isDummy {
    // Process this dummy
    next := s.successors[current][0]  // Dummies have exactly 1 successor
    current = next
}
// current is now the target node
```

---

## Denormalization

After coordinates are assigned (Phase 5), dummy nodes are converted to bend points:

```go
// denormalize converts dummy chains to edge bend points.
// Called in Phase 6 (Edge Routing).
func (s *layoutState) denormalize() {
    for _, firstDummy := range s.dummyChains {
        dummy := s.nodes[firstDummy]
        edge := dummy.edgeLabel
        if edge == nil {
            continue
        }

        // Initialize points array
        edge.points = make([]EdgePoint, 0)

        // Walk the chain, collecting coordinates
        current := firstDummy
        for {
            node := s.nodes[current]
            if !node.isDummy {
                break
            }

            // Add dummy's position as bend point
            edge.points = append(edge.points, EdgePoint{
                X: node.x,
                Y: node.y,
            })

            // Move to next in chain
            successors := s.successors[current]
            if len(successors) == 0 {
                break
            }
            current = successors[0]
        }
    }
}
```

---

## Complexity Analysis

| Operation | Time | Space |
|-----------|------|-------|
| Process each edge | O(1) per edge | O(1) |
| Create dummies | O(span) per long edge | O(span) per edge |
| Add to layers | O(1) per dummy | O(1) |
| **Total** | **O(E × L)** | **O(E × L)** |

Where L is the maximum edge span (number of layers an edge crosses).

### Worst Case

In pathological cases (e.g., star graph with very long edges), the number of dummy nodes can be O(V × L), where L is the total number of layers. For typical graphs, it's much smaller.

---

## Testing

### Test Cases

```go
func TestNormalize_NoLongEdges(t *testing.T) {
    // A → B → C (all edges span 1 layer)
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    initialNodes := len(state.nodes)
    state.addDummyNodes()

    // No dummies should be added
    if len(state.nodes) != initialNodes {
        t.Errorf("Expected no dummies, got %d extra nodes",
            len(state.nodes)-initialNodes)
    }
}

func TestNormalize_SingleLongEdge(t *testing.T) {
    // A → B → C with long edge A → C
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("A", "C")  // Long edge spans 2 layers

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    dummyCount := state.addDummyNodes()

    // A→C spans ranks 0→2, needs 1 dummy at rank 1
    if dummyCount != 1 {
        t.Errorf("Expected 1 dummy, got %d", dummyCount)
    }
}

func TestNormalize_VeryLongEdge(t *testing.T) {
    // Chain of 5 nodes, plus edge from first to last
    g := NewGraph()
    for i := 0; i < 5; i++ {
        g.AddNode(fmt.Sprintf("N%d", i), NodeOptions{Width: 100, Height: 50})
    }
    for i := 0; i < 4; i++ {
        g.AddEdge(fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
    }
    g.AddEdge("N0", "N4")  // Long edge spans 4 layers

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()

    dummyCount := state.addDummyNodes()

    // N0→N4 spans ranks 0→4, needs 3 dummies
    if dummyCount != 3 {
        t.Errorf("Expected 3 dummies, got %d", dummyCount)
    }
}

func TestNormalize_DummyConnections(t *testing.T) {
    // Verify each dummy has exactly 1 in-edge and 1 out-edge
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("A", "C")  // Creates 1 dummy

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()

    for id, node := range state.nodes {
        if !node.isDummy {
            continue
        }

        inDegree := len(state.predecessors[id])
        outDegree := len(state.successors[id])

        if inDegree != 1 {
            t.Errorf("Dummy %s has in-degree %d, expected 1", id, inDegree)
        }
        if outDegree != 1 {
            t.Errorf("Dummy %s has out-degree %d, expected 1", id, outDegree)
        }
    }
}

func TestNormalize_DummyInCorrectLayer(t *testing.T) {
    // Verify dummies are in the correct layers
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddNode("D", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("C", "D")
    g.AddEdge("A", "D")  // Spans 3 layers, needs 2 dummies

    state := newLayoutState(g, DefaultOptions())
    state.makeAcyclic()
    state.assignLayers()
    state.addDummyNodes()

    for id, node := range state.nodes {
        if !node.isDummy {
            continue
        }

        // Verify node is in the layer array at its rank
        found := false
        for _, nodeID := range state.layers[node.rank] {
            if nodeID == id {
                found = true
                break
            }
        }

        if !found {
            t.Errorf("Dummy %s (rank %d) not found in layers[%d]",
                id, node.rank, node.rank)
        }
    }
}
```

---

## Visual Examples

### Example 1: Single Long Edge

```
BEFORE NORMALIZATION:

Rank 0:    A ─────────┐
           │          │
Rank 1:    B          │ (long edge)
           │          │
Rank 2:    C ◄────────┘

Edge A→C spans 2 layers (0→2)


AFTER NORMALIZATION:

Rank 0:    A ─────────┐
           │          │
           ▼          ▼
Rank 1:    B        [D1] ← dummy node
           │          │
           ▼          ▼
Rank 2:    C ◄────────┘

New edges: A→D1, D1→C
Original A→C is removed
D1.edgeLabel points to original edge
```

### Example 2: Multiple Long Edges

```
BEFORE:

Rank 0:    A ─────────────┐
           │              │
Rank 1:    B              │
           │              │
Rank 2:    C ─────────┐   │
           │          │   │
Rank 3:    D ◄────────┼───┘
                      │
Rank 4:    E ◄────────┘

Long edges: A→D (spans 3), C→E (spans 2)


AFTER:

Rank 0:    A ─────────────┐
           │              │
           ▼              ▼
Rank 1:    B            [D1]
           │              │
           ▼              ▼
Rank 2:    C ─────────┐ [D2]
           │          │   │
           ▼          ▼   ▼
Rank 3:    D ◄────────┼───┘
                    [D3]
                      │
                      ▼
Rank 4:    E ◄────────┘

Dummy chains:
  A→D: A → D1 → D2 → D
  C→E: C → D3 → E
```

### Example 3: State After Normalization

```go
// After normalizing the graph above:
s.nodes = {
    "A":  {rank: 0, isDummy: false, ...},
    "B":  {rank: 1, isDummy: false, ...},
    "C":  {rank: 2, isDummy: false, ...},
    "D":  {rank: 3, isDummy: false, ...},
    "E":  {rank: 4, isDummy: false, ...},
    "_dummy_1": {rank: 1, isDummy: true, edgeLabel: edgeA_D},
    "_dummy_2": {rank: 2, isDummy: true, edgeLabel: edgeA_D},
    "_dummy_3": {rank: 3, isDummy: true, edgeLabel: edgeC_E},
}

s.layers = {
    [0]: ["A"],
    [1]: ["B", "_dummy_1"],
    [2]: ["C", "_dummy_2"],
    [3]: ["D", "_dummy_3"],
    [4]: ["E"],
}

s.dummyChains = ["_dummy_1", "_dummy_3"]

s.edges = {
    {A, B},
    {B, C},
    {C, D},
    {D, E},
    {A, _dummy_1},      // First segment of A→D
    {_dummy_1, _dummy_2}, // Middle segment
    {_dummy_2, D},      // Last segment
    {C, _dummy_3},      // First segment of C→E
    {_dummy_3, E},      // Last segment
}
```

---

## Post-Conditions

After this phase completes:

1. ✅ Every edge in `s.edges` spans exactly one rank
2. ✅ Dummy nodes have `width=0`, `height=0`, `isDummy=true`
3. ✅ Each dummy points to its original edge via `edgeLabel`
4. ✅ `s.dummyChains` contains first dummy of each chain
5. ✅ `s.layers` includes dummy nodes in correct positions
6. ✅ Adjacency lists are updated for new edges

---

## Next Phase

→ [Phase 4: Crossing Minimization](./PHASE_4_CROSSING_MINIMIZATION.md)
