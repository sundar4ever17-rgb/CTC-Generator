// Synthetic integration fixtures. Never run against the working review database.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

assert.equal(process.env.CTC_TEST_FIXTURES, '1', 'Use a dedicated test environment');
const base = (process.env.TEST_BASE_URL || 'http://host.docker.internal:8094').replace(/\/$/, '').replace(/\/api$/, '') + '/api';
const origin = process.env.TEST_ORIGIN || 'http://localhost:3024';
const production = process.env.TEST_PRODUCTION === '1';
const artifacts = process.env.ARTIFACT_DIR || '/artifacts';
const run = Date.now().toString(36);
const outcomes = [];
class Client {
  cookie = ''; csrf = '';
  async request(method, route, body) {
    const response = await fetch(base + route, { method, headers: { Origin: origin, Cookie: this.cookie, 'X-CSRF-Token': this.csrf, 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(90000) });
    const cookie = response.headers.getSetCookie().find(v => v.startsWith('ctc_session='));
    if (cookie) this.cookie = cookie.split(';')[0];
    const bytes = Buffer.from(await response.arrayBuffer());
    let data; try { data = JSON.parse(bytes.toString()); } catch { data = bytes.toString(); }
    return { status: response.status, data, bytes, headers: response.headers };
  }
  async login(email, password) {
    const data = ok(await this.request('POST', '/auth/login', { email, password }));
    this.csrf = data.csrfToken;
    if (data.user.mustChangePassword) {
      ok(await this.request('POST', '/auth/password', { currentPassword: password, newPassword: 'RulesEnrolled!2026' }));
      await this.login(email, 'RulesEnrolled!2026');
    }
  }
}
function ok(response, status = 200) { assert.equal(response.status, status, JSON.stringify(response.data)); return response.data; }
function line(result, code) { const found = result.lines.find(l => l.code === code); assert.ok(found, `Missing ${code}`); return found; }
const number = value => ({ op: 'number', value: String(value) });
const ref = value => ({ op: 'ref', ref: value });
const expression = (op, ...args) => ({ op, args });
async function check(name, action) {
  try { await action(); console.log('PASS ' + name); outcomes.push({ name, passed: true }); }
  catch (error) { outcomes.push({ name, passed: false, error: error.stack }); console.error('FAIL ' + name + '\n' + error.stack); throw error; }
}
const cb = new Client(), hr = new Client();
const input = { candidateName: 'TEST_FIXTURE Rules Candidate', candidateReference: 'RULES-' + run, designation: 'Test Engineer', department: 'Test', joiningDate: '2026-12-01', calculationDate: '2026-11-18', locationId: 'test-location', gradeId: 'test-grade', qualification: 'NONE', gross: '100000', basic: '20000', taxRegime: 'new', npsOptIn: true, job: { sixthDay: false, fieldService: false, relocation: false, deputation: false, additionalHra: false }, ctcYear: 1, notes: 'Synthetic acceptance data only' };
const benefits = ['SUPERANNUATION','ANNUAL_SPECIAL','SIXTH_DAY','FIELD_SERVICE','RELOCATION','DEPUTATION','ADDITIONAL_HRA','PERFORMANCE','ADDITIONAL_PERFORMANCE','LTM1','LTM2'];
const legacy = { name: 'TEST_FIXTURE configurable baseline ' + run, source: 'client', effectiveFrom: '2026-01-01', effectiveTo: '2027-12-31', data: {
  annualSpecialGroup: 'statutory', grades: [{ id: 'test-grade', name: 'Test grade', basicMin: '7000', basicMax: '50000', conveyance: '7300', conveyanceCap: '7300', professionalAllowance: { CA: '1000', CMA: '1000', CS: '1000' } }], locations: [{ id: 'test-location', name: 'Test location' }], basicMethod: 'manual', basicMethodConfirmed: true,
  pf: { basicThreshold: '25000', inclusionShare: '0.5', basisCap: '25000', rate: '0.12' }, nps: { oldRate: '0.10', newRate: '0.14' }, esi: { rate: '0.0325', ceiling: '21000', eligibilityBasis: 'gross', contributionBasis: 'gross', ceilingInclusive: true }, gratuityRate: '0.0481',
  benefits: benefits.map(component => ({ component, kind: 'none', period: component === 'SUPERANNUATION' ? 'monthly' : 'annual', notes: 'TEST_FIXTURE explicit non-applicability' })), rounding: { scale: 2, mode: 'half_up', confirmed: true }, annualizationConfirmed: true, hraApproved: true, templateApproved: true,
  offerTemplate: { companyName: 'TEST_FIXTURE COMPANY', address: 'Synthetic environment', signatory: 'Test Signatory', body: 'Synthetic offer wording for acceptance tests only.' }
} };
let converted, initialSaved, originalRevision, originalPDF, dynamic, dynamicSaved, dynamicResult;
const preview = async (configuration, candidate = input) => ok(await cb.request('POST', '/configurations/preview', { configuration, input: candidate }));
try {
  await check('C&B and HR sessions', async () => {
    if (production) {
      const admin = new Client(); await admin.login(process.env.TEST_ADMIN_EMAIL, process.env.TEST_ADMIN_PASSWORD);
      for (const [role, client] of [['cb', cb], ['hr', hr]]) {
        const email = `rules-${role}-${run}@fixture.local`;
        ok(await admin.request('POST', '/users', { name: `TEST_FIXTURE Rules ${role}`, email, password: 'RulesTemporary!2026', role }), 201);
        await client.login(email, 'RulesTemporary!2026');
      }
    } else {
      await cb.login('cb@demo.local', 'DemoCTC!2026'); await hr.login('hr@demo.local', 'DemoCTC!2026');
    }
  });
  await check('conversion and preview do not save configuration; HR cannot author rules', async () => {
    const before = ok(await cb.request('GET', '/configurations?limit=100'));
    converted = ok(await cb.request('POST', '/configurations/convert', legacy));
    assert.equal(converted.data.rules.version, 2); assert.equal(converted.data.rules.components.length, 20);
    const expected = await preview(legacy), actual = await preview(converted);
    for (const item of expected.lines) {
      const counterpart = line(actual, item.code);
      for (const key of ['amount','monthly','annual','status','period','group']) assert.equal(counterpart[key], item[key], `${item.code}.${key}`);
    }
    assert.deepEqual(actual.totals, expected.totals);
    assert.equal(actual.coreValid, true); assert.equal(actual.complete, true);
    assert.deepEqual(ok(await cb.request('GET', '/configurations?limit=100')), before);
    ok(await hr.request('POST', '/configurations/convert', legacy), 403);
    ok(await hr.request('POST', '/configurations/preview', { configuration: converted, input }), 403);
  });
  await check('existing policy and saved PDF remain usable', async () => {
    const saved = ok(await cb.request('POST', '/configurations', legacy), 201);
    ok(await cb.request('POST', `/configurations/${saved.id}/publish`, {}));
    initialSaved = ok(await hr.request('POST', '/cases', { configurationId: saved.id, input }), 201);
    originalRevision = ok(await hr.request('GET', `/cases/${initialSaved.caseId}`)).revisions[0];
    const pdf = await hr.request('GET', `/revisions/${initialSaved.revisionId}/documents/ctc?mode=final&format=pdf`);
    ok(pdf); assert.equal(pdf.bytes.subarray(0, 5).toString(), '%PDF-'); originalPDF = pdf.bytes;
  });
  await check('new components, custom conditions, dependencies and category calculate automatically', async () => {
    dynamic = structuredClone(converted); dynamic.name = 'TEST_FIXTURE dynamic rules ' + run;
    dynamic.data.rules.inputs.push({ key: 'skillAmount', label: 'Skill amount', type: 'money', required: true }, { key: 'remote', label: 'Remote assignment', type: 'boolean', required: true });
    dynamic.data.rules.groups.push({ code: 'recognition', name: 'Recognition payments', inGross: false });
    dynamic.data.rules.components.push(
      { code: 'SKILL', name: 'Skill Allowance', group: 'inclusion', rules: [{ id: 'skill-input', expression: ref('input.custom.skillAmount'), period: 'monthly' }] },
      { code: 'REMOTE', name: 'Remote Allowance', group: 'exclusion', rules: [{ id: 'remote-rate', eligibility: ref('input.custom.remote'), expression: expression('mul', ref('component.BASIC'), number('0.05')), period: 'monthly' }] },
      { code: 'RECOGNITION', name: 'Recognition Award', group: 'recognition', rules: [{ id: 'recognition-annual', expression: expression('add', ref('component.SKILL'), number('4000')), period: 'annual' }] }
    );
    dynamicResult = await preview(dynamic, { ...input, custom: { skillAmount: '3000', remote: true } });
    assert.equal(dynamicResult.coreValid, true, JSON.stringify(dynamicResult.issues));
    assert.equal(dynamicResult.complete, true, JSON.stringify(dynamicResult.issues));
    assert.equal(line(dynamicResult, 'SKILL').monthly, '3000.00'); assert.equal(line(dynamicResult, 'SPECIAL').monthly, '27000.00');
    assert.equal(line(dynamicResult, 'REMOTE').monthly, '1000.00'); assert.equal(line(dynamicResult, 'HRA').monthly, '35900.00');
    assert.equal(line(dynamicResult, 'RECOGNITION').annual, '7000.00');
    assert.equal(dynamicResult.totals.inclusion, '50000.00'); assert.equal(dynamicResult.totals.exclusion, '50000.00');
    assert.equal(dynamicResult.totals.annualCTC, '1235860.00');
    assert.equal(dynamicResult.groups.find(g => g.code === 'recognition').name, 'Recognition payments');
    assert(dynamicResult.trace.some(t => t.component === 'RECOGNITION'));
  });
  await check('missing inputs stay unresolved; explicit false is not applicable; negative balancing is blocked', async () => {
    const missing = await preview(dynamic); assert.equal(missing.coreValid, false); assert.equal(missing.canFinalize, false); assert.equal(line(missing, 'SKILL').amount, null);
    const declined = await preview(dynamic, { ...input, custom: { skillAmount: '3000', remote: false } });
    assert.equal(line(declined, 'REMOTE').status, 'not_applicable'); assert.equal(line(declined, 'REMOTE').amount, '0.00'); assert.equal(line(declined, 'HRA').amount, '36900.00');
    const invalid = await preview(dynamic, { ...input, custom: { skillAmount: '60000', remote: true } });
    assert.equal(invalid.coreValid, false); assert.equal(invalid.canFinalize, false);
  });
  await check('formula graph rejects cycles, unknown references and unsupported format', async () => {
    for (const change of [
      c => { c.data.rules.components.find(x => x.code === 'SKILL').rules[0].expression = ref('component.SPECIAL'); },
      c => { c.data.rules.components.find(x => x.code === 'SKILL').rules[0].expression = ref('component.MISSING'); },
      c => { c.data.rules.components.find(x => x.code === 'SKILL').rules[0].expression = { op: 'boolean', value: 'true' }; },
      c => { c.data.rules.components.find(x => x.code === 'SKILL').rules[0].expression = { op: 'execute', value: 'arbitrary script' }; },
      c => { c.data.rules.version = 999; }
    ]) {
      const broken = structuredClone(dynamic); change(broken);
      ok(await cb.request('POST', '/configurations', broken), 400);
    }
  });
  await check('configurable targets reconcile individually without changing the baseline', async () => {
    const ratio = structuredClone(converted);
    ratio.data.rules.groups.find(g => g.code === 'inclusion').target = expression('mul', ref('input.gross'), number('0.6'));
    ratio.data.rules.groups.find(g => g.code === 'exclusion').target = expression('mul', ref('input.gross'), number('0.4'));
    const result = await preview(ratio);
    assert.equal(result.coreValid, true, JSON.stringify(result.issues)); assert.equal(result.totals.inclusion, '60000.00'); assert.equal(result.totals.exclusion, '40000.00');
    assert(result.groups.filter(g => g.inGross).every(g => g.valid && g.difference === '0.00'));
    assert.equal((await preview(converted)).totals.inclusion, '50000.00');
    ratio.data.rules.groups.find(g => g.code === 'exclusion').target = expression('mul', ref('input.gross'), number('0.5'));
    assert.equal((await preview(ratio)).coreValid, false, 'Targets which do not sum to Gross must block generation');
  });
  await check('Basic can be calculated and saved without a manual Basic input', async () => {
    const calculated = structuredClone(converted); calculated.name = 'TEST_FIXTURE calculated Basic ' + run;
    calculated.data.rules.components.find(c => c.code === 'BASIC').rules[0].expression = expression('mul', ref('input.gross'), number('0.2'));
    const candidate = { ...input, basic: '' };
    const result = await preview(calculated, candidate);
    assert.equal(result.coreValid, true, JSON.stringify(result.issues));
    assert.equal(line(result, 'BASIC').amount, '20000.00'); assert.equal(line(result, 'NPS').amount, '2800.00');
    const saved = ok(await cb.request('POST', '/configurations', calculated), 201);
    ok(await cb.request('POST', `/configurations/${saved.id}/publish`, {}));
    const stored = ok(await hr.request('POST', '/cases', { configurationId: saved.id, input: candidate }), 201);
    const revision = ok(await hr.request('GET', `/cases/${stored.caseId}`)).revisions[0];
    assert.equal(revision.input.basic, ''); assert.equal(line(revision.result, 'BASIC').amount, '20000.00');
  });
  await check('new rule policy saves, publishes, persists a revision and appears in final documents', async () => {
    const saved = ok(await cb.request('POST', '/configurations', dynamic), 201);
    const reloaded = ok(await cb.request('GET', `/configurations/${saved.id}`)); assert.deepEqual(reloaded.data.rules, saved.data.rules);
    ok(await cb.request('POST', `/configurations/${saved.id}/publish`, {}));
    dynamicSaved = ok(await hr.request('POST', '/cases', { configurationId: saved.id, input: { ...input, custom: { skillAmount: '3000', remote: true } }, caseId: initialSaved.caseId, expectedRevisionId: initialSaved.revisionId }), 201);
    const details = ok(await hr.request('GET', `/cases/${initialSaved.caseId}`));
    const newest = details.revisions.find(r => r.id === dynamicSaved.revisionId);
    assert.equal(line(newest.result, 'SPECIAL').amount, '27000.00'); assert.notEqual(newest.result.engineVersion, originalRevision.result.engineVersion);
    assert.deepEqual(details.revisions.find(r => r.id === initialSaved.revisionId), originalRevision);
    for (const kind of ['ctc','offer']) {
      const html = await hr.request('GET', `/revisions/${dynamicSaved.revisionId}/documents/${kind}?mode=final&format=html`); ok(html);
      for (const label of ['Skill Allowance','Recognition payments','Recognition Award','Remote Allowance']) assert(html.data.includes(label), `Missing ${label} in ${kind}`);
      const pdf = await hr.request('GET', `/revisions/${dynamicSaved.revisionId}/documents/${kind}?mode=final&format=pdf`); ok(pdf);
      assert.equal(pdf.bytes.subarray(0, 5).toString(), '%PDF-');
      await mkdir(artifacts, { recursive: true }); await writeFile(path.join(artifacts, `rules-${kind}-final.pdf`), pdf.bytes);
    }
    const oldPDF = await hr.request('GET', `/revisions/${initialSaved.revisionId}/documents/ctc?mode=final&format=pdf`); ok(oldPDF); assert.deepEqual(oldPDF.bytes, originalPDF);
    const oldConfig = ok(await cb.request('GET', `/configurations/${originalRevision.configurationId}`)); assert.equal(oldConfig.data.rules, undefined);
  });
} finally {
  await mkdir(artifacts, { recursive: true }); await writeFile(path.join(artifacts, 'rules-api-results.json'), JSON.stringify({ outcomes, passed: outcomes.every(x => x.passed) }, null, 2));
}
