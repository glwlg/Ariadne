# Ariadne native screenshot / pinned image 第 1 轮修改记录

- 修改角色：第 1 轮修改 subagent
- 模型配置要求：gpt-5.5 xhigh, service_tier=priority
- 修改时间：2026-07-04
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 上一轮审查文档：`.codex-audit/native-capture-pinned-image-iteration-00-audit-2026-07-04/audit-notes.md`
- 本轮范围：`internal/pinnedimage/service.go`、`internal/pinnedimage/service_test.go`、`frontend/src/components/pinned/PinnedImageWindow.vue`、`frontend/src/App.vue`、`frontend/src/style.css`、`cmd/capturesmoke/main_windows.go`
- Git 操作：未暂存、未提交、未回滚他人改动

## 本轮处理的问题

1. 修复 `pinned-image-warm` 预热窗口泄漏到用户面的问题。
2. 修复预热窗口复用后仍被后续 `ensureWarmWindow` 当成 warm 窗口处理的问题。
3. 提升贴图窗口 Esc 关闭路径的稳定性，尤其是没有正常图片内容时也不依赖图片态节点。
4. 排查并修正 `capturesmoke -pin-latency-only` 无法等待到 native 截图覆盖层的问题。

## 修改文件

- `internal/pinnedimage/service.go`
  - 保留隐藏 warm 窗口预热，但不再把 `pinned-image-warm` 复用成真实贴图窗口。
  - warm 窗口 URL 改为 `/?view=pinned-image&warm=1`，不再携带不存在的 `pinId`。
  - 创建 warm 窗口后显式 `Hide()`，避免预热窗口在 WebView 初始化期间短暂可见。
  - 真实贴图窗口继续使用 `pinned-image-<id>` 独立窗口名，避免后续 `ensureWarmWindow` 隐藏或污染真实贴图。

- `internal/pinnedimage/service_test.go`
  - 更新 warm 窗口配置测试，覆盖 hidden warm URL 和 1x1 尺寸。

- `frontend/src/App.vue`
  - 识别 `?view=pinned-image&warm=1`，把 warm 模式传给贴图窗口组件。

- `frontend/src/components/pinned/PinnedImageWindow.vue`
  - 增加 warm 模式：只准备透明 1x1 隐藏窗口，不加载 pin 数据，不显示“贴图已失效”。
  - 普通贴图窗口挂载后主动聚焦根节点，Esc 改为 document 捕获阶段处理。
  - `closeWindow()` 使用当前 active pin id，保留正常图片态右键菜单、OCR、双击关闭、缩放和复制能力。

- `frontend/src/style.css`
  - 增加 `.pinned-image-surface.is-warm` 样式，禁止交互和拖拽。
  - 增加贴图窗口根节点 focus outline 清理。

- `cmd/capturesmoke/main_windows.go`
  - native 截图覆盖层由 `Ariadne.CaptureHost.exe` 独立进程创建，smoke 原来只按 Ariadne 主进程 PID 枚举窗口，因此等不到覆盖层。
  - 覆盖层等待改为全局枚举标题包含“截图覆盖层”的窗口；贴图窗口仍按 Ariadne 主进程 PID 验证。
  - `windowSample` 增加 `processId`，报告中能看出 overlay 与 pinned window 分属哪个进程。

## 修改前后行为

- 修改前：
  - warm URL 带 `pinId=__prewarm__`，前端会尝试读取不存在的 pin，可能显示“贴图已失效”。
  - warm 窗口被复用成真实贴图后仍叫 `pinned-image-warm`，后续 `ensureWarmWindow` 可能把真实贴图当 warm 窗口隐藏。
  - `capturesmoke -pin-latency-only` 只枚举 Ariadne 主进程窗口，无法发现 native host 进程里的覆盖层。

- 修改后：
  - warm 窗口进入专用 warm 模式，隐藏、不可交互、不加载 pin、不显示失效态。
  - 真实贴图窗口始终是独立 `pinned-image-<id>`，不会被 warm 管理逻辑回收或隐藏。
  - Esc 关闭绑定在 document 捕获阶段，窗口挂载后主动聚焦根节点，不再依赖正常图片态。
  - smoke 能识别 native host 覆盖层，并继续验证 Wails 贴图窗口创建耗时。

## 验证命令和结果

- `go test -count=1 ./internal/pinnedimage ./internal/captureoverlay ./internal/nativecapture`
  - 结果：通过。
  - 输出摘要：`ok ariadne/internal/pinnedimage`、`ok ariadne/internal/captureoverlay`、`internal/nativecapture [no test files]`。

- `go test -count=1 ./cmd/capturesmoke`
  - 结果：通过。

- `pnpm --dir frontend build`
  - 结果：通过。
  - 备注：仍有既有 `@vueuse/core` / Rolldown `INVALID_ANNOTATION` 警告，退出码为 0。

- `wails3 task windows:build`
  - 第一次：退出码为 0，但日志显示 `bin\native-capture\Ariadne.CaptureHost.exe` 被运行中的当前仓库进程占用，native host 清理步骤报 `Access to the path ... is denied`。
  - 处理：只结束了路径位于当前仓库 `bin\...` 下的 `ariadne.exe` / `Ariadne.CaptureHost.exe` 测试残留进程。
  - 第二次：通过；`bin\ariadne.exe` 更新时间为 2026-07-04 10:59:03，`bin\native-capture\Ariadne.CaptureHost.exe` 更新时间为 2026-07-04 10:58:50。
  - 备注：同样只有既有 `@vueuse/core` / Rolldown `INVALID_ANNOTATION` 警告。

- `go run ./cmd/capturesmoke -exe bin/ariadne.exe -output .codex-audit/native-capture-pinned-image-iteration-01-modification-2026-07-04/capturesmoke-pin-latency.json -pin-latency-only -max-pin-ms 1200 -timeout-ms 15000`
  - 结果：通过，报告写入 `capturesmoke-pin-latency.json`。
  - 关键证据：
    - `open_overlay_alt_a`：通过，486ms，窗口为 `Ariadne - 原生截图覆盖层`，`processId=174292`。
    - `pin_selection`：通过，312ms，窗口为 `截图贴图 260x180`，`processId=273800`，低于 1200ms。
    - `pass=true`。

## 桌面验证结果

- 完整 Computer Use 桌面流程本轮未继续展开，按用户要求不再因桌面控制限制阻塞。
- 轻量 OS 窗口枚举验证：
  - 启动本轮构建的 `bin\ariadne.exe`，等待 4 秒后枚举该进程可见窗口。
  - 结果只看到 `Ariadne - 网速小窗`，未看到 `Ariadne - 贴图` 或“贴图已失效”窗口。
  - 这说明预热路径没有再把失效贴图窗口暴露到可见窗口列表。
- smoke 真实桌面链路已覆盖：热键打开 native 覆盖层、拖选、按 P 创建 Wails 贴图窗口、测量 pin latency。

## 未处理事项和残余风险

- 失效态 Esc 关闭已做代码路径修复并通过前端构建，但未用 Computer Use 构造一个故意失效 pin 窗口做手动桌面复验。
- 正常贴图右键 OCR、复制 OCR、双击关闭保留在原组件路径中，本轮未重写；smoke 验证了正常贴图窗口可创建，但未手动打开右键 OCR 菜单。
- 当前工作区存在大量本轮范围外的既有未提交改动，本轮没有暂存、提交或回滚。
- 进程检查中仍存在一个路径不可读的 `ariadne` 进程，未确认属于当前仓库，未强制结束。
