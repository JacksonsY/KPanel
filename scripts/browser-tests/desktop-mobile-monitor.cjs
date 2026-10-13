const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const { chromium } = require(process.env.KPANEL_PLAYWRIGHT_MODULE || 'playwright')

const url = process.env.COMBINED_PREVIEW
assert.equal(new URL(url).hostname, '127.0.0.1')
const candidate = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
if (process.env.REPORT_CANDIDATE) assert.equal(candidate, process.env.REPORT_CANDIDATE)
const out = process.env.EVIDENCE_DIR
fs.mkdirSync(out, { recursive: true })
const baseline = process.env.MONITOR_BASELINE
const baselineCSS = baseline && execFileSync('git', ['show', `${baseline}:web/src/styles/desktop.css`], { encoding: 'utf8' })
const report = { candidate, baseline: baseline || null, mode: 'mock-ui', zoom: 'effective CSS viewport and device scale emulate browser zoom', cases: [], errors: [] }

;(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.KPANEL_BROWSER_EXECUTABLE || undefined })
  const deadline = setTimeout(() => void browser.close(), 55_000)
  try {
    const cases = baseline ? [[375, 1, 'zh-CN', 'light']] : [
      ...[320, 375, 430].flatMap(width => [1, 1.25, 2].map(zoom => [width, zoom, 'zh-CN', 'light'])),
      ...[1, 1.25, 2].map(zoom => [375, zoom, 'en-US', 'dark']),
      [375, 1, 'zh-CN', 'dark'], [375, 2, 'en-US', 'light'], [1280, 1, 'en-US', 'light'],
    ]
    for (const [width, zoom, locale, theme] of cases) {
      const context = await browser.newContext({ viewport: { width: Math.round(width / zoom), height: Math.round(900 / zoom) }, deviceScaleFactor: zoom })
      const page = await context.newPage()
      page.setDefaultTimeout(7000)
      page.on('pageerror', error => report.errors.push(error.message))
      await context.addInitScript(({ locale, theme }) => {
        localStorage.setItem('kejilion-panel-desktop-mode', 'desktop')
        localStorage.setItem('kejilion-panel-locale', locale)
        localStorage.setItem('kejilion-panel-theme', theme)
      }, { locale, theme })
      let fail = false
      await page.route('**/api/v1/system/summary', async route => {
        if (fail) return route.fulfill({ status: 503, json: { title: 'Mock unavailable' } })
        const response = await route.fetch()
        const data = await response.json()
        data.hostname = 'production-server-with-a-very-long-hostname-0123456789'
        data.os = 'Ubuntu 24.04.3 LTS (Noble Numbat)'
        data.cpu.cores = 192
        data.load = { one: 128.37, five: 125.42, fifteen: 120.36 }
        data.network.receivedBytes = 1023.9 * 1024 ** 4
        data.network.sentBytes = 999.9 * 1024 ** 4
        await route.fulfill({ json: data })
      })
      await page.goto(url)
      await page.locator('.desktop__icons').waitFor()
      const mobile = width / zoom <= 900
      if (mobile) await page.locator('.desktop__pager-dot').first().click()
      const monitor = page.locator(mobile ? '.desktop__widget-page .desktop-monitor' : '.desktop__icons > .desktop-widget-slot .desktop-monitor')
      await monitor.locator('.desktop-monitor__host').waitFor()
      if (mobile) await page.waitForFunction(() => document.querySelector('.desktop__icons').scrollLeft < 1)
      if (baselineCSS) {
        assert.ok(!baselineCSS.includes('.desktop__widget-page .desktop-monitor__list'))
        await page.evaluate(() => {
          const style = [...document.querySelectorAll('style')].find(element => element.textContent.includes('.desktop-monitor__list'))
          if (!style) throw new Error('desktop stylesheet missing')
          style.textContent = style.textContent.replace(/\.desktop__widget-page \.desktop-monitor[^{}]*\{[^{}]*\}/g, '')
        })
      }
      const geometry = await monitor.evaluate(root => {
        const rect = element => { const r = element.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom } }
        const card = rect(root)
        const metrics = [...root.querySelectorAll('.desktop-monitor__metric')].map(rect)
        const issues = []
        for (const element of root.querySelectorAll('dd, .desktop-monitor__network-line > *, .desktop-monitor__host-block strong, .desktop-monitor__host-block small')) {
          const r = rect(element)
          if (r.left < card.left - 1 || r.right > card.right + 1 || r.bottom > card.bottom + 1 || element.scrollWidth > element.clientWidth + 1) issues.push(`overflow: ${element.textContent.trim()}`)
        }
        for (let a = 0; a < metrics.length; a++) for (let b = a + 1; b < metrics.length; b++) {
          const x = metrics[a], y = metrics[b]
          if (x.left < y.right - 1 && y.left < x.right - 1 && x.top < y.bottom - 1 && y.top < x.bottom - 1) issues.push(`metric overlap ${a}/${b}`)
        }
        for (const line of root.querySelectorAll('.desktop-monitor__network-line')) {
          const children = [...line.children].map(rect)
          for (let a = 0; a < children.length; a++) for (let b = a + 1; b < children.length; b++) {
            const x = children[a], y = children[b]
            if (x.left < y.right - 1 && y.left < x.right - 1 && x.top < y.bottom - 1 && y.top < x.bottom - 1) issues.push(`counter overlap ${a}/${b}`)
          }
        }
        const fontSizes = [...root.querySelectorAll('dt, dd, small, strong')].map(element => parseFloat(getComputedStyle(element).fontSize))
        const listStyle = getComputedStyle(root.querySelector('.desktop-monitor__list'))
        return { card, issues, minFontSize: Math.min(...fontSizes), text: root.textContent, rows: listStyle.gridTemplateRows }
      })
      report.cases.push({ width, zoom, locale, theme, mobile, ...geometry })
      if (mobile && zoom === 1) await page.screenshot({ path: path.join(out, `${width}-${locale}-${theme}.png`) })
      // The change is phone-only; desktop geometry is recorded without changing its existing clipping contract.
      if (mobile && !baseline) {
        assert.deepEqual(geometry.issues, [], JSON.stringify(report.cases.at(-1)))
        assert.ok(geometry.minFontSize >= 12)
      }
      if (!mobile) assert.equal(geometry.rows, '37px 37px 37px')
      if (width === 375 && zoom === 1 && locale === 'zh-CN' && theme === 'light' && !baseline) {
        await page.keyboard.press('Tab')
        await page.locator('.desktop__widget-page-edit').focus()
        assert.ok(await page.locator('.desktop__widget-page-edit').evaluate(element => element === document.activeElement && getComputedStyle(element).outlineStyle !== 'none'))
        fail = true
        await page.reload()
        await page.locator('.desktop__pager-dot').first().click()
        await page.waitForFunction(() => !document.querySelector('.desktop__widget-page .desktop-monitor__loading'))
        assert.equal(await page.locator('.desktop__widget-page .desktop-monitor__metric').count(), 6)
        report.failureState = '503 response renders six placeholder metrics without throwing'
      }
      await context.close()
    }
    if (baseline) assert.ok(report.cases.some(item => item.issues.length), 'baseline must reproduce the overflow')
    assert.deepEqual(report.errors, [])
    report.passed = true
  } finally {
    clearTimeout(deadline)
    await browser.close()
    fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2))
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
