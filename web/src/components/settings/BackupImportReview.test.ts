// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import BackupImportReview from './BackupImportReview.vue'
import { completeBackupReview, type BackupReviewResult } from '@/lib/backup'

function foreignReview(version = 2): BackupReviewResult {
  return { available: true, review: { version, revision: 'host', modules: [{
    module: 'docker', origin: 'foreign', volumes: [{
      name: '<img src=x onerror=alert(1)>', driver: 'local',
      config: { Name: 'app-data', Driver: 'local', Labels: { purpose: 'database' }, DriverOpts: {} },
    }],
  }] } }
}

describe('foreign backup resource plan', () => {
  it('displays every declared volume and its exact creation configuration as text', () => {
    const wrapper = mount(BackupImportReview, { props: { result: foreignReview(), modules: ['docker'] } })
    expect(wrapper.text()).toContain('恢复所需数据卷')
    expect(wrapper.text()).toContain('<img src=x onerror=alert(1)>')
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.get('.volume-review pre').text()).toContain('"DriverOpts": {}')
    expect(wrapper.get('.volume-review pre').text()).toContain('"purpose": "database"')
    expect(wrapper.text()).toContain('复用同名本地数据卷')
    wrapper.unmount()
  })

  it('requires the volume-aware review format for foreign resources', () => {
    expect(completeBackupReview(foreignReview(1), ['docker'])).toBe(false)
    expect(completeBackupReview(foreignReview(2), ['docker'])).toBe(true)
    expect(completeBackupReview(foreignReview(3), ['docker'])).toBe(false)
    const local = foreignReview(1)
    local.review!.modules[0]!.origin = 'local'
    expect(completeBackupReview(local, ['docker'])).toBe(true)
  })
})
