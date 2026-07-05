# Ariadne 启动器第 1 轮复审

日期：2026-07-03
角色：第 1 轮复审 subagent，独立产品审查，只审查不改代码
模型要求：gpt-5.5 xhigh，service_tier=priority
目标 URL：`http://127.0.0.1:5175/?view=launcher`
本轮修改记录：`.codex-audit/launcher-iteration-01-modification-2026-07-03/audit-fixes.md`
上轮审查：`.codex-audit/launcher-iteration-00-audit-2026-07-03/audit-notes.md`

## 结论

达到“审查不出什么问题来”。本轮复验上一轮 2 个 P1 和 3 个 P2 均已落地，未发现新的有意义 P0/P1/P2。

可接受残余风险：

- 没有在桌面运行时真实执行高风险工作流；`run_workflow` 二次确认以启动器前端链路、后端工作流测试和浏览器非破坏性状态验证为证据。
- 浏览器环境控制台仍有 Wails 开发环境提示 warning，非本轮新增，不影响启动器审查结论。
- 源码中仍有 `frontend/src/services/workflowApi.ts` 的工作流导出 fallback 文案包含“开发态 fallback”，但该文案不在启动器默认可见面，也不属于本轮启动器目标流程。

## 审查范围和证据

- 复审文档：读取本轮修改记录和上一轮审查记录，逐项复验 P1/P2。
- 代码证据：`frontend/src/components/launcher/AriadneLauncher.vue`、`frontend/src/stores/launcher.ts`、`frontend/src/services/ariadneApi.ts`、`frontend/src/services/workflowApi.ts`、`frontend/src/data/seed.ts`、`internal/search/service.go`、`internal/launchers/service.go`、`internal/workflows/service.go` 及相关测试。
- 浏览器动作：刷新目标页面，搜索“网关”，展开更多菜单，搜索“生成器”，输入 `uuid 3`，检查 DOM、可访问名称、控制台、布局溢出和截图。
- 响应式尺寸：使用内置 browser 验证 480x640 和 860x468；未使用 Playwright CLI。
- 未执行动作：没有点击真实启动外部程序、删除、覆盖或昂贵副作用按钮。

## 截图清单

截图目录：`.codex-audit/launcher-iteration-01-audit-2026-07-03/`

- `01-after-refresh-first-screen.png`：刷新后的折叠态首屏。
- `02-search-gateway-results.png`：搜索“网关”后的结果、预览、状态条。
- `03-more-menu-open.png`：更多菜单展开态。
- `04-command-builder-generator.png`：搜索“生成器”后的插件命令参数区。
- `05-plugin-results-uuid-3.png`：`uuid 3` 生成 3 条可复制结果。
- `06-480x640-gateway.png`：480x640 小窗口布局。
- `07-860x468-more-menu.png`：860x468 下更多菜单展开态。

## P0

未发现。

## P1

未发现。

## P2

未发现。

## 已确认落地的修复

### 上轮 P1-1：默认可见文案清理

已落地。浏览器默认可见面和插件结果面均未再出现 `preview actions`、`开发态 fallback`、`动作来源`、`文件结果默认动作`、`plugin_trigger -> plugin_result`、`plugin_trigger → plugin_result`、`协议` 等上一轮指出的实现/开发态词。

证据：

- `02-search-gateway-results.png`、`04-command-builder-generator.png`、`05-plugin-results-uuid-3.png` 可见预览文案改为“默认操作”“结果动作”“生成后复制”“复制结果”等产品语义。
- DOM 检查：搜索“网关”、“生成器”和 `uuid 3` 时 banned terms 均为 `[]`。
- 测试证据：`internal/search/service_test.go` 增加 `TestSeedPreviewCopyUsesProductLanguage`，覆盖种子插件预览文案。

### 上轮 P1-2：`run_workflow` 二次确认链路

已落地。`frontend/src/stores/launcher.ts` 的 `run_workflow` 分支现在会：

- 计算包含 `workflowId` 和 `input` 的确认 key；
- 首次风险响应设置 `pendingConfirmationKey`；
- 第二次点击同一动作时向 `runWorkflow` 传 `confirmed: true`；
- 风险确认期间保留底部操作区，用户可再次点击同一工作流动作。

证据：

- `frontend/src/stores/launcher.ts:246-286` 显示 `runWorkflow({ workflowId, input, clipboardText, confirmed })` 和 `setPendingConfirmation(...)` 已接入。
- `frontend/src/stores/launcher.ts:458-468` 的确认 key 已纳入 `workflowId`、`input`。
- `frontend/src/components/launcher/AriadneLauncher.vue:440-443` 在 `lastAction.requiresConfirmation` 时继续显示操作区。
- `go test ./internal/search ./internal/launchers ./internal/workflows` 通过；`internal/workflows/service_test.go` 已覆盖未确认拦截和确认后执行。

### 上轮 P2-1：更多菜单可访问名称

已落地。更多菜单按钮具备稳定可访问名称和标题。

证据：

- DOM：`getByRole('button', { name: '更多操作' })` 唯一匹配，`title` 为“更多操作”。
- `03-more-menu-open.png`、`07-860x468-more-menu.png` 展示菜单可正常展开。
- `frontend/src/components/launcher/AriadneLauncher.vue:455-458` 增加 `aria-label="更多操作"` 和 `title="更多操作"`。

### 上轮 P2-2：480x640 和 860x468 布局

已落地。两个指定尺寸未见横向溢出、菜单裁切或主操作裁切。

证据：

- 480x640：`documentOverflowX=false`，`.palette-results` 宽 460px，`.palette-shell` 宽 462px，`rowOverflowCount=0`，状态条和操作区在 460px 内换行展示。
- 860x468：`documentOverflowX=false`，菜单 bounds 为 left 656 / right 834 / top 321 / bottom 406，`menuWithinViewport=true`，`rowOverflowCount=0`。
- 截图：`06-480x640-gateway.png`、`07-860x468-more-menu.png`。

### 上轮 P2-3：结果上限与计数

已落地。搜索聚合、provider 和自定义启动项都有明确上限，前端计数可展示“显示 N / M 项”。

证据：

- `internal/search/service.go`：provider 结果上限为 60，聚合展示上限为 40，并返回 `TotalResults`。
- `internal/launchers/service.go`：自定义启动项搜索结果上限为 `launcherSearchResultLimit`。
- `frontend/src/stores/launcher.ts:37-43`：`resultCountLabel` 在总数大于展示数时显示 `显示 ${shown} / ${total} 项`。
- `internal/search/service_test.go:423-442` 覆盖 provider 和聚合上限；`internal/launchers/service_test.go:130-149` 覆盖自定义启动项上限。

## 浏览器检查结果

- 刷新后首屏搜索框自动聚焦，折叠态可读。
- 搜索“网关”显示 1 项，预览层级清晰，更多菜单可展开并位于视口内。
- 搜索“生成器”显示插件命令参数区，默认文案是产品语义。
- `uuid 3` 返回 3 条结果，计数为“3 项”，每条主操作为“复制结果”。
- 控制台未见业务 error；仅有 Wails 浏览器开发环境 warning。
- 未出现原生 `alert` / `confirm` / `prompt`。

## 验证命令

- `pnpm --dir frontend build`：独立复跑，通过；仍有既有 `@vueuse/core` Rolldown `INVALID_ANNOTATION` warning，退出码 0。
- `go test ./internal/search ./internal/launchers ./internal/workflows`：独立复跑，通过，结果为 cached。
- `git diff --check`：独立复跑，通过，退出码 0；仅输出 LF/CRLF 工作区提示。

## 是否达到“审查不出什么问题来”

是。上一轮 P1/P2 均已复验通过，本轮未发现新的有意义 P0/P1/P2。无需进入下一轮修改。
