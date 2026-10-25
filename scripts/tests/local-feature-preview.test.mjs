import assert from 'node:assert/strict';
import { test } from 'node:test';
import { handleMockVirtualMachines } from '../mock-virtual-machines.mjs';

test('VM preview rejects malformed actions and remains available', async () => {
  const replies = [];
  const send = (_response, status, body) => replies.push({ status, body });
  const url = new URL('http://127.0.0.1/api/v1/virtual-machines/actions');
  for (const input of [null, { requestId: 'bad-name', action: 'create', name: { toString: null } }]) {
    assert.equal(await handleMockVirtualMachines({ method: 'POST' }, {}, url, { send, readJSON: async () => input }), true);
    assert.equal(replies.at(-1).status, 422);
  }
  await handleMockVirtualMachines({ method: 'GET' }, {}, new URL('http://127.0.0.1/api/v1/virtual-machines'), { send });
  assert.equal(replies.at(-1).status, 200);
  assert.ok(replies.at(-1).body.machines.length > 0);
});

import {
  choosePort,
  parseAffectedJourneys,
  parseArgs,
  validateLoopbackTarget,
  validateKnownOptions,
  validateStartOptions,
} from '../local-feature-preview.mjs';

test('argument parsing keeps the preview contract explicit', () => {
  assert.deepEqual(
    parseArgs(['start', '--scope', 'docker-compose', '--mode', 'mock', '--grade', 'draft']),
    {
      command: 'start',
      options: { scope: 'docker-compose', mode: 'mock', grade: 'draft' },
    },
  );
});

test('argument parsing rejects duplicate and unknown options', () => {
  assert.throws(() => parseArgs(['start', '--scope', 'a', '--scope', 'b']), /duplicate option/);
  assert.throws(() => validateKnownOptions('start', { scope: 'a', token: 'secret' }), /unknown option/);
});

test('integration preview only accepts secret-free loopback targets', () => {
  assert.equal(validateLoopbackTarget('http://127.0.0.1:8866/api'), 'http://127.0.0.1:8866');
  assert.equal(validateLoopbackTarget('http://localhost:8080'), 'http://localhost:8080');
  assert.throws(() => validateLoopbackTarget('https://example.com'), /only accepts loopback/);
  assert.throws(() => validateLoopbackTarget('http://user:secret@127.0.0.1:8080'), /must not contain credentials/);
});

test('acceptance preview requires a clean checkpoint', () => {
  const journeys = '打开页面 => 页面可用 | 执行操作 => 结果可回读 | 制造失败 => 提示可恢复';
  assert.throws(
    () => validateStartOptions({ scope: 'docker-compose', mode: 'mock', grade: 'acceptance', journeys }, { clean: false }),
    /requires a clean checkpoint/,
  );
  assert.deepEqual(
    validateStartOptions({ scope: 'docker-compose', mode: 'mock', grade: 'draft', profile: 'visual', journeys }, { clean: false }),
    {
      scope: 'docker-compose',
      mode: 'mock',
      grade: 'draft',
      profile: 'visual',
      affectedJourneys: [
        '打开页面 => 页面可用',
        '执行操作 => 结果可回读',
        '制造失败 => 提示可恢复',
      ],
    },
  );
});

test('preview journeys and acceptance profile are explicit and bounded', () => {
  assert.deepEqual(
    parseAffectedJourneys('打开页面 => 页面可用 | 执行操作 => 结果可回读 | 制造失败 => 提示可恢复'),
    ['打开页面 => 页面可用', '执行操作 => 结果可回读', '制造失败 => 提示可恢复'],
  );
  assert.throws(() => parseAffectedJourneys('打开页面 => 页面可用'), /3-7 entries/);
  assert.throws(
    () => parseAffectedJourneys('打开页面 | 执行操作 => 结果 | 制造失败 => 提示'),
    /step => expected result/,
  );
  assert.throws(
    () => validateStartOptions({
      scope: 'docker-compose',
      mode: 'mock',
      profile: 'full-matrix',
      journeys: '打开页面 => 页面可用 | 执行操作 => 结果 | 制造失败 => 提示',
    }, { clean: true }),
    /--profile must be/,
  );
  assert.deepEqual(
    validateStartOptions({ scope: 'legacy-draft', mode: 'mock', grade: 'draft' }, { clean: false }),
    {
      scope: 'legacy-draft',
      mode: 'mock',
      grade: 'draft',
      profile: 'interaction',
      affectedJourneys: [],
    },
  );
  assert.throws(
    () => validateStartOptions({ scope: 'legacy-acceptance', mode: 'mock', grade: 'acceptance' }, { clean: true }),
    /acceptance preview requires --journeys/,
  );
});

test('mock and integration options cannot be silently mixed', () => {
  const journeys = '打开页面 => 页面可用 | 执行操作 => 结果可回读 | 制造失败 => 提示可恢复';
  assert.throws(
    () => validateStartOptions({ scope: 'docker-compose', mode: 'mock', 'api-target': 'http://127.0.0.1:8080', journeys }, { clean: true }),
    /only valid in integration mode/,
  );
  assert.throws(
    () => validateStartOptions({ scope: 'docker-compose', mode: 'integration', journeys }, { clean: true }),
    /requires --api-target/,
  );
  assert.throws(
    () => validateStartOptions({ scope: 'docker-compose', mode: 'mock', 'change-origin': 'yes', journeys }, { clean: true }),
    /must be true or false/,
  );
});

test('automatic port selection returns distinct available ports', async () => {
  const first = await choosePort(undefined, 49100);
  const second = await choosePort(undefined, 49100, new Set([first]));
  assert.notEqual(first, second);
});
