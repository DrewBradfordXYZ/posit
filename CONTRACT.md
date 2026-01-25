# Output Contract

This document specifies the invariants that `Layout()` guarantees. These invariants are enforced by `TestContract_*` tests in `contract_test.go`. Any change to posit that violates these guarantees will fail the build.

Consumers of posit's output (serialization layers, renderers, UI frameworks) can rely on these guarantees without reading implementation code.

## Coordinate System

- Origin is top-left. X increases rightward, Y increases downward.
- `NodeLayout.Position` (X, Y) is the **top-left corner** of the node rectangle.
- Node center is at `(X + Width/2, Y + Height/2)`.
- All coordinates are finite (never NaN or Inf).
- All coordinates are non-negative for the default TopToBottom direction.

## Node Guarantees

- Every node ID passed to `AddNode()` appears as a key in `Layout.Nodes`.
- No extra keys appear — `Layout.Nodes` keys are exactly the input node IDs.
- `NodeLayout.Width` and `NodeLayout.Height` match the input `NodeOptions.Width` and `NodeOptions.Height` (the algorithm positions nodes, it does not resize them).
- Exception: cluster nodes (`IsCluster: true`) are resized to contain their children.
- No two non-cluster nodes overlap (their bounding rectangles do not intersect).

## Edge Guarantees

- Every edge added via `AddEdge()` appears in `Layout.Edges`.
- `EdgeLayout.From` and `EdgeLayout.To` match the source and target node IDs from input.
- `EdgeLayout.Points` has at least 2 entries (start point, end point).
- The first point is on or near the source node boundary.
- The last point is on or near the target node boundary.
- All edge points have finite coordinates.
- If `SourcePort` was specified on input, `EdgeLayout.SourcePort` echoes it back.
- If `TargetPort` was specified on input, `EdgeLayout.TargetPort` echoes it back.

### Edge Attachment Sides

- `EdgeLayout.SourceSide` and `EdgeLayout.TargetSide` are always populated.
- Valid values are `Top`, `Bottom`, `Left`, `Right` (the `Side` type).
- Sides are computed using geometric boundary intersection: where a ray from the node center toward the connected node exits the node boundary.
- For typical hierarchical layouts, opposite sides face each other (e.g., source `Bottom` → target `Top` in TopToBottom direction).
- For diagonal arrangements, the side depends on which boundary edge the connecting ray exits first.
- Side values are in user coordinate space (after direction transformation), matching how they would be rendered.

## Port Guarantees

- For `PortFixedPos`: the port offset used in routing equals the input offset. No `PortLayout` entry is emitted (the consumer already knows the position).
- For `PortFixedSide`, `PortFixedOrder`, `PortFree`, `PortFixedOffset`: `NodeLayout.Ports` is populated with computed positions.
- Port offsets are within `[0, sideLength]` where sideLength is `Width` (for Top/Bottom) or `Height` (for Left/Right).
- For `PortFixedOrder`: ports on the same side appear in the order specified by their `Order` field.

## Layer Guarantees (TopToBottom direction)

- For any edge A→B where A and B are in different layers: `A.Y < B.Y` (source is above target). This holds after cycle removal — some input edges may be reversed internally, but the output positions reflect the DAG layering.
- Nodes on the same layer share the same Y coordinate.
- Layer spacing is at least `RankSep` (default: 100).

## Determinism

- The same graph (same nodes, same edges, same options) always produces the exact same Layout. There is no randomness in the algorithm.
- Node insertion order affects layout (earlier nodes are placed first in initial ordering). This is deterministic given the same `AddNode()` call sequence.

## Edge Keys

- For edges without an explicit ID: the key is `"from->to"`.
- For edges with an explicit ID: the key is `"from->to:id"`.
- For self-loops: the key follows the same pattern (`"A->A"` or `"A->A:id"`).
- Multi-edges between the same pair use distinct IDs to produce distinct keys.

## Labels

- If an edge has `LabelWidth > 0` and `LabelHeight > 0`, `EdgeLayout.Label` is non-nil.
- The label position is between the source and target nodes.
- `Label.Width` and `Label.Height` match the input dimensions.

## What This Contract Does NOT Guarantee

- **Specific coordinates**: The exact X/Y values may change between versions as the algorithm improves. Do not hardcode expected positions.
- **Crossing count**: The algorithm minimizes crossings heuristically. The number of crossings may change between versions.
- **Edge path shape**: The intermediate points of edge paths may change. Only the endpoints (attachment to nodes) are stable.
- **Performance**: Timing is not part of the contract. It's measured separately via `go test -bench`.
