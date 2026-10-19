# AGENTS.md

DO NOT send optional commentary

## 本机安装与升级

使用本仓库构建的 setup EXE 安装或升级，保留 `%APPDATA%\Ariadne` 用户数据。开始菜单的 Ariadne 入口和卸载入口是安装器必需项，桌面快捷方式可选。静默安装也必须遵守此规则；旧 `--no-shortcuts` 参数只影响桌面快捷方式。

完成安装后，必须验证 `Get-StartApps` 能找到 Ariadne、开始菜单快捷方式指向本次安装的 `ariadne.exe`，并且 `install_receipt.json` 记录了两个开始菜单入口。只验证 EXE 存在或进程启动不能视为安装完成。

安装/卸载测试必须使用独立测试产品名和临时的安装、开始菜单、桌面目录，不得读写真实 Ariadne 的快捷方式、自启动注册表值或用户数据。

## Agent skills

### Issue tracker

Issues and PRDs for this repo live in GitHub Issues for `glwlg/Ariadne`. See `docs/agents/issue-tracker.md`.

### Triage labels

The repo uses the canonical five-label triage vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo for domain documentation. See `docs/agents/domain.md`.
