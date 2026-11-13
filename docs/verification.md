# Verification record

Verified locally on 11 November 2026 against the supplied five-page URD. The running demo uses Next.js, Go and PostgreSQL; Chromium generates saved-revision PDFs.

## Results

| Check | Result |
| --- | --- |
| Go engine and application tests | Passed: 17 top-level tests, including table-driven boundary cases |
| Independent API acceptance | Passed: 16/16 workflows against isolated test database/API 8090 |
| Chromium browser acceptance | Passed: 12/12 workflows at localhost:3020; no uncaught browser errors |
| Next.js production build and TypeScript | Passed |
| Frontend production dependency audit | 0 vulnerabilities reported |
| Go govulncheck with Go 1.27.4 | 0 reachable vulnerabilities; 0 vulnerabilities in imported packages |
| Generated documents | Draft/final CTC and offer PDFs verified with pdftotext; sample pages visually inspected |
| Responsive UI | Desktop 1440px and mobile 390px inspected; no page-width overflow |
| Service startup | Database healthy; frontend and API running on their dedicated local ports |

The Go scan additionally reports module-only advisory GO-2026-5932 for unused `golang.org/x/crypto/openpgp`. The application imports bcrypt, not OpenPGP. This is not a comprehensive penetration test or production certification.

## Covered behavior

The engine and API tests verify all 20 salary components; the URD worked example; PF thresholds; NPS opt-in and both regimes; grade and qualification validation; negative balancing components; input precision and numeric bounds; unknown versus explicitly inapplicable rules; ESI equality boundaries and wage bases; percentage monthly/annual bases; and missing job parameters.

A synthetic complete configuration tests nonzero Professional Allowance, Superannuation, Annual Special Allowance, all five policy payments and all four variable-pay components. It combines explicit monthly, annual, one-time, percentage and schedule rules. Independently expected results are Inclusion/Exclusion 50000 each, HRA 34900, annual statutory 40860, policy 63000 and variable 41000 for year 1: final CTC 1344860. Year 2 variable 54000 gives CTC 1357860. Missing year 3 keeps the schedule pending and blocks finalization. These values are test inputs, not client policies.

Workflow tests exercise login/session/logout, missing CSRF and hostile Origin rejection, role restrictions, HR ownership, administrator user lifecycle/session revocation, configuration publication, revision immutability across master changes, PDF access and persistence, and append-only audit events. Saved PDF byte equality and extracted amounts are checked.

Browser tests cover HR single-screen calculation, NPS/tax-regime changes, invalid residual feedback, revision save/reopen, both PDF downloads, read-only HR masters, audit, C&B configuration/clarifications, auditor navigation, mobile layout, and administrator creation/deactivation of a fictional user. The tax selector's accessible labels and NPS click target were corrected during verification.

## Evidence

- `artifacts/api-results.json`: individual API results.
- `artifacts/browser-results.json`: individual browser results and error list.
- `artifacts/api-ctc-draft.pdf`, `api-offer-draft.pdf`, `api-ctc-final.pdf`, `api-offer-final.pdf`: isolated fixture outputs; corresponding `.txt` files contain extracted text.
- `artifacts/browser-ctc-draft.pdf`, `browser-offer-draft.pdf`: real browser downloads from the URD demo.
- `artifacts/generator-desktop.png`, `generator-mobile.png`, `login-desktop.png`, `configuration-cb.png`, `users-desktop.png`: screen captures.

Artifacts are generated local files and are excluded from source control. Synthetic complete policies remain only in the separate `ctc-generator-test` database. The main demo contains the URD illustration and fictional candidate/user records.

## Reproduce

From the project directory with Docker Desktop running:

```powershell
docker compose up --build -d
docker run --rm -v "${PWD}/backend:/app" -w /app golang:1.27.4-bookworm go test ./...
docker compose -p ctc-generator-test -f compose.test.yaml up -d
docker build -f scripts/Dockerfile.qa -t ctc-generator-qa .
docker run --rm -v "${PWD}/artifacts:/artifacts" -e TEST_BASE_URL=http://host.docker.internal:8090 -e APP_URL=http://localhost:3020 ctc-generator-qa npm test
```

The test runner maps Chromium's localhost hostname to the Docker host while retaining the application's actual localhost Origin. API tests explicitly use the allowed Origin and separately test rejection of an untrusted Origin. No application security setting is relaxed for testing. API tests create synthetic configuration only on port 8090. Browser tests use fictional candidates/users in the main demo; the user created by the test is deactivated at the end.

## Limits and client decisions

The application does not resolve the URD's missing grade matrix, PA/slab values, retirement/ESI rules, benefit policies, variable-pay schedules, HRA Finance approval, rounding/annualization decisions or official offer wording. The main sample therefore permits watermarked drafts and blocks final issuance. Supported master operations are explicit; unsupported formula methods require review instead of silent approximation.

The current Basic method is manual, and annualization is a full 12-month convention requiring confirmation. Joining-date proration and automatic effective-date selection are not assumed. Corporate OIDC, production hosting/TLS/secrets, backups, retention, operational monitoring, load testing and a formal migration pipeline remain production preparation work.
