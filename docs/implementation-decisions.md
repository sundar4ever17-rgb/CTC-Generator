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
