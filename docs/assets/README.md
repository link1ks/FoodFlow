# FoodFlow 应用图标

主文件：[foodflow-icon.png](../../web/public/foodflow-icon.png)，1254 × 1254。README、浏览器标签与 Apple 主屏幕图标共用此 PNG，不重复保存副本。

标志保留森林绿小锅、金色锅盖与叶芽，减少手脚、木勺和蒸汽细节。软件图标母版采用完整方形暖白背景，由使用平台施加圆角遮罩；当前网页不是可离线安装的 PWA。

2026-10-12 使用 Codex 内置 imagegen 编辑生成，未使用应用模型密钥或备用 API 脚本。旧插画与未选版本保留在本地生成存储/归档，不随源码发布。最终文件为原始输出，未自行抠图或修改像素。

<details>
<summary>生成提示词与修订</summary>

首次设计：

```text
Use case: style-transfer / logo-brand. Input image is the existing FoodFlow cooking-pot spirit mascot, used as the identity reference. Redesign this mascot into a polished standalone SOFTWARE APPLICATION ICON for FoodFlow, retaining a forest-green anthropomorphic cooking pot, warm golden lid with a little two-leaf sprout, and a friendly tiny face. Change the detailed painted full-body mascot into a clean compact brand symbol. Square icon composition: one generously rounded-square warm ivory tile (#faf2dd), perfectly centered, with genuine transparent alpha only outside its rounded corners. Inside the tile, a bold forest-green pot silhouette (#193f30), two simple small cream eyes and a small curved happy smile, one slightly tilted golden lid (#e9bd63) and one simple leaf sprout (#89ab65). Pot handles are short simple symmetric shapes. Pot symbol fills about 68 percent of tile width with comfortable balanced optical margins. No arms, legs, wooden spoon, floating steam, scene or other decorations. Clean geometric vector-like edges with very restrained soft dimensional shading; premium friendly kitchen productivity app identity, readable at 32px and 48px, strong silhouette and contrast, simple 4-color design. Straight-on centered composition, no perspective mockup, no surrounding scene, no multiple icons, no lettering, no text, no watermark. Deliver only the complete one app icon tile on a square canvas. Keep the same recognizable kitchen-utensil spirit identity; the pot itself is the character, never a human or elf.
```

最终修订（输入为首次设计结果，`transparent_background=false`）：

```text
Use case: precise-object-edit. Edit the provided FoodFlow app icon into a production-ready square software app icon master. Preserve the exact friendly forest-green pot, golden lid, two-leaf sprout, short pot handles, cream eyes and smile, composition and colors. Replace only the tile and surroundings: the entire square image canvas must have a completely clean solid opaque warm ivory background #faf2dd, full bleed to all four edges. Remove the existing rounded tile boundary, ALL transparent areas, jagged cutout edges, stray flecks, rough halos and marks. No corners cut out; an OS can apply its own app-icon mask later. Maintain balanced safe margins around the pot. Smooth crisp icon artwork, restrained shading, no painted grain, no shadow outside a tile, no text, no border, no watermark, no mockup. Output one full square opaque icon, artwork fills the same central area; do not change the mascot identity.
```

</details>
