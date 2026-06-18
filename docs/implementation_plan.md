# Flow Redesign：6 张设计图对齐实施计划

将现有 WorkMemoryCenter 的 6 个子页面 UI 调整为与设计图一致。

> [!IMPORTANT]
> 本次改动集中在 **模板 (template)** 和 **样式 (CSS)** 层面。不涉及 Go 后端、store 逻辑或数据模型变更。功能逻辑（事件处理、store 调用）全部保留，只调整 UI 结构和视觉呈现。

## 差异分析总览

| 设计图 | 当前实现 | 关键差异 |
|---|---|---|
| #03 心流 Cognitive Canvas | 三栏已实现（rail + canvas + inspector） | ❶ 缺少顶部时间轴导航条（带时间刻度 + 当前指针）❷ 缺少右侧 Agent Inspector 面板（OCR 预览/质检/影响分析/隐私边界）❸ 左栏缺少截图/OCR/剪贴板分类证据缩略图卡 ❹ 底部 Command Dock（Ask/Summarize/Search/Handoff/Optimize 5 个入口）不存在，只有 status-strip ❺ 搜索栏 + Ctrl K 快捷键缺失 ❻ 右侧 "窗口" 浮层面板（列出当前打开的程序窗口）缺失 |
| #04 时间线 Forensic Lanes | 多轨已实现 | ❶ 顶部缺少日期导航 + 时间范围选择器 + "工作时间" 筛选 ❷ 多轨泳道缺少截图缩略图预览卡 ❸ 右侧检查器缺少证据检查器面板（截图大图 + OCR 文本 + 质检状态 + 证据元数据 + 操作按钮）❹ 底部缺少批量操作 Dock（补跑OCR/加入复盘/导出所选/复制链接/标记敏感）❺ 缺少时间密度图（底部小波形图）|
| #07 洞察 Pattern Radar | 雷达节点已实现 | ❶ 顶部缺少日期导航 + 时间范围选择器 + "本地归纳/AI归纳" 切换按钮组 ❷ 节点之间缺少连线（关联图谱）❸ 节点缺少详细分类标识（自动化机会/知识沉淀/待复盘/风险信号不同颜色图标）❹ 右侧 Inspector 缺少"关键证据链"列表 + "建议动作"卡片网格 ❺ 底部缺少操作 Dock（运行AI归纳/生成每日复盘/创建任务包/导出洞察报告/更新索引）|
| #11 草稿 Timeline-Synced Brief | 三栏已实现 | ❶ 缺少顶部证据时间线缩略图条（与草稿段落编号联动）❷ 左侧草稿库缺少条目计数/引用计数/收藏 ❸ 右侧缺少"来源段落"编号列表 + "证据详情"（截图+OCR+时间）❹ 右侧缺少"AI润色状态"面板（模型/风格/内容优化/结构优化/术语统一/润色完成时间）❺ 底部缺少操作 Dock（生成日报/生成复盘/生成知识/生成清单/AI润色 + 导出Markdown/导出PDF/保存知识）|
| #15 资产 Agent Package Hub | 三栏已实现 | ❶ 顶部缺少面包屑导航（资产库 > 代理任务 > 具体任务）❷ 中间任务包缺少结构化 Goal/Context/Evidence/Boundaries/Acceptance 编号分区 ❸ 右侧缺少"就绪度评估"环形图（82/100 分数）❹ 右侧缺少"缺失证据"列表 + "风险边界"面板 ❺ 底部缺少操作 Dock（复制任务包/生成工作流/生成检查清单/保存为Skill/安装前确认 + 导出）❻ 底部候选产物区缺少"预览工作流/预览清单/预览Skill"标签 |
| #16 规则 Pipeline Control Room | 管线+规则已实现 | ❶ 顶部缺少管线可视化流程图（捕获来源→排除规则→质检→OCR→语义索引→导出/资产，带箭头连接器和统计数字）❷ 左侧缺少"捕获来源"列表（屏幕截图/OCR文本/剪贴板/应用上下文/窗口标题 各带今日数量和运行状态）❸ 中间排除规则缺少表格形式（规则名/匹配条件/命中次数/动作/状态/优先级/操作列）❹ 右侧 Inspector 缺少"影响预览"（阻止数量/减少体积/节省索引）+ "受影响应用 Top 5" + "最近命中规则" ❺ 底部缺少操作 Dock ❻ 缺少"敏感凭据模式"面板 |

## 公共差异（影响所有页面）

1. **Command Dock（底部操作栏）**：设计图中每个页面底部都有一个深色操作 Dock，当前只有简单的 `status-strip`
2. **顶部全局搜索栏**：设计图中有 `Q 搜索证据...` + `Ctrl K` 快捷键
3. **时间轴导航条**：#03、#04、#11 都有顶部横向时间轴（带日期选择 + 时间刻度 + 当前指针），当前不存在
4. **Agent Inspector 右侧面板**：设计图中是统一的"证据检查器"面板样式（标题栏 + 内容分区），当前各页用简单的 `flow-quiet-panel` 堆叠
5. **侧栏品牌区**：设计图 `Ariadne` logo + `本地优先·自动整理` 副标题，当前是 `心流` + `你的第二大脑`

---

## Proposed Changes

由于 [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue) 是 3287 行的巨石组件，改动将分阶段进行，每阶段按页面聚焦。

### Phase 1：公共基础（侧栏 + 顶栏 + Command Dock + Inspector 面板样式）

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- **侧栏品牌区**：`Ariadne` logo + `心流` + `本地优先·自动整理` 副标题（对齐设计图 #16 左上角）
- **侧栏用户区**：底部增加用户头像 + `luwei` + `本地模式` 标签
- **顶部全局搜索栏**：在 `flow-stage-top` 中增加搜索框 + `Ctrl K` 快捷键提示，加入"时间机器"按钮和同步/刷新图标
- **Command Dock**：替换 `status-strip` 为全宽深色底部操作 Dock，各页面内容不同（通过 `activeFlowPage` 条件渲染）
- **Agent Inspector 面板模板**：统一右侧面板为 `Agent Inspector` 标题 + 可折叠/关闭 + 分区（当前回答基于、证据详情、OCR/质检/影响分析）

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增 `.flow-command-dock` 样式（深色背景 #18181b、圆角、flex 布局、按钮 hover 效果）
- 新增 `.flow-global-search` 搜索栏样式
- 新增 `.flow-agent-inspector` 统一面板样式（标题栏、折叠/关闭、分区间距）
- 新增 `.flow-time-ruler` 横向时间轴样式（刻度、当前指针动画）
- 更新 `.flow-sidebar-brand` 样式
- 新增 `.flow-user-badge` 用户标签样式

---

### Phase 2：#03 心流 · Cognitive Canvas

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 在 `flow-cognitive-home` 顶部增加横向时间轴导航条（`flow-time-ruler`），带时间刻度 06:00~22:00 + 当前时间指针
- 顶部增加 `时间粒度: 5分钟` 选择器 + `本地优先` 标签 + 搜索栏
- 左侧 `flow-cognitive-rail` 下方增加截图/OCR/剪贴板分类证据缩略图卡片（按时间排列，显示缩略图 + `+N` 计数）
- 中间 `flow-cognitive-canvas` 的结论卡片增加：置信度 `86%` + 生成时间 + "交给代理 >" 按钮
- 右侧 Inspector 重构为 Agent Inspector 面板：当前回答基于 → 本地证据范围 → 选中证据详情（ID/预览/OCR/元数据/日志 tab）→ 质检状态 → 影响分析 → 隐私与边界
- 中间增加"窗口"面板浮层（企业微信/Chrome/DataGrip/Teams 等当前窗口列表）
- 底部 Command Dock：Ask（高亮主按钮）/ Summarize / Search / Handoff / Optimize + 当前范围 + 证据统计

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增证据缩略图卡片样式 `.flow-evidence-thumb`
- 新增证据节点连线样式 `.flow-evidence-connector`
- 新增窗口列表浮层样式 `.flow-window-panel`

---

### Phase 3：#04 时间线 · Forensic Lanes

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 替换 `flow-page-header flow-timeline-hero` 为设计图样式：左侧日期导航（`< 2026-06-17 今天 >`）+ 时间范围选择 + "工作时间(09:00-18:30)" 筛选 + "时间机器" 按钮 + 搜索栏
- 增加筛选条：时间/证据类型/应用/质检状态 筛选 tag
- 多轨泳道 `flow-forensic-lanes` 增加截图缩略图卡片（替换纯文本事件卡）
- 各轨道标题左对齐 + 总数 + 图标（截图/OCR文本/剪贴板/VS Code/Chrome/企微）
- 截图泳道卡片增加缩略图预览 + 绿色勾选状态 + 警告三角标记
- OCR泳道卡片增加可信度百分比
- 应用泳道（VS Code/Chrome/企微）增加时间段条形图
- 右侧 Inspector 重构为"证据检查器"：已选择数量 → 截图大图 + OCR 文本 + 执行时间 → "查看完整文本" → 质检状态（清晰度/完整性/截切 通过/适中/正常）→ 证据信息（来源应用/窗口标题/分辨率/文件大小/捕获时间）→ 操作按钮（打开证据/加入复盘/补跑OCR）
- 底部增加密度图 minimap（24h 小波形）
- 底部 Command Dock：已选择 N 条 + 补跑/重跑OCR+质检 / 加入复盘 / 导出所选 / 复制链接 / 标记敏感 + 更多操作

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增截图缩略图泳道样式 `.flow-forensic-thumb`
- 新增时间段条形图样式 `.flow-forensic-app-bar`
- 新增密度图样式 `.flow-density-map`
- 新增证据检查器面板子样式

---

### Phase 4：#07 洞察 · Pattern Radar

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 替换 `flow-page-header` 为设计图样式：日期导航 + 时间范围 + "本地归纳 / AI 归纳" 切换按钮组 + 归纳进度百分比 + "刷新/视图/筛选/..." 工具按钮
- 雷达图节点增加分类颜色+图标：自动化机会(蓝闪电)、知识沉淀(紫文件)、重复流程(中心齿轮)、待复盘(绿搜索)、风险信号(红警告)
- 节点之间增加 SVG 连线（实线=强关联、虚线=弱关联、点线=潜在关联），节点大小反映证据链数量
- 各节点增加悬停工具提示（置信度/证据链数量/可自动化/节省时间/时间范围）
- 中心节点增加右键菜单：交给代理/生成自动化/生成检查清单/加入草稿
- 右侧 Inspector 重构："洞察详情"标题 → 高价值/中价值标签 → AI 归纳原因 → 关键证据链（最近5条，带时间+版本+证据数）→ 建议动作（4个卡片网格）→ 隐私与边界 → 关联信息
- 底部添加图例条（连线类型 + 置信度圈大小 + 时间范围标注）
- 底部 Command Dock：运行AI归纳 / 生成每日复盘 / 创建任务包 / 导出洞察报告 / 更新索引 + 洞察库统计

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增雷达连线 SVG 样式 `.flow-radar-link`
- 新增分类图标/颜色 token
- 新增图例条样式 `.flow-radar-legend`

---

### Phase 5：#11 草稿 · Timeline-Synced Brief

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 替换 `flow-page-header` 为设计图样式：`+ 新建草稿` 按钮 + 列表/网格切换 + 日期导航 + 时间范围 + 工作时段 + "自动整理中·本地模型" 状态
- 左侧草稿库增加：条目卡片（标题 + 自动生成时间 + "时间同步" 红标 + 证据数/引用数/收藏 + 三点菜单）
- 中间增加"证据时间线"缩略图条（与段落编号联动，点击缩略图跳到对应段落，图例: 截图/OCR/剪贴板/窗口）
- 中间草稿文档区增加：版本标签 `v2·AI润色版` + 字数/证据条数 + 段落右侧编号气泡（点击展开证据）
- 右侧 Inspector 重构为 Agent Inspector：来源段落（编号列表，点击展开证据）→ 证据详情（截图 + OCR文本 + 时间 + 分辨率）→ AI 润色状态（模型/风格/内容优化✓/结构优化✓/术语统一✓/润色完成时间）→ 外发与使用（隐私级别/敏感字段/预览外发版/确认外发）
- 底部 Command Dock：生成日报(高亮) / 生成复盘 / 生成知识 / 生成清单 / AI润色 + 导出Markdown / 导出PDF / 保存知识

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增段落编号气泡样式 `.flow-draft-paragraph-badge`
- 新增 AI 润色状态面板样式 `.flow-polish-status`

---

### Phase 6：#15 资产 · Agent Package Hub

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 替换 `flow-page-header` 为设计图样式：面包屑 `资产库 > 代理任务 > 分析 Q2 客户流失原因并给出改进建议` + 版本标签 `v1.2` + "可交给Codex" 标签 + 搜索栏
- 任务包中间区增加结构化编号分区：`#1 目标(Goal)` → `#2 背景(Context)` → `#3 证据(Evidence)` (截图缩略图行 + "添加证据") → `#4 边界(Boundaries)` → `#5 验收标准(Acceptance)`
- 左侧资产库增加：代理任务列表 (15) + 工作流 (7) + 检查清单 (8) + Skill (6)，每项带版本号 + 日期 + "已保存"/"可交给XXX" 标签
- 右侧 Inspector 重构为 Agent Inspector：就绪度评估环形图 (82/100) + 分项评分 → 缺失证据列表（关键/重要标签）→ 风险边界面板（数据敏感度/外部访问/联系人触达）→ 元数据 → 操作历史时间线
- 底部候选产物增加 "预览工作流 →" / "预览清单 →" / "预览Skill →" + "生成更多资产" 按钮
- 底部 Command Dock：复制任务包(Markdown包) / 生成工作流 / 生成检查清单 / 保存为Skill / 安装前确认 + 导出 Zip/JSON

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增就绪度环形图样式 `.flow-readiness-ring`
- 新增结构化分区编号样式 `.flow-package-section-numbered`

---

### Phase 7：#16 规则 · Pipeline Control Room

#### [MODIFY] [WorkMemoryCenter.vue](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/components/workmemory/WorkMemoryCenter.vue)
- 替换 `flow-page-header` 为设计图样式：日期选择 + 时间范围 + "本地模式·采集中" 状态标签 + 搜索栏 + 通知铃铛
- 左侧重构为"捕获来源"列表：屏幕截图(今日N条·运行中) / OCR文本 / 剪贴板 / 应用上下文 / 窗口标题，各带计数和运行状态灯 + 手动补记区 + 数据导入区
- 中间上部增加**采集流水线可视化**（横向流程图）：捕获来源 →(箭头)→ 排除规则(运行中·N条/日·阻止N条) →→ 质检(通过率N%) →→ OCR(完成N条·待处理N条) →→ 语义索引(已索引N条·队列N条) →→ 导出/资产(已导出N条)
- 中间下部替换为**排除规则表格**（分 tab：应用进程/窗口关键词/路径片段/内容正则/敏感凭据形态），每行：规则名 / 匹配条件 / 命中次数(今日) / 动作 / 作用范围 / 状态开关 / 优先级 / 编辑删除
- 右侧 Inspector 重构为 Agent Inspector：影响预览（阻止数量/减少体积/节省索引 + vs上周对比）→ 受影响应用 Top 5 → 最近命中规则（规则名+路径+阻止采集 tag）→ 敏感凭据模式（已启用 + 高敏感 + 管理按钮）→ 本地边界（所有规则均在本地生效·不上传云端·本地优先·数据不出本机）
- 底部 Command Dock：手动补记 / 导入材料 / 保存排除规则(高亮) / 刷新索引 / 导出数据 + 更多操作

#### [MODIFY] [style.css](file:///p:/workspace/glwlg/app/Ariadne/frontend/src/style.css)
- 新增管线流程图样式 `.flow-pipeline-visual`（箭头连接器、阶段卡片）
- 新增排除规则表格样式 `.flow-rules-table`
- 新增影响预览卡片样式 `.flow-impact-preview`

---

## Open Questions

> [!IMPORTANT]
> **1. Command Dock 按钮功能对接**
> 设计图中有些 Command Dock 按钮（如 Summarize、Handoff、Optimize）目前没有对应的 store 方法或后端 API。是否：
> - A) 先添加 UI 按钮，暂时 disabled + tooltip "即将推出"
> - B) 只添加已有后端支持的按钮

> [!IMPORTANT]
> **2. 管线可视化流程图的实时数据**
> #16 设计图中采集管线各阶段显示实时统计（今日捕获条数、阻止条数、通过率、队列数等）。当前 store 中有哪些统计字段可用？是否需要新增后端 API 获取管线统计？
> - A) 先用 mock 数据展示 UI
> - B) 暂不实现，只做管线可视化样式

> [!IMPORTANT]
> **3. 就绪度评估环形图**
> #15 设计图有 82/100 的就绪度评估。是否需要实现评分算法（根据证据/边界/验收完整度计算），还是简单根据字段填充率估算？

> [!IMPORTANT]
> **4. 改动范围确认**
> WorkMemoryCenter.vue 已有 3287 行。这次改动会显著增加其体积。是否：
> - A) 继续在单文件中改动（保持当前架构一致性）
> - B) 趁此机会拆分为子组件（FlowPage.vue / TimelinePage.vue / InsightsPage.vue / DraftsPage.vue / AssetsPage.vue / RulesPage.vue + CommandDock.vue + AgentInspector.vue），但这会增加工作量

## Verification Plan

### Manual Verification
- 逐页面与设计图进行像素级视觉对比
- 验证响应式布局在 1280x820（默认）和 1040x640（最小）下正常
- 验证暗色模式下所有新增样式正确应用
- 验证所有已有功能（问答/时间线筛选/洞察归纳/草稿生成/资产操作/规则保存）保持正常
