# FoodFlow 应用图标

主文件：[foodflow-icon.png](../../web/public/foodflow-icon.png)。README、浏览器标签与 Apple 主屏幕图标共用此 PNG。

绿色碗托起金橙色蒸汽，蒸汽向上舒展为两片嫩叶，表达「一餐热饭，滋养生机」。保留奶油白圆角底板、边框高光和右下方柔和投影；只去掉底板之外的米色背景，外部使用真正的透明 alpha。底板内部保持不透明，投影使用半透明像素，可适应深浅页面。README 以 112px 居中显示。

2026-10-19 原始设计使用 Codex 内置 imagegen，按用户参考调整配色与背景。内置抠图尝试仍有边缘杂点，随后经用户明确授权，用本地像素蒙版从原始已选图提取。圆角轮廓使用抗锯齿处理，内部 RGB 像素逐像素保持一致。原图不含 alpha，投影透明度依据外部背景与阴影的明暗差重建并平滑，不能称为恢复了原始透明通道。

最终文件为 1254×1254 RGBA PNG。检查包括内部像素不变、四角全透明、半透明投影、非透明区域没有分离杂点，以及深浅背景合成预览。原图、未选草案、蒙版处理程序和详细验证记录保留在本地，仓库只保留当前发布图标。主屏幕图标链接不代表离线 PWA 支持；登录页和应用导航现有的线条锅标识不在本次替换范围。


## Real ingredient photo presentation

The 91 catalogue images remain derived from the licensed Wikimedia Commons photographs linked in `web/public/ingredient-photos/credits.html`. On 2026-10-19 the user explicitly chose local Python photo processing over image generation. Reviewed foreground masks and detail crops place real source pixels onto a #faf8f3 canvas, 480×360 with a maximum 422×302 subject area. No generated food, recoloring, inpainting or reconstruction is used. Full-frame textures (rice, soybeans and similar) use an explicitly recorded detail crop; containers already present in photographs can remain. This creates consistent backgrounds/framing, not identical original lighting/camera angles. Family uploads are untouched.

Local mask candidates came from rembg 2.0.85 (`isnet-general-use`/`u2net`), followed by manual outline/detail selection and review. Blind automatic results were rejected where they removed pale food, selected market labels or retained hands. Only the reviewed masks in `scripts/photos/masks/` are required to reproduce the delivered bytes; runtime and CI never download or invoke segmentation models. Each manifest entry records the original Git revision/path/hash, reviewed mask hash, output hash, edit description and original attribution/license. Originals remain in Git history and the cited source pages; private working copies/model weights stay in ignored `.cache`.

Reproduce offline with Python and Pillow 12.0.0 (matching WebP encoder): `python scripts/photos/render.py --out .cache/photo-check`. Fetch the manifest's original revision if using a shallow checkout. The script checks input/mask/output hashes and refuses to overwrite delivered images. New content-derived filenames prevent old photo cache reuse. Browser acceptance decodes all 91 assets, checks dimensions/opaque cream corners, and verifies the cream header on desktop/mobile. These checks do not automatically establish artistic quality or perfect segmentation of arbitrary future uploads; visual review remains necessary.
