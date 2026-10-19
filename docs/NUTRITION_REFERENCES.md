# Catalogue nutrition references

The 200-item catalogue has 198 reference profiles. All 109 entries introduced in
migration 037 now have reference values; ten earlier unavailable entries were
also matched. Generic fish and shellfish remain unspecified: select a named
species for meaningful composition. Data are per 100 g edible portion, not per
100 ml, per egg, or the user's inventory quantity.

## Sources and reproduction

- [USDA SR Legacy April 2018](https://fdc.nal.usda.gov/download-datasets/):
  retained original 79 profiles and added 19 explicitly selected FDC IDs.
- [Food and Drug Administration food composition dataset](https://data.gov.tw/dataset/8543):
  85 selected sample IDs, downloaded 2026-10-19 from the official CSV resource
  `https://data.fda.gov.tw/data/opendata/export/20/csv`. Attribution: 衛生福利部食品藥物管理署,
  食品營養成分資料集; Government Open Data License version 1.0 as stated on the
  dataset page. Sample descriptions and raw/processed state are retained.
- [Hong Kong Centre for Food Safety Nutrient Information Inquiry](https://www.cfs.gov.hk/tc_chi/nutrient/searchmenu.php):
  15 named records, retrieved 2026-10-19, with original source attribution
  (Chinese CDC CFC2002Ed2 or CFS Food Research Laboratory). Queries submit
  `food_id`, `fg_id`, `fsg_id`, `inShortMode=0` to `fc-myreport.php` by POST;
  the ordinary report URL alone does not reproduce the query.

Reviewed identifiers and reference-specific qualifications are in
[`nutrition-reference-mapping.json`](../scripts/nutrition-reference-mapping.json).
Selected values, source IDs and downloaded file SHA-256 hashes are retained in
[`reference-records.json`](../internal/nutrition/reference-records.json).
Original downloads are retained locally in ignored `.cache/nutrition-data`.

To reproduce, first run the existing `node scripts/import-nutrition.cjs <USDA CSV directory>`
for the original 91 entries. Download/extract the official sources and save the
selected CFS POST responses as `cfs-<food_id>.html`, then run
`python scripts/extend-nutrition.py <download directory>`.
The latter imports exact IDs, validates units/numeric values and produces the
extended profiles, selected reference records and migration 038. It does not
fuzzy-match food names or fill missing measurements with zero.

## Interpretation limits

Cultivars, meat cuts, edible parts and processing matter. The catalogue's broad
names use a named representative where necessary: e.g. almonds means sweet
almonds, silken tofu uses a named soft MORI-NU reference, cooking wine uses yellow
rice wine (not a claim about salted cooking-wine brands), and silver-ear and
lion's-mane mushrooms use fresh edible portions. Check the retained reference
description and package labels for a particular product. Missing source fields
remain absent; cooking wine does not have invented carbohydrate/fat values.

CFC2002Ed2 fibre values are **insoluble fibre**, not total dietary fibre. They are
stored separately as `insoluble_fiber_g`, labelled accordingly on cards, and are
not inserted into total-fibre calculations. USDA/CFC carbohydrate is total
carbohydrate; the CFS laboratory fruit record uses available carbohydrate, as
documented by the source. The detailed reference view retains these notes.

Migration 038 updates current database references for future snapshots; it does
not rewrite immutable consumption/ledger history. The small hover preview keeps
the per-100-g basis and numbers; provenance remains in the catalogue's expanded
nutrition view and these source records.

## Card stock progress

Each positive batch displays its expiry (or no date) and remaining quantity.
The bar is remaining quantity divided by cumulative positive ledger quantities
for that same batch and household, including quantity corrections upward.
It is a remaining-stock indicator, not a claim that every reduction was eaten:
waste and downward corrections also reduce stock. Separate batches have separate
bars; an unavailable historical denominator produces no invented percentage.
Unknown expiry remains unknown in storage and receives the requested yellow
priority treatment without fabricating an expiry date.
