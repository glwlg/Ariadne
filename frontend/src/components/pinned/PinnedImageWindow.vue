<script setup lang="ts">
import { Copy, FileText, Maximize2, Minus, Pin, Plus, ScanLine, X } from '@lucide/vue'
import { Clipboard, Window } from '@wailsio/runtime'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { copyClipboardImage } from '../../services/clipboardApi'
import { recognizeCaptureOCR, recognizeClipboardImageOCR } from '../../services/ocrApi'
import { closePinnedImage, getPinnedImage } from '../../services/pinnedImageApi'
import { createOCRSelection } from '../../lib/ocrSelection'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import type { OCRResult, PinnedImage } from '../../types/ariadne'

const props = defineProps<{
  pinId: string
}>()

const image = ref<PinnedImage | null>(null)
const zoom = ref(1)
const shadowEnabled = ref(false)
const feedback = ref('')
const ocrResult = ref<OCRResult | null>(null)
const isRecognizingOCR = ref(false)
const isLoading = ref(true)
const contextMenu = ref({ visible: false, x: 0, y: 0 })
const menuExpanded = ref(false)
const ocrSelection = createOCRSelection(ocrResult)
const ocrLines = ocrSelection.ocrLines
const selectedOCRLineCount = ocrSelection.selectedOCRLineCount
const imageStyle = computed(() => ({
  transform: `scale(${zoom.value})`,
}))

const contentSize = computed(() => {
  const current = image.value
  const windowWidth = Math.max(1, Number(current?.windowWidth || current?.width || 1))
  const windowHeight = Math.max(1, Number(current?.windowHeight || current?.height || 1))
  return {
    width: windowWidth,
    height: windowHeight,
  }
})

const contentStyle = computed(() => ({
  '--pin-content-width': `${contentSize.value.width}px`,
  '--pin-content-height': `${contentSize.value.height}px`,
}))

const ocrMaxHeight = computed(() => Math.max(120, contentSize.value.height))

const ocrStatusText = computed(() => {
  if (!ocrResult.value) return ''
  if (!ocrResult.value.ok) return ocrResult.value.error || 'OCR 不可用'
  if (!ocrLines.value.length) return ocrResult.value.text ? '1 段文字' : '未识别到文字'
  return `${ocrLines.value.length} 行 · 已选 ${selectedOCRLineCount.value}`
})

const copySourceLabel = computed(() => {
  const current = image.value
  if (!current) return '复制'
  if (current.text) return '复制文本'
  if (current.canCopy) return '复制图片'
  if (current.imagePath) return '复制图片路径'
  return '复制'
})

const contextActions = computed(() => {
  const current = image.value
  return [
    {
      id: 'copy-source',
      label: copySourceLabel.value,
      icon: Copy,
      disabled: !current || (!current.canCopy && !current.imagePath),
      run: copySource,
    },
    {
      id: 'ocr',
      label: isRecognizingOCR.value ? 'OCR 中' : 'OCR 文字识别',
      icon: FileText,
      disabled: !current?.canOcr || isRecognizingOCR.value,
      run: recognizeOCR,
    },
    {
      id: 'copy-selected-ocr',
      label: '复制选中 OCR',
      icon: Copy,
      disabled: !selectedOCRLineCount.value,
      run: copySelectedOCRText,
    },
    {
      id: 'copy-full-ocr',
      label: '复制 OCR 全文',
      icon: Copy,
      disabled: !ocrResult.value?.text,
      run: copyFullOCRText,
    },
    {
      id: 'zoom-in',
      label: '放大',
      icon: Plus,
      disabled: zoom.value >= 3,
      run: () => zoomBy(0.1),
    },
    {
      id: 'zoom-out',
      label: '缩小',
      icon: Minus,
      disabled: zoom.value <= 0.25,
      run: () => zoomBy(-0.1),
    },
    {
      id: 'reset-zoom',
      label: '原始比例',
      icon: Maximize2,
      disabled: zoom.value === 1,
      run: resetZoom,
    },
    {
      id: 'shadow',
      label: shadowEnabled.value ? '关闭阴影' : '打开阴影',
      icon: ScanLine,
      disabled: !current,
      run: toggleShadow,
    },
    {
      id: 'close',
      label: '关闭贴图',
      icon: X,
      disabled: false,
      run: closeWindow,
    },
  ]
})

onMounted(async () => {
  document.documentElement.classList.add('pinned-image-document')
  await prepareWindow()
  image.value = await getPinnedImage(props.pinId)
  isLoading.value = false
  if (image.value?.windowWidth && image.value.windowHeight) {
    try {
      await Window.SetSize(image.value.windowWidth, image.value.windowHeight)
    } catch {
      // Runtime calls are unavailable in browser-only dev mode.
    }
  }
  window.addEventListener('keydown', handleKeyDown)
})

onBeforeUnmount(() => {
  document.documentElement.classList.remove('pinned-image-document')
  window.removeEventListener('keydown', handleKeyDown)
})

async function prepareWindow() {
  try {
    await Window.SetFrameless(true)
    await Window.SetAlwaysOnTop(true)
    await Window.SetBackgroundColour(0, 0, 0, 0)
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

async function copySource() {
  const current = image.value
  if (!current) return
  if (current.text) {
    try {
      await Clipboard.SetText(current.text)
      showFeedback('文本已复制')
    } catch {
      showFeedback('复制失败')
    }
    return
  }
  if (current.source === 'clipboard' && current.sourceId) {
    const result = await copyClipboardImage(current.sourceId)
    showFeedback(result.ok ? '图片已复制' : result.message || '复制失败')
    return
  }
  if (current.imagePath) {
    try {
      await Clipboard.SetText(current.imagePath)
      showFeedback('路径已复制')
    } catch {
      showFeedback('复制失败')
    }
  }
}

async function recognizeOCR() {
  const current = image.value
  if (!current?.canOcr || !current.sourceId || isRecognizingOCR.value) return
  isRecognizingOCR.value = true
  try {
    const result =
      current.source === 'capture'
        ? await recognizeCaptureOCR(current.sourceId)
        : current.source === 'clipboard'
          ? await recognizeClipboardImageOCR(current.sourceId)
          : null
    if (!result) {
      showFeedback('当前贴图不支持 OCR')
      return
    }
    ocrResult.value = result
    ocrSelection.clearOCRLineSelection()
    showFeedback(result.ok ? (result.text ? 'OCR 已完成' : '未识别到文字') : result.error || 'OCR 不可用')
  } catch {
    showFeedback('OCR 识别失败')
  } finally {
    isRecognizingOCR.value = false
  }
}

async function copySelectedOCRText() {
  const text = ocrSelection.selectedOCRText.value
  if (!text) {
    showFeedback('先选择 OCR 行')
    return
  }
  try {
    await Clipboard.SetText(text)
    showFeedback('已复制选中文字')
  } catch {
    showFeedback('复制失败')
  }
}

async function copyFullOCRText() {
  const text = ocrResult.value?.text
  if (!text) {
    showFeedback('没有可复制的 OCR 文本')
    return
  }
  try {
    await Clipboard.SetText(text)
    showFeedback('OCR 文本已复制')
  } catch {
    showFeedback('复制失败')
  }
}

function zoomBy(delta: number) {
  zoom.value = clampZoom(zoom.value + delta)
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  zoom.value = clampZoom(zoom.value + (event.deltaY < 0 ? 0.08 : -0.08))
}

function resetZoom() {
  zoom.value = 1
}

function handleSurfacePointerDown(event: PointerEvent) {
  if (contextMenu.value.visible) {
    if (!isContextMenuTarget(event.target)) {
      closeContextMenu()
    }
  }
}

function isContextMenuTarget(target: EventTarget | null) {
  const element = target instanceof HTMLElement ? target : null
  return Boolean(element?.closest('.pinned-image-context-menu'))
}

function toggleShadow() {
  shadowEnabled.value = !shadowEnabled.value
}

async function closeWindow() {
  await closePinnedImage(props.pinId)
  try {
    await Window.Close()
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function handleKeyDown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (contextMenu.value.visible) {
      closeContextMenu()
      return
    }
    void closeWindow()
  }
  if (event.key === '0' && (event.ctrlKey || event.metaKey)) {
    resetZoom()
  }
  if ((event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey)) && image.value) {
    event.preventDefault()
    void openContextMenuAt(Math.round(contentSize.value.width / 2), Math.round(contentSize.value.height / 2))
  }
}

function openContextMenu(event: MouseEvent) {
  event.preventDefault()
  event.stopPropagation()
  void openContextMenuAt(event.clientX, event.clientY)
}

async function openContextMenuAt(x: number, y: number) {
  const menuWidth = 190
  const menuHeight = Math.min(330, 34 * contextActions.value.length + 12)
  await expandWindowForContextMenu(x, y, menuWidth, menuHeight)
  const viewportWidth = Math.max(window.innerWidth, Math.ceil(x + menuWidth + 8))
  const viewportHeight = Math.max(window.innerHeight, Math.ceil(y + menuHeight + 8))
  contextMenu.value = {
    visible: true,
    x: Math.max(0, Math.min(x, viewportWidth - menuWidth - 4)),
    y: Math.max(0, Math.min(y, viewportHeight - menuHeight - 4)),
  }
}

async function expandWindowForContextMenu(x: number, y: number, menuWidth: number, menuHeight: number) {
  const current = image.value
  if (!current) return
  const baseWidth = Math.max(1, current.windowWidth || current.width)
  const baseHeight = Math.max(1, current.windowHeight || current.height)
  const nextWidth = Math.max(baseWidth, Math.ceil(x + menuWidth + 8))
  const nextHeight = Math.max(baseHeight, Math.ceil(y + menuHeight + 8))
  if (nextWidth === baseWidth && nextHeight === baseHeight) {
    menuExpanded.value = false
    return
  }
  try {
    await Window.SetSize(nextWidth, nextHeight)
    menuExpanded.value = true
  } catch {
    menuExpanded.value = false
  }
}

function closeContextMenu() {
  contextMenu.value.visible = false
  if (menuExpanded.value) {
    menuExpanded.value = false
    void restoreImageWindowSize()
  }
}

async function restoreImageWindowSize() {
  const current = image.value
  if (!current) return
  const width = Math.max(1, current.windowWidth || current.width)
  const height = Math.max(1, current.windowHeight || current.height)
  try {
    await Window.SetSize(width, height)
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

async function runContextAction(action: { disabled: boolean; run: () => void | Promise<void> }) {
  if (action.disabled) return
  closeContextMenu()
  await action.run()
}

function showFeedback(message: string) {
  feedback.value = message
  window.setTimeout(() => {
    if (feedback.value === message) feedback.value = ''
  }, 1400)
}

function clampZoom(value: number) {
  return Math.min(3, Math.max(0.25, Number(value.toFixed(2))))
}
</script>

<template>
  <main
    class="pinned-image-surface"
    :class="{ 'has-shadow': shadowEnabled }"
    aria-label="贴图窗口"
    @pointerdown="handleSurfacePointerDown"
    @wheel="onWheel"
    @contextmenu="openContextMenu"
    @dblclick="closeWindow"
  >
    <div v-if="isLoading" class="pinned-image-empty">
      <Pin :size="20" />
      <span>载入贴图</span>
    </div>

    <template v-else-if="image">
      <figure class="pinned-image-stage" :style="contentStyle">
        <div
          class="pinned-image-zoom-layer"
          :style="imageStyle"
        >
          <OCRImageOverlay
            :src="image.dataUrl"
            :width="ocrResult?.width || image.width"
            :height="ocrResult?.height || image.height"
            :lines="ocrLines"
            :is-line-selected="ocrSelection.isOCRLineSelected"
            :max-height="ocrMaxHeight"
            @toggle-line="ocrSelection.toggleOCRLine"
          />
        </div>
      </figure>

      <div v-if="ocrResult" class="pinned-image-ocr-strip" @pointerdown.stop @dblclick.stop>
        <span>{{ ocrStatusText }}</span>
        <button v-if="ocrLines.length" type="button" @click.stop="ocrSelection.selectAllOCRLines()">全选</button>
        <button v-if="ocrLines.length" type="button" @click.stop="ocrSelection.clearOCRLineSelection()">清空</button>
        <button
          v-if="ocrLines.length"
          type="button"
          :disabled="!selectedOCRLineCount"
          @click.stop="copySelectedOCRText"
        >
          复制选中
        </button>
        <button v-if="ocrResult.text" type="button" @click.stop="copyFullOCRText">复制全文</button>
      </div>

      <div v-if="feedback" class="pinned-image-feedback">{{ feedback }}</div>
      <div
        v-if="contextMenu.visible"
        class="pinned-image-context-menu"
        :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
        role="menu"
        aria-label="贴图右键菜单"
        @pointerdown.stop
        @dblclick.stop
        @wheel.stop
        @contextmenu.prevent
      >
        <button
          v-for="action in contextActions"
          :key="action.id"
          type="button"
          role="menuitem"
          :disabled="action.disabled"
          @click.stop="runContextAction(action)"
        >
          <component :is="action.icon" :size="13" />
          <span>{{ action.label }}</span>
        </button>
      </div>
    </template>

    <div v-else class="pinned-image-empty">
      <Pin :size="20" />
      <span>贴图已失效</span>
      <button type="button" @click="closeWindow">关闭</button>
    </div>
  </main>
</template>
