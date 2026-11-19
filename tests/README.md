# Isolated acceptance tests

These suites create synthetic `TEST_FIXTURE` records and users. They must run
against a dedicated test database. They do not delete records or recreate a
database. `CTC_TEST_FIXTURES=1` explicitly confirms the target is isolated when
using a port other than the original acceptance port 8090.

## Configurable engine verification

Use a separate environment for rules fixtures; do not point these suites at the
working review instance or a business database.

```powershell
docker build -t ctc-generator-backend:configurable-rules backend
docker build -t ctc-generator-frontend:configurable-rules frontend
docker compose -f compose.rules-test.yaml up -d
docker build -f scripts/Dockerfile.qa -t ctc-generator-qa .
docker run --rm -v "${PWD}/artifacts/rules-api:/artifacts" -e CTC_TEST_FIXTURES=1 -e TEST_BASE_URL=http://host.docker.internal:8094 -e TEST_ORIGIN=http://localhost:3024 ctc-generator-qa npm run test:rules
docker run --rm -v "${PWD}/frontend/tests/rules-browser.mjs:/qa/rules-browser.mjs:ro" -v "${PWD}/artifacts/rules-browser:/artifacts" -e CTC_TEST_FIXTURES=1 -e APP_URL=http://localhost:3024 ctc-generator-qa node rules-browser.mjs
```

The original API/browser suites also run against these isolated ports. The
production runner includes `rules-api.mjs` after the legacy security/PDF checks,
covering actual restricted-role inserts for new component identities and groups.
Use a different API port, such as 8096, if the rules demo test stack is running.

The Go test `TestRuleMigrationPreservesHistoryAndPersistsDynamicPolicy` accepts
`RULES_TEST_DATABASE_URL` only for a **new empty** database whose name ends in
`_test`, together with `CTC_TEST_FIXTURES=1`. It adopts the preceding schema and
verifies the new migration preserves existing snapshots and PDF bytes. Keep this
database separate from the settings tests' `TEST_DATABASE_URL` fixture.

## Build and run the upgrade environment

```powershell
docker build -t ctc-generator-backend:production-upgrade backend
docker build -t ctc-generator-frontend:production-upgrade frontend
docker compose -f compose.upgrade-test.yaml up -d
docker build -f scripts/Dockerfile.qa -t ctc-generator-qa .
docker run --rm -v "${PWD}/artifacts/upgrade-api:/artifacts" -e TEST_BASE_URL=http://host.docker.internal:8092 -e TEST_ORIGIN=http://localhost:3022 -e CTC_TEST_FIXTURES=1 ctc-generator-qa npm run test:api
docker run --rm -v "${PWD}/artifacts/production-upgrade/browser:/artifacts" -e APP_URL=http://localhost:3022 ctc-generator-qa npm run test:browser
```

This uses ports 3022/8092 and a separate persistent database. It does not replace
the original demo database or containers. Repeated sign-in-heavy tests can hit
the shared authentication throttle; let its window expire or create a fresh
isolated test database. Do not weaken production limits or delete business data
to make tests pass. The latest local review instance uses an additional ignored
`artifacts/final-api.override.yaml` pointing at a new test database; prior test
data is preserved.

The browser URL and configured `APP_ORIGIN` must both be `http://localhost:3022`
for this upgrade environment. Chromium's hostname resolver maps localhost to the
Docker host without changing the browser Origin. Browser tests do not rewrite
authentication headers. API-only tests use the host alias for network access and
explicitly send `TEST_ORIGIN=http://localhost:3022`.

Run the complete production-mode TLS/security/PDF/backup/restore suite with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-production.ps1 -Image ctc-generator-backend:production-upgrade -ApiPort 8094
```

The runner creates uniquely named isolated containers, databases, test secrets
and volumes. It stops its own containers on completion and retains data/evidence;
it never targets a supplied live database. The PowerShell execution-policy flag
applies only to that invocation. On platforms with PowerShell 7, invoke the script
with `pwsh` instead.

Go tests include an opt-in `TestConfiguredIdlePeriodPreservesValidSessionOnConcurrentLogin`.
Set `CTC_TEST_FIXTURES=1` and `TEST_DATABASE_URL` to a newly created empty database
whose name ends in `_test`. The test refuses a populated target and does not drop
tables. CI provisions a dedicated database and runs this regression explicitly.

The existing `api.mjs` suite checks URD arithmetic, all component groups,
immutable revisions, access control and extracted PDF text. `browser.mjs`
checks the browser workflow. Run them with the project QA image, which includes
Chromium, Playwright and `pdftotext`.

`security.mjs` adds strict request parsing, explicit boolean elections, profile
validation, two-HR record isolation, CSRF, permission boundaries, password
changes and session revocation, optimistic revisions, pagination, ownership
assignment, effective dates and audit secrecy. It runs with Node 24:

```powershell
docker run --rm -v "${PWD}/tests:/tests:ro" -v "${PWD}/artifacts/production:/artifacts" -e TEST_BASE_URL=http://host.docker.internal:8092 -e TEST_ORIGIN=http://localhost:3022 -e CTC_TEST_FIXTURES=1 node:24 node /tests/security.mjs
```

Production-mode tests require a separate production-configured test instance
with PostgreSQL TLS, a restricted runtime database role and an administrator
whose temporary password has already been changed. Set `TEST_PRODUCTION=1`,
`TEST_ADMIN_EMAIL`, `TEST_ADMIN_PASSWORD`, `TEST_BASE_URL`, `TEST_ORIGIN`, and
`CTC_TEST_FIXTURES=1`. The suite creates and enrolls its own test C&B and HR users.
It supplies a fully synthetic dated compensation configuration; those values
are test inputs and must never become company master data.

Idle expiry can additionally be exercised when Node and Docker CLI run on the
same host: set `TEST_DB_CONTAINER` to an isolated `ctc-generator-test-*`,
`ctc-security-test-*`, or `ctc-upgrade-test-*` database container and optionally
`TEST_DB_NAME`/`TEST_DB_USER`. This advances only the test HR user's session
last-seen timestamps. Without that setting, the report explicitly marks that
integration check skipped.

The Go integration test `TestConfiguredIdlePeriodPreservesValidSessionOnConcurrentLogin`
also verifies that a second login preserves sessions still valid under an
administrator-configured 60-minute idle period. It requires
`CTC_TEST_FIXTURES=1` and `TEST_DATABASE_URL` pointing to a newly created empty
database whose name ends in `_test`. It refuses existing tables and performs no
cleanup that deletes data. Run it with `go test ./internal/app -run
TestConfiguredIdlePeriod -v` from `backend/`; ordinary unit runs skip it when
the database environment variable is absent.

Startup configuration rejection probes launch short-lived containers with
networking disabled and no database mounts:

```powershell
./tests/startup.ps1 -Image ctc-generator-backend:production-upgrade
```

`startup.mjs` is the portable equivalent for a host with Node and Docker CLI.
Set `TEST_BACKEND_IMAGE` when using a non-default image tag. Both variants check
environment, HTTPS origin, PostgreSQL TLS, runtime credential separation,
conflicting secret sources and pool bounds. Result JSON files are written to
`ARTIFACT_DIR` (or the PowerShell `ArtifactDirectory` parameter).
