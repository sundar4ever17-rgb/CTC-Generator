"""Build Revision 2 from the single approved estimation data source."""
from pathlib import Path
import json
import re

from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR, MSO_AUTO_SIZE
from pptx.enum.shapes import MSO_SHAPE

ROOT = Path('/proposal')
SOURCE = json.loads((ROOT / 'build/proposal-content-v2.json').read_text(encoding='utf-8-sig'))
E = SOURCE['estimate']
OUT = ROOT / 'ctc-generator-project-proposal-v2.pptx'
INK, WHITE, PRIMARY, MUTED, ACCENT = '172B4D', 'FFFFFF', '005A70', 'EDF2F6', 'AE6A20'
PHASES = E['phases']
P = {p['id']: p for p in PHASES}
delivery_days = sum(p['days'] for p in PHASES)
delivery_cost = sum(p['days'] * E['day_rate'] for p in PHASES)
warranty_cost = E['warranty_person_days'] * E['day_rate']
base_cost = delivery_cost + warranty_cost
reserve_cost = base_cost * E['reserve_percent'] // 100
budget = base_cost + reserve_cost
maintenance_monthly = E['maintenance_hours_per_month'] * E['hour_rate']
maintenance_annual = maintenance_monthly * 12
role_days = [sum(p['roles'][i] for p in PHASES) for i in range(len(E['role_names']))]
assert delivery_days == E['delivery_days'] == sum(role_days)
assert delivery_cost == E['delivery_cost']
assert warranty_cost == E['warranty_cost']
assert base_cost == E['base_cost']
assert reserve_cost == E['reserve_cost']
assert budget == E['total_budget']
assert maintenance_monthly == E['maintenance_monthly']
assert maintenance_annual == E['maintenance_annual']
assert E['day_rate'] == E['hours_per_day'] * E['hour_rate']
assert all(p['cost'] == p['days'] * E['day_rate'] == sum(p['roles']) * E['day_rate'] for p in PHASES)

def money(value, prefix=False):
    s = str(int(value))
    if len(s) > 3:
        lead, tail = s[:-3], s[-3:]
        parts = []
        while lead:
            parts.insert(0, lead[-2:])
            lead = lead[:-2]
        s = ','.join(parts + [tail])
    return ('INR ' if prefix else '') + s

def week_span(ids):
    lo = min(P[i]['start_week'] for i in ids)
    hi = max(P[i]['end_week'] for i in ids)
    return f'W{lo}' if lo == hi else f'W{lo}–{hi}'

prs = Presentation()
prs.slide_width = Inches(13.333)
prs.slide_height = Inches(7.5)
prs.core_properties.title = 'Brakes India | Automated CTC Generator | Project Proposal & Estimation'
prs.core_properties.subject = SOURCE['subtitle']
prs.core_properties.author = '[Company / name]'
prs.core_properties.keywords = 'Brakes India, CTC Generator, production, project estimate'
prs.core_properties.comments = SOURCE['revision'] + '; all commercial figures are planning estimates.'
slides_meta = []

def rgb(v):
    return RGBColor.from_string(v)

def box(slide, x, y, w, h, fill=MUTED):
    s = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(x), Inches(y), Inches(w), Inches(h))
    s.fill.solid()
    s.fill.fore_color.rgb = rgb(fill)
    s.line.fill.background()
    return s

def text(slide, value, x, y, w, h, size=24, color=INK, bold=False, align=PP_ALIGN.LEFT):
    s = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    tf = s.text_frame
    tf.clear()
    tf.word_wrap = True
    tf.auto_size = MSO_AUTO_SIZE.NONE
    tf.margin_left = tf.margin_right = tf.margin_top = tf.margin_bottom = 0
    tf.vertical_anchor = MSO_ANCHOR.TOP
    for i, line in enumerate(value.split('\n')):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.text = line
        p.alignment = align
        p.space_before = Pt(0)
        p.space_after = Pt(8)
        p.line_spacing = 1.10
        for r in p.runs:
            r.font.name = 'Arial'
            r.font.size = Pt(size)
            r.font.bold = bold
            r.font.color.rgb = rgb(color)
    return s

def slide(kicker, title, notes, source='Estimate', layout='content'):
    sl = prs.slides.add_slide(prs.slide_layouts[6])
    sl.background.fill.solid()
    sl.background.fill.fore_color.rgb = rgb(WHITE)
    box(sl, .70, .62, .56, .055, ACCENT)
    text(sl, kicker, .70, .79, 11.93, .34, size=15, color=PRIMARY, bold=True)
    text(sl, title, .70, 1.34, 11.93, 1.14, size=32, bold=True)
    idx = len(prs.slides)
    if idx != 1:
        box(sl, .70, 6.50, 11.93, .014, MUTED)
        text(sl, source, .70, 6.64, 10.70, .24, size=12, color=PRIMARY)
        text(sl, str(idx), 11.93, 6.64, .70, .24, size=12, color=PRIMARY, align=PP_ALIGN.RIGHT)
    sl.notes_slide.notes_text_frame.text = notes
    slides_meta.append({'section': kicker, 'title': title, 'layout': layout, 'notes': notes})
    return sl

def bullets(sl, lines, y=2.67, gap=.78, size=24):
    for i, line in enumerate(lines):
        yy = y + i * gap
        box(sl, .75, yy + .13, .095, .095, PRIMARY)
        text(sl, line, 1.04, yy, 11.3, .61, size=size)

def table(sl, rows, widths, y=2.65, height=3.4, size=22):
    t = sl.shapes.add_table(len(rows), len(widths), Inches(.70), Inches(y), Inches(sum(widths)), Inches(height)).table
    for i, width in enumerate(widths):
        t.columns[i].width = Inches(width)
    for ri, row in enumerate(rows):
        for ci, value in enumerate(row):
            c = t.cell(ri, ci)
            c.text = str(value)
            c.margin_left = c.margin_right = Inches(.18)
            c.margin_top = c.margin_bottom = Inches(.08)
            c.vertical_anchor = MSO_ANCHOR.MIDDLE
            c.fill.solid()
            c.fill.fore_color.rgb = rgb(PRIMARY if ri == 0 else (MUTED if ri % 2 else WHITE))
            c.text_frame.word_wrap = True
            c.text_frame.auto_size = MSO_AUTO_SIZE.NONE
            for p in c.text_frame.paragraphs:
                p.space_before = p.space_after = Pt(0)
                p.line_spacing = 1.05
                for r in p.runs:
                    r.font.name = 'Arial'
                    r.font.size = Pt(size)
                    r.font.bold = ri == 0
                    r.font.color.rgb = rgb(WHITE if ri == 0 else INK)
    return t

# Main proposal: claims and figures are drawn from the shared revision source.
s = slide('1. Project Overview',
          'Brakes India can progress from demo to production.',
          'This revision estimates remaining production work, crediting the existing demo. '
          'It covers UI, backend, database, salary logic, testing, client acceptance, deployment and handover. '
          'The budget includes the defect-warranty provision and a separately agreed contingency allowance. '
          'Taxes, infrastructure and optional maintenance are excluded. Confirm the policy and environment assumptions in Week 1.',
          layout='title')
text(s, SOURCE['subtitle'], .70, 2.67, 11.6, 1.05, size=27, color=PRIMARY)
box(s, .70, 4.12, 11.93, 1.08)
text(s, f"{E['baseline_weeks']}-week baseline  |  {money(budget, True)} planning budget", .96, 4.43, 11.4, .50, size=26, bold=True)
text(s, SOURCE['revision'], .74, 5.67, 8.8, .35, size=18)
text(s, '[Presenter]', 10.1, 5.67, 2.45, .35, size=18, align=PP_ALIGN.RIGHT)

s = slide('Proposal agenda', 'Review scope, delivery, cost and release decisions.',
          'Review the reusable demo foundation and agreed URD scope first. '
          'Then discuss effort, schedule, cost, client acceptance, deployment and ongoing support. '
          'The appendix contains every phase estimate and key calculation rules. '
          'The Word/PDF proposal is the detailed commercial and delivery reference.')
bullets(s, ['Scope and demo reuse', 'Timeline, resources and cost', 'Acceptance, release and support', 'Detailed phase estimates'], gap=.82)

s = slide('2. Existing Demo Application', 'The demo already supports the core offer workflow.',
          'Existing source contains sequential calculations, twenty components, saved revisions, versioned masters, '
          'role controls, audit and PDF generation. Reuse these components rather than charging for a full rebuild. '
          'The seed values are an illustration, and prior local checks are not Brakes India UAT. '
          'Production needs approved policies, official templates and operational preparation.', source='Demo review')
table(s, [['Available', 'Production work'], ['Calculations and PDFs', 'Approve policies/templates'],
          ['Inputs and revisions', 'Validate client scenarios'], ['Masters and access', 'Confirm production controls'],
          ['Local deployment', 'Production setup']], [5.96, 5.97], height=3.25)

s = slide('3. Proposed Development Scope', 'Production retains the URD’s five groups and 20 components.',
          'Retain the existing Next.js, Go and PostgreSQL stack and single-screen workflow. '
          'Complete bounded UI/API and rule adjustments, client configuration, official CTC/offer formats and operational controls. '
          'Payroll, payslips, HRMS/ATS, tax computation, investment declarations and reverse-CTC solving remain excluded. '
          'Corporate SSO and additional infrastructure requirements must be assessed before baseline approval.', source='URD')
bullets(s, ['Gross drives equal groups without circular formulas.',
            'Invalid balances block CTC and offer output.',
            'Qualification, grade and policy govern allowances.'], gap=1.02)

s = slide('5. Project Deliverables', 'Delivery includes working software, test evidence and operational handover.',
          'Deliver the production application and source, approved versioned masters and Brakes India PDF templates. '
          'Supply calculation reference results, internal reports, defect records and signed UAT results. '
          'Handover includes database release scripts, deployment and rollback steps, backup instructions and an HR/C&B walkthrough. '
          'Client acceptance and operational readiness are release conditions.', source='Scope')
bullets(s, ['Application, source and configured masters', 'Approved CTC and offer PDFs',
            'Test evidence and signed UAT results', 'Release instructions and HR/C&B walkthrough'], gap=.82)

s = slide('6. Estimated Development Timeline', f"Parallel delivery targets go-live in {E['baseline_weeks']} weeks.",
          f"The baseline is {E['baseline_weeks']} elapsed working weeks, with a {E['planning_window']} planning window. "
          'Frontend, backend, database and rules progress in parallel. Internal QA starts during development. '
          'The UAT environment is deployed and verified by Week 7, client cycles run in Weeks 8–9, '
          'and production release follows written acceptance. Up to two contingency weeks are included in the planning window; client delays may move dates.')
timeline = [('Confirm', ['P1']), ('Build', ['P2', 'P3', 'P4', 'P5', 'P6']),
            ('Test', ['P7', 'P8']), ('UAT', ['P9']), ('Go-live', ['P10'])]
chart_x, chart_w = 3.18, 9.44
for i, (label, ids) in enumerate(timeline):
    y = 2.58 + i * .60
    start = min(P[k]['start_week'] for k in ids)
    end = max(P[k]['end_week'] for k in ids)
    text(s, label, .82, y+.07, 2.2, .36, size=22)
    box(s, chart_x, y, chart_w, .47, MUTED)
    x = chart_x + (start - 1) / E['baseline_weeks'] * chart_w
    width = (end - start + 1) / E['baseline_weeks'] * chart_w
    box(s, x, y, width, .47, PRIMARY)
    text(s, week_span(ids), x+.06, y+.08, max(width-.12, .65), .32, size=20, color=WHITE, bold=True)
text(s, f"Planning window: {E['planning_window']}", .82, 5.84, 11.45, .40, size=22, color=PRIMARY, bold=True)

s = slide('Estimation basis', 'Effort pricing uses a transparent blended rate.',
          f"One person-day is {E['hours_per_day']} hours. The proposed blended billing rate is {money(E['day_rate'], True)}/day, "
          f"equivalent to {money(E['hour_rate'], True)}/hour; it includes supplier overhead and is not an approved vendor quotation. "
          'Two engineers cover the build, with shared QA, lead/analyst and DevOps. '
          'The table covers delivery effort; the warranty allowance is separate. Client reviewer time is excluded.')
role_labels = ['Lead/analyst', 'Frontend', 'Backend/database', 'QA', 'DevOps']
table(s, [['Role', 'Person-days']] + [[name, days] for name, days in zip(role_labels, role_days)], [8.15, 3.78], y=2.55, height=3.05)
text(s, f"{money(E['day_rate'], True)}/day  |  {E['hours_per_day']} hours/day  |  {money(E['hour_rate'], True)}/hour", .83, 5.85, 11.70, .40, size=21, color=PRIMARY)

s = slide('7. Project Cost', f"The recommended planning budget is {money(budget, True)}.",
          f"Delivery costs {money(delivery_cost, True)} for {delivery_days} person-days. "
          f"The {E['warranty_calendar_days']}-day warranty provision adds {money(warranty_cost, True)}, giving a {money(base_cost, True)} base. "
          f"A {E['reserve_percent']}% reserve of {money(reserve_cost, True)} covers agreed policy, UAT or environment uncertainty. "
          'Reserve use needs agreement; it is not automatically charged. No extra management or warranty fee is added. '
          'Payment milestones remain for the commercial order.')
table(s, [['Cost element', 'INR'], ['Delivery', money(delivery_cost)],
          [f"{E['warranty_calendar_days']}-day warranty", money(warranty_cost)], ['Base', money(base_cost)],
          [f"Reserve ({E['reserve_percent']}%)", money(reserve_cost)], ['Planning budget', money(budget)]], [7.90, 4.03], y=2.52, height=3.08)
text(s, 'Excludes taxes, infrastructure and maintenance.', .84, 5.85, 11.60, .40, size=21, color=PRIMARY)

s = slide('8. Assumptions and Client Requirements', 'Two UAT cycles must close before release approval.',
          'Approve policies, document formats, access responsibilities and reference calculations in Week 1. '
          'Nominate HR/C&B, Finance and IT reviewers; return consolidated decisions within two working days. '
          'Each UAT cycle has five working days. Internal tests cover all components, boundaries, permissions and PDFs. '
          'The second client cycle verifies fixes. Require written acceptance, matching approved results and no open critical/high defects.', source='Acceptance')
bullets(s, [f"Policies and examples: Week {P['P1']['end_week']}.",
            f"UAT: Weeks {P['P9']['start_week']} and {P['P9']['end_week']}.",
            'Fix feedback; reconcile figures.', 'Written sign-off; no critical/high defects.'], gap=.82)

s = slide('Deployment and Go-Live', 'Production release requires operational and business verification.',
          'Include one UAT and one production environment. UAT deployment/verification is covered in coordination; '
          'production setup labour is included in deployment. Brakes India provides or funds the environments. '
          'Load one clean approved master-data set and a correction pass; exclude historical payroll/candidate migration. '
          'Remove demo data and credentials, verify representative calculations and documents, and record joint business/IT approval.', source='Release')
bullets(s, ['Configure TLS, secrets, backups and logs.', 'Load approved masters; verify access.',
            'Rehearse migration, restore and rollback.', 'Verify calculations, PDFs and audit history.'], gap=.82)

s = slide('Post Go-Live Warranty', f"The included warranty covers defects for {E['warranty_calendar_days']} days.",
          SOURCE['warranty']['period'] + ' ' + SOURCE['warranty']['coverage'] + ' ' + SOURCE['warranty']['limits'], source='Warranty')
bullets(s, ['Starts at production go-live.',
            'First two weeks: closer release support.',
            f"{E['warranty_person_days']} person-days reserved; no client-hour cap.",
            'Accepted-scope defects; enhancements excluded.'], gap=.82)

s = slide('Post Go-Live Maintenance', 'Maintenance provides a defined monthly support allowance.',
          f"After warranty, optional maintenance is {money(maintenance_monthly, True)}/month for up to {E['maintenance_hours_per_month']} hours. "
          'Unused hours expire; extra work needs approval. Cover incident analysis, routine updates, release checks and existing-rule configuration. '
          'Support is Monday–Friday, 09:00–18:00 IST excluding agreed holidays. Proposed acknowledgement: critical blockers within four business hours; others within one business day. '
          'Resolution depends on diagnosis; confirm service targets in the support agreement.', source='Support')
table(s, [['Item', 'Allowance / fee'], ['Monthly fee', money(maintenance_monthly, True)],
          ['Included hours', str(E['maintenance_hours_per_month'])], ['12-month term', money(maintenance_annual, True)],
          ['Additional hour', money(E['hour_rate'], True)]], [6.00, 5.93], height=2.95)
text(s, 'After warranty; business hours only.', .84, 5.85, 11.60, .40, size=21, color=PRIMARY)

s = slide('9. Conclusion', 'Approve the baseline to schedule production delivery.',
          'The existing demo provides the starting point. Confirm the pending policies, official templates, '
          'environment constraints, reviewers and commercial assumptions in Week 1. '
          'The baseline is subject to these assumptions and timely client decisions. '
          'The appendix provides the full effort and cost breakdown; parallel phase windows do not add together.', layout='statement')
box(s, .70, 2.75, 11.93, 1.02)
text(s, f"{E['planning_window']}  |  {money(budget, True)} planning budget", .97, 3.02, 11.38, .50, size=28, color=PRIMARY, bold=True)
text(s, 'Confirm policies, environments, reviewers and commercial terms.', .96, 4.24, 11.35, 1.10, size=27)
text(s, 'Taxes/infrastructure excluded; maintenance optional.', .96, 5.78, 11.35, .41, size=21, color=PRIMARY)

phase_short = {'P1':'Requirements', 'P2':'Frontend', 'P3':'Backend', 'P4':'Database',
               'P5':'Salary logic', 'P6':'PDFs', 'P7':'Testing', 'P8':'Fixes/regression',
               'P9':'UAT', 'P10':'Deployment', 'P11':'Coordination'}
phase_sets = [
    (['P1','P2','P3','P4'], 'Initial development completes the reusable foundation.',
     'P1 confirms requirements, policies and acceptance cases. P2 refines the existing user interface. '
     'P3 completes API and access-control adjustments. P4 establishes migrations and prepares approved master data. '
     'Costs equal effort multiplied by the shared daily rate. Windows show elapsed timing; they overlap. '
     'Detailed activities and assumptions are in the Word/PDF proposal.'),
    (['P5','P6','P7','P8'], 'Rules, documents and internal testing establish release quality.',
     'P5 configures all twenty components and reconciles salary boundaries. P6 completes the approved CTC and offer formats. '
     'P7 provides independent functional, calculation, permission, document and operational testing. '
     'P8 fixes internally found defects and repeats regression. Development includes developer checks; '
     'these testing and correction allocations are distinct. Client-UAT fixes belong to P9.'),
    (['P9','P10','P11'], 'Acceptance, deployment and handover complete the delivery effort.',
     'P9 includes two client test/feedback cycles, in-scope UAT fixes and written approval. '
     'P10 covers production preparation, approved data loading, deployment and verification. '
     'P11 covers coordination, the UAT release in Week 7, operational notes and HR/C&B handover. '
     'Client review time is excluded. Warranty effort and contingency appear separately in the cost summary.')
]
for ids, title, notes in phase_sets:
    s = slide('Appendix: Phase estimates', title, notes, source='INR; days=person-days')
    rows = [['Phase', 'Window', 'Days', 'Cost']]
    rows += [[f"{pid} {phase_short[pid]}", P[pid]['window'], str(P[pid]['days']), money(P[pid]['days']*E['day_rate'])] for pid in ids]
    table(s, rows, [5.32, 2.00, 1.25, 3.36], height=3.35, size=22)
    if ids[-1] == 'P11':
        text(s, f"Delivery total: {delivery_days} days / {money(delivery_cost, True)}", .85, 6.00, 11.50, .34, size=20, color=PRIMARY, bold=True)

s = slide('4. Key Features and Functionalities | Appendix', 'Gross fixes targets before balancing.',
          'The URD requires monthly Gross as input, with Inclusion and Exclusion each equal to half of Gross. '
          'Calculate fixed Inclusion values, then Exclusion values, then Special Allowance and HRA. '
          'No circular formulas. Employer PF, NPS and Superannuation already sit within Gross. '
          'Unequal totals or negative balancing values prevent output; HRA has no Basic-based cap.', source='URD')
box(s, .70, 2.62, 11.93, .74)
text(s, 'Inclusion = Exclusion = Gross/2', .95, 2.80, 11.40, .42, size=25, color=PRIMARY, bold=True)
text(s, 'Special Allowance = Gross/2 − Basic − Professional Allowance', .95, 3.73, 11.40, .92, size=24)
text(s, 'HRA (uncapped) = Gross/2 − Conveyance − PF − NPS − Superannuation', .95, 4.76, 11.40, .95, size=24)

s = slide('Appendix: PF/NPS', 'PF and NPS follow the supplied URD rules.',
          'Apply PF tiers in order. Half-Inclusion means fifty percent of Inclusion wages, not half of Basic. '
          'All PF amounts are INR. NPS is optional: when selected, fourteen percent of Basic for New Regime or '
          'ten percent for Old Regime; otherwise zero. Gratuity follows 4.81% of Inclusion and Employer ESI '
          '3.25% subject to the approved wage basis/limit. Keep rates and ceilings configurable.', source='URD')
table(s, [['Condition', 'PF'], ['Basic > 25,000', '12% × Basic'],
          ['Otherwise: half-Inclusion ≤25,000', '12% × half-Inclusion'], ['Otherwise', '3,000']],
      [7.20, 4.73], height=2.85, size=22)
text(s, 'INR. NPS: New 14% / Old 10% of Basic; opt-out 0.', .89, 5.82, 11.55, .43, size=22, color=PRIMARY)

def visible_text(sl):
    values = []
    for sh in sl.shapes:
        if sh.has_text_frame:
            values.append(sh.text)
        if sh.has_table:
            values.extend(c.text for row in sh.table.rows for c in row.cells)
    return '\n'.join(values)

qa, errors = [], []
for idx, (sl, meta) in enumerate(zip(prs.slides, slides_meta), 1):
    vis = visible_text(sl)
    count = len(vis.split())
    bounds = []
    for sh in sl.shapes:
        if (sh.left < Inches(.59) or sh.top < Inches(.59) or
            sh.left+sh.width > prs.slide_width-Inches(.59) or sh.top+sh.height > prs.slide_height-Inches(.59)):
            bounds.append(sh.name)
    row = {'slide': idx, **meta, 'visible_words': count, 'notes_words': len(meta['notes'].split()),
           'title_words': len(meta['title'].split()), 'bounds_violations': bounds, 'text': vis}
    qa.append(row)
    if count > 40: errors.append(f'Slide {idx}: {count} visible words')
    if row['notes_words'] > 80: errors.append(f'Slide {idx}: {row["notes_words"]} note words')
    if row['title_words'] > 12: errors.append(f'Slide {idx}: title exceeds twelve words')
    if bounds: errors.append(f'Slide {idx}: out of bounds: {bounds}')
    if any(s in vis for s in ['[To confirm]', '[To agree]', '[Amount]', '[Currency]', '[Client name]']):
        errors.append(f'Slide {idx}: stale placeholder')

arithmetic = {'delivery_days': delivery_days, 'delivery_cost': delivery_cost,
              'warranty_cost': warranty_cost, 'base_cost': base_cost, 'reserve_cost': reserve_cost,
              'total_budget': budget, 'maintenance_monthly': maintenance_monthly,
              'maintenance_annual': maintenance_annual, 'role_days': role_days}
(ROOT / 'build/presentation-v2-qa.json').write_text(json.dumps(
    {'file': str(OUT), 'slide_count': len(prs.slides), 'arithmetic': arithmetic, 'slides': qa, 'errors': errors}, indent=2), encoding='utf-8')
if errors:
    raise SystemExit('\n'.join(errors))
prs.save(OUT)
print(json.dumps({'file': str(OUT), 'slides': len(prs.slides), 'words': [q['visible_words'] for q in qa],
                  'notes': [q['notes_words'] for q in qa], 'arithmetic': arithmetic, 'errors': errors}))
