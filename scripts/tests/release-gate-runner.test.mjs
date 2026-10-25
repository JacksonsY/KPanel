import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const repoRoot = resolve(import.meta.dirname, '..', '..');
const scriptPath = resolve(repoRoot, 'scripts', 'run-release-gate.sh');

test('native VM qualification refuses every invalid environment before effects', () => {
  const source = readFileSync(resolve(repoRoot, 'packaging/tests/virtualmachine/native-host.sh'), 'utf8');
  const boundary = source.indexOf('\n: "${RUNNER_TEMP');
  assert.ok(boundary > 0 && boundary < 1500);
  const prefix = source.slice(0, boundary);
  assert.doesNotMatch(prefix, /\b(?:sudo|docker|mkdir|install)\b/);
  const bash = process.env.KPANEL_TEST_BASH || (process.platform === 'win32' ? 'C:\\Program Files\\Git\\bin\\bash.exe' : 'bash');
  const valid = { ...process.env, GITHUB_ACTIONS: 'true', RUNNER_ENVIRONMENT: 'github-hosted', GITHUB_REPOSITORY: 'kejilion/KPanel', GITHUB_REF: 'refs/heads/main' };
  const probe = env => spawnSync(bash, ['-c', prefix + '\nprintf guard_reached'], { env, encoding: 'utf8' });
  assert.equal(probe(valid).status, 0);
  for (const key of ['GITHUB_ACTIONS', 'RUNNER_ENVIRONMENT', 'GITHUB_REPOSITORY', 'GITHUB_REF']) {
    const rejected = probe({ ...valid, [key]: 'invalid' });
    assert.notEqual(rejected.status, 0, key);
    assert.doesNotMatch(rejected.stdout, /guard_reached/, key);
  }
  const postcondition = source.split('diff -u "$evidence/go-sources-before.sha256" "$evidence/go-sources-after.sha256"')[1]?.split('node --input-type=module')[0];
  assert.ok(postcondition && postcondition.length < 500);
  const wrongHead = spawnSync(bash, ['-c', 'set -euo pipefail; git(){ if [ "$1" = rev-parse ]; then printf wrong-head; fi; };\n' + postcondition + '\nprintf guard_reached'], { env: { ...valid, EXPECTED_CANDIDATE: 'a'.repeat(40) }, encoding: 'utf8' });
  assert.notEqual(wrongHead.status, 0);
  assert.doesNotMatch(wrongHead.stdout, /guard_reached/);
});

test('release gate runner fixes candidate identity and nested Docker wiring', () => {
  const script = readFileSync(scriptPath, 'utf8');
  assert.match(script, /git show-ref --verify --quiet/);
  assert.match(script, /git merge-base --is-ancestor/);
  assert.match(script, /git status --short --untracked-files=all/);
  assert.match(script, /--entrypoint sh/);
  assert.match(script, /-v \/var\/run\/docker\.sock:\/var\/run\/docker\.sock/);
  assert.match(script, /-v "\$repo_root:\$repo_root"/);
  assert.match(script, /-w "\$repo_root"/);
  assert.match(script, /"\$runner_id"/);
  assert.match(script, /HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy/);
  assert.match(script, /proxy_args\+=\(--env "\$proxy_name"\)/);
  assert.match(script, /network_args\+=\(--network host\)/);
  assert.doesNotMatch(script, /echo[^\n]*\$\{!proxy_name/);
  assert.match(script, /make verify-release/);
  assert.match(script, /packaging\/tests\/app-conf-lifecycle\.sh/);
});

test('release gate runner has valid Bash syntax', () => {
  const bash = process.env.KPANEL_TEST_BASH ||
    (process.platform === 'win32' ? 'C:\\Program Files\\Git\\bin\\bash.exe' : 'bash');
  const result = spawnSync(bash, ['-n', 'scripts/run-release-gate.sh'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
});
