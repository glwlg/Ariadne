import assert from 'node:assert/strict'
import { defaultJsonNodeExpanded } from '../src/lib/apiJsonExpansion.ts'

assert.equal(defaultJsonNodeExpanded({ overview: { total: 1 } }), true, '对象字段默认展开')
assert.equal(defaultJsonNodeExpanded({ nested: { branch: { leaf: true } } }, 3), true, '深层对象字段默认展开')
assert.equal(defaultJsonNodeExpanded([{ id: 1 }]), true, '数组字段默认展开')
assert.equal(defaultJsonNodeExpanded('plain text'), false, '标量不需要展开状态')

console.log('api JSON expansion tests passed')
