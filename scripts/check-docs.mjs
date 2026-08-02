import fs from 'node:fs'
import path from 'node:path'

const root = process.cwd()
const failures = []

function read(relativePath) {
  const absolute = path.join(root, relativePath)
  if (!fs.existsSync(absolute)) {
    failures.push(`missing file: ${relativePath}`)
    return ''
  }
  return fs.readFileSync(absolute, 'utf8')
}

function requireText(relativePath, patterns) {
  const content = read(relativePath)
  for (const [label, pattern] of patterns) {
    if (!pattern.test(content)) failures.push(`${relativePath}: missing ${label}`)
  }
}

const requiredFiles = [
  'site/_config.yml',
  'site/_includes/head_custom.html',
  'site/assets/js/navigation-state.js',
  'site/_sass/custom/custom.scss',
  'site/index.md',
  'site/getting-started.md',
  'site/local-build.md',
  'site/docker.md',
  'site/how-to-use.md',
  'site/features/dashboard.md',
  'site/features/channels.md',
  'site/features/keys.md',
  'site/features/presets.md',
  'site/features/capture.md',
  'site/features/logs.md',
  'site/features/healthcheck.md',
  'site/features/retry.md',
  'site/reference/environment.md',
  'site/reference/api.md',
  'site/reference/operations.md',
  'site/reference/troubleshooting.md',
]
for (const file of requiredFiles) read(file)

requireText('site/_config.yml', [
  ['GitHub Pages URL', /^url:\s*["']?https:\/\/buukow\.github\.io/m],
  ['project baseurl', /^baseurl:\s*["']?\/PhonyC/m],
  ['Just the Docs theme', /just-the-docs/],
  ['remote theme plugin', /jekyll-remote-theme/],
])

requireText('site/_includes/head_custom.html', [
  ['navigation state script include', /navigation-state\.js/],
])

requireText('site/assets/js/navigation-state.js', [
  ['navigation storage key', /phonyg-docs:expanded-navigation/],
  ['delayed state persistence', /setTimeout\(writeState, 0\)/],
  ['ARIA state restoration', /aria-expanded/],
])

requireText('site/_sass/custom/custom.scss', [
  ['emoji font fallback', /Noto Color Emoji/],
  ['emoji presentation', /font-variant-emoji:\s*emoji/],
])

requireText('site/index.md', [
  ['home navigation order', /^nav_order:\s*1$/m],
])

requireText('site/getting-started.md', [
  ['quick start navigation order', /^nav_order:\s*2$/m],
  ['quick start child navigation', /^has_children:\s*true$/m],
  ['local build entry', /\/local-build\//],
  ['Docker entry', /\/docker\//],
  ['how-to-use entry', /\/how-to-use\//],
])

const quickStartChildren = [
  ['site/local-build.md', 1, '/local-build/'],
  ['site/docker.md', 2, '/docker/'],
  ['site/how-to-use.md', 3, '/how-to-use/'],
]
for (const [file, order, permalink] of quickStartChildren) {
  requireText(file, [
    ['quick start parent', /^parent:\s*快速开始$/m],
    [`child navigation order ${order}`, new RegExp(`^nav_order:\\s*${order}$`, 'm')],
    [`permalink ${permalink}`, new RegExp(`^permalink:\\s*${permalink.replaceAll('/', '\\/')}$`, 'm')],
  ])
}

requireText('site/features/index.md', [
  ['features top navigation order', /^nav_order:\s*3$/m],
])

requireText('site/reference/index.md', [
  ['reference top navigation order', /^nav_order:\s*4$/m],
])

requireText('site/how-to-use.md', [
  ['administrator initialization step', /^## 1\. 初始化管理员$/m],
  ['channel creation step', /^## 2\. 创建上游渠道$/m],
  ['user key creation step', /^## 3\. 创建用户 Key$/m],
  ['first request step', /^## 4\. 发出请求$/m],
  ['OpenAI Chat Completions example', /\/v1\/chat\/completions/],
  ['OpenAI Responses example', /\/v1\/responses/],
  ['Anthropic Messages example', /\/v1\/messages/],
])

requireText('site/docker.md', [
  ['full Docker Compose', /^## Docker Compose$/m],
  ['minimal Docker Compose', /^## 最简 Docker Compose$/m],
])
const dockerContent = read('site/docker.md')
for (const removedSection of ['检查状态与日志', '升级', '备份与回滚']) {
  if (new RegExp(`^## ${removedSection}$`, 'm').test(dockerContent)) {
    failures.push(`site/docker.md: removed section still exists: ${removedSection}`)
  }
}

const envPatterns = [
  'PHONYG_ADDR',
  'PHONYG_DATA_DIR',
  'PHONYG_JWT_SECRET',
  'PHONYG_MAX_BODY_BYTES',
  'PHONYG_JWT_TTL_HOURS',
]
for (const file of ['site/docker.md', 'site/reference/environment.md']) {
  requireText(file, envPatterns.map((name) => [name, new RegExp(name)]))
}

requireText('site/features/presets.md', [
  ['starred preset title', /^title:\s*客户端预设 ⭐$/m],
  ['force override', /强制覆盖/],
  ['fill missing', /缺失补全/],
  ['parent inheritance', /继承/],
  ['built-in Codex preset', /codex-(?:tui|enhanced)/i],
  ['built-in Claude preset', /claude-(?:cli|enhanced)/i],
])

requireText('site/features/healthcheck.md', [
  ['starred healthcheck title', /^title:\s*自动测活与增强模式 ⭐$/m],
  ['enhanced healthcheck', /自动测活增强/],
  ['stream-first behavior', /stream-first/i],
  ['non-stream fallback', /fallback/i],
  ['temporary disable recovery', /临时禁用.*恢复|恢复.*临时禁用/s],
])

requireText('site/features/capture.md', [
  ['starred capture title', /^title:\s*请求捕获 ⭐$/m],
  ['capture only', /只捕获不转发|capture-only/],
  ['captured response', /captured/],
  ['re-arm', /重新布防/],
  ['403 unarmed behavior', /403/],
  ['session headers', /Session-Id|Thread-Id|X-Claude-Code-Session-Id/],
  ['filtered authentication headers', /Authorization/],
  ['multi-value limitation', /第一个值|首值/],
  ['zero token log semantics', /Token.*0|零 Token/s],
  ['client response image', /capture-client-response\.png/],
])

const requiredImages = [
  'dashboard.png',
  'channels.png',
  'keys.png',
  'presets.png',
  'preset-editor.png',
  'healthcheck-enhanced.png',
  'capture.png',
  'capture-headers.png',
  'logs.png',
  'capture-client-response.png',
]
for (const image of requiredImages) {
  const relative = `site/assets/images/${image}`
  const absolute = path.join(root, relative)
  if (!fs.existsSync(absolute) || fs.statSync(absolute).size < 100) failures.push(`missing or empty image: ${relative}`)
}

const markdownFiles = []
function walk(directory) {
  if (!fs.existsSync(directory)) return
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) walk(absolute)
    else if (entry.name.endsWith('.md')) markdownFiles.push(absolute)
  }
}
walk(path.join(root, 'site'))
for (const file of markdownFiles) {
  const content = fs.readFileSync(file, 'utf8')
  const relative = path.relative(root, file)
  if (/\]\(\/assets\/images\//.test(content)) failures.push(`${relative}: image URL ignores /PhonyC baseurl`)
  if (/src=["']\/assets\/images\//.test(content)) failures.push(`${relative}: image src ignores /PhonyC baseurl`)
}

if (failures.length) {
  console.error(`Documentation validation failed (${failures.length}):`)
  for (const failure of failures) console.error(`- ${failure}`)
  process.exit(1)
}

console.log(`Documentation validation passed: ${requiredFiles.length} pages, ${requiredImages.length} images`)
