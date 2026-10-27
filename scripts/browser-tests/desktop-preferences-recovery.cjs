// Real Chrome / mock API: fail the first GET, then exercise recovery through UI.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const { execFileSync } = require('node:child_process')
const { chromium } = require(process.env.KPANEL_PLAYWRIGHT_MODULE || 'playwright')
const base = new URL(process.env.COMBINED_PREVIEW).origin
assert.equal(new URL(base).hostname, '127.0.0.1')
const candidate = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
assert.equal(candidate, process.env.REPORT_CANDIDATE)
assert.equal(execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim(), '')
const out = process.env.EVIDENCE_DIR
fs.mkdirSync(out, { recursive: true })
const report = { candidate, mode: 'mock-ui', cases: [], errors: [] }
const desktop = { mode: 'desktop', windows: [{ id: 41, path: '/settings', geometry: { left: 80, top: 80, width: 800, height: 600 }, minimized: false, maximized: false, snap: '', z: 101 }], windowSizes: [], sideSplitRatio: .6, focusedId: 41 }

;(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.KPANEL_BROWSER_EXECUTABLE || undefined })
  report.browser = browser.version()
  report.playwright = require((process.env.KPANEL_PLAYWRIGHT_MODULE || 'playwright') + '/package.json').version
  try {
    for (const journey of ['timer', 'online', 'focus', 'user-edit', 'in-flight-edit']) {
      const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
      try {
        await context.addInitScript(() => {
          localStorage.setItem('kejilion-panel-desktop-mode', 'classic')
          localStorage.setItem('kejilion-panel-desktop-windows', '[]')
          localStorage.setItem('kejilion-panel-locale', 'zh-CN')
        })
        const page = await context.newPage()
        page.setDefaultTimeout(15000)
        page.on('pageerror', error => report.errors.push({ journey, message: error.message }))
        let gets = 0, writes = [], release
        const gate = new Promise(resolve => { release = resolve })
        let server = { resourceVersion: 'remote-v1', locale: 'zh-CN', desktop: structuredClone(desktop) }
        await page.route(base + '/api/v1/settings/preferences', async route => {
          if (route.request().method() === 'GET') {
            gets++
            if (gets === 1) return route.fulfill({ status: 503, json: { title: 'temporary preference failure' } })
            await gate
            return route.fulfill({ json: server })
          }
          const patch = route.request().postDataJSON()
          assert.equal(patch.expectedResourceVersion, server.resourceVersion)
          writes.push(patch)
          server = { ...server, ...patch, resourceVersion: `saved-${writes.length}` }
          return route.fulfill({ json: server })
        })
        await page.goto(base, { waitUntil: 'domcontentloaded' })
        await page.locator('.desktop-entry-button').waitFor()
        await page.getByText('无法读取服务端界面状态', { exact: true }).waitFor()
        assert.equal(await page.locator('.desktop-window').count(), 0)
        if (journey === 'user-edit') {
          await page.locator('.desktop-entry-button').click()
          await page.locator('.desktop__icons').waitFor()
        }
        if (journey === 'online' || journey === 'focus') {
          await page.evaluate(event => window.dispatchEvent(new Event(event)), journey)
        }
        await page.waitForFunction(() => performance.getEntriesByType('resource').filter(e => e.name.includes('/settings/preferences')).length >= 1)
        // Wait until the retried request has reached the route handler, without
        // replacing the product's timer or invoking the synchronization module.
        await new Promise((resolve, reject) => {
          const started = Date.now()
          const interval = setInterval(() => {
            if (gets >= 2) { clearInterval(interval); resolve() }
            else if (Date.now() - started > 12000) { clearInterval(interval); reject(new Error('no preference retry')) }
          }, 25)
        })
        if (journey === 'in-flight-edit') {
          await page.locator('.desktop-entry-button').click()
          await page.locator('.desktop__icons').waitFor()
        }
        const edited = journey.endsWith('edit')
        const saved = edited ? page.waitForResponse(response => response.url().endsWith('/settings/preferences') && response.request().method() === 'PUT') : undefined
        release()
        if (edited) {
          await saved
          assert.equal(server.desktop.mode, 'desktop')
          assert.deepEqual(server.desktop.windows, [])
          assert.equal(await page.locator('.desktop-window').count(), 0, 'late GET replaced the user desktop')
          assert(writes.some(patch => patch.desktop?.windows.length === 0))
        } else {
          await page.locator('.desktop-window').waitFor()
          assert.equal(await page.locator('.desktop-entry-button').count(), 0)
          assert.equal(writes.length, 0, 'restoring configured preferences must not write back')
        }
        await page.screenshot({ path: `${out}/${journey}.png` })
        report.cases.push({ journey, gets, writes: writes.length, restoredWindows: await page.locator('.desktop-window').count(), passed: true })
      } finally { await context.close() }
    }
    assert.deepEqual(report.errors, [])
  } catch (error) { report.failure = error.stack; throw error }
  finally {
    await browser.close()
    report.cleanup = 'browser-and-contexts-closed'
    fs.writeFileSync(`${out}/result.json`, JSON.stringify(report, null, 2))
  }
  console.log(JSON.stringify(report))
})().catch(error => { console.error(error); process.exitCode = 1 })
