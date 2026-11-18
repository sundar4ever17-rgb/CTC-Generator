# Production upgrade verification and release gates

Verified locally on 16 November 2026. The existing stack and workflows were upgraded in place. These results establish tested implementation behavior; they do not approve missing compensation policies or authorize live deployment.

Local review: `http://localhost:3022` (isolated test environment containing synthetic acceptance records). The original demo remains at port 3020. Final tested local images are `ctc-generator-backend:production-upgrade` (`sha256:6a5df6feb1c5857d723417ae3e0d249fdd896dd8dbe4f55640c91faa1d4be853`) and `ctc-generator-frontend:production-upgrade` (`sha256:99d71c08d9048c3a905f6b52ac216ea114f20c9f30b369a426bb98ddab3351bb`). These are local image identifiers; publish reviewed registry digests only through the approved release process.

## Implemented phases

| Phase | Delivered |
| --- | --- |
| Architecture | Modular auth, environment, migration and security-settings services; bounded connection pool and renderer; redacted structured request logs and health probes |
| Database | Transactional migrations 001–003 with advisory lock, checksum ledger and unknown-version rejection; explicit runtime grants; preserved existing records; immutable supporting history |
| Authentication and roles | Production bootstrap command; bcrypt; temporary-password enrollment; own-password change; shared login throttles; CSRF; secure cookies; configurable idle/absolute expiry; credential/access revocation; Admin/C&B/HR permissions and HR ownership |
| Master configuration | Empty production authoring workflow, typed editable grades/rates/ceilings/slabs/policies/templates, versioned effective windows and complete-publication checks |
| Calculation | Original URD sequence retained; engine 1.2.0; strict election/profile/date validation, amount bounds and precision checks; order-independent eligibility evaluation; no guessed client rules |
| Candidate workflow | Saved revisions, optimistic concurrency, audited HR reassignment, server search and paging, historical configuration retrieval |
| Documents | Saved-revision CTC and offer PDFs, finalization gates, escaped content, bounded sandboxed rendering, checksum verification, download/preview audit and improved page breaks |
| Audit | Transactional events for business/security changes, immutable history, secret-free auth/setting events and role-scoped searchable audit views |
| Testing | Engine/app/race/vet tests, API and browser workflows, production security/TLS/PDF checks, concurrent-save and configurable-idle regressions, migration and restore checks |
| Deployment | Separate HTTPS production Compose, secret-file configuration, private verified-TLS PostgreSQL, non-owner runtime role, sandbox preflight, backup/restore scripts, CI and operational runbooks |

## Actual verification

| Check | Result and evidence |
| --- | --- |
| Go unit tests / race detector / vet | Passed; 31 top-level tests in source including the separately invoked database integration test. Ordinary runs skip that explicit opt-in integration test. |
| Original independent API acceptance | 16/16 passed on the final engine 1.2.0 image. `artifacts/upgrade-api/api-results.json` and extracted draft/final CTC/offer PDFs. |
| Security acceptance | 12/12 groups passed, including actual simultaneous saves (one successor committed, the stale writer received 409), record ownership, password revocation, admin settings, effective dates and audit secrecy. `artifacts/upgrade-security/security-results.json`. |
| Production-mode acceptance | 12/12 groups passed using verified PostgreSQL TLS and a non-owner runtime login; secure-cookie/bootstrap enrollment and final PDFs passed. Latest run: `artifacts/ctc-upgrade-test-prod-20261116104512-1ab4/`. |
| Startup guards | 8/8 passed without network access: environment, HTTPS origin, DB TLS, secret separation and pool bounds. `artifacts/upgrade-security/startup-results.json`. |
| Session expiry regression | Passed with a dedicated empty test database: a 40-minute-idle session survives another login under a 60-minute policy; it expires at 61 minutes while the newer session remains valid. `artifacts/upgrade-security/session-policy-results.json`. Default idle-expiry test also passed separately. |
| Migration future-version guard | Rejected an unknown migration before creating application tables, preserving the test sentinel/ledger. `artifacts/upgrade-security/migration-guard-results.json`. |
| Original browser acceptance | 12/12 workflows passed on the isolated frontend, including both draft PDFs, mobile layout, roles and revision reopening. `artifacts/production-upgrade/browser/`. |
| Additional frontend checks | Six security checks and three history/settings checks passed. Some production UI states and extended history are mocked; API enforcement and actual settings changes are tested separately. See `frontend-review.md`. |
| Production PDF rendering | Both final PDFs rendered with Chromium's sandbox active, non-root, read-only filesystem, no elevated capabilities and reviewed seccomp. Repeated downloads were byte-identical; restored PDF bytes matched stored SHA-256 checksums. Extracted text shows the correct amounts, engine version and explicit effective date. |
| Frontend build / TypeScript / dependency audit | Passed; zero known production npm dependency vulnerabilities reported at scan time. |
| Go vulnerability check | Zero reachable vulnerabilities and zero vulnerabilities in imported packages; one module-only advisory remains in an unused dependency package. This is not a complete image/OS security assessment. |
| Backup / recovery | TLS custom-format dump, checksum verification, restore into a new empty database, re-grant and content/count verification passed. A second restore into a populated target was refused. Latest `recovery-results.json` is in the production evidence directory above. |
| Populated upgrade | A backed-up production-mode fixture upgraded from 002 to 003; engine 1.2.0 served its historical engine 1.1.0 PDFs unchanged. `artifacts/production-validation/upgrade-preservation-results.json`. |

Repeated tests against shared demo logins correctly triggered the persistent login throttle. Final acceptance was rerun against a newly created isolated test database rather than weakening authentication or deleting limiter records. Production-mode suites use independently generated fixture accounts. No synthetic policy was inserted into the original demo or a live business database.

Local startup follow-up: corrected the upgrade Compose browser origin from port 3020 to 3022. Earlier browser harnesses had rewritten Origin and masked this mismatch. Normal browser login and logout at `http://localhost:3022` now pass without interception; the harnesses no longer rewrite the Origin header. This was a local review configuration correction, not a relaxation of API origin validation.

## Original database preservation

Original backup: `artifacts/backups/ctc-pre-production-upgrade.dump` (custom-format archive verified), SHA-256 `49646C97B00B0C9444B4EE353AD4169019C6993D81A391F96C756FBFD84E4B0C`.

The original local database still contains 6 users, 7 configurations, 5 candidates, 5 revisions and 9 documents. It was not migrated or replaced. The backup was restored into a separate database and all three migrations were applied there. Candidate, configuration, revision and document row fingerprints matched before and after; migration repeatability also passed. Evidence: `artifacts/backups/restored-before-migration.txt` and `restored-after-migration.txt`.

Backups contain sensitive information and are excluded from git. The logical dump itself is not encrypted; production operators must use restricted encrypted storage and off-host backup retention as documented in `operations.md`.

## Pending client decisions and go-live gates

- Supply and approve the actual grade matrix, Professional Allowance slabs, Superannuation policy, ESI wage definition/ceiling, annual allowances, policy payments and variable-pay schedules. No annexure containing these approved values was available.
- Confirm Basic selection, rounding, annualization/proration, HRA treatment, effective-date/version selection and final offer wording/signatory. Supply approved manual examples beyond the single illustrative URD example; synthetic tests do not replace business acceptance.
- Confirm global Admin/C&B oversight, HR ownership and whether the retained read-only Auditor role is required. Configure named real users; never promote demo/test identities or records into business operation without review.
- Select hosting, domain, secret manager, corporate identity/MFA requirements, data residency/retention, backup retention, recovery objectives, monitoring destinations and incident owners. Local password authentication is implemented; corporate SSO/MFA is not.
- Scan final OS/container images, exercise browser authentication through the actual HTTPS hostname, repeat sandbox/restore checks on the intended host, and measure capacity against agreed concurrency, document volume and availability targets. No high-availability or load benchmark claim is made for the single-host deployment.
- Obtain client calculation/template acceptance and explicit approval before live deployment or any destructive database operation.

Final offers remain blocked when required approved configuration is absent. No live environment was deployed. Detailed reproduction commands are in `tests/README.md`, `deployment.md`, `operations.md` and `frontend-review.md`.
