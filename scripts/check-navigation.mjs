import { chromium } from 'playwright'

const baseUrl = (process.env.DOCS_BASE_URL || 'http://127.0.0.1:23342/PhonyC/').replace(/\/$/, '')
const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 1440, height: 700 } })
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
  const clickExpander = (locator) => locator.evaluate((button) => button.click())

  const initialUrl = page.url()
  await page.locator('#site-nav .nav-list-item > .nav-list-link', { hasText: '快速开始' }).first().click()
  await page.waitForTimeout(20)
  assert(page.url() === initialUrl, 'top-level menu title navigated instead of expanding')
  assert(await quickStart.getAttribute('aria-expanded') === 'true', 'top-level menu title did not expand its submenu')
  await clickExpander(quickStart)

  await clickExpander(quickStart)
  await clickExpander(features)
  await clickExpander(reference)
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
  await clickExpander(persistedQuickStart)
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

  await page.goto(`${baseUrl}/features/channels/`, { waitUntil: 'networkidle' })
  const scrollbarStyles = await page.evaluate(() => ({
    body: getComputedStyle(document.body).scrollbarWidth,
    navigation: getComputedStyle(document.querySelector('#site-nav')).scrollbarWidth,
    navigationOverflowX: getComputedStyle(document.querySelector('#site-nav')).overflowX,
  }))
  assert(scrollbarStyles.body === 'none', 'document scrollbar is still visible')
  assert(scrollbarStyles.navigation === 'none', 'navigation scrollbar is still visible')
  assert(scrollbarStyles.navigationOverflowX === 'hidden', 'navigation can still show a horizontal scrollbar')
  const toc = page.locator('.page-toc')
  await toc.waitFor({ state: 'visible' })
  assert(await toc.getByText('本页目录', { exact: true }).count() === 1, 'page table of contents title is missing')
  assert(await toc.locator('.page-toc-link').count() >= 3, 'page table of contents does not include page headings')
  const desktopLayout = await page.evaluate(() => {
    const sidebar = document.querySelector('.side-bar')?.getBoundingClientRect()
    const content = document.querySelector('#main-content')?.getBoundingClientRect()
    const tocBox = document.querySelector('.page-toc')?.getBoundingClientRect()
    return sidebar && content && tocBox
      ? { sidebarRight: sidebar.right, contentLeft: content.left, contentRight: content.right, tocLeft: tocBox.left }
      : null
  })
  assert(desktopLayout, 'three-column layout elements are missing')
  assert(desktopLayout.sidebarRight <= desktopLayout.contentLeft, 'sidebar overlaps the main content')
  assert(desktopLayout.contentRight <= desktopLayout.tocLeft, 'main content overlaps the page table of contents')
  const tocOverflow = await toc.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    innerOverflowY: getComputedStyle(element.querySelector('.page-toc-inner')).overflowY,
    innerOverflowX: getComputedStyle(element.querySelector('.page-toc-inner')).overflowX,
  }))
  assert(tocOverflow.scrollWidth <= tocOverflow.clientWidth, 'page table of contents has horizontal overflow')
  assert(tocOverflow.innerOverflowX === 'visible', 'page table of contents has an internal horizontal scrollbar')
  assert(tocOverflow.innerOverflowY === 'visible', 'page table of contents has an internal vertical scrollbar')

  const tocTopBeforeScroll = await toc.locator('.page-toc-inner').evaluate((element) => element.getBoundingClientRect().top)
  const firstActiveTarget = await toc.locator('.page-toc-link.active').getAttribute('data-target')
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight))
  await page.waitForFunction(() => window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 2)
  const tocTopAfterScroll = await toc.locator('.page-toc-inner').evaluate((element) => element.getBoundingClientRect().top)
  const lastActiveTarget = await toc.locator('.page-toc-link.active').getAttribute('data-target')
  assert(Math.abs(tocTopBeforeScroll - tocTopAfterScroll) < 1, 'page table of contents is not fixed to the viewport')
  assert(firstActiveTarget !== lastActiveTarget, 'page table of contents did not track the current section')

  await page.setViewportSize({ width: 820, height: 700 })
  assert(!(await toc.isVisible()), 'page table of contents should be hidden on narrow screens')

  console.log(JSON.stringify({ baseUrl, expandedTopLevelMenus: 3, starredLinks: 3, threeColumnLayout: true, status: 'passed' }, null, 2))
} finally {
  await browser.close()
}
