import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const require = createRequire(import.meta.url)
const { createPinia, setActivePinia } = require('pinia')

const originalWindow = globalThis.window
const scheduledTimers = new Map()
let nextTimerId = 1

function installMockWindow() {
  scheduledTimers.clear()
  globalThis.window = {
    setTimeout(callback, delay) {
      const id = nextTimerId++
      const handle = setTimeout(() => {
        scheduledTimers.delete(id)
        callback()
      }, delay)
      handle.unref?.()
      scheduledTimers.set(id, handle)
      return id
    },
    clearTimeout(id) {
      const handle = scheduledTimers.get(id)
      if (handle) {
        clearTimeout(handle)
        scheduledTimers.delete(id)
      }
    },
  }
}

function loadTs(path, mocks = {}) {
  const cache = new Map()
  function load(url) {
    const filename = fileURLToPath(url)
    if (cache.has(filename)) return cache.get(filename)
    const exports = {}
    cache.set(filename, exports)
    const source = ts.transpileModule(readFileSync(url, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
      fileName: filename,
    }).outputText
    function resolve(name) {
      if (Object.hasOwn(mocks, name)) return mocks[name]
      if (!name.startsWith('.')) return require(name)
      return load(new URL(name.endsWith('.ts') ? name : `${name}.ts`, url))
    }
    new Function('require', 'exports', source)(resolve, exports)
    return exports
  }
  setActivePinia(createPinia())
  return load(new URL(path, import.meta.url))
}

try {
  installMockWindow()

  // 1. [UX-05] Launcher keyboard, IME, native button/link, and open menu guards
  const { resolveLauncherKeyAction } = loadTs('../src/components/launcher/launcherKeyGuard.ts')
  const searchInput = { tagName: 'INPUT', closest: () => null }
  const footerButton = {
    tagName: 'BUTTON',
    closest: (sel) => (sel.includes('button') ? footerButton : null),
  }
  const menuItem = {
    tagName: 'DIV',
    closest: (sel) => (sel.includes('[role="menu"]') || sel.includes('[role="menuitem"]') ? menuItem : null),
  }

  assert.equal(resolveLauncherKeyAction({ key: 'Enter', isComposing: true, keyCode: 13, target: searchInput }, searchInput), 'ignore')
  assert.equal(resolveLauncherKeyAction({ key: 'Escape', isComposing: false, keyCode: 229, target: searchInput }, searchInput), 'ignore')
  assert.equal(resolveLauncherKeyAction({ key: 'Enter', isComposing: false, keyCode: 13, target: footerButton }, searchInput), 'ignore')
  assert.equal(resolveLauncherKeyAction({ key: 'ArrowDown', isComposing: false, keyCode: 40, target: menuItem }, searchInput), 'ignore')
  assert.equal(
    resolveLauncherKeyAction(
      { key: 'Escape', isComposing: false, keyCode: 27, target: searchInput },
      searchInput,
      (sel) => (sel.includes('[data-state="open"]') ? {} : null),
    ),
    'ignore',
  )
  assert.equal(resolveLauncherKeyAction({ key: 'Enter', isComposing: false, keyCode: 13, target: searchInput }, searchInput), 'enter')
  assert.equal(resolveLauncherKeyAction({ key: 'Escape', isComposing: false, keyCode: 27, target: searchInput }, searchInput), 'escape')
  assert.equal(resolveLauncherKeyAction({ key: 'ArrowDown', isComposing: false, keyCode: 40, target: searchInput }, searchInput), 'down')
  assert.equal(resolveLauncherKeyAction({ key: 'ArrowUp', isComposing: false, keyCode: 38, target: searchInput }, searchInput), 'up')
  console.log('PASS UX-05 launcher IME, menu, and native button key guards')

  // 2. [UX-12] QR preview data URL generator
  const { encodeQRCodeMatrix, qrTextToDataUrl } = loadTs('../src/components/launcher/qrPreview.ts')
  const matrix = encodeQRCodeMatrix('https://fancun.app')
  assert.ok(matrix && matrix.length >= 21 && matrix.length === matrix[0].length)
  assert.equal(matrix[0][0], true, 'QR finder pattern top-left outer ring must be dark')
  assert.equal(matrix[1][1], false, 'QR finder pattern inner ring must be light')
  assert.equal(matrix[2][2], true, 'QR finder pattern center must be dark')
  const dataUrl = qrTextToDataUrl('https://fancun.app')
  assert.ok(dataUrl.startsWith('data:image/svg+xml;charset=utf-8,'))
  console.log('PASS UX-12 launcher QR SVG preview generation')

  // 3. [UX-01], [UX-12], [PERF-01] Launcher missing actions, single feedback timer, and search debounce
  const calls = {
    openView: [],
    clipboardDelete: [],
    clipboardClearUnpinned: 0,
    captureClearUnpinned: 0,
    clipboardQR: [],
    clipboardToCapture: [],
    clipboardSetText: [],
    searchQueries: [],
    recordUse: [],
  }

  const { useLauncherStore } = loadTs('../src/stores/launcher.ts', {
    '@wailsio/runtime': {
      Clipboard: {
        SetText: async (text) => {
          calls.clipboardSetText.push(text)
        },
        Text: async () => '',
      },
    },
    '../lib/toolWindows': {
      toolViewForCommand: (cmd) => {
        if (cmd === 'open_network_mini') return 'network-mini'
        if (cmd === 'open_network_monitor') return 'network-monitor'
        return undefined
      },
    },
    './appShell': {
      useAppShellStore: () => ({
        openView: async (view) => {
          calls.openView.push(view)
          return true
        },
      }),
    },
    '../services/captureApi': {
      toggleCapturePin: async () => ({ lastSaveError: '' }),
      clearUnpinnedCaptureEntries: async () => {
        calls.captureClearUnpinned += 1
        return { lastSaveError: '' }
      },
    },
    '../services/captureOverlayApi': {
      openCaptureOverlay: async () => ({ ok: true, message: 'ok' }),
    },
    '../services/clipboardApi': {
      addClipboardText: async () => ({}),
      copyClipboardImage: async () => ({ ok: true, message: 'copied' }),
      toggleClipboardPin: async () => ({ lastSaveError: '' }),
      deleteClipboardEntry: async (id) => {
        calls.clipboardDelete.push(id)
        return { lastSaveError: '' }
      },
      clearUnpinnedClipboardEntries: async () => {
        calls.clipboardClearUnpinned += 1
        return { lastSaveError: '' }
      },
      decodeClipboardImageQRCode: async (id) => {
        calls.clipboardQR.push(id)
        if (id === 'qr-ok') return { ok: true, text: 'https://qr.example/ok' }
        return { ok: false, error: '未检测到二维码' }
      },
      addClipboardImageToCapture: async (id) => {
        calls.clipboardToCapture.push(id)
        return { lastCaptureError: '', lastSaveError: '' }
      },
    },
    '../services/fancunApi': {
      createFancunSearchRequest: (query) => {
        calls.searchQueries.push(query)
        return {
          promise: Promise.resolve({ query, results: [], elapsedMs: 5, totalResults: 0 }),
          cancel() {},
        }
      },
      executeFancunAction: async () => ({ ok: true, message: 'ok' }),
      isSearchCancelled: () => false,
      recordResultUse: async (id) => {
        calls.recordUse.push(id)
        return true
      },
      setResultFavorite: async () => true,
    },
    '../services/qrScanApi': { decodeCaptureQRCode: async () => ({ ok: false }) },
    '../services/imageIndexApi': { indexRecentImages: async () => ({ ok: true, indexed: 0, skipped: 0, failed: 0 }) },
    '../services/pinnedImageApi': {
      openPinnedCapture: async () => ({ ok: true, message: 'ok' }),
      openPinnedClipboardImage: async () => ({ ok: true, message: 'ok' }),
      openPinnedQRText: async () => ({ ok: true, message: 'ok' }),
    },
    '../services/workflowApi': { runWorkflow: async () => ({ ok: true, message: 'ok', steps: [] }) },
  })

  const launcher = useLauncherStore()

  // open_mini and open_center
  await launcher.triggerAction({ id: 'open_mini', label: '打开小窗', kind: 'plugin', payload: { command: 'open_network_mini' } })
  await launcher.triggerAction({ id: 'open_center', label: '打开完整中心', kind: 'plugin', payload: { command: 'open_network_monitor' } })
  assert.deepEqual(calls.openView, ['network-mini', 'network-monitor'])
  assert.equal(launcher.lastAction?.ok, true)

  // clipboard_delete requires confirmation then deletes
  const deleteAction = { id: 'clipboard_delete', label: '删除', kind: 'danger', payload: { clipboardId: 'clip-1', requiresConfirmation: true } }
  await launcher.triggerAction(deleteAction)
  assert.equal(calls.clipboardDelete.length, 0)
  assert.equal(launcher.lastAction?.requiresConfirmation, true)
  await launcher.triggerAction(deleteAction)
  assert.deepEqual(calls.clipboardDelete, ['clip-1'])
  assert.equal(launcher.lastAction?.ok, true)

  // clear_unpinned dispatches clipboard vs capture
  const clearClip = { id: 'clear_unpinned', label: '确认清理', kind: 'danger', payload: { command: 'clipboard:clear_unpinned' } }
  await launcher.triggerAction(clearClip)
  assert.equal(calls.clipboardClearUnpinned, 0)
  await launcher.triggerAction(clearClip)
  assert.equal(calls.clipboardClearUnpinned, 1)

  const clearCap = { id: 'clear_unpinned', label: '确认清理', kind: 'danger', payload: { command: 'capture:clear_unpinned' } }
  await launcher.triggerAction(clearCap)
  assert.equal(calls.captureClearUnpinned, 0)
  await launcher.triggerAction(clearCap)
  assert.equal(calls.captureClearUnpinned, 1)

  // recognize_clipboard_qr success + failure
  await launcher.triggerAction({ id: 'recognize_clipboard_qr', label: '识别二维码', kind: 'plugin', payload: { clipboardId: 'qr-ok' } })
  assert.deepEqual(calls.clipboardQR, ['qr-ok'])
  assert.deepEqual(calls.clipboardSetText, ['https://qr.example/ok'])
  assert.equal(launcher.lastAction?.ok, true)

  await launcher.triggerAction({ id: 'recognize_clipboard_qr', label: '识别二维码', kind: 'plugin', payload: { clipboardId: 'qr-miss' } })
  assert.equal(launcher.lastAction?.ok, false)
  assert.equal(launcher.feedbackTone, 'danger')

  // clipboard_image_to_capture
  await launcher.triggerAction({ id: 'clipboard_image_to_capture', label: '加入截图历史', kind: 'plugin', payload: { clipboardId: 'clip-img-1' } })
  assert.deepEqual(calls.clipboardToCapture, ['clip-img-1'])
  assert.equal(launcher.lastAction?.ok, true)

  // UX-12: single lastActionTimer ensures earlier short timer does not clobber newer feedback
  await launcher.triggerAction({ id: 'open_mini', label: 'A', kind: 'plugin', payload: { command: 'open_network_mini' }, feedback: { successLabel: 'first', durationMs: 25 } })
  await launcher.triggerAction({ id: 'open_center', label: 'B', kind: 'plugin', payload: { command: 'open_network_monitor' }, feedback: { successLabel: 'second', durationMs: 120 } })
  await new Promise((resolve) => setTimeout(resolve, 45))
  assert.equal(launcher.lastAction?.message, 'second', 'first action timer must be cancelled so second feedback remains visible')

  // PERF-01: setQuery debounces rapid typing and clears synchronously on empty string
  launcher.setQuery('a')
  launcher.setQuery('ab')
  launcher.setQuery('abc')
  assert.equal(calls.searchQueries.length, 0, 'rapid typing should debounce before invoking search')
  assert.equal(launcher.isSearching, true)
  await new Promise((resolve) => setTimeout(resolve, 130))
  assert.deepEqual(calls.searchQueries, ['abc'])

  // Preserve query and results across hide/blur while clearing transient feedback and armed confirmation
  launcher.results = [{ id: 'res-1', title: 'demo', subtitle: '', type: 'file', icon: 'file', score: 100, badge: '文件', preview: { kind: 'text', title: 'demo', subtitle: '', summary: '', meta: [] }, actions: [] }]
  launcher.select('res-1')
  await launcher.triggerAction(deleteAction)
  assert.equal(launcher.lastAction?.requiresConfirmation, true)
  launcher.clearTransientState()
  assert.equal(launcher.query, 'abc', 'clearTransientState must preserve launcher search query')
  assert.equal(launcher.results.length, 1, 'clearTransientState must preserve launcher search results')
  assert.equal(launcher.selectedId, 'res-1', 'clearTransientState must preserve selected result')
  assert.equal(launcher.isExpanded, true, 'clearTransientState must keep expanded state when query/results remain')
  assert.equal(launcher.lastAction, null, 'clearTransientState must clear transient action toast')
  await launcher.triggerAction(deleteAction)
  assert.equal(calls.clipboardDelete.length, 1, 'armed confirmation must have been cleared by clearTransientState')
  const launcherVueSource = readFileSync(new URL('../src/components/launcher/FancunLauncher.vue', import.meta.url), 'utf8')
  assert.match(launcherVueSource, /async function hideLauncher\(\)\s*\{\s*launcher\.clearTransientState\(\)/)
  assert.match(launcherVueSource, /function onVisibilityChange\(\)\s*\{\s*if \(document\.hidden\)\s*\{\s*launcher\.clearTransientState\(\)/)

  launcher.setQuery('')
  assert.equal(launcher.isSearching, false)
  assert.equal(launcher.results.length, 0)
  console.log('PASS UX-01/UX-12/PERF-01 launcher actions, feedback timer, search preservation, and debounce')
} finally {
  globalThis.window = originalWindow
}
