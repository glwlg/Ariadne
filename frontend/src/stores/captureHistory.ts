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
import { executeAriadneAction } from '../services/ariadneApi'
import { openPinnedCapture } from '../services/pinnedImageApi'
import { decodeCaptureQRCode, decodeCurrentScreenQRCode } from '../services/qrScanApi'
import { recognizeCaptureOCR, recognizeCurrentScreenOCR } from '../services/ocrApi'
import { createOCRSelection } from '../lib/ocrSelection'
import type { CaptureHistoryEntry, CaptureHistoryStatus, OCRResult, PreviewAction, QRScanResult } from '../types/ariadne'

export const useCaptureHistoryStore = defineStore('capture-history', () => {
  const query = ref('')
  const entries = ref<CaptureHistoryEntry[]>([])
  const status = ref<CaptureHistoryStatus | null>(null)
  const selectedId = ref('')
  const imageDataUrl = ref('')
  const feedback = ref('')
  const qrResult = ref<QRScanResult | null>(null)
  const ocrResult = ref<OCRResult | null>(null)
  const deleteArmedId = ref('')
  const clearArmed = ref(false)
  const isLoading = ref(false)
  const isCapturing = ref(false)
  const isScanningQR = ref(false)
  const isRecognizingOCR = ref(false)
  const ocrSelection = createOCRSelection(ocrResult)

  const selectedEntry = computed(() => entries.value.find((entry) => entry.id === selectedId.value) ?? entries.value[0] ?? null)
  const pinnedCount = computed(() => status.value?.pinnedCount ?? entries.value.filter((entry) => entry.pinned).length)

  async function load(nextQuery = query.value) {
    isLoading.value = true
    try {
      query.value = nextQuery
      const [nextStatus, nextEntries] = await Promise.all([
        getCaptureStatus(),
        listCaptureEntries(nextQuery, 300),
      ])
      status.value = nextStatus
      entries.value = nextEntries
      if (!entries.value.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      await loadImage(selectedId.value)
    } catch {
      showFeedback('截图历史加载失败')
    } finally {
      isLoading.value = false
    }
  }

  function select(id: string) {
    selectedId.value = id
    deleteArmedId.value = ''
    qrResult.value = null
    ocrResult.value = null
    ocrSelection.clearOCRLineSelection()
    void loadImage(id)
  }

  async function setQuery(value: string) {
    await load(value)
  }

  async function captureScreen() {
    isCapturing.value = true
    try {
      status.value = await captureCurrentScreen('manual')
      const error = status.value.lastCaptureError || status.value.lastSaveError
      showFeedback(error ? `捕获失败: ${shortError(error)}` : '已捕获当前屏幕')
      await load(query.value)
    } catch {
      showFeedback('捕获失败')
    } finally {
      isCapturing.value = false
    }
  }

  async function openOverlay() {
    const result = await openCaptureOverlay()
    showFeedback(result.message || (result.ok ? '已打开截图覆盖层' : '打开截图覆盖层失败'))
  }

  async function copyPath(entry = selectedEntry.value) {
    if (!entry) return
    try {
      await Clipboard.SetText(entry.imagePath)
      showFeedback('已复制路径')
    } catch {
      showFeedback('复制失败')
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
    status.value = await toggleCapturePin(entry.id)
    showFeedback(status.value.lastSaveError ? `置顶失败: ${shortError(status.value.lastSaveError)}` : entry.pinned ? '已取消置顶' : '已置顶')
    await load(query.value)
  }

  async function pinImage(entry = selectedEntry.value) {
    if (!entry) return
    const result = await openPinnedCapture(entry.id)
    showFeedback(result.message || (result.ok ? '已创建贴图' : '创建贴图失败'))
  }

  async function scanQRCode(entry = selectedEntry.value) {
    if (!entry) return
    isScanningQR.value = true
    try {
      const result = await decodeCaptureQRCode(entry.id)
      qrResult.value = result
      showFeedback(result.ok ? '已识别二维码' : result.error || '未识别到二维码')
    } catch {
      showFeedback('二维码识别失败')
    } finally {
      isScanningQR.value = false
    }
  }

  async function scanCurrentScreenQRCode() {
    isScanningQR.value = true
    try {
      const result = await decodeCurrentScreenQRCode()
      qrResult.value = result
      showFeedback(result.ok ? '已识别当前屏幕二维码' : result.error || '未识别到二维码')
      await load(query.value)
    } catch {
      showFeedback('二维码识别失败')
    } finally {
      isScanningQR.value = false
    }
  }

  async function copyQRText() {
    const text = qrResult.value?.text
    if (!text) return
    try {
      await Clipboard.SetText(text)
      showFeedback('二维码内容已复制')
    } catch {
      showFeedback('复制失败')
    }
  }

  async function recognizeText(entry = selectedEntry.value) {
    if (!entry) return
    isRecognizingOCR.value = true
    try {
      const result = await recognizeCaptureOCR(entry.id)
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(result.ok ? (result.text ? '已识别截图文字' : '未识别到文字') : result.error || 'OCR 不可用')
    } catch {
      showFeedback('OCR 识别失败')
    } finally {
      isRecognizingOCR.value = false
    }
  }

  async function recognizeCurrentScreenText() {
    isRecognizingOCR.value = true
    try {
      const result = await recognizeCurrentScreenOCR()
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(result.ok ? (result.text ? '已识别当前屏幕文字' : '未识别到文字') : result.error || 'OCR 不可用')
      await load(query.value)
    } catch {
      showFeedback('OCR 识别失败')
    } finally {
      isRecognizingOCR.value = false
    }
  }

  async function copyOCRText() {
    const text = ocrResult.value?.text
    if (!text) return
    try {
      await Clipboard.SetText(text)
      showFeedback('OCR 文本已复制')
    } catch {
      showFeedback('复制失败')
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

  async function deleteEntry(entry = selectedEntry.value) {
    if (!entry) return
    if (deleteArmedId.value !== entry.id) {
      deleteArmedId.value = entry.id
      showFeedback('再次点击确认删除')
      return
    }
    status.value = await deleteCaptureEntry(entry.id)
    deleteArmedId.value = ''
    showFeedback(status.value.lastSaveError ? `删除失败: ${shortError(status.value.lastSaveError)}` : '已删除')
    await load(query.value)
  }

  async function clearUnpinned() {
    if (!clearArmed.value) {
      clearArmed.value = true
      showFeedback('再次点击确认清空未置顶')
      return
    }
    status.value = await clearUnpinnedCaptureEntries()
    clearArmed.value = false
    showFeedback(status.value.lastSaveError ? `清空失败: ${shortError(status.value.lastSaveError)}` : '已清空未置顶')
    await load(query.value)
  }

  async function loadImage(id: string) {
    imageDataUrl.value = id ? await getCaptureThumbnailDataURL(id) : ''
  }

  async function executePreviewAction(action: PreviewAction) {
    const response = await executeAriadneAction(action)
    showFeedback(response.message)
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 1800)
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
    deleteArmedId,
    clearArmed,
    isLoading,
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

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
