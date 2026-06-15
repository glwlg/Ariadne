# Ariadne: x-tools Wails 3 + Vue 3 重构方案

- 日期：2026-06-13
- 主题：以 Ariadne 为新产品代号，用 Wails 3 + Vue 3 重构 x-tools 主体验，优先解决视觉上限与交互质感问题
- 状态：执行中，当前实现台账见 `docs/plans/2026-06-13-ariadne-implementation-ledger.md`

## 1. 背景与判断

当前 x-tools 已经基于 Python + PyQt6 / qfluentwidgets 做过多轮 UI 与交互优化，核心能力也逐步成型：搜索、插件、预览动作槽、截图贴图、剪贴板历史、Hosts 管理、工作流宏、工作记忆等都已经进入可用状态。

继续沿用 Qt 的主要问题不是功能做不出来，而是审美与交互质感的上限越来越明显：

1. Qt Widgets / qfluentwidgets 容易保留控件味，难以做出足够现代的 command palette 体验。
2. 圆角、阴影、模糊、半透明、动效、响应式布局和主题 token 的调试成本偏高。
3. 很多视觉细节需要在 Python UI 代码里硬编码，长期维护不经济。
4. 现有主窗口、设置页和预览区越复杂，越难保持一致的设计语言。

因此，本次重构的主要动机不是“Python 一定慢”，而是：

1. 提升视觉上限。
2. 提升交互质感。
3. 建立更现代、更可演进的设计系统。
4. 为后续跨平台和复杂多窗口能力留出架构空间。

性能仍然重要，但不作为唯一重构依据。已有本机指标显示搜索链路并未明显失控，后续应在重构过程中用真实启动、搜索、窗口唤起、内存和打包数据持续验证。

### 1.1 命名与品牌定位

新产品代号确定为 `Ariadne`。

`Ariadne` 来自希腊神话中的 Ariadne's thread：阿里阿德涅给忒修斯一根线，让他能在迷宫中找到路径并返回。这个隐喻适合新的产品方向：

1. 用户的工作材料、截图、剪贴板、文件、窗口和命令历史像迷宫一样分散。
2. 工作记忆、搜索、预览和上下文关联是帮助用户找回路径的线索。
3. 主搜索窗口是入口，工作记忆中心是线索网络，后续个人代理基于这些线索提出行动建议。
4. 这个名字强调“找到线索与上下文”，而不是直白强调“记录、监控、追踪”。

命名策略：

```text
Product codename: Ariadne
Internal short name: Ari
Legacy engineering name: x-tools
Tagline: Find the thread through your work.
中文解释：在工作迷宫中找回线索。
```

迁移期不立即全量替换 `x-tools`：

1. 现有 Python / PyQt 版本继续使用 `x-tools` 作为工程名和发布名。
2. Ariadne 新版使用 `Ariadne` 作为窗口标题、视觉稿名称和新体验代号。
3. 配置目录、安装包名、进程名和仓库名先不急于改动，避免破坏现有用户数据与发布流程。
4. 等 Ariadne 新版体验通过验收后，再制定正式品牌化迁移方案，包括图标、安装包、配置目录、日志目录和旧数据迁移。
5. 若未来需要公开发布，应在发布前补做商标、域名和包名检索。

## 2. 目标与边界

### 2.1 目标

1. 用 Wails 3 + Vue 3 + Vite 构建 Ariadne 的完整新版桌面应用。
2. 完成主搜索窗口、工作记忆中心、剪贴板历史、工作流宏、设置页、工具窗口和系统能力的整体重构。
3. 建立新的视觉系统：现代工具型、安静、锋利、高密度但不拥挤。
4. 保留 x-tools 已经验证过的交互思想：
   - 结果类型感知。
   - 插件可声明预览动作。
   - 复制、执行等轻量反馈优先在界面内完成。
   - 键盘优先，鼠标辅助。
5. 定义新前后端边界，为 Everything 搜索、应用启动、插件动作、文件打开、托盘、快捷键等能力提供稳定接口。
6. 在完整重构过程中验证 Wails 3 在 Windows 上的窗口、托盘、全局快捷键、WebView2 渲染、打包体积、启动速度和交互流畅度。

### 2.2 设计边界

1. Ariadne 不是简单换壳，而是以新视觉系统和新前后端边界重建完整桌面体验。
2. 不把现有 Python 插件运行时作为新版长期主路径；必要时可用 legacy bridge 过渡。
3. 不引入 Element Plus、Naive UI 这类后台系统风格组件库。
4. 不牺牲 Windows 主路径稳定性去追求名义上的跨平台完整度。
5. 不在窗口、快捷键、托盘、数据迁移和核心功能未验收前直接替换现有发布版。

## 3. 技术选型

### 3.1 桌面壳与后端

采用 Wails 3 作为桌面应用框架：

- Go 负责系统集成、性能敏感逻辑和原生能力。
- 前端使用系统 WebView 渲染现代 UI。
- Windows 使用 WebView2，适合现代 CSS、动画和调试。
- Wails 3 提供窗口、托盘、菜单、快捷键、剪贴板、自动启动等管理 API。

风险：Wails 3 截至 2026-06-13 官方仍处于 Alpha 状态，release 也是 alpha 预发布。它适合作为 Ariadne 新主线的技术底座候选，但必须在窗口、快捷键、托盘、打包和数据迁移上充分验证后再替换现有稳定版。

参考：

- Wails 3 状态：https://v3.wails.io/status/
- Wails 3 Manager API：https://v3.wails.io/concepts/manager-api/
- Wails 3 First App：https://v3.wails.io/quick-start/first-app/

### 3.2 前端框架

采用 Vue 3 + Vite + TypeScript：

1. Vue 3 的 `script setup` 和 Composition API 适合工具型界面，状态与模板关系清晰。
2. Vite 开发体验成熟，适合快速迭代 UI。
3. Vue 模板对搜索结果、预览面板、设置表单、插件参数面板这类结构化界面可读性好。
4. 中文开发生态更友好，长期维护成本低。
5. Wails 3 支持 Vue 模板。

Vite 工具链的生态判断：

- VoidZero 是 Vite、Vitest、Rolldown、Oxc 等工具背后的公司。
- Cloudflare 于 2026-06-04 宣布收购 VoidZero，并承诺相关项目继续开源、厂商中立、社区驱动。
- 该事件增强的是整个 Vite 工具链生态，不只服务于 Vue，但 Vue 与 Vite 的历史关系和生态协同更强。

参考：

- Cloudflare 收购 VoidZero：https://www.cloudflare.com/press/press-releases/2026/cloudflare-acquires-voidzero-to-build-the-future-of-the-ai-native-web/
- VoidZero 加入 Cloudflare：https://blog.cloudflare.com/voidzero-joins-cloudflare/

### 3.3 UI 基础设施

采用：

```text
Vue 3
Vite
TypeScript
Tailwind CSS
shadcn-vue
Reka UI
Pinia
```

定位：

1. Tailwind CSS：作为样式 token、间距、颜色、状态、布局的实现层。
2. shadcn-vue：作为现代组件源码起点，不作为不可修改的黑盒组件库。
3. Reka UI：作为菜单、弹窗、Tooltip、Select、Command、Popover 等无样式交互 primitive。
4. Pinia：管理搜索状态、插件状态、设置状态、窗口状态和临时 UI 状态。

不采用 Element Plus / Naive UI 作为主依赖。原因是它们更适合后台管理系统，默认视觉容易把 x-tools 带回通用 SaaS 后台气质，不符合 command palette 工具的目标。

参考：

- shadcn-vue：https://www.shadcn-vue.com/
- Reka UI：https://reka-ui.com/docs/overview/introduction
- Nuxt UI：https://ui.nuxt.com/
- Tailwind Plus：https://tailwindcss.com/plus

## 4. 视觉与设计系统

### 4.1 设计方向

目标气质：

1. 现代工具型。
2. 高密度但清晰。
3. 冷静、锋利、轻量。
4. 键盘优先。
5. 不做营销页、不做后台系统、不做花哨卡片堆叠。

参考产品气质：

1. Raycast：命令面板、键盘流、动作菜单。
2. Linear：克制排版、状态层次、低噪声信息密度。
3. Arc：细腻动效、空间感、现代桌面质感。
4. Spotlight：快速唤起、低干扰输入。

这些是体验参考，不做逐像素模仿。

### 4.2 配色方案

采用成熟色彩体系组合，而不是手工拍脑袋调色：

```text
Neutral: Zinc
Primary: Teal
Success: Jade
Warning: Amber
Danger: Tomato
Info / Selection: Cyan or Teal alpha
```

命名为 `Graphite Teal`。

选择理由：

1. `Zinc` 比 `Slate` 更少冷蓝味，比 `Stone` 更现代，适合作为工具型中性色。
2. `Teal` 有技术感但不抢内容，避免普通蓝色工具味和紫色 AI SaaS 味。
3. `Jade / Amber / Tomato` 来自成熟 UI 色阶，适合语义状态。
4. Radix Colors 提供 light / dark / alpha 色阶，适合构建可访问的 UI token。

参考：

- Radix Colors：https://www.radix-ui.com/colors
- Tailwind Colors：https://tailwindcss.com/docs/customizing-colors
- shadcn-vue theming：https://www.shadcn-vue.com/docs/theming

### 4.3 基础 token 草案

Light mode 为主，Dark mode 作为深色模式可选；Ariadne 默认不能是黑色界面。

```css
:root {
  --font-sans: Inter, "Segoe UI", "Microsoft YaHei", sans-serif;
  --radius-sm: 6px;
  --radius-md: 8px;
  --radius-lg: 12px;
}

.dark {
  --background: var(--zinc-950);
  --foreground: var(--zinc-50);
  --surface: var(--zinc-900);
  --surface-raised: color-mix(in oklch, var(--zinc-900) 82%, var(--zinc-800));
  --surface-hover: var(--zinc-800);
  --border: var(--zinc-800);
  --border-strong: var(--zinc-700);
  --muted: var(--zinc-400);
  --muted-foreground: var(--zinc-500);
  --primary: var(--teal-400);
  --primary-hover: var(--teal-300);
  --primary-soft: var(--teal-a3);
  --success: var(--jade-400);
  --warning: var(--amber-400);
  --danger: var(--tomato-400);
}

.light {
  --background: var(--zinc-50);
  --foreground: var(--zinc-950);
  --surface: #ffffff;
  --surface-raised: var(--zinc-100);
  --surface-hover: var(--zinc-200);
  --border: var(--zinc-200);
  --border-strong: var(--zinc-300);
  --muted: var(--zinc-500);
  --muted-foreground: var(--zinc-600);
  --primary: var(--teal-600);
  --primary-hover: var(--teal-700);
  --primary-soft: var(--teal-a3);
  --success: var(--jade-600);
  --warning: var(--amber-600);
  --danger: var(--tomato-600);
}
```

实际实现时不要求完全使用以上变量名，但必须保持 token 化，不允许把颜色散落在组件内。

## 5. 产品架构

### 5.1 新架构分层

```text
frontend/
  Vue UI
  stores
  components
  design tokens
  Wails bindings adapter

backend/
  Go application shell
  window manager
  tray manager
  global hotkey manager
  search service
  plugin service
  platform service
  settings service

legacy/
  Python app remains available during migration
```

重构期间不删除 Python 代码。Ariadne 新版与现有应用并行存在，等完整体验、数据迁移和发布链路通过验收后再决定正式切换节奏。

### 5.2 前后端通信模型

前端不直接理解系统细节，统一调用 Go backend service：

```ts
interface SearchService {
  search(query: string): Promise<SearchResponse>
  executeAction(action: PreviewAction): Promise<ActionResult>
}

interface WindowService {
  showLauncher(): Promise<void>
  hideLauncher(): Promise<void>
  toggleLauncher(): Promise<void>
}

interface SettingsService {
  getSettings(): Promise<AppSettings>
  updateSettings(patch: Partial<AppSettings>): Promise<AppSettings>
}
```

后端返回结构化结果，而不是让前端猜路径、类型或行为。

## 6. 核心数据模型

### 6.1 搜索结果

```ts
type SearchResultType =
  | "file"
  | "app"
  | "plugin_trigger"
  | "plugin_result"
  | "workflow"
  | "clipboard"
  | "memory"
  | "command";

interface SearchResult {
  id: string;
  type: SearchResultType;
  title: string;
  subtitle?: string;
  icon?: ResultIcon;
  score?: number;
  payload?: Record<string, unknown>;
  preview?: PreviewDescriptor;
  actions: PreviewAction[];
}
```

### 6.2 预览动作

必须保留现有 `get_preview_actions` 思想：动作由结果或插件显式声明，不能仅凭 `path` 字段推断。

```ts
interface PreviewAction {
  id: string;
  label: string;
  icon?: string;
  shortcut?: string;
  kind:
    | "open"
    | "open_parent"
    | "copy"
    | "pin"
    | "run"
    | "plugin"
    | "danger";
  payload?: Record<string, unknown>;
  feedback?: {
    successLabel?: string;
    durationMs?: number;
  };
}
```

原则：

1. `file` / `app` 可以有默认打开动作。
2. `copy_result`、二维码、计算结果、文本结果不应因为存在 `path` 字段而继承文件动作。
3. 前两个动作可直接展示，更多动作进入菜单。
4. 复制成功等轻量反馈应优先显示在按钮或局部状态上，不使用 Windows 通知。

## 7. 完整重构范围

Ariadne 的目标是替代现有 x-tools 桌面体验，而不是只交付一个搜索面板。实施可以分批，但产品范围应覆盖现有核心能力和已经规划的工作记忆方向。

### 7.1 桌面壳与系统能力

1. Wails 3 应用启动、生命周期管理和单例运行。
2. 无边框主窗口、设置窗口、独立工具窗口和必要的浮动窗口。
3. 全局快捷键：
   - `Alt + Q`：搜索启动器。
   - `Alt + S/C/A`：截图、复制、贴图相关能力。
   - 后续可配置快捷键。
4. 系统托盘：
   - 显示/隐藏主窗口。
   - 打开设置。
   - 网络监控入口。
   - 工作记忆开关。
   - 诊断导出。
   - 退出应用。
5. 开机启动、配置读写、日志、诊断和指标采集。
6. 安装包、卸载、旧配置迁移和回滚策略。

### 7.2 主搜索与启动器

1. Command palette 风格启动器：
   - 快速唤起。
   - 搜索输入。
   - 结果列表。
   - 右侧预览。
   - 动作按钮与更多菜单。
   - 键盘导航。
   - 内联状态反馈。
   - 空查询时只显示干净搜索框，不展示固定工具按钮或桌面软件首页。
2. Everything 文件搜索。
3. 应用启动搜索。
4. 自定义启动项。
5. 收藏与最近使用排序。
6. 插件触发与命令直输。
7. 参数面板和命令补全。
8. 文件打开、打开所在目录、复制路径。
9. 非文件结果按结果类型展示动作，不继承文件动作。
10. 精确插件命令应优先于 Everything 文件结果，例如 `net` 应先打开网络监控，而不是先打开名为 `net` 的文件夹。

### 7.3 插件、动作与工作流

1. 新插件协议：
   - 关键词。
   - 命令 schema。
   - 结果类型。
   - 预览内容。
   - 预览动作。
   - 平台能力声明。
2. 内置插件迁移：
   - 计算器。
   - 时间戳。
   - Base64。
   - Hash。
   - JSON 格式化。
   - JSON 对比入口。
   - URL 编解码。
   - UUID。
   - 二维码生成。
   - 系统命令。
   - Hosts。
   - 剪贴板。
   - 截图历史。
   - 工作流。
   - 工作记忆。
3. 工作流宏：
   - 搜索入口执行。
   - 可视化编辑。
   - 命令链变量。
   - 执行结果回传。
4. Python legacy bridge 只作为过渡方案，不作为 Ariadne 长期插件主路径。

### 7.4 工作记忆与上下文中心

1. 工作记忆中心。
2. 时间线视图。
3. 截图历史。
4. 剪贴板历史。
5. 手动收藏与笔记。
6. 自动截图与屏幕时间机器。
7. OCR、摘要、标签和语义检索。
8. 工作日报、复盘、知识草稿和经验发现。
9. 隐私模式、采集开关、排除规则和数据导出。
10. 个人代理能力入口：
    - 记忆检索。
    - 工作总结。
    - 动作建议。
    - 外部代理任务包生成。

### 7.5 独立工具窗口

1. 截图覆盖层。
2. 贴图窗口。
3. OCR 文本选择与复制。
4. 二维码识别。
5. Hosts 管理。
6. JSON 对比。
7. 剪贴板历史中心。
8. 截图历史中心。
9. 网络监控中心与后续任务栏贴边小窗。
10. 设置中心。

截图、贴图、OCR 这类强原生窗口能力需要单独验证实现方式。可以使用 Go 原生能力、Wails 窗口、WebView UI 或过渡期 Python 子进程，但最终产品体验必须纳入 Ariadne 的统一入口、设置、指标和视觉系统。

### 7.6 发布、迁移与兼容

1. 旧配置读取与迁移。
2. 旧历史数据迁移。
3. 新安装包和卸载程序。
4. 旧版并存策略。
5. 回滚策略。
6. 图标、窗口标题、进程名和品牌资产切换。

## 8. 实施批次与切换策略

分批实施是为了降低 Wails 3、WebView2、数据迁移和强原生窗口能力的风险，不代表 Ariadne 只做局部功能。每个批次都应朝完整目标态推进，并保持与现有 x-tools 可回退。

### 8.1 验证构建：主窗口与系统能力

目标：先验证 Ariadne 的技术底座和视觉方向是否成立。

交付：

1. Ariadne Wails 应用骨架。
2. Vue 3 + Vite + TypeScript 前端工程。
3. Graphite Teal token 与基础组件。
4. 高保真主搜索窗口。
5. 全局快捷键、托盘、窗口显示/隐藏和焦点恢复。
6. Everything 搜索、应用搜索和最小动作分发。
7. 对比截图、启动时间、窗口唤起时间、搜索 p95 和内存记录。

验收重点：

1. 视觉是否明显优于当前 Qt 版。
2. 键盘流是否顺。
3. WebView2 是否有明显输入延迟。
4. 窗口显示/隐藏是否稳定。
5. Wails 3 Alpha 是否出现阻塞级问题。

### 8.2 主线能力：搜索、插件与设置

1. Everything 搜索。
2. 应用扫描。
3. 插件触发与结果协议。
4. 预览动作。
5. 收藏与使用频率排序。
6. 命令补全和参数面板。
7. 工作流宏执行。
8. 设置中心基础能力。
9. 托盘与诊断入口。
10. 配置读写和旧配置兼容。

### 8.3 工作记忆与历史中心

1. 剪贴板历史。
2. 截图历史。
3. 工作记忆中心。
4. 时间线视图。
5. OCR、摘要、标签和搜索。
6. 自动截图与屏幕时间机器。
7. 工作日报、复盘、知识草稿和经验发现。
8. 隐私模式、排除规则、导入导出和数据清理。

### 8.4 独立工具与完整发布

1. 截图覆盖层和贴图窗口。
2. Hosts 管理。
3. JSON 对比。
4. 网络监控。
5. 工作流宏可视化管理。
6. 完整设置中心。
7. 新安装包、卸载程序和升级流程。
8. 旧数据迁移。
9. 回滚机制。
10. 老版本并存策略。
11. 图标、品牌资产、窗口标题、进程名和配置目录正式切换。

## 9. 验收标准

### 9.1 体验标准

1. 主搜索窗口视觉明显优于当前 Qt 版。
2. 输入、搜索、上下移动、执行动作没有肉眼可见卡顿。
3. 结果列表与预览区信息层次清晰。
4. 暗色主题精致，亮色主题可用。
5. 复制成功、执行成功、失败提示都在局部完成，不打断搜索流。
6. 非文件结果不会出现文件专属动作。

### 9.2 性能标准

建议目标：

1. 应用冷启动到可响应：小于 800ms，理想小于 500ms。
2. `Alt + Q` 到窗口可输入：小于 120ms。
3. 搜索 p95：小于 100ms。
4. 输入连续字符无明显丢帧。
5. 打包体积较现有 Python onedir 明显下降。
6. 常驻内存可接受，且低于现有 PyQt 版本。

### 9.3 工程标准

1. 前端组件不直接调用平台 API。
2. 颜色、圆角、阴影、字体、动效必须 token 化。
3. 搜索结果和动作必须结构化。
4. 新插件协议需要文档化。
5. 至少覆盖搜索服务、动作分发、结果类型行为的基础测试。
6. 不破坏现有 Python 版主线。

## 10. 风险与对策

### 10.1 Wails 3 Alpha 风险

风险：API 或行为仍可能变化，Windows 窗口边界、快捷键、托盘等能力可能存在边缘问题。

对策：

1. 先做可验证构建，不直接替换现有发布版。
2. 记录 Wails 版本。
3. 避免使用过于冷门或未稳定的 API。
4. 保留 Python 版作为可回退主线。

### 10.2 WebView 窗口质感风险

风险：无边框、阴影、透明、窗口定位、焦点恢复可能不如预期。

对策：

1. 优先验证窗口显示/隐藏、焦点、快捷键和多屏定位。
2. 不在未验证前承诺替换截图覆盖层。
3. 必要时对主搜索窗口使用 Wails，对截图类能力保留原生或 Python 子进程。

### 10.3 插件生态重写风险

风险：现有 Python 插件全部迁移到 Go/TS 成本较高。

对策：

1. 先定义新插件协议。
2. 高频插件优先用 Go 重写。
3. 复杂或低频插件可通过 sidecar 或 legacy bridge 暂时保留。
4. 不把 Python 兼容层作为 Ariadne 的长期主路径。

### 10.4 视觉风格漂移风险

风险：引入 Tailwind 和 shadcn 后，如果没有 token 和组件规范，仍可能变成拼贴 UI。

对策：

1. 先定义 Graphite Teal token。
2. 建立核心组件清单。
3. 主搜索窗口先做高保真，不急于铺开设置页。
4. 每个新界面必须复用 token，不允许局部随意调色。

## 11. 推荐目录结构

若采用并行重构，可新增：

```text
experiments/
  ariadne/
    go.mod
    main.go
    frontend/
      package.json
      src/
        app/
        components/
        features/
        stores/
        styles/
        bindings/
    internal/
      search/
      plugins/
      platform/
      settings/
```

若 Ariadne 转为正式主线，可再调整为：

```text
app/
  desktop/
  frontend/
  internal/
legacy-python/
```

建议先放在 `experiments/ariadne` 或独立分支，避免污染现有 PyQt 发布链路；等完整验收后再迁入主线目录。

## 12. 当前推进点

当前推进点是建立 Ariadne 可验证构建。它不是最终范围收缩，而是完整重构的工程起点：

1. 建立 Ariadne Wails 3 + Vue 3 + Vite 项目。
2. 接入 Tailwind / shadcn-vue / Reka UI。
3. 实现 Graphite Teal token。
4. 做主搜索窗口高保真版本。
5. 实现 Everything 查询和最小动作分发。
6. 用截图、启动时间、输入延迟、搜索 p95 和主观视觉对比来决定是否正式迁移。

如果可验证构建无法明显赢过当前 Qt 版，则保留现有 Python 主线，只把局部性能和 UI 问题继续在现有架构内治理。
