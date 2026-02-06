# Posit

Pure Go Sugiyama layout engine. Computes X/Y positions for nodes in directed graphs.

## Output Contract

Before modifying any output types (`Layout`, `NodeLayout`, `EdgeLayout`, `PortLayout`, `LabelLayout`), read `CONTRACT.md`. It specifies the invariants that consumers rely on.

After any change, run:

```bash
go test -run TestContract -v
```

These tests enforce the contract across 11 graph topologies (16 contract test functions). If they fail, the change violates a guarantee that downstream consumers depend on.

## Key Files

### Public API
- `posit.go` — Public API: Graph, Options, Layout types
- `CONTRACT.md` — Output invariants (what consumers can rely on)

### Pipeline Phases (in execution order)
- `acyclic.go` — Cycle detection and edge reversal
- `greedy_fas.go` — Greedy FAS algorithm for cycle removal (Eades-Lin-Smyth 1993)
- `rank.go` — Layer assignment (longest path)
- `simplex.go` — Network Simplex for Y ranking and X coordinates (Gansner et al. 1993)
- `normalize.go` — Dummy node insertion for long edges
- `order.go` — Crossing minimization (barycenter/median heuristics)
- `position.go` — Coordinate assignment (Brandes-Köpf default, or X simplex)
- `overlap.go` — Cross-layer overlap resolution
- `port.go` — Port offset computation
- `route.go` — Edge routing and spline generation

### Supporting
- `state.go` — Internal layout pipeline and `buildLayout()` (where output is constructed)
- `boundary.go` — Geometry helpers: ray-rectangle intersection, side inference
- `components.go` — Disconnected component layout
- `direction.go` — Layout direction (TB, BT, LR, RL)

### Tests
- `contract_test.go` — Property tests enforcing the contract
- `stress_test.go` — Large graph stress tests
- `benchmark_test.go` — Performance benchmarks (use `benchstat` to compare)
- `*_test.go` — Unit tests for each module

## Documentation

- `docs/ARCHITECTURE.md` — Detailed architecture and design decisions
- `_ref/` — Reference implementations (ELK, MSAGL, dagre)

## Testing

```bash
go test -short ./...          # Full suite (skips stress tests)
go test -run TestContract -v  # Contract invariants only
go test -run TestStress -v    # Stress tests only
go test -bench=. -count=5     # Performance (use benchstat to compare)
```

## Architecture

The layout pipeline runs in phases: cycle removal → ranking → dummy nodes → crossing minimization → coordinate assignment → cross-layer overlap resolution → port computation → edge routing → component packing. Each phase is a method on `layoutState` in `state.go`.

The algorithm is deterministic: same input always produces same output. There is no randomness.
