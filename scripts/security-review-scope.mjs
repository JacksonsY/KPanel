#!/usr/bin/env node
// PROJECT_RULES.md 5.4 security review scope: which security-sensitive package groups a range touched and
// whether a later Security-Review trailer covers each one. Advisory only; it never blocks a candidate.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const POLICY_PATH = '.governance/security-audit/boundary-policy.json';

function git(root, args) {
  return execFileSync('git', ['-C', root, ...args], { encoding: 'utf8', maxBuffer: 64 << 20 });
}

export function loadReviewPolicy(root) {
  const policy = JSON.parse(readFileSync(resolve(root, POLICY_PATH), 'utf8'));
  const review = policy.securityReview;
  if (!review || !Array.isArray(review.paths) || review.paths.length === 0
    || review.paths.some((entry) => typeof entry !== 'string' || entry === '' || entry.startsWith('/'))) {
    throw new Error(POLICY_PATH + ': securityReview.paths must be a non-empty list of repository path prefixes');
  }
  return {
    paths: review.paths,
    ignoreSuffixes: policy.ignoreSuffixes ?? [],
    ignoreSegments: policy.ignoreSegments ?? [],
  };
}

export function reviewGroup(policy, path) {
  if (!policy.paths.some((prefix) => path.startsWith(prefix))) return null;
  if (policy.ignoreSuffixes.some((suffix) => path.endsWith(suffix))) return null;
  if (path.split('/').some((segment) => policy.ignoreSegments.includes(segment))) return null;
  return dirname(path).replaceAll('\\', '/');
}

// "Security-Review: skipped reason=<why>" covers every group; otherwise scope=a,b lists package groups.
export function parseReviewTrailer(value) {
  const text = value.trim();
  if (/^skipped(?:\s|$)/.test(text)) return { skipped: true, scope: [], valid: /\sreason=\S/.test(text) };
  const scope = text.match(/(?:^|\s)scope=(\S+)/)?.[1]?.split(',').filter(Boolean) ?? [];
  const findings = /(?:^|\s)findings=H\d+\/M\d+\/L\d+(?:\s|$)/.test(text);
  return { skipped: false, scope, valid: scope.length > 0 && findings };
}

function covers(trailer, group) {
  return trailer.skipped || trailer.scope.some((entry) => group === entry || group.startsWith(entry + '/'));
}

export function reviewScopeState(root, baseRef, headRef = 'HEAD') {
  const policy = loadReviewPolicy(root);
  const range = baseRef + '..' + headRef;
  // Newest first; each commit carries its touched review groups and its Security-Review trailers.
  const commits = [];
  const log = git(root, ['log', '--topo-order', '--no-renames', '--name-only',
    '--format=%x1e%H%x1f%(trailers:key=Security-Review,valueonly,separator=%x1d)%x1f', range]);
  for (const record of log.split('\x1e').slice(1)) {
    const [hash, trailers, files] = record.split('\x1f');
    const groups = new Set();
    for (const path of (files ?? '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean)) {
      const group = reviewGroup(policy, path);
      if (group) groups.add(group);
    }
    commits.push({
      hash,
      groups,
      trailers: trailers.split('\x1d').map((value) => value.trim()).filter(Boolean).map(parseReviewTrailer),
    });
  }
  const groups = new Map();
  commits.forEach((commit, index) => {
    for (const group of commit.groups) if (!groups.has(group)) groups.set(group, index);
  });
  const result = [];
  for (const [group, newestTouch] of [...groups].sort(([a], [b]) => a.localeCompare(b))) {
    let state = 'missing';
    for (let index = 0; index < commits.length; index += 1) {
      if (!commits[index].trailers.some((trailer) => trailer.valid && covers(trailer, group))) continue;
      state = index <= newestTouch ? 'recorded' : 'stale';
      break;
    }
    result.push({ group, state });
  }
  const invalid = commits.some((commit) => commit.trailers.some((trailer) => !trailer.valid));
  const overall = result.length === 0 ? 'not-applicable'
    : result.some((entry) => entry.state === 'missing') ? 'missing'
      : result.some((entry) => entry.state === 'stale') ? 'stale' : 'recorded';
  return { overall, groups: result, invalidTrailer: invalid };
}

export function formatState(state) {
  const pending = state.groups.filter((entry) => entry.state !== 'recorded').map((entry) => entry.group);
  return state.overall
    + ' groups=' + state.groups.length
    + (pending.length > 0 ? ' pending=' + pending.join(',') : '')
    + (state.invalidTrailer ? ' invalid_trailer=true' : '');
}

function usage() {
  return 'Usage: node scripts/security-review-scope.mjs --base <ref> [--head <ref>] [--repo <dir>] [--format=text|json]';
}

function main(argv) {
  const options = { repo: process.cwd(), head: 'HEAD', format: 'text' };
  for (let index = 0; index < argv.length; index += 1) {
    const [flag, inline] = argv[index].split(/=(.*)/s, 2);
    const value = () => inline ?? argv[++index];
    if (flag === '--base') options.base = value();
    else if (flag === '--head') options.head = value();
    else if (flag === '--repo') options.repo = value();
    else if (flag === '--format') options.format = value();
    else if (flag === '--help') return process.stdout.write(usage() + '\n');
    else throw new Error('unknown argument ' + argv[index]);
  }
  if (!options.base || !['text', 'json'].includes(options.format)) throw new Error('--base is required');
  const root = git(options.repo, ['rev-parse', '--show-toplevel']).trim();
  const state = reviewScopeState(root, options.base, options.head);
  if (options.format === 'json') process.stdout.write(JSON.stringify(state, null, 2) + '\n');
  else {
    process.stdout.write('security_review=' + formatState(state) + '\n');
    for (const entry of state.groups) process.stdout.write(entry.state + '\t' + entry.group + '\n');
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    process.stderr.write('Security review scope check failed: ' + error.message + '\n' + usage() + '\n');
    process.exitCode = 2;
  }
}
