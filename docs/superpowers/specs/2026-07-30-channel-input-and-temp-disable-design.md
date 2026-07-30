# Channel Input Stability and Temporary Disable Design

## Goals

- Keep every editable model-mapping input focused while its value changes.
- Audit all frontend controlled inputs for the same remount-on-change failure mode.
- Present channel state as exactly one of enabled, disabled, or temporarily disabled.
- Make manual enable and manual health checks follow the confirmed state-transition rules.
- Publish release 1.1 and replace the existing service on port 23346 after its image build succeeds.

## Model Mapping Editor

Each model draft receives a frontend-only stable row identifier when it is loaded or created. React list keys use that identifier and never use editable model names or array values. API payloads omit the identifier. Adding, editing, reordering through deletion, and changing either model field therefore preserves the identity and focus of every surviving row.

The frontend audit covers every controlled `input`, `select`, and `textarea`, with special attention to elements rendered inside mapped lists or conditional branches. Any other key derived from the value being edited will be replaced with a stable record identity. Unrelated layout or styling changes are out of scope.

## Channel State Model

The existing database fields remain authoritative:

| `enabled` | `temp_disabled` | Displayed state | Routing eligibility |
| --- | --- | --- | --- |
| false | either | 停用 | no |
| true | false | 启用 | yes |
| true | true | 临时禁用 | no |

The channel list displays only one state badge. It no longer displays the label “测活临时禁用”.

The action button is derived from effective state:

- Enabled: show “停用”; clicking writes `enabled=false`.
- Disabled: show “启用”; clicking writes `enabled=true, temp_disabled=false`.
- Temporarily disabled: show “启用”; clicking writes `enabled=true, temp_disabled=false`.

This makes a user enable action an explicit override of automatic temporary disablement.

## Temporary Disable Transitions

Only live API traffic (including channels attempted during automatic retry) and automatic health checks may enter temporary disablement after receiving a configured disable status code.

Automatic health checks continue to test manually enabled channels, including temporarily disabled channels. A configured disable response sets temporary disablement; a successful response clears it.

Manual health checks are allowed for all channels:

- Manually disabled: record status, latency, and error only; never alter state.
- Enabled: record results only; never alter state regardless of response.
- Temporarily disabled: a successful response clears `temp_disabled`; a failed response leaves it set.

Network errors and missing-model errors are failures and never recover a temporarily disabled channel.

## Backend and API Changes

The existing channel update API accepts the explicit state combination used by the UI. Store logic normalizes `enabled=false` to clear stale temporary disablement, so manually disabled records do not retain a hidden temporary state.

The manual test endpoint no longer rejects manually disabled channels. It snapshots the channel’s state before testing and allows recovery only when that snapshot was `enabled=true, temp_disabled=true` and the health check succeeds. Test-result persistence continues to update status, latency, timestamp, and error fields for every channel state.

## Testing

- Frontend build and type checking must pass.
- A focused UI regression test verifies that typing multiple characters into a selected model input keeps the same DOM element focused.
- Static review verifies all mapped controlled inputs use stable keys.
- Store tests cover explicit enable clearing temporary disablement and disabling normalization.
- Health-check tests cover disabled reporting without transition, enabled reporting without transition, temporary-disabled success recovery, and temporary-disabled failure retention.
- Existing Go tests and frontend build run before commits and again before release.

## Release and Deployment

The release version is 1.1. The workflow publishes `1.1`, `latest`, the Git tag-derived `v1.1`, and the commit SHA tag for linux/amd64. Documentation is updated accordingly.

After pushing `main` and tag `v1.1`, poll GitHub Actions once per minute. When the matching image build succeeds, pull through the Nanjing University mirror, preferring `ghcr.nju.edu.cn/ghcr.io/buukow/phonyc:1.1` and trying the mirror’s alternate path form if necessary.

Before replacement, inspect the existing port-23346 container and preserve its container name, mounts/volumes, environment, published ports, and restart policy. Recreate only that container with the new image. Verify the container is healthy/running, port 23346 is listening, and `/api/health` succeeds. If replacement fails, retain enough inspected configuration to restore the previous image.

The supplied GitHub PAT is used only in an ephemeral push command or credential helper and is never stored in the repository, persistent Git configuration, shell history, or logs.
