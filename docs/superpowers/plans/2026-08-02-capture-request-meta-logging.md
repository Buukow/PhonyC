# Capture Request Metadata Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make header-capture requests visible in the PhonyG management console request log without forwarding them upstream.

**Architecture:** Reuse the existing `Handler.logMeta` path from `handleCaptureOnly`. After the local capture attempt, insert one request metadata row with HTTP 200, the parsed client model, no channel or ordinary user-key ID, and an empty error summary. Add a focused handler test that verifies the row in the test SQLite database while preserving the existing capture response behavior.

**Tech Stack:** Go, Gin, SQLite, Go testing, existing PhonyG store/proxy abstractions.

---

### Task 1: Add a failing request-log assertion to the capture test

**Files:**
- Modify: `/root/git/PhonyC/internal/proxy/handler_test.go:181-224`

- [x] **Step 1: Extend `TestCaptureAnyModelSuccessShape` to query `request_meta` after the request**

  Keep the existing capture-key request and response assertions. After `r.ServeHTTP`, call `st.ListRequestMeta(store.LogFilter{Path: "/v1/chat/completions", Limit: 10})` and assert exactly one row with:
  - `StatusCode == 200`
  - `ErrorSummary == ""`
  - `ClientModel == "totally-unknown-model-xyz"`
  - `UpstreamModel == ""`
  - `ChannelID == nil`
  - `UserKeyID == nil`
  - `Method == "POST"`
  - `ImpersonationMode == "passthrough"`

  Because `logMeta` writes asynchronously, poll for a short bounded period (for example, 100 iterations with 5ms sleeps), failing with the last query error or row count if no row appears. This avoids a race in the test without changing production behavior.

- [x] **Step 2: Run the focused test to verify it fails before implementation**

  Run: `go test ./internal/proxy -run TestCaptureAnyModelSuccessShape -count=1`

  Expected: FAIL because no `request_meta` row is currently inserted by the capture-only branch.

### Task 2: Log capture-only requests through the existing metadata helper

**Files:**
- Modify: `/root/git/PhonyC/internal/proxy/handler.go:360-387`

- [x] **Step 1: Add the `logMeta` call in `handleCaptureOnly`**

  After `TryCapture` returns and before `writeCaptureClientSuccess`, call:

  ```go
  h.logMeta(
      reqID,
      nil,
      model,
      "",
      nil,
      c.Request.Method,
      path,
      http.StatusOK,
      0,
      time.Since(start),
      "",
      "passthrough",
      usage.Tokens{},
  )
  ```

  The method must receive the request start time. Update the call site at `/root/git/PhonyC/internal/proxy/handler.go:117` and the function signature at `/root/git/PhonyC/internal/proxy/handler.go:360` to pass `start`. Do not call `stats`, because the synthetic capture key has no ordinary user-key ID and capture requests should not affect per-key statistics.

- [x] **Step 2: Run the focused test to verify it passes**

  Run: `go test ./internal/proxy -run TestCaptureAnyModelSuccessShape -count=1`

  Expected: PASS, including the new metadata assertions.

### Task 3: Verify regression coverage and formatting

**Files:**
- Modify: `/root/git/PhonyC/internal/proxy/handler.go`
- Modify: `/root/git/PhonyC/internal/proxy/handler_test.go`

- [x] **Step 1: Format changed Go files**

  Run: `gofmt -w internal/proxy/handler.go internal/proxy/handler_test.go`

- [x] **Step 2: Run all proxy and store tests**

  Run: `go test ./internal/proxy ./internal/store -count=1`

  Expected: PASS with no new failures.

- [x] **Step 3: Run the full Go test suite**

  Run: `go test ./... -count=1`

  Expected: PASS.

- [x] **Step 4: Review the diff and commit the implementation**

  Run:

  ```bash
  git diff --check
  git diff -- internal/proxy/handler.go internal/proxy/handler_test.go
  git add internal/proxy/handler.go internal/proxy/handler_test.go
  git commit -m "feat: log capture-only requests"
  ```
