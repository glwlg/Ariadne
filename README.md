# 方寸 Fancun

<p align="center">
  <img src="assets/logo.png" width="96" height="96" alt="方寸 Fancun">
</p>

<p align="center">
  <strong>面向 Windows 的本地效率入口：命令启动器、截图/剪贴板历史和桌面工具中心。</strong>
</p>

<p align="center">
  <a href="https://github.com/glwlg/Ariadne/releases"><img alt="下载" src="https://img.shields.io/badge/Download-Windows%20x64-0f766e"></a>
  <img alt="平台" src="https://img.shields.io/badge/Windows-10%2F11-2563eb">
  <img alt="Wails" src="https://img.shields.io/badge/Wails-3.0.0-8b5cf6">
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26-00add8">
</p>

## 简介

方寸（Fancun）是一个轻量的 Windows 桌面效率工具。它把常用启动、文件搜索、截图历史、剪贴板历史、Hosts 管理、JSON 对比、网络监控和 API 调试等能力放到统一入口里，用快捷键快速呼出，并尽量把数据保留在本机。

工作记忆中心已拆出到同级目录 `../Ariadne-work-memory`，当前应用保留截图、贴图 OCR 和对应模型配置。

## 功能亮点

| 模块 | 能力 |
| --- | --- |
| 命令启动器 | `Alt+Q` 呼出搜索框，聚合应用、文件、插件、工具中心和历史记录。 |
| 截图历史 | 屏幕/窗口/区域截图、标注、OCR、二维码识别、置顶图片和历史检索。 |
| 剪贴板历史 | 文本和图片剪贴板监听、搜索、复制回写、OCR、二维码识别和截图归档。 |
| 工具中心 | Hosts 管理、JSON 对比、网络监控、工作流宏、设置中心和 API 调试窗口。 |
| iPhone 投屏 | 启动器输入 `投屏` 或 `airplay` 打开独立工具窗口，集中管理接收开关、连接状态、配对码、帧率、网络授权和日志。 |
| 本地数据 | 运行数据写入 `%APPDATA%\Fancun\fancun.sqlite`，安装目录和用户数据目录分离。 |
| 发布包 | 用户级安装器，支持安装界面、安装位置、快捷方式、自启动、卸载和内置文件索引。 |

## 快速安装

1. 打开 [Releases](https://github.com/glwlg/Ariadne/releases)。
2. 下载最新的 `FancunSetup-dev-windows-x64.exe`。
3. 双击安装器，阅读用户协议，按需选择安装位置、桌面快捷方式和随 Windows 启动。安装器始终创建开始菜单入口和卸载入口。

静默安装和升级同样会创建或修复开始菜单入口。`--no-desktop-shortcut` 只跳过桌面快捷方式；
旧参数 `--no-shortcuts` / `-NoShortcuts` 现在也只跳过桌面快捷方式，不再关闭开始菜单入口。
显式禁用开始菜单入口的参数会报错，不执行安装。

`fancun-dev-windows-x64.zip` 是排查用 payload，正常安装优先使用 setup exe。

默认安装到：

```text
%LOCALAPPDATA%\Programs\Fancun
```

用户数据默认保存在：

```text
%APPDATA%\Fancun
```

卸载时从开始菜单运行 `Uninstall Fancun`。

## 从 Ariadne 迁入

Fancun 安装器在启动新后台服务前迁移旧数据；直接运行程序时也会检查迁移状态。配置、数据库、截图、剪贴板记录和安全凭据复制到 Fancun，旧目录和旧凭据保留。截图保存目录及记录中的文件路径会同步更新，旧版回滚检查点仍可在 Fancun 中恢复。

安装升级会停止旧版程序并移除旧 `AriadneFileSearch` 服务注册，使用新的 `FancunFileSearch` 服务与 `com.glwlg.fancun` 自启动标识。单独运行新 EXE 前，请先退出 Ariadne 并停止其后台服务。

迁移会检查普通用户目录和旧 MSIX 虚拟目录。目标已有独立数据，或检测到多个旧配置来源时，会提示冲突并停止，不覆盖现有内容。迁移完成后再次启动不会重复导入。

多个旧配置来源并存时，可在确认来源后使用 PowerShell 显式选择主目录：

```powershell
.\FancunSetup-dev-windows-x64.exe --quiet --legacy-roaming-source "$env:APPDATA\Ariadne"
```

迁移记录会保存所选来源，后续启动和升级沿用该记录。未选择的旧数据目录仍保留，不会合并或覆盖。

## 从源码运行

### 环境要求

- Windows 10/11 x64
- Go 1.26+（项目工具链为 Go 1.26.10）
- Wails 3.0.0（CLI、Go 模块和前端运行时保持同一版本）
- Node.js + pnpm

### 安装 Wails CLI

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
wails3 version
```

### 安装前端依赖

```powershell
cd frontend
pnpm install
```

### 本地开发

```powershell
pnpm dev
```

### 构建前端

```powershell
pnpm build
```

### 后端测试

```powershell
go test . ./cmd/... ./internal/...
```

### 生成 Windows 发布包

发布前可单独运行完整验收：

```powershell
wails3 task windows:verify
```

检查包含前端回归和类型检查、后端测试、原生截图、iPhone 点击/拖拽和跨进程通信，以及 setup EXE 的隔离安装、升级修复和卸载。安装测试使用独立产品名和临时目录，验证两个开始菜单入口的目标及用户数据保留。

```powershell
wails3 task windows:package
```

`windows:package` 和 `windows:setup` 会先执行上述检查；任一检查失败即停止打包。

搜索来源并行执行，每个来源同时最多处理一个请求；已完成的结果分批显示，无需等待较慢的文件索引，并保留当前键盘选中项。取消旧查询不会等待不支持取消的来源返回，结果去重和同分排序仍按来源注册顺序确定。收藏和最近使用按记录保存；工作记忆按变化的条目及数据组保存，保存失败时数据库事务回滚。工具窗口的标题、启动命令和尺寸统一维护在 `internal/toolregistry/views.json`。

主日志 `logs/fancun.log` 达到 5 MB 时轮转，保留 `.1`、`.2`、`.3` 三份备份；单次超大写入也会分段轮转。原有超大日志会先归档，随轮转逐步淘汰。

发布包会生成到：

```text
dist\release\FancunSetup-dev-windows-x64.exe
dist\release\SHA256SUMS
```

安装器内部包含完整应用载荷；默认不会保留中间 ZIP。

### 在 WSL 开发并生成 Windows 安装包

WSL 构建面向 Windows x64，不需要安装 Linux 桌面依赖。准备 Go、Node.js、pnpm、与 `go.mod` 对应版本的 Wails CLI，以及 .NET 8 SDK（发布时会下载 Windows 桌面目标包）。本机的 Wails 和 .NET 工具安装在独立目录，不影响其他项目：

```bash
cd /home/luwei/workspace/glwlg/Ariadne
export PATH="$HOME/.local/opt/ariadne/bin:$PATH"
pnpm --dir frontend install --frozen-lockfile
./scripts/build-windows-wsl.sh
```

脚本执行前端及原生组件回归测试，交叉编译两个 WPF 程序和 Windows 主程序，再生成 `dist/release/FancunSetup-dev-windows-x64.exe` 与 `SHA256SUMS`。设置 `FANCUN_VERSION` 可指定发布版本。完整 Windows 后端、安装器和快捷方式测试需要先把 WSL 工作树复制到 Windows 本地磁盘，再在副本中执行 `go test -count=1 . ./cmd/... ./internal/...`；Windows 的 Go 无法直接在 WSL 的 UNC 路径锁定文件。WSL 构建本身不运行这些 Windows 测试。

投屏使用的 `bin/uxplay` 是 Windows UxPlay/GStreamer 运行库。WSL 脚本会核对其来源版本和构建输入，缺失或过期时直接停止。首次准备或源码变化后，可在 Windows PowerShell 中将 WSL 源码暂存到 P 盘并构建：

```powershell
$source = '\\wsl.localhost\Ubuntu-26.04\home\luwei\workspace\glwlg\Ariadne'
$work = 'P:\workspace\glwlg\app\Fancun\.codex-tmp\wsl-uxplay'
New-Item -ItemType Directory -Force -Path "$work\source\scripts\uxplay", "$work\source\assets" | Out-Null
Copy-Item -LiteralPath "$source\scripts\build-uxplay.ps1" -Destination "$work\source\scripts\build-uxplay.ps1" -Force
Copy-Item -Path "$source\scripts\uxplay\*" -Destination "$work\source\scripts\uxplay" -Recurse -Force
Copy-Item -LiteralPath "$source\assets\logo.ico" -Destination "$work\source\assets\logo.ico" -Force
& "$work\source\scripts\build-uxplay.ps1" -BuildDir "$work\build" -OutputDir "$work\runtime"
```

成功后在 WSL 中复制构建结果，再执行上述打包命令：

```bash
rm -rf bin/uxplay
cp -a /mnt/p/workspace/glwlg/app/Fancun/.codex-tmp/wsl-uxplay/runtime bin/uxplay
```

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `wails3 task windows:build` | 构建 Windows 可执行文件。 |
| `wails3 task native:uxplay` | 构建自带的 Windows x64 投屏运行库。 |
| `wails3 task native:iphone-control` | 构建蓝牙 iPhone 控制窗；从投屏中心的“手机控制（可选）”打开。 |
| `wails3 task windows:mcp-build` | 构建仅供隔离测试使用、启用 Wails MCP 的开发 EXE。 |
| `wails3 task windows:mcp-dev` | 在回环地址和随机端口运行 MCP 开发构建。 |
| `wails3 task windows:package` | 构建用户级 Windows 安装器和更新校验文件，不保留 ZIP。 |
| `wails3 task windows:msix` | 生成未签名 MSIX 布局。 |
| `wails3 task windows:search-perf` | 运行搜索性能基准。 |
| `wails3 task windows:perf` | 运行桌面性能探针。 |
| `wails3 task windows:autostart-smoke` | 验证自启动注册路径。 |

MCP 开发构建输出为 `bin\fancun-mcp.exe`，不会被 Windows 发布任务打包。它允许测试客户端控制 Wails WebView 窗口，因此只能在隔离测试桌面运行；正式的 `bin\fancun.exe` 不包含 `mcp` build tag。

iPhone 投屏的连接方法、局域网授权、运行库构建和验证见 [投屏说明](docs/airplay.md)。

## 项目结构

```text
.
├── assets/                  # 图标和品牌资源
├── cmd/                     # 构建、打包、性能和迁移命令
├── docs/                    # 设计、计划、ADR 和代理协作文档
├── frontend/                # Vue 3 前端
├── internal/                # Go 服务、存储、平台集成和工具中心
├── winres/                  # Windows 图标、manifest 和版本资源
└── Taskfile.yml             # Wails/Windows 任务入口
```

## 设计原则

- 快捷入口保持轻量，复杂工具在独立窗口中打开。
- 前端不从文件路径推断动作，搜索结果必须显式携带 `actions`。
- 本地优先，涉及外部 AI 或敏感数据的能力必须有清晰确认。
- 安装目录和用户数据目录分离，卸载默认保留用户数据。
- 工具中心保持独立边界，避免把所有功能堆回启动器主窗口。

## 路线图

- 完成更多工具中心的真实桌面烟测。
- 完善 MSIX 签名和安装验证。
- 补齐更多 API 调试场景的导入、环境和断言体验。

## 贡献

当前仓库主要服务 Fancun 的本地演进。提交前建议至少运行：

```powershell
go test . ./cmd/... ./internal/...
pnpm --dir frontend build
```

如果改动涉及 Wails 绑定、工具窗口、发布包或 Windows 平台能力，请同时运行对应的 `wails3 task windows:*` 任务。

## 许可证

仓库当前没有声明开源许可证。使用、分发或二次开发前，请先和维护者确认授权边界。
