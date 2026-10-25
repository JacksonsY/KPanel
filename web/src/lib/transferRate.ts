interface Sample { bytes: number; time: number }
interface TransferSamples { phase: string; samples: Sample[] }

/** Display-only throughput from recent observed bytes; never persists progress. */
export class TransferRates {
  private readonly transfers = new Map<string, TransferSamples>()
  private readonly windowMs = 5_000
  private readonly staleMs = 7_500

  observe(id: string, bytes: number, time: number, phase = 'transfer'): void {
    if (!Number.isFinite(bytes) || bytes < 0 || !Number.isFinite(time)) { this.remove(id); return }
    let transfer = this.transfers.get(id)
    const last = transfer?.samples.at(-1)
    if (!transfer || !last || transfer.phase !== phase || bytes < last.bytes || time < last.time || time - last.time > this.staleMs) {
      transfer = { phase, samples: [] }
      this.transfers.set(id, transfer)
    }
    const samples = transfer.samples
    if (samples.at(-1)?.time === time) samples.pop()
    samples.push({ bytes, time })
    while (samples.length > 2 && samples[1]!.time <= time - this.windowMs) samples.shift()
    if (samples.length > 64) samples.splice(0, samples.length - 64)
  }

  read(id: string, now: number): number | undefined {
    const samples = this.transfers.get(id)?.samples
    const first = samples?.[0], last = samples?.at(-1)
    if (!first || !last || last.time - first.time < 250 || now - last.time > this.staleMs) return undefined
    return Math.max(0, (last.bytes - first.bytes) * 1_000 / (last.time - first.time))
  }

  remove(id: string): void { this.transfers.delete(id) }
  fresh(id: string, now: number): boolean {
    const last = this.transfers.get(id)?.samples.at(-1)
    return Boolean(last && now - last.time <= this.staleMs)
  }
  retain(ids: Set<string>): void {
    for (const id of this.transfers.keys()) if (!ids.has(id)) this.remove(id)
  }
}
