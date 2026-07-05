# Native Capture Migration 第 3 轮独立复审

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 角色：`damo` 第 3 轮独立复审子代理
- 范围：复验第 2 轮剩余 P2：路径型马赛克预览覆盖范围和预览绘制性能；顺带确认 native 打码、QR、pin service、标注导出、打包路径没有回退。
- 约束：只审查；未修改业务代码；未暂存；未提交。

## 审查范围和证据

已读：

- 第 2 轮独立复审：`.codex-audit/native-capture-migration-iteration-02-audit-2026-07-03/audit-notes.md`
- 第 3 轮修改记录：`.codex-audit/native-capture-migration-iteration-03-modification-2026-07-03/audit-fixes.md`
- 本轮主要代码：`native/Ariadne.CaptureHost/OverlayWindow.cs`
- 顺带核验：`native/Ariadne.CaptureHost/ScreenCapture.cs`、`internal/captureoverlay/service.go`、`internal/releasepack/package.go`、`internal/msixpack/package.go`、`Taskfile.yml`

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
git diff --check -- native/Ariadne.CaptureHost internal/nativecapture internal/captureoverlay internal/clipboardhistory internal/releasepack internal/msixpack main.go Taskfile.yml frontend/src/components/capture/CaptureOverlayWindow.vue frontend/src/components/pinned/PinnedImageWindow.vue frontend/src/types/ariadne.ts frontend/src/style.css
```

结果：退出码 0；仅有现有 LF 文件将被 Git 触碰为 CRLF 的 warning，未发现 whitespace error。

```powershell
Add-Type -AssemblyName PresentationCore; Add-Type -AssemblyName WindowsBase; $g = [System.Windows.Media.GeometryGroup]::new(); $g.Children.Add([System.Windows.Media.RectangleGeometry]::new([System.Windows.Rect]::new(0,0,10,10))); $g.Children.Add([System.Windows.Media.RectangleGeometry]::new([System.Windows.Rect]::new(5,0,10,10))); "default=$($g.FillRule); p3=$($g.FillContains([System.Windows.Point]::new(3,5))); p7=$($g.FillContains([System.Windows.Point]::new(7,5))); p12=$($g.FillContains([System.Windows.Point]::new(12,5)))"; $g.FillRule = [System.Windows.Media.FillRule]::Nonzero; "nonzero=$($g.FillRule); p3=$($g.FillContains([System.Windows.Point]::new(3,5))); p7=$($g.FillContains([System.Windows.Point]::new(7,5))); p12=$($g.FillContains([System.Windows.Point]::new(12,5)))"
```

结果：`default=EvenOdd; p3=True; p7=False; p12=True`，改为 `Nonzero` 后 `p7=True`。

`git diff --name-only --cached` 结果为空，确认未暂存。

## 截图清单

无。未做 native host 实机截图、像素级视觉 QA、多 DPI/多屏手工截图验证。本轮判断基于 WPF 预览/导出代码路径对比、WPF `GeometryGroup` 行为探针、编译和测试。

## 已确认落地的修复

- 路径型马赛克预览单点覆盖范围已从 `size x size` 改为与导出一致的 `block * 2`：`native/Ariadne.CaptureHost/OverlayWindow.cs:1417-1427` 对应最终导出 `OverlayWindow.cs:1670-1676`。
- 第 2 轮指出的 per-point 位图创建风险已基本消除：`AddMosaicPreviewPath` 在点循环里只收集 `Rect` 和 `RectangleGeometry`，随后只调用一次 `AddMosaicPreviewRect(union, ...)`；截图裁剪和 `TransformedBitmap` 创建集中在 `OverlayWindow.cs:1451-1463`，不再按每个采样点创建。

## P0

无。

native 打码路径未回退：`internal/captureoverlay/service.go:509-556` 仍在保存、复制、贴图、历史写入等 side effect 前处理 `redact_copy` / `AutoRedact`；相关 Go 测试通过。

## P1

无。

顺带确认：

- QR native 路径仍从历史图解码并复制文本：`internal/captureoverlay/service.go:608-620`，测试通过。
- native pin 仍进入 pinnedimage service，不是假的固定 ID：`internal/captureoverlay/service.go:588-604`、`service.go:677-692`，测试通过。
- 标注最终导出仍在 native host 内渲染 PNG，并把 operations 作为元数据导出：`native/Ariadne.CaptureHost/OverlayWindow.cs:1509-1563`、`OverlayWindow.cs:1574-1584`、`OverlayWindow.cs:1776-1780`。
- release/msix 打包仍把 `native-capture/Ariadne.CaptureHost.exe` 当 required runtime：`internal/releasepack/package.go:88-92`、`package.go:263-280`、`internal/msixpack/package.go:93-97`、`package.go:330-347`；相关测试通过。

## P2

### P2-1 路径型马赛克预览的重叠区域会被 WPF 默认 EvenOdd 裁剪抵消，仍可能小于最终导出覆盖范围

证据：

- 新实现用默认 `GeometryGroup` 作为路径型马赛克预览的 `Clip`：`native/Ariadne.CaptureHost/OverlayWindow.cs:1433-1438`。代码没有设置 `FillRule`。
- WPF `GeometryGroup` 默认 `FillRule` 为 `EvenOdd`。本轮探针验证两个重叠矩形时，重叠点 `p7` 在默认规则下 `FillContains=False`；改成 `Nonzero` 后同一点 `FillContains=True`。
- 路径型马赛克连续绘制时会高频追加采样点：`OverlayWindow.cs:1016-1019`、`OverlayWindow.cs:1810-1815`。每个点的预览矩形宽高是 `block * 2`，相邻点非常容易重叠：`OverlayWindow.cs:1417-1427`。
- 最终导出不是用 `GeometryGroup` 裁剪，而是逐点调用 `DrawMosaicBlocks` 画矩形：`OverlayWindow.cs:1670-1676`。重叠区域会被后续块覆盖，不会被抵消。

影响：

- 第 3 轮修改解决了块尺寸和 per-point 位图创建，但新裁剪组合方式会让路径型马赛克预览在重叠区域出现空洞或断续。
- 用户看到的预览覆盖范围仍可能小于最终导出，尤其是连续拖动时的密集采样路径。这延续了第 2 轮 P2 的核心所见即所得问题。
- 最终导出仍会覆盖这些区域，未发现数据泄漏风险；因此评级为 P2，而非 P0/P1。

建议：

- 下一轮只改 `native/Ariadne.CaptureHost/OverlayWindow.cs`。
- 在 `GeometryGroup` 上显式设置 `FillRule = FillRule.Nonzero`，或改用真正的 union 几何组合，确保重叠路径块在预览中保持填充。
- 保留当前“每条路径只裁剪/降采样一次”的方向，不要退回 per-point 位图创建。

## 新问题专项检查

- 坐标边界：路径预览先把每个点的 `block * 2` 矩形 intersect 到选区，再对 union 裁剪源图；`ScreenCapture.CropSource` 也会 clamp 物理裁剪矩形。未发现越界异常路径。
- 多 DPI / 多屏：本轮没有改 `SelectionPhysical()` 和 DIP/physical 入口；未发现新的明显坐标偏移风险，但未做多屏实机验证。
- 预览性能：per-point `CroppedBitmap/TransformedBitmap` 已消除；剩余风险是长路径每次重绘仍会重建一次 union bitmap 和一组矩形几何，但未达到本轮应拦截的明显 P2。
- native 打码、QR、pin service、标注导出、打包路径：未发现回退，验证命令通过。

## 结论

当前未达到“审查不出什么问题来”。

第 2 轮 P2 已部分解决：路径块尺寸和 per-point 位图创建问题已收敛。但第 3 轮新实现使用默认 `GeometryGroup.FillRule=EvenOdd`，会在重叠路径块处产生预览空洞，导致路径型马赛克预览覆盖范围仍可能小于最终导出。

建议进入第 4 轮小修改，聚焦 `OverlayWindow.cs` 的路径型马赛克 clip union 语义；不需要扩大到 native 打码、QR、pin service、标注导出或打包路径。
