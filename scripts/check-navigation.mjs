import { chromium } from 'playwright'

const baseUrl = (process.env.DOCS_BASE_URL || 'http://127.0.0.1:23342/PhonyC/').replace(/\/$/, '')
const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
const page = await context.newPage()

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

try {
  await page.goto(`${baseUrl}/`, { waitUntil: 'networkidle' })
  await page.evaluate(() => localStorage.removeItem('phonyg-docs:expanded-navigation'))
  await page.reload({ waitUntil: 'networkidle' })

  const quickStart = page.getByRole('button', { name: '快速开始 submenu' })
  const features = page.getByRole('button', { name: '功能指南 submenu' })
  const reference = page.getByRole('button', { name: '配置参考 submenu' })

  await quickStart.click()
  await features.click()
  await reference.click()
  await page.waitForTimeout(20)
  assert(await page.locator('#site-nav > ul > li.nav-list-item.active').count() >= 3, 'multiple top-level menus are not expanded together')
  assert(await quickStart.getAttribute('aria-expanded') === 'true', 'quick start did not remain expanded')
  assert(await features.getAttribute('aria-expanded') === 'true', 'features did not remain expanded')
  assert(await reference.getAttribute('aria-expanded') === 'true', 'reference did not remain expanded')

  await page.goto(`${baseUrl}/features/`, { waitUntil: 'networkidle' })
  await page.waitForTimeout(20)
  assert(await page.locator('#site-nav > ul > li.nav-list-item.active').count() >= 3, 'expanded menu state did not persist across pages')

  const persistedQuickStart = page.getByRole('button', { name: '快速开始 submenu' })
  const persistedFeatures = page.getByRole('button', { name: '功能指南 submenu' })
  const persistedReference = page.getByRole('button', { name: '配置参考 submenu' })
  await persistedQuickStart.click()
  assert(await persistedQuickStart.getAttribute('aria-expanded') === 'false', 'quick start did not collapse independently')
  assert(await persistedFeatures.getAttribute('aria-expanded') === 'true', 'collapsing quick start changed features')
  assert(await persistedReference.getAttribute('aria-expanded') === 'true', 'collapsing quick start changed reference')

  const starred = page.locator('#site-nav a.nav-list-link').filter({ hasText: '⭐' })
  assert(await starred.count() === 3, 'expected three starred navigation links')
  for (let i = 0; i < await starred.count(); i += 1) {
    const link = starred.nth(i)
    const text = await link.textContent()
    const box = await link.boundingBox()
    const font = await link.evaluate((element) => getComputedStyle(element).fontFamily)
    const starBox = await link.evaluate((element) => {
      const textNode = Array.from(element.childNodes).find((node) => node.nodeType === Node.TEXT_NODE && node.textContent.includes('⭐'))
      if (!textNode) return null
      const index = textNode.textContent.indexOf('⭐')
      const range = document.createRange()
      range.setStart(textNode, index)
      range.setEnd(textNode, index + 1)
      const rect = range.getBoundingClientRect()
      return { width: rect.width, height: rect.height }
    })
    assert(text?.includes('⭐'), `starred title text missing at index ${i}`)
    assert(box && box.width > 0 && box.height > 0, `starred navigation link is not visible at index ${i}`)
    assert(starBox && starBox.width > 0 && starBox.height > 0, `star glyph has no visible layout at index ${i}`)
    assert(/emoji/i.test(font), `emoji font fallback missing at index ${i}: ${font}`)
  }

  console.log(JSON.stringify({ baseUrl, expandedTopLevelMenus: 3, starredLinks: 3, status: 'passed' }, null, 2))
} finally {
  await browser.close()
}
