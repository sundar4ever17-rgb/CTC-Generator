import json
from pathlib import Path

ROOT = Path('/proposal')
data = json.loads((ROOT / 'build/proposal-content.json').read_text(encoding='utf-8-sig'))
data.update({
    'title': 'Project Proposal & Estimation',
    'subtitle': 'Automated CTC Generator & Compensation Structuring System',
    'client': 'Brakes India',
    'revision': 'Revision 2 | 13 November 2026',
    'metadata': 'Prepared for: Brakes India    |    Prepared by: [Company / name]',
    'overview': 'Develop the existing demo into a production application for Brakes India to prepare consistent CTC breakups and offer letters. This proposal covers requirements confirmation, UI and backend completion, database preparation, salary rules, testing, client acceptance, deployment and go-live. The estimate credits the existing implementation and covers the remaining production work; previous demo development is not charged again.',
    'estimate_basis': 'Basis: the supplied five-page URD, existing demo source and documentation, and the requested estimation and support updates. All amounts are proposed planning estimates, subject to confirmation of the stated assumptions at the end of Week 1.',
    'demo_note': 'The main demo uses the URD illustration and watermarked drafts. Prior local tests demonstrate working calculation and document paths, including synthetic final-output tests; they are not Brakes India policy approval or production UAT.',
    'scope': 'Reuse the current Next.js, Go and PostgreSQL application. Complete agreed UI and API adjustments; load approved client policies; resolve Basic, rounding and access decisions; finish Brakes India document templates; establish controlled database releases; test the full workflow; and prepare production configuration, deployment and handover.',
    'scope_exclusions': 'URD exclusions remain: payroll, payslips, HRMS/ATS integration, month-wise income-tax computation, investment declarations and CTC-target reverse calculation. No new functional modules are included.',
    'deliverables': [
        'Production application and source code covering the agreed URD scope.',
        'Approved, versioned salary masters and Brakes India CTC/offer PDF templates.',
        'Calculation reference results, internal test report, defect log and signed client UAT results.',
        'Database release scripts, deployment/rollback and backup instructions, release checklist and short HR/C&B walkthrough.'
    ]
})

# Days are total supplier effort across roles, not elapsed duration.
phases = [
    ('P1', 'Requirements and demo review', 'W1', 1, 1, 8,
     'Confirm policies, Basic/rounding choices, access rules, UI changes and acceptance cases; approve the delivery baseline.', [4,1,2,1,0]),
    ('P2', 'UI / frontend development', 'W2–4', 2, 4, 10,
     'Refine the existing candidate/configuration screens, Brakes India styling, validation messages and responsive behaviour.', [1,8,1,0,0]),
    ('P3', 'Backend development', 'W2–5', 2, 5, 10,
     'Complete agreed API changes; validate permissions, sessions, audit history, error handling and document access.', [1,1,8,0,0]),
    ('P4', 'Database and data preparation', 'W2–3', 2, 3, 6,
     'Create versioned migrations and rollback steps; map and validate one approved master-data set and a correction pass.', [0,0,4,0,2]),
    ('P5', 'Salary logic and configuration', 'W3–6', 3, 6, 14,
     'Configure all 20 components; complete agreed URD rule adjustments and reconcile grade, qualification, PF, NPS and balance boundaries.', [1,3,9,1,0]),
    ('P6', 'CTC / offer documents', 'W5–6', 5, 6, 6,
     'Apply one approved CTC format and one offer template; verify fields, wording, figures, pagination and final-output checks.', [0,4,2,0,0]),
    ('P7', 'Internal testing', 'W4–7', 4, 7, 12,
     'Run independent calculation, API, UI, permissions and PDF tests; check dependencies, security configuration and agreed load behaviour.', [0,0,0,12,0]),
    ('P8', 'Internal defect fixes / regression', 'W6–7', 6, 7, 8,
     'Fix defects raised by internal testing and repeat affected regression checks; prepare the UAT release.', [0,4,3,1,0]),
    ('P9', 'Client UAT and final approval', 'W8–9', 8, 9, 8,
     'Support two client test/feedback cycles, resolve in-scope UAT defects, retest and obtain written business approval.', [1,1,1,5,0]),
    ('P10', 'Production deployment / go-live', 'W9–10', 9, 10, 7,
     'Set up the agreed environment, TLS/secrets, backups and logs; load approved masters; rehearse release/rollback; deploy and verify.', [0,1,1,0,5]),
    ('P11', 'Coordination and handover', 'W1–10', 1, 10, 5,
     'Track decisions, prepare handover notes and walkthrough; deploy and verify the UAT release in Week 7 using the included DevOps allowance.', [4,0,0,0,1]),
]
rate = 12000
data['estimate'] = {
    'currency': 'INR', 'hours_per_day': 8, 'days_per_week': 5,
    'day_rate': rate, 'hour_rate': 1500,
    'baseline_weeks': 10, 'planning_window': '10–12 weeks',
    'contingency_weeks': 2, 'warranty_calendar_days': 90, 'warranty_person_days': 6,
    'reserve_percent': 15,
    'maintenance_hours_per_month': 16, 'maintenance_monthly': 24000,
    'maintenance_annual': 288000,
    'role_names': ['Lead / analyst / coordination', 'Frontend engineering', 'Backend / database engineering', 'QA', 'DevOps'],
    'phases': [dict(id=i, name=n, window=w, start_week=s, end_week=e, days=d,
                    cost=d*rate, activity=a, roles=r) for i,n,w,s,e,d,a,r in phases],
    'rate_note': 'A proposed blended billing rate of INR 12,000 per person-day (8 hours), equivalent to INR 1,500/hour, is used for all phases. It includes the mixed delivery roles and normal supplier overhead; it is not a salary rate, existing vendor quote or Brakes India-approved price.',
    'rate_reference': 'The selected rate was checked against published Indian supplier guidance as a reasonableness reference. The project price is calculated from the reviewed work packages, not from a generic application price.',
    'rate_reference_label': 'ITSolvez: Software Development Hourly Rates in India (2026)',
    'rate_reference_url': 'https://itsolvez.com/blog/software-development-hourly-rates-india',
    'schedule_note': 'Baseline: 10 elapsed working weeks from kickoff, with a 10–12-week planning window including up to two weeks of delivery contingency. A working week has five working days; holidays and client delays may move calendar dates. Parallel phases must not be added as consecutive weeks.',
    'staffing_note': 'Two engineers cover frontend and backend work during the build, supported by shared QA, lead/analyst and DevOps capacity. They are allocated as needed, not five people full-time for ten weeks. Brakes India reviewer time is a client dependency and is excluded from supplier person-days.',
    'cost_note': 'The 15% reserve covers uncertainty in policy interpretation, UAT rework and environment preparation. It is a planning allowance, not an automatic additional charge; use requires agreement. The two-week schedule buffer is covered by this reserve, not charged as another phase.',
    'double_count_note': 'Development includes developer checks. P7 covers independent testing; P8 covers its fixes and regression. P9 includes client-UAT fixes only. Coordination and the warranty provision are explicit; no extra management or warranty fee is added.',
    'exclusions': 'All figures exclude applicable taxes, hosting/infrastructure consumption, domains, paid licences, external security certification and optional maintenance. Production setup labour is included in P10. Brakes India provides or directly funds the agreed environments. Payment milestones will be agreed in the commercial order.'
}
est = data['estimate']
est['delivery_days'] = sum(p['days'] for p in est['phases'])
est['delivery_cost'] = sum(p['cost'] for p in est['phases'])
est['warranty_cost'] = est['warranty_person_days'] * rate
est['base_days'] = est['delivery_days'] + est['warranty_person_days']
est['base_cost'] = est['delivery_cost'] + est['warranty_cost']
est['reserve_cost'] = est['base_cost'] * est['reserve_percent'] // 100
est['total_budget'] = est['base_cost'] + est['reserve_cost']
est['role_days'] = [sum(p['roles'][i] for p in est['phases']) for i in range(5)]
assert est['delivery_days'] == 94 and est['base_days'] == 100
assert est['total_budget'] == 1380000
assert all(sum(p['roles']) == p['days'] for p in est['phases'])

data['milestones'] = [
    ['End W1', 'Requirements, policies, estimate assumptions and acceptance cases approved.'],
    ['End W6', 'Configured application and approved PDF templates ready for final internal validation.'],
    ['End W7', 'Internal checks passed; delivery team has deployed and verified the UAT release.'],
    ['End W9', 'Two UAT cycles closed and written client sign-off received.'],
    ['End W10', 'Production deployed, final verification passed and handover complete.'],
    ['W11–12, if needed', 'Contingency for agreed rework; not an extra scheduled work package.']
]
data['warranty'] = {
    'period': '90 calendar days from production go-live, included in the base estimate. The first two weeks provide closer release support within the same warranty period.',
    'coverage': 'Correct reproducible defects against the accepted scope and approved calculation results. The six-person-day allowance is an internal cost provision, not a client hour cap or a separate charge for covered defects.',
    'limits': 'New features, changed business policies requiring new logic, third-party outages and client-caused infrastructure changes are outside defect warranty.'
}
data['maintenance'] = {
    'pricing': 'Optional maintenance starts after warranty: INR 24,000/month for up to 16 hours. A full 12-month maintenance term costs INR 2,88,000, excluding taxes and infrastructure.',
    'coverage': 'Application incident investigation, routine dependency/security updates, release verification and configuration assistance within existing functionality, within the included hours.',
    'terms': 'Unused hours expire monthly. Additional work is estimated first and charged at INR 1,500/hour only after approval. Major enhancements and new policy operations require a separate estimate.',
    'service': 'Proposed support window: Monday–Friday, 09:00–18:00 IST, excluding agreed holidays. Acknowledge critical production blockers within four business hours and other requests within one business day. Resolution time depends on diagnosis; no 24×7 coverage is assumed. Confirm these service targets in the support agreement.'
}
data['client_requirements'] = [
    'By the end of Week 1: approve the grade matrix, Basic ranges/selection, Conveyance caps, Professional Allowance slabs, Variable Pay bands and LTM schedules.',
    'Confirm Superannuation, ESI wage ceiling/basis, Annual Special Allowance slabs/classification, policy-payment eligibility/amounts/periods, rounding and annualisation. Finance/Tax confirms uncapped HRA.',
    'Supply the official Brakes India CTC/offer formats, required fields, signatory details, HR/C&B edit permissions and approved manual calculation examples. Missing rules are not inferred from demo or test data.',
    'Nominate HR/C&B, Finance and IT decision-makers. Return consolidated decisions within two working days and provide reviewers for two five-working-day UAT cycles in Weeks 8 and 9.',
    'Provide source access and deployment constraints at kickoff; ready the UAT environment by Week 7 and production access before Week 9. Delayed inputs or approvals move dates and may require re-planning.'
]
data['execution_assumptions'] = [
    'One Brakes India organisation, an English web application, one CTC format and one offer template; retain the existing technology stack and single-screen workflow.',
    'One UAT environment and one production environment. One clean, approved master-data set, one initial production load and one correction pass are included; historical candidate/payroll migration is excluded.',
    'Policies fit the existing fixed, percentage, conditional and scheduled rule types, with bounded changes within the URD. Final matrices, data volume, concurrency and retention requirements are confirmed in Week 1.',
    'Existing role-based login is retained. Corporate SSO, high availability/disaster-recovery infrastructure, external integrations and independent penetration-test certification are not included; mandatory client IT requirements must be assessed before baseline approval.'
]
data['testing_requirements'] = [
    'Internal tests cover all 20 components, grade/qualification boundaries, all PF branches, both NPS regimes/opt-out, ESI eligibility, balancing failures, annual payments, permissions, saved revisions and matching PDFs.',
    'UAT cycle 1 validates real client scenarios; cycle 2 verifies consolidated feedback and fixes. Approved manual results must match to the rupee using the agreed rounding convention.',
    'Release requires written HR/C&B acceptance, approved masters/template, no open critical or high-severity defects, and an agreed disposition for any minor issues.'
]
data['deployment_requirements'] = 'Before release: validate production access, TLS/secrets, controlled database migrations, backup/restore, logs/monitoring and rollback; remove demo credentials/data, load and reconcile approved masters and users. After deployment: verify login and permissions, representative calculations, saved revisions, CTC/offer PDFs and audit history; record joint business/IT go-live approval.'
data['conclusion'] = 'Plan for 10 weeks to production go-live, with a 10–12-week delivery window and a recommended budget of INR 13,80,000 including contingency and a 90-day defect warranty, excluding taxes and infrastructure. Optional maintenance is INR 24,000/month after warranty. Confirm the pending policies, environment requirements and commercial basis in Week 1 before fixing the delivery baseline.'
data['basis'] = 'Sources: client URD; demo source, README, requirements, architecture and verification records. Effort estimates are engineering judgement based on reuse and remaining work; previous demo test results are not current production certification.'

# Remove obsolete commercial placeholders to avoid stale claims in the revised source.
for key in ['timeline_intro', 'timeline_headers', 'timeline_rows', 'timeline_total',
            'cost_intro', 'cost_headers', 'cost_rows', 'assumptions']:
    data.pop(key, None)

(ROOT / 'build/proposal-content-v2.json').write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding='utf-8')
print(json.dumps({'delivery_days': est['delivery_days'], 'base_days': est['base_days'],
                  'base_cost': est['base_cost'], 'reserve': est['reserve_cost'],
                  'planning_budget': est['total_budget'], 'role_days': est['role_days']}))
