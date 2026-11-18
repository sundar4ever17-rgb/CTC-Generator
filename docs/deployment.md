# Production deployment

The existing Next.js, Go and PostgreSQL stack is retained. `compose.yaml` remains a loopback-only demo. Production uses **only** `compose.production.yaml`; do not merge the demo file into it. This is a single application host deployment with an externally managed private PostgreSQL database, not a multi-host high-availability service. Hosting accounts, DNS, production secrets and client-approved policies are required before launch. No live deployment is performed by the implementation work.

## Deployment layout

Internet HTTPS reaches Caddy on ports 80/443. Caddy sends `/api/*` directly to Go and all other paths to Next.js on an internal Docker network. The API and frontend have no published ports. The API connects to a separate PostgreSQL service using TLS with certificate and hostname verification. Set a firewall rule allowing database access only from the application and backup hosts. Restrict application outbound network access to DNS, the database and approved operational destinations using the host/cloud firewall.

The API, frontend and proxy have resource and process limits and rotating container logs. Application containers run as non-root with read-only root filesystems and dropped capabilities. Caddy retains only the low-port binding capability. Chromium retains its own sandbox using the reviewed `scripts/security/chromium-seccomp.json` profile. Temporary browser files are in memory; persisted PDFs are in PostgreSQL with their saved revisions.

## Prepare the release

1. Run all unit, integration, browser and PDF acceptance checks against an isolated staging database. Never run `tests/api.mjs` or `tests/browser.mjs` against production; these tests intentionally create synthetic policy and candidate records.
2. Build from a clean reviewed commit with `docker build -t YOUR_REGISTRY/ctc-api:RELEASE backend` and `docker build -t YOUR_REGISTRY/ctc-web:RELEASE frontend`. The frontend's API build destination must remain `http://backend:8080` for the supplied topology. Scan both resulting images, including Chromium and OS packages. Push and record their immutable image digests. Select and scan a current Caddy 2 image and record its digest as well.
3. Copy `.env.production.example` to `.env.production`; fill the domain, ACME contact, image digest references and secret file paths. The compose file fails when essential values are empty. Do not put passwords in `.env.production`.
4. Create DNS A/AAAA records for the selected domain and allow inbound TCP 80/443. Do not publish a broken IPv6 record. Caddy obtains and renews HTTPS certificates; preserve its `caddy_data` volume. [Caddy automatic HTTPS documentation](https://caddyserver.com/docs/automatic-https) explains the domain and network requirements.
5. Supply runtime and migration connection strings through separate secret files. Each connection string must use `sslmode=verify-full&sslrootcert=/run/secrets/postgres_ca`. Percent-encode reserved password characters. The hostname must match the server certificate. Supply the trusted CA PEM from the database provider. Do not disable certificate verification.

Compose secret files are bind mounts, not an encrypted secret store. Provision them through the organization's secret manager, keep them out of git and backup logs, restrict host directory/file permissions, and make only the intended container uid able to read them. On Linux the non-root API uid is 10001; bind-mounted secret files must be readable by that uid. The migration and bootstrap credentials are mounted only in short-lived operation containers. [Docker secret documentation](https://docs.docker.com/compose/how-tos/use-secrets/) describes these semantics.

## Database roles and migrations

Use a dedicated database owned by a dedicated migration login, with a second application login. Neither login should be a superuser or have CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS. The runtime login must not own the database, schema or tables, or belong to an owner/admin role. A database administrator provisions these identities and sets passwords interactively through a secure channel. Do not copy public demo passwords or active `@demo.local` users into production.

The production server verifies the migration version at startup and never performs DDL. The migration command is the sole schema-changing deployment step. Migrations use a lock and transaction, preserve existing records, and retain a checksum/version ledger. Existing demo records are preserved by upgrades; any conversion of a populated demo into business data needs separate client review. The safer initial launch uses a fresh production database and approved configuration without synthetic candidate data.

Before **every** schema change to a populated database, take a verified backup using `scripts/backup-database.ps1` and ensure a recent restore drill has passed. Review migration SQL before execution. Obtain explicit approval for any destructive change and for the live deployment itself. The current implementation adds constraints and indexes without deleting business records; validation can deliberately fail if legacy data violates a new constraint.

After approval, use the following commands from the repository root on the deployment host:

```powershell
docker compose --env-file .env.production -f compose.production.yaml config --quiet
docker compose --env-file .env.production -f compose.production.yaml --profile operations run --rm migrate
# As database owner, run scripts/grant-runtime-access.sql with psql -v app_role=YOUR_RUNTIME_ROLE.
# Use a secure pg_service.conf connection; do not put a connection password on the command line.
docker compose --env-file .env.production -f compose.production.yaml --profile operations run --rm bootstrap-admin
./scripts/test-browser-sandbox.ps1 -Image YOUR_TESTED_API_IMAGE_DIGEST
docker compose --env-file .env.production -f compose.production.yaml up -d --wait
```

Run the grant script after each migration. It grants explicit permissions per table: the runtime can append immutable history but cannot rewrite or delete it; configuration updates are limited to publication fields. No automatic grants to future tables are installed. The bootstrap operation reads `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_NAME` and the secret password file. It is an explicit first-administrator command, not a server-startup password reset. After successful bootstrap, remove the bootstrap password from the host secret store; its service is inactive unless explicitly run. Retain a valid secret path in the environment configuration for later reviewed recovery procedures.

The host must support unprivileged user namespaces for Chromium. The preflight runs without network access, without elevated capabilities and with the same seccomp and filesystem restrictions used by the API. If it fails, resolve the host kernel/AppArmor policy; do not disable Chromium's sandbox. It was exercised successfully on the development Docker Linux host, but must also pass on the actual deployment host.

## First use and release gates

- Sign in as the bootstrap administrator and complete the required password change. Create named Admin, C&B and HR accounts; avoid shared identities. Confirm access and deactivation/session-revocation behavior in staging.
- Review the administrator application settings for idle and maximum session lifetimes with the security owner. Administrative security-setting changes require the current administrator password and are audited.
- Have C&B enter the supplied grade matrix, rates, ceilings, policies, effective dates and document wording. Record client confirmations and publish only approved versions. Missing policy annexures are not replaced by fixture data.
- Verify the URD illustration, all additional approved manual examples, PF/ESI boundaries, optional NPS, qualification gates, balancing/parity and annual schedules. Obtain business sign-off on rounding and annualization.
- Generate and inspect final CTC and offer PDFs from the approved configuration; confirm fonts, signature fields, all five groups, dates and totals. Verify saved revisions and documents remain unchanged after later configuration versions.
- Test HTTPS redirects, session cookies, idle expiry, API roles, HR record ownership and error behavior from the deployed hostname. Check `/api/live` and `/api/health` through the proxy.
- Confirm backup encryption, off-host retention, recovery objectives, a successful isolated restore, alert destinations, incident ownership and access review cadence. These require the customer's operational decisions.
- Obtain explicit business and deployment approval before directing actual users to the application.

## Scaling and rollback

Sessions, configuration, revisions, documents and authentication throttles live in PostgreSQL. API workers can be replicated behind a load balancer once database connection limits and document-rendering load have been measured. Keep aggregate pool sizes below the database connection budget, reserving capacity for migrations, backups and operators. The supplied compose topology uses one API instance and is not a benchmark or availability guarantee. Profile PDF CPU/memory and database storage growth before raising concurrency.

For a failed release, preserve logs, stop new writes and route traffic back to the previous tested image only when its schema compatibility is established. Schema rollback is not automated: never drop columns/tables to make an old binary run. Restore into a separate empty database and validate before a separately approved cutover if data recovery is required. Keep the original database and backup evidence intact.

CI (`.github/workflows/verify.yml`) runs Go race tests/vet/vulnerability checks, a locked frontend build/typecheck/audit, independent API/PDF plus security acceptance, and an isolated production-mode TLS/sandbox/recovery test. It uploads only synthetic result JSON/PDF evidence, not fixture secrets or database archives. It does not deploy. Browser acceptance, host sandbox preflight, image scanning, production backup restore drills, load testing and client policy sign-off remain release checks against the chosen environment.

For a reproducible local production-mode integration check, build the backend image then run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-production.ps1 -Image ctc-generator-backend:production-upgrade -ApiPort 8094`. This creates a new uniquely named test database/volume/network with a short-lived test certificate, separate migration/runtime roles, required password enrollment, security fixtures, sandboxed final PDFs and a backup/restore drill. It stops only its own containers at completion and retains their test volumes and `artifacts/ctc-upgrade-test-prod-*` evidence; it never connects to the real production database. This validates API production mode and database TLS; public DNS, real HTTPS issuance and browser behavior at the customer's domain still require staging verification.
