import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../src/components/capture/CaptureOverlayWindow.vue', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../src/style.css', import.meta.url), 'utf8')

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

function assertKeepsToolActive(name) {
  const body = functionBody(name)
  assert.equal(
    body.includes('editMode.value = false'),
    false,
    `${name} should keep the active annotation tool selected after creating an annotation`,
  )
}

assertKeepsToolActive('endAnnotation')
assertKeepsToolActive('addNumberAnnotation')

const commitTextBody = functionBody('commitTextAnnotation')
const newTextBranch = commitTextBody.slice(commitTextBody.indexOf('if (!text) return'))
assert.notEqual(newTextBranch, '', 'missing new text annotation branch')
assert.equal(
  newTextBranch.includes('editMode.value = false'),
  false,
  'commitTextAnnotation should keep the text tool selected after creating new text',
)

assert.match(
  source,
  /const canHitAnnotations = computed/,
  'capture overlay should separate annotation hit-testing from non-edit selection mode',
)
assert.match(
  source,
  /const canMoveAnnotations = computed\(\(\) => canHitAnnotations\.value && !editMode\.value\)/,
  'non-edit annotation selection should still be gated by edit mode',
)

const beginAnnotationBody = functionBody('beginAnnotation')
assert.ok(
  beginAnnotationBody.indexOf('if (beginMoveAnnotation(event)) return') >= 0
    && beginAnnotationBody.indexOf('if (beginMoveAnnotation(event)) return') < beginAnnotationBody.indexOf('const point = boundedAnnotationPoint(event)'),
  'beginAnnotation should drag an existing annotation before drawing a new one',
)

const beginMoveBody = functionBody('beginMoveAnnotation')
assert.match(
  beginMoveBody,
  /if \(!canHitAnnotations\.value\) return false/,
  'beginMoveAnnotation should remain available while an annotation tool is active',
)
assert.match(
  beginMoveBody,
  /return true/,
  'beginMoveAnnotation should report when it captures an existing annotation',
)

assert.match(source, /@pointermove\.stop="moveAnnotationCanvas"/, 'editing canvas should route pointer moves through the move-or-draw handler')
assert.match(source, /@pointerup\.stop="endAnnotationCanvas"/, 'editing canvas should route pointer release through the move-or-draw handler')
assert.match(styles, /capture-annotation-canvas\.is-hovering-annotation/, 'editing canvas should show a move cursor over existing annotations')

console.log('capture annotation flow tests passed')
