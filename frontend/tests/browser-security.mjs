import assert from 'node:assert/strict';
import { chromium } from 'playwright-core';
import { lookup } from 'node:dns/promises';

// Run from the QA image with this file mounted under /qa. Uses the isolated demo API.
const base = process.env.APP_URL || 'http://localhost:3022';
const hostAddress = (await lookup('host.docker.internal')).address;
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', args: ['--no-sandbox', '--disable-dev-shm-usage', '--host-resolver-rules=MAP localhost ' + hostAddress] });
const context = await browser.newContext();
// Only selected UI-state responses below are mocked. Authentication and all other
// browser requests use their actual Origin without request interception.
async function fetchMockSource(route) {
  const url = new URL(route.request().url());
  if (url.hostname === 'localhost') url.hostname = 'host.docker.internal';
  return route.fetch({ url: url.toString(), headers: await route.request().allHeaders() });
}
const page = await context.newPage();
page.setDefaultTimeout(15000);
const errors = [];
page.on('pageerror', error => errors.push(error.message));
async function login(role) {
  await page.getByLabel('Email address').fill(process.env[`${role.toUpperCase()}_EMAIL`] || `${role}@demo.local`);
  await page.getByLabel('Password', { exact: true }).fill(process.env[`${role.toUpperCase()}_PASSWORD`] || 'DemoCTC!2026');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('navigation', { name: 'Main navigation' }).waitFor();
}
async function logout() {
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await page.getByRole('button', { name: 'Sign in', exact: true }).waitFor();
}
try {
  const response = await page.goto(base);
  await page.getByRole('button', { name: 'Sign in', exact: true }).waitFor();
  assert.equal(await page.getByLabel('Email address').inputValue(), '');
  assert.equal(await page.getByLabel('Password', { exact: true }).inputValue(), '');
  assert.equal(await page.getByText('DemoCTC!2026', { exact: false }).count(), 0);
  const policy = response.headers()['content-security-policy'];
  assert.match(policy, /script-src[^;]*'nonce-[^']+'/);
  assert.doesNotMatch(policy.split(';').find(s => s.trim().startsWith('script-src')), /unsafe-inline|unsafe-eval/);
  assert.match(response.headers()['cache-control'], /no-store/);
  await page.evaluate(() => { const button = document.createElement('button'); button.id = 'untrusted-probe'; button.textContent = 'CSP test'; button.setAttribute('onclick', 'window.__untrustedScriptRan = true'); document.body.appendChild(button); });
  await page.locator('#untrusted-probe').click();
  assert.equal(await page.evaluate(() => window.__untrustedScriptRan), undefined);
  const secondResponse = await page.reload();
  assert.notEqual(secondResponse.headers()['content-security-policy'], policy);
  console.log('PASS credential-free login, nonce CSP, inline event handler blocked, uncached responses');

  await login('hr');
  await page.getByLabel('Candidate name', { exact: true }).fill('PRIVATE UNSAVED CANDIDATE');
  await logout();
  await login('auditor');
  assert.equal(await page.getByRole('navigation').getByRole('button', { name: 'Generate CTC', exact: true }).count(), 0);
  assert.equal(await page.getByText('PRIVATE UNSAVED CANDIDATE', { exact: true }).count(), 0);
  await logout();
  await login('hr');
  assert.notEqual(await page.getByLabel('Candidate name', { exact: true }).inputValue(), 'PRIVATE UNSAVED CANDIDATE');
  console.log('PASS cross-account state clearing and auditor navigation');

  await page.route('**/api/bootstrap', async route => {
    const res = await fetchMockSource(route);
    const data = await res.json();
    await route.fulfill({ response: res, json: { ...data, demo: false, configurations: [] } });
  });
  await page.reload();
  await page.getByLabel('Candidate name', { exact: true }).waitFor();
  assert.equal(await page.getByLabel('Candidate name', { exact: true }).inputValue(), '');
  assert.equal(await page.getByLabel('Monthly gross', { exact: true }).inputValue(), '');
  assert.equal(await page.getByRole('button', { name: 'Load URD example', exact: true }).count(), 0);
  console.log('PASS blank production workflow without demo sample loading');

  await logout();
  await login('cb');
  await page.getByRole('heading', { name: 'Configuration', exact: true }).waitFor();
  assert.equal(await page.getByLabel('Configuration name').inputValue(), '');
  assert.equal(await page.getByLabel('Data source').locator('option[value="urd-demo"]').count(), 0);
  await page.getByRole('button', { name: 'Add grade', exact: true }).click();
  assert.equal(await page.getByLabel('Grade name').inputValue(), '');
  assert.equal(await page.getByLabel('Basic minimum').inputValue(), '');
  console.log('PASS first production configuration can be authored without a demo seed');

  await page.route('**/api/auth/me', async route => {
    const res = await fetchMockSource(route);
    const data = await res.json();
    await route.fulfill({ response: res, json: { ...data, user: { ...data.user, mustChangePassword: true } } });
  });
  await page.reload();
  await page.getByRole('heading', { name: 'Set your personal password' }).waitFor();
  assert.equal(await page.getByRole('navigation', { name: 'Main navigation' }).count(), 0);
  assert.equal(await page.getByLabel(/^New password/).getAttribute('minlength'), '15');
  console.log('PASS mandatory password change blocks workspace');
  assert.deepEqual(errors, []);
  console.log('PASS no uncaught browser errors');
} finally { await browser.close(); }
