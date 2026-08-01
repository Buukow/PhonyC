# Capture Request Metadata Logging

## Goal

Make requests handled by the header-capture key visible in the PhonyG management
console's request log while preserving capture-only behavior: the request is
recorded locally and is never routed to an upstream channel.

## Design

`Handler.handleCaptureOnly` will call the existing `logMeta` helper after the
capture attempt and before returning the protocol-shaped success response.
The metadata will use:

- HTTP status `200` for both an armed capture and an already-captured/not-armed
  response, because the client-facing response remains successful.
- `client_model` from the request body, falling back to `phonyg-capture`.
- Empty `upstream_model` and `channel_id = NULL`, because no upstream request
  is made.
- Empty `error_summary`, so successful capture-only requests do not count
  toward error-rate aggregates or appear in the recent-errors list.
- `impersonation_mode = "passthrough"` for the synthetic capture key.
- Zero token counts and the measured local request duration.
- The authenticated capture key is not associated with a user-key ID, so the
  log does not expose or create ordinary key statistics.

No database schema changes or new API endpoints are needed. Existing request
log list/detail rendering will display the record through `request_meta`.

## Error Handling

Logging is best-effort, matching all existing `logMeta` call sites. A database
write failure must not change the capture response or cause upstream routing.

## Verification

Add a handler test that enables and arms capture, sends a capture-key request,
then polls the test database until one `request_meta` row appears with status
200, `capture-only`, the expected path/model, and a nil channel/key ID.
