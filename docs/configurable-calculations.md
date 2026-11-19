# Configurable calculations

The existing URD calculation remains available for legacy configuration versions.
Configuration versions with `data.rules.version = 2` use the configurable engine.
Converting a version does not save it, publish it, or recalculate historical records.

## Configuration workflow

1. Sign in as Compensation & Benefits and open **Configuration**.
2. Select the existing base version and choose **Configure calculation rules**.
3. Use **Calculation rules > Components** to add or edit a salary component.
   Choose its group, payment period, amount expression, scope, eligibility, and
   validation limits. Expressions can reference inputs, master parameters, other
   components, group targets, and appropriate group totals.
4. Use **Groups & targets** for category names, display order, Gross participation,
   target formulas, and the balancing component. Each Gross group has exactly one
   unconditional balancing component. Additional groups contribute to annual CTC.
5. Use **Policy inputs** to define additional amount, number, yes/no, choice, or date
   fields. The generator displays these fields for the selected configuration.
6. Inspect **Dependencies**, then **Preview draft** with representative candidate
   inputs. Preview does not create candidate cases or calculation revisions.
7. Save a new version and publish when the rules are approved and complete.

Existing grade, location, statutory parameter, rounding, confirmation, and document
template controls remain available. In a converted policy, the generic rule editor
owns calculation definitions; the old benefit editor is no longer a second editor
for those formulas. A reference to an existing master parameter reads its value
from the same configuration version.

The initial conversion retains all 20 URD components and the 50:50 target formulas.
A different target ratio or accounting classification is a change to business
policy, not a correction to the URD. Configuration capability does not imply that
a particular alternative policy has been approved.

## Rules and calculation behavior

Rules are structured expressions, not executable JavaScript, Go, or SQL. Supported
operations include arithmetic, comparisons, AND/OR/NOT, if/then/otherwise, minimum
and maximum, explicit lookup tables, annual schedules, and group residuals.

Rule scope chooses which rule applies to a component. Eligibility determines
whether the selected rule applies to the candidate. A failed eligibility condition
produces explicit non-applicability; missing information remains unresolved.
Ambiguous scopes are rejected. Use one conditional expression for branching that
cannot be represented as disjoint equality scopes.

`component.CODE` references a rounded calculated component amount.
`target.GROUP` references the independently calculated target, while `total.GROUP`
references actual monthly component totals. These are distinct: using an actual
total that includes the current component would create a circular dependency.
The engine derives dependencies from formulas, conditions, and limits, and rejects
cycles and missing references before a configuration can be saved.

A balancing rule calculates its group target minus all other applicable components
in that group. Adding an Inclusion component therefore reduces Special Allowance
under the converted URD policy. Adding an Exclusion component reduces HRA. Negative
residuals block documents; the engine does not silently clamp them to zero.

Gross groups contain monthly components and must reconcile individually to their
targets. Their targets and actual totals must reconcile exactly to Monthly Gross.
For an unequal configured split, each group's target difference is checked rather
than requiring the two different targets to equal one another.

Validation minimum/maximum values reject an out-of-range result. Use the minimum
expression operation to cap a result intentionally. Monthly, annual, and one-time
amounts have explicit periods. Monthly rounded amounts are annualized by 12 as in
the original engine. New components do not silently enter statutory wage bases:
the configured expressions determine those bases.

The supported expression operations allow new components and many new policies
without backend changes. A new operation, payroll integration, reverse CTC solver,
or other capability beyond this expression language still requires development.

## Persistence and compatibility

`configurations.data` is the authoritative versioned snapshot. New group, component,
and rule-set database projections are inserted from that snapshot transactionally.
Stable component codes are registered on insertion; labels, membership, ordering,
and rules belong to individual configuration versions.

The `004_configurable_rules` migration adds the projections and integrity guards.
It does not rewrite historical configuration JSON, calculated results, or stored
PDF bytes. The legacy `basic` projection can be null when a v2 policy calculates
Basic rather than accepting a manually entered value; input snapshots preserve
what the user actually supplied.

Saved revisions retain the actual evaluator version, rule traces, dynamic group
metadata, and input/result snapshots. Historical displays use those snapshots.
Saved PDF downloads retain their original bytes and checksum. C&B authoring,
publication, HR record ownership, CSRF protection, and existing final-document
gates remain enforced.

Production deployments require the new migration and updated
`scripts/grant-runtime-access.sql`. The runtime receives insert permission for
component identities and the new projection tables, without schema privileges.
The application must be upgraded with the migration: older binaries intentionally
reject newer schema versions. To continue using an old policy, select its legacy
configuration within the upgraded application. A full binary/database rollback
requires the normal backup and restore process.

## Validation

Use `compose.rules-test.yaml` for isolated synthetic testing. The rule API tests
exercise conversion, equivalence, new components and categories, custom inputs,
dependencies, invalid rules, configurable targets, calculated Basic, persistence,
permissions, and final documents. The browser tests exercise the guided editor,
preview, save, generator, mobile layout, and read-only HR access.

The original Go, API, browser, and production security suites remain applicable.
The production runner also executes the rules API suite under the restricted
database runtime role. See `tests/README.md` for commands.

Verified on 2026-11-18:

- Go engine and application tests passed, including legacy/v2 differential cases
  and isolated migration/history-preservation tests.
- Existing API acceptance: 16/16; configurable-rule API workflows: 9/9.
- Existing browser workflows: 12/12; configurable-rule browser workflows: 7/7.
- Frontend TypeScript and production build passed; mobile layout checked.
- Production-mode security: 12 passed, with the optional idle-expiry simulation
  skipped because that runner did not supply its test database container hook.
- Configurable-rule workflows passed under the restricted production runtime role,
  including both final PDFs. Backup/restore and stored PDF checksums passed.
- Local review services at `http://localhost:3022` were updated. Existing
  configuration, revision, and document counts and content hashes matched before
  and after migration. Conversion/preview was verified without saving a new policy.

Local evidence is under `artifacts/configurable-engine/` and
`artifacts/ctc-upgrade-test-prod-20261118093243-b594/`. These are ignored local
artifacts, not production policy data. Test containers were stopped after checks;
their isolated volumes are retained.
