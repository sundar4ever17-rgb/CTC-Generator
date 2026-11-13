"""Check delivered content, arithmetic and rendered text for Revision 2."""
import json, re
from pathlib import Path
import pymupdf
from docx import Document
from pptx import Presentation
from PIL import Image, ImageOps

root = Path('/proposal')
data = json.loads((root / 'build/proposal-content-v2.json').read_text())
e = data['estimate']
norm = lambda s: re.sub(r'\s+', ' ', re.sub(r'(?<=-)\s+', '', s)).strip()
compact = lambda s: re.sub(r'\s+', '', s)
def money(n):
    s = str(int(n))
    return s if len(s) <= 3 else re.sub(r'(\d)(?=(\d{2})+$)', r'\1,', s[:-3]) + ',' + s[-3:]

assert sum(p['days'] for p in e['phases']) == 94
assert sum(p['cost'] for p in e['phases']) == 1128000
assert all(p['cost'] == p['days'] * 12000 for p in e['phases'])
assert e['base_cost'] == 1128000 + 72000 == 1200000
assert e['reserve_cost'] == e['base_cost'] * 0.15 == 180000
assert e['total_budget'] == e['base_cost'] + e['reserve_cost'] == 1380000
assert e['maintenance_monthly'] == 16 * 1500 == 24000
assert e['maintenance_annual'] == 12 * 24000 == 288000

pdf = pymupdf.open(root / 'ctc-generator-project-proposal-v2.pdf')
docx = Document(root / 'ctc-generator-project-proposal-v2.docx')
pdf_text = '\n'.join(p.get_text() for p in pdf)
docx_text = '\n'.join(p.text for p in docx.paragraphs)
docx_text += '\n' + '\n'.join(c.text for t in docx.tables for r in t.rows for c in r.cells)
assert len(pdf) == 5, f'Unexpected page count: {len(pdf)}'
for section in ['1. Project Overview', '2. Existing Demo Application', '3. Proposed Development Scope',
                '4. Key Features and Functionalities', '5. Project Deliverables',
                '6. Estimated Development Timeline and Activities', '7. Project Cost and Post Go-Live Support',
                '8. Assumptions and Client Requirements', '9. Conclusion']:
    assert section in norm(pdf_text), section
for obsolete in ['[Currency]', '[Amount]', '[To be confirmed]', 'No development estimate was supplied', 'No commercial estimate was supplied']:
    assert obsolete not in pdf_text and obsolete not in docx_text, obsolete

snippets = []
for key in ['overview', 'estimate_basis', 'demo_note', 'scope', 'scope_exclusions', 'deployment_requirements', 'conclusion', 'basis']:
    snippets.append(data[key])
for key in ['demo_rows', 'component_rows', 'rules_rows']:
    snippets.extend(c for r in data[key] for c in r)
for key in ['deliverables', 'client_requirements', 'execution_assumptions', 'testing_requirements']:
    snippets.extend(data[key])
for p in e['phases']:
    snippets += [p['name'], p['activity'], p['window'], money(p['cost'])]
snippets += [e[k] for k in ['rate_note', 'schedule_note', 'staffing_note', 'cost_note', 'double_count_note', 'exclusions']]
snippets += list(data['warranty'].values())
snippets += [data['maintenance'][k] for k in ['coverage', 'terms', 'service']]
missing = [s for s in snippets if norm(s) not in norm(pdf_text)]
assert not missing, missing
for amount in [1128000, 72000, 1200000, 180000, 1380000, 24000, 288000]:
    assert money(amount) in pdf_text, amount

previews=[]
for i,p in enumerate(pdf):
    target = root / f'build/revision2-page-{i+1}.png'
    p.get_pixmap(matrix=pymupdf.Matrix(1.4,1.4), alpha=False).save(target)
    im=Image.open(target).convert('RGB'); im.thumbnail((440,623))
    previews.append(ImageOps.expand(im,border=7,fill='#dce4ea'))
sheet=Image.new('RGB',(3*previews[0].width,2*previews[0].height),'#dce4ea')
for i,im in enumerate(previews): sheet.paste(im,((i%3)*im.width,(i//3)*im.height))
sheet.save(root/'build/revision2-document-contact-sheet.png')
(root/'build/revision2-document-text.txt').write_text(pdf_text,encoding='utf-8')
result={'document_pages':len(pdf),'sections_verified':9,'source_blocks_verified':len(snippets),
        'arithmetic_verified':True,'stale_placeholders':False,'document_words':len(pdf_text.split())}

ppt_path=root/'ctc-generator-project-proposal-v2.pptx'
deck_pdf=root/'build/ctc-generator-project-proposal-v2.pdf'
if ppt_path.exists() and deck_pdf.exists():
    prs=Presentation(ppt_path); rendered=pymupdf.open(deck_pdf)
    assert len(prs.slides)==len(rendered)
    slide_texts=[]; issues=[]; thumbs=[]
    for i,(slide,page) in enumerate(zip(prs.slides,rendered)):
        texts=[]
        for sh in slide.shapes:
            if sh.has_text_frame:texts.append(sh.text)
            if sh.has_table:texts.extend(c.text for r in sh.table.rows for c in r.cells)
        for s in texts:
            if compact(s) and compact(s) not in compact(page.get_text()):issues.append({'slide':i+1,'text':s})
        slide_texts.append('\n'.join(texts))
        target=root/f'build/revision2-slide-{i+1:02d}.png'
        page.get_pixmap(matrix=pymupdf.Matrix(1.0,1.0),alpha=False).save(target)
        im=Image.open(target).convert('RGB');im.thumbnail((640,360))
        thumbs.append(ImageOps.expand(im,border=7,fill='#dce4ea'))
    assert not issues, issues
    all_slides='\n'.join(slide_texts)
    for obsolete in ['[Currency]', '[Amount]', '[To confirm]', 'No estimate supplied']:
        assert obsolete not in all_slides, obsolete
    assert 'Brakes India' in all_slides
    native_rows = [[c.text for c in row.cells] for sl in prs.slides
                   for sh in sl.shapes if sh.has_table for row in sh.table.rows]
    for phase in e['phases']:
        matches = [r for r in native_rows if r[0].startswith(phase['id'] + ' ')]
        assert len(matches) == 1, (phase['id'], matches)
        assert matches[0][1:] == [phase['window'], str(phase['days']), money(phase['cost'])], matches[0]
    rows=(len(thumbs)+2)//3
    sheet=Image.new('RGB',(3*thumbs[0].width,rows*thumbs[0].height),'#dce4ea')
    for i,im in enumerate(thumbs):sheet.paste(im,((i%3)*im.width,(i//3)*im.height))
    sheet.save(root/'build/revision2-slides-contact-sheet.png')
    result.update({'slides':len(prs.slides),'render_missing_text':issues,'cross_format_phase_rows_verified':11,
                   'native_tables':sum(1 for sl in prs.slides for sh in sl.shapes if sh.has_table)})
    (root/'build/revision2-slide-text.txt').write_text(all_slides,encoding='utf-8')
(root/'build/revision2-validation.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
print(json.dumps(result,indent=2))
