# Phase 9: Dagre Parity & Code Quality

**Status:** Complete

**Scope:** Missing features, bug fixes, and code quality improvements identified through comprehensive code review against dagre reference implementation.

## Table of Contents

- [Goal](#goal)
- [Review Summary](#review-summary)
- [9.1 Missing Features](#91-missing-features)
- [9.2 Bug Fixes](#92-bug-fixes)
- [9.3 Code Quality](#93-code-quality)
- [9.4 Performance Optimizations](#94-performance-optimizations)
- [Implementation Priority](#implementation-priority)

---

## Goal

Achieve full feature parity with dagre and improve code quality based on comprehensive code review. This phase addresses gaps identified by comparing our implementation against `_refs/dagre/lib/`.

---

## Review Summary

### Algorithmic Review (vs dagre)

| Module | Posit File | Dagre Reference | Parity |
|--------|------------|-----------------|--------|
| Cycle removal | `acyclic.go` | `acyclic.js`, `greedy-fas.js` | 70% |
| Layer assignment | `rank.go` | `rank/*.js` | 80% |
| Network simplex | `simplex.go` | `rank/network-simplex.js` | 95% |
| Normalization | `normalize.go` | `normalize.js` | 60% |
| Crossing minimization | `order.go` | `order/*.js` | 75% |
| Coordinate assignment | `position.go` | `position/bk.js` | 70% |
| Edge routing | `route.go` | `layout.js` | 60% |

### Code Quality Rating: 7.5/10

**Strengths:**
- Clean API surface
- Good file organization by algorithm phase
- Correct core algorithm implementations
- No external dependencies
- Proper nil checks

**Areas for Improvement:**
- Error handling (silent failures)
- API design (ambiguous edge keys, variadic options)
- Non-deterministic map iteration
- Missing documentation

---

## 9.1 Missing Features

### 9.1.1 Edge Label Positioning (High Priority)

**Reference:** `normalize.js:38-58`, `layout.js:285-298`

Dagre supports edge labels with automatic positioning:

```javascript
// dagre normalize.js - tracks label position
if (vRank === labelRank) {
    attrs.width = edgeLabel.width;
    attrs.height = edgeLabel.height;
    attrs.dummy = "edge-label";
    attrs.labelpos = edgeLabel.labelpos;
}
```

**Current State:** Posit's dummy nodes are always zero-sized with no label tracking.

**Implementation Plan:**
1. Add `labelWidth`, `labelHeight`, `labelPos` to `EdgeOptions`
2. Modify `normalize.go` to track `labelRank` and create label-sized dummies
3. Add `Label` field to `EdgeLayout` with X/Y coordinates
4. Modify `route.go` to extract label coordinates during denormalization

**Estimated scope:** ~80 lines

---

### 9.1.2 Greedy FAS Algorithm (High Priority)

**Reference:** `greedy-fas.js` (125 lines)

The Eades/Lin/Smyth heuristic for weighted feedback arc sets produces significantly better results for graphs with weighted edges.

**Current State:** Posit only uses DFS-based cycle detection.

**Algorithm:**
```
1. Partition nodes into buckets based on in/out degree
2. Repeatedly extract sources (no incoming) and sinks (no outgoing)
3. When no sources/sinks remain, pick node with max (out-degree - in-degree)
4. Edges pointing "backward" in this ordering are reversed
```

**Implementation Plan:**
1. Add `Acyclicer` option: `DFS` (default) or `Greedy`
2. Create `greedy_fas.go` implementing the bucket-based algorithm
3. Integrate in `acyclic.go` via switch statement

**Estimated scope:** ~100 lines

---

### 9.1.3 Self-Edge Rendering (Medium Priority)

**Reference:** `layout.js:345-388`

**Current State:** Self-loops are removed and never restored (see `acyclic.go:51-57`).

**Implementation Plan:**
1. Track self-loops separately instead of removing
2. Add self-loop path generation (curved return path)
3. Include in `EdgeLayout` output

**Estimated scope:** ~60 lines

---

### 9.1.4 Tight-Tree Ranker (Medium Priority)

**Reference:** `rank/index.js:47-50`

A middle-ground between fastest (longest-path) and optimal (network simplex).

```javascript
// dagre offers tight-tree as an option
case "tight-tree":
    longestPath(g);
    feasibleTree(g);  // Tighten without full simplex optimization
```

**Implementation Plan:**
1. Add `TightTree` to `RankAlgorithm` enum
2. Implement as longest-path followed by `feasibleTree()` without pivot loop

**Estimated scope:** ~20 lines

---

### 9.1.5 Type-2 Conflict Detection (Medium Priority)

**Reference:** `position/bk.js:83-127`

Required for proper compound graph support. Detects when inner segments cross non-inner segments.

**Current State:** Not implemented.

**Estimated scope:** ~50 lines

---

### 9.1.6 Block Graph Compaction (Medium Priority)

**Reference:** `position/bk.js:267-287`

Dagre builds explicit ordering constraints between adjacent blocks for sophisticated compaction.

**Current State:** Posit uses simpler recursive placement.

**Implementation Plan:**
1. Build block graph during horizontal compaction
2. Use two-pass iteration (predecessors then successors)

**Estimated scope:** ~80 lines

---

### 9.1.7 Compound Graph Support (Low Priority)

**Reference:** `order/sort-subgraph.js`

Support for nested subgraphs (clusters).

**Current State:** Not implemented.

**Estimated scope:** ~200+ lines (significant effort)

---

## 9.2 Bug Fixes

### 9.2.1 Smallest Width Alignment (High Priority)

**Location:** `position.go:503-534`

**Issue:** Aligns all coordinate sets to have the same minimum X, rather than finding the alignment with smallest actual width.

**Current (incorrect):**
```go
func (s *layoutState) alignCoordinatesToSmallest(xss map[string]map[string]float64) {
    // Find minimum X for each alignment
    // Shift each alignment so its minimum equals the global minimum
}
```

**Dagre (correct):**
```javascript
function findSmallestWidthAlignment(g, xss) {
    // Calculate actual width (max - min considering node widths)
    // Return the alignment with smallest width
}
```

**Impact:** Suboptimal layout width.

**Fix:** Calculate `max(x + width/2) - min(x - width/2)` for each alignment, pick smallest.

---

### 9.2.2 Self-Loops Lost (High Priority)

**Location:** `acyclic.go:51-57`

**Issue:** Self-loops are removed and never restored.

```go
// Current: removes self-loops permanently
if v == w {
    selfLoops = append(selfLoops, key)
    continue
}
```

**Impact:** Self-referential edges disappear from output.

**Fix:** Track self-loops and generate curved paths in `route.go`.

---

### 9.2.3 Y-Coordinate Reference Point (Medium Priority)

**Location:** `position.go:42`

**Issue:** Uses top of node rather than center for Y coordinate.

**Dagre:** `g.node(v).y = prevY + maxHeight / 2` (center-based)

**Posit:** `s.nodes[id].y = y` (top-based)

**Impact:** May cause integration issues with renderers expecting center-based coordinates.

**Fix:** Add `MaxHeight/2` offset, or document coordinate system clearly.

---

### 9.2.4 Non-Deterministic Output (Medium Priority)

**Locations:**
- `simplex.go:114-118` - Arbitrary root node selection
- `acyclic.go:42-44` - DFS starting order affects edge reversal

**Issue:** Map iteration order is non-deterministic, leading to different layouts for same input.

**Fix:** Sort node IDs before iteration in order-sensitive operations.

---

### 9.2.5 FeasibleTree Efficiency (Low Priority)

**Location:** `simplex.go:110-176`

**Issue:** Adjusts ALL tree node ranks on each edge addition.

**Dagre:** Uses two-phase approach (DFS for tight edges, then min-slack only when needed).

**Impact:** Performance only; correctness is fine.

---

## 9.3 Code Quality

### 9.3.1 Error Handling (High Priority)

**Issue:** `Layout()` cannot return errors; failures are silent.

**Locations:**
- `posit.go:202-236` - Main entry point
- `acyclic.go:61-64` - `reverseEdge()` silently returns on nil
- `simplex.go:403-406` - `enterEdge()` returns empty struct on failure

**Recommendation:** Consider `(*Layout, error)` return type or at minimum validate options.

---

### 9.3.2 Edge Key Ambiguity (High Priority)

**Location:** `state.go:134-136`

**Issue:** Edge keys use `->` concatenation which is ambiguous if node IDs contain that string.

```go
edgeID := key.from + "->" + key.to
layout.Edges[edgeID] = EdgeLayout{...}
```

**Fix Options:**
1. Document restriction on node IDs
2. Use a different separator (e.g., `\x00`)
3. Change to struct key or `[2]string`

---

### 9.3.3 Variadic Options Pattern (Medium Priority)

**Location:** `posit.go:202-206`

**Issue:** Only first option used; additional options silently ignored.

```go
func (g *Graph) Layout(opts ...Options) *Layout {
    if len(opts) > 0 {
        opt = opts[0]  // Others ignored
    }
}
```

**Recommendation:** Use functional options pattern or single `*Options` pointer.

---

### 9.3.4 Redundant edgeSeen Map (Low Priority)

**Location:** `state.go:87-108`

**Issue:** `edgeSeen` map is redundant since we already check `s.edges[key]`.

**Fix:** Remove `edgeSeen` and use `s.edges[key]` check directly.

---

### 9.3.5 Documentation Gaps (Medium Priority)

**Missing:**
- Package-level documentation with usage examples
- Algorithm complexity information
- Coordinate system explanation (top-left origin, Y increases downward)
- Limitations (no edge labels, no compound graphs)

---

## 9.4 Performance Optimizations

### 9.4.1 O(1) Edge Lookup in Graph (Medium Priority)

**Location:** `posit.go:183-190`

**Issue:** `HasEdge` is O(n) but could be O(1).

```go
func (g *Graph) HasEdge(from, to string) bool {
    for _, e := range g.edges {  // O(n) scan
        if e.from == from && e.to == to {
            return true
        }
    }
    return false
}
```

**Fix:** Add edge map to `Graph` struct.

---

### 9.4.2 Neighbors Allocation in Simplex (Low Priority)

**Location:** `simplex.go:73-81`

**Issue:** Allocates new slice on each `neighbors()` call in hot path.

**Fix:** Maintain adjacency lists in `spanningTree` or use callback pattern.

---

### 9.4.3 Position Lookup in Crossing Count (Low Priority)

**Location:** `position.go:288-314`

**Issue:** O(n) position lookup in `edgeCrossesSegment`.

**Fix:** Pass position maps as parameters.

---

## Implementation Priority

### Phase 9.1: Critical Fixes (Recommended First)

| Item | Type | Impact | Effort |
|------|------|--------|--------|
| 9.2.1 Smallest width alignment | Bug | Layout quality | ~20 lines |
| 9.2.2 Self-loops lost | Bug | Feature completeness | ~60 lines |
| 9.3.2 Edge key ambiguity | Quality | Correctness | ~10 lines |
| 9.2.4 Non-deterministic output | Bug | Reproducibility | ~20 lines |

**Estimated total:** ~110 lines

### Phase 9.2: Feature Parity

| Item | Type | Impact | Effort |
|------|------|--------|--------|
| 9.1.1 Edge label positioning | Feature | Feature parity | ~80 lines |
| 9.1.2 Greedy FAS | Feature | Layout quality | ~100 lines |
| 9.1.4 Tight-tree ranker | Feature | Performance option | ~20 lines |

**Estimated total:** ~200 lines

### Phase 9.3: Quality & Polish

| Item | Type | Impact | Effort |
|------|------|--------|--------|
| 9.3.1 Error handling | Quality | Robustness | ~50 lines |
| 9.3.5 Documentation | Quality | Usability | ~100 lines |
| 9.4.1 O(1) edge lookup | Performance | API performance | ~20 lines |

**Estimated total:** ~170 lines

### Phase 9.4: Advanced (Optional)

| Item | Type | Impact | Effort |
|------|------|--------|--------|
| 9.1.5 Type-2 conflicts | Feature | Compound graphs | ~50 lines |
| 9.1.6 Block graph compaction | Feature | Layout quality | ~80 lines |
| 9.1.7 Compound graphs | Feature | Feature parity | ~200+ lines |

---

## Testing Requirements

### New Tests for Bug Fixes

```go
func TestSmallestWidthAlignment(t *testing.T) {
    // Verify the alignment with smallest actual width is chosen
}

func TestSelfLoopsPreserved(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.MustAddEdge("A", "A")  // Self-loop

    layout := g.Layout()

    if _, ok := layout.Edges["A->A"]; !ok {
        t.Error("Self-loop should be in output")
    }
}

func TestDeterministicLayout(t *testing.T) {
    // Run layout 10 times on same graph
    // Verify all outputs are identical
}
```

### New Tests for Features

```go
func TestEdgeLabelPositioning(t *testing.T) {
    g := NewGraph()
    g.AddNode("A", NodeOptions{Width: 100, Height: 50})
    g.AddNode("B", NodeOptions{Width: 100, Height: 50})
    g.AddEdge("A", "B", EdgeOptions{
        LabelWidth: 40,
        LabelHeight: 20,
    })

    layout := g.Layout()
    edge := layout.Edges["A->B"]

    if edge.Label == nil {
        t.Error("Edge label should have coordinates")
    }
}

func TestGreedyFAS(t *testing.T) {
    g := buildWeightedCyclicGraph()

    layout := g.Layout(Options{Acyclicer: Greedy})

    // Verify fewer/lower-weight edges are reversed
}
```

---

## Summary

Phase 9 addresses the gaps between posit and dagre identified through comprehensive code review:

| Category | Items | Priority |
|----------|-------|----------|
| Bug Fixes | 5 issues | High |
| Missing Features | 7 features | Mixed |
| Code Quality | 5 issues | Medium |
| Performance | 3 optimizations | Low |

**Total estimated effort:** ~600-800 lines of code + tests

After Phase 9, posit will achieve **~98% feature parity** with dagre and improved code quality.

---

## References

1. Dagre source: `_refs/dagre/lib/`
2. Gansner, E.R., et al. "A Technique for Drawing Directed Graphs." IEEE TSE, 1993.
3. Brandes, U., Köpf, B. "Fast and Simple Horizontal Coordinate Assignment." GD 2001.

---

## Back to Overview

← [Phase 0: Overview](./PHASE_0_OVERVIEW.md)
