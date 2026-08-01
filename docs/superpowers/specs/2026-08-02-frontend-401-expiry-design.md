# Frontend 401 Session Expiry Handling

## Goal

When an authenticated frontend API request receives HTTP 401, clear the stale JWT, notify the user that the login session has expired, and return to `/login`.

## Behavior

- The shared `web/src/lib/api.ts` request helper handles the behavior once for all pages.
- `/api/auth/login` is excluded so invalid credentials continue to display the normal login error.
- Other 401 responses clear `phonyg_token`, show one browser alert with `登录状态已过期，请重新登录`, and navigate to `/login`.
- A module-level guard prevents duplicate alerts and redirects when several requests fail concurrently.
- Requests made while already on `/login` do not trigger the expiry alert.
- The original `ApiError` is still thrown after session-expiry handling so callers retain normal error flow.

## Verification

Frontend tests cover protected 401 handling, login 401 exclusion, duplicate suppression, token clearing, and redirect behavior.
