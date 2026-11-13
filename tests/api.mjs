// Synthetic configurations below are TEST_FIXTURE data only. Use the isolated test database.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import path from 'node:path';

const base = (process.env.TEST_BASE_URL || 'http://host.docker.internal:8090').replace(/\/$/, '').replace(/\/api$/, '') + '/api';
const origin = process.env.TEST_ORIGIN || 'http://localhost:3020';
const artifacts = process.env.ARTIFACT_DIR || '/artifacts';
const password = process.env.DEMO_PASSWORD || 'DemoCTC!2026';
const runId = Date.now().toString(36);
const outcomes = [];
const clients = {};
let demo, input, saved, finalConfig, finalSaved;

class Client {
  cookie = ''; csrf = ''; user = null;
  async request(method, route, body, options = {}) {
    const headers = { Accept: 'application/json', ...options.headers };
    if (this.cookie) headers.Cookie = this.cookie;
    if (!['GET', 'HEAD'].includes(method)) {
      headers.Origin ??= origin;
      if (options.csrf !== false && this.csrf) headers['X-CSRF-Token'] = this.csrf;
    }
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    const response = await fetch(base + route, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(90000) });
    const cookie = response.headers.getSetCookie?.().find(value => value.startsWith('ctc_session='));
    if (cookie) this.cookie = cookie.split(';')[0];
    const buffer = Buffer.from(await response.arrayBuffer());
    let data;
    try { data = JSON.parse(buffer.toString('utf8')); } catch { data = buffer.toString('utf8'); }
    return { status: response.status, headers: response.headers, data, buffer };
  }
  async login(email, pass = password) {
    const r = await this.request('POST', '/auth/login', { email, password: pass });
    expect(r, 200);
    this.user = r.data.user; this.csrf = r.data.csrfToken;
    assert.ok(this.cookie && this.csrf && this.user?.id, 'login returns a session, CSRF token and user');
    return r;
  }
}
function expect(r, status) {
  assert.equal(r.status, status, `HTTP ${r.status}: ${typeof r.data === 'string' ? r.data.slice(0, 300) : JSON.stringify(r.data)}`);
  return r.data;
}
function line(result, code) { const value = result.lines.find(l => l.code === code); assert.ok(value, `missing ${code}`); return value; }
function money(actual, expected) { assert.notEqual(actual, null); assert.equal(Number(actual), expected); }
async function calculate(client, config = demo, changes = {}) { return expect(await client.request('POST', '/calculate', { configurationId: config.id, input: { ...input, ...changes } }), 200); }
async function check(name, action) {
  const start = Date.now();
  try { await action(); outcomes.push({ name, passed: true, milliseconds: Date.now() - start }); console.log('PASS ' + name); }
  catch (error) { outcomes.push({ name, passed: false, error: error.stack || String(error) }); console.error('FAIL ' + name + '\n' + error.stack); }
}
function completeFixture() {
  const data = structuredClone(demo.data);
  data.annualSpecialGroup = 'statutory';
  data.basicMethod = 'manual'; data.basicMethodConfirmed = true;
  data.rounding = { scale: 2, mode: 'half_up', confirmed: true };
  data.annualizationConfirmed = true; data.hraApproved = true; data.templateApproved = true;
  data.grades = [{ id: 'test-grade', name: 'TEST_FIXTURE grade (not client policy)', basicMin: '7000', basicMax: '50000', conveyance: '7300', conveyanceCap: '7300', professionalAllowance: { CA: '1000', CMA: '1000', CS: '1000' } }];
  data.locations = [{ id: 'test-location', name: 'TEST_FIXTURE location' }];
  data.esi = { rate: '0.0325', ceiling: '21000', eligibilityBasis: 'gross', contributionBasis: 'gross', ceilingInclusive: true };
  data.benefits = demo.data.benefits.map(b => ({ component: b.component, kind: 'none', period: b.component === 'SUPERANNUATION' ? 'monthly' : 'annual', notes: 'TEST_FIXTURE explicit inapplicability for isolated acceptance only' }));
  data.offerTemplate = { companyName: 'TEST_FIXTURE COMPANY', address: 'Test environment only', signatory: 'Test Signatory', body: 'TEST_FIXTURE offer wording for automated acceptance only. This is not a client employment offer.' };
  return { name: 'TEST_FIXTURE complete ' + runId, source: 'client', data };
}
async function createFixture(patch = () => {}, label = 'variant') {
  const fixture = completeFixture(); fixture.name += ' ' + label; patch(fixture.data);
  return expect(await clients.cb.request('POST', '/configurations', fixture), 201);
}
async function document(client, revisionId, kind, mode, label) {
  const response = await client.request('GET', `/revisions/${revisionId}/documents/${kind}?format=pdf&mode=${mode}`);
  expect(response, 200); assert.match(response.headers.get('content-type') || '', /application\/pdf/);
  assert.equal(response.buffer.subarray(0, 5).toString(), '%PDF-');
  await mkdir(artifacts, { recursive: true });
  const pdfPath = path.join(artifacts, label + '.pdf'); await writeFile(pdfPath, response.buffer);
  const text = execFileSync('pdftotext', ['-layout', pdfPath, '-'], { encoding: 'utf8' });
  assert.match(text, /Inclusion Wages/); assert.match(text, /Exclusion Wages/);
  assert.match(text, /Statutory Payments/); assert.match(text, /Policy-Based Payments/); assert.match(text, /Variable Pay/);
  assert.match(text, /36900(?:\.00)?/); assert.match(text, /50000(?:\.00)?/);
  if (mode === 'draft') assert.match(text, /DEMO\s*\/\s*DRAFT/);
  if (mode === 'final') assert.doesNotMatch(text, /DEMO\s*\/\s*DRAFT/);
  await writeFile(path.join(artifacts, label + '.txt'), text);
  return { response, text };
}

await check('health and unauthenticated access', async () => {
  const anonymous = new Client(); expect(await anonymous.request('GET', '/health'), 200);
  expect(await anonymous.request('GET', '/bootstrap'), 401);
  expect(await anonymous.request('POST', '/auth/login', { email: 'hr@demo.local', password: 'wrong-password' }), 401);
});
await check('login, session, catalog, and URD-only baseline', async () => {
  for (const role of ['hr', 'cb', 'admin', 'auditor']) { clients[role] = new Client(); await clients[role].login(role + '@demo.local'); assert.equal(clients[role].user.role, role); }
  const bootstrap = expect(await clients.hr.request('GET', '/bootstrap'), 200);
  assert.equal(bootstrap.components.length, 20); assert.equal(new Set(bootstrap.components.map(c => c.group)).size, 5);
  demo = bootstrap.configurations.find(c => c.source === 'urd-demo'); assert.ok(demo); assert.ok(bootstrap.clarifications.length >= 8);
  input = { candidateName: 'TEST_FIXTURE Candidate ' + runId, candidateReference: 'TEST-' + runId, designation: 'Test Engineer', department: 'Acceptance', joiningDate: '2026-12-01', locationId: demo.data.locations[0].id, gradeId: demo.data.grades[0].id, qualification: 'NONE', gross: '100000', basic: '20000', taxRegime: 'new', npsOptIn: true, job: { sixthDay: false, fieldService: false, relocation: false, deputation: false, additionalHra: false }, ctcYear: 1, notes: 'Automated acceptance fixture' };
  const me = expect(await clients.hr.request('GET', '/auth/me'), 200); assert.equal(me.user.id, clients.hr.user.id);
});
await check('CSRF, cross-origin, and role permissions', async () => {
  expect(await clients.hr.request('POST', '/calculate', { configurationId: demo.id, input }, { csrf: false }), 403);
  expect(await clients.hr.request('POST', '/calculate', { configurationId: demo.id, input }, { headers: { Origin: 'https://evil.invalid' } }), 403);
  expect(await clients.hr.request('POST', '/configurations', completeFixture()), 403);
  expect(await clients.admin.request('POST', '/configurations', completeFixture()), 403);
  expect(await clients.auditor.request('POST', '/calculate', { configurationId: demo.id, input }), 403);
  expect(await clients.hr.request('GET', '/users'), 403);
});
await check('URD exact worked example and unresolved rules', async () => {
  const r = await calculate(clients.hr); assert.equal(r.lines.length, 20); assert.equal(r.coreValid, true); assert.equal(r.complete, false); assert.equal(r.canFinalize, false);
  money(line(r, 'BASIC').monthly, 20000); money(line(r, 'PROFESSIONAL').monthly, 0); money(line(r, 'CONVEYANCE').monthly, 7300);
  money(line(r, 'PF').monthly, 3000); money(line(r, 'NPS').monthly, 2800); money(line(r, 'SPECIAL').monthly, 30000); money(line(r, 'HRA').monthly, 36900);
  money(line(r, 'GRATUITY').monthly, 2405); money(r.totals.inclusion, 50000); money(r.totals.exclusion, 50000); money(r.totals.difference, 0); money(r.totals.knownAnnualTotal, 1228860);
  assert.equal(r.totals.annualCTC, null); assert.equal(line(r, 'ESI').annual, null); assert.equal(line(r, 'ANNUAL_SPECIAL').annual, null);
  assert.ok(r.issues.some(i => i.severity === 'pending'));
});
await check('NPS elections and exact PF branches', async () => {
  let r = await calculate(clients.hr, demo, { taxRegime: 'old' }); money(line(r, 'NPS').monthly, 2000); money(line(r, 'HRA').monthly, 37700);
  r = await calculate(clients.hr, demo, { npsOptIn: false }); money(line(r, 'NPS').monthly, 0); money(line(r, 'HRA').monthly, 39700);
  for (const [gross, basic, pf] of [['80000','20000',2400],['100000','25000',3000],['120000','25000',3000],['120000','30000',3600],['60000','25000',1800],['60000','25001',3000.12]]) {
    r = await calculate(clients.hr, demo, { gross, basic }); money(line(r, 'PF').monthly, pf);
  }
});
await check('negative residual, unknown grade, qualification, and numeric validation', async () => {
  for (const changes of [{ gross: '20000', basic: '20000' }, { gross: '100000.01' }, { gross: '1e100000000' }, { gross: '100000', basic: '-1' }, { gradeId: 'missing-grade' }, { qualification: 'NOT_A_QUALIFICATION' }]) {
    const r = await calculate(clients.hr, demo, changes); assert.equal(r.canFinalize, false); assert.ok(r.issues.some(i => i.severity === 'error'), JSON.stringify(changes));
  }
  const r = await calculate(clients.hr, demo, { qualification: 'CA' }); assert.equal(line(r, 'PROFESSIONAL').monthly, null); assert.equal(r.canFinalize, false);
});
await check('save, read revision, and draft/final gate', async () => {
  saved = expect(await clients.hr.request('POST', '/cases', { configurationId: demo.id, input }), 201);
  assert.ok(saved.caseId && saved.revisionId); const record = expect(await clients.hr.request('GET', '/cases/' + saved.caseId), 200);
  assert.equal(record.revisions[0].configurationId, demo.id); assert.deepEqual(record.revisions[0].input, input);
  money(line(record.revisions[0].result, 'HRA').monthly, 36900);
  const final = await clients.hr.request('GET', `/revisions/${saved.revisionId}/documents/offer?format=pdf&mode=final`); assert.ok([400,409,422].includes(final.status));
  const invalidSave = await clients.hr.request('POST', '/cases', { configurationId: demo.id, input: { ...input, gross: '20000' } }); expect(invalidSave, 422);
});
await check('draft CTC and offer PDFs preserve saved figures', async () => {
  await document(clients.hr, saved.revisionId, 'ctc', 'draft', 'api-ctc-draft');
  const offer = await document(clients.hr, saved.revisionId, 'offer', 'draft', 'api-offer-draft'); assert.match(offer.text, /Offer Letter/); assert.match(offer.text, /Pending/);
});
await check('isolated complete master configuration and publication', async () => {
  // This suite is intentionally pointed at the dedicated test backend (8090).
  assert.ok(process.env.CTC_TEST_FIXTURES === '1' || /:8090(?:\/|$)/.test(base), 'Explicit CTC_TEST_FIXTURES=1 required outside the dedicated 8090 backend');
  finalConfig = await createFixture();
  const updatedInput = { ...input, gradeId: 'test-grade', locationId: 'test-location' };
  let r = expect(await clients.hr.request('POST', '/calculate', { configurationId: finalConfig.id, input: updatedInput }), 200); assert.equal(r.canFinalize, false, 'unpublished config cannot finalize');
  finalConfig = expect(await clients.cb.request('POST', `/configurations/${finalConfig.id}/publish`, {}), 200); assert.equal(finalConfig.status, 'published');
  r = expect(await clients.hr.request('POST', '/calculate', { configurationId: finalConfig.id, input: updatedInput }), 200);
  assert.equal(r.coreValid, true); assert.equal(r.complete, true, JSON.stringify(r.issues)); assert.equal(r.canFinalize, true, JSON.stringify(r.issues)); money(r.totals.annualCTC, 1228860);
  finalSaved = expect(await clients.hr.request('POST', '/cases', { configurationId: finalConfig.id, input: updatedInput }), 201);
});
await check('final CTC and offer PDFs with matching totals', async () => {
  const ctc = await document(clients.hr, finalSaved.revisionId, 'ctc', 'final', 'api-ctc-final'); assert.match(ctc.text, /1228860(?:\.00)?/);
  const offer = await document(clients.hr, finalSaved.revisionId, 'offer', 'final', 'api-offer-final'); assert.match(offer.text, /TEST_FIXTURE COMPANY/); assert.match(offer.text, /1228860(?:\.00)?/);
});
await check('percentage basis periods, ESI equality, and missing job parameter', async () => {
  for (const [period, expected] of [['monthly',2000],['annual',24000]]) {
    const config = await createFixture(data => { data.benefits.find(b => b.component === 'PERFORMANCE').kind = 'percent'; Object.assign(data.benefits.find(b => b.component === 'PERFORMANCE'), { rate: '0.10', basis: 'basic', basisPeriod: period, period: 'annual' }); }, period);
    const r = await calculate(clients.hr, config, { gradeId: 'test-grade', locationId: 'test-location' }); money(line(r, 'PERFORMANCE').annual, expected);
  }
  for (const [inclusive, expected] of [[true,3250],[false,0]]) {
    const config = await createFixture(data => { data.esi.ceiling = '100000'; data.esi.ceilingInclusive = inclusive; }, 'esi-' + inclusive);
    const r = await calculate(clients.hr, config, { gradeId: 'test-grade', locationId: 'test-location' }); money(line(r, 'ESI').monthly, expected);
  }
  const config = await createFixture(data => { Object.assign(data.benefits.find(b => b.component === 'SIXTH_DAY'), { kind: 'fixed', amount: '500', period: 'monthly', conditions: [{ field: 'sixthDay', equals: true }] }); }, 'job');
  const r = await calculate(clients.hr, config, { gradeId: 'test-grade', locationId: 'test-location', job: {} }); assert.equal(line(r, 'SIXTH_DAY').monthly, null); assert.equal(r.canFinalize, false);
  const bad = completeFixture(); bad.data.grades[0].conveyance = '7301'; expect(await clients.cb.request('POST', '/configurations', bad), 400);
});
await check('all benefit groups with nonzero values, periods, conditions, and LTM years', async () => {
  // TEST_FIXTURE arithmetic only: these values are deliberately not client/demo master defaults.
  let config = await createFixture(data => {
    const rules = [
      { component: 'SUPERANNUATION', kind: 'percent', rate: '0.10', basis: 'basic', basisPeriod: 'monthly', period: 'monthly' },
      { component: 'ANNUAL_SPECIAL', kind: 'fixed', amount: '12000', period: 'annual' },
      { component: 'SIXTH_DAY', kind: 'fixed', amount: '500', period: 'monthly', conditions: [{ field: 'sixthDay', equals: true }] },
      { component: 'FIELD_SERVICE', kind: 'percent', rate: '0.02', basis: 'gross', basisPeriod: 'monthly', period: 'monthly', conditions: [{ field: 'fieldService', equals: true }] },
      { component: 'RELOCATION', kind: 'fixed', amount: '15000', period: 'one_time', conditions: [{ field: 'relocation', equals: true }] },
      { component: 'DEPUTATION', kind: 'fixed', amount: '6000', period: 'annual', conditions: [{ field: 'deputation', equals: true }] },
      { component: 'ADDITIONAL_HRA', kind: 'percent', rate: '0.05', basis: 'basic', basisPeriod: 'monthly', period: 'monthly', conditions: [{ field: 'additionalHra', equals: true }] },
      { component: 'PERFORMANCE', kind: 'percent', rate: '0.10', basis: 'basic', basisPeriod: 'annual', period: 'annual' },
      { component: 'ADDITIONAL_PERFORMANCE', kind: 'fixed', amount: '4000', period: 'annual' },
      { component: 'LTM1', kind: 'schedule', period: 'annual', schedule: [{ year: 1, amount: '10000' }, { year: 2, amount: '20000' }] },
      { component: 'LTM2', kind: 'schedule', period: 'annual', schedule: [{ year: 1, amount: '3000' }, { year: 2, amount: '6000' }] }
    ];
    data.benefits = rules.map(rule => ({ ...rule, notes: 'TEST_FIXTURE nonzero arithmetic coverage; not client compensation policy' }));
  }, 'all-nonzero');
  config = expect(await clients.cb.request('POST', `/configurations/${config.id}/publish`, {}), 200);
  const changes = { gradeId: 'test-grade', locationId: 'test-location', qualification: 'CA', ctcYear: 1, job: { sixthDay: true, fieldService: true, relocation: true, deputation: true, additionalHra: true } };
  const result = await calculate(clients.hr, config, changes);
  assert.equal(result.lines.length, 20); assert.equal(result.coreValid, true); assert.equal(result.complete, true, JSON.stringify(result.issues)); assert.equal(result.canFinalize, true);
  const monthly = { BASIC: 20000, PROFESSIONAL: 1000, SPECIAL: 29000, HRA: 34900, CONVEYANCE: 7300, PF: 3000, NPS: 2800, SUPERANNUATION: 2000, ESI: 0, GRATUITY: 2405, SIXTH_DAY: 500, FIELD_SERVICE: 2000, ADDITIONAL_HRA: 1000 };
  for (const [code, expected] of Object.entries(monthly)) money(line(result, code).monthly, expected);
  const annual = { ANNUAL_SPECIAL: 12000, SIXTH_DAY: 6000, FIELD_SERVICE: 24000, RELOCATION: 15000, DEPUTATION: 6000, ADDITIONAL_HRA: 12000, PERFORMANCE: 24000, ADDITIONAL_PERFORMANCE: 4000, LTM1: 10000, LTM2: 3000 };
  for (const [code, expected] of Object.entries(annual)) money(line(result, code).annual, expected);
  assert.equal(line(result, 'RELOCATION').period, 'one_time'); assert.equal(line(result, 'RELOCATION').monthly, null);
  assert.equal(line(result, 'PERFORMANCE').monthly, null); assert.equal(line(result, 'ANNUAL_SPECIAL').monthly, null);
  money(result.totals.inclusion, 50000); money(result.totals.exclusion, 50000); money(result.totals.difference, 0);
  // Independent group totals: statutory = 2405*12+12000; policy = 6000+24000+15000+6000+12000;
  // variable year 1 = 24000+4000+10000+3000. PF/NPS/Super remain inside the 1200000 annual Gross.
  money(result.totals.statutoryAnnual, 40860); money(result.totals.policyAnnual, 63000); money(result.totals.variableAnnual, 41000);
  money(result.totals.annualCTC, 1344860); money(result.totals.knownAnnualTotal, 1344860);
  const year2 = await calculate(clients.hr, config, { ...changes, ctcYear: 2 });
  money(line(year2, 'LTM1').annual, 20000); money(line(year2, 'LTM2').annual, 6000); money(year2.totals.variableAnnual, 54000); money(year2.totals.annualCTC, 1357860);
  const missingYear = await calculate(clients.hr, config, { ...changes, ctcYear: 3 });
  assert.equal(line(missingYear, 'LTM1').annual, null); assert.equal(line(missingYear, 'LTM2').annual, null); assert.equal(missingYear.canFinalize, false);
  const ineligible = await calculate(clients.hr, config, { ...changes, job: { sixthDay: false, fieldService: false, relocation: false, deputation: false, additionalHra: false } });
  for (const code of ['SIXTH_DAY','FIELD_SERVICE','RELOCATION','DEPUTATION','ADDITIONAL_HRA']) { money(line(ineligible, code).annual, 0); assert.equal(line(ineligible, code).status, 'not_applicable'); }
  money(ineligible.totals.policyAnnual, 0); money(ineligible.totals.annualCTC, 1281860);
  const stored = expect(await clients.hr.request('POST', '/cases', { configurationId: config.id, input: { ...input, ...changes } }), 201);
  const loaded = expect(await clients.hr.request('GET', '/cases/' + stored.caseId), 200).revisions[0]; assert.deepEqual(loaded.result, result);
});
await check('immutable revisions survive new configuration versions', async () => {
  const before = expect(await clients.hr.request('GET', '/cases/' + saved.caseId), 200).revisions[0];
  const changed = await createFixture(data => { data.grades[0].conveyance = '100'; }, 'changed');
  expect(await clients.cb.request('POST', `/configurations/${changed.id}/publish`, {}), 200);
  const after = expect(await clients.hr.request('GET', '/cases/' + saved.caseId), 200).revisions[0]; assert.deepEqual(after, before);
  const second = expect(await clients.hr.request('POST', '/cases', { caseId: saved.caseId, configurationId: demo.id, input: { ...input, npsOptIn: false } }), 201); assert.notEqual(second.revisionId, saved.revisionId);
  const record = expect(await clients.hr.request('GET', '/cases/' + saved.caseId), 200); assert.equal(record.revisions.length, 2);
  money(line(record.revisions.find(r => r.id === saved.revisionId).result, 'HRA').monthly, 36900); money(line(record.revisions.find(r => r.id === second.revisionId).result, 'HRA').monthly, 39700);
});
await check('user management, owner isolation, and session revocation', async () => {
  const email = `test-${runId}@fixture.local`;
  const user = expect(await clients.admin.request('POST', '/users', { name: 'TEST_FIXTURE HR', email, password: 'FixturePass!2026', role: 'hr' }), 201);
  const outsider = new Client(); await outsider.login(email, 'FixturePass!2026');
  expect(await outsider.request('GET', '/cases/' + saved.caseId), 404);
  expect(await outsider.request('GET', `/revisions/${saved.revisionId}/documents/ctc?format=pdf&mode=draft`), 404);
  expect(await outsider.request('POST', '/cases', { caseId: saved.caseId, configurationId: demo.id, input }), 404);
  const list = expect(await outsider.request('GET', '/cases'), 200); assert.ok(!list.some(c => c.id === saved.caseId));
  expect(await clients.admin.request('PATCH', '/users/' + user.id, { active: false }), 200);
  expect(await outsider.request('GET', '/auth/me'), 401);
  expect(await new Client().request('POST', '/auth/login', { email, password: 'FixturePass!2026' }), 401);
  expect(await clients.auditor.request('GET', '/cases/' + saved.caseId), 200);
});
await check('audit trail records configuration, revisions, users, documents', async () => {
  const events = expect(await clients.cb.request('GET', '/audit'), 200); assert.ok(Array.isArray(events));
  const actions = new Set(events.map(e => e.action));
  for (const action of ['configuration.create','configuration.publish','ctc.save','user.create','user.update']) assert.ok(actions.has(action), `Missing ${action}`);
  assert.ok(events.some(e => /document/.test(e.action)), 'document generation audited');
  assert.ok(!JSON.stringify(events).includes('FixturePass!2026'), 'audit does not leak passwords');
});
await check('logout invalidates session', async () => { expect(await clients.hr.request('POST', '/auth/logout', {}), 200); expect(await clients.hr.request('GET', '/auth/me'), 401); });
await mkdir(artifacts, { recursive: true });
await writeFile(path.join(artifacts, 'api-results.json'), JSON.stringify({ base, at: new Date().toISOString(), outcomes }, null, 2));
const failed = outcomes.filter(r => !r.passed);
console.log(`\nAPI acceptance: ${outcomes.length - failed.length}/${outcomes.length} passed`);
if (failed.length) process.exitCode = 1;