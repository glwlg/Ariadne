<script setup lang="ts">
import {
  ArrowUpRight,
  Check,
  Copy,
  Eraser,
  Grid3X3,
  Hash,
  Minus,
  Pencil,
  Pin,
  QrCode,
  Redo2,
  RotateCcw,
  Save,
  Square,
  Type,
  X,
} from '@lucide/vue'
import { Clipboard as WailsClipboard, Dialogs, Window } from '@wailsio/runtime'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  cancelCaptureOverlay,
  captureOverlaySelection,
  getCaptureOverlaySession,
} from '../../services/captureOverlayApi'
import {
  mapVisualSelectionToSourcePixels,
} from '../../lib/captureGeometry'
import type {
  CaptureOverlayAnnotationOperation,
  CaptureOverlayResult,
  CaptureOverlaySelectionRequest,
  CaptureOverlaySession,
  ScreenBounds,
} from '../../types/ariadne'

const props = defineProps<{
  sessionId: string
}>()

type AnnotationTool = CaptureOverlayAnnotationOperation['kind']
type Point = { x: number; y: number }
type VisualSelectionRect = { left: number; top: number; width: number; height: number; displayWidth: number; displayHeight: number }
type ResizeAnchor = 'tl' | 't' | 'tr' | 'r' | 'br' | 'b' | 'bl' | 'l'

const resizeHandles: Array<{ anchor: ResizeAnchor; label: string }> = [
  { anchor: 'tl', label: '左上调整' },
  { anchor: 't', label: '上边调整' },
  { anchor: 'tr', label: '右上调整' },
  { anchor: 'r', label: '右边调整' },
  { anchor: 'br', label: '右下调整' },
  { anchor: 'b', label: '下边调整' },
  { anchor: 'bl', label: '左下调整' },
  { anchor: 'l', label: '左边调整' },
]

const colorPalette = ['#dc2626', '#f97316', '#facc15', '#22c55e', '#14b8a6', '#2563eb', '#7c3aed', '#111827', '#ffffff']

const session = ref<CaptureOverlaySession | null>(null)
const feedback = ref('')
const isLoading = ref(true)
const isBusy = ref(false)
const dragStart = ref<{ x: number; y: number } | null>(null)
const dragEnd = ref<{ x: number; y: number } | null>(null)
const selectionPointerId = ref<number | null>(null)
const resizePointerId = ref<number | null>(null)
const resizeAnchor = ref<ResizeAnchor | null>(null)
const resizeOrigin = ref<{ left: number; top: number; width: number; height: number } | null>(null)
const editTool = ref<AnnotationTool>('rect')
const editMode = ref(false)
const annotationColor = ref('#dc2626')
const annotationThickness = ref(3)
const annotationStart = ref<{ x: number; y: number } | null>(null)
const annotationPointerId = ref<number | null>(null)
const annotationOperations = ref<CaptureOverlayAnnotationOperation[]>([])
const redoAnnotationOperations = ref<CaptureOverlayAnnotationOperation[]>([])
const draftAnnotation = ref<CaptureOverlayAnnotationOperation | null>(null)
const selectedAnnotationIndex = ref<number | null>(null)
const movingAnnotationPointerId = ref<number | null>(null)
const movingAnnotationOrigin = ref<{ point: Point; operations: CaptureOverlayAnnotationOperation[]; moved: boolean } | null>(null)
const textEditor = ref<{ x: number; y: number; text: string; index?: number } | null>(null)
const textInputRef = ref<HTMLInputElement | null>(null)
const currentMousePoint = ref<Point>({ x: 0, y: 0 })
const colorFormat = ref<'rgb' | 'hex'>('rgb')
let sampleCanvas: HTMLCanvasElement | null = null
let sampleContext: CanvasRenderingContext2D | null = null
let sampleImageSrc = ''

const selection = computed(() => {
  if (!dragStart.value || !dragEnd.value) return null
  const left = Math.min(dragStart.value.x, dragEnd.value.x)
  const top = Math.min(dragStart.value.y, dragEnd.value.y)
  const width = Math.abs(dragEnd.value.x - dragStart.value.x)
  const height = Math.abs(dragEnd.value.y - dragStart.value.y)
  return { left, top, width, height }
})

const hasSelection = computed(() => {
  const rect = selection.value
  return Boolean(rect && rect.width >= 2 && rect.height >= 2)
})

const isSelecting = computed(() => selectionPointerId.value !== null)
const isResizingSelection = computed(() => resizePointerId.value !== null)
const canShowToolbar = computed(() => hasSelection.value && !isSelecting.value && !isResizingSelection.value)
const canMoveAnnotations = computed(() => (
  hasSelection.value
  && annotationOperations.value.length > 0
  && !editMode.value
  && !textEditor.value
  && !isSelecting.value
  && !isResizingSelection.value
))

const selectionStyle = computed(() => {
  const rect = selection.value
  if (!rect) return {}
  return {
    left: `${rect.left}px`,
    top: `${rect.top}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
  }
})

const toolbarStyle = computed(() => {
  const rect = selection.value
  if (!rect) return {}
  const viewport = overlayViewport()
  const top = Math.min(viewport.height - 42, rect.top + rect.height + 10)
  const toolbarWidth = Math.min(960, viewport.width - 24)
  const left = Math.max(12, Math.min(viewport.width - toolbarWidth - 12, rect.left))
  return { left: `${left}px`, top: `${Math.max(12, top)}px` }
})

const selectionViewBox = computed(() => {
  const rect = selection.value
  return rect ? `0 0 ${Math.max(1, Math.round(rect.width))} ${Math.max(1, Math.round(rect.height))}` : '0 0 1 1'
})

const annotationPreviewOperations = computed(() => {
  const operations = [...annotationOperations.value]
  if (draftAnnotation.value) operations.push(draftAnnotation.value)
  return operations
})

const pointerColor = computed(() => samplePointerColor())

const pointerColorText = computed(() => {
  const pixel = pointerColor.value
  if (!pixel) return '---'
  return formatPixelColor(pixel, colorFormat.value)
})

const pointerHexText = computed(() => {
  const pixel = pointerColor.value
  return pixel ? formatPixelColor(pixel, 'hex') : '#------'
})

const pointerRgbText = computed(() => {
  const pixel = pointerColor.value
  return pixel ? formatPixelColor(pixel, 'rgb') : 'rgb(-, -, -)'
})

const magnifierStyle = computed(() => {
  const point = currentMousePoint.value
  const size = 132
  const margin = 18
  const viewport = overlayViewport()
  const offsetX = point.x > viewport.width - size - 72 ? -size - 24 : 24
  const offsetY = point.y > viewport.height - size - 92 ? -size - 34 : 24
  return {
    left: `${Math.max(margin, Math.min(viewport.width - size - margin, point.x + offsetX))}px`,
    top: `${Math.max(margin, Math.min(viewport.height - size - margin, point.y + offsetY))}px`,
  }
})

const magnifierImageStyle = computed(() => {
  const current = session.value
  if (!current?.imageUrl) return {}
  const sourceSize = overlaySourceSize(current)
  const sourceX = sourcePointerX()
  const sourceY = sourcePointerY()
  const sampleSize = 24
  const lensSize = 96
  const scale = lensSize / sampleSize
  return {
    width: `${Math.round(sourceSize.width * scale)}px`,
    height: `${Math.round(sourceSize.height * scale)}px`,
    transform: `translate(${Math.round(lensSize / 2 - sourceX * scale)}px, ${Math.round(lensSize / 2 - sourceY * scale)}px)`,
  }
})

onMounted(async () => {
  document.documentElement.classList.add('capture-overlay-document')
  await prepareWindow()
  session.value = await getCaptureOverlaySession(props.sessionId)
  isLoading.value = false
  window.addEventListener('keydown', handleKeyDown)
})

onBeforeUnmount(() => {
  document.documentElement.classList.remove('capture-overlay-document')
  window.removeEventListener('keydown', handleKeyDown)
  sampleCanvas = null
  sampleContext = null
  sampleImageSrc = ''
})

async function prepareWindow() {
  try {
    await Window.SetFrameless(true)
    await Window.SetAlwaysOnTop(true)
    await Window.SetBackgroundColour(244, 244, 245, 255)
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function beginSelection(event: PointerEvent) {
  if (event.button !== 0) return
  if ((event.target as HTMLElement | null)?.closest('button')) return
  if (editMode.value || textEditor.value) return
  const point = boundedPoint(event)
  dragStart.value = point
  dragEnd.value = point
  selectionPointerId.value = event.pointerId
  annotationOperations.value = []
  redoAnnotationOperations.value = []
  draftAnnotation.value = null
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function moveSelection(event: PointerEvent) {
  currentMousePoint.value = boundedPoint(event)
  if (selectionPointerId.value !== event.pointerId || !dragStart.value || editMode.value) return
  dragEnd.value = boundedPoint(event)
}

function endSelection(event: PointerEvent) {
  if (selectionPointerId.value !== event.pointerId || !dragStart.value) return
  try {
    ;(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId)
  } catch {
    // Pointer capture may already be released.
  }
  dragEnd.value = boundedPoint(event)
  selectionPointerId.value = null
  if (!hasSelection.value) {
    showFeedback('拖拽选择一个区域')
  }
}

function cancelSelectionPointer(event: PointerEvent) {
  if (selectionPointerId.value !== event.pointerId) return
  selectionPointerId.value = null
  if (!hasSelection.value) {
    showFeedback('拖拽选择一个区域')
  }
}

function beginResizeSelection(anchor: ResizeAnchor, event: PointerEvent) {
  if (event.button !== 0 || !selection.value) return
  event.preventDefault()
  if (textEditor.value) commitTextAnnotation()
  if (annotationOperations.value.length) {
    annotationOperations.value = []
    redoAnnotationOperations.value = []
    selectedAnnotationIndex.value = null
    showFeedback('调整选区后已清空标注')
  }
  resizeAnchor.value = anchor
  resizePointerId.value = event.pointerId
  resizeOrigin.value = { ...selection.value }
  editMode.value = false
  annotationPointerId.value = null
  annotationStart.value = null
  draftAnnotation.value = null
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function moveResizeSelection(event: PointerEvent) {
  if (resizePointerId.value !== event.pointerId || !resizeAnchor.value || !resizeOrigin.value) return
  const point = boundedPoint(event)
  const origin = resizeOrigin.value
  let left = origin.left
  let top = origin.top
  let right = origin.left + origin.width
  let bottom = origin.top + origin.height
  if (resizeAnchor.value.includes('l')) left = point.x
  if (resizeAnchor.value.includes('r')) right = point.x
  if (resizeAnchor.value.includes('t')) top = point.y
  if (resizeAnchor.value.includes('b')) bottom = point.y
  applySelectionRect(left, top, right, bottom)
}

function endResizeSelection(event: PointerEvent) {
  if (resizePointerId.value !== event.pointerId) return
  try {
    ;(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId)
  } catch {
    // Pointer capture may already be released.
  }
  resizePointerId.value = null
  resizeAnchor.value = null
  resizeOrigin.value = null
}

function cancelResizeSelection(event: PointerEvent) {
  if (resizePointerId.value !== event.pointerId) return
  resizePointerId.value = null
  resizeAnchor.value = null
  resizeOrigin.value = null
}

function handleContextMenu(event: MouseEvent) {
  event.preventDefault()
  if (hasSelection.value || selection.value) {
    resetSelection()
    return
  }
  void closeWindow(true)
}

async function runSelectionAction(action: CaptureOverlaySelectionRequest['action'], savedPath = '') {
  if (textEditor.value) commitTextAnnotation()
  const current = session.value
  const rect = selection.value
  if (!current || !rect || rect.width < 2 || rect.height < 2) {
    showFeedback('先拖拽选择区域')
    return
  }
  isBusy.value = true
  try {
    const operations = annotationOperations.value.map(cloneAnnotationOperation)
    const visualRect = mapSelectionToVisualRect(rect)
    const sourceRect = mapSelectionToSourcePixels(current, visualRect)
    const sessionOperations = scaleAnnotationOperations(
      operations,
      sourceRect.width / Math.max(1, visualRect.width),
      sourceRect.height / Math.max(1, visualRect.height),
    )
    const result = await captureOverlaySelection({
      sessionId: current.id,
      x: visualRect.left,
      y: visualRect.top,
      width: visualRect.width,
      height: visualRect.height,
      coordinateSpace: 'visual',
      displayWidth: visualRect.displayWidth,
      displayHeight: visualRect.displayHeight,
      action,
      savedPath,
      operations: sessionOperations,
      renderedImage: await renderAnnotatedSelectionPNG(sourceRect, sessionOperations),
    })
    await handleResult(result, action)
  } catch (error) {
    showFeedback(error instanceof Error ? error.message : '截图失败')
  } finally {
    isBusy.value = false
  }
}

async function runSaveAsAction() {
  const rect = selection.value
  if (!rect || rect.width < 2 || rect.height < 2) {
    showFeedback('先拖拽选择区域')
    return
  }
  let savedPath = ''
  try {
    savedPath = await Dialogs.SaveFile({
      Title: '另存截图',
      Filename: defaultCaptureFilename(),
      ButtonText: '保存',
      Filters: [{ DisplayName: 'PNG 图片', Pattern: '*.png' }],
    })
  } catch {
    showFeedback('无法打开保存对话框')
    return
  }
  if (!savedPath) {
    showFeedback('已取消另存')
    return
  }
  await runSelectionAction('save_as', savedPath)
}

async function handleResult(result: CaptureOverlayResult, action: CaptureOverlaySelectionRequest['action']) {
  if (action === 'qr' && result.qr?.ok && result.qr.text) {
    try {
      await WailsClipboard.SetText(result.qr.text)
      showFeedback(`二维码已复制: ${clip(result.qr.text, 38)}`)
    } catch {
      showFeedback(`已识别二维码: ${clip(result.qr.text, 38)}`)
    }
    window.setTimeout(() => void closeWindow(false), 500)
    return
  }
  showFeedback(result.savedPath ? `${result.message}: ${clip(result.savedPath, 44)}` : result.message || (result.ok ? '已保存截图' : '截图失败'))
  if (result.ok) {
    window.setTimeout(() => void closeWindow(false), 450)
  }
}

async function closeWindow(cancel = true) {
  if (cancel && session.value?.id) {
    await cancelCaptureOverlay(session.value.id)
  }
  try {
    await Window.Close()
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function resetSelection() {
  dragStart.value = null
  dragEnd.value = null
  selectionPointerId.value = null
  resizePointerId.value = null
  resizeAnchor.value = null
  resizeOrigin.value = null
  editMode.value = false
  annotationStart.value = null
  annotationPointerId.value = null
  annotationOperations.value = []
  redoAnnotationOperations.value = []
  draftAnnotation.value = null
  selectedAnnotationIndex.value = null
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  textEditor.value = null
  feedback.value = ''
}

function handleKeyDown(event: KeyboardEvent) {
  if (textEditor.value) {
    if (event.key === 'Escape') {
      event.preventDefault()
      textEditor.value = null
    }
    return
  }
  if (event.key === 'Escape') {
    void closeWindow(true)
  } else if (event.key === 'Shift' && !event.repeat) {
    colorFormat.value = colorFormat.value === 'rgb' ? 'hex' : 'rgb'
    showFeedback(`取色格式: ${colorFormat.value.toUpperCase()}`)
  } else if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key.toLowerCase() === 'z') {
    event.preventDefault()
    redoAnnotation()
  } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
    event.preventDefault()
    undoAnnotation()
  } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'y') {
    event.preventDefault()
    redoAnnotation()
  } else if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    void runSaveAsAction()
  } else if (event.key === 'Enter') {
    void runSelectionAction('copy')
  } else if (event.key.toLowerCase() === 'p') {
    void runSelectionAction('pin')
  } else if (event.key.toLowerCase() === 'q') {
    void runSelectionAction('qr')
  } else if (event.key.toLowerCase() === 'r') {
    toggleAnnotationTool('rect')
  } else if (event.key.toLowerCase() === 'l') {
    toggleAnnotationTool('line')
  } else if (event.key.toLowerCase() === 'a') {
    toggleAnnotationTool('arrow')
  } else if (event.key.toLowerCase() === 'b') {
    toggleAnnotationTool('pen')
  } else if (event.key.toLowerCase() === 'm') {
    toggleAnnotationTool('mosaic')
  } else if (event.key.toLowerCase() === 't') {
    toggleAnnotationTool('text')
  } else if (event.key.toLowerCase() === 'n') {
    toggleAnnotationTool('number')
  } else if (event.key.toLowerCase() === 'e') {
    toggleAnnotationTool('eraser')
  } else if (event.key.toLowerCase() === 'v' && !event.ctrlKey && !event.metaKey) {
    activateAnnotationSelect()
  } else if (event.key.toLowerCase() === 'c' && !event.ctrlKey && !event.metaKey) {
    event.preventDefault()
    void copyPointerColor()
  } else if ((event.key === 'Backspace' || event.key === 'Delete') && selectedAnnotationIndex.value !== null) {
    event.preventDefault()
    deleteSelectedAnnotation()
  } else if (event.key === 'Backspace' && annotationOperations.value.length) {
    undoAnnotation()
  }
}

function toggleAnnotationTool(tool: AnnotationTool) {
  if (!hasSelection.value) {
    showFeedback('先拖拽选择区域')
    return
  }
  if (textEditor.value) commitTextAnnotation()
  selectedAnnotationIndex.value = null
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  editTool.value = tool
  editMode.value = true
  showFeedback(annotationToolLabel(tool))
}

function activateAnnotationSelect() {
  if (!annotationOperations.value.length) {
    showFeedback('暂无标注可选择')
    return
  }
  if (textEditor.value) commitTextAnnotation()
  editMode.value = false
  annotationStart.value = null
  annotationPointerId.value = null
  draftAnnotation.value = null
  showFeedback('选择并拖动已有标注')
}

function beginAnnotation(event: PointerEvent) {
  if (event.button !== 0) return
  if (!editMode.value || !selection.value) return
  const point = boundedAnnotationPoint(event)
  if (editTool.value === 'text') {
    startTextEditor(point)
    return
  }
  if (editTool.value === 'number') {
    addNumberAnnotation(point)
    return
  }
  annotationStart.value = point
  annotationPointerId.value = event.pointerId
  draftAnnotation.value = createAnnotation(point, point)
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function moveAnnotation(event: PointerEvent) {
  if (annotationPointerId.value !== event.pointerId || !editMode.value || !annotationStart.value) return
  const point = boundedAnnotationPoint(event)
  if (draftAnnotation.value && isPathTool(editTool.value)) {
    draftAnnotation.value = appendPointToOperation(draftAnnotation.value, point)
    return
  }
  draftAnnotation.value = createAnnotation(annotationStart.value, point)
}

function endAnnotation(event: PointerEvent) {
  if (annotationPointerId.value !== event.pointerId || !editMode.value || !annotationStart.value) return
  try {
    ;(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId)
  } catch {
    // Pointer capture may already be released.
  }
  const operation = draftAnnotation.value && isPathTool(editTool.value)
    ? appendPointToOperation(draftAnnotation.value, boundedAnnotationPoint(event))
    : createAnnotation(annotationStart.value, boundedAnnotationPoint(event))
  annotationStart.value = null
  annotationPointerId.value = null
  draftAnnotation.value = null
  if (!isUsefulAnnotation(operation)) {
    showFeedback('标注区域太小')
    return
  }
  annotationOperations.value = [...annotationOperations.value, operation]
  redoAnnotationOperations.value = []
  selectedAnnotationIndex.value = annotationOperations.value.length - 1
}

function cancelAnnotationPointer(event: PointerEvent) {
  if (annotationPointerId.value !== event.pointerId) return
  annotationPointerId.value = null
  annotationStart.value = null
  draftAnnotation.value = null
}

function createAnnotation(start: { x: number; y: number }, end: { x: number; y: number }): CaptureOverlayAnnotationOperation {
  const x = Math.round(start.x)
  const y = Math.round(start.y)
  const endX = Math.round(end.x)
  const endY = Math.round(end.y)
  const color = annotationColor.value
  const strokeWidth = annotationThickness.value
  if (editTool.value === 'pen' || editTool.value === 'mosaic' || editTool.value === 'eraser') {
    return {
      kind: editTool.value,
      x,
      y,
      points: [roundPoint(start), roundPoint(end)],
      color,
      strokeWidth,
      pixelSize: Math.max(8, strokeWidth * 4),
    }
  }
  if (editTool.value === 'line') {
    return { kind: 'line', x, y, endX, endY, color, strokeWidth }
  }
  if (editTool.value === 'arrow') {
    return { kind: 'arrow', x, y, endX, endY, color, strokeWidth: Math.max(3, strokeWidth) }
  }
  const left = Math.min(x, endX)
  const top = Math.min(y, endY)
  const width = Math.abs(endX - x)
  const height = Math.abs(endY - y)
  return { kind: 'rect', x: left, y: top, width, height, color, strokeWidth }
}

function isUsefulAnnotation(operation: CaptureOverlayAnnotationOperation) {
  if (operation.kind === 'arrow' || operation.kind === 'line') {
    return Math.abs((operation.endX ?? operation.x) - operation.x) + Math.abs((operation.endY ?? operation.y) - operation.y) >= 8
  }
  if (operation.kind === 'pen' || operation.kind === 'mosaic' || operation.kind === 'eraser') {
    return pathLength(operation.points ?? []) >= 8
  }
  if (operation.kind === 'text') return Boolean(operation.text?.trim())
  if (operation.kind === 'number') return true
  return Number(operation.width ?? 0) >= 6 && Number(operation.height ?? 0) >= 6
}

function clearAnnotations() {
  annotationOperations.value = []
  redoAnnotationOperations.value = []
  draftAnnotation.value = null
  annotationStart.value = null
  annotationPointerId.value = null
  selectedAnnotationIndex.value = null
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  showFeedback('已清空标注')
}

function undoAnnotation() {
  if (!annotationOperations.value.length) return
  const nextOperations = annotationOperations.value.slice(0, -1)
  const undone = annotationOperations.value[annotationOperations.value.length - 1]
  annotationOperations.value = nextOperations
  redoAnnotationOperations.value = [undone, ...redoAnnotationOperations.value]
  selectedAnnotationIndex.value = null
  showFeedback(annotationOperations.value.length ? '已撤销上一条标注' : '已清空标注')
}

function redoAnnotation() {
  if (!redoAnnotationOperations.value.length) return
  const [operation, ...rest] = redoAnnotationOperations.value
  annotationOperations.value = [...annotationOperations.value, operation]
  redoAnnotationOperations.value = rest
  selectedAnnotationIndex.value = annotationOperations.value.length - 1
  showFeedback('已重做上一条标注')
}

function deleteSelectedAnnotation() {
  const index = selectedAnnotationIndex.value
  if (index === null || !annotationOperations.value[index]) return
  annotationOperations.value = annotationOperations.value.filter((_, operationIndex) => operationIndex !== index)
  redoAnnotationOperations.value = []
  selectedAnnotationIndex.value = null
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  showFeedback('已删除选中标注')
}

function isPathTool(tool: AnnotationTool) {
  return tool === 'pen' || tool === 'mosaic' || tool === 'eraser'
}

function appendPointToOperation(operation: CaptureOverlayAnnotationOperation, point: Point): CaptureOverlayAnnotationOperation {
  const points = operation.points ? [...operation.points] : [{ x: operation.x, y: operation.y }]
  const next = roundPoint(point)
  const last = points[points.length - 1]
  if (!last || Math.abs(last.x - next.x) + Math.abs(last.y - next.y) >= 2) {
    points.push(next)
  }
  return { ...operation, points }
}

function addNumberAnnotation(point: Point) {
  const operation: CaptureOverlayAnnotationOperation = {
    kind: 'number',
    x: Math.round(point.x),
    y: Math.round(point.y),
    color: annotationColor.value,
    strokeWidth: annotationThickness.value,
    number: nextNumberValue(),
    fontSize: Math.max(14, 12 + annotationThickness.value * 2),
  }
  annotationOperations.value = [...annotationOperations.value, operation]
  redoAnnotationOperations.value = []
  selectedAnnotationIndex.value = annotationOperations.value.length - 1
  showFeedback(`序号 ${operation.number}`)
}

function startTextEditor(point: Point) {
  if (textEditor.value) commitTextAnnotation()
  textEditor.value = { x: Math.round(point.x), y: Math.round(point.y), text: '' }
  void nextTick(() => textInputRef.value?.focus())
}

function startExistingTextEditor(index: number) {
  const operation = annotationOperations.value[index]
  if (!operation || operation.kind !== 'text') return
  if (textEditor.value) commitTextAnnotation()
  selectedAnnotationIndex.value = index
  textEditor.value = {
    x: Math.round(operation.x),
    y: Math.round(operation.y),
    text: operation.text ?? '',
    index,
  }
  void nextTick(() => textInputRef.value?.focus())
}

function commitTextAnnotation() {
  const editor = textEditor.value
  if (!editor) return
  const text = editor.text.trim()
  textEditor.value = null
  if (editor.index !== undefined) {
    if (!text) {
      annotationOperations.value = annotationOperations.value.filter((_, index) => index !== editor.index)
      selectedAnnotationIndex.value = null
      redoAnnotationOperations.value = []
      return
    }
    const nextOperations = [...annotationOperations.value]
    const previous = nextOperations[editor.index]
    if (!previous) return
    nextOperations[editor.index] = {
      ...previous,
      kind: 'text',
      x: editor.x,
      y: editor.y,
      text,
      color: previous.color || annotationColor.value,
      strokeWidth: previous.strokeWidth ?? annotationThickness.value,
      fontSize: previous.fontSize ?? Math.max(16, 12 + annotationThickness.value * 3),
    }
    annotationOperations.value = nextOperations
    redoAnnotationOperations.value = []
    selectedAnnotationIndex.value = editor.index
    return
  }
  if (!text) return
  const operation: CaptureOverlayAnnotationOperation = {
    kind: 'text',
    x: editor.x,
    y: editor.y,
    text,
    color: annotationColor.value,
    strokeWidth: annotationThickness.value,
    fontSize: Math.max(16, 12 + annotationThickness.value * 3),
  }
  annotationOperations.value = [...annotationOperations.value, operation]
  redoAnnotationOperations.value = []
  selectedAnnotationIndex.value = annotationOperations.value.length - 1
}

function beginMoveAnnotation(event: PointerEvent) {
  if (!canMoveAnnotations.value) return
  if (event.button !== 0) return
  currentMousePoint.value = boundedPoint(event)
  const point = boundedAnnotationPoint(event)
  const index = findAnnotationAtPoint(point)
  if (index === null) {
    selectedAnnotationIndex.value = null
    return
  }
  if (event.detail >= 2 && annotationOperations.value[index]?.kind === 'text') {
    event.preventDefault()
    startExistingTextEditor(index)
    return
  }
  event.preventDefault()
  selectedAnnotationIndex.value = index
  movingAnnotationPointerId.value = event.pointerId
  movingAnnotationOrigin.value = {
    point,
    operations: annotationOperations.value.map(cloneAnnotationOperation),
    moved: false,
  }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function editTextAnnotationAtPoint(event: MouseEvent) {
  if (!canMoveAnnotations.value) return
  const point = boundedAnnotationPoint(event)
  const index = findAnnotationAtPoint(point)
  if (index === null || annotationOperations.value[index]?.kind !== 'text') return
  event.preventDefault()
  selectedAnnotationIndex.value = index
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  startExistingTextEditor(index)
}

function moveSelectedAnnotation(event: PointerEvent) {
  currentMousePoint.value = boundedPoint(event)
  const origin = movingAnnotationOrigin.value
  const pointerId = movingAnnotationPointerId.value
  const index = selectedAnnotationIndex.value
  if (!origin || pointerId !== event.pointerId || index === null) return
  const point = boundedAnnotationPoint(event)
  const deltaX = point.x - origin.point.x
  const deltaY = point.y - origin.point.y
  if (Math.abs(deltaX) + Math.abs(deltaY) > 1) origin.moved = true
  const baseOperation = origin.operations[index]
  if (!baseOperation) return
  const nextOperations = origin.operations.map(cloneAnnotationOperation)
  nextOperations[index] = translateAnnotationOperation(baseOperation, deltaX, deltaY)
  annotationOperations.value = nextOperations
}

function endMoveAnnotation(event: PointerEvent) {
  if (movingAnnotationPointerId.value !== event.pointerId) return
  try {
    ;(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId)
  } catch {
    // Pointer capture may already be released.
  }
  const moved = movingAnnotationOrigin.value?.moved
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
  redoAnnotationOperations.value = []
  if (moved) showFeedback('已移动标注')
}

function cancelMoveAnnotation(event: PointerEvent) {
  if (movingAnnotationPointerId.value !== event.pointerId) return
  movingAnnotationPointerId.value = null
  movingAnnotationOrigin.value = null
}

function translateAnnotationOperation(operation: CaptureOverlayAnnotationOperation, deltaX: number, deltaY: number): CaptureOverlayAnnotationOperation {
  return {
    ...operation,
    x: Math.round(operation.x + deltaX),
    y: Math.round(operation.y + deltaY),
    endX: operation.endX === undefined ? undefined : Math.round(operation.endX + deltaX),
    endY: operation.endY === undefined ? undefined : Math.round(operation.endY + deltaY),
    points: operation.points?.map((point) => ({ x: Math.round(point.x + deltaX), y: Math.round(point.y + deltaY) })),
  }
}

function findAnnotationAtPoint(point: Point) {
  for (let index = annotationOperations.value.length - 1; index >= 0; index -= 1) {
    if (annotationContainsPoint(annotationOperations.value[index], point)) return index
  }
  return null
}

function annotationContainsPoint(operation: CaptureOverlayAnnotationOperation, point: Point) {
  const stroke = Math.max(6, operationStrokeWidth(operation) * 2)
  if (operation.kind === 'rect' || (operation.kind === 'mosaic' && !operation.points?.length)) {
    const left = Math.min(operation.x, operation.x + Number(operation.width ?? 0)) - stroke
    const right = Math.max(operation.x, operation.x + Number(operation.width ?? 0)) + stroke
    const top = Math.min(operation.y, operation.y + Number(operation.height ?? 0)) - stroke
    const bottom = Math.max(operation.y, operation.y + Number(operation.height ?? 0)) + stroke
    return point.x >= left && point.x <= right && point.y >= top && point.y <= bottom
  }
  if (operation.kind === 'line' || operation.kind === 'arrow') {
    return distanceToSegment(point, { x: operation.x, y: operation.y }, { x: operation.endX ?? operation.x, y: operation.endY ?? operation.y }) <= stroke
  }
  if (operation.kind === 'pen' || operation.kind === 'mosaic' || operation.kind === 'eraser') {
    const points = operation.points ?? [{ x: operation.x, y: operation.y }]
    return distanceToPolyline(point, points) <= Math.max(stroke, operation.kind === 'mosaic' ? Number(operation.pixelSize ?? 16) : 0)
  }
  if (operation.kind === 'text') {
    const fontSize = operationFontSize(operation)
    const width = Math.max(24, (operation.text ?? '').length * fontSize * 0.62)
    return point.x >= operation.x - 4 && point.x <= operation.x + width + 4 && point.y >= operation.y - 4 && point.y <= operation.y + fontSize * 1.3 + 4
  }
  if (operation.kind === 'number') {
    return distanceBetween(point, { x: operation.x, y: operation.y }) <= Math.max(14, operationFontSize(operation)) + stroke
  }
  return false
}

function distanceToPolyline(point: Point, points: Point[]) {
  if (!points.length) return Number.POSITIVE_INFINITY
  let distance = distanceBetween(point, points[0])
  for (let index = 1; index < points.length; index += 1) {
    distance = Math.min(distance, distanceToSegment(point, points[index - 1], points[index]))
  }
  return distance
}

function distanceToSegment(point: Point, start: Point, end: Point) {
  const dx = end.x - start.x
  const dy = end.y - start.y
  if (dx === 0 && dy === 0) return distanceBetween(point, start)
  const ratio = Math.max(0, Math.min(1, ((point.x - start.x) * dx + (point.y - start.y) * dy) / (dx * dx + dy * dy)))
  return distanceBetween(point, { x: start.x + ratio * dx, y: start.y + ratio * dy })
}

function distanceBetween(a: Point, b: Point) {
  return Math.hypot(a.x - b.x, a.y - b.y)
}

function nextNumberValue() {
  return annotationOperations.value.filter((operation) => operation.kind === 'number').length + 1
}

function pathLength(points: Point[]) {
  let length = 0
  for (let index = 1; index < points.length; index += 1) {
    length += Math.abs(points[index].x - points[index - 1].x) + Math.abs(points[index].y - points[index - 1].y)
  }
  return length
}

function roundPoint(point: Point): Point {
  return { x: Math.round(point.x), y: Math.round(point.y) }
}

function boundedAnnotationPoint(event: MouseEvent | PointerEvent) {
  const target = event.currentTarget as HTMLElement
  const bounds = target.getBoundingClientRect()
  return {
    x: Math.max(0, Math.min(bounds.width, event.clientX - bounds.left)),
    y: Math.max(0, Math.min(bounds.height, event.clientY - bounds.top)),
  }
}

function annotationToolLabel(tool: AnnotationTool) {
  if (tool === 'line') return '直线标注'
  if (tool === 'arrow') return '箭头标注'
  if (tool === 'pen') return '画笔'
  if (tool === 'mosaic') return '马赛克'
  if (tool === 'text') return '文字'
  if (tool === 'number') return '序号'
  if (tool === 'eraser') return '橡皮擦'
  return '矩形标注'
}

function applySelectionRect(left: number, top: number, right: number, bottom: number) {
  const bounds = overlayImageBoundsInSurface()
  const x1 = Math.max(bounds.left, Math.min(bounds.left + bounds.width, left))
  const x2 = Math.max(bounds.left, Math.min(bounds.left + bounds.width, right))
  const y1 = Math.max(bounds.top, Math.min(bounds.top + bounds.height, top))
  const y2 = Math.max(bounds.top, Math.min(bounds.top + bounds.height, bottom))
  dragStart.value = { x: Math.min(x1, x2), y: Math.min(y1, y2) }
  dragEnd.value = { x: Math.max(x1, x2), y: Math.max(y1, y2) }
}

function cloneAnnotationOperation(operation: CaptureOverlayAnnotationOperation): CaptureOverlayAnnotationOperation {
  return {
    ...operation,
    points: operation.points?.map((point) => ({ ...point })),
  }
}

function mapSelectionToVisualRect(rect: { left: number; top: number; width: number; height: number }): VisualSelectionRect {
  const imageBounds = overlayImageBoundsInSurface()
  const left = clampNumber(rect.left - imageBounds.left, 0, imageBounds.width)
  const top = clampNumber(rect.top - imageBounds.top, 0, imageBounds.height)
  const right = clampNumber(rect.left + rect.width - imageBounds.left, left, imageBounds.width)
  const bottom = clampNumber(rect.top + rect.height - imageBounds.top, top, imageBounds.height)
  return {
    left: Math.round(left),
    top: Math.round(top),
    width: Math.max(1, Math.round(right - left)),
    height: Math.max(1, Math.round(bottom - top)),
    displayWidth: Math.max(1, Math.round(imageBounds.width)),
    displayHeight: Math.max(1, Math.round(imageBounds.height)),
  }
}

function mapSelectionToSourcePixels(current: CaptureOverlaySession, visualRect: VisualSelectionRect) {
  const sourceSize = overlaySourceSize(current)
  const displayRect = { left: 0, top: 0, width: visualRect.displayWidth, height: visualRect.displayHeight }
  return mapVisualSelectionToSourcePixels(visualRect, sourceSize, displayRect, displayRect)
}

function scaleAnnotationOperations(operations: CaptureOverlayAnnotationOperation[], scaleX: number, scaleY: number) {
  const scaleWidth = Math.max(1, (Math.abs(scaleX) + Math.abs(scaleY)) / 2)
  return operations.map((operation) => ({
    ...operation,
    x: Math.round(operation.x * scaleX),
    y: Math.round(operation.y * scaleY),
    width: operation.width === undefined ? undefined : Math.round(operation.width * scaleX),
    height: operation.height === undefined ? undefined : Math.round(operation.height * scaleY),
    endX: operation.endX === undefined ? undefined : Math.round(operation.endX * scaleX),
    endY: operation.endY === undefined ? undefined : Math.round(operation.endY * scaleY),
    strokeWidth: operation.strokeWidth === undefined ? undefined : Math.max(1, Math.round(operation.strokeWidth * scaleWidth)),
    pixelSize: operation.pixelSize === undefined ? undefined : Math.max(1, Math.round(operation.pixelSize * scaleWidth)),
    fontSize: operation.fontSize === undefined ? undefined : Math.max(1, Math.round(operation.fontSize * scaleWidth)),
    points: operation.points?.map((point) => ({ x: Math.round(point.x * scaleX), y: Math.round(point.y * scaleY) })),
  }))
}

async function renderAnnotatedSelectionPNG(sessionRect: { x: number; y: number; width: number; height: number }, operations: CaptureOverlayAnnotationOperation[]) {
  const current = session.value
  if (!current?.imageUrl || !operations.length) return ''
  const width = Math.max(1, Math.round(sessionRect.width))
  const height = Math.max(1, Math.round(sessionRect.height))
  const image = await loadImage(current.imageUrl)
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) return ''
  context.drawImage(
    image,
    Math.round(sessionRect.x),
    Math.round(sessionRect.y),
    width,
    height,
    0,
    0,
    width,
    height,
  )
  const baseCanvas = document.createElement('canvas')
  baseCanvas.width = width
  baseCanvas.height = height
  baseCanvas.getContext('2d')?.drawImage(canvas, 0, 0)
  for (const operation of operations) {
    drawCanvasOperation(context, baseCanvas, operation)
  }
  return canvas.toDataURL('image/png').replace(/^data:image\/png;base64,/, '')
}

function loadImage(src: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('截图背景载入失败'))
    image.src = src
  })
}

function drawCanvasOperation(context: CanvasRenderingContext2D, baseCanvas: HTMLCanvasElement, operation: CaptureOverlayAnnotationOperation) {
  const strokeWidth = Math.max(1, Number(operation.strokeWidth ?? annotationThickness.value))
  const color = operation.color || annotationColor.value
  context.save()
  context.lineCap = 'round'
  context.lineJoin = 'round'
  context.strokeStyle = color
  context.fillStyle = color
  context.lineWidth = strokeWidth
  if (operation.kind === 'rect') {
    context.strokeRect(
      Math.round(operation.x) + 0.5,
      Math.round(operation.y) + 0.5,
      Math.round(operation.width ?? 0),
      Math.round(operation.height ?? 0),
    )
  } else if (operation.kind === 'line') {
    drawCanvasLine(context, operation.x, operation.y, operation.endX ?? operation.x, operation.endY ?? operation.y)
  } else if (operation.kind === 'arrow') {
    drawCanvasArrow(context, operation.x, operation.y, operation.endX ?? operation.x, operation.endY ?? operation.y, strokeWidth)
  } else if (operation.kind === 'pen') {
    drawCanvasPath(context, operation.points ?? [{ x: operation.x, y: operation.y }])
  } else if (operation.kind === 'mosaic') {
    applyCanvasMosaic(context, operation)
  } else if (operation.kind === 'eraser') {
    applyCanvasEraser(context, baseCanvas, operation, strokeWidth)
  } else if (operation.kind === 'text') {
    drawCanvasText(context, operation)
  } else if (operation.kind === 'number') {
    drawCanvasNumber(context, operation)
  }
  context.restore()
}

function drawCanvasLine(context: CanvasRenderingContext2D, x1: number, y1: number, x2: number, y2: number) {
  context.beginPath()
  context.moveTo(x1, y1)
  context.lineTo(x2, y2)
  context.stroke()
}

function drawCanvasArrow(context: CanvasRenderingContext2D, x1: number, y1: number, x2: number, y2: number, strokeWidth: number) {
  drawCanvasLine(context, x1, y1, x2, y2)
  const angle = Math.atan2(y2 - y1, x2 - x1)
  const headLength = Math.max(12, strokeWidth * 4)
  context.beginPath()
  context.moveTo(x2, y2)
  context.lineTo(x2 - Math.cos(angle - Math.PI / 6) * headLength, y2 - Math.sin(angle - Math.PI / 6) * headLength)
  context.moveTo(x2, y2)
  context.lineTo(x2 - Math.cos(angle + Math.PI / 6) * headLength, y2 - Math.sin(angle + Math.PI / 6) * headLength)
  context.stroke()
}

function drawCanvasPath(context: CanvasRenderingContext2D, points: Point[]) {
  if (!points.length) return
  context.beginPath()
  context.moveTo(points[0].x, points[0].y)
  for (const point of points.slice(1)) {
    context.lineTo(point.x, point.y)
  }
  context.stroke()
}

function applyCanvasMosaic(context: CanvasRenderingContext2D, operation: CaptureOverlayAnnotationOperation) {
  const points = operation.points ?? []
  const pixelSize = Math.max(6, Number(operation.pixelSize ?? annotationThickness.value * 4))
  if (points.length > 1) {
    forEachPathSample(points, Math.max(4, pixelSize / 2), (point) => {
      mosaicRect(context, point.x - pixelSize, point.y - pixelSize, pixelSize * 2, pixelSize * 2, pixelSize)
    })
    return
  }
  mosaicRect(context, operation.x, operation.y, operation.width ?? pixelSize * 2, operation.height ?? pixelSize * 2, pixelSize)
}

function mosaicRect(context: CanvasRenderingContext2D, x: number, y: number, width: number, height: number, pixelSize: number) {
  const left = Math.max(0, Math.floor(x))
  const top = Math.max(0, Math.floor(y))
  const right = Math.min(context.canvas.width, Math.ceil(x + width))
  const bottom = Math.min(context.canvas.height, Math.ceil(y + height))
  for (let blockY = top; blockY < bottom; blockY += pixelSize) {
    for (let blockX = left; blockX < right; blockX += pixelSize) {
      const blockWidth = Math.min(pixelSize, right - blockX)
      const blockHeight = Math.min(pixelSize, bottom - blockY)
      const color = averageCanvasColor(context, blockX, blockY, blockWidth, blockHeight)
      context.fillStyle = color
      context.fillRect(blockX, blockY, blockWidth, blockHeight)
    }
  }
}

function averageCanvasColor(context: CanvasRenderingContext2D, x: number, y: number, width: number, height: number) {
  const data = context.getImageData(x, y, Math.max(1, width), Math.max(1, height)).data
  let red = 0
  let green = 0
  let blue = 0
  const count = data.length / 4
  for (let index = 0; index < data.length; index += 4) {
    red += data[index]
    green += data[index + 1]
    blue += data[index + 2]
  }
  return `rgb(${Math.round(red / count)}, ${Math.round(green / count)}, ${Math.round(blue / count)})`
}

function applyCanvasEraser(
  context: CanvasRenderingContext2D,
  baseCanvas: HTMLCanvasElement,
  operation: CaptureOverlayAnnotationOperation,
  strokeWidth: number,
) {
  const points = operation.points ?? [{ x: operation.x, y: operation.y }]
  const radius = Math.max(4, strokeWidth * 2)
  forEachPathSample(points, Math.max(2, radius / 2), (point) => {
    context.save()
    context.beginPath()
    context.arc(point.x, point.y, radius, 0, Math.PI * 2)
    context.clip()
    context.drawImage(baseCanvas, 0, 0)
    context.restore()
  })
}

function forEachPathSample(points: Point[], step: number, callback: (point: Point) => void) {
  if (!points.length) return
  callback(points[0])
  for (let index = 1; index < points.length; index += 1) {
    const start = points[index - 1]
    const end = points[index]
    const distance = Math.max(Math.abs(end.x - start.x), Math.abs(end.y - start.y), 1)
    const samples = Math.max(1, Math.ceil(distance / step))
    for (let sample = 1; sample <= samples; sample += 1) {
      callback({
        x: start.x + ((end.x - start.x) * sample) / samples,
        y: start.y + ((end.y - start.y) * sample) / samples,
      })
    }
  }
}

function drawCanvasText(context: CanvasRenderingContext2D, operation: CaptureOverlayAnnotationOperation) {
  const fontSize = Math.max(12, Number(operation.fontSize ?? 20))
  context.font = `${fontSize}px "Microsoft YaHei", "Segoe UI", sans-serif`
  context.textBaseline = 'top'
  context.fillText(operation.text ?? '', operation.x, operation.y)
}

function drawCanvasNumber(context: CanvasRenderingContext2D, operation: CaptureOverlayAnnotationOperation) {
  const radius = Math.max(10, Number(operation.fontSize ?? 18))
  const label = String(operation.number ?? 1)
  context.beginPath()
  context.arc(operation.x, operation.y, radius, 0, Math.PI * 2)
  context.fillStyle = 'rgba(255, 255, 255, 0.92)'
  context.fill()
  context.stroke()
  context.font = `bold ${Math.max(12, radius)}px "Segoe UI", sans-serif`
  context.textAlign = 'center'
  context.textBaseline = 'middle'
  context.fillStyle = operation.color || annotationColor.value
  context.fillText(label, operation.x, operation.y + 0.5)
  context.textAlign = 'start'
}

async function copyPointerColor() {
  try {
    const pixel = samplePointerColor()
    if (!pixel) {
      showFeedback('取色失败')
      return
    }
    const text = formatPixelColor(pixel, colorFormat.value)
    await WailsClipboard.SetText(text)
    showFeedback(`颜色已复制: ${text}`)
  } catch {
    showFeedback('取色失败')
  }
}

function samplePointerColor() {
  const context = ensureSampleContext()
  const canvas = sampleCanvas
  const current = session.value
  if (!context || !canvas || !current) return null
  const x = Math.max(0, Math.min(canvas.width - 1, Math.round(sourcePointerX())))
  const y = Math.max(0, Math.min(canvas.height - 1, Math.round(sourcePointerY())))
  const pixel = context.getImageData(x, y, 1, 1).data
  return { red: pixel[0], green: pixel[1], blue: pixel[2] }
}

function ensureSampleContext() {
  const current = session.value
  if (!current?.imageUrl) return null
  if (sampleContext && sampleCanvas && sampleImageSrc === current.imageUrl) return sampleContext
  const imageElement = document.querySelector<HTMLImageElement>('.capture-overlay-image')
  if (!imageElement?.complete || !imageElement.naturalWidth || !imageElement.naturalHeight) return null
  sampleCanvas = document.createElement('canvas')
  sampleCanvas.width = imageElement.naturalWidth
  sampleCanvas.height = imageElement.naturalHeight
  sampleContext = sampleCanvas.getContext('2d', { willReadFrequently: true })
  sampleImageSrc = current.imageUrl
  sampleContext?.drawImage(imageElement, 0, 0)
  return sampleContext
}

function sourcePointerX() {
  const current = session.value
  if (!current) return currentMousePoint.value.x
  const bounds = overlayImageBoundsInSurface()
  const sourceSize = overlaySourceSize(current)
  const scaleX = sourceSize.width / Math.max(1, bounds.width)
  return (currentMousePoint.value.x - bounds.left) * scaleX
}

function sourcePointerY() {
  const current = session.value
  if (!current) return currentMousePoint.value.y
  const bounds = overlayImageBoundsInSurface()
  const sourceSize = overlaySourceSize(current)
  const scaleY = sourceSize.height / Math.max(1, bounds.height)
  return (currentMousePoint.value.y - bounds.top) * scaleY
}

function formatPixelColor(pixel: { red: number; green: number; blue: number }, format: 'hex' | 'rgb') {
  if (format === 'hex') {
    return `#${[pixel.red, pixel.green, pixel.blue].map((value) => value.toString(16).padStart(2, '0')).join('').toUpperCase()}`
  }
  return `rgb(${pixel.red}, ${pixel.green}, ${pixel.blue})`
}

function handleWheel(event: WheelEvent) {
  if (!editMode.value || !hasSelection.value) return
  event.preventDefault()
  annotationThickness.value = Math.max(1, Math.min(24, annotationThickness.value + (event.deltaY < 0 ? 1 : -1)))
  showFeedback(`粗细 ${annotationThickness.value}`)
}

function svgPoints(points?: Point[]) {
  return (points ?? []).map((point) => `${point.x},${point.y}`).join(' ')
}

function operationColor(operation: CaptureOverlayAnnotationOperation) {
  return operation.color || annotationColor.value
}

function operationStrokeWidth(operation: CaptureOverlayAnnotationOperation) {
  return Math.max(1, Number(operation.strokeWidth ?? annotationThickness.value))
}

function operationFontSize(operation: CaptureOverlayAnnotationOperation) {
  return Math.max(12, Number(operation.fontSize ?? 20))
}

function defaultCaptureFilename() {
  const now = new Date()
  const stamp = [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, '0'),
    String(now.getDate()).padStart(2, '0'),
    String(now.getHours()).padStart(2, '0'),
    String(now.getMinutes()).padStart(2, '0'),
    String(now.getSeconds()).padStart(2, '0'),
  ].join('')
  return `ariadne-capture-${stamp}.png`
}

function boundedPoint(event: PointerEvent) {
  const surface = overlaySurfaceRect()
  const bounds = overlayImageBoundsInSurface()
  const x = event.clientX - surface.left
  const y = event.clientY - surface.top
  return {
    x: Math.max(bounds.left, Math.min(bounds.left + bounds.width, x)),
    y: Math.max(bounds.top, Math.min(bounds.top + bounds.height, y)),
  }
}

function overlayViewport() {
  const bounds = overlaySurfaceRect()
  return {
    width: bounds.width || window.innerWidth || 1,
    height: bounds.height || window.innerHeight || 1,
  }
}

function overlaySourceSize(current: CaptureOverlaySession) {
  const image = document.querySelector<HTMLImageElement>('.capture-overlay-image')
  if (image?.naturalWidth && image.naturalHeight) {
    return {
      width: image.naturalWidth,
      height: image.naturalHeight,
    }
  }
  const nativeBounds = sessionNativeBounds(current)
  return {
    width: Math.max(1, nativeBounds.width),
    height: Math.max(1, nativeBounds.height),
  }
}

function overlayImageRect() {
  const image = document.querySelector<HTMLImageElement>('.capture-overlay-image')
  const bounds = image?.getBoundingClientRect()
  if (bounds?.width && bounds.height) return bounds
  return overlaySurfaceRect()
}

function overlayImageBoundsInSurface() {
  const image = overlayImageRect()
  const surface = overlaySurfaceRect()
  return {
    left: image.left - surface.left,
    top: image.top - surface.top,
    width: image.width || surface.width || window.innerWidth || 1,
    height: image.height || surface.height || window.innerHeight || 1,
  }
}

function overlaySurfaceRect() {
  return document.querySelector<HTMLElement>('.capture-overlay-surface')?.getBoundingClientRect() ?? {
    left: 0,
    top: 0,
    width: window.innerWidth || 1,
    height: window.innerHeight || 1,
  }
}

function sessionNativeBounds(current: CaptureOverlaySession): ScreenBounds {
  const bounds = current.nativeBounds
  if (bounds && bounds.width > 0 && bounds.height > 0) return bounds
  return current.bounds
}

function showFeedback(message: string) {
  feedback.value = message
  window.setTimeout(() => {
    if (feedback.value === message) feedback.value = ''
  }, 1800)
}

function clip(value: string, limit: number) {
  const text = value.trim().replace(/\s+/g, ' ')
  return text.length > limit ? `${text.slice(0, limit - 3)}...` : text
}

function clampNumber(value: number, min: number, max: number) {
  const lower = Math.min(min, max)
  const upper = Math.max(min, max)
  return Math.max(lower, Math.min(upper, value))
}
</script>

<template>
  <main
    class="capture-overlay-surface"
    aria-label="截图覆盖层"
    @pointerdown="beginSelection"
    @pointermove="moveSelection"
    @pointerup="endSelection"
    @pointercancel="endSelection"
    @lostpointercapture="cancelSelectionPointer"
    @contextmenu="handleContextMenu"
    @wheel="handleWheel"
  >
    <img v-if="session?.imageUrl" class="capture-overlay-image" :src="session.imageUrl" alt="" draggable="false" />
    <div class="capture-overlay-dim" />
    <div v-if="session?.imageUrl" class="capture-magnifier" :style="magnifierStyle" aria-hidden="true">
      <div class="capture-magnifier-lens">
        <img class="capture-magnifier-image" :src="session.imageUrl" alt="" :style="magnifierImageStyle" draggable="false" />
        <span class="capture-magnifier-crosshair is-horizontal" />
        <span class="capture-magnifier-crosshair is-vertical" />
      </div>
      <div class="capture-magnifier-meta">
        <span class="capture-magnifier-swatch" :style="{ background: pointerHexText }" />
        <strong>{{ pointerColorText }}</strong>
        <small>{{ colorFormat.toUpperCase() }} · C 复制 · Shift 切换</small>
      </div>
    </div>

    <div v-if="isLoading" class="capture-overlay-toast">载入屏幕快照</div>
    <div v-else-if="!session" class="capture-overlay-toast">
      截图会话已失效
      <button type="button" @click.stop="closeWindow(false)">关闭</button>
    </div>

    <div
      v-if="selection"
      class="capture-selection"
      :class="{ 'is-ready': hasSelection, 'is-editing': editMode, 'is-drawing': isSelecting, 'is-resizing': isResizingSelection, 'has-editable-annotations': canMoveAnnotations }"
      :style="selectionStyle"
    >
      <span v-if="hasSelection" class="capture-selection-size">
        {{ Math.round(selection.width) }} x {{ Math.round(selection.height) }}
        <template v-if="annotationOperations.length"> · {{ annotationOperations.length }} 标注</template>
      </span>
      <svg
        v-if="annotationPreviewOperations.length"
        class="capture-annotation-layer"
        :viewBox="selectionViewBox"
        preserveAspectRatio="none"
      >
        <defs>
          <pattern id="capture-mosaic-preview" width="12" height="12" patternUnits="userSpaceOnUse">
            <rect width="12" height="12" fill="rgba(15, 118, 110, 0.22)" />
            <path d="M 0 6 H 12 M 6 0 V 12" stroke="rgba(15, 118, 110, 0.44)" stroke-width="1" />
          </pattern>
          <marker id="capture-arrow-head" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto" markerUnits="strokeWidth">
            <path d="M 0 0 L 8 4 L 0 8 z" fill="#dc2626" />
          </marker>
        </defs>
        <template v-for="(operation, index) in annotationPreviewOperations" :key="`${operation.kind}-${index}`">
          <rect
            v-if="operation.kind === 'rect'"
            :class="['capture-annotation-rect', { 'is-selected': selectedAnnotationIndex === index }]"
            :x="operation.x"
            :y="operation.y"
            :width="operation.width"
            :height="operation.height"
            :stroke="operationColor(operation)"
            :stroke-width="operationStrokeWidth(operation)"
          />
          <line
            v-else-if="operation.kind === 'line'"
            :class="['capture-annotation-line', { 'is-selected': selectedAnnotationIndex === index }]"
            :x1="operation.x"
            :y1="operation.y"
            :x2="operation.endX"
            :y2="operation.endY"
            :stroke="operationColor(operation)"
            :stroke-width="operationStrokeWidth(operation)"
          />
          <line
            v-else-if="operation.kind === 'arrow'"
            :class="['capture-annotation-arrow', { 'is-selected': selectedAnnotationIndex === index }]"
            :x1="operation.x"
            :y1="operation.y"
            :x2="operation.endX"
            :y2="operation.endY"
            :stroke="operationColor(operation)"
            :stroke-width="operationStrokeWidth(operation)"
          />
          <polyline
            v-else-if="operation.kind === 'pen'"
            :class="['capture-annotation-pen', { 'is-selected': selectedAnnotationIndex === index }]"
            :points="svgPoints(operation.points)"
            :stroke="operationColor(operation)"
            :stroke-width="operationStrokeWidth(operation)"
          />
          <polyline
            v-else-if="operation.kind === 'mosaic' && operation.points?.length"
            :class="['capture-annotation-mosaic-path', { 'is-selected': selectedAnnotationIndex === index }]"
            :points="svgPoints(operation.points)"
            :stroke-width="Math.max(10, operationStrokeWidth(operation) * 4)"
          />
          <rect
            v-else-if="operation.kind === 'mosaic'"
            :class="['capture-annotation-mosaic', { 'is-selected': selectedAnnotationIndex === index }]"
            :x="operation.x"
            :y="operation.y"
            :width="operation.width"
            :height="operation.height"
          />
          <polyline
            v-else-if="operation.kind === 'eraser'"
            :class="['capture-annotation-eraser', { 'is-selected': selectedAnnotationIndex === index }]"
            :points="svgPoints(operation.points)"
            :stroke-width="Math.max(8, operationStrokeWidth(operation) * 4)"
          />
          <text
            v-else-if="operation.kind === 'text'"
            :class="['capture-annotation-text', { 'is-selected': selectedAnnotationIndex === index }]"
            :x="operation.x"
            :y="operation.y"
            :fill="operationColor(operation)"
            :font-size="operationFontSize(operation)"
          >
            {{ operation.text }}
          </text>
          <g v-else-if="operation.kind === 'number'" :class="['capture-annotation-number', { 'is-selected': selectedAnnotationIndex === index }]">
            <circle
              :cx="operation.x"
              :cy="operation.y"
              :r="Math.max(12, operationFontSize(operation))"
              fill="rgba(255, 255, 255, 0.92)"
              :stroke="operationColor(operation)"
              :stroke-width="operationStrokeWidth(operation)"
            />
            <text
              :x="operation.x"
              :y="operation.y"
              :fill="operationColor(operation)"
              :font-size="operationFontSize(operation)"
              text-anchor="middle"
              dominant-baseline="central"
              font-weight="700"
            >
              {{ operation.number }}
            </text>
          </g>
        </template>
      </svg>
      <button
        v-for="handle in resizeHandles"
        :key="handle.anchor"
        class="capture-selection-handle"
        :class="`is-${handle.anchor}`"
        type="button"
        :title="handle.label"
        :aria-label="handle.label"
        @pointerdown.stop.prevent="beginResizeSelection(handle.anchor, $event)"
        @pointermove.stop.prevent="moveResizeSelection"
        @pointerup.stop.prevent="endResizeSelection"
        @pointercancel.stop.prevent="endResizeSelection"
        @lostpointercapture.stop.prevent="cancelResizeSelection"
      />
      <button
        v-if="canMoveAnnotations"
        class="capture-annotation-interaction-canvas"
        type="button"
        title="选择并拖动已有标注，双击文字可编辑"
        aria-label="选择并拖动已有标注"
        @pointerdown.stop="beginMoveAnnotation"
        @pointermove.stop="moveSelectedAnnotation"
        @pointerup.stop="endMoveAnnotation"
        @pointercancel.stop="endMoveAnnotation"
        @lostpointercapture.stop="cancelMoveAnnotation"
        @dblclick.stop.prevent="editTextAnnotationAtPoint"
      />
      <button
        v-if="editMode"
        class="capture-annotation-canvas"
        type="button"
        :title="annotationToolLabel(editTool)"
        :aria-label="annotationToolLabel(editTool)"
        @pointerdown.stop.prevent="beginAnnotation"
        @pointermove.stop="moveAnnotation"
        @pointerup.stop="endAnnotation"
        @pointercancel.stop="endAnnotation"
        @lostpointercapture.stop="cancelAnnotationPointer"
      />
      <input
        v-if="textEditor"
        ref="textInputRef"
        v-model="textEditor.text"
        class="capture-text-editor"
        :style="{ left: `${textEditor.x}px`, top: `${textEditor.y}px`, color: annotationColor, fontSize: `${Math.max(16, 12 + annotationThickness * 3)}px` }"
        type="text"
        @pointerdown.stop
        @keydown.enter.prevent.stop="commitTextAnnotation"
        @keydown.escape.prevent.stop="textEditor = null"
        @blur="commitTextAnnotation"
      />
    </div>

    <div v-if="canShowToolbar" class="capture-overlay-toolbar" :style="toolbarStyle" @pointerdown.stop>
      <button type="button" :disabled="isBusy" title="识别二维码" @click="runSelectionAction('qr')">
        <QrCode :size="14" />
        二维码
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'rect' }"
        :disabled="isBusy"
        title="矩形标注"
        @click="toggleAnnotationTool('rect')"
      >
        <Square :size="14" />
        矩形
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'arrow' }"
        :disabled="isBusy"
        title="箭头标注"
        @click="toggleAnnotationTool('arrow')"
      >
        <ArrowUpRight :size="14" />
        箭头
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'line' }"
        :disabled="isBusy"
        title="直线"
        @click="toggleAnnotationTool('line')"
      >
        <Minus :size="14" />
        直线
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'pen' }"
        :disabled="isBusy"
        title="画笔"
        @click="toggleAnnotationTool('pen')"
      >
        <Pencil :size="14" />
        画笔
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'mosaic' }"
        :disabled="isBusy"
        title="马赛克"
        @click="toggleAnnotationTool('mosaic')"
      >
        <Grid3X3 :size="14" />
        马赛克
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'text' }"
        :disabled="isBusy"
        title="文字"
        @click="toggleAnnotationTool('text')"
      >
        <Type :size="14" />
        文字
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'number' }"
        :disabled="isBusy"
        title="序号"
        @click="toggleAnnotationTool('number')"
      >
        <Hash :size="14" />
        序号
      </button>
      <button
        type="button"
        :class="{ 'is-active': editMode && editTool === 'eraser' }"
        :disabled="isBusy"
        title="橡皮擦"
        @click="toggleAnnotationTool('eraser')"
      >
        <Eraser :size="14" />
        橡皮
      </button>
      <button
        type="button"
        :class="{ 'is-active': !editMode && selectedAnnotationIndex !== null }"
        :disabled="isBusy || !annotationOperations.length"
        title="选择/移动已有标注"
        @click="activateAnnotationSelect"
      >
        选择
      </button>
      <div class="capture-color-strip" role="group" aria-label="标注颜色">
        <button
          v-for="color in colorPalette"
          :key="color"
          class="capture-color-swatch"
          :class="{ 'is-active': annotationColor === color }"
          type="button"
          :title="color"
          :style="{ '--swatch-color': color }"
          @click="annotationColor = color"
        />
      </div>
      <label class="capture-thickness-control" title="鼠标滚轮也可调节粗细">
        <span>粗细</span>
        <input v-model.number="annotationThickness" type="range" min="1" max="24" step="1" />
        <strong>{{ annotationThickness }}</strong>
      </label>
      <button type="button" :disabled="isBusy || !annotationOperations.length" title="撤销标注" @click="undoAnnotation">
        <RotateCcw :size="14" />
      </button>
      <button type="button" :disabled="isBusy || !redoAnnotationOperations.length" title="重做标注" @click="redoAnnotation">
        <Redo2 :size="14" />
      </button>
      <button type="button" :disabled="isBusy || !annotationOperations.length" title="清空标注" @click="clearAnnotations">
        清空
      </button>
      <button type="button" :disabled="isBusy || selectedAnnotationIndex === null" title="删除选中标注" @click="deleteSelectedAnnotation">
        删除
      </button>
      <button type="button" :disabled="isBusy" title="重新选择" @click="resetSelection">
        <X :size="14" />
      </button>
      <button type="button" :disabled="isBusy" title="保存到截图历史" @click="runSelectionAction('capture')">
        <Check :size="14" />
        保存
      </button>
      <button type="button" :disabled="isBusy" title="另存为" @click="runSaveAsAction">
        <Save :size="14" />
        另存
      </button>
      <button type="button" :disabled="isBusy" title="复制到剪贴板 (Enter)" @click="runSelectionAction('copy')">
        <Copy :size="14" />
        复制
      </button>
      <button type="button" :disabled="isBusy" title="贴图 (P)" @click="runSelectionAction('pin')">
        <Pin :size="14" />
        贴图
      </button>
    </div>

    <button class="capture-overlay-close" type="button" title="退出" @click.stop="closeWindow(true)">
      <X :size="16" />
    </button>

    <div class="capture-overlay-hint">
      <span>拖拽选择区域</span>
      <kbd>Enter</kbd>
      <span>复制</span>
      <kbd>P</kbd>
      <span>贴图</span>
      <kbd>Q</kbd>
      <span>扫码</span>
      <kbd>R/A/M</kbd>
      <span>标注</span>
      <kbd>T/N/E</kbd>
      <span>文字/序号/橡皮</span>
      <kbd>V</kbd>
      <span>选标注</span>
      <kbd>Del</kbd>
      <span>删标注</span>
      <kbd>C</kbd>
      <span>取色</span>
      <kbd>Shift</kbd>
      <span>{{ pointerRgbText }} / {{ pointerHexText }}</span>
      <kbd>Ctrl+S</kbd>
      <span>另存</span>
      <kbd>Esc</kbd>
      <span>退出</span>
    </div>

    <div v-if="feedback" class="capture-overlay-feedback">{{ feedback }}</div>
  </main>
</template>
