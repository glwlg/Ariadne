# 网络小窗

Windows 网络小窗由 `Fancun.CaptureHost` 的 WPF 窗口显示，与截图共用原生进程，不创建 WebView。其他平台保留现有窗口实现。

网卡计数和速率计算继续使用 Go 的网络监控服务。小窗可见时每秒采集一次，界面只保留当前数值，不缓存历史记录。独立的 `<capture-pipe>-network-mini` 命名管道传递当前窗口位置、可见状态和流量，并带回打开进程网络中心或关闭小窗的操作。截图等待用户操作不会阻塞此管道。

贴边位置、多屏选择、全屏隐藏和启动恢复继续由窗口服务管理，沿用 `fancun.sqlite` 中的网络小窗配置。截图期间临时隐藏；在截图期间主动关闭后，不会自动重新打开。发送到原生窗口的坐标和尺寸使用物理像素；小窗不激活、不显示任务栏按钮。

原生组件或通信发生错误时，窗口服务返回具体错误，不创建 WebView 作为替代。

## 验证

```powershell
go test -count=1 ./internal/toolwindows ./internal/nativecapture ./internal/networkmonitor
dotnet run --project native/Fancun.CaptureHost.Tests/Fancun.CaptureHost.Tests.csproj -c Release
wails3 task native:capture-host
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-native-network-mini.ps1 `
  -ExePath bin/native-capture/Fancun.CaptureHost.exe `
  -DurationSeconds 120 `
  -OutputPath dist/perf/network-mini-native.json
```

界面测试启动独立管道和原生进程，不启动或替换已安装的 Fancun，不修改用户数据、快捷方式或自启动设置。检查物理坐标、速率显示、不抢焦点、双击、右键关闭、显示和隐藏、截图管道阻塞期间的独立通信，以及测试进程没有 WebView 子进程。每十秒记录工作集、私有内存和句柄数；两分钟采样只能说明该测试时段的表现，不能替代长时间运行验证。
