import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('../src/components/capture/CaptureOverlayWindow.vue', import.meta.url), 'utf8')
const styles = [
  '../src/style.css',
  '../src/styles/surfaces-settings-api-network-overlays.css',
].map((path) => readFileSync(new URL(path, import.meta.url), 'utf8')).join('\n')

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

assert.match(source, /function defaultToolbarPosition\(/, 'capture toolbar should choose a position outside the selection')
assert.match(source, /function beginToolbarDrag\(/, 'capture toolbar should expose a drag interaction')
assert.match(source, /@pointerdown\.stop\.prevent="beginToolbarDrag"/, 'toolbar drag handle should own pointer capture')
assert.match(styles, /capture-toolbar-drag-handle/, 'capture toolbar should show a dedicated drag handle')

// UI-07: dynamic arrow marker color matching operationColor
assert.match(source, /v-for="color in arrowMarkerColors"/, 'UI-07: should define SVG markers for each arrow color')
assert.match(source, /arrowMarkerId\(operationColor\(operation\)\)/, 'UI-07: arrow line marker-end should bind to operationColor')

// UI-13: dark surface for high-luminance text editor and 740px toolbar width
assert.match(source, /'is-dark-surface': isTextEditorDarkSurface/, 'UI-13: high-luminance annotation colors should switch text editor to dark surface')
assert.match(source, /Math\.min\(740,/, 'UI-13: toolbarStyle should estimate 740px width instead of 960px')

// UX-07: double-submit close guard, IME guard, modifier guard, Shift keyup solo toggle, resize preserves annotations
const runSelectionBody = functionBody('runSelectionAction')
assert.match(runSelectionBody, /if \(isBusy\.value \|\| isClosing\.value\) return/, 'UX-07: runSelectionAction must ignore duplicate submits while busy or closing')
assert.match(runSelectionBody, /if \(!scheduledClose\) \{[\s\S]*isBusy\.value = false/, 'UX-07: isBusy must remain true while closeWindow timer is pending')

const textEnterBody = functionBody('handleTextEditorEnter')
assert.match(textEnterBody, /event\.isComposing \|\| event\.keyCode === 229/, 'UX-07: text editor Enter must guard IME composition')

const keyDownBody = functionBody('handleKeyDown')
assert.match(keyDownBody, /event\.isComposing \|\| event\.keyCode === 229/, 'UX-07: handleKeyDown must guard IME composition')
assert.match(keyDownBody, /!hasAnyModifier && !event\.shiftKey && lowerKey === 'p'/, 'UX-07: single-letter tool shortcuts must require no Ctrl/Meta/Alt/Shift modifiers')

const keyUpBody = functionBody('handleKeyUp')
assert.match(keyUpBody, /event\.key === 'Shift' && shiftPressedSolo/, 'UX-07: colorFormat toggle should run on solo Shift keyup only')

const beginResizeBody = functionBody('beginResizeSelection')
assert.equal(beginResizeBody.includes('annotationOperations.value = []'), false, 'UX-07: beginResizeSelection must not wipe existing annotations')
assert.match(beginResizeBody, /resizeAnnotationSnapshot/, 'UX-07: beginResizeSelection should snapshot annotations for coordinate preservation')

console.log('capture annotation flow tests passed')
