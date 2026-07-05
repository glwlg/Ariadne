# Native Capture Migration 第 3 轮修改记录

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 范围：路径型马赛克预览一致性和交互性能

## 本轮处理的问题

1. P2-1 路径型马赛克预览小于最终导出范围。
   - `native/Ariadne.CaptureHost/OverlayWindow.cs` 中路径型马赛克预览改为使用与导出一致的 `block * 2` 覆盖范围。
   - 预览不再画 `size x size` 小块，避免提交后遮挡面积变大的所见即所得差异。

2. 路径型马赛克预览 per-point 裁剪/缩放导致拖动卡顿风险。
   - 新增路径预览合并逻辑：先计算所有路径块的 union，再裁剪一次截图源图、降采样一次。
   - 使用相对 `GeometryGroup` 裁剪出每个路径块，避免每个采样点创建一个 `CroppedBitmap/TransformedBitmap`。

## 验证

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

## 残余风险

- 未做实机截图/像素级 QA；本轮验证为代码审查和 WPF 编译。
