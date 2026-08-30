# Glyph Protocol V1.1.1

A security-critical, zero-dependency Go implementation of a spatial glyph protocol for LLM intent communication. The model may propose glyph triplets, but the runtime independently authorizes every effect via immutable session-owned slots and one-time user approval.

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
