# PhonyG Full Rename Design

## Scope

Rename the product and all technical identifiers from PhonyC to PhonyG without compatibility aliases. This includes user-visible text, Go module/import paths, binary and command paths, environment variables, database filename, browser token key, capture identifiers, Docker user/image examples, deployment scripts, package metadata, tests, and documentation.

The repository directory itself remains at its current filesystem path because renaming the workspace is outside the requested code/document change. The GitHub repository is not renamed or pushed.

## Compatibility

- `PHONYC_*` variables are replaced by `PHONYG_*` and are no longer read.
- `phonyc.db` becomes `phonyg.db`; no automatic migration or fallback is added.
- Browser authentication uses `phonyg_token`; existing sessions using `phonyc_token` are not imported.
- Binary, service, process, and Docker examples use `phonyg`.
- The Go module becomes `github.com/phonyg/phonyg`; all internal imports follow it.

## Create-page Actions

The channel and Key page-header create buttons are hidden whenever either a create or edit form is open, so users do not mistake the header action for a save button. The form retains its explicit Save and Cancel buttons. When no create/edit form is open, the header button remains available as the entry point for creating a record; removing it unconditionally would leave no creation entry point in the current UI.

## Verification

- Search tracked source and documentation for remaining case variants of PhonyC/phonyc/PHONYC, excluding historical Git metadata and this specification's compatibility description.
- Run Go formatting and all Go tests.
- Run frontend tests and production build.
- Verify the channel and Key header action is absent during creation/editing and present when the form is closed.
- Commit changes locally and do not push.
