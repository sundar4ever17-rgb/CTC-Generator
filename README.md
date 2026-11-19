# CTC Generator

A Next.js + TypeScript, Go, and PostgreSQL application based on the attached CTC Generator URD. The Go API is the authoritative calculation engine. PDFs are rendered from saved revisions using sandboxed Chromium in production.

The production upgrade includes explicit checksummed migrations, restricted database access, administrator bootstrap, password/session controls, role and record authorization, approved versioned policies with effective-date checks, immutable calculation/document history, and a separate HTTPS deployment. See the [gap analysis](docs/production-gap-analysis.md), [deployment instructions](docs/deployment.md), [backup and operations runbook](docs/operations.md), and [upgrade verification and release gates](docs/production-verification.md).

**Business release is gated on client-approved policy annexures, manual acceptance examples and offer wording.** Synthetic test policies and the URD illustration are not production master data. No live deployment has been performed.

## Open the upgraded local application

The upgraded review instance is available at **http://localhost:3022**. Keep Docker Desktop running with Linux containers. To start the existing local review database and services again:

```powershell
cd "D:\CTC GENERATOR"
docker compose -f compose.upgrade-test.yaml -f artifacts/final-api.override.yaml up -d
```

The local sample accounts listed below work in this review instance. The override preserves the current review database; on a fresh checkout without that local artifact, build the two `production-upgrade` images and use only `compose.upgrade-test.yaml`, as described in `tests/README.md`. The configured browser origin must match `http://localhost:3022`. Normal browser login/logout has been verified without request interception.

This is local review mode with synthetic data. Real business deployment uses the separate production configuration and deployment runbook.

To inspect the review database in DBeaver, create a PostgreSQL connection with host `127.0.0.1`, port `5443`, database `ctc_upgrade_final_test`, username `ctc`, and password `isolated_test_only`. SSL is disabled for this local review connection. Open `Schemas > public > Tables` to browse records. Port 5443 is bound only to localhost; port 5442 belongs to the original demo. These settings do not apply to production, which requires verified TLS and separate restricted database identities.

## Configurable calculation rules

C&B users can open **Configuration**, select a base version, and choose
**Configure calculation rules**. The new **Calculation rules** editor manages
salary components, categories, formulas, eligibility, group targets, and additional
HR inputs. Preview changes, then save a new version. Existing versions, calculations,
and PDFs retain their original behavior and figures. Conversion starts with the
same URD 50:50 split; alternative policies require client approval.

See [configuration workflow and engine behavior](docs/configurable-calculations.md)
and [isolated verification commands](tests/README.md).

## Run the original local demo

Prerequisite: Docker Desktop running Linux containers, with Docker Compose.

From the project directory:

```powershell
docker compose up --build -d
```

Open **http://localhost:3020**. The API is at http://localhost:8088/api and PostgreSQL is available locally on port 5442. The application uses its own Compose project and named database volume.

The PowerShell convenience scripts are `scripts/start-demo.ps1` and `scripts/stop-demo.ps1`.

## Local demo accounts

| Role | Email | Password |
| --- | --- | --- |
| Compensation & Benefits | cb@demo.local | DemoCTC!2026 |
| HR | hr@demo.local | DemoCTC!2026 |
| Administrator | admin@demo.local | DemoCTC!2026 |
| Auditor | auditor@demo.local | DemoCTC!2026 |

These are deliberately public demonstration credentials, usable only with non-sensitive sample information. Services bind to the local loopback interface. Set `DEMO_PASSWORD` before initializing a fresh demo database to use a different seed password. Changing the environment variable is not a password reset for existing accounts. Manage existing passwords through the administrator screen.

## Business scope and sample data

The attached URD is the business authority. The seeded configuration is explicitly the URD's worked illustration: Monthly Gross 100000, Basic 20000, Conveyance 7300, New Regime, NPS opted in, no Professional Allowance and no Superannuation. It demonstrates PF 3000, NPS 2800, Special Allowance 30000 and HRA 36900, with Inclusion and Exclusion both 50000.

This illustration is not an approved grade matrix. Grade bounds, ESI wage bases/ceiling, Annual Special Allowance slabs, policy payment rules, variable pay rules, rounding/annualization approval and the official offer template remain clarification items. Missing amounts are never silently treated as zero. Known annual subtotals are labeled as incomplete; the demo does not represent them as final CTC.

The five URD groups and all 20 components are retained. Monthly Special Allowance differs from Annual Special Allowance; Exclusion HRA differs from Additional HRA. Employer PF, NPS and Superannuation are already inside the URD Gross definition.

A watermarked draft CTC sheet or offer preview can demonstrate the document workflow when core parity is valid. Final output requires complete confirmed configuration and document fields. Negative balancing amounts or invalid parity block document generation.

## Configuration and access

Compensation & Benefits maintains versioned master data. Editing produces a new version, preserving earlier saved revisions. Supported benefit definitions use explicit amounts, percentages of permitted upstream bases, eligibility conditions and schedules. Unsupported business formulas remain clarification items; arbitrary executable expressions are not accepted.

HR works with candidate inputs and assigned CTC records. Administrators manage users. Compensation configuration editing requires C&B authority. This distinction is a proposed interpretation of the URD's edit-access wording and must be confirmed before production rollout.

## Project layout

- `frontend/`: Next.js application and UI components.
- `backend/`: Go API, calculation engine, PostgreSQL schema and PDF generation.
- `tests/`: independent acceptance tests.
- `docs/`: implementation decisions and verification notes.
- `compose.yaml`: local demo services and persistent database.
- `compose.test.yaml`: isolated acceptance-test database and API.

## Tests

Engine tests run in the Go toolchain container:

```powershell
docker run --rm -v "${PWD}/backend:/app" -w /app golang:1.27.4-bookworm go test ./...
```

The acceptance suite uses a separate database because it creates synthetic `TEST_FIXTURE` policies to exercise complete configuration and final-document workflows. Those fixtures are testing inputs, not client policy and not demo seed data.

After building the backend image:

```powershell
docker compose -p ctc-generator-test -f compose.test.yaml up -d
```

See [verification commands and results](docs/verification.md), [architecture and ER diagrams](docs/architecture.md), and [URD traceability and open questions](docs/requirements.md).

## Production deployment

Use `compose.production.yaml` separately from the demo file. Supply image digests, domain and secret-file paths using `.env.production.example`. Provision private PostgreSQL with verified TLS and distinct migration/runtime roles. Run the explicit `migrate` and one-time `bootstrap-admin` commands, apply the runtime grants, and complete the admin's initial password change. Follow [the deployment runbook](docs/deployment.md) for the exact sequence, backup requirements and approval gates.

Production refuses insecure database transport, HTTP origins, active demo accounts, elevated runtime database roles and missing/outdated migrations. It never seeds demo users or configuration. Admin controls named users, ownership assignment and security session settings; C&B maintains and publishes approved policies; HR manages its assigned candidate/CTC records. A legacy read-only Auditor role is retained.

All new production calculations require an explicit calculation effective date separate from joining date. Existing records are never recalculated or overwritten; subsequent edits create revisions with a concurrency check. Pending rules and invalid parity block final offers. Corporate SSO/MFA, client retention periods, recovery objectives and capacity targets must be agreed before launch; they are not silently assumed.

Upgrade regression tests use `compose.upgrade-test.yaml` on local ports 3022/8092 with a separate database, preserving the original demo and its records. See [test commands](tests/README.md).

Payroll, payslips, income-tax calculations, investment declarations, HRMS/ATS integration and CTC-target reverse solving remain outside the URD's first release.
