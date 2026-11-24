# 方寸 Fancun Logo

## 设计

产品中文名为「方寸」，英文名为 Fancun。标识采用方寸山的形象：三座流动的青黛山峦，以松绿和浅青玉色表现山脊，一条暖金色山径穿过山谷。保留初版三峰及蜿蜒山径的造型，以青绿山水和矿物色替代蓝紫、粉色，材质调整为柔和的玉石质感。外部背景保留真实透明通道。

配色参考：青黛阴影 `#183F43`、松绿山体 `#367568` / `#589781`、青玉高光 `#A8C7AE`、暖金山径 `#D5AC69` / `#F1D9A0`。这些颜色用于 imagegen 提示，最终图片包含连续渐变。

## 资源

- `assets/logo.png`：1254 × 1254 RGBA 原图，透明通道范围为 0–255，四角完全透明。
- `assets/logo.ico`：包含 16、32、48、64、128、256 像素的透明 Windows 图标。
- `frontend/public/favicon.png`：256 × 256 透明网页图标。
- `frontend/src/components/ui/FancunMark.vue`：品牌图片组件。

应用、安装器、原生辅助程序及 Windows 资源的构建均引用上述图标。已安装程序中的嵌入图标需重新构建并安装后更新，UxPlay 运行时也需按现有构建流程重建。旧字母 SVG 已移除。本次使用内置 imagegen 调整配色并抠掉背景；Pillow 仅用于尺寸与格式转换，没有矢量源文件。

## 青黛暖金版提示词

模式：内置 imagegen；两次均设置 `transparent_background: true`。第一次输入为项目原有的蓝紫山形图标，第二次输入为改色后的图片。最终输出已保存到项目资源，未修改界面主题。

### 配色与材质调整

```text
Edit target: the attached existing Fangcun Mountain / 方寸 Fancun desktop app logo. The user specifically requests changing THE ICON'S COLORS to better suit a serene Chinese sacred mountain. This is a palette/material refinement, not a new silhouette. Preserve the same exact three flowing mountain peaks, asymmetric ridges, taller central peak, gently concave slopes, low wide foothill silhouette, and the same sweeping winding path through the valley. Preserve the composition and recognizability. Replace ALL electric cyan / cobalt / purple / magenta with a restrained, beautiful Chinese mineral-pigment palette: deep blue-green qingdai ink shadows (#183F43), rich pine jade / shilv green midtones (#367568, #589781), and soft pale celadon highlights (#A8C7AE). The peaks should still have enough rich midtone saturation and dimensional contrast to read clearly at 24px. Make the existing pink path warm pale mineral gold / ochre (#D5AC69, highlights #F1D9A0), like a softly sunlit mountain trail; not metallic foil, no bright yellow, no orange neon. Mountain surfaces have a satin carved-jade / mineral ceramic feel with broad refined highlights, gentle shallow dimensional shading, calm natural depth, no glossy neon reflections, no grain or photorealistic texture. The gold path must be clearly legible and integrated into the mountain, with no change to its winding geometry. Only recolor and subtly refine the surface material of the existing icon. No extra objects: no sun, moon, stars, clouds, mist, trees, temple, pagoda, lettering, text, app tile or frame. IMPORTANT: remove any edge debris or stray floating pixels from the old extraction. Deliver a clean high-quality antialiased cutout of the solid mountain and path on a REAL TRANSPARENT ALPHA square canvas. Fully opaque mountain and path interiors; alpha zero everywhere outside the precise emblem silhouette, including below the base. No halo, glow, atmospheric haze, ground plane, floor shadow, reflection, painted checkerboard, black backdrop or white backdrop. Center the emblem with balanced transparent padding while keeping its original wide mountain proportions. Must look equally clean against white and dark backgrounds.
```

### 透明背景提取

```text
Use case: precise background removal / transparent logo extraction. Input is the finished NEW COLOR Fangcun Mountain icon. KEEP ONLY the physical opaque jade mountain body and its integrated winding gold path, with its beautiful existing celadon/blue-green/gold coloring, silhouette, gradients and design exactly intact. The requested edit is BACKGROUND EXTRACTION ONLY. Everything outside the precise solid mountain contour must be removed to genuine alpha zero: all black/dark backdrop, all green atmospheric halo and glow above and around the peaks, all gold reflection or colored shadow below the foothill, ALL floor and haze. No aura, no floating pixels, no feathered exterior. The top edge is the precise rounded peak outline; side edges follow the solid physical mountain ridges; bottom edge follows the clean smooth foothill baseline only. Fully opaque solid mountain interior, only 1-2 pixel antialiased boundary. Do not darken, recolor, crop into, or change the mountain or path. Place the clean isolated solid icon on a square transparent canvas with approximately 8% transparent padding and centered vertically. TRUE TRANSPARENT PNG. Do not draw a black background, white background, checkerboard, tile or any replacement backdrop. No text. Return the isolated mountain icon only with actual alpha channel.
```

## 初版生成与抠图提示词

风格参考：`/home/luwei/workspace/glwlg/astrorder/frontend/public/logo.png`。第二张输入为已弃用的字母 F 图标，仅供材质参考。两次调用均设置 `transparent_background: true`。

### 山形生成

```text
Use case: logo-brand / style-transfer.
Create a completely new standalone logo for 方寸 Fancun, a versatile Windows desktop productivity toolbox. The user rejected the letter-based logo and specifically requests the image of 方寸山, a small serene mystical mountain. DO NOT depict a letter, monogram or alphabetic glyph.
Input image 1 (Astrorder's blue-violet star with a pink ribbon) is the primary STYLE reference. Input image 2 is the rejected F-shaped mark: borrow ONLY the beautiful smooth blue/violet/pink dimensional material, NEVER its F shape.
Subject: an iconic miniature sacred mountain sculpted from three broad flowing peaks, a taller elegant central summit and two lower asymmetrical companion ridges. Forms feel serene, assured and simple, like one compact sculptural mountain island with flowing foothills. A single vivid rose-pink winding path emerges at the front foothill, sweeps gracefully through a valley, then disappears behind a ridge, expressing a path through Fangcun Mountain. The path is integrated into the mountain, not an orbital ring around it. Mountain contours should be gently concave and rounded at their feet, with a few clean tapered summits. Keep the silhouette distinctly LANDSCAPE / MOUNTAIN, never an A, M, F, or other letter. No symmetry suggesting a monogram. No cutout triangle that reads as an A.
Style: premium minimalist sculptural app brand logo, polished satin glass/clay material with broad soft highlights, electric cobalt-blue upper left blending into deep violet lower right, vivid pink-to-rose path. Match the sister logo's beautiful saturated gradients and subtle 3D volume. Flatly readable front-facing silhouette with shallow depth; no realistic mountain texture. Visually quiet, crafted, approachable, iconic and immediately legible at 24px.
Composition: one centered compact emblem, roughly square footprint with balanced 10% transparent padding, front-facing, gently wider than tall. Only the mountain and its integrated pink path. No detached scenery or additional objects. No sun, moon, stars, clouds, temple, pagoda, trees, lettering, text, wordmark, watermark, square border or app tile.
Background: REAL TRANSPARENT ALPHA throughout the exterior and any negative spaces. Remove the background entirely. Clean antialiased cutout edges, no white or black backdrop, no painted checkerboard, no ground plane, no cast shadow outside the emblem, no exterior glow or haze. Deliver only the finished transparent Fangcun Mountain logo.
```

### 背景抠除

输入为上一阶段的山形原图。

```text
Use case: background-extraction.
Edit this Fangcun Mountain app logo. KEEP the mountain's three flowing blue-violet peaks and its beautiful winding pink path exactly as designed. Remove ALL of the surrounding blue/purple atmospheric glow, haze, floor, reflection and shadows. Make a clean close cutout around the actual sculpted mountain silhouette only, including the front foothill edge. The solid mountain and path must be fully opaque; only their crisp antialiased silhouette boundary may have partially transparent pixels. Everywhere beyond the silhouette must have alpha exactly zero, including directly below the foothills and around every peak. No soft aura anywhere. This must work as a clean transparent app icon on BOTH white and dark backgrounds. Place the isolated mountain centered on a SQUARE transparent canvas with about 10% padding on all sides, maintaining the original mountain proportions. No extra elements, no background fill, no black, white or checkerboard pixels painted as background, no text. Deliver only the sharply isolated opaque mountain emblem on a genuine transparent alpha background.
```
