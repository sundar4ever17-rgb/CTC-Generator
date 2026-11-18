# Operations, backup and recovery

Assign an application owner, database operator and incident contact before launch. Define required recovery point and recovery time objectives, audit/candidate retention, authorized backup readers and data residency with the customer. No retention period or automatic deletion of business data is assumed by this project.

## Health, logs and alerts

`GET /api/live` indicates the process can serve requests. `GET /api/health` checks database connectivity and is the readiness probe. Probe readiness from the application's HTTPS hostname every minute and alert on consecutive failures; also alert on elevated API 5xx, authentication failure spikes, repeated 429 responses, high request/PDF latency, container restart loops, memory/CPU saturation, database connection pressure and low database/backup disk space. Select thresholds from staging measurements and the agreed service objectives.

Container logs go to stdout with local rotation. Forward them to the organization's restricted log service, correlate API request identifiers and investigate failures without logging request bodies, candidate searches, salary data, session cookies, CSRF tokens, passwords or connection strings. Caddy access logs are deliberately disabled because URLs can contain candidate search values. Keep infrastructure error logs and application audit records access-controlled; restrict database administrator access separately.

After any kernel, container runtime, Chromium or seccomp upgrade, run `scripts/test-browser-sandbox.ps1` and generate both document types in staging. Rebuild and scan images regularly because Chromium and OS package updates are distinct from Go/npm dependency checks. Pin deployed image digests and retain the release manifest.

## Backups

Enable the database provider's encrypted automated backups and continuous WAL archiving/point-in-time recovery according to agreed recovery objectives. A logical dump is useful before migrations and for portable recovery, but is not a substitute for point-in-time recovery. PostgreSQL documents that `pg_dump` makes a consistent single-database snapshot while the database is in use; cluster roles and provider settings need separate backup/provisioning records. [PostgreSQL backup documentation](https://www.postgresql.org/docs/current/backup.html)

Use a dedicated read-only backup identity permitted to read all application tables (including bytea PDFs) and required sequences. Store the service configuration and CA in the secret manager. The example `scripts/pg_service.conf.example` uses `sslmode=verify-full`; replace its placeholders in a restricted file outside the repository. The service must be named `ctc` and the CA path inside the container must remain `/run/secrets/postgres_ca`.

```powershell
./scripts/backup-database.ps1 -ServiceFile C:/secure/backup-pg-service.conf -CertificateFile C:/secure/database-ca.pem -Destination D:/secure/ctc-backups
```

The script uses the PostgreSQL 18 client by default, creates a custom-format archive, verifies its archive listing, and writes a SHA-256 sidecar. For a newer server, select a compatible approved client image with `-PostgresImage`; prefer a pinned digest in scheduled jobs. It does not print database credentials or place them in process arguments. A readable archive listing is not a successful recovery test.

The output contains sensitive personal and salary data and is **not encrypted by pg_dump**. Write it to an access-controlled encrypted volume and copy it immediately to encrypted off-host storage under separate credentials, with retention/object-lock policies selected by the customer. Protect the checksum manifest separately when tamper detection is required. No script automatically deletes old archives. Monitor backup completion, age, size anomalies, upload success and retention. Schedule this through the organization's approved scheduler only after recovery objectives and destinations are agreed.

## Restore drill and recovery

1. Provision a **new, empty, isolated** database, disconnected from production users. Record its exact host and database name. Use a schema-owner restore login with TLS verification and a dedicated service file. Use only trusted archives.
2. Obtain the archive and matching `.sha256` sidecar. The restore script validates the hash, verifies the target database name and refuses a target containing user tables, views or sequences. It never uses `--clean`, DROP DATABASE or DELETE.
3. Run the following only after the operator has approved that explicit empty target. Restoring a backup into a live environment or replacing a database requires separate user approval.

```powershell
./scripts/restore-database.ps1 -ServiceFile C:/secure/restore-pg-service.conf -CertificateFile C:/secure/database-ca.pem -Archive D:/secure/ctc-backups/SELECTED_ARCHIVE.dump -ExpectedDatabase ctc_restore_drill -ConfirmRestoreToEmptyDatabase
```

4. `pg_restore` runs in one transaction with exit-on-error. Reapply `scripts/grant-runtime-access.sql` for the isolated runtime role after restoration; roles and grants are intentionally not imported. [PostgreSQL restore documentation](https://www.postgresql.org/docs/current/app-pgrestore.html) explains archive options and trusted-source requirements.
5. Compare table counts and latest revision/audit timestamps with the backup manifest; check foreign keys, schema version and stored document checksums. Open saved revisions and PDFs in the isolated app. Compare previously approved calculation examples without altering historical snapshots. Record elapsed restore time and the newest recovered record timestamp against the agreed recovery objectives.
6. A production recovery cutover is a separately approved operation. Preserve the original database, freeze writes as required, obtain reconciliation/business sign-off and switch the connection secret only when validation succeeds. Record who approved the cutover and when it occurred. Do not remove the original data during the incident.

Backups include password hashes and session records. Do not expose a restored environment to users until existing sessions have been revoked through an approved operational step and access has been isolated. Revoke or rotate compromised credentials in the source system when recovering from a security incident; restoring data does not undo a credential compromise.

## Routine access and integrity checks

Review active accounts and roles; deactivate leavers through Admin so their sessions are revoked. Keep C&B policy publication access limited to approved people. Investigate failed configuration publication, parity failures and repeated document errors through audit history and request identifiers. Confirm the runtime database role cannot alter schemas, revise historical records or manage database roles. Reapply reviewed privileges after migrations and test least privilege before service startup.

Do not apply deletion/archival rules to candidates, offers or audit history without an approved retention policy and a tested migration. Existing history is intentionally append-only. Long-term growth requires periodic capacity review of PostgreSQL tables, indexes, WAL and stored PDFs; the initial deployment does not silently prune any records.

## Local operational verification — 16 November 2026

The production-mode integration runner passed against the backend containing engine 1.2.0 and migrations 001–003, using an isolated PostgreSQL 18 database over hostname-verified TLS, separate schema-owner/runtime identities and the production Chromium sandbox restrictions. All 12 security acceptance groups passed, including administrator settings and simultaneous revision saves. Both final PDFs were generated and repeated downloads matched exactly.

A populated custom-format backup restored into a new empty database; revision, audit, migration and document counts matched, and restored PDF content hashes matched their stored checksums. A repeat restore was explicitly rejected because the target was populated. A separate populated 002→003 upgrade preserved historical engine 1.1.0 revision/audit/document counts and returned the original saved PDFs byte-for-byte through engine 1.2.0. All test containers were stopped after verification; data volumes and local evidence were retained. These checks used synthetic policies and do not approve missing client policy annexures, certify load capacity or validate a live customer deployment.

Evidence is in `artifacts/ctc-upgrade-test-prod-20261116103949-4b2b/` (security and recovery results plus final PDFs) and `artifacts/production-validation/upgrade-preservation-results.json`. Reproduce with `scripts/test-production.ps1` as described in the deployment guide. The production-mode Node suite skips its optional Docker-driven idle-clock check; that check is verified separately by the application's isolated database security tests.
