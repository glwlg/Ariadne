import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { defaultJsonNodeExpanded } from '../src/lib/apiJsonExpansion.ts'

assert.equal(defaultJsonNodeExpanded({ overview: { total: 1 } }), true, '对象字段默认展开')
assert.equal(defaultJsonNodeExpanded({ nested: { branch: { leaf: true } } }, 3), true, '深层对象字段默认展开')
assert.equal(defaultJsonNodeExpanded([{ id: 1 }]), true, '数组字段默认展开')
assert.equal(defaultJsonNodeExpanded('plain text'), false, '标量不需要展开状态')

const responsePanel = readFileSync(new URL('../src/components/apitesting/ApiResponsePanel.vue', import.meta.url), 'utf8')
assert.match(responsePanel, /LARGE_JSON_TEXT_BYTES/, 'PERF-07: ApiResponsePanel 应定义大 JSON 文本阈值')
assert.match(responsePanel, /LARGE_JSON_NODE_LIMIT/, 'PERF-07: ApiResponsePanel 应定义大 JSON 节点数阈值')
assert.match(responsePanel, /仍以树形展开 \(节点较多\)/, 'PERF-07: 应提供手动切换树形展开的按钮')

console.log('api JSON expansion tests passed')
