# Ariadne 实现台账

- 日期：2026-06-13
- 工作目录：`P:\workspace\glwlg\app\x-tools`
- 实现目录：`experiments/ariadne`
- 目标：记录 Ariadne 完整重构的当前工程证据、已验证能力、未完成差距和后续验收口径
- 维护规则：继续 Ariadne 重构前先读本文件；每次完成构建、验证、能力接入或发现阻塞后，都要更新本文件，不要凭印象重复找工具链或重复判断产物状态

## 0.0 开工状态卡

> 任何继续 Ariadne 重构的 Codex 会话都先读本节。它是防止重复找 Go、重复找 Wails、重复构建和重复验证的当前事实入口。

### 先不要重复做

1. 不要因为当前 PowerShell `PATH` 找不到 `go` 或 `wails3` 就判定环境缺失；本项目使用便携工具链。
2. 不要因为没有重新 build 就判定 app 不存在；`experiments\ariadne\bin\ariadne.exe` 已构建成功。
3. 不要重新搭 Wails/Vue 工程骨架；Ariadne 工程已在 `experiments\ariadne`。
4. 不要把 `wails3 build` 当成正确命令；当前 Windows 构建任务是 `wails3 task windows:build`。
5. 不要用普通 `%APPDATA%\Ariadne` 单点判断 Codex 启动场景下的数据是否写入；需要同时看 MSIX virtualized path。

### 固定工具链

```powershell
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
```

- Go：`C:\Users\luwei\.codex\tools\go1.26.4.windows-amd64\go\bin\go.exe`
- Wails 3 CLI：`C:\Users\luwei\.codex\go-bin\wails3.exe`
- Wails 版本：`v3.0.0-alpha.98`
- 正确构建目录：`P:\workspace\glwlg\app\x-tools\experiments\ariadne`

### 最新 app 产物

- exe：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\bin\ariadne.exe`
- 大小：`31915008` bytes
- 更新时间：`2026-06-15 22:13:43`
- SHA256：`DDA13649357D46653A23A5ED23F4EE789E625B82D299BBA03B60E952B9439D24`
- release zip：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\release\ariadne-dev-windows-x64.zip`
- release zip 大小：`16695757` bytes
- release zip 更新时间：`2026-06-15 22:13:44`
- release zip SHA256：`5B8CAF01B5D29B19EE5DC071E2B160F1D08361337B08D63863AE72D2B84C084C`
- MSIX layout：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0-msix`
- MSIX manifest：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0-msix\msix-manifest.json`
- 最近构建命令：`wails3 task windows:package`
- 最近 MSIX layout 命令：`wails3 task windows:msix`
- 最近开机启动烟测命令：`wails3 task windows:autostart-smoke`
- 最近 bindings：`469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`
- 最近心流设计图对齐修正：针对 2026-06-15 用户反馈“和设计图差距有点大”，继续按 Product Design 参考图收敛心流首页。`WorkMemoryCenter.vue` 默认入口现在是左侧垂直导航、顶部居中状态胶囊、中央大对话框、回答卡、右侧今日摘要/可优化流程/连接与模型卡片；旧底部状态条隐藏，原始证据默认收起；顶部运行开关压缩为图标级控件。`style.css` 调整 `flow-*` 左栏宽度、顶部留白、主栏/右栏比例、问答卡高度、卡片阴影和响应式布局，减少旧式全宽看板和传统工具条观感。验证：`git diff --check -- experiments/ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue experiments/ariadne/frontend/src/style.css`、`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31915008` bytes，时间 `2026-06-15 22:13:43`，SHA256 `DDA13649357D46653A23A5ED23F4EE789E625B82D299BBA03B60E952B9439D24`；新 release zip 为 `16695757` bytes，时间 `2026-06-15 22:13:44`，SHA256 `5B8CAF01B5D29B19EE5DC071E2B160F1D08361337B08D63863AE72D2B84C084C`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实桌面视觉复验。
- 最近心流界面重设计：针对 2026-06-15 用户反馈“工作记忆中心界面乱、第二大脑不应制造决策负担、轻量浏览和搜索应改成对话框、改名为心流”，按 Product Design 方向 `1+2` 将工作记忆中心默认入口重构为“心流”对话式第二大脑。默认页提供提问框，可直接问“我今天干了些什么”“今天有哪些人找过我”“今天我的哪些工作流可以优化”；今日摘要、自动洞察和证据计数轻量展示，原始明细默认收起，仅通过证据抽屉或时间线按需打开；洞察、草稿、资产、规则拆为独立页，经验发现不再在主界面要求接受/驳回，只有转任务包、工作流、清单或 Skill 等落地动作才需要确认；右侧证据明细改为抽屉，避免选中记录和全局高频操作混杂。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31911936` bytes，时间 `2026-06-15 21:56:03`，SHA256 `EF5B5CC813EE6272AA46F48F14038F7F8C0841A46F728187F86B23029EB7ECE8`；新 release zip 为 `16694930` bytes，时间 `2026-06-15 21:56:04`，SHA256 `B418BB33E698033A8EAD1F9EACA3EE80F959655FE89238A31386565E956E9D2F`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实桌面视觉复验。
- 最近普通工具窗口最大化修复：针对 2026-06-15 用户反馈“最大化后任务栏点不了、最大化和还原按钮像全屏 icon”，确认 `WindowControls.vue` 直接调用 Wails `Window.ToggleMaximise()` 会在当前窗口壳下把窗口铺到完整屏幕区域，透明区域可能覆盖 Windows 任务栏；同时 `Maximize2` 对角箭头更像全屏。修复后普通工具窗口最大化不再走原生 maximize，而是保存当前 `Window.Position()`/`Window.Size()`，读取当前屏幕 `WorkArea`，用 `Window.SetPosition(workArea.x, workArea.y)` + `Window.SetSize(workArea.width, workArea.height)` 手动铺满可用工作区，避免覆盖任务栏；再次点击恢复保存 frame，缺失 frame 时按当前 WorkArea 居中回到 1120x720 附近默认尺寸；按钮图标改为标准单方框最大化和 CSS 双重方框还原。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31875072` bytes，时间 `2026-06-15 21:15:37`，SHA256 `2F0069D9179A72D720B9D3A211ADE941D0E1A1B57B8CDBB631C076C4F32DA052`；新 release zip 为 `16686983` bytes，时间 `2026-06-15 21:15:38`，SHA256 `B9DF3AF80F37417FAC4439272ABF7F93756991D059CAA4874F4D9B6E2A43AD53`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实任务栏点击视觉复验。
- 最近 OCR Python 弹窗、中文乱码和旧 x-tools venv 依赖修复：针对 2026-06-15 用户反馈“OCR 是否调用 x-tools、最终不应依赖 x-tools、中文全是乱码、一直弹 Python 窗口”，确认 Ariadne OCR 是 `internal/ocr` 的 RapidOCR bridge，不是调用旧 x-tools UI，但此前默认查找仓库根 `.venv\Scripts\python.exe`，且 Windows GUI 程序启动控制台版 Python 时未设置无窗口创建，导致黑色控制台弹出；bridge 用 `ensure_ascii=false` 输出 JSON 但未强制 UTF-8 stdout，导致中文被系统编码污染。修复后 Windows OCR 子进程设置 `HideWindow` + `CREATE_NO_WINDOW`，环境强制 `PYTHONIOENCODING=utf-8` 和 `PYTHONUTF8=1`，Python bridge 重配 stdout/stderr 为 UTF-8，并把 RapidOCR 第三方 stdout 重定向到 stderr，stdout 只输出 UTF-8 JSON。OCR Python 查找顺序改为 `ARIADNE_OCR_PYTHON`、`ARIADNE_OCR_HOME`、Ariadne 安装目录/runtime、`%LOCALAPPDATA%\Ariadne\ocr-python`、系统 `python`；仓库 `.venv` / `uv` 只在显式 `ARIADNE_OCR_ALLOW_REPO_PYTHON=1` 开发模式下启用，且不接受 `pythonw.exe` 这类无 stdout 的解释器。release manifest/README 已改为说明核心包独立于旧 x-tools/PyQt/PyInstaller，OCR 需要显式 Ariadne OCR runtime。验证：`go test ./internal/ocr -v`、`go test ./internal/ocr ./internal/releasepack -v`、`go test ./...`、`py_compile rapidocr_bridge.py`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31873024` bytes，时间 `2026-06-15 17:48:47`，SHA256 `73A39FEDCA9524F0B31EB2F3A08EDB9346CA59A35D404674EBE65252007D75EE`；新 release zip 为 `16685764` bytes，时间 `2026-06-15 17:48:48`，SHA256 `4501DBD416054946A4710A6DF4C2DD61795D92FD768D7E7F3B2A38D37D535B31`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实 OCR 点击复验。
- 最近工作记忆主动沉淀桥接：针对 2026-06-15 用户要求“继续完善主动沉淀”，剪贴板历史和截图历史新增 entry observer，主进程在工作记忆启用、隐私模式关闭且对应来源开关启用时，把剪贴板监听/截图历史入库事件写入工作记忆。剪贴板文本/图片分别以 `clipboard_text`/`clipboard_image` 类型沉淀，截图历史以 `screenshot` 类型沉淀，按 signature/content hash 去重，并跳过 `manual_capture`、`time_machine` 等已由工作记忆自身写入的来源，避免重复。平台 `remember` action 现在对剪贴板/截图结果真实写入工作记忆；工作记忆中心新增“开启主动沉淀”按钮，只开启剪贴板和截图历史来源，不静默开启屏幕时间机器。验证：`go test ./internal/workmemory -v`、`go test ./internal/clipboardhistory ./internal/capturehistory ./internal/platform -v`、`go test ./...`、`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 189 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31864320` bytes，时间 `2026-06-15 17:39:11`，SHA256 `9DFDC41A3B377C5670C6DCA3FBD34FD3F2381E81FA40DDD9666CBA6423B3323F`；新 release zip 为 `16682861` bytes，时间 `2026-06-15 17:39:12`，SHA256 `5DB286D7273C0EB2A1E8CCDC0AB2F67C51D4BB57A91E1D74A605845F2D194348`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实桌面主动沉淀 UI 烟测。
- 最近设置中心铺满态外框线条补回：针对 2026-06-15 用户澄清“我指的是外框线条没有”，确认上一轮为去掉居中卡片壳，把 `html.window-controls-visible .launcher-shell` 设成 `border: 0`，导致设置中心最外层窗口轮廓缺失。本轮仅补回铺满态外轮廓：`border: 1px solid var(--border-strong)` 并加很轻的 inset 线，继续保持 `border-radius: 0`、不恢复大阴影/浮动卡片。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31835136` bytes，时间 `2026-06-15 17:01:32`，SHA256 `3FB8F707D3B5DAC9B6B052E877BE557A21CFF28D6FF7F899916BB7E717CB9DE8`；新 release zip 为 `16668663` bytes，时间 `2026-06-15 17:01:33`，SHA256 `7608387964FB466540D7ACF8453FF98938E947F64EE4F962110B28BEAA43898A`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心顶部冗余按钮删除与铺满态结构线增强：针对 2026-06-15 用户反馈“缺了边框线条，都和背景融为一体了，右上角的保守默认、启动器两个按钮可以删掉”，删除设置中心 header 右侧私有的“保守默认”pill 和“启动器”按钮，仅保留 App 层普通窗口控制；保留其他工具中心各自返回启动器入口不变。普通中心铺满态下补清晰结构线：header/tool-header 使用 `border-strong` 底线和 surface 背景，settings workspace 用更强分隔线背景，左侧 rail 加右边框，settings summary/panel/page header 统一加强边框，footer/status strip 加顶部分隔线和 raised 背景；不恢复居中卡片壳、圆角、大阴影或 blur。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31835136` bytes，时间 `2026-06-15 16:56:26`，SHA256 `2AFA66098E2432154DBCADBE485EF1C70FD37322B53208ECCBE6BD696388112F`；新 release zip 为 `16668759` bytes，时间 `2026-06-15 16:56:27`，SHA256 `2C15C2CC1BA4CD3128F8CFC869CA7A02FCFFA45AE8663278B03A8676607B3AB8`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心最大化铺满、壳边框移除和启动项点击保护：针对 2026-06-15 用户反馈“最大化非常丑陋，非最大化状态也有一圈丑陋的边框，Everything 依旧点不动”，确认设置中心当前常在主无边框窗口 active view 内显示，上一轮只处理 `tool-window-document` 不足以覆盖主壳 fallback。本轮将 `html.window-controls-visible` 下的普通中心视图改为 `100vw/100vh` 铺满窗口，`.app-frame` 不再居中卡片，`.launcher-shell` 在普通中心视图中去除边框、圆角、阴影和 blur，最大化/非最大化都不再出现中间浮动壳；窗口控制条改为轻量标题栏控件，取消外框/阴影，靠近右上角。启动项列表补 `data-no-drag`、`@pointerdown.stop` 和显式 `--wails-draggable: no-drag`，点击后通过 `settings.showFeedback()` 显示“已选择启动项：...”以便确认事件触发；同时保持 `selectLauncher()` 更新 draft 并重置滚动。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31834624` bytes，时间 `2026-06-15 16:49:38`，SHA256 `684744C7A302ADA98145FEE9F64E2977850D40A7D92A6D0EB2CE4EFCD4D4DD7F`；新 release zip 为 `16668710` bytes，时间 `2026-06-15 16:49:39`，SHA256 `B1BCBFD42BBF56B4589BB22D723D7C8D29DA04A557C8F7A0533A3A98D55AF263`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心窗口控制、截图热键对齐、启动项选择和切页滚动修复：针对 2026-06-15 用户反馈“还是没有最小化/最大化/还原/关闭，截图这里没对齐，启动项 Everything 点了没反应，滚动条状态会带到下一页”，新增通用前端 `WindowControls.vue`，在 App 层只对普通工具中心显示最小化、最大化/还原、关闭控件，启动器、截图覆盖层、贴图和网速小窗不显示；关闭行为复用 `appShell.closeCurrentWindow()`，独立工具窗关闭，主壳 fallback 返回启动器。设置中心右侧内容加 `settingsContentRef`，切换分类和选择启动项时重置右侧滚动到顶部；启动项列表按钮明确 `type="button"` 并走 `selectLauncher()`，降低点击被默认行为或拖拽层吞掉的风险；快捷键页的 `settings-hotkey-grid settings-policy-grid` 改为三等列，贴图热键与主窗口/截图对齐。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31834112` bytes，时间 `2026-06-15 16:41:59`，SHA256 `B9B83FAAAFC26B275807E984496BC3EA4C83DA143D330DCC97443A7BB2A49319`；新 release zip 为 `16668763` bytes，时间 `2026-06-15 16:42:00`，SHA256 `ECD2CA33662D2299E706ACCF943F893238E5039E5A45885D5D6B6A21C67DEA0A`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心产品设计重排与普通工具窗原生窗口控制：针对 2026-06-15 用户反馈“太乱了，什么都挤在一个页面”以及“窗口还是应该要有最小化最大化还原关闭这些按钮的，只不过启动器这类特殊窗口不要”，按 Product Design brief 将设置中心从单页堆叠改为左侧任务导航 + 右侧单一 active page；日常入口拆为通用、快捷键、插件、启动项、截图、工作记忆、AI 与向量、隐私规则、数据与存储，高级维护单独收纳平台诊断、数据导入和回滚检查点。默认页不再展示迁移/兼容/诊断噪音，插件配置保留独立页。普通 tool-window 现在使用系统标题栏和原生最小化/最大化/还原/关闭；`network-mini` 继续无边框，截图覆盖层、贴图和启动器仍走专用特殊窗口策略。验证：`pnpm --dir experiments\ariadne\frontend exec vue-tsc --noEmit`、`pnpm --dir experiments\ariadne\frontend build`、`go test ./internal/toolwindows -v`、`go test ./...`、`wails3 task windows:package` 均通过；Vite/Rolldown 仍有第三方 `@vueuse/core` pure annotation 警告和大 chunk 提示，不影响产物；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31832064` bytes，时间 `2026-06-15 16:28:50`，SHA256 `8D84CE2283DAB93553C34BD4F55F2B7BD24FF0E0B8BE168DF8E716A95DF47051`；新 release zip 为 `16668498` bytes，时间 `2026-06-15 16:28:51`，SHA256 `5B30C743D3E66111F2A4258EE24F2E9706A2D081C43BBC759EB53D02F76DB7DD`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心插件配置可见化与兼容信息降噪：针对 2026-06-15 用户反馈“还是一堆兼容说明，到现在没看到插件配置”，设置中心左侧摘要移除旧版配置/旧历史/旧版运行常驻 meter，底部状态条移除旧版迁移提示；配置存储、平台诊断、搜索数据、数据导入、数据保护改为默认折叠维护区。主表单在快捷键下方新增“插件”配置区，直接读取 Go `plugins.List()` manifest，展示插件名称、描述、用法、关键词和能力，并通过 `settings.plugins.enabled[id]` 启停插件；前端主插件区隐藏 `legacy_python` 桥，避免把过渡兼容入口混进日常插件配置。后端 `plugins.Service` 新增运行时 enabled map，搜索触发器和 `Execute` 均尊重停用状态，设置保存后通过 settings change handler 同步；settings 归一化插件 id 为小写，避免旧配置 `UUID`/`JSON` 导入后状态和运行时不一致。验证：`pnpm --dir frontend exec vue-tsc --noEmit`、`go test ./internal/plugins ./internal/settings -v`、`pnpm --dir frontend build`、`go test ./...`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 187 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31819776` bytes，时间 `2026-06-15 14:16:11`，SHA256 `31B09ED363217ABBDFBA21BE0DE066ACEED9955E9EC7A7677C9F5B22E1D9BEE9`；新 release zip 为 `16667018` bytes，时间 `2026-06-15 14:16:12`，SHA256 `1BFE15668C6E198430D7A56A8125AF45538ED401431E4ACAB259EC5646A1F4C1`。本轮只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近设置中心迁移入口清理和配置项补齐：针对 2026-06-15 用户反馈“设置中心左边一堆迁移相关，数据已迁移好就不要再出现 x-tools；设置中心少了很多配置项”，后端 `LegacyConfigStatus` 和 `LegacyDataStatus` 新增 `needsImport`，旧配置状态会把可迁移配置映射到当前 Ariadne settings 并与安全凭据状态对比，旧历史状态会统计剪贴板、截图和工作记忆中 `legacy_x_tools` 导入记录数；当旧配置/旧历史已覆盖当前数据时，设置中心默认隐藏旧版 x-tools 迁移面板，左侧摘要和底部状态条也不再常驻迁移项。回滚检查点从旧版面板拆出为独立“数据保护”面板；设置中心补齐语言、截图自动保存/质量、工作记忆截图质量/经验发现天数、Skill/工作流建议、外部代理、OpsCore 同步、trace、外部代理任务目录、收藏永久保留等已存在后端字段。验证：`go test ./internal/settings ./internal/migration ./internal/clipboardhistory ./internal/capturehistory ./internal/workmemory -v`、`go test ./...`、`pnpm --dir frontend build`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 186 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31803904` bytes，时间 `2026-06-15 13:17:50`，SHA256 `BA84913974BE00F823209859FD1EE41D67950F8961E999136C0E2997B03805F4`；新 release zip 为 `16660599` bytes，时间 `2026-06-15 13:17:52`，SHA256 `769247A677782721DCF8C939DF518601CCC79E24D64F88D4F9E38875ACCB74F8`。本轮按用户限制只走后台命令行，未操作鼠标、窗口、热键或桌面画面；因此不声明真实设置中心视觉复验。
- 最近网速小窗持续置顶闪烁修复：针对 2026-06-15 用户反馈“重置时小窗会跑到屏幕中间一瞬间、无遮挡也不断重置导致左下角小窗一直闪烁”，确认上一轮把完整 `applyNetworkMiniPlacement()` 放进 900ms monitor tick，导致每次 tick 都执行 `SetScreen/SetSize/SetRelativePosition`，Wails 会出现可见重排闪烁。本轮拆分“定位”和“层级保活”：打开、配置变化、全屏自动隐藏恢复时仍使用完整定位；常规 tick 只调用 `refreshNetworkMiniTaskbarLayer()`，执行不移动不改尺寸的 Win32 owner/topmost 保活，且不带 `SWP_SHOWWINDOW`，避免周期性重排和闪烁；Windows/非 Windows taskbar helper 均补齐对应入口。验证：`go test ./internal/toolwindows -v`、`go test ./...`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 183 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31787008` bytes，时间 `2026-06-15 11:14:17`，SHA256 `E0CC45B673283D1ABB0F840B6CB6586E38839790F4F927E5C4CE2657099F5A6C`；新 release zip 为 `16649728` bytes，时间 `2026-06-15 11:14:18`，SHA256 `432237F325EFBAE764F7ABCB79FE06B7B92845E93AE27062D86CA4146D15581C`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实闪烁视觉复验。
- 最近网速小窗任务栏层级持续置顶修复：针对 2026-06-15 用户反馈“网速小窗的层级没有任务栏高”，复核旧版 `src\ui\network_monitor.py` 后确认旧 x-tools 使用 `ToolTip` 小窗，并在 `showEvent()` 和每次 `update_traffic()` 中持续 `anchor_to_taskbar()` + `keep_on_top()`；`keep_on_top()` 会重复 `SetWindowLongPtrW(GWLP_HWNDPARENT=Shell_TrayWnd)` 与 `SetWindowPos(HWND_TOPMOST)`。Ariadne 此前只在打开/配置变化时应用一次任务栏 owner/topmost，容易被任务栏后续 Z-order 压住。本轮改为网络小窗可见时由 900ms monitor tick 持续重算任务栏位置并恢复 owner/topmost；Windows native 层补 `WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_NOACTIVATE`，移除 `WS_EX_APPWINDOW`，继续使用 `SWP_NOACTIVATE` 避免周期性抢焦点。验证：`go test ./internal/toolwindows -v`、`go test ./...`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 183 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31785984` bytes，时间 `2026-06-15 11:02:19`，SHA256 `9B241F89ABA767C8FCBD1AEAFEA7A0C1CF6473ED2ADDFF7D2D176931A8C7888C`；新 release zip 为 `16649084` bytes，时间 `2026-06-15 11:02:19`，SHA256 `61E63E969EE4847708144A91920590C159BF0614E6BC2D105D07C4292CA9A64E`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实任务栏层级视觉复验。
- 最近贴图全局快捷键注入与截图工具栏高频动作右移：针对 2026-06-15 用户反馈“贴图为什么不注入、截图工具栏最左四个应移到最右、贴图和复制应靠近鼠标位置”，shell manager 已从主窗口/截图两个全局热键扩展为主窗口/截图/贴图三个热键，`ApplyHotkeys`、`RetryHotkeyRegistration`、运行态 `ShellStatus` 和设置中心状态 pill 均新增贴图热键；贴图热键触发 `pinnedImageService.OpenCurrentClipboard()`，读取当前系统剪贴板，图片直接创建贴图，文本生成轻量 SVG 文本贴图并支持右键复制文本；设置页不再显示“仅保存/暂未注册”，重复快捷键校验扩展到三者互斥。截图覆盖层工具栏把原最左侧四个动作移到最右侧，顺序调整为低频编辑在左、保存/另存/复制/贴图在右，复制和贴图靠近右端鼠标区域。新增 `TestOpenCurrentClipboardImageStagesPinnedImage`、`TestOpenCurrentClipboardTextStagesTextPinnedImage`。验证：`go test ./internal/shell ./internal/pinnedimage ./internal/clipboardhistory ./internal/platform -v`、`go test ./...`、`wails3 generate bindings -clean=false`、`pnpm --dir frontend build`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 183 Methods, 5 Enums, 170 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31784960` bytes，时间 `2026-06-15 10:46:46`，SHA256 `FCFAAF10F687ACE7D620FF6ACC028239BFB47326E888D49F5C7B933B566DC421`；新 release zip 为 `16648273` bytes，时间 `2026-06-15 10:46:47`，SHA256 `0AF49B4D2BBB1AECC6C5019BD37AFC9B8094841BB6555C34AF4696096998DBF7`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实桌面贴图热键或工具栏点击复验。
- 最近 F1 截图快捷键、截图不隐藏自身窗口、任务栏网速小窗修复：针对 2026-06-15 用户反馈“截图快捷键不能设置为 F1、截图时会隐藏工具本身、网速监控没有嵌入任务栏”，后端 `internal/shell.ParseHotkey()` 允许裸 `F1`-`F24`，仍拒绝裸字母/数字/空格；设置页按键捕获和文本规范化同步允许裸功能键，并阻止浏览器 F1 帮助抢占。截图覆盖层 `Open()` 改为直接 `CaptureScreenPNG()` 后创建 overlay，不再先隐藏 Ariadne 自有窗口，`captureWindowShouldHide()` 当前固定返回 false。网络小窗改为 x-tools 风格任务栏左侧小条，默认 anchor 为 `taskbar-left`、尺寸 `156x40`、透明背景、无 header/footer，打开时不再 `Focus()` 抢焦点；Windows 下通过 `Shell_TrayWnd` + `SetWindowLongPtrW(GWLP_HWNDPARENT)` + `SetWindowPos(HWND_TOPMOST|SWP_NOACTIVATE)` 挂到任务栏层级，同时保留旧四角 anchor 兼容。验证：`go test ./internal/shell ./internal/captureoverlay ./internal/toolwindows -v`、`go test ./...`、`pnpm --dir frontend build`、`wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31770624` bytes，时间 `2026-06-15 10:35:17`，SHA256 `038C672CAB4F6BD9E31E3DC3922FA3657FEABB8491A8F5E522CEA18EE2E79196`；新 release zip 为 `16643785` bytes，时间 `2026-06-15 10:35:18`，SHA256 `89E5BD1DBBCDF4291C876523BC9EB8316CD15032682B2FBF431B165ECA3DF4FC`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实桌面 F1 注册、截图不遮挡或任务栏嵌入复验。
- 最近启动器折叠态白底外框与展开跳动修复：针对 2026-06-15 用户反馈“白色背景下仍能看到明显边框、初始化位置太中间、输入展开时跳动”，新增 `internal/launcherwindow` 和前端 `launcherGeometry` 共享几何口径；折叠/展开统一使用 `860px` 窗口宽度，折叠高度 `96px`，展开高度 `468px`，窗口初始位置按展开面板高度预留后居中。Go shell manager、tool-window launcher bridge、主窗口初始 placement、前端 appShell 和 launcher resize 都不再在 launcher 展开/折叠路径重复 `Center()`；展开只增大高度，不改变锚点。折叠态 `.palette-shell` 去掉外层背景、边框、阴影和 blur，只保留搜索框本体，避免白底下出现外层壳子轮廓。定位优先使用窗口当前屏幕，失败时回落主屏；前后端都用向下取整，避免奇数尺寸屏幕出现 1px 定位差。新增 `TestReservedRelativePositionUsesExpandedLauncherHeight`、小屏 clamp 和折叠/展开同宽测试。验证：`go test ./internal/launcherwindow ./internal/shell ./internal/toolwindows -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:package` 均通过；bindings 为 `469 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31762432` bytes，时间 `2026-06-15 09:57:54`，SHA256 `71D09DBE9F6EF5DC68B1BB636EA63423FAD397C9D93F1DD4D5264830815383C2`；新 release zip 为 `16638203` bytes，时间 `2026-06-15 09:57:55`，SHA256 `04CE7A04F0F48405BF95E68A26AD3E7390FEBD868A96F715C4358EC2DDFB3DDB`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实桌面 Alt+Q/输入展开视觉复验。
- 最近贴图原生拖动与选区原点对齐修复并重新打包：针对 2026-06-15 用户反馈“大小对了但位置又偏、拖动贴图一抖一抖”，确认 JS pointermove 中循环 `Window.Position/SetPosition` 会引入异步 IPC 抖动，`PinnedImageWindow.vue` 已移除 JS 拖动状态、pointer capture、move queue 和运行时 `Window.SetPosition` 调用；贴图主体、stage、zoom layer 和图片主体改为 Wails 原生 `--wails-draggable: drag`，菜单、OCR 条、OCR 框、按钮等交互区继续 `no-drag`。区域截图创建贴图时去掉旧的 `x-15/y-15`“附近”偏移，改为窗口左上角对齐最终 native 选区左上角。继续保留 exact-size 贴图窗口、`1x1` 最小窗口、无顶部工具条、默认无阴影、无边框/圆角/内边距/`object-fit` 缩放、图片左上角锚定、右键菜单临时扩大透明窗口防裁剪；后续 10:35 修复已把截图捕获改为不再隐藏 Ariadne 自有窗口。新增/更新 `TestCaptureSelectionPinsAtSelectionOrigin`、visual resolved origin 和 native offset origin 期望。验证：`go test ./internal/captureoverlay ./internal/pinnedimage -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:package` 均通过；bindings 为 `468 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`。新 `bin\ariadne.exe` 为 `31757312` bytes，时间 `2026-06-15 09:33:38`，SHA256 `52C1A03C27DB8B81A72EFBDD17082A12D5A9E0686494046166214F381E021388`；新 release zip 为 `16632699` bytes，时间 `2026-06-15 09:33:39`，SHA256 `824954631B0A4580D27A8007611F4DD6FFA48C3EA08070BE3AE10FB4AC5FB147`；manifest 内 `app/ariadne.exe` 同为 `31757312` bytes / SHA256 `52c1a03c27db8b81a72efbdd17082a12d5a9e0686494046166214f381e021388`。本轮按用户要求只走后台命令行，不控制鼠标、窗口、快捷键或桌面画面；因此不声明真实桌面贴图位置/拖动/右键菜单复验。MSIX layout 本轮未重建。
- 最近开机启动 HKCU Run 非破坏性烟测：新增 `internal/shell.RunAutostartSmoke()`、`cmd/autostartsmoke` 和 `windows:autostart-smoke` Taskfile 入口；烟测写入唯一临时 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\com.glwlg.ariadne.smoke.*` 值，命令为当前 `ariadne.exe --hidden` 绝对路径，回读后复用 autostart audit 验证 value name、exe 路径和 `--hidden`，最后删除临时值。报告 `dist\perf\ariadne-autostart-smoke-latest.json` 为 `ok=true`、`cleanupOk=true`、`auditValid=true`、`hiddenArgPresent=true`、`commandMatchesExe=true`，PowerShell 复查返回 `CLEANED=...`。验证：`go test ./internal/shell -v`、`go test ./...`、`wails3 task windows:autostart-smoke` 和 `wails3 task windows:msix` 均通过；新 exe/layout 为 `31762432` bytes，时间 `2026-06-15 09:04:45`，SHA256 `28482F9D19E72C3D29EE8A1E1687A088F415ECF7D87E29CFBA214881D6396D8A`。本项验证注册表写入/回读/清理权限和隐藏启动命令，不等同于真实设置页点击开关。
- 最近 Codex Skill 安装诊断补强：`internal/skills.InstallDiagnostics()` 新增只读本地诊断，可扫描 Codex skills 目录、解析已安装 `SKILL.md`、核验 `.ariadne-refresh.json` 与 `.ariadne-refresh.touch` marker 是否一致，并在工作记忆知识面板安装成功后显示本地发现/握手状态。新增安装成功、目标目录缺失、marker 不一致三类测试。验证：`go test ./internal/skills -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:msix` 均通过；bindings 更新为 `468 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`；新 exe/layout 为 `31762944` bytes，时间 `2026-06-15 08:56:41`，SHA256 `3EEC2B29CB4E6D566F78A3F853AC13C46A2747BD6ECF8BA726DE39E9E06707A8`。该诊断只证明本地 Codex skill 发现目录和 Ariadne refresh marker 成立，仍不声明运行中的 Codex runtime 已热加载该 skill。
- 最近搜索收藏取消空状态清理：`StateStore.SetFavorite(false)` 现在会删除既非收藏也无最近使用的空记录，避免设置中心“收藏/最近使用”计数残留；已有最近使用的结果取消收藏后仍保留最近使用、移除收藏 boost/tag，并恢复 `收藏` action。新增 `TestSearchUnfavoriteWithoutRecentUseRemovesEmptyRecord` 和 `TestSearchUnfavoriteKeepsRecentUsageButRemovesFavoriteBoost`。验证：`go test ./internal/search -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:msix` 均通过。
- 最近 unsigned MSIX layout 同步：`wails3 task windows:msix` 通过，重新生成 `dist\msix\ariadne-0-0-0-0-msix`；layout 内 `Ariadne.exe` 为 `31762432` bytes，时间 `2026-06-15 09:04:45`，SHA256 `28482F9D19E72C3D29EE8A1E1687A088F415ECF7D87E29CFBA214881D6396D8A`，`msix-manifest.json` 记录 `createdAt=1781485485`、`packed=false` 且 candidate `.msix` 路径为 `dist\msix\ariadne-0-0-0-0.msix`。本机探测 `makeappx.exe` 和 `signtool.exe` 均未发现，`C:\Program Files (x86)\Windows Kits\10\bin` 不存在，因此签名 `.msix` 打包和安装/卸载验收仍被外部 SDK/证书条件阻断。
- 最近旧 Python 内置插件 Go 主路径覆盖审计：新增 `TestLegacyPythonBuiltinsHaveNativeGoCoverage`，直接枚举仓库 `src/plugins/*.py`，要求 16 个旧内置插件全部映射到 Ariadne 非 `legacy_python` 的 Go manifest 或工具窗口入口：calculator、timestamp、base64、hash、json、json_compare、url、uuid、custom_launch、qr、system_commands、hosts、clipboard、capture_history、workflow、work_memory。验证：`go test ./internal/plugins -v` 和 `go test ./...` 均通过。本项只新增覆盖测试和台账，不改生产代码；Python legacy bridge 继续保留给未知外部插件或临时过渡，不再作为当前内置插件的长期主路径。
- 最近设置中心安全密钥保存/清除链路补强：设置中心 AI / Embedding / Milvus 密钥行新增稳定 `data-secret-*` 测试钩子、保存/清除禁用态和内联结果反馈；保存密钥会清掉旧的清除二次确认状态，避免用户刚保存新密钥后沿用旧清除确认。`internal/secrets` 测试覆盖三类密钥的 trim/save/status/preview clear/confirmed clear，`internal/settings` 测试覆盖旧配置导入后 AI/Embedding/Milvus 密钥迁移 notes 可见。验证：`go test ./internal/secrets ./internal/settings -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；当时 exe 为 `31729152` bytes，时间 `2026-06-15 08:26:18`，SHA256 `8A37DB8FA4CF07E68FA1A3B986DF9FED621A273625FBFC0159F18B34F58FAF56`。当前 Computer Use 仍被 `GetCursorPos failed: 拒绝访问 (0x80070005)` 阻断，因此本项不声明真实桌面点击保存/清除验收通过。
- 历史截图选区/贴图拖动回归修复（08:47，已被 09:33 原生拖动方案覆盖）：用户再次反馈“截图内容与框选无关、贴图位置偏离、贴图不能拖动”后，当时把截图请求改为前端提交当前 `<img>` 内的 visual 坐标和实际显示尺寸，由 Go 统一映射到 overlay PNG 源像素并裁剪；贴图初始位置不再由前端传 `pinX/pinY`，而是 Go 使用最终裁剪出的 native 选区换算到 Wails DIP；贴图窗口拖动一度改为前端运行时 `Window.Position/SetPosition` 直接移动当前窗口，后端新增 `SetPinnedPosition` 只同步状态。该 JS 拖动路线后续被用户反馈抖动，当前有效拖动方案是上方 09:33 记录的 Wails 原生 `--wails-draggable: drag`。本历史记录的验证：`pnpm test:capture-geometry`、`go test ./internal/captureoverlay ./internal/pinnedimage -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:msix` 均通过；当时 exe/layout 为 `31762432` bytes，时间 `2026-06-15 09:04:45`，SHA256 `28482F9D19E72C3D29EE8A1E1687A088F415ECF7D87E29CFBA214881D6396D8A`。Win32 截图 smoke 在当前 Codex 桌面会话被 `BitBlt 失败: Access is denied.` 阻断，未声明真实桌面截图内容、贴图定位或拖动验收通过；测试启动的 Ariadne 进程已由 smoke 清理。
- 最近贴图窗口右键菜单验证：`PinnedImageWindow.vue` 新增局部右键菜单，复用已有复制、OCR、OCR 选中文本/全文复制、缩放、原始比例、阴影和关闭动作；菜单会避让窗口边缘，Esc 优先关闭菜单，Shift+F10/ContextMenu 可在窗口中心打开；平台 capability 文案已补 `右键菜单`。验证：`go test ./internal/platform -v`、`pnpm build`、`go test ./...`、`wails3 task windows:build` 均通过；当前工具发现未暴露 Computer Use 控制接口，本项不声明真实贴图右键点击验收。
- 最近 MSIX layout 验证：新增 `internal/msixpack` 测试、`cmd/msixpack` CLI，以及 `windows:msix` / `windows:msix-pack` Taskfile 入口；最新 `wails3 task windows:msix` 通过，生成 unsigned full-trust MSIX layout，包含 `Ariadne.exe`、`AppxManifest.xml`、`README-msix.txt`、`msix-manifest.json` 和三份 PNG logo。layout 内 `Ariadne.exe` 已同步到当前 `bin\ariadne.exe`，为 `31762432` bytes，时间 `2026-06-15 09:04:45`，SHA256 `28482F9D19E72C3D29EE8A1E1687A088F415ECF7D87E29CFBA214881D6396D8A`；manifest 记录 candidate `.msix` 路径但 `packed=false`。当前机器 `makeappx.exe` 与 `signtool.exe` 均不存在，`C:\Program Files (x86)\Windows Kits\10\bin` 不存在，且未配置匹配 Publisher 的签名证书，所以不能声明签名 `.msix` 安装/卸载验收通过。
- 最近 shell 修复验证：Win32 探针确认启动前 Alt+Q/Alt+A 均可注册，Ariadne 运行中两者均返回 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)`；launcher `WS_EX_TOPMOST=false`，Esc 后隐藏，Alt+A 打开 `Ariadne - 截图覆盖层`，Alt+Q 可重新唤起 launcher，合成拖动将 launcher 从 `(900,648)` 移到 `(1070,713)`。
- 历史截图/贴图回归修复记录（02:09，已被 08:47 当前方案覆盖）：当时尝试 surface/window 坐标、source-local session 像素裁剪、显式 `pinX/pinY` 和 CSS native drag；`go test ./internal/captureoverlay ./internal/capturehistory ./internal/pinnedimage -v`、`go test ./...`、`pnpm test:capture-geometry`、`pnpm build` 和 `wails3 task windows:build` 通过，但真实桌面复验被 Computer Use `GetCursorPos` 与 Win32 `BitBlt Access is denied` 阻断。该记录只保留为排查历史，不能作为当前拖动方案。
- 历史截图/贴图回归复核记录（02:23，已被 09:33 当前方案覆盖）：当时复核并重建了 `31724032` bytes / `2026-06-15 02:23:19` / SHA256 `F0B44F6329D95B197109CB1229C2DACD97424854B21C9D3439661E8130A2972D` 的 exe，但仍未完成真实截图内容、近选区贴图和拖动断言；当前有效方案以上方 09:33 visual 坐标 + 后端裁剪 + 选区原点对齐 + Wails 原生 drag-region 记录为准。
- 最近工作记忆 URL 级排除规则验证：`WorkMemorySettings` / `CapturePolicy` 新增 `excludeUrls`，设置中心和工作记忆中心排除规则面板新增 URL 域名/路径输入，旧配置导入兼容 `exclude_urls` / `exclude_url_patterns`。执行层会在采集窗口标题、材料导入、OCR 写回、导出和所有 `entryExcluded` 消费者中按 URL 域名/路径阻断，避免仅靠内容关键字误伤或漏掉 `https://...` 材料。验证：`go test ./internal/settings ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；新 exe 为 `31724032` bytes，时间 `2026-06-15 02:23:19`，SHA256 `F0B44F6329D95B197109CB1229C2DACD97424854B21C9D3439661E8130A2972D`。本项是代码/构建验证，尚未声明真实桌面编辑保存 URL 排除规则。
- 最近 URL/JSON/Base64 文本插件旧版语义对齐：旧 Python `url` 插件使用 `urllib.parse.quote/unquote`，不是 query-string 编码；Ariadne Go 主路径已从 `QueryEscape/QueryUnescape` 改为旧版兼容语义，空格编码为 `%20`、斜杠保持 safe、百分号转义可解码，字面 `+` 不会被错误解成空格。旧 Python `json` 插件使用 `json.dumps(..., indent=4, ensure_ascii=False)`；Ariadne Go 主路径已改为 4 空格缩进，并通过 `json.Encoder.SetEscapeHTML(false)` 保持中文和 `<>&` 等字符不被转义。旧 Python `base64` 插件只在解码结果可按 UTF-8 文本读取时才显示解码结果；Ariadne Go 主路径已补 `utf8.Valid` 检查，`/w==` 这类合法 Base64 二进制输入只保留编码结果，避免复制乱码。新增 `TestURLPluginMatchesLegacyQuoteSemantics`、`TestJSONPluginMatchesLegacyDumpSemantics` 和 `TestBase64PluginMatchesLegacyUTF8DecodeSemantics`，并更新 `TestTextPluginsExecuteOnGoPath` 期望；验证：`go test ./internal/plugins ./internal/workflows -v`、`go test ./...` 和 `wails3 task windows:build` 均通过，当时 exe 为 `31697920` bytes，时间 `2026-06-15 01:38:56`，SHA256 `2023CF818FFC3348BEBC3668352C57A0AD063C84CE84CC8C857675C55703B570`。
- 最近平台诊断和设置文案校准：平台 capability 不再把截图覆盖层标注/文本选择写成“待接入”，`screenshot_overlay` 文案现在覆盖保存、复制、自动保存、自动贴图、二维码、放大镜取色、选区缩放和标注；`pinned_image` 文案覆盖区域截图贴图、拖动、缩放、复制和图片 OCR。设置中心也不再宣称 API key 安全存储后续接入，明确使用 Windows Credential Manager 或环境变量；截图后复制/贴图文案改为当前已接入策略。验证：`go test ./internal/platform -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。
- 最近自定义启动项插件主路径补齐：`internal/plugins` 新增 `custom_launch` manifest，覆盖旧版 `launch` / `start` / `启动项` 关键词；`launch [query]` 不再需要 Python legacy bridge，返回显式 `open_settings` 工具窗口动作，引导用户在设置中心维护启动项，同时实际启动项仍由 `launchers` provider 直接进入主搜索结果。新增测试覆盖 manifest、命令补全、action surface 和非文件动作约束。验证：`go test ./internal/plugins ./internal/search -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。
- 最近系统命令执行层补齐：`run_system` 不再落入普通命令启动器路径，而是由 `platform.ExecuteAction` 映射到受控 Windows 命令；支持锁屏、休眠、清空回收站、关机和重启，全部要求二次确认，未知命令直接拒绝。平台 capability 新增 `system_commands`，说明所有系统命令动作都受二次确认保护。验证：`go test ./internal/platform ./internal/plugins ./internal/workflows -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；测试通过注入 runner 捕获命令映射，没有真实执行锁屏、休眠、清空回收站、关机或重启。
- 最近截图放大镜/取色验证：Computer Use 真实桌面验证截图覆盖层放大镜和颜色 HUD 可见；`C` 复制当前像素为 `rgb(255, 255, 255)`，`Shift` 切换格式后 `C` 复制为 `#FFFFFF`。
- 最近截图标注编辑验证：Computer Use 真实桌面验证已有标注可二次选中、拖动和删除；画矩形后按 `V` 进入选择模式，拖动矩形并保存后结果图显示矩形位于移动后位置，历史 actions 为 `overlay,selection,copy,annotated,rect`；另一次选中矩形后按 `Delete` 保存，历史 actions 回到 `overlay,selection,copy` 且结果图无红色矩形。测试条目和图片均已清理。
- 最近截图文字编辑修复：文字标注创建时 annotation canvas 的 `pointerdown` 已阻止默认按钮抢焦点，文字二次编辑新增显式 `dblclick` 处理，不再只依赖 `PointerEvent.detail`；`go test ./internal/captureoverlay ./internal/pinnedimage`、`pnpm build` 和 `wails3 task windows:build` 均通过。最终桌面复验因 Computer Use 连续返回 `GetCursorPos failed: 拒绝访问 (0x80070005)` 阻断，不能声明文字双击重编辑真实桌面验收通过。
- 最近 Python 旧插件桥验证：新增 `internal/legacybridge` 和显式 `legacy <旧插件关键词> [query]` 插件入口，主搜索不会把旧 Python 插件纳入普通召回；runner 通过 JSON stdout 协议调用旧 `src/plugins`，导入/执行期间的第三方 stdout 噪声隔离到 stderr，结果统一映射为非文件 `plugin_result` 和显式 copy/remember actions。`runner.py` 已嵌入 Go 二进制，源码路径缺失时会写入用户缓存目录再执行。`go test ./internal/legacybridge ./internal/plugins -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；真实旧插件 runner smoke：`c 1+1` 返回 `{"name":"= 2","path":"2","type":"calc_result"}`。
- 最近工作记忆本地 enrichment 验证：`normalizeEntry` 统一补齐本地摘要、内容类型和标签；手动笔记可从 JSON/API/SQL/命令/路径/错误/待办/网络/数据库/配置/代码等线索补全 tags，并把默认 note 细分为 `json`、`sql`、`command`、`error_log`、`url`、`todo`、`file_path` 或 `code`；OCR 写回也复用同一套本地分类，不调用外部 AI。`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。
- 最近工作记忆重复/相似画面合并验证：自动时间机器条目新增 `imageSignature`、`imageFingerprint`、`mergedCount`、`lastMergedAt`；同一 PNG signature 的 `time_machine` 自动截图会合并进已有工作记忆，近似画面会用保守的平均亮度 + 64 位图像指纹汉明距离判断合并，状态分别记录 `LastSkippedReason=重复画面已合并` 或 `LastSkippedReason=相似画面已合并`，预览 metadata 显示已合并次数。手动补记不参与该去重，仍保留用户主动证据。`go test ./internal/workmemory -run "TimeMachine.*(Duplicate|Similar|Different)" -v`、`go test ./internal/workmemory -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 均通过。
- 最近旧版敏感凭据迁移验证：`settings.ImportLegacyConfig()` 现在可把旧版配置中出现的 `ai_api_key` / `openai_api_key` / `embedding_api_key` / `milvus_token` 等明文密钥迁入 Windows Credential Manager 目标 `Ariadne/OpenAI/APIKey`、`Ariadne/Embedding/APIKey`、`Ariadne/Milvus/Token`，不会写入 Ariadne JSON；安全存储不可用时会跳过并在旧配置 notes 中说明。设置中心旧版配置区会显示迁移 notes。验证：`go test ./internal/settings -v`、`go test ./internal/settings ./internal/secrets ./internal/securestore -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:build` 和 `ARIADNE_TEST_CREDENTIAL_MANAGER=1 go test ./internal/securestore -run TestWindowsCredentialManagerRoundTrip -v` 均通过。
- 最近网络监控小窗多屏策略验证：`network-mini` 新增 `screenMode` / `screenId` 持久化配置和状态，支持跟随鼠标所在屏幕、主屏和指定屏幕三种模式；状态区分配置屏幕 ID 与当前实际落点 `activeScreenId`，前端小窗新增短标签屏幕模式按钮，避免固定宽度下溢出。`go test ./internal/toolwindows -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；当前会话未暴露可调用 Computer Use 工具，因此不声明真实多显示器桌面点击验收。
- 最近性能报告：`experiments\ariadne\dist\perf\ariadne-perf-latest.json`
- 最近性能摘要：冷启动 p95 `686ms` / 目标 `800ms`，已通过 target 但未达 `500ms` ideal；平均工作集 `51179520` bytes，release zip 比旧安装器小 `92.12%`，窗口 `760x96` 且 `hasCaption=false`、`hasThickFrame=false`、`isTopmost=true`；Alt+Q 注册探测显示 Ariadne 启动前热键可用、运行中被 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)` 阻止，说明 Ariadne 已占用全局热键；本轮 Win32 合成 Alt+Q 可见+前台 p95 `22ms` / 目标 `120ms`，但仍建议发布前保留真实键鼠焦点落点复验。
- 最近搜索性能报告：`experiments\ariadne\dist\perf\ariadne-search-perf-latest.json`
- 最近搜索性能摘要：`wails3 task windows:search-perf` 记录 `320` 个样本、`16` 个查询，搜索 p95 `9ms` / 目标 `100ms`，rolling service p95 `9ms`，`2400` 个结果 action surface 全部通过，Everything SDK 可用并记录 `1740` 个 Everything 文件结果命中；`Everything64.dll` 查询现在返回显式 `file-search-coverage-hint` / `Everything 未命中文件` 覆盖提示，仍不能替代真实桌面 UI 文件结果命中复验。

### 已完成并有证据

1. Wails 3 + Vue 3 + Vite + TypeScript + Pinia 工程骨架。
2. Ariadne 主搜索壳：Alt+Q 主窗口只承载浅色无边框 launcher；空查询只显示搜索框，输入后展开为结果列表、右侧预览、局部动作和更多菜单，不再把工具中心塞进同一个主窗口。
3. Go 服务边界：`search`、`plugins`、`settings`、`platform`、`workmemory`。
4. Start Menu 应用搜索、Everything 文件搜索 Go provider、自定义启动项、搜索收藏与最近使用。
5. 设置中心、平台诊断、MSIX AppData virtualization 显示。
6. 工作记忆中心，包含真实屏幕时间机器采集后端路径、截图历史证据引用和截图预览字段。
7. 文本剪贴板历史中心和主搜索聚合。
8. 截图历史中心和主搜索聚合。
9. Hosts 管理中心、旧 `.x-tools` profiles 迁移、预览和二次确认写入链路。
10. 工作流宏中心、旧 `x-tools` workflows 迁移、`{clipboard}` / `{input}` / `{prev}` 命令链执行、高风险步骤二次确认和 JSON 导入导出。
11. JSON 对比中心：Go 语义差异服务、Vue 双编辑器页面、格式化、文件导入到左/右、交换、复制报告入口和 `jsondiff`/`open_json_compare` 搜索路由；大输入会保留语义统计/报告，并按预算跳过昂贵行级 diff、截断返回明细或截断超长格式化预览。
12. 桌面壳：Wails SingleInstance、Wails SystemTray、Windows `RegisterHotKey` Alt+Q、窗口关闭隐藏到托盘、设置驱动的 Wails Autostart 钩子和独立工具窗口服务已接入；开机启动注册表写入仍需人工开关烟测。
13. 网络监控中心：Windows IP Helper API 网卡计数、Go 速率差分、`net`/`network`/`网速` 搜索入口、Vue 中心 UI、托盘路由和 `net mini` 贴边小窗已接入；小窗支持四角贴边持久化、跟随鼠标所在屏幕/主屏/指定屏幕模式、锁定尺寸/位置和前台全屏窗口自动隐藏。
14. 主题同步：默认 Graphite Teal light，只有 v7+ 显式 `dark` 才给 `html` 加 `.dark`；`system` 不再作为主题选项，黑色不再作为默认外观。
15. 设置 schema v7 迁移：v7 之前旧实验配置里的 `dark` / `system` 会迁移并写回为 `light`，v7+ 用户显式选择的 `dark` 才保留为深色模式。
16. 二维码识别：新增 Go `qrscan` 服务，支持识别截图历史记录或用户显式触发的当前屏幕；截图历史中心和搜索结果均有显式识别入口。
17. 文本剪贴板自动监听：Ariadne 启动后会建立当前剪贴板基线，只记录启动后的文本变化；隐私模式或关闭剪贴板来源时会暂停监听，剪贴板中心显示监听状态。
18. 图片剪贴板历史：Ariadne 可监听系统剪贴板图片，保存 PNG 到 `clipboard_images`，在剪贴板中心预览，搜索 `图片` / 尺寸可命中，并支持复制图片回系统剪贴板、识别图片二维码和加入截图历史。
19. 工作记忆数据闭环：Go 服务和 Vue 中心已接入手动笔记、敏感内容本地标记、删除当前记忆、清理未收藏记忆、可读 ZIP 导出；导出包含 `README.md`、`timeline.md`、`work_memory.json` 和存在的图片证据，默认跳过敏感条目。
20. 旧版并存诊断：平台状态会检测旧版 `x-tools.exe` 进程、旧配置路径、配置大小和 Alt+Q 冲突可能性；设置中心摘要显示 `旧版运行`。
21. 启动器设置入口：`设置` / `settings` / `config` / `theme` 会返回 `设置中心`，且排序高于 Everything 文件结果。
22. 贴图窗口：新增 Go `pinnedimage` 服务，截图历史、剪贴板图片和二维码结果可创建独立置顶无边框贴图窗口；主窗口不会因贴图窗口路由被重置为 launcher。
23. 区域截图覆盖层：新增 Go `captureoverlay` 服务，`shot` / `区域截图` 从启动器打开全屏截图覆盖层；拖拽选区后可保存到截图历史、创建贴图或识别二维码，主窗口会在覆盖层关闭后恢复。
24. 默认浅色边界：主窗口、工具中心和覆盖层控件默认使用 Graphite Teal 浅色 token；黑色/深色控件只在 `.dark` 深色模式下启用，截图覆盖层仅保留功能性暗化遮罩。
25. 本地 OCR：新增 Go `ocr` 服务和嵌入式 RapidOCR bridge，支持截图历史、当前屏幕、剪贴板图片和工作记忆图片识别；工作记忆写回 OCR 文本并受敏感内容策略约束。
26. OCR 行级文本选择：截图历史、剪贴板历史和工作记忆的 OCR 结果面板支持逐行选择、全选、清空、复制选中和复制全文，并显示每行置信度与坐标。
27. OCR 图片叠框选择：截图历史、剪贴板历史和工作记忆图片预览共用 `OCRImageOverlay`，OCR 行框可在图片上直接点击选择；叠框命中层已扩大并走 `pointerdown`，避免 WebView 空按钮点击丢失。
28. 贴图 OCR 联动：截图/剪贴板贴图窗口会显示 OCR 按钮，识别后在贴图上显示可点选 OCR 行框，并提供全选、清空、复制选中和复制全文；二维码贴图不暴露 OCR。
29. 截图高级编辑：区域截图覆盖层支持选区缩放手柄、矩形、直线、箭头、画笔、马赛克、文字、序号、橡皮、颜色/粗细调节、撤销/重做、清空、已有标注选择/拖动/删除、文字双击重编辑、保存到截图历史、另存为外部 PNG、贴图和二维码识别；前端会按原生像素渲染最终 PNG，后端保留标注操作兜底渲染并写入截图历史 actions。
30. 图片 OCR 索引底座：新增 Go `imageindex` 服务，批量索引最近截图历史和剪贴板图片 OCR 文本，敏感 OCR 结果默认屏蔽正文；`img index` / `ocr index` / `图片索引` 可从启动器触发索引，已索引的非敏感 OCR 文本会进入主搜索。
31. 工作记忆本地语义检索：`workmemory.Search` 已接入本地 token/短语向量相似度，支持中英技术词 alias（数据库、连接、报错、网关、代理、截图/OCR 等），语义命中会在预览证据中标注 `匹配=本地语义匹配`；`SemanticStatus` 明确 provider 为 `local_term_vector`，不伪装为外部 embedding/Milvus。
32. 工作记忆与图片 OCR 索引保留策略：`workMemory.retentionDays` / `keepFavoritesForever` 现在会真实清理过期非收藏工作记忆；图片 OCR 索引会清理过期派生索引和源截图/剪贴板图片已不存在的 stale 索引。
33. 剪贴板历史与截图历史保留策略：`workMemory.retentionDays` / `keepFavoritesForever` 现在也会清理过期未置顶截图、剪贴板文本和剪贴板图片；置顶条目按设置保留，截图 PNG 和剪贴板图片文件会走现有安全删除逻辑。
34. 截图历史与剪贴板图片缩略图分层：新写入的大图会生成 `capture_thumbnails` / `clipboard_thumbnails` 预览 PNG；服务加载时会回填旧大图缺失的缩略图；历史中心预览优先读缩略图，打开、复制、OCR、贴图仍使用原图；删除、清空、裁剪和保留策略会联动清理缩略图。
35. 工作记忆时间机器执行链路：后台 worker 会按设置 interval 自动采集，设置变更会重启 worker；排除应用/窗口标题规则会在截图前阻断采集并记录跳过状态，工作记忆中心会显示 worker、暂停和跳过原因。
36. 工作记忆时间机器保护策略：Windows `GetLastInputInfo` 与输入桌面状态已接入自动时间机器，支持空闲暂停、锁屏暂停、采集范围/多屏策略状态记录；手动补记不被空闲暂停拦截，设置中心可见 `空闲暂停`、`锁屏暂停`、`空闲阈值秒`、`采集范围` 和 `多屏策略`。
37. 截图采集范围真实执行链路：`capturehistory.CaptureScreenWithOptions` 已接入全部屏幕、主屏幕、前台窗口和按显示器分条采集；工作记忆时间机器会把 `captureScope` / `multiMonitor` 传给截图历史服务，截图记录和工作记忆证据都会写入范围、多屏和区域 metadata。
38. 工作记忆自动 OCR 策略：`workMemory.autoOcr` 会同步到 `CapturePolicy`，自动时间机器和手动补记在生成截图型工作记忆后会调用本地 OCR 服务写回 `ocrText` / `ocrStatus`，状态中记录最近自动 OCR 成功或失败；敏感条目仍按 OCR 服务策略阻断。
39. 工作记忆前台窗口切换触发：时间机器 worker 会轮询 Windows foreground window，先建立窗口基线，再在窗口签名变化且满足冷却时间时触发 `CaptureTimeMachineWindowSwitch()`；设置中心可持久化 `windowSwitchCaptureEnabled` 和 `windowSwitchCooldownSeconds`，状态面板会显示窗口切换触发/冷却秒数。
40. 工作记忆 SQLite FTS：工作记忆 JSON 仍是权威数据源，旁路生成可重建的 `work_memory.fts.sqlite`；`Search` 会优先使用 SQLite FTS5 命中和 snippet 证据，再回退到内存关键词与本地 token/短语语义检索；`SemanticStatus` 明确显示 `sqlite_fts5+local_term_vector`，仍不伪装外部 embedding/Milvus。
41. 工作记忆本地经验发现首版：`DiscoverExperiences` 会从最近工作记忆中按证据归纳重复问题、自动化机会和知识沉淀缺口，跳过敏感条目；工作记忆中心已接入 `经验发现` 侧栏、`发现经验` 按钮和 `转任务包` 动作。当前是本地可解释规则，不冒充外部 AI 经验发现。
42. 工作记忆经验发现决策闭环：每条 insight 可被标记为 `accepted`、`rejected`、`later` 或 `task_package`，状态持久化到 `work_memory.json` 的 `experienceDecisions`，重新生成报告和重启后都会回填到 insight；前端提供接受、稍后、驳回、转任务包四个局部动作。
43. 旧版历史数据迁移首版：新增 `internal/migration` 服务和设置页入口，读取旧 `%APPDATA%\x-tools` 下的 `clipboard_history.json`、`capture_history.json`、`work_memory\entries.json`，只复制历史和图片到 Ariadne 数据目录，不删除或改写旧数据；支持状态预览、dry-run、导入结果统计和重复导入去重。
44. 工作记忆候选工作流/检查清单草稿：`GenerateWorkflowDraft` 和 `GenerateChecklistDraft` 可从经验线索 evidence 生成 review-only 草稿；工作记忆中心右侧新增 `候选工作流`、`检查清单` 面板，并在 insight 动作中提供 `转工作流` / `转清单`，不会自动保存为正式工作流或外发内容。
45. 搜索性能与 Everything 诊断：`search.Service` 维护最近 200 次非空查询滚动耗时，平台诊断暴露搜索 p95/平均/最近耗时和目标 100ms；`filesearch.Service` 暴露 Everything DLL、ready、最近查询耗时、结果数和错误，设置中心平台诊断可见这些状态。
46. 发布回滚检查点首版：新增 `internal/release` 服务和设置中心入口，可统计 Ariadne 标准/虚拟化本地数据目录并创建 `ariadne-rollback-*.zip`，zip 内包含 `manifest.json`、数据文件和恢复说明；该动作只打包 Ariadne 数据，不删除、不覆盖旧版 x-tools 数据。
47. 发布包首版：新增 `internal/releasepack` 和 `cmd/releasepack`，`wails3 task windows:package` 会构建 Ariadne 并生成用户级 Windows release zip，内含 `app/ariadne.exe`、品牌图标、`manifest.json`、`scripts/install.ps1`、`scripts/uninstall.ps1` 和 README；安装脚本默认安装到 `%LOCALAPPDATA%\Programs\Ariadne`，检测旧版 x-tools 并提示 Alt+Q 并存冲突，不删除旧版或用户数据。
48. 启动器与工具窗口边界修正：新增 `internal/toolwindows` 服务；工作记忆、剪贴板、截图历史、Hosts、工作流、JSON 对比、网络监控和设置从 launcher 启动时打开独立 frameless 工具窗口，主 launcher 隐藏并保持搜索器职责；托盘工具入口也走独立工具窗口，不再把主窗口膨胀成后台面板。
49. 本地日志与诊断包导出：新增 `internal/applog`，标准日志写入 `%APPDATA%\Ariadne\logs\ariadne.log`；`platform.Status()` 暴露日志路径、大小、错误和 `diagnostic_logs` capability；设置中心平台诊断可触发 `ExportDiagnostics()`，生成包含 `platform_status.json`、`metrics.json` 和日志文件的诊断 zip。
50. 旧版并存确认式交接：`platform.ResolveLegacyConflict()` 支持设置中心二次确认后关闭旧版 `x-tools.exe` 并调用 shell hotkey retry；默认先对旧版窗口发送 `WM_CLOSE`，只有用户明确点强制结束才终止进程。发布包 manifest/README 已提示可在 Ariadne 设置中心执行交接，不静默删除旧版或旧数据。
51. 回滚检查点确认式恢复：`release.RestoreRollbackCheckpoint()` 支持设置中心二次确认后恢复最近检查点；恢复前自动创建 `pre_restore` 检查点，只恢复当前 Ariadne 数据根，保留 backups 目录，不解压到旧版 x-tools 数据目录。
52. 用户级安装脚本快捷方式与 receipt 清理：release `install.ps1` 支持重定向 `StartMenuDir` / `DesktopDir` 进行安全烟测，安装时写入 `install_receipt.json`，`uninstall.ps1` 会读取 receipt 并清理开始菜单、卸载和桌面快捷方式；脚本烟测不触碰真实用户开始菜单。
53. 工作记忆候选工作流确认保存：`internal/workflows.SaveWorkflowDraft()` 将工作记忆经验发现生成的 `WorkflowDraft` 转为正式工作流，未确认时只返回确认要求和风险原因，确认后写入 `workflows.json`；工作记忆中心候选工作流面板新增两次确认的 `保存到工作流` 局部动作。
54. 工作记忆检查清单确认保存：新增 `internal/checklists` 服务，`SaveChecklistDraft()` 将工作记忆经验发现生成的 `ChecklistDraft` 转为正式检查清单资产，未确认时只返回确认要求和风险原因，确认后写入 `checklists.json`；工作记忆中心检查清单面板新增两次确认的 `保存为清单` 局部动作。
55. 工作记忆本地 Skill 确认保存：新增 `internal/skills` 服务，`SaveSkillDraft()` 将工作记忆知识草稿转为本地可复用 Skill 资产，未确认时只返回确认要求和风险原因，确认后写入 `skills.json`；工作记忆中心知识面板新增两次确认的 `保存为 Skill` 局部动作。
56. Codex Skill 包导出：`internal/skills.ExportSkillPackage()` 可将已确认的本地 Skill 资产导出为 `skill_exports/<skill-id>/SKILL.md` 和 `<skill-id>.zip`，zip 内保留 `<skill-id>/SKILL.md` 标准结构；导出前二次确认。
57. Codex Skill live 安装：`internal/skills.InstallSkillPackage()` 可在二次确认后把已确认本地 Skill 写入 live Codex skills 发现目录，默认目标为 `$CODEX_HOME\skills` 或 `%USERPROFILE%\.codex\skills`；默认不覆盖已存在目录，前端知识面板的 `安装到 Codex` 第二次点击会显式带 `overwrite=true`，并保留本地风险提示。确认安装后会写入 Ariadne refresh marker，供运行时或后续工具检测 newly installed skill；`internal/skills.InstallDiagnostics()` 可只读扫描目标目录、解析 `SKILL.md` 并核验 marker/manifest 一致性。
58. Wails Windows 启动硬化：`go.mod` 使用本地 `third_party/wails/v3` replace，只补 Wails v3.0.0-alpha.98 的 Windows screen 枚举路径；当 Codex 启动上下文中 `GetCursorPos` 不可用时，screen cache 回退到主屏而不是 fatal 退出。
59. 可重复性能验收工具：新增 `internal/perfcheck` 和 `cmd/perfcheck`，`wails3 task windows:perf` 会启动最新 `bin\ariadne.exe` 做 Win32 冷启动/窗口样式/工作集采样、Alt+Q 注册探测和合成 Alt+Q 唤起采样，并对比 Ariadne release zip、旧版 `x-tools-setup.exe` 和旧版 onedir 包体积，结果写入 `dist\perf\ariadne-perf-latest.json`；测试后会停止其启动的 Ariadne 进程。当前热键注册证据有效，最新合成 Alt+Q 可见+前台 p95 已通过 `120ms` target；仍需 Computer Use 或真实键盘补真实焦点落点。
60. 工作记忆本地日报草稿（含复盘线索）：`GenerateDailyDraft()` 现在会基于当天非敏感工作记忆生成结构化本地日报，包含今日概览、主要工作、待跟进、复盘线索、隐私边界和证据 ID；如果当天没有记录会降级使用最近非敏感记录。敏感记忆会被跳过，不进入正文或 evidence；工作记忆中心日报面板改为可滚动的保留换行文本。
61. 可重复搜索 p95 基准工具：新增 `internal/searchbench` 和 `cmd/searchbench`，`wails3 task windows:search-perf` 默认用临时 `APPDATA/LOCALAPPDATA` 组装真实 search provider 栈，运行固定查询套件、warmup、多轮采样、分查询 p95、慢样本、rolling `PerformanceStatus()`、Everything 状态和 `contracts.ValidateActionSurfaces` 校验，结果写入 `dist\perf\ariadne-search-perf-latest.json`；CLI 保留 `-real-appdata` 用于需要真实用户数据样本时手动开启。
62. 启动器搜索查询取消/过期响应防护：`frontend/src/services/ariadneApi.ts` 新增 `createAriadneSearchRequest()`，保留 Wails `CancellablePromise.cancel()` 能力并对取消错误做显式归类；`frontend/src/stores/launcher.ts` 对每次查询递增 serial，发起新查询或 reset 时取消上一条请求，旧请求即使完成也不能写回结果、选中项或耗时。
63. Provider 级搜索取消：`search.Service.Search` 已改为 Wails 可注入的 `context.Context` 首参，前端取消 Wails `CancellablePromise` 后 Go 聚合器会停止后续 provider；provider 可选实现 `SearchContext`，Everything 文件搜索已接入 context 检查，取消查询不会污染搜索 p95 样本或 Everything 最近查询诊断。
64. 网络监控贴边小窗首版：新增独立 `network-mini` 工具窗口，搜索 `net mini` / `net 小窗` 和网络监控中心 `小窗` 按钮可打开无边框小窗；早期实现为 `318 x 168` 右下贴边，当前有效实现已改为 x-tools 风格 `156 x 40` 任务栏左侧小条，并通过 Windows taskbar owner 进入任务栏层级；托盘菜单保留 `网速小窗`。
65. 网络监控小窗位置与全屏隐藏：`toolwindows` 新增 `network_mini_window.json` 持久化配置、四角 anchor、非法 anchor 拒绝、默认全屏自动隐藏和 Windows 前台全屏窗口探测；小窗 UI 可切换贴边角落和全屏隐藏开关，后端 watcher 会在前台窗口全屏时隐藏 `network-mini`，退出全屏后恢复。
66. 插件命令补全与参数面板：插件 trigger 结果现在显式携带 `CommandSchema`、completion keyword 和 `prepare_command` action；launcher Enter 会把命令前缀或示例写回搜索框，右侧预览展示参数字段、必填/可选状态、命令草稿和示例填入按钮，仍遵守结果显式 action 协议。
66a. 自定义启动项插件入口：`custom_launch` 已迁移为 Go 插件 manifest，`launch` / `start` / `启动项` 提供设置中心管理入口和命令补全，不再依赖 Python legacy bridge；实际启动项命中仍由 `launchers` provider 直接返回，避免重复实现启动项搜索。
66b. 系统命令执行层：`system_commands` 已接入平台 capability 和受控执行映射，`sys lock` / `system sleep` / `sys empty` / `sys shutdown` / `sys restart` 都返回显式 `run_system` action；所有系统命令都必须二次确认，关机/重启同时标记为 danger，未知命令不会被当作普通本机命令执行。
67. 工作记忆材料导入入口：`workmemory.ImportMaterials()` 支持用户显式路径导入 Markdown/文本、图片、PDF、Office 文档和 Ariadne 工作记忆导出 zip；图片和 zip evidence 会复制/提取到 `work_memory_images`，docx/xlsx/pptx 会从本地 zip/xml 提取可搜索正文，PDF 做本地 best-effort 文本提取，隐私模式会阻断导入，前端工作记忆中心“数据包”面板已接入路径、标签、收藏和敏感标记入口。
68. 工作记忆排除规则执行层补强：`CapturePolicy` 已接入排除路径、URL 域名/路径和排除内容模式；用户显式导入命中排除路径/URL/内容会跳过，OCR 写回命中排除 URL/内容会标记 `blocked_excluded` 且不写入正文，导出包会跳过被排除条目并在结果与 `work_memory.json` / `README.md` 中记录 `skippedExcludedCount`。
69. 工作记忆筛选导出：`ExportDataWithOptions()` 新增指定最近时间范围、标签和条目 ID 的导出路径；导出结果会返回 `filteredOutCount` 和 `filter`，导出包的 `work_memory.json` / `README.md` 会记录筛选条件和筛出数量；前端工作记忆中心“数据包”面板新增最近天数、标签和条目 ID 输入，不填时保持原有全量导出语义。
70. 工作记忆中心排除规则配置界面：右侧新增 `排除规则` 面板，可编辑排除应用、窗口关键词、路径片段、URL 域名/路径和内容正则；保存时复用 settings store 的 `updateWorkMemoryRuntime()`，由 settings change handler 注入 `CapturePolicy`，规则会继续优先于采集、OCR、导入、导出和经验发现。
71. 工作记忆时间机器回放视图：工作记忆中心新增 `时间机器回放` 面板，按截图型工作记忆的 `captureId` 时间升序组织帧，支持定位最近、上一帧、下一帧；当前帧会同步选中详情区并通过 `getCaptureImageDataURL()` 加载截图预览，无截图帧时按钮禁用并显示空状态。
72. 工作记忆独立问题复盘草稿：新增 `GenerateRetrospectiveDraft(ids)`，可基于用户选定的非敏感工作记忆生成本地问题复盘草稿，包含复盘概览、问题背景、时间线、初步原因、处理过程、遗留风险、隐私边界和证据 ID；敏感记忆会跳过并记录跳过数量。工作记忆中心详情动作区新增 `复盘草稿` 按钮，右侧新增独立 `复盘` 面板，不再只把复盘线索塞进日报。
73. 工作记忆复盘证据多选：左侧时间线新增复盘证据组选择，点击条目仍只打开详情，圆形勾选专门用于加入/移出复盘证据；`选择筛选` 会选择当前筛选中的前 12 条非敏感记忆，敏感记忆显示红色禁用状态且不会进入复盘证据。`复盘草稿` 按钮优先使用证据组，没有证据组时回退当前详情记忆，反馈使用后端返回的真实 evidence 数量。
74. 工作记忆本地定期草稿调度：settings schema 升到 v8，新增 `draftScheduleEnabled`、`draftScheduleIntervalMinutes`、定期日报、定期复盘和定期经验发现开关；`workmemory.ApplyDraftSchedule()`、`ScheduledDraftStatus()` 和 `RunScheduledDraftsNow()` 会按策略生成本地日报、复盘草稿和经验发现报告，只使用非敏感工作记忆 evidence，不调用外部 AI。工作记忆中心新增 `定期草稿` 状态/手动运行面板，设置中心新增对应开关和间隔配置；隐私模式或工作记忆禁用时会暂停调度。
75. 工作记忆 AI 日报润色首版：新增 `internal/aiclient` OpenAI-compatible chat completion 客户端，`workmemory.PolishDraft()` 只在 AI 设置启用、隐私模式关闭且用户二次确认后外发当前草稿；API key 从用户环境变量优先读取，缺省时回退到 Windows Credential Manager，不写入 Ariadne `config.json` 或仓库。工作记忆中心日报面板新增 `AI 润色` / `确认外发润色` 局部按钮，成功后用润色草稿替换当前日报草稿并保留 evidence ID。
76. 工作记忆外部 embedding + 向量存储：新增 OpenAI-compatible embedding 客户端、`RefreshEmbeddingIndex()`、`SemanticSearchExternal()`、工作记忆中心 `语义索引` 面板、embedded 本地 `work_memory_vectors.json` 缓存和 Milvus REST 向量存储适配器；索引只处理非敏感工作记忆，隐私模式会阻断刷新/搜索，API key 从用户环境变量优先读取，缺省时回退到 Windows Credential Manager。Milvus collection 使用 Ariadne 专用 schema 和按本机 `work_memory.json` 路径生成的 namespace，刷新时先清理当前 namespace 再 upsert，搜索时用 namespace filter 并回填本地 entry，避免不同数据目录互相污染。
77. 工作记忆外部 AI 经验发现：新增 `ExperienceDiscoverer` 接口和 OpenAI-compatible chat completion 适配器，`DiscoverExperiencesAI()` 只在 AI 设置启用、`experienceDiscoveryEnabled` 打开、隐私模式关闭且用户二次确认后外发非敏感 evidence 摘要；外发 payload 不包含截图路径、图片文件或完整内部结构，并会对未标敏但疑似 token/password/key 的条目再做兜底过滤。工作记忆中心保留本地 `发现经验` 按钮，新增 `AI 发现` / `确认外发发现` 局部按钮和风险提示；设置中心新增 `AI 经验发现` 开关；后台定期经验报告仍只使用本地规则，AI 失败时返回本地规则报告降级但不冒充 AI 输出。
78. Codex Skill 安装刷新握手：`internal/skills.InstallSkillPackage()` 在确认安装写入 `SKILL.md` 后，会在 Codex skills 根目录写入 `.ariadne-refresh.json` 和 `.ariadne-refresh.touch`，并在 `InstallResult` 返回 `refreshRequested`、`refreshManifest`、`refreshMarker`；工作记忆中心安装成功后显示刷新握手路径并调用 `InstallDiagnostics()` 展示本地发现/握手状态。该能力提供本地可检测信号，供 Codex runtime 或后续工具发现 newly installed skill；当前仍不能证明正在运行的 Codex 已热加载该 skill。
79. JSON 对比大输入性能预算：`internal/jsoncompare.CompareJSONText()` 对统一行 diff 增加 LCS 行数乘积预算，超过预算时跳过行级 diff 但继续返回语义差异统计和 report；返回给前端的差异明细按 2000 条截断，格式化预览按 240 KiB 截断，并返回 `diffTruncated`、`differencesTruncated`、`formattedTruncated`、`performanceNote`，JSON 对比中心会显示性能预算提示。
80. 启动器窗口行为修复：主 `Ariadne` launcher 不再在 Go 创建、shell 唤起、toolwindows 回退和前端 resize/mount 时设置 always-on-top；Esc 会隐藏 launcher；launcher 和 tool-window 文档启用 Wails3 `--wails-draggable: drag`，输入框、按钮、列表、编辑区、截图覆盖层和贴图区域显式 `no-drag`；`ShellStatus` 改为 camelCase JSON 字段并新增截图热键状态。
81. 截图热键运行态接入：`shell.Manager` 现在同时管理启动器热键和截图热键，初始读取 settings 中的 `toggleWindow` / `screenshot`，设置保存后调用 `ApplyHotkeys()` 重新注册；Alt+A 通过 Go 回调直接打开 `captureoverlay.Open()`，不再只停留在设置 UI 字段。
80. JSON 文件导入入口：JSON 对比中心新增 `文件到左侧` / `文件到右侧` 按钮，通过 WebView 文件选择读取 `.json` / JSON 文本文件，写入对应编辑器后复用现有 compare 和局部反馈路径；不需要新增后端文件权限，也不触碰系统目录。
81. JSON 拖放导入入口：JSON 对比中心左右编辑面板支持拖入 JSON 文件或纯文本/JSON 文本，拖放时使用 Graphite Teal 高亮投放区域，落入后复用同一侧写入、compare 和局部反馈路径。
82. 搜索收藏/最近使用清理入口：`search.Service.ClearUsage()` 会清空 `search_state.json` 中的收藏与最近使用记录，失败时恢复内存状态；设置中心新增“搜索数据”面板，显示状态路径、记录数、前 5 条记录和二次确认的清理按钮。
83. Everything 文件元数据增强：文件搜索结果会对命中路径执行本地 `os.Stat` 元数据读取，普通文件在 preview meta 和 payload 中暴露大小、修改时间、类型和来源，目录会显示 `目录` 标签与 folder icon，元数据不可读时保留结果并标注元数据不可用；这不改变文件动作协议，非文件结果仍不会继承文件动作。
84. Windows 品牌资源嵌入：新增 Graphite Teal Ariadne 图标源 `assets/ariadne-icon.svg`，前端 `favicon.svg` 与 release `logo.png` / `logo.ico` 同源生成；`winres/winres.json` 和 `wails3 task windows:resources` 会生成 Go 可链接的 `ariadne_resource.syso`，把 Ariadne 图标、manifest 和 VersionInfo 嵌入 `ariadne.exe`，避免继续暴露旧 `x-tools` 或默认 Wails 品牌。
85. 网络监控小窗多屏策略：`toolwindows` 已把小窗屏幕选择从主屏假设升级为 `cursor` / `primary` / `screen` 三种持久化模式；`cursor` 模式按 Windows cursor 所在 physical bounds 选择显示器，失败时回退主屏；旧配置只有 `screenId` 时会作为指定屏幕模式兼容加载。当前任务栏小条 UI 移除了角落/屏幕模式按钮，后端仍保留旧 anchor 和屏幕模式兼容。
85. 冷启动路径轻量化：启动时只同步注入 AI/embedding runtime，工作记忆 runtime、保留策略和剪贴板监听改为首屏后延迟维护；各保留策略无删除时不再无意义保存，持久化工作记忆空库不再写 demo seed，SQLite FTS 改为有真实条目时初始化或搜索时懒加载。最新 `wails3 task windows:perf` 记录冷启动 p95 `686ms`，已低于 `800ms` target。
86. Windows Credential Manager 密钥存储：新增 `internal/securestore`、`internal/secrets` 和设置中心“安全密钥存储”面板，支持 AI API key、embedding API key 和可选 Milvus token 的状态查看、保存、二次确认清除；运行时解析顺序为环境变量优先、Credential Manager 回退，界面和日志不回显密钥正文。

### 最近验证

1. `go test ./...` 通过。
2. `pnpm build` 通过。
3. `wails3 generate bindings` 通过。
4. `wails3 task windows:package` 通过并生成上述 exe 与 release zip。
5. 本轮检查清单资产保存验证：新增 `internal/checklists` 单测覆盖“未确认不持久化”“确认后写入并可 reload”“无效草稿拒绝”；`go test ./internal/checklists -v` 和 `go test ./...` 通过；`wails3 generate bindings` 更新为 `462 Packages, 21 Services, 146 Methods, 5 Enums, 131 Models, 0 Events`；`pnpm build` 和 `wails3 task windows:package` 通过，release manifest 中 `app/ariadne.exe` 为 `30420480` bytes、sha256 `13f68df1ecac573b30425a0cda36030837f7fa2e88d8ef3534a99dc33a401cb7`，release zip 为 `16290649` bytes。Computer Use 已尝试启动最新 `bin\ariadne.exe` 做桌面复验，但没有出现 targetable Ariadne 窗口且无残留进程，日志只记录启动行；本项不计为桌面点击通过。
6. 本轮本地 Skill 资产保存验证：新增 `internal/skills` 单测覆盖“未确认不持久化”“确认后写入并可 reload”“无效草稿拒绝”；`go test ./internal/skills -v` 和 `go test ./...` 通过；`wails3 generate bindings` 更新为 `463 Packages, 22 Services, 150 Methods, 5 Enums, 135 Models, 0 Events`；`pnpm build` 和 `wails3 task windows:package` 通过，release manifest 中 `app/ariadne.exe` 为 `30460928` bytes、sha256 `b71300d5993fabd8a6e8aa87ea4d53c87f91216a4a221bf7a0faf478171a21db`，release zip 为 `16307433` bytes。延续本轮 Computer Use 桌面限制：新增保存按钮未声明为桌面点击通过。
7. 本轮 Codex Skill 包导出验证：新增 `ExportSkillPackage` 单测覆盖“未确认不写导出文件”“确认后生成 SKILL.md”“zip 内包含 `<skill-id>/SKILL.md`”“未知 Skill 拒绝导出”；`go test ./internal/skills -v` 和 `go test ./...` 通过；`wails3 generate bindings` 更新为 `463 Packages, 22 Services, 151 Methods, 5 Enums, 137 Models, 0 Events`；`pnpm build` 和 `wails3 task windows:package` 通过，release manifest 中 `app/ariadne.exe` 为 `30488064` bytes、sha256 `a6abd6335d2bd808179bee9c0248de86d047078d5387be42ef62a4305ec70a38`，release zip 为 `16319397` bytes。Computer Use 再次尝试启动最新 `bin\ariadne.exe`，仍没有 targetable Ariadne 窗口且无残留进程，日志只记录启动行；本项不计为桌面点击通过。
8. 本轮 Codex Skill live 安装与 Wails 启动硬化验证：新增 `InstallSkillPackage` 单测覆盖“未确认不写目标目录”“确认后写入 live skill 目录结构”“已有同名目录默认要求覆盖确认”“显式 overwrite 后重写 SKILL.md”“未知 Skill 拒绝安装”；`go test ./internal/skills -v` 和 `go test ./...` 通过；`wails3 generate bindings` 更新为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`；`pnpm build` 和 `wails3 task windows:package` 通过。release manifest 中 `app/ariadne.exe` 为 `30507520` bytes、sha256 `be60b17fb01e5e704bfee303e159ce3f70c667ca1a533bbdd34dfb9b9d93f8e1`，release zip 为 `16329351` bytes，最终时间 `2026-06-14 14:04:56`。Win32 桌面烟测启动最新 `bin\ariadne.exe` 后进程保持运行，主窗口标题 `Ariadne`，尺寸 `760x96`，`hasCaption=false`，`hasThickFrame=false`，`isTopmost=true`；测试后已停止该进程。
9. 本轮可重复性能验收验证：新增 `internal/perfcheck` 单测覆盖 p95/平均汇总、包体积对比、热键预算和失败样本告警；`go test ./internal/perfcheck -v` 与 `go test ./...` 通过；`wails3 generate bindings` 仍为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`；`pnpm build` 与 `wails3 task windows:package` 通过，最新 exe 时间 `2026-06-14 14:19:35`，release zip 时间 `2026-06-14 14:19:36`。最新 `wails3 task windows:perf` 记录冷启动 p95 `581ms`、平均工作集 `42916523` bytes、release zip `16329351` bytes、旧版安装器 `209328192` bytes、旧版 onedir `348156892` bytes、release zip 减少 `92.20%`；三次冷启动样本窗口均为 `760x96`、`hasCaption=false`、`hasThickFrame=false`、`isTopmost=true`。Alt+Q 注册探测记录 `beforeAvailable=true`、`duringBlocked=true`、`duringErrorCode=1409`，说明 Ariadne 已占用全局热键；三次合成 Alt+Q 样本均未在 `8000ms` 内让隐藏 launcher 变为可见且前台，本项不计为 Alt+Q 唤起耗时通过。测试后确认无残留 Ariadne 或 Go 进程。
10. 本轮本地日报草稿（含复盘线索）验证：`GenerateDailyDraft()` 已从固定说明文升级为本地结构化日报，按当天/近期非敏感工作记忆整理概览、主要工作、待跟进、复盘线索、隐私边界和证据 ID；新增单测覆盖敏感记忆不进入 body/evidence、待跟进和复盘章节存在。`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build`、`wails3 task windows:package` 均通过；bindings 仍为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`。最新 exe 时间 `2026-06-14 14:35:39`、大小 `30536192` bytes，release zip 时间 `2026-06-14 14:35:40`、大小 `16342857` bytes。最新 `wails3 task windows:perf` 记录冷启动 p95 `548ms`、平均工作集 `43110400` bytes、release zip 比旧安装器小 `92.19%`，Alt+Q 注册探测仍为 `beforeAvailable=true`、`duringBlocked=true`、`duringErrorCode=1409`；合成 Alt+Q 唤起耗时样本仍未通过。测试后确认无残留 Ariadne 或 Go 进程。
11. 本轮可重复搜索 p95 基准验证：新增 `internal/searchbench` 单测覆盖 p95/分查询汇总、Everything 文件结果识别、慢样本和 action surface 失败告警；`go test ./internal/searchbench -v` 与 `go test ./...` 通过。`wails3 task windows:search-perf` 生成 `dist\perf\ariadne-search-perf-latest.json`，报告时间 `2026-06-14 14:45:13`，大小 `108154` bytes；本机记录 `320` 个样本、`16` 个查询、搜索 p95 `7ms` / 目标 `100ms`、rolling p95 `7ms`、`2380` 个结果动作校验 `0` 失败，Everything DLL `P:\workspace\glwlg\app\x-tools\Everything64.dll` 可用且跨查询记录 `1740` 个 Everything 文件结果命中。`Everything64.dll` 查询本身仍为 `0` 结果，因此真实桌面 UI 文件结果命中和索引覆盖提示仍需补。
12. 本轮启动器搜索查询取消验证：`go test ./...`、`pnpm build` 通过；`wails3 task windows:build` 通过并重新生成 bindings，仍为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`。该轮 exe 为 `30536704` bytes，时间 `2026-06-14 14:50:12`。该轮只声明前端类型/构建验证和 Wails build 验证，未声明真实快速输入桌面验收；后端 provider 级 `context.Context` 中断已在后一条验证中补齐。
13. 本轮 provider 级搜索取消验证：新增 `search.ContextResultProvider`、Wails `Search(context.Context, query)` 入口、Everything `SearchContext` 和取消不污染诊断逻辑；新增单测覆盖预取消不调用 provider、context-aware provider 取消后不再调用剩余 provider、取消不记录搜索性能样本、取消不写 Everything 最近查询，以及 context-aware Everything client 被优先调用。`go test ./internal/search ./internal/filesearch ./internal/searchbench -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:search-perf` 均通过；bindings 仍为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`，生成的前端 `Search(query)` 参数形态不变。最新 exe 为 `30544896` bytes，时间 `2026-06-14 14:59:05`；最新搜索报告大小 `108169` bytes，时间 `2026-06-14 14:59:24`，仍为 `320` 样本、p95 `7ms`、rolling p95 `7ms`、`2380` 个 action 校验 `0` 失败、Everything 文件结果命中 `1740`。
14. 本轮网络监控贴边小窗验证：新增 `network-mini` 工具窗口、插件结果和托盘入口；`go test ./internal/toolwindows ./internal/plugins ./internal/shell -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；bindings 仍为 `463 Packages, 22 Services, 152 Methods, 5 Enums, 139 Models, 0 Events`。该轮 exe 为 `30556672` bytes，时间 `2026-06-14 15:08:24`。本轮工具层单测覆盖 `network-mini` 接受、固定尺寸、最大尺寸、置顶、禁用 resize 和右下 work area 贴边定位；前端构建覆盖 `NetworkMiniWindow.vue`、`open_network_mini` action 和中心页 `小窗` 按钮。该轮没有声明 Computer Use 桌面点击通过；位置持久化和全屏隐藏代码路径已由下一条验证补齐。
15. 本轮网络监控小窗位置持久化与全屏隐藏验证：新增四角 anchor、`network_mini_window.json` 持久化、`NetworkMiniStatus` / `SetNetworkMiniAnchor` / `SetNetworkMiniAutoHideFullscreen` Wails API、Windows 前台全屏窗口 watcher 和小窗内角落/全屏隐藏切换按钮；新增单测覆盖四角坐标、默认右下+全屏隐藏、anchor 持久化、非法 anchor 不落盘、全屏隐藏开关持久化，以及旧配置缺失 `autoHideFullscreen` 时保留默认开启。`go test ./internal/toolwindows ./internal/plugins ./internal/shell -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；bindings 更新为 `463 Packages, 22 Services, 157 Methods, 5 Enums, 140 Models, 0 Events`，最新 exe 为 `30592000` bytes，时间 `2026-06-14 15:20:23`。Win32 临时 `APPDATA` 烟测可启动最新 exe 并创建 WebView launcher，但当前会话 `System.Windows.Forms.SendKeys.SendWait('net mini')` 返回 `Access is denied`，因此本项仍不声明真实桌面点击打开小窗或真实全屏隐藏验收；烟测进程已在 finally 中终止，随后确认无 `ariadne` / `go` 残留进程。
16. 本轮插件命令补全与参数面板验证：`plugins.triggerResults` 会给全部内置插件 trigger 输出 `commandSchema` payload、completion keyword 和 `prepare_command` action，`calculator` completion 优先使用 usage 中的 `calc` 而不是短别名 `c`；launcher store 接住 `prepare_command` 后写回搜索框，右侧预览新增参数字段、命令草稿和示例填入。`go test ./internal/plugins ./internal/search -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；bindings 仍为 `463 Packages, 22 Services, 157 Methods, 5 Enums, 140 Models, 0 Events`，最新 exe 为 `30600704` bytes，时间 `2026-06-14 15:36:57`。本轮未声明 Computer Use/Browser 真实渲染验收，因为当前只暴露线程/自动化/子代理工具，没有可调用的 Computer Use 或 Browser 工具。
17. 本轮工作记忆材料导入验证：新增 `ImportMaterials` 单测覆盖 Markdown 可搜索导入、图片证据复制到 `work_memory_images`、Ariadne 导出 zip 回灌并提取 evidence、目录/不支持文件跳过，以及隐私模式阻断导入；`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。bindings 更新为 `463 Packages, 22 Services, 158 Methods, 5 Enums, 143 Models, 0 Events`，最新 exe 为 `30649344` bytes，时间 `2026-06-14 15:46:51`。本项为 Go/bindings/frontend/build 验证，未声明真实桌面导入按钮点击或真实文件路径粘贴验收。
18. 本轮工作记忆文档材料导入验证：`ImportMaterials` 扩展支持 PDF、docx/xlsx/pptx 和旧版 doc/xls/ppt 元数据导入；新增单测覆盖 docx 本地 zip/xml 正文提取、PDF literal 文本提取、旧版 Office 二进制文档元数据保留与搜索。`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。bindings 最新为 `464 Packages, 22 Services, 158 Methods, 5 Enums, 143 Models, 0 Events`，最新 exe 为 `30755328` bytes，时间 `2026-06-14 15:54:10`。PDF 提取是本地 best-effort；扫描版或压缩流 PDF 后续仍应走 OCR 或专用解析器补强。
19. 本轮工作记忆排除规则执行层验证：`CapturePolicy` 新增 `ExcludePaths` / `ExcludeContentPatterns` 并从 settings runtime 注入；新增单测覆盖材料导入命中排除路径/内容跳过、OCR 写回命中排除内容不持久化、导出跳过被排除条目并写入 `skippedExcludedCount`。后续已补 `ExcludeURLs`，覆盖窗口标题 URL、导入 URL、OCR URL 和导出 URL 排除。`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。bindings 仍为 `464 Packages, 22 Services, 158 Methods, 5 Enums, 143 Models, 0 Events`，该轮 exe 为 `30766592` bytes，时间 `2026-06-14 16:02:23`。
20. 本轮工作记忆筛选导出验证：新增 `ExportDataWithOptions` 和 `ExportRequest` / `ExportFilter`，单测 `TestExportDataWithOptionsFiltersByTimeTagsAndIDs` 覆盖按最近时间+标签筛选、按条目 ID 筛选、`filteredOutCount` 统计以及导出包 filter metadata/README 摘要。`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过。bindings 更新为 `464 Packages, 22 Services, 159 Methods, 5 Enums, 145 Models, 0 Events`，最新 exe 为 `30777344` bytes，时间 `2026-06-14 16:10:20`。本项为 Go/bindings/frontend/build 验证，未声明真实桌面导出筛选按钮点击验收。
21. 本轮工作记忆中心排除规则配置界面验证：`frontend/src/stores/workMemory.ts` 新增 `exclusionDraft`、`loadExclusionRules()`、`saveExclusionRules()` 和规则数量摘要；`WorkMemoryCenter.vue` 新增排除规则 textarea 与保存按钮，后续已扩展为应用、窗口、路径、URL、内容五类规则；`style.css` 补齐窄侧栏布局。`pnpm build`、`go test ./...` 和 `wails3 task windows:build` 均通过。bindings 仍为 `464 Packages, 22 Services, 159 Methods, 5 Enums, 145 Models, 0 Events`，最新 exe 为 `30781440` bytes，时间 `2026-06-14 16:17:27`。Computer Use 只读验证真实窗口：launcher 输入 `memory` 后进入 `Ariadne - 工作记忆`，文本树可见 `工作记忆中心`、`排除规则`、`应用进程`、`窗口关键词`、`路径片段`、`内容正则` 和 `保存排除规则`；本轮没有点击保存排除规则，不声明真实编辑/保存验收。
22. 本轮工作记忆时间机器回放视图验证：`frontend/src/stores/workMemory.ts` 新增 `playbackEntries`、`playbackEntry`、`playbackPosition`、`startPlayback()`、`stepPlayback()` 和 `selectPlayback()`，从截图型工作记忆 `captureId` 加载回放帧；`WorkMemoryCenter.vue` 新增回放预览、定位最近、上一帧和下一帧控件；`style.css` 补齐固定比例预览框和长标题约束。`pnpm build`、`go test ./...` 和 `wails3 task windows:build` 均通过，bindings 仍为 `464 Packages, 22 Services, 159 Methods, 5 Enums, 145 Models, 0 Events`，最新 exe 为 `30785536` bytes，时间 `2026-06-14 16:24:58`。Computer Use 只读验证真实工作记忆中心可见 `时间机器回放`、`暂无截图帧`、`上一帧`、`开始回放`、`下一帧`，无截图帧时三枚按钮禁用；本轮没有开启时间机器或写入真实截图数据。
23. 本轮工作记忆独立问题复盘草稿验证：新增 `GenerateRetrospectiveDraft(ids)` 后端路径和前端 `复盘草稿` 动作，单测覆盖按用户选择 evidence 生成复盘、敏感记忆不进入 body/evidence、证据按时间线排序、正文包含复盘概览/问题背景/时间线/初步原因/遗留风险/隐私边界和敏感跳过提示。`go test ./internal/workmemory -run "DailyDraft|RetrospectiveDraft|KnowledgeDraft" -v`、`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 均通过；bindings 更新为 `464 Packages, 22 Services, 160 Methods, 5 Enums, 145 Models, 0 Events`，最新 exe 为 `30819840` bytes，时间 `2026-06-14 16:32:11`。Computer Use 只读验证真实工作记忆中心可见 `日报草稿`、`复盘草稿`、`知识草稿`、右侧 `复盘` 面板和占位文案；本轮没有点击生成复盘或写入外部内容。
24. 本轮工作记忆复盘证据多选验证：`frontend/src/stores/workMemory.ts` 新增复盘证据组、12 条上限、可见非敏感记忆批量选择、敏感记忆阻断和后端 evidence 数量反馈；`WorkMemoryCenter.vue` 把时间线拆成复盘选择按钮和详情打开按钮，`style.css` 补齐紧凑选择条、圆形勾选和敏感禁用样式。`pnpm build`、`go test ./...` 和 `wails3 task windows:build` 均通过；bindings 仍为 `464 Packages, 22 Services, 160 Methods, 5 Enums, 145 Models, 0 Events`，最新 exe 为 `30823424` bytes，时间 `2026-06-14 16:47:53`。临时 `APPDATA` 真进程桌面截图验证：3 条测试记忆中 `选择筛选` 只勾选 2 条非敏感记忆，敏感记忆保持红色未选，按钮显示 `复盘草稿(2)`，点击后局部反馈 `复盘草稿已生成 · 2 条证据`；截图 `C:\Users\luwei\AppData\Local\Temp\ariadne-retro-final-20260614-164822.png`。本轮没有使用真实用户数据目录，临时目录已清理；当前会话未暴露可调用 Computer Use/sky 工具，因此使用 Win32 截图烟测替代。
25. 本轮工作记忆本地定期草稿调度验证：settings schema 升级到 v8，`WorkMemorySettings` 新增定期草稿开关、间隔和日报/复盘/经验发现类型开关；`workmemory` 新增 `ApplyDraftSchedule`、`ScheduledDraftStatus` 和 `RunScheduledDraftsNow`，手动运行会生成本地日报、复盘草稿和经验发现报告，并跳过敏感条目。验证命令：`pnpm build`、`go test ./internal/settings -v`、`go test ./internal/workmemory -run "ScheduledDrafts|DailyDraft|RetrospectiveDraft|DiscoverExperiences" -v`、`go test ./...` 和 `wails3 task windows:build` 均通过；bindings 更新为 `464 Packages, 22 Services, 163 Methods, 5 Enums, 147 Models, 0 Events`，最新 exe 为 `30861312` bytes，时间 `2026-06-14 17:04:53`。临时 `APPDATA` 真进程桌面截图验证：初始 `定期草稿` 面板显示 `未启用`、`间隔 240 分钟 · 最近 未运行` 和 `立即运行`，点击后显示最近运行时间、`2 条非敏感证据 · 日报 已生成 · 复盘 已生成...`，局部反馈为 `定期草稿已运行 · 2 条证据`；截图 `C:\Users\luwei\AppData\Local\Temp\ariadne-schedule-panel-20260614-170724.png` 和 `C:\Users\luwei\AppData\Local\Temp\ariadne-schedule-run-20260614-170813.png`。本轮没有使用真实用户数据目录，临时目录已清理；当前会话未暴露可调用 Computer Use/sky 工具，因此使用 Win32 截图烟测替代。
26. 本轮 AI/embedding 接口本机配置与 AI 日报润色验证：Windows User env 已配置 `OPENAI__API_KEY`、`OPENAI__BASE_URL`、`OPENAI__MODEL`、`EMBED__API_KEY`、`EMBED__BASE_URL`、`EMBED__MODEL`；Ariadne 本地 `C:\Users\luwei\AppData\Roaming\Ariadne\config.json` 只保存 provider/base/model 和 enabled 状态，检查确认 `ContainsSecret=false`。`internal/aiclient` 新增 OpenAI-compatible chat completion 客户端并支持 `OPENAI__API_KEY`；`workmemory.PolishDraft()` 覆盖 AI 未启用、隐私模式阻断、确认前不外发、确认后调用 polisher、保留 evidence ID。验证命令：`go test ./internal/aiclient ./internal/workmemory -run "Polish|DailyDraft" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 更新为 `465 Packages, 22 Services, 165 Methods, 5 Enums, 150 Models, 0 Events`，最新 exe 为 `30901760` bytes，时间 `2026-06-14 17:25:13`。接口轻量连通性检查只调用 `/models`，返回 HTTP `200` 且包含配置模型名；未发送工作记忆正文。
27. 本轮外部 embedding + 内置向量缓存验证：Ariadne 本地配置已设为 `ai.embeddingEnabled=true`、`embeddingProvider=openai-compatible`、`embeddingBaseUrl=http://10.64.251.169:4000/v1`、`embeddingModel=/model/qwen_eb`、`vectorStoreType=embedded`、`vectorCollection=ariadne_work_memory`，配置文件检查 `ContainsSecret=false`。`Test-NetConnection 192.168.1.100:19530` 返回 `True`，但 Milvus 只验证可达，没有作为存储使用；无业务内容 embedding 连通性测试返回 `2560` 维向量，未发送工作记忆正文。新增单测覆盖内置向量缓存持久化、敏感记忆跳过、隐私模式阻断、缺少客户端失败和 Milvus 不静默 fallback。验证命令：`go test ./internal/workmemory ./internal/aiclient -run "Semantic|Embedding|Milvus|Polish" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 更新为 `465 Packages, 22 Services, 168 Methods, 5 Enums, 153 Models, 0 Events`，最新 exe 为 `30969856` bytes，时间 `2026-06-14 17:44:12`。仓库敏感串扫描未发现 API key 赋值或 bearer secret 字面量。
28. 本轮 Milvus REST 向量存储适配验证：新增 `internal/workmemory/milvus_store.go`，不引入外部 SDK，直接使用当前 Milvus REST `/v2/vectordb` API；单测用 `httptest` 覆盖 collection 创建、namespace delete、upsert、load、search、敏感条目不写入、不落 embedded cache，以及缺少 Milvus URI 的明确错误。对本机 `192.168.1.100:19530` 做临时 collection live 探测，`create`、`upsert`、`load`、`search`、`delete namespace`、`drop` 均返回 `code=0`，未发送真实工作记忆正文。Ariadne 本地配置已切到 `vectorStoreType=milvus`、`vectorStoreUri=milvus://192.168.1.100:19530`、`vectorCollection=ariadne_work_memory`，配置文件检查 `ContainsSecret=false`；真实工作记忆批量刷新仍需用户在 App 内显式点击 `刷新索引`。验证命令：`go test ./internal/workmemory -run "Milvus|Embedding|Semantic" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 保持 `465 Packages, 22 Services, 168 Methods, 5 Enums, 153 Models, 0 Events`，最新 exe 为 `31011328` bytes，时间 `2026-06-14 17:59:31`。仓库敏感串扫描无命中，无残留 `ariadne` / `go` 进程。
29. 本轮外部 AI 经验发现验证：新增 `workmemory.ExperienceDiscoverer`、`ExperienceDiscoveryPolicy`、`DiscoverExperiencesAI()` 和 `aiclient.OpenAICompatibleExperienceDiscoverer`；前端工作记忆中心保留本地 `发现经验`，新增 `AI 发现` / `确认外发发现` 和风险提示，设置中心新增 `AI 经验发现` 开关。单测覆盖确认前不外发、隐私模式阻断、`experienceDiscoveryEnabled` 策略接入、敏感/疑似敏感条目过滤、AI 返回 evidence 归一化、外部失败时保留本地规则报告，以及 OpenAI-compatible `/chat/completions` 请求/JSON 解析。验证命令：`go test ./internal/workmemory -run "Experience|DiscoverExperiences" -v`、`go test ./internal/aiclient -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:build` 均通过；bindings 更新为 `465 Packages, 22 Services, 170 Methods, 5 Enums, 156 Models, 0 Events`，最新 exe 为 `31070208` bytes，时间 `2026-06-14 18:18:22`。仓库敏感串扫描无命中；本轮未调用真实外部 AI 经验发现，也未发送真实工作记忆正文，仍需真实桌面点击二次确认流程验收。
30. 本轮 Codex Skill 安装刷新握手验证：`InstallSkillPackage()` 安装确认后除 `SKILL.md` 外，还会写入 `.ariadne-refresh.json` 和 `.ariadne-refresh.touch`；manifest 记录 `action=skills.refresh`、`source=ariadne`、`skillId`、`installedDir`、`requestedAt` 和 marker id，前端类型、fallback、store 反馈和工作记忆中心结果文本同步显示 refresh marker。后续已补 `InstallDiagnostics()`：扫描 Codex skills 目录、解析已安装 `SKILL.md`、核验 marker/manifest 一致性，并把结果显示到工作记忆知识面板。`go test ./internal/skills -v` 覆盖预检不写 marker、确认安装写入 SKILL/manifest/touch、manifest JSON 可解析且绑定到安装目录、安装成功诊断、缺失目录诊断和 marker 不一致诊断；`go test ./...`、`pnpm build`、`wails3 task windows:build`、`wails3 task windows:msix` 均通过。最新 bindings 为 `468 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`，最新 exe/layout 为 `31762944` bytes，时间 `2026-06-15 08:56:41`，SHA256 `3EEC2B29CB4E6D566F78A3F853AC13C46A2747BD6ECF8BA726DE39E9E06707A8`。本轮未证明正在运行的 Codex runtime 会自动热加载该 marker，只证明 Ariadne 已写出并能本地核验可检测刷新握手。
31. 本轮 JSON 对比大输入性能预算、文件导入与拖放导入验证：新增 `CompareResult.diffTruncated`、`differencesTruncated`、`formattedTruncated` 和 `performanceNote`，Go 单测覆盖 1100 行数组跳过昂贵 unified diff 但保留 `~ $[1099]` 语义报告、超长 payload 截断格式化预览但仍返回“两个 JSON 语义一致”，以及返回差异明细截断到 2000 条但 `changed`/summary 保留完整数量。前端类型、binding normalize、fallback、真实统计总数显示、JSON 对比中心性能预算提示、`文件到左侧` / `文件到右侧` 按钮，以及左右编辑面板拖入文件/文本导入已同步；`go test ./internal/jsoncompare -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过。bindings 仍为 `465 Packages, 22 Services, 170 Methods, 5 Enums, 156 Models, 0 Events`，最新 exe 为 `31084544` bytes，时间 `2026-06-14 18:39:14`；无残留 `ariadne` / `go` 进程。
32. 本轮搜索收藏/最近使用清理验证：新增 `ClearUsageResult`、`StateStore.Clear()` 和 `Service.ClearUsage()`；`TestSearchUsageStateCanBeCleared` 覆盖收藏与最近使用记录清理、结果计数和重新加载后的空状态。前端新增 `searchUsageApi.ts`、settings store 状态加载/二次确认清理，以及设置中心“搜索数据”面板。验证命令：`go test ./internal/search -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 更新为 `465 Packages, 22 Services, 171 Methods, 5 Enums, 157 Models, 0 Events`，最新 exe 为 `31092224` bytes，时间 `2026-06-14 18:45:36`；无残留 `ariadne` / `go` 进程。
33. 本轮 Milvus 配置口径复核：普通 `%APPDATA%\Ariadne\config.json` 和 Codex/MSIX virtualized `Ariadne\config.json` 均已同步为 `ai.enabled=true`、`embeddingEnabled=true`、`embeddingProvider=openai-compatible`、`embeddingBaseUrl=http://10.64.251.169:4000/v1`、`embeddingModel=/model/qwen_eb`、`vectorStoreType=milvus`、`vectorStoreUri=milvus://192.168.1.100:19530`、`vectorCollection=ariadne_work_memory`，两份配置均检查 `ContainsSecret=false`；当时 API key 仍只放 Windows User env，最新已接入 Windows Credential Manager 回退，见本节第 38 条。当前 Milvus `192.168.1.100:19530` 的 TCP 和 REST `/v2/vectordb/collections/list` 均可达，REST 返回 `code=0`，未发送真实工作记忆正文。平台诊断文案已从过期的“embedding 待接入”修正为 SQLite FTS、本地语义检索、外部 embedding、内置向量缓存和 Milvus 均已接入，并新增回归断言。验证命令：`go test ./internal/platform ./internal/workmemory -run "StatusReports|Milvus|Embedding|Semantic" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 保持 `465 Packages, 22 Services, 171 Methods, 5 Enums, 157 Models, 0 Events`，最新 exe 为 `31092736` bytes，时间 `2026-06-14 18:51:17`。真实工作记忆批量刷新仍需用户在 Work Memory Center 显式点击 `刷新索引`。
34. 本轮 Windows 品牌资源嵌入验证：新增 `winres/winres.json`、`assets/ariadne-icon.svg`，并把 `frontend/public/favicon.svg`、`assets/logo.png`、`assets/logo.ico` 切到同一 Graphite Teal Ariadne 标识；修正 `Taskfile.yml` 的 `windows:resources` 为 `--out ariadne_resource.syso --no-suffix`，避免生成无扩展名资源导致 Go 链接器不嵌入。新增 `internal/releasepack` 测试覆盖 Ariadne VersionInfo、无旧 `x-tools` 品牌泄漏、manifest long-path 支持和 `.syso` 输出名。验证命令：`wails3 task windows:resources`、`go test ./internal/releasepack -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build`、`wails3 task windows:package` 均通过；当前最新 `bin\ariadne.exe` 为 `31386624` bytes，时间 `2026-06-14 19:30:06`，SHA256 `b9013347aea26304623c52ef96a9181eefd126d9392946ce9e8653ed0f87327a`，Windows VersionInfo 为 `ProductName=Ariadne`、`FileDescription=Ariadne command launcher and work memory center`、`InternalName=ariadne`、`OriginalFilename=ariadne.exe`；release zip 为 `16463901` bytes，时间 `2026-06-14 19:30:07`，SHA256 `2096a7d1ec72a678264e0598f7ea70855a0322c842c3869482baa7d7052204ec`。随后 `wails3 task windows:perf` 对当前品牌包重跑，记录冷启动 p95 `648ms` 已达 `800ms` 目标、平均工作集 `51059371` bytes、release zip 比旧安装器小 `92.13%`、Win32 合成 Alt+Q 可见+前台 p95 `32ms` 达到 `120ms` 目标。
34. 本轮自定义命令启动项确认与失败诊断验证：`contracts.ActionResult` 新增 `requiresConfirmation` 和 `riskReasons`，命令启动项仍显式声明 `danger` action；`platform.ExecuteAction()` 首次执行危险命令只返回 inline 二次确认，不调用 runner，确认后才通过可注入 command runner 启动进程。后端新增参数解析、Windows 路径保留、工作目录校验、runner 失败上下文诊断；前端 launcher 记录短期 pending confirmation key，只有第二次点击同一结果/同一命令 action 才发送 `confirmed=true`，换查询或 reset 会清掉确认态。验证命令：`go test ./internal/platform ./internal/launchers -run "Danger|Command|Launcher|Split" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；bindings 保持 `465 Packages, 22 Services, 171 Methods, 5 Enums, 157 Models, 0 Events`，最新 exe 为 `31106560` bytes，时间 `2026-06-14 18:58:31`。本轮未执行真实用户命令，也未声明真实桌面二次点击烟测。
35. 本轮 Everything 索引覆盖提示验证：`filesearch.Service` 会在文件/路径类查询缺 DLL、查询失败或 0 命中时返回 `file-search-coverage-hint` 诊断结果，并在 `EverythingStatus.coverageHint`、平台 `FileSearchStatus.coverageHint` 和设置中心平台诊断中显示修复提示；普通非文件查询 0 命中不插入该提示。新增单测覆盖文件类 0 命中提示、普通查询不提示、缺 DLL 提示、平台 capability note 使用覆盖提示；前端类型、fallback 和设置页 warning note 已同步。验证命令：`go test ./internal/filesearch ./internal/platform ./internal/search -run "Coverage|Everything|FileSearch|SearchAggregatesEverything" -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build`、`wails3 task windows:search-perf` 均通过；bindings 保持 `465 Packages, 22 Services, 171 Methods, 5 Enums, 157 Models, 0 Events`，最新 exe 为 `31116288` bytes，时间 `2026-06-14 19:05:37`。最新搜索基准为 `320` 样本、搜索 p95 `6ms`、rolling p95 `6ms`、`2400` 个 action surface 校验 `0` 失败、Everything 文件结果命中 `1740`；`Everything64.dll` 查询返回 `file-search-coverage-hint` / `Everything 未命中文件`，不声明真实桌面 UI 文件命中通过。
36. 本轮 Everything 文件元数据增强验证：`filesearch.fileToResult()` 新增本地文件/目录 metadata 读取，普通文件 preview meta 暴露 `类型`、`大小`、`修改时间`，payload 暴露 `sizeBytes`、`modifiedAt`、`isDirectory`；目录结果使用 `folder` icon 和 `目录` tag，且不会写入误导性的 `sizeBytes`。新增单测 `TestFileResultIncludesFilesystemMetadata` 和 `TestDirectoryResultUsesFolderMetadata` 覆盖临时文件固定大小/修改时间和目录元数据。验证命令：`go test ./internal/filesearch -v`、`go test ./internal/search ./internal/searchbench ./internal/contracts -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build`、`wails3 task windows:search-perf` 均通过；bindings 保持 `465 Packages, 22 Services, 171 Methods, 5 Enums, 157 Models, 0 Events`，最新 exe 为 `31123456` bytes，时间 `2026-06-14 19:11:52`。最新搜索基准为 `320` 样本、搜索 p95 `9ms`、rolling p95 `9ms`、`2400` 个 action surface 校验 `0` 失败、Everything 文件结果命中 `1740`；metadata stat 后仍在 100ms 目标内。
37. 本轮冷启动路径轻量化和 Milvus 当前配置复核：`main.go` 把保留策略、work-memory runtime 和剪贴板 watcher 延迟到首屏后维护；`workmemory` 持久化空库不再写 demo seed，FTS 改为有真实条目后初始化或搜索时懒加载；保留策略无删除时不再写盘。普通 `%APPDATA%\Ariadne\config.json` 与 Codex/MSIX virtualized 配置均为 `ai.enabled=true`、`embeddingEnabled=true`、`embeddingProvider=openai-compatible`、`embeddingBaseUrl=http://10.64.251.169:4000/v1`、`embeddingModel=/model/qwen_eb`、`vectorStoreType=milvus`、`vectorStoreUri=milvus://192.168.1.100:19530`、`vectorCollection=ariadne_work_memory`，且 `ContainsSecret=false`。Milvus REST `/v2/vectordb/collections/list` 返回 `code=0`，当前返回集合列表中尚无 `ariadne_work_memory`，说明没有自动把真实工作记忆批量写入；真实索引仍需 Work Memory Center 显式点击 `刷新索引`。验证命令：`go test ./internal/workmemory ./internal/capturehistory ./internal/clipboardhistory ./internal/imageindex -v`、`go test ./...`、`pnpm build`、`wails3 task windows:package`、`wails3 task windows:perf` 均通过；最新 exe 为 `31386624` bytes，release zip 为 `16463901` bytes，冷启动 p95 `648ms` / target `800ms`，Alt+Q 合成可见+前台 p95 `32ms` / target `120ms`，平均工作集 `51059371` bytes。
38. 本轮 Windows Credential Manager 密钥存储验证：新增 `internal/securestore` Windows 凭据适配、`internal/secrets` Wails 服务、设置中心安全密钥存储面板、AI/embedding/Milvus token 的状态/保存/二次确认清除路径，运行时解析顺序为环境变量优先、Credential Manager 回退。验证命令：`go test ./internal/secrets ./internal/aiclient ./internal/workmemory -run "Secret|OpenAI|Embedding|Milvus" -v`、`ARIADNE_TEST_CREDENTIAL_MANAGER=1 go test ./internal/securestore -run TestWindowsCredentialManagerRoundTrip -v`、`ARIADNE_TEST_CONFIGURED_CREDENTIALS=1 go test ./internal/securestore -run TestConfiguredAriadneCredentialsReadable -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:package`、`wails3 task windows:perf` 均通过；bindings 更新为 `467 Packages, 23 Services, 175 Methods, 5 Enums, 162 Models, 0 Events`，最新 exe 为 `31438848` bytes、SHA256 `54475515a3b62c0a81865c18deaf05121a534a6273e81723ddd7a9c8848b6584`，release zip 为 `16486039` bytes、SHA256 `83be4d8d661b952bf92620bdb6792571d46cceb5683baa85eb9eac840e251969`，冷启动 p95 `686ms` / target `800ms`，Alt+Q 合成可见+前台 p95 `22ms` / target `120ms`。本轮未把真实密钥写入仓库或 Ariadne JSON 配置，也未打印密钥正文；当前会话没有可调用的 Computer Use 工具，因此设置页按钮只声明 Go/bindings/frontend/package 验证，不声明真实桌面点击验收。
39. 本轮 release 默认用户目录安装/卸载烟测：确认真实默认路径无既有 Ariadne 安装或快捷方式后，执行 `dist\release\ariadne-dev-windows-x64\scripts\install.ps1 -CreateDesktopShortcut -SkipLegacyCheck`，安装到 `C:\Users\luwei\AppData\Local\Programs\Ariadne`，创建 `ariadne.exe`、`logo.ico`、`uninstall.ps1`、`install_receipt.json`、开始菜单 `Ariadne.lnk`、开始菜单 `Uninstall Ariadne.lnk` 和桌面 `Ariadne.lnk`；receipt 记录三条快捷方式路径。安装后的 exe 为 `31438848` bytes、SHA256 `54475515a3b62c0a81865c18deaf05121a534a6273e81723ddd7a9c8848b6584`，VersionInfo 为 `ProductName=Ariadne`、`InternalName=ariadne`、`OriginalFilename=ariadne.exe`；快捷方式目标分别指向安装目录 exe 和安装目录 `uninstall.ps1`。从默认安装目录启动 exe 成功，进程路径为 `C:\Users\luwei\AppData\Local\Programs\Ariadne\ariadne.exe`，主窗口标题 `Ariadne`，随后停止进程。执行 `uninstall.ps1 -Synchronous` 后安装目录、开始菜单快捷方式、桌面快捷方式均不存在，HKCU Run 中无 `Ariadne` 值，未使用 `-RemoveUserData`，用户数据保留。
40. 本轮真实用户目录完整回滚演练：新增 gated 测试 `TestRealUserRollbackSmoke`，默认 `go test ./...` 跳过，只有显式 `ARIADNE_TEST_REAL_ROLLBACK=1` 才写入真实 Ariadne 数据根。该测试对当前标准 `%APPDATA%\Ariadne` 和 Codex/MSIX virtualized `LocalCache\Roaming\Ariadne` 根写入隔离 sentinel，创建 safety checkpoint 和 smoke checkpoint，篡改 sentinel 并写入 stale 文件，然后调用 `RestoreRollbackCheckpoint(confirm=true, createPreRestoreBackup=true)` 验证 sentinel 恢复为 checkpoint 内容、stale 文件被删除；成功后清理本轮 sentinel 目录和 smoke 备份文件，失败时保留备份便于人工恢复。验证命令：`go test ./internal/release -v` 默认通过且跳过真实 smoke；`ARIADNE_TEST_REAL_ROLLBACK=1 go test ./internal/release -run TestRealUserRollbackSmoke -v` 通过；`go test ./...` 通过。验收后确认两个真实数据根均无 `.ariadne-rollback-smoke` 目录，无残留 `ariadne` / `go` 进程。
5. 本轮旧版交接验证：新增 platform 单测覆盖“未确认不执行”和“确认后旧版退出 + Alt+Q 重试”；`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:package` 均通过，bindings 当时更新为 `461 Packages, 20 Services, 140 Methods, 5 Enums, 122 Models, 0 Events`。临时 `APPDATA` 桌面烟测启动该轮 exe，初始 launcher 为 `760 x 96`，输入 `settings` 打开 `1120 x 720` 设置中心并生成日志，截图 `C:\Users\luwei\AppData\Local\Temp\codex-shot-ariadne-handoff-20260614-125651.png`；临时数据目录已清理。
6. 本轮回滚恢复验证：新增 release 单测覆盖恢复确认门禁、恢复前 `pre_restore` 检查点、覆盖恢复、stale 文件清理、backups 目录保留和同秒检查点文件名去重；`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:package` 均通过，bindings 更新为 `461 Packages, 20 Services, 141 Methods, 5 Enums, 125 Models, 0 Events`。临时 `APPDATA` 桌面烟测启动最新 exe，初始 launcher 为 `760 x 96`，输入 `settings` 打开 `1120 x 720` 设置中心并生成日志，截图 `C:\Users\luwei\AppData\Local\Temp\codex-shot-ariadne-restore-20260614-130613.png`；临时数据目录已清理。
5. 早前 Computer Use 已验证真实桌面窗口中的设置、平台诊断、Start Menu 应用搜索、自定义启动项收藏、剪贴板中心、截图中心、Hosts 中心、工作流中心和 JSON 对比中心。
6. 早前 Computer Use 已验证 Ariadne 主窗口：启动为折叠搜索器 `760x112`，输入 `net` 后展开为 `760x430`，文本树只显示 launcher、搜索结果和局部动作；期间曾返回 stale 句柄，按 `list_windows` 恢复后验证通过。
7. 验证发现旧版 `x-tools.exe` 同时运行会抢占 Alt+Q，Ariadne 启动时无法注册该热键；关闭旧版后 Ariadne 注册成功，临时 `RegisterHotKey` 探测返回 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)`。本轮已补平台旧版运行/热键冲突诊断。
8. Computer Use 已验证网络监控主路径：新版启动器输入 `net` 后网络监控命令排在 Everything 文件结果之前；点击主动作进入网络监控中心 `980x640`，显示实时下载/上传、`2 / 2` 网卡和链路速率；Alt+Q 返回启动器。
9. 本轮视觉证据：`C:\Users\luwei\AppData\Local\Temp\ariadne-launcher-qa.png`，截图尺寸 `760x112`，浅色搜索框，无原生标题栏。
10. 临时 `APPDATA` 真实进程烟测验证自动剪贴板监听：隐藏启动 `ariadne.exe --hidden` 后修改系统剪贴板，临时 `clipboard_history.json` 写入 1 条 `source=clipboard_watcher` 文本记录；测试结束后已恢复剪贴板并删除临时目录。
11. 本轮主题修正验证：`go test ./...` 通过；`pnpm build` 通过；`wails3 task windows:build` 通过；bindings 仍为 `85 Methods / 59 Models`；仍只有 Reka UI 依赖链中的 `@vueuse/core` pure annotation 非致命警告。
12. 早前 Computer Use 验证真实窗口：启动为浅色 `760x112` 折叠搜索器，无原生标题栏；输入 `settings` 后展开为 `760x430`，`设置中心` 排第一，Enter 进入设置中心 `1120x720`，可见主题字段。
13. PowerShell 验证 Codex MSIX virtualized config 已迁移为 `version=7`、`general.theme=light`，路径 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\config.json`。
14. 本轮贴图窗口验证：`go test ./...` 通过；`pnpm build` 通过；`wails3 task windows:build` 通过；bindings 更新为 `92 Methods / 76 Models`；Computer Use 验证截图历史中心新增 `创建贴图`，点击后出现独立窗口 `截图贴图 2560x1440`，贴图窗口尺寸 `720x425`，含图片图形、来源标签、复制/缩放/阴影/关闭控件，关闭后只剩主 `Ariadne` 窗口。
15. 本轮区域截图验证：`go test ./...` 通过；`pnpm build` 通过；`wails3 task windows:build` 通过；bindings 更新为 `98 Methods / 81 Models`；Computer Use 验证真实启动器为浅色 `760x112` 搜索器，输入 `shot` 后首项为 `区域截图 / 截图覆盖层`，Enter 打开可发现的 `Ariadne - 截图覆盖层` `2560x1440` 窗口，拖拽选区后显示 `保存`、`贴图`、`二维码`，按 Enter 保存 `overlay_selection` `400x260` 后关闭覆盖层并恢复主窗口。本轮测试截图记录和 PNG 已清理。
16. 本轮 OCR 验证：`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；RapidOCR bridge 对临时 PNG 返回文本；Computer Use 验证截图历史中心 `OCR 当前屏幕` 在真实桌面窗口返回 OCR 结果面板并可复制文字。本轮测试截图记录和 PNG 已清理。
17. 本轮 OCR 文本选择验证：`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 通过；Computer Use 验证真实截图历史中心 `OCR 当前屏幕` 后显示 `125 行`、`全选`、`清空`、`复制选中`、`复制全文`、逐行置信度/坐标；点击 `全选` 后状态变为 `已选 125` 且 `复制选中` 可用。本轮测试截图记录和 PNG 已清理。
18. 本轮主题与 OCR 叠框验证：`go test ./...`、`pnpm build`、`wails3 task windows:build` 通过；Computer Use 验证真实启动器仍为 `760x112` 浅色搜索器，Codex MSIX virtualized config 为 `version=7` / `general.theme=light`，设置页主题只剩 `Graphite Teal Light（默认）` 与 `Graphite Teal Dark（深色模式，手动开启）`；截图历史中心 `OCR 当前屏幕` 后图片叠框可见，真实坐标点击叠框后状态变为 `131 行 · 已选 1` 且 `复制选中` 可用。本轮测试截图记录和 PNG 已清理。
19. 本轮贴图 OCR 联动验证：`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 通过；Computer Use 验证截图贴图窗口包含 `OCR 文字识别`，点击后出现 `110 行 · 已选 0`、逐行 `选择 OCR 第 X 行` 叠框、`全选`、`清空`、`复制选中`、`复制全文`；选择第 1 行后状态变为 `已选 1` 且 `复制选中` 可用。验证后已关闭 Ariadne 窗口并清理测试进程。
20. 本轮截图高级编辑验证：`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 通过；bindings 更新为 `107 Methods / 86 Models`，产物为 `23819776` bytes、`2026-06-14 08:22:05`。Go 单测覆盖矩形标注、马赛克、`save_as` 另存自动补 `.png`、外部 PNG 写入、`annotated` / `mosaic` / `save_as` actions。真实桌面复验未完成：当前 Codex/PowerShell 非交互启动上下文下，Wails v3.0.0-alpha.98 在应用窗口创建前 fatal `GetCursorPos failed`；同上下文 Win32 `GetCursorPos=False`，这是运行环境/上游 alpha 初始化阻塞，不是截图编辑编译失败。
21. 本轮图片 OCR 索引验证：新增 `internal/imageindex` 单测覆盖截图和剪贴板图片批量索引、`ocr`/`img` 前缀搜索、重复索引跳过、截图/剪贴板结果显式动作约束、敏感 OCR 文本屏蔽。`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 更新为 `113 Methods / 90 Models`，产物为 `23897088` bytes、`2026-06-14 08:31:35`。真实桌面复验仍受上述 Wails `GetCursorPos failed` 启动阻塞影响。
22. 本轮工作记忆本地语义检索验证：`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 通过；新增 `workmemory` 单测覆盖 `数据库连不上` 命中英文 `PostgreSQL / connection refused` 记忆、语义结果证据标注、显式 action surface 和 `SemanticStatus` 不冒充外部向量库。bindings 更新为 `114 Methods / 91 Models`，产物为 `23916032` bytes、`2026-06-14 08:56:59`。当时真实桌面复验受 Wails `GetCursorPos failed` 启动阻塞影响；当前状态以 0.0 状态卡最新记录为准。
23. 本轮保留策略验证：`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 通过；新增 `workmemory` 单测覆盖过期非收藏清理、过期收藏保留，新增 `imageindex` 单测覆盖过期索引清理和源记录缺失 stale 清理。设置变更和启动初始化都会调用 `ApplyRetentionPolicy`。bindings 更新为 `116 Methods / 93 Models`，产物为 `23927808` bytes、`2026-06-14 09:03:06`。
24. 本轮历史保留策略验证：`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 通过；新增 `capturehistory` 单测覆盖过期未置顶截图记录和 PNG 文件清理、过期置顶截图保留，新增 `clipboardhistory` 单测覆盖过期剪贴板文本/图片清理、剪贴板图片文件清理、过期置顶文本保留。设置变更和启动初始化现在会同时调用 workmemory、capturehistory、clipboardhistory 和 imageindex 的保留策略。bindings 更新为 `118 Methods / 95 Models`，产物为 `23941120` bytes、`2026-06-14 09:06:53`。
25. 本轮缩略图分层验证：`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 通过；新增 `internal/imagepreview` 缩略图工具，`capturehistory` / `clipboardhistory` 单测覆盖缩略图生成、旧记录缺失缩略图回填、`ThumbnailDataURL`、状态统计，以及删除时原图和缩略图一起清理。bindings 更新为 `120 Methods / 95 Models`，产物为 `23968768` bytes、`2026-06-14 09:20:51`。
26. 本轮 Computer Use 启动器烟测：`2026-06-14 09:24:07` 验证最终 `bin\ariadne.exe` 主窗口可抓取；折叠态为浅色 `760x112` 搜索器，输入 `capture` 后展开为 `760x430`，首项为 `打开截图历史中心`，并可清空查询恢复 `760x112`。本轮仅验证启动器层，不把缩略图历史中心预览计为桌面验收。
27. 本轮工作记忆时间机器执行链路验证：`go test ./internal/workmemory -run "TimeMachine|CapturePolicy|ApplySettings|Privacy" -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build` 和 `wails3 task windows:build` 通过；新增 `TestTimeMachineWorkerCapturesOnInterval`、`TestCapturePolicyBlocksExcludedApp`、`TestCapturePolicyBlocksExcludedWindowKeyword` 和 `TestApplySettingsReportsWorkerStateAndInterval`。bindings 更新为 `121 Methods / 96 Models`，产物为 `23977984` bytes、`2026-06-14 09:31:32`。Computer Use 验证搜索 `memory` 首项为 `打开工作记忆中心`，Enter 打开 `1120x720` 工作记忆中心，文本树包含 `时间机器 暂停`、`隐私 关闭`、`采集` 和 `跳过`；本轮未点击开启时间机器，避免写入真实桌面截图数据。
28. 本轮工作记忆时间机器保护策略验证：`go test ./internal/workmemory -run "TimeMachine|CapturePolicy|ApplySettings|Idle|Lock|Strategy|Privacy" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；新增 `TestTimeMachinePauseOnIdleSkipsCapture`、`TestManualCaptureIgnoresIdlePause`、`TestTimeMachinePauseOnLockSkipsCapture`、`TestCaptureStrategyIsRecordedOnCaptureEntries`。bindings 仍为 `121 Methods / 96 Models`，产物为 `24000512` bytes、`2026-06-14 09:46:57`。Computer Use 验证最新 exe：`memory` 打开工作记忆中心后显示 `范围 全部屏幕 · 合并`、`保护 受保护`、`策略 全部屏幕 / 合并 · 空闲阈值 10m · 锁屏暂停`；设置中心滚动到工作记忆采集区后显示 `空闲暂停`、`锁屏暂停`、`空闲阈值秒`、`采集范围`、`多屏策略`。本轮没有开启真实自动截图。
29. 本轮截图范围真实执行链路验证：`go test ./internal/capturehistory -run "CaptureScreenWithOptions|Thumbnail|Retention|Persists" -v`、`go test ./internal/workmemory -run "TimeMachine|CapturePolicy|ApplySettings|Idle|Lock|Strategy|Privacy" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 更新为 `122 Methods / 97 Models`，产物为 `24022016` bytes、`2026-06-14 09:54:59`。临时 `APPDATA` 真实进程烟测启动 `ariadne.exe --hidden`，配置 `timeMachineEnabled=true`、`captureScope=primary_screen`、`multiMonitor=primary_only`、`pauseOnIdle=false`、`pauseOnLock=false`，13 秒后临时目录写入 `work_memory.json`、`capture_history.json` 和 1 张 PNG；最新工作记忆为 `source=time_machine`、`captureId=9299df01d609`、`3840x2160`、tags 包含 `范围:主屏幕` / `多屏:仅主屏`，截图历史 actions 包含 `screen` / `primary_screen` / `primary_only`，tags 包含 `区域:0,0,3840x2160`。烟测结束后已终止进程并删除临时目录。
30. 本轮自动 OCR 策略验证：`go test ./internal/workmemory -run "AutoOCR|TimeMachine|CapturePolicy|ApplySettings|Idle|Lock|Strategy|Privacy" -v`、`go test ./internal/ocr -run "RecognizeWorkMemory" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 仍为 `122 Methods / 97 Models`，产物为 `24030720` bytes、`2026-06-14 10:06:03`。临时 `APPDATA` 真实进程烟测启动 `ariadne.exe --hidden`，配置 `timeMachineEnabled=true`、`autoOcr=true`、`captureScope=primary_screen`、`multiMonitor=primary_only`、`pauseOnIdle=false`、`pauseOnLock=false`，自动写入 `work_memory.json`、`capture_history.json` 和 PNG；最新工作记忆为 `source=time_machine`、`3840x2160`、`ocrStatus=done:rapidocr_onnxruntime`、OCR 文本长度 `2489`，截图历史 actions 包含 `screen` / `primary_screen` / `primary_only`，烟测不输出 OCR 正文，结束后已终止进程并删除临时目录。
31. 本轮 SQLite FTS 验证：新增 `zombiezen.com/go/sqlite v1.4.2` 作为 CGO-free SQLite/FTS5 依赖；`go test ./internal/workmemory -run "SQLiteFTS|Semantic|OCR|Search|Delete|Retention" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 更新为 `457 Packages, 17 Services, 122 Methods, 5 Enums, 97 Models, 0 Events`，产物为 `30007808` bytes、`2026-06-14 10:26:05`。临时 `APPDATA` 真实进程烟测启动 `ariadne.exe --hidden`，配置时间机器 10 秒间隔但关闭 auto OCR，写入 `work_memory.json`、`capture_history.json`、PNG 和 `work_memory.fts.sqlite`；FTS 文件存在且大小 `4096` bytes，烟测未输出截图/OCR 正文，结束后已终止进程并删除临时目录。
32. 本轮经验发现验证：新增 `DiscoverExperiences` 服务和工作记忆中心 `经验发现` 侧栏；`go test ./internal/workmemory -run "DiscoverExperiences|Drafts|Semantic|SQLiteFTS|OCR|Search" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 更新为 `457 Packages, 17 Services, 123 Methods, 5 Enums, 99 Models, 0 Events`，产物为 `30043648` bytes、`2026-06-14 10:36:49`。Computer Use 能枚举到 Ariadne 窗口，但本轮捕获状态返回 `foreground window did not report a process id`，未把按钮点击计为桌面验收；验证后已终止测试 Ariadne 进程。
33. 本轮经验发现决策闭环验证：新增 `SetExperienceInsightDecision` 服务、`ExperienceDecision` / `ExperienceDecisionResult` 模型和工作记忆中心接受/稍后/驳回按钮；`go test ./internal/workmemory -run "Experience|DiscoverExperiences|Drafts" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过；bindings 更新为 `457 Packages, 17 Services, 124 Methods, 5 Enums, 101 Models, 0 Events`，产物为 `30058496` bytes、`2026-06-14 10:46:47`。本轮未再声明桌面点击验收，仍以后端持久化测试、前端构建和 bindings 作为证据。
34. 本轮旧历史数据迁移验证：新增 `internal/migration` 单测伪造旧版剪贴板文本/图片、截图图片和工作记忆图片，验证 status/dry-run/导入/重复导入去重、缺失来源不阻断部分导入，并确认图片复制到 Ariadne 目录而不是继续引用旧路径；`go test ./internal/migration ./internal/clipboardhistory ./internal/capturehistory ./internal/workmemory -run "Legacy|Import" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过。bindings 更新为 `458 Packages, 18 Services, 129 Methods, 5 Enums, 106 Models, 0 Events`，产物为 `30134784` bytes、`2026-06-14 11:08:37`。Computer Use 只读验证真实设置中心可见 `旧历史数据`、`刷新历史` 和 `迁移旧历史`；本轮没有点击迁移按钮，避免导入用户真实旧历史。
35. 本轮候选工作流/检查清单草稿验证：新增 `GenerateWorkflowDraft`、`GenerateChecklistDraft` 和 workmemory 单测 `TestGenerateWorkflowDraftFromEvidence` / `TestGenerateChecklistDraftFromEvidence`；`go test ./internal/workmemory -run "WorkflowDraft|ChecklistDraft" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过。bindings 更新为 `458 Packages, 18 Services, 131 Methods, 5 Enums, 109 Models, 0 Events`，产物为 `30163456` bytes、`2026-06-14 11:20:05`。Computer Use 真实窗口验证 `mem center` 可打开工作记忆中心，文本树可见 `工作记忆中心`、`候选工作流`、`检查清单`、`经验发现` 和 `发现经验`；未点击会持久化决策或导入历史的按钮。
36. 本轮搜索性能与 Everything 诊断验证：新增 search/filesearch/platform 单测覆盖搜索 p95 快照、Everything 成功/失败状态和平台诊断汇总；`go test ./internal/search ./internal/filesearch ./internal/platform -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过。bindings 更新为 `458 Packages, 18 Services, 132 Methods, 5 Enums, 112 Models, 0 Events`，产物为 `30199808` bytes、`2026-06-14 11:34:07`。Computer Use 验证真实设置中心平台诊断显示 `搜索 p95 51ms · 目标 100ms · 样本 2`、`最近搜索 settings · 36ms · 2 项`、`Everything 可用` 和 `settings 35ms / 0 项`；本轮 `Everything64.dll` 查询未返回文件结果，因此 Everything 文件结果真实命中仍不能算完成。
37. 本轮发布回滚检查点验证：新增 `internal/release` 单测覆盖数据根统计、检查点 zip 写入、`manifest.json` 恢复说明、多个 virtualized 数据根 archiveName 去重、旧备份目录排除和空数据根检查点；`go test ./internal/release -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 通过。bindings 更新为 `459 Packages, 19 Services, 134 Methods, 5 Enums, 116 Models, 0 Events`，产物为 `30248448` bytes、`2026-06-14 12:04:15`。当时 Computer Use 启动该轮 exe 未出现可抓取窗口；PowerShell stderr 捕获为 Wails v3.0.0-alpha.98 在窗口创建前 fatal `GetCursorPos failed`，因此该轮不声明回滚检查点 UI 的真实桌面只读验收。
38. 本轮用户级发布包验证：新增 `internal/releasepack` 单测覆盖 release zip、manifest、install/uninstall PowerShell 脚本、旧版 x-tools 并存提示、Ariadne.previous 回滚目录说明、缺失 exe 失败路径，以及 `-NoShortcuts -SkipProcessStop` 临时目录安装、重复安装回滚、`-Synchronous` 卸载烟测；`go test ./internal/releasepack -v`、`go test ./...` 和 `wails3 task windows:package` 通过。生成包为 `experiments\ariadne\dist\release\ariadne-dev-windows-x64.zip`，大小 `16213810` bytes；zip 内含 `README.txt`、`app/ariadne.exe`、`app/logo.ico`、`manifest.json`、`scripts/install.ps1` 和 `scripts/uninstall.ps1`。本轮没有在真实 `%LOCALAPPDATA%\Programs\Ariadne` 执行安装/卸载，也没有创建真实开始菜单快捷方式。
39. 本轮启动器/工具窗口边界修正验证：新增 `internal/toolwindows` 服务和单测，`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:package` 均通过；bindings 更新为 `460 Packages, 20 Services, 138 Methods, 5 Enums, 117 Models, 0 Events`，exe 为 `30264320` bytes、`2026-06-14 12:12:49`，release zip 为 `16218140` bytes。捕获式直接执行曾在窗口创建前触发 Wails v3.0.0-alpha.98 `GetCursorPos failed`，后续确认正常桌面 `Start-Process` 启动可创建窗口，阻塞点不是包本身不可运行。
40. 本轮 Windows GUI 子系统与 launcher 视觉复验：`Taskfile.yml` 的 Windows 构建改为 `go build -ldflags="-H windowsgui"`，避免 release exe 启动时带出黑色控制台窗口；桌面截图确认该轮 `bin\ariadne.exe` 只出现 Ariadne 窗口。同步修正 palette 响应式断点，输入 `hosts` 后 `860 x 468` launcher 保留结果列表和右侧预览，Enter 打开 `1120 x 720` 独立 `Hosts 管理` 工具窗口，Alt+Q 召回后窗口列表只剩 `760 x 96` 搜索器；随后统一 Go/TS 壳层初始尺寸为 `760 x 96`，避免前端挂载后二次收缩。该轮 exe 为 `30264320` bytes、`2026-06-14 12:33:00`，release zip 为 `16217970` bytes、`2026-06-14 12:33:01`。关键截图：`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-14_12-27-57.png`、`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-14_12-28-29.png`、`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-14_12-29-02.png`、`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-14_12-33-19.png`。
41. 本轮本地日志与诊断包验证：新增 `internal/applog` 单测覆盖日志文件写入和状态；新增 `internal/platform` 单测覆盖日志状态进入 capability/metrics，以及 `ExportDiagnostics()` 生成含 `README.md`、`diagnostics/platform_status.json`、`diagnostics/metrics.json`、`logs/ariadne.log` 的 zip。`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:package` 均通过；bindings 当时更新为 `461 Packages, 20 Services, 139 Methods, 5 Enums, 119 Models, 0 Events`。临时 `APPDATA` 桌面烟测启动该轮 exe，初始窗口 `760 x 96`，生成 `Ariadne\logs\ariadne.log`，日志大小 `46` bytes，截图 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-14_12-43-26.png`；临时目录已清理。该轮 exe 为 `30307840` bytes、`2026-06-14 12:42:56`，release zip 为 `16239549` bytes、`2026-06-14 12:42:57`。
42. 本轮 release 安装/卸载脚本增强验证：新增 `StartMenuDir` / `DesktopDir` 安装参数、`install_receipt.json` 和基于 receipt 的快捷方式清理；`go test ./internal/releasepack -v` 覆盖临时目录真实 PowerShell 安装、重装、开始菜单快捷方式、卸载快捷方式、桌面快捷方式、receipt 和同步卸载清理。`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:package` 均通过，bindings 仍为 `461 Packages, 20 Services, 141 Methods, 5 Enums, 125 Models, 0 Events`。实际 release 包烟测安装到 `C:\Users\luwei\AppData\Local\Temp\ariadne-release-smoke-20260614-131403`，验证 `ariadne.exe`、`uninstall.ps1`、`install_receipt.json`、开始菜单快捷方式、卸载快捷方式和桌面快捷方式存在；卸载后安装目录与三类快捷方式均删除，Ariadne 进程数为 0，临时烟测目录已删除。安装后的 exe 在该自动化上下文中只到服务初始化并写入临时日志/数据，未形成可捕获桌面窗口，因此不计作安装后 UI 验收。
43. 本轮候选工作流正式保存验证：新增 `workflows.DraftSaveRequest` / `DraftSaveResult` 和 `SaveWorkflowDraft()`，单测覆盖未确认不落盘、风险原因返回、确认后生成 `memory-<draft>` 正式工作流、保留 trigger/input/output/evidence 到 description、重载 `workflows.json` 后仍存在，以及无步骤草稿拒绝保存。工作记忆中心候选工作流面板新增 `保存到工作流` / `确认保存` 两次确认按钮和局部保存结果。`go test ./internal/workflows -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings`、`wails3 task windows:package` 均通过；bindings 更新为 `461 Packages, 20 Services, 142 Methods, 5 Enums, 127 Models, 0 Events`。最新 release 包脚本烟测安装到 `C:\Users\luwei\AppData\Local\Temp\ariadne-release-smoke-20260614-132305`，验证 exe、receipt 和三类快捷方式存在，卸载后全部删除且无残留进程，临时目录已清理。Computer Use 尝试启动最新 `bin\ariadne.exe` 未暴露可捕获窗口，因此该 UI 按钮本轮只声明 Go/build/bindings 验证，不声明真实桌面点击验收。

### 下一步不要跑偏

当前继续迁移窗口型插件时，优先补：

1. 启动器/工具窗口职责边界已修正：主窗口只承载 launcher，工具中心走独立工具窗口；不要再把工作记忆、设置、Hosts、JSON 对比等页面塞回 `main` 窗口 activeView。截图高级编辑首版、截图覆盖层、区域选择、本地 OCR、行级文本选择、OCR 图片叠框选择、贴图 OCR 联动、图片 OCR 索引底座、工作记忆本地语义检索、外部 embedding + embedded/Milvus 向量存储、外部 AI 经验发现、主要本地保留策略、缩略图分层/回填、搜索 p95/Everything 诊断、可重复搜索 p95 基准、前端搜索过期响应防护、provider 级搜索取消、插件命令补全和参数面板已完成；下一步不要重复实现这些，优先补真实桌面复验和发布迁移。
2. 旧版剪贴板/截图/工作记忆历史数据迁移首版、Ariadne 本地数据回滚检查点首版、确认式恢复、用户级 release zip 首版、旧版并存确认式交接、临时目录安装/卸载脚本、重定向快捷方式烟测、真实默认用户目录安装/卸载烟测、真实用户目录完整回滚演练、品牌发布资产和 unsigned MSIX layout 生成链路已完成；签名 `.msix` 安装器仍需 Windows SDK `makeappx.exe`、`signtool.exe` 和签名证书后补验。旧版运行/Alt+Q 冲突诊断已完成。
3. 网络监控贴边小窗、锁定尺寸/位置、四角位置持久化、多屏模式和前台全屏窗口自动隐藏代码路径已接入：`net mini`、中心页 `小窗` 按钮和托盘 `网速小窗` 会打开 `network-mini` 置顶小窗；仍需补真实桌面点击复验、真实全屏应用隐藏/恢复复验、真实多显示器复验和旧版拖拽细节对齐。
4. 工作记忆时间机器 worker、设置 interval 重启、前台窗口切换触发、排除应用/窗口标题阻断、空闲/锁屏暂停、采集范围真实执行链路、安全临时 `APPDATA` 自动采集烟测、自动 OCR 写回烟测、SQLite FTS、本地经验发现首版、经验发现决策状态持久化、外部 embedding + embedded/Milvus 向量存储、外部 AI 经验发现、旧历史数据迁移首版、候选工作流/检查清单草稿、候选工作流正式保存、回滚检查点首版、真实用户目录回滚演练和 unsigned MSIX layout 已覆盖；下一步不要重复补这些，优先补签名 MSIX 外部条件后的安装验收和剩余桌面复验。
5. 工作流下一步不要再补“导入导出/高风险确认/候选工作流保存”；这些已完成并由 Go 测试、前端构建和 bindings 覆盖。检查清单、本地 Skill 资产确认保存、Codex skill 包导出、live Codex skill 目录安装和 Ariadne refresh marker 握手也已接入；剩余是运行中 Codex 是否实际监听该 marker 的热加载验收，或其他任务包体系。

完成任一项后，必须在本节更新：

1. 代码路径。
2. 验证命令。
3. bindings 数量。
4. exe 大小与更新时间。
5. Computer Use 或人工桌面验证结果。
6. 仍未完成的缺口。

## 0. 防重复执行清单

后续继续 Ariadne 重构时，先看本节，不要重复做已确认的环境和工程事实。

### 0.1 已确认环境事实

1. 当前 shell 的系统 `PATH` 里可能没有 `go` 和 `wails3`，这不是项目没构建出来，而是环境变量会话丢失。
2. 已安装便携 Go：`C:\Users\luwei\.codex\tools\go1.26.4.windows-amd64\go\bin\go.exe`。
3. 已安装便携 Wails 3 CLI：`C:\Users\luwei\.codex\go-bin\wails3.exe`。
4. 使用 Go/Wails 命令前，先在当前 PowerShell 会话临时注入：

```powershell
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
```

5. Go 便携包来源为官方下载页：`https://go.dev/dl/`，已校验 `go1.26.4.windows-amd64.zip` SHA256：

```text
3ca8fb4630b07c419cbdd51f754e31363cfcfb83b3a5354d9e895c90be2cc345
```

6. Wails 版本固定为 `v3.0.0-alpha.98`，与 `experiments/ariadne/go.mod` 一致。
7. `experiments\ariadne\bin\ariadne.exe` 已经构建出来过；不要因为找不到 `go/wails3` 就重新判定应用未构建。
8. 当前 app 产物存在：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\bin\ariadne.exe`，大小 `30544896` bytes，更新时间 `2026-06-14 14:59:05`；当前 release zip 为 `16342857` bytes，更新时间 `2026-06-14 14:35:40`。
9. `Everything64.dll` 存在于仓库根目录：`P:\workspace\glwlg\app\x-tools\Everything64.dll`，大小 `90280` bytes，更新时间 `2026-02-09 10:35:56`。
10. 需要确认 app 是否已构建时，先查 `experiments\ariadne\bin\ariadne.exe` 文件元数据，不要直接重新跑 build。

### 0.2 已完成但仍需继续验收的事项

1. Wails 3 + Vue 3 Ariadne 工程骨架已存在。
2. 主搜索窗口已存在并通过 Computer Use 做过真实桌面验证。
3. 工作记忆中心 UI 已存在并通过 Computer Use 做过真实桌面验证。
4. 设置中心 UI 和 Go settings 服务已实现，已重新生成 Wails bindings、重建 exe，并用 Computer Use 复核。
5. `StorageStatus` 曾出现过“界面显示已写入，但 PowerShell 看不到 `C:\Users\luwei\AppData\Roaming\Ariadne\config.json`”的矛盾；结论是 Codex MSIX 环境启动 Ariadne 时触发 AppData virtualization，真实文件在 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\config.json`。
6. `StorageStatus` 现在会暴露 `virtualizedPath`、`virtualizedExists`、`virtualizedBytes`，设置页显示 `MSIX 实际路径`。
7. Start Menu 应用搜索已接入 Go 主路径，并用 Computer Use 验证过 `calculator` 查询。
8. Everything 文件搜索 provider 已接入 Go 主路径，代码和测试已通过；真实 UI 检索仍需单独复验并记录截图。
9. 自定义启动项 provider 已接入 Go 主路径，支持应用、文件、文件夹、URL 和需要确认的命令类启动项。
10. 搜索收藏与最近使用排序已接入本地状态文件；前端会在动作成功后记录使用，收藏/取消收藏作为显式 preview action 出现，并已通过 Computer Use 复验收藏动作。
11. 设置中心已加入自定义启动项可视化管理区：启动项列表、新建、编辑表单、启用开关、关键词/标签、保存和二次确认删除。
12. 启动项后端 `Status` 会报告 `lastSaveError`；前端保存/删除启动项时会把写盘错误作为 inline feedback 暴露，不再静默吞掉。
13. 剪贴板历史已接入 Ariadne Go 主路径：自动文本/图片监听、文本和 PNG 持久化、搜索、置顶、删除、清空未置顶、主搜索聚合、图片预览、复制图片回剪贴板、剪贴板历史中心 UI、剪贴板图片贴图动作、二维码识别和图片 OCR。
14. 截图历史已接入 Ariadne Go 主路径：Windows GDI 当前屏幕捕获、区域截图覆盖层、PNG 持久化、搜索、置顶、删除、清空未置顶、主搜索聚合、截图历史中心 UI、二维码识别、OCR、贴图动作和 MSIX 实际路径诊断。截图历史服务已支持全部屏幕、主屏幕、前台窗口和按显示器分条采集；工作记忆时间机器 worker 已能按策略调用该能力，并已接入空闲/锁屏暂停。
15. Hosts 管理已接入 Ariadne Go 主路径和 Vue 中心 UI：旧 `.x-tools` profiles 迁移、本地/远程方案、启用开关、冲突检测、应用前预览、Ariadne marker 合并、系统写入二次确认和 UAC 写入链路已实现。真实桌面已验证打开中心和生成预览；未执行系统 hosts 写入烟测。
16. 工作流宏已接入 Ariadne Go 主路径和 Vue 中心 UI：旧 `x-tools\config.json` workflows 迁移、默认宏、可视化步骤编辑、变量 `{clipboard}`/`{input}`/`{prev}`、搜索入口、命令链执行、结果回传和复制已实现。真实桌面已验证工作流中心运行旧配置宏；启动器 `wf ...` 搜索由 Go 测试覆盖，UIA 输入链路未作为桌面验收依据。
17. JSON 对比已接入 Ariadne Go 主路径和 Vue 中心 UI：语义差异、格式化输出、报告、行 diff、`jsondiff` 命令结果和 `open_json_compare` 插件路由已实现。真实桌面已验证从搜索器输入 `jsondiff` 打开默认样例，并返回 `发现 4 处差异：新增 2，删除 1，变更 1`。
18. 网络监控已接入 Ariadne Go 主路径和 Vue 中心 UI：Windows IP Helper API 读取网卡累计字节，Go 服务做 1s 速率差分，搜索器 `net`/`network`/`网速` 打开中心窗口；`net mini`、中心页 `小窗` 按钮和托盘 `网速小窗` 已能打开 x-tools 风格任务栏左侧 `156 x 40` 网速小条。小窗默认 `taskbar-left`，Windows 下挂到 `Shell_TrayWnd` 任务栏层级并避免抢焦点，后端仍保留旧四角 anchor、多屏模式和前台全屏自动隐藏兼容；仍需补真实桌面点击复验、真实任务栏嵌入复验和真实多显示器复验。
19. `net` 精确插件命令现在在搜索排序中高于 Everything 文件结果；新增 `internal/search` 回归测试覆盖这个场景。
20. 前端主题同步已接入 `lib/theme.ts`：默认 `light`，设置保存/重置/导入会立即同步主题，`.dark` 只作为显式深色模式 token。
21. 设置 schema 已升到 v7：v7 之前旧实验配置的 `dark` / `system` 会迁移并写回为 `light`；当前 v7 仅允许用户主动选择 `dark`。
22. 工作记忆中心的时间机器/隐私开关现在会同步持久化到 settings，再同步 workmemory 服务，避免中心页和设置页状态不一致。
23. 二维码识别服务已接入：`internal/qrscan` 使用 `gozxing` 解码截图历史图片，前端截图历史中心有“识别二维码”和“识别当前屏幕”入口，启动器截图结果也有显式 `recognize_qr` 动作。
24. 平台旧版运行诊断已接入：`EnvironmentStatus.legacyRuntime` 报告旧版进程、路径、旧配置状态和 Alt+Q 冲突可能性；设置中心摘要显示 `旧版运行`，本轮 Computer Use 验证为 `未运行`。
25. 启动器设置入口已接入并排序加权：`settings` / `设置` 查询下 `设置中心` 高于 Everything 文件结果，主动作 `打开设置` 进入设置中心。
26. 旧版剪贴板历史、截图历史和工作记忆历史迁移首版已接入 Go migration 服务和设置中心入口；当前由 Go 单测、前端构建和 Wails 构建验证，未自动点击真实迁移按钮。
27. Ariadne 本地数据回滚检查点首版已接入 Go release 服务和设置中心入口；早期由 `internal/release` 单测、前端构建和 Wails 构建验证，最新已补真实用户目录 gated 回滚演练，见本节第 40 条。

### 0.3 本轮最新证据

1. `go test ./...` 通过，包含 `internal/apps`、`internal/applog`、`internal/filesearch`、`internal/launchers`、`internal/search`、`internal/settings`、`internal/hosts`、`internal/workflows`、`internal/ocr`、`internal/migration`、`internal/release`、`internal/releasepack`、`internal/perfcheck`、`internal/toolwindows` 和 `internal/platform` 等当前能力测试。
2. `pnpm build` 通过。
3. `wails3 generate bindings` / `wails3 task windows:build` 最新数量：`469 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`。
4. `wails3 task windows:build` 通过，产物 `experiments\ariadne\bin\ariadne.exe`，大小 `31762432` bytes，更新时间 `2026-06-15 09:57:54`，SHA256 `71D09DBE9F6EF5DC68B1BB636EA63423FAD397C9D93F1DD4D5264830815383C2`。
5. `wails3 task windows:package` 生成 `experiments\ariadne\dist\release\ariadne-dev-windows-x64.zip`，大小 `16638203` bytes，更新时间 `2026-06-15 09:57:55`，SHA256 `04CE7A04F0F48405BF95E68A26AD3E7390FEBD868A96F715C4358EC2DDFB3DDB`；包内含 app、manifest、install/uninstall PowerShell 和 README，manifest 内 `app/ariadne.exe` 为 `31762432` bytes / SHA256 `71d09dbe9f6ef5dc68b1bb636ea63423fad397c9d93f1dd4d5264830815383c2`。
5.1 `wails3 task windows:perf` 最近一次性能报告早于当前 `860x96` 启动器几何修复：冷启动 p95 `686ms` / 目标 `800ms`，已通过 target 但未达 `500ms` ideal；平均工作集 `51179520` bytes，release zip 比旧安装器小 `92.12%`；该报告记录的窗口尺寸/置顶状态不作为当前窗口行为结论；Alt+Q 注册探测记录 `beforeAvailable=true`、`duringBlocked=true`、`duringErrorCode=1409`；Win32 合成 Alt+Q 可见+前台 p95 `22ms` / 目标 `120ms`，探针退出后确认无残留 Ariadne 或 Go 进程。
5.2 `wails3 task windows:search-perf` 通过并写入 `dist\perf\ariadne-search-perf-latest.json`：报告大小 `108169` bytes、时间 `2026-06-14 14:59:24`，`320` 个样本、`16` 个查询、搜索 p95 `7ms` / 目标 `100ms`、rolling p95 `7ms`、`2380` 个 action surface 校验 `0` 失败、Everything 文件结果命中 `1740`。
5.3 `network-mini` 代码与构建验证通过：`go test ./internal/toolwindows ./internal/plugins ./internal/shell -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 均通过；本项覆盖固定尺寸、置顶、四角 anchor、`network_mini_window.json` 持久化、非法 anchor 拒绝、默认全屏自动隐藏和旧配置兼容，只声明代码、构建和单测通过，不声明真实桌面点击或真实全屏隐藏通过。
5.4 插件命令补全与参数面板代码验证通过：`go test ./internal/plugins ./internal/search -v` 覆盖 trigger schema、completion keyword 和搜索聚合；`pnpm build` 覆盖 launcher 参数面板、示例填入和 `prepare_command` 前端类型；当前未做真实桌面渲染验收。
5.5 工作记忆材料导入、排除规则、筛选导出、时间机器回放、独立问题复盘草稿和本地定期草稿调度验证通过：`go test ./internal/workmemory -v` 覆盖 Markdown/文本、图片、PDF、Office Open XML、旧版 Office 元数据、Ariadne 导出 zip、目录/不支持文件、隐私模式、导入排除路径/URL/内容、OCR 排除 URL/内容写回阻断、导出排除计数、按最近时间/标签/条目 ID 筛选导出、选定 evidence 生成问题复盘草稿并跳过敏感记忆，以及定期草稿只使用非敏感 evidence 生成日报/复盘/经验发现；`pnpm build` 覆盖工作记忆中心数据包导入 UI、导出排除计数、筛选导出输入、排除规则配置面板、时间机器回放面板、复盘草稿按钮/侧栏和定期草稿状态面板；Computer Use 只读验证排除规则面板、回放面板和复盘按钮/面板在真实工作记忆中心可见。最新 Win32 截图烟测进一步验证复盘证据多选和定期草稿手动运行：`选择筛选` 只选非敏感记忆，敏感记忆保持禁用，`复盘草稿(2)` 生成后局部反馈 `2 条证据`；`定期草稿` 面板从 `未启用` 手动运行后显示最近运行时间、`2 条非敏感证据` 和日报/复盘/经验发现生成状态。当前未做真实用户数据目录下的材料路径粘贴、筛选导出按钮点击、排除规则编辑/保存或有截图帧回放按钮点击验收。
5.6 JSON 对比大输入预算验证通过：`go test ./internal/jsoncompare -v` 覆盖大行数 unified diff 跳过和超长格式化预览截断；`go test ./...`、`pnpm build` 和 `wails3 task windows:build` 通过；前端 JSON 对比中心显示性能预算提示，但真实桌面长文本滚动体验仍未验收。
5.7 工作记忆窗口切换与本地 Milvus 配置复核：`go test ./...` 通过；临时 `APPDATA` 真实桌面进程烟测启动 `ariadne.exe --hidden`，配置 `timeMachineEnabled=true`、`autoCaptureIntervalSeconds=86400`、`windowSwitchCaptureEnabled=true`、`windowSwitchCooldownSeconds=3`，前台切换到记事本后写入 1 条 `屏幕时间机器窗口切换记录`，`appName=notepad.exe`、`windowTitle=无标题 - 记事本`、`captureId=62586a2876b9`，并写入 1 条截图历史记录；烟测目录和进程均已清理。本机普通 `%APPDATA%\Ariadne\config.json` 与 Codex/MSIX virtualized 配置均已迁到 settings v9，并保留 `ai.enabled=true`、`embeddingEnabled=true`、`embeddingProvider=openai-compatible`、`embeddingBaseUrl=http://10.64.251.169:4000/v1`、`embeddingModel=/model/qwen_eb`、`vectorStoreType=milvus`、`vectorStoreUri=milvus://192.168.1.100:19530`、`vectorCollection=ariadne_work_memory`；API key 不写入 JSON，运行时优先 Windows User env，缺省时回退 Windows Credential Manager。Milvus REST `/v2/vectordb/collections/list` 返回 `code=0`，当前列表尚无 `ariadne_work_memory`，未自动发送真实工作记忆正文，真实 embedding 刷新仍需 Work Memory Center 显式点击 `刷新索引`。
5.8 Windows Credential Manager 验证：新增 `securestore` 真实 Windows 凭据读写层和 `secrets` Wails 服务；设置中心 AI 面板可查看 `Ariadne/OpenAI/APIKey`、`Ariadne/Embedding/APIKey`、`Ariadne/Milvus/Token` 的保存状态、active source、环境变量覆盖状态，并提供保存/二次确认清除。已把当前 OpenAI 与 embedding API key 写入本机 Windows Credential Manager，`cmdkey /list` 可见 OpenAI/Embedding 目标，Milvus token 未提供所以保持未保存；`ARIADNE_TEST_CREDENTIAL_MANAGER=1 go test ./internal/securestore -run TestWindowsCredentialManagerRoundTrip -v` 和 `ARIADNE_TEST_CONFIGURED_CREDENTIALS=1 go test ./internal/securestore -run TestConfiguredAriadneCredentialsReadable -v` 均通过，未打印密钥正文。
5.8a 旧版敏感凭据迁移验证：`settings.ImportLegacyConfig()` 支持从旧版 `config.json` 的 `work_memory` / 顶层 / 嵌套 AI 配置中识别 `ai_api_key`、`openai_api_key`、`OPENAI__API_KEY`、`embedding_api_key`、`embed_api_key`、`EMBED__API_KEY`、`milvus_token`、`MILVUS_TOKEN`、`vector_store_token` 等常见旧字段；导入时把 AI、embedding 和 Milvus 密钥分别写入 Windows Credential Manager 目标 `Ariadne/OpenAI/APIKey`、`Ariadne/Embedding/APIKey`、`Ariadne/Milvus/Token`，不写入 Ariadne JSON。安全存储不可用时会跳过密钥并在 `LegacyConfigStatus.notes` 说明；设置中心旧配置区域已显示这些 notes。验证：`go test ./internal/settings -v`、`go test ./internal/settings ./internal/secrets ./internal/securestore -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:build`、`ARIADNE_TEST_CREDENTIAL_MANAGER=1 go test ./internal/securestore -run TestWindowsCredentialManagerRoundTrip -v` 均通过。`bin\ariadne.exe` 更新为 `31683584` bytes，时间 `2026-06-15 00:49:15`，SHA256 `16C0B4D2C60B6C30D3AB123080DA96D4DB3B23053E01B9CE3F5363351D4FEDB9`。
5.9 截图已有标注编辑验证：`go test ./internal/captureoverlay ./internal/pinnedimage`、`pnpm build` 和 `wails3 task windows:build` 通过；Computer Use 真实桌面复验 `Alt+A` 覆盖层框选、画矩形、按 `V` 进入选择模式、拖动已有矩形并按 Enter 保存，历史记录 `58bc7d4e59b1` 为 `600x449` 且 actions 包含 `overlay,selection,copy,annotated,rect`，结果图显示矩形位于移动后位置；此前同轮验证还覆盖选中矩形后 `Delete` 删除，保存结果 actions 不再包含 `annotated/rect`。本轮产生的测试截图历史条目、PNG 和测试进程均已清理。
5.10 截图文字编辑修复验证：`CaptureOverlayWindow.vue` 修复文字工具点击创建输入框时 annotation canvas button 抢焦点的问题，并新增显式 `dblclick` 路径打开已有文字标注编辑器；`go test ./internal/captureoverlay ./internal/pinnedimage`、`pnpm build` 和 `wails3 task windows:build` 均通过。Computer Use 在修复前已分步证明 `文字` 按钮可见、修复 `pointerdown.prevent` 后文字输入框可出现并能接收 `OLD` 输入、提交后 `选择` 按钮启用；随后发现双击只选中文字但不打开编辑框，因此补了 `dblclick` 处理。最终重建后的桌面复验被 Computer Use 输入层阻断：`launch_app`、`activate_window` 和后续输入连续返回 `GetCursorPos failed: 拒绝访问 (0x80070005)`；本项不声明最终文字双击重编辑桌面验收通过。
5.11 Python 旧插件桥验证：新增 `internal/legacybridge` Go 服务和 `runner.py`，通过受控 JSON stdout 协议 list/execute 旧 `src/plugins`；旧插件导入或执行期间第三方 stdout 噪声会重定向到 stderr，避免污染协议；旧插件结果统一映射为 `plugin_result`，只暴露显式 copy/remember actions，不继承文件动作。插件中心新增 `legacy_python` manifest 和 `legacy <plugin-keyword> [query]` 显式入口，主程序默认挂载旧桥但普通搜索不会自动召回旧 Python 插件。`runner.py` 已嵌入 Go 二进制，源码路径缺失时会写入用户缓存目录再执行，单测覆盖该 fallback。验证：`go test ./internal/legacybridge ./internal/plugins -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 通过；真实旧插件 runner smoke `c 1+1` 返回 `calc_result` 值 `2`。
5.12 工作记忆本地 enrichment 验证：新增 `enrichEntry()` 统一处理本地摘要、内容类型推断、标签补全和敏感兜底；默认 note 会按内容细分为 `json`、`sql`、`command`、`error_log`、`url`、`todo`、`file_path` 或 `code`，文件/截图/OCR 等既有类型保持不被覆盖。分类覆盖 URL、错误、JSON、SQL、待办、命令、路径、API、网络、数据库、配置、开发、代码等本地线索；OCR 文本写回后同样补齐这些标签。验证：`go test ./internal/workmemory -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 通过。
5.13 工作记忆重复/相似画面合并验证：`Entry` 新增 `imageSignature`、`imageFingerprint`、`mergedCount` 和 `lastMergedAt`；`entryFromCapture` 记录截图历史 PNG signature，并从 PNG 生成平均亮度 + 64 位图像指纹。`addEntry` 只对 `time_machine` 来源做合并：完全相同 signature 命中时更新已有 entry 的合并计数、最后合并时间、`重复画面合并` 标签和状态 `LastSkippedReason=重复画面已合并`；signature 不同但尺寸兼容、亮度差和指纹汉明距离均在保守阈值内时，记录 `相似画面合并` 和 `LastSkippedReason=相似画面已合并`。手动补记不参与合并，避免用户主动证据被吞掉。预览 metadata 会显示“重复画面 已合并 N 次”。验证：`go test ./internal/workmemory -run "TimeMachine.*(Duplicate|Similar|Different)" -v`、`go test ./internal/workmemory -v`、`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:build` 通过。
5.14 网络监控小窗多屏策略验证：`NetworkMiniStatus` 新增 `screenMode`、配置 `screenId`、实际 `activeScreenId`、屏幕列表和屏幕标签；`SetNetworkMiniScreenMode(mode, screenID)` 支持 `cursor`、`primary` 和指定屏幕，非法模式或缺失屏幕 ID 不落盘；Windows 下 `cursor` 模式通过 `GetCursorPos` 和屏幕 physical bounds 选择当前屏幕，失败时回退主屏，非 Windows 构建回退主屏。`NetworkMiniWindow.vue` 新增屏幕模式切换按钮，短标签避免小窗 footer 溢出。验证：`go test ./internal/toolwindows -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 通过；`tool_search` 当前未暴露 Computer Use 桌面控制工具，因此本项只声明代码、单测、前端构建和 Wails 构建通过，不声明真实多显示器点击验收。
6. 启动器/工具窗口代码证据：主窗口使用透明无边框 launcher 文档；`open_tool` 通过 `internal/toolwindows` 打开独立工具窗口，主 launcher 隐藏；托盘非 launcher 入口也转为独立工具窗口。
7. 最新桌面复验证据：正常桌面 `Start-Process` 启动最新 `bin\ariadne.exe` 成功；GUI 子系统构建后不再出现黑色控制台窗口。Win32/截图验证折叠 launcher 为 `760 x 96` 浅色无标题栏搜索器，输入 `hosts` 后扩展为 `860 x 468` 结果+右侧预览，Enter 打开 `1120 x 720` 独立 `Hosts 管理` 工具窗口，Alt+Q 召回后只剩折叠搜索器。临时 `APPDATA` 烟测确认启动会写入 `Ariadne\logs\ariadne.log`。
8. Computer Use 打开设置中心，截图验证 `Ariadne 配置 已读回`、`MSIX 实际路径` 和 `2505 bytes` 可见。
9. PowerShell 验证逻辑路径不存在、虚拟化实际路径存在：`VirtualizedExists=true`、`VirtualizedLength=2505`、`VirtualizedLastWriteTime=2026-06-13 19:30:38`、`Version=1`。
10. Computer Use 打开设置中心并滚动左侧栏，截图验证 `平台诊断` 面板可见：`windows/amd64`、`go1.26.4`、`exe 20.0 MB`、`能力 4 已接入 · 4 待接入`、Everything DLL 路径和 Wails PATH 状态。
11. Computer Use 在主搜索输入 `calculator`，验证真实 Start Menu 应用扫描结果 `Calculator` 排在第一，右侧 preview 显示 Windows 应用快捷方式，动作是 `打开应用` 和 `复制快捷方式路径`，没有文件专用动作。
12. 代码证据：`main.go` 已创建 `filesearch.NewService()`，并将其传入 `search.NewService(fileSearchService, appService, launcherService, pluginService, workMemoryService)`。
13. `platform` capability 的 `file_search` 现在跟随 `Everything64.dll` 是否定位成功；当前仓库根目录 DLL 存在，因此平台诊断应显示文件搜索已接入。注意：第 10 条截图是在该能力计数更新前拍的，后续 UI 复验时需要刷新记录。
11. 代码证据：`main.go` 已创建 `launchers.NewService()`，并将其传入 `search.NewService(fileSearchService, appService, launcherService, pluginService, workMemoryService)`，同时作为 Wails service 暴露。
12. `platform` capability 已新增 `custom_launchers` 和 `search_ranking`；第 7 条截图是在这两个 capability 增加前拍的，后续平台诊断 UI 复验时需要刷新记录。
13. Computer Use 输入 `ariadne config`，验证自定义启动项 `Ariadne 配置目录` 返回，右侧 preview 显示 `自定义启动项 · 文件夹`，动作是 `打开`、`复制目标`、`更多`。
14. Computer Use 打开 `更多` 菜单，验证出现 `加入记忆` 和 `收藏`；点击 `收藏` 后 `search_state.json` 写入 Codex MSIX virtualized path，再次打开菜单显示 `取消收藏`。
15. 设置中心自定义启动项编辑 UI 已在真实 Ariadne 窗口中显示；关键截图：`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_20-11-57.png`。
16. Windows UI Automation 在新二进制上确认设置中心存在 `Ariadne 配置目录`、`Everything`、`新建启动项`、`保存启动项`、`保存设置` 等按钮节点。
17. UIA 合成输入不作为启动项“保存落盘”的最终验收依据：它能清空表单，但对 WebView 输入事件与 Vue `v-model` 同步不够可信。启动项持久化当前由 Go 单测覆盖，真实桌面创建/删除仍需用 Computer Use 或人工输入复验。
18. Windows UI Automation 打开剪贴板历史中心，点击 `收集当前剪贴板`，真实写入 `C:\Users\luwei\AppData\Roaming\Ariadne\clipboard_history.json`，文件长度 `473` bytes，记录文本 `ariadne clipboard smoke gateway token 2026-06-13`。
19. Windows UI Automation 点击 `置顶` 后，PowerShell 验证 `clipboard_history.json` 中该条 `pinned=true`。
20. Windows UI Automation 回到启动器，输入 `clip token`，主搜索返回剪贴板历史结果，动作是 `复制内容` 和 `取消置顶`，没有文件动作。关键截图：`C:\Users\luwei\AppData\Local\Temp\ariadne-clipboard-search.png`。
21. 验证后已停止 Ariadne 进程并删除烟测写入的 `clipboard_history.json`，避免污染真实用户数据；视觉证据截图保留在 Temp。
22. Computer Use 打开 Ariadne 真实桌面窗口，验证截图历史中心空状态、`捕获当前屏幕`、`清空未置顶`、`启动器` 和本地持久化说明可见；当前入口应从搜索结果进入，不再使用顶部固定按钮。
23. Computer Use 点击 `捕获当前屏幕`，UI 返回 `1 张`、`2560 x 1440`、`已捕获当前屏幕`，详情动作显示 `打开`、`打开所在文件夹`、`复制路径`、`置顶`、`删除`。
24. Computer Use 点击 `置顶` 后，UI 返回 `置顶 1`、`已置顶`、`取消置顶`。
25. Computer Use 回启动器输入 `cap 2560x1440`，主搜索返回截图历史结果 `截图 06-13 20:58 · 2560x1440`，主动作是 `打开` 和 `打开所在文件夹`；展开 `更多` 菜单后验证 `复制路径`、`取消置顶`、`加入记忆`、`收藏`。
26. PowerShell 验证 Codex 启动场景下截图历史 JSON 写入 MSIX 虚拟化路径：`C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\capture_history.json`，记录 `pinned=true`、`width=2560`、`height=1440`；虚拟化实际 PNG 位于同目录 `capture_images`，大小 `168126` bytes。
27. 截图历史服务已补充 `virtualizedPath`、`virtualizedImageDir` 等状态字段，前端详情会显示 `MSIX 实际目录`，避免后续再用普通 `%APPDATA%` 误判未写文件。
28. 验证后已关闭 Ariadne，并删除普通路径与 MSIX 虚拟化路径下本轮烟测生成的 `capture_history.json` 和 `capture_images`，避免留下桌面截图数据。
29. 代码证据：`main.go` 已创建 `hosts.NewService()`，并作为 Wails service 暴露；`platform` capability 已新增 `hosts`。
30. Hosts 管理中心已可通过主搜索 `hosts` 命令结果打开；不要再恢复顶部固定工具按钮。
31. Computer Use 打开 Hosts 管理中心后，真实 UI 显示旧版 `.x-tools` 迁移出的 6 个启用方案：`公用`、`广告`、`国联`、`算力调度平台`、`CF优选`、`k8s`。
32. Codex MSIX 启动场景下，Hosts 配置实际写入路径为 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\hosts_profiles.json`，当前大小 `4254` bytes。这是旧配置迁移后的真实数据，不是烟测临时数据，不要清理。
33. Computer Use 点击 `生成预览` 后，UI 显示 `最终行数 147`、`新增/移除 +2 / -5`，差异包含 `ARIADNE HOSTS START/END` 新 marker 并移除旧 `X-TOOLS HOSTS START/END` marker。
34. Hosts 写入系统文件未做烟测；当前只验证了预览和二次确认前的 UI，不触发 UAC 写入。
35. 代码证据：`main.go` 已创建 `workflows.NewService(pluginService)`，将其传入 `search.NewService(...)` 并作为 Wails service 暴露；`platform` capability 已新增 `workflow_macros`。
36. 工作流服务首次加载时从旧版 `C:\Users\luwei\AppData\Roaming\x-tools\config.json` 导入 workflows，并写入 Codex MSIX virtualized path：`C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\workflows.json`，当前大小 `339` bytes，包含 `clip-url-md5` 两步工作流。
37. Computer Use 打开工作流中心后，真实 UI 显示 `工作流宏`、`已导入旧配置`、`1 个`、`新建工作流`、`保存工作流`、`删除工作流`、`运行`、变量 `{clipboard}`/`{input}`/`{prev}` 和旧配置路径。
38. Computer Use 临时设置剪贴板为 `ariadne workflow smoke 2026-06-13` 后点击 `运行`，UI 显示两步执行：`url ariadne workflow smoke 2026-06-13` -> `hash ariadne+workflow+smoke+2026-06-13`，最终 MD5 为 `dd0432f9fd7c831001f1203372ec3755`，并显示 `工作流完成：clip-url-md5（共 2 步），结果已复制`。
39. 验证后已恢复原剪贴板文本、关闭 Ariadne，并确认没有残留 Ariadne 进程。
40. Computer Use 对启动器搜索框的 UIA `SetValue` 仍报 `0x80070057`，真实键盘输入能写入查询但 UIA 文本树没有稳定返回结果区；因此主搜索 `wf ...` 桌面验收暂不计入完成，当前由 `internal/search` Go 测试证明搜索聚合。
41. 代码证据：`main.go` 已创建 `jsoncompare.NewService()` 并作为 Wails service 暴露；`platform` capability 已新增 `json_compare`。
42. `internal/jsoncompare/service_test.go` 覆盖对象 key 顺序不算语义差异、新增/删除/变更路径、解析错误行列和非标识符 key 路径。
43. 前端已新增 `JsonCompareCenter.vue`、`stores/jsonCompare.ts`、`services/jsonCompareApi.ts`，并通过 `appShell.openJsonCompare()`、`jsondiff` 命令结果和 `open_json_compare` 路由进入。
44. Computer Use 启动真实 Ariadne 窗口后，默认只显示浅色折叠搜索框；输入 `jsondiff` 后展开唯一命令结果 `打开 JSON 对比工具`，按 Enter 打开 `JSON 对比中心`，默认样例显示 `存在差异`、`发现 4 处差异：新增 2，删除 1，变更 1`，语义报告包含 `~ $.items[1]: 2 -> 3`、`+ $.items[2]: 4`、`- $.meta.drop: 1`、`+ $.meta.add: 2`。
45. Computer Use 点击 `格式化两侧` 后，页面保持正确差异结果；本轮未点击 `复制报告`，避免覆盖用户真实剪贴板。
46. 验证后已关闭 Ariadne，并确认没有残留 Ariadne 进程。
47. 桌面壳代码证据：`main.go` 已接入 `shell.NewManager(...)`，配置 Wails `SingleInstance`、`Windows.DisableQuitOnLastWindowClosed`、隐藏式 `--hidden` 启动参数、托盘图标和 shutdown 热键清理。
48. 桌面壳代码证据：`internal/shell` 已新增 Windows `RegisterHotKey` 封装、热键解析测试、Wails SystemTray 菜单、关闭隐藏到托盘、`ExecJS` + Wails event 双通道前端导航。
49. 设置联动证据：`settings.RegisterChangeHandler(...)` 会在 `runOnStartup` 改变时调用 Wails Autostart；当前未自动写注册表，开机启动注册仍需用户打开设置开关后做人工烟测。
50. 平台诊断证据：`platform.WithShellStatus(...)` 让 `single_instance`、`global_hotkey`、`tray`、`autostart` 根据真实 shell runtime 状态显示，不再静态写死为未完成。
51. Computer Use + Win32 验证：关闭旧版 `x-tools.exe` 和旧 Ariadne 后，Alt+Q 探测先成功，说明热键空闲；启动最新 Ariadne 后临时注册同一 Alt+Q 返回 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)`，说明 Ariadne 已占用。
52. Computer Use + Win32 验证：当时 `bin\ariadne.exe` 启动后窗口尺寸为 `760x132`；输入 `jsondiff` 并回车后窗口尺寸为 `1180x760`；从资源管理器按 Alt+Q 后窗口回到 `760x132`。最新窗口尺寸已在 0.0 状态卡更新为 `760x112`。
53. 验证发现：旧版 `C:\Users\luwei\AppData\Local\Programs\x-tools\x-tools.exe` 同时运行时会抢占 Alt+Q，Ariadne 启动时无法注册热键。后续旧版并存/迁移策略必须处理该冲突，不能把它误判为 Ariadne 热键代码失效。
54. 网络监控代码证据：新增 `internal/networkmonitor`，Windows 使用 `GetIfEntry2Ex` 读取非 loopback 启用网卡的 `InOctets`/`OutOctets`，服务层按快照时间差计算上传/下载 B/s。
55. 网络监控前端证据：新增 `NetworkMonitorCenter.vue`、`NetworkMiniWindow.vue`、`stores/networkMonitor.ts`、`services/networkMonitorApi.ts`、`services/toolWindowsApi.ts`、`toolwindows` bindings 和 `networkmonitor` Wails bindings；`AppView` 新增 `network-monitor` 和 `network-mini`，中心窗口尺寸为 `980x640`，当前任务栏小条尺寸为 `156x40`。
56. 网络监控搜索证据：`plugins` 新增 `network_monitor` manifest，关键词 `net`、`network`、`traffic`、`网速`、`网络监控`；`toolWindowResult` 分数提升到 `120`，新增 `TestExactPluginCommandBeatsEverythingFileResults`，确保精确插件命令不被 Everything 文件结果压过；`net mini` / `net 小窗` 返回 `network-mini` 结果，主动作是 `open_network_mini`。
57. 主题代码证据：新增 `frontend/src/lib/theme.ts`，启动时从 settings 同步主题；设置保存、重置、旧配置导入后立即 `applyTheme`；默认归一化为 `light`，黑色只来自 v7+ 显式 `dark`。
58. Computer Use 验证：当时新版 `bin\ariadne.exe` 启动即为 `760x132` 折叠启动器；输入 `net` 后第一项是 `打开网络监控`，不再被 Everything 文件结果压过。最新窗口尺寸已在 0.0 状态卡更新为 `760x112`。
59. Computer Use 验证：点击 `打开网络监控` 主动作后窗口为 `980x640`，文本树显示 `网络监控`、`刷新中`、`2 / 2 网卡`、`下载 545 KB/s`、`上传 13.5 KB/s`、`1.0 Gbps`、`本机计数` 和 `1s 刷新`。
60. Computer Use 验证：在网络监控中心按 Alt+Q 后窗口回到折叠启动器，文本树只显示 `Ariadne launcher` 和搜索输入；这证明 Alt+Q 首屏仍是干净搜索器，不是固定工具集合。
60.1 网络监控小窗代码证据：`internal/toolwindows` 接受 `network-mini`，当前固定 `156x40`、默认 `taskbar-left`，禁用 resize，按任务栏左侧区域推导位置；Windows 下 `taskbar_windows.go` 将窗口 owner 设为 `Shell_TrayWnd` 并 `SetWindowPos(HWND_TOPMOST|SWP_NOACTIVATE)`，打开时不抢焦点；`network_mini_window.json` 继续兼容旧四角位置、屏幕模式、指定屏幕 ID 和全屏自动隐藏开关；`internal/shell` 托盘菜单新增 `网速小窗`；前端中心页新增 `小窗` 图标按钮，小窗内只保留上下行速率显示。
61. 工作记忆代码证据：`internal/workmemory` 现在接入 `capturehistory.Service`，`CaptureCurrentScreen()` 和 `CaptureTimeMachineNow()` 会写入截图历史并在工作记忆条目中保存 `captureId`、`imagePath`、尺寸和字节数。
62. 工作记忆前端证据：`WorkMemoryCenter.vue` 会按 `captureId` 加载截图预览；`stores/workMemory.ts` 的时间机器/隐私开关会先持久化到 settings，再同步 workmemory 服务。
63. 设置主题代码证据：`currentSettingsVersion=7`，v7 前旧实验 `dark` / `system` 会迁移并写回为 `light`，`normalizeTheme` 只保留 v7+ 显式 `dark` 作为深色模式；新增测试覆盖旧 dark/system 迁移持久化和当前 dark 保留。
64. 最新验证命令：`go test ./...` 通过；`pnpm build` 通过；`wails3 task windows:build` 通过。
65. 当时 bindings：`429 Packages, 13 Services, 85 Methods, 5 Enums, 59 Models, 0 Events`。
66. 当时产物：`experiments\ariadne\bin\ariadne.exe`，大小 `23477760` bytes，更新时间 `2026-06-14 01:25:53`。
67. Computer Use 启动最终 exe 后曾返回 stale 句柄；通过 `list_windows` 重新选择当前窗口后，验证折叠启动器为 `760x112`，输入 `net` 后展开为 `760x430`，首个结果是 `打开网络监控`。
68. 本轮未把工作记忆时间机器按钮点击作为桌面验收结论：Wails WebView 在 Computer Use 下没有稳定文本树，盲点坐标未产生 `work_memory.json`/`capture_history.json`；后端真实采集链路由 Go 测试覆盖。
69. 二维码识别代码证据：新增 `internal/qrscan` Wails service，`DecodeCapture(captureId)` 读取截图历史图片并用 `gozxing/qrcode` 解码；`DecodeCurrentScreen()` 显式捕获当前屏幕后解码，不后台扫屏。
70. 二维码识别前端证据：新增 `frontend/src/services/qrScanApi.ts`，截图历史中心新增 `识别当前屏幕`、`识别二维码`、局部结果面板和 `复制内容`；启动器对截图结果的 `recognize_qr` action 会识别并内联反馈。
71. 二维码识别搜索证据：插件注册表新增 `qr_scan` manifest，关键词 `qrscan`、`scanqr`、`识别二维码`、`二维码识别` 打开截图历史中心。
72. 最新验证命令：`go test ./...` 通过，包含 `internal/qrscan` 对生成 QR PNG、空白 PNG 和缺失 capture ID 的测试；`pnpm build` 通过；`wails3 task windows:build` 通过。
73. 当时 bindings：`429 Packages, 13 Services, 85 Methods, 5 Enums, 59 Models, 0 Events`。
74. 当时产物：`experiments\ariadne\bin\ariadne.exe`，大小 `23477760` bytes，更新时间 `2026-06-14 01:25:53`。
75. Computer Use 本轮可启动 Ariadne；对无边框 Wails WebView 偶发 stale 句柄时，需刷新 `list_windows` 后再取当前窗口。二维码识别链路仍由 Go 测试覆盖，二维码按钮点击不计为桌面验收。
76. 验证期间临时写入的 `qr-smoke-20260613` 截图历史和 `capture-qr-smoke.png` 已在确认该路径没有既有截图历史后删除，避免污染用户数据。
77. 剪贴板自动监听代码证据：`internal/clipboardhistory` 新增 Windows `CF_UNICODETEXT` 读取、后台 watcher、启动基线、隐私模式/剪贴板来源暂停和 watcher 状态字段。
78. 剪贴板自动监听前端证据：剪贴板中心显示 `监听中` / `监听暂停` 和 watcher 错误原因，打开中心时会轻量刷新列表。
79. 剪贴板自动监听验证：`go test ./...` 通过，新增测试覆盖启动基线不记录旧内容、变化后记录 `clipboard_watcher`、重复内容跳过、错误上报和隐私/来源暂停。
80. 剪贴板自动监听真实进程烟测：临时 `APPDATA` 隐藏启动 `ariadne.exe --hidden`，设置系统剪贴板后临时 `clipboard_history.json` 写入 1 条 `source=clipboard_watcher` 记录；测试结束恢复剪贴板并删除临时目录。
81. 最新验证命令：`go test ./...` 通过；`pnpm build` 通过；`wails3 generate bindings` 为 `429 Packages, 13 Services, 85 Methods, 5 Enums, 59 Models, 0 Events`；`wails3 task windows:build` 通过。
82. 图片剪贴板历史代码证据：`internal/clipboardhistory` 新增 `EntryImage`、`clipboard_images` PNG 存储、Windows `CF_DIB/CF_DIBV5` 读取、PNG 转 `CF_DIB` 复制回剪贴板、`ImageDataURL(id)` 预览接口和图片文件生命周期清理。
83. 图片剪贴板历史前端证据：`ClipboardCenter.vue` 可显示图片数量、图片行图标、图片尺寸、data URL 预览和 `复制图片` 动作；启动器 `copy_clipboard_image` action 走后端复制图片，不当作文本处理。
84. 图片剪贴板历史验证：`internal/clipboardhistory` 测试覆盖图片持久化、搜索、PreviewImage action surface、data URL、删除/清空时删除 PNG、watcher 图片基线不污染历史。
85. 图片剪贴板真实进程烟测：临时 `APPDATA` 隐藏启动 `ariadne.exe --hidden`，向系统剪贴板写入 7x5 bitmap 后，临时 `clipboard_history.json` 写入 1 条 `source=clipboard_watcher` 图片记录，PNG 文件存在；测试结束恢复文本剪贴板并删除临时目录。
86. 旧版运行诊断代码证据：`internal/platform` 新增 Windows Toolhelp 进程扫描，跳过当前进程并检测 `x-tools.exe`；`EnvironmentStatus.legacyRuntime` 暴露进程、路径、旧配置和 `hotkeyConflictLikely`。
87. 设置入口排序代码证据：`internal/search` seed results 新增 `settings-center`，主动作是 `open_tool/open_settings`；回归测试覆盖 `设置` 和高分文件结果同场时设置中心排第一。
88. 窗口验收钩子：`frontend/src/stores/appShell.ts` 会为不同 view 设置 `document.title`，不影响无边框视觉，但可作为后续 Computer Use/Win32 验收辅助信号。
89. Computer Use 最新桌面验证：启动最终 exe 后是浅色 `760x112` 折叠搜索器，无原生标题栏；输入 `settings` 展开为 `760x430`，首项为 `设置中心`，Enter 打开设置中心 `1120x720`；设置中心左侧摘要显示 `旧版运行 未运行`。
90. 最新验证命令：`go test ./...` 通过；`pnpm build` 通过；`wails3 task windows:build` 通过；Vite/Rolldown 仍只有 `@vueuse/core` pure annotation 非致命警告。
91. 当时 bindings：`429 Packages, 13 Services, 85 Methods, 5 Enums, 59 Models, 0 Events`。
92. 当时产物：`experiments\ariadne\bin\ariadne.exe`，大小 `23477760` bytes，更新时间 `2026-06-14 01:25:53`。
93. 贴图窗口代码证据：新增 `internal/pinnedimage` Wails service，支持 `OpenCapture`、`OpenClipboardImage`、`OpenQRText`、`GetPinned` 和 `ClosePinned`；`main.go` 已注册该服务并在创建 app 后 `Attach(app)`。
94. 贴图窗口动作证据：截图历史结果新增显式 `pin_capture_image` action，剪贴板图片结果新增显式 `pin_clipboard_image` action；启动器 store 对这两个 action 和 QR `pin_qr` 走 `pinnedImageApi`，不再落到通用“已发送”反馈。
95. 贴图窗口前端证据：新增 `PinnedImageWindow.vue` 和 `pinnedImageApi.ts`；`App.vue` 对 `?view=pinned-image&pinId=...` 只渲染贴图窗口，不安装主 shell 导航监听，也不会调用 `openLauncher()`。
96. 贴图窗口 UI 证据：截图历史中心详情动作新增 `创建贴图`，剪贴板图片详情动作新增 `贴到屏幕`；贴图窗口透明无边框、始终置顶，支持拖动、滚轮缩放、双击关闭、复制来源、缩放重置、阴影开关和关闭。
97. 贴图窗口测试证据：新增 `internal/pinnedimage` 单测覆盖截图贴图、剪贴板图片贴图、二维码贴图和窗口创建失败清理；截图历史和剪贴板历史单测覆盖新增 action id。
98. 当时验证命令：`go test ./...` 通过；`pnpm build` 通过；`wails3 generate bindings` 通过；`wails3 task windows:build` 通过；Vite/Rolldown 仍只有 `@vueuse/core` pure annotation 非致命警告。
99. 当时 bindings：`458 Packages, 18 Services, 129 Methods, 5 Enums, 106 Models, 0 Events`。
100. 当时产物：`experiments\ariadne\bin\ariadne.exe`，大小 `30134784` bytes，更新时间 `2026-06-14 11:08:37`。
101. Computer Use 贴图窗口验证：启动真实 Ariadne 后，主窗口仍是 `Ariadne` launcher；搜索 `capture` 打开截图历史中心，点击 `捕获当前屏幕` 后详情动作包含 `创建贴图`；点击后出现独立窗口 `截图贴图 2560x1440`，Computer Use 读取窗口尺寸 `720x425`，文本树包含 `截图历史`、`复制`、`缩小`、`原始比例`、`阴影`、`关闭` 和图片图形；点击 `关闭` 后只剩主 `Ariadne` 窗口。
102. OCR 代码证据：新增 `internal/ocr` 服务和嵌入式 `rapidocr_bridge.py`，`main.go` 注册为 Wails service，并把 OCR capability 暴露到平台诊断。
103. OCR 前端证据：截图历史中心、剪贴板历史中心和工作记忆中心已接入 OCR 入口；工作记忆图片 OCR 成功后会写回 `ocrText` 和 `ocrStatus`，可被关键词搜索。
104. OCR 验证证据：RapidOCR bridge 对临时 PNG 返回 `Ariadne / OCR / smoke 2026` 文本；Computer Use 验证真实截图历史中心点击 `OCR 当前屏幕` 后返回 `OCR 文字识别` 面板和 `复制文字` 按钮。
105. OCR 文本选择代码证据：新增 `frontend/src/lib/ocrSelection.ts` 和 `frontend/src/lib/ocrDisplay.ts`；截图历史、剪贴板历史和工作记忆 store 共享行级选择逻辑，UI 支持逐行选择、全选、清空、复制选中和复制全文。
106. OCR 文本选择桌面证据：Computer Use 验证真实截图历史中心 `OCR 当前屏幕` 后返回 `125 行`、`已选 0`、`全选`、`清空`、禁用的 `复制选中`、`复制全文` 和逐行置信度/坐标；点击 `全选` 后变为 `已选 125`，`复制选中` 可用。
107. OCR 图片叠框代码证据：新增 `frontend/src/components/ocr/OCRImageOverlay.vue`，截图历史、剪贴板历史和工作记忆图片预览共用该组件；叠框使用 OCR `rect` 按原图尺寸映射到预览图百分比坐标，并通过共享 `ocrSelection` 状态联动文本行选择。
108. OCR 图片叠框桌面证据：Computer Use 验证真实截图历史中心 `OCR 当前屏幕` 后图片预览出现 `选择 OCR 第 X 行` 叠框；修正命中层后，真实坐标点击叠框使状态变为 `131 行 · 已选 1`，`复制选中` 可用；本轮测试截图记录和 PNG 已清理。
109. 贴图 OCR 联动代码证据：`pinnedimage.PinnedImage` 新增 `canOcr` 能力位，截图和剪贴板图片贴图可 OCR，二维码贴图不暴露 OCR；`PinnedImageWindow.vue` 复用 `recognizeCaptureOCR` / `recognizeClipboardImageOCR`、`OCRImageOverlay` 和 `ocrSelection`，提供 OCR 按钮、叠框选择和复制按钮组。
110. 贴图 OCR 联动桌面证据：Computer Use 验证真实 `截图贴图 2560x1440` 窗口含 `OCR 文字识别` 按钮，点击后显示 `110 行 · 已选 0`、逐行 `选择 OCR 第 X 行`、`全选`、`清空`、禁用的 `复制选中` 和 `复制全文`；选择第 1 行后变为 `已选 1`，`复制选中` 可用。验证后已关闭窗口并结束无窗口测试进程。
111. 截图高级编辑代码证据：`internal/captureoverlay.SelectionRequest` 新增 `operations` 和 `savedPath`，`AnnotationOperation` 支持 `rect`、`arrow`、`mosaic`；`CaptureSelection` 会渲染选区 PNG、应用标注、写入截图历史，并在 `save_as` 时另存外部 PNG。
112. 截图高级编辑前端证据：`CaptureOverlayWindow.vue` 新增矩形、箭头、马赛克、撤销、清空、另存按钮和快捷键，选区内 SVG 预览与后端 `operations` 保持结构化协议。
113. 截图高级编辑测试证据：`internal/captureoverlay` 单测覆盖矩形标注像素、马赛克改写、`save_as` 自动补 `.png`、外部文件写入、`SavedPath` 回传，以及 `annotated` / `mosaic` / `save_as` action tags。
114. 截图高级编辑桌面复验历史阻塞：早前 Codex/PowerShell 启动上下文运行该轮 `ariadne.exe --hidden` 会在 Wails v3.0.0-alpha.98 应用初始化阶段 fatal `GetCursorPos failed`，窗口尚未创建；该启动阻塞已由本地 `third_party/wails/v3` screen fallback 补丁解除，当前最新 exe 可通过 Win32 启动烟测。
115. 图片 OCR 索引代码证据：新增 `internal/imageindex` Wails service，持久化到 `%APPDATA%\Ariadne\image_index.json`；支持 `IndexRecent`、`IndexCapture`、`IndexClipboardImage`、`Search`、`Status` 和 `List`，并注册进主搜索 provider 链。
116. 图片 OCR 索引启动器证据：`img index`、`image index`、`ocr index`、`图片索引` 返回 `索引最近图片 OCR` 结果；前端 `launcher` 对 `image_index_recent` 走 `imageIndexApi.indexRecentImages`，反馈 `已索引 N 张，跳过 N 张`，不落到通用“已发送”。
117. 图片 OCR 索引隐私证据：批量 OCR 识别到疑似密码、token、密钥时，索引条目保留 `sensitive/redacted` 状态但不写入 `text`，主搜索不会命中敏感正文。
118. 图片 OCR 索引测试证据：`internal/imageindex` 单测覆盖截图+剪贴板图片批量索引、重复跳过、`ocr` 前缀搜索、显式 action surface、剪贴板结果不暴露 `open_parent`、敏感 OCR 正文不可搜索。
119. 工作记忆本地检索代码证据：`internal/workmemory.Service.Search` 现在优先读取 SQLite FTS5 命中，再做内存关键词匹配和本地语义相似度打分；命中结果设置 `Score`，并在 preview evidence 中显示 `匹配=SQLite FTS`、`匹配=关键词` 或 `匹配=本地语义匹配`，FTS 命中会附带 `命中` snippet。
120. 工作记忆本地检索状态证据：持久化工作记忆会初始化 `work_memory.fts.sqlite`，`SemanticStatus()` 返回 `provider=sqlite_fts5+local_term_vector`、`ftsEnabled=true`、`external=false`；无路径测试实例仍降级为 `local_term_vector`。这是当时本地 FTS + token/短语向量阶段的证据；当前外部 embedding 与 embedded/Milvus 向量存储状态以 0.0 状态卡和最近验证为准。
121. 工作记忆本地语义检索测试证据：`TestSemanticSearchFindsRelatedTechnicalMemory` 覆盖中文查询 `数据库连不上` 命中英文 `PostgreSQL / connection refused` 记忆；`TestSemanticStatusIsHonestLocalProvider` 防止把本地语义检索误报为外部向量库。
122. 工作记忆保留策略代码证据：`internal/workmemory.ApplyRetentionPolicy(retentionDays, keepFavorites)` 会按 `CreatedAt` 清理过期非收藏条目，并保留 `keepFavoritesForever=true` 的收藏条目；`main.go` 在设置变更和启动初始化时调用该策略。
123. 图片索引保留策略代码证据：`internal/imageindex.ApplyRetentionPolicy(retentionDays)` 会清理过期索引，以及源截图历史/剪贴板图片记录已不存在的 stale 派生索引；`main.go` 在设置变更和启动初始化时调用该策略。
124. 保留策略测试证据：`TestRetentionPolicyRemovesOldEntriesButKeepsFavorites` 覆盖过期非收藏清理和收藏保留；`TestRetentionPolicyRemovesExpiredAndStaleEntries` 覆盖过期图片 OCR 索引与 stale 源记录清理。
125. 截图历史保留策略代码证据：`internal/capturehistory.ApplyRetentionPolicy(retentionDays, keepPinned)` 会清理过期未置顶截图记录，并通过现有安全删除逻辑移除对应 PNG；`keepFavoritesForever=true` 时保留置顶截图。
126. 剪贴板历史保留策略代码证据：`internal/clipboardhistory.ApplyRetentionPolicy(retentionDays, keepPinned)` 会清理过期未置顶文本和图片记录，图片条目会同步删除 `clipboard_images` 内对应 PNG；`keepFavoritesForever=true` 时保留置顶条目。
127. 历史保留策略测试证据：`TestCaptureHistoryRetentionRemovesOldUnpinnedImages` 覆盖截图记录和 PNG 清理、置顶保留、近期保留；`TestClipboardHistoryRetentionRemovesOldUnpinnedEntriesAndImages` 覆盖剪贴板文本/图片清理、图片文件清理、置顶保留、近期保留。
128. 缩略图分层代码证据：新增 `internal/imagepreview.CreatePNGThumbnail`，截图历史和剪贴板图片写入大图时会生成 512px 长边预览 PNG，分别放入 `capture_thumbnails` 和 `clipboard_thumbnails`；服务加载时会为旧大图补齐缺失缩略图。
129. 缩略图分层协议证据：`capturehistory.Entry` 和 `clipboardhistory.Entry` 新增 `thumbnailPath`、`thumbnailWidth`、`thumbnailHeight`、`thumbnailBytes`；两个服务均新增 `ThumbnailDataURL`，状态里暴露缩略图目录、数量和字节数。
130. 缩略图分层前端证据：截图历史中心和剪贴板历史中心的图片预览改为优先调用 `ThumbnailDataURL`；打开、复制图片、加入截图历史、OCR 和贴图仍使用原图路径。
131. 缩略图分层测试证据：`TestCaptureHistoryCreatesThumbnailAndDeletesItWithEntry`、`TestCaptureHistoryBackfillsMissingThumbnail`、`TestClipboardHistoryCreatesThumbnailAndDeletesItWithImage` 和 `TestClipboardHistoryBackfillsMissingThumbnail` 覆盖缩略图元数据、旧记录缺失缩略图回填、文件存在、DataURL、状态统计，以及删除时原图/缩略图同步清理。
132. 启动器桌面烟测证据：Computer Use 对最终 exe 先遇到旧 stale window id，按当前窗口 id 恢复后成功抓取主窗口；输入 `capture` 后文本树包含 `打开截图历史中心`、`区域截图` 和 `搜索 38ms`，清空后回到仅搜索框的 `760x112` 折叠态。
133. 时间机器 worker 代码证据：`internal/workmemory/service.go` 新增 `WorkerRunning`、`LastSkippedAt`、`LastSkippedReason` 状态字段，`ApplySettings` 会在 interval 变更且时间机器运行中时重启后台 worker，`captureScreen` 在截图前检查隐私和采集排除策略。
134. 时间机器设置策略证据：`main.go` 启动和 settings 变更时会调用 `ApplyCapturePolicy`，将 `workMemory.excludeApps` 和 `excludeWindowKeywords` 同步到 workmemory 服务，再应用时间机器 interval 和开关状态。
135. 时间机器前端状态证据：`WorkMemoryCenter.vue` 显示 `暂停` / `运行中` / `待启动`，统计区新增 `跳过`，并在 `pauseReason` 或 `lastSkippedReason` 存在时显示局部状态；`types/ariadne.ts` 与 `stores/workMemory.ts` 已同步新状态字段。
136. 时间机器测试与桌面证据：新增 workmemory 单测覆盖 worker interval 自动采集、排除 app、排除窗口标题和 interval 设置状态；`go test ./...`、`wails3 generate bindings`、`pnpm build`、`wails3 task windows:build` 均通过，Computer Use 验证 `memory` 可打开 `1120x720` 工作记忆中心并显示 `时间机器 暂停`、`隐私 关闭`、`采集`、`跳过`。
137. 时间机器空闲/锁屏代码证据：`internal/workmemory/activity_windows.go` 通过 `GetLastInputInfo` 读取空闲秒数，并通过输入桌面可切换状态判断锁屏/不可交互桌面；非 Windows provider 明确返回 unavailable，不伪造能力。
138. 时间机器保护策略证据：`CapturePolicy` 新增 `captureScope`、`multiMonitor`、`pauseOnIdle`、`idlePauseSeconds`、`pauseOnLock`，`main.go` 会从 settings 同步这些字段；`captureScreen` 只对自动 `time_machine` 应用空闲/锁屏暂停，用户手动补记不被空闲暂停拦截。
139. 时间机器 UI 证据：工作记忆中心显示 `范围`、`保护`、`策略 全部屏幕 / 合并 · 空闲阈值 10m · 锁屏暂停`；设置中心工作记忆采集区显示 `空闲暂停`、`锁屏暂停`、`空闲阈值秒`、`采集范围`、`多屏策略`，并将策略输入同排显示避免被裁切。
140. 时间机器保护测试证据：`TestTimeMachinePauseOnIdleSkipsCapture`、`TestManualCaptureIgnoresIdlePause`、`TestTimeMachinePauseOnLockSkipsCapture`、`TestCaptureStrategyIsRecordedOnCaptureEntries` 覆盖暂停、手动例外和策略 metadata；Computer Use 验证当时 `24000512` bytes exe 的工作记忆中心和设置中心文本树。
141. 截图范围执行代码证据：`capturehistory.CaptureScreenWithOptions(source, CaptureOptions)` 新增 Wails/Go 服务方法；Windows 侧通过 `GetForegroundWindow` / `GetWindowRect` 捕获前台窗口，通过 `EnumDisplayMonitors` / `GetMonitorInfoW` 枚举显示器，通过主屏 system metrics 捕获主屏。
142. 截图范围执行数据证据：截图历史条目 actions 会写入 `primary_screen`、`primary_only`、`per_monitor`、`monitor_N` 等策略，tags 会写入 `范围:*`、`多屏:*`、`区域:x,y,wxh`、`显示器:n/m`；搜索 haystack 会包含这些 tags/actions。
143. 工作记忆范围执行证据：`workmemory.OptionScreenCapturer` 会优先调用 `CaptureScreenWithOptions`，把 `CapturePolicy.captureScope` / `multiMonitor` 传给截图历史；无该接口的测试或 legacy capturer 仍回退到 `CaptureScreen`。
144. 自动采集烟测证据：临时 `APPDATA` 启动 `ariadne.exe --hidden`，10 秒 interval 后真实写入工作记忆、截图历史和 PNG；写入记录带 `范围:主屏幕`、`多屏:仅主屏`、`primary_screen`、`primary_only` 和 `区域:0,0,3840x2160`，验证后临时目录已删除。
145. 自动 OCR 代码证据：`workmemory.RegisterAutoOCRProcessor` 在 `main.go` 中把工作记忆服务接到 `ocrService.RecognizeWorkMemory`，`applyWorkMemoryRuntime` 会把 `settings.WorkMemory.AutoOCR` 同步到 `CapturePolicy.AutoOCR`；`processAutoOCR` 在截图型工作记忆写入后同步调用 OCR 并记录 `lastAutoOcrAt`、`lastAutoOcrId` 和 `lastAutoOcrError`。
146. 自动 OCR 前端状态证据：`WorkMemoryStatus`、`workMemoryApi`、`stores/workMemory` 和 `WorkMemoryCenter.vue` 已同步 `autoOcrEnabled`、`lastAutoOcrAt`、`lastAutoOcrId`、`lastAutoOcrError`，工作记忆中心统计区会显示 `OCR 自动`、`OCR 手动` 或错误状态。
147. 自动 OCR 测试证据：新增 `TestAutoOCRProcessorRunsAfterCaptureWhenEnabled`、`TestAutoOCRProcessorDoesNotRunWhenDisabled`、`TestAutoOCRProcessorFailureIsRecorded`，覆盖开启后写回、关闭后不运行和失败状态记录；旧时间机器测试已显式关闭 idle/lock 暂停，避免受当前桌面空闲状态影响。
148. 自动 OCR 临时进程烟测证据：临时 `APPDATA` 启动 `ariadne.exe --hidden`，配置 `autoOcr=true`、`timeMachineEnabled=true`、`captureScope=primary_screen`、`multiMonitor=primary_only`、`pauseOnIdle=false`、`pauseOnLock=false`；最新工作记忆 `ocrStatus=done:rapidocr_onnxruntime`、OCR 文本长度 `2489`，截图记录 actions 为 `screen,primary_screen,primary_only`，未输出 OCR 正文，验证后进程和临时目录均已清理。
149. SQLite FTS 代码证据：新增 `internal/workmemory/fts.go`，使用 `zombiezen.com/go/sqlite` 打开 FTS5 虚表；`work_memory.json` 仍是权威数据，`work_memory.fts.sqlite` 是可重建索引，保存、OCR 写回、删除、清理和保留策略后都会重建索引；索引失败只进入 `SemanticStatus.LastIndexError`，不阻断 JSON 保存。
150. SQLite FTS 测试证据：新增 `TestSQLiteFTSIndexPersistsAndReportsEvidence`、`TestSQLiteFTSIndexRebuildsAfterOCRAndDelete`、`TestSemanticStatusReportsSQLiteFTSForPersistentStore`，覆盖重载后 FTS 命中、snippet evidence、OCR 写回后索引、删除后索引清理和状态上报。
151. SQLite FTS 临时进程烟测证据：临时 `APPDATA` 启动 `ariadne.exe --hidden`，时间机器写入工作记忆和截图历史后生成 `work_memory.fts.sqlite`，大小 `4096` bytes；烟测未输出截图/OCR 正文，验证后进程和临时目录均已清理。
152. 工作记忆经验发现代码证据：`internal/workmemory.Service.DiscoverExperiences(periodDays)` 会过滤敏感条目和周期外条目，按本地可解释规则生成 `repeated_issue`、`automation_opportunity`、`knowledge_gap` 三类 insight；每条 insight 带 `reason`、`recommendation`、`evidence`、`confidence`、`severity` 和 `requiresReview=true`，不会自动执行建议。
153. 工作记忆经验发现前端证据：`frontend/src/components/workmemory/WorkMemoryCenter.vue` 新增 `经验发现` 侧栏、`发现经验` 按钮、置信度/类型/严重度/证据数量展示和 `转任务包` 动作；`stores/workMemory.ts` 会把 insight 转为 agent task package 草稿。
154. 工作记忆经验发现测试证据：新增 `TestDiscoverExperiencesFindsRepeatedIssuesAndAutomation` 和 `TestDiscoverExperiencesExcludesSensitiveEntries`，覆盖重复 PostgreSQL 连接问题、自动化机会识别、知识沉淀缺口以及敏感条目排除。
155. 工作记忆经验发现验收证据：`go test ./internal/workmemory -run "DiscoverExperiences|Drafts|Semantic|SQLiteFTS|OCR|Search" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 均通过；bindings 为 `457 Packages, 17 Services, 123 Methods, 5 Enums, 99 Models, 0 Events`，exe 为 `30043648` bytes、`2026-06-14 10:36:49`。Computer Use 本轮未完成按钮点击验收，原因是可枚举 Ariadne 窗口但状态捕获返回 `foreground window did not report a process id`。
156. 工作记忆经验发现决策代码证据：`internal/workmemory.Service.SetExperienceInsightDecision` 支持 `accepted`、`rejected`、`later`、`task_package` 和清除状态；决策持久化为 `experienceDecisions`，`DiscoverExperiences` 会把 `decisionStatus`、`decisionUpdatedAt` 和 `taskPackageId` 回填到 insight。
157. 工作记忆经验发现决策前端证据：`WorkMemoryCenter.vue` 在每条 insight 上显示 `待处理/已接受/已驳回/稍后处理/已转任务包`，并提供接受、稍后、驳回、转任务包按钮；`stores/workMemory.ts` 会在局部报告状态里即时回写决策并保持 inline feedback。
158. 工作记忆经验发现决策测试证据：新增 `TestExperienceDecisionPersistsAndDecoratesReports` 和 `TestExperienceDecisionRejectsUnknownStatus`，覆盖决策写入、报告回填、重载后保留、任务包状态和未知状态拒绝。
159. 工作记忆经验发现决策验收证据：`go test ./internal/workmemory -run "Experience|DiscoverExperiences|Drafts" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 均通过；bindings 为 `457 Packages, 17 Services, 124 Methods, 5 Enums, 101 Models, 0 Events`，exe 为 `30058496` bytes、`2026-06-14 10:46:47`。
160. 旧历史数据迁移代码证据：新增 `internal/migration` Wails service，`main.go` 注册该服务；`clipboardhistory`、`capturehistory`、`workmemory` 分别新增 legacy import 方法，迁移时复制旧图片文件到 Ariadne 数据目录。
161. 旧历史数据迁移前端证据：新增 `frontend/src/services/migrationApi.ts`、`LegacyDataStatus`/`LegacyImportResult` 类型、Pinia settings store 状态和设置中心 `旧历史数据` 状态/刷新/迁移 UI；配置导入和历史迁移动作保持分离。
162. 旧历史数据迁移测试证据：新增 `TestServiceImportsLegacyHistoryData` 和 `TestServiceImportIgnoresMissingLegacySources`，覆盖旧剪贴板文本/图片、截图图片、工作记忆图片、dry-run 不变更、导入路径复制、重复导入去重和部分旧历史文件缺失时继续导入已存在来源。
163. 旧历史数据迁移验收证据：`go test ./internal/migration ./internal/clipboardhistory ./internal/capturehistory ./internal/workmemory -run "Legacy|Import" -v`、`go test ./...`、`pnpm build`、`wails3 generate bindings` 和 `wails3 task windows:build` 均通过；bindings 为 `458 Packages, 18 Services, 129 Methods, 5 Enums, 106 Models, 0 Events`，exe 为 `30134784` bytes、`2026-06-14 11:08:37`。Computer Use 只读验证设置中心真实可见 `旧历史数据`、`刷新历史`、`迁移旧历史`，未点击迁移按钮。

## 1. 当前结论

Ariadne 已从文档方案推进到可构建的 Wails 3 + Vue 3 工程骨架，并完成了以下可验证进展：

1. Wails 3 应用可构建为 Windows 可执行文件。
2. Vue 3 + Vite + TypeScript + Pinia + Tailwind CSS + Reka UI 前端可构建。
3. Graphite Teal 视觉 token 已落到 `frontend/src/style.css`，默认是浅色；黑色只保留为 `.dark` 深色模式 token。
4. 主搜索窗口已有 launcher/palette 形态：默认折叠为单搜索框，输入后展开结果列表、右侧预览、前两个动作、更多菜单、键盘导航和内联反馈；工具中心从搜索结果启动为独立窗口，不再把同一个主窗口膨胀成后台面板。
5. Go 后端已建立 search、plugins、settings、platform、workmemory 服务边界。
6. 插件 manifest 已覆盖现有主要内置插件。
7. 计算器、时间戳、Base64、Hash、JSON、URL、UUID、二维码、系统命令、Hosts、剪贴板、截图历史、工作流、工作记忆已经进入 Ariadne 插件协议。
8. 常用文本类插件已有 Go 主路径执行逻辑。
9. `PreviewAction` 已作为显式协议存在，后端测试覆盖非文件结果不得继承文件动作。
10. 工作记忆服务已有本地时间线、搜索、手动补记、手动笔记、删除/清理、可读 ZIP 导出、日报草稿、独立问题复盘草稿、知识草稿、外部代理任务包接口和本地定期草稿调度。
11. 主搜索窗口已通过 Computer Use 在真实 Ariadne 桌面窗口中验证基础渲染、插件搜索和复制反馈。
12. 工作记忆中心 UI 已接入真实桌面壳，时间线、详情、手动补记、日报草稿、知识草稿和外部代理任务包已通过 Computer Use 验证；复盘草稿按钮/面板已完成 Computer Use 只读可见性验证。
13. 设置中心 UI 已接入桌面壳，覆盖旧配置导入、应用与截图、快捷键、工作记忆采集、排除规则、AI/embedding/外部代理和数据保留。
14. settings Go 服务已支持结构化设置、保守默认值、旧版配置安全导入、旧版敏感凭据安全迁移、持久化、存储状态和写后读回校验。
15. platform Go 服务已支持运行时诊断、轻量指标和诚实 capability 矩阵；设置中心已展示平台诊断摘要。
16. Everything 文件搜索已接入 Go provider，能通过 Everything SDK 查询并返回文件型结果动作面；真实桌面 UI 检索还需复验。
17. 自定义启动项已接入 Go provider，配置持久化到 `launchers.json`，搜索结果显式声明打开、复制目标、加入记忆和收藏动作，并已在设置中心提供可视化管理区。
18. 搜索使用状态已接入 `search_state.json`，支持 `RecordUse`、`SetFavorite`、`UsageStatus`，并在排序时提升收藏和最近使用结果。
19. 文本剪贴板历史已接入 Go provider 和 Vue 中心 UI，支持手动收集当前剪贴板、持久化、搜索、置顶、删除、清空未置顶和从主搜索复用。
20. 截图历史已接入 Go provider 和 Vue 中心 UI，支持手动捕获当前屏幕、PNG 持久化、预览、搜索、置顶、删除、清空未置顶和从主搜索复用。
21. Hosts 管理已接入 Go service 和 Vue 中心 UI，支持旧配置迁移、方案编辑、启用开关、远程拉取、冲突检测、应用前预览、Ariadne marker 合并和系统写入二次确认。
22. 工作流宏已接入 Go service 和 Vue 中心 UI，支持旧配置迁移、持久化、搜索入口、变量渲染、命令链执行、步骤回传和最终结果复制。
23. JSON 对比已接入 Go service 和 Vue 中心 UI，支持左右 JSON 编辑、语义差异、规范化输出、报告、行 diff、格式化、交换和复制报告入口。
24. 桌面壳已接入 Wails SingleInstance、SystemTray、Windows Alt+Q 全局热键、关闭隐藏到托盘和设置驱动的 Wails Autostart 钩子；干净环境下已通过 Computer Use + Win32 尺寸验证。
25. 网络监控已接入 Go service 和 Vue 中心 UI，支持 Windows 网卡计数、上传/下载速率、累计流量、网卡列表、中心窗口、`net mini` 贴边小窗搜索入口和托盘入口。
26. 启动器命名已回收：功能中心返回按钮显示 `启动器`，不再把 Alt+Q 首屏称为 `命令中心`。
27. 前端主题同步已接入，默认是浅色；黑色只作为深色模式，而不是默认 UI。
28. 贴图窗口已接入 Go service 和 Vue 独立窗口，支持截图、区域截图、剪贴板图片和二维码文本创建置顶无边框贴图；区域截图贴图会贴近选区原生位置打开，贴图窗口拖动已通过 Computer Use 真实桌面验证。
29. 本地 OCR 已接入 Go service、RapidOCR bridge 和 Vue 入口，支持当前屏幕、截图历史、剪贴板图片和工作记忆图片识别，工作记忆会写回 OCR 文本用于关键词搜索。
30. OCR 行级文本选择已接入截图历史、剪贴板历史和工作记忆 OCR 结果面板，支持逐行选择、全选、清空、复制选中和复制全文。
31. OCR 图片叠框选择已接入截图历史、剪贴板历史和工作记忆图片预览，图片上的 OCR 框可直接选择对应文本行。
32. 贴图 OCR 联动已接入截图/剪贴板贴图窗口，支持 OCR 按钮、图片叠框行选择、全选/清空和选中/全文复制。
33. 截图高级编辑首版已接入区域截图覆盖层，支持矩形、箭头、马赛克、撤销、清空、另存为外部 PNG，并由 Go 单测覆盖渲染和落盘。
34. 图片 OCR 索引底座已接入 Go service、Wails bindings、主搜索 provider 和启动器 action，支持最近截图/剪贴板图片批量 OCR 索引、非敏感 OCR 搜索和敏感正文屏蔽。
35. 工作记忆本地语义检索已接入主搜索 provider 路径，支持一批中英技术词 alias 和相似度打分；这是早期本地语义检索记录，当前外部 embedding 与 embedded/Milvus 向量存储已另接入。
36. 工作记忆与图片 OCR 索引保留策略已接入设置保存和启动初始化路径；工作记忆按天清理过期非收藏，图片索引按天和源记录存在性清理派生索引。
37. 截图历史和剪贴板历史按天保留策略已接入设置保存和启动初始化路径；置顶条目按 `keepFavoritesForever` 保留，截图 PNG 与剪贴板图片文件会随记录清理。
38. 截图历史和剪贴板图片缩略图分层及旧数据回填已接入；历史中心预览优先读缩略图，原图仍用于打开、复制、OCR、贴图和加入截图历史。
39. 工作记忆时间机器后台 worker、设置 interval 重启、排除应用/窗口标题阻断和跳过状态已接入；工作记忆中心会显示 worker/暂停/跳过状态，临时数据目录隐藏进程烟测已验证自动截图闭环。
40. 工作记忆时间机器空闲/锁屏暂停已接入 Windows runtime 和可注入测试 provider；设置页与工作记忆中心都会显示空闲阈值、锁屏暂停、采集范围和多屏策略。
41. 截图历史和工作记忆时间机器已支持真实范围采集：全部屏幕、主屏幕、前台窗口和按显示器分条；临时 `APPDATA` 真实进程烟测证明自动时间机器会写入截图历史和工作记忆证据。
42. 工作记忆自动 OCR 策略已接入：开启 `workMemory.autoOcr` 后，时间机器生成的截图型工作记忆会自动调用本地 RapidOCR 写回 OCR 文本；临时 `APPDATA` 真实进程烟测已证明 `ocrStatus=done:rapidocr_onnxruntime`。
43. 工作记忆 SQLite FTS 已接入：持久化工作记忆会生成 `work_memory.fts.sqlite`，搜索结果可通过 FTS snippet 暴露命中证据；内存关键词和本地语义检索仍保留为回退，当前外部 embedding 与 embedded/Milvus 向量存储已另接入。
44. 工作记忆本地经验发现首版已接入：可从最近工作记忆中归纳重复问题、自动化机会和知识沉淀缺口，保留 evidence 引用，工作记忆中心可生成报告并把 insight 转为外部代理任务包草稿；当前仍不是外部 AI 经验发现。
45. 工作记忆经验发现处理状态已接入：用户可以接受、驳回、标记稍后或转成任务包，状态可持久化并在重新生成报告时回填。
46. 旧版历史数据迁移首版已接入：设置中心可预览旧 `%APPDATA%\x-tools` 剪贴板历史、截图历史和工作记忆历史，并通过 Go migration 服务复制历史与图片到 Ariadne 数据目录，重复导入会去重；本轮没有点击真实旧历史迁移按钮。
47. 工作记忆候选工作流/检查清单草稿已接入：经验线索可生成工作流步骤草稿和排查清单草稿，保留 evidence 引用，工作记忆中心已展示草稿面板和局部转换动作；候选工作流可经二次确认保存为正式工作流，检查清单草稿可经二次确认保存为正式 `checklists.json` 资产，知识草稿可经二次确认保存为本地 `skills.json` Skill 资产，并可导出为 Codex-compatible skill 包或二次确认安装到 live Codex skill 目录。
48. 搜索性能和 Everything 诊断已接入：搜索服务记录滚动 p95/平均/最近查询耗时，平台诊断和设置中心展示 100ms 目标、样本数、最近搜索结果数与 Everything 最近查询/错误状态；这为后续完整性能验收提供运行态数据入口。
49. 发布回滚检查点首版已接入：设置中心可读取 Ariadne 标准 AppData 与 MSIX virtualized 数据根状态，并通过 Go release 服务创建包含 `manifest.json` 的本地 zip 检查点。
49.1 回滚确认式恢复首版已接入：设置中心可二次确认恢复最近检查点；Go release 服务恢复前先创建 `pre_restore` 检查点，恢复时清理 stale 文件并保留 backups 目录，不恢复到旧版 x-tools 数据目录。
50. 用户级发布包首版已接入：`wails3 task windows:package` 可生成无 Python/PyQt 依赖的 Ariadne release zip，内含安装/卸载 PowerShell 脚本、manifest、品牌图标、旧版并存提示和安装目录回滚说明；临时目录安装/重复安装/卸载脚本烟测和真实默认用户目录安装/卸载烟测均已通过。
50.1 MSIX layout 发布链路已接入：`wails3 task windows:msix` 可生成 unsigned full-trust MSIX layout 和 `msix-manifest.json`；`windows:msix-pack` 已接到 `makeappx.exe`，但当前机器缺 `makeappx.exe`、`signtool.exe` 和签名证书，所以还不能声明已签名 `.msix` 安装/卸载验收通过。
51. 主 launcher 与工具中心边界已重切：`internal/toolwindows` 会为工作记忆、剪贴板、截图历史、Hosts、工作流、JSON 对比、网络监控和设置创建独立 frameless 工具窗口；主窗口保留透明背景、搜索器尺寸和 Alt+Q 输入职责。
52. Windows release exe 已改为 GUI 子系统构建：`Taskfile.yml` 使用 `go build -ldflags="-H windowsgui"`，避免启动时露出控制台窗口；正常桌面启动已验证只显示 Ariadne launcher/tool window。
53. 旧版并存确认式交接已接入：设置中心检测到旧版运行或 Alt+Q 冲突时提供二次确认的 `交接 Alt+Q` 动作；后端先发送 `WM_CLOSE`，可选强制结束，并在旧版退出后重试 Ariadne 全局快捷键注册。

这不代表完整重构已完成。当前还没有替代旧 Python/PyQt 主线，也没有完成签名 `.msix` 安装/卸载验收、Everything 文件结果真实桌面 UI 命中复验和完整窗口体系验收；本轮已修复 Codex 启动上下文中的 Wails `GetCursorPos failed` fatal，恢复最新 Win32 桌面启动复验，补齐 Codex skill live 安装链路，并新增可重复的冷启动/包体/内存/热键注册性能验收工具。网络监控贴边小窗的四角位置持久化、多屏模式和前台全屏窗口自动隐藏代码路径已接入，但仍缺真实桌面点击复验、真实全屏应用隐藏/恢复复验、真实多显示器复验和旧版拖拽细节对齐。贴图窗口右键菜单已接入但仍缺真实桌面点击复验；截图高级编辑、图片 OCR 索引、本地语义检索、保留策略、缩略图分层、经验发现按钮、AI 经验发现二次确认按钮、旧历史迁移按钮、回滚检查点入口、候选工作流保存按钮、候选检查清单保存按钮、知识草稿保存/导出/安装 Skill 按钮、语义索引刷新按钮和发布包安装后 UI 仍缺完整真实桌面交互复验。工作记忆时间机器的 core worker、设置同步、排除规则、空闲暂停、锁屏暂停、范围采集、临时数据目录自动采集烟测、自动 OCR 写回、SQLite FTS、本地经验发现首版、经验发现处理状态、本地日报草稿、AI 日报润色首版、外部 AI 经验发现、外部 embedding + embedded/Milvus 向量存储、独立问题复盘草稿、本地定期草稿调度、旧版历史迁移首版、候选工作流/检查清单草稿、候选工作流正式保存、候选检查清单正式保存、本地 Skill 正式保存、Codex skill 包导出与 live 安装、Ariadne refresh marker 握手、搜索 p95/Everything 诊断、可重复搜索 p95 基准、前端搜索过期响应防护、provider 级搜索取消、回滚检查点首版与确认式恢复、真实用户目录回滚演练、启动器/工具窗口边界、用户级发布包首版、unsigned MSIX layout、release 脚本默认用户目录安装/卸载烟测、旧版并存确认式交接和冷启动/包体/内存/热键注册性能报告已实现并测试；当前冷启动 p95 已过 `800ms` target 但未达 `500ms` ideal。仍缺运行中 Codex 对 refresh marker 的实际热加载验收、Alt+Q 真实键鼠焦点落点、真实交互长时间搜索 p95、真实工作记忆 embedding 刷新按钮点击验收、发布后 UI 回滚按钮点击复验、签名 MSIX 安装验收，以及 Everything SDK 同步 IPC 调用进入 `Everything_QueryW` 后不可被抢占的更深层取消能力。

## 2. 工程状态

### 2.1 新增目录

```text
experiments/ariadne/
  main.go
  Taskfile.yml
  internal/
    apps/
    capturehistory/
    clipboardhistory/
    contracts/
    filesearch/
    hosts/
    jsoncompare/
    launchers/
    ocr/
    platform/
    plugins/
    release/
    search/
    settings/
    shell/
    workmemory/
    workflows/
  frontend/
    src/
    bindings/
```

### 2.2 Wails 3 构建约定

当前 `wails3 build` 会查找平台命名空间任务。Windows 下需要 `windows:build`。因此 `Taskfile.yml` 已提供：

```yaml
tasks:
  build:
    cmds:
      - wails3 task windows:build

  windows:build:
    cmds:
      - powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path 'bin' | Out-Null"
      - wails3 generate bindings
      - powershell -NoProfile -Command "Set-Location frontend; pnpm build"
      - go build -o "bin/ariadne.exe" .
```

参考：

- Wails 3 build customization: https://v3.wails.io/guides/build/customization/
- Wails 3 build system: https://v3.wails.io/concepts/build-system/

## 3. 已实现能力

### 3.1 后端契约

文件：

- `experiments/ariadne/internal/contracts/types.go`
- `experiments/ariadne/internal/contracts/actions.go`

已具备：

1. `SearchResultType`
2. `PreviewActionKind`
3. `PreviewKind`
4. `SearchResult`
5. `PreviewDescriptor`
6. `PreviewAction`
7. `ActionResult`
8. 复制动作工厂 `CopyAction`
9. 插件动作工厂 `PluginAction`
10. 工作记忆动作工厂 `RememberAction`
11. 动作表面校验 `ValidateActionSurface`

关键规则：

1. 每条搜索结果必须显式声明 actions。
2. copy action 必须声明 inline feedback。
3. 非文件结果不得显示“打开文件”或“打开所在文件夹”。
4. 非文件结果不得暴露 `open_parent`。

### 3.2 插件服务

文件：`experiments/ariadne/internal/plugins/service.go`

Manifest 已覆盖：

1. `calculator`
2. `timestamp`
3. `base64`
4. `hash`
5. `json`
6. `json_compare`
7. `url`
8. `uuid`
9. `qr`
10. `system_commands`
11. `hosts`
12. `clipboard`
13. `capture_history`
14. `workflow`
15. `work_memory`

已有 Go 主路径执行：

1. `calc 12*(8+3)` 返回计算结果。
2. `base64 hello` 返回编码结果，并在可解码时返回解码结果。
3. `hash hello` 返回 MD5、SHA1、SHA256。
4. `json {"ok":true}` 返回格式化和压缩结果。
5. `url hello world` 返回旧 Python `quote` 语义的 URL 编码结果：空格为 `%20`，`+` 不按 query-string 空格处理，斜杠保持 safe。
6. `uuid 2` 返回 UUID v4。
7. `qr <text>` 返回二维码预览结果和贴图动作。
8. `sys shutdown` 这类高风险命令标记为 `danger` action。

仍缺：

1. Hosts、JSON 对比、工作流管理、剪贴板历史中心、截图历史中心、区域截图覆盖层、截图高级编辑、网络监控、贴图窗口、本地 OCR、行级 OCR 文本选择和 OCR 图片叠框选择已接入 Ariadne Vue/Wails 路径；放大镜、RGB/HEX 取色复制、已有标注选择/拖动/删除已有历史真实桌面复验证据。2026-06-15 用户反馈当前构建仍有截图内容错位、贴图位置偏离、贴图不可拖动/拖动抖动，本轮把截图请求改为前端提交 `<img>` 内 visual 坐标和实际显示尺寸，由 Go 统一映射并裁剪；贴图初始位置由 Go 从最终 native 选区换算到 Wails DIP，并取消旧 `x-15/y-15` 偏移；贴图拖动改为 Wails 原生 `--wails-draggable: drag`，避免 JS `Window.Position/SetPosition` 异步 IPC 抖动；代码/构建/打包验证已通过；截图内容、贴图位置、贴图拖动和截图文字双击重编辑仍需在可用 Computer Use 捕获/输入或 Win32 BitBlt 可用的桌面补真实复验。
2. 真实系统执行：锁屏、休眠、清空回收站等必须接入确认流程。
3. 真实历史数据：文本/图片剪贴板历史和截图历史已接持久化存储；工作记忆持久化索引仍需继续增强。
4. Python legacy bridge 已有受控过渡入口：`legacy <plugin-keyword> [query]`，仅显式调用旧 Python 插件，不参与普通搜索召回，也不把旧插件 `path` 字段映射为文件动作；长期插件主路径仍是 Go/Wails。

### 3.3 搜索服务

文件：`experiments/ariadne/internal/search/service.go`

已具备：

1. 本地 seed 搜索。
2. 插件 provider 聚合。
3. 工作记忆 provider 聚合。
4. Start Menu 应用 provider 聚合。
5. Everything 文件 provider 聚合。
6. 自定义启动项 provider 聚合。
7. 搜索收藏与最近使用状态排序。
8. 工作流宏 provider 聚合。
9. 结果 ID 去重。
10. provider score 稳定排序。
11. 搜索响应耗时记录。
12. Wails `Search` 入口接收 `context.Context`，前端取消 Wails `CancellablePromise` 时 Go 聚合器会停止后续 provider，并且取消查询不写入搜索性能样本。

### 3.3.1 应用搜索

文件：

- `experiments/ariadne/internal/apps/service.go`
- `experiments/ariadne/internal/apps/service_test.go`

已具备：

1. 扫描用户开始菜单：`%APPDATA%\Microsoft\Windows\Start Menu\Programs`。
2. 扫描系统开始菜单：`%PROGRAMDATA%\Microsoft\Windows\Start Menu\Programs`。
3. 递归识别 `.lnk` 快捷方式。
4. 空查询不返回应用结果，避免主面板被系统应用刷屏。
5. 应用结果类型为 `ResultApp`，preview 明确标注为 Windows 应用快捷方式。
6. 主动作是 `打开应用`，由 Windows Shell 打开 `.lnk`，不解析或改写目标路径。
7. 辅助动作是 `复制快捷方式路径` 和 `加入记忆`。
8. 非文件应用结果不暴露 `打开文件` 或 `打开所在文件夹`。

仍缺：

1. 图标提取。
2. UWP/AppX 深度枚举。
3. 启动失败诊断和用户确认策略。
4. 最近使用和收藏排序已由 search state 统一接管，但应用搜索的真实 UI 复验仍需补充。

### 3.3.2 文件搜索

文件：

- `experiments/ariadne/internal/filesearch/service.go`
- `experiments/ariadne/internal/filesearch/everything_windows.go`
- `experiments/ariadne/internal/filesearch/everything_other.go`
- `experiments/ariadne/internal/filesearch/service_test.go`

已具备：

1. 从工作目录和 exe 目录向上定位 `Everything64.dll`，并保留常见安装目录 fallback。
2. Windows 下通过 Everything SDK 调用 `Everything_SetSearchW`、`Everything_SetRequestFlags`、`Everything_SetMax`、`Everything_QueryW`、`Everything_GetNumResults`、`Everything_GetResultFileNameW` 和 `Everything_GetResultPathW`。
3. 2 个字符以下查询直接跳过，避免短查询刷屏。
4. 文件结果类型为 `ResultFile`，preview 明确标注来源为 `Everything SDK`。
5. 文件结果动作是 `打开`、`打开所在文件夹`、`复制路径` 和 `加入记忆`。
6. SDK 加载或 IPC 查询失败时返回空结果并记录 `LastError()`，不阻断其他 provider。
7. 非 Windows 平台有 stub，明确报告 Everything SDK 仅支持 Windows。
8. `Status()` 暴露 DLL 路径、ready、最近查询、耗时、结果数和错误；设置中心平台诊断可见 Everything 最近状态。
9. `wails3 task windows:search-perf` 会通过真实 search provider 栈记录 Everything 文件结果命中数量；最近报告跨 `设置`、`net`、`workflow`、`cap`、`gateway`、`README.md` 查询记录 `1740` 个 Everything 文件结果。
10. `SearchContext(ctx, query)` 已接入 provider 级取消检查；请求取消时不会调用 Everything，也不会把取消请求写入最近查询诊断。Windows SDK client 会在同步 `Everything_QueryW` 调用前后检查 context。
11. 文件/路径类查询在 Everything 可用但 0 命中、SDK 查询失败或 DLL 缺失时会返回 `file-search-coverage-hint` 诊断结果，并把 `coverageHint` 暴露给平台诊断和设置中心；提示会明确建议检查目标盘/目录是否加入 Everything 索引。
12. 文件命中会读取本地元数据，preview meta 和 payload 暴露类型、大小、修改时间、目录识别和元数据错误；目录结果使用 `folder` icon 和 `目录` tag。

仍缺：

1. 真实桌面 UI 文件结果命中复验和截图记录；CLI 基准已有 Everything 文件结果命中，但 `Everything64.dll` 当前作为覆盖提示探针返回诊断结果，不算文件命中。
2. Everything SDK 同步 IPC 调用一旦进入 `Everything_QueryW` 后仍不能被抢占；当前只能在调用前后和结果处理阶段响应 context 取消。
3. 系统原生文件图标、命中片段和更丰富的排序解释；大小、修改时间和目录识别已接入。
4. 文件搜索的真实收藏/最近使用 UI 复验。

### 3.3.3 自定义启动项

文件：

- `experiments/ariadne/internal/launchers/service.go`
- `experiments/ariadne/internal/launchers/service_test.go`
- `experiments/ariadne/frontend/src/services/launchersApi.ts`
- `experiments/ariadne/frontend/src/stores/settings.ts`
- `experiments/ariadne/frontend/src/components/settings/SettingsCenter.vue`

已具备：

1. 启动项配置持久化到 `%APPDATA%\Ariadne\launchers.json`。
2. 支持应用、文件、文件夹、URL 和命令类启动项。
3. 搜索会匹配名称、关键词和目标路径。
4. 非命令启动项使用 `open` 动作交给 Windows Shell 打开。
5. 命令类启动项使用 `danger` 动作，并在 payload 中标记 `requiresConfirmation=true`。
6. 每条启动项结果显式声明 `打开`、`复制目标`、`加入记忆`，再由搜索服务追加收藏动作。
7. 启动项服务已作为 Wails service 暴露，设置页可直接调用 `List`、`Upsert`、`Remove` 和 `Status`。
8. 设置中心已提供启动项列表、新建、编辑、启用/停用、关键词、标签、保存和二次确认删除 UI。
9. `Status.lastSaveError` 会把写盘失败暴露给前端，设置页以 inline feedback 显示保存/删除失败。
10. 禁用启动项不会进入搜索结果；删除默认启动项会写入 tombstone，重启后不会被默认项合并回来。
11. 命令类启动项首次执行只返回 `requiresConfirmation=true` 的局部确认反馈，后端不启动进程。
12. launcher 只在短期 pending confirmation 命中同一结果/同一命令 action 时发送 `confirmed=true`，换查询或 reset 会清除确认态。
13. 确认后命令经平台 command runner 启动；参数解析保留引号和值内空格，也保留 Windows 路径反斜杠。
14. 无效工作目录、命令启动失败和 runner 错误会带回命令、参数和工作目录上下文，作为 launcher 局部失败反馈。

仍缺：

1. 启动项设置页的真实桌面创建/删除落盘复验。
2. 命令类启动项二次点击确认、成功反馈和失败反馈的真实桌面烟测。

### 3.3.4 收藏与最近使用排序

文件：

- `experiments/ariadne/internal/search/state.go`
- `experiments/ariadne/internal/search/service.go`
- `experiments/ariadne/frontend/src/services/ariadneApi.ts`
- `experiments/ariadne/frontend/src/stores/launcher.ts`

已具备：

1. 本地状态文件 `%APPDATA%\Ariadne\search_state.json`。
2. `RecordUse(resultID)` 记录使用次数和最近使用时间。
3. `SetFavorite(resultID, favorite)` 记录收藏状态。
4. `UsageStatus()` 暴露状态路径、数量和记录列表。
5. 搜索排序会给收藏、近期使用和使用次数加权。
6. 搜索服务会给每条结果追加 `收藏` 或 `取消收藏` 显式 preview action。
7. 前端执行动作成功后调用 `RecordUse`。
8. 前端处理 `favorite/unfavorite` 动作时调用 `SetFavorite`，并使用局部 inline feedback。
9. 搜索服务记录最近 200 次非空查询耗时，`PerformanceStatus()` 暴露 p95、平均、最大值、最近查询和 100ms 目标；设置中心平台诊断可见。
10. `internal/searchbench` / `cmd/searchbench` 提供可重复搜索 p95 基准，默认临时 `APPDATA`、固定查询、多轮采样、分查询 p95、慢样本、Everything 状态和 action surface 校验；最近报告 p95 `9ms` / 目标 `100ms`。
11. 前端 launcher 搜索已接入 superseded 请求取消和 serial 过期响应防护，避免快速输入时旧查询覆盖新结果。
12. 后端 provider 级 `context.Context` 查询取消已接入，Wails 取消会传递到 Go 搜索聚合器；context-aware provider 优先走 `SearchContext`，已覆盖 Everything 文件搜索。
13. 设置中心已接入收藏/最近使用数据清理入口，二次确认后调用 `ClearUsage()` 清空 `search_state.json` 并刷新状态。

仍缺：

1. 真实桌面 UI 点击取消收藏和重新排序复验。
2. 设置中心收藏/最近使用清理按钮真实桌面点击复验。
3. 真实交互长时间样本下的搜索 p95 复验和快速输入桌面验收。

### 3.3.5 剪贴板历史

文件：

- `experiments/ariadne/internal/clipboardhistory/service.go`
- `experiments/ariadne/internal/clipboardhistory/service_test.go`
- `experiments/ariadne/frontend/src/services/clipboardApi.ts`
- `experiments/ariadne/frontend/src/stores/clipboardHistory.ts`
- `experiments/ariadne/frontend/src/components/clipboard/ClipboardCenter.vue`
- `experiments/ariadne/frontend/src/stores/launcher.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`

已具备：

1. 文本剪贴板历史持久化到 `%APPDATA%\Ariadne\clipboard_history.json`。
2. 手动收集当前系统剪贴板文本，使用 Wails runtime `Clipboard.Text()` 读取。
3. Ariadne 内部 copy action 成功后会写入剪贴板历史，作为工作记忆数据源之一。
4. 支持按文本、摘要、类型、来源、标签搜索。
5. 支持 `clip token` / `clipboard token` 前缀搜索。
6. 支持置顶、取消置顶、删除和清空未置顶；清空未置顶保留 pinned 记录。
7. 剪贴板历史结果类型为 `ResultClipboard`，动作显式声明 `复制内容`、`置顶/取消置顶`、`加入记忆`，不继承文件动作。
8. 主搜索聚合剪贴板历史 provider，搜索结果会进入收藏/最近使用排序链路。
9. 前端剪贴板历史中心已接入：搜索、列表、详情、收集当前剪贴板、复制、置顶、二次确认删除、二次确认清空未置顶。
10. 平台诊断 capability 已新增 `clipboard_history`，并明确当前覆盖文本历史、图片历史和图片 OCR。
11. 自动文本剪贴板监听已接入：启动时只建立当前剪贴板基线，后续文本变化写入 `clipboard_history.json`，来源标记为 `clipboard_watcher`。
12. 隐私模式或关闭工作记忆剪贴板来源时会暂停自动监听；剪贴板中心显示 watcher 运行状态和错误原因。
13. 图片剪贴板历史已接入：Windows `CF_DIB/CF_DIBV5` 读取为 PNG、保存到 `clipboard_images`、搜索 `图片` 或尺寸、中心 UI 预览、复制图片回系统剪贴板。
14. 图片条目删除、清空未置顶和 maxEntries 裁剪会同步清理对应 PNG 文件。

仍缺：

1. 剪贴板图片 OCR 的自动触发策略和批量调度；手动 OCR 与图片 OCR 索引入口已接入。
2. 与工作记忆持久化索引的深度合并；当前是独立历史服务，可作为后续工作记忆来源。
3. 真实桌面复验；新图缩略图分层、旧数据缩略图回填与基础按天清理已接入。

### 3.3.6 截图历史

文件：

- `experiments/ariadne/internal/capturehistory/service.go`
- `experiments/ariadne/internal/capturehistory/capture_windows.go`
- `experiments/ariadne/internal/capturehistory/capture_other.go`
- `experiments/ariadne/internal/capturehistory/service_test.go`
- `experiments/ariadne/internal/captureoverlay/service.go`
- `experiments/ariadne/internal/captureoverlay/service_test.go`
- `experiments/ariadne/internal/pinnedimage/service.go`
- `experiments/ariadne/internal/pinnedimage/service_test.go`
- `experiments/ariadne/frontend/src/services/captureApi.ts`
- `experiments/ariadne/frontend/src/services/captureOverlayApi.ts`
- `experiments/ariadne/frontend/src/services/pinnedImageApi.ts`
- `experiments/ariadne/frontend/src/stores/captureHistory.ts`
- `experiments/ariadne/frontend/src/components/capture/CaptureHistoryCenter.vue`
- `experiments/ariadne/frontend/src/components/capture/CaptureOverlayWindow.vue`
- `experiments/ariadne/frontend/src/components/pinned/PinnedImageWindow.vue`
- `experiments/ariadne/frontend/src/stores/launcher.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`

已具备：

1. Windows 下使用 GDI `BitBlt` 捕获虚拟屏幕，编码为 PNG。
2. 截图历史持久化到 `%APPDATA%\Ariadne\capture_history.json`，图片写入 `%APPDATA%\Ariadne\capture_images`。
3. `Status` 暴露 `virtualizedPath`、`virtualizedImageDir`、`virtualizedImageCount`、`virtualizedImageBytes`，用于 Codex/MSIX 启动场景下定位真实写入路径。
4. 支持按尺寸、来源、路径、标签、时间搜索。
5. 支持 `cap 2560x1440` / `capture <query>` / `shot <query>` 前缀搜索。
6. 支持置顶、取消置顶、删除和清空未置顶；删除/清空只移除 Ariadne 自己截图目录内的图片。
7. 截图历史结果类型为 `ResultCapture`，文件型动作显式声明 `打开`、`打开所在文件夹`、`复制路径`、`创建贴图`、`置顶/取消置顶`、`加入记忆`。
8. 主搜索聚合截图历史 provider，搜索结果会进入收藏/最近使用排序链路。
9. 前端截图历史中心已接入：搜索、列表、详情、PNG 预览、捕获当前屏幕、打开、打开所在文件夹、复制路径、置顶、二次确认删除、二次确认清空未置顶。
10. 平台诊断 capability 已新增 `capture_history`，并明确覆盖 Windows GDI 截屏、PNG 持久化、搜索、置顶和中心 UI。
11. 贴图窗口已接入：`pinnedimage` 服务会为截图历史、剪贴板图片和二维码文本创建独立置顶无边框 Wails window；前端贴图窗口支持透明背景、拖动、滚轮缩放、双击关闭、复制来源、右键菜单、阴影开关、关闭，以及截图/剪贴板贴图的 OCR 叠框选择。
12. 区域截图覆盖层已接入截图高级编辑首版：选区内可画矩形、箭头和马赛克，支持撤销、清空、保存、另存为外部 PNG、贴图和二维码识别；后端将结构化 `AnnotationOperation` 渲染进 PNG，再写入截图历史。

仍缺：

1. 截图高级编辑的真实桌面交互复验；代码和单测已覆盖矩形、箭头、马赛克、另存 PNG。
2. 贴图窗口右键菜单和更多快捷操作已接入，仍需真实桌面点击复验。
3. OCR 行级文本选择、图片叠框选择、贴图 OCR 联动、图片 OCR 索引底座、工作记忆本地语义检索、外部 embedding + embedded/Milvus 向量存储、外部 AI 经验发现、工作记忆/图片索引/剪贴板/截图历史保留策略、缩略图分层和旧数据回填已接入；待做项是真实桌面复验、外部 AI 经验发现二次确认按钮复验和发布迁移。
4. 后台自动截图 worker、排除规则、空闲暂停、锁屏暂停、多显示器/活动窗口真实裁剪执行链路、临时数据目录真实启用烟测和自动 OCR 写回已接入。
5. 与工作记忆持久化索引和保留策略的深度合并。

### 3.3.7 Hosts 管理

文件：

- `experiments/ariadne/internal/hosts/service.go`
- `experiments/ariadne/internal/hosts/service_test.go`
- `experiments/ariadne/frontend/src/services/hostsApi.ts`
- `experiments/ariadne/frontend/src/stores/hosts.ts`
- `experiments/ariadne/frontend/src/components/hosts/HostsCenter.vue`
- `experiments/ariadne/frontend/src/stores/launcher.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`

已具备：

1. Hosts 方案持久化到 `%APPDATA%\Ariadne\hosts_profiles.json`。
2. 首次加载时可从旧版 `~\.x-tools\hosts_profiles.json` 迁移 local/remote profiles。
3. 系统 Hosts 作为只读 profile 展示，不允许删除或改写为普通方案。
4. 支持本地方案、远程方案、启用/停用、保存、删除和远程 http/https 拉取。
5. 远程拉取有 10 秒超时和 2 MB body 限制，失败信息通过 `lastRemoteError` 暴露。
6. 应用前预览会剥离 Ariadne marker 和旧 X-TOOLS marker，再合并所有启用方案。
7. 预览会统计最终行数、新增/移除行数，并检测同一 hostname 被多个 IP 映射的冲突。
8. 写入系统 Hosts 必须二次确认；未确认时只返回预览，不改系统文件。
9. Windows 写入通过临时文件和 PowerShell `RunAs` 触发 UAC，不在后台静默提升权限。
10. `Status` 暴露 hosts 路径、配置路径、legacy 路径、MSIX virtualized path、系统 hosts 可读状态和最近错误。
11. 前端 Hosts 管理中心已接入：方案列表、只读系统 Hosts、编辑区、启用开关、远程 URL、生成预览、应用到系统、写入边界说明和 inline feedback。
12. 主搜索 `hosts` 插件结果会路由到 Hosts 管理中心；固定顶部工具按钮已移除，保持 Alt+Q 入口是纯搜索器。
13. 平台诊断 capability 已新增 `hosts`，说明写入系统 Hosts 需要用户确认和 UAC。

仍缺：

1. 真实系统 Hosts 写入烟测；当前只验证到预览和二次确认前，不触发 UAC。
2. 远程 Hosts UI 拉取的真实网络复验。
3. 大型 Hosts 文件的前端性能和长文本滚动验收。
4. 冲突列表的真实 UI 构造复验。
5. 与设置中心的数据清理策略和恢复演练整合。

### 3.3.8 工作流宏

文件：

- `experiments/ariadne/internal/workflows/service.go`
- `experiments/ariadne/internal/workflows/service_test.go`
- `experiments/ariadne/frontend/src/services/workflowApi.ts`
- `experiments/ariadne/frontend/src/stores/workflows.ts`
- `experiments/ariadne/frontend/src/components/workflows/WorkflowCenter.vue`
- `experiments/ariadne/frontend/src/stores/launcher.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`

已具备：

1. 工作流模型迁移旧版 `id/name/description/steps(command,pick)`。
2. 默认宏覆盖 `clip-md5`、`clip-url-encode`、`clip-base64-encode` 和 `now-timestamp`。
3. 首次加载时可从旧版 `%APPDATA%\x-tools\config.json` 的 `workflows` 导入。
4. Ariadne 持久化到 `%APPDATA%\Ariadne\workflows.json`，支持 tombstone 删除默认宏。
5. 支持 `wf`、`workflow`、`flow`、`macro`、`工作流`、`宏` 查询前缀。
6. 工作流搜索结果类型为 `ResultWorkflow`，显式声明 `运行工作流`、`编辑步骤` 和 `复制命令` actions。
7. 工作流运行支持 `{clipboard}`、`{input}`、`{prev}` 变量；未知变量会失败并显示具体变量名。
8. 命令链执行复用 Go 插件服务 `Execute(keyword, query)`，不重复实现文本插件逻辑。
9. 禁止递归调用工作流，避免宏套宏造成不可控执行。
10. 每一步运行结果会记录渲染后的命令、命中标题、输出和失败原因。
11. 前端工作流中心已接入：列表、编辑、步骤增删、运行输入、变量说明、运行结果、旧配置路径和 inline feedback。
12. 启动器 `run_workflow` action 已接入 workflow API，成功后把最终结果写回剪贴板并写入文本剪贴板历史。
13. 平台诊断 capability 已新增 `workflow_macros`。

仍缺：

1. 启动器 `wf ...` 真实输入后的结果区桌面复验；当前 UIA 输入链路不稳定，Go 搜索聚合测试已覆盖。
2. 工作流中心真实编辑保存/删除临时宏并清理的桌面复验。
3. 命令类、高风险动作和系统命令步骤的确认策略细化。
4. 工作流与工作记忆经验发现已有 review-only 候选工作流草稿、检查清单草稿和外部代理任务包草稿；候选工作流、检查清单、本地 Skill 保存、Codex-compatible skill 包导出、live Codex skill 目录安装和 Ariadne refresh marker 握手都已具备用户二次确认链路，仍缺运行中 Codex 实际热加载 newly installed skill 的验收或任务包体系的完整链路。
5. 设置中心中的工作流数据清理、导入导出和备份/回滚入口。

### 3.3.9 JSON 对比

文件：

- `experiments/ariadne/internal/jsoncompare/service.go`
- `experiments/ariadne/internal/jsoncompare/service_test.go`
- `experiments/ariadne/frontend/src/services/jsonCompareApi.ts`
- `experiments/ariadne/frontend/src/stores/jsonCompare.ts`
- `experiments/ariadne/frontend/src/components/jsoncompare/JsonCompareCenter.vue`
- `experiments/ariadne/frontend/src/stores/appShell.ts`
- `experiments/ariadne/frontend/src/stores/launcher.ts`

已具备：

1. Go service 暴露 `Compare` 和 `Format`，Wails bindings 已生成。
2. 对象字段顺序默认不算语义差异。
3. 支持新增、删除、变更三类语义差异，并输出旧版兼容的中文 summary/report。
4. 解析失败会显示左右侧标签和行列位置。
5. 支持规范化格式输出和简化 unified diff。
6. Vue 中心页支持左右 JSON 编辑、对比、格式化两侧、剪贴板到左/右、文件到左/右、拖放到左/右、交换、复制报告、示例和清空。
7. 主搜索 `jsondiff` 命令结果与插件 `open_json_compare` 路由都会打开 JSON 对比中心；固定顶部 `JSON` 按钮已移除。
8. 平台诊断 capability 已新增 `json_compare`。
9. 大体积 JSON 对比增加性能预算：行级 diff 超过预算时跳过 O(n*m) LCS，但继续返回语义差异统计和报告；超长格式化预览会截断并在 UI 显示性能提示。

仍缺：

1. 大体积 JSON 的真实桌面滚动体验、文件选择导入点击和拖放导入还未做压力/桌面验收；diff 性能预算已有 Go 测试和前端构建覆盖。
2. `复制报告` 本轮未做桌面点击验收，避免覆盖用户真实剪贴板；复制链路由 Wails Clipboard API 和前端构建覆盖。

### 3.3.10 网络监控

文件：

- `experiments/ariadne/internal/networkmonitor/service.go`
- `experiments/ariadne/internal/toolwindows/service.go`
- `experiments/ariadne/internal/toolwindows/fullscreen_windows.go`
- `experiments/ariadne/internal/toolwindows/fullscreen_other.go`
- `experiments/ariadne/internal/plugins/service.go`
- `experiments/ariadne/internal/shell/manager.go`
- `experiments/ariadne/frontend/src/components/network/NetworkMonitorCenter.vue`
- `experiments/ariadne/frontend/src/components/network/NetworkMiniWindow.vue`
- `experiments/ariadne/frontend/src/stores/networkMonitor.ts`
- `experiments/ariadne/frontend/src/services/toolWindowsApi.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`
- `experiments/ariadne/frontend/src/stores/launcher.ts`

已具备：

1. Windows 使用 IP Helper API 读取启用网卡累计收发字节，Go service 做速率差分。
2. 网络监控中心显示下载/上传速率、累计流量、网卡数量和刷新状态。
3. 启动器 `net` / `network` / `网速` 打开 `network-monitor` 独立工具窗口，不污染 Alt+Q 主 launcher。
4. `net mini` / `net 小窗` / `net 贴边` 返回 `network-mini` 结果，主动作 `open_network_mini` 打开贴边小窗。
5. 网络监控中心顶部 `小窗` 图标按钮可打开 `network-mini`，托盘菜单也有 `网速小窗`。
6. `network-mini` 当前固定 `156 x 40`，禁用 resize，默认按任务栏左侧区域定位并在 Windows 下设置 taskbar owner；旧四角贴边 anchor 仍作为兼容路径保留。
7. 小窗显示速率、主网卡、刷新/异常/暂停，并提供四角贴边切换和全屏自动隐藏开关；anchor 和开关持久化到 `network_mini_window.json`。
8. Windows 下 watcher 会检测前台窗口是否全屏，并在开启自动隐藏时隐藏/恢复 `network-mini`。

仍缺：

1. 真实桌面点击复验：从启动器、中心页按钮和托盘打开小窗，并验证置顶层级、尺寸和关闭行为。
2. 真实全屏应用隐藏/恢复复验；当前只声明 Win32 探测代码路径和 Go 单测覆盖。
3. 多屏 follow、屏幕 work area 变化响应和显式选择屏幕。
4. 旧版拖拽/锁定细节对齐；当前小窗采用四角 anchor 切换，不提供自由拖拽后保存。

### 3.4 工作记忆服务

文件：`experiments/ariadne/internal/workmemory/service.go`

已具备：

1. `Status`
2. `Timeline`
3. `Search`
4. `SetTimeMachineEnabled`
5. `SetPrivacyMode`
6. `CaptureCurrentScreen`
7. `GenerateDailyDraft`
8. `GenerateRetrospectiveDraft`
9. `GenerateKnowledgeDraft`
10. `GenerateAgentTaskPackage`
11. `ApplyDraftSchedule`
12. `ScheduledDraftStatus`
13. `RunScheduledDraftsNow`
14. `ApplyDraftPolishPolicy`
15. `PolishDraft`

关键约束：

1. 隐私模式开启时，屏幕时间机器不能恢复。
2. 隐私模式开启时，手动补记会被阻止。
3. 日报、复盘草稿、知识草稿和外部代理任务包保留 evidence IDs。
4. 外部代理任务包必须 `RequiresReview=true`。
5. 本地定期草稿调度只使用非敏感工作记忆 evidence；隐私模式或工作记忆禁用会暂停调度。
6. AI 草稿润色只在用户二次确认后调用外部 provider；API key 优先读环境变量，缺省时读 Windows Credential Manager，不进入 Ariadne 配置文件或导出包。

仍缺：

1. 按显示器分条采集后的多条工作记忆归并/展示策略；截图历史已能按显示器分条写入多条截图记录。
2. 外部 AI 经验发现真实桌面二次确认流程验收；代码路径、确认门禁、隐私阻断和 Go/前端构建验证已接入。
3. 真实 Windows 锁屏/解锁场景下的人工或安全自动化复验；Go 注入测试已覆盖锁屏暂停逻辑。
4. embedding / Milvus 真实工作记忆刷新按钮点击验收；embedded 与 Milvus REST 代码路径和本机临时 collection 探测已完成。
5. 更完整的敏感内容识别；URL 级排除规则已补齐到采集、导入、OCR 和导出链路。

### 3.5 前端主搜索窗口

文件：

- `experiments/ariadne/frontend/src/components/launcher/AriadneLauncher.vue`
- `experiments/ariadne/frontend/src/stores/launcher.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`
- `experiments/ariadne/frontend/src/services/toolWindowsApi.ts`
- `experiments/ariadne/frontend/src/style.css`
- `experiments/ariadne/internal/toolwindows/service.go`

已具备：

1. Alt+Q 入口默认是无原生标题栏的折叠搜索框，不显示桌面首页式工具按钮。
2. 默认浅色 Graphite Teal，黑色只作为 `.dark` 深色模式 token。
3. 空查询返回空结果，不展示 seed 列表。
4. 输入后展开结果列表、右侧预览和当前动作区。
5. 结果类型图标。
6. 前两个动作直接展示。
7. 额外动作进入 `更多` 菜单。
8. 复制结果使用按钮附近内联反馈。
9. 上下键导航。
10. Enter 执行主动作。
11. Graphite Teal 视觉 token。
12. copy action 优先使用 Wails runtime `Clipboard.SetText`，浏览器开发态 fallback 到 `navigator.clipboard`。
13. 剪贴板写入失败不会静默中断局部反馈链路。
14. `open_tool` 动作会调用 `toolwindows` 服务打开剪贴板历史、截图历史、Hosts 管理、工作流宏、JSON 对比、网络监控中心、网络监控小窗、设置或工作记忆的独立 frameless 工具窗口；主 launcher 会隐藏而不是变成后台大窗口。
15. Launcher 会根据折叠/展开状态调用 Wails `Window.SetSize`，折叠态为 `760 x 96`，搜索展开态为 `860 x 468` 的结果+预览 palette；工具页尺寸由独立工具窗口服务管理。
16. copy action 成功后会记录到 Ariadne 文本剪贴板历史。
17. 设置项到 DOM `.dark`/light class 的主题切换联动已接入，旧实验 dark/system 配置会迁移回默认 light；当前 UI 只保留 light/dark 两项。
18. 主 launcher 文档背景为透明，Wails 主窗口使用 `BackgroundTypeTransparent` 和 `DisableFramelessWindowDecorations`，降低原生窗口边框/标题栏残留风险。
19. 快速输入时会取消上一条 Wails `Search` CancellablePromise，并用 `searchRequestSerial` 阻止旧响应覆盖当前查询结果、选中项和耗时；开发态 fallback 仍保留。

仍缺：

1. 多屏定位、失焦隐藏和焦点恢复仍需继续做更完整的真实 Wails 验收；独立工具窗口和 Alt+Q 回到折叠启动器已在本轮通过 Win32/截图复验。
2. 多尺寸和长文本渲染验收。

### 3.6 前端工作记忆中心

文件：

- `experiments/ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue`
- `experiments/ariadne/frontend/src/stores/workMemory.ts`
- `experiments/ariadne/frontend/src/services/workMemoryApi.ts`
- `experiments/ariadne/frontend/src/stores/appShell.ts`
- `experiments/ariadne/frontend/src/style.css`

已具备：

1. 启动器和工作记忆中心之间的视图切换。
2. 工作记忆状态栏：时间机器状态、隐私状态、暂停原因。
3. 搜索栏：可按 OCR、剪贴板、截图、窗口标题、标签和证据文本过滤。
4. 时间线列表：展示来源、应用、时间、收藏和敏感标记。
5. 详情区：展示摘要、捕获上下文、原文、来源、应用、时间和标签。
6. 手动补记：调用后端 `CaptureCurrentScreen`，并在局部 UI 显示 `已补记当前屏幕`。
7. 日报草稿：调用后端 `GenerateDailyDraft`，展示 `今日工作日报草稿` 和局部反馈。
8. 复盘草稿：调用后端 `GenerateRetrospectiveDraft`，基于当前复盘证据组或当前详情记忆生成独立问题复盘草稿，右侧 `复盘` 面板展示正文和证据 ID；时间线支持多选复盘证据，`选择筛选` 只纳入非敏感记忆，敏感记忆不能加入复盘证据。
9. 知识草稿：调用后端 `GenerateKnowledgeDraft`，展示 `知识条目草稿` 和局部反馈。
10. 外部代理任务包：调用后端 `GenerateAgentTaskPackage`，展示 `Requires review` 和局部反馈。
11. Wails binding 调用失败时有本地 fallback 和局部失败反馈，开发态不会空白。
12. 工作记忆详情动作区已移到标题下方，避免被详情内容滚动挤到不可见区域。
13. 手动笔记：右侧面板可录入标题、正文、标签、收藏和敏感标记，并调用 `AddNote` 写入工作记忆。
14. 数据操作：详情区可二次确认删除当前记忆；右侧数据包面板可导出可读 ZIP，清理未收藏记忆也采用二次点击确认。
15. 材料导入：右侧数据包面板可粘贴显式文件路径导入 Markdown/文本、图片、PDF、Office 文档和 Ariadne 工作记忆导出 zip，支持附加标签、收藏和敏感标记；图片证据进入 Ariadne 自有 `work_memory_images` 目录，docx/xlsx/pptx 和可解析 PDF 正文会进入本地搜索。
16. 筛选导出：右侧数据包面板可按最近天数、标签和条目 ID 导出工作记忆；导出结果显示敏感跳过、排除跳过和筛选外数量，不填筛选条件时仍走原全量导出。
17. 排除规则配置：右侧面板可编辑排除应用、窗口关键词、路径片段、URL 域名/路径和内容正则；保存后走 settings runtime 更新链路，并继续由后端 `CapturePolicy` 统一执行。
18. 时间机器回放：右侧面板可按时间顺序回看截图型工作记忆，定位最近、上一帧和下一帧会同步详情区；无截图帧时显示空状态并禁用回放按钮。
19. 定期草稿：右侧面板显示定期草稿开关状态、间隔、最近运行、最近错误和最近生成的日报/复盘/经验发现摘要，并提供 `立即运行` 手动触发；设置中心可配置开关、间隔和三类草稿类型。
20. AI 日报润色：日报面板提供 `AI 润色` 和 `确认外发润色` 两步按钮，第一次只展示风险和确认要求，第二次才调用外部 provider；润色成功后用返回草稿替换当前日报草稿并保留 evidence。
21. AI 经验发现：经验发现面板保留本地 `发现经验`，另提供 `AI 发现` 和 `确认外发发现` 两步按钮，第一次展示 provider/model 和外发风险，第二次才调用外部 provider；外部失败时显示本地规则报告降级。

仍缺：

1. 复盘和知识草稿的外部 AI 摘要/润色接入；本地日报、AI 日报润色首版、外部 AI 经验发现、独立问题复盘和知识结构化草稿已接入。
2. 工作记忆材料导入、筛选导出、排除规则配置、有截图帧时间机器回放和复盘草稿生成的真实桌面材料路径/规则粘贴、导出按钮点击、保存按钮点击、回放按钮点击、复盘按钮点击和错误反馈复验；排除规则面板、无截图帧回放空状态和复盘按钮/面板只读可见性已通过 Computer Use，复盘多选与生成、定期草稿面板和手动运行已通过临时 `APPDATA` Win32 截图烟测。
3. 持久化数据源与 embedding 检索；SQLite FTS 已作为本地可重建索引接入。

### 3.7 设置中心

文件：

- `experiments/ariadne/internal/settings/service.go`
- `experiments/ariadne/internal/settings/service_test.go`
- `experiments/ariadne/frontend/src/components/settings/SettingsCenter.vue`
- `experiments/ariadne/frontend/src/stores/settings.ts`
- `experiments/ariadne/frontend/src/services/settingsApi.ts`
- `experiments/ariadne/frontend/src/services/launchersApi.ts`
- `experiments/ariadne/frontend/src/types/ariadne.ts`

已具备：

1. 结构化设置模型：应用、快捷键、截图、工作记忆、AI、插件。
2. 保守默认值：主题默认 `light`，时间机器默认关闭、AI/embedding 默认关闭、敏感导出默认关闭、远程桌面和密码类窗口默认排除。
3. 设置归一化：主题、截图质量、采集间隔、捕获范围、多显示器策略、保留天数、排除列表、trace mode。
4. 旧版 x-tools 配置状态：读取 `%APPDATA%\x-tools\config.json` 是否存在和可导入 key。
5. 旧版配置安全导入：导入用户偏好和安全边界；旧版明文 AI/embedding/Milvus 密钥如存在，只迁入 Windows Credential Manager，不写入 Ariadne JSON。
6. 设置持久化：默认写入 `%APPDATA%\Ariadne\config.json`。
7. 存储状态：暴露配置路径、目录状态、文件大小、目录 entries、`APPDATA`、`LOCALAPPDATA`、`UserConfigDir`、工作目录和可执行文件路径。
8. 写后读回校验：保存后立即读取并反序列化 JSON，`StorageStatus.readBackOk` 作为 UI 保存成功的条件之一。
9. MSIX/AppData virtualization 诊断：当 Ariadne 由 Codex MSIX 环境启动并写入被重定向到 `LocalCache\Roaming` 时，`StorageStatus` 会暴露实际路径和字节数。
10. 前端设置页：配置存储、旧版导入、应用与截图、快捷键、自定义启动项、工作记忆采集、排除与敏感内容、AI/embedding/外部代理、数据保留。
11. 保存、恢复默认、导入旧配置均使用局部 inline feedback。
12. 启动项保存/删除失败会显示后端 `lastSaveError` 摘要，不吞掉写盘错误。

当前复验结论：

1. Wails bindings 已重新生成，最新为 `404 Packages / 11 Services / 63 Methods / 51 Models`。
2. Ariadne exe 已重建。
3. Computer Use 已确认设置中心和 `MSIX 实际路径` 可见。
4. PowerShell 已确认 Codex MSIX 虚拟化实际路径存在并在保存后更新时间。
5. 逻辑路径 `C:\Users\luwei\AppData\Roaming\Ariadne\config.json` 在当前 Codex 启动场景下不存在，这是 MSIX virtualization 造成的，不是设置服务未写入。
6. Windows UI Automation 已确认新二进制设置中心存在自定义启动项管理按钮节点；真实创建/删除落盘仍需 Computer Use 或人工输入复验。

### 3.8 平台诊断

文件：

- `experiments/ariadne/internal/platform/service.go`
- `experiments/ariadne/internal/platform/service_test.go`
- `experiments/ariadne/frontend/src/services/platformApi.ts`
- `experiments/ariadne/frontend/src/components/settings/SettingsCenter.vue`

已具备：

1. `PlatformStatus`：应用名、legacy 名称、capabilities、diagnostics、metrics。
2. 运行时诊断：OS、架构、Go 版本、PID、工作目录、exe 路径、exe 字节数、`APPDATA`、`LOCALAPPDATA`、Everything DLL 路径、Go/Wails CLI 路径。
3. 轻量指标：Go heap alloc、Go runtime sys、exe size。
4. 本地日志：标准日志写入 `%APPDATA%\Ariadne\logs\ariadne.log`，`Status()` 暴露日志路径、存在状态、大小、更新时间和最近错误。
5. 诚实 capability 矩阵：
   - 已接入：preview actions、settings、work memory 基础能力、Start Menu app scan、custom launchers、clipboard history、capture history、hosts management、workflow macros、search ranking，以及当前运行态注入后的 single instance、global hotkey、tray、autostart。
   - 条件接入：Everything file search，前提是成功定位 `Everything64.dll` 且 Everything 后台服务可用。
6. 诊断包导出：`ExportDiagnostics()` 生成 `%APPDATA%\Ariadne\diagnostics\ariadne-diagnostics-*.zip`，包含 `README.md`、平台状态、metrics 和当前日志，不打包工作记忆导出、截图图像、剪贴板图片或旧版数据。
7. 旧版交接：`ResolveLegacyConflict()` 需要显式确认，默认温和关闭旧版进程，可选强制结束，并通过 `platform.WithHotkeyRetry(...)` 调用 shell manager 重试 Alt+Q 注册。
8. 设置中心展示平台诊断摘要：系统、Go runtime、exe 大小、PID、能力接入数量、Everything DLL、Wails PATH 状态、日志路径、诊断包导出反馈和旧版交接入口。

当前边界：

1. 该能力是诊断和验收基础；shell 状态来自 `platform.WithShellStatus(...)` 注入，未注入时不会把单例、托盘、全局快捷键误报为完成。
2. Everything DLL 已定位，Go 查询主路径已接入；若 Everything 后台服务不可用，搜索会降级为空结果。
3. 自定义启动项、文本剪贴板历史、截图历史、Hosts 管理、工作流宏和搜索排序已接入代码、bindings 和构建；启动项搜索/收藏、剪贴板中心、剪贴板主搜索、截图中心、截图主搜索、Hosts 中心预览和工作流中心运行已完成真实桌面复验，设置页创建/删除落盘、启动器工作流搜索输入和系统 Hosts 写入仍需复验。
4. 桌面壳单例、托盘、Alt+Q 和关闭隐藏到托盘已完成真实桌面复验；开机启动注册表写入仍需用户打开设置开关后单独烟测。
5. Wails CLI 在 Codex 启动的桌面进程 PATH 中未暴露是预期现象；构建命令仍使用台账 0.1 中记录的便携路径。

## 4. 验证记录

### 4.1 环境

本机现状：

1. Node / npm / pnpm 可用。
2. 当前 PowerShell 会话不一定能直接找到 `go` 和 `wails3`，先使用 0.1 的临时 PATH 注入。
3. Go 使用 `C:\Users\luwei\.codex\tools\go1.26.4.windows-amd64\go\bin\go.exe`。
4. Wails 3 CLI 使用 `C:\Users\luwei\.codex\go-bin\wails3.exe`。
5. Wails 版本：`v3.0.0-alpha.98`。
6. `Everything64.dll` 存在于仓库根目录。
7. WebView2 注册表项存在。

### 4.2 已通过命令

#### 2026-06-14 截图覆盖层释放选框与旧版行为补齐

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
gofmt -w main.go internal\captureoverlay\service.go internal\captureoverlay\service_test.go internal\clipboardhistory\service.go
go test ./internal/captureoverlay ./internal/clipboardhistory
go test ./...
wails3 generate bindings
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

结果：

- `go test ./internal/captureoverlay ./internal/clipboardhistory` 通过。
- `go test ./...` 通过。
- `wails3 generate bindings` 通过。
- `pnpm build` 通过；仍有 `@vueuse/core` pure annotation 和 chunk size 警告，不阻塞构建。
- `wails3 task windows:build` 通过，更新 `experiments\ariadne\bin\ariadne.exe`。

本轮修复：

1. 前端截图覆盖层按 `pointerId` 跟踪选区拖拽和标注拖拽；`pointerup`、`lostpointercapture` 都会释放活动指针，工具栏只在选区完成后显示，修复鼠标松开后仍处于拖拽状态的问题。
2. 补回旧版 x-tools 的关键截图语义：Enter 复制选区到系统剪贴板并结束覆盖层，右键有选区时清空选区、无选区时退出，Q 识别二维码，Ctrl+Z/Ctrl+Y 撤销重做标注。
3. 后端截图覆盖层新增 `copy` 动作，保存截图历史后调用系统剪贴板写图；设置中心截图策略会注入覆盖层运行态，支持自动复制、自动贴图和自动另存。
4. 标注工具已补齐选区缩放手柄、矩形、直线、箭头、画笔、马赛克、文字、序号、橡皮、颜色/粗细调节和撤销/重做；后续又补齐放大镜颜色 HUD、RGB/HEX 取色复制、已有标注二次选中/移动/删除和文字双击重编辑。已有标注移动/删除已经 Computer Use 桌面验证；文字双击重编辑当前为代码和构建验证，尚未单独声明桌面点击验收。

Computer Use 真实桌面复验：

- 启动 `experiments\ariadne\bin\ariadne.exe` 后按 `Alt+A`，打开 `Ariadne - 截图覆盖层`。
- 从 `(400,300)` 拖拽到 `(620,460)` 后，覆盖层进入完成态并显示 `复制`、`保存`、`另存`、`贴图`、`二维码`、`矩形`、`箭头`、`直线`、`画笔`、`马赛克`、`文字`、`序号`、`橡皮`、颜色和粗细控件，证明 `pointerup` 后选框已释放。
- 按 Enter 后覆盖层关闭；虚拟化实际路径 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\capture_history.json` 出现 `overlay/selection/copy` 记录和 PNG 文件。本次验证产生的临时截图记录、截图 PNG 和剪贴板监听 PNG 已清理。

#### 2026-06-15 工作记忆相似画面去重验证

修复：

1. `Entry` 增加 `imageFingerprint` 字段，`entryFromCapture` 在截图型工作记忆生成时从 PNG 计算保守图像指纹。
2. 自动时间机器合并策略从“同一 PNG signature”扩展为“同一 signature 或尺寸兼容、平均亮度接近、64 位指纹汉明距离不超过阈值的相似画面”。
3. 相似画面合并只作用于 `time_machine` 来源；`manual_capture` 仍保留每次用户主动补记，避免吞掉人工证据。

验证：

- `go test ./internal/workmemory -run "TimeMachine.*(Duplicate|Similar|Different)" -v` 通过，覆盖同 signature 合并、相似 fingerprint 合并、明显不同画面不合并和手动截图不合并。
- `go test ./internal/workmemory -v` 通过。
- `go test ./...` 通过。
- `wails3 generate bindings` 通过，bindings 保持 `468 Packages, 23 Services, 179 Methods, 5 Enums, 165 Models, 0 Events`，前端模型已包含 `imageFingerprint`。
- `pnpm build` 通过；仍有既有 Rolldown `@vueuse/core` pure annotation 和 chunk size 警告。
- `wails3 task windows:build` 通过。
- `bin\ariadne.exe` 更新为 `31667200` bytes，时间 `2026-06-15 00:38:45`，SHA256 `1FB592F36345298FC58799AE5BEAFCE0CB6B7A4D6688614C472DAFA7115812E0`。

#### 2026-06-15 MSIX layout 发布链路补齐

目标：

1. 把前序半成品 `internal/msixpack` 从“可编译代码”补成可实际调用、可测试、可记录产物的发布链路。
2. 不把 unsigned layout 伪装成已签名可安装 MSIX；明确 Windows SDK 和签名证书缺口。

修复：

1. 新增 `cmd/msixpack` CLI，支持 `-version`、`-product`、`-package-name`、`-publisher`、`-exe`、`-logo`、`-output`、`-pack` 和 `-makeappx` 参数。
2. `Taskfile.yml` 新增 `windows:msix` 和 `windows:msix-pack`：前者构建 Ariadne 并生成 unsigned MSIX layout，后者显式调用 `makeappx.exe` 打包。
3. `internal/msixpack.Manifest` 新增 `candidateMsixPath` 和 `msixFile`；未执行 pack 时 `packed=false` 且 `msixPath` 为空，避免台账或自动化误以为 `.msix` 已存在。
4. `internal/msixpack.Build()` 改为 pack 成功后再写最终 `msix-manifest.json`；pack 成功时记录 `.msix` 文件 metadata。
5. 新增 `internal/msixpack/package_test.go`，覆盖 unsigned layout 内容、AppxManifest full-trust/runFullTrust 关键字段、版本归一化、缺失 exe 失败、缺失 makeappx 失败和 Taskfile 入口。

产物：

- MSIX layout：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0-msix`
- manifest：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0-msix\msix-manifest.json`
- Appx manifest：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0-msix\AppxManifest.xml`
- candidate package：`P:\workspace\glwlg\app\x-tools\experiments\ariadne\dist\msix\ariadne-0-0-0-0.msix`
- layout 包含：`Ariadne.exe`、`AppxManifest.xml`、`README-msix.txt`、`msix-manifest.json`、`Assets\Square44x44Logo.png`、`Assets\Square150x150Logo.png`、`Assets\StoreLogo.png`
- 当前 `msix-manifest.json` 记录 `packed=false`，因为本机没有 `makeappx.exe`，也没有 `signtool.exe`。
- 最新同步后的 layout 内 `Ariadne.exe` 为 `31762944` bytes，时间 `2026-06-15 08:56:41`，SHA256 `3EEC2B29CB4E6D566F78A3F853AC13C46A2747BD6ECF8BA726DE39E9E06707A8`；当前 `bin\ariadne.exe` 为 `31762944` bytes，时间 `2026-06-15 08:56:41`，SHA256 相同。

验证：

- `go test ./internal/msixpack -v` 通过。
- `go run ./cmd/msixpack -version dev` 通过并生成 unsigned layout。
- `wails3 task windows:msix` 通过；该任务执行 bindings 生成、`pnpm build`、Windows resources、`go build -ldflags="-H windowsgui"` 和 `cmd/msixpack`。
- `go test ./...` 通过。
- 本机探测：`makeappx.exe` 不存在，`signtool.exe` 不存在，`C:\Program Files (x86)\Windows Kits\10\bin` 不存在。

本轮未完成：

- 没有生成已签名 `.msix`，也没有执行 `Add-AppxPackage` 安装/卸载验收；阻断条件是当前机器缺 Windows SDK `makeappx.exe`、`signtool.exe` 和与 `Publisher=CN=Ariadne` 匹配的签名证书。
- 在这些外部条件具备前，用户级 release zip 仍是可安装发布包，MSIX layout 是发布迁移链路的待签名候选产物。

#### 2026-06-15 贴图窗口右键菜单补齐

目标：

1. 补齐贴图窗口旧版细节里的右键菜单与更多快捷操作，不再只依赖顶栏小按钮。
2. 继续遵守 Ariadne 的局部反馈原则，不使用系统通知。

修复：

1. `PinnedImageWindow.vue` 新增自绘右键菜单，右键不弹系统菜单；菜单动作包括复制来源、OCR 文字识别、复制选中 OCR、复制 OCR 全文、放大、缩小、原始比例、阴影开关和关闭贴图。
2. 菜单会按当前窗口大小避让边缘；贴图窗口内点击菜单外会先关闭菜单，不会同时触发拖动；Esc 在菜单打开时优先关闭菜单，Shift+F10 或 ContextMenu 键可从键盘打开。
3. 菜单复用现有 `copySource()`、`recognizeOCR()`、`copySelectedOCRText()`、`copyFullOCRText()`、`zoomBy()`、`resetZoom()`、`toggleShadow()` 和 `closeWindow()`，因此复制成功、OCR 失败、选择缺失等反馈仍显示在贴图窗口内。
4. `style.css` 新增 Graphite Teal 浅/深色右键菜单样式，并把菜单加入 `no-drag` 交互区域。
5. `platform` capability 文案补充“右键菜单”，并更新单测防止文案回退。

验证：

- `go test ./internal/platform -v` 通过。
- `pnpm build` 通过；仍有既有 Rolldown `@vueuse/core` pure annotation 和 chunk size 警告。
- `go test ./...` 通过。
- `wails3 task windows:build` 通过，bindings 为 `468 Packages, 23 Services, 179 Methods, 5 Enums, 165 Models, 0 Events`。
- `bin\ariadne.exe` 更新为 `31695360` bytes，时间 `2026-06-15 01:13:14`，SHA256 `4E81139623A20C11AF7B846E00740F9DD1CCDD68D9754A09108A9ADAA816EC4C`。

本轮未完成：

- 当前工具发现未暴露 Computer Use 控制接口；因此本项只声明代码、类型构建、Go 测试和 Wails 构建验证通过，不声明真实贴图窗口右键点击验收。
- 后续有桌面控制能力时，需补一次真实贴图右键菜单验收：右键打开菜单、禁用状态正确、复制/OCR/缩放/阴影/关闭动作可用，菜单不影响贴图拖动。

#### 2026-06-15 截图 visual 坐标协议与贴图窗口直接拖动修复（当前方案）

用户反馈：

1. 当前截图保存内容与实际框选区域无关。
2. 区域截图后的贴图初始位置偏离实际截图位置。
3. 贴图窗口不能拖动。

修复：

1. 本节取代上一版“截图 visual 坐标协议与贴图后端拖动修复”的关键结论：上一版仍让后端根据 WebView surface 尺寸反推裁剪区域，后续容易再次混入物理像素、DIP 和 client size 偏差，不能继续作为当前方案。
2. 前端选区统一存储为 surface/window 坐标，并在拖拽、选区框、工具栏和完成态中都使用这套坐标；坐标点会被 clamp 到 `.capture-overlay-image` 在 surface 中的实际 DOM rect 内。
3. 提交给后端的截图请求改为前端先用实际 `<img>` bounds 计算当前图像内的 visual 坐标和实际显示尺寸，再以 `coordinateSpace=visual` 提交给 Go；Go 端唯一负责映射到 overlay PNG 源像素并裁剪。
4. 保存、复制和标注最终 PNG 使用同一个由 visual 坐标解析出的源像素矩形；左上角用 `floor`、右下角用 `ceil`，避免 Wails DIP/physical 缩放后四舍五入丢掉实际框选边缘。
5. 贴图初始位置不再发送 `pinPositioned/pinX/pinY`；Go 使用最终裁剪出的 native 选区左上角，通过 Wails `PhysicalToDipPoint` 换算到窗口坐标，并且不再追加旧的 `x-15/y-15`“附近”偏移，保证贴图窗口原点和裁剪来源是同一个坐标链路。
6. `PinnedImageWindow.vue` 不再用 Vue pointer capture + 当前窗口运行时 `Window.Position/SetPosition` 模拟拖动；贴图 surface、stage、zoom layer 和图片主体使用 Wails 原生 `--wails-draggable: drag`，菜单、OCR 条、OCR 框和按钮继续 `no-drag`，避免 JS 异步 IPC 造成拖动抖动。
7. `pinnedimage.Service` 保留 `MovePinned` / `SetPinnedPosition` 兼容接口，但贴图窗口当前主拖动路径不再调用它们。
7. 贴图窗口的 stage 使用 image-derived 固定内容尺寸，避免 `OCRImageOverlay` / image wrapper 把贴图内容拉伸到整个透明窗口，看起来像贴图内容和窗口位置都错乱。
8. 前端截图几何计算保留在 `frontend/src/lib/captureGeometry.ts`，轻量 Node + TypeScript 自测脚本覆盖 source-local 映射、image/surface offset、fractional floor/ceil 和 near-selection pin 位置；当前组件只把 visual 坐标交给 Go，source-local 计算只用于前端渲染标注 PNG。

验证：

- `go test ./internal/captureoverlay ./internal/pinnedimage -v` 通过；既有 session/visual coordinate-space 测试覆盖 source-local 裁剪、surface 缩放、actual surface size 优先、visual 选区解析出的 native 贴图位置、选区原点对齐和后端位置同步。
- `go test ./...` 通过。
- `pnpm test:capture-geometry` 通过，覆盖前端 source-local 几何换算。
- `pnpm build` 通过；仍有既有 Rolldown `@vueuse/core` pure annotation 和 chunk size 警告。
- `wails3 task windows:build` 通过，bindings 为 `468 Packages, 23 Services, 181 Methods, 5 Enums, 169 Models, 0 Events`。
- `wails3 task windows:msix` 通过，unsigned MSIX layout 已同步到当前 exe。
- 当前 release zip 版本见 0.0 状态卡；本节早期 MSIX layout 验证仍是历史记录，本轮原生拖动修复没有重建 MSIX layout。

桌面复验状态：

- Computer Use 已按用户偏好尝试；轻量 `list_apps` 可用，PowerShell 启动新 exe 后也能枚举到 `Ariadne` 窗口，但 `launch_app` 和 `get_window_state/activate_window` 均返回 `GetCursorPos failed: 拒绝访问 (0x80070005)`，没有进入目标窗口截图/拖动步骤。
- 仓库中存在 Ariadne 原生 Win32 `cmd/capturesmoke`，但本轮没有切换到该前台鼠标脚本做替代验收，避免把用户要求的 Computer Use 验证路径和 Win32 合成输入路径混在一起。

本轮未完成：

- 当前会话不能声明截图内容、贴图位置和贴图拖动真实桌面验收通过。
- 仍需在允许 Computer Use 捕获/输入的真实桌面中复验：Alt+A 框选后保存内容必须等于视觉选区；按 `P` 后贴图左上角应对齐选区左上角；贴图主体应通过 Wails 原生 drag region 顺滑拖动，菜单/OCR 控件仍可点击。

#### 2026-06-15 截图局部像素协议与贴图显式拖动修复（历史中间方案）

说明：本节保留当时排查和验证记录；当前有效方案以上一节“截图 visual 坐标协议与贴图窗口直接拖动修复”为准。多屏 per-display session 和 source-local session 像素裁剪能力继续保留；当前 UI 请求走 visual 坐标，贴图拖动主路径已改为 Wails 原生 `--wails-draggable: drag`，不再是前端运行时 `Window.SetPosition()` 直接移动。

用户反馈：

1. 当前截图保存内容仍与实际框选区域无关。
2. 区域截图后的贴图初始位置偏离实际截图位置。
3. 贴图窗口不能拖动。

修复：

1. `capturehistory` 导出物理显示器 bounds，`captureoverlay.Open()` 不再创建一个覆盖整个虚拟屏幕的大 session；改为按显示器裁剪截图并创建多个同组 overlay session/window。
2. 每个 overlay session 都有自己的 `Bounds`（Wails DIP 窗口 bounds）、`Native`（该显示器原生像素 bounds）和已经裁剪到该显示器的 `ImageURL/pngBytes`，选区保存时只在当前显示器子图内裁剪。
3. 任意一个 overlay session 完成截图或取消后，后端会清理同组其它显示器 session，并关闭其它覆盖层窗口，避免多屏覆盖层残留。
4. 前端选区、放大镜、取色和标注最终渲染统一以 `.capture-overlay-image.getBoundingClientRect()` 作为显示视口，再映射到当前 session 的 `nativeBounds`，不再直接使用整个 surface 尺寸推导截图坐标。
5. 贴图窗口一度删除前端 pointer delta 调 `MovePinned` 的拖动路径，改为让图片主体、stage 和 zoom layer 使用 Wails 原生 `--wails-draggable: drag`，工具栏、按钮和 OCR 控件继续 `no-drag`；后续又短暂改过 JS `Window.SetPosition()` 路径，最终当前方案已回到 Wails 原生 drag region 来解决抖动。

验证：

- `go test ./internal/captureoverlay ./internal/pinnedimage -v` 通过，新增 `TestOverlaySessionsSplitAndCropPerDisplay` 覆盖虚拟屏幕负原点、多显示器裁剪、单 display session 保存正确像素以及完成一个 session 后清理 sibling sessions。
- `go test ./...` 通过。
- `pnpm build` 通过；仍有既有 Rolldown `@vueuse/core` pure annotation 和 chunk size 警告。
- `wails3 task windows:build` 通过，bindings 为 `468 Packages, 23 Services, 179 Methods, 5 Enums, 165 Models, 0 Events`。
- 新增 `cmd/capturesmoke` 可重复烟测，报告路径 `dist\smoke\capture-pin-smoke-latest.json`；烟测会使用临时 `APPDATA/LOCALAPPDATA` 启动 `bin\ariadne.exe`，探测 Alt+A/Alt+Q 注册，尝试截图覆盖层、选区拖拽、按 `P` 贴图、检查保存 PNG 尺寸、贴图位置和贴图拖动。
- 当前 Codex 桌面会话的真实烟测阻断在系统截图权限：Computer Use 控制返回 `GetCursorPos failed: 拒绝访问 (0x80070005)`；Win32 smoke 证明 Alt+A/Alt+Q 启动前可注册、Ariadne 运行中均返回 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)`，并通过 `WM_HOTKEY` fallback 触发截图回调，但 Ariadne 日志记录 `open screenshot overlay: BitBlt 失败: Access is denied.`，因此未能继续到内容对比、近选区贴图和贴图拖动步骤。
- `bin\ariadne.exe` 更新为 `31667200` bytes，时间 `2026-06-15 00:38:45`，SHA256 `1FB592F36345298FC58799AE5BEAFCE0CB6B7A4D6688614C472DAFA7115812E0`。

本轮未完成：

- 当前会话不能声明本轮截图内容/贴图位置/贴图拖动真实桌面复验通过，因为截图 API 被 `BitBlt Access is denied` 阻断。
- 仍需在可截图的真实桌面中用 Computer Use 或人工做一次验收：Alt+A 框选后保存内容必须等于视觉选区；按 `P` 贴图应贴近选区左上角；贴图主体可拖动，按钮/OCR 控件仍可点击。

#### 2026-06-14 截图坐标、贴图位置与贴图拖动修复

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
go test ./...
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

结果：

- `go test ./...` 通过。
- `pnpm build` 通过；仍有 `@vueuse/core` pure annotation 和 chunk size 警告，不阻塞构建。
- `wails3 task windows:build` 通过，bindings 更新为 `467 Packages, 23 Services, 178 Methods, 5 Enums, 164 Models, 0 Events`。
- `bin\ariadne.exe` 更新为 `31515136` bytes，SHA256 `5D15210F48BE6FEF1C86469DAFB728F24B77E60A20D3B63CB3B531FCD1230797`。

本轮修复：

1. 覆盖层前端不再把 CSS 选区坐标直接发给 Go；改为按 `session.bounds / window.innerSize` 映射到原生屏幕像素，并将标注操作同步缩放。
2. 前端用原始截图数据按原生选区渲染最终 PNG，再交给后端保存，避免 HiDPI 下截图内容与框选区域错位。
3. 贴图服务新增 `OpenCaptureAt`，区域截图贴图会按选区原生左上角减 `15px` 打开，不再落在默认位置。
4. 贴图服务新增 `MovePinned`，贴图窗口拖动改为前端 pointer delta 调 Go 后端移动 Wails 窗口，避开透明 frameless WebView 原生拖动不稳定的问题。

Computer Use 真实桌面复验：

- `Alt+A` 打开 `Ariadne - 截图覆盖层`，从 `(520,340)` 拖到 `(820,560)` 后按 `P`，生成 `截图贴图 450x330`；Win32 窗口位置为 `(765,495)`，符合原生选区 `(780,510)` 附近减 `15px` 的预期。
- 对该贴图从窗口内 `(120,120)` 拖到 `(300,240)` 后，Win32 窗口位置从 `(765,495)` 移到 `(945,615)`，证明贴图可拖动。
- 再次以资源管理器为背景，从 `(800,300)` 拖到 `(1160,540)` 后按 `P`，生成 `539x360` PNG；抽看 `capture-20260614-221201.244140300-3998c5eafa4c.png`，内容包含实际框选的资源管理器面包屑、工具栏和文件夹列表，不再与框选区域无关。
- 第二张贴图窗口初始位置为 `(1185,434)`，匹配选区原生左上角附近。

#### 2026-06-14 截图放大镜与取色复制补齐

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
go test ./internal/captureoverlay ./internal/pinnedimage
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

结果：

- `go test ./internal/captureoverlay ./internal/pinnedimage` 通过。
- `pnpm build` 通过；仍有 `@vueuse/core` pure annotation 和 chunk size 警告，不阻塞构建。
- `wails3 task windows:build` 通过，bindings 仍为 `467 Packages, 23 Services, 178 Methods, 5 Enums, 164 Models, 0 Events`。
- `bin\ariadne.exe` 更新为 `31519744` bytes，SHA256 `A8C695FDB10974D586473FEA9962EC11D63360209D3AB4A1144FF36E24DF99A3`。

本轮修复：

1. 覆盖层新增跟随鼠标的放大镜，直接使用当前截图位图按原生坐标放大显示，中心十字指示当前像素。
2. 放大镜 HUD 显示当前像素颜色，支持 RGB/HEX 两种格式；`Shift` 切换格式。
3. `C` 复制当前像素颜色到系统剪贴板，复用缓存 canvas 读取像素，避免每次取色重新解码整张截图。

Computer Use 真实桌面复验：

- 启动最新 `bin\ariadne.exe` 后用 `Alt+A` 打开 `Ariadne - 截图覆盖层`，鼠标移动到覆盖层后放大镜和颜色 HUD 可见。
- 按 `C` 后系统剪贴板为 `rgb(255, 255, 255)`。
- 按 `Shift` 切换格式后再按 `C`，系统剪贴板为 `#FFFFFF`。
- 验证后停止 Ariadne 测试进程。

#### 2026-06-14 截图已有标注选中、移动、删除与文字重编辑

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
go test ./internal/captureoverlay ./internal/pinnedimage
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

结果：

- `go test ./internal/captureoverlay ./internal/pinnedimage` 通过。
- `pnpm build` 通过；仍有 `@vueuse/core` pure annotation 和 chunk size 警告，不阻塞构建。
- `wails3 task windows:build` 通过，bindings 仍为 `467 Packages, 23 Services, 178 Methods, 5 Enums, 164 Models, 0 Events`。
- `bin\ariadne.exe` 更新为 `31524864` bytes，SHA256 `B56A92D5C52D23D8EB1B9D17612C313CA6D827383BE831D3DB3557BCF063F636`。

本轮修复：

1. 截图覆盖层新增 `V` 选择模式，退出当前绘制工具后可以命中已有标注。
2. 已有矩形、直线、箭头、画笔、马赛克、序号和文字均可参与命中检测；选中项显示高亮描边或阴影。
3. 拖动选中标注会平移其坐标；`Delete` / `Backspace` 会删除选中标注。
4. 双击文字标注会打开原位置文本编辑框；提交空文本会删除该文字标注。文字编辑打开时，全局截图快捷键不会抢占输入，避免键入 `r`、`v`、`t` 时切换工具。
5. 文字工具创建输入框时，标注 canvas 的 `pointerdown` 现在阻止默认按钮焦点行为，避免输入框被同一次点击空提交；已有文字重编辑新增显式 `dblclick` 处理，不再只依赖 `PointerEvent.detail`。

Computer Use 真实桌面复验：

- 启动最新 `bin\ariadne.exe`，用 `Alt+A` 打开 `Ariadne - 截图覆盖层`；从 `(420,300)` 拖到 `(820,600)` 建立选区，按 `R` 绘制矩形，再按 `V` 进入选择模式。
- 将矩形从原绘制区域拖到更靠右下的位置后按 Enter 保存；虚拟化截图历史临时记录 `58bc7d4e59b1` 为 `600x449`，actions 为 `overlay,selection,copy,annotated,rect`，抽看结果图显示红色矩形位于拖动后的新位置。
- 此前同轮复验选中矩形后按 `Delete` 再保存，临时记录 `33477cf5a826` 的 actions 为 `overlay,selection,copy`，抽看结果图没有红色矩形，证明删除生效。
- 上述临时截图历史条目、PNG、缩略图和 Ariadne 测试进程均已清理。
- 后续文字重编辑复验中，Computer Use 分步确认 `文字` 按钮可见，修复 `pointerdown.prevent` 后输入框可出现并接收 `OLD` 输入，提交后 `选择` 按钮启用；双击未进入编辑框后补入显式 `dblclick` 处理。最终重建后的桌面复验被 Computer Use 输入层阻断，连续返回 `GetCursorPos failed: 拒绝访问 (0x80070005)`，所以文字双击重编辑仍保留为待真实桌面复验项。

#### 2026-06-14 shell/window 修复验证

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
gofmt -w main.go internal\shell\manager.go internal\shell\hotkey_test.go internal\platform\service.go internal\toolwindows\service.go
go test ./internal/shell ./internal/platform
go test ./...
wails3 generate bindings
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

结果：

- `go test ./internal/shell ./internal/platform` 通过。
- `go test ./...` 通过。
- `wails3 generate bindings` 通过：`467 Packages, 23 Services, 175 Methods, 5 Enums, 162 Models, 0 Events`。
- `pnpm build` 通过；仍有 `@vueuse/core` pure annotation 和 chunk size 警告，不阻塞构建。
- `wails3 task windows:build` 通过，`bin\ariadne.exe` 更新为 `31453696` bytes，SHA256 `BE0C466445CD4EED46D86130F98A4AC38CDA1113D88D12ED7F33CBF4A3566D22`。
- Win32 桌面探针通过：启动前 Alt+Q/Alt+A 均可注册；Ariadne 运行中 Alt+Q/Alt+A 均被 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)` 阻止；launcher 不是 topmost；Esc 隐藏 launcher；Alt+A 打开截图覆盖层；Alt+Q 重新唤起 launcher；合成拖动使 launcher 坐标从 `(900,648)` 变为 `(1070,713)`。
- 额外尝试通过启动器搜索自动打开设置窗口做工具窗口拖动复验，但该 UI 自动化链路未稳定打开设置窗口；本轮对工具窗口的结论来自同一 Wails `--wails-draggable` CSS 机制和代码覆盖，仍建议后续用 Computer Use/人工点开任意工具窗口做一次真实拖动确认。

#### 2026-06-13 最新已执行

```powershell
cd experiments\ariadne
$goRoot = Join-Path $env:USERPROFILE '.codex\tools\go1.26.4.windows-amd64\go'
$goBin = Join-Path $env:USERPROFILE '.codex\go-bin'
$env:GOROOT = $goRoot
$env:GOBIN = $goBin
$env:PATH = "$goRoot\bin;$goBin;$env:PATH"
gofmt -w main.go internal\platform\service.go internal\hosts\service.go internal\hosts\service_test.go
go test ./...
wails3 generate bindings
Set-Location frontend
pnpm build
Set-Location ..
wails3 task windows:build
```

本轮此前截图历史补充时，实际 gofmt 覆盖还包括：

```powershell
gofmt -w internal\capturehistory\service.go internal\capturehistory\capture_windows.go internal\capturehistory\capture_other.go internal\capturehistory\service_test.go
```

测试结果：

```text
?    ariadne                    [no test files]
ok   ariadne/internal/apps
ok   ariadne/internal/capturehistory
ok   ariadne/internal/clipboardhistory
ok   ariadne/internal/contracts
ok   ariadne/internal/filesearch
ok   ariadne/internal/hosts
ok   ariadne/internal/launchers
ok   ariadne/internal/platform
ok   ariadne/internal/plugins
ok   ariadne/internal/search
ok   ariadne/internal/settings
ok   ariadne/internal/workflows
ok   ariadne/internal/workmemory
```

绑定生成：

```text
Processed: 404 Packages, 11 Services, 63 Methods, 5 Enums, 51 Models, 0 Events
```

前端与桌面构建结果：构建通过，`bin\ariadne.exe` 更新为 `21751296` bytes，时间 `2026-06-13 22:41:35`。

已知警告仍存在：

```text
[INVALID_ANNOTATION] @vueuse/core pure annotation comment ignored due to position
```

该警告来自 Reka UI 间接依赖 `@vueuse/core`，当前不阻塞构建。

#### 较早验证记录

```powershell
cd experiments\ariadne
go test ./...
```

结果：

```text
ok ariadne/internal/contracts
ok ariadne/internal/plugins
ok ariadne/internal/search
ok ariadne/internal/settings
ok ariadne/internal/workmemory
```

```powershell
cd experiments\ariadne
wails3 generate bindings
```

结果：

```text
Processed: 395 Packages, 5 Services, 17 Methods, 3 Enums, 19 Models, 0 Events
```

注意：这是设置中心扩展前的 bindings 数量。新增 `StorageStatus` 字段后必须重新执行 `wails3 generate bindings`，不能沿用该数量作为最新证据。

```powershell
cd experiments\ariadne
wails3 build
```

结果：构建通过，产物为：

```text
experiments\ariadne\bin\ariadne.exe
```

### 4.3 Computer Use 桌面验证

验证方式：使用 Computer Use 启动 `experiments\ariadne\bin\ariadne.exe`，对真实 Ariadne 桌面窗口执行 UI Automation 和输入事件。

已验证：

1. 窗口标题为 `Ariadne`。
2. WebView 内容默认渲染为浅色折叠搜索框；没有可见原生标题栏，也没有固定顶部工具按钮。
3. 输入 `uuid 2` 后返回两条 `UUID v4` 结果。
4. `uuid 2` 结果显示 `复制结果` action。
5. `uuid 2` 结果不显示 `打开文件`。
6. `uuid 2` 结果不显示 `打开所在文件夹`。
7. 点击真实窗口里的 `复制结果` 后，界面内出现 `已复制`。
8. 点击 `工作记忆` 后进入 `工作记忆中心`。
9. 工作记忆中心展示时间线、详情、日报、知识、外部代理和隐私模式入口。
10. 点击 `手动补记` 后新增 `手动补记当前屏幕`，并显示 `已补记当前屏幕`。
11. 点击 `日报草稿` 后展示 `今日工作日报草稿`，并显示 `日报草稿已生成`。
12. 点击 `知识草稿` 后展示 `知识条目草稿`，并显示 `知识草稿已生成`。
13. 点击 `任务包` 后展示外部代理任务包和 `Requires review`，并显示 `外部代理任务包已生成`。
14. 点击 `启动器` 后返回主搜索窗口。
15. 点击 `设置` 后进入 `设置中心`。
16. 设置中心展示 `应用与截图`、快捷键、旧版 x-tools 导入和底部 `保存设置`。
17. 设置中心左侧 `Ariadne 配置` 显示 `已读回`。
18. 在 Codex MSIX 启动场景下，设置中心显示 `MSIX 实际路径`，指向 `LocalCache\Roaming\Ariadne\config.json`，并显示 `2505 bytes`。
19. 点击 `保存设置` 后，PowerShell 验证虚拟化实际文件更新时间变为 `2026-06-13 19:30:38`，版本为 `1`。
20. 设置中心左侧 `平台诊断` 面板显示 `windows/amd64`、`go1.26.4`、`exe 20.0 MB`、`能力 4 已接入 · 4 待接入`、Everything DLL 路径和 Wails PATH 状态。
21. 输入 `calculator` 后，真实 Start Menu 应用扫描结果 `Calculator` 排在第一，右侧预览显示应用快捷方式说明。
22. `Calculator` 应用结果显示 `打开应用` 和 `复制快捷方式路径`，不显示 `打开文件` 或 `打开所在文件夹`。
23. 输入 `ariadne config` 后，真实自定义启动项结果 `Ariadne 配置目录` 返回，右侧 preview 显示 `自定义启动项 · 文件夹`。
24. `Ariadne 配置目录` 显示 `打开`、`复制目标` 和 `更多`，没有文件专用的 `打开文件` 文案。
25. 打开 `更多` 菜单后显示 `加入记忆` 和 `收藏`。
26. 点击 `收藏` 后，Codex MSIX virtualized path 写入 `search_state.json`，记录 `launcher-ariadne-config-dir` 的 `favorite=true`。
27. 再次打开 `更多` 菜单后，动作显示为 `取消收藏`。
28. 新二进制启动后，Windows UI Automation 读取到设置中心自定义启动项相关按钮：`Ariadne 配置目录`、`Everything`、`新建启动项`、`保存启动项`、`保存设置`。
29. 设置中心自定义启动项管理区截图已确认可见：`C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_20-11-57.png`。
30. 剪贴板历史中心可通过启动器搜索结果打开；后续不要恢复顶部固定 `剪贴板` 按钮。
31. 剪贴板历史中心点击 `收集当前剪贴板` 后，PowerShell 确认 `clipboard_history.json` 写入测试文本，长度 `473` bytes。
32. 剪贴板历史中心点击 `置顶` 后，PowerShell 确认记录 `pinned=true`。
33. 启动器输入 `clip token` 后返回剪贴板历史结果，右侧 preview 显示来源、类型、置顶和记录时间，动作是 `复制内容`、`取消置顶`、`更多`。
34. 剪贴板历史中心关键截图：`C:\Users\luwei\AppData\Local\Temp\ariadne-clipboard-center.png`；主搜索剪贴板结果关键截图：`C:\Users\luwei\AppData\Local\Temp\ariadne-clipboard-search.png`。
35. 验证后已关闭 Ariadne 并删除测试 `clipboard_history.json`，避免污染真实历史。
36. Computer Use 打开截图历史中心，空状态显示 `0 张`、`捕获当前屏幕`、`清空未置顶`。
37. Computer Use 点击 `捕获当前屏幕` 后显示 `1 张`、`2560 x 1440`、`已捕获当前屏幕`，详情动作显示 `打开`、`打开所在文件夹`、`复制路径`、`置顶`、`删除`。
38. Computer Use 点击 `置顶` 后显示 `置顶 1`、`已置顶`、`取消置顶`。
39. Computer Use 回启动器输入 `cap 2560x1440` 后返回截图历史结果，主动作是 `打开` 和 `打开所在文件夹`。
40. Computer Use 展开截图历史结果 `更多` 菜单后显示 `复制路径`、`取消置顶`、`加入记忆`、`收藏`。
41. PowerShell 确认截图历史在 Codex MSIX 启动场景下写入虚拟化实际路径，JSON 记录 `pinned=true`、`width=2560`、`height=1440`，虚拟化 PNG 文件大小 `168126` bytes。
42. 验证后已关闭 Ariadne，并删除普通路径与 MSIX 虚拟化路径下的测试 `capture_history.json` 和 `capture_images`，避免污染真实历史或留下桌面截图。
43. Computer Use 启动最新 `bin\ariadne.exe` 后，确认默认是浅色折叠搜索框。
44. 通过搜索结果进入 `Hosts 管理中心` 后，页面显示 `新建方案`、`保存方案`、`删除方案`、`生成预览`、`应用到系统`、`写入边界` 和 `MSIX 实际路径`。
45. Hosts 中心真实读取到旧版 `.x-tools` 迁移出的 6 个启用方案：`公用`、`广告`、`国联`、`算力调度平台`、`CF优选`、`k8s`。
46. Codex MSIX 启动场景下，Hosts 配置实际路径为 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\hosts_profiles.json`，大小 `4254` bytes；这是旧配置迁移数据，保留不清理。
47. 点击 `生成预览` 后，页面显示 `最终行数 147`、`新增/移除 +2 / -5`，并展示 `ARIADNE HOSTS START/END` 新 marker 以及旧 `X-TOOLS HOSTS START/END` marker 移除差异。
48. 回到 `启动器` 后关闭 Ariadne；PowerShell 确认没有残留 Ariadne 进程。
49. Computer Use 打开工作流中心，页面显示 `工作流宏`、`已导入旧配置`、`1 个`、`clip-url-md5`、`新建工作流`、`保存工作流`、`删除工作流`、`运行`、变量 `{clipboard}`、`{input}`、`{prev}` 和旧配置路径。
50. PowerShell 确认 Codex MSIX 启动场景下工作流配置写入 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\workflows.json`，大小 `339` bytes，包含 1 个旧配置迁移工作流 `clip-url-md5` 和 2 个步骤。
51. Computer Use 设置剪贴板烟测文本后，在工作流中心点击 `运行`，验证 `url {clipboard}` 与 `hash {prev}` 两步链路真实执行，最终 MD5 `dd0432f9fd7c831001f1203372ec3755`，并显示 `工作流完成：clip-url-md5（共 2 步），结果已复制`。
52. 验证后已恢复原剪贴板文本、关闭 Ariadne，并确认无残留 Ariadne 进程。
53. 尝试启动器 `wf clip-url-md5 smoke` 桌面输入时，UIA `SetValue` 仍报 `0x80070057`；真实键盘输入能写入查询，但 UIA 文本树没有稳定返回结果区。本项不计为桌面通过，当前由 Go 搜索聚合测试覆盖。
54. 此前 Computer Use 启动 22:15:36 构建的 `bin\ariadne.exe`，确认窗口默认只有浅色搜索框，输入 `jsondiff` 后展开唯一命令结果 `打开 JSON 对比工具`，按 Enter 进入 JSON 对比中心。
55. JSON 对比中心默认样例显示 `存在差异`、`发现 4 处差异：新增 2，删除 1，变更 1`，说明搜索器启动工具路径和 JSON compare 服务在最终包中仍可用。
56. Computer Use 启动 22:41:35 构建的最新 `bin\ariadne.exe`，Win32 读取初始窗口尺寸为 `760x132`，临时注册 Alt+Q 返回 `ERROR_HOTKEY_ALREADY_REGISTERED (1409)`，说明 Ariadne 已注册全局热键。
57. Computer Use 键盘输入 `jsondiff` 并回车后，Win32 读取窗口尺寸为 `1180x760`，说明 JSON 对比中心路由和窗口尺寸切换仍然正常。
58. 从资源管理器窗口按 Alt+Q 后，Win32 读取 Ariadne 窗口尺寸回到 `760x132`，说明全局热键能唤起干净折叠启动器。
59. 验证期间发现旧版 `x-tools.exe` 同时运行会抢占 Alt+Q；关闭旧版后 Ariadne 注册成功。本项必须进入旧版并存/发布迁移策略。

待补验：

1. 输入 `Everything64.dll` 或其他稳定文件名，确认真实桌面 UI 返回 Everything 文件结果。
2. 复验设置中心平台诊断的 capability 计数，确认 `file_search` 跟随 DLL 存在变为已接入。第 20 条截图是在 file search capability 计数更新前拍的。
3. 复验设置中心平台诊断的 capability 计数，确认 `custom_launchers` 和 `search_ranking` 进入已接入数量。
4. 点击 `取消收藏` 并确认状态回写。
5. 执行一个非危险动作后确认 `search_state.json` 记录最近使用。
6. 构造多个匹配结果，确认收藏或最近使用结果真实置顶。
7. 用 Computer Use 或人工真实键鼠输入创建、保存、删除一个临时启动项，并确认 `launchers.json` 写入和清理。
8. 自动文本/图片剪贴板监听已完成并通过临时 `APPDATA` 真实进程烟测；剪贴板图片二维码识别、图片 OCR 和加入截图历史已由 Go 测试覆盖，剪贴板图片 OCR 的真实桌面复验仍可补。
9. Hosts 系统文件写入仍需人工确认的 UAC 烟测；自动化验证只到生成预览，不触发系统 hosts 改写。
10. Hosts 远程 URL 拉取、冲突列表和大文件滚动性能仍需单独复验。
11. 工作流中心创建/保存/删除临时宏的真实桌面复验仍需补；本轮只验证旧配置迁移宏运行。
12. 启动器 `wf ...` 搜索结果区的真实桌面输入复验仍需补；本轮 Go 测试已证明搜索 provider 聚合。
13. 开机启动设置开关仍需人工打开/关闭烟测，确认 HKCU Run 注册项和 `--hidden` 启动行为。
14. 设置中心平台诊断需要刷新截图，确认 `single_instance`、`global_hotkey`、`tray`、`autostart` 进入当前运行态 capability 矩阵。

验证中发现并修复：

1. Wails WebView 中原先先调用 `navigator.clipboard.writeText`，若剪贴板 API 不可用会阻断后续 action 反馈。
2. 修复后 copy action 优先走 Wails runtime `Clipboard.SetText`，并在失败时仍返回局部失败反馈，不再静默中断。
3. 工作记忆详情里的草稿动作原先被详情内容滚动挤到不可见区域，已移动到标题下方并取消该区域自动贴底。
4. 设置中心曾误导性显示逻辑 `%APPDATA%` 路径，而 PowerShell 看不到文件。定位为 Codex MSIX AppData virtualization 后，已增加 `virtualizedPath` 诊断并在 UI 中展示实际路径。
5. UIA `SetValue`/合成输入能改变 WebView 可访问值，但不应作为 Vue `v-model` 和 Wails `Upsert` 的落盘验收依据；启动项设置页保存需要真实输入链路复验。
6. Wails `Window.EmitEvent` 到前端监听不够可靠，Alt+Q 返回 launcher 时曾保留旧查询并展开到 `980x620`；已改为 `EmitEvent` + `ExecJS` DOM `CustomEvent` 双通道导航，并在外部唤起 launcher 时 reset 查询。
7. 旧版 `x-tools.exe` 并存时会占用 Alt+Q，导致 Ariadne 启动时热键注册失败；该冲突需要在迁移提示或诊断中显式处理。

说明：

- `隐私模式`入口已在真实窗口中确认可见；具体切换行为由 Go 后端测试覆盖，Computer Use 验证没有通过 UI 自动化去切换隐私设置本身。
- 桌面验证优先使用 Computer Use；当该工具未暴露时，本轮早期使用过 Windows UI Automation 和 Windows 截图 helper 验证真实 Ariadne 桌面窗口，不走浏览器模拟。设置存储关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_19-30-12.png`，平台诊断关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_19-45-51.png`，应用搜索关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_19-42-57.png`，自定义启动项搜索关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_20-03-55.png`，收藏菜单关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_20-04-49.png`，启动项设置管理区关键截图为 `C:\Users\luwei\AppData\Local\Temp\codex-shot-2026-06-13_20-11-57.png`，剪贴板中心关键截图为 `C:\Users\luwei\AppData\Local\Temp\ariadne-clipboard-center.png`，剪贴板主搜索关键截图为 `C:\Users\luwei\AppData\Local\Temp\ariadne-clipboard-search.png`。截图历史和 Hosts 本轮使用 Computer Use 文本树验证，测试截图数据已清理，Hosts 旧配置迁移数据保留。
- 收藏状态文件在 Codex MSIX 启动场景下写入 `C:\Users\luwei\AppData\Local\Packages\OpenAI.Codex_2p2nqsd0c76g0\LocalCache\Roaming\Ariadne\search_state.json`，长度 `197` bytes，更新时间 `2026-06-13 20:04:21`。

## 5. 测试覆盖

新增测试：

1. `internal/apps/service_test.go`
   - 开始菜单 `.lnk` 会返回 `ResultApp`。
   - 空查询不返回应用结果，避免主面板刷屏。
   - 应用结果动作面合法，不暴露 `open_parent`。
   - 精确应用名匹配优先于弱匹配。
2. `internal/contracts/actions_test.go`
   - 非文件结果暴露文件动作会失败。
   - copy action 缺少 inline feedback 会失败。
   - 文件结果允许打开文件和打开所在文件夹。
3. `internal/plugins/service_test.go`
   - 所有迁移目标内置插件 manifest 存在。
   - 常用文本插件走 Go 主路径执行。
   - copy-only 插件结果不暴露文件动作。
   - 高风险系统命令标记为 danger。
4. `internal/search/service_test.go`
   - 搜索聚合插件和工作记忆 provider。
   - 搜索聚合 Everything 文件 provider。
   - 搜索聚合 Start Menu 应用 provider，应用结果可按 score 排到前面。
   - 搜索聚合自定义启动项 provider。
   - 搜索聚合剪贴板历史 provider，且剪贴板结果不暴露 `open_parent`。
   - 搜索聚合截图历史 provider，且截图结果允许 `open_parent`。
   - 收藏和最近使用状态会提升排序并追加标签。
   - 搜索使用状态会持久化。
   - 搜索结果去重。
   - 文件和剪贴板结果动作规则不同。
5. `internal/launchers/service_test.go`
   - 自定义启动项会返回 `ResultCommand`，并显式声明打开动作。
   - 命令类启动项标记为 `danger`，并要求确认。
   - 启动项配置会持久化到 `launchers.json`。
   - 禁用启动项不会进入搜索结果。
   - 删除默认启动项会持久化 tombstone，重启后不会被默认配置重新合并回来。
   - 保存失败会出现在 `Status.lastSaveError`，前端可据此反馈。
6. `internal/clipboardhistory/service_test.go`
   - 文本剪贴板历史会持久化并能搜索。
   - 剪贴板结果动作面合法，非文件结果不暴露 `open_parent`。
   - 重复文本会去重刷新，并保留置顶状态。
   - `clip token` 前缀查询会按剪贴板历史搜索。
   - 清空未置顶会保留 pinned 记录。
   - 保存失败会出现在 `Status.lastSaveError`。
7. `internal/capturehistory/service_test.go`
   - 截图历史会持久化图片和 JSON 元数据并能搜索。
   - 截图结果动作面合法，并允许 `open_parent`。
   - 置顶状态会持久化。
   - 删除和清空未置顶会移除 Ariadne 截图目录内的图片，保留 pinned 记录。
   - `ImageDataURL` 能返回 PNG data URL 供前端预览。
8. `internal/hosts/service_test.go`
   - Hosts 预览会合并启用方案并检测 hostname/IP 冲突。
   - 系统 Hosts profile 是只读保护项，不能被删除。
   - 远程 Hosts profile 可通过 httptest 拉取并持久化。
   - 未确认时 `ApplyEnabledProfiles(false)` 只返回预览，不写 hosts 文件。
   - Hosts 打开动作符合 preview action 协议。
9. `internal/workflows/service_test.go`
   - 工作流运行会串联 `{input}`、`{prev}` 和插件结果，并选取 copy output。
   - `wf <id> <input>` 搜索会返回 `ResultWorkflow` 和显式运行/编辑动作。
   - 旧版 `%APPDATA%\x-tools\config.json` workflows 会导入并持久化。
   - 未知变量会失败，并把变量名写入步骤失败原因。
   - 删除默认宏会持久化 tombstone，重启后不会被默认宏合并回来。
   - 工作记忆候选工作流草稿未确认不持久化，确认后保存为正式工作流并可从 `workflows.json` reload。
10. `internal/checklists/service_test.go`
   - 工作记忆检查清单草稿未确认不持久化。
   - 确认后保存为正式检查清单资产并可从 `checklists.json` reload。
   - 无效检查清单草稿会被拒绝且不写入存储。
10. `internal/jsoncompare/service_test.go`
   - 对象 key 顺序不会被判定为语义差异。
   - 新增、删除、变更路径和计数与旧版行为一致。
   - 解析错误会返回左右侧标签和行号。
   - 非标识符 object key 会输出 `$["bad-key"]` 路径。
11. `internal/filesearch/service_test.go`
   - Everything 原始结果会转换为 `ResultFile`。
   - 文件结果暴露 `open_parent` 和复制路径动作。
   - 短查询不会触发 Everything。
   - Everything SDK/IPC 错误会记录到 `LastError()`，并返回空结果。
12. `internal/settings/service_test.go`
   - 保守默认值覆盖时间机器、AI、敏感导出和远程桌面排除。
   - 设置更新会归一化并持久化。
   - `StorageStatus` 会报告文件存在、读回成功、读回字节数和版本。
   - 损坏配置文件会报告 readback error。
   - MSIX `LocalCache\Roaming` 虚拟化路径会被检测并报告。
   - 旧版 x-tools 配置会映射安全的用户偏好、工作记忆、AI 和插件开关。
13. `internal/platform/service_test.go`
   - capability 矩阵默认不会把未注入的 shell runtime 误报为已完成；注入 `ShellStatus` 后会把单例、热键、托盘和 autostart 反映为当前运行态能力。
   - 会把 Start Menu app scan、自定义启动项和搜索排序标记为已接入，并让 file search 跟随 Everything DLL 是否可定位。
   - open action 缺少 path 会失败，不会静默假成功。
   - danger action 不会静默执行，会返回需要确认。
   - runtime diagnostics 会暴露进程 ID 和 runtime metrics。
   - ancestor lookup 能从 Ariadne 子目录定位仓库根目录的 `Everything64.dll`。
14. `internal/ocr/service_test.go`
   - 截图历史 OCR 会使用显式 capture ID 对应的图片，不从 `path` 字段推断动作。
   - 剪贴板 OCR 只接受图片记录，文本剪贴板记录会被拒绝。
   - 工作记忆图片 OCR 会写回 `ocrText`，并让图片记忆可按 OCR 文本搜索。
   - 敏感工作记忆会阻止 OCR，避免把敏感图片内容写入索引。
15. `internal/shell/hotkey_test.go`
   - `alt+q` 会解析为 Windows Alt + Q + no-repeat。
   - `alt+a` 会解析为 Windows Alt + A + no-repeat，用于截图覆盖层热键。
   - 裸 `F1`-`F24` 功能键会被接受；其他缺少修饰键的字母、数字和空格仍会被拒绝。
   - 支持 `ctrl+shift+f12` 这类功能键组合。
16. `internal/workmemory/service_test.go`
   - 工作记忆搜索返回证据。
   - 隐私模式阻止时间机器和手动补记。
   - 关闭隐私模式会清理隐私暂停原因。
   - 日报、知识草稿、外部代理任务包保留 evidence。
   - 本地日报会生成概览、主要工作、待跟进、复盘线索、隐私边界和证据 ID，并跳过敏感记忆。
   - 外部 AI 经验发现确认前不调用 provider，隐私模式阻断外发，敏感/疑似敏感 evidence 不进入外部 job，AI 返回 evidence 会按本地非敏感条目归一化，失败时保留本地规则报告降级。
   - 图片记忆写回 OCR 后可被关键词搜索，导出包会包含 OCR 文本。
17. `internal/skills/service_test.go`
   - 工作记忆知识草稿未确认不持久化。
   - 确认后保存为本地 Skill 资产并可从 `skills.json` reload。
   - 无效 Skill 草稿会被拒绝且不写入存储。
   - Codex skill 包导出未确认不写文件，确认后生成 `SKILL.md` 和包含 `<skill-id>/SKILL.md` 的 zip。
   - Codex skill live 安装未确认不写目标目录，确认后写入 `<skill-id>/SKILL.md`，已有同名目录默认阻断，显式 overwrite 后覆盖。
18. `internal/perfcheck/service_test.go`
   - 性能汇总会计算 count、min、max、average 和 p95。
   - 包体积对比会统计 exe、release zip、旧安装器、旧 onedir 总大小和文件数。
   - 预算结果会标记冷启动目标/理想值、Alt+Q 唤起目标、包体积是否小于旧安装器，并记录失败样本告警。
19. `internal/migration/service_test.go`
   - 旧版剪贴板文本/图片、截图图片和工作记忆图片可从旧 `%APPDATA%\x-tools` JSON 导入。
   - dry-run 不会改写 Ariadne 目标服务。
   - 导入会把图片复制到 Ariadne 数据目录，不继续引用旧路径。
   - 重复导入会按签名或 ID 去重。
20. 前端构建验证
   - `frontend/src/services/ariadneApi.ts` 保留 Wails `CancellablePromise.cancel()` 搜索取消能力，并把取消错误归类为 `SearchCancelledError`。
   - `frontend/src/stores/launcher.ts` 使用 serial 防止旧搜索响应覆盖新查询。
   - `pnpm build` 和 `wails3 task windows:build` 均通过。

## 6. 完整重构差距

以下内容仍未完成，不能把 Ariadne 标记为完整替代：

1. 桌面壳
   - 平台诊断与轻量 runtime metrics 已接入设置中心。
   - 单例运行已接入 Wails SingleInstance，二次启动会唤起现有窗口。
   - 托盘已接入 Wails SystemTray，包含启动器、工作记忆、剪贴板、截图、Hosts、JSON 对比、工作流、设置和退出入口。
   - 全局快捷键已接入 Windows `RegisterHotKey`，干净环境下 Alt+Q 可回到折叠启动器，Alt+A 可打开截图覆盖层，设置保存后会重新注册启动器/截图热键；旧版 x-tools 并存时仍可能抢占 Alt+Q，需要发布迁移提示或自动冲突诊断。
   - 开机启动已接入 Wails Autostart 设置钩子；仍需用户打开设置开关后做注册表写入/禁用烟测。
   - 独立工具窗口生命周期仍需继续做完整回归；单主窗口生命周期已接入关闭隐藏到托盘，launcher 已验证非置顶、Esc 隐藏和可拖动，贴图 OCR 联动、截图覆盖层、任务栏左侧网络监控小条和 OCR 图片叠框选择已接入；贴图近选区打开和拖动在旧产物中过了 Computer Use，本轮坐标回归修复后又补了贴图 1:1 exact-size、去顶部菜单、默认无阴影、右键菜单裁剪规避、Wails 原生拖动和选区原点对齐，并已通过命令行重新构建 `bin\ariadne.exe` 和 release zip；后续又补了裸 F1-F24 热键、截图捕获不隐藏 Ariadne 自有窗口和任务栏 owner 小条，但按用户要求未做新的桌面复验。
2. 搜索
   - Start Menu 应用扫描已完成基础接入。
   - Everything SDK 基础 Go 查询已接入。
   - 搜索 p95 统计和 Everything 最近查询/错误诊断已接入设置中心；可重复搜索 p95 基准已接入并记录真实 Everything 文件结果命中；前端搜索过期响应防护、provider 级查询取消和 Everything 索引覆盖提示已接入；Everything 文件结果真实桌面 UI 命中复验和快速输入桌面验收仍未完成。
   - 自定义启动项基础 provider、持久化、搜索、设置页可视化编辑、命令类启动项二次确认执行链路和启动失败诊断已接入；设置页真实创建/删除落盘复验，以及命令类启动项二次点击确认/成功/失败反馈的真实桌面烟测仍未完成。
   - 收藏和最近使用排序、设置中心数据清理入口已接入；真实 UI 取消收藏、重排置顶复验和清理按钮点击仍未完成。
3. 插件
   - 截图覆盖层已有历史真实桌面交互复验：Alt+A 唤起、拖拽释放选区、工具栏完成态、Enter 复制并关闭、放大镜、RGB/HEX 取色复制、已有标注二次选中/拖动/删除均曾通过 Computer Use。2026-06-15 用户反馈当前构建仍有选区内容、贴图位置和贴图拖动回归，本轮不再沿用旧结论，已改为前端提交 `<img>` 内 visual 坐标和实际显示尺寸，由 Go 统一裁剪；贴图初始位置由 Go 从最终 native 选区换算到 Wails DIP；前端 `captureGeometry` 已补自测覆盖 image/surface offset、floor/ceil 边界和 near-selection pin 位置；`go test ./internal/captureoverlay ./internal/pinnedimage -v`、`go test ./...`、`pnpm test:capture-geometry`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:msix` 已通过，但当时 Win32 smoke 的 `BitBlt` 返回 `Access is denied`，真实内容/贴图/拖动复验未完成。随后针对用户反馈的贴图轻微缩小、顶部菜单、右键菜单裁剪、位置偏移和拖动抖动问题，当前有效源码已改为 exact-size 贴图窗口、`1x1` 最小窗口、无顶部工具条、默认无阴影、无边框/圆角/内边距/`object-fit` 缩放、图片左上角锚定、选区原点对齐、Wails 原生 `--wails-draggable: drag` 拖动，并在右键菜单打开期间临时扩大透明窗口避免 WebView 裁剪；后续 10:35 修复又把截图捕获改为不隐藏 Ariadne 自有窗口。该后续修复已通过 `go test ./internal/captureoverlay ./internal/pinnedimage -v`、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:package`，但按用户要求只走后台命令行，未做桌面操作复验。高级编辑已覆盖选区缩放手柄、矩形、直线、箭头、画笔、马赛克、文字、序号、橡皮、颜色/粗细调节、撤销/重做和文字双击重编辑代码路径。旧版完整复刻仍缺截图内容、近选区贴图、贴图拖动、1:1 无痕贴图、右键菜单不裁剪、文字双击重编辑的真实桌面点击验收，以及更多旧版细节级对齐。JSON 对比窗口、网络监控中心、任务栏网络监控小条、贴图窗口、贴图 OCR 联动、本地 OCR、行级 OCR 文本选择和 OCR 图片叠框选择已完成 Ariadne Go/TS 迁移；网络监控小窗已改为任务栏左侧小条并保留旧位置兼容，仍缺真实桌面点击复验、真实任务栏嵌入复验和真实多显示器复验。
   - 工作流宏基础执行、中心 UI、高风险步骤确认和 JSON 导入导出已迁移；本地经验发现已能转外部代理任务包草稿、候选工作流草稿和检查清单草稿，并持久化处理状态；候选工作流已能经用户确认保存为正式工作流，检查清单草稿已能经用户确认保存为正式本地清单资产，知识草稿已能经用户确认保存为本地 Skill 资产、导出 Codex-compatible skill 包、安装到 live Codex skill 目录并写入 Ariadne refresh marker。仍缺运行中 Codex 实际热加载 newly installed skill 的验收。
   - Python legacy bridge 已接入受控显式入口；仓库当前 `src/plugins` 下 16 个旧内置插件已全部有非 `legacy_python` 的 Ariadne Go manifest 或工具窗口入口，并由 `TestLegacyPythonBuiltinsHaveNativeGoCoverage` 防回退。legacy bridge 仅保留给未知外部插件、临时过渡或未来未迁移扩展，不再作为当前内置插件长期主路径。
   - 插件参数面板和命令补全代码路径已接入 launcher；仍需真实桌面渲染和键盘交互复验。
4. 工作记忆
   - 基础中心 UI 已完成。
   - 屏幕时间机器 worker、设置 interval 重启、排除应用/窗口标题阻断、排除路径/内容对导入/OCR/导出的执行层阻断、手动补记、手动笔记、删除/清理和可读导出已接入。
   - 本地 OCR 写回、时间机器自动 OCR 写回、SQLite FTS、本地经验发现首版、经验发现处理状态、本地日报草稿、AI 日报润色首版、外部 AI 经验发现、外部 embedding、内置向量缓存和 Milvus REST 向量存储首版、支持多选证据组的独立问题复盘草稿、本地定期草稿调度、候选工作流草稿、检查清单草稿、候选工作流正式保存、候选检查清单正式保存、本地 Skill 正式保存、Codex skill 包导出、live 安装和 Ariadne refresh marker 握手已接入；旧版工作记忆 `entries.json` 历史迁移首版已接入；Markdown/文本、图片、PDF、Office 文档和 Ariadne 导出 zip 材料导入、筛选导出、五类排除规则配置和时间机器回放代码路径已接入。运行中 Codex 实际热加载 newly installed skill 仍未验收；外部 AI 经验发现仍缺真实桌面二次确认流程验收。
   - 工作记忆材料导入、筛选导出、排除规则配置和有截图帧时间机器回放仍缺真实桌面材料路径/规则粘贴、导出按钮点击、保存按钮点击、回放按钮点击和错误反馈复验；排除规则面板和无截图帧回放空状态可见性已用 Computer Use 只读验证。
   - 文本剪贴板历史、截图历史、图片 OCR 索引、工作记忆本地语义检索、外部 embedding 内置缓存、Milvus REST 向量存储、工作记忆/图片索引/剪贴板/截图历史保留策略、缩略图分层和旧数据回填已接入独立服务，OCR 文本可写入工作记忆；旧版剪贴板/截图/工作记忆历史迁移首版已接入，完整发布迁移仍未完成。
5. 设置
   - 设置中心 UI 和 Go settings 服务已完成。
   - 设置持久化已完成写后读回校验，并完成 Computer Use 与 PowerShell 双重复验。
   - Codex MSIX 启动场景下的 AppData virtualization 已检测并在 UI 中显示。
   - 旧配置导入已覆盖安全用户偏好和旧版敏感凭据安全迁移；旧历史数据迁移已另接 `migration` 服务。旧版明文 AI/embedding/Milvus 密钥只会写入 Windows Credential Manager，不会进入 Ariadne JSON；安全存储不可用时会跳过并在旧配置 notes 中说明。
   - API key 等敏感凭据已接入 Windows Credential Manager 首版；AI/embedding 当前密钥已写入本机 Credential Manager，运行时环境变量仍优先。设置中心密钥保存/清除按钮和旧配置密钥迁移 notes 已补 Go/前端构建级验证；剩余差距是真实桌面点击烟测，以及发布安装后 Credential Manager 读写权限复验。
   - 设置中心关闭入口和快捷键编辑已修复：关闭按钮从会被小屏媒体查询隐藏的 `.header-tools` 中独立出来，快捷键输入支持手动文本和按键捕获，裸 `F1`-`F24` 可作为截图等全局快捷键，并新增前端校验、局部应用按钮和运行态注册状态；贴图快捷键已从“仅保存配置”补齐为第三个全局热键，触发当前剪贴板图片/文本贴图。Taskfile 已把 bindings 生成改为 `-clean=false` 规避本机 Wails alpha clean rename `Access is denied`。该项已通过 `pnpm --dir frontend build`、`go test ./...` 和 `wails3 task windows:package`，但按用户要求未做真实桌面点击复验。
6. 独立工具窗口
   - 文本剪贴板历史中心已完成 Wails UI 迁移。
   - 截图历史中心已完成 Wails UI 迁移。
   - Hosts 管理中心已完成 Wails UI 迁移，系统写入仍需人工确认的 UAC 烟测。
   - 工作流宏中心已完成 Wails UI 迁移，创建/保存/删除临时宏的真实桌面复验仍需补。
   - 截图覆盖层已完成 Wails UI 迁移；拖拽释放、Enter 复制、右键清空/退出、Q 扫码入口、放大镜、RGB/HEX 取色复制、标注工具、已有标注二次选中/拖动/删除和截图设置副作用已接入并有历史桌面复验证据。2026-06-15 回归修复后，覆盖层选区提交改为前端 visual 坐标 + 实际显示尺寸，Go 端统一映射和裁剪；贴图位置由 Go 使用最终 native 选区换算，目前有 Go 测试、前端 `captureGeometry` 自测和构建验证，真实内容/贴图/拖动复验仍被当时 Win32 `BitBlt Access is denied` 权限错误阻断。后续贴图 1:1 对齐修复已把贴图窗口改为源尺寸 exact-size、默认无顶部菜单/无阴影/无边框缩放、选区原点对齐和 Wails 原生 drag-region 拖动，右键菜单通过临时扩大透明窗口避免 WebView 裁剪；后续 10:35 修复已把截图捕获改为不隐藏 Ariadne 自有窗口。该补丁已通过 Go 测试、`go test ./...`、`pnpm build`、`wails3 task windows:build` 和 `wails3 task windows:package`，但按用户要求未做新的桌面操作复验。旧版完整复刻仍缺截图内容、近选区贴图、贴图拖动、1:1 无痕贴图、右键菜单不裁剪、文字双击重编辑的真实桌面点击验收，以及更多旧版细节级对齐。JSON 对比、二维码识别、网络监控中心、任务栏网络监控小条、贴图窗口、贴图 OCR 联动、本地 OCR、行级 OCR 文本选择和 OCR 图片叠框选择已完成 Wails UI 迁移；网络监控小窗已改为任务栏左侧小条并保留旧位置兼容，仍缺真实桌面点击复验、真实任务栏嵌入复验和真实多显示器复验。
   - 自动文本/图片剪贴板监听和图片历史已迁移；剪贴板图片二维码识别、图片 OCR 和加入截图历史链路已接入。
7. 发布迁移
   - 旧配置读取、旧版剪贴板/截图/工作记忆历史迁移首版、Ariadne 本地数据回滚检查点首版、确认式回滚恢复、真实用户目录完整回滚演练、用户级 release zip 首版、旧版并存确认式交接、临时目录和真实默认用户目录 release 脚本安装/卸载烟测、Windows exe 品牌资源、release 图标和 unsigned MSIX layout 已接入；签名 `.msix` 安装/卸载验收仍未完成，当前阻断是本机缺 Windows SDK `makeappx.exe`、`signtool.exe` 和匹配 Publisher 的签名证书。
8. 验收
   - 已完成主搜索窗口、工作记忆中心、设置中心和截图覆盖层的 Computer Use 桌面窗口基础验证。
   - 还没有完整设计稿对比、暗色/亮色、多尺寸、长文本溢出和移动尺寸截图验收。
   - 已有运行态搜索 p95 统计入口、一次设置中心只读复验、可重复搜索 p95 基准报告，以及可重复的 Wails 冷启动/内存/体积/Alt+Q 注册 Win32 性能报告；仍缺 Alt+Q 真实唤起耗时和真实交互长时间搜索 p95 数据。

## 7. 后续验收口径

完整验收必须逐项证明：

1. `bin\ariadne.exe` 能作为桌面应用正常运行。
2. `Alt+Q` 能稳定唤起主窗口，焦点落在输入框。
3. Everything 搜索、应用搜索、插件搜索、工作记忆搜索能在同一入口协作。
4. 非文件结果没有文件动作。
5. 复制和执行反馈留在局部 UI。
6. 工作记忆中心能展示时间线、截图、剪贴板、笔记、证据引用、AI 草稿和工作记忆筛选导出入口。
7. 隐私模式和排除规则优先于采集、OCR、AI、embedding、导出。
8. 截图、贴图、OCR、二维码、Hosts、JSON 对比、剪贴板、截图历史、工作流、网络监控中心、任务栏网络监控小条和设置中心都有可运行窗口；其中网络监控小窗仍需补真实桌面点击、真实任务栏嵌入和真实多显示器位置策略验收。
9. 旧数据迁移和回滚路径通过测试。
10. 性能指标有本机实测记录；当前已有冷启动、内存、包体积、Alt+Q 注册、Win32 合成 Alt+Q 可见+前台耗时和可重复搜索 p95 基准。当前冷启动 p95 `686ms` 已达 `800ms` target，但未达 `500ms` ideal；仍需补真实键鼠焦点落点复验和真实交互长时间搜索 p95。

在这些证据齐备前，Ariadne 只能视为完整重构进行中，不能替换现有 Python/PyQt 发布主线。
