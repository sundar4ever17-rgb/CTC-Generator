# Frontend production upgrade

The existing Next.js and TypeScript application was retained. The API remains authoritative for authorization, calculation, effective-date selection and document eligibility.

## Gaps corrected

| Demo gap | Implemented behavior |
| --- | --- |
| Public credentials prefilled and displayed on sign-in | Empty credential fields; no account/password hints in the login UI |
| Fictional candidate and salary defaults used everywhere | Production starts with blank inputs; the URD illustration is available only when the authenticated server bootstrap explicitly reports demo mode |
| Session expiry retained candidate data in parent state | All candidate, case, revision, configuration and modal state is cleared; delayed responses cannot restore another account's editing state |
| No personal password-change workflow | Own-password form and a mandatory password-change screen before workspace bootstrap |
| User edits sent role/active even when unchanged | Only changed access fields are submitted, preserving sessions for name-only edits; self-deactivation/demotion controls are disabled |
| No first-configuration workflow in an empty production database | C&B can author the first configuration with empty grades, locations, statutory values and policy data |
| Effective dates were unavailable in the editor | Configuration start/end dates and a distinct calculation effective date are editable |
| Large record/audit lists loaded without navigation | Server search and bounded pages; revision history pages; older configurations can be loaded and historical revisions fetch their configuration on demand |
| Case ownership had no administrator workflow | Administrators can assign records to an active HR user |
| No administrative session-controls screen | Administrator-only idle/absolute session settings with current-password confirmation |
| Document fetches bypassed session handling | Shared authenticated downloads handle expiry, timeouts, content type and safe filenames |
| HTML lacked a script policy | Per-response nonce CSP with dynamic rendering; no inline-script or eval allowance in production; frame, MIME, referrer and browser permission headers |
| Container dependencies were resolved with npm install | Reproducible installation from the lockfile using npm ci |
| Extended navigation could push sign-out below short screens | Scrollable sidebar, verified at a 720px viewport height |

Configuration JSON is checked for an editable structure before being applied. Server validation still determines valid business rules. The empty editor's rounding controls are unconfirmed; no business configuration is published automatically.

## Verification

Verified on 16 November 2026 in the isolated `compose.upgrade-test.yaml` stack, frontend port 3022. Existing demo services and their data were not replaced.

- TypeScript check and optimized Docker/Next.js build passed.
- `npm audit --omit=dev --json` reported zero known production dependency vulnerabilities at the time of this run.
- The original browser suite passed all 12 workflows: sign-in, URD calculations and parity, NPS/tax changes, invalid residual blocking, both draft PDFs, revision history, HR read-only configuration, mobile layout, auditor restrictions, C&B editor, administrator users and uncaught-error checks. Outputs are in `artifacts/production-upgrade/browser`.
- `frontend/tests/browser-security.mjs` verifies empty login fields, changing CSP nonces, blocked inline event handlers, no-store responses, cross-account state clearing, production inputs without sample data, first C&B configuration authoring and forced password changes. Production bootstrap/password-required responses are mocked for these UI states; production API enforcement is covered separately by backend tests.
- `frontend/tests/history-settings-browser.mjs` checks the real administrator settings endpoint with current values and password confirmation, then exercises 51-revision navigation and loading a configuration missing from bootstrap. The extended revision list is mocked in the browser, avoiding synthetic persisted salary history.

Run the additional browser checks using the existing QA image:

```powershell
docker run --rm -e APP_URL=http://localhost:3022 -v "${PWD}/frontend/tests/browser-security.mjs:/qa/frontend-security.mjs:ro" ctc-generator-qa node frontend-security.mjs
docker run --rm -e APP_URL=http://localhost:3022 -v "${PWD}/frontend/tests/history-settings-browser.mjs:/qa/history-settings-browser.mjs:ro" ctc-generator-qa node history-settings-browser.mjs
```

The earlier acceptance harness rewrote Origin headers and masked a local port mismatch. That global interception has been removed, and the upgrade API now expects its actual browser origin, `http://localhost:3022`. Chromium maps localhost to the Docker host without changing request headers. Only the specific UI-state and extended-history responses described above are mocked; authentication and ordinary browser traffic are unmodified. Their mock-source fetches use the Docker host alias for network access while preserving the original request headers.

Production rollout still requires approved compensation policy data, offer wording, deployment secrets, HTTPS, recovery validation and client acceptance described in the main readiness and operations documents. UI checks do not establish approval of missing business rules.
