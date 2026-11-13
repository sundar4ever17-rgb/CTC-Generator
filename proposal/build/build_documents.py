import json
from pathlib import Path

from docx import Document
from docx.shared import Inches, Pt, RGBColor
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT, WD_CELL_VERTICAL_ALIGNMENT

ROOT = Path('/proposal')
data = json.loads((ROOT / 'build/proposal-content.json').read_text(encoding='utf-8'))
INK = '172B4D'
TEAL = '005A70'
PALE = 'EDF2F6'
GOLD = 'AE6A20'
WHITE = 'FFFFFF'

doc = Document()
sec = doc.sections[0]
sec.page_width = Inches(8.2677)
sec.page_height = Inches(11.6929)
sec.top_margin = Inches(.57)
sec.bottom_margin = Inches(.58)
sec.left_margin = Inches(.66)
sec.right_margin = Inches(.66)
sec.header_distance = Inches(.23)
sec.footer_distance = Inches(.25)
WIDTH = 6.9477

styles = doc.styles
for name in ['Normal', 'Body Text', 'Title', 'Subtitle', 'Heading 1', 'Heading 2', 'List Bullet']:
    styles[name].font.name = 'Arial'
    styles[name].font.color.rgb = RGBColor.from_string(INK)
normal = styles['Normal']
normal.font.size = Pt(10.5)
normal.paragraph_format.space_after = Pt(5)
normal.paragraph_format.line_spacing = 1.07
normal.paragraph_format.widow_control = True
styles['Title'].font.size = Pt(27)
styles['Title'].font.bold = True
styles['Title'].paragraph_format.space_after = Pt(3)
styles['Subtitle'].font.size = Pt(14)
styles['Subtitle'].font.italic = False
styles['Subtitle'].font.color.rgb = RGBColor.from_string(TEAL)
styles['Subtitle'].paragraph_format.space_after = Pt(8)
for name in ['Heading 1', 'Heading 2']:
    styles[name].font.size = Pt(12)
    styles[name].font.bold = True
    styles[name].font.color.rgb = RGBColor.from_string(TEAL)
    styles[name].paragraph_format.space_before = Pt(9)
    styles[name].paragraph_format.space_after = Pt(4)
    styles[name].paragraph_format.keep_with_next = True
styles['List Bullet'].font.size = Pt(10.2)
styles['List Bullet'].paragraph_format.space_after = Pt(4)
styles['List Bullet'].paragraph_format.line_spacing = 1.03
styles['List Bullet'].paragraph_format.left_indent = Inches(.14)
styles['List Bullet'].paragraph_format.first_line_indent = Inches(-.14)

def shade(cell, fill):
    elem = OxmlElement('w:shd')
    elem.set(qn('w:fill'), fill)
    cell._tc.get_or_add_tcPr().append(elem)

def cell_margins(cell, top=70, start=90, bottom=70, end=90):
    tcpr = cell._tc.get_or_add_tcPr()
    margin = OxmlElement('w:tcMar')
    for side, val in [('top', top), ('start', start), ('bottom', bottom), ('end', end)]:
        item = OxmlElement('w:' + side)
        item.set(qn('w:w'), str(val))
        item.set(qn('w:type'), 'dxa')
        margin.append(item)
    tcpr.append(margin)

def paragraph(text, size=None, bold=False, color=None, after=None):
    p = doc.add_paragraph()
    r = p.add_run(text)
    r.bold = bold
    if size:
        r.font.size = Pt(size)
    if color:
        r.font.color.rgb = RGBColor.from_string(color)
    if after is not None:
        p.paragraph_format.space_after = Pt(after)
    return p

def heading(text):
    doc.add_paragraph(text, style='Heading 1')

def table(headers, rows, widths, size=9.4, padding=65):
    t = doc.add_table(rows=1, cols=len(headers))
    t.alignment = WD_TABLE_ALIGNMENT.CENTER
    t.autofit = False
    for col, width in zip(t.columns, widths):
        col.width = Inches(width)
    pr = t._tbl.tblPr
    borders = OxmlElement('w:tblBorders')
    for edge in ['top', 'left', 'bottom', 'right', 'insideH', 'insideV']:
        e = OxmlElement('w:' + edge)
        e.set(qn('w:val'), 'single')
        e.set(qn('w:sz'), '4')
        e.set(qn('w:color'), 'FFFFFF' if edge == 'insideV' else 'DCE4EA')
        borders.append(e)
    pr.append(borders)
    for idx, row_texts in enumerate([headers] + rows):
        row = t.rows[0] if idx == 0 else t.add_row()
        rowpr = row._tr.get_or_add_trPr()
        no_split = OxmlElement('w:cantSplit')
        rowpr.append(no_split)
        if idx == 0:
            repeat = OxmlElement('w:tblHeader')
            rowpr.append(repeat)
        for j, (cell, text) in enumerate(zip(row.cells, row_texts)):
            cell.width = Inches(widths[j])
            cell.vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER
            cell_margins(cell, top=padding, bottom=padding)
            shade(cell, TEAL if idx == 0 else (PALE if idx % 2 else WHITE))
            p = cell.paragraphs[0]
            p.paragraph_format.space_before = Pt(0)
            p.paragraph_format.space_after = Pt(0)
            p.paragraph_format.line_spacing = 1.02
            run = p.add_run(text)
            run.font.name = 'Arial'
            run.font.size = Pt(size)
            run.bold = idx == 0 or j == 0
            run.font.color.rgb = RGBColor.from_string(WHITE if idx == 0 else INK)
    return t

header = sec.header.paragraphs[0]
header.alignment = WD_ALIGN_PARAGRAPH.RIGHT
hr = header.add_run('CTC GENERATOR  /  PRODUCTION PROPOSAL')
hr.font.name = 'Arial'
hr.font.size = Pt(8)
hr.font.color.rgb = RGBColor.from_string(TEAL)

footer = sec.footer.paragraphs[0]
footer.style = styles['Normal']
footer.paragraph_format.tab_stops.add_tab_stop(Inches(WIDTH), WD_ALIGN_PARAGRAPH.RIGHT)
r = footer.add_run('CTC Generator | Project Proposal\t')
r.font.size = Pt(8)
r.font.color.rgb = RGBColor.from_string(TEAL)
for instr in ['PAGE']:
    f = OxmlElement('w:fldSimple')
    f.set(qn('w:instr'), instr)
    footer._p.append(f)

doc.core_properties.title = 'CTC Generator - Project Proposal'
doc.core_properties.subject = 'Production development proposal based on the client URD and existing demo'
doc.core_properties.author = ''
doc.core_properties.keywords = 'CTC Generator, proposal, production, URD'
doc.core_properties.comments = ''

# Page 1: what exists and what is proposed.
doc.add_paragraph(data['title'], style='Title')
doc.add_paragraph(data['subtitle'], style='Subtitle')
p = paragraph(data['metadata'], size=8.6, after=6)
ppr = p._p.get_or_add_pPr()
pb = OxmlElement('w:pBdr')
bottom = OxmlElement('w:bottom')
for key, val in [('val', 'single'), ('sz', '10'), ('color', GOLD), ('space', '8')]:
    bottom.set(qn('w:' + key), val)
pb.append(bottom)
ppr.append(pb)
heading('1. Project Overview')
paragraph(data['overview'])
heading('2. Existing Demo Application')
paragraph(data['demo_intro'])
table(data['demo_headers'], data['demo_rows'], [1.18, 2.80, 2.9677], size=10, padding=85)
paragraph(data['demo_note'], size=9.2, after=3)
heading('3. Proposed Development Scope')
paragraph(data['scope'])
paragraph(data['scope_exclusions'], size=10)

# Page 2: requirements and delivery.
doc.add_page_break()
heading('4. Key Features and Functionalities')
paragraph(data['feature_intro'])
table(data['component_headers'], data['component_rows'], [1.44, 5.5077], size=10.5, padding=80)
paragraph('Business rules follow the supplied URD; final values and conventions require client approval.', size=9.4, after=4)
table(data['rules_headers'], data['rules_rows'], [1.44, 5.5077], size=10.5, padding=80)
heading('5. Project Deliverables')
for item in data['deliverables']:
    p = doc.add_paragraph(item, style='List Bullet')

# Page 3: schedule, commercial placeholders and client inputs.
doc.add_page_break()
heading('6. Estimated Development Timeline')
paragraph(data['timeline_intro'])
table(data['timeline_headers'], data['timeline_rows'], [1.65, 3.6477, 1.65], size=10, padding=75)
paragraph(data['timeline_total'], size=9.1, bold=True, after=5)
heading('7. Project Cost')
paragraph(data['cost_intro'])
table(data['cost_headers'], data['cost_rows'], [2.1, 4.8477], size=10, padding=70)
heading('8. Assumptions and Client Requirements')
for item in data['assumptions']:
    doc.add_paragraph(item, style='List Bullet')
heading('9. Conclusion')
paragraph(data['conclusion'])
paragraph(data['basis'], size=8, color=TEAL, after=0)

dest = ROOT / 'ctc-generator-project-proposal.docx'
doc.save(dest)
print(json.dumps({'created': str(dest), 'paragraphs': len(doc.paragraphs), 'tables': len(doc.tables)}))
