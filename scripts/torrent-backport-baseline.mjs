#!/usr/bin/env node

// Local replacements hide upstream version identity from vulnerability databases.
// Keep the actual-source scans and add a conservative scan of the original version.
import { execFileSync, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { isAbsolute, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const MODULE = 'github.com/anacrolix/torrent';
const VERSION = 'v1.61.0';
const REPLACEMENT = './third_party/torrent';
const SOURCE_MANIFEST_SHA256 = '3165b3c3e3fcdefe8dc52fba19a0a872eb26238692d6fedff4db2fc48ecf2580';
const repoRoot = resolve(import.meta.dirname, '..');
const hash = (bytes) => createHash('sha256').update(bytes).digest('hex');

export function baselineMod(source) {
  const directive = /^replace github\.com\/anacrolix\/torrent v1\.61\.0 => \.\/third_party\/torrent\r?\n/gm;
  if ([...source.matchAll(directive)].length !== 1) throw new Error('Expected one exact version-qualified torrent replacement');
  return source.replace(directive, '');
}

export function parseModuleJSON(text) {
  const values = [];
  let depth = 0, start = -1, quoted = false, escaped = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (escaped) escaped = false;
      else if (c === '\\') escaped = true;
      else if (c === '"') quoted = false;
      continue;
    }
    if (c === '"') quoted = true;
    else if (c === '{') { if (depth++ === 0) start = i; }
    else if (c === '}') {
      if (--depth < 0) throw new Error('Malformed module graph');
      if (depth === 0) values.push(JSON.parse(text.slice(start, i + 1)));
    } else if (depth === 0 && !/\s/.test(c)) throw new Error('Malformed module graph');
  }
  if (depth || quoted || !values.length) throw new Error('Incomplete module graph');
  return values;
}

export function verifyModuleGraphs(actual, baseline) {
  for (const graph of [actual, baseline]) {
    if (graph.some(m => !m.Path || m.Error) || new Set(graph.map(m => m.Path)).size !== graph.length) throw new Error('Invalid module graph');
  }
  const current = actual.find(m => m.Path === MODULE);
  const original = baseline.find(m => m.Path === MODULE);
  if (current?.Version !== VERSION || current.Replace?.Path !== REPLACEMENT || current.Replace.Version
      || original?.Version !== VERSION || original.Replace) throw new Error('Torrent baseline identity mismatch');
  const normalized = graph => graph.map(m => ({ Path: m.Path, Version: m.Version ?? '',
    ...(m.Path !== MODULE && m.Replace ? { Replace: { Path: m.Replace.Path, Version: m.Replace.Version ?? '' } } : {}) }))
    .sort((a, b) => a.Path.localeCompare(b.Path));
  if (JSON.stringify(normalized(actual)) !== JSON.stringify(normalized(baseline))) throw new Error('Baseline changed the dependency graph');
  return hash(JSON.stringify(normalized(baseline)));
}

export function assertTrivyInventory(report) {
  const packages = (report.Results ?? []).flatMap(r => r.Packages ?? []);
  if (!packages.some(p => p.Name === MODULE && ['1.61.0', VERSION].includes(p.Version))) {
    throw new Error('Trivy baseline inventory is missing torrent v1.61.0');
  }
  return packages.length;
}

export function validateBackport(repo) {
  const directory = join(repo, 'third_party/torrent');
  const provenance = JSON.parse(readFileSync(join(directory, 'KPANEL_BACKPORT.json'), 'utf8'));
  if (provenance.module !== MODULE || provenance.version !== VERSION || provenance.replacement !== REPLACEMENT
      || provenance.sourceManifestSha256 !== SOURCE_MANIFEST_SHA256 || provenance.files?.length !== 453
      || new Set(provenance.files.map(f => f.path)).size !== 453) throw new Error('Invalid backport provenance');
  const files = readdirSync(directory, { recursive: true, withFileTypes: true });
  if (files.some(f => f.isSymbolicLink())) throw new Error('Backport source must not contain symlinks');
  for (const file of provenance.files) {
    if (!file.path || isAbsolute(file.path) || file.path.includes('\\') || file.path.split('/').some(p => p === '..' || p === '.')) throw new Error('Unsafe source manifest path');
    const target = join(directory, file.path);
    if (!lstatSync(target).isFile() || hash(readFileSync(target)) !== file.sha256) throw new Error('Backport source digest mismatch: ' + file.path);
  }
  const manifestHash = hash(provenance.files.map(f => `${f.sha256}  ${f.path}\n`).join(''));
  if (manifestHash !== provenance.sourceManifestSha256) throw new Error('Backport source manifest digest mismatch');
  const names = files.filter(f => f.isFile()).map(f => relative(directory, join(f.parentPath, f.name)).replaceAll('\\', '/'));
  const expected = new Set([...provenance.files.map(f => f.path), 'KPANEL_BACKPORT.md', 'KPANEL_BACKPORT.json']);
  if (names.length !== expected.size || names.some(name => !expected.has(name))) throw new Error('Unexpected backport source files');
  return provenance;
}

export function moduleEnvironment(modfile, environment = process.env) {
  if (/(?:^|\s)["']?-mod(?:file)?(?:=|\s|$)/.test(environment.GOFLAGS ?? '')) throw new Error('Conflicting GOFLAGS module override');
  if (modfile && /["\r\n]/.test(modfile)) throw new Error('Unsupported module-file path');
  // Go's quoted.Split only recognizes a quote at the start of the entire argument.
  return { ...environment, GOWORK: 'off', GOFLAGS: [environment.GOFLAGS ?? '', '-mod=readonly',
    ...(modfile ? [`"-modfile=${modfile.replaceAll('\\', '/')}"`] : [])].join(' ').trim() };
}

function go(args, cwd, env) {
  return execFileSync('go', args, { cwd, env, encoding: 'utf8', timeout: 180_000, maxBuffer: 16 * 1024 * 1024, windowsHide: true });
}

function prepare(repo, directory, basename) {
  if (!lstatSync(directory).isDirectory() || lstatSync(directory).isSymbolicLink() || readdirSync(directory).length) throw new Error('Baseline directory must be empty and private');
  const provenance = validateBackport(repo);
  const sourceMod = readFileSync(join(repo, 'go.mod'), 'utf8');
  const sourceSum = readFileSync(join(repo, 'go.sum'));
  const modfile = join(directory, basename + '.mod');
  const sumfile = join(directory, basename + '.sum');
  writeFileSync(modfile, baselineMod(sourceMod), { flag: 'wx', mode: 0o600 });
  writeFileSync(sumfile, sourceSum, { flag: 'wx', mode: 0o600 });
  const initial = { sourceMod: hash(sourceMod), sourceSum: hash(sourceSum), baselineMod: hash(readFileSync(modfile)), baselineSum: hash(readFileSync(sumfile)) };
  const actualEnvironment = moduleEnvironment();
  const baselineEnvironment = moduleEnvironment(modfile);
  const actualMod = JSON.parse(go(['mod', 'edit', '-json'], repo, actualEnvironment));
  const originalMod = JSON.parse(go(['mod', 'edit', '-json', '-modfile=' + modfile], repo, actualEnvironment));
  const replacements = (actualMod.Replace ?? []).filter(r => r.Old?.Path === MODULE);
  if (replacements.length !== 1 || replacements[0].Old.Version !== VERSION
      || replacements[0].New?.Path !== REPLACEMENT || replacements[0].New.Version
      || actualMod.Require?.filter(r => r.Path === MODULE && r.Version === VERSION).length !== 1) throw new Error('Unexpected torrent module declaration');
  const expectedMod = { ...actualMod, Replace: actualMod.Replace.filter(r => r !== replacements[0]) };
  if (!expectedMod.Replace.length) expectedMod.Replace = null;
  if (JSON.stringify(expectedMod) !== JSON.stringify(originalMod)) throw new Error('Baseline changed other module declarations');
  const actualGraph = go(['list', '-m', '-json', 'all'], repo, actualEnvironment);
  const originalGraph = go(['list', '-m', '-json', 'all'], repo, baselineEnvironment);
  const graphSha256 = verifyModuleGraphs(parseModuleJSON(actualGraph), parseModuleJSON(originalGraph));
  const unchanged = () => {
    const current = { sourceMod: hash(readFileSync(join(repo, 'go.mod'))), sourceSum: hash(readFileSync(join(repo, 'go.sum'))), baselineMod: hash(readFileSync(modfile)), baselineSum: hash(readFileSync(sumfile)) };
    if (JSON.stringify(initial) !== JSON.stringify(current)) throw new Error('Scanner changed the module manifests');
  };
  unchanged();
  console.log(JSON.stringify({ torrentBaseline: 'qualified-original-version-coverage', module: MODULE, version: VERSION,
    graphSha256, sourceManifestSha256: provenance.sourceManifestSha256, ...initial }));
  return { environment: baselineEnvironment, unchanged };
}

function run(command, args, cwd, env) {
  const result = spawnSync(command, args, { cwd, env, stdio: 'inherit', timeout: 15 * 60 * 1000, windowsHide: true });
  if (result.error || result.signal || result.status !== 0) throw new Error('Baseline command failed: ' + command + ' exit=' + result.status + ' ' + (result.error?.message ?? result.signal ?? ''));
}

export function main(argv) {
  if (argv[0] === 'manifest' && argv.length === 2) {
    const directory = resolve(argv[1]);
    // Go expects go.mod/go.sum; the helper's alternate basename is used for govulncheck.
    prepare(repoRoot, directory, 'go');
    return;
  }
  if (argv[0] === 'check-trivy' && argv.length === 2) {
    const count = assertTrivyInventory(JSON.parse(readFileSync(argv[1], 'utf8')));
    console.log('torrent_baseline_trivy=pass package_count=' + count + ' module=' + MODULE + ' version=' + VERSION);
    return;
  }
  if (argv[0] !== 'govuln' || argv.length !== 1) throw new Error('usage: torrent-backport-baseline.mjs govuln|manifest DIRECTORY|check-trivy REPORT');
  const directory = mkdtempSync(join(tmpdir(), 'kpanel-torrent-baseline-'));
  try {
    const baseline = prepare(repoRoot, directory, 'baseline');
    const makefile = readFileSync(join(repoRoot, 'Makefile'), 'utf8');
    const pins = [...new Set([...makefile.matchAll(/govulncheck@(v\d+\.\d+\.\d+)/g)].map(m => m[1]))];
    if (pins.length !== 1) throw new Error('Expected one fixed govulncheck version in Makefile');
    const bin = join(directory, 'bin');
    mkdirSync(bin, { mode: 0o700 });
    run('go', ['install', 'golang.org/x/vuln/cmd/govulncheck@' + pins[0]], repoRoot, { ...process.env, GOWORK: 'off', GOFLAGS: '', GOBIN: bin });
    const scanner = join(bin, 'govulncheck' + (process.platform === 'win32' ? '.exe' : ''));
    console.log('torrent_baseline_govuln=start tool=' + pins[0] + ' tool_sha256=' + hash(readFileSync(scanner)));
    run(scanner, ['./...'], repoRoot, baseline.environment);
    baseline.unchanged();
    console.log('torrent_baseline_govuln=pass coverage=original-upstream-version-conservative');
  } finally {
    // The path is created in this invocation, outside the checkout; never accept a caller path here.
    if (resolve(directory).startsWith(resolve(tmpdir()) + (process.platform === 'win32' ? '\\' : '/'))) rmSync(directory, { recursive: true, force: true });
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(process.argv.slice(2)); }
  catch (error) { process.stderr.write('torrent_baseline=fail ' + error.message + '\n'); process.exitCode = 1; }
}
