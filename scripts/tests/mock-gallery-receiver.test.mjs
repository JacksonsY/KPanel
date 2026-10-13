import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { test } from 'node:test'
import { createGalleryReceiver } from '../mock-gallery-receiver.mjs'

const hash = (value) => createHash('sha256').update(value).digest('hex')
function fixture(options = {}) {
  const files = new Map([['/home/gallery', { entry: { kind: 'directory' } }]])
  const events = []
  const handler = createGalleryReceiver({
    find: (path) => files.get(path),
    publish: (path, data, modifiedAt) => {
      const entry = { path, kind: 'file', sizeBytes: data.length, modifiedAt }
      files.set(path, { entry, data }); return entry
    },
    readBody: async (req, limit) => { assert.ok(req.data.length <= limit); return req.data },
    observe: (event) => events.push(event), ...options,
  })
  const call = async (method, body = {}, data = '') => {
    const url = new URL('http://127.0.0.1/api/v1/files/transfer/sessions')
    if (method !== 'POST') for (const [key, value] of Object.entries(body)) url.searchParams.set(key, value)
    const response = { destroyed: false }
    await handler({ method, body, data: Buffer.from(data), resume() {} }, response, url,
      (_res, status, result) => Object.assign(response, { status, result }), async (req) => req.body)
    return response
  }
  const create = (changes = {}) => call('POST', { operation: 'create', input: {
    directory: '/home/gallery', name: 'photo.png', kind: 'file', sizeBytes: 6, sourceKey: 'b'.repeat(64),
    modifiedAt: '2020-01-01T00:00:00.000Z', overwrite: false, ...changes,
  } })
  return { call, create, files, events }
}

test('receives, reports a prefix, replays identical chunks and commits exactly once', async () => {
  const f = fixture()
  const { result: created } = await f.create()
  const key = { id: created.id, sourceKey: 'b'.repeat(64) }
  assert.equal(created.chunkBytes, 8 << 20)
  assert.equal((await f.call('PUT', { ...key, offset: 0, sha256: hash('abc') }, 'abc')).result.offset, 3)
  assert.equal(f.files.has('/home/gallery/photo.png'), false)
  assert.equal((await f.call('GET', key)).result.prefixSha256, hash('abc'))
  assert.equal((await f.call('PUT', { ...key, offset: 0, sha256: hash('abc') }, 'abc')).result.offset, 3)
  assert.equal((await f.call('PUT', { ...key, offset: 0, sha256: hash('bad') }, 'bad')).status, 409)
  assert.equal((await f.call('GET', { ...key, sourceKey: 'c'.repeat(64) })).status, 404)
  assert.equal((await f.call('POST', { ...key, operation: 'commit', sizeBytes: 6, sha256: hash('abcdef') })).status, 409)
  await f.call('PUT', { ...key, offset: 3, sha256: hash('def') }, 'def')
  for (let i = 0; i < 2; i++) assert.equal((await f.call('POST', { ...key, operation: 'commit', sizeBytes: 6, sha256: hash('abcdef') })).result.state, 'complete')
  assert.equal(f.files.get('/home/gallery/photo.png').data.toString(), 'abcdef')
  assert.equal(f.files.get('/home/gallery/photo.png').entry.modifiedAt, '2020-01-01T00:00:00.000Z')
})

test('bounded transient failures do not advance a prefix and abort removes the session', async () => {
  const f = fixture({ failChunks: 3 })
  const { result: created } = await f.create()
  const key = { id: created.id, sourceKey: 'b'.repeat(64) }
  for (let i = 0; i < 3; i++) assert.equal((await f.call('PUT', { ...key, offset: 0, sha256: hash('abc') }, 'abc')).status, 503)
  assert.equal((await f.call('GET', key)).result.offset, 0)
  assert.equal((await f.call('PUT', { ...key, offset: 0, sha256: hash('abc') }, 'abc')).result.offset, 3)
  await f.call('POST', { ...key, operation: 'abort' })
  assert.equal((await f.call('GET', key)).status, 404)
  assert.equal(f.files.has('/home/gallery/photo.png'), false)
})

test('rejects oversized files, traversal and aggregate/session budget exhaustion', async () => {
  const f = fixture()
  assert.equal((await f.create({ sizeBytes: (16 << 20) + 1 })).status, 422)
  assert.equal((await f.create({ name: '../escape.png' })).status, 422)
  assert.equal((await f.create({ sizeBytes: 16 << 20 })).status, 200)
  assert.equal((await f.create({ sizeBytes: 16 << 20 })).status, 200)
  assert.equal((await f.create({ sizeBytes: 1 })).status, 429)
  const empty = fixture()
  for (let i = 0; i < 16; i++) assert.equal((await empty.create({ sizeBytes: 0 })).status, 200)
  assert.equal((await empty.create({ sizeBytes: 0 })).status, 429)
})

test('cancelled delayed chunks cannot recreate an aborted session', async () => {
  const f = fixture({ delayMs: 30 })
  const { result: created } = await f.create()
  const key = { id: created.id, sourceKey: 'b'.repeat(64) }
  const pending = f.call('PUT', { ...key, offset: 0, sha256: hash('abc') }, 'abc')
  await f.call('POST', { ...key, operation: 'abort' })
  assert.equal((await pending).status, undefined)
  assert.equal((await f.call('GET', key)).status, 404)
})
