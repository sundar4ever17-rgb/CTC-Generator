// Checks unsaved editor behavior only; never saves a policy or candidate record.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { lookup } from 'node:dns/promises';
import { chromium } from 'playwright-core';

const base = process.env.APP_URL || 'http://localhost:3022';
const out = process.env.ARTIFACT_DIR || '/artifacts';
await mkdir(out, { recursive: true });
const address = (await lookup('host.docker.internal')).address;
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', args: ['--no-sandbox', '--disable-dev-shm-usage', '--host-resolver-rules=MAP localhost ' + address] });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(15000);
const errors = [], outcomes = [], saves = [];
page.on('pageerror', error => errors.push(error.message));
page.on('request', request => {
  if (request.method() === 'POST' && /\/api\/(configurations|cases)$/.test(new URL(request.url()).pathname)) saves.push(request.url());
});
async function step(name, run) {
  try { await run(); outcomes.push({ name, passed: true }); console.log('PASS ' + name); }
  catch (error) {
    outcomes.push({ name, passed: false, error: error.message });
    await page.screenshot({ path: path.join(out, 'add-component-failure.png'), fullPage: true });
    throw error;
  }
}
async function openEditor() {
  await page.goto(base);
  await page.getByRole('navigation', { name: 'Main navigation' }).waitFor({ state: 'attached' });
  const toggle = page.getByRole('button', { name: 'Open navigation', exact: true });
  if (await toggle.isVisible()) await toggle.click();
  await page.getByRole('navigation', { name: 'Main navigation' }).getByRole('button', { name: 'Configuration', exact: true }).click();
  const original = await page.evaluate(async () => {
    const boot = await (await fetch('/api/bootstrap')).json();
    return boot.configurations.find(c => c.source === 'urd-demo' && !c.data.rules)?.id;
  });
  assert(original, 'An unchanged URD policy must remain available');
  await page.getByLabel('Base configuration').selectOption(original);
  await page.getByRole('button', { name: 'Configure calculation rules', exact: true }).click();
  await page.getByRole('tab', { name: 'Components', exact: true }).waitFor();
}
async function addAndCheck(expectedCount) {
  await page.getByRole('button', { name: 'Add component', exact: true }).click();
  await page.waitForFunction(count => document.querySelectorAll('details.rule-component').length === count, expectedCount);
  const component = page.locator('details.rule-component').last();
  await page.waitForFunction(() => {
    const editor = document.querySelector('details.rule-component:last-child');
    return editor?.open && editor.querySelector('input[aria-label="Component name"]') === document.activeElement;
  });
  const state = await component.evaluate(el => ({ open: el.open, top: el.getBoundingClientRect().top, viewport: innerHeight }));
  assert.equal(state.open, true, `New component is collapsed: ${JSON.stringify(state)}`);
  const name = component.getByLabel('Component name', { exact: true });
  assert(await name.evaluate(el => { const box = el.getBoundingClientRect(); return box.top >= 0 && box.bottom <= innerHeight; }), 'New component name should be in the viewport');
  assert.equal(await page.getByLabel('Find a component').inputValue(), '', 'Adding clears filters which would hide the new row');
  return component;
}
try {
  await page.goto(base);
  await page.getByLabel('Email address').fill('cb@demo.local');
  await page.getByLabel('Password', { exact: true }).fill('DemoCTC!2026');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('navigation', { name: 'Main navigation' }).waitFor({ state: 'attached' });
  await step('Add opens, reveals and focuses a new component without saving', async () => {
    await openEditor();
    const component = await addAndCheck(21);
    const name = component.getByLabel('Component name', { exact: true });
    await name.pressSequentially('Travel Allowance');
    assert.equal(await name.inputValue(), 'Travel Allowance');
    assert(await name.evaluate(el => el === document.activeElement));
    const code = component.getByLabel('Component code', { exact: true });
    await code.fill('TRAVEL'); await code.pressSequentially('_EXTRA');
    assert.equal(await code.inputValue(), 'TRAVEL_EXTRA');
    assert(await code.evaluate(el => el === document.activeElement));
    await component.locator(':scope > summary').click();
    assert.equal(await component.evaluate(el => el.open), false, 'The user can collapse the component after the one-time reveal');
    await addAndCheck(22);
    await page.screenshot({ path: path.join(out, 'add-component-desktop.png') });
  });
  await step('Add reveals the component when a search filter was active', async () => {
    await openEditor();
    await page.getByLabel('Find a component').fill('Basic Salary');
    assert.equal(await page.locator('details.rule-component').count(), 1);
    await addAndCheck(21);
  });
  await step('Add works on mobile even when the search had no matches', async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await openEditor();
    await page.getByLabel('Find a component').fill('no matching salary component');
    assert.equal(await page.locator('details.rule-component').count(), 0);
    await addAndCheck(21);
    await page.screenshot({ path: path.join(out, 'add-component-mobile.png') });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
  });
  assert.deepEqual(errors, []);
  assert.deepEqual(saves, [], 'No policy or record may be saved by this regression check');
} finally {
  await writeFile(path.join(out, 'add-component-results.json'), JSON.stringify({ outcomes, errors, saves }, null, 2));
  await browser.close();
}
