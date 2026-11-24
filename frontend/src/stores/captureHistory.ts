import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import {
  captureCurrentScreen,
  clearUnpinnedCaptureEntries,
  deleteCaptureEntry,
  getCaptureThumbnailDataURL,
  getCaptureStatus,
  listCaptureEntries,
  toggleCapturePin,
} from '../services/captureApi'
import { openCaptureOverlay } from '../services/captureOverlayApi'
import { executeFancunAction } from '../services/fancunApi'
import { openPinnedCapture } from '../services/pinnedImageApi'
import { decodeCaptureQRCode, decodeCurrentScreenQRCode } from '../services/qrScanApi'
import { recognizeCaptureOCR, recognizeCurrentScreenOCR } from '../services/ocrApi'
import { createOCRSelection } from '../lib/ocrSelection'
import type { CaptureHistoryEntry, CaptureHistoryStatus, OCRResult, PreviewAction, QRScanResult } from '../types/fancun'

const SEARCH_DEBOUNCE_MS = 120
const ARMED_TIMEOUT_MS = 5000

export const useCaptureHistoryStore = defineStore('capture-history', () => {
  const query = ref('')
  const entries = ref<CaptureHistoryEntry[]>([])
  const status = ref<CaptureHistoryStatus | null>(null)
  const selectedId = ref('')
  const imageDataUrl = ref('')
  const feedback = ref('')
  const feedbackTone = ref<'success' | 'danger'>('success')
  const loadError = ref('')
  const qrResult = ref<QRScanResult | null>(null)
  const ocrResult = ref<OCRResult | null>(null)
  const deleteArmedId = ref('')
  const clearArmed = ref(false)
  const isLoading = ref(false)
  const isLoadingImage = ref(false)
  const isCapturing = ref(false)
  const isScanningQR = ref(false)
  const isRecognizingOCR = ref(false)
  const ocrSelection = createOCRSelection(ocrResult)
  let loadSequence = 0
  let imageSequence = 0
  let ocrSequence = 0
  let qrSequence = 0
  let loadedImageId = ''
  let pendingImageId = ''
  let pendingImage: Promise<void> | null = null
  let queryDebounceTimer: number | undefined
  let feedbackTimer: number | undefined
  let deleteArmedTimer: number | undefined
  let clearArmedTimer: number | undefined

  const selectedEntry = computed(() => entries.value.find((entry) => entry.id === selectedId.value) ?? entries.value[0] ?? null)
  const pinnedCount = computed(() => status.value?.pinnedCount ?? entries.value.filter((entry) => entry.pinned).length)

  function clearQueryDebounce() {
    if (queryDebounceTimer !== undefined) {
      window.clearTimeout(queryDebounceTimer)
      queryDebounceTimer = undefined
    }
  }

  async function load(nextQuery = query.value) {
    clearQueryDebounce()
    const sequence = ++loadSequence
    isLoading.value = true
    loadError.value = ''
    try {
      query.value = nextQuery
      const [nextStatus, nextEntries] = await Promise.all([
        getCaptureStatus(),
        listCaptureEntries(nextQuery, 300),
      ])
      if (sequence !== loadSequence) return
      status.value = nextStatus
      entries.value = nextEntries
      if (!entries.value.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      await loadImage(selectedId.value)
    } catch (error) {
      if (sequence === loadSequence) {
        loadError.value = formatError(error, '截图历史加载失败')
        showFeedback(`截图历史加载失败: ${shortError(loadError.value)}`, 'danger')
      }
    } finally {
      if (sequence === loadSequence) {
        isLoading.value = false
      }
    }
  }

  function select(id: string) {
    if (selectedId.value !== id) {
      ocrSequence++
      qrSequence++
      isRecognizingOCR.value = false
      isScanningQR.value = false
    }
    selectedId.value = id
    disarmDelete()
    qrResult.value = null
    ocrResult.value = null
    ocrSelection.clearOCRLineSelection()
    void loadImage(id)
  }

  function setQuery(value: string) {
    clearQueryDebounce()
    query.value = value
    if (!value.trim()) {
      return load(value)
    }
    return new Promise<void>((resolve) => {
      queryDebounceTimer = window.setTimeout(() => {
        queryDebounceTimer = undefined
        void load(value).finally(resolve)
      }, SEARCH_DEBOUNCE_MS)
    })
  }

  async function captureScreen() {
    isCapturing.value = true
    try {
      status.value = await captureCurrentScreen('manual')
      const error = status.value.lastCaptureError || status.value.lastSaveError
      showFeedback(error ? `捕获失败: ${shortError(error)}` : '已捕获当前屏幕', error ? 'danger' : 'success')
      await load(query.value)
    } catch (error) {
      showFeedback(`捕获失败: ${shortError(formatError(error, '捕获失败'))}`, 'danger')
    } finally {
      isCapturing.value = false
    }
  }

  async function openOverlay() {
    try {
      const result = await openCaptureOverlay()
      showFeedback(result.message || (result.ok ? '已打开截图覆盖层' : '打开截图覆盖层失败'), result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(`打开截图覆盖层失败: ${shortError(formatError(error, '覆盖层不可用'))}`, 'danger')
    }
  }

  async function copyPath(entry = selectedEntry.value) {
    if (!entry) return
    try {
      await Clipboard.SetText(entry.imagePath)
      showFeedback('已复制路径', 'success')
    } catch (error) {
      try {
        await navigator.clipboard?.writeText(entry.imagePath)
        showFeedback('已复制路径', 'success')
      } catch {
        showFeedback(`复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
      }
    }
  }

  async function openImage(entry = selectedEntry.value) {
    if (!entry) return
    await executePreviewAction({
      id: 'open_capture',
      label: '打开',
      kind: 'open',
      payload: { path: entry.savedPath || entry.imagePath },
      feedback: { successLabel: '已打开', durationMs: 1400 },
    })
  }

  async function openFolder(entry = selectedEntry.value) {
    if (!entry) return
    await executePreviewAction({
      id: 'open_capture_parent',
      label: '打开所在文件夹',
      kind: 'open_parent',
      payload: { path: entry.imagePath },
      feedback: { successLabel: '已打开所在文件夹', durationMs: 1400 },
    })
  }

  async function togglePin(entry = selectedEntry.value) {
    if (!entry) return
    try {
      status.value = await toggleCapturePin(entry.id)
      showFeedback(
        status.value.lastSaveError ? `置顶失败: ${shortError(status.value.lastSaveError)}` : entry.pinned ? '已取消置顶' : '已置顶',
        status.value.lastSaveError ? 'danger' : 'success',
      )
      await load(query.value)
    } catch (error) {
      showFeedback(`置顶失败: ${shortError(formatError(error, '操作失败'))}`, 'danger')
    }
  }

  async function pinImage(entry = selectedEntry.value) {
    if (!entry) return
    try {
      const result = await openPinnedCapture(entry.id)
      showFeedback(result.message || (result.ok ? '已创建贴图' : '创建贴图失败'), result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(`创建贴图失败: ${shortError(formatError(error, '贴图服务不可用'))}`, 'danger')
    }
  }

  async function scanQRCode(entry = selectedEntry.value) {
    if (!entry) return
    const targetId = entry.id
    const seq = ++qrSequence
    isScanningQR.value = true
    try {
      const result = await decodeCaptureQRCode(targetId)
      if (seq !== qrSequence || selectedId.value !== targetId) return
      qrResult.value = result
      showFeedback(result.ok ? '已识别二维码' : result.error || '未识别到二维码', result.ok ? 'success' : 'danger')
    } catch (error) {
      if (seq === qrSequence && selectedId.value === targetId) {
        showFeedback(`二维码识别失败: ${shortError(formatError(error, '二维码识别失败'))}`, 'danger')
      }
    } finally {
      if (seq === qrSequence) {
        isScanningQR.value = false
      }
    }
  }

  async function scanCurrentScreenQRCode() {
    const seq = ++qrSequence
    isScanningQR.value = true
    try {
      const result = await decodeCurrentScreenQRCode()
      if (seq !== qrSequence) return
      qrResult.value = result
      showFeedback(result.ok ? '已识别当前屏幕二维码' : result.error || '未识别到二维码', result.ok ? 'success' : 'danger')
      await load(query.value)
    } catch (error) {
      if (seq === qrSequence) {
        showFeedback(`二维码识别失败: ${shortError(formatError(error, '二维码识别失败'))}`, 'danger')
      }
    } finally {
      if (seq === qrSequence) {
        isScanningQR.value = false
      }
    }
  }

  async function copyQRText() {
    const text = qrResult.value?.text
    if (!text) return
    try {
      await Clipboard.SetText(text)
      showFeedback('二维码内容已复制', 'success')
    } catch (error) {
      showFeedback(`复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
    }
  }

  async function recognizeText(entry = selectedEntry.value) {
    if (!entry) return
    const targetId = entry.id
    const seq = ++ocrSequence
    isRecognizingOCR.value = true
    try {
      const result = await recognizeCaptureOCR(targetId)
      if (seq !== ocrSequence || selectedId.value !== targetId) return
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(
        result.ok ? (result.text ? '已识别截图文字' : '未识别到文字') : result.error || 'OCR 不可用',
        result.ok ? 'success' : 'danger',
      )
    } catch (error) {
      if (seq === ocrSequence && selectedId.value === targetId) {
        showFeedback(`OCR 识别失败: ${shortError(formatError(error, 'OCR 识别失败'))}`, 'danger')
      }
    } finally {
      if (seq === ocrSequence) {
        isRecognizingOCR.value = false
      }
    }
  }

  async function recognizeCurrentScreenText() {
    const seq = ++ocrSequence
    isRecognizingOCR.value = true
    try {
      const result = await recognizeCurrentScreenOCR()
      if (seq !== ocrSequence) return
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(
        result.ok ? (result.text ? '已识别当前屏幕文字' : '未识别到文字') : result.error || 'OCR 不可用',
        result.ok ? 'success' : 'danger',
      )
      await load(query.value)
    } catch (error) {
      if (seq === ocrSequence) {
        showFeedback(`OCR 识别失败: ${shortError(formatError(error, 'OCR 识别失败'))}`, 'danger')
      }
    } finally {
      if (seq === ocrSequence) {
        isRecognizingOCR.value = false
      }
    }
  }

  async function copyOCRText() {
    const text = ocrResult.value?.text
    if (!text) return
    try {
      await Clipboard.SetText(text)
      showFeedback('OCR 文本已复制', 'success')
    } catch (error) {
      showFeedback(`复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
    }
  }

  async function copySelectedOCRText() {
    const text = ocrSelection.selectedOCRText.value
    if (!text) {
      showFeedback('先选择 OCR 行', 'danger')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('已复制选中文字', 'success')
    } catch (error) {
      showFeedback(`复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
    }
  }

  async function deleteEntry(entry = selectedEntry.value) {
    if (!entry) return
    disarmClear()
    if (deleteArmedId.value !== entry.id) {
      armDelete(entry.id)
      showFeedback('再次点击确认删除', 'danger')
      return
    }
    disarmDelete()
    try {
      status.value = await deleteCaptureEntry(entry.id)
      showFeedback(
        status.value.lastSaveError ? `删除失败: ${shortError(status.value.lastSaveError)}` : '已删除',
        status.value.lastSaveError ? 'danger' : 'success',
      )
      await load(query.value)
    } catch (error) {
      showFeedback(`删除失败: ${shortError(formatError(error, '删除失败'))}`, 'danger')
    }
  }

  async function clearUnpinned() {
    disarmDelete()
    if (!clearArmed.value) {
      armClear()
      showFeedback('再次点击确认清空未置顶', 'danger')
      return
    }
    disarmClear()
    try {
      status.value = await clearUnpinnedCaptureEntries()
      showFeedback(
        status.value.lastSaveError ? `清空失败: ${shortError(status.value.lastSaveError)}` : '已清空未置顶',
        status.value.lastSaveError ? 'danger' : 'success',
      )
      await load(query.value)
    } catch (error) {
      showFeedback(`清空失败: ${shortError(formatError(error, '清空失败'))}`, 'danger')
    }
  }

  async function loadImage(id: string) {
    if (!id) {
      imageSequence++
      loadedImageId = ''
      pendingImageId = ''
      pendingImage = null
      isLoadingImage.value = false
      imageDataUrl.value = ''
      return
    }
    if (pendingImage && pendingImageId === id) return pendingImage
    const sequence = ++imageSequence
    pendingImageId = ''
    pendingImage = null
    if (loadedImageId === id && imageDataUrl.value) {
      isLoadingImage.value = false
      return
    }
    loadedImageId = ''
    imageDataUrl.value = ''
    pendingImageId = id
    isLoadingImage.value = true
    pendingImage = (async () => {
      try {
        const image = await getCaptureThumbnailDataURL(id)
        if (sequence !== imageSequence || selectedId.value !== id) return
        imageDataUrl.value = image
        loadedImageId = id
      } catch (error) {
        if (sequence === imageSequence && selectedId.value === id) {
          showFeedback(`预览图加载失败: ${shortError(formatError(error, '加载失败'))}`, 'danger')
        }
      } finally {
        if (sequence === imageSequence) {
          pendingImage = null
          pendingImageId = ''
          isLoadingImage.value = false
        }
      }
    })()
    return pendingImage
  }

  async function executePreviewAction(action: PreviewAction) {
    try {
      const response = await executeFancunAction(action)
      showFeedback(response.message, response.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(`操作失败: ${shortError(formatError(error, '执行失败'))}`, 'danger')
    }
  }

  function armDelete(id: string) {
    disarmDelete()
    deleteArmedId.value = id
    deleteArmedTimer = window.setTimeout(() => {
      deleteArmedTimer = undefined
      if (deleteArmedId.value === id) deleteArmedId.value = ''
    }, ARMED_TIMEOUT_MS)
  }

  function disarmDelete() {
    deleteArmedId.value = ''
    if (deleteArmedTimer !== undefined) {
      window.clearTimeout(deleteArmedTimer)
      deleteArmedTimer = undefined
    }
  }

  function armClear() {
    disarmClear()
    clearArmed.value = true
    clearArmedTimer = window.setTimeout(() => {
      clearArmedTimer = undefined
      clearArmed.value = false
    }, ARMED_TIMEOUT_MS)
  }

  function disarmClear() {
    clearArmed.value = false
    if (clearArmedTimer !== undefined) {
      window.clearTimeout(clearArmedTimer)
      clearArmedTimer = undefined
    }
  }

  function showFeedback(message: string, tone: 'success' | 'danger' = 'success') {
    if (feedbackTimer !== undefined) {
      window.clearTimeout(feedbackTimer)
      feedbackTimer = undefined
    }
    feedback.value = message
    feedbackTone.value = tone
    const duration = tone === 'danger' ? 5000 : 2000
    feedbackTimer = window.setTimeout(() => {
      feedbackTimer = undefined
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, duration)
  }

  return {
    query,
    entries,
    status,
    selectedId,
    selectedEntry,
    imageDataUrl,
    qrResult,
    ocrResult,
    ocrLines: ocrSelection.ocrLines,
    selectedOCRLineCount: ocrSelection.selectedOCRLineCount,
    pinnedCount,
    feedback,
    feedbackTone,
    loadError,
    deleteArmedId,
    clearArmed,
    isLoading,
    isLoadingImage,
    isCapturing,
    isScanningQR,
    isRecognizingOCR,
    load,
    select,
    setQuery,
    captureScreen,
    openOverlay,
    copyPath,
    openImage,
    openFolder,
    pinImage,
    togglePin,
    scanQRCode,
    scanCurrentScreenQRCode,
    copyQRText,
    recognizeText,
    recognizeCurrentScreenText,
    copyOCRText,
    copySelectedOCRText,
    isOCRLineSelected: ocrSelection.isOCRLineSelected,
    toggleOCRLine: ocrSelection.toggleOCRLine,
    selectAllOCRLines: ocrSelection.selectAllOCRLines,
    clearOCRLineSelection: ocrSelection.clearOCRLineSelection,
    deleteEntry,
    clearUnpinned,
  }
})

function formatError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message.trim()
  if (typeof error === 'string' && error.trim()) return error.trim()
  return fallback
}

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
