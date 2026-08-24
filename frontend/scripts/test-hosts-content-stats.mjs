import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'

const moduleUrl = new URL('../src/lib/hostsContentStats.ts', import.meta.url)
const source = await readFile(moduleUrl, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const loaded = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`)

assert.deepEqual(loaded.calculateHostsContentStats(''), {
  totalLines: 0,
  validRules: 0,
  commentLines: 0,
  emptyLines: 0,
  duplicateRules: 0,
})

assert.deepEqual(loaded.calculateHostsContentStats([
  '# system',
  '127.0.0.1 localhost',
  '',
  '10.0.0.1 service.local alias.local # note',
  '10.0.0.2 SERVICE.local',
  'not-an-ip ignored.local',
  'invalid',
].join('\n')), {
  totalLines: 7,
  validRules: 3,
  commentLines: 1,
  emptyLines: 1,
  duplicateRules: 1,
})

console.log('hostsContentStats tests passed')
