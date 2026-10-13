// In-memory UI fixture only: no durable ACK, disk recovery or production storage.
import { createHash, randomBytes } from 'node:crypto'

const CHUNK = 8 << 20
const FILE_LIMIT = 16 << 20
const TOTAL_LIMIT = 32 << 20
const digest = (data) => createHash('sha256').update(data).digest('hex')
const validKey = (value) => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value)

export function createGalleryReceiver({ find, publish, readBody, delayMs = 0, failChunks = 0, observe = () => {} }) {
  const sessions = new Map()
  const delay = Math.min(2000, Math.max(0, Number(delayMs) || 0))
  let failures = Math.min(3, Math.max(0, Number(failChunks) || 0))
  const snapshot = (record) => ({ ...record.session })
  return async (request, response, url, send, readJSON) => {
    if (url.pathname !== '/api/v1/files/transfer/sessions') return false
    const problem = (status, title) => send(response, status, { status, title, code: 'mock_receive_error' })
    try {
      const now = Date.now()
      for (const [id, record] of sessions) if (Date.parse(record.session.expiresAt) <= now) sessions.delete(id)
      const body = request.method === 'POST' ? await readJSON(request) : Object.fromEntries(url.searchParams)
      if (request.method === 'POST' && body.operation === 'create') {
        const input = body.input
        if (!input || input.kind !== 'file' || !validKey(input.sourceKey) || !Number.isSafeInteger(input.sizeBytes)
          || input.sizeBytes < 0 || input.sizeBytes > FILE_LIMIT || typeof input.name !== 'string'
          || !input.name.trim() || /[/\\]/.test(input.name) || ['.', '..'].includes(input.name)
          || find(input.directory)?.entry.kind !== 'directory' || !/^\/home\/gallery(?:\/|$)/.test(input.directory)
          || !Number.isFinite(Date.parse(input.modifiedAt))) { problem(422, '模拟上传参数无效或文件超过 16 MiB'); return true }
        const reserved = [...sessions.values()].reduce((sum, record) => sum + record.input.sizeBytes, 0)
        if (sessions.size >= 16 || reserved + input.sizeBytes > TOTAL_LIMIT) { problem(429, '模拟上传队列已满'); return true }
        const id = randomBytes(32).toString('hex')
        const session = { id, state: 'receiving', offset: 0, sizeBytes: input.sizeBytes, prefixSha256: digest(Buffer.alloc(0)), chunkBytes: CHUNK, expiresAt: new Date(now + 3600_000).toISOString() }
        sessions.set(id, { input: { ...input }, session, data: Buffer.alloc(0) })
        observe({ operation: 'create', id, sizeBytes: input.sizeBytes, modifiedAt: input.modifiedAt })
        send(response, 200, { ...session }); return true
      }
      const record = sessions.get(body.id)
      if (!record || body.sourceKey !== record.input.sourceKey) { problem(404, '模拟接收会话不存在'); return true }
      if (request.method === 'GET') {
        observe({ operation: 'status', id: body.id, offset: record.session.offset })
        send(response, 200, snapshot(record)); return true
      }
      if (request.method === 'POST' && body.operation === 'abort') {
        sessions.delete(body.id); observe({ operation: 'abort', id: body.id }); send(response, 200, snapshot(record)); return true
      }
      if (request.method === 'PUT') {
        if (failures > 0) { failures--; request.resume(); problem(503, '模拟网络暂时不可用，请重试'); return true }
        const data = await readBody(request, CHUNK)
        if (delay) await new Promise((resolve) => setTimeout(resolve, delay))
        if (request.aborted || response.destroyed || sessions.get(body.id) !== record) return true
        const offset = Number(body.offset)
        if (record.session.state !== 'receiving' || !Number.isSafeInteger(offset) || offset < 0 || !data.length
          || offset > record.session.offset || offset + data.length > record.input.sizeBytes || digest(data) !== body.sha256) { problem(409, '模拟上传分块校验失败'); return true }
        if (offset < record.session.offset) {
          if (offset + data.length > record.session.offset || !record.data.subarray(offset, offset + data.length).equals(data)) { problem(409, '模拟重传内容不一致'); return true }
        } else {
          record.data = Buffer.concat([record.data, data])
          record.session.offset = record.data.length
          record.session.prefixSha256 = digest(record.data)
        }
        observe({ operation: 'chunk', id: body.id, offset, acknowledgedOffset: record.session.offset })
        send(response, 200, snapshot(record)); return true
      }
      if (request.method === 'POST' && body.operation === 'commit') {
        if (body.sizeBytes !== record.input.sizeBytes || record.session.offset !== body.sizeBytes || body.sha256 !== record.session.prefixSha256) { problem(409, '模拟上传未完整接收'); return true }
        if (record.session.state !== 'complete') {
          const path = `${record.input.directory}/${record.input.name}`
          if (find(path) && !record.input.overwrite) { problem(409, '模拟目标文件已存在'); return true }
          record.session.entry = publish(path, record.data, record.input.modifiedAt)
          record.session.state = 'complete'
        }
        observe({ operation: 'commit', id: body.id }); send(response, 200, snapshot(record)); return true
      }
      problem(405, '模拟接收操作不支持'); return true
    } catch { problem(422, '模拟上传请求无效'); return true }
  }
}
