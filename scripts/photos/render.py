"""Reproduce reviewed real-photo edits from immutable Git originals and masks.

Requires Pillow 12.0.0. Usage: python scripts/photos/render.py --out .cache/photo-check
No network, generative model, database access or source-asset overwrites.
"""
import argparse
import hashlib
import json
import subprocess
from io import BytesIO
from pathlib import Path

from PIL import Image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    output = args.out.resolve()
    if output == root / "web/public/ingredient-photos":
        raise ValueError("Use a scratch output directory; review before publishing")
    output.mkdir(parents=True, exist_ok=True)
    photos = json.loads((root / "web/public/ingredient-photos/manifest.json").read_text("utf-8"))
    for photo in photos:
        original = photo["original_asset"]
        raw = subprocess.check_output(
            ["git", "show", original["revision"] + ":web/public" + original["src"]], cwd=root
        )
        if hashlib.sha256(raw).hexdigest() != original["sha256"]:
            raise ValueError("Original photo checksum mismatch")
        mask_bytes = (root / photo["mask"]).read_bytes()
        if hashlib.sha256(mask_bytes).hexdigest() != photo["mask_sha256"]:
            raise ValueError("Reviewed mask checksum mismatch")
        image = Image.open(BytesIO(raw)).convert("RGBA")
        mask = Image.open(BytesIO(mask_bytes)).convert("L")
        if image.size != mask.size:
            raise ValueError("Mask must match original photo dimensions")
        box = mask.point(lambda value: 255 if value > 100 else 0).getbbox()
        if box is None:
            raise ValueError("Empty foreground")
        image.putalpha(mask)
        image = image.crop(box)
        scale = min(422 / image.width, 302 / image.height)
        image = image.resize((round(image.width * scale), round(image.height * scale)), Image.Resampling.LANCZOS)
        canvas = Image.new("RGB", (480, 360), (250, 248, 243))
        canvas.paste(image, ((480 - image.width) // 2, (360 - image.height) // 2), image)
        target = output / Path(photo["src"]).name
        canvas.save(target, quality=92, method=6)
        if hashlib.sha256(target.read_bytes()).hexdigest() != photo["sha256"]:
            raise ValueError("Rendered checksum differs; check Pillow/WebP versions")
    print(f"Verified {len(photos)} reproduced photographs")


if __name__ == "__main__":
    main()
