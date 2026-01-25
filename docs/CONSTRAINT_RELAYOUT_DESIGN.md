# Constraint-Based Re-Layout for Stacked Nodes

**Status: TODO** - Design complete, implementation pending.

## Goal

Replace the post-hoc nudging in `spread.go` with constraint-based re-layout that feeds separation requirements back into the Brandes-Köpf coordinate assignment algorithm.

## Why This Is Better

| Aspect | Current (Nudge) | Proposed (Constraints) |
|--------|-----------------|------------------------|
| Uses Posit's optimization | No | Yes |
| Considers full graph | No | Yes |
| Margin calculation | Fixed 15px | Algorithm-determined |
| May create new conflicts | Yes | No (holistic) |
| Integration | Post-hoc hack | Native BK integration |

## Key Insight

The `separation()` function in `position.go:479-515` is called for every pair of adjacent nodes during horizontal compaction. It already handles NodeSep, dummy penalties, and cluster padding. Adding constraint checks here naturally integrates separation requirements into BK optimization.

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

## Data Structures

```go
// In state.go - add to layoutState

type separationConstraint struct {
    leftID, rightID string   // Same-layer node IDs (ordered by layer position)
    minGap          float64  // Minimum gap between right edge of left and left edge of right
}

// Add field to layoutState:
separationConstraints []separationConstraint
```

## Implementation Steps

### 1. Add constraint storage to layoutState (`state.go`)
- Add `separationConstraints []separationConstraint` field
- Initialize as empty slice in `newLayoutState()`

### 2. Modify separation() to check constraints (`position.go`)
```go
func (s *layoutState) separation(leftID, rightID string) float64 {
    // ... existing NodeSep, dummy, cluster logic ...

    // NEW: Check for separation constraints
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

### 6. Update pipeline (`posit.go`)
- When `SpreadStackedNodes=true`, use constraint-based path
- Remove/simplify `spreadStackedNodes()` phase (becomes no-op)

### 7. Add MaxStackingIterations option (`posit.go`)
```go
type Options struct {
    // ... existing ...
    MaxStackingIterations int // Default: 3
}
```

## Files to Modify

| File | Changes |
|------|---------|
| `state.go` | Add `separationConstraints` field to `layoutState` |
| `position.go` | Modify `separation()`, add `assignXCoordinatesWithConstraints()` |
| `spread.go` | Refactor detection into reusable function, simplify/remove nudging |
| `posit.go` | Add `MaxStackingIterations` option, update pipeline |

## Convergence Strategy

1. Run initial BK coordinate assignment
2. Detect stacked pairs using existing threshold logic
3. For each stacked pair:
   - Determine which node should be left vs right (by layer order)
   - Create constraint: `minGap = threshold + margin`
4. Re-run BK with constraints
5. Check if all constraints satisfied
6. If not, iterate (max 3 times)
7. Break early if no new stacking detected

## Edge Cases

- **Long edges**: Use dummy chains to find original endpoints, then find same-layer representatives
- **Same-layer edges**: Direct constraint between the two nodes
- **Already separated**: No constraint needed if `|centerA - centerB| > threshold`
- **No convergence**: After 3 iterations, accept current positions (rare)

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
