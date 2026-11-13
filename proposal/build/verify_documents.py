import json
import re
from pathlib import Path
import pymupdf as fitz
from docx import Document
from PIL import Image, ImageOps, ImageDraw

root = Path('/proposal')
docx = Document(root / 'ctc-generator-project-proposal.docx')
pdf = fitz.open(root / 'ctc-generator-project-proposal.pdf')
required = [
    '1. Project Overview', '2. Existing Demo Application',
    '3. Proposed Development Scope', '4. Key Features and Functionalities',
    '5. Project Deliverables', '6. Estimated Development Timeline',
    '7. Project Cost', '8. Assumptions and Client Requirements', '9. Conclusion'
]
docx_text = '\n'.join(p.text for p in docx.paragraphs)
for table in docx.tables:
    for row in table.rows:
        docx_text += '\n' + ' | '.join(c.text for c in row.cells)
pdf_text = '\n'.join(p.get_text() for p in pdf)
normalise = lambda s: re.sub(r'\s+', ' ', re.sub(r'(?<=-)\s+', '', s)).strip()
assert all(t in docx_text for t in required)
assert all(t in normalise(pdf_text) for t in required)
assert '[Currency] [Amount]' in normalise(pdf_text)
assert '[To be confirmed]' in normalise(pdf_text)
assert len(pdf) == 3, f'Expected 3 pages, received {len(pdf)}'

content = json.loads((root / 'build/proposal-content.json').read_text())
snippets = []
for key in ['overview', 'demo_intro', 'demo_note', 'scope', 'scope_exclusions', 'feature_intro',
            'timeline_intro', 'timeline_total', 'cost_intro', 'conclusion', 'basis']:
    snippets.append(content[key])
for key in ['demo_rows', 'component_rows', 'rules_rows', 'timeline_rows', 'cost_rows']:
    snippets.extend(cell for row in content[key] for cell in row)
snippets.extend(content['deliverables'])
snippets.extend(content['assumptions'])
missing = [s for s in snippets if normalise(s) not in normalise(pdf_text)]
assert not missing, f'PDF missing source text: {missing}'

previews = []
for i, page in enumerate(pdf):
    pix = page.get_pixmap(matrix=fitz.Matrix(1.5, 1.5), alpha=False)
    path = root / f'build/proposal-page-{i+1}.png'
    pix.save(path)
    previews.append(path)

thumbs = []
for path in previews:
    thumb = Image.open(path).convert('RGB')
    thumb.thumbnail((595, 842))
    thumbs.append(ImageOps.expand(thumb, border=8, fill='#d6dfe5'))
montage = Image.new('RGB', (sum(t.width for t in thumbs), max(t.height for t in thumbs)), '#d6dfe5')
x = 0
for thumb in thumbs:
    montage.paste(thumb, (x, 0))
    x += thumb.width
montage.save(root / 'build/proposal-contact-sheet.png')
(root / 'build/proposal-extracted.txt').write_text(pdf_text, encoding='utf-8')
result = {
    'pages': len(pdf), 'sections_verified': required, 'content_snippets_verified': len(snippets),
    'word_tables': len(docx.tables), 'pdf_words': len(pdf_text.split()),
    'pages_content': [{'page': i + 1, 'words': len(p.get_text().split()),
                      'sections': [t for t in required if t in normalise(p.get_text())]} for i, p in enumerate(pdf)],
    'placeholders_verified': True
}
(root / 'build/document-validation.json').write_text(json.dumps(result, indent=2), encoding='utf-8')
print(json.dumps(result, indent=2))
