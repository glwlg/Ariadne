# Native Capture Migration 首轮审查

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 范围：截图功能从 WebView/Vue 迁移到 native WPF/Win32 host；不引入 WinUI 3；Windows 截图路径不继续依赖 WebView；视觉体验尽量复刻原有截图体验。
- 本轮性质：首轮独立审查，只写审查文档，不修改业务代码、不暂存、不提交。

## 审查证据

已读代码：

- 原 WebView/Vue 截图入口与状态：`frontend/src/App.vue:10-17`, `frontend/src/App.vue:43-51`, `frontend/src/App.vue:168-170`
- 原截图协议与类型：`frontend/src/types/ariadne.ts:120-186`, `frontend/src/types/ariadne.ts:1816-1832`
- 原截图覆盖层：`frontend/src/components/capture/CaptureOverlayWindow.vue`
- 原贴图窗口：`frontend/src/components/pinned/PinnedImageWindow.vue`
- 原 OCR 选区：`frontend/src/components/ocr/OCRImageOverlay.vue`, `frontend/src/lib/ocrSelection.ts`
- 原视觉样式：`frontend/src/style.css:12163-13148`, `frontend/src/style.css:14250-14544`
- 当前 native host：`native/Ariadne.CaptureHost/*`
- Go native manager 与接入：`internal/nativecapture/*`, `internal/captureoverlay/service.go`, `main.go`
- 打包链路：`Taskfile.yml`, `internal/releasepack/package.go`, `internal/msixpack/package.go`

未做的验证：

- 未运行 native host 实机交互截图，也未对 WPF 窗口做像素级视觉 QA。本轮视觉结论基于 WPF 代码与原 CSS/模板对比。
- 未运行构建/测试命令，避免除审查文档外写入仓库产物或缓存。仓库当前已有 `bin/native-capture/Ariadne.CaptureHost.exe`，但本轮未执行。

## 原体验必须迁移清单

1. 截图窗口入口与 WebView 路由  
   原应用通过 `?view=capture-overlay` 和 `?view=pinned-image` 加载 Vue 窗口，且这两类窗口设置 `alwaysOnTop=true`、`frameless=true`、贴图透明背景、不可 resize。证据：`frontend/src/App.vue:10-17`, `frontend/src/App.vue:43-51`, `frontend/src/App.vue:168-170`。

2. 选区协议与动作  
   原 `CaptureOverlaySelectionRequest` 支持 `capture`、`copy`、`redact_copy`、`pin`、`qr`、`save_as`，并能传递 `coordinateSpace`、显示尺寸、贴图位置、标注操作和前端渲染后的 PNG。证据：`frontend/src/types/ariadne.ts:136-152`。

3. 标注模型  
   原标注操作包括矩形、直线、箭头、画笔、荧光笔、马赛克、文字、序号、橡皮擦。证据：`frontend/src/types/ariadne.ts:159-174`。

4. 选区创建、缩放和移动  
   原覆盖层有拖拽创建选区、8 个缩放把手、重新选择、移动已有标注。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:54-63`, `frontend/src/components/capture/CaptureOverlayWindow.vue:299-360`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1897-1923`。

5. 标注绘制和导出一致性  
   原覆盖层把标注操作按选区比例缩放，并通过 `renderAnnotatedSelectionPNG` 把用户看到的标注结果作为 `renderedImage` 发给后端。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:410-441`, `internal/captureoverlay/service.go:1684-1697`。

6. 工具栏完整度  
   原工具栏包含扫码、所有标注工具、保存到历史、另存、撤销/重做/清空/删除、复制、打码复制、贴图、颜色选择和粗细滑杆。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:1954-2111`。

7. 快捷键完整度  
   原快捷键包括 Esc 关闭、Enter 复制、Shift+Enter 打码复制、P 贴图、Q 扫码、R/L/A/B/H/M/T/N/E/V 切工具、C 取色、Ctrl+S 另存、Ctrl+Z/Y 撤销重做、Del 删除标注。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:563-625`, `frontend/src/components/capture/CaptureOverlayWindow.vue:2119-2141`。

8. 放大镜与取色  
   原覆盖层有跟随鼠标的圆形放大镜、十字线、RGB/HEX 显示和 C 复制颜色。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:164-209`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1431-1440`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1742-1752`。

9. 打码/OCR 脱敏  
   原后端支持 `redact_copy` 与自动打码策略，能用 OCR 几何区域遮盖手机号和关键词；设置页承诺“复制、保存、贴图前遮盖命中的文字区域”。证据：`internal/captureoverlay/service.go:302-327`, `internal/captureoverlay/service.go:365-376`, `internal/captureoverlay/service.go:618-639`, `internal/captureoverlay/service.go:642-718`, `frontend/src/components/settings/SettingsCenter.vue:685-706`。

10. QR 扫码  
    原覆盖层 `qr` 动作会解码选区二维码，成功后复制文本并关闭窗口。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:513-522`, `internal/captureoverlay/service.go:460-475`。

11. 截图策略  
    原设置包含截图后复制、贴图、自动保存、质量、自动打码、手机号和关键词。证据：`frontend/src/types/ariadne.ts:1822-1832`, `frontend/src/components/settings/SettingsCenter.vue:664-712`, `main.go:441-451`。

12. DPI/多屏坐标  
    原 WebView 路径保留 `bounds` 与 `nativeBounds`，并在视觉坐标、DIP 坐标和物理像素之间转换。证据：`frontend/src/lib/captureGeometry.ts:35-84`, `internal/captureoverlay/service.go:1405-1440`, `internal/captureoverlay/service_test.go:96-157`。

13. 贴图窗口交互  
    原贴图支持右键菜单复制、OCR、复制选中 OCR、复制全文、放大/缩小/原始比例、阴影、关闭，滚轮缩放，Esc 关闭，双击关闭，OCR 选区框点击选择。证据：`frontend/src/components/pinned/PinnedImageWindow.vue:65-130`, `frontend/src/components/pinned/PinnedImageWindow.vue:191-254`, `frontend/src/components/pinned/PinnedImageWindow.vue:378-453`, `frontend/src/components/ocr/OCRImageOverlay.vue:64-81`。

14. 贴图状态与窗口位置  
    原 pinned image service 会创建可查询的 `PinnedImage`，保存 `windowWidth/windowHeight/windowX/windowY/positioned/canOcr/copyAction`，并用 WebView 窗口打开；测试覆盖位置同步和 OCR 标志。证据：`internal/pinnedimage/service.go:32-52`, `internal/pinnedimage/service.go:86-112`, `internal/pinnedimage/service.go:261-324`, `internal/pinnedimage/service_test.go:16-70`。

15. 视觉质感  
    原覆盖层有浅色/主题化玻璃面板、backdrop blur、柔和阴影、主题色选区、放大镜、圆角工具条、hint pill、toast；贴图有透明背景、hover/context menu、OCR strip、阴影切换。证据：`frontend/src/style.css:12203-12335`, `frontend/src/style.css:12568-12750`, `frontend/src/style.css:12851-13148`, `frontend/src/style.css:14250-14544`。

## 当前 native 实现对比

已确认符合目标的部分：

- 未引入 WinUI 3：`native/Ariadne.CaptureHost/Ariadne.CaptureHost.csproj:1-12` 是 `net8.0-windows` + `UseWPF=true`。
- Windows Open 路径走 native manager：`main.go:164-168`, `internal/captureoverlay/service.go:200-206`, `internal/captureoverlay/service.go:253-267`。
- native host 通过 named pipe 暴露 `ping/shutdown/capture`：`native/Ariadne.CaptureHost/HostServer.cs:45-102`。
- native 截图使用 Win32/GDI 与 per-monitor DPI：`native/Ariadne.CaptureHost/NativeMethods.cs:22-32`, `native/Ariadne.CaptureHost/NativeMethods.cs:80-147`, `native/Ariadne.CaptureHost/ScreenCapture.cs:24-47`。
- native 覆盖层有多屏窗口、dim mask、选区边框、移动/缩放、尺寸标签、基础工具栏、Esc/Enter/P/S：`native/Ariadne.CaptureHost/CaptureCoordinator.cs:22-43`, `native/Ariadne.CaptureHost/OverlayWindow.cs:105-213`, `native/Ariadne.CaptureHost/OverlayWindow.cs:223-320`, `native/Ariadne.CaptureHost/OverlayWindow.cs:323-405`, `native/Ariadne.CaptureHost/OverlayWindow.cs:453-663`。
- native 贴图窗口有透明窗口、拖动、hover 工具栏、复制/关闭、右键菜单：`native/Ariadne.CaptureHost/PinWindow.cs:25-50`, `native/Ariadne.CaptureHost/PinWindow.cs:53-113`, `native/Ariadne.CaptureHost/PinWindow.cs:116-125`。
- Taskfile 会先发布 native host，再构建/打包 Ariadne：`Taskfile.yml:14-28`, `Taskfile.yml:35-56`。
- release/msix 包装会复制 `native-capture` 目录：`internal/releasepack/package.go:83-92`, `internal/msixpack/package.go:87-97`。

## P0

### P0-1 native 截图路径绕过 OCR 打码/脱敏，可能泄漏用户已要求遮盖的信息

证据：

- 原路径支持 `redact_copy` 和自动打码策略：`frontend/src/types/ariadne.ts:145-150`, `frontend/src/components/capture/CaptureOverlayWindow.vue:588-590`, `frontend/src/components/capture/CaptureOverlayWindow.vue:2086-2088`, `internal/captureoverlay/service.go:365-376`, `internal/captureoverlay/service.go:629-639`。
- 设置页文案明确承诺自动打码会在“复制、保存、贴图前”遮盖命中文字区域：`frontend/src/components/settings/SettingsCenter.vue:685-706`。
- native request/response 只有 `Command` 和 `AutoPin`，没有打码动作、OCR 策略、标注、QR 等字段：`native/Ariadne.CaptureHost/CaptureModels.cs:3-20`, `internal/nativecapture/manager_windows.go:33-48`。
- `openNative` 只把 `AutoPin` 传给 native host：`internal/captureoverlay/service.go:253-258`。
- `captureNativeSelection` 解码 native PNG 后直接保存/复制/贴图，未调用 `redactionPolicyForAction`、`redactSelectionPNGWithCache` 或任何 OCR 打码逻辑：`internal/captureoverlay/service.go:483-577`。

影响：

- 用户启用自动打码或使用原来的“打码并复制”工作流时，迁移后的 Windows native 路径无法执行脱敏。截图可能被复制、保存或贴到屏幕时保留手机号/关键词等敏感内容。

建议修改：

- native host 必须支持 `redact_copy` 动作，或至少把原始选区 PNG 和动作返回 Go 后，由 `captureNativeSelection` 在保存/复制/贴图前复用现有 OCR redaction pipeline。
- 自动打码策略要在 native 路径与 WebView 路径保持一致，并补 native-path 单元测试：autoRedact+save/capture、redact_copy、OCR missing geometry、OCR provider failure。

## P1

### P1-1 native 覆盖层缺少原核心工具：标注、马赛克/橡皮、文字/序号、撤销重做、颜色/粗细

证据：

- 原标注模型覆盖 9 类操作：`frontend/src/types/ariadne.ts:159-174`。
- 原工具栏暴露完整标注/撤销/重做/清空/删除/颜色/粗细：`frontend/src/components/capture/CaptureOverlayWindow.vue:1954-2111`。
- 原导出会把标注后的 PNG 发给后端：`frontend/src/components/capture/CaptureOverlayWindow.vue:410-441`。
- native toolbar 只有复制、贴图、另存、取消：`native/Ariadne.CaptureHost/OverlayWindow.cs:199-213`。
- native response 没有 `operations` 或 `renderedImage`：`native/Ariadne.CaptureHost/CaptureModels.cs:9-20`。

影响：

- 迁移后 Windows 截图变成只能选区/复制/贴图/保存，原有截图工具最常用的标注和打码能力大面积消失，体验明显倒退。

建议修改：

- 在 WPF overlay 内实现至少与原 toolbar 同级的工具集合：矩形、箭头、直线、画笔、荧光、马赛克、文字、序号、橡皮、选择/移动、撤销/重做、清空/删除、颜色、粗细。
- 保存/复制/贴图前应生成与屏幕预览一致的 final PNG，再交给 Go 做历史/剪贴板/自动保存/打码。

### P1-2 native 覆盖层缺少 QR 扫码入口

证据：

- 原协议和 UI 有 `qr` 动作：`frontend/src/types/ariadne.ts:145`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1957-1959`。
- 原结果处理成功后复制二维码文本并关闭：`frontend/src/components/capture/CaptureOverlayWindow.vue:513-522`。
- 后端原路径会 `qrscan.DecodeImagePath`：`internal/captureoverlay/service.go:460-475`。
- native toolbar/key handling 没有 QR：`native/Ariadne.CaptureHost/OverlayWindow.cs:199-213`, `native/Ariadne.CaptureHost/OverlayWindow.cs:296-320`。

影响：

- 截图扫码是原覆盖层的一等动作。迁移后 Windows 主路径无法扫码，属于明确功能倒退。

建议修改：

- native host 返回 `Action="qr"` 的 PNG 给 Go，由现有 `qrscan.DecodeImagePath` 处理；或 native host 自行扫码但结果 shape 要和 `CaptureResult.QR` 对齐。

### P1-3 native 覆盖层缺少“保存到截图历史”独立动作

证据：

- 原 action union 包含 `capture`：`frontend/src/types/ariadne.ts:145`。
- 原 toolbar 有“保存到截图历史”按钮：`frontend/src/components/capture/CaptureOverlayWindow.vue:2060-2062`。
- native toolbar 只有复制、贴图、另存、取消：`native/Ariadne.CaptureHost/OverlayWindow.cs:199-213`。
- native `HandleKeyDown` 只有 Enter/C、P、S：`native/Ariadne.CaptureHost/OverlayWindow.cs:296-320`。

影响：

- 用户不能只把选区保存进截图历史而不触发复制、贴图或另存对话框。虽然 native copy/pin 也会入历史，但动作语义和用户控制能力退化。

建议修改：

- 增加 `capture` 按钮和快捷键，native response `Action="capture"`，Go 侧继续写入 capture history。

### P1-4 native 贴图没有接入原 pinned-image service，贴图能力和状态不可追踪

证据：

- 原 `pinnedimage.Service` 会创建 `PinnedImage`，保存 window size/position/canOcr/copyAction，并通过 `GetPinned/ClosePinned/SetPinnedPosition` 管理状态：`internal/pinnedimage/service.go:32-52`, `internal/pinnedimage/service.go:184-258`, `internal/pinnedimage/service.go:261-324`。
- 原 Vue 贴图支持复制、OCR、选中 OCR 复制、缩放、阴影、关闭：`frontend/src/components/pinned/PinnedImageWindow.vue:65-130`, `frontend/src/components/pinned/PinnedImageWindow.vue:191-254`, `frontend/src/components/pinned/PinnedImageWindow.vue:378-453`。
- native `PinWindow` 只在 WPF host 进程中显示图片，工具栏/右键菜单只有复制和关闭：`native/Ariadne.CaptureHost/PinWindow.cs:53-113`。
- Go native path 对 `shouldPin` 只返回一个 `PinID: "native"` 的假结果，没有调用 `s.openPinnedSelection` 或在 pinned service 中登记实际贴图：`internal/captureoverlay/service.go:567-576`。

影响：

- 贴图窗口无法 OCR、选择 OCR 文本、缩放、切阴影、持久同步位置或通过原服务关闭/管理。`PinID="native"` 也无法对应真实可查询对象。

建议修改：

- 明确产品选择：如果贴图也要 native 化，应让 native PinWindow 具备原能力并有可管理的 pin id；如果只迁移截图覆盖层，native action=pin 应把 capture entry 交回 Go 的 pinnedimage service 打开原贴图窗口。当前两边都不完整。

### P1-5 native 视觉只做到基础 WPF 工具条，未达到原截图体验复刻级别

证据：

- 原视觉有放大镜、玻璃面板、主题色、完整 hint、toast、选区尺寸和标注状态、颜色条、粗细滑杆：`frontend/src/style.css:12207-12335`, `frontend/src/style.css:12568-12750`, `frontend/src/style.css:14250-14544`。
- native overlay 有 dim mask、选区阴影、dark toolbar、hint，但没有放大镜/取色、主题化、标注态、颜色条、粗细控件：`native/Ariadne.CaptureHost/OverlayWindow.cs:122-183`, `native/Ariadne.CaptureHost/OverlayWindow.cs:199-213`, `native/Ariadne.CaptureHost/NativeVisuals.cs:40-107`。

影响：

- 从用户视角看，native 主路径会像“简化版截图弹窗”，达不到用户要求的“尽量复刻原有截图体验的级别”。

建议修改：

- 先按原 CSS 拆出 native visual spec：浅色 glass toolbar、主题色 selection、hint pill、toast、magnifier、color swatches、thickness slider、tool active/disabled states、busy/feedback states。再用 WPF 控件复刻，不要只加功能按钮。

### P1-6 native 主路径没有并发打开保护

证据：

- 原 WebView `Open()` 在非 native 路径使用 `tryBeginOpen/finishOpen` 防并发：`internal/captureoverlay/service.go:214-217`, `internal/captureoverlay/service.go:269-283`。
- Windows native 分支在进入该 guard 之前直接 `return s.openNative(native)`：`internal/captureoverlay/service.go:200-206`。
- native manager 每次 `Capture` 都会直接 roundTrip 到同一 named pipe：`internal/nativecapture/manager_windows.go:75-87`, `internal/nativecapture/manager_windows.go:151-170`。

影响：

- 连按截图热键或 shell 重入时，可能同时发起多个 native capture request。Named pipe server 只开一个实例，用户可能遇到请求排队、失败或覆盖层状态混乱。

建议修改：

- native 路径复用 `tryBeginOpen/finishOpen`，并在 manager 层把 capture request 串行化或返回“截图覆盖层正在打开”。

## P2

### P2-1 native 缺少放大镜和取色

证据：

- 原放大镜/取色：`frontend/src/components/capture/CaptureOverlayWindow.vue:164-209`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1431-1440`, `frontend/src/components/capture/CaptureOverlayWindow.vue:1742-1752`。
- native overlay 只有 cursor/selection，没有同类控件：`native/Ariadne.CaptureHost/OverlayWindow.cs:105-213`。

建议修改：

- WPF overlay 根据鼠标位置裁剪 `_capture.Source` 附近像素，显示圆形放大镜、十字线和 RGB/HEX；支持 C 复制、Shift 切换格式。

### P2-2 native save/copy/pin 的消息和历史 tags 不能体现标注/打码/QR 等语义

证据：

- 原 `actionTags` 会记录 action、sideEffects 和每类 operation：`internal/captureoverlay/service.go:1546-1560`。
- native path 调用 `actionTags(action, nil, sideEffects)`，永远没有 `annotated` 或具体操作 tag：`internal/captureoverlay/service.go:538`。

建议修改：

- 等 native 标注/QR/打码动作补齐后，同步把 action 和 operation metadata 返回 Go，保持历史记录可检索。

### P2-3 release/msix 打包代码把 native-capture 当 optional，测试没有防止漏包

证据：

- `copyOptionalDirectory` 在 `native-capture` 目录不存在时返回 nil：`internal/releasepack/package.go:263-267`, `internal/msixpack/package.go:330-334`。
- 当前 release/msix 测试只断言 app/logo/manifest/readme，不断言 `native-capture/Ariadne.CaptureHost.exe`：`internal/releasepack/package_test.go:49-125`, `internal/msixpack/package_test.go:13-91`。
- Taskfile 会先构建 native host：`Taskfile.yml:14-28`, `Taskfile.yml:35-56`，因此标准 task 下风险较低。

建议修改：

- 对 `windows:package` 和 `windows:msix` 的结果增加 native host 文件断言；如果用户安装包里的 Windows 截图必须 native，releasepack/msixpack 应在 Windows 构建中把缺失 native host 视为错误，而不是静默省略。

### P2-4 native host 崩溃或退出后 manager 可能不会自动重启

证据：

- `startLocked` 只判断 `m.command != nil && m.command.Process != nil` 就认为已启动：`internal/nativecapture/manager_windows.go:114-117`。
- 如果 host 进程异常退出但 `m.command` 未清空，下一次 `Capture` 可能直接 roundTrip 到不存在的 pipe，且不会重建进程：`internal/nativecapture/manager_windows.go:75-87`, `internal/nativecapture/manager_windows.go:151-170`。

建议修改：

- 在 roundTrip 失败且命令进程已退出/pipe 不存在时清空 command 并重试启动一次；补 host crash/restart 单元或集成 smoke。

### P2-5 缺少当前 native 视觉证据

证据：

- 本轮仅从 WPF/CSS 代码对比判断视觉，没有 native host 实机截图或多 DPI 截图。

建议验证方式：

- 运行已构建的 `bin/ariadne.exe`，用截图热键分别在 100%/150% DPI、单屏/副屏/负坐标副屏场景录制或截图。
- 对比原 Vue 覆盖层截图：初始悬停、选区创建中、选区完成、toolbar 打开、贴图 hover、贴图右键菜单。
- 检查 WPF overlay 是否遮挡任务栏、是否漏截透明/阴影窗口、是否跨 DPI 坐标准确。

## 结论

当前实现已经把 Windows `Open()` 主入口切到 native WPF/Win32 host，并具备基础选区、复制、贴图、另存、DPI/多屏和打包链路；没有引入 WinUI 3。

但它还没有达到“迁移原截图体验”的标准。最高风险是 native 路径绕过 OCR 打码/脱敏；其次是标注、QR、保存到历史、贴图 OCR/缩放/状态管理和原视觉质感缺失。当前结论：未达到“审查不出什么问题来”，应进入修改阶段。

## 下一轮修改建议

优先顺序：

1. 先修 P0：native 路径必须复用现有 Go redaction/QR/history pipeline，至少补 `redact_copy` 和 autoRedact 的行为一致性。
2. 补 P1 核心工具：`capture`、`qr`、完整标注/马赛克/橡皮/文字/序号、撤销重做、颜色/粗细。
3. 明确贴图架构：native PinWindow 完整化，或 native 截图后调用 Go pinnedimage service；不要保留 `PinID="native"` 这种不可追踪结果。
4. 做视觉 QA：对照原 CSS/截图复刻 glass toolbar、magnifier、hint/toast、主题色和 hover/active/disabled 状态。
5. 补验证：Go native path 单元测试、packaging native payload 断言、native host smoke/手工截图证据。
