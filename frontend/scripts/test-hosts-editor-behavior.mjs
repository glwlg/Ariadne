import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const component = await readFile(new URL('../src/components/hosts/HostsCenter.vue', import.meta.url), 'utf8')
const editorPane = await readFile(new URL('../src/components/hosts/HostsEditorPane.vue', import.meta.url), 'utf8')
const store = await readFile(new URL('../src/stores/hosts.ts', import.meta.url), 'utf8')

const editorColumn = component.match(/<section class="hosts-editor-column[\s\S]*?<\/section>/)?.[0] ?? ''
const inspector = component.match(/<section class="hosts-inspector-section">[\s\S]*?<\/section>/)?.[0] ?? ''

assert.ok(editorColumn.includes('<HostsEditorPane'), '中心列必须保留 Hosts 编辑器')
assert.ok(!editorColumn.includes('方案类型'), '方案类型不能常驻中心列')
assert.ok(!editorColumn.includes('远程 URL'), '远程 URL 不能常驻中心列')

assert.ok(inspector.includes('aria-label="方案名称"'), '右侧方案信息必须提供名称编辑控件')
assert.ok(inspector.includes("!hosts.draft.system"), '系统方案必须保持名称只读')
assert.ok(inspector.includes("hosts.updateDraft({ title:"), '名称输入必须写入 draft.title')
assert.ok(inspector.includes('aria-label="方案类型"'), '右侧方案信息必须保留类型选择')
assert.ok(inspector.includes("hosts.draft.type === 'remote'"), '远程设置必须只在远程方案显示')
assert.ok(inspector.includes('aria-label="远程 URL"'), '远程方案必须保留 URL 编辑')
assert.ok(component.includes('@click="hosts.fetchRemote(hosts.draft)"'), '远程方案必须保留拉取操作')

assert.match(store, /status\.value = await upsertHostsProfile\(draft\.value\)[\s\S]*status\.value = await fetchRemoteHosts\(profile\.id\)/, '拉取前必须持久化当前草稿')

// PERF-05: virtualized line-number window without split('\n') in HostsEditorPane
assert.match(editorPane, /countHostsLines\(props\.content\)/, 'PERF-05: HostsEditorPane 应使用 countHostsLines 计数')
assert.doesNotMatch(editorPane, /\.split\(['"]\\n['"]\)/, 'PERF-05: HostsEditorPane 不得调用 split(\\n) 分配行数组')
assert.match(editorPane, /v-for="line in visibleRange\.lines"/, 'PERF-05: 行号槽必须使用虚拟可见窗口渲染')
assert.match(editorPane, /hosts-line-spacer/, 'PERF-05: 虚拟行号窗口必须通过 spacer 维持滚动高度')
assert.match(component, /window\.setTimeout\([\s\S]*refreshDraftMetrics\(content\)[\s\S]*180\)/, 'PERF-05: HostsCenter 应防抖计算 draftStats 和 draftBytes')

// UX-02 & UX-03: diff/finalContent preview & rollback controls
assert.match(component, /hosts\.preview\.diffText/, 'UX-02: 必须渲染 hosts.preview.diffText')
assert.match(component, /hosts\.preview\.finalContent/, 'UX-02: 必须支持 hosts.preview.finalContent 预览')
assert.match(component, /hosts\.preview\.addedLines/, 'UX-02: 必须展示新增行数')
assert.match(component, /hosts\.preview\.removedLines/, 'UX-02: 必须展示移除行数')
assert.match(component, /hosts\.preview\.enabledProfiles/, 'UX-02: 必须展示纳入预览的方案列表')
assert.match(component, /v-for="conflict in hosts\.preview\.conflicts"/, 'UX-02: 必须展示完整冲突列表而非截断')
assert.match(component, /hosts\.rollbackHosts\(\)/, 'UX-03: 必须绑定回滚操作')
assert.match(component, /hosts\.canRollback/, 'UX-03: 必须检查 canRollback 状态')
assert.match(component, /hosts\.lastBackupAt/, 'UX-03: 必须展示最近备份时间')

// UI-11, UI-12, UI-19, UI-24: error/loading/empty/armed/switch accessibility
assert.match(component, /'is-danger': hosts\.feedbackTone === 'danger'/, 'UI-11: 错误反馈必须绑定 is-danger')
assert.match(component, /tool-loading-state/, 'UI-12: 必须包含加载占位状态')
assert.match(component, /tool-empty-state/, 'UI-12: 必须包含空状态占位')
assert.match(component, /'is-armed-danger': hosts\.rollbackArmed/, 'UI-19: 回滚二次确认必须绑定 is-armed-danger')
assert.match(component, /'is-armed-danger': hosts\.applyArmed/, 'UI-19: 写入二次确认必须绑定 is-armed-danger')
assert.match(component, /role="switch"[\s\S]*:aria-checked="profile\.enabled"[\s\S]*hosts-switch-btn/, 'UI-24: 方案启用开关必须是独立可访问按钮')

console.log('hosts editor behavior tests passed')
