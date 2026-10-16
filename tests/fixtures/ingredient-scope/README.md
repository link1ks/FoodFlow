# Ingredient-only image evaluation fixtures

Frozen before the first live scope evaluation on 2026-10-16. Sixteen cases: eight existing licensed catalogue photographs, two public non-food photographs, the project mascot illustration, three synthetic blank/text/instruction images, and two synthetic multi-food composites. Labels are hand-authored by the implementer, not independently reviewed; this is a bounded adapter evaluation, not representative user accuracy.

- `keyboard.jpg`: Jreph, [Keyboard with mouse](https://commons.wikimedia.org/wiki/File:Keyboard_with_mouse.jpg), [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/). Resized to fit 960 pixels; EXIF orientation applied and metadata removed.
- `mouse.jpg`: Jbtonystark, [Computer Mice](https://commons.wikimedia.org/wiki/File:Computer_Mice.jpg), [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Resized to fit 960 pixels; EXIF orientation applied and metadata removed.
- `mixed-vegetables.jpg`, `mixed-egg-tomato.jpg`: test composites of existing catalogue photos. Tomato: Softeis, CC BY-SA 3.0; cucumber: Olaasunbo, CC0; egg: Evan-Amos, public domain. Composites are distributed under CC BY-SA 4.0. Source links and exact attribution are retained in [catalogue photo inventory](../../../web/src/catalogPhotos.json) and [photo licenses](../../../web/public/ingredient-photos/manifest.json).
- `blank.png`, `printed-name.png`, `instruction.png`: newly generated synthetic fixture canvases, project license. No personal photographs, household data or authenticated screenshots.
- Other photographs and mascot remain at their original repository paths in `manifest.json`, with their existing licenses.

Run `go run ./cmd/intake-scope-eval` offline. Real calls require explicit task authorization, `-paid -authorized-token-limit 320000`, and a fresh ignored `.cache/harness` journal. The journal prevents blind resume; each sample is sent once, without retries or redirects. Token reservations are conservative local limits, not a provider billing or free-quota guarantee.
