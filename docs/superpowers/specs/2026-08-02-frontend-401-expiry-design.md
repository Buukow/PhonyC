# Frontend 401 Session Expiry Handling

## Goal

When an authenticated frontend API request receives HTTP 401, clear the stale JWT, notify the user that the login session has expired, and return to `/login`.

## Behavior

- The shared `web/src/lib/api.ts` request helper handles the behavior once for all pages.
- Session expiry handling applies only when the failed request was sent with a non-empty `phonyg_token`. Public requests without a Token never report an expired login.
- `/api/auth/login` is excluded by normalized URL pathname, including absolute URLs and URLs with query parameters, so invalid credentials continue to display the normal login error.
- An eligible 401 clears `phonyg_token`, shows one browser alert with `登录状态已过期，请重新登录`, and navigates to `/login`.
- A module-level guard is set before alerting and prevents duplicate alerts and redirects when several requests fail concurrently. `setToken()` resets the guard after a successful login so a later expiry can notify again.
- If an eligible request receives 401 while already on `/login`, the helper still clears the Token and throws `ApiError`, but skips the alert and navigation.
- The original `ApiError` is still thrown after session-expiry handling so callers retain normal error flow.

## Verification

Frontend tests cover protected 401 handling, normalized login URL exclusion, tokenless 401 behavior, duplicate suppression, token clearing, redirect behavior, login-page behavior, and guard reset after `setToken()`.
