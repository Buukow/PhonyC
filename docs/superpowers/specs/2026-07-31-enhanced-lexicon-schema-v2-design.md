# Enhanced Lexicon Schema v2 Design

## Scope

Revise only the enhanced-healthcheck lexicon schema, prompt generator, settings normalization, preview behavior, and corresponding settings-page copy. Stream-first execution and all channel state-transition rules remain unchanged.

## Schema

The canonical lexicon object contains these fields:

- `schema_version`: system-managed integer, initially `2`.
- `prefix`: non-empty string array.
- `target_patterns`: non-empty string array whose entries each contain exactly one `{target}` placeholder.
- `modal_words`: non-empty string array; empty strings remain allowed.
- `short_rules`: non-empty string array.
- `targets`: non-empty string array.

Remove the former `modifier` field. The default `target_patterns` are:

1. `什么是{target}`
2. `{target}是什么`
3. `{target}的原理`
4. `{target}有什么用`
5. `{target}怎么理解`
6. `{target}是做什么的`
7. `{target}的作用`
8. `{target}的基本概念`

The backend owns the canonical schema and defaults. The frontend obtains and displays the backend-provided canonical JSON rather than maintaining an independent schema.

## Structural Normalization

Use one shared normalization function for startup migration and settings saves.

Normalization works at the top-level field boundary only:

1. Parse one JSON object. Malformed JSON or a non-object value is invalid and is not overwritten.
2. Delete fields not present in the current canonical schema, including deprecated fields such as `modifier`.
3. Add each missing canonical field using that field's compiled default value. This includes `target_patterns` and `schema_version`.
4. Set `schema_version` to the current system version even when an older value exists.
5. Do not merge, reorder, add, or remove entries inside an existing user array.
6. Validate all resulting fields. Existing fields with a wrong type, empty required array, invalid element, or invalid target-pattern placeholder cause a clear validation error; normalization must not silently replace those values.

On application startup, normalize the stored lexicon. If normalization succeeds and changes the JSON structure, write the normalized value back to the settings store and log the removed and added field names. If the stored JSON is malformed or contains invalid existing values, keep it unchanged, log the validation error, and let runtime prompt generation use the compiled default lexicon for safety.

On settings save, normalize before the settings transaction is committed. Return the normalized JSON in persisted settings. Invalid existing values reject the entire settings update atomically with a field-specific error.

The preview endpoint normalizes and validates an in-memory copy, generates from that copy, and never writes settings.

## Prompt Generation

For every prompt:

1. Choose one `prefix`, one raw `target`, and one `target_patterns` entry independently and uniformly.
2. Replace the pattern's single `{target}` placeholder with the raw target. Treat the expanded result as one target segment.
3. Choose `{prefix}{target}` or `{target}{prefix}` with equal probability.
4. Include one random `short_rules` value with 40% probability. If included, place it at the beginning or end with equal probability.
5. Append a modal word at most once in the entire prompt: with 30% probability choose one eligible non-empty segment uniformly and append one uniformly selected `modal_words` value. The selected modal value may be empty, so the attempt may have no visible effect.
6. Between every adjacent segment, independently insert `，` with 60% probability; otherwise concatenate directly.
7. Append `。` with 30% probability; otherwise append no terminal punctuation.

All selections described as random use uniform selection. `modifier` no longer participates in generation.

## Settings UI

Update the explanatory template text to show `{prefix}{target}` or `{target}{prefix}` and retain the concise phrase `语气词与标点随机`. The JSON editor exposes `schema_version` and `target_patterns` along with the other canonical fields. Restore-default and preview use the updated backend schema and generator.

## Compatibility and Failure Handling

- An old valid lexicon containing `modifier` is upgraded automatically: remove `modifier`, add default `target_patterns`, and add/update `schema_version`; preserve every entry in all remaining arrays.
- Future releases apply the same field-level rule: obsolete fields disappear, newly required fields receive defaults, and existing array contents remain user-owned.
- Missing fields caused by user deletion are restored on startup or save.
- Invalid existing field values are never silently repaired. Saving reports the error; startup preserves the stored input and falls back to compiled defaults at runtime.
- Normalization must produce deterministic formatted JSON so repeated startups and saves do not continually rewrite unchanged settings.

## Testing

- Schema tests cover old-schema migration, unknown-field deletion, missing-field insertion, schema-version update, deterministic output, user-array preservation, malformed JSON, wrong types, empty required arrays, non-string entries, and invalid/missing/multiple `{target}` placeholders.
- Generator tests cover all target patterns, both outer templates, uniform forced selections, short-rule inclusion and placement, at-most-one modal suffix, comma boundaries, terminal period, and absence of modifier behavior.
- Startup tests verify successful migration is persisted and logged, while invalid stored JSON remains unchanged and runtime generation falls back safely.
- Admin API tests verify save-time normalization is atomic and preview normalization is non-persistent.
- Frontend tests verify the new template text, canonical schema display, restore-default behavior, preview, and validation errors.
- Run all Go tests, frontend tests, production frontend build, and Docker image build.
