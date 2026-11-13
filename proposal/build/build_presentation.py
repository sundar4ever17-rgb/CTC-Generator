"""Build the editable, URD-based CTC Generator proposal presentation."""
from pathlib import Path
import json
import re

from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR, MSO_AUTO_SIZE
from pptx.enum.shapes import MSO_SHAPE

ROOT = Path('/proposal')
SOURCE = json.loads((ROOT / 'build/proposal-content.json').read_text(encoding='utf-8-sig'))
OUT = ROOT / 'ctc-generator-project-proposal.pptx'

INK = '172B4D'
WHITE = 'FFFFFF'
PRIMARY = '005A70'
MUTED = 'EDF2F6'
ACCENT = 'AE6A20'

prs = Presentation()
prs.slide_width = Inches(13.333)
prs.slide_height = Inches(7.5)
prs.core_properties.title = 'CTC Generator | Project Proposal'
prs.core_properties.subject = 'Production development proposal based on the client URD and existing demo'
prs.core_properties.author = '[Company / name]'
prs.core_properties.keywords = 'CTC Generator, proposal, URD, production'
prs.core_properties.comments = 'Editable client proposal. Dates, duration and commercial figures are placeholders.'

slides_meta = []

def rgb(v):
    return RGBColor.from_string(v)

def shape(slide, x, y, w, h, fill=MUTED):
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
    tf.margin_left = tf.margin_right = 0
    tf.margin_top = tf.margin_bottom = 0
    tf.vertical_anchor = MSO_ANCHOR.TOP
    for i, line in enumerate(value.split('\n')):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.text = line
        p.alignment = align
        p.space_before = Pt(0)
        p.space_after = Pt(8)
        p.line_spacing = 1.12
        for r in p.runs:
            r.font.name = 'Arial'
            r.font.size = Pt(size)
            r.font.bold = bold
            r.font.color.rgb = rgb(color)
    return s

def make_slide(kicker, title, notes, source='URD', layout='content'):
    sl = prs.slides.add_slide(prs.slide_layouts[6])
    sl.background.fill.solid()
    sl.background.fill.fore_color.rgb = rgb(WHITE)
    shape(sl, .70, .62, .56, .055, ACCENT)
    text(sl, kicker, .70, .79, 11.93, .34, size=15, color=PRIMARY, bold=True)
    text(sl, title, .70, 1.34, 11.93, 1.12, size=32, bold=True)
    idx = len(prs.slides)
    if idx != 1:
        shape(sl, .70, 6.50, 11.93, .014, MUTED)
        text(sl, source, .70, 6.64, 10.70, .24, size=12, color=PRIMARY)
        text(sl, str(idx), 11.93, 6.64, .70, .24, size=12, color=PRIMARY, align=PP_ALIGN.RIGHT)
    sl.notes_slide.notes_text_frame.text = notes
    slides_meta.append({'section': kicker, 'title': title, 'layout': layout, 'notes': notes})
    return sl

def bullets(slide, lines, y=2.65, gap=.78, size=24):
    for i, line in enumerate(lines):
        yy = y + i*gap
        shape(slide, .75, yy+.13, .095, .095, PRIMARY)
        text(slide, line, 1.04, yy, 11.3, .58, size=size)

def table(slide, rows, widths, y=2.65, height=3.4, font_size=22):
    x=.70
    w=sum(widths)
    t=slide.shapes.add_table(len(rows), len(widths), Inches(x), Inches(y), Inches(w), Inches(height)).table
    for i, width in enumerate(widths):
        t.columns[i].width=Inches(width)
    for ri, row in enumerate(rows):
        for ci, value in enumerate(row):
            c=t.cell(ri,ci)
            c.text=value
            c.margin_left=c.margin_right=Inches(.20)
            c.margin_top=c.margin_bottom=Inches(.10)
            c.vertical_anchor=MSO_ANCHOR.MIDDLE
            c.fill.solid()
            c.fill.fore_color.rgb=rgb(PRIMARY if ri == 0 else (MUTED if ri%2 else WHITE))
            for p in c.text_frame.paragraphs:
                p.space_before=p.space_after=Pt(0)
                p.line_spacing=1.08
                for r in p.runs:
                    r.font.name='Arial'
                    r.font.size=Pt(font_size)
                    r.font.bold=ri==0
                    r.font.color.rgb=rgb(WHITE if ri==0 else INK)
    return t

# One claim per slide. The nine main slides preserve the requested section labels.
s=make_slide('1. Project Overview',
    'The CTC Generator demo provides a foundation for production.',
    'Client proposal; plan for approximately ten minutes across the nine main slides. '
    'The source is the supplied five-page URD and the existing demo documentation. '
    'The objective is consistent CTC breakups and offer letters using the existing foundation. '
    'Client, presenter and date remain placeholders.', layout='title')
text(s, 'Project proposal', .70, 2.70, 11.93, .45, size=22, color=PRIMARY, bold=True)
text(s, 'Consistent CTC breakups and offer letters through one controlled HR workflow.', .70, 3.37, 10.8, 1.08, size=28)
shape(s, .70, 5.13, 11.93, .80, MUTED)
text(s, '[Client name]  |  [Presenter]  |  [Date]', .96, 5.37, 11.30, .36, size=20)

s=make_slide('2. Existing Demo Application',
    'The demo supports the core offer workflow.',
    'The demo already contains sequential calculation, all five groups and 20 components, '
    'candidate inputs, saved revisions, versioned configuration, access controls, audit history and PDF generation. '
    'Its main dataset is the URD illustration. Final-document paths were exercised with synthetic test data; '
    'that does not establish approved client policy or production acceptance. Reference: README and verification record.',
    source='Demo documentation')
table(s, [['Available now','Production need'],
          ['Calculations and PDFs','Approved rules/template'],
          ['Versioned masters','Confirmed access'],
          ['Local deployment','Production environment']], [5.96,5.97], height=2.85)
text(s, 'Drafts use illustrative policies.', .90, 5.81, 11.40, .40, size=22, color=PRIMARY)

s=make_slide('3. Proposed Development Scope',
    'Production work completes and validates the existing application.',
    'Complete the agreed URD scope using approved client data. Keep grade matrices, slabs, '
    'percentages and ceilings configurable by authorised users. No payroll, payslips, '
    'HRMS/ATS integration, month-wise tax computation, investment declarations or reverse-CTC solving '
    'is included. The written proposal contains the complete scope boundary.', source='URD; demo')
bullets(s, ['Configure approved rules and client master data.',
            'Apply the official offer template.',
            'Validate calculations, permissions and outputs.',
            'Prepare deployment and handover.'], y=2.58, gap=.73)
text(s, 'URD scope only.', 1.04, 5.75, 11.20, .40, size=21, color=PRIMARY, bold=True)

s=make_slide('4. Key Features and Functionalities',
    'All five compensation groups remain intact.',
    'Preserve all 20 components and the gross-driven calculation sequence. Basic and Conveyance '
    'follow grades; Professional Allowance is qualification-gated; policy payments and Variable Pay '
    'follow approved rules. Annual Special Allowance classification remains a client decision. '
    'Matching CTC and offer figures retain their input and rule-version history. '
    'The next three appendix slides contain calculation details; the Word/PDF proposal lists every component.')
text(s, '20 components', .78, 2.67, 3.70, .65, size=30, color=PRIMARY, bold=True)
for i, group in enumerate([row[0] for row in SOURCE['component_rows']]):
    yy=2.62+i*.57
    shape(s, 4.83, yy, 7.80, .49, MUTED)
    text(s, group, 5.06, yy+.07, 7.35, .34, size=22)
text(s, 'Balanced calculations\nMatching PDFs\nTraceable revisions', .78, 3.58, 3.78, 1.80, size=23)

s=make_slide('5. Project Deliverables',
    'Delivery includes the application, evidence and handover.',
    'Deliver the production application and source code, approved master configuration, '
    'CTC Breakup Sheet and Offer Letter PDF templates. Provide calculation and workflow '
    'test evidence for client UAT sign-off. Complete deployment setup, handover notes '
    'and the short HR/C&B walkthrough required for the single-screen workflow.', source='Proposal scope')
bullets(s, ['Production application and source code',
            'Configured masters and document templates',
            'Test evidence and client UAT sign-off',
            'Deployment notes and HR/C&B walkthrough'], gap=.82)

s=make_slide('6. Estimated Development Timeline',
    'Dates remain open until client inputs are confirmed.',
    'No development estimate was supplied. These phases show the proposed delivery sequence only, '
    'not fixed durations or dates. Agree estimates after the missing policies, offer template '
    'and access decisions are confirmed. The written proposal carries total duration, start-date '
    'and go-live placeholders.', source='Proposal')
table(s, [['Phase','Duration'], ['Confirmation','[To confirm]'],
          ['Development','[To confirm]'], ['Testing / UAT','[To confirm]'],
          ['Handover','[To confirm]']], [7.30,4.63], y=2.56, height=2.98)
text(s, 'Total: [To confirm]', .86, 5.79, 5.10, .39, size=20, color=PRIMARY)
text(s, 'Start / go-live: [To agree]', 6.15, 5.79, 6.30, .39, size=20, color=PRIMARY)

s=make_slide('7. Project Cost',
    'Commercial terms remain subject to agreement.',
    'No commercial estimate was supplied. Total project cost, currency, applicable taxes or '
    'charges and payment terms are deliberately left open. The quotation will be finalised '
    'after scope and client inputs are confirmed. These placeholders are not an offer or estimate.', source='Proposal')
table(s, [['Item','Placeholder'], ['Total','[Currency] [Amount]'],
          ['Taxes / charges','[To confirm]'], ['Payment terms','[To agree]']], [5.70,6.23], height=2.85)
text(s, 'No estimate supplied.', .91, 5.81, 11.30, .40, size=22, color=PRIMARY)

s=make_slide('8. Assumptions and Client Requirements',
    'Client approvals determine production readiness.',
    'Obtain Basic ranges, Conveyance caps, Professional Allowance slabs, Variable Pay bands '
    'and schedules; Superannuation, ESI and policy-payment rules; Annual Special Allowance '
    'slabs/classification. Confirm Basic selection, rounding, annualisation and Finance/Tax '
    'acceptance of uncapped HRA. Supply the official offer template, signatory, HR/C&B '
    'access decisions, reviewers, environment access and approved manual calculation examples. '
    'Missing policies must not become zero or be inferred from test data.', source='URD; demo')
bullets(s, ['Grade matrix, benefits and payment schedules',
            'Basic, rounding, annualisation and HRA decisions',
            'Offer wording and authorised signatory',
            'Access rights, hosting and UAT reviewers'], gap=.82)

s=make_slide('9. Conclusion',
    'Confirm the inputs, then agree delivery and commercial terms.',
    'The demo provides a practical foundation. The next decision is to approve pending '
    'business rules, the offer template and editing responsibilities. Then agree scope, '
    'development dates and commercial terms before production delivery. '
    'The following appendix slides support discussion of the core calculation rules.', source='Proposal', layout='statement')
shape(s, .70, 2.73, 11.93, 1.80, MUTED)
text(s, 'Next step: approve the pending rules, template and access decisions.', 1.02, 3.10, 11.25, 1.10, size=29, color=PRIMARY, bold=True)
text(s, 'Timeline: [To agree]', 1.02, 5.13, 5.45, .55, size=24)
text(s, 'Cost: [To agree]', 7.00, 5.13, 5.30, .55, size=24)

s=make_slide('Appendix: Balancing rules',
    'Gross determines balancing allowances.',
    'URD section 4.2: enter monthly Gross, set equal targets, establish Basic and '
    'Professional Allowance, then calculate Conveyance, Employer PF, NPS and Superannuation. '
    'Balance Special Allowance, then HRA. PF here means Employer PF. '
    'No circular formulas; unequal totals or negative residuals block output. '
    'HRA has no cap relative to Basic.', source='URD §4.2')
shape(s, .70, 2.62, 11.93, .74, MUTED)
text(s, 'Inclusion = Exclusion = Gross/2', .95, 2.80, 11.40, .42, size=25, color=PRIMARY, bold=True)
text(s, 'Special Allowance = Gross/2 − Basic − Professional Allowance', .95, 3.73, 11.40, .92, size=24)
text(s, 'HRA (uncapped) = Gross/2 − Conveyance − PF − NPS − Superannuation', .95, 4.76, 11.40, .95, size=24)
text(s, 'Negative balances block output.', .95, 5.86, 11.40, .37, size=20, color=PRIMARY)

s=make_slide('Appendix: Employer PF',
    'Employer PF follows three URD tiers.',
    'Apply these rules in order. When Basic exceeds INR 25,000, Employer PF is '
    '12% of Basic. Otherwise test 50% of the Inclusion target against INR 25,000. '
    'Use 12% of that basis when within the cap, otherwise INR 3,000. '
    'These are the client URD rules, not a separate interpretation of statutory law.', source='URD §4.4')
t = table(s, [['Condition','Employer PF'], ['Basic > INR 25,000','12% × Basic'],
          ['Otherwise: 50% × Inclusion ≤ INR 25,000','12% × (50% × Inclusion)'],
          ['Otherwise','INR 3,000']], [7.26,4.67], y=2.62, height=3.02, font_size=21)
for row, height in zip(t.rows, [.54, .70, 1.08, .70]):
    row.height = Inches(height)
text(s, 'Apply sequentially.', .94, 5.91, 11.40, .39, size=20, color=PRIMARY)

s=make_slide('Appendix: Retirement and statutory rules',
    'NPS and statutory rates follow the URD.',
    'NPS is optional; when opted in it uses 14% of Basic for the New Regime '
    'or 10% for the Old Regime. Opting out makes NPS zero, with HRA balancing '
    'the Exclusion group. ESI uses the supplied URD rate and confirmed wage-limit '
    'eligibility/basis. Gratuity is 4.81% of Inclusion. Keep rates and ceilings '
    'configurable and obtain the outstanding client policy details.', source='URD §§4.3,8')
t = table(s, [['Component','Rule'],
          ['NPS','Opt-in: New 14%; Old 10% of Basic\nOpt-out: zero.'],
          ['ESI','3.25%; wage-limit eligibility'],
          ['Gratuity','4.81% of Inclusion']], [3.30,8.63], y=2.62, height=3.02, font_size=21)
for row, height in zip(t.rows, [.64, 1.10, .64, .64]):
    row.height = Inches(height)
text(s, 'Rates/ceilings configurable', .94, 5.91, 11.40, .39, size=20, color=PRIMARY)

prs.save(OUT)

# Structural inspection: all native text and tables, bounded placement, concise text.
def visible_text(slide):
    texts=[]
    for sh in slide.shapes:
        if sh.has_text_frame:
            texts.append(sh.text)
        if sh.has_table:
            texts.extend(cell.text for row in sh.table.rows for cell in row.cells)
    return '\n'.join(texts)

qa=[]
errors=[]
for idx,(sl,meta) in enumerate(zip(prs.slides,slides_meta),1):
    vis=visible_text(sl)
    count=len(vis.split())
    bounds=[]
    for sh in sl.shapes:
        if sh.left<Inches(.59) or sh.top<Inches(.59) or sh.left+sh.width>prs.slide_width-Inches(.59) or sh.top+sh.height>prs.slide_height-Inches(.59):
            bounds.append(sh.name)
    fonts=[]
    for sh in sl.shapes:
        frames=[]
        if sh.has_text_frame: frames.append(sh.text_frame)
        if sh.has_table: frames.extend(c.text_frame for r in sh.table.rows for c in r.cells)
        for frame in frames:
            for p in frame.paragraphs:
                for run in p.runs:
                    if run.text:
                        fonts.append(run.font.size.pt if run.font.size else None)
    row={'slide':idx,**meta,'visible_words':count,'notes_words':len(meta['notes'].split()),
         'title_words':len(meta['title'].split()),'bounds_violations':bounds,
         'minimum_text_pt':min(f for f in fonts if f is not None),'text':vis}
    qa.append(row)
    if count>40: errors.append(f'Slide {idx}: {count} visible words')
    if row['notes_words']>80: errors.append(f'Slide {idx}: notes too long')
    if row['title_words']>12: errors.append(f'Slide {idx}: title too long')
    if bounds: errors.append(f'Slide {idx}: out-of-bounds shapes {bounds}')
(ROOT/'build/presentation-qa.json').write_text(json.dumps({'file':str(OUT),'slide_count':len(prs.slides),'slides':qa,'errors':errors},indent=2),encoding='utf-8')
if errors:
    raise SystemExit('\n'.join(errors))
print(json.dumps({'file':str(OUT),'slide_count':len(prs.slides),'visible_words':[q['visible_words'] for q in qa],'notes_words':[q['notes_words'] for q in qa],'errors':errors}))
