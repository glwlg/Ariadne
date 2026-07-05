# Ariadne native screenshot / pinned image 首轮审查

- 审查角色：首轮审查 subagent
- 模型配置要求：gpt-5.5 xhigh, service_tier=priority
- 审查时间：2026-07-04
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 目标边界：当前 native screenshot / pinned image 桌面流程；重点覆盖贴图闪退、贴图窗口细节、右键 OCR、双击/Esc 关闭、截图标注工具栏图标/光标/滚轮粗细/取消选择。
- 执行约束：只审查，不改业务代码，不暂存 Git，不提交 Git；未使用 Playwright。

## 审查范围和证据

本轮使用的证据来自当前运行版 `P:\workspace\glwlg\app\Ariadne\bin\ariadne.exe`、仓库当前代码、Windows 桌面截图、Computer Use 可获取的窗口状态，以及仓库内 `capturesmoke` / Go 测试结果。

关键运行证据：

- `bin\ariadne.exe` 存在，大小 34,559,488 字节，修改时间 2026-07-04 10:31:30。
- `bin\native-capture\Ariadne.CaptureHost.exe` 存在，大小 71,616,601 字节，修改时间 2026-07-04 10:31:20。
- `go test ./internal/captureoverlay ./internal/pinnedimage` 通过。
- `go run ./cmd/capturesmoke -exe bin/ariadne.exe -output .codex-audit/native-capture-pinned-image-iteration-00-audit-2026-07-04/capturesmoke-pin-latency.json -pin-latency-only -max-pin-ms 1200 -timeout-ms 15000` 失败：Ariadne 启动并注册 Alt+A/Alt+Q 后，15 秒内未出现“截图覆盖层”。
- `go run ./cmd/capturesmoke -exe bin/ariadne.exe -output .codex-audit/native-capture-pinned-image-iteration-00-audit-2026-07-04/capturesmoke-current-profile.json -pin-latency-only -max-pin-ms 1200 -timeout-ms 15000 -use-current-profile` 失败：新实例在窗口出现前退出。
- Computer Use 启动指定 exe 时两次失败，错误为 `GetCursorPos failed: 拒绝访问。 (0x80070005)`。后续只能读取到一个已存在的 `Ariadne - 贴图`窗口，无法完整可靠地驱动“截图选择 -> P 贴图”真实桌面流程。

代码证据：

- `internal/pinnedimage/service.go` 当前会创建 `pinned-image-warm` 预热窗口，并在复用预热窗口时调用 `applyPinnedWindow`。
- `frontend/src/components/pinned/PinnedImageWindow.vue` 当前贴图窗口默认 `shadowEnabled = false`，支持右键菜单、OCR、双击关闭、Esc 关闭逻辑。
- `native/Ariadne.CaptureHost/OverlayWindow.cs` 当前工具栏使用 `NativeVisuals.IconButton("lucide:*")`，支持滚轮调粗细、工具按钮激活状态、Esc 取消截图、P 贴图、Q 扫码。
- `native/Ariadne.CaptureHost/NativeVisuals.cs` 当前只手工覆盖了部分 lucide 图标；缺失图标会降级为圆形占位。

## 截图清单

- `01-desktop-after-smoke-failure.png`：smoke 失败后桌面状态，无截图覆盖层、无正常贴图窗口。
- `02-invalid-pinned-window.png`：Computer Use 捕获到的 `Ariadne - 贴图`窗口，尺寸 136 x 39，仅可见贴图图标；辅助树显示“贴图已失效 / 关闭”。
- `03-invalid-pinned-right-click-no-menu.png`：对失效贴图右键后未出现右键 OCR 菜单。
- `04-invalid-pinned-after-esc-still-open.png`：对失效贴图按 Esc 后窗口仍存在。

## P0/P1/P2 发现

### P0

1. 截图到贴图主流程在当前运行版不可验证且 smoke 失败，核心桌面流程被阻断。

   证据：`capturesmoke-pin-latency.json` 中 hotkey 注册成功，`fallback_post_screenshot_hotkey` 也成功投递，但 `wait capture overlay` 失败：15 秒内未出现“截图覆盖层”。当前用户配置下的 `capturesmoke-current-profile.json` 也失败，新实例在窗口出现前退出。桌面截图 `01-desktop-after-smoke-failure.png` 没有覆盖层或正常贴图。

   影响：用户反馈的“贴图闪退/截图贴图”无法进入稳定主路径，后续贴图速度、右键 OCR、标注工具栏等体验无法在真实流程上验收。

2. 当前存在可见但失效的 `Ariadne - 贴图`窗口，疑似预热窗口或失效 pin 状态泄漏到用户面。

   证据：Computer Use 枚举到 `Ariadne - 贴图`，截图尺寸仅 136 x 39；辅助树显示“贴图已失效 / 关闭”。该窗口不是有效截图贴图，也不符合“无阴影无右上按钮但显示图片本体”的预期。

   影响：用户会看到一个空白/残缺小窗，容易理解为贴图闪退或贴图创建失败；同时它占用贴图窗口标题和交互焦点。

### P1

1. Esc 关闭在当前失效贴图窗口上不生效。

   证据：对 `Ariadne - 贴图`按 Esc 后，窗口仍然存在，见 `04-invalid-pinned-after-esc-still-open.png`。代码里 Esc 关闭绑定在前端 `window.addEventListener('keydown', handleKeyDown)`，但当前窗口焦点或失效状态下没有可靠触发关闭。

   影响：用户要求的“Esc 关闭”在异常贴图状态下不可用，出问题时只能靠双击或其他方式关闭，恢复成本偏高。

2. 右键 OCR 菜单在当前可见贴图窗口上不可用。

   证据：失效贴图右键后未出现菜单，见 `03-invalid-pinned-right-click-no-menu.png`。代码上 `v-else-if="image"` 才渲染右键菜单，失效状态只显示“贴图已失效”和关闭按钮。

   影响：如果正常贴图未出现而用户只看到失效贴图，右键 OCR 入口等于缺失。由于本轮无法生成正常贴图，正常图片态右键 OCR 未完成桌面验证。

3. 截图工具栏的 lucide 图标覆盖不完整，缺失图标会显示圆形占位。

   证据：`OverlayWindow.cs` 使用了 `lucide:arrow-up-right`、`lucide:highlighter`、`lucide:grid-3x3`、`lucide:mouse-pointer-2` 等；`NativeVisuals.cs` 只对部分名称有 path，默认分支为圆形。当前代码已覆盖本轮看到的主要名称，但该机制本身没有显式失败提示，后续新增图标容易退化。

   影响：截图标注工具栏需要接近原 Wails/lucide 风格；任何缺失图标都会变成无语义圆形，降低可识别性。

### P2

1. 贴图窗口没有右上关闭按钮是符合当前目标，但异常态仍出现“关闭”文本按钮，且实际截图里按钮不可见。

   证据：`PinnedImageWindow.vue` 正常图片态无右上按钮；失效态有“关闭”按钮。`02-invalid-pinned-window.png` 中窗口尺寸压缩后只看到图标，辅助树才显示“关闭”。

   影响：异常恢复入口视觉不可见，可访问树和实际视觉不一致。

2. 工具栏 hover 光标、再次点击取消选择、滚轮调粗细只完成代码审查，未完成真实桌面验证。

   证据：`OverlayWindow.cs` 中控制区光标设为 Arrow，选区/标注区会切换 Cross/Pen/Hand/SizeAll；`AdjustThicknessWithWheel` 会在存在选区时调整 `_thickness` 并显示反馈；工具按钮 `ActivateTool` 与 `UpdateToolStates` 存在激活态逻辑。但由于截图覆盖层无法打开，未能用真实桌面截图确认 hover 光标、滚轮反馈和再次点击是否取消选择。

   影响：这些交互属于高频细节，仍需下一轮修复后复验。

## 已确认正常项

- `go test ./internal/captureoverlay ./internal/pinnedimage` 通过，说明当前单元测试层没有直接回归。
- 双击当前失效贴图窗口可以关闭；Computer Use 双击后窗口列表中不再出现 `Ariadne - 贴图`。
- 正常图片贴图态代码上默认 `shadowEnabled = false`，且 Wails 窗口配置为 frameless / transparent / always-on-top / disabled native decorations。
- 正常图片贴图态代码上包含右键菜单、OCR 文字识别、复制 OCR 全文、复制选中 OCR、缩放和关闭入口。

## 未能验证的交互项

以下项目因 Computer Use 启动/点击链路受限，以及 `capturesmoke` 无法打开截图覆盖层，未能完成真实桌面验证：

- 正常截图选区出现速度。
- 正常截图后按 P 创建有效图片贴图的耗时和尺寸。
- 正常图片贴图是否完全无阴影、无右上按钮。
- 正常图片贴图右键 OCR 菜单是否可见、位置是否正确、OCR 结果条是否可用。
- 正常图片贴图 Esc 是否关闭。
- 截图覆盖层工具栏图标实际视觉是否全部接近原 Wails/lucide 风格。
- 工具栏 hover 光标实际表现。
- 标注工具再次点击是否取消选择。
- 鼠标滚轮调粗细的实际反馈和绘制结果。

Computer Use 限制记录：

- 使用 `sky.launch_app({ app: "P:\\workspace\\glwlg\\app\\Ariadne\\bin\\ariadne.exe" })` 启动时失败，错误 `GetCursorPos failed: 拒绝访问。 (0x80070005)`。
- 重置 JS kernel 后重试，错误一致。
- 后续 `list_apps` / `list_windows` / `get_window_state` 可读取已有窗口，但不适合继续作为完整桌面流程驱动依据。

## 是否达到“审查不出什么问题来”

没有达到。

当前至少有 2 个 P0 和 3 个 P1。最关键的问题不是视觉细节，而是当前运行版无法稳定进入“截图覆盖层 -> 贴图”主路径，并且已经出现可见的失效贴图窗口。

## 下一轮修改建议

1. 先修复主流程可达性：让 `cmd/capturesmoke -pin-latency-only` 能在当前 `bin\ariadne.exe` 上稳定打开截图覆盖层并生成贴图窗口。
2. 排查 `pinned-image-warm` 预热窗口复用逻辑，确保预热窗口永远不可见、不可交互、不会显示“贴图已失效”，复用时先完成 pinId / URL / 前端数据同步，再 Show/Focus。
3. 修复异常贴图窗口关闭路径：失效态也应保证 Esc 可关闭，视觉上关闭入口必须可见或避免异常窗口出现。
4. 主流程恢复后，用真实桌面复验：右键 OCR、双击/Esc、无阴影无右上按钮、贴图尺寸、工具栏图标、hover 光标、滚轮粗细、再次点击取消选择。
5. 给 native toolbar 图标增加覆盖检查或测试，避免新增 `lucide:*` 名称自动退化为圆形占位。
