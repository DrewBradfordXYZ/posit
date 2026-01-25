# Constraint-Based Stacking Prevention

**Status: TODO** - Design complete, implementation pending.

## Goal

Replace the post-hoc nudging in `spread.go` with constraint-based separation that integrates directly into the Brandes-Köpf coordinate assignment algorithm.

## Two Approaches

We support two methods for preventing stacked nodes:

| Aspect | Method A: Single-Pass | Method B: Re-Run |
|--------|----------------------|------------------|
| When constraints applied | During first BK pass | After detecting actual stacking |
| Based on | Graph structure (edges) | Actual positions |
| Performance | Single pass (faster) | 2-3 passes (slower) |
| Precision | May over-separate | Precise, only fixes real stacking |
| Complexity | Simpler | More complex |

**Recommendation**: Try Method A first. Fall back to Method B if needed.

## Key Insight

The `separation()` function in `position.go:479-515` is called for every pair of adjacent nodes during horizontal compaction. It already handles NodeSep, dummy penalties, and cluster padding. Adding constraint checks here naturally integrates separation requirements into BK optimization.

---

# Method A: Single-Pass (Edge-Based)

**Principle**: If two same-layer nodes both have edges to/from a common node in another layer, they need extra separation to prevent stacking at that shared target.

## Why It Works

```
Layer 0:    [A]    [B]     ← A and B both connect to C
              \    /
               \  /
Layer 1:      [C]
```

If A and B are too close, their edges to C create crossing chaos. The fix: ensure A and B are separated by at least C's width, so one can be clearly left of C and one clearly right.

## Algorithm

```
separation(leftID, rightID):
  |
  |-- (1) Base separation (existing NodeSep, dummy, cluster logic)
  |
  |-- (2) Find shared cross-layer targets:
  |       - Nodes that both leftID and rightID connect to
  |       - In different layers (cross-layer edges)
  |
  |-- (3) For each shared target:
  |       - Increase separation to at least target.width + margin
  |       - This ensures leftID and rightID can be on opposite sides
  |
  |-- (4) Return max of base and constraint-based separation
```

## Implementation

```go
func (s *layoutState) separation(leftID, rightID string) float64 {
    left := s.nodes[leftID]
    right := s.nodes[rightID]

    // ... existing NodeSep, dummy, cluster logic ...

    // Single-pass stacking prevention
    if s.opts.SpreadStackedNodes {
        sharedTargets := s.findSharedCrossLayerTargets(leftID, rightID)
        for _, target := range sharedTargets {
            // Need enough separation so left and right can be on opposite sides of target
            minSep := target.width + s.opts.NodeSep
            if minSep > sep {
                sep = minSep
            }
        }
    }

    return left.width/2 + sep + right.width/2
}

// findSharedCrossLayerTargets returns nodes that both a and b connect to
// (as successors or predecessors) in different layers.
func (s *layoutState) findSharedCrossLayerTargets(a, b string) []*layoutNode {
    nodeA := s.nodes[a]
    nodeB := s.nodes[b]
    if nodeA == nil || nodeB == nil {
        return nil
    }

    var shared []*layoutNode

    // Check successors (nodes that a and b both point to)
    aSucc := make(map[string]bool)
    for _, succ := range s.successors[a] {
        aSucc[succ] = true
    }
    for _, succ := range s.successors[b] {
        if aSucc[succ] {
            target := s.nodes[succ]
            if target != nil && target.rank != nodeA.rank {
                shared = append(shared, target)
            }
        }
    }

    // Check predecessors (nodes that both point to a and b)
    aPred := make(map[string]bool)
    for _, pred := range s.predecessors[a] {
        aPred[pred] = true
    }
    for _, pred := range s.predecessors[b] {
        if aPred[pred] {
            source := s.nodes[pred]
            if source != nil && source.rank != nodeA.rank {
                shared = append(shared, source)
            }
        }
    }

    return shared
}
```

## What We Know During Single-Pass

| Information | Available? | How to Use |
|-------------|------------|------------|
| Shared target node | Yes | Use target.width for separation |
| Target's layer | Yes | Confirm it's cross-layer |
| Exact separation applied | Yes | `target.width + NodeSep` |
| Which pairs were separated | Yes (can log) | Track for debugging |

## Limitations

- May over-separate if nodes wouldn't actually stack (false positives)
- Doesn't consider actual final positions
- Only handles direct fan-in/fan-out patterns

---

# Method B: Re-Run (Position-Based)

**Principle**: Run BK once, detect actual stacking from positions, add constraints, re-run BK.

## Algorithm Flow

```
assignCoordinates() [MODIFIED]
  |
  |-- (1) Initial BK pass (existing)
  |
  |-- (2) Detect stacking (reuse spread.go detection)
  |       - Find pairs with centers within threshold
  |       - Include both short edges and dummy chains
  |
  |-- (3) If stacking found:
  |       - Create SeparationConstraint for each stacked pair
  |       - Re-run BK with constraints (separation() checks them)
  |       - Iterate up to 3 times until converged
  |
  |-- (4) Normalize X coordinates
```

## Data Structures (Method B)

```go
// In state.go - add to layoutState

type separationConstraint struct {
    leftID, rightID string   // Same-layer node IDs (ordered by layer position)
    minGap          float64  // Minimum gap between right edge of left and left edge of right
}

// Add field to layoutState:
separationConstraints []separationConstraint
```

## Implementation Steps (Method B)

### 1. Add constraint storage to layoutState (`state.go`)
- Add `separationConstraints []separationConstraint` field
- Initialize as empty slice in `newLayoutState()`

### 2. Modify separation() to check constraints (`position.go`)
```go
func (s *layoutState) separation(leftID, rightID string) float64 {
    // ... existing NodeSep, dummy, cluster logic ...

    // Method B: Check for explicit separation constraints
    for _, c := range s.separationConstraints {
        if (c.leftID == leftID && c.rightID == rightID) ||
           (c.leftID == rightID && c.rightID == leftID) {
            constraintSep := left.width/2 + c.minGap + right.width/2
            if constraintSep > sep {
                sep = constraintSep
            }
        }
    }

    return left.width/2 + sep + right.width/2
}
```

### 3. Implement constraint-based coordinate assignment (`position.go`)
- New function: `assignXCoordinatesWithConstraints()`
- Calls existing `assignXCoordinatesBK()`
- After first pass, detects stacking and creates constraints
- Re-runs BK with constraints (up to 3 iterations)

### 4. Integrate stacking detection (`spread.go` → `position.go`)
- Move/refactor detection logic from `nudgeAllStackedEdgePairs()`
- New function: `detectStackedPairs() []stackedPair`
- Checks both `s.edges` and `s.dummyChains` for long edges

### 5. Convert stacked pairs to same-layer constraints
- For cross-layer stacking, find same-layer neighbors in alignment blocks
- Create constraints between same-layer nodes that need separation

## Convergence Strategy (Method B)

1. Run initial BK coordinate assignment
2. Detect stacked pairs using existing threshold logic
3. For each stacked pair:
   - Determine which node should be left vs right (by layer order)
   - Create constraint: `minGap = threshold + margin`
4. Re-run BK with constraints
5. Check if all constraints satisfied
6. If not, iterate (max 3 times)
7. Break early if no new stacking detected

---

# Combining Both Methods

The methods can be used together for defense in depth:

```go
func (s *layoutState) separation(leftID, rightID string) float64 {
    // Base separation
    sep := s.opts.NodeSep
    // ... existing dummy, cluster logic ...

    // Method A: Proactive edge-based separation (single-pass)
    if s.opts.SpreadStackedNodes {
        for _, target := range s.findSharedCrossLayerTargets(leftID, rightID) {
            minSep := target.width + s.opts.NodeSep
            sep = max(sep, minSep)
        }
    }

    // Method B: Explicit constraints from re-run detection
    for _, c := range s.separationConstraints {
        if c.matches(leftID, rightID) {
            sep = max(sep, c.minGap)
        }
    }

    return left.width/2 + sep + right.width/2
}
```

**Strategy**:
1. Method A catches most cases in single pass
2. Method B (if enabled) catches remaining edge cases via re-run

---

# Configuration

## Options

```go
type Options struct {
    // ... existing ...

    // SpreadStackedNodes enables stacking prevention.
    // When true, nodes connected by edges are separated to prevent
    // edge crossing chaos.
    SpreadStackedNodes bool

    // StackingThreshold is the X-distance within which nodes are
    // considered "stacked". Default: 50% of average node width.
    StackingThreshold float64

    // MaxStackingIterations controls Method B re-run limit.
    // Default: 3. Set to 0 to disable Method B (single-pass only).
    MaxStackingIterations int
}
```

## Files to Modify

| File | Changes |
|------|---------|
| `position.go` | Add `findSharedCrossLayerTargets()`, modify `separation()` |
| `state.go` | Add `separationConstraints` field (for Method B) |
| `spread.go` | Refactor detection, simplify/remove nudging |
| `posit.go` | Add `MaxStackingIterations` option |

---

# Edge Cases

- **Long edges**: Use dummy chains to find original endpoints
- **Same-layer edges**: Direct constraint between the two nodes
- **Already separated**: No extra separation needed
- **No convergence**: After max iterations, accept current positions

## Verification

```bash
# All tests pass
go test -short ./...

# Contract invariants maintained
go test -run TestContract -v

# Existing spread tests still pass (behavior may differ but results valid)
go test -run TestSpread -v

# Performance acceptable
go test -bench=BenchmarkLayout -count=5
```

## Test Cases

1. **Fan-in**: Multiple sources → single target (stacked sources spread apart)
2. **Fan-out**: Single source → multiple targets (stacked targets spread apart)
3. **Long edge**: Stacking across multiple layers (constraint propagates)
4. **Diamond**: A→B, A→C, B→D, C→D (no false positives)
5. **Convergence**: Graph that requires 2 iterations to stabilize
6. **No stacking**: Non-stacked graph produces identical output

## Migration

1. Implement constraint system (no behavior change initially)
2. Wire in when `SpreadStackedNodes=true`
3. Keep nudging as fallback (deprecate later)
4. After validation, remove nudging code entirely

## References

- [SPREAD_STACKED_NODES_DESIGN.md](SPREAD_STACKED_NODES_DESIGN.md) - Current nudge-based implementation
- [STACKED_NODE_EDGE_CROSSING_PROBLEM.md](STACKED_NODE_EDGE_CROSSING_PROBLEM.md) - Problem statement
- `position.go:479-515` - The `separation()` function (injection point)
