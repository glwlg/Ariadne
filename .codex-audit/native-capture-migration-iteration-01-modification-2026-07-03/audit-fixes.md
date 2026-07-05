# Native Capture Migration 第 1 轮修改记录

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 角色：`damo` 第 1 轮修改
- 范围：Go/native 管道、WPF 覆盖层、测试、release/msix 打包断言
- 未修改范围：`frontend/*`、`main.go`、`Taskfile.yml`

## 本轮处理的问题

1. P0-1 native 路径绕过 OCR 打码/脱敏。
   - `captureNativeSelection` 现在会在保存、自动保存、复制、贴图、写历史前，根据 `redact_copy` 或 `policy.AutoRedact` 调用现有 OCR redaction pipeline。
   - `redact_copy` 会强制开启打码策略；`AutoRedact` 对 `capture/copy/pin/save_as` 生效；`qr` 不打码。
   - 写入截图历史和另存文件的 PNG 都使用已处理后的字节，不再直接保存 native 原始 PNG。

2. P1-2 native QR Go 管道缺失。
   - native response `Action="qr"` 时，Go 会先写入截图历史，再调用 `qrscan.DecodeImagePath`。
   - 返回的 `CaptureResult.QR` 填充 `Source/CaptureID/ImagePath/Width/Height`，行为与原 `CaptureSelection` 对齐。

3. P1-4 native 贴图返回不可管理的 `PinID="native"`。
   - native pin/AutoPin 现在在截图历史 entry 创建后调用 `s.openPinnedSelection(entry.ID, ...)`。
   - 返回的 pin id 来自 pinnedimage service，不再构造假的 `native` pin id。
   - Go response 已接入 native `X/Y`、`PinPositioned/PinX/PinY` 字段；当前 WPF completion 尚未填 `X/Y`，见残余风险。

4. P1-6 native `Open()` 缺并发打开保护。
   - native 分支现在复用 `tryBeginOpen/finishOpen`。
   - 连按热键时会在启动 native host 前返回“截图覆盖层正在打开”。

5. P2-4 manager 对 host/pipe 异常缺少重启重试，Capture 未串行。
   - `nativecapture.Manager.Capture` 现在持有 manager mutex，串行化 Capture 请求。
   - round trip 失败且调用上下文未超时时，会丢弃旧进程、重启 host，并对同一请求重试一次。
   - pipe 连接设置 deadline，避免读写无期限挂起。

6. P2-3 release/msix 对 native host 漏包没有测试保护。
   - release zip 和 msix layout 现在要求 `native-capture/Ariadne.CaptureHost.exe` 存在；缺失时构建失败。
   - 测试断言 manifest/layout/zip 中包含 native host payload，并新增缺失 native host 的失败用例。

7. P1-1/P1-2/P1-3/P1-5 WPF 覆盖层缺核心工具和视觉复刻。
   - WPF 覆盖层补齐保存历史、复制、打码复制、贴图、扫码、另存、重新选择入口。
   - 补齐矩形、直线、箭头、画笔、荧光笔、马赛克、文字、序号、橡皮擦、选择/移动标注、撤销、重做、清空、删除。
   - 增加颜色条、粗细滑杆、放大镜、RGB/HEX 取色、反馈 toast、浅色 glass toolbar、选区尺寸标签和可换行 hint。
   - 完成时返回 `x/y/operations`，PNG 已包含用户看到的标注结果，Go 侧继续用 operations 写历史 tags。

8. native QR 成功后缺少文本复制动作。
   - 原 WebView 路径由前端复制二维码文本；native 路径没有前端回调，因此 Go 侧在 native QR 成功后写入系统文本剪贴板。
   - 新增 `clipboardhistory.WriteTextToSystemClipboard` 并补 native QR 文本复制测试。

## 修改文件

- `internal/captureoverlay/service.go`
- `internal/captureoverlay/service_test.go`
- `internal/nativecapture/manager_windows.go`
- `internal/nativecapture/manager_other.go`
- `internal/releasepack/package.go`
- `internal/releasepack/package_test.go`
- `internal/msixpack/package.go`
- `internal/msixpack/package_test.go`
- `internal/clipboardhistory/service.go`
- `internal/clipboardhistory/clipboard_windows.go`
- `internal/clipboardhistory/clipboard_other.go`
- `native/Ariadne.CaptureHost/CaptureModels.cs`
- `native/Ariadne.CaptureHost/NativeVisuals.cs`
- `native/Ariadne.CaptureHost/ScreenCapture.cs`
- `native/Ariadne.CaptureHost/OverlayWindow.cs`
- `.codex-audit/native-capture-migration-iteration-01-modification-2026-07-03/audit-fixes.md`

## 修改前后行为

- 修改前：native PNG 解码后直接另存、复制或返回假贴图；AutoRedact/redact_copy 不会保护 native 主路径。
- 修改后：native PNG 先通过现有 OCR 打码逻辑处理，再进入历史、另存、复制和贴图流程；失败时明确返回“自动打码失败”。
- 修改前：native QR action 没有 Go 侧解码和 `CaptureResult.QR`。
- 修改后：native QR action 写历史、返回二维码解码结果，并复制二维码文本。
- 修改前：native pin 返回 `PinID="native"`，pinnedimage service 不可追踪。
- 修改后：native pin 通过 pinnedimage service 打开并返回真实 pin 结果。
- 修改前：release/msix 缺 `native-capture` 目录会静默漏包。
- 修改后：标准包缺 native host 会明确失败。
- 修改前：WPF 覆盖层只有复制、贴图、另存、取消。
- 修改后：WPF 覆盖层具备原截图工具面和更接近原 CSS 的浅色 glass 视觉。

## 验证命令和结果

```powershell
go test ./internal/captureoverlay ./internal/nativecapture ./internal/releasepack ./internal/msixpack
```

结果：通过。

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

```powershell
go test ./internal/captureoverlay ./internal/nativecapture ./internal/releasepack ./internal/msixpack ./internal/clipboardhistory
```

结果：通过。

```powershell
wails3 task native:capture-host
wails3 task windows:build
```

结果：通过；frontend build 仍有既有 `@vueuse/core` Rolldown `INVALID_ANNOTATION` warning，但命令退出 0。

```powershell
go run ./cmd/releasepack -version dev
```

结果：通过；生成 `dist/release/AriadneSetup-dev-windows-x64.exe`，大小 89,147,392 bytes，SHA256 `d4d35f1d64201e89f44e33855f32bbc2fed68616e32380854371bc2fb805d5d7`；包清单包含 `app/native-capture/Ariadne.CaptureHost.exe`，大小 71,614,860 bytes，SHA256 `fb0b76f1784531a1c733e567be7bd4dc4b13355c26ccf7242c16b07e4a6e4426`。

```powershell
bin\native-capture\Ariadne.CaptureHost.exe --pipe <smoke-pipe>
```

结果：named pipe `ping` 返回 `ready`，`shutdown` 返回 `closing`，host 正常退出。

```powershell
git diff --check -- internal/captureoverlay/service.go internal/captureoverlay/service_test.go internal/nativecapture/manager_windows.go internal/nativecapture/manager_other.go internal/releasepack/package.go internal/releasepack/package_test.go internal/msixpack/package.go internal/msixpack/package_test.go
```

结果：通过；仅 PowerShell/Git 输出工作区 LF 将来可能被替换为 CRLF 的提示。

## 浏览器验证

未运行 browser。原因：本轮主路径是 native WPF/Win32 host，浏览器无法提供截图覆盖层实机交互证据；本轮验证以 WPF 编译、Go 单元测试和后续 Windows 构建为准。

## 未处理事项和残余风险

- `Open()` 的公开返回类型仍是 `OpenResult`；native QR 的文本复制已在 Go 内部完成，因此普通热键路径不再依赖前端消费 `CaptureResult.QR`。
- 本轮尚未做实机像素截图 QA；已完成代码和构建验证，仍需要复审代理确认视觉/交互是否还有 P1/P2。

## Git

未暂存、未提交、未推送。
