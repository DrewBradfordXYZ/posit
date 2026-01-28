# Design

## Server-Side Layout

Posit is designed for server-side layout, where the server owns application state and pushes UI updates to clients.

Layout is computed once on the server and shared to all clients. This enables multiplayer (all clients see the same layout) and smaller browser bundles (no layout library shipped to the client).

The application provides node dimensions and layout constraints to posit, shifting computation to the server and improving client performance.

The client only shows confirmed state from the server, displaying optimistic updates with pending styles.

## Constraint Vocabulary

Posit's constraint vocabulary lets callers express exactly what they know and delegate what they don't.

The application and the layout algorithm have different knowledge:

- **Application logic** knows domain facts: field order, row heights from CSS rules, semantic groupings
- **Layout algorithm** knows geometry: computed node positions, edge crossing counts, available space

### Port Constraints

Posit provides five port constraints, each mapping to a specific knowledge state:

| Constraint | Consumer provides | Posit computes |
|---|---|---|
| `PortFixedPos` | Side + Offset | — |
| `PortFixedSide` | Side | Offset (crossing-optimized) |
| `PortFixedOrder` | Side + Order | Offset (evenly distributed) |
| `PortFree` | — | Side + Offset |
| `PortFixedOffset` | Offset | Side |

Each mode corresponds to what the consumer actually knows:

| Consumer knows... | Consumer doesn't know... | Use |
|---|---|---|
| Everything | — | `PortFixedPos` |
| Which side | Where on that side | `PortFixedSide` / `PortFixedOrder` |
| Nothing | — | `PortFree` |
| The exact offset (from CSS/domain) | Which side (depends on geometry) | `PortFixedOffset` |

### PortFixedOffset for Schema Diagrams

`PortFixedOffset` addresses schema diagrams where table nodes have field rows at fixed Y positions:

```go
Node{
    ID: "users",
    Ports: []Port{
        {ID: "fk-orders", Offset: 34, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
        {ID: "fk-profiles", Offset: 54, Constraint: PortFixedOffset, Axis: PortAxisHorizontal},
    },
}
```

The consumer declares the offset (row position), and Posit determines the optimal side based on computed node positions.

## Design Consequences

### Single-Pass Results

Because the constraint vocabulary expresses the consumer's exact knowledge state, Posit produces correct results in a single pass. The consumer doesn't need to:

- Run layout twice (once to learn positions, once to specify them)
- Create duplicate entities for both possible sides
- Post-process results to fix geometric decisions

### Determinism and Cacheability

Same input always produces same output. Since the consumer doesn't inject geometric guesses that might vary:

- Same graph structure + same domain constraints = same layout
- Results can be cached by graph hash
- No layout jitter from inconsistent guesses

### Avoiding DOM Measurement

In schema diagrams, each table node contains field rows rendered by HTML/CSS. The port for each field attaches at that row's Y position. Without server-computed offsets, the client would need to:

1. Query the DOM to find where each field row actually rendered (`getBoundingClientRect`)
2. Trigger layout reflow if the DOM has pending changes
3. Repeat for every port on every node

With `PortFixedOffset`, the consumer declares the offset they already know from their CSS/layout rules. The server computes the side and echoes the offset back. The client renders with simple addition:

```js
portX = nodeLeft + serverOX
portY = nodeTop + serverOY
```

No DOM queries. No layout reflow.

## Future Extensions

### Edge Constraints

- "I know this edge should be short" → weight (already exists)
- "I know this edge should avoid node X" → obstacle constraint
- "I know these edges are related" → bundle constraint

### Layout Hints (Soft Preferences)

- "Node A is probably near Node B" → proximity hint for incremental layout
- "This subgraph is more important" → emphasis hint for spacing
- "The user last saw this arrangement" → stability hint

### Multi-Client Rendering

Since the server owns layout state, multiple clients can render the same graph with different approaches (SVG, Canvas, ASCII) without duplicating layout logic.

### Layout as a Service

The single-pass, deterministic nature enables a stateless service pattern:

```
POST /layout { graph, constraints } → { positions }
```

The constraint vocabulary ensures the request contains exactly what the consumer knows, and the response contains everything needed to render.
