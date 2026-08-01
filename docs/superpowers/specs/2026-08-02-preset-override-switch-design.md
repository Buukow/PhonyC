# Preset Override Switch Design

## Goal

Replace the visual editor's missing-completion checkboxes with explicit
two-state switches and make all four built-in client presets force their
fingerprint headers by default.

## Behavior

- `fill_missing=false`: `强制覆盖`, switch right, default for new rules.
- `fill_missing=true`: `缺失补全`, switch left.
- A top-level Header switch defines the default for all nested JSON fields.
- A child without an explicit `children_fill_missing` entry inherits its
  parent's effective mode and is labeled as inherited.
- Switching a child stores an explicit Boolean entry. `恢复继承` removes it.
- Changing a parent does not clear explicit child entries.
- A parent with explicit descendants displays `含自定义子项`.

The existing resolver and JSON schema already implement these Boolean
semantics, so no schema migration is required.

## Built-in Presets

`codex-tui`, `codex-enhanced`, `claude-cli`, and `claude-enhanced` use
`fill_missing=false` for every top-level rule. Startup seeding refreshes all
four immutable built-in definitions so deployed databases receive the new
defaults. Custom presets retain their stored rules.

## Verification

- Seed tests prove all built-ins override client values.
- Resolver tests continue covering both force override and missing completion.
- Frontend tests cover parent switching, inherited child modes, explicit child
  modes, restore inheritance, and new Header defaults.
