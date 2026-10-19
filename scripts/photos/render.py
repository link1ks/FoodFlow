"""Reproduce studio thumbnails from retained imagegen originals (Pillow 12).
Usage: python scripts/photos/render.py --sources .cache/studio-sources --out .cache/studio-check
This crops/resizes generated sheets only; never calls a model or edits user photos.
"""
import argparse, hashlib, json
from io import BytesIO
from pathlib import Path
from PIL import Image

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sources',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    root=Path(__file__).resolve().parents[2]
    if args.out.resolve()==root/'web/public/ingredient-photos':
        raise ValueError('Render to a scratch directory for review')
    args.out.mkdir(parents=True,exist_ok=True)
    entries=json.loads((root/'web/public/ingredient-photos/manifest.json').read_text('utf-8-sig'))
    for entry in entries:
        raw=(args.sources/entry['source_asset']).read_bytes()
        if hashlib.sha256(raw).hexdigest()!=entry['source_sha256']:
            raise ValueError('Source hash mismatch: '+entry['name'])
        img=Image.open(BytesIO(raw)).convert('RGB').crop(entry['crop']).resize((464,348),Image.Resampling.LANCZOS)
        canvas=Image.new('RGB',(480,360),'#faf8f3');canvas.paste(img,(8,6))
        output=BytesIO();canvas.save(output,format='WEBP',quality=88,method=6)
        data=output.getvalue()
        if hashlib.sha256(data).hexdigest()!=entry['sha256']:
            raise ValueError('Output hash mismatch: '+entry['name'])
        (args.out/Path(entry['src']).name).write_bytes(data)
    print(f'Verified {len(entries)} studio reference images')
if __name__=='__main__':main()
