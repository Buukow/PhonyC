# Preset Override Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace preset missing-completion checkboxes with inheritance-aware two-state switches and make all built-in client presets force-override their fingerprint headers.

**Architecture:** Keep the existing `fill_missing` and `children_fill_missing` JSON schema and resolver semantics. Change startup seeds so all four built-ins use `fill_missing=false`, then replace the visual editor checkbox with a reusable mode-switch UI that distinguishes inherited and explicit child settings. Existing custom preset JSON remains untouched.

**Tech Stack:** Go, SQLite seed logic, React, TypeScript, Tailwind CSS, Vitest, Testing Library.

---

### Task 1: Make built-in presets force override

**Files:**
- Modify: `/root/git/PhonyC/internal/seed/presets.go`
- Modify: `/root/git/PhonyC/internal/seed/presets_test.go`

- [ ] Update basic and enhanced built-in rules to `FillMissing: false`.
- [ ] Force-refresh all four immutable built-in rows during startup while leaving custom presets unchanged.
- [ ] Update seed tests to prove client Header values are overwritten by each basic and enhanced preset.
- [ ] Run `go test ./internal/seed -count=1` and expect PASS.

### Task 2: Add inheritance-aware mode switches

**Files:**
- Modify: `/root/git/PhonyC/web/src/pages/Presets.tsx`
- Modify: `/root/git/PhonyC/web/src/pages/Presets.test.tsx`

- [ ] Add a two-state button switch: left `缺失补全`, right/default `强制覆盖`.
- [ ] Pass each node its inherited mode and whether it has an explicit child override.
- [ ] A child switch stores an explicit `children_fill_missing[path]` value without changing descendants or the parent.
- [ ] Add `恢复继承` for explicit child entries; deleting it removes the map entry.
- [ ] Parent changes update only `fill_missing`, preserving explicit child entries.
- [ ] Display `含自定义子项` on parent/object rows with explicit descendant entries.
- [ ] Keep new Headers defaulted to force override.
- [ ] Replace checkbox tests with switch, inheritance, explicit override, restore inheritance, and default-mode tests.
- [ ] Run `npm test -- --run web/src/pages/Presets.test.tsx` from `/root/git/PhonyC/web` and expect PASS.

### Task 3: Verify and commit

**Files:**
- Modify: `/root/git/PhonyC/internal/seed/presets.go`
- Modify: `/root/git/PhonyC/internal/seed/presets_test.go`
- Modify: `/root/git/PhonyC/web/src/pages/Presets.tsx`
- Modify: `/root/git/PhonyC/web/src/pages/Presets.test.tsx`
- Modify: `/root/git/PhonyC/docs/superpowers/plans/2026-08-02-preset-override-switch.md`

- [ ] Run `gofmt -w internal/seed/presets.go internal/seed/presets_test.go`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `npm test -- --run` and `npm run build` from `/root/git/PhonyC/web`.
- [ ] Run `git diff --check` and review the focused diff.
- [ ] Commit with `git commit -m "feat: add preset override switches"`.
