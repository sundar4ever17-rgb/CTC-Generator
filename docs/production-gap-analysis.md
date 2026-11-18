# Production upgrade gap analysis

Assessment: 16 November 2026. Primary business source: the supplied five-page `User Requirement Document (URD) - CTC generator.pdf`, with its extracted text in `proposal/build/urd-source.txt`. This is an implementation work record, not a go-live certification.

## Existing system and retained architecture

Next.js/TypeScript presents a single-screen workflow and proxies same-origin API requests to Go. Go owns calculation, authorization, transactions and Chromium document rendering. PostgreSQL stores versioned configuration, candidates, cases, immutable revisions, calculation lines, template snapshots, document bytes and audit events. Retain this stack; financial arithmetic already uses exact decimals and typed upstream-only formulas.

The running local database initially contained 6 users, 7 configurations, 5 candidates and 5 calculation revisions. A custom-format PostgreSQL backup was taken before upgrade work: `artifacts/backups/ctc-pre-production-upgrade.dump`, SHA-256 `49646C97B00B0C9444B4EE353AD4169019C6993D81A391F96C756FBFD84E4B0C`. Its archive table of contents was verified. Existing business records must not be deleted or silently rewritten. Upgrade rehearsals and tests use separate databases.

## Gaps and implementation phases

| Phase | Evidence / gap | Required change |
| --- | --- | --- |
| Architecture | Runtime boot executes DDL; DB errors can log parameter data; HTTP requests lack correlation | Separate migration command; bounded connection pool; structured redacted request logging; health/liveness; request bounds |
| Database | Repeated unversioned schema SQL; no migration ledger/checksum; incomplete immutability; indexes and concurrency guards incomplete | Transactional checksummed migrations with advisory lock, baseline adoption without data deletion, immutable supporting snapshots, least-privilege runtime role, backup/restore rehearsal |
| Authentication | Public demo login defaults, no user password-change flow, process-local login limiter, absolute expiry only | Production startup validation, one-time bootstrap CLI, shared throttling, idle expiry, forced initial/reset password change, session revocation and auditable auth events |
| Authorization | HR ownership already exists; admin/C&B currently have broad record access | Preserve documented global admin/C&B oversight and HR owner isolation, test direct API bypass, prevent last-admin removal races, document scope for client approval |
| Master data | Versions and dates stored, but dates unused; incomplete policies can be published; first production configuration UX depends on demo | Explicit calculation effective date, publication validation, non-demo empty authoring path; never infer missing policy values |
| Calculation | Core URD example and PF/NPS/parity already tested; some malformed/null/unbounded input accepted | Strict typed input, profile/date limits, edge-case tests, preserve unknown vs inapplicable rules and exact balancing |
| Candidate workflows | Revisions preserved but concurrent stale edit can silently append; list capped at 500 | Optimistic revision conflict detection, bounded paged APIs, owner reassignment with audit, preserve historical input snapshots |
| Documents | Saved PDFs are cached and escaped; parallel Chromium launches unbounded; no access audit | Bounded renderer, stored-byte checksum verification, access audit, immutable documents/templates, retain finalization gates |
| Testing | Existing tests use demo API and some browser tests mutate original demo | Isolate all upgrade tests and restore rehearsal; add production-mode startup/security/authorization cases; rerun workflow/PDF/browser checks |
| Deployment | Only loopback demo Compose, default DB password/superuser, no operational runbooks | Separate production deployment, private verified-TLS DB, secrets files, HTTPS proxy, non-root images, backups/restore, CI and monitoring/checklist |

## URD coverage and unresolved client decisions

All five groups and twenty components are represented. Existing sequential execution follows Gross → fixed Inclusion → fixed Exclusion → Special Allowance → HRA. The supplied example expects PF 3000, NPS 2800, Special Allowance 30000 and HRA 36900, with both groups 50000. Configurable ESI, Gratuity, policy payments and variable schedules exist, with pending rules explicitly blocking final CTC.

No approved grade annexure, policy payment matrix, Superannuation policy, ESI wage definitions/ceiling, annual payout schedules, rounding/proration approval or official offer template is present. The URD explicitly leaves these open. Tests using synthetic policies demonstrate implementation behavior only. A production release requires the client to approve and enter those values and supply approved manual calculation examples. The exact open-item register remains in `requirements.md`.

Explicit calculation effective date is introduced separately from joining date; no joining-date proration or automatic latest-rule substitution is inferred. Admin and C&B global oversight is retained from the existing system; HR can view/change only owned records. Corporate SSO/MFA, retention periods, hosting provider, domain, availability targets and recovery objectives require organization-specific decisions. Local password authentication is hardened, not represented as corporate identity integration.

## Completion evidence

Implementation and actual test results are recorded in `production-verification.md` when available. Production deployment and destructive database operations require user approval. A successful build does not constitute approval of compensation policy or permission to deploy.
