import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { validateGitHubRun } from '../github-release-l3.mjs';
import { checkEnvironment, loadPolicy } from '../check-environment-policy.mjs';

const valid = { GITHUB_ACTIONS: 'true', RUNNER_ENVIRONMENT: 'github-hosted', GITHUB_REPOSITORY: 'kejilion/KPanel',
  GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '1', GITHUB_SHA: 'a'.repeat(40) };
test('L3 rejects self-hosted, fork, missing identity and unsupported platform', () => {
  validateGitHubRun(valid, 'linux');
  for (const [field, value] of [['GITHUB_ACTIONS', 'false'], ['RUNNER_ENVIRONMENT', 'self-hosted'], ['GITHUB_REPOSITORY', 'attacker/KPanel'], ['GITHUB_RUN_ID', ''], ['GITHUB_SHA', 'main']]) {
    assert.throws(() => validateGitHubRun({ ...valid, [field]: value }, 'linux'));
  }
  assert.throws(() => validateGitHubRun(valid, 'win32'));
});
test('GitHub environment never grants production access', () => {
  const policy = loadPolicy();
  assert.equal(checkEnvironment(policy, 'github-hosted', 'browser-validation').role, 'validation');
  for (const purpose of ['production-deploy', 'production-safety-check', 'staging-deploy']) {
    assert.throws(() => checkEnvironment(policy, 'github-hosted', purpose));
  }
});
test('preview pipeline binds trusted workflows, atomic Git writes and explicit downstream calls', () => {
  const read = path => readFileSync(new URL('../../' + path, import.meta.url), 'utf8');
  const flow = read('.github/workflows/preview-release.yml');
  assert.match(flow, /needs: \[inputs, l3\]/);
  assert.match(flow, /needs: \[inputs, main-ci\]/);
  for (const entry of ['ci.yml', 'release-l3.yml', 'release.yml']) assert.ok(flow.includes(`uses: ./.github/workflows/${entry}`));
  assert.doesNotMatch(flow, /self-hosted|\bssh\b|DOCKERHUB_TOKEN/);
  const controller = read('scripts/github-preview-release.mjs');
  assert.match(controller, /merge-base', '--is-ancestor', 'origin\/main', sha/);
  assert.match(controller, /Version tag already exists/);
  assert.match(controller, /Candidate branch does not match/);
  assert.match(controller, /Release automation must be integrated on main/);
  assert.match(controller, /'push', '--atomic'/);
  assert.match(controller, /force-with-lease=refs\/heads\/main/);
  assert.doesNotMatch(controller, /shell:\s*true|console\.log\(.*auth/);
});

test('version image guard admits only an authoritative missing manifest and rejects occupied/uncertain tags', () => {
  const moduleUrl = new URL('../verify-github-public-release.mjs', import.meta.url).href;
  for (const [status, body, expected] of [
    [404, { errors: [{ code: 'MANIFEST_UNKNOWN' }] }, 0],
    [200, { manifests: [] }, 1],
    [503, {}, 1],
    [404, { errors: [{ code: 'NAME_UNKNOWN' }] }, 1],
  ]) {
    const script = `globalThis.fetch=async url=>url.includes('auth.docker.io') ? new Response(JSON.stringify({token:'fixture'})) : new Response(${JSON.stringify(JSON.stringify(body))},{status:${status}});process.argv[2]='unused';await import(${JSON.stringify(moduleUrl)});`;
    const result = spawnSync(process.execPath, ['--input-type=module', '-e', script], { encoding: 'utf8', timeout: 5000,
      env: { ...process.env, ...valid, RUNNER_TEMP: tmpdir(), EXPECTED_VERSION: '1.27.0-rc.3', EXPECTED_SHA: 'a'.repeat(40), PUBLIC_EVIDENCE: tmpdir() + '/fixture' } });
    assert.ifError(result.error);
    assert.equal(result.status, expected, `${status}: ${result.stderr}`);
  }
});
