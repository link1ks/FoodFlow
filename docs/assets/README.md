# FoodFlow 应用图标

主文件：[foodflow-icon.png](../../web/public/foodflow-icon.png)。README、浏览器标签与 Apple 主屏幕图标共用此 PNG。

绿色碗托起金橙色蒸汽，蒸汽向上舒展为两片嫩叶，表达「一餐热饭，滋养生机」。保留奶油白圆角底板、边框高光和右下方柔和投影；只去掉底板之外的米色背景，外部使用真正的透明 alpha。底板内部保持不透明，投影使用半透明像素，可适应深浅页面。README 以 112px 居中显示。

2026-10-19 原始设计使用 Codex 内置 imagegen，按用户参考调整配色与背景。内置抠图尝试仍有边缘杂点，随后经用户明确授权，用本地像素蒙版从原始已选图提取。圆角轮廓使用抗锯齿处理，内部 RGB 像素逐像素保持一致。原图不含 alpha，投影透明度依据外部背景与阴影的明暗差重建并平滑，不能称为恢复了原始透明通道。

最终文件为 1254×1254 RGBA PNG。检查包括内部像素不变、四角全透明、半透明投影、非透明区域没有分离杂点，以及深浅背景合成预览。原图、未选草案、蒙版处理程序和详细验证记录保留在本地，仓库只保留当前发布图标。主屏幕图标链接不代表离线 PWA 支持；登录页和应用导航现有的线条锅标识不在本次替换范围。


## Studio ingredient references

The user subsequently selected AI-generated photorealistic studio references over the earlier real-photo treatment. All 200 catalogue assets now share a cream background, soft studio light and centered 4:3 framing. They are explicitly described as generated references, not photographs of actual stock or evidence of freshness. Family uploads retain priority. Hover attribution is removed; the separate image explanation remains accessible.

`scripts/photos/studio-sources.json` records generation prompts and source hashes; the public image manifest records source/crop/output hashes. Original generation outputs are retained locally in ignored `.cache/studio-sources`, not downloaded by CI. Earlier licensed photographs, attribution and masks remain in Git history at `6dcadac`. The current generated assets make no claim to those photographs' licenses.

With the retained original outputs and Pillow 12.0.0, reproduce offline using `python scripts/photos/render.py --sources .cache/studio-sources --out .cache/studio-check`. This checks source and output hashes and refuses to overwrite delivered assets. A fresh checkout needs the retained originals to reproduce; generated images cannot be recreated byte-for-byte by repeating a prompt. Browser acceptance checks all 200 images, dimensions and opaque cream corners. Visual review checks framing and recognizable subjects; those automated checks alone do not establish artistic quality.
