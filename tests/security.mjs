// Synthetic security fixtures only. Never point this suite at a live database.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import path from 'node:path';

const base = (process.env.TEST_BASE_URL || 'http://host.docker.internal:8090').replace(/\/$/, '').replace(/\/api$/, '') + '/api';
const production = process.env.TEST_PRODUCTION === '1';
assert.ok(process.env.CTC_TEST_FIXTURES === '1' || (!production && new URL(base).port === '8090'), 'Explicit CTC_TEST_FIXTURES=1 is required outside the isolated demo test port');
const origin = process.env.TEST_ORIGIN || 'http://localhost:3020';
const artifacts = process.env.ARTIFACT_DIR || '/artifacts';
const runId = Date.now().toString(36);
const temporaryPassword = 'SecurityFixture!2026';
const enrolledPassword = 'EnrolledFixture!2026';
const changedPassword = 'ChangedFixture!2026';
const outcomes = [];
let admin, cb, hr1, hr2, config, input, record, secondRecord;

function expect(response, status) {
  assert.equal(response.status, status, `HTTP ${response.status}: ${JSON.stringify(response.data).slice(0, 500)}`);
  return response.data;
}
class Client {
  cookie = ''; csrf = ''; user = null; password = temporaryPassword;
  async request(method, route, body, options = {}) {
    const headers = { Accept: 'application/json', ...options.headers };
    if (this.cookie) headers.Cookie = this.cookie;
    if (!['GET', 'HEAD'].includes(method)) {
      headers.Origin ??= origin;
      if (this.csrf && options.csrf !== false) headers['X-CSRF-Token'] = this.csrf;
    }
    if (body !== undefined) headers['Content-Type'] ??= 'application/json';
    const response = await fetch(base + route, { method, headers, body: options.raw ?? (body === undefined ? undefined : JSON.stringify(body)), signal: AbortSignal.timeout(90000) });
    const cookie = response.headers.getSetCookie().find(value => value.startsWith('ctc_session='));
    if (cookie) this.cookie = cookie.split(';')[0];
    const text = await response.text();
    let data;
    try { data = JSON.parse(text); } catch { data = text; }
    return { status: response.status, headers: response.headers, data, cookie };
  }
  async login(email, password = this.password) {
    const response = await this.request('POST', '/auth/login', { email, password });
    const data = expect(response, 200);
    this.csrf = data.csrfToken; this.user = data.user; this.password = password;
    assert.ok(this.cookie && this.csrf && this.user.id);
    assert.match(response.cookie, /HttpOnly/i);
    assert.match(response.cookie, /SameSite=Strict/i);
    if (production) assert.match(response.cookie, /Secure/i);
    return response;
  }
}
async function check(name, action) {
  const start = Date.now();
  try { await action(); outcomes.push({ name, passed: true, milliseconds: Date.now() - start }); console.log('PASS ' + name); }
  catch (error) { outcomes.push({ name, passed: false, error: error.stack || String(error) }); console.error('FAIL ' + name + '\n' + error.stack); }
}
async function createUser(role, label) {
  const email = `security-${runId}-${label}@fixture.local`;
  const user = expect(await admin.request('POST', '/users', { name: `TEST_FIXTURE ${label}`, email, password: temporaryPassword, role }), 201);
  const client = new Client();
  await client.login(email);
  assert.equal(client.user.id, user.id);
  if (client.user.mustChangePassword) {
    expect(await client.request('GET', '/bootstrap'), 403);
    expect(await client.request('POST', '/auth/password', { currentPassword: temporaryPassword, newPassword: enrolledPassword }), 200);
    expect(await client.request('GET', '/auth/me'), 401);
    await client.login(email, enrolledPassword);
    assert.equal(client.user.mustChangePassword, false);
  }
  return client;
}
function completeFixture() {
  const benefits = ['SUPERANNUATION','ANNUAL_SPECIAL','SIXTH_DAY','FIELD_SERVICE','RELOCATION','DEPUTATION','ADDITIONAL_HRA','PERFORMANCE','ADDITIONAL_PERFORMANCE','LTM1','LTM2'].map(component => ({ component, kind: 'none', period: component === 'SUPERANNUATION' ? 'monthly' : 'annual', notes: 'TEST_FIXTURE explicit non-applicability; not company policy' }));
  return { name: 'TEST_FIXTURE security ' + runId, source: 'client', effectiveFrom: '2026-01-01', effectiveTo: '2026-12-31', data: {
    annualSpecialGroup: 'statutory', basicMethod: 'manual', basicMethodConfirmed: true,
    grades: [{ id: 'security-grade', name: 'TEST_FIXTURE grade', basicMin: '7000', basicMax: '50000', conveyance: '7300', conveyanceCap: '7300', professionalAllowance: { CA: '1000', CMA: '1000', CS: '1000' } }],
    locations: [{ id: 'security-location', name: 'TEST_FIXTURE location' }],
    pf: { basicThreshold: '25000', inclusionShare: '0.5', basisCap: '25000', rate: '0.12' }, nps: { oldRate: '0.10', newRate: '0.14' },
    esi: { rate: '0.0325', ceiling: '21000', eligibilityBasis: 'gross', contributionBasis: 'gross', ceilingInclusive: true }, gratuityRate: '0.0481', benefits,
    rounding: { scale: 2, mode: 'half_up', confirmed: true }, annualizationConfirmed: true, hraApproved: true, templateApproved: true,
    offerTemplate: { companyName: 'TEST_FIXTURE COMPANY', address: 'Test environment only', signatory: 'Test Signatory', body: 'Synthetic security acceptance wording. This is not an employment offer.' }
  } };
}
const saveBody = overrides => ({ configurationId: config.id, input, ...overrides });

await check('isolated fixture setup and protected authentication cookies', async () => {
  admin = new Client();
  const email = process.env.TEST_ADMIN_EMAIL || (!production ? 'admin@demo.local' : '');
  const password = process.env.TEST_ADMIN_PASSWORD || (!production ? process.env.DEMO_PASSWORD || 'DemoCTC!2026' : '');
  assert.ok(email && password, 'Production-mode test instance requires TEST_ADMIN_EMAIL and TEST_ADMIN_PASSWORD');
  await admin.login(email, password);
  assert.equal(admin.user.mustChangePassword, false, 'Enroll the isolated test administrator before running this suite');
  cb = await createUser('cb', 'cb');
  hr1 = await createUser('hr', 'hr-one');
  hr2 = await createUser('hr', 'hr-two');
  config = expect(await cb.request('POST', '/configurations', completeFixture()), 201);
  config = expect(await cb.request('POST', `/configurations/${config.id}/publish`, {}), 200);
  input = { candidateName: 'TEST_FIXTURE Security Candidate', candidateReference: 'SEC-' + runId, designation: 'Test Engineer', department: 'Acceptance', joiningDate: '2027-01-01', calculationDate: '2026-11-16', locationId: 'security-location', gradeId: 'security-grade', qualification: 'NONE', gross: '100000', basic: '20000', taxRegime: 'new', npsOptIn: true, job: { sixthDay: false, fieldService: false, relocation: false, deputation: false, additionalHra: false }, ctcYear: 1, notes: 'Synthetic security fixture only' };
  record = expect(await hr1.request('POST', '/cases', saveBody()), 201);
  secondRecord = expect(await hr1.request('POST', '/cases', saveBody({ input: { ...input, candidateReference: 'SEC-SECOND-' + runId } })), 201);
});

await check('strict request JSON, explicit elections, and profile bounds', async () => {
  expect(await hr1.request('POST', '/calculate', { ...saveBody(), unexpected: true }), 400);
  expect(await hr1.request('POST', '/calculate', saveBody({ input: { ...input, grosss: '1' } })), 400);
  expect(await hr1.request('POST', '/calculate', saveBody({ input: { ...input, job: { relocation: null } } })), 400);
  expect(await hr1.request('POST', '/calculate', saveBody({ input: { ...input, npsOptIn: null } })), 400);
  expect(await hr1.request('POST', '/calculate', saveBody(), { headers: { 'Content-Type': 'text/plain' } }), 415);
  expect(await hr1.request('POST', '/calculate', saveBody(), { raw: JSON.stringify(saveBody()) + '{}' }), 400);
  expect(await hr1.request('POST', '/cases', saveBody({ input: { ...input, joiningDate: '2026-02-30' } })), 422);
  expect(await hr1.request('POST', '/cases', saveBody({ input: { ...input, candidateName: 'x'.repeat(201) } })), 422);
});

await check('CSRF, role boundaries, and cross-origin rejection', async () => {
  expect(await hr1.request('POST', '/calculate', saveBody(), { csrf: false }), 403);
  expect(await hr1.request('POST', '/calculate', saveBody(), { headers: { Origin: 'https://untrusted.invalid' } }), 403);
  expect(await hr1.request('POST', '/configurations', completeFixture()), 403);
  expect(await admin.request('POST', '/configurations', completeFixture()), 403);
  expect(await cb.request('GET', '/users'), 403);
  expect(await hr1.request('GET', '/users'), 403);
  expect(await new Client().request('GET', '/cases/' + record.caseId), 401);
});

await check('session settings require administrator authority and current password', async () => {
  expect(await hr1.request('GET', '/settings'), 403);
  expect(await cb.request('GET', '/settings'), 403);
  const current = expect(await admin.request('GET', '/settings'), 200);
  assert.ok(current.sessionIdleMinutes >= 5 && current.sessionIdleMinutes <= 60);
  assert.ok(current.sessionMaxHours >= 1 && current.sessionMaxHours <= 24);
  const settings = { sessionIdleMinutes: current.sessionIdleMinutes, sessionMaxHours: current.sessionMaxHours };
  expect(await hr1.request('PATCH', '/settings', { ...settings, currentPassword: hr1.password }), 403);
  expect(await cb.request('PATCH', '/settings', { ...settings, currentPassword: cb.password }), 403);
  expect(await admin.request('PATCH', '/settings', { ...settings, currentPassword: 'IncorrectFixture!2026' }), 400);
  expect(await admin.request('PATCH', '/settings', { ...settings, sessionIdleMinutes: 4, currentPassword: admin.password }), 400);
  expect(await admin.request('PATCH', '/settings', { ...settings, sessionMaxHours: 25, currentPassword: admin.password }), 400);
  // Preserve the existing operational values while verifying the authorized
  // settings transaction and audit. Other suites can share this test instance.
  expect(await admin.request('PATCH', '/settings', { ...settings, currentPassword: admin.password }), 200);
  const after = expect(await admin.request('GET', '/settings'), 200);
  assert.equal(after.sessionIdleMinutes, settings.sessionIdleMinutes);
  assert.equal(after.sessionMaxHours, settings.sessionMaxHours);
});

await check('two HR users cannot read, modify, enumerate or render each other records', async () => {
  expect(await hr2.request('GET', '/cases/' + record.caseId), 404);
  expect(await hr2.request('POST', '/cases', saveBody({ caseId: record.caseId, expectedRevisionId: record.revisionId })), 404);
  expect(await hr2.request('GET', `/revisions/${record.revisionId}/documents/ctc?format=html&mode=draft`), 404);
  expect(await hr2.request('GET', `/revisions/${record.revisionId}/documents/offer?format=pdf&mode=final`), 404);
  const records = expect(await hr2.request('GET', '/cases?limit=100&offset=0'), 200);
  assert.ok(!records.some(item => item.id === record.caseId || item.id === secondRecord.caseId));
});

await check('optimistic revisions reject a stale update and preserve history', async () => {
  const previous = record.revisionId;
  const next = expect(await hr1.request('POST', '/cases', saveBody({ caseId: record.caseId, expectedRevisionId: previous, input: { ...input, npsOptIn: false } })), 201);
  expect(await hr1.request('POST', '/cases', saveBody({ caseId: record.caseId, expectedRevisionId: previous })), 409);
  const revisions = expect(await hr1.request('GET', '/cases/' + record.caseId), 200).revisions;
  assert.equal(revisions.length, 2);
  assert.equal(revisions.find(item => item.id === previous).input.npsOptIn, true);
  assert.equal(revisions.find(item => item.id === next.revisionId).input.npsOptIn, false);
  record = next;
});

await check('simultaneous saves from one revision commit exactly one successor', async () => {
  const previous = record.revisionId;
  const attempts = await Promise.all([
    hr1.request('POST', '/cases', saveBody({ caseId: record.caseId, expectedRevisionId: previous, input: { ...input, notes: 'TEST_FIXTURE competing update A' } })),
    hr1.request('POST', '/cases', saveBody({ caseId: record.caseId, expectedRevisionId: previous, input: { ...input, notes: 'TEST_FIXTURE competing update B' } }))
  ]);
  assert.deepEqual(attempts.map(response => response.status).sort(), [201, 409]);
  record = attempts.find(response => response.status === 201).data;
  const revisions = expect(await hr1.request('GET', '/cases/' + record.caseId), 200).revisions;
  assert.equal(revisions.length, 3);
  assert.deepEqual(revisions.map(revision => revision.number).sort(), [1, 2, 3]);
  assert.equal(revisions.filter(revision => /competing update/.test(revision.input.notes)).length, 1);
});

await check('bounded pagination is stable and invalid parameters are rejected', async () => {
  const first = expect(await hr1.request('GET', '/cases?limit=1&offset=0'), 200);
  const second = expect(await hr1.request('GET', '/cases?limit=1&offset=1'), 200);
  assert.equal(first.length, 1); assert.equal(second.length, 1); assert.notEqual(first[0].id, second[0].id);
  for (const query of ['limit=-1', 'limit=1001', 'offset=-1', 'limit=hello']) expect(await hr1.request('GET', '/cases?' + query), 400);
});

await check('password changes verify current password and revoke every existing session', async () => {
  const parallel = new Client(); await parallel.login(hr1.user.email, hr1.password);
  expect(await hr1.request('POST', '/auth/password', { currentPassword: 'IncorrectFixture!2026', newPassword: changedPassword }), 400);
  expect(await hr1.request('POST', '/auth/password', { currentPassword: hr1.password, newPassword: 'short' }), 400);
  expect(await hr1.request('POST', '/auth/password', { currentPassword: hr1.password, newPassword: changedPassword }), 200);
  expect(await hr1.request('GET', '/auth/me'), 401);
  expect(await parallel.request('GET', '/auth/me'), 401);
  expect(await new Client().request('POST', '/auth/login', { email: hr1.user.email, password: hr1.password }), 401);
  await hr1.login(hr1.user.email, changedPassword);
});

await check('only administrators can reassign records and ownership changes immediately', async () => {
  expect(await hr1.request('PATCH', `/cases/${record.caseId}/owner`, { ownerUserId: hr2.user.id }), 403);
  expect(await cb.request('PATCH', `/cases/${record.caseId}/owner`, { ownerUserId: hr2.user.id }), 403);
  expect(await admin.request('PATCH', `/cases/${record.caseId}/owner`, { ownerUserId: cb.user.id }), 400);
  expect(await admin.request('PATCH', `/cases/${record.caseId}/owner`, { ownerUserId: hr2.user.id }), 200);
  expect(await hr1.request('GET', '/cases/' + record.caseId), 404);
  expect(await hr1.request('GET', `/revisions/${record.revisionId}/documents/ctc?format=html&mode=draft`), 404);
  expect(await hr2.request('GET', '/cases/' + record.caseId), 200);
  expect(await hr2.request('GET', `/revisions/${record.revisionId}/documents/ctc?format=html&mode=draft`), 200);
});

await check('effective dates block out-of-window calculations', async () => {
  const result = expect(await hr2.request('POST', '/calculate', saveBody({ input: { ...input, calculationDate: '2027-01-01' } })), 200);
  assert.equal(result.canFinalize, false);
  assert.ok(result.issues.some(issue => issue.code === 'CONFIG_NOT_EFFECTIVE'));
  expect(await hr2.request('POST', '/cases', saveBody({ input: { ...input, calculationDate: '2027-01-01' } })), 422);
  if (production) {
    const { calculationDate, ...undated } = input;
    const response = await hr2.request('POST', '/calculate', saveBody({ input: undated }));
    assert.ok([400, 422].includes(response.status), 'Production calculations require an explicit policy date');
    const illustration = completeFixture(); illustration.source = 'urd-demo';
    expect(await cb.request('POST', '/configurations', illustration), 400);
    const incomplete = completeFixture(); incomplete.data.esi.ceiling = null;
    const draft = expect(await cb.request('POST', '/configurations', incomplete), 201);
    expect(await cb.request('POST', `/configurations/${draft.id}/publish`, {}), 422);
  }
});

await check('audit trail covers authentication changes and ownership without secrets', async () => {
  const events = expect(await admin.request('GET', '/audit?limit=100&offset=0'), 200);
  assert.ok(events.some(event => event.entityId === hr1.user.id && /password/.test(event.action)), 'Password change must be audited');
  assert.ok(events.some(event => event.entityId === record.caseId && /assign|owner/.test(event.action)), 'Assignment must be audited');
  assert.ok(events.some(event => /settings/.test(event.action)), 'Application settings updates must be audited');
  const serialized = JSON.stringify(events);
  for (const secret of [temporaryPassword, enrolledPassword, changedPassword, admin.password, hr1.csrf, hr1.cookie.split('=')[1]]) assert.ok(!serialized.includes(secret), 'Audit must not contain passwords, CSRF tokens or session secrets');
});

if (process.env.TEST_DB_CONTAINER) {
  await check('idle sessions expire using isolated database clock adjustment', async () => {
    const container = process.env.TEST_DB_CONTAINER;
    const database = process.env.TEST_DB_NAME || 'ctc_test';
    assert.match(container, /^(ctc-generator-test|ctc-security-test|ctc-upgrade-test)-[a-z0-9-]+$/);
    assert.match(database, /^ctc_[a-z0-9_]*test$/);
    assert.match(hr2.user.id, /^[a-f0-9]{32}$/);
    const settings = expect(await admin.request('GET', '/settings'), 200);
    assert.ok(Number.isInteger(settings.sessionIdleMinutes) && settings.sessionIdleMinutes >= 5 && settings.sessionIdleMinutes <= 60);
    execFileSync('docker', ['exec', container, 'psql', '-U', process.env.TEST_DB_USER || 'ctc', '-d', database, '-v', 'ON_ERROR_STOP=1', '-c', `UPDATE sessions SET last_seen_at=now()-interval '${settings.sessionIdleMinutes + 1} minutes' WHERE user_id='${hr2.user.id}'`], { stdio: 'pipe' });
    expect(await hr2.request('GET', '/auth/me'), 401);
  });
} else {
  outcomes.push({ name: 'idle session expiration database integration', skipped: true, reason: 'Set TEST_DB_CONTAINER to an isolated ctc-generator-test database container to exercise clock adjustment' });
}

await mkdir(artifacts, { recursive: true });
await writeFile(path.join(artifacts, production ? 'production-security-results.json' : 'security-results.json'), JSON.stringify({ base, production, at: new Date().toISOString(), outcomes }, null, 2));
const failed = outcomes.filter(outcome => outcome.passed === false);
console.log(`Security acceptance: ${outcomes.filter(outcome => outcome.passed).length} passed, ${failed.length} failed, ${outcomes.filter(outcome => outcome.skipped).length} skipped`);
if (failed.length) process.exitCode = 1;
