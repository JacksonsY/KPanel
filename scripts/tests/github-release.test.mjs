import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync, mkdtempSync, rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { resolveBash } from '../run-repo-bash.mjs';
import { validateGitHubRun } from '../github-release-l3.mjs';
import { checkEnvironment, loadPolicy } from '../check-environment-policy.mjs';

const valid = { GITHUB_ACTIONS: 'true', RUNNER_ENVIRONMENT: 'github-hosted', GITHUB_REPOSITORY: 'kejilion/KPanel',
  GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '1', GITHUB_SHA: 'a'.repeat(40) };

const workflowStep = (workflow, name) => {
  const step = workflow.split(`      - name: ${name}\n`)[1]?.split('\n      - ')[0];
  assert.ok(step, `step ${name} exists`);
  const body = step.split('        run: |\n')[1];
  assert.ok(body, `step ${name} has a shell body`);
  return body.split('\n').filter(line => line.startsWith('          ')).map(line => line.slice(10)).join('\n');
};

test('release input validation uses the explicit tag when GitHub retains the caller main ref', () => {
  const read = name => readFileSync(new URL('../../' + name, import.meta.url), 'utf8').replaceAll('\r\n', '\n');
  const workflow = read('.github/workflows/release.yml');
  assert.doesNotMatch(workflow, /GITHUB_REF_NAME/);
  assert.match(workflow, /RELEASE_TAG: \$\{\{ inputs\.tag \|\| github\.ref_name \}\}/);
  const root = mkdtempSync(join(tmpdir(), 'kpanel-release-ref-'));
  try {
    const version = read('VERSION').trim();
    const prefix = `git() {
case "$*" in
  'rev-parse HEAD'|"rev-parse $RELEASE_TAG^{commit}") printf '%s' "$RELEASE_SHA";;
  "cat-file -t refs/tags/$RELEASE_TAG") printf tag;;
  'merge-base --is-ancestor HEAD origin/main') return 0;;
  *) printf 'unexpected Git request: %s' "$*" >&2; return 1;;
esac
}\n`;
    const result = spawnSync(resolveBash(), ['--noprofile', '--norc', '-c', prefix + workflowStep(workflow, 'Validate release inputs')], {
      cwd: new URL('../../', import.meta.url), encoding: 'utf8', timeout: 10_000,
      env: { ...process.env, ...valid, GITHUB_REF_NAME: 'main', RELEASE_TAG: 'v' + version, RELEASE_SHA: 'b'.repeat(40),
        DOCKERHUB_IMAGE: 'kjlion/kejilion-panel', DOCKERHUB_USERNAME: 'kjlion', DOCKERHUB_TOKEN: 'fixture',
        RUNNER_TEMP: root, GITHUB_ENV: join(root, 'env'), GITHUB_OUTPUT: join(root, 'outputs') },
    });
    assert.ifError(result.error);
    assert.equal(result.status, 0, result.stderr);
    assert.match(readFileSync(join(root, 'outputs'), 'utf8'), new RegExp(`version=${version.replaceAll('.', '\\.')}`));
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test('reusable CI explicitly exports the frozen identity after runner default injection', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8').replaceAll('\r\n', '\n');
  assert.doesNotMatch(workflow, /^\s+GITHUB_(?:SHA|REF_NAME):/m);
  const result = spawnSync(resolveBash(), ['--noprofile', '--norc', '-c',
    'make() { printf "%s|%s" "$GITHUB_SHA" "$GITHUB_REF_NAME"; }\n' + workflowStep(workflow, 'Run change-aware verification')], {
    encoding: 'utf8', timeout: 10_000,
    env: { ...process.env, GITHUB_SHA: 'a'.repeat(40), GITHUB_REF_NAME: 'main', KPANEL_CI_SHA: 'b'.repeat(40), KPANEL_CI_REF_NAME: 'release/validated-candidate' },
  });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, 'b'.repeat(40) + '|release/validated-candidate');
  const l3 = readFileSync(new URL('../../.github/workflows/release-l3.yml', import.meta.url), 'utf8');
  assert.doesNotMatch(l3, /^\s+GITHUB_(?:SHA|REF_NAME):/m);
  assert.match(l3, /export GITHUB_SHA="\$KPANEL_CANDIDATE_SHA" GITHUB_REF_NAME=release\/validated-candidate/);
});
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
  assert.match(controller, /'ls-tree', '-r', ref/);
  assert.match(controller, /'packaging\/tests'/);
  assert.match(controller, /'Makefile'/);
  assert.match(controller, /'push', '--atomic'/);
  assert.match(controller, /force-with-lease=refs\/heads\/main/);
  assert.match(controller, /before !== process.env.RELEASE_BASE/);
  assert.match(controller, /Preview version must advance/);
  assert.doesNotMatch(controller, /shell:\s*true|console\.log\(.*auth/);
});

test('public acceptance retries cannot rebuild or repush the published image', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8');
  const publicJob = workflow.slice(workflow.indexOf('\n  public-acceptance:'));
  const verifier = readFileSync(new URL('../../.github/workflows/release-public-verify.yml', import.meta.url), 'utf8');
  assert.match(publicJob, /needs: release/);
  assert.match(publicJob, /contents: read/);
  assert.doesNotMatch(publicJob, /build-push-action|DOCKERHUB_TOKEN|contents: write|gh release edit/);
  assert.match(publicJob, /uses: \.\/\.github\/workflows\/release-public-verify\.yml/);
  assert.doesNotMatch(verifier, /build-push-action|DOCKERHUB_TOKEN|contents: write|gh release edit/);
  assert.match(verifier, /workflow_call:/);
  assert.match(verifier, /workflow_dispatch:/);
  assert.match(verifier, /restore-snapshot/);
  assert.ok(workflow.indexOf('Reject an already occupied') < workflow.indexOf('- name: Build and push multi-architecture image'));
});

test('skipped reusable L3 cannot silently skip or satisfy public preview acceptance', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8');
  const publicJob = workflow.slice(workflow.indexOf('\n  public-acceptance:'), workflow.indexOf('\n  preview-completion:'));
  assert.match(publicJob, /if:.*!failure\(\).* !cancelled\(\).*needs\.release\.result == 'success'.*stable == 'false'/);
  const completion = workflow.slice(workflow.indexOf('\n  preview-completion:'));
  assert.match(completion, /needs: \[release, public-acceptance\]/);
  assert.match(completion, /if:.*!failure\(\).* !cancelled\(\).*contains\(inputs\.tag \|\| github\.ref_name, '-rc\.'\)/);
  assert.match(completion, /PUBLIC_RESULT: \$\{\{ needs\.public-acceptance\.result \}\}/);
  assert.match(completion, /run: test "\$PUBLIC_RESULT" = success/);
});

test('read-only recovery verifies the published source tag before executing candidate helpers', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release-public-verify.yml', import.meta.url), 'utf8').replaceAll('\r\n', '\n');
  assert.match(workflow, /fetch-depth: 0/);
  assert.ok(workflow.indexOf('Validate frozen public source') < workflow.indexOf('node scripts/verify-github-public-release.mjs'));
  const version = readFileSync(new URL('../../VERSION', import.meta.url), 'utf8').trim();
  const prefix = `git() {
case "$*" in
  'rev-parse HEAD') printf '%s' "$EXPECTED_SHA";;
  "cat-file -t refs/tags/v$EXPECTED_VERSION") printf tag;;
  "rev-parse refs/tags/v$EXPECTED_VERSION^{commit}")
    if [[ "$WRONG_TAG" = 1 ]]; then printf '%040d' 0; else printf '%s' "$EXPECTED_SHA"; fi;;
  'merge-base --is-ancestor HEAD origin/main') return 0;;
  *) return 1;;
esac
}\n`;
  for (const [ref, wrongTag, expected] of [['refs/heads/main', '0', 0], ['refs/heads/feature/unreviewed', '0', 1], ['refs/heads/main', '1', 1]]) {
    const result = spawnSync(resolveBash(), ['--noprofile', '--norc', '-c', prefix + workflowStep(workflow, 'Validate frozen public source')], {
      cwd: new URL('../../', import.meta.url), encoding: 'utf8', timeout: 10_000,
      env: { ...process.env, GITHUB_EVENT_NAME: 'workflow_dispatch', GITHUB_REF: ref, EXPECTED_VERSION: version,
        EXPECTED_SHA: 'b'.repeat(40), IMAGE_DIGEST: 'sha256:' + 'a'.repeat(64), WRONG_TAG: wrongTag },
    });
    assert.ifError(result.error);
    assert.equal(result.status, expected, result.stderr);
  }
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
