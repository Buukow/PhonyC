# PhonyG Jekyll Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and publish a complete Chinese PhonyG documentation site with Jekyll/Just the Docs, real screenshots from the running `23346` instance, and GitHub Pages deployment at `https://buukow.github.io/PhonyC/`.

**Architecture:** Keep the public site isolated under `site/`, build it with a locked Ruby bundle, and deploy `site/_site` through a dedicated Pages workflow. Keep screenshot automation in a root Playwright package, capture read-only management-console states into `site/assets/images/`, and validate navigation, required content, image references, and the `/PhonyC` baseurl before publishing.

**Tech Stack:** Jekyll, Just the Docs, jekyll-remote-theme, GitHub Pages Actions, Markdown/Liquid/Sass, Node.js, Playwright Chromium, Bash, Go/React existing project tests.

---

## File Structure

- `site/Gemfile`, `site/Gemfile.lock`: locked Jekyll and theme dependencies.
- `site/_config.yml`: Pages URL/baseurl, theme, plugins, search, navigation and exclusions.
- `site/_sass/custom/custom.scss`: PhonyG warm sage theme overrides and screenshot styling.
- `site/assets/css/just-the-docs.scss`: theme entry point importing Just the Docs and custom styles.
- `site/assets/images/*`: committed management-console screenshots and capture-client placeholder/asset.
- `site/index.md`: branded documentation landing page.
- `site/getting-started.md`: first-run setup and minimum API calls.
- `site/local-build.md`: source build and development workflow.
- `site/docker.md`: Docker run/Compose, all environment variables, persistence, health, upgrade and rollback.
- `site/features/*.md`: dashboard, channels, keys, presets, capture, logs, healthcheck and retry guides.
- `site/reference/*.md`: environment, API routes/protocols, upgrade/backup and troubleshooting references.
- `scripts/check-docs.mjs`: deterministic structural/content/image validation.
- `scripts/capture-doc-screenshots.mjs`: read-only Playwright screenshot workflow.
- `package.json`, `package-lock.json`: root documentation automation package and commands.
- `.github/workflows/pages.yml`: build and deploy the Jekyll site.
- `README.md`: public documentation link and local preview instructions.

### Task 1: Add documentation validation harness

**Files:**
- Create: `package.json`
- Create: `package-lock.json`
- Create: `scripts/check-docs.mjs`
- Test: `scripts/check-docs.mjs`

- [ ] **Step 1: Write the failing structural validator**

Create a Node script that asserts required site files, navigation order, all five `PHONYG_*` variables, capture semantics, required image names, and baseurl-safe image links. It must fail while `site/` is absent.

- [ ] **Step 2: Run the validator to verify it fails**

Run: `npm run docs:check`

Expected: non-zero exit with missing `site/_config.yml` and content pages.

- [ ] **Step 3: Add the root documentation package**

Define scripts:

```json
{
  "scripts": {
    "docs:check": "node scripts/check-docs.mjs",
    "docs:screenshots": "node scripts/capture-doc-screenshots.mjs"
  },
  "devDependencies": {
    "playwright": "<locked-compatible-version>"
  }
}
```

Generate and commit the lockfile with `npm install --package-lock-only` or `npm install`.

- [ ] **Step 4: Re-run the validator and preserve the expected failure**

Run: `npm run docs:check`

Expected: it still fails only because site files are not implemented.

- [ ] **Step 5: Commit**

```bash
git add package.json package-lock.json scripts/check-docs.mjs
git commit -m "test: add documentation validation harness"
```

### Task 2: Scaffold Jekyll and the PhonyG visual system

**Files:**
- Create: `site/Gemfile`
- Create: `site/Gemfile.lock`
- Create: `site/_config.yml`
- Create: `site/_sass/custom/custom.scss`
- Create: `site/assets/css/just-the-docs.scss`
- Create: `site/_includes/footer_custom.html`
- Create: `site/index.md`
- Modify: `.gitignore`

- [ ] **Step 1: Add the locked Jekyll bundle and configuration**

Configure `url: https://buukow.github.io`, `baseurl: /PhonyC`, Chinese locale, Just the Docs remote theme, `jekyll-remote-theme`, search, heading anchors and external repository links. Exclude `_site`, `.jekyll-cache`, vendor bundle and internal development files.

- [ ] **Step 2: Add the theme entry point and warm sage overrides**

Implement PhonyG colors, responsive content width, screenshot frames/captions, feature grids, callouts, table overflow and code-block polish without replacing Just the Docs navigation behavior.

- [ ] **Step 3: Add the landing page**

Explain the gateway in one sentence, show feature cards for protocol routing, client impersonation, health checks and capture, and link to Quick Start, Local Build, Docker and Capture.

- [ ] **Step 4: Install Ruby dependencies and build**

Run:

```bash
cd site
bundle config set path vendor/bundle
bundle install
bundle exec jekyll build --destination _site --baseurl /PhonyC
```

Expected: successful build; landing page generated at `site/_site/index.html`.

- [ ] **Step 5: Run the docs validator**

Run: `npm run docs:check`

Expected: failure now lists only unimplemented content and screenshots.

- [ ] **Step 6: Commit**

```bash
git add .gitignore site
git commit -m "feat: scaffold PhonyG Jekyll documentation"
```

### Task 3: Write setup and deployment documentation

**Files:**
- Create: `site/getting-started.md`
- Create: `site/local-build.md`
- Create: `site/docker.md`
- Create: `site/reference/environment.md`
- Create: `site/reference/api.md`
- Create: `site/reference/operations.md`
- Create: `site/reference/troubleshooting.md`

- [ ] **Step 1: Write Quick Start and Local Build**

Document first administrator setup, health endpoint, minimum OpenAI/Anthropic calls, Go/Node prerequisites, frontend embedding, `make build`, direct run and development commands.

- [ ] **Step 2: Write Docker immediately after Local Build in navigation**

Document GHCR tags, `docker run`, a complete Compose file, `/data`, non-root user, health check, upgrade, backup and rollback.

- [ ] **Step 3: Document every runtime environment variable**

Include exact defaults and semantics for `PHONYG_ADDR`, `PHONYG_DATA_DIR`, `PHONYG_JWT_SECRET`, `PHONYG_MAX_BODY_BYTES`, and `PHONYG_JWT_TTL_HOURS` in both Docker context and the reference page.

- [ ] **Step 4: Write API, operations and troubleshooting references**

Cover `/v1/chat/completions`, `/v1/completions`, `/v1/responses`, `/v1/messages`, `/v1/models`, `/api/health`, management authentication, SQLite backup, failed health checks, invalid presets and common baseurl/port issues.

- [ ] **Step 5: Run content validation and Jekyll build**

Run:

```bash
npm run docs:check
cd site && bundle exec jekyll build --destination _site --baseurl /PhonyC
```

Expected: deployment/reference assertions pass; remaining failures concern feature pages/images.

- [ ] **Step 6: Commit**

```bash
git add site/getting-started.md site/local-build.md site/docker.md site/reference
git commit -m "docs: add setup and deployment guides"
```

### Task 4: Write complete feature documentation

**Files:**
- Create: `site/features/dashboard.md`
- Create: `site/features/channels.md`
- Create: `site/features/keys.md`
- Create: `site/features/presets.md`
- Create: `site/features/logs.md`
- Create: `site/features/healthcheck.md`
- Create: `site/features/retry.md`

- [ ] **Step 1: Document dashboard, channels and model mapping**

Explain statistics, recent errors, model popularity, protocol selection, priority ordering, timeouts, extra headers, upstream model discovery, client/upstream model mapping and temporary disable states.

- [ ] **Step 2: Document user keys and impersonation modes**

Explain passthrough, preset and custom modes, key creation, enable/disable lifecycle, custom header JSON and preset binding.

- [ ] **Step 3: Document structured presets in depth**

Cover four built-ins, visual/JSON editing, protected headers, templates, time expressions, generators, remove headers, previews, parent inheritance and child overrides. State that the switch defaults right to Force Override and moves left for Fill Missing.

- [ ] **Step 4: Document health checks and enhanced mode**

Explain interval/jitter, first enabled model mapping, manual versus automatic state transitions, configured temporary-disable codes, recovery, lexicon schema, prompt randomization, stream-first execution and non-stream fallback.

- [ ] **Step 5: Document retry and request logs**

Explain retry limit/status filters, interaction with routing, log filters, latency/token columns, error summaries and capture-only rows.

- [ ] **Step 6: Run validator and build**

Run: `npm run docs:check && (cd site && bundle exec jekyll build --destination _site --baseurl /PhonyC)`

Expected: all textual content assertions pass; only screenshot asset assertions may remain.

- [ ] **Step 7: Commit**

```bash
git add site/features
git commit -m "docs: document PhonyG feature set"
```

### Task 5: Write the request capture deep-dive

**Files:**
- Create: `site/features/capture.md`
- Create: `site/assets/images/capture-client-response.png`

- [ ] **Step 1: Add the four-step capture tutorial**

Explain enable/arm, fixed capture key, client Base URL/key setup, protocol-shaped `captured` response, viewing headers, re-arm and saving as a preset.

- [ ] **Step 2: Add exact capture boundaries**

State capture-only/no-upstream behavior, first-request semantics, 403 while unarmed, filtered auth/transport/hop headers, retained session/business headers, and the repeated-header limitation where only the first value is stored.

- [ ] **Step 3: Add capture request-log semantics**

State status 200, no normal user key/channel ID, zero token usage, empty error summary and passthrough metadata without presenting the log as the third primary screenshot.

- [ ] **Step 4: Add the user-provided image slot**

Create an explicit temporary placeholder at `site/assets/images/capture-client-response.png` if the real image has not yet been supplied. The page caption must say it will show an AI client receiving `captured`; replace it byte-for-byte when the user provides the screenshot.

- [ ] **Step 5: Run validator and build**

Run: `npm run docs:check && (cd site && bundle exec jekyll build --destination _site --baseurl /PhonyC)`

Expected: capture semantics and required image reference pass.

- [ ] **Step 6: Commit**

```bash
git add site/features/capture.md site/assets/images/capture-client-response.png
git commit -m "docs: add request capture guide"
```

### Task 6: Implement read-only screenshot automation and capture 23346

**Files:**
- Create: `scripts/capture-doc-screenshots.mjs`
- Create: `site/assets/images/dashboard.png`
- Create: `site/assets/images/channels.png`
- Create: `site/assets/images/keys.png`
- Create: `site/assets/images/presets.png`
- Create: `site/assets/images/preset-editor.png`
- Create: `site/assets/images/healthcheck-enhanced.png`
- Create: `site/assets/images/capture.png`
- Create: `site/assets/images/capture-headers.png`
- Create: `site/assets/images/logs.png`
- Modify: `site/**/*.md`

- [ ] **Step 1: Write screenshot-script input validation**

Require `PHONYG_DOCS_URL` and one authentication mechanism from environment. Never log credentials or tokens. Fail early if the target health endpoint is unavailable.

- [ ] **Step 2: Implement login and safe navigation**

Use an isolated browser context, fixed desktop viewport and localStorage/API login. Allow navigation, scroll, expand/collapse and frontend-only tabs; forbid save/delete/test/arm/refresh/generate actions.

- [ ] **Step 3: Implement deterministic screenshot helpers**

Wait for page headers and loading completion, disable CSS animation, use stable output names, record only URL/file/dimensions, and exit non-zero on missing selectors.

- [ ] **Step 4: Install Chromium and run against 23346**

Run:

```bash
npm ci
npx playwright install chromium
PHONYG_DOCS_URL=http://127.0.0.1:23346 \
PHONYG_DOCS_USERNAME="$PHONYG_DOCS_USERNAME" \
PHONYG_DOCS_PASSWORD="$PHONYG_DOCS_PASSWORD" \
npm run docs:screenshots
```

Expected: all PNG files are created without changing persistent application data.

- [ ] **Step 5: Visually inspect every PNG**

Check full-resolution images for clipping, loading placeholders, accidental modal overlays and unreadable text. Re-run individual captures if necessary.

- [ ] **Step 6: Embed screenshots in their matching pages**

Use baseurl-safe Liquid image URLs, descriptive alt text and captions. Presets must show the override switch/tree editor; healthcheck must show enhanced settings; capture must show capture state and captured headers.

- [ ] **Step 7: Run validator and build**

Run: `npm run docs:check && (cd site && bundle exec jekyll build --destination _site --baseurl /PhonyC)`

Expected: PASS with all images present and referenced.

- [ ] **Step 8: Commit**

```bash
git add scripts/capture-doc-screenshots.mjs site/assets/images site
git commit -m "docs: add management console screenshots"
```

### Task 7: Add GitHub Pages deployment and README links

**Files:**
- Create: `.github/workflows/pages.yml`
- Modify: `README.md`

- [ ] **Step 1: Add the Pages workflow**

Implement checkout, Ruby/Bundler cache explicitly against `site/Gemfile` and
`site/Gemfile.lock`, `actions/configure-pages`, artifact upload and
`actions/deploy-pages`. Set job-level `BUNDLE_GEMFILE: site/Gemfile` (or an
equivalent `working-directory: site` on every Bundler step), run `bundle
install`, then build from the repository root with:

```bash
BUNDLE_GEMFILE=site/Gemfile bundle exec jekyll build \
  --source site --destination site/_site --baseurl /PhonyC
```

The `ruby/setup-ruby` Bundler cache must use the lockfile belonging to
`site/Gemfile`, never look for a root Gemfile. Set minimal permissions, Pages
environment and concurrency.

- [ ] **Step 2: Update README**

Add the public documentation URL near the title, update stale image-version examples to the current version where appropriate, and add exact local documentation preview commands.

- [ ] **Step 3: Validate workflow syntax and docs links**

Run: `git diff --check && npm run docs:check`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/pages.yml README.md
git commit -m "ci: deploy Jekyll documentation to Pages"
```

### Task 8: Full verification and live preview on port 23342

**Files:**
- Modify as needed: `site/**`, `scripts/**`, `.github/workflows/pages.yml`, `README.md`

- [ ] **Step 1: Stop the brainstorming companion server**

Free port `23342` without stopping the running PhonyG container on `23346`.

- [ ] **Step 2: Run all project and documentation checks**

Run:

```bash
go test ./... -count=1
cd web && npm test -- --run && npm run build
cd ..
npm run docs:check
cd site && bundle exec jekyll build --destination _site --baseurl /PhonyC
git diff --check
```

Expected: all commands pass.

- [ ] **Step 3: Start the Jekyll preview on `0.0.0.0:23342`**

Run in a persistent session:

```bash
cd site
bundle exec jekyll serve --host 0.0.0.0 --port 23342 --baseurl /PhonyC
```

Expected: `http://202.189.7.62:23342/PhonyC/` returns HTTP 200.

- [ ] **Step 4: Browser-smoke-test the built site**

Use headless Chromium to visit the homepage, Docker, Presets, Healthcheck and Capture pages under `/PhonyC/`; verify navigation, search assets, screenshots and no browser console errors.

- [ ] **Step 5: Commit final fixes**

```bash
git add -A
git commit -m "docs: finalize PhonyG documentation site"
```

- [ ] **Step 6: Push and verify Pages**

Push `main`, wait for the Pages workflow, verify `https://buukow.github.io/PhonyC/`, and report the workflow URL. If GitHub Pages is not configured to use Actions, enable that repository setting and re-run the workflow.
