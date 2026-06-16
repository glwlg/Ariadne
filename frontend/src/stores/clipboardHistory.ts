import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import {
  addClipboardImageToCapture,
  clearUnpinnedClipboardEntries,
  collectCurrentClipboard,
  copyClipboardImage,
  decodeClipboardImageQRCode,
  deleteClipboardEntry,
  getClipboardThumbnailDataURL,
  getClipboardStatus,
  listClipboardEntries,
  toggleClipboardPin,
} from '../services/clipboardApi'
import { openPinnedClipboardImage } from '../services/pinnedImageApi'
import { recognizeClipboardImageOCR } from '../services/ocrApi'
import { createOCRSelection } from '../lib/ocrSelection'
import type { ClipboardHistoryEntry, ClipboardHistoryStatus, OCRResult, QRScanResult } from '../types/ariadne'

export const useClipboardHistoryStore = defineStore('clipboard-history', () => {
  const query = ref('')
  const entries = ref<ClipboardHistoryEntry[]>([])
  const status = ref<ClipboardHistoryStatus | null>(null)
  const selectedId = ref('')
  const imageDataUrl = ref('')
  const qrResult = ref<QRScanResult | null>(null)
  const ocrResult = ref<OCRResult | null>(null)
  const feedback = ref('')
  const deleteArmedId = ref('')
  const clearArmed = ref(false)
  const isLoading = ref(false)
  const isRecognizingOCR = ref(false)
  const ocrSelection = createOCRSelection(ocrResult)

  const selectedEntry = computed(() => entries.value.find((entry) => entry.id === selectedId.value) ?? entries.value[0] ?? null)
  const pinnedCount = computed(() => status.value?.pinnedCount ?? entries.value.filter((entry) => entry.pinned).length)

  async function load(nextQuery = query.value) {
    isLoading.value = true
    try {
      query.value = nextQuery
      const [nextStatus, nextEntries] = await Promise.all([
        getClipboardStatus(),
        listClipboardEntries(nextQuery, 300),
      ])
      status.value = nextStatus
      entries.value = nextEntries
      if (!entries.value.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      await loadImage(selectedId.value)
    } catch {
      showFeedback('剪贴板历史加载失败')
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

  async function collectCurrentText() {
    try {
      status.value = await collectCurrentClipboard('manual')
      showFeedback(status.value.lastSaveError ? `保存失败: ${shortError(status.value.lastSaveError)}` : '已收集当前剪贴板')
      await load(query.value)
    } catch {
      showFeedback('读取当前剪贴板失败')
    }
  }

  async function copyEntry(entry = selectedEntry.value) {
    if (!entry) return
    if (entry.type === 'image') {
      const result = await copyClipboardImage(entry.id)
      showFeedback(result.ok ? '图片已复制' : result.message || '复制图片失败')
      return
    }
    try {
      await Clipboard.SetText(entry.text)
      showFeedback('已复制')
    } catch {
      showFeedback('复制失败')
    }
  }

  async function addImageToCapture(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    const nextStatus = await addClipboardImageToCapture(entry.id)
    const error = nextStatus.lastCaptureError || nextStatus.lastSaveError
    showFeedback(error ? `加入失败: ${shortError(error)}` : '已加入截图历史')
  }

  async function pinImage(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    const result = await openPinnedClipboardImage(entry.id)
    showFeedback(result.message || (result.ok ? '已创建贴图' : '创建贴图失败'))
  }

  async function scanQRCode(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    const result = await decodeClipboardImageQRCode(entry.id)
    qrResult.value = result
    showFeedback(result.ok ? '已识别二维码' : result.error || '未识别到二维码')
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
    if (!entry || entry.type !== 'image') return
    isRecognizingOCR.value = true
    try {
      const result = await recognizeClipboardImageOCR(entry.id)
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(result.ok ? (result.text ? '已识别图片文字' : '未识别到文字') : result.error || 'OCR 不可用')
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

  async function togglePin(entry = selectedEntry.value) {
    if (!entry) return
    status.value = await toggleClipboardPin(entry.id)
    showFeedback(status.value.lastSaveError ? `置顶失败: ${shortError(status.value.lastSaveError)}` : entry.pinned ? '已取消置顶' : '已置顶')
    await load(query.value)
  }

  async function deleteEntry(entry = selectedEntry.value) {
    if (!entry) return
    if (deleteArmedId.value !== entry.id) {
      deleteArmedId.value = entry.id
      showFeedback('再次点击确认删除')
      return
    }
    status.value = await deleteClipboardEntry(entry.id)
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
    status.value = await clearUnpinnedClipboardEntries()
    clearArmed.value = false
    showFeedback(status.value.lastSaveError ? `清空失败: ${shortError(status.value.lastSaveError)}` : '已清空未置顶')
    await load(query.value)
  }

  async function loadImage(id: string) {
    const entry = entries.value.find((item) => item.id === id)
    imageDataUrl.value = entry?.type === 'image' ? await getClipboardThumbnailDataURL(id) : ''
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
    imageDataUrl,
    qrResult,
    ocrResult,
    ocrLines: ocrSelection.ocrLines,
    selectedOCRLineCount: ocrSelection.selectedOCRLineCount,
    selectedEntry,
    pinnedCount,
    feedback,
    deleteArmedId,
    clearArmed,
    isLoading,
    isRecognizingOCR,
    load,
    select,
    setQuery,
    collectCurrentText,
    copyEntry,
    addImageToCapture,
    pinImage,
    scanQRCode,
    copyQRText,
    recognizeText,
    copyOCRText,
    copySelectedOCRText,
    isOCRLineSelected: ocrSelection.isOCRLineSelected,
    toggleOCRLine: ocrSelection.toggleOCRLine,
    selectAllOCRLines: ocrSelection.selectAllOCRLines,
    clearOCRLineSelection: ocrSelection.clearOCRLineSelection,
    togglePin,
    deleteEntry,
    clearUnpinned,
  }
})

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
