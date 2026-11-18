// Read-only container startup probes: no network, ports or database volumes.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const image = process.env.TEST_BACKEND_IMAGE || 'ctc-generator-backend';
const artifacts = process.env.ARTIFACT_DIR || path.resolve('artifacts');
const outcomes = [];
const baseline = { APP_ENV: 'production', APP_ORIGIN: 'https://ctc-test.invalid', DATABASE_URL: 'postgres://fixture:fixture@127.0.0.1/ctc_test?sslmode=verify-full' };
const cases = [
  ['explicit environment required', { APP_ENV: 'development' }, /APP_ENV must explicitly/],
  ['HTTPS origin required', { APP_ORIGIN: 'http://ctc-test.invalid' }, /HTTPS origin/],
  ['origin paths rejected', { APP_ORIGIN: 'https://ctc-test.invalid/path' }, /HTTPS origin/],
  ['database TLS verification required', { DATABASE_URL: 'postgres://fixture:fixture@127.0.0.1/ctc_test?sslmode=disable' }, /sslmode=verify-full/],
  ['demo credentials rejected', { DEMO_PASSWORD: 'SyntheticTestPassword!2026' }, /demo or bootstrap credentials/],
  ['bootstrap credentials excluded from runtime', { BOOTSTRAP_ADMIN_PASSWORD: 'SyntheticTestPassword!2026' }, /demo or bootstrap credentials/],
  ['conflicting secret sources rejected', { DATABASE_URL_FILE: '/missing-secret' }, /set only DATABASE_URL or DATABASE_URL_FILE/],
  ['unsafe pool size rejected', { DB_POOL_MAX_CONNS: '0' }, /DB_POOL_MAX_CONNS must be/],
];
for (const [name, overrides, message] of cases) {
  try {
    const environment = { ...baseline, ...overrides };
    const args = ['run', '--rm', '--network', 'none'];
    for (const [key, value] of Object.entries(environment)) args.push('-e', `${key}=${value}`);
    args.push(image);
    const result = spawnSync('docker', args, { encoding: 'utf8', timeout: 20000 });
    assert.ifError(result.error);
    assert.notEqual(result.status, 0, 'Unsafe configuration must not start');
    assert.match(result.stdout + result.stderr, message);
    assert.doesNotMatch(result.stdout + result.stderr, /SyntheticTestPassword!2026/);
    outcomes.push({ name, passed: true }); console.log('PASS ' + name);
  } catch (error) { outcomes.push({ name, passed: false, error: error.message }); console.error('FAIL ' + name + ': ' + error.message); }
}
await mkdir(artifacts, { recursive: true });
await writeFile(path.join(artifacts, 'startup-results.json'), JSON.stringify({ at: new Date().toISOString(), outcomes }, null, 2));
if (outcomes.some(outcome => !outcome.passed)) process.exitCode = 1;
