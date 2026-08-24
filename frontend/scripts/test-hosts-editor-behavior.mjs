import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const component = await readFile(new URL('../src/components/hosts/HostsCenter.vue', import.meta.url), 'utf8')
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

console.log('hosts editor behavior tests passed')
