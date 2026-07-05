# Ariadne native screenshot / pinned image 第 3 轮独立复审

- 审查角色：独立复审 subagent
- 模型配置要求：gpt-5.5 xhigh, service_tier=priority
- 审查时间：2026-07-04
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 功能边界：native screenshot / pinned image 桌面流程；重点覆盖贴图闪退、贴图延迟、贴图无阴影/无右上按钮、右键 OCR、双击/Esc 关闭、标注工具图标/光标/滚轮/取消选择。
- 执行约束：只审查，不改业务代码，不暂存、不提交；未使用 Playwright。

## 审查范围和证据

已读取并交叉核对的文档和证据：

- `.codex-audit/native-capture-pinned-image-iteration-00-audit-2026-07-04/audit-notes.md`
- `.codex-audit/native-capture-pinned-image-iteration-01-modification-2026-07-04/audit-fixes.md`
- `.codex-audit/native-capture-pinned-image-iteration-02-modification-2026-07-04/audit-fixes.md`
- `.codex-audit/native-capture-pinned-image-iteration-03-modification-2026-07-04/audit-fixes.md`
- `.codex-audit/native-capture-pinned-image-iteration-03-modification-2026-07-04/capturesmoke-full-maincheck.json`
- 当前 `git diff`，重点文件包括 `internal/captureoverlay/service.go`、`internal/pinnedimage/service.go`、`frontend/src/components/pinned/PinnedImageWindow.vue`、`native/Ariadne.CaptureHost/*`、`cmd/capturesmoke/main_windows.go`、`main.go`、`Taskfile.yml`。

本轮新增验证：

- `go test -count=1 ./internal/captureoverlay ./internal/pinnedimage ./internal/nativecapture ./cmd/capturesmoke`
  - 结果：通过。
- `pnpm --dir frontend build`
  - 结果：通过。
  - 备注：仍有既有 `@vueuse/core` / Rolldown `INVALID_ANNOTATION` 警告，退出码为 0。
- `go run ./cmd/capturesmoke -exe bin/ariadne.exe -output .codex-audit/native-capture-pinned-image-iteration-03-audit-2026-07-04/capturesmoke-full-review.json -max-pin-ms 1200 -timeout-ms 15000`
  - 结果：通过，`pass=true`。
  - 关键数据：`open_overlay_alt_a=397ms`，`pin_selection=312ms`，`compare_capture_content=100.00% / mean_abs_diff=0.000`，`drag_pinned_window attempts=1 delta=71,43`。
  - 贴图窗口：`title="截图贴图 260x180"`，`hasCaption=false`，`hasThickFrame=false`，`isTopmost=true`。
- `git diff --check`
  - 结果：无空白错误；只输出本仓库已知 LF/CRLF 工作区提示。

Computer Use 记录：

- 本轮未使用 Playwright。
- 当前工具发现未暴露可调用的 Computer Use 工具；因此没有继续做鼠标/键盘级手动桌面驱动。首轮报告里已有 `GetCursorPos failed: 拒绝访问。 (0x80070005)` 的限制记录。本轮不把该限制作为阻塞，改用 `capturesmoke`、当前 diff 和代码路径核对。

## P0/P1/P2 发现

### P0

未发现。

### P1

未发现。

### P2

未发现有意义的 P2。

## 已确认落地的修复

1. 截图到贴图主流程已恢复并稳定通过 smoke。
   - 首轮 P0 中“15 秒内未出现截图覆盖层”的问题已由 native overlay 全局窗口枚举修复。
   - 最新主会话完整 smoke 和本轮复跑均通过，贴图窗口能创建、定位、拖动。

2. 贴图延迟满足当前验收口径。
   - 最新主会话 smoke：`pin_selection=314ms`。
   - 本轮复跑：`pin_selection=312ms`。
   - 两次均低于 `max-pin-ms=1200`。

3. 预热窗口失效态泄漏已收敛。
   - `pinned-image-warm` 改为 `?view=pinned-image&warm=1`，前端 warm 模式不加载 pin、不显示“贴图已失效”，后端创建后立即隐藏。
   - 真实贴图窗口使用 `pinned-image-<id>`，不再复用 warm 窗口名。

4. 贴图窗口默认无阴影、无右上按钮、无原生标题栏。
   - 前端 `shadowEnabled` 默认 `false`。
   - 正常图片态模板没有右上关闭工具条。
   - smoke 采集到的 Wails 贴图窗口 `hasCaption=false`、`hasThickFrame=false`。

5. 右键 OCR 路径保留。
   - `PinnedImageWindow.vue` 右键菜单仍包含 `OCR 文字识别`、`复制选中 OCR`、`复制 OCR 全文`。
   - 截图贴图来源为 `capture`，`newPinnedImage()` 对 `capture` 设置 `canOcr=true`。
   - OCR 动作仍走 `recognizeCaptureOCR(sourceId)`；native pin 先落入截图历史再通过 pinned service 打开，因此 sourceId 仍是可识别的 capture ID。

6. 双击和 Esc 关闭路径保留。
   - 正常贴图面板 `@dblclick="closeWindow"`。
   - Esc 改为 `document.addEventListener('keydown', ..., true)` 捕获阶段处理，关闭前端窗口并调用 `ClosePinned(activePinId)`。
   - 失效态也保留“关闭”按钮；即使 pin 数据不存在，`Window.Close()` 仍会执行。

7. 右键 OCR 以外的贴图交互未见回退。
   - 滚轮缩放、复制源、复制 OCR、全选/清空 OCR 行、关闭贴图仍在同一组件路径中。
   - menu 打开时会临时扩展窗口，关闭菜单后恢复图片窗口尺寸。

8. native 标注工具主要交互保留。
   - 工具栏当前使用的 `lucide:*` 名称均在 `NativeVisuals.cs` 有对应图形分支，没有看到当前工具按钮会落到默认圆形占位。
   - hover 光标路径保留：控件区域 Arrow，标注编辑 Pen/IBeam，选择工具 Hand，移动 SizeAll，边缘 resize cursor。
   - 鼠标滚轮粗细路径保留：`PreviewMouseWheel -> SetThickness()`，反馈文案为当前粗细。
   - 再次点击同一工具取消选择路径保留：`ActivateTool()` 检测 active 后把 `_tool` 置回 `None` 并清空选中标注。
   - Esc 取消截图、P 贴图、Q 扫码、Enter 复制、Shift+Enter 打码复制均保留在 native host 键盘处理里。

9. 右键 QR / OCR 相关后端路径未被 pin 修复破坏。
   - native `Q` 走 `Finish("qr")`，后端 `captureNativeSelection()` 仍用截图历史 entry 解码二维码并复制文本。
   - pin 的右键 OCR 是 pinned image 组件功能，不依赖 native host 常驻状态。

## 是否达到“审查不出什么问题来”

达到。

本轮没有发现仍需下一轮修改的有意义 P0/P1/P2。允许收敛。

## 可接受残余风险

- 本轮没有可用 Computer Use，因此右键 OCR 菜单、双击/Esc、hover 光标、滚轮粗细、再次点击取消选择没有做鼠标级手动录制；结论来自最新完整 smoke、代码路径和构建/测试。
- 当前完整 smoke 覆盖默认/隔离配置下的 pin 延迟；如果用户显式开启自动打码，当前代码仍会把相关截图动作纳入 OCR/打码策略，延迟可能包含 OCR 成本。这属于显式策略路径，不作为本轮默认贴图延迟回归。
- smoke 当前是单显示器、当前 DPI/桌面环境证据；多显示器和特殊 DPI 仍依赖既有 native bounds 代码审查与单元测试，不是本轮新增桌面实测覆盖。
- `cmd/capturesmoke` 的内容比较和拖动验证仍依赖真实桌面输入事件；已经通过重试和阈值收紧降低误杀，但远程桌面焦点或系统输入拦截仍可能造成环境型失败。

## 下一轮修改建议

无必须进入下一轮的修改建议。

若后续继续加固，建议只补自动化覆盖：给 smoke 增加右键菜单/OCR 入口、Esc 关闭、双击关闭、标注工具激活/取消和滚轮粗细的可观测步骤；当前不需要为这些残余风险阻塞收敛。
