# CTC Generator implementation decisions

- Approved stack: Next.js + TypeScript frontend, Go API, PostgreSQL, Chromium PDF generation.
- The attached URD remains the business source of truth. The application is an offer structuring tool, not a payroll or tax-computation system.
- All 20 components remain visible in the five URD groups.
- Employer PF, NPS and Superannuation are already in the URD Gross definition.
- The URD example is illustrative. It must not be promoted to an approved grade matrix.
- Missing rule data is distinct from zero and not applicable. Final offers require complete configuration, valid calculations, and an approved template.
- Demonstration documents must be visibly marked DRAFT / DEMO and disclose unresolved items. A parity or negative-residual failure blocks document output.
- Published master versions and saved revisions are immutable; documents reference one revision and template version.
- Local demo services bind to loopback ports 3020 (UI), 8088 (API), and 5442 (PostgreSQL). Other Docker projects are untouched.
- Production SSO, hosting, retention, operational targets, and client policy approval remain deployment decisions.
- Production uses an explicit calculation effective date, distinct from joining date; an out-of-window configuration blocks saving. Effective-from is mandatory for production publication. No joining-date proration or implicit latest-version selection is assumed.
- Admin manages users, HR record ownership and bounded session settings. C&B alone edits/publishes compensation policies. HR reads/writes owned records; existing Admin/C&B global oversight and read-only Auditor access are retained and documented for client sign-off.
- Database changes use explicit transactional, checksummed migrations. Production startup verifies schema compatibility and restricted database privileges; it never provisions accounts or changes schema. One-time bootstrap is a separate operator command.
- Engine version 1.2.0 distinguishes the stricter validations and corrected order-independent policy condition evaluation. Previously saved calculation snapshots and PDF bytes retain their original engine version and figures.
- Final documents require complete approved configuration and valid candidate fields; production additionally checks dated client configuration and refuses legacy undated/demo issuance. Existing document bytes remain preserved.
