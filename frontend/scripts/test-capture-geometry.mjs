import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const source = readFileSync(new URL('../src/lib/captureGeometry.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
    verbatimModuleSyntax: false,
  },
  reportDiagnostics: true,
})

if (compiled.diagnostics?.length) {
  const errors = compiled.diagnostics
    .filter((diagnostic) => diagnostic.category === ts.DiagnosticCategory.Error)
    .map((diagnostic) => ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n'))
  assert.deepEqual(errors, [])
}

const moduleUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString('base64')}`
const {
  mapVisualSelectionToPinPosition,
  mapVisualSelectionToSourcePixels,
} = await import(moduleUrl)

assert.deepEqual(
  mapVisualSelectionToSourcePixels(
    { left: 50, top: 20, width: 30, height: 15 },
    { width: 200, height: 160 },
    { left: 0, top: 0, width: 100, height: 80 },
    { left: 0, top: 0, width: 100, height: 80 },
  ),
  { x: 100, y: 40, width: 60, height: 30 },
)

assert.deepEqual(
  mapVisualSelectionToSourcePixels(
    { left: 61, top: 37, width: 23, height: 17 },
    { width: 200, height: 100 },
    { left: 50, top: 25, width: 100, height: 50 },
    { left: 0, top: 0, width: 200, height: 100 },
  ),
  { x: 22, y: 24, width: 46, height: 34 },
)

assert.deepEqual(
  mapVisualSelectionToSourcePixels(
    { left: 10.2, top: 3.4, width: 4.1, height: 2.2 },
    { width: 200, height: 120 },
    { left: 0, top: 0, width: 100, height: 60 },
    { left: 0, top: 0, width: 100, height: 60 },
  ),
  { x: 20, y: 6, width: 9, height: 6 },
)

assert.deepEqual(
  mapVisualSelectionToPinPosition(
    { left: 50, top: 40 },
    { x: 300, y: 150, width: 100, height: 80 },
    { left: 0, top: 0, width: 100, height: 80 },
    { left: 0, top: 0, width: 100, height: 80 },
  ),
  { x: 335, y: 175 },
)

assert.deepEqual(
  mapVisualSelectionToPinPosition(
    { left: 61, top: 37 },
    { x: 300, y: 150, width: 100, height: 80 },
    { left: 50, top: 25, width: 100, height: 50 },
    { left: 0, top: 0, width: 200, height: 100 },
  ),
  { x: 296, y: 154 },
)

console.log('capture geometry tests passed')
