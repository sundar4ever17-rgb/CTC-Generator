import json
import re
from pathlib import Path
import pymupdf
from pptx import Presentation
from PIL import Image, ImageOps, ImageDraw

root = Path('/proposal')
prs = Presentation(root / 'ctc-generator-project-proposal.pptx')
pdf = pymupdf.open(root / 'build/ctc-generator-project-proposal.pdf')
normalise = lambda s: re.sub(r'\s+', '', s)
assert len(pdf) == len(prs.slides) == 12
issues = []
thumbs = []
for i, (slide, page) in enumerate(zip(prs.slides, pdf)):
    actual = normalise(page.get_text())
    for shape in slide.shapes:
        texts = []
        if shape.has_text_frame:
            texts.append(shape.text)
        if shape.has_table:
            texts.extend(cell.text for row in shape.table.rows for cell in row.cells)
        for text in texts:
            if normalise(text) and normalise(text) not in actual:
                issues.append({'slide': i + 1, 'missing': text})
    path = root / f'build/slide-{i+1:02d}.png'
    page.get_pixmap(matrix=pymupdf.Matrix(1.1, 1.1), alpha=False).save(path)
    im = Image.open(path).convert('RGB')
    im.thumbnail((640, 360))
    thumbs.append(ImageOps.expand(im, border=8, fill='#dce4ea'))
sheet = Image.new('RGB', (3 * thumbs[0].width, 4 * thumbs[0].height), '#dce4ea')
for i, thumb in enumerate(thumbs):
    sheet.paste(thumb, ((i % 3) * thumb.width, (i // 3) * thumb.height))
sheet.save(root / 'build/slides-contact-sheet.png')
result = {'slide_count': len(pdf), 'missing_text': issues,
          'native_tables': sum(1 for s in prs.slides for shape in s.shapes if shape.has_table),
          'speaker_notes': all(s.has_notes_slide for s in prs.slides)}
(root / 'build/presentation-render-validation.json').write_text(json.dumps(result, indent=2), encoding='utf-8')
print(json.dumps(result, indent=2))
assert not issues, 'Some slide text was not found in PDF rendering'
