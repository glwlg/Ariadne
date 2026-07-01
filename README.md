# Ariadne

<p align="center">
  <img src="assets/logo.png" width="96" alt="Ariadne Logo">
</p>

<p align="center">
  <strong>面向 Windows 的本地效率入口：命令启动器、工作记忆、截图/剪贴板历史和桌面工具中心。</strong>
</p>

<p align="center">
  <a href="https://github.com/glwlg/Ariadne/releases"><img alt="下载" src="https://img.shields.io/badge/Download-Windows%20x64-0f766e"></a>
  <img alt="平台" src="https://img.shields.io/badge/Windows-10%2F11-2563eb">
  <img alt="Wails" src="https://img.shields.io/badge/Wails-3-8b5cf6">
  <img alt="Vue" src="https://img.shields.io/badge/Vue-3-42b883">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00add8">
</p>

## 简介

Ariadne 是一个轻量的 Windows 桌面效率工具。它把常用启动、文件搜索、截图历史、剪贴板历史、Hosts 管理、JSON 对比、网络监控、工作记忆和 API 调试等能力放到统一入口里，用快捷键快速呼出，并尽量把数据保留在本机。

## 功能亮点

| 模块 | 能力 |
| --- | --- |
| 命令启动器 | `Alt+Q` 呼出搜索框，聚合应用、文件、插件、工具中心和历史记录。 |
| 工作记忆 | 本地时间线、手动笔记、截图证据、草稿生成、经验发现和可导出的任务包。 |
| 截图历史 | 屏幕/窗口/区域截图、标注、OCR、二维码识别、置顶图片和历史检索。 |
| 剪贴板历史 | 文本和图片剪贴板监听、搜索、复制回写、OCR、二维码识别和截图归档。 |
| 工具中心 | Hosts 管理、JSON 对比、网络监控、工作流宏、设置中心和 API 调试窗口。 |
| 本地数据 | 运行数据写入 `%APPDATA%\Ariadne\ariadne.sqlite`，安装目录和用户数据目录分离。 |
| 发布包 | 用户级安装器，支持安装界面、安装位置、快捷方式、自启动、卸载和内置文件索引。 |

## 快速安装

1. 打开 [Releases](https://github.com/glwlg/Ariadne/releases)。
2. 下载最新的 `AriadneSetup-dev-windows-x64.exe`。
3. 双击安装器，阅读用户协议，按需选择安装位置、开始菜单入口、桌面快捷方式和随 Windows 启动。

`ariadne-dev-windows-x64.zip` 是排查用 payload，正常安装优先使用 setup exe。

默认安装到：

```text
%LOCALAPPDATA%\Programs\Ariadne
```

用户数据默认保存在：

```text
%APPDATA%\Ariadne
```

卸载时从开始菜单运行 `Uninstall Ariadne`。

## 从源码运行

### 环境要求

- Windows 10/11 x64
- Go
- Wails 3
- Node.js + pnpm

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

```powershell
wails3 task windows:package
```

发布包会生成到：

```text
dist\release\AriadneSetup-dev-windows-x64.exe
dist\release\ariadne-dev-windows-x64.zip
```

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `wails3 task windows:build` | 构建 Windows 可执行文件。 |
| `wails3 task windows:package` | 构建用户级 Windows 安装器和 zip 发布包。 |
| `wails3 task windows:msix` | 生成未签名 MSIX 布局。 |
| `wails3 task windows:search-perf` | 运行搜索性能基准。 |
| `wails3 task windows:perf` | 运行桌面性能探针。 |
| `wails3 task windows:autostart-smoke` | 验证自启动注册路径。 |

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
- 继续收敛工作记忆的本地索引、隐私边界和导出格式。

## 贡献

当前仓库主要服务 Ariadne 的本地演进。提交前建议至少运行：

```powershell
go test . ./cmd/... ./internal/...
pnpm --dir frontend build
```

如果改动涉及 Wails 绑定、工具窗口、发布包或 Windows 平台能力，请同时运行对应的 `wails3 task windows:*` 任务。

## 许可证

仓库当前没有声明开源许可证。使用、分发或二次开发前，请先和维护者确认授权边界。
