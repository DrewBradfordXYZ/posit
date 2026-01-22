# Phase 1: Cycle Removal

**File:** `acyclic.go`

## Table of Contents

- [Goal](#goal)
- [Why This Phase is Necessary](#why-this-phase-is-necessary)
- [Algorithm: DFS-Based Cycle Detection](#algorithm-dfs-based-cycle-detection)
- [Implementation](#implementation)
- [Edge Reversal](#edge-reversal)
- [Undoing the Reversal](#undoing-the-reversal)
- [Edge Cases](#edge-cases)
- [Complexity Analysis](#complexity-analysis)
- [Testing](#testing)
- [Visual Examples](#visual-examples)

---

## Goal

Transform a potentially cyclic directed graph into a **Directed Acyclic Graph (DAG)** by identifying and reversing "back edges" — edges that create cycles.

### Input
- Directed graph (may contain cycles)

### Output
- Directed Acyclic Graph (DAG)
- List of edges that were reversed (for later restoration)

---

## Why This Phase is Necessary

The Sugiyama algorithm requires a DAG because:

1. **Layer assignment assumes edges flow in one direction** — from lower ranks to higher ranks
2. **Cycles would create contradictory constraints** — if A→B and B→C→A exist, then A must be both above and below B

```
Problem with cycles:

    A ────→ B
    ^       │
    │       v
    └────── C

If A → B: rank(A) < rank(B)
If B → C: rank(B) < rank(C)
If C → A: rank(C) < rank(A)

Contradiction: rank(A) < rank(B) < rank(C) < rank(A)
```

By reversing one edge (e.g., C→A becomes A→C), we break the cycle and can assign consistent ranks.

---

## Algorithm: DFS-Based Cycle Detection

The algorithm uses Depth-First Search to find a **Feedback Arc Set (FAS)** — a set of edges whose removal makes the graph acyclic.

### Key Insight

An edge (v, w) creates a cycle **if and only if** w is already on the current DFS path. These are called **back edges**.

### Node States During DFS

| State | Meaning |
|-------|---------|
| Unvisited | Node not yet encountered |
| In Stack (Gray) | Node is on current DFS path |
| Visited (Black) | Node and all descendants processed |

### Pseudocode

```
Algorithm DFS-FAS(G):
    fas = []           // Feedback Arc Set (edges to reverse)
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
            else if w not in visited:
                dfs(w)

        delete stack[v]    // v is no longer on current path

    // Start DFS from all nodes (handles disconnected components)
    for each node v in G.nodes():
        dfs(v)

    return fas
```

---

## Implementation

### Main Function

```go
// makeAcyclic reverses edges to break cycles in the graph.
// Returns the list of edges that were reversed.
func (s *layoutState) makeAcyclic() []edgeKey {
    visited := make(map[string]bool, len(s.nodes))
    onStack := make(map[string]bool) // currently in DFS path

    var reversed []edgeKey

    var dfs func(v string)
    dfs = func(v string) {
        if visited[v] {
            return
        }
        visited[v] = true
        onStack[v] = true

        // Iterate over copy to allow modification during iteration
        successors := make([]string, len(s.successors[v]))
        copy(successors, s.successors[v])

        for _, w := range successors {
            if onStack[w] {
                // Back edge found - reverse it
                key := edgeKey{from: v, to: w}
                s.reverseEdge(key)
                reversed = append(reversed, key)
            } else if !visited[w] {
                dfs(w)
            }
        }

        delete(onStack, v)
    }

    // Start DFS from all nodes
    for id := range s.nodes {
        dfs(id)
    }

    s.reversedEdges = reversed
    return reversed
}
```

### Edge Reversal Helper

```go
// reverseEdge flips an edge's direction and updates adjacency lists.
func (s *layoutState) reverseEdge(key edgeKey) {
    edge := s.edges[key]
    if edge == nil {
        return
    }

    // Remove from adjacency lists
    s.successors[key.from] = removeString(s.successors[key.from], key.to)
    s.predecessors[key.to] = removeString(s.predecessors[key.to], key.from)

    // Create reversed edge
    newKey := edgeKey{from: key.to, to: key.from}
    delete(s.edges, key)

    edge.key = newKey
    edge.reversed = true
    s.edges[newKey] = edge

    // Add to adjacency lists in new direction
    s.successors[newKey.from] = append(s.successors[newKey.from], newKey.to)
    s.predecessors[newKey.to] = append(s.predecessors[newKey.to], newKey.from)
}

// removeString removes first occurrence of s from slice.
func removeString(slice []string, s string) []string {
    for i, v := range slice {
        if v == s {
            return append(slice[:i], slice[i+1:]...)
        }
    }
    return slice
}
```

---

## Edge Reversal

When we find a back edge, we don't remove it — we reverse its direction:

```
Before reversal:

    A ────→ B
    ^       │
    │       v
    └────── C

Edge C→A is a back edge (A is ancestor of C in DFS)

After reversal:

    A ────→ B
    │       │
    v       v
    ← ← ← ← C

Edge is now A→C (marked as reversed=true)
```

### Why Reverse Instead of Remove?

1. **Preserves connectivity** — All original relationships are maintained
2. **Enables restoration** — We can flip the edge back after layout
3. **Visual hint** — Reversed edges can be styled differently (dashed, etc.)

---

## Undoing the Reversal

After coordinate assignment (Phase 5), reversed edges are flipped back to their original direction:

```go
// undoReversals restores all reversed edges to original direction.
func (s *layoutState) undoReversals() {
    for _, key := range s.reversedEdges {
        edge := s.edges[key]
        if edge == nil {
            continue
        }

        // Reverse the points array
        for i, j := 0, len(edge.points)-1; i < j; i, j = i+1, j-1 {
            edge.points[i], edge.points[j] = edge.points[j], edge.points[i]
        }

        // Swap the edge key back to original direction
        delete(s.edges, key)
        originalKey := edgeKey{from: key.to, to: key.from}
        edge.key = originalKey
        edge.reversed = false
        s.edges[originalKey] = edge
    }
}
```

---

## Edge Cases

### 1. Self-Loops

An edge (v, v) is always a back edge:

```go
// Handle self-loops before main DFS
func (s *layoutState) removeSelfLoops() []edgeKey {
    var selfLoops []edgeKey

    for key := range s.edges {
        if key.from == key.to {
            selfLoops = append(selfLoops, key)
            s.removeEdge(key)
        }
    }

    return selfLoops
}
```

### 2. Disconnected Components

The algorithm handles disconnected components by iterating over all nodes:

```go
for id := range s.nodes {
    dfs(id)  // Will skip already-visited nodes
}
```

### 3. Already Acyclic Graph

No edges are reversed; the algorithm still runs in O(V + E) but produces an empty feedback arc set.

### 4. Multiple Cycles Sharing Edges

The DFS naturally handles overlapping cycles — each back edge is reversed only once.

---

## Complexity Analysis

| Operation | Time | Space |
|-----------|------|-------|
| DFS traversal | O(V + E) | O(V) for recursion stack |
| Edge reversal | O(1) per edge | O(1) |
| **Total** | **O(V + E)** | **O(V)** |

---

## Testing

### Test Cases

```go
func TestAcyclic_SimpleDAG(t *testing.T) {
    // A → B → C (no cycles)
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")

    state := newLayoutState(g, DefaultOptions())
    reversed := state.makeAcyclic()

    if len(reversed) != 0 {
        t.Errorf("Expected no reversed edges for DAG, got %d", len(reversed))
    }
}

func TestAcyclic_SingleCycle(t *testing.T) {
    // A → B → C → A (one cycle)
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddNode("C", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "C")
    g.AddEdge("C", "A")  // Creates cycle

    state := newLayoutState(g, DefaultOptions())
    reversed := state.makeAcyclic()

    if len(reversed) != 1 {
        t.Errorf("Expected 1 reversed edge, got %d", len(reversed))
    }

    // Verify graph is now acyclic
    if hasCycle(state) {
        t.Error("Graph still has cycle after makeAcyclic")
    }
}

func TestAcyclic_MultipleCycles(t *testing.T) {
    // Two independent cycles
    g := NewGraph()
    // Cycle 1: A → B → A
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B")
    g.AddEdge("B", "A")

    // Cycle 2: X → Y → X
    g.AddNode("X", NodeOptions{Width: 100, Height: 50})
    g.AddNode("Y", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("X", "Y")
    g.AddEdge("Y", "X")

    state := newLayoutState(g, DefaultOptions())
    reversed := state.makeAcyclic()

    if len(reversed) < 2 {
        t.Errorf("Expected at least 2 reversed edges, got %d", len(reversed))
    }
}

// Helper to verify acyclicity
func hasCycle(s *layoutState) bool {
    visited := make(map[string]bool)
    onStack := make(map[string]bool)

    var dfs func(v string) bool
    dfs = func(v string) bool {
        if onStack[v] {
            return true // cycle found
        }
        if visited[v] {
            return false
        }

        visited[v] = true
        onStack[v] = true

        for _, w := range s.successors[v] {
            if dfs(w) {
                return true
            }
        }

        delete(onStack, v)
        return false
    }

    for id := range s.nodes {
        if dfs(id) {
            return true
        }
    }
    return false
}
```

---

## Visual Examples

### Example 1: Simple Cycle

```
BEFORE (with cycle):

    A ────→ B
    ^       │
    │       v
    └────── C

DFS from A:
  Visit A (stack: {A})
    Visit B (stack: {A, B})
      Visit C (stack: {A, B, C})
        Edge C→A: A is in stack!
        → Back edge found, reverse C→A
      Leave C (stack: {A, B})
    Leave B (stack: {A})
  Leave A (stack: {})

AFTER (acyclic):

    A ────→ B
    │       │
    v       v
    ─────→ C

Edge A→C is marked as reversed=true
Original direction (C→A) will be restored after layout
```

### Example 2: Complex Graph

```
BEFORE:

    A ────→ B ────→ C
    │       │       │
    v       v       v
    D ←──── E ────→ F
    │               │
    └───────────────┘

Cycle 1: A → D → ... (no, D has no path back to A)
Cycle 2: D → F → D? (yes, if F→D exists)

Let's say we have: D → F and F → D
DFS would find F→D as back edge and reverse it.

AFTER:

    A ────→ B ────→ C
    │       │       │
    v       v       v
    D ←──── E ────→ F
    │               ^
    └───────────────┘

Edge D→F reversed from F→D
```

---

## Alternative: Greedy FAS

For weighted graphs where some edges are more important, dagre supports a greedy heuristic:

> P. Eades, X. Lin, and W. F. Smyth. "A fast and effective heuristic for the feedback arc set problem."

This algorithm prioritizes keeping high-weight edges in their original direction. The simple DFS approach is usually sufficient for layout purposes.

---

## Post-Conditions

After this phase completes:

1. ✅ Graph has no cycles (can be verified with DFS)
2. ✅ `s.reversedEdges` contains all edges that were flipped
3. ✅ Each reversed edge has `edge.reversed = true`
4. ✅ Adjacency lists (`s.successors`, `s.predecessors`) are consistent

---

## Next Phase

→ [Phase 2: Layer Assignment](./PHASE_2_LAYER_ASSIGNMENT.md)
