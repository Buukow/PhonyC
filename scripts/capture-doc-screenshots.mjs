import fs from 'node:fs/promises'
import path from 'node:path'
import { chromium } from 'playwright'

const baseURL = (process.env.PHONYG_DOCS_URL || '').replace(/\/$/, '')
const username = process.env.PHONYG_DOCS_USERNAME || ''
const password = process.env.PHONYG_DOCS_PASSWORD || ''
const suppliedToken = process.env.PHONYG_DOCS_TOKEN || ''
const outputDir = path.resolve('site/assets/images')

function fail(message) {
  console.error(`Screenshot capture failed: ${message}`)
  process.exit(1)
}

if (!baseURL) fail('PHONYG_DOCS_URL is required')
if (!suppliedToken && (!username || !password)) {
  fail('provide PHONYG_DOCS_TOKEN or both PHONYG_DOCS_USERNAME and PHONYG_DOCS_PASSWORD')
}

const health = await fetch(`${baseURL}/api/health`).catch(() => null)
if (!health?.ok) fail('target /api/health is unavailable')

await fs.mkdir(outputDir, { recursive: true })

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({
  viewport: { width: 1600, height: 1050 },
  deviceScaleFactor: 1,
  colorScheme: 'light',
  locale: 'zh-CN',
})
const page = await context.newPage()

await page.addInitScript(() => {
  const style = document.createElement('style')
  style.textContent = '*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}'
  document.documentElement.appendChild(style)
})

page.on('dialog', async (dialog) => dialog.dismiss())

async function login() {
  if (suppliedToken) {
    await page.goto(baseURL, { waitUntil: 'domcontentloaded' })
    await page.evaluate((token) => localStorage.setItem('phonyc_token', token), suppliedToken)
    return
  }
  await page.goto(`${baseURL}/login`, { waitUntil: 'networkidle' })
  await page.locator('input:not([type="password"])').first().fill(username)
  await page.locator('input[type="password"]').fill(password)
  await Promise.all([
    page.waitForURL((url) => !url.pathname.endsWith('/login')),
    page.getByRole('button', { name: '登录' }).click(),
  ])
}

async function prepare(route, pageTitle) {
  await page.goto(`${baseURL}${route}`, { waitUntil: 'networkidle' })
  const heading = page.getByRole('heading', { name: pageTitle }).first()
  await heading.waitFor({ state: 'visible', timeout: 15000 })
  await page.waitForTimeout(350)
}

async function savePage(fileName) {
  const output = path.join(outputDir, fileName)
  await page.screenshot({ path: output, fullPage: true })
  const stat = await fs.stat(output)
  console.log(`${fileName}\t${stat.size} bytes\t${page.url()}`)
}

async function saveViewport(fileName) {
  const output = path.join(outputDir, fileName)
  await page.screenshot({ path: output, fullPage: false })
  const stat = await fs.stat(output)
  console.log(`${fileName}\t${stat.size} bytes\t${page.url()}`)
}

try {
  await login()

  await prepare('/', '概览')
  await savePage('dashboard.png')

  await prepare('/channels', '渠道')
  await savePage('channels.png')

  await prepare('/keys', '用户 Key')
  await savePage('keys.png')

  await prepare('/presets', '客户端预设')
  await savePage('presets.png')
  const enhancedRow = page.locator('tr', { hasText: 'Codex 增强' }).first()
  await enhancedRow.getByRole('button', { name: '编辑' }).click()
  await page.getByText('可视化编辑', { exact: true }).first().waitFor({ state: 'visible' })
  await page.waitForTimeout(250)
  await savePage('preset-editor.png')

  await prepare('/settings', '设置')
  const enhancedToggle = page.getByRole('checkbox', { name: '自动测活增强' })
  if (!(await enhancedToggle.isChecked())) await enhancedToggle.check()
  await page.getByText('增强测活 JSON 词库').waitFor({ state: 'visible' })
  await savePage('healthcheck-enhanced.png')

  await prepare('/capture', '请求捕获')
  await savePage('capture.png')
  const capturedHeading = page.getByText('捕获到的请求头', { exact: true }).first()
  if (await capturedHeading.isVisible()) {
    await capturedHeading.scrollIntoViewIfNeeded()
    await page.waitForTimeout(150)
  }
  await saveViewport('capture-headers.png')

  await prepare('/logs', '请求日志')
  await savePage('logs.png')

  const placeholder = path.join(outputDir, 'capture-client-response.png')
  try {
    await fs.access(placeholder)
  } catch {
    await page.setContent(`<!doctype html><html lang="zh-CN"><style>
      html,body{margin:0;width:1400px;height:820px;background:#f4f1ea;font-family:system-ui,-apple-system,"Segoe UI","PingFang SC",sans-serif;color:#34473b}
      body{display:grid;place-items:center}.window{width:1120px;border:1px solid #cbc6ba;border-radius:18px;background:#fff;box-shadow:0 24px 70px #34473b22;overflow:hidden}
      .bar{padding:15px 20px;background:#34473b;color:#fff;font-weight:700}.body{padding:56px}.label{color:#7d867f;font-size:18px}.response{margin:24px 0;padding:26px;border-radius:13px;background:#eff4ee;border:1px solid #9aad9d;font:700 32px ui-monospace,monospace;color:#3f6048}
      .note{padding-top:24px;border-top:1px solid #e1ddd4;color:#7b827d;font-size:18px;line-height:1.7}.badge{display:inline-block;padding:7px 12px;border-radius:99px;background:#fff1d5;color:#8c6126;font-weight:700}
    </style><body><div class="window"><div class="bar">AI 客户端 · 捕获响应截图占位</div><div class="body"><div class="label">PhonyG 返回：</div><div class="response">captured</div><div class="note"><span class="badge">等待替换</span> 请将用户提供的 AI 客户端成功返回截图覆盖为 <code>site/assets/images/capture-client-response.png</code>。</div></div></div></body></html>`)
    await page.screenshot({ path: placeholder, fullPage: true })
    const stat = await fs.stat(placeholder)
    console.log(`capture-client-response.png\t${stat.size} bytes\tplaceholder`)
  }
} catch (error) {
  let message = error instanceof Error ? error.message : 'unknown browser error'
  for (const secret of [password, suppliedToken]) {
    if (secret) message = message.replaceAll(secret, '[redacted]')
  }
  fail(message)
} finally {
  await context.close()
  await browser.close()
}
