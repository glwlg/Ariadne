import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../src/components/capture/CaptureOverlayWindow.vue', import.meta.url), 'utf8')

function functionBody(name) {
  const signatureIndex = source.indexOf(`function ${name}(`)
  assert.notEqual(signatureIndex, -1, `missing function ${name}`)
  const openBrace = source.indexOf('{', signatureIndex)
  assert.notEqual(openBrace, -1, `missing function body for ${name}`)
  let depth = 0
  for (let index = openBrace; index < source.length; index += 1) {
    const char = source[index]
    if (char === '{') depth += 1
    if (char === '}') depth -= 1
    if (depth === 0) {
      return source.slice(openBrace + 1, index)
    }
  }
  throw new Error(`unterminated function body for ${name}`)
}

const mountedStart = source.indexOf('onMounted(async () => {')
assert.notEqual(mountedStart, -1, 'missing capture overlay onMounted hook')
const mountedEnd = source.indexOf('})', mountedStart)
assert.notEqual(mountedEnd, -1, 'missing capture overlay onMounted hook end')
const mountedBody = source.slice(mountedStart, mountedEnd)

assert.match(
  mountedBody,
  /void prepareWindow\(\)/,
  'capture overlay should not await window decoration calls before loading the screenshot session',
)
assert.doesNotMatch(
  mountedBody,
  /await prepareWindow\(\)/,
  'awaiting prepareWindow keeps the overlay grey while runtime window calls complete',
)
assert.match(
  mountedBody,
  /await preloadOverlayImage\(nextSession\.imageUrl\)/,
  'capture overlay should preload and decode the screenshot before it becomes interactive',
)
assert.ok(
  mountedBody.indexOf('await preloadOverlayImage(nextSession.imageUrl)') < mountedBody.indexOf('session.value = nextSession'),
  'capture overlay should assign the session after the background image is ready',
)

const beginSelectionBody = functionBody('beginSelection')
assert.match(
  beginSelectionBody,
  /isLoading\.value \|\| !session\.value/,
  'capture overlay should not start a selection while the screenshot image is still loading',
)

assert.match(source, /loading="eager"/, 'capture overlay image should request eager loading')
assert.match(source, /decoding="sync"/, 'capture overlay image should request sync decoding')
assert.match(source, /fetchpriority="high"/, 'capture overlay image should request high fetch priority')
assert.match(source, /@load="handleOverlayImageLoad"/, 'capture overlay image load should force first-frame repaint')

console.log('capture overlay load flow tests passed')
