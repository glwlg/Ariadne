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
  onClipboardUpdated,
  toggleClipboardPin,
} from '../services/clipboardApi'
import { openPinnedClipboardImage } from '../services/pinnedImageApi'
import { recognizeClipboardImageOCR } from '../services/ocrApi'
import { createOCRSelection } from '../lib/ocrSelection'
import type { ClipboardHistoryEntry, ClipboardHistoryStatus, OCRResult, QRScanResult } from '../types/fancun'

const SEARCH_DEBOUNCE_MS = 150
const ARMED_TIMEOUT_MS = 5000

export const useClipboardHistoryStore = defineStore('clipboard-history', () => {
  const query = ref('')
  const contentTypeFilter = ref('all')
  const entries = ref<ClipboardHistoryEntry[]>([])
  const status = ref<ClipboardHistoryStatus | null>(null)
  const selectedId = ref('')
  const imageDataUrl = ref('')
  const qrResult = ref<QRScanResult | null>(null)
  const ocrResult = ref<OCRResult | null>(null)
  const feedback = ref('')
  const feedbackTone = ref<'success' | 'danger'>('success')
  const loadError = ref('')
  const deleteArmedId = ref('')
  const clearArmed = ref(false)
  const isLoading = ref(false)
  const isLoadingImage = ref(false)
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

  const filteredEntries = computed(() => {
    const filter = contentTypeFilter.value.trim().toLowerCase()
    if (!filter || filter === 'all') return entries.value
    if (filter === 'image') return entries.value.filter((entry) => entry.type === 'image')
    if (filter === 'text') return entries.value.filter((entry) => entry.type !== 'image')
    return entries.value.filter((entry) => entry.contentType?.toLowerCase() === filter)
  })
  const selectedEntry = computed(() => {
    const list = filteredEntries.value
    return list.find((entry) => entry.id === selectedId.value) ?? list[0] ?? null
  })
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
        getClipboardStatus(),
        listClipboardEntries(nextQuery, 300),
      ])
      if (sequence !== loadSequence) return
      status.value = nextStatus
      entries.value = nextEntries
      const activeList = filteredEntries.value
      if (!activeList.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = activeList[0]?.id ?? ''
      }
      await loadImage(selectedId.value)
    } catch (error) {
      if (sequence === loadSequence) {
        loadError.value = formatError(error, '剪贴板历史加载失败')
        showFeedback(`剪贴板历史加载失败: ${shortError(loadError.value)}`, 'danger')
      }
    } finally {
      if (sequence === loadSequence) isLoading.value = false
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

  function setContentTypeFilter(filter: string) {
    contentTypeFilter.value = filter || 'all'
    const activeList = filteredEntries.value
    if (!activeList.some((entry) => entry.id === selectedId.value)) {
      select(activeList[0]?.id ?? '')
    }
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

  function subscribeUpdates() {
    return onClipboardUpdated(() => {
      if (!isLoading.value) {
        void load(query.value)
      }
    })
  }

  async function collectCurrentText() {
    try {
      status.value = await collectCurrentClipboard('manual')
      showFeedback(
        status.value.lastSaveError ? `保存失败: ${shortError(status.value.lastSaveError)}` : '已收集当前剪贴板',
        status.value.lastSaveError ? 'danger' : 'success',
      )
      await load(query.value)
    } catch (error) {
      showFeedback(`读取当前剪贴板失败: ${shortError(formatError(error, '读取失败'))}`, 'danger')
    }
  }

  async function copyEntry(entry = selectedEntry.value) {
    if (!entry) return
    if (entry.type === 'image') {
      try {
        const result = await copyClipboardImage(entry.id)
        showFeedback(result.ok ? '图片已复制' : result.message || '复制图片失败', result.ok ? 'success' : 'danger')
      } catch (error) {
        showFeedback(`复制图片失败: ${shortError(formatError(error, '复制失败'))}`, 'danger')
      }
      return
    }
    try {
      await Clipboard.SetText(entry.text)
      showFeedback('已复制', 'success')
    } catch (error) {
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(entry.text)
          showFeedback('已复制', 'success')
          return
        }
      } catch {
        // Fall through to error message below.
      }
      showFeedback(`复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
    }
  }

  async function addImageToCapture(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    try {
      const nextStatus = await addClipboardImageToCapture(entry.id)
      const error = nextStatus.lastCaptureError || nextStatus.lastSaveError
      showFeedback(error ? `加入失败: ${shortError(error)}` : '已加入截图历史', error ? 'danger' : 'success')
    } catch (error) {
      showFeedback(`加入失败: ${shortError(formatError(error, '加入截图历史失败'))}`, 'danger')
    }
  }

  async function pinImage(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    try {
      const result = await openPinnedClipboardImage(entry.id)
      showFeedback(result.message || (result.ok ? '已创建贴图' : '创建贴图失败'), result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(`创建贴图失败: ${shortError(formatError(error, '贴图服务不可用'))}`, 'danger')
    }
  }

  async function scanQRCode(entry = selectedEntry.value) {
    if (!entry || entry.type !== 'image') return
    const targetId = entry.id
    const seq = ++qrSequence
    isScanningQR.value = true
    try {
      const result = await decodeClipboardImageQRCode(targetId)
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
    if (!entry || entry.type !== 'image') return
    const targetId = entry.id
    const seq = ++ocrSequence
    isRecognizingOCR.value = true
    try {
      const result = await recognizeClipboardImageOCR(targetId)
      if (seq !== ocrSequence || selectedId.value !== targetId) return
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      showFeedback(
        result.ok ? (result.text ? '已识别图片文字' : '未识别到文字') : result.error || 'OCR 不可用',
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

  async function togglePin(entry = selectedEntry.value) {
    if (!entry) return
    try {
      status.value = await toggleClipboardPin(entry.id)
      showFeedback(
        status.value.lastSaveError ? `置顶失败: ${shortError(status.value.lastSaveError)}` : entry.pinned ? '已取消置顶' : '已置顶',
        status.value.lastSaveError ? 'danger' : 'success',
      )
      await load(query.value)
    } catch (error) {
      showFeedback(`置顶失败: ${shortError(formatError(error, '操作失败'))}`, 'danger')
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
      status.value = await deleteClipboardEntry(entry.id)
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
      status.value = await clearUnpinnedClipboardEntries()
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
    const entry = entries.value.find((item) => item.id === id)
    if (pendingImage && pendingImageId === id) return pendingImage
    const sequence = ++imageSequence
    pendingImageId = ''
    pendingImage = null
    if (entry?.type !== 'image') {
      loadedImageId = ''
      isLoadingImage.value = false
      imageDataUrl.value = ''
      return
    }
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
        const image = await getClipboardThumbnailDataURL(id)
        if (sequence !== imageSequence || selectedId.value !== id) return
        imageDataUrl.value = image
        loadedImageId = id
      } catch (error) {
        if (sequence === imageSequence && selectedId.value === id) {
          showFeedback(`剪贴板图片加载失败: ${shortError(formatError(error, '加载失败'))}`, 'danger')
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
    contentTypeFilter,
    entries,
    filteredEntries,
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
    feedbackTone,
    loadError,
    deleteArmedId,
    clearArmed,
    isLoading,
    isLoadingImage,
    isScanningQR,
    isRecognizingOCR,
    load,
    select,
    setQuery,
    setContentTypeFilter,
    subscribeUpdates,
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

function formatError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message.trim()
  if (typeof error === 'string' && error.trim()) return error.trim()
  return fallback
}

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
