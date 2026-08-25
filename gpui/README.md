# Ariadne GPUI

这是 Ariadne 的 GPUI 重写入口，与现有 Go/Wails 应用并行演进。

当前可用功能：

- Launcher：空查询折叠、输入搜索、结果列表、右侧预览、键盘/鼠标选择
- 命令：json、base64、url、uuid，结果可直接复制
- 当前工作目录文件/文件夹搜索与 Explorer 打开
- 设置中心：亮石墨主题、开机启动状态、快捷键展示
- Hosts 管理：本地方案编辑、行数统计、预览反馈
- JSON 对比：双侧编辑、格式化、差异摘要
- 网络监控：本机连接检查与运行环境信息
- 剪贴板历史：读取当前系统文本剪贴板并重新复制

工作记忆模块暂不实现，截图历史也暂不接入。

## 运行

需要 Rust stable 工具链：

    cargo run --manifest-path gpui/Cargo.toml

## 迁移边界

当前工具页使用本地状态和 Windows 原生能力；Everything、SQLite 持久化、全局快捷键和旧数据迁移会在核心交互稳定后接入。