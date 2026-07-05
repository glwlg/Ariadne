# Ariadne 启动器第 1 轮修改记录

日期：2026-07-03
角色：第 1 轮修改 subagent，直接改代码并写修改记录
模型要求：gpt-5.5 xhigh，service_tier=priority
目标 URL：`http://127.0.0.1:5175/?view=launcher`
dev server：5175 已在运行，本轮未另起服务
上一轮审查：`.codex-audit/launcher-iteration-00-audit-2026-07-03/audit-notes.md`

## 本轮处理的问题

- P1-1：清理启动器默认可见文案中的 `preview actions`、`开发态 fallback`、`动作来源`、`plugin_trigger -> plugin_result` 等实现/开发态词，改为“默认操作”“结果动作”“生成后复制”等产品语义。
- P1-2：`run_workflow` 接入启动器已有二次确认状态。首次返回 `requiresConfirmation` 时保留待确认 key，第二次点击同一工作流动作会传 `confirmed: true`。
- P2-1：更多菜单图标按钮增加 `aria-label="更多操作"` 和 `title="更多操作"`。
- P2-2：480px 小窗口采用支持策略，结果区改为单列且主操作下移，状态条和操作区可换行，不再横向裁切。
- P2-3：增加结果规模边界：搜索聚合最多展示 40 项，单 provider 最多纳入 60 项，自定义启动项 provider 最多返回 20 项；前端显示“显示 N / M 项”。

## 修改文件

- `frontend/src/components/launcher/AriadneLauncher.vue`
- `frontend/src/stores/launcher.ts`
- `frontend/src/services/ariadneApi.ts`
- `frontend/src/services/workflowApi.ts`
- `frontend/src/data/seed.ts`
- `frontend/src/types/ariadne.ts`
- `frontend/src/style.css`
- `internal/contracts/types.go`
- `internal/search/service.go`
- `internal/search/service_test.go`
- `internal/launchers/service.go`
- `internal/launchers/service_test.go`

未触碰用户明确标注的 unrelated 文件：`cmd/capturesmoke/main_windows.go`、`internal/captureoverlay/service.go`、`internal/captureoverlay/service_test.go`。

## 修改前后行为

- 修改前：UUID 插件预览和文件种子结果会暴露实现协议、开发态 fallback 或动作来源说明。修改后：默认预览只展示用户可理解的对象、状态和动作。
- 修改前：高风险工作流首次点击后只显示“需要确认”，再次点击仍不传确认参数。修改后：同一动作在确认窗口期内再次点击会携带 `confirmed: true`。
- 修改前：更多菜单按钮只有三点图标。修改后：按钮具备稳定可访问名称“更多操作”。
- 修改前：480x640 下结果行和状态条容易横向拥挤。修改后：结果行、状态条和操作区在 480px 内无横向溢出。
- 修改前：聚合结果和启动项 provider 缺少规模上限。修改后：后端返回和前端渲染都有明确边界，并在计数中体现展示数量。

## 验证命令和结果

- `go test ./internal/search ./internal/launchers ./internal/workflows`：通过。
- `pnpm --dir frontend build`：通过；仍有既有 `@vueuse/core` Rolldown `INVALID_ANNOTATION` 警告，退出码 0。
- `git diff --check`：通过，退出码 0。
- 文案扫描：`rg "preview actions|开发态 fallback preview action|动作来源|文件结果默认动作|plugin_trigger -> plugin_result|plugin_trigger → plugin_result|协议" ...` 对本轮启动器相关默认文案无命中。

## 浏览器证据

截图目录：`.codex-audit/launcher-iteration-01-modification-2026-07-03/`

- `01-collapsed.png`：折叠态。
- `02-search-results.png`：搜索“网关”的结果和状态条。
- `03-more-menu.png`：更多菜单展开态。
- `04-command-builder.png`：搜索“生成器”的插件命令参数区。
- `05-plugin-results.png`：`uuid 3` 生成 3 条可复制结果。
- `06-480x640-search.png`：480x640 小窗口搜索结果。
- `07-860x468-more-menu.png`：860x468 下更多菜单展开态。

浏览器检查结果：

- 折叠态、搜索结果、更多菜单、插件命令、`uuid 3` 插件结果均未命中上一轮指出的开发态/协议词。
- 更多菜单按钮 `aria-label` 为“更多操作”，按钮数量为 1。
- 480x640：`documentOverflowX=false`，结果行 `rowOverflowCount=0`，状态条和操作区宽度在 shell 内。
- 860x468：更多菜单 `withinViewport=true`，结果区和状态条无横向溢出。
- 控制台未见业务 error；仅有 Wails 浏览器开发环境提示 warning。
- 未出现原生 JS dialog。

## 未处理事项和残余风险

- 未点击会真实触发昂贵或破坏性副作用的动作。
- 高风险工作流二次确认使用前端代码链路检查和后端安全测试验证，未在真实桌面运行时执行高风险工作流。
- 当前仓库仍有本轮开始前已存在的 unrelated 脏文件，本轮未回滚、未暂存、未提交。
