import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import { toggleCapturePin } from '../services/captureApi'
import { openCaptureOverlay } from '../services/captureOverlayApi'
import { addClipboardText, copyClipboardImage, toggleClipboardPin } from '../services/clipboardApi'
import {
  createAriadneSearchRequest,
  executeAriadneAction,
  isSearchCancelled,
  recordResultUse,
  setResultFavorite,
  type AriadneSearchRequest,
} from '../services/ariadneApi'
import { decodeCaptureQRCode } from '../services/qrScanApi'
import { indexRecentImages } from '../services/imageIndexApi'
import { openPinnedCapture, openPinnedClipboardImage, openPinnedQRText } from '../services/pinnedImageApi'
import { runWorkflow } from '../services/workflowApi'
import { useAppShellStore } from './appShell'
import type { ActionResult, PreviewAction, SearchResponse, SearchResult } from '../types/ariadne'

export const useLauncherStore = defineStore('launcher', () => {
  const query = ref('')
  const results = ref<SearchResult[]>([])
  const selectedId = ref('')
  const lastAction = ref<ActionResult | null>(null)
  const isTimeMachineEnabled = ref(false)
  const privacyMode = ref(false)
  const elapsedMs = ref(0)
  const pendingConfirmationKey = ref('')
  let activeSearchRequest: AriadneSearchRequest | null = null
  let searchRequestSerial = 0
  let pendingConfirmationTimer: number | undefined
  const isExpanded = computed(() => query.value.trim().length > 0 || results.value.length > 0 || Boolean(lastAction.value))

  const selectedResult = computed(() => {
    return results.value.find((result) => result.id === selectedId.value) ?? results.value[0] ?? null
  })

  function select(id: string) {
    selectedId.value = id
  }

  function moveSelection(delta: number) {
    if (!results.value.length) {
      selectedId.value = ''
      return
    }

    const index = Math.max(
      0,
      results.value.findIndex((result) => result.id === selectedId.value),
    )
    const next = (index + delta + results.value.length) % results.value.length
    selectedId.value = results.value[next].id
  }

  async function triggerAction(action: PreviewAction) {
    const activeResult = selectedResult.value
    const resultId = String(action.payload?.targetId ?? activeResult?.id ?? '')
    const command = String(action.payload?.command ?? '')

    if (action.id === 'prepare_command' || (action.kind === 'run' && activeResult?.type === 'plugin_trigger')) {
      if (command) {
        await applyCommandSuggestion(command.endsWith(' ') ? command : `${command} `)
        return
      }
    }

    if (action.id === 'open_tool') {
      const appShell = useAppShellStore()
      if (command === 'open_clipboard_center') {
        appShell.openClipboard()
      } else if (command === 'open_capture_center') {
        appShell.openCaptureHistory()
      } else if (command === 'open_capture_overlay') {
        const response = await openCaptureOverlay()
        lastAction.value = { ok: response.ok, message: response.message || (response.ok ? '已打开截图覆盖层' : '打开截图覆盖层失败') }
        window.setTimeout(() => {
          lastAction.value = null
        }, action.feedback?.durationMs ?? 1400)
        return
      } else if (command === 'open_work_memory_center') {
        appShell.openWorkMemory()
      } else if (command === 'open_hosts') {
        appShell.openHosts()
      } else if (command === 'open_workflow_center') {
        appShell.openWorkflow()
      } else if (command === 'open_json_compare') {
        appShell.openJsonCompare()
      } else if (command === 'open_network_monitor') {
        appShell.openNetworkMonitor()
      } else if (command === 'open_network_mini') {
        appShell.openNetworkMini()
      } else if (command === 'open_settings') {
        appShell.openSettings()
      }
      lastAction.value = { ok: true, message: action.feedback?.successLabel ?? `${action.label} 已打开` }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if ((action.id === 'capture_pin' || action.id === 'capture_unpin') && action.payload?.captureId) {
      const captureId = String(action.payload.captureId)
      await toggleCapturePin(captureId)
      lastAction.value = {
        ok: true,
        message: action.feedback?.successLabel ?? (action.id === 'capture_pin' ? '已置顶' : '已取消置顶'),
      }
      await refreshResults()
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'pin_capture_image' && action.payload?.captureId) {
      const captureId = String(action.payload.captureId)
      const response = await openPinnedCapture(captureId)
      lastAction.value = { ok: response.ok, message: response.message || (response.ok ? '已创建贴图' : '创建贴图失败') }
      if (response.ok && activeResult?.id) {
        void recordResultUse(activeResult.id)
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'recognize_qr' && action.payload?.captureId) {
      const captureId = String(action.payload.captureId)
      const result = await decodeCaptureQRCode(captureId)
      if (result.ok && result.text) {
        const copied = await writeClipboardText(result.text)
        lastAction.value = {
          ok: copied,
          message: copied ? `已识别并复制二维码: ${clip(result.text, 42)}` : `已识别二维码: ${clip(result.text, 42)}`,
        }
      } else {
        lastAction.value = { ok: false, message: result.error || '未识别到二维码' }
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1800)
      return
    }

    if (action.id === 'pin_qr' && action.payload?.text) {
      const response = await openPinnedQRText(String(action.payload.text))
      lastAction.value = { ok: response.ok, message: response.message || (response.ok ? '已创建贴图' : '创建贴图失败') }
      if (response.ok && activeResult?.id) {
        void recordResultUse(activeResult.id)
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'image_index_recent') {
      const limit = Number(action.payload?.limit ?? 30)
      const response = await indexRecentImages({ limit })
      lastAction.value = {
        ok: response.ok,
        message: response.ok
          ? `已索引 ${response.indexed} 张，跳过 ${response.skipped} 张`
          : `图片索引失败 ${response.failed} 张${response.lastError ? `：${response.lastError}` : ''}`,
      }
      if (response.ok && activeResult?.id) {
        void recordResultUse(activeResult.id)
      }
      await refreshResults()
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1800)
      return
    }

    if ((action.id === 'favorite' || action.id === 'unfavorite') && resultId) {
      const favorite = Boolean(action.payload?.favorite)
      const saved = await setResultFavorite(resultId, favorite)
      lastAction.value = {
        ok: saved,
        message: saved ? (action.feedback?.successLabel ?? (favorite ? '已收藏' : '已取消收藏')) : '收藏失败',
      }
      await refreshResults()
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if ((action.id === 'clipboard_pin' || action.id === 'clipboard_unpin') && action.payload?.clipboardId) {
      const clipboardId = String(action.payload.clipboardId)
      await toggleClipboardPin(clipboardId)
      lastAction.value = {
        ok: true,
        message: action.feedback?.successLabel ?? (action.id === 'clipboard_pin' ? '已置顶' : '已取消置顶'),
      }
      await refreshResults()
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'pin_clipboard_image' && action.payload?.clipboardId) {
      const clipboardId = String(action.payload.clipboardId)
      const response = await openPinnedClipboardImage(clipboardId)
      lastAction.value = { ok: response.ok, message: response.message || (response.ok ? '已创建贴图' : '创建贴图失败') }
      if (response.ok && activeResult?.id) {
        void recordResultUse(activeResult.id)
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'copy_clipboard_image' && action.payload?.clipboardId) {
      const clipboardId = String(action.payload.clipboardId)
      const response = await copyClipboardImage(clipboardId)
      lastAction.value = response
      if (response.ok && activeResult?.id) {
        void recordResultUse(activeResult.id)
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    if (action.id === 'run_workflow' && action.payload?.workflowId) {
      const workflowId = String(action.payload.workflowId)
      const input = String(action.payload.input ?? '')
      let clipboardText = ''
      try {
        clipboardText = await Clipboard.Text()
      } catch {
        clipboardText = ''
      }
      const result = await runWorkflow({ workflowId, input, clipboardText })
      if (result.ok && result.output) {
        const copied = await writeClipboardText(result.output)
        lastAction.value = {
          ok: copied,
          message: copied ? `${result.message}，结果已复制` : `${result.message}，复制失败`,
        }
        if (copied) {
          void addClipboardText(result.output, 'workflow')
          if (activeResult?.id) {
            void recordResultUse(activeResult.id)
          }
        }
      } else {
        lastAction.value = { ok: false, message: result.message }
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1800)
      return
    }

    if (action.kind === 'copy') {
      const text = String(action.payload?.text ?? activeResult?.detail ?? activeResult?.title ?? '')
      const copied = await writeClipboardText(text)
      const response = await executeAriadneAction(action)
      lastAction.value = copied ? response : { ok: false, message: '复制失败' }
      if (copied && activeResult?.id) {
        void recordResultUse(activeResult.id)
        void addClipboardText(text, 'action')
      }
      window.setTimeout(() => {
        lastAction.value = null
      }, action.feedback?.durationMs ?? 1400)
      return
    }

    const confirmationKey = actionConfirmationKey(action, activeResult)
    const actionToExecute = pendingConfirmationKey.value === confirmationKey
      ? { ...action, payload: { ...(action.payload ?? {}), confirmed: true, confirm: true } }
      : action
    const response = await executeAriadneAction(actionToExecute)
    lastAction.value = response
    if (response.requiresConfirmation) {
      setPendingConfirmation(confirmationKey, action)
    } else {
      clearPendingConfirmation()
    }
    if (response.ok && activeResult?.id) {
      void recordResultUse(activeResult.id)
    }
    const feedbackDuration = response.requiresConfirmation ? confirmationTimeoutMs(action) : (action.feedback?.durationMs ?? 1400)
    window.setTimeout(() => {
      lastAction.value = null
    }, feedbackDuration)
  }

  async function writeClipboardText(text: string) {
    try {
      await Clipboard.SetText(text)
      return true
    } catch {
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(text)
          return true
        }
      } catch {
        return false
      }
    }
    return false
  }

  function runPrimaryAction() {
    const primary = selectedResult.value?.actions[0]
    if (primary) {
      void triggerAction(primary)
    }
  }

  async function refreshResults(value = query.value) {
    const requestSerial = ++searchRequestSerial
    activeSearchRequest?.cancel('superseded')
    activeSearchRequest = null
    clearPendingConfirmation()
    query.value = value
    if (!value.trim()) {
      results.value = []
      selectedId.value = ''
      elapsedMs.value = 0
      return
    }

    const request = createAriadneSearchRequest(value)
    activeSearchRequest = request
    let response: SearchResponse
    try {
      response = await request.promise
    } catch (error) {
      if (isSearchCancelled(error)) {
        return
      }
      if (requestSerial === searchRequestSerial) {
        results.value = []
        selectedId.value = ''
        elapsedMs.value = 0
      }
      return
    } finally {
      if (requestSerial === searchRequestSerial) {
        activeSearchRequest = null
      }
    }

    if (requestSerial !== searchRequestSerial || query.value !== value) {
      return
    }
    results.value = response.results
    elapsedMs.value = response.elapsedMs
    if (!results.value.some((result) => result.id === selectedId.value)) {
      selectedId.value = results.value[0]?.id ?? ''
    }
  }

  function setQuery(value: string) {
    void refreshResults(value)
  }

  async function applyCommandSuggestion(value: string) {
    lastAction.value = null
    clearPendingConfirmation()
    await refreshResults(value)
  }

  function setPendingConfirmation(key: string, action: PreviewAction) {
    clearPendingConfirmation()
    pendingConfirmationKey.value = key
    pendingConfirmationTimer = window.setTimeout(() => {
      if (pendingConfirmationKey.value === key) {
        pendingConfirmationKey.value = ''
      }
    }, confirmationTimeoutMs(action))
  }

  function clearPendingConfirmation() {
    pendingConfirmationKey.value = ''
    if (pendingConfirmationTimer !== undefined) {
      window.clearTimeout(pendingConfirmationTimer)
      pendingConfirmationTimer = undefined
    }
  }

  function reset() {
    searchRequestSerial++
    activeSearchRequest?.cancel('reset')
    activeSearchRequest = null
    clearPendingConfirmation()
    query.value = ''
    results.value = []
    selectedId.value = ''
    lastAction.value = null
    elapsedMs.value = 0
  }

  return {
    query,
    selectedId,
    selectedResult,
    results,
    isExpanded,
    lastAction,
    isTimeMachineEnabled,
    privacyMode,
    elapsedMs,
    setQuery,
    applyCommandSuggestion,
    reset,
    select,
    moveSelection,
    runPrimaryAction,
    triggerAction,
    refreshResults,
  }
})

function actionConfirmationKey(action: PreviewAction, result: SearchResult | null) {
  return [
    result?.id ?? '',
    action.id,
    action.kind,
    String(action.payload?.command ?? ''),
    String(action.payload?.arguments ?? ''),
    String(action.payload?.workingDir ?? ''),
  ].join('|')
}

function confirmationTimeoutMs(action?: PreviewAction) {
  const requested = action?.feedback?.durationMs ?? 0
  return Math.max(5000, requested + 2600)
}

function clip(text: string, max: number) {
  const normalized = text.replace(/\s+/g, ' ').trim()
  return normalized.length > max ? `${normalized.slice(0, max - 3)}...` : normalized
}
