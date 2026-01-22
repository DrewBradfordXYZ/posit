# Phase 0: Algorithm Overview

This document provides an overview of the Sugiyama algorithm execution phases as implemented in the **posit** library.

## Table of Contents

- [Implementation Status](#implementation-status)
- [Introduction](#introduction)
- [The 6-Phase Pipeline](#the-6-phase-pipeline)
- [Data Flow Summary](#data-flow-summary)
- [State Management](#state-management)
- [Entry Point](#entry-point)
- [Phase Dependencies](#phase-dependencies)

---

## Implementation Status

| Phase | Status | File | Tests |
|-------|--------|------|-------|
| 1. Cycle Removal | ✅ Complete | `acyclic.go` | 9 tests |
| 2. Layer Assignment | ✅ Complete | `rank.go` | 11 tests |
| 3. Dummy Nodes | ✅ Complete | `normalize.go` | 15 tests |
| 4. Crossing Minimization | ⏳ Stub | `stubs.go` | - |
| 5. Coordinate Assignment | ⏳ Stub | `stubs.go` | - |
| 6. Edge Routing | ⏳ Stub | `stubs.go` | - |

**Foundation:** `state.go` contains `layoutState`, `layoutNode`, `layoutEdge`, and `edgeKey` types.

---

## Introduction

The Sugiyama algorithm transforms the complex 2D graph layout problem into a series of simpler 1D problems. Each phase has a single responsibility and operates on shared internal state, building upon the results of previous phases.

### Core Insight

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

## The 6-Phase Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                        INPUT                                     │
│  Directed graph (possibly cyclic) with node dimensions          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 1: CYCLE REMOVAL                                         │
│  ─────────────────────                                          │
│  File: acyclic.go                                               │
│  Purpose: Transform cyclic graph into DAG                       │
│  Method: DFS-based back-edge detection and reversal             │
│  Output: Acyclic graph with some edges marked as "reversed"     │
│  Complexity: O(V + E)                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 2: LAYER ASSIGNMENT (Ranking)                            │
│  ────────────────────────────────────                           │
│  File: rank.go                                                  │
│  Purpose: Assign each node to a discrete layer (Y-coordinate)   │
│  Methods: Longest Path (fast) or Network Simplex (optimal)      │
│  Output: Each node has a `rank` attribute (0, 1, 2, ...)        │
│  Complexity: O(V + E) to O(V * E) depending on algorithm        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 3: DUMMY NODE INSERTION (Normalization)                  │
│  ─────────────────────────────────────────────                  │
│  File: normalize.go                                             │
│  Purpose: Ensure all edges span exactly one layer               │
│  Method: Insert invisible "dummy" nodes for long edges          │
│  Output: Normalized graph where every edge is between           │
│          adjacent layers                                        │
│  Complexity: O(E * L) where L = max edge span                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 4: CROSSING MINIMIZATION (Ordering)                      │
│  ─────────────────────────────────────────                      │
│  File: order.go                                                 │
│  Purpose: Order nodes within each layer to minimize crossings   │
│  Method: Barycenter heuristic with layer sweeps                 │
│  Output: Each node has an `order` attribute (position in layer) │
│  Complexity: O(k * L * E log V) where k = iterations            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 5: COORDINATE ASSIGNMENT (Positioning)                   │
│  ─────────────────────────────────────────────                  │
│  File: position.go                                              │
│  Purpose: Assign actual X and Y pixel coordinates               │
│  Method: Brandes-Kopf algorithm (4-alignment median)            │
│  Output: Each node has `x` and `y` coordinates                  │
│  Complexity: O(V + E)                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PHASE 6: EDGE ROUTING                                          │
│  ─────────────────────                                          │
│  File: route.go                                                 │
│  Purpose: Generate final edge paths as polylines                │
│  Method: Collect dummy node positions, restore reversed edges   │
│  Output: Edge paths as arrays of points, dummy nodes removed    │
│  Complexity: O(E * L)                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        OUTPUT                                    │
│  Layout with X/Y positions for all nodes                        │
│  Edge paths as polylines (arrays of points)                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Data Flow Summary

| Phase | Input | Output | Key Attribute Added |
|-------|-------|--------|---------------------|
| 1. Cycle Removal | Possibly cyclic digraph | DAG | `edge.reversed` |
| 2. Layer Assignment | DAG | Layered DAG | `node.rank` |
| 3. Dummy Nodes | Layered DAG | Normalized graph | dummy nodes |
| 4. Crossing Min | Normalized graph | Ordered layers | `node.order` |
| 5. Coordinates | Ordered graph | Positioned graph | `node.x`, `node.y` |
| 6. Edge Routing | Positioned graph | Final layout | `edge.points` |

---

## State Management

All phases operate on a shared `layoutState` structure:

```go
type layoutState struct {
    // Configuration
    opts Options

    // Node data (includes dummies during processing)
    nodes map[string]*layoutNode

    // Edge data
    edges map[edgeKey]*layoutEdge

    // Adjacency lists for fast traversal
    successors   map[string][]string
    predecessors map[string][]string

    // Layer structure (built in Phase 2, refined in Phase 3-4)
    layers [][]string  // layers[rank] = ordered node IDs

    // Tracking for cleanup
    reversedEdges []edgeKey  // edges to flip back
    dummyChains   []string   // first dummy in each chain
}
```

### layoutNode Structure

```go
type layoutNode struct {
    id     string
    width  float64
    height float64

    // Phase 2 output
    rank int

    // Phase 4 output
    order int

    // Phase 5 output
    x, y float64

    // Dummy node tracking
    isDummy   bool
    edgeLabel *layoutEdge  // original edge for dummies
}
```

### layoutEdge Structure

```go
type layoutEdge struct {
    key      edgeKey
    weight   float64
    minlen   int
    reversed bool

    // Phase 6 output
    points []EdgePoint
}
```

---

## Entry Point

The layout process is orchestrated in `state.go`:

```go
func (g *Graph) Layout(opts ...Options) *Layout {
    opt := resolveOptions(opts)

    // Initialize internal state
    state := newLayoutState(g, opt)

    // Execute phases in order
    state.makeAcyclic()        // Phase 1
    state.assignLayers()       // Phase 2
    state.addDummyNodes()      // Phase 3
    state.minimizeCrossings()  // Phase 4
    state.assignCoordinates()  // Phase 5
    state.routeEdges()         // Phase 6

    // Build output
    return state.buildLayout()
}
```

---

## Phase Dependencies

```
                    ┌─────────────┐
                    │ state.go    │  Foundation for all phases
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ acyclic.go  │  Requires: adjacency lists
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ rank.go     │  Requires: acyclic graph
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ normalize.go│  Requires: rank assignments
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ order.go    │  Requires: layers, dummy nodes
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ position.go │  Requires: node order within layers
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ route.go    │  Requires: coordinates, dummy positions
                    └─────────────┘
```

### Stubbing for Incremental Development

Phases can be stubbed for early testing:

```go
// Stub for order.go - just use initial order
func (s *layoutState) minimizeCrossings() {
    // Keep DFS traversal order
}

// Stub for position.go - simple grid layout
func (s *layoutState) assignCoordinates() {
    for rank, layer := range s.layers {
        y := float64(rank) * s.opts.RankSep
        x := 0.0
        for _, nodeID := range layer {
            node := s.nodes[nodeID]
            node.x = x
            node.y = y
            x += node.width + s.opts.NodeSep
        }
    }
}
```

---

## Key Invariants by Phase

| Phase | Invariant |
|-------|-----------|
| 1. Acyclic | No back-edges exist in successor traversal |
| 2. Rank | All edges go from lower rank to higher rank |
| 3. Normalize | All edges span exactly 1 rank |
| 4. Order | Crossing count is locally minimal |
| 5. Position | No node overlaps; respects NodeSep/RankSep |
| 6. Route | All edges have valid point arrays |

---

## Complexity Summary

| Phase | Time Complexity | Space Complexity |
|-------|-----------------|------------------|
| Cycle Removal | O(V + E) | O(V) |
| Layer Assignment (Longest Path) | O(V + E) | O(V) |
| Layer Assignment (Network Simplex) | O(V * E) typical | O(V + E) |
| Dummy Node Insertion | O(E * L) | O(E * L) |
| Crossing Minimization | O(k * L * E log V) | O(V + E) |
| Coordinate Assignment | O(V + E) | O(V) |
| Edge Routing | O(E * L) | O(E * L) |

Where: V = vertices, E = edges, L = layers, k = crossing minimization iterations

---

## Next Steps

Continue to the individual phase documents for detailed implementation guidance:

1. ✅ [Phase 1: Cycle Removal](./PHASE_1_CYCLE_REMOVAL.md) — Complete
2. ✅ [Phase 2: Layer Assignment](./PHASE_2_LAYER_ASSIGNMENT.md) — Complete
3. ✅ [Phase 3: Dummy Nodes](./PHASE_3_DUMMY_NODES.md) — Complete
4. [Phase 4: Crossing Minimization](./PHASE_4_CROSSING_MINIMIZATION.md) — **Next**
5. [Phase 5: Coordinate Assignment](./PHASE_5_COORDINATE_ASSIGNMENT.md)
6. [Phase 6: Edge Routing](./PHASE_6_EDGE_ROUTING.md)
