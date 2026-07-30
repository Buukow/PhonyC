# Enhanced Healthcheck Design

## Settings and Scope

Add `auto_test_enhanced_enabled` and `auto_test_enhanced_lexicon` settings. The default enhanced setting is disabled. When disabled, every healthcheck keeps the existing fixed prompt and one non-streaming request.

When enabled, every healthcheck call path must use one shared prompt generator and one shared stream-first executor. This includes scheduled automatic checks, the settings-page “立即测活一轮” batch check, and individual channel checks; future healthcheck entry points must call the same executor. A new prompt is generated independently for every channel attempt.

The lexicon JSON contains only `prefix`, `modifier`, `modal_words`, `short_rules`, and `targets` string arrays. `prefix`, `modal_words`, `short_rules`, and `targets` must be non-empty arrays. `modifier` must exist and may contain empty strings. Unknown fields are rejected so misspellings do not silently change behavior. Empty strings are allowed only in `modifier` and `modal_words`. Saving invalid JSON is rejected and does not replace the last valid setting. Runtime parsing failure falls back to the compiled default lexicon and records a warning.

The compiled default lexicon is the single canonical JSON value and contains exactly the user-provided arrays from the approved request, excluding `punctuation`. It is exposed by the backend settings/default response. On first load the editor displays this formatted canonical JSON; saved edits reload unchanged after successful validation; “恢复默认词库” restores the same canonical value.

## Prompt Generation

For each prompt, randomly select one of these templates with equal probability:

1. `{prefix}{modifier}{target}`
2. `{modifier}{target}{prefix}`

Choose one value independently from each relevant array. Treat each non-empty prefix, modifier, target, and optional short rule as one segment.

- Include one random short rule with 40% probability. If included, place it at the beginning or end with equal probability.
- For each non-empty segment, independently append one randomly selected modal word with 30% probability. The selected modal word may itself be empty.
- Between every adjacent segment, independently insert `，` with 60% probability; otherwise concatenate directly.
- Append `。` with 30% probability; otherwise append no terminal punctuation.

The lexicon has no `punctuation` field. Randomness is injected behind a small interface so tests can force every probability boundary and selection.

## Stream-first Healthcheck

Enhanced checks attempt `stream: true` first. A streaming attempt succeeds only when its HTTP status is 200–399 and the response yields at least one recognizable item containing actual non-empty response content. SSE termination frames such as `[DONE]`, comments/heartbeats, blank data frames, and parsed events containing only empty deltas do not count. For non-SSE streaming content types, at least one non-whitespace response chunk that is not a known terminal marker counts. A 2xx/3xx body containing only control frames is a streaming failure.

Network errors, timeout, HTTP 4xx/5xx, empty streams, and unrecognizable empty content trigger one `stream: false` fallback request. The fallback request uses the identical protocol, path, model, prompt/messages, model parameters, authentication, extra headers, and channel context; only the `stream` field changes from `true` to `false`. The fallback result is authoritative for health state, temporary disable/recovery, stored status, latency, and error. If streaming succeeds, its result is authoritative and no fallback is sent.

If streaming fails but fallback succeeds, the channel is healthy and is not temporarily disabled. If both fail, only the fallback status/error is evaluated against configured temporary-disable codes. Latency records total elapsed time across both attempts when fallback occurs. Healthcheck metadata error/detail records that a stream fallback occurred without changing existing public API response compatibility; `ChannelResult` adds a boolean `stream_fallback` field.

Stream-first behavior follows existing state-transition rules exactly. Scheduled automatic checks and the settings-page batch “立即测活一轮” use automatic semantics: enabled channels are temporarily disabled when the authoritative final status is configured for disablement, and temporary-disabled channels recover on authoritative final success. Individual channel checks use manual semantics: manually disabled and normally enabled channels only record the authoritative final result and never change state; an already temporary-disabled channel recovers only on authoritative final success.

## Settings UI

Add an “自动测活增强” checkbox below the main auto-healthcheck switch. When enabled, expand a configuration area containing:

- an explanatory summary of the two templates and probabilities;
- a formatted JSON textarea for the lexicon;
- inline JSON/schema validation errors;
- “恢复默认词库” and “随机预览” buttons;
- a preview result that uses the same backend generator through a preview API, preventing frontend/backend algorithm drift.

The fixed prompt input remains visible but is marked as used only when enhanced mode is disabled.

## API and Validation

The settings update endpoint validates `auto_test_enhanced_lexicon` before writing any submitted settings, making the update atomic with respect to validation. Add an authenticated preview endpoint that accepts the current unsaved lexicon JSON and returns a generated prompt or a validation error.

## Testing

- Generator tests force both templates, short-rule omission/inclusion and placement, modal suffixes, comma joins, terminal period, empty modifier, and default fallback.
- Validation tests cover malformed JSON, missing/unknown fields, empty required arrays, invalid values, and valid defaults.
- Healthcheck tests cover streaming success, network/status/empty-stream fallback, fallback success, both-attempt failure, total latency semantics, final-state disable/recovery behavior, and global manual-check use.
- Admin API tests cover atomic settings validation and preview.
- Frontend tests cover canonical initial JSON display, conditional expansion, editing/save/reload, restore-default, preview success/error, and the fixed-prompt disabled-mode hint.
- Entry-point tests prove scheduled, settings-page batch, and individual checks all use the same generator/executor while retaining their exact automatic/manual state semantics.
- Stream parser tests reject `[DONE]`-only streams, comments/heartbeats, blank data, and empty deltas, and request tests assert fallback payload/header parity except for `stream`.
- Run all Go tests, frontend tests, and the production frontend build.
