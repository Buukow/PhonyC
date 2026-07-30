# Channel Input Stability and Temporary Disable Design

## Goals

- Keep every editable model-mapping input focused while its value changes.
- Audit all frontend controlled inputs for the same remount-on-change failure mode.
- Present channel state as exactly one of enabled, disabled, or temporarily disabled.
- Make manual enable and manual health checks follow the confirmed state-transition rules.
- Publish release 1.1 and replace the existing service on port 23346 after its image build succeeds.

## Model Mapping Editor

Each model draft receives a frontend-only stable row identifier when it is loaded or created. React list keys use that identifier and never use editable model names or array values. API payloads omit the identifier. Adding, editing, reordering through deletion, and changing either model field therefore preserves the identity and focus of every surviving row.

The frontend audit covers every editable or focusable field and its complete keyed ancestor chain. It checks mapped lists, conditional branches, changing component types, wrapper identity, and keys derived directly or indirectly from edited values. The audited locations and result are recorded in the implementation commit or test source. Any unstable identity found will be replaced with a stable record identity. Unrelated layout or styling changes are out of scope.

## Channel State Model

The existing database fields remain authoritative:

| `enabled` | `temp_disabled` | Displayed state | Routing eligibility |
| --- | --- | --- | --- |
| false | either | 停用 | no |
| true | false | 启用 | yes |
| true | true | 临时禁用 | no |

The channel list displays exactly one state badge with one of these exact texts: `停用`, `启用`, or `临时禁用`. It never renders the legacy `测活临时禁用` badge.

The action button is derived from effective state:

- Enabled: show “停用”; clicking writes `enabled=false`.
- Disabled: show “启用”; clicking writes `enabled=true, temp_disabled=false`.
- Temporarily disabled: show “启用”; clicking writes `enabled=true, temp_disabled=false`.

This makes a user enable action an explicit override of automatic temporary disablement.

## Temporary Disable Transitions

Only live API traffic (including channels attempted during automatic retry) and automatic health checks may enter temporary disablement after receiving a configured disable status code.

Automatic health checks continue to test manually enabled channels, including temporarily disabled channels. A configured disable response sets temporary disablement; a successful response clears it.

Automatic health-check eligibility is exact: `enabled=false` channels are skipped entirely; `enabled=true, temp_disabled=false` and `enabled=true, temp_disabled=true` channels are tested. A manually disabled channel can never acquire `temp_disabled=true`.

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
- Focused UI regression tests verify that typing multiple characters into both selected-model fields keeps the same DOM element focused, and cover each other mapped or conditionally rendered editable-field class found by the audit.
- The test/audit artifact lists every editable-field location and confirms its full keyed ancestor chain is stable while editing.
- UI state tests cover all `enabled`/`temp_disabled` combinations, assert exactly one badge with the required text, assert absence of `测活临时禁用`, and assert the exact Enable/Disable action text.
- UI/API integration tests verify disabled and temporarily disabled Enable actions persist `enabled=true, temp_disabled=false`, while enabled Disable persists `enabled=false, temp_disabled=false`.
- Store tests cover explicit enable clearing temporary disablement and disabling normalization.
- Health-check tests cover disabled reporting without transition, enabled reporting without transition, temporary-disabled success recovery, and temporary-disabled failure retention.
- Automatic-health-check tests cover disabled-channel exclusion, configured-code disablement, non-configured-code non-disablement, and successful recovery.
- Live proxy and automatic-retry tests cover configured-code disablement and non-configured-code non-disablement for every attempted channel.
- Existing Go tests and frontend build run before commits and again before release.

The existing setting `auto_test_disable_status_codes`, parsed through `store.ParseStatusCodeList`, is the single configured source for temporary-disable status codes in live traffic, retry traffic, and automatic health checks.

## Release and Deployment

The release version is 1.1. The workflow publishes `1.1`, `latest`, the Git tag-derived `v1.1`, and the commit SHA tag for linux/amd64. Documentation is updated accordingly.

After pushing `main` and tag `v1.1`, poll GitHub Actions once per minute. When the matching image build succeeds, obtain the published image digest from the workflow or registry. Pull through the Nanjing University mirror, preferring `ghcr.nju.edu.cn/ghcr.io/buukow/phonyg:1.1` and trying the mirror’s alternate path form if necessary, and verify the pulled image digest matches the published digest. Deployment uses the verified digest rather than a mutable tag.

Before replacement, identify the container publishing host port 23346 and securely save its complete `docker inspect` result, exact old image digest, and applicable recreation configuration. Prefer its existing Compose or service definition if present, changing only the image. Otherwise reproduce all applicable container and host settings, including name, mounts, environment, ports, restart policy, networks and aliases, command/entrypoint, labels, user, working directory, healthcheck, logging, DNS/hosts, capabilities, and devices.

The old container is renamed and retained in a stopped state until verification passes. The new container must attach to the same networks and retain dependent-service connectivity. Verification is bounded: it must remain running for 15 seconds and `http://127.0.0.1:23346/api/health` must return HTTP 200 with the expected healthy JSON body within five attempts at two-second intervals. Any creation, runtime, port, health, network, or connectivity failure automatically removes the failed replacement and restores the prior container/image/configuration. The retained old container is removed only after all verification passes.

The supplied GitHub PAT is never placed in a command argument, URL, repository file, persistent Git configuration, shell history, or captured log. Push authentication uses a temporary permission-restricted askpass/credential helper that receives the secret without echoing it; the helper and any temporary credential material are removed immediately afterward, and persistent credential configuration is checked to remain absent.
