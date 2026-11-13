"""Create the revised Brakes India proposal from the shared estimation model."""
import json
from pathlib import Path

ROOT = Path('/proposal')
# Reuse the established document theme/helpers without executing the old content build.
style_source = (ROOT / 'build/build_documents.py').read_text(encoding='utf-8').split('# Page 1:')[0]
style_source = style_source.replace("build/proposal-content.json", "build/proposal-content-v2.json")
exec(compile(style_source, 'document_theme', 'exec'))

from docx.opc.constants import RELATIONSHIP_TYPE as RT

E = data['estimate']
doc.core_properties.title = 'Brakes India | Automated CTC Generator | Project Proposal & Estimation'
doc.core_properties.subject = 'Production delivery estimate, client acceptance, go-live and support'
doc.core_properties.comments = 'Revision 2. Planning estimate based on the supplied URD and existing demo.'
header.text = 'BRAKES INDIA  /  CTC GENERATOR PROPOSAL'
for r in header.runs:
    r.font.name = 'Arial'
    r.font.size = Pt(8)
    r.font.color.rgb = RGBColor.from_string(TEAL)
styles['Title'].font.size = Pt(24)
styles['Subtitle'].font.size = Pt(13)
styles['Heading 2'].font.size = Pt(10.7)
styles['Heading 2'].paragraph_format.space_before = Pt(7)

def money(n):
    s = str(int(n))
    if len(s) > 3:
        last = s[-3:]
        head = s[:-3]
        groups = []
        while head:
            groups.insert(0, head[-2:])
            head = head[:-2]
        s = ','.join(groups + [last])
    return s

def subhead(value):
    doc.add_paragraph(value, style='Heading 2')

def bullet(value, size=10):
    p = doc.add_paragraph(value, style='List Bullet')
    for r in p.runs:
        r.font.size = Pt(size)
    return p

def note(value, size=9):
    return paragraph(value, size=size, after=5)

def link(p, label, url):
    hyp = OxmlElement('w:hyperlink')
    hyp.set(qn('r:id'), p.part.relate_to(url, RT.HYPERLINK, is_external=True))
    run = OxmlElement('w:r')
    props = OxmlElement('w:rPr')
    col = OxmlElement('w:color'); col.set(qn('w:val'), TEAL); props.append(col)
    size = OxmlElement('w:sz'); size.set(qn('w:val'), '16'); props.append(size)
    under = OxmlElement('w:u'); under.set(qn('w:val'), 'single'); props.append(under)
    run.append(props)
    txt = OxmlElement('w:t'); txt.text = label; run.append(txt)
    hyp.append(run); p._p.append(hyp)

# Page 1: client, recommendation, existing foundation and proposed work.
doc.add_paragraph(data['title'], style='Title')
doc.add_paragraph(data['subtitle'], style='Subtitle')
paragraph(data['metadata'], size=9, after=3)
paragraph(data['revision'], size=9, color=TEAL, after=8)
table(['Delivery window', 'Planning budget', 'Included warranty'],
      [[E['planning_window'], 'INR ' + money(E['total_budget']), str(E['warranty_calendar_days']) + ' calendar days']],
      [2.05, 2.7977, 2.1], size=11, padding=95)
note('Budget includes contingency; applicable taxes, infrastructure and optional maintenance are extra.', 9)
heading('1. Project Overview')
paragraph(data['overview'])
note(data['estimate_basis'], 9.2)
heading('2. Existing Demo Application')
table(data['demo_headers'], data['demo_rows'], [1.16, 2.79, 2.9977], size=9.8, padding=70)
note(data['demo_note'], 9)
heading('3. Proposed Development Scope')
paragraph(data['scope'], size=10.2)
note(data['scope_exclusions'], 9.7)

# Page 2: retained URD scope and concrete outputs.
doc.add_page_break()
heading('4. Key Features and Functionalities')
paragraph(data['feature_intro'])
table(data['component_headers'], data['component_rows'], [1.44, 5.5077], size=10, padding=75)
note('Business rules below follow the supplied URD; client-approved policy values and calculation conventions remain prerequisites.', 9.4)
table(data['rules_headers'], data['rules_rows'], [1.44, 5.5077], size=10, padding=70)
heading('5. Project Deliverables')
for item in data['deliverables']:
    bullet(item)

# Page 3: fully reconciled activity, timing, effort and cost breakdown.
doc.add_page_break()
heading('6. Estimated Development Timeline and Activities')
paragraph(E['schedule_note'], size=10.2)
note('One person-day (PD) = eight hours of supplier effort. Costs below are in INR at ' + money(E['day_rate']) + '/PD. Overlapping phases share the delivery team.', 9.5)
rows = []
for p in E['phases']:
    rows.append([p['id'] + '  ' + p['name'] + '\n' + p['activity'],
                 str(p['end_week'] - p['start_week'] + 1) + 'w\n' + p['window'], str(p['days']), money(p['cost'])])
rows.append(['Total through production go-live', '10 weeks', str(E['delivery_days']), money(E['delivery_cost'])])
phase_table = table(['Work package and activities', 'Timing', 'PD', 'Cost (INR)'], rows,
                    [4.5877, .75, .42, 1.19], size=9.7, padding=75)
for row in list(phase_table.rows)[1:-1]:
    cell = row.cells[0]
    label, activity = cell.text.split('\n', 1)
    p = cell.paragraphs[0]
    p.clear()
    for value, weight in [(label + '\n', True), (activity, False)]:
        r = p.add_run(value)
        r.font.name = 'Arial'
        r.font.size = Pt(9.7)
        r.font.bold = weight
        r.font.color.rgb = RGBColor.from_string(INK)
subhead('Delivery milestones')
milestones = '  |  '.join([row[0] + ': ' + row[1] for row in data['milestones'] if row[0] in ['End W1', 'End W7', 'End W9', 'End W10']])
note(milestones, 9.5)
note(E['double_count_note'], 9.3)

# Page 4: commercial model, staffing, warranty and optional operating support.
doc.add_page_break()
heading('7. Project Cost and Post Go-Live Support')
paragraph(E['rate_note'], size=10)
summary = [
    ['Delivery through go-live', str(E['delivery_days']) + ' PD', money(E['delivery_cost'])],
    ['Included warranty provision', str(E['warranty_person_days']) + ' PD', money(E['warranty_cost'])],
    ['Base estimate', str(E['base_days']) + ' PD', money(E['base_cost'])],
    ['Uncertainty reserve', str(E['reserve_percent']) + '% of base', money(E['reserve_cost'])],
    ['Recommended planning budget', 'Base + reserve', money(E['total_budget'])]
]
table(['Cost item', 'Basis', 'Amount (INR)'], summary, [3.4, 1.85, 1.6977], size=10.2, padding=75)
note(E['cost_note'], 9.5)
subhead('Team and effort assumptions')
paragraph(E['staffing_note'], size=9.8)
note('Delivery allocation: ' + '; '.join(n + ' ' + str(d) + ' PD' for n,d in zip(E['role_names'], E['role_days'])) + '. Total: 94 PD. Warranty provision is additional to this allocation.', 9.3)
subhead('7.1 Included warranty')
paragraph(data['warranty']['period'], size=10)
paragraph(data['warranty']['coverage'], size=10)
note(data['warranty']['limits'], 9.4)
subhead('7.2 Optional maintenance after warranty')
table(['Service', 'Proposed estimate'], [
    ['Monthly maintenance', 'INR ' + money(E['maintenance_monthly']) + ' for up to ' + str(E['maintenance_hours_per_month']) + ' hours'],
    ['Full 12-month maintenance term', 'INR ' + money(E['maintenance_annual'])],
    ['Additional approved effort', 'INR ' + money(E['hour_rate']) + '/hour']
], [3.4,3.5477], size=10, padding=70)
paragraph(data['maintenance']['coverage'], size=9.8)
note(data['maintenance']['terms'], 9.5)
note(data['maintenance']['service'], 9.2)
note(E['exclusions'], 9.2)

# Page 5: explicit client inputs and release conditions.
doc.add_page_break()
heading('8. Assumptions and Client Requirements')
subhead('8.1 Client inputs and availability')
for item in data['client_requirements']:
    bullet(item, 9.7)
subhead('8.2 Scope and environment assumptions')
for item in data['execution_assumptions']:
    bullet(item, 9.7)
subhead('8.3 Testing, UAT and final approval')
for item in data['testing_requirements']:
    bullet(item, 9.7)
subhead('8.4 Production setup and go-live verification')
paragraph(data['deployment_requirements'], size=9.7)
heading('9. Conclusion')
paragraph(data['conclusion'], size=10)
note(data['basis'], 8)
p = note('Rate reference only; the selected INR 1,500/hour is a proposed blended planning rate, not a supplier quotation. ', 8)
link(p, E['rate_reference_label'], E['rate_reference_url'])

out = ROOT / 'ctc-generator-project-proposal-v2.docx'
doc.save(out)
print(json.dumps({'file': str(out), 'tables': len(doc.tables), 'expected_pages': 5,
                  'delivery_days': E['delivery_days'], 'budget': E['total_budget']}))
