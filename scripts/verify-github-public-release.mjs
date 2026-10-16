import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const sha256 = value => createHash('sha256').update(value).digest('hex');
const env = process.env;
const version = env.EXPECTED_VERSION;
if (!/^\d+\.\d+\.\d+-rc\.[1-9]\d*$/.test(version ?? '') || !/^[a-f0-9]{40}$/.test(env.EXPECTED_SHA ?? '')) throw new Error('Exact preview identity required');
if (env.GITHUB_ACTIONS !== 'true' || env.RUNNER_ENVIRONMENT !== 'github-hosted') throw new Error('GitHub-hosted execution required');
const directory = env.PUBLIC_EVIDENCE;
if (!directory?.startsWith(env.RUNNER_TEMP + '/')) throw new Error('Evidence must be owned runner temporary data');
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
  mkdirSync(directory, { recursive: false });
  writeFileSync(snapshotPath, JSON.stringify({ tag: latestRelease.tag_name, digest: latestImage.digest }, null, 2) + '\n', { flag: 'wx' });
  process.exit(0);
}
if (process.argv[2] !== 'verify') throw new Error('snapshot or verify required');
const before = JSON.parse(readFileSync(snapshotPath));
if (before.tag !== latestRelease.tag_name || before.digest !== latestImage.digest) throw new Error('Preview publication changed the stable channel');
const target = await manifest(version), preview = await manifest('preview');
if (target.digest !== env.IMAGE_DIGEST || preview.digest !== target.digest) throw new Error('Version/preview digest mismatch');
const release = await json(`https://api.github.com/repos/kejilion/KPanel/releases/tags/v${version}`);
if (release.draft || !release.prerelease || release.tag_name !== `v${version}` || release.assets.length !== 15) throw new Error('Release identity mismatch');
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
if (env.GITHUB_ENV) writeFileSync(env.GITHUB_ENV, `EXPECTED_NOTES=${result.notesCount}\nEXPECTED_UPGRADE_NOTES=${result.upgradeNotesCount}\nEXPECTED_STABLE_VERSION=${before.tag.slice(1)}\n`, { flag: 'a' });
console.log(`public_release=pass version=${version} digest=${target.digest} assets=${assets.length} platforms=${platforms.length}`);
