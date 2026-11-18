import assert from 'node:assert/strict';
import { chromium } from 'playwright-core';
import { lookup } from 'node:dns/promises';

const base = process.env.APP_URL || 'http://localhost:3022';
const hostAddress = (await lookup('host.docker.internal')).address;
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', args: ['--no-sandbox', '--disable-dev-shm-usage', '--host-resolver-rules=MAP localhost ' + hostAddress] });
const context = await browser.newContext();
// Only selected history responses below are mocked; actual browser Origin and
// cookies are retained. Node fetches need the Docker host alias separately from
// Chromium's localhost resolver rule.
async function fetchMockSource(route, requestUrl = route.request().url()) {
  const url = new URL(requestUrl);
  if (url.hostname === 'localhost') url.hostname = 'host.docker.internal';
  return route.fetch({ url: url.toString(), headers: await route.request().allHeaders() });
}
const page = await context.newPage(); page.setDefaultTimeout(15000);
const errors = []; page.on('pageerror', e => errors.push(e.message));
async function login(role) {
  await page.goto(base); await page.getByLabel('Email address').fill(`${role}@demo.local`);
  await page.getByLabel('Password', { exact: true }).fill('DemoCTC!2026');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('navigation').waitFor();
}
async function nav(name) { await page.getByRole('navigation').getByRole('button', { name, exact: true }).click(); }
try {
  await login('admin'); await nav('Security settings');
  await page.getByLabel(/^Idle session timeout/).waitFor();
  const idle = await page.getByLabel(/^Idle session timeout/).inputValue();
  const absolute = await page.getByLabel(/^Maximum session duration/).inputValue();
  assert(Number(idle) >= 5 && Number(idle) <= 60); assert(Number(absolute) >= 1 && Number(absolute) <= 24);
  await page.getByLabel('Current administrator password').fill('DemoCTC!2026');
  await page.getByRole('button', { name: 'Save security settings' }).click();
  await page.getByText('Session security settings updated.', { exact: true }).waitFor();
  assert.equal(await page.getByLabel(/^Idle session timeout/).inputValue(), idle);
  assert.equal(await page.getByLabel('Current administrator password').inputValue(), '');
  console.log('PASS administrator reads and saves current session settings with password confirmation');
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await page.getByRole('button', { name: 'Sign in', exact: true }).waitFor();

  let configFetches = 0; const offsets = [];
  await page.route('**/api/bootstrap', async route => {
    const response = await fetchMockSource(route); const data = await response.json();
    await route.fulfill({ response, json: { ...data, configurations: [] } });
  });
  // Expand only browser responses to exercise paging without creating 51 salary revisions.
  await page.route(/\/api\/cases\/[^/?]+\?/, async route => {
    const response = await fetchMockSource(route); const data = await response.json();
    const offset = Number(new URL(route.request().url()).searchParams.get('offset')); offsets.push(offset);
    const original = data.revisions[0];
    if (!original && offset > 0) {
      const first = await fetchMockSource(route, route.request().url().replace(/offset=\d+/, 'offset=0'));
      const source = await first.json();
      await route.fulfill({ response, json: { ...source, case: { ...source.case, revisionCount: 51 }, revisions: [{ ...source.revisions[0], number: 1 }] } }); return;
    }
    const revisions = offset === 0 ? Array.from({ length: 50 }, (_, i) => ({ ...original, id: `${original.id}-${i}`, number: 51 - i })) : [{ ...original, number: 1 }];
    await route.fulfill({ response, json: { ...data, case: { ...data.case, revisionCount: 51 }, revisions } });
  });
  page.on('request', r => { if (/\/api\/configurations\/[^/?]+$/.test(r.url())) configFetches++; });
  await login('cb'); await nav('CTC records');
  await page.getByRole('button', { name: 'Open', exact: true }).first().click();
  await page.getByRole('dialog').getByRole('button', { name: 'Next page', exact: true }).click();
  await page.getByRole('dialog').getByText('Page 2 · 1 record shown', { exact: true }).waitFor();
  await page.getByRole('button', { name: 'Create next revision', exact: true }).click();
  await page.getByRole('heading', { name: 'Generate CTC', exact: true }).waitFor();
  await page.waitForFunction(() => document.querySelector('[aria-label="Configuration version"]')?.value);
  assert(offsets.includes(50)); assert.equal(configFetches, 1);
  console.log('PASS revision history pagination and loading a configuration outside bootstrap history');
  assert.deepEqual(errors, []); console.log('PASS no uncaught browser errors');
} finally { await browser.close(); }
