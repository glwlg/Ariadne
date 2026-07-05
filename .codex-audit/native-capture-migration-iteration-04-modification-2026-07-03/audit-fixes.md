# Native Capture Migration 第 4 轮修改记录

- 日期：2026-07-03
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 范围：路径型马赛克预览 clip union 语义

## 本轮处理的问题

1. P2-1 路径型马赛克预览的重叠区域被 WPF 默认 `EvenOdd` 裁剪抵消。
   - `native/Ariadne.CaptureHost/OverlayWindow.cs` 中路径型马赛克预览的 `GeometryGroup` 显式设置 `FillRule = FillRule.Nonzero`。
   - 重叠路径块在预览中保持填充，不再出现被偶奇规则挖空的空洞。

## 验证

```powershell
$env:DOTNET_ROOT='C:\Users\luwei\.codex\tools\dotnet-sdk-8.0'; $env:PATH="$env:DOTNET_ROOT;$env:PATH"; dotnet build native\Ariadne.CaptureHost\Ariadne.CaptureHost.csproj -c Release
```

结果：通过，0 warning / 0 error。

```powershell
Add-Type -AssemblyName PresentationCore; Add-Type -AssemblyName WindowsBase; $g=[System.Windows.Media.GeometryGroup]::new(); $g.FillRule=[System.Windows.Media.FillRule]::Nonzero; ...
```

结果：`fillRule=Nonzero; overlap=True`。

## 残余风险

- 未做实机截图/像素级 QA；本轮验证为代码审查、WPF 编译和 WPF geometry 行为探针。
