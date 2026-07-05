# Native Capture Migration 第 1 轮独立复审

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 角色：`damo` 第 1 轮独立复审子代理
- 范围：截图功能迁移到 native WPF/Win32 host；不引入 WinUI 3；Windows 截图主路径避免 WebView；样式和功能尽量复刻原 WebView/Vue 截图体验。
- 约束：只审查；未修改业务代码；未暂存；未提交。

## 审查范围和证据

已读：

- 首轮审查：`.codex-audit/native-capture-migration-iteration-00-audit-2026-07-03/audit-notes.md`
- 第 1 轮修改记录：`.codex-audit/native-capture-migration-iteration-01-modification-2026-07-03/audit-fixes.md`
- 当前 WPF：`native/Ariadne.CaptureHost/CaptureModels.cs`、`OverlayWindow.cs`、`ScreenCapture.cs`、`NativeVisuals.cs`、`PinWindow.cs`、`HostServer.cs`
- Go 接入：`internal/nativecapture/*`、`internal/captureoverlay/service.go`、`internal/captureoverlay/service_test.go`、`internal/clipboardhistory/*`、`internal/releasepack/*`、`internal/msixpack/*`、`main.go`、`Taskfile.yml`
- 原体验参考：`frontend/src/components/capture/CaptureOverlayWindow.vue`、`frontend/src/components/pinned/PinnedImageWindow.vue`、`frontend/src/types/ariadne.ts`、`frontend/src/style.css` 相关段落

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

结果：通过；仅有 Git 提示若 Git 触碰这些 LF 文件会替换为 CRLF。

## 截图清单

无。未做 native host 实机截图、像素级视觉 QA、多 DPI/多屏手工截图验证。本轮视觉判断只基于 WPF 代码和原 Vue/CSS 代码对比，不假装做了实机视觉 QA。

## 已确认落地的修复

- P0 脱敏主路径已修：`captureNativeSelection` 先解码 native PNG，再在 `redactionPolicyForAction` 命中时调用 `redactSelectionPNG`，之后才执行另存、自动保存、复制、贴图和写历史。证据：`internal/captureoverlay/service.go:509-556`。
- `redact_copy` 与 AutoRedact 有 native 测试覆盖：`save_as + AutoRedact` 会验证历史图和外部另存图像素已打码，`redact_copy` 会验证剪贴板路径使用已打码图片。证据：`internal/captureoverlay/service_test.go:473-526`。覆盖未显式枚举 native `AutoRedact + pin/copy`，但实现顺序在所有非 QR side effects 前统一打码。
- QR 已接入 Go 管道并复制文本：native `Action="qr"` 写历史后调用 `qrscan.DecodeImagePath`，成功时用 `WriteTextToSystemClipboard` 写系统剪贴板。证据：`internal/captureoverlay/service.go:608-627`、`internal/clipboardhistory/service.go:39-45`、`internal/clipboardhistory/clipboard_windows.go:372-405`、`internal/captureoverlay/service_test.go:528-558`。
- 贴图不再返回假 `PinID="native"`：native pin/AutoPin 经 `openPinnedSelection` 进入 pinnedimage service，并用 native response 的 X/Y 定位。证据：`internal/captureoverlay/service.go:588-604`、`internal/captureoverlay/service.go:633-644`、`internal/captureoverlay/service.go:677-692`、`internal/captureoverlay/service_test.go:560-582`。
- 并发打开保护已补：native `Open()` 分支复用 `tryBeginOpen/finishOpen`。证据：`internal/captureoverlay/service.go:201-210`、`internal/captureoverlay/service_test.go:427-456`。
- WPF 覆盖层已补齐 capture/copy/redact_copy/pin/qr/save_as、标注工具、撤销重做、颜色、粗细、放大镜、反馈 toast、导出 PNG。证据：`native/Ariadne.CaptureHost/OverlayWindow.cs:302-375`、`OverlayWindow.cs:580-697`、`OverlayWindow.cs:1456-1510`、`OverlayWindow.cs:1513-1543`、`OverlayWindow.cs:1545-1827`。
- 打包漏包风险已修：release/msix 使用 required native-capture 目录并测试缺失 host 失败。证据：`internal/releasepack/package.go:88-92`、`internal/releasepack/package.go:263-303`、`internal/msixpack/package.go:93-97`、`internal/msixpack/package.go:330-370`。
- native host 生命周期比首轮稳：manager 串行 `Capture`，round trip 失败后丢弃旧进程、重启并重试一次。证据：`internal/nativecapture/manager_windows.go:103-134`、`manager_windows.go:198-230`。
- 未引入 WinUI 3：WPF host 仍是 `net8.0-windows` + WPF，主入口在 Windows 上构造 native manager。证据：`native/Ariadne.CaptureHost/Ariadne.CaptureHost.csproj`、`main.go:164-168`。

## P0

无。

首轮 P0 已解决。native 路径在保存、复制、贴图和历史写入前统一处理 `redact_copy` / `AutoRedact`；QR 路径按预期跳过打码。

## P1

无。

首轮 P1 的核心功能面已基本补齐：动作集合、标注集合、撤销重做、颜色/粗细、QR、pin service 接入、并发保护和打包断言都有代码与测试/编译证据。样式代码也已经不是裸 WPF：有浅色 glass toolbar、阴影、圆角、放大镜、hint、反馈 toast、选区尺寸和两行工具条。

## P2

### P2-1 WPF 马赛克预览和最终导出不一致，复刻原体验仍有可见差距

证据：

- 原 Vue 矩形马赛克预览会在选区内放置截图图片并通过缩放形成像素化预览，用户在提交前看到的就是接近最终效果的马赛克区域。证据：`frontend/src/components/capture/CaptureOverlayWindow.vue:1771-1784`、`CaptureOverlayWindow.vue:1566-1588`。
- 当前 WPF 预览阶段只画半透明深色块或深色小方块：`native/Ariadne.CaptureHost/OverlayWindow.cs:1400-1433`。
- 当前 WPF 最终导出时才调用 `DrawMosaicBlocks` 对截图像素采样生成真正马赛克：`native/Ariadne.CaptureHost/OverlayWindow.cs:1576-1579`、`OverlayWindow.cs:1614-1669`。

影响：

- 用户用马赛克遮盖内容时，屏幕预览不是最终导出效果；复制、保存、贴图后的 PNG 会和操作时看到的遮盖形态不同。它不阻断主流程，但削弱“所见即所得”和“尽量复刻原 WebView/Vue 截图体验”。

建议：

- WPF 预览层复用导出同一套马赛克绘制逻辑，或在 `_annotationLayer` 中用裁剪后的 `ImageBrush/VisualBrush` 加 nearest-neighbor scale 生成像素化预览；路径型马赛克也应尽量显示实际像素化块，而不是深色遮罩。

## 新问题专项检查

- WPF 渲染 PNG 与 Go side effects：native host 返回已渲染 PNG，Go 侧不再重新应用 operations，只用 operations 写历史 tags；没有发现双重标注。证据：`native/Ariadne.CaptureHost/OverlayWindow.cs:1497-1510`、`internal/captureoverlay/service.go:496-556`、`service.go:1643-1657`。
- 双贴图：未发现。`PinWindow` 目前没有在截图完成路径中实例化，native pin 进入 Go pinnedimage service。证据：`rg "new PinWindow"` 无调用；`internal/captureoverlay/service.go:588-604`。
- QR 复制：已修，Go 侧成功解码后写文本剪贴板。证据同上。
- 坐标错位：代码层面未发现明显错误。WPF response 返回物理屏幕 X/Y，Go pin 使用该位置并在有 app screen 时转换为 DIP。证据：`OverlayWindow.cs:1503-1506`、`internal/captureoverlay/service.go:633-644`、`service.go:677-690`。未做多屏实机验证。
- toolbar 小屏溢出：WPF 使用两行 `WrapPanel`，并按窗口宽度设置最大宽度和上下避让。证据：`OverlayWindow.cs:302-375`、`OverlayWindow.cs:737-753`。未做小屏实机截图。
- host process 残留：manager 有 `Stop()` shutdown/kill，失败 round trip 会 discard + restart；未发现新增明显残留问题。证据：`internal/nativecapture/manager_windows.go:137-158`、`manager_windows.go:198-214`。
- 打包漏包：已修为 required native host。证据同上。

## 结论

第 1 轮修改已经解决首轮 P0/P1 主问题，代码层面基本达到“不引入 WinUI 3、Windows 截图覆盖层主路径避免 WebView、功能大体复刻原 Vue 截图体验”的目标。

但仍有一个有意义 P2：WPF 马赛克预览不是最终像素化效果，和原 Vue 的所见即所得预览有差距。因此当前不建议写“无有意义 P0/P1/P2，允许收敛”；建议进入下一轮小修改，聚焦修复马赛克预览一致性。实机视觉 QA 仍是未验证残余风险。
