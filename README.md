# Glyph Protocol V1.1.1

A security-critical, zero-dependency Go implementation of a spatial glyph protocol for LLM intent communication. The model may propose glyph triplets, but the runtime independently authorizes every effect via immutable session-owned slots and one-time user approval.

## Vision & Research Direction

### Original Intent

The original vision was a **spatial alphabet as a high-integrity intent channel**: the model would be forced to speak in a constrained visual/structural language whose valid utterances are sparse, precise, and difficult to sample accidentally or via injection. Natural language would never be the authority. The core engine would be a deterministic interpreter of pure spatial tokens. Human confirmation would be secondary or absent for the core claim.

In short: make the *channel itself* hard to abuse, rather than relying primarily on post-hoc filters or human gates.

### What V1.1.1 Actually Is

V1.1.1 is a well-engineered **constrained command language + human approval gate**:

- Strict JSON of integer sequences
- Visual glyph allowlist and fixed 3-glyph instructions
- Compiled command table
- Session-bound, versioned, TTL-protected slots
- Atomic TOCTOU-resistant approval for writes

This is useful engineering and raises the cost of many classes of misuse. It is **not** yet a fundamentally new trust model. It is closer to a hardened version of “tool use with human-in-the-loop confirmation.” The spatial layer currently functions more as an encoding detail and visual allowlist than as the primary integrity mechanism.

### Relationship to Aegis Provenance

[Aegis Provenance](https://github.com/holeyfield33-art/aegis-provenance) is a complementary system from the same author. Aegis is a provenance-enforcing context proxy and egress gate: it wraps inputs in signed spans, derives trust from origin, and blocks sensitive tool calls that cannot be attributed to trusted user intent.

Aegis is currently the stronger, more mature, and more rigorously tested system for the job of “stop unauthorized tool use.” Glyph should not compete with it as a primary enforcement layer.

### Forward Plan

1. **Treat Aegis as the production-grade runtime enforcement layer.** Continue hardening it.

2. **Reposition Glyph as an experimental high-integrity *proposal* channel** that can feed into or sit alongside Aegis.
   - Sparsify the valid glyph space dramatically (longer sequences, geometric constraints, low-density valid utterances).
   - Make successful emission of a valid high-entropy spatial sequence a strong additional signal of user-session intent.
   - Measure, under adversarial prompting, whether the spatial constraint meaningfully raises the cost of unauthorized sensitive actions compared to ordinary free-form tool calls.

3. **Kill the near-term claim** that a spatial alphabet alone constitutes a high-integrity channel against capable models. The value to test is whether a deliberately sparse spatial proposal language can act as a useful *amplifier* of a strong provenance/egress system, not as a replacement for it.

4. **Success metric for the research direction**: measurable reduction in successful sensitive-tool proposals under adversarial conditions when a verified spatial key is required, evaluated against the same attack corpora used by Aegis.

This repository remains the clean, zero-dependency foundation for that experiment.

## Security Claims

- Accepts only strictly defined JSON protocol.
- Compiled command allowlist.
- No arbitrary code, shell, network, or file access.
- All writes require explicit user confirmation.
- Strict parser rejects duplicate keys, unknown fields, out-of-range glyphs, and malformed structure.
- Session-bound, versioned, TTL-protected slots.
- Atomic approval + slot consumption (TOCTOU resistant).
- Rate limiting (10 requests / 60s per session).
- Audit receipts for every outcome.

## Architecture

```
[LLM / Client]
      │
      ▼
 Strict JSON Parser  →  Glyph Visual Validation  →  Command Lookup
      │
      ▼
 Session + Slot Check  →  Permission Bitmask  →  Action Dispatch
      │                                              │
      │                                    (read-only or CONFIRMATION_REQUIRED)
      ▼
 Atomic Approval (for SAVE_DRAFT) → Draft Store
```

## Glyph Encoding

Each glyph is a 7-bit value:

| Field     | Bits | Values                                      |
|-----------|------|---------------------------------------------|
| Base      | 2    | Triangle, Chevron, Square, Diamond          |
| Rotation  | 2    | Up, Right, Down, Left                       |
| Marker    | 3    | None, Center, Top, Right, Bottom, Left      |

Square and Diamond only allow RotationUp. Markers 6–7 are invalid.

## Commands (V1)

Instructions are fixed 3-glyph triplets: `[verb, object, modifier]`.

| Verb | Action          | Requires Slot | Requires Confirmation | Permission   |
|------|-----------------|---------------|-----------------------|--------------|
| 10   | DISPLAY_SLOT    | yes           | no                    | PermView     |
| 20   | CLASSIFY_SLOT   | yes           | no                    | PermView     |
| 30   | SUMMARIZE_SLOT  | yes           | no                    | PermView     |
| 40   | SAVE_DRAFT      | yes           | yes                   | PermWriteDraft |

Object glyph 0–7 selects the slot index.

## Build & Run

```bash
go test ./...
go run ./cmd/server
```

Server listens on `:8080`. Open http://localhost:8080 for the demo UI.

## API

### POST /v1/session
Creates a new session. Returns `{ "session_id": "..." }`.

### POST /v1/stage
Headers: `Authorization: Bearer <session_id>`
Body: `{ "slot": 0, "text": "..." }`
Stages user text into a slot (max 500 bytes, TTL 60s).

### POST /v1/propose
Headers: `Authorization: Bearer <session_id>`
Body: `{ "v": 1, "seq": [10, 3, 0] }`
Proposes a glyph sequence. Returns SUCCESS with action results or CONFIRMATION_REQUIRED for SAVE_DRAFT.

### POST /v1/approve
Headers: `Authorization: Bearer <session_id>`
Body: `{ "approval_id": "...", "decision": "approve" | "deny" }`

### GET /v1/drafts
Headers: `Authorization: Bearer <session_id>`
Returns saved drafts.

## Project Layout

```
glyph-protocol/
├── cmd/server/main.go
├── internal/
│   ├── protocol/   # glyph encoding, strict parser, command table
│   ├── session/    # session + immutable slots + rate limit
│   ├── approval/   # pending approvals with TOCTOU bindings
│   ├── engine/     # orchestration
│   ├── audit/      # receipts
│   └── ui/static/  # minimal demo UI
├── go.mod
├── README.md
└── SECURITY.md
```

## License

MIT
