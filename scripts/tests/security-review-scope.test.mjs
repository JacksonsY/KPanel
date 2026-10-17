import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { parseReviewTrailer, reviewScopeState } from '../security-review-scope.mjs';

const script = resolve(fileURLToPath(new URL('../security-review-scope.mjs', import.meta.url)));
const POLICY = {
  schemaVersion: 1,
  ignoreSuffixes: ['_test.go'],
  ignoreSegments: ['testdata'],
  securityReview: { paths: ['internal/auth/', 'internal/cluster/', 'internal/panel/file'] },
};

function fixture() {
  const root = mkdtempSync(join(tmpdir(), 'kpanel-security-review-'));
  const git = (...args) => execFileSync('git', ['-C', root, ...args], { encoding: 'utf8' }).trim();
  const write = (path, text = 'package x\n') => {
    mkdirSync(join(root, path, '..'), { recursive: true });
    writeFileSync(join(root, path), text);
  };
  const commit = (message) => {
    git('add', '-A');
    git('commit', '--allow-empty', '-m', message);
  };
  git('init', '--initial-branch=main');
  git('config', 'user.name', 'KPanel Test');
  git('config', 'user.email', 'kpanel@example.invalid');
  write('.governance/security-audit/boundary-policy.json', JSON.stringify(POLICY));
  commit('test: baseline');
  return { root, git, write, commit, base: git('rev-parse', 'HEAD'), cleanup: () => rmSync(root, { recursive: true, force: true }) };
}

const summary = (state) => state.groups.map((entry) => entry.state + ':' + entry.group).join(' ');

test('trailer parsing requires a scope and finding counts, or a skip reason', () => {
  assert.deepEqual(parseReviewTrailer('v1 scope=internal/auth,internal/store reviewer=claude findings=H0/M1/L2'),
    { skipped: false, scope: ['internal/auth', 'internal/store'], valid: true });
  assert.equal(parseReviewTrailer('v1 scope=internal/auth').valid, false);
  assert.equal(parseReviewTrailer('v1 findings=H0/M0/L0').valid, false);
  assert.deepEqual(parseReviewTrailer('skipped scope=internal/auth reason=rename-only'),
    { skipped: true, scope: ['internal/auth'], valid: true });
  assert.equal(parseReviewTrailer('skipped reason=rename-only').valid, false);
  assert.equal(parseReviewTrailer('skipped scope=internal/auth').valid, false);
});

test('only security-sensitive non-test paths create review groups', () => {
  const state = fixture();
  try {
    state.write('internal/auth/service_test.go');
    state.write('internal/cluster/testdata/peer.json', '{}');
    state.write('internal/panel/theme.go');
    state.write('docs/notes.md', 'notes\n');
    state.commit('chore: unrelated paths');
    assert.equal(reviewScopeState(state.root, state.base).overall, 'not-applicable');

    state.write('internal/panel/files.go');
    state.commit('feat: file handler');
    const result = reviewScopeState(state.root, state.base);
    assert.equal(result.overall, 'missing');
    assert.equal(summary(result), 'missing:internal/panel');
  } finally {
    state.cleanup();
  }
});

test('a covering trailer records a group until the group changes again', () => {
  const state = fixture();
  try {
    state.write('internal/auth/service.go');
    state.write('internal/cluster/relay/v1.go');
    state.commit('feat: auth and relay');
    state.commit('review: auth only\n\nSecurity-Review: v1 scope=internal/auth reviewer=claude findings=H0/M1/L0');
    let result = reviewScopeState(state.root, state.base);
    assert.equal(summary(result), 'recorded:internal/auth missing:internal/cluster/relay');
    assert.equal(result.overall, 'missing');

    state.commit('review: relay\n\nSecurity-Review: v1 scope=internal/cluster reviewer=claude findings=H0/M0/L0');
    result = reviewScopeState(state.root, state.base);
    assert.equal(result.overall, 'recorded');

    state.write('internal/auth/service.go', 'package auth\n// changed\n');
    state.commit('fix: auth follow-up');
    result = reviewScopeState(state.root, state.base);
    assert.equal(summary(result), 'stale:internal/auth recorded:internal/cluster/relay');
    assert.equal(result.overall, 'stale');
  } finally {
    state.cleanup();
  }
});

test('a skip covers only the groups it names, and a malformed trailer covers nothing', () => {
  const state = fixture();
  try {
    state.write('internal/cluster/service.go');
    state.commit('feat: cluster without review');
    state.write('internal/auth/service.go');
    state.commit('feat: auth\n\nSecurity-Review: skipped reason=comment-only');
    let result = reviewScopeState(state.root, state.base);
    assert.equal(summary(result), 'missing:internal/auth missing:internal/cluster');
    assert.equal(result.invalidTrailer, true);

    state.commit('docs: skip auth\n\nSecurity-Review: skipped scope=internal/auth reason=comment-only');
    result = reviewScopeState(state.root, state.base);
    assert.equal(summary(result), 'recorded:internal/auth missing:internal/cluster');
  } finally {
    state.cleanup();
  }
});

test('a trailer on a sibling branch does not cover an unreviewed merged change', () => {
  const state = fixture();
  try {
    state.git('checkout', '-q', '-b', 'unreviewed');
    state.write('internal/auth/a.go');
    state.commit('feat: unreviewed auth change');
    state.git('checkout', '-q', 'main');
    state.git('checkout', '-q', '-b', 'reviewed');
    state.write('internal/auth/b.go');
    state.commit('feat: reviewed auth change\n\nSecurity-Review: v1 scope=internal/auth reviewer=claude findings=H0/M0/L0');
    state.git('checkout', '-q', 'main');
    state.git('merge', '-q', '--no-ff', '-m', 'merge unreviewed', 'unreviewed');
    state.git('merge', '-q', '--no-ff', '-m', 'merge reviewed', 'reviewed');
    assert.equal(summary(reviewScopeState(state.root, state.base)), 'stale:internal/auth');
  } finally {
    state.cleanup();
  }
});

test('a merge counts as a touch only for its own conflict resolution', () => {
  const state = fixture();
  const review = 'Security-Review: v1 scope=internal/auth reviewer=claude findings=H0/M0/L0';
  try {
    state.write('internal/auth/a.go', 'package auth\n// base\n');
    state.commit('feat: auth base\n\n' + review);
    const base = state.git('rev-parse', 'HEAD');
    state.git('checkout', '-q', '-b', 'side');
    state.write('internal/auth/a.go', 'package auth\n// side\n');
    state.commit('feat: side\n\n' + review);
    state.git('checkout', '-q', 'main');
    state.write('internal/auth/b.go', 'package auth\n// main\n');
    state.commit('feat: main\n\n' + review);
    state.git('merge', '-q', '--no-ff', '-m', 'merge side cleanly', 'side');
    assert.equal(reviewScopeState(state.root, base).overall, 'recorded');

    state.git('checkout', '-q', '-b', 'other', base);
    state.write('internal/auth/a.go', 'package auth\n// other\n');
    state.commit('feat: other\n\n' + review);
    state.git('checkout', '-q', 'main');
    spawnSync('git', ['-C', state.root, 'merge', '-q', '--no-ff', 'other'], { encoding: 'utf8' });
    state.write('internal/auth/a.go', 'package auth\n// resolved by hand\n');
    state.commit('merge other with a hand resolution');
    assert.equal(summary(reviewScopeState(state.root, base)), 'stale:internal/auth');
  } finally {
    state.cleanup();
  }
});

test('file-level directories need the file named, so a later candidate cannot cover an earlier one', () => {
  const state = fixture();
  try {
    state.write('.governance/security-audit/boundary-policy.json', JSON.stringify({
      ...POLICY, securityReview: { paths: ['internal/panel/'], fileLevelDirs: ['internal/panel'] },
    }));
    state.commit('test: file-level policy');
    const base = state.git('rev-parse', 'HEAD');
    state.write('internal/panel/files.go');
    state.commit('feat: candidate A, never reviewed');
    state.write('internal/panel/passkey.go');
    state.commit('feat: candidate B\n\nSecurity-Review: v1 scope=internal/panel reviewer=claude findings=H0/M0/L0');
    assert.equal(summary(reviewScopeState(state.root, base)),
      'missing:internal/panel/files.go missing:internal/panel/passkey.go');

    state.commit('review: B\n\nSecurity-Review: v1 scope=internal/panel/passkey.go reviewer=claude findings=H0/M1/L0');
    assert.equal(summary(reviewScopeState(state.root, base)),
      'missing:internal/panel/files.go recorded:internal/panel/passkey.go');

    state.commit('docs: skip A\n\nSecurity-Review: skipped scope=internal/panel/files.go reason=route-registration-only');
    assert.equal(reviewScopeState(state.root, base).overall, 'recorded');
  } finally {
    state.cleanup();
  }
});

test('the CLI reports groups and fails closed only on unusable input', () => {
  const state = fixture();
  try {
    state.write('internal/cluster/service.go');
    state.commit('feat: cluster');
    let result = spawnSync(process.execPath, [script, '--repo', state.root, '--base', state.base], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /^security_review=missing groups=1 pending=internal\/cluster\nmissing\tinternal\/cluster\n$/);

    state.write('.governance/security-audit/boundary-policy.json', JSON.stringify({ schemaVersion: 1 }));
    state.commit('test: policy without review paths');
    result = spawnSync(process.execPath, [script, '--repo', state.root, '--base', state.base], { encoding: 'utf8' });
    assert.equal(result.status, 2);
    assert.match(result.stderr, /securityReview\.paths must be a non-empty list/);
  } finally {
    state.cleanup();
  }
});
