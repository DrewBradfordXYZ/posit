# Plan: PortFixedOffset Constraint

## Problem

Schema graph visualizations need ports at exact Y positions (aligned with rendered field rows) but dynamic side selection (left/right based on connected node positions). The existing constraints don't cover this:

| Constraint | Side | Offset | Gap |
|---|---|---|---|
| `PortFixedPos` | Fixed | Fixed | Can't adapt side to layout |
| `PortFixedSide` | Fixed | Computed | Can't fix offset to CSS rows |
| `PortFixedOrder` | Fixed | Computed (ordered) | Can't fix offset to CSS rows |
| `PortFree` | Computed | Computed | Can't fix offset to CSS rows |
| **`PortFixedOffset`** | **Computed** | **Fixed** | **New** |

## Use Case

A table node has FK fields rendered at fixed Y positions (34 + idx*20 + 10). Each field needs a port that:
- Always connects at the exact pixel offset matching the CSS row height
- Appears on whichever side faces the connected node (left if peer is to the left, right if peer is to the right)

Current workaround: create BOTH left and right ports per field with `PortFixedPos`, then determine sides post-layout. This works but:
1. Doubles the port count (layout performance)
2. Side selection happens after layout, so Posit can't optimize node placement for the actual port sides
3. Edges are bound to one side's port during layout but may render on the other

## Context: Consumer Integration (datastar-flow)

datastar-flow is layout-agnostic — it receives node/edge positions via SSE and renders them with Datastar. The edge endpoint system uses a three-level priority:

1. **Server-computed offsets** (`data-source-ox`/`data-source-oy`, `data-target-ox`/`data-target-oy`) — highest priority, set from Posit's port layout output
2. **DOM handle elements** (`.source-handle`/`.target-handle`) — used when server offsets aren't available
3. **Node center fallback** — when neither handle nor offset exists

With `PortFixedPos` (current workaround), the server computes both left and right port positions, determines the correct side post-layout, and sends the winning offset via SSE. With `PortFixedOffset`, Posit handles side selection internally, and the server simply reads `PortLayout.Side` and `PortLayout.Offset` from the output — eliminating the duplicate-port workaround entirely.

The edge endpoint positions land on the **node border** at the exact port offset. Posit's `getPortPosition()` already computes this correctly for all port types — `PortFixedOffset` just needs to ensure the offset is preserved as-is while the side is computed.

## Design

### API

```go
const (
    PortFixedPos    PortConstraint = iota
    PortFixedSide
    PortFixedOrder
    PortFree
    PortFixedOffset  // New: fixed offset, algorithm chooses side
)
```

Usage:

```go
g.AddNode("users", posit.NodeOptions{
    Width: 200, Height: 100,
    Ports: []posit.PortOptions{
        {ID: "field-3", Offset: 44, Constraint: posit.PortFixedOffset, Axis: posit.PortAxisHorizontal},
        {ID: "field-18", Offset: 69, Constraint: posit.PortFixedOffset, Axis: posit.PortAxisHorizontal},
    },
})
```

- `Offset`: exact distance from node origin along the chosen side (preserved as-is)
- `Axis`: restricts side candidates (`PortAxisHorizontal` → Left/Right only)
- `Side`: ignored (Posit chooses), or used as initial hint for tie-breaking

### Output

`PortFixedOffset` ports appear in `NodeLayout.Ports` with their computed side:

```go
layout.Nodes["users"].Ports["field-3"] == PortLayout{
    ID:     "field-3",
    Side:   posit.Right,   // chosen by algorithm
    Offset: 44,            // preserved from input
}
```

### Edge Binding

Edges specify the port ID without a side prefix:

```go
g.MustAddEdge("orders", "users", posit.EdgeOptions{
    SourcePort: "field-3",   // Posit resolves to the computed side
    TargetPort: "field-18",
})
```

This is different from the current workaround where edges must specify "r-3" or "l-3".

## Implementation

The infrastructure for `PortFixedOffset` already exists from the `PortFree` implementation. The side selection algorithm (`assignFreeSides`, `bestSide`, `portConnectedDirection`) and axis constraints (`PortAxisHorizontal`, `PortAxisVertical`) are fully operational. `PortFixedOffset` is essentially `PortFree` minus the offset recomputation step.

### Phase 1: Add Constant (posit.go)

Add `PortFixedOffset` to the `PortConstraint` enum:

```go
const (
    PortFixedPos    PortConstraint = iota
    PortFixedSide
    PortFixedOrder
    PortFree
    PortFixedOffset  // New: fixed offset, algorithm chooses side
)
```

### Phase 2: Side Selection (port.go)

In `assignFreeSides`, include `PortFixedOffset` in the side computation loop:

```go
func (s *layoutState) assignFreeSides(node *layoutNode) {
    // ...
    for i := range node.ports {
        port := &node.ports[i]
        if port.Constraint != PortFree && port.Constraint != PortFixedOffset {
            continue
        }
        dx, dy := s.portConnectedDirection(node.id, port.ID, nodeCX, nodeCY)
        port.Side = s.bestSide(dx, dy, port.Axis)
    }
}
```

This reuses the exact same `bestSide()` + `portConnectedDirection()` logic already proven for `PortFree`.

### Phase 3: Offset Preservation (port.go)

In `computePortOffsets`, include `PortFixedOffset` in the `hasComputed` check but **skip** the offset distribution step. The key change is in `computeSideOffsets`:

```go
func (s *layoutState) computeSideOffsets(node *layoutNode, side Side, indices []int) {
    // Filter out PortFixedOffset — their offsets are preserved as-is
    var computeIndices []int
    for _, idx := range indices {
        if node.ports[idx].Constraint != PortFixedOffset {
            computeIndices = append(computeIndices, idx)
        }
    }
    // Evenly distribute only the non-fixed ports
    // ...
}
```

`PortFixedOffset` ports still participate in side grouping (so the layout knows they exist on a given side) but their offset values are never overwritten.

### Phase 4: Crossing Minimization (order.go)

In `preassignFreeSides` (the pre-coordinate phase that uses layer/order deltas), include `PortFixedOffset`:

```go
if port.Constraint != PortFree && port.Constraint != PortFixedOffset {
    continue
}
```

This gives crossing minimization awareness of the port's chosen side during node ordering.

### Phase 5: Layout Export (state.go)

In `buildLayout`, include `PortFixedOffset` ports in the output:

```go
if port.Constraint == PortFixedSide || port.Constraint == PortFixedOrder ||
   port.Constraint == PortFree || port.Constraint == PortFixedOffset {
    nodeLayout.Ports[port.ID] = PortLayout{
        ID:     port.ID,
        Side:   port.Side,    // computed
        Offset: port.Offset,  // preserved from input
    }
}
```

### Phase 6: Edge Port Resolution

When an edge references a `PortFixedOffset` port by ID, the routing phase resolves it to the computed side. This already works for `PortFree` ports — `PortFixedOffset` follows the same code path in `getPortPosition()` since it reads `port.Side` and `port.Offset` which are both populated.

## Files Changed

| File | Change | Complexity |
|------|--------|-----------|
| `posit.go` | Add `PortFixedOffset` constant | Trivial (one line) |
| `port.go` | Include in `assignFreeSides`, skip in `computeSideOffsets` | Small (2 conditionals) |
| `order.go` | Include in `preassignFreeSides` | Small (1 conditional) |
| `state.go` | Include in `buildLayout` port export | Trivial (1 conditional) |
| `features_test.go` | Add `TestPortFixedOffset_*` tests | Medium (6 test cases) |

Total estimated diff: ~50 lines of production code, ~150 lines of tests.

## Test Cases

1. **Basic side selection**: Node A (left) → Node B (right) with `PortFixedOffset` port. Port should be assigned to Right side.
2. **Offset preservation**: Declared offset 44 appears unchanged in output regardless of computed side.
3. **Axis constraint**: `PortAxisHorizontal` restricts to Left/Right only.
4. **Multiple ports same node**: Ports on same side maintain their declared offsets (no reordering).
5. **Schema pattern**: Hub node with 5+ FK fields connecting to nodes in various directions. Each port gets correct side based on connected node position.
6. **TopToBottom layout**: Vertically stacked nodes default to Left or Right (not Top/Bottom when axis is horizontal).

## Consumer Migration (basetypes)

After implementing in Posit, the basetypes graph code simplifies:

```go
// Before: duplicate ports + post-layout side determination
ports = append(ports, posit.PortOptions{
    ID: fmt.Sprintf("r-%d", f.ID), Side: posit.Right,
    Offset: yOffset, Constraint: posit.PortFixedPos,
})
ports = append(ports, posit.PortOptions{
    ID: fmt.Sprintf("l-%d", f.ID), Side: posit.Left,
    Offset: yOffset, Constraint: posit.PortFixedPos,
})

// After: single port, Posit chooses side
ports = append(ports, posit.PortOptions{
    ID:         fmt.Sprintf("port-%d", f.ID),
    Offset:     yOffset,
    Constraint: posit.PortFixedOffset,
    Axis:       posit.PortAxisHorizontal,
})
```

And `ComputePortPositions` simplifies to reading the computed side from `layout.Nodes[id].Ports[portID].Side` instead of computing it independently.

### datastar-flow SSE Integration

The SSE payload for edge endpoints changes from:

```go
// Before: server determines side, sends computed offset
if side == "right" {
    fmt.Fprintf(w, "data-source-ox=\"%f\" data-source-oy=\"%f\"", nodeWidth, yOffset)
} else {
    fmt.Fprintf(w, "data-source-ox=\"0\" data-source-oy=\"%f\"", yOffset)
}
```

To reading directly from Posit's output:

```go
// After: Posit determined the side, read from PortLayout
portLayout := layout.Nodes[nodeID].Ports[portID]
ox, oy := portLayout.Offset, 0.0
if portLayout.Side == posit.Right {
    ox = nodeWidth
}
oy = portLayout.Offset
fmt.Fprintf(w, "data-source-ox=\"%f\" data-source-oy=\"%f\"", ox, oy)
```

## Comparison with Other Layout Engines

### vs. ELK

ELK handles this with its port constraint system:
- `FIXED_POS`: equivalent to Posit's `PortFixedPos`
- `FIXED_SIDE`: equivalent to Posit's `PortFixedSide`
- `FIXED_ORDER`: equivalent to Posit's `PortFixedOrder`
- `FREE`: equivalent to Posit's `PortFree`

ELK has no direct `PortFixedOffset` equivalent — users achieve this with `FIXED_POS` and pre-computing the side (the same workaround basetypes currently uses). `PortFixedOffset` is a Posit-specific enhancement that eliminates this workaround.

### vs. dagre

dagre has no port support. Edge endpoints always land at node border intersection points.

### vs. msagljs

msagljs has `FloatingPort` and `RelativeFloatingPort` with complex constraint systems. The closest equivalent to `PortFixedOffset` is `RelativeFloatingPort` with a fixed offset parameter, but msagljs's port system is significantly more complex than what Posit needs.
