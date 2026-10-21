import { createHash } from 'node:crypto';
import { appendFileSync, lstatSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const fail = message => { throw new Error(message); };
const sha = /^[a-f0-9]{40}$/;
const digestPattern = /^sha256:[a-f0-9]{64}$/;
const versionPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
const releaseVersionPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-rc\.([1-9]\d*))?$/;
const previewPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-rc\.([1-9]\d*)$/;
const mediaTypes = [
  'application/vnd.oci.image.index.v1+json',
  'application/vnd.oci.image.manifest.v1+json',
  'application/vnd.docker.distribution.manifest.list.v2+json',
  'application/vnd.docker.distribution.manifest.v2+json',
].join(', ');

const stableVersionIsOlder = (left, right) => {
  const leftParts = left.split('.').map(BigInt), rightParts = right.split('.').map(BigInt);
  for (let i = 0; i < leftParts.length; i++) {
    if (leftParts[i] !== rightParts[i]) return leftParts[i] < rightParts[i];
  }
  return false;
};

export function expectedReleaseAssetNames(version) {
  if (!releaseVersionPattern.test(version ?? '')) fail('Valid release version required for the asset inventory');
  return [
    'kejilion-agent-linux-amd64', 'kejilion-agent-linux-arm64',
    'kejilion-node-linux-amd64', 'kejilion-node-linux-arm64',
    'kejilion-panel-meta-' + version + '.tar.gz',
    'kpanel-mcp-darwin-amd64', 'kpanel-mcp-darwin-arm64',
    'kpanel-mcp-linux-amd64', 'kpanel-mcp-linux-arm64',
    'kpanel-mcp-windows-amd64.exe', 'kpanel-mcp-windows-arm64.exe',
    'KPANEL_MODIFIED_COMPONENTS.json', 'LICENSE', 'SHA256SUMS', 'THIRD_PARTY_NOTICES.md',
  ].sort();
}

function validateAssetEntries(assets, version) {
  const names = expectedReleaseAssetNames(version);
  if (!Array.isArray(assets) || assets.length !== names.length || assets.some(asset => !asset || typeof asset !== 'object')) {
    fail('Release asset manifest must contain the exact 15 expected files');
  }
  const sorted = [...assets].sort((left, right) => left.name < right.name ? -1 : left.name > right.name ? 1 : 0);
  for (let i = 0; i < names.length; i++) {
    const asset = sorted[i];
    if (asset?.name !== names[i] || !Number.isSafeInteger(asset.size) || asset.size <= 0 || !digestPattern.test(asset.digest ?? '')) {
      fail('Release asset manifest has a missing, duplicate or invalid asset');
    }
  }
  return sorted;
}

export function createReleaseAssetManifest({ releaseDirectory, expected, controllerSha, snapshotSha256 }) {
  if (!sha.test(controllerSha ?? '') || !/^[a-f0-9]{64}$/.test(snapshotSha256 ?? '')) fail('Frozen controller and snapshot identity required');
  const assets = expectedReleaseAssetNames(expected.version).map(name => {
    const path = join(releaseDirectory, name);
    const stat = lstatSync(path);
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size <= 0 || stat.size > 128 * 1024 * 1024) {
      fail('Invalid local release asset: ' + name);
    }
    const bytes = readFileSync(path);
    return { name, size: bytes.length, digest: 'sha256:' + sha256(bytes) };
  }).sort((left, right) => left.name < right.name ? -1 : left.name > right.name ? 1 : 0);
  validateAssetEntries(assets, expected.version);
  return {
    schemaVersion: 1,
    phase: 'before-channel-promotion',
    candidate: expected.candidate,
    version: expected.version,
    runId: expected.runId,
    attempt: expected.attempt,
    controllerSha,
    imageDigest: expected.digest,
    snapshotSha256,
    assets,
  };
}

export function validateReleaseAssetInventory(release, manifest, { allowPublished = false } = {}) {
  if (!manifest || !release || !releaseVersionPattern.test(manifest.version ?? '')
    || (release.draft !== true && !(allowPublished && release.draft === false))
    || release.tag_name !== 'v' + manifest.version
    || release.prerelease !== manifest.version.includes('-rc.') || !Array.isArray(release.assets)) {
    fail('Original GitHub draft does not match the frozen asset manifest');
  }
  const remote = release.assets.map(asset => {
    if (!asset || typeof asset !== 'object' || asset.state !== 'uploaded') fail('A release asset is not fully uploaded');
    return { name: asset.name, size: asset.size, digest: asset.digest };
  }).sort((left, right) => left.name < right.name ? -1 : left.name > right.name ? 1 : 0);
  const frozen = validateAssetEntries(manifest.assets, manifest.version);
  if (remote.length !== frozen.length || JSON.stringify(remote) !== JSON.stringify(frozen)) {
    fail('GitHub draft assets do not match the original release bytes');
  }
}

export function validateRecoveryInputs(env) {
  const input = {
    operation: env.RECOVERY_OPERATION,
    runId: env.PUBLICATION_RUN_ID,
    attempt: env.PUBLICATION_ATTEMPT,
    candidate: env.EXPECTED_SHA,
    version: env.EXPECTED_VERSION,
    digest: env.IMAGE_DIGEST,
  };
  if (!['publish-draft', 'restore-latest'].includes(input.operation)
    || !/^[1-9]\d*$/.test(input.runId ?? '') || !/^[1-9]\d*$/.test(input.attempt ?? '')
    || !sha.test(input.candidate ?? '') || !versionPattern.test(input.version ?? '')
    || !digestPattern.test(input.digest ?? '') || env.GITHUB_REF !== 'refs/heads/main') {
    fail('Recovery requires main and exact operation/run/attempt/candidate/stable-version/digest inputs');
  }
  return input;
}

function artifactFiles(root) {
  if (!root) fail('Original publication artifacts are required');
  const rootStat = lstatSync(root);
  if (!rootStat.isDirectory() || rootStat.isSymbolicLink()) fail('Artifact root must be a real directory');
  const found = new Map();
  let count = 0;
  let totalBytes = 0;
  const walk = (directory, depth) => {
    if (depth > 8) fail('Artifact directory nesting limit exceeded');
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (++count > 128) fail('Artifact file count limit exceeded');
      const path = join(directory, entry.name);
      if (entry.isSymbolicLink()) fail('Symbolic links are not accepted in publication artifacts');
      if (entry.isDirectory()) { walk(path, depth + 1); continue; }
      if (!entry.isFile()) fail('Unexpected non-file publication artifact entry');
      const stat = lstatSync(path);
      totalBytes += stat.size;
      if (totalBytes > 16 * 1024 * 1024) fail('Publication artifact size limit exceeded');
      const matches = found.get(entry.name) ?? [];
      matches.push(path);
      found.set(entry.name, matches);
    }
  };
  walk(root, 0);
  return name => {
    const paths = found.get(name) ?? [];
    if (paths.length !== 1) fail(`Expected one ${name} in its exact publication artifact`);
    return paths[0];
  };
}

const readUnique = (lookup, name, maxBytes) => {
  const path = lookup(name);
  const stat = lstatSync(path);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size > maxBytes) fail(`Invalid publication artifact file: ${name}`);
  return { path, bytes: readFileSync(path) };
};

export function validatePublicationArtifacts({ snapshotDirectory, publicationDirectory, expected }) {
  // Both uploads must preserve the same frozen bytes; a matching filename alone is not release identity.
  const snapshotFile = artifactFiles(snapshotDirectory);
  const publicationFile = artifactFiles(publicationDirectory);
  const snapshotArtifact = readUnique(snapshotFile, 'stable-before.json', 4096);
  const identityArtifact = readUnique(snapshotFile, 'publication-identity.json', 4096);
  const snapshotManifest = readUnique(snapshotFile, 'release-assets.json', 4096);
  const publishedSnapshot = readUnique(publicationFile, 'stable-before.json', 4096);
  const publishedIdentity = readUnique(publicationFile, 'publication-identity.json', 4096);
  const publishedManifest = readUnique(publicationFile, 'release-assets.json', 64 * 1024);
  const notes = readUnique(publicationFile, 'RELEASE_NOTES.md', 1024 * 1024);
  const components = readUnique(publicationFile, 'KPANEL_MODIFIED_COMPONENTS.json', 8 * 1024 * 1024);
  if (!snapshotArtifact.bytes.equals(publishedSnapshot.bytes)
    || !identityArtifact.bytes.equals(publishedIdentity.bytes)
    || !snapshotManifest.bytes.equals(publishedManifest.bytes)) fail('Snapshot and publication artifacts disagree');
  const identity = JSON.parse(identityArtifact.bytes.toString('utf8'));
  const snapshot = JSON.parse(snapshotArtifact.bytes.toString('utf8'));
  const assetManifest = JSON.parse(publishedManifest.bytes.toString('utf8'));
  if (identity.schemaVersion !== 1 || identity.phase !== 'before-publication'
    || identity.candidate !== expected.candidate || identity.version !== expected.version
    || identity.runId !== expected.runId || identity.attempt !== expected.attempt
    || !sha.test(identity.controllerSha ?? '') || identity.snapshotSha256 !== sha256(snapshotArtifact.bytes)
    || identity.assetManifestSha256 !== sha256(publishedManifest.bytes)
    || identity.notesSha256 !== sha256(notes.bytes)) fail('Original publication artifact identity mismatch');
  const manifestAssets = validateAssetEntries(assetManifest.assets, expected.version);
  if (assetManifest.schemaVersion !== 1 || assetManifest.phase !== 'before-channel-promotion'
    || assetManifest.candidate !== expected.candidate || assetManifest.version !== expected.version
    || assetManifest.runId !== expected.runId || assetManifest.attempt !== expected.attempt
    || assetManifest.controllerSha !== identity.controllerSha || assetManifest.imageDigest !== expected.digest
    || assetManifest.snapshotSha256 !== identity.snapshotSha256) fail('Frozen release asset identity mismatch');
  if (!/^v\d+\.\d+\.\d+$/.test(snapshot.tag ?? '') || !versionPattern.test(snapshot.tag.slice(1))
    || !stableVersionIsOlder(snapshot.tag.slice(1), expected.version)
    || !digestPattern.test(snapshot.digest ?? '') || !digestPattern.test(snapshot.previousPreview?.digest ?? '')) {
    fail('Invalid frozen stable-channel snapshot');
  }
  const expectedTrain = expected.version + '-rc.';
  if (typeof snapshot.previousPreview?.version !== 'string' || !snapshot.previousPreview.version.startsWith(expectedTrain)
    || !previewPattern.test(snapshot.previousPreview.version)) fail('Frozen preview must belong to the exact stable release train');
  if (!notes.bytes.toString('utf8').includes(expected.digest)) fail('Frozen release notes do not name the exact image digest');
  if (/[\r\n]/.test(notes.path)) fail('Release notes path cannot contain line breaks');
  const componentAsset = manifestAssets.find(asset => asset.name === 'KPANEL_MODIFIED_COMPONENTS.json');
  if (!componentAsset || componentAsset.digest !== 'sha256:' + sha256(components.bytes) || componentAsset.size !== components.bytes.length) {
    fail('Component manifest bytes do not match the frozen release asset inventory');
  }
  return { identity, snapshot, assetManifest, notesPath: notes.path, notesSha256: sha256(notes.bytes), componentsSha256: sha256(components.bytes) };
}

export function validateFailedPublication({ run, workflow, jobs, expected, identity }) {
  // Stable releases enter through the annotated tag push; preview dispatch rejects stable versions.
  if (workflow?.path !== '.github/workflows/release.yml' || workflow?.state !== 'active'
    || run?.workflow_id !== workflow.id || run?.path !== '.github/workflows/release.yml'
    || run?.id !== Number(expected.runId) || run?.run_attempt !== Number(expected.attempt)
    || run?.event !== 'push' || run?.status !== 'completed' || run?.conclusion !== 'failure'
    || run?.head_branch !== 'v' + expected.version || run?.head_sha !== expected.candidate
    || identity?.controllerSha !== expected.candidate) {
    fail('Original run is not the exact failed stable-tag release attempt');
  }
  const releaseJobs = (jobs?.jobs ?? []).filter(job => job.name === 'release');
  if (releaseJobs.length !== 1 || releaseJobs[0].conclusion !== 'failure') fail('Original release job did not fail exactly once');
  const steps = releaseJobs[0].steps ?? [];
  const promoted = steps.filter(step => step.name === 'Promote image to its release channel');
  const published = steps.filter(step => step.name === 'Publish GitHub release');
  if (promoted.length !== 1 || promoted[0].conclusion !== 'success'
    || published.length !== 1 || published[0].conclusion !== 'failure') {
    fail('Recovery only supports a successful image promotion followed by a failed GitHub publish step');
  }
}

export function classifyRecoveryState({ operation, snapshot, release, assetManifest, version, expectedDigest, latestReleaseTag,
  latestDigest, versionDigest, previewDigest, componentDigest }) {
  // The release workflow moves Docker latest before publishing GitHub; accept only these bounded state transitions.
  const tag = `v${version ?? ''}`;
  if (!versionPattern.test(version ?? '') || release?.tag_name !== tag || release?.prerelease !== false) {
    fail('Stable draft identity is invalid');
  }
  validateReleaseAssetInventory(release, assetManifest, { allowPublished: true });
  const componentAssets = release.assets.filter(asset => asset.name === 'KPANEL_MODIFIED_COMPONENTS.json');
  if (componentAssets.length !== 1 || componentAssets[0].digest !== `sha256:${componentDigest}`) {
    fail('Draft component manifest does not match the original publication artifact');
  }
  if (versionDigest !== expectedDigest) {
    fail('Immutable version image digest is not bound to the requested publication');
  }
  if (previewDigest !== snapshot.previousPreview.digest) fail('Preview channel changed after the frozen publication snapshot');
  if (operation === 'publish-draft') {
    if (release.draft === true && latestReleaseTag === snapshot.tag && latestDigest === versionDigest) return 'publish';
    if (release.draft === false && latestReleaseTag === tag && latestDigest === versionDigest) return 'already-published';
    fail('Publishing is allowed only from the exact frozen draft state or its completed public state');
  }
  if (operation === 'restore-latest') {
    if (release.draft !== true || latestReleaseTag !== snapshot.tag) fail('Rollback is limited to an unpublished draft stable release');
    if (latestDigest === versionDigest) return 'restore-latest';
    if (latestDigest === snapshot.digest) return 'already-restored';
    fail('Docker latest no longer matches the promoted image or the frozen prior stable digest');
  }
  fail('Unsupported recovery operation');
}

async function boundedResponse(response, limit = 4 * 1024 * 1024) {
  const chunks = [];
  let size = 0;
  for await (const chunk of response.body) {
    size += chunk.length;
    if (size > limit) fail('Remote response size limit exceeded');
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

async function requestJson(url, headers, limit = 4 * 1024 * 1024) {
  const response = await fetch(url, { headers, signal: AbortSignal.timeout(30_000) });
  const bytes = await boundedResponse(response, limit);
  if (!response.ok) fail(`Remote verification failed with HTTP ${response.status}`);
  return JSON.parse(bytes.toString('utf8'));
}

export async function freezeReleaseAssets(env) {
  const expected = {
    candidate: env.EXPECTED_SHA,
    version: env.VERSION,
    digest: env.IMAGE_DIGEST,
    runId: env.GITHUB_RUN_ID,
    attempt: env.GITHUB_RUN_ATTEMPT,
  };
  if (env.GITHUB_ACTIONS !== 'true' || env.GITHUB_REPOSITORY !== 'kejilion/KPanel'
    || !sha.test(expected.candidate ?? '') || !releaseVersionPattern.test(expected.version ?? '')
    || !digestPattern.test(expected.digest ?? '') || !/^[1-9]\d*$/.test(expected.runId ?? '')
    || !/^[1-9]\d*$/.test(expected.attempt ?? '') || !sha.test(env.GITHUB_SHA ?? '')
    || !env.GH_TOKEN || !env.PUBLIC_EVIDENCE || !env.ASSET_DIRECTORY) fail('Exact hosted release publication identity required');
  const snapshotPath = join(env.PUBLIC_EVIDENCE, 'stable-before.json');
  const identityPath = join(env.PUBLIC_EVIDENCE, 'publication-identity.json');
  const snapshotBytes = readFileSync(snapshotPath);
  const identity = JSON.parse(readFileSync(identityPath, 'utf8'));
  const snapshotSha256 = sha256(snapshotBytes);
  if (identity.schemaVersion !== 1 || identity.phase !== 'before-publication'
    || identity.candidate !== expected.candidate || identity.version !== expected.version
    || identity.runId !== expected.runId || identity.attempt !== expected.attempt
    || identity.controllerSha !== env.GITHUB_SHA || identity.snapshotSha256 !== snapshotSha256
    || identity.assetManifestSha256 || identity.notesSha256) fail('Existing publication identity is stale or already bound');
  const notesPath = join(env.ASSET_DIRECTORY, 'RELEASE_NOTES.md');
  const notesStat = lstatSync(notesPath);
  if (!notesStat.isFile() || notesStat.isSymbolicLink() || notesStat.size <= 0 || notesStat.size > 1024 * 1024) {
    fail('Invalid original final release notes');
  }
  const notesBytes = readFileSync(notesPath);
  const manifest = createReleaseAssetManifest({ releaseDirectory: env.ASSET_DIRECTORY, expected,
    controllerSha: env.GITHUB_SHA, snapshotSha256 });
  const manifestBytes = Buffer.from(JSON.stringify(manifest, null, 2) + '\n');
  const api = env.GITHUB_API_URL || 'https://api.github.com';
  const headers = { Accept: 'application/vnd.github+json', Authorization: `Bearer ${env.GH_TOKEN}`,
    'X-GitHub-Api-Version': '2022-11-28' };
  const release = await requestJson(`${api}/repos/${env.GITHUB_REPOSITORY}/releases/tags/v${expected.version}`, headers);
  validateReleaseAssetInventory(release, manifest);
  const manifestPath = join(env.PUBLIC_EVIDENCE, 'release-assets.json');
  writeFileSync(manifestPath, manifestBytes, { flag: 'wx' });
  identity.assetManifestSha256 = sha256(manifestBytes);
  identity.notesSha256 = sha256(notesBytes);
  const pendingIdentity = join(env.PUBLIC_EVIDENCE, 'publication-identity.next.json');
  writeFileSync(pendingIdentity, JSON.stringify(identity, null, 2) + '\n', { flag: 'wx' });
  renameSync(pendingIdentity, identityPath);
  return manifest;
}

async function dockerManifest(ref, registryHeaders) {
  const response = await fetch(`https://registry-1.docker.io/v2/kjlion/kejilion-panel/manifests/${ref}`, {
    headers: registryHeaders, signal: AbortSignal.timeout(30_000),
  });
  const bytes = await boundedResponse(response);
  if (!response.ok) fail(`Docker registry verification failed with HTTP ${response.status}`);
  const digest = `sha256:${sha256(bytes)}`;
  if (response.headers.get('docker-content-digest') !== digest) fail('Docker registry manifest digest mismatch');
  return digest;
}

function validateMainCandidate(expected, env, controllerSha) {
  if (env.GITHUB_ACTIONS !== 'true' || env.GITHUB_REPOSITORY !== 'kejilion/KPanel'
    || env.GITHUB_REF !== 'refs/heads/main' || !sha.test(env.GITHUB_SHA ?? '')) fail('Recovery must run on the trusted main workflow');
  const git = (...args) => execFileSync('git', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout: 20_000 }).trim();
  if (git('rev-parse', 'HEAD') !== env.GITHUB_SHA
    || readFileSync('VERSION', 'utf8').trim() !== expected.version
    || git('cat-file', '-t', `refs/tags/v${expected.version}`) !== 'tag'
    || git('rev-parse', `refs/tags/v${expected.version}^{commit}`) !== expected.candidate) fail('Stable tag or controller source does not match exact recovery inputs');
  execFileSync('git', ['merge-base', '--is-ancestor', expected.candidate, 'origin/main'], { stdio: 'ignore', timeout: 20_000 });
  if (controllerSha !== undefined) {
    if (controllerSha !== expected.candidate) fail('Original stable-tag controller must be the exact candidate');
  }
}

async function inspect(env) {
  // Writer jobs call this again immediately before mutation so their plan reflects current public state.
  const expected = validateRecoveryInputs(env);
  validateMainCandidate(expected, env);
  const repository = env.GITHUB_REPOSITORY;
  const token = env.GH_TOKEN;
  if (!token) fail('GitHub read token is required');
  const githubHeaders = { Accept: 'application/vnd.github+json', Authorization: `Bearer ${token}`,
    'X-GitHub-Api-Version': '2022-11-28' };
  const api = env.GITHUB_API_URL || 'https://api.github.com';
  const [workflow, run, jobs, latestRelease, release, snapshotData] = await Promise.all([
    requestJson(`${api}/repos/${repository}/actions/workflows/release.yml`, githubHeaders),
    requestJson(`${api}/repos/${repository}/actions/runs/${expected.runId}/attempts/${expected.attempt}`, githubHeaders),
    requestJson(`${api}/repos/${repository}/actions/runs/${expected.runId}/attempts/${expected.attempt}/jobs`, githubHeaders),
    requestJson(`${api}/repos/${repository}/releases/latest`, githubHeaders),
    requestJson(`${api}/repos/${repository}/releases/tags/v${expected.version}`, githubHeaders),
    Promise.resolve(validatePublicationArtifacts({ snapshotDirectory: env.SNAPSHOT_ARTIFACT_DIR,
      publicationDirectory: env.PUBLICATION_ARTIFACT_DIR, expected })),
  ]);
  validateMainCandidate(expected, env, snapshotData.identity.controllerSha);
  validateFailedPublication({ run, workflow, jobs, expected: { ...expected, repository }, identity: snapshotData.identity });
  if (release.tag_name !== `v${expected.version}`) {
    fail('GitHub release draft has an invalid tag identity');
  }
  const dockerToken = await requestJson('https://auth.docker.io/token?service=registry.docker.io&scope=repository:kjlion/kejilion-panel:pull', {});
  const registryHeaders = { Authorization: `Bearer ${dockerToken.token}`, Accept: mediaTypes };
  const [latestDigest, versionDigest, previewDigest, previousStableDigest] = await Promise.all([
    dockerManifest('latest', registryHeaders), dockerManifest(expected.version, registryHeaders),
    dockerManifest('preview', registryHeaders), dockerManifest(snapshotData.snapshot.digest, registryHeaders),
  ]);
  if (versionDigest !== expected.digest || previousStableDigest !== snapshotData.snapshot.digest) {
    fail('Immutable version image or frozen previous stable digest no longer matches');
  }
  const plan = classifyRecoveryState({ operation: expected.operation, snapshot: snapshotData.snapshot,
    release, assetManifest: snapshotData.assetManifest, version: expected.version, expectedDigest: expected.digest,
    latestReleaseTag: latestRelease.tag_name, latestDigest, versionDigest, previewDigest,
    componentDigest: snapshotData.componentsSha256 });
  if (env.GITHUB_OUTPUT) {
    appendFileSync(env.GITHUB_OUTPUT, `plan=${plan}\n`);
    appendFileSync(env.GITHUB_OUTPUT, `snapshot=${JSON.stringify(snapshotData.snapshot)}\n`);
    appendFileSync(env.GITHUB_OUTPUT, `notes_path=${snapshotData.notesPath}\n`);
  }
  return { plan, snapshot: snapshotData.snapshot, notesPath: snapshotData.notesPath };
}

if (process.argv[2] === 'freeze-assets') {
  try {
    const manifest = await freezeReleaseAssets(process.env);
    process.stdout.write(`release_assets_frozen=${manifest.assets.length}\n`);
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
} else if (process.argv[2] === 'validate-inputs') {
  try {
    validateMainCandidate(validateRecoveryInputs(process.env), process.env);
    process.stdout.write('stable_recovery_inputs=valid\n');
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
} else if (process.argv[2] === 'inspect') {
  try {
    const result = await inspect(process.env);
    process.stdout.write(`stable_recovery=${result.plan}\n`);
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
} else if (process.argv[2] !== undefined) {
  process.stderr.write('Usage: node scripts/stable-release-recovery.mjs freeze-assets|validate-inputs|inspect\n');
  process.exitCode = 2;
}
