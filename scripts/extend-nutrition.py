"""Import reviewed reference mappings from official downloads; no fuzzy matching.

python scripts/extend-nutrition.py .cache/nutrition-data
Input: tfda/20_2.csv, usda/**/food{,_nutrient}.csv, cfs-<food_id>.html.
Fetch CFS reports with POST food_id/fg_id/fsg_id/inShortMode=0.
Missing nutrient measurements remain absent, never zero-filled.
"""
import csv
import hashlib
import html
import json
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
mapping = json.loads(Path('scripts/nutrition-reference-mapping.json').read_text(encoding='utf8'))
profiles_path = Path('internal/nutrition/profiles.json')
profiles = json.loads(profiles_path.read_text(encoding='utf8'))
records = {}

def number(value):
    value = value.strip()
    if not value:
        return None
    if not re.fullmatch(r'\d+(\.\d+)?', value):
        raise ValueError('Unexpected nutrient value: ' + value)
    return value

def add(name, id, food, source, url, nutrients, note, raw_file):
    assert 'energy_kcal' in nutrients and 'protein_g' in nutrients and len(nutrients) >= 3, name
    profile = dict(status='reference', basis='每100克可食部', reference_food=food,
                   source=source, source_id=str(id), source_url=url, note=note,
                   nutrients=nutrients)
    profiles[name] = profile
    records[name] = dict(profile=profile, source_file_sha256=hashlib.sha256(raw_file.read_bytes()).hexdigest())

tfda_file = root / 'tfda/20_2.csv'
tfda = {}
keys = {'熱量':'energy_kcal', '粗蛋白':'protein_g', '粗脂肪':'fat_g',
        '總碳水化合物':'carbs_g', '膳食纖維':'fiber_g', '鈉':'sodium_mg'}
with tfda_file.open(encoding='utf-8-sig', newline='') as f:
    for row in csv.DictReader(f):
        if row['整合編號'] not in mapping['tfda'].values():
            continue
        entry = tfda.setdefault(row['整合編號'], {'row': row, 'nutrients': {}})
        key = keys.get(row['分析項'])
        if key:
            expected = 'kcal' if key == 'energy_kcal' else 'mg' if key == 'sodium_mg' else 'g'
            assert row['含量單位'] == expected
            value = number(row['每100克含量'])
            if value is not None:
                entry['nutrients'][key] = value
for name, id in mapping['tfda'].items():
    entry = tfda[id]
    row = entry['row']
    add(name, id, row['樣品名稱'], '食品药物管理署 · 食品营养成分资料集',
        'https://data.gov.tw/dataset/8543', entry['nutrients'],
        mapping['notes'].get(name, '') + row['內容物描述'] + '；样品参考值，非当前批次检测值。', tfda_file)

food_file = next((root / 'usda').rglob('food.csv'))
nutrient_file = food_file.with_name('food_nutrient.csv')
with food_file.open(encoding='utf8', newline='') as f:
    foods = {r['fdc_id']: r['description'] for r in csv.DictReader(f)}
values = {}
keys = {'1008':'energy_kcal', '1003':'protein_g', '1004':'fat_g',
        '1005':'carbs_g', '1079':'fiber_g', '1093':'sodium_mg'}
with nutrient_file.open(encoding='utf8', newline='') as f:
    for row in csv.DictReader(f):
        if int(row['fdc_id']) not in mapping['usda'].values() or row['nutrient_id'] not in keys:
            continue
        values.setdefault(row['fdc_id'], {})[keys[row['nutrient_id']]] = number(row['amount'])
for name, id in mapping['usda'].items():
    add(name, id, foods[str(id)], 'USDA FoodData Central · SR Legacy 2018',
        f'https://fdc.nal.usda.gov/food-details/{id}/nutrients', values[str(id)],
        mapping['notes'].get(name, '参考食品的生熟与加工状态见原始名称；不是当前批次检测值。'), nutrient_file)

def plain(text):
    return html.unescape(re.sub('<[^>]+>', '', text)).strip()

keys = {'能量*':'energy_kcal', '蛋白質':'protein_g', '脂肪':'fat_g',
        '碳水化合物*':'carbs_g', '膳食纖維*':'fiber_g', '鈉':'sodium_mg'}
for name, id in mapping['cfs'].items():
    file = root / f'cfs-{id}.html'
    text = file.read_text(encoding='utf8')
    rows = [[plain(c) for c in re.findall(r'<t[dh][^>]*>(.*?)</t[dh]>', r, re.S | re.I)]
            for r in re.findall(r'<tr[^>]*>(.*?)</tr>', text, re.S | re.I)]
    source = next(r[-1] for r in rows if r[0] == '資料來源:')
    food = next(r[-1] for r in rows if r[0] == '食物名稱:')
    nutrients = {}
    for r in rows:
        if len(r) != 3 or r[0] not in keys or r[2] in ('NA', 'ND', 'Trace', ''):
            continue
        key = keys[r[0]]
        # Chinese table's fibre is insoluble fibre, not total dietary fibre.
        if key == 'fiber_g' and 'CFC2002Ed2' in source:
            key = 'insoluble_fiber_g'
        nutrients[key] = number(r[2])
    note = mapping['notes'].get(name, '') + '每100克可食部参考；生熟及加工状态见原始名称。'
    if 'CFC2002Ed2' in source:
        note += '纤维项为不溶性纤维，不等同于总膳食纤维。'
    else:
        note += '碳水为可利用碳水化合物。'
    add(name, id, food, '香港食物安全中心 · ' + source,
        'https://www.cfs.gov.hk/tc_chi/nutrient/searchmenu.php', nutrients, note, file)

profiles_path.write_text(json.dumps(profiles, ensure_ascii=False, indent=2) + '\n', encoding='utf8')
Path('internal/nutrition/reference-records.json').write_text(json.dumps(records, ensure_ascii=False, indent=2) + '\n', encoding='utf8')
# Only current reference profiles change; immutable ledger snapshots remain intact.
def quote(s):
    return "'" + s.replace("'", "''") + "'"
sql = ['-- +goose Up', '-- Verified reference extension; historical ledger snapshots are unchanged.']
for name in records:
    sql.append('INSERT INTO ingredient_nutrition_profiles(catalog_id,version,profile) SELECT id,\'official-reference-20261019-v2\',' +
               quote(json.dumps(profiles[name], ensure_ascii=False, separators=(',', ':'))) +
               '::jsonb FROM ingredient_catalog WHERE name=' + quote(name) +
               ' ON CONFLICT(catalog_id) DO UPDATE SET version=EXCLUDED.version,profile=EXCLUDED.profile,updated_at=now();')
sql.extend(['', '-- +goose Down', '-- Reference corrections are intentionally retained on rollback.', 'SELECT 1;'])
Path('sql/schema/038_nutrition_reference_extension.sql').write_text('\n'.join(sql) + '\n', encoding='utf8')
print(len(profiles), 'profiles;', len(records), 'verified additions; missing:', [n for n,p in profiles.items() if p['status'] != 'reference'])
