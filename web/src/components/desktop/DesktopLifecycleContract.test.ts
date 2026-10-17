import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(relativePath: string): string {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8')
}

const jobs = source('../../views/JobsView.vue')
const ai = source('../../views/AiView.vue')
const terminal = source('../terminal/HostTerminal.vue')
const taskTerminal = source('../apps/AppInteractiveTerminal.vue')
const controller = source('../../composables/useTerminalController.ts')
const scriptWindow = source('../../views/AppScriptView.vue')

describe('desktop background lifecycle contract', () => {
  it('pauses active-job refreshes while the activity window is inactive', () => {
    expect(jobs).toContain('desktopWindowActiveKey')
    expect(jobs).toContain('!current.signal.aborted && desktopWindowActive.value')
    expect(jobs).toMatch(/watch\(desktopWindowActive,[\s\S]*?else\s*\{\s*controller\?\.abort\(\)\s*if \(timer\) window\.clearTimeout\(timer\)/)
  })

  it('buffers AI stream deltas without scheduling background animation frames', () => {
    expect(ai).toContain('desktopWindowActiveKey')
    expect(ai).toContain('desktopWindowActive.value&&!streamFrame')
    expect(ai).toContain("if(!active){if(streamFrame)cancelAnimationFrame(streamFrame)")
  })

  it('streams terminals in visible windows and pauses hidden ones without closing the session', () => {
    for (const component of [terminal, taskTerminal]) {
      // Visibility, not focus, gates output so side-by-side terminals stay live.
      expect(component).toContain('useTerminalController(')
      expect(component).not.toContain('desktopWindowActive.value')
    }
    expect(controller).toContain('useTerminalActivity()')
    expect(controller).toMatch(/canReceiveOutput\(\): boolean \{\s*return [^}]*streaming\.value && !writeFlow\.blocked/)
    expect(controller).toMatch(/function stopOutput\(\): void \{\s*outputGeneration\+\+[\s\S]*?subscription\?\.close\(\)[\s\S]*?pollController\?\.abort\(\)/)
    expect(controller).toContain('watch(activity.streaming, active => { if (active) startOutput(); else stopOutput() })')
  })

  it('keeps the script terminal mounted when its window loses focus', () => {
    expect(scriptWindow).not.toContain('desktopWindowActiveKey')
    expect(scriptWindow).toMatch(/<AppInteractiveTerminal\s+v-else-if="job"/)
  })
})
