# Native Capture Migration 第 2 轮独立复审

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 角色：`damo` 第 2 轮独立复审子代理
- 范围：复验第 1 轮剩余 P2：WPF 马赛克预览与最终导出一致性；顺带确认 native 打码、QR、pin service、标注导出、打包路径没有重新引入 P0/P1。
- 约束：只审查；未修改业务代码；未暂存；未提交。

## 审查范围和证据

已读：

- 第 1 轮独立复审：`.codex-audit/native-capture-migration-iteration-01-audit-2026-07-03/audit-notes.md`
- 第 2 轮修改记录：`.codex-audit/native-capture-migration-iteration-02-modification-2026-07-03/audit-fixes.md`
- 本轮主要代码：`native/Ariadne.CaptureHost/OverlayWindow.cs`
- 顺带核验：`native/Ariadne.CaptureHost/ScreenCapture.cs`、`native/Ariadne.CaptureHost/CaptureModels.cs`、`internal/captureoverlay/service.go`、`internal/captureoverlay/service_test.go`、`internal/nativecapture/manager_windows.go`、`internal/releasepack/*`、`internal/msixpack/*`

验证命令：

```powershell
go test ./internal/captureoverlay ./internal/nativecapture ./internal/releasepack ./internal/msixpack ./internal/clipboardhistory
```

结果：通过。

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

```powershell
git diff --check -- native/Ariadne.CaptureHost internal/nativecapture internal/captureoverlay internal/clipboardhistory internal/releasepack internal/msixpack main.go Taskfile.yml frontend/src/components/capture/CaptureOverlayWindow.vue frontend/src/components/pinned/PinnedImageWindow.vue frontend/src/types/ariadne.ts frontend/src/style.css
```

结果：退出码 0；仅有现有 LF 文件将被 Git 触碰为 CRLF 的 warning，未发现 whitespace error。

## 截图清单

无。未做 native host 实机截图、像素级视觉 QA、多 DPI/多屏手工截图验证。本轮马赛克判断基于 WPF 预览和导出代码路径对比，不假装已有实机截图证据。

## 已确认落地的修复

- 第 2 轮已把矩形马赛克预览从深色遮罩改为基于截图像素的低分辨率预览：`native/Ariadne.CaptureHost/OverlayWindow.cs:1414-1456`。这比第 1 轮 P2 指出的“只画半透明深色块”明显接近最终导出。
- 矩形马赛克预览会裁剪当前选区内截图源图并用 `NearestNeighbor` 放大显示：`OverlayWindow.cs:1423-1453`。
- 最终导出仍由 `DrawMosaic` / `DrawMosaicBlocks` 基于同一截图裁剪图绘制：`OverlayWindow.cs:1638-1685`。

## P0

无。

顺带确认未重新引入首轮 P0：native 路径仍在保存、复制、贴图、历史写入等 side effect 前统一处理 `redact_copy` / `AutoRedact`，证据为 `internal/captureoverlay/service.go:509-556`；相关测试仍通过。

## P1

无。

顺带确认：

- QR native 路径仍从历史图解码并复制文本：`internal/captureoverlay/service.go:608-627`，测试通过。
- native pin 仍进入 pinnedimage service，不是假的 `PinID="native"`：`internal/captureoverlay/service.go:588-604`、`service.go:677-692`，测试通过。
- 标注最终导出仍在 native host 内渲染 PNG，并把 operations 作为元数据导出：`native/Ariadne.CaptureHost/OverlayWindow.cs:1510-1534`、`OverlayWindow.cs:1747-1751`；Go 侧没有重新叠加 operations。
- release/msix 打包仍把 `native-capture/Ariadne.CaptureHost.exe` 当 required runtime：`internal/releasepack/package.go:88-92`、`package.go:263-280`、`internal/msixpack/package.go:93-97`、`package.go:330-347`；相关测试通过。

## P2

### P2-1 路径型马赛克预览仍不是最终导出效果，并且新预览有明显交互性能风险

证据：

- 路径型马赛克预览在每个采样点画的是 `size x size` 的预览块：`native/Ariadne.CaptureHost/OverlayWindow.cs:1402-1408`。
- 最终导出同一路径型马赛克在每个点画的是 `block * 2` 宽高的区域：`OverlayWindow.cs:1640-1647`。
- 因此默认 `PixelSize=12` 时，用户拖动时看到的是约 `12 x 12` 的块，但最终 PNG 会落成约 `24 x 24` 的块；这是肉眼可见的所见即所得差距。
- 新预览还在每次重绘时对每个路径点都执行截图裁剪和 `TransformedBitmap` 降采样：`OverlayWindow.cs:1402-1435`。而鼠标移动时 `UpdateDraftAnnotation` 会追加点并重绘，随后 `UpdateSelectionVisuals` 又会再次重绘：`OverlayWindow.cs:530-557`、`OverlayWindow.cs:1009-1024`、`OverlayWindow.cs:699-729`。长一点的马赛克笔画会快速累积为大量 WPF `Border/Image/CroppedBitmap/TransformedBitmap` 对象，存在明显卡顿风险。

影响：

- 矩形马赛克的第 1 轮 P2 已有明显改善，但路径/画笔式马赛克仍会出现预览范围小于最终导出范围的问题。
- 用户用马赛克笔刷遮盖敏感信息时，提交前看到的遮盖范围偏小，复制、保存或贴图后的 PNG 可能比预期遮挡更多内容。
- 长笔画时的 per-point 图像裁剪/缩放会把标注预览从轻量绘制变成高频位图创建，风险集中在最需要连续拖动反馈的交互上。

建议：

- 下一轮只改 `native/Ariadne.CaptureHost/OverlayWindow.cs`，让路径型马赛克预览使用和 `DrawMosaic` 相同的 `block * 2` 覆盖范围，或直接复用统一的绘制 helper。
- 避免每个点创建一个裁剪位图。可以基于选区截图生成一次缓存/Visual，再按最终导出的块网格绘制，或把预览和导出统一到同一套 `DrawingContext` 块绘制逻辑。

## 新问题专项检查

- 坐标边界：矩形预览会先把 `localRect` clamp 到选区，再按 `SelectionPhysical()` 推导物理裁剪区域；代码层面未发现越界异常路径。证据：`OverlayWindow.cs:1414-1431`、`ScreenCapture.cs:45-50`、`ScreenCapture.cs:73-80`。
- 多 DPI / 多屏：Selection DIP 到 physical 的转换仍集中在 `SelectionPhysical()`；未发现本轮马赛克预览新增明显坐标偏移问题，但未做多屏实机验证。证据：`OverlayWindow.cs:2100-2107`。
- native 打码、QR、pin service、标注导出、打包路径：未发现 P0/P1 回退，且本轮验证命令通过。

## 结论

第 2 轮修改把矩形马赛克预览推进到了可接受方向，但第 1 轮剩余 P2 还不能完全关闭：路径型马赛克仍存在明显预览/导出尺寸不一致，且新预览实现引入了高频拖动下的性能风险。

当前未达到“审查不出什么问题来”。建议进入第 3 轮小修改，聚焦路径型马赛克预览覆盖范围和预览绘制性能；不需要扩大到 native 打码、QR、pin service、标注导出或打包路径。
