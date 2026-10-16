import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { isAbsolute, relative, resolve } from 'node:path';
import { checkEnvironment, loadPolicy } from './check-environment-policy.mjs';

export function validateGitHubRun(env, platform) {
  if (platform !== 'linux' || env.GITHUB_ACTIONS !== 'true' || env.RUNNER_ENVIRONMENT !== 'github-hosted'
      || env.GITHUB_REPOSITORY !== 'kejilion/KPanel' || !/^\d+$/.test(env.GITHUB_RUN_ID ?? '')
      || !/^\d+$/.test(env.GITHUB_RUN_ATTEMPT ?? '') || !/^[a-f0-9]{40}$/.test(env.GITHUB_SHA ?? '')) {
    throw new Error('L3 requires the canonical repository on a GitHub-hosted Linux runner');
  }
}

export function runGitHubL3() {
  validateGitHubRun(process.env, process.platform);
  checkEnvironment(loadPolicy(), 'github-hosted', 'candidate-validation');
  const repo = resolve(import.meta.dirname, '..');
  const directory = process.env.KPANEL_L3_EVIDENCE;
  if (!directory || !isAbsolute(directory) || !relative(repo, directory).startsWith('..')) {
    throw new Error('KPANEL_L3_EVIDENCE must be outside the checkout');
  }
  mkdirSync(directory, { recursive: false }); // retries require a new run ID / attempt directory
  const command = (program, args) => {
    const result = spawnSync(program, args, { cwd: repo, encoding: 'utf8', shell: false, timeout: 60_000 });
    if (result.status !== 0 || result.error) throw new Error(`${program} failed: ${result.stderr ?? result.error}`);
    return result.stdout.trim();
  };
  const candidate = command('git', ['rev-parse', 'HEAD']);
  const record = { schemaVersion: 1, candidate, environment: 'github-hosted',
    runId: process.env.GITHUB_RUN_ID, attempt: process.env.GITHUB_RUN_ATTEMPT,
    runUrl: `${process.env.GITHUB_SERVER_URL}/kejilion/KPanel/actions/runs/${process.env.GITHUB_RUN_ID}`,
    runner: { image: process.env.ImageOS, version: process.env.ImageVersion },
    tools: { go: command('go', ['version']), node: process.version, docker: command('docker', ['version', '--format', '{{.Server.Version}}']) },
    startedAt: new Date().toISOString(), status: 'failed' };
  try {
    if (candidate !== (process.env.KPANEL_CANDIDATE_SHA || process.env.GITHUB_SHA)) throw new Error('candidate identity mismatch');
    if (command('git', ['status', '--porcelain', '--untracked-files=all'])) throw new Error('candidate worktree must be clean');
    // The unique quality entry retains source, security, scene packs and image checks.
    const result = spawnSync('bash', ['scripts/run-release-gate.sh', '--github-hosted', candidate, 'v1.26.0'],
      { cwd: repo, stdio: 'inherit', shell: false, timeout: 100 * 60_000,
        env: { ...process.env, KPANEL_SOURCE_CHECK_ARTIFACT_DIR: resolve(directory, 'source') } });
    if (result.error || result.status !== 0) throw new Error(`release gate failed: exit=${result.status}`);
    if (command('git', ['rev-parse', 'HEAD']) !== candidate || command('git', ['status', '--porcelain', '--untracked-files=all'])) {
      throw new Error('candidate changed during L3');
    }
    record.status = 'passed';
  } finally {
    record.finishedAt = new Date().toISOString();
    record.entrySha256 = createHash('sha256').update(readFileSync(resolve(repo, 'scripts/run-release-gate.sh'))).digest('hex');
    writeFileSync(resolve(directory, 'receipt.json'), JSON.stringify(record, null, 2) + '\n', { flag: 'wx' });
  }
}
