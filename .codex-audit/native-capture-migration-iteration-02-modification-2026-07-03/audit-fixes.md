# Native Capture Migration 第 2 轮修改记录

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 范围：WPF 马赛克预览一致性

## 本轮处理的问题

1. P2-1 WPF 马赛克预览和最终导出不一致。
   - 修改 `native/Ariadne.CaptureHost/OverlayWindow.cs` 的预览层。
   - 矩形马赛克和路径马赛克预览现在从实际截图选区裁剪源图，按块大小降采样，再用 nearest-neighbor 放大显示。
   - 预览不再使用半透明深色遮罩，视觉上更接近最终导出的像素化结果。

## 验证

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

## 残余风险

- 未做实机截图/像素级 QA；本轮验证为代码审查和 WPF 编译。
- 马赛克预览与导出现在同样基于截图像素，但预览使用 WPF 降采样显示，导出使用 DrawingContext 块采样，两者仍可能有细小采样差异。
