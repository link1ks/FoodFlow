"""Offline, exact-ID nutrition import. Outputs are review artifacts, never migrations."""
import argparse
import csv
import hashlib
import html
import json
import re
from pathlib import Path

HERE = Path(__file__).resolve().parent
USDA_KEYS = {"1008": "energy_kcal", "1003": "protein_g", "1004": "fat_g",
             "1005": "carbs_g", "1079": "fiber_g", "1093": "sodium_mg"}


def number(value):
    value = value.strip()
    if not value:
        return None
    if not re.fullmatch(r"\d+(\.\d+)?", value, flags=re.ASCII):
        raise ValueError("invalid nonnegative decimal measurement")
    return value


def put(values, key, raw):
    value = number(raw)
    if value is not None:
        if key in values:
            raise ValueError("duplicate nutrient measurement")
        values[key] = value


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def usda(root, ids):
    files = sorted(root.rglob("food.csv"))
    if len(files) != 1:
        raise ValueError("expected exactly one USDA food.csv")
    food_file = files[0]
    nutrient_file = food_file.with_name("food_nutrient.csv")
    foods, values = {}, {}
    wanted = {str(i) for i in ids}
    with food_file.open(encoding="utf-8-sig", newline="") as f:
        for row in csv.DictReader(f):
            ident = row["fdc_id"]
            if ident in wanted:
                if ident in foods:
                    raise ValueError("duplicate food ID")
                foods[ident] = row["description"]
    with nutrient_file.open(encoding="utf-8-sig", newline="") as f:
        for row in csv.DictReader(f):
            ident, key = row["fdc_id"], USDA_KEYS.get(row["nutrient_id"])
            if ident in wanted and key:
                put(values.setdefault(ident, {}), key, row["amount"])
    if set(foods) != wanted or set(values) != wanted:
        raise ValueError("missing mapped USDA food")
    return foods, values, food_file, nutrient_file


def build(root, base, extension):
    profiles, records, hashes = {}, {}, {}

    def sha(path):
        relative = path.relative_to(root).as_posix()
        if relative not in hashes:
            hashes[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
        return hashes[relative]

    def add(name, ident, food, source, url, nutrients, note, raw):
        if not food.strip() or not {"energy_kcal", "protein_g"} <= nutrients.keys() or len(nutrients) < 3:
            raise ValueError("mapped profile lacks required reference measurements")
        profile = dict(status="reference", basis="每100克可食部", reference_food=food,
                       source=source, source_id=str(ident), source_url=url,
                       note=note, nutrients=nutrients.copy())
        profiles[name] = profile
        records[name] = dict(profile=profile, source_file_sha256=sha(raw))

    ids = list(base["mapping"].values()) + list(extension["usda"].values())
    foods, values, food_file, nutrient_file = usda(root / "usda", ids)
    sha(food_file)
    sha(nutrient_file)
    for mapping, notes, default in (
        (base["mapping"], base["notes"], "参考食品的品种、生熟与加工状态见原始名称；用于了解营养组成，不代表当前批次检测值。"),
        (extension["usda"], extension["notes"], "参考食品的生熟与加工状态见原始名称；不是当前批次检测值。"),
    ):
        for name, ident in mapping.items():
            add(name, ident, foods[str(ident)], "USDA FoodData Central · SR Legacy 2018",
                f"https://fdc.nal.usda.gov/food-details/{ident}/nutrients", values[str(ident)],
                notes.get(name, default), nutrient_file)
    # Missing original entries are placeholders; reviewed extensions may replace them.
    for name, note in base["missing"].items():
        if name not in profiles:
            profiles[name] = dict(status="unavailable", basis="每100克可食部", note=note, nutrients={})

    tfda_file = root / "tfda/20_2.csv"
    entries = {}
    keys = {"熱量": "energy_kcal", "粗蛋白": "protein_g", "粗脂肪": "fat_g",
            "總碳水化合物": "carbs_g", "膳食纖維": "fiber_g", "鈉": "sodium_mg"}
    with tfda_file.open(encoding="utf-8-sig", newline="") as f:
        for row in csv.DictReader(f):
            if row["整合編號"] not in extension["tfda"].values():
                continue
            entry = entries.setdefault(row["整合編號"], {"row": row, "nutrients": {}})
            key = keys.get(row["分析項"])
            if key:
                unit = "kcal" if key == "energy_kcal" else "mg" if key == "sodium_mg" else "g"
                if row["含量單位"] != unit:
                    raise ValueError("unexpected TFDA nutrient unit")
                put(entry["nutrients"], key, row["每100克含量"])
    for name, ident in extension["tfda"].items():
        entry = entries[ident]
        row = entry["row"]
        add(name, ident, row["樣品名稱"], "食品药物管理署 · 食品营养成分资料集",
            "https://data.gov.tw/dataset/8543", entry["nutrients"],
            extension["notes"].get(name, "") + row["內容物描述"] + "；样品参考值，非当前批次检测值。", tfda_file)

    keys = {"能量*": "energy_kcal", "蛋白質": "protein_g", "脂肪": "fat_g",
            "碳水化合物*": "carbs_g", "膳食纖維*": "fiber_g", "鈉": "sodium_mg"}
    for name, ident in extension["cfs"].items():
        file = root / f"cfs-{ident}.html"
        raw = file.read_text(encoding="utf-8")
        rows = [[html.unescape(re.sub("<[^>]+>", "", c)).strip()
                 for c in re.findall(r"<t[dh][^>]*>(.*?)</t[dh]>", r, re.S | re.I)]
                for r in re.findall(r"<tr[^>]*>(.*?)</tr>", raw, re.S | re.I)]
        portions = [re.sub(r"\s+", "", r[-1]) for r in rows if r and r[0].strip() == "分量:"]
        if portions != ["100克可食用部份"]:
            raise ValueError("CFS report must describe exactly 100 g edible portion")
        source = next(r[-1] for r in rows if r and r[0] == "資料來源:")
        food = next(r[-1] for r in rows if r and r[0] == "食物名稱:")
        nutrients = {}
        for row in rows:
            if len(row) != 3 or row[0] not in keys or row[2] in ("NA", "ND", "Trace", ""):
                continue
            key = keys[row[0]]
            unit = "kcal" if key == "energy_kcal" else "mg" if key == "sodium_mg" else "g"
            if {"千卡": "kcal", "克": "g", "毫克": "mg"}.get(row[1], row[1]) != unit:
                raise ValueError("unexpected CFS nutrient unit")
            if key == "fiber_g" and "CFC2002Ed2" in source:
                key = "insoluble_fiber_g"
            put(nutrients, key, row[2])
        note = extension["notes"].get(name, "") + "每100克可食部参考；生熟及加工状态见原始名称。"
        note += ("纤维项为不溶性纤维，不等同于总膳食纤维。" if "CFC2002Ed2" in source
                 else "碳水为可利用碳水化合物。")
        add(name, ident, food, "香港食物安全中心 · " + source,
            "https://www.cfs.gov.hk/tc_chi/nutrient/searchmenu.php", nutrients, note, file)
    return profiles, records, hashes


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sources", type=Path, required=True, help="local usda/, tfda/, cfs-*.html directory")
    parser.add_argument("--out", type=Path, required=True, help="new review directory under .cache/")
    args = parser.parse_args()
    root, target = args.sources.resolve(), args.out.resolve()
    cache = HERE.parent / ".cache"
    if not target.is_relative_to(cache) or target == cache or target.exists():
        parser.error("--out must be a new directory under this repository's .cache/")
    profiles, records, hashes = build(root, read_json(HERE / "nutrition-base-mapping.json"),
                                      read_json(HERE / "nutrition-reference-mapping.json"))
    current = read_json(HERE.parent / "internal/nutrition/profiles.json")
    changed = sorted(k for k in current.keys() | profiles.keys() if current.get(k) != profiles.get(k))
    manifest = dict(version=1, profiles=len(profiles),
                    reference=sum(p["status"] == "reference" for p in profiles.values()),
                    changed_profiles=changed, source_files=hashes,
                    mapping_sha256={p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in
                                    (HERE / "nutrition-base-mapping.json", HERE / "nutrition-reference-mapping.json")})
    target.mkdir(parents=True)
    for name, data in (("profiles.json", profiles), ("reference-records.json", records), ("manifest.json", manifest)):
        (target / name).write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"profiles": len(profiles), "changed_profiles": len(changed), "reference": manifest["reference"]}))


if __name__ == "__main__":
    main()
