# Posit's Architectural Advantage: Constraint Vocabulary at the Information Boundary

## Core Insight

Layout engines traditionally offer constraints that assume the consumer has already made geometric decisions. Posit's constraint vocabulary is designed around the **information boundary** between the consumer and the layout engine:

- The consumer knows **domain facts** (field order, row heights, semantic groupings)
- The layout engine knows **geometry** (relative node positions, edge crossing counts, available space)

Posit's API lets consumers declare exactly what they know and explicitly delegate what they don't. This eliminates the workarounds that other layout engines force on consumers.

## The Problem with Traditional Constraint Models

### ELK / Browser-First Pattern

ELK's port constraint vocabulary:

| Constraint | Consumer must provide | Layout computes |
|---|---|---|
| `FIXED_POS` | Side + Offset | Nothing (fully specified) |
| `FIXED_SIDE` | Side | Offset |
| `FIXED_ORDER` | Side + Order | Offset |
| `FREE` | Nothing | Side + Offset |

The gap: there's no mode for "I know the offset (from CSS) but not the side (which depends on peer positions)." This forces consumers into workarounds:

1. **Duplicate ports**: Create both left and right ports at the same offset, pick the winner after layout
2. **Two-pass layout**: Run layout to learn positions, then re-run with correct sides
3. **Guess and accept**: Pre-compute sides from graph topology, accept that some will be wrong

All three are symptoms of a constraint vocabulary that doesn't match the consumer's actual knowledge state.

### Why This Matters Less in the Browser

When layout runs in the browser (elkjs), these workarounds are tolerable:
- Duplicate ports are just JSON objects in memory (no network cost)
- Two-pass layout is cheap (engine is already loaded, previous frame provides context)
- Guessing is often good enough (consumers can correct on next render frame)

The browser's ability to iterate cheaply masks the vocabulary gap.

### Why This Matters on the Server

When layout runs on the server and results are delivered via SSE/WebSocket:
- Every workaround adds latency or complexity
- Duplicate ports mean more data serialized and transmitted
- Two-pass layout means two full computations + two network deliveries
- Guessing means accepting incorrect results that can't be cheaply corrected

The server-first pattern exposes the vocabulary gap as a real performance and correctness problem.

## Posit's Constraint Vocabulary

Posit adds the missing constraint mode:

| Constraint | Consumer provides | Posit computes |
|---|---|---|
| `PortFixedPos` | Side + Offset | — |
| `PortFixedSide` | Side | Offset (crossing-optimized) |
| `PortFixedOrder` | Side + Order | Offset (evenly distributed) |
| `PortFree` | — | Side + Offset |
| **`PortFixedOffset`** | **Offset** | **Side** |

Each mode maps to a specific knowledge state:

| Consumer knows... | Consumer doesn't know... | Use |
|---|---|---|
| Everything | — | `PortFixedPos` |
| Which side | Where on that side | `PortFixedSide` / `PortFixedOrder` |
| Nothing | — | `PortFree` |
| The exact offset (from CSS/domain) | Which side (depends on geometry) | `PortFixedOffset` |

The vocabulary is **complete** — every combination of "known offset" and "known side" has a constraint mode. Consumers never need to fake knowledge they don't have.

## Downstream Consequences

### 1. Single-Pass Authoritative Results

Because the constraint vocabulary expresses the consumer's exact knowledge state, Posit produces correct results in a single pass. No iteration, no post-processing, no "fix-up" phase on the client.

```
Consumer → declares domain constraints → Posit → authoritative layout → SSE → Client renders
```

vs. the traditional pattern:

```
Consumer → guesses geometric constraints → ELK → partial result → Consumer fixes → re-renders
```

### 2. Thinner Client Layer

The client becomes a pure rendering layer. It receives positions and renders them. No layout logic, no side determination, no port position computation. This means:

- **Framework-agnostic**: Any rendering stack (Datastar, React, Svelte, Canvas, WebGL) works with zero layout logic
- **Simpler client code**: No `ComputePortPositions()` function, no side-selection heuristics
- **Fewer bugs**: Layout decisions are made once, in one place, with full information

### 3. Server as Source of Truth

The layout result is authoritative. When a user drags a node and releases:

1. Client sends new position to server
2. Server runs `IncrementalLayout` (sub-millisecond for schema graphs)
3. Server pushes updated positions via SSE
4. Client renders

Port sides update correctly because Posit recomputes them with the new geometry. The client never needs to independently determine if a port should flip from left to right.

### 4. Better Layout Quality

Because Posit makes geometric decisions with full information (all node positions, all edge connections), it produces better results than a consumer guessing before layout:

- Port sides are based on actual computed positions, not topology guesses
- Crossing minimization accounts for the actual port placement
- Edge routing knows where ports actually are, not where the consumer hoped they'd be

### 5. Deterministic and Cacheable

Same input always produces same output. Since the consumer doesn't inject geometric guesses that might vary between runs, the layout is stable:

- Same graph structure + same domain constraints = same layout
- Results can be cached by graph hash
- No "jitter" from different geometric guesses on different runs

## Competitive Positioning

### vs. ELK (elkjs)

ELK is a comprehensive layout engine designed for the browser. Its constraint vocabulary is powerful but assumes the consumer can iterate cheaply. For server-rendered applications:

- ELK requires workarounds that add latency and complexity
- Posit's vocabulary eliminates those workarounds
- Posit produces authoritative results in a single server computation

### vs. dagre

dagre has no port support at all. Edge endpoints are computed via boundary intersection. For schema diagrams with field-level connections, dagre is not viable.

### vs. msagljs

msagljs (Microsoft's layout engine) has `FloatingPort` and `RelativeFloatingPort` — complex systems that offer flexibility but require significant consumer-side configuration. Posit's vocabulary is simpler: declare what you know, delegate what you don't.

### The Advantage

Posit is the only layout engine where the constraint vocabulary is designed for the **server-first, single-pass, authoritative-result** pattern. This isn't a minor API difference — it's an architectural choice that eliminates an entire class of workarounds and produces a cleaner separation of concerns.

## Future Opportunities

The information-boundary principle extends beyond ports:

### Edge Constraints at the Boundary

The same pattern could apply to edge routing preferences:

- "I know this edge should be short" (weight) ✅ already exists
- "I know this edge should avoid node X" (obstacle constraint) — consumer knows semantics, Posit knows geometry
- "I know these edges are related" (bundle constraint) — consumer knows grouping, Posit knows routing

### Layout Hints

Consumers may know soft preferences that shouldn't be hard constraints:

- "Node A is probably near Node B" (proximity hint for incremental layout)
- "This subgraph is more important" (emphasis hint for spacing)
- "The user last saw this arrangement" (stability hint from prior session)

Each of these follows the pattern: consumer provides domain knowledge, Posit makes the geometric decision.

### Multi-Client Rendering

Since the server owns layout state, multiple clients can render the same graph simultaneously with different rendering approaches (SVG, Canvas, ASCII art) without each needing layout logic. The layout is computed once and shared.

### Layout as a Service

The single-pass, deterministic, cacheable nature of Posit's computation makes it viable as a stateless service:

```
POST /layout { graph, constraints } → { positions }
```

The constraint vocabulary ensures the request contains exactly what the consumer knows, and the response contains everything needed to render — no back-and-forth negotiation about geometric decisions.
