import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const sha256 = value => createHash('sha256').update(value).digest('hex');
const env = process.env;
const version = env.EXPECTED_VERSION;
if (!/^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-rc\.[1-9]\d*)?$/.test(version ?? '') || !/^[a-f0-9]{40}$/.test(env.EXPECTED_SHA ?? '')) throw new Error('Exact release identity required');
const isPreview = version.includes('-rc.');
if (env.GITHUB_ACTIONS !== 'true' || env.RUNNER_ENVIRONMENT !== 'github-hosted') throw new Error('GitHub-hosted execution required');
const directory = env.PUBLIC_EVIDENCE;
if (!directory?.startsWith(env.RUNNER_TEMP + '/')) throw new Error('Evidence must be owned runner temporary data');
if (process.argv[2] === 'restore-snapshot') {
  const snapshot = JSON.parse(env.STABLE_SNAPSHOT);
  if (!/^v\d+\.\d+\.\d+$/.test(snapshot.tag) || !/^sha256:[a-f0-9]{64}$/.test(snapshot.digest)
      || !/^\d+\.\d+\.\d+-rc\.[1-9]\d*$/.test(snapshot.previousPreview?.version)
      || !/^sha256:[a-f0-9]{64}$/.test(snapshot.previousPreview?.digest)) throw new Error('Invalid frozen publication snapshot');
  if (!isPreview && snapshot.tag === `v${version}`) throw new Error('Stable recovery requires the original pre-publication baseline');
  mkdirSync(directory, { recursive: false });
  writeFileSync(join(directory, 'stable-before.json'), JSON.stringify(snapshot, null, 2) + '\n', { flag: 'wx' });
  writeFileSync(env.GITHUB_ENV, `PREVIOUS_PREVIEW_VERSION=${snapshot.previousPreview.version}\nPREVIOUS_PREVIEW_IMAGE=docker.io/kjlion/kejilion-panel@${snapshot.previousPreview.digest}\n`, { flag: 'a' });
  process.exit(0);
}
if (process.argv[2] === 'restore-artifact') {
  const runId = env.PUBLICATION_RUN_ID;
  const attempt = env.PUBLICATION_ATTEMPT;
  const artifactDirectory = env.SNAPSHOT_ARTIFACT_DIR;
  if (!/^[1-9]\d*$/.test(runId ?? '') || !/^[1-9]\d*$/.test(attempt ?? '')
      || !artifactDirectory?.startsWith(env.RUNNER_TEMP + '/')) throw new Error('Exact original publication artifact required');
  const snapshotBytes = readFileSync(join(artifactDirectory, 'stable-before.json'));
  const identity = JSON.parse(readFileSync(join(artifactDirectory, 'publication-identity.json'), 'utf8'));
  const snapshot = JSON.parse(snapshotBytes);
  if (identity.schemaVersion !== 1 || identity.phase !== 'before-publication'
      || identity.candidate !== env.EXPECTED_SHA || identity.version !== version
      || identity.runId !== runId || identity.attempt !== attempt
      || !/^[a-f0-9]{40}$/.test(identity.controllerSha ?? '')
      || identity.snapshotSha256 !== sha256(snapshotBytes)) throw new Error('Original publication artifact identity mismatch');
  if (!/^v\d+\.\d+\.\d+$/.test(snapshot.tag) || !/^sha256:[a-f0-9]{64}$/.test(snapshot.digest)
      || !/^\d+\.\d+\.\d+-rc\.[1-9]\d*$/.test(snapshot.previousPreview?.version)
      || !/^sha256:[a-f0-9]{64}$/.test(snapshot.previousPreview?.digest)) throw new Error('Invalid frozen publication snapshot');
  if (!isPreview && snapshot.tag === `v${version}`) throw new Error('Stable recovery requires the original pre-publication baseline');
  mkdirSync(directory, { recursive: false });
  writeFileSync(join(directory, 'stable-before.json'), snapshotBytes, { flag: 'wx' });
  writeFileSync(env.GITHUB_ENV, `PREVIOUS_PREVIEW_VERSION=${snapshot.previousPreview.version}\nPREVIOUS_PREVIEW_IMAGE=docker.io/kjlion/kejilion-panel@${snapshot.previousPreview.digest}\n`, { flag: 'a' });
  process.exit(0);
}
const request = async (url, headers = {}, limit = 64 * 1024 * 1024) => {
  const response = await fetch(url, { headers, signal: AbortSignal.timeout(180_000) });
  if (!response.ok) throw new Error(`HTTP ${response.status} for ${url}`);
  const chunks = []; let size = 0;
  for await (const chunk of response.body) { size += chunk.length; if (size > limit) throw new Error('Response limit exceeded'); chunks.push(chunk); }
  return { bytes: Buffer.concat(chunks), headers: response.headers };
};
const json = async (url, headers) => JSON.parse((await request(url, headers, 4 * 1024 * 1024)).bytes);
const token = (await json('https://auth.docker.io/token?service=registry.docker.io&scope=repository:kjlion/kejilion-panel:pull')).token;
const headers = { Authorization: `Bearer ${token}`, Accept: 'application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json' };
const registry = 'https://registry-1.docker.io/v2/kjlion/kejilion-panel';
if (process.argv[2] === 'unused') {
  const response = await fetch(`${registry}/manifests/${version}`, { headers, signal: AbortSignal.timeout(30_000) });
  if (response.status !== 404) throw new Error(`Refusing version image publication: registry HTTP ${response.status}; existing or uncertain tags cannot be overwritten`);
  const body = await response.json();
  if (!Array.isArray(body.errors) || body.errors.length !== 1 || body.errors[0].code !== 'MANIFEST_UNKNOWN') throw new Error('Registry did not establish an unused version');
  console.log(`version_image=unused version=${version}`);
  process.exit(0);
}
const manifest = async ref => {
  const r = await request(`${registry}/manifests/${ref}`, headers, 4 * 1024 * 1024);
  const digest = 'sha256:' + sha256(r.bytes);
  if (r.headers.get('docker-content-digest') !== digest || (ref.startsWith('sha256:') && ref !== digest)) throw new Error('Registry manifest integrity mismatch');
  return { digest, body: JSON.parse(r.bytes) };
};
const latestRelease = await json('https://api.github.com/repos/kejilion/KPanel/releases/latest');
const latestImage = await manifest('latest');
const snapshotPath = join(directory, 'stable-before.json');
if (process.argv[2] === 'snapshot') {
  if (!isPreview && latestRelease.tag_name === `v${version}`) throw new Error('Stable version already occupies Latest');
  mkdirSync(directory, { recursive: false });
  const previous = await manifest('preview');
  const native = previous.body.manifests.find(m => m.platform?.os === 'linux' && m.platform?.architecture === 'amd64');
  if (!native) throw new Error('Previous preview lacks the required upgrade platform');
  const previousManifest = await manifest(native.digest);
  const config = await json(`${registry}/blobs/${previousManifest.body.config.digest}`, headers);
  const previousVersion = config.config.Labels['org.opencontainers.image.version'];
  if (!/^\d+\.\d+\.\d+-rc\.[1-9]\d*$/.test(previousVersion) || previousVersion === version) throw new Error('Valid distinct previous preview required');
  writeFileSync(snapshotPath, JSON.stringify({ tag: latestRelease.tag_name, digest: latestImage.digest,
    previousPreview: { version: previousVersion, digest: previous.digest } }, null, 2) + '\n', { flag: 'wx' });
  writeFileSync(join(directory, 'publication-identity.json'), JSON.stringify({ schemaVersion: 1,
    candidate: env.EXPECTED_SHA, version, controllerSha: env.GITHUB_SHA,
    runId: env.GITHUB_RUN_ID, attempt: env.GITHUB_RUN_ATTEMPT,
    snapshotSha256: sha256(readFileSync(snapshotPath)), phase: 'before-publication',
  }, null, 2) + '\n', { flag: 'wx' });
  writeFileSync(env.GITHUB_OUTPUT, `snapshot=${JSON.stringify(JSON.parse(readFileSync(snapshotPath)))}\n`, { flag: 'a' });
  writeFileSync(env.GITHUB_ENV, `PREVIOUS_PREVIEW_VERSION=${previousVersion}\nPREVIOUS_PREVIEW_IMAGE=docker.io/kjlion/kejilion-panel@${previous.digest}\n`, { flag: 'a' });
  process.exit(0);
}
if (process.argv[2] !== 'verify') throw new Error('snapshot or verify required');
const before = JSON.parse(readFileSync(snapshotPath));
if (isPreview && (before.tag !== latestRelease.tag_name || before.digest !== latestImage.digest)) throw new Error('Preview publication changed the stable channel');
const target = await manifest(version), preview = await manifest('preview');
if (target.digest !== env.IMAGE_DIGEST || (isPreview ? preview.digest !== target.digest : latestImage.digest !== target.digest)) throw new Error('Version/channel digest mismatch');
if (!isPreview && (latestRelease.tag_name !== `v${version}` || preview.digest !== before.previousPreview?.digest)) throw new Error('Stable publication changed preview or did not become Latest');
const release = await json(`https://api.github.com/repos/kejilion/KPanel/releases/tags/v${version}`);
if (release.draft || release.prerelease !== isPreview || release.tag_name !== `v${version}` || release.assets.length !== 15) throw new Error('Release identity mismatch');
const tagRef = await json(`https://api.github.com/repos/kejilion/KPanel/git/ref/tags/v${version}`);
if (tagRef.object.type !== 'tag') throw new Error('Annotated release tag required');
const tagObject = await json(tagRef.object.url);
if (tagObject.object.type !== 'commit' || tagObject.object.sha !== env.EXPECTED_SHA) throw new Error('Published tag source mismatch');
const platforms = [];
const dockerfile = readFileSync('Dockerfile', 'utf8');
for (const arch of ['amd64', 'arm64']) {
  const matches = target.body.manifests.filter(m => m.platform?.os === 'linux' && m.platform?.architecture === arch);
  if (matches.length !== 1) throw new Error('Multiarchitecture image incomplete');
  const m = await manifest(matches[0].digest);
  const raw = await request(`${registry}/blobs/${m.body.config.digest}`, headers, 4 * 1024 * 1024);
  if ('sha256:' + sha256(raw.bytes) !== m.body.config.digest) throw new Error('OCI config integrity mismatch');
  const config = JSON.parse(raw.bytes), labels = config.config.Labels;
  if (config.architecture !== arch || labels['org.opencontainers.image.version'] !== version || labels['org.opencontainers.image.revision'] !== env.EXPECTED_SHA) throw new Error('OCI source/version mismatch');
  for (const key of ['io.kejilion.script.revision', 'io.kejilion.script.sha256']) {
    if (!dockerfile.includes(`${key}="${labels[key]}"`)) throw new Error('Managed script identity mismatch');
  }
  platforms.push({ arch, digest: matches[0].digest, configDigest: m.body.config.digest, labels });
}
const assets = [], assetDirectory = join(directory, 'assets');
mkdirSync(assetDirectory);
const pending = [...release.assets];
await Promise.all(Array.from({ length: 3 }, async () => {
  while (pending.length) {
    const asset = pending.shift();
    const prefix = `https://github.com/kejilion/KPanel/releases/download/v${version}/`;
    if (!/^[A-Za-z0-9_.-]+$/.test(asset.name) || asset.name.includes('..') || asset.browser_download_url !== prefix + asset.name || !/^sha256:[0-9a-f]{64}$/.test(asset.digest) || asset.size > 50 * 1024 * 1024) throw new Error('Unsafe release asset metadata');
    const raw = (await request(asset.browser_download_url, {}, asset.size)).bytes;
    const path = join(assetDirectory, asset.name);
    writeFileSync(path, raw, { flag: 'wx' });
    if (raw.length !== asset.size || 'sha256:' + sha256(raw) !== asset.digest || sha256(readFileSync(path)) !== sha256(raw)) throw new Error(`Asset integrity failed: ${asset.name}`);
    assets.push({ name: asset.name, size: asset.size, digest: asset.digest });
  }
}));
const sums = readFileSync(join(assetDirectory, 'SHA256SUMS'), 'utf8').trim().split(/\r?\n/);
if (sums.length !== 11) throw new Error('Checksum inventory incomplete');
for (const line of sums) {
  const match = line.match(/^([a-f0-9]{64})\s+\*?([A-Za-z0-9_.-]+)$/);
  if (!match || !assets.some(a => a.name === match[2] && a.digest === 'sha256:' + match[1])) throw new Error('SHA256SUMS mismatch');
}
const notes = release.body.split('### 版本更新内容')[1]?.split(/^### /m)[0] ?? '';
let upgrades = false; const summaries = [], warnings = [];
for (const line of notes.split('\n')) {
  if (line.startsWith('#### ')) upgrades = line.includes('升级注意事项');
  else if (line.startsWith('- ')) (upgrades ? warnings : summaries).push(line.slice(2));
}
if (!summaries.length || !warnings.length || !release.body.includes(target.digest)) throw new Error('Release notes incomplete');
const result = { status: 'passed', candidate: env.EXPECTED_SHA, version, digest: target.digest, stable: before, platforms, assets, checksumCount: sums.length,
  notesCount: Math.min(8, summaries.length), upgradeNotesCount: Math.min(3, warnings.length), checkedAt: new Date().toISOString() };
writeFileSync(join(directory, 'public-release.json'), JSON.stringify(result, null, 2) + '\n', { flag: 'wx' });
if (env.GITHUB_ENV) writeFileSync(env.GITHUB_ENV, `EXPECTED_NOTES=${result.notesCount}\nEXPECTED_UPGRADE_NOTES=${result.upgradeNotesCount}\nEXPECTED_STABLE_VERSION=${isPreview ? before.tag.slice(1) : version}\n`, { flag: 'a' });
console.log(`public_release=pass version=${version} digest=${target.digest} assets=${assets.length} platforms=${platforms.length}`);
