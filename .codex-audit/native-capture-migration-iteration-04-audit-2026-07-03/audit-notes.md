# Native Capture Migration 第 4 轮独立复审

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 角色：`damo` 第 4 轮独立复审子代理
- 范围：复验第 3 轮剩余 P2：路径型马赛克预览 `GeometryGroup` 重叠裁剪语义；确认第 2/3 轮路径型马赛克覆盖范围、per-point 位图创建、预览/导出一致性已收敛；顺带确认 native 打码、QR、pin service、标注导出、打包路径没有回退。
- 约束：只审查；未修改业务代码；未暂存；未提交。

## 审查范围和证据

已读：

- 第 2 轮独立复审：`.codex-audit/native-capture-migration-iteration-02-audit-2026-07-03/audit-notes.md`
- 第 3 轮独立复审：`.codex-audit/native-capture-migration-iteration-03-audit-2026-07-03/audit-notes.md`
- 第 3 轮修改记录：`.codex-audit/native-capture-migration-iteration-03-modification-2026-07-03/audit-fixes.md`
- 第 4 轮修改记录：`.codex-audit/native-capture-migration-iteration-04-modification-2026-07-03/audit-fixes.md`
- 本轮主要代码：`native/Ariadne.CaptureHost/OverlayWindow.cs`
- 顺带核验：`internal/captureoverlay/service.go`、`internal/nativecapture/manager_windows.go`、`internal/releasepack/package.go`、`internal/msixpack/package.go`、`Taskfile.yml`

验证命令：

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

```powershell
go test ./internal/captureoverlay ./internal/nativecapture ./internal/releasepack ./internal/msixpack ./internal/clipboardhistory
```

结果：通过。

```powershell
Add-Type -AssemblyName PresentationCore; Add-Type -AssemblyName WindowsBase; ... GeometryGroup overlap probe ...
```

结果：`default=EvenOdd; overlap=False`，显式 `Nonzero` 后 `nonzero=Nonzero; overlap=True`。

```powershell
git diff --check -- native/Ariadne.CaptureHost internal/nativecapture internal/captureoverlay internal/clipboardhistory internal/releasepack internal/msixpack main.go Taskfile.yml frontend/src/components/capture/CaptureOverlayWindow.vue frontend/src/components/pinned/PinnedImageWindow.vue frontend/src/types/ariadne.ts frontend/src/style.css
```

结果：退出码 0；仅有现有 LF 文件将被 Git 触碰为 CRLF 的 warning，未发现 whitespace error。

```powershell
git diff --name-only --cached
```

结果：空，确认未暂存。

## 截图清单

无。未做 native host 实机截图、像素级视觉 QA、多 DPI/多屏手工截图验证。本轮判断基于 WPF 预览/导出代码路径对比、WPF `GeometryGroup` 行为探针、编译和测试。

## 已确认落地的修复

- 第 4 轮已在路径型马赛克预览的 `GeometryGroup` 上显式设置 `FillRule = FillRule.Nonzero`：`native/Ariadne.CaptureHost/OverlayWindow.cs:1433`。
- WPF 探针确认默认 `EvenOdd` 会让重叠矩形的交叠点 `FillContains=False`，显式 `Nonzero` 后交叠点 `FillContains=True`。因此第 3 轮指出的“重叠路径块被偶奇规则抵消，预览出现空洞”已关闭。
- 路径型马赛克预览仍保持第 3 轮的收敛方向：每个路径点使用与导出一致的 `block * 2` 范围：`OverlayWindow.cs:1417-1426` 对应最终导出 `OverlayWindow.cs:1670-1676`。
- per-point 位图创建没有回退：路径点循环只收集 `Rect` 和 `RectangleGeometry`，随后只调用一次 `AddMosaicPreviewRect(union, ...)`；截图裁剪和 `TransformedBitmap` 仍集中在 `OverlayWindow.cs:1451-1463`。
- 标注导出路径仍在 native host 内把当前选区和 operations 渲染为 PNG：`OverlayWindow.cs:1509-1563`、`OverlayWindow.cs:1566-1588`；Go 侧接收 native 已渲染 PNG 后再执行保存、复制、贴图和 QR side effect。

## P0

无。

native 打码没有回退：`internal/captureoverlay/service.go:509-518` 仍在 native response 进入保存、复制、贴图、历史写入前处理 `redact_copy` / `AutoRedact`，并在 `service.go:521-566` 记录 redacted side effect 和消息。

## P1

无。

顺带确认：

- QR native 路径仍从历史图解码并复制文本：`internal/captureoverlay/service.go:608-627`，相关测试通过。
- native pin 仍进入 pinnedimage service，不是固定假 ID：`internal/captureoverlay/service.go:588-604`、`service.go:677-692`，相关测试通过。
- native capture host 路径仍定位到 `native-capture/Ariadne.CaptureHost.exe`：`internal/nativecapture/manager_windows.go:241-246`。
- release/msix 打包仍把 `native-capture/Ariadne.CaptureHost.exe` 当 required runtime：`internal/releasepack/package.go:88-92`、`package.go:263-282`、`internal/msixpack/package.go:93-97`、`package.go:330-350`。
- `Taskfile.yml` 仍有 `native:capture-host` 发布任务，并输出到 `bin\native-capture`：`Taskfile.yml:24-28`。

## P2

无。

第 2/3 轮关于路径型马赛克的 P2 已收敛：

- 覆盖范围：预览路径块和导出路径块都使用 `point.X - block, point.Y - block, block * 2, block * 2`。
- 性能：预览没有退回每个采样点各自裁剪、缩放一张位图。
- 重叠语义：`GeometryGroup.FillRule=Nonzero` 避免重叠区域被默认 `EvenOdd` 抵消，预览不再因为 clip 规则出现空洞。

## 新问题专项检查

- 未发现新的有意义 P0/P1/P2。
- 仍未覆盖实机截图、像素级 QA、多 DPI/多屏手工验证；这是可接受残余风险，不影响本轮代码级收敛判断。
- 当前工作树本来是大范围 dirty，本轮只新增本审查文档；未修改业务代码、未暂存、未提交。

## 结论

无有意义 P0/P1/P2，允许收敛。

第 4 轮修改关闭了第 3 轮最后一个 P2：路径型马赛克预览的 `GeometryGroup` 已显式 `Nonzero`，重叠区域不会再被默认 `EvenOdd` 挖空。native 打码、QR、pin service、标注导出、打包路径未发现回退。
