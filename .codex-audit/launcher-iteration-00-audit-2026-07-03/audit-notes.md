# Ariadne 启动器首轮产品审查

日期：2026-07-03
模型要求：gpt-5.5 xhigh，service_tier=priority
审查角色：首轮审查 subagent，只审查不改代码
本轮修改记录：无，本轮是首轮审查

## 结论

未达到“审查不出什么问题来”。没有发现 P0，但仍有 2 个 P1 和 3 个 P2。主要问题集中在：用户可见文案暴露实现/开发态信息、工作流类高风险动作的确认链路在启动器里无法闭环、更多菜单可访问名称缺失、小窗口约束和结果规模风险。

## 审查范围和证据

- 代码入口：`frontend/src/components/launcher/AriadneLauncher.vue`、`frontend/src/stores/launcher.ts`、`frontend/src/lib/launcherGeometry.ts`、`frontend/src/style.css`、`frontend/src/services/ariadneApi.ts`、`internal/search/service.go`、`internal/workflows/service.go`、`internal/launchers/service.go`。
- 真实页面：`http://127.0.0.1:5175/?view=launcher`。
- 5173 已被其他页面占用，按前端 dev server 启动到 5175：`pnpm --dir frontend dev --host 127.0.0.1 --port 5175`。
- 浏览器动作：刷新页面、输入搜索、切换选中、打开更多菜单、检查命令参数区、用键盘 ArrowDown/Enter 验证插件命令填入、检查 480x640 小窗口和 860x468 启动器窗口尺寸。
- 控制台：本轮浏览器 `error/warn` 日志为空；未出现原生 JS dialog。
- 未点击会启动外部程序、真实截图覆盖层、删除或昂贵副作用的动作。

## 截图清单

- `01-launcher-collapsed.png`：折叠态首屏。
- `02-search-results.png`：搜索“网关”后的结果、预览、状态条、主操作。
- `03-command-builder.png`：搜索“生成器”后的插件命令参数区。
- `04-more-menu.png`：结果更多菜单展开态。
- `05-keyboard-enter-generated.png`：键盘 ArrowDown 选中插件触发器、Enter 填入 `uuid` 后生成结果。
- `06-small-window.png`：480x640 小窗口约束。
- `07-wails-height-more-menu.png`：860x468 启动器窗口尺寸下更多菜单展开态。

## P0

未发现 P0。

## P1

### P1-1 用户可见文案暴露开发态和实现协议，违反产品文案约束

证据：
- 浏览器截图 `03-command-builder.png`、`05-keyboard-enter-generated.png` 可见插件预览里出现“preview actions”“开发态 fallback”等实现/开发态词。
- `frontend/src/services/ariadneApi.ts:158` 把 `开发态 fallback preview action` 放进预览 meta。
- `internal/search/service.go:327` 暴露“动作来源 / 文件结果默认动作”。
- `internal/search/service.go:360-363` 暴露“preview actions”和 `plugin_trigger -> plugin_result`。

影响：
- 这是启动器主阅读面，不是技术详情区。用户搜索后看到的是实现协议而不是对象、状态、结果和下一步动作，直接降低可信度和完成感。
- 该问题不只存在于浏览器 fallback，后端 seed 结果也含同类文案，桌面运行时也可能露出。

下一轮建议：
- 把启动器预览里的实现词改为产品语义，例如“生成后可直接复制结果”“命令示例”“默认操作”等。
- 若确需保留协议/实现信息，放到开发日志或显式技术详情中，不放在默认预览主面。

### P1-2 工作流高风险确认在启动器里无法闭环

证据：
- `internal/workflows/service.go:403-410` 高风险工作流会返回 `RequiresConfirmation: true`。
- `frontend/src/stores/launcher.ts:240-264` 的 `run_workflow` 分支直接调用 `runWorkflow({ workflowId, input, clipboardText })`，只把失败消息写入 `lastAction`。
- 通用确认状态只在 `frontend/src/stores/launcher.ts:286-300` 的 `executeAriadneAction` 分支处理；`run_workflow` 分支没有设置 `pendingConfirmationKey`，也没有在第二次点击时传 `confirmed: true`。

影响：
- 风险工作流在启动器里会提示需要确认，但用户再次点击同一动作仍然发送未确认请求，无法继续。
- 这会阻断“搜索 -> 运行工作流”的核心便捷路径，也让状态条的确认提示变成不可操作反馈。

下一轮建议：
- 让 `run_workflow` 复用启动器统一确认机制，或为该分支补齐同等的 pending confirmation 状态。
- 确认态需要保留风险原因，并让第二次点击同一主操作明确传入 `confirmed: true`。

## P2

### P2-1 更多菜单触发按钮没有可访问名称

证据：
- 浏览器 DOM snapshot 中更多按钮显示为 `button:`，没有名称。
- `04-more-menu.png` 展示该按钮只有三点图标。
- `frontend/src/components/launcher/AriadneLauncher.vue:456-459` 的 `AriButton` 只包含 `MoreHorizontal`，未提供 `aria-label` 或等价文本。

影响：
- 屏幕阅读器无法说明该按钮用途。
- 键盘用户和自动化测试也缺少稳定语义定位。

下一轮建议：
- 给更多菜单按钮补产品化可访问名称，例如“更多操作”。

### P2-2 480px 小窗口下结果行和状态条出现裁切/拥挤

证据：
- `06-small-window.png` 中结果行右侧主操作按钮被裁切，底部状态条和操作区拥挤换行。
- 浏览器测得 480x640 下 `.palette-shell` 宽 456px，但 `.palette-results` 实际宽 519px，超出 shell；预览区被隐藏。
- 相关样式在 `frontend/src/style.css:13150-13179` 只把结果区改成 1 列并隐藏预览，没有约束结果行按钮和状态条。

影响：
- 在小窗口或窄屏验证时，启动器的主操作和状态信息不稳定。
- 如果桌面运行时不支持小于当前设计宽度，也应由窗口最小宽度或布局规则明确约束，而不是自然裁切。

下一轮建议：
- 二选一：明确设置并验证启动器最小支持宽度；或补齐小窗口布局，让结果主操作、状态条、更多菜单在 480px 宽度内不裁切。

### P2-3 结果列表没有前端虚拟化或顶层结果上限，启动项 provider 可返回无界结果

证据：
- `frontend/src/components/launcher/AriadneLauncher.vue:307-316` 对 `launcher.results` 全量 `v-for` 渲染。
- `internal/search/service.go:102-112` 排序后直接返回全部聚合结果，没有顶层 cap。
- `internal/launchers/service.go:63-80` 对所有启用启动项命中后全部 append，没有 provider 内上限。

影响：
- 当前 seed 和常见 provider 看起来很快，但用户启动项、文件、历史数据增长后，列表渲染、滚动和选中同步会变成输入时的高频成本。
- 搜索请求虽然有取消和串行号防竞态，但旧请求的渲染结果规模没有保护。

下一轮建议：
- 后端聚合层或 provider 层限制默认返回数量，并把总命中数作为状态展示。
- 如果目标是展示大量结果，前端需要虚拟列表和稳定行高，而不是全量 DOM。

## 已确认表现良好的点

- 刷新 `/?view=launcher` 后搜索框自动聚焦，折叠态首屏能在几秒内理解为启动器/命令搜索入口。
- 输入搜索后结果区、预览区、状态条和主操作区层级整体清楚。
- 搜索请求有取消和竞态保护：`activeSearchRequest.cancel()`、`searchRequestSerial`、查询值校验组合能防旧结果覆盖新输入。
- 键盘 ArrowDown 可移动选中；在插件触发器上按 Enter 会填入命令而不是直接执行外部程序。
- 更多菜单在 860x468 启动器窗口尺寸下未被窗口边界裁切。
- 浏览器控制台无 error/warn，未出现原生 alert/confirm/prompt。

## 验证命令

- `pnpm --dir frontend build`：通过。构建输出包含 Rolldown 对 `@vueuse/core` 的 `INVALID_ANNOTATION` 警告；非本轮新增，但仍建议后续统一处理依赖构建噪音。
- `git diff --check`：退出码 0。仅提示 3 个既有 unrelated 文件下次 Git 触碰会 LF 转 CRLF：`cmd/capturesmoke/main_windows.go`、`internal/captureoverlay/service.go`、`internal/captureoverlay/service_test.go`。

## 下一轮修改建议

1. 先修 P1-1：清掉启动器默认可见面的开发态/协议文案，统一改为产品语义。
2. 再修 P1-2：把 `run_workflow` 接入统一二次确认链路，并验证高风险工作流第二次点击能继续。
3. 修 P2-1：给更多菜单图标按钮补可访问名称。
4. 修 P2-2：确定启动器小窗口策略，并用浏览器验证 480x640 和 860x468。
5. 修 P2-3：为结果返回和渲染规模加明确边界，至少覆盖自定义启动项无界增长。

## 是否达到“审查不出什么问题来”

否。当前仍有 2 个 P1 和 3 个 P2，建议进入下一轮修改。
