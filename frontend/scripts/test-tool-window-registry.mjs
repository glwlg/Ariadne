import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import ts from 'typescript'

const url = new URL('../src/lib/toolWindows.ts', import.meta.url)
const source = ts.transpileModule(readFileSync(url, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
}).outputText
const exports = {}
new Function('require', 'exports', source)(createRequire(url), exports)
const { toolViews, isToolView, toolViewForCommand } = exports
for (const [view, definition] of Object.entries(toolViews)) {
  assert.equal(isToolView(view), true)
  assert.equal(toolViewForCommand(definition.command), view)
}
for (const view of ['launcher', 'toString', '__proto__', 'constructor', 'unknown']) {
  assert.equal(isToolView(view), false)
}
assert.equal(toolViewForCommand('open_capture_overlay'), undefined)
assert.equal(toolViewForCommand('unknown'), undefined)
assert.equal(toolViews.capture.title, 'Ariadne - 截图历史')
assert.equal(toolViews.hosts.width, 1360)
assert.equal(toolViews.settings.minWidth, 980)
console.log('Tool window registry checks passed.')
