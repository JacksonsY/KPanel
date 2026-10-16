import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
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
