import { describe, expect, it } from 'vitest'
import { TransferRates } from './transferRate'

describe('recent observed transfer rate', () => {
  it('starts at the observed offset and expires stale readings', () => {
    const rates = new TransferRates()
    rates.observe('job', 1 << 30, 0)
    expect(rates.read('job', 0)).toBeUndefined()
    rates.observe('job', (1 << 30) + 2048, 1000)
    expect(rates.read('job', 1000)).toBe(2048)
    expect(rates.read('job', 9000)).toBeUndefined()
    rates.observe('job', (1 << 30) + 8192, 9000)
    expect(rates.read('job', 9000)).toBeUndefined()
  })

  it('shows zero for fresh stalled counters and resets on retry or phase changes', () => {
    const rates = new TransferRates()
    rates.observe('job', 0, 0)
    rates.observe('job', 2048, 1000)
    rates.observe('job', 2048, 6000)
    expect(rates.read('job', 6000)).toBe(0)
    rates.observe('job', 0, 7000)
    expect(rates.read('job', 7000)).toBeUndefined()
    rates.observe('job', 1024, 8000, 'publish')
    expect(rates.read('job', 8000)).toBeUndefined()
  })

  it('keeps concurrent jobs separate and forgets completed or removed tasks', () => {
    const rates = new TransferRates()
    for (const id of ['a', 'b']) rates.observe(id, 0, 0)
    rates.observe('a', 1000, 1000)
    rates.observe('b', 2000, 1000)
    expect(rates.read('a', 1000)).toBe(1000)
    expect(rates.read('b', 1000)).toBe(2000)
    rates.retain(new Set(['b']))
    expect(rates.read('a', 1000)).toBeUndefined()
    rates.observe('b', Number.NaN, 2000)
    expect(rates.read('b', 2000)).toBeUndefined()
  })
})
