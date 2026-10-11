import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const repoRoot = resolve(import.meta.dirname, '..', '..');
const scriptPath = resolve(repoRoot, 'scripts', 'security-scan.sh');
const ignorePath = resolve(repoRoot, '.trivyignore.yaml');

test('nested Trivy scans receive ambient proxy names without persisting values', () => {
  const script = readFileSync(scriptPath, 'utf8');
  assert.match(script, /HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy/);
  assert.match(script, /proxy_args\+=\(--env "\$proxy_name"\)/);
  assert.match(script, /network_args\+=\(--network host\)/);
  assert.doesNotMatch(script, /--env "\$proxy_name=/);
  assert.doesNotMatch(script, /echo[^\n]*\$\{!proxy_name/);
});

test('the release-runner root exception is narrow, reasoned and time bounded', () => {
  const script = readFileSync(scriptPath, 'utf8');
  const ignore = readFileSync(ignorePath, 'utf8');
  assert.match(script, /--ignorefile \/src\/\.trivyignore\.yaml/);
  assert.match(ignore, /id: AVD-DS-0002/);
  assert.match(ignore, /- packaging\/release-runner\/Dockerfile/);
  assert.match(ignore, /expired_at: 2026-12-21/);
  assert.match(ignore, /host Docker socket/);
  assert.doesNotMatch(ignore, /id:\s*\*/);
});

test('bounded inventory routing keeps all secrets, exact targets, real statuses and original logs', () => {
  const script = readFileSync(scriptPath, 'utf8');
  assert.match(script, /--scanners secret/);
  assert.match(script, /--scanners vuln,misconfig/);
  assert.match(script, /--scanners vuln,secret/);
  assert.match(script, /-v "\$baseline_dir\/\$scope:\/inventory:ro"/);
  assert.match(script, /baseline_root="\$repo_root\/\.codex-tmp"/);
  assert.match(script, /-L "\$baseline_root"/);
  assert.match(script, /mktemp -d "\$baseline_root\/torrent-baseline\.XXXXXXXX"/);
  assert.doesNotMatch(script, /mktemp -d "\$\{TMPDIR/);
  assert.match(script, /torrent-backport-baseline\.mjs" scan-inventories "\$baseline_dir"/);
  assert.match(script, /--format json \\\s+--list-all-pkgs/);
  assert.match(script, /torrent-backport-baseline\.mjs" check-report "\$baseline_dir" "\$scope" "\$report" "\$scanner_status" "\$stderr"/);
  assert.match(script, /set -euo pipefail/);
  assert.match(script, /for scope in production selected standalone/);
  assert.match(script, /scanner_status=\$\?/);
  assert.ok(script.indexOf('cat -- "$report"') < script.indexOf(' check-report '));
  assert.ok(script.indexOf('cat -- "$stderr"') < script.indexOf(' check-report '));
  assert.doesNotMatch(script, /\|\| true|--skip-dirs|--exit-code 0|--ignore-unfixed/);
  const secretScan = script.slice(script.indexOf('docker run --rm'), script.indexOf('      /src') + 10);
  assert.match(secretScan, /--scanners secret/);
  assert.doesNotMatch(secretScan, /--skip|--ignorefile/);
  assert.deepEqual([...script.matchAll(/--skip-files ([^\s]+)/g)].map(m => m[1]), [
    '/src/go.sum', '/src/third_party/torrent/go.mod', '/src/third_party/torrent/go.sum', '/src/third_party/torrent/Dockerfile',
  ]);
  assert.equal([...script.matchAll(/--severity HIGH,CRITICAL/g)].length, 4);
  assert.equal([...script.matchAll(/--exit-code 1/g)].length, 4);
});

test('security scan proxy wiring has valid Bash syntax', () => {
  const bash = process.env.KPANEL_TEST_BASH ||
    (process.platform === 'win32' ? 'C:\\Program Files\\Git\\bin\\bash.exe' : 'bash');
  const result = spawnSync(bash, ['-n', 'scripts/security-scan.sh'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
});
