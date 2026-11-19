import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { lookup } from 'node:dns/promises';
import { chromium } from 'playwright-core';

assert.equal(process.env.CTC_TEST_FIXTURES, '1', 'This workflow creates a synthetic policy in an isolated test database');
const base = process.env.APP_URL || 'http://localhost:3024';
const out = process.env.ARTIFACT_DIR || '/artifacts';
await mkdir(out, { recursive: true });
const address = (await lookup('host.docker.internal')).address;
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', args: ['--no-sandbox','--disable-dev-shm-usage','--host-resolver-rules=MAP localhost ' + address] });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await context.newPage(); page.setDefaultTimeout(20000);
const errors = [], outcomes = []; page.on('pageerror', e => errors.push(e.message));
let policy;
async function step(name, action) {
  try { await action(); console.log('PASS ' + name); outcomes.push({ name, passed: true }); }
  catch (error) { outcomes.push({ name, passed: false, error: error.stack }); await page.screenshot({ path: path.join(out, 'rules-browser-failure.png'), fullPage: true }); throw error; }
}
async function nav(name) {
  const toggle = page.getByRole('button', { name: 'Open navigation', exact: true });
  if (await toggle.isVisible()) await toggle.click();
  await page.getByRole('navigation', { name: 'Main navigation' }).getByRole('button', { name, exact: true }).click();
}
async function login(role) {
  await page.goto(base); await page.getByLabel('Email address').fill(`${role}@demo.local`);
  await page.getByLabel('Password', { exact: true }).fill('DemoCTC!2026');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('navigation', { name: 'Main navigation' }).waitFor({ state: 'attached' });
}
async function hasMoney(id, value) {
  await page.waitForFunction(({ id, value }) => {
    const text = document.querySelector(`[data-testid="${id}"]`)?.textContent?.replace(/[^0-9.-]/g, '');
    return !!text && Number(text) === Number(value);
  }, { id, value });
}
try {
  await step('convert original configuration through C&B screen', async () => {
    await login('cb'); await nav('Configuration');
    const legacy = await page.getByLabel('Base configuration').locator('option').evaluateAll(options => options.find(o => o.textContent.includes('URD worked example'))?.value);
    assert(legacy); await page.getByLabel('Base configuration').selectOption(legacy);
    await page.getByRole('button', { name: 'Configure calculation rules', exact: true }).click();
    await page.getByRole('tab', { name: 'Components', exact: true }).waitFor();
    assert.equal(await page.locator('details.rule-component').count(), 20);
  });
  await step('add an Inclusion allowance using the guided editor', async () => {
    await page.getByRole('button', { name: 'Add component', exact: true }).click();
    const component = page.locator('details.rule-component').last();
    await page.waitForFunction(() => document.querySelector('details.rule-component:last-child')?.open === true);
    assert(await component.getByLabel('Component name', { exact: true }).evaluate(el => el === document.activeElement));
    await component.getByLabel('Component code').fill('SKILL_UI');
    await component.getByLabel('Component name', { exact: true }).fill('Skill Allowance UI');
    await component.getByLabel('Group').selectOption('inclusion');
    await component.getByLabel('Amount calculation value', { exact: true }).fill('3000');
    await page.getByLabel('Configuration name', { exact: true }).fill('TEST_FIXTURE UI configurable ' + Date.now());
    await page.screenshot({ path: path.join(out, 'rules-editor-desktop.png'), fullPage: true });
  });
  await step('preview calculates the new component and balancing amount without saving a case', async () => {
    await page.getByRole('tab', { name: 'Preview draft', exact: true }).click();
    await page.getByLabel('Preview monthly gross').fill('100000'); await page.getByLabel('Preview Basic salary').fill('20000');
    const response = page.waitForResponse(r => r.url().endsWith('/api/configurations/preview') && r.request().method() === 'POST');
    await page.getByRole('button', { name: 'Run preview', exact: true }).click();
    const resultResponse = await response; assert.equal(resultResponse.status(), 200, await resultResponse.text());
    const result = await resultResponse.json();
    assert.equal(result.lines.find(l => l.code === 'SKILL_UI').amount, '3000.00');
    assert.equal(result.lines.find(l => l.code === 'SPECIAL').amount, '27000.00');
    assert.equal(result.coreValid, true, JSON.stringify(result.issues));
    await page.getByText('Group targets and core calculations reconcile.', { exact: true }).waitFor();
    await page.screenshot({ path: path.join(out, 'rules-preview-desktop.png'), fullPage: true });
  });
  await step('save a new version and calculate it in Generate CTC', async () => {
    const response = page.waitForResponse(r => r.url().endsWith('/api/configurations') && r.request().method() === 'POST');
    await page.getByRole('button', { name: 'Save new version', exact: true }).first().click();
    const saved = await response; assert.equal(saved.status(), 201, await saved.text()); policy = await saved.json();
    assert.equal(policy.data.rules.components.length, 21);
    await nav('Generate CTC'); await page.getByLabel('Configuration version').selectOption(policy.id);
    await page.getByLabel('Monthly gross', { exact: true }).fill('100000'); await page.getByLabel('Basic salary', { exact: true }).fill('20000');
    await hasMoney('result-special', '27000.00'); await hasMoney('result-skill_ui', '3000.00');
    await page.screenshot({ path: path.join(out, 'rules-generator-desktop.png'), fullPage: true });
  });
  await step('rule editing remains usable on mobile without page overflow', async () => {
    await nav('Configuration'); await page.getByLabel('Base configuration').selectOption(policy.id);
    await page.getByRole('tab', { name: 'Calculation rules', exact: true }).click();
    await page.getByRole('tab', { name: 'Groups & targets', exact: true }).click();
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForFunction(() => document.querySelector('.sidebar').getBoundingClientRect().right <= 1);
    await page.screenshot({ path: path.join(out, 'rules-mobile.png'), fullPage: true });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'Rules screen overflows the mobile viewport');
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  });
  await step('HR can inspect the dynamic policy but cannot edit or preview a draft', async () => {
    await login('hr'); await nav('Configuration'); await page.getByLabel('Base configuration').selectOption(policy.id);
    await page.getByRole('tab', { name: 'Calculation rules', exact: true }).click();
    assert.equal(await page.getByRole('button', { name: 'Add component', exact: true }).count(), 0);
    assert.equal(await page.getByRole('button', { name: 'Save new version', exact: true }).count(), 0);
    assert.equal(await page.getByRole('tab', { name: 'Preview draft', exact: true }).count(), 0);
    const component = page.locator('details.rule-component').last(); await component.locator('summary').first().click();
    assert(await component.getByLabel('Component name', { exact: true }).isDisabled());
    assert.deepEqual(errors, []);
  });
  await step('HR custom inputs drive calculations and formula-based Basic needs no manual field', async () => {
    await nav('Generate CTC');
    const options = page.getByLabel('Configuration version');
    const dynamicId = await options.locator('option').evaluateAll(items => items.find(o => o.textContent.includes('TEST_FIXTURE dynamic rules'))?.value);
    const calculatedId = await options.locator('option').evaluateAll(items => items.find(o => o.textContent.includes('TEST_FIXTURE calculated Basic'))?.value);
    assert(dynamicId && calculatedId, 'Run rules-api.mjs before the browser suite to create the integration fixtures');
    await options.selectOption(dynamicId);
    await page.getByLabel('Skill amount', { exact: true }).fill('3000');
    await page.getByLabel('Remote assignment', { exact: true }).selectOption('true');
    await hasMoney('result-special', '27000.00'); await hasMoney('result-remote', '1000.00');
    await options.selectOption(calculatedId);
    assert.equal(await page.getByLabel('Basic salary', { exact: true }).count(), 0);
    await hasMoney('result-basic', '20000.00'); await hasMoney('result-nps', '2800.00');
    assert.deepEqual(errors, []);
  });
} finally {
  await writeFile(path.join(out, 'rules-browser-results.json'), JSON.stringify({ outcomes, errors }, null, 2));
  await browser.close();
}
