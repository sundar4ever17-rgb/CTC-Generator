# URD traceability and clarification register

Source: `User Requirement Document (URD) - CTC generator.pdf`, five pages. This file describes intended coverage; actual verification results are recorded separately in `verification.md`.

| URD requirement | Application coverage | Policy status |
| --- | --- | --- |
| FR-01 Five component groups | Generator, stored calculation lines, CTC and offer annexure | All 20 named components retained |
| FR-02 Basic grade range | Versioned grade editor and backend validation | Actual grade annexure and selection method pending |
| FR-03 Qualification gate | NONE/CA/CMA/CS input; grade-specific slabs | Recognition/slab values pending |
| FR-04 Uncapped balancing HRA | Exclusion target minus already calculated fixed components | Finance/Tax treatment approval pending |
| FR-05 Three-tier Employer PF | Decimal calculation using Basic threshold and half of Inclusion | URD parameters seeded; client confirmation of applicability/boundaries remains |
| FR-06 Optional regime-dependent NPS | Explicit opt-in; Old 10%, New 14% of Basic | URD rates seeded |
| FR-07 Balancing Special Allowance | Inclusion target minus Basic and Professional Allowance | Nonnegative required |
| FR-08 Gross input and parity | Equal targets, actual group sums, Gross reconciliation | Precision convention must be approved |
| FR-09 Sequential engine | Go calculation module with upstream-only bases | No iterative/circular formulas |
| FR-10 ESI and Gratuity | Versioned rates and ESI wage parameters; Gratuity on Inclusion | ESI ceiling/bases/boundary pending |
| FR-11 Policy-Based Payments | Typed amount/percentage rules, job/location/grade conditions | All five payment policies pending |
| FR-12 Variable Pay | Grade/location rule matching, explicit bases and annual schedules | Percentages/bases/LTM schedules pending |
| FR-13 CTC and Offer PDFs | Saved revision + versioned template; draft/final gates | Official offer template/fields pending |
| FR-14 Audit trail | Database audit events and revision history | Retention/access scope need production confirmation |

## Additional required components

Superannuation: policy percentage and approved upstream base where applicable. The URD worked example explicitly has none; this is an illustration, not a company-wide exclusion.

Annual Special Allowance: a separate annual payout slab. Its group assignment is configurable after client confirmation; it is never merged with monthly Special Allowance or counted twice.

Conveyance: grade amount/cap. The URD example uses 7300; the production grade matrix is not supplied.

## Clarifications retained in the application

1. Complete grade ranges, Conveyance amounts/caps, Professional Allowance slabs and variable bands.
2. Basic selection/default method.
3. Superannuation eligibility, rates and basis.
4. ESI ceiling, wage definitions and comparison boundary.
5. Annual Special Allowance eligibility, slabs, annual timing and classification.
6. Conditions, amounts/bases, caps and periods for 6th Day, Field Service, Relocation, Deputation and Additional HRA.
7. Performance percentages, target/guaranteed treatment and LTM schedules.
8. Rounding, input precision, annualization, one-time benefits and proration.
9. Finance/Tax acceptance of uncapped HRA.
10. Official offer template, required personal/employment fields and authorized signatory.
11. Meaning of C&B-only edit access versus HR input editing.
12. Effective-date selection, retention, SSO, hosting and operational requirements.

## Demonstration versus approved configuration

The seeded URD illustration can demonstrate known calculations and watermarked documents while making missing policy data visible. Final output is permitted only after client configuration is complete, required confirmations are supplied, the configuration is published, and calculation/document validation passes.

Synthetic values in the acceptance suite belong to an isolated test database and verify arithmetic and application behavior. They are not proposed business rules.

The current engine exposes supported parameterized operations. A client-supplied policy requiring a new operation must be reviewed and implemented explicitly; the application does not accept arbitrary formula scripts or silently reinterpret that policy.
