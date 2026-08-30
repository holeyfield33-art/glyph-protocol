# Security Model — Glyph Protocol V1.1.1

## Non-claims

- This does **not** solve prompt injection.
- This does **not** prove user intent beyond the approval step.
- This does **not** execute arbitrary programs.
- This does **not** support files, network, shell, or databases.
- The UI has zero security authority; only the core engine decides.

## Threat Model

**Attacker may:**
- Control LLM output (malicious glyph sequences).
- Craft malformed or adversarial JSON.
- Attempt concurrent approval / slot races.
- Replay old approvals or slots.
- Flood the rate limiter.

**Attacker cannot:**
- Allocate or steal slots belonging to another session.
- Grant themselves permissions.
- Bypass the confirmation gate for writes.
- Execute unknown commands (compiled allowlist only).
- Cause the parser to accept duplicate keys, unknown fields, or out-of-range values.
- Mutate staged content after the hash is recorded.

## Hardening Measures

1. **Strict JSON parser** — token-level walk, rejects duplicates, unknown keys, non-integers, trailing data.
2. **Visual glyph allowlist** — only unambiguous base/rotation/marker combinations accepted.
3. **Fixed instruction length** — exactly 3 glyphs per command; length must be divisible by 3.
4. **Compiled command table** — no string matching, no regex, no dynamic dispatch beyond a switch.
5. **Session-bound slots** — ownership checked on every use; content is copied and hashed.
6. **Version + hash binding** — approvals bind to exact slot version and SHA-256.
7. **Atomic consume** — approval and “slot used” flag are set under lock; concurrent use fails.
8. **TTL** — slots and approvals expire after 60 seconds.
9. **Rate limit** — 10 requests per 60 s per session.
10. **Zero external dependencies** — only Go standard library.

## Dependencies

- Only Go standard library.
- No regex, unsafe, plugins, cgo, or dynamic code loading.

## Audit

Every request produces a receipt containing:
- Hashed session ID
- Sequence hash
- Action, slot version, content hash (when applicable)
- Outcome code and reason

Receipts are intended for external append-only logging (not implemented in the demo binary).
