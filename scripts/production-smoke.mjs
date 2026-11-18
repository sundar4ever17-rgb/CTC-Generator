// Synthetic isolated-test runner. Never point this at a live application.
import assert from 'node:assert/strict';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
assert.equal(process.env.CTC_TEST_FIXTURES, '1');
const base = process.env.TEST_BASE_URL + '/api';
const origin = process.env.TEST_ORIGIN;
const out = process.env.ARTIFACT_DIR || '/artifacts';
let cookie = '', csrf = '';
async function request(method, route, body) {
  const headers = { Origin: origin, 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf };
  const response = await fetch(base + route, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(90000) });
  const set = response.headers.getSetCookie().find(value => value.startsWith('ctc_session='));
  if (set) { cookie = set.split(';')[0]; assert.match(set, /Secure/); }
  assert.equal(response.status, 200, `${method} ${route}: ${response.status} ${response.ok ? '' : await response.text()}`);
  return response;
}
const enroll = process.argv[2] === 'enroll';
const login = await (await request('POST', '/auth/login', { email: process.env.TEST_ADMIN_EMAIL, password: enroll ? process.env.TEST_INITIAL_PASSWORD : process.env.TEST_ADMIN_PASSWORD })).json();
csrf = login.csrfToken;
if (enroll) {
  assert.equal(login.user.mustChangePassword, true);
  await request('POST', '/auth/password', { currentPassword: process.env.TEST_INITIAL_PASSWORD, newPassword: process.env.TEST_ADMIN_PASSWORD });
  console.log('PASS production bootstrap enrollment and secure cookie');
} else {
  assert.equal(login.user.mustChangePassword, false);
  const cases = await (await request('GET', '/cases?limit=100')).json();
  const record = cases.find(item => item.candidateReference?.startsWith('SEC-')) || cases[0];
  assert.ok(record, 'Security suite must create a synthetic case first');
  const details = await (await request('GET', '/cases/' + record.id)).json();
  const revision = details.revisions[0];
  assert.ok(revision);
  await mkdir(out, { recursive: true });
  for (const kind of ['ctc', 'offer']) {
    const route = `/revisions/${revision.id}/documents/${kind}?format=pdf&mode=final`;
    const response = await request('GET', route);
    const bytes = Buffer.from(await response.arrayBuffer());
    assert.equal(bytes.subarray(0, 5).toString(), '%PDF-');
    assert.ok(bytes.length > 10000);
    const saved = Buffer.from(await (await request('GET', route)).arrayBuffer());
    assert.deepEqual(saved, bytes, 'Saved PDF bytes remain immutable');
    await writeFile(path.join(out, `production-${kind}-final.pdf`), bytes);
    console.log(`PASS production sandboxed ${kind} PDF and cached byte equality (${bytes.length} bytes)`);
  }
}
