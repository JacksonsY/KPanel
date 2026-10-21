import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync, writeFileSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
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

test('preview recovery inputs are preserved before publication and bind original snapshot bytes', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8');
  const start = workflow.indexOf('      - name: Preserve frozen preview recovery inputs before publication');
  assert.ok(start > workflow.indexOf('run: node scripts/verify-github-public-release.mjs snapshot'));
  assert.ok(start < workflow.indexOf('      - name: Build and push multi-architecture image'));
  const upload = workflow.slice(start, workflow.indexOf('\n      - ', start + 1));
  assert.match(upload, /if: steps.snapshot.outcome == 'success'/);
  assert.match(upload, /if-no-files-found: error/);
  assert.match(upload, /snapshot-\$\{\{ github.run_id \}\}-\$\{\{ github.run_attempt \}\}/);
  assert.match(upload, /stable-before.json/);
  assert.match(upload, /publication-identity.json/);
  assert.doesNotMatch(upload, /continue-on-error/);
  const root = mkdtempSync(join(tmpdir(), 'kpanel-frozen-snapshot-')).replaceAll('\\', '/');
  try {
    const moduleUrl = new URL('../verify-github-public-release.mjs', import.meta.url).href;
    const script = `import {createHash} from 'node:crypto';
const digest = body => 'sha256:'+createHash('sha256').update(JSON.stringify(body)).digest('hex');
const image = {config:{digest:'sha256:'+'c'.repeat(64)}};
const index = {manifests:[{platform:{os:'linux',architecture:'amd64'},digest:digest(image)}]};
globalThis.fetch = async url => {
  if(url.includes('auth.docker.io')) return Response.json({token:'fixture'});
  if(url.includes('/releases/latest')) return Response.json({tag_name:'v1.26.0'});
  if(url.includes('/blobs/')) return Response.json({config:{Labels:{'org.opencontainers.image.version':'1.27.0-rc.4'}}});
  const body = url.endsWith(digest(image)) ? image : index;
  return new Response(JSON.stringify(body),{headers:{'docker-content-digest':digest(body)}});
};process.argv[2]='snapshot';await import(${JSON.stringify(moduleUrl)});`;
    const result = spawnSync(process.execPath, ['--input-type=module', '-e', script], {
      encoding: 'utf8', timeout: 5000, env: { ...process.env, ...valid, EXPECTED_VERSION: '1.27.0-rc.5',
        EXPECTED_SHA: 'b'.repeat(40), RUNNER_TEMP: root, PUBLIC_EVIDENCE: root + '/snapshot',
        GITHUB_OUTPUT: root + '/outputs', GITHUB_ENV: root + '/env' },
    });
    assert.ifError(result.error);
    assert.equal(result.status, 0, result.stderr);
    const raw = readFileSync(root + '/snapshot/stable-before.json');
    const identity = JSON.parse(readFileSync(root + '/snapshot/publication-identity.json'));
    assert.equal(identity.candidate, 'b'.repeat(40));
    assert.equal(identity.controllerSha, valid.GITHUB_SHA);
    assert.equal(identity.version, '1.27.0-rc.5');
    assert.equal(identity.runId, valid.GITHUB_RUN_ID);
    assert.equal(identity.attempt, valid.GITHUB_RUN_ATTEMPT);
    assert.equal(identity.snapshotSha256, createHash('sha256').update(raw).digest('hex'));
    assert.equal(JSON.parse(raw).previousPreview.version, '1.27.0-rc.4');
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

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
test('CI reads real Git trailers and only raises the verification floor', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8').replaceAll('\r\n', '\n');
  const body = workflowStep(workflow, 'Run change-aware verification');
  for (const [trailers, expected] of [
    ['', 'auto'], ['Verification-Level: L0', 'auto'], ['Verification-Level: L1', 'auto'],
    ['Verification-Level: L2', '2'], ['Verification-Level: L3', '2'],
    ['verification-level: L2', '2'], ['Verification-Level: garbage', null],
    ['Verification-Level: L2\nVerification-Level: L1', null],
    ['Verification-Level: L2\nVerification-Level: L2', null],
    ['Verification-Level: $(echo injected)', null], ['Verification-Level:', null],
  ]) {
    const result = spawnSync(resolveBash(), ['--noprofile', '--norc', '-c',
      'set -euo pipefail\nunset VERIFY_LEVEL\ngit() { if [[ "$1" == log ]]; then printf "%s" "$TEST_COMMIT_MESSAGE"; else command git "$@"; fi; }\n' +
      'make() { printf "%s" "${VERIFY_LEVEL:-auto}"; }\n' + body], {
      encoding: 'utf8', timeout: 10_000,
      env: { ...process.env, TEST_COMMIT_MESSAGE: 'Candidate\n\n' + trailers + '\n',
        KPANEL_CI_SHA: 'b'.repeat(40), KPANEL_CI_REF_NAME: 'feature/candidate' },
    });
    assert.ifError(result.error);
    if (expected === null) {
      assert.notEqual(result.status, 0, trailers);
      assert.equal(result.stdout, '', trailers);
    } else {
      assert.equal(result.status, 0, result.stderr);
      assert.equal(result.stdout, expected, trailers);
    }
  }
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
  assert.match(flow, /l3:\s+needs: inputs/);
  assert.match(flow, /promote:\s+needs: \[inputs, candidate-ci, l3\]/);
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
  const publicJob = workflow.slice(workflow.indexOf('\n  public-acceptance:'), workflow.indexOf('\n  preview-completion:'));
  const verifier = readFileSync(new URL('../../.github/workflows/release-public-verify.yml', import.meta.url), 'utf8');
  assert.match(publicJob, /needs: release/);
  assert.match(publicJob, /contents: read/);
  assert.doesNotMatch(publicJob, /build-push-action|DOCKERHUB_TOKEN|contents: write|gh release edit/);
  assert.match(publicJob, /uses: \.\/\.github\/workflows\/release-public-verify\.yml/);
  assert.doesNotMatch(verifier, /build-push-action|DOCKERHUB_TOKEN|contents: write|gh release edit/);
  assert.match(verifier, /workflow_call:/);
  assert.match(verifier, /workflow_dispatch:/);
  assert.match(verifier, /restore-snapshot/);
  assert.match(verifier, /actions\/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333/);
  assert.match(verifier, /publication-run-id:/);
  assert.match(verifier, /publication-attempt:/);
  assert.doesNotMatch(verifier.split('  workflow_dispatch:')[1].split('\npermissions:')[0], /stable-snapshot:/);
  assert.ok(workflow.indexOf('Reject an already occupied') < workflow.indexOf('- name: Build and push multi-architecture image'));
});

test('public acceptance runs for both channels and stable archival waits for its success', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8');
  const publicJob = workflow.slice(workflow.indexOf('\n  public-acceptance:'), workflow.indexOf('\n  preview-completion:'));
  assert.match(publicJob, /if:.*!failure\(\).* !cancelled\(\).*needs\.release\.result == 'success'/);
  assert.doesNotMatch(publicJob, /stable == 'false'/);
  const completion = workflow.slice(workflow.indexOf('\n  preview-completion:'));
  assert.match(completion, /needs: \[release, public-acceptance\]/);
  assert.match(completion, /if:.*!failure\(\).* !cancelled\(\).*contains\(inputs\.tag \|\| github\.ref_name, '-rc\.'\)/);
  assert.match(completion, /PUBLIC_RESULT: \$\{\{ needs\.public-acceptance\.result \}\}/);
  assert.match(completion, /run: test "\$PUBLIC_RESULT" = success/);
  const stableCompletion = workflow.slice(workflow.indexOf('\n  stable-completion:'));
  assert.match(stableCompletion, /needs: \[release, public-acceptance\]/);
  assert.match(stableCompletion, /needs\.public-acceptance\.result == 'success'.*needs\.release\.outputs\.stable == 'true'/);
  assert.ok(workflow.indexOf('\n  public-acceptance:') < workflow.indexOf('name: Archive published candidate branch'));
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
        EXPECTED_SHA: 'b'.repeat(40), IMAGE_DIGEST: 'sha256:' + 'a'.repeat(64), PUBLICATION_RUN_ID: '123',
        PUBLICATION_ATTEMPT: '1', WRONG_TAG: wrongTag },
    });
    assert.ifError(result.error);
    assert.equal(result.status, expected, result.stderr);
  }
});

test('public recovery binds only the trusted controller runner recipe and rejects changed product identities', () => {
  const workflow = readFileSync(new URL('../../.github/workflows/release-public-verify.yml', import.meta.url), 'utf8').replaceAll('\r\n', '\n');
  const body = workflowStep(workflow, 'Bind controller-owned test runner recipe');
  assert.ok(workflow.indexOf('Verify public release assets') < workflow.indexOf('Bind controller-owned'));
  assert.ok(workflow.indexOf('Bind controller-owned') < workflow.indexOf('Verify public release in real browsers'));
  for (const scenario of ['updated-recipe', 'untrusted-controller', 'dirty-product', 'wrong-head', 'wrong-version', 'missing-controller']) {
    const root = mkdtempSync(join(tmpdir(), 'kpanel-recipe-'));
    const git = (...args) => {
      const r = spawnSync('git', args, { cwd: root, encoding: 'utf8', timeout: 10_000 });
      assert.equal(r.status, 0, r.stderr);
      return r.stdout.trim();
    };
    try {
      git('init', '-q');
      git('config', 'user.name', 'Fixture'); git('config', 'user.email', 'fixture@example.invalid');
      git('config', 'core.autocrlf', 'false');
      mkdirSync(join(root, 'packaging/release-runner'), { recursive: true });
      const path = 'packaging/release-runner/Dockerfile';
      const original = 'FROM example@sha256:' + 'a'.repeat(64) + '\nRUN old-fixed-recipe\n';
      const recipe = original.replace('old-fixed', 'new-fixed');
      writeFileSync(join(root, path), original); writeFileSync(join(root, 'VERSION'), '1.27.0-rc.5\n');
      git('add', '.'); git('commit', '-qm', 'frozen product');
      const candidate = git('rev-parse', 'HEAD');
      writeFileSync(join(root, path), recipe); git('add', path); git('commit', '-qm', 'test runner only');
      const controller = git('rev-parse', 'HEAD');
      git('update-ref', 'refs/remotes/origin/main', scenario === 'untrusted-controller' ? candidate : controller);
      git('checkout', '--detach', '-q', candidate);
      if (scenario === 'dirty-product') writeFileSync(join(root, 'VERSION'), 'dirty\n');
      const evidence = mkdtempSync(join(tmpdir(), 'kpanel-recipe-evidence-'));
      try {
        const r = spawnSync(resolveBash(), ['--noprofile', '--norc', '-c', body], { cwd: root, encoding: 'utf8', timeout: 15_000,
          env: { ...process.env, GITHUB_SHA: scenario === 'missing-controller' ? '0'.repeat(40) : controller,
            EXPECTED_SHA: scenario === 'wrong-head' ? controller : candidate,
            EXPECTED_VERSION: scenario === 'wrong-version' ? '1.27.0-rc.4' : '1.27.0-rc.5',
            GITHUB_RUN_ID: '123', GITHUB_RUN_ATTEMPT: '1', PUBLIC_EVIDENCE: evidence } });
        assert.ifError(r.error);
        assert.equal(r.status === 0, scenario === 'updated-recipe', `${scenario}: ${r.stderr}`);
        assert.equal(git('rev-parse', 'HEAD'), candidate);
        if (scenario === 'updated-recipe') {
          const receipt = JSON.parse(readFileSync(join(evidence, 'test-runner-recipe.json')));
          assert.equal(receipt.candidate, candidate); assert.equal(receipt.controller, controller);
          assert.equal(receipt.original, original); assert.equal(receipt.recipe, recipe);
          assert.equal(receipt.originalSha256, createHash('sha256').update(original).digest('hex'));
          assert.equal(receipt.recipeSha256, createHash('sha256').update(recipe).digest('hex'));
          assert.match(receipt.diff, /-RUN old-fixed-recipe/); assert.match(receipt.diff, /\+RUN new-fixed-recipe/);
          assert.equal(git('diff', '--name-only'), path);
          assert.equal(readFileSync(join(root, 'VERSION'), 'utf8'), '1.27.0-rc.5\n');
        } else assert.equal(readFileSync(join(root, path), 'utf8'), original);
      } finally { rmSync(evidence, { recursive: true, force: true }); }
    } finally { rmSync(root, { recursive: true, force: true }); }
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
