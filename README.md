# CTC Generator demo

A local Next.js + TypeScript, Go, and PostgreSQL implementation of the attached CTC Generator URD. The Go API is the authoritative calculation engine. PDFs are rendered from saved revisions using Chromium.

## Run

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

## Production preparation

The deployment design preserves typed configuration, exact decimal arithmetic, immutable input/result snapshots, role checks and audit history. Before production, supply and approve the missing client policy annexures and offer template; resolve rounding and effective-date conventions; integrate corporate identity as required; configure TLS, secret management, private database/document access, backups, retention and operational monitoring. The included local demo credentials and infrastructure settings are not production settings.

Payroll, payslips, income-tax calculations, investment declarations, HRMS/ATS integration and CTC-target reverse solving remain outside the URD's first release.
