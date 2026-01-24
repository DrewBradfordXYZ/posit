# Posit

Pure Go Sugiyama layout engine. Computes X/Y positions for nodes in directed graphs.

## Output Contract

Before modifying any output types (`Layout`, `NodeLayout`, `EdgeLayout`, `PortLayout`, `LabelLayout`), read `CONTRACT.md`. It specifies the invariants that consumers rely on.

After any change, run:

```bash
go test -run TestContract -v
```

These tests enforce the contract across 11 graph topologies. If they fail, the change violates a guarantee that downstream consumers depend on.

## Key Files

- `posit.go` — Public API: Graph, Options, Layout types
- `CONTRACT.md` — Output invariants (what consumers can rely on)
- `contract_test.go` — Property tests enforcing the contract
- `state.go` — Internal layout pipeline and `buildLayout()` (where output is constructed)
- `order.go` — Crossing minimization
- `rank.go` — Layer assignment
- `position.go` — Coordinate assignment (Brandes-Kopf)
- `route.go` — Edge routing
- `port.go` — Port offset computation

## Testing

```bash
go test -short ./...          # Full suite (skips stress tests)
go test -run TestContract -v  # Contract invariants only
go test -bench=. -count=5     # Performance (use benchstat to compare)
```

## Architecture

The layout pipeline runs in phases: cycle removal → ranking → dummy nodes → crossing minimization → coordinate assignment → port computation → edge routing. Each phase is a method on `layoutState` in `state.go`.

The algorithm is deterministic: same input always produces same output. There is no randomness.
