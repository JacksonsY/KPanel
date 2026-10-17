#!/usr/bin/env node
// PROJECT_RULES.md 5.4 security review scope: which security-sensitive package groups a range touched and
// whether a Security-Review trailer covers each touch. Advisory only; it never blocks a candidate.
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
  const fileLevelDirs = review.fileLevelDirs ?? [];
  if (!Array.isArray(fileLevelDirs) || fileLevelDirs.some((entry) => typeof entry !== 'string' || entry === '')) {
    throw new Error(POLICY_PATH + ': securityReview.fileLevelDirs must be a list of directories');
  }
  return {
    paths: review.paths,
    fileLevelDirs,
    ignoreSuffixes: policy.ignoreSuffixes ?? [],
    ignoreSegments: policy.ignoreSegments ?? [],
  };
}

function fileLevel(policy, directory) {
  return policy.fileLevelDirs.some((entry) => directory === entry || directory.startsWith(entry + '/'));
}

// Large directories are reviewed file by file, so each file is its own group there; elsewhere the directory is.
export function reviewGroup(policy, path) {
  if (!policy.paths.some((prefix) => path.startsWith(prefix))) return null;
  if (policy.ignoreSuffixes.some((suffix) => path.endsWith(suffix))) return null;
  if (path.split('/').some((segment) => policy.ignoreSegments.includes(segment))) return null;
  const directory = dirname(path).replaceAll('\\', '/');
  return fileLevel(policy, directory) ? path : directory;
}

// Both forms name the groups they cover with scope=a,b: a review adds raw finding counts,
// "skipped" adds a reason. A skip never covers groups it does not name.
export function parseReviewTrailer(value) {
  const text = value.trim();
  const skipped = /^skipped(?:\s|$)/.test(text);
  const scope = text.match(/(?:^|\s)scope=(\S+)/)?.[1]?.split(',').filter(Boolean) ?? [];
  const detail = skipped ? /(?:^|\s)reason=\S/.test(text) : /(?:^|\s)findings=H\d+\/M\d+\/L\d+(?:\s|$)/.test(text);
  return { skipped, scope, valid: scope.length > 0 && detail };
}

// A file-level group is covered only by naming the file: a directory scope there would claim files nobody exported.
function covers(policy, trailer, group) {
  if (fileLevel(policy, dirname(group))) return trailer.scope.includes(group);
  return trailer.scope.some((entry) => group === entry || group.startsWith(entry + '/'));
}

export function reviewScopeState(root, baseRef, headRef = 'HEAD') {
  const policy = loadReviewPolicy(root);
  // --cc lists, for a merge, only paths that differ from every parent, so a merge counts as a touch
  // only for its own conflict resolution; the merged commits carry their changes themselves.
  const log = git(root, ['log', '--no-renames', '--cc', '--name-only',
    '--format=%x1e%H%x1f%(trailers:key=Security-Review,valueonly,separator=%x1d)%x1f', baseRef + '..' + headRef]);
  const touches = new Map();
  const trailers = [];
  for (const record of log.split('\x1e').slice(1)) {
    const [hash, values, files] = record.split('\x1f');
    for (const path of (files ?? '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean)) {
      const group = reviewGroup(policy, path);
      if (!group) continue;
      if (!touches.has(group)) touches.set(group, new Set());
      touches.get(group).add(hash);
    }
    for (const value of values.split('\x1d').map((entry) => entry.trim()).filter(Boolean)) {
      trailers.push({ hash, ...parseReviewTrailer(value) });
    }
  }
  // A touch is covered only by a trailer on the touch itself or on a descendant of it, never by a trailer
  // that merely sorts later on a sibling branch.
  const reachable = new Map();
  const ancestors = (hash) => {
    if (!reachable.has(hash)) {
      reachable.set(hash, new Set(git(root, ['rev-list', baseRef + '..' + hash]).split(/\r?\n/).filter(Boolean)));
    }
    return reachable.get(hash);
  };
  const result = [];
  for (const [group, commits] of [...touches].sort(([a], [b]) => a.localeCompare(b))) {
    const relevant = trailers.filter((trailer) => trailer.valid && covers(policy, trailer, group));
    let covered = 0;
    for (const commit of commits) {
      if (relevant.some((trailer) => ancestors(trailer.hash).has(commit))) covered += 1;
    }
    result.push({ group, state: covered === commits.size ? 'recorded' : covered === 0 ? 'missing' : 'stale' });
  }
  const overall = result.length === 0 ? 'not-applicable'
    : result.some((entry) => entry.state === 'missing') ? 'missing'
      : result.some((entry) => entry.state === 'stale') ? 'stale' : 'recorded';
  return { overall, groups: result, invalidTrailer: trailers.some((trailer) => !trailer.valid) };
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
  if (!options.base) throw new Error('--base is required');
  if (!['text', 'json'].includes(options.format)) throw new Error('--format must be text or json');
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
