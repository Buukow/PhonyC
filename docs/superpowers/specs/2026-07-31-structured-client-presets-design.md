# Structured Client Presets Design

## Scope

Upgrade client presets from a text-only header map to a versioned structured rule document. The feature covers preset creation and editing, request-header resolution, template validation, protected headers, in-memory dynamic generators, preview, capture-to-preset conversion, and legacy migration.

The default editor is a visual tree editor that follows the existing PhonyG card, input, button, badge, spacing, color, and responsive layout system. A native JSON editor remains available as a second synchronized view.

## Preset Document

Store the canonical preset rule as one JSON document in a new preset column while retaining the current columns during migration. Schema version 1 is:

```json
{
  "schema_version": 1,
  "headers": {
    "User-Agent": {
      "value": "codex-tui/{{version}}",
      "fill_missing": false
    },
    "X-Codex-Turn-Metadata": {
      "value": {
        "installation_id": "{{generator:installation}}",
        "session_id": "{{client_header:Session-Id}}",
        "turn_started_at_unix_ms": "{{time:unix_ms}}"
      },
      "fill_missing": true,
      "children_fill_missing": {
        "installation_id": true,
        "session_id": true,
        "turn_started_at_unix_ms": false
      }
    }
  },
  "remove_headers": [],
  "generators": {}
}
```

Header names are matched case-insensitively and emitted with the configured canonical spelling. Header values may be strings, numbers, booleans, null, arrays, or objects in the editor. Non-string values are serialized as compact JSON when written to an HTTP Header.

## Missing Completion

The checkbox label is `缺失补全`.

For a top-level Header rule:

| 缺失补全 | Client Header exists | Client Header missing |
|---|---|---|
| Checked | Preserve the original client value | Add the resolved preset value |
| Unchecked | Replace with the resolved preset value | Add the resolved preset value |

For a JSON-object Header, child paths apply the same rule independently after the client Header is parsed as JSON:

- A checked child preserves an existing client child value and fills only when that child path is absent.
- An unchecked child replaces an existing child value and also adds it when absent.
- A parent Header that is completely absent is constructed from the preset value and included.
- Arrays are tree-editable, but `缺失补全` applies to array elements by numeric index. Existing indices are preserved when checked; missing indices are filled. The editor must clearly display array indices.

Parent/child checkbox interaction is deliberately not tri-state:

- Checking a parent selects every descendant.
- A descendant may then be unchecked independently without changing the parent checkbox.
- Unchecking a parent clears every descendant.
- Reloading reproduces the explicitly stored parent and child selections exactly.

Client JSON Header parse failures are outside the first-release special-handling scope. Resolution returns a clear request error rather than silently generating a contradictory partial value.

## Protected Headers

Preset rules and remove rules cannot write, remove, reference through resolved output, or override authentication and transport-controlled headers:

- `Authorization`
- `X-Api-Key`
- `Host`
- `Content-Length`
- `Accept-Encoding`
- hop-by-hop headers such as `Connection`, `Keep-Alive`, `Transfer-Encoding`, `Upgrade`, `TE`, `Trailer`, proxy authentication headers, and `Proxy-Connection`

The visual editor disables these names and explains the restriction. JSON save, preview, capture conversion, and runtime resolution enforce the same backend validation so bypassing the UI is impossible. Protocol authentication remains authoritative.

## Template Language

Templates use double braces only. Plain words such as `version` are never interpreted implicitly.

Supported references:

```text
{{version}}
{{client_header:Header-Name}}
{{resolved_header:Header-Name}}
{{generator:generator_name}}
```

- `client_header` reads the untouched incoming client Header, case-insensitively.
- `resolved_header` reads the value after all preceding preset resolution. Header dependency order is determined by a validated dependency graph, not object-map iteration order.
- References may appear inside longer strings.
- Unknown references, self-reference, indirect cycles, protected-header references, and type-incompatible placement reject save and preview.

Time expressions are evaluated in the configured server timezone, currently `Asia/Shanghai`:

```text
{{time:year}}
{{time:month}}
{{time:day}}
{{time:hour}}
{{time:minute}}
{{time:second}}
{{time:millisecond}}
{{time:unix}}
{{time:unix_ms}}
```

Every field is zero-padded to its conventional width except Unix timestamps. All time expressions in one request use one captured request timestamp so boundary changes cannot produce inconsistent fields.

## Generators

Generators are named and reusable. Multiple references to one generator within the same applicable refresh period return the same value. The visual editor provides forms and insertion menus; users do not need to compose syntax manually.

Supported generator output types:

- `uuid`: UUID-compatible identifier.
- `random`: fixed-length random characters.

Random character sets:

- `digits`
- `lowercase`
- `uppercase`
- `letters`
- `alnum`
- optional exclusion of ambiguous characters such as `0`, `O`, `1`, `I`, and `l`

Random length is an integer from 1 through 256. Randomness uses `crypto/rand`.

Each generator has one refresh mode:

### Per Request

Generate once on first use in each request and reuse it for every reference during that request. The next request receives a new value.

### Interval

Generate at service initialization, keep the current value in memory, and refresh after a configured rolling duration. Supported duration range is 1 second through 365 days. A background scheduler refreshes due generators; request-time expiry checks provide a fallback if the scheduler is delayed. Interval timing starts from the last in-memory generation time and is not aligned to wall-clock boundaries.

### Increment

Generate a random numeric initial value of the configured width at service initialization. The first resolution returns that initial value; each subsequent request that uses the generator increments it by the configured positive step. All references within one request receive the same assigned value. Leading zeroes are retained. Overflow behavior is configurable as `wrap`, `regenerate`, `expand`, or `error`, with `wrap` as the default.

Only numeric generators support increment mode. Runtime uses an in-memory lock or atomic operation so concurrent requests do not receive the same assigned increment value.

### Runtime Fixed

Generate once at service initialization and reuse the value until service restart or manual refresh.

Generator state is intentionally not persisted. Normal and abnormal restarts reinitialize interval, increment, and runtime-fixed state. No rollback, replay protection, crash recovery, or cross-restart uniqueness is required.

Rule changes that affect generator type, character set, length, mode, duration, step, or overflow reset that generator's in-memory state immediately. Deleting a referenced generator is rejected. Deleting a preset removes its runtime state. Editing a built-in preset and saving as a new custom preset creates independent generator state.

## Remove Headers

`remove_headers` is part of the structured visual tree rather than a separate raw input. The editor supports adding, renaming, and deleting removal entries. Protected headers cannot be added. Removal occurs before preset Header resolution, matching the existing behavior, so a non-protected removed Header may subsequently be added by a preset rule.

## Visual Editor

The editor has two tabs: `可视化编辑` and `JSON 编辑`, with visual mode selected by default.

Visual mode supports:

- Expand/collapse per object or array node.
- Expand all and collapse all.
- Add Header, object field, and array element.
- Delete and rename nodes.
- Select value type and edit the typed value.
- `缺失补全` checkbox on every Header and nested field.
- Generator list with create, edit, delete, manual refresh, current masked/unmasked value, generated time, and next refresh time where applicable.
- Template-variable insertion menu for version, client Header, resolved Header, generator, and time expressions.
- Inline errors attached to exact nodes.
- Search/filter by Header or child-path name.
- Unsaved-change protection when cancelling or leaving the page.

JSON mode edits the same canonical document. Switching from JSON to visual mode requires valid JSON and valid schema. The editor formats canonical JSON on explicit `格式化`, successful save, restore, and initial load; it does not rewrite text on every keystroke.

Both views include `预览结果`, `差异预览`, `恢复`, and `保存` actions. Preview accepts optional simulated client Headers and returns:

- Final upstream Header values.
- Added fields.
- Overridden fields.
- Client values preserved by `缺失补全`.
- Removed fields.
- Rules skipped because their conditions did not apply.
- Generated values and refresh metadata, with sensitive values maskable.

Preview evaluates against an isolated in-memory generator context by default and must not advance live increment counters or refresh live generator state. A separate explicit `手动刷新` action changes live runtime state.

## Built-in Presets and Capture

Built-in preset rows remain immutable as stored system definitions. Selecting `编辑` opens the complete editor, but saving requires a new unique preset name and creates a custom preset. The UI labels this action `另存为新预设`; it never silently updates the built-in row.

Captured request Headers convert into schema version 1 with:

- One top-level rule per captured non-protected Header.
- `fill_missing: false` by default, preserving the current force-override behavior.
- No generators unless the user adds them later.
- Empty `remove_headers`.

Overwriting a custom captured preset remains available after validation. A capture cannot overwrite a built-in preset.

## Legacy Compatibility and Migration

Existing `headers_json` plus `remove_headers` values are converted lazily or during startup into the structured document:

- Every existing Header becomes a top-level rule with `fill_missing: false`.
- Existing `{{version}}` remains valid.
- Existing remove entries move into structured `remove_headers`.
- No generator is created automatically.
- Existing Header values and spellings are preserved.

The migration writes a canonical versioned document but retains legacy columns during the first compatible release for rollback/read compatibility. New runtime resolution uses the structured document when present and falls back to legacy fields only when structured data is absent. Invalid legacy JSON is left unchanged, logged, and surfaced as an editor validation error rather than overwritten.

## Runtime Architecture

Use these bounded components:

- `PresetSchema`: parse, normalize, validate, migrate, and format rule documents.
- `TemplateCompiler`: tokenize expressions, build resolved-header dependency graphs, detect cycles, and compile reusable templates.
- `GeneratorManager`: own per-preset in-memory generator state, scheduler lifecycle, concurrent increment assignment, and manual refresh.
- `HeaderResolver`: apply removals, protected-header rules, `缺失补全`, JSON child merging, template evaluation, and final serialization.
- `PresetEditor`: maintain synchronized visual/JSON state without implementing backend-only validation rules independently.

Snapshot reload compiles preset rules before publishing them. Invalid rules prevent the affected preset from becoming active and return a clear administrative error; the previous valid snapshot remains in use.

## Validation

Save and preview reject:

- Malformed JSON or wrong schema types.
- Empty or duplicate Header names under case-insensitive matching.
- Protected Header writes, removals, or prohibited references.
- Unknown template expressions or generator names.
- Direct or indirect resolved-header cycles.
- Invalid generator names, types, lengths, character sets, intervals, steps, or overflow modes.
- Increment mode on non-numeric output.
- Empty character sets after exclusions.
- Duplicate generator names.
- Removal entries that conflict only by case.
- Invalid `children_fill_missing` paths.

Errors include a stable code, readable Chinese message, and JSON/tree path so both editor views can highlight the exact location.

## API Changes

Preset create/update/get/list responses expose the structured document while retaining legacy fields during compatibility. Add authenticated endpoints for:

- Validate/normalize without saving.
- Preview using simulated client Headers.
- Manually refresh one generator.
- Retrieve current generator runtime status with masked values by default.

Create and update are transactional. Runtime state changes only after database persistence and snapshot compilation succeed.

## Testing

- Schema tests cover canonical parsing, every validation rule, deterministic formatting, legacy migration, invalid legacy preservation, and protected-header enforcement.
- Resolution tests cover all four `缺失补全` cases, case-insensitive names, nested objects, arrays by index, removals, JSON parse errors, and compact serialization.
- Parent/child tests cover select-all, descendant opt-out without parent mutation, parent clear-all, and reload preservation.
- Template tests cover version, client/resolved references, dependency ordering, cycles, time consistency, missing references, and embedded expressions.
- Generator tests cover UUID, every charset, requested lengths, ambiguity exclusions, crypto-random error handling, per-request reuse, interval refresh and request-time fallback, increment concurrency/overflow/leading zeroes, runtime-fixed reuse, manual refresh, rule-change reset, and restart reinitialization.
- API tests cover atomic create/update, validation paths, isolated preview, masked runtime status, manual refresh, built-in save-as, capture conversion, and custom overwrite restrictions.
- Frontend tests cover default visual mode, tree operations, JSON synchronization and invalid-switch blocking, `缺失补全` interactions, template insertion, generator forms, inline errors, search, preview diff, unsaved-change protection, and responsive layout.
- Integration tests prove proxy resolution preserves existing client values when checked, fills missing values, force-overrides when unchecked, replaces upstream authentication correctly, and never emits protected or automatically injected `Accept-Encoding` headers.
- Run Go vet/tests, frontend tests, production build, Docker build, and a direct-versus-relayed Header comparison using an isolated Codex client.
