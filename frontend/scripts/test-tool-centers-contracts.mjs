import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const require = createRequire(import.meta.url)

function readComponent(relPath) {
  return readFileSync(new URL(`../src/${relPath}`, import.meta.url), 'utf8')
}

// 1. CaptureHistoryCenter contracts (UI-01, UI-11, UI-12, UI-19, UX-10, UX-13, PERF-08)
{
  const src = readComponent('components/capture/CaptureHistoryCenter.vue')
  assert.match(src, /class="tool-toolbar capture-toolbar"/, 'UI-01: CaptureHistoryCenter toolbar should have capture-toolbar class')
  assert.match(src, /class="inline-feedback is-danger"/, 'UI-11: lastCaptureError must have is-danger')
  assert.match(src, /'is-danger': capture\.feedbackTone === 'danger'/, 'UI-11: capture feedback must bind is-danger')
  assert.match(src, /capture\.isLoading/, 'UI-12: CaptureHistoryCenter must check capture.isLoading before empty state')
  assert.match(src, /capture\.isLoadingImage/, 'UI-12: CaptureHistoryCenter must check capture.isLoadingImage before unavailable image')
  assert.match(src, /'is-armed-danger': capture\.clearArmed/, 'UI-19: clearArmed button must have is-armed-danger')
  assert.match(src, /'is-armed-danger': capture\.deleteArmedId === selected\.id/, 'UI-19: deleteArmed button must have is-armed-danger')
  assert.match(src, /runWithWindowHidden[\s\S]*Window\.Hide\(\)[\s\S]*Window\.Show\(\)/, 'UX-13: screen capture/QR/OCR must temporarily hide and restore window')
  assert.ok(src.indexOf('new Intl.DateTimeFormat') < src.indexOf('function formatTime('), 'PERF-08: Intl.DateTimeFormat must be hoisted to module scope')
}

// 2. PinnedImageWindow contracts (UI-08, UI-17, UX-10, UX-16)
{
  const src = readComponent('components/pinned/PinnedImageWindow.vue')
  assert.match(src, /'is-zoomed': isZoomed/, 'UI-08: PinnedImageWindow must bind is-zoomed when zoom > 1')
  assert.match(src, /Math\.round\(contentSize\.value\.width \* zoom\.value\)/, 'UI-08: PinnedImageWindow must scale zoom-layer layout dimensions when zoomed')
  assert.match(src, /async function loadPinnedImage[\s\S]*try \{[\s\S]*\} catch \{[\s\S]*image\.value = null[\s\S]*\} finally \{[\s\S]*isLoading\.value = false/, 'UI-17: loadPinnedImage must reset isLoading in finally and clear image on catch')
  assert.match(src, /OCR 识别中\.\.\./, 'UX-10: PinnedImageWindow must show inline OCR loading indicator')
  assert.match(src, /handleContextMenuKeyDown[\s\S]*ArrowDown[\s\S]*ArrowUp[\s\S]*Escape/, 'UX-16: PinnedImageWindow menu must support keyboard traversal')
  assert.match(src, /firstEnabled\?\.focus\(\)/, 'UX-16: opening menu must focus first enabled menu item')
}

// 3. ClipboardCenter contracts (UI-11, UI-12, UI-19, UX-13, PERF-08)
{
  const src = readComponent('components/clipboard/ClipboardCenter.vue')
  assert.match(src, /clipboard\.subscribeUpdates\(\)/, 'PERF-08: ClipboardCenter must subscribe to desktop updates')
  assert.match(src, /isDesktopRuntime\(\)/, 'PERF-08: polling must only run as non-desktop fallback')
  assert.ok(src.indexOf('new Intl.DateTimeFormat') < src.indexOf('function formatTime('), 'PERF-08: Intl.DateTimeFormat must be hoisted to module scope')
  assert.match(src, /contentTypeFilters[\s\S]*all[\s\S]*text[\s\S]*image[\s\S]*json[\s\S]*url[\s\S]*code/, 'UX-13: ClipboardCenter must expose content type filters')
  assert.match(src, /clipboard\.filteredEntries/, 'UX-13: ClipboardCenter must render filteredEntries')
  assert.match(src, /@dblclick="clipboard\.copyEntry\(entry\)"/, 'UX-13: double-click copy on row')
  assert.match(src, /@keydown\.enter\.prevent="clipboard\.copyEntry\(entry\)"/, 'UX-13: Enter copy on row')
  assert.match(src, /'is-danger': clipboard\.feedbackTone === 'danger'/, 'UI-11: clipboard feedback must bind is-danger')
  assert.match(src, /clipboard\.isLoading/, 'UI-12: ClipboardCenter must check isLoading')
  assert.match(src, /clipboard\.isLoadingImage/, 'UI-12: ClipboardCenter must check isLoadingImage')
  assert.match(src, /'is-armed-danger': clipboard\.clearArmed/, 'UI-19: clearArmed must bind is-armed-danger')
}

// 4. WorkflowCenter contracts (UI-11, UI-12, UI-19, UI-22, UX-06, PERF-08)
{
  const src = readComponent('components/workflows/WorkflowCenter.vue')
  assert.equal(src.includes('>STEPS<'), false, 'UI-22: STEPS kicker must be localized')
  assert.match(src, /<span class="preview-kicker">步骤<\/span>/, 'UI-22: kicker must be localized to 步骤')
  assert.match(src, /'is-danger': workflows\.feedbackTone === 'danger'/, 'UI-11: workflow feedback must bind is-danger')
  assert.match(src, /workflows\.isLoading/, 'UI-12: WorkflowCenter must show loading state')
  assert.match(src, /'is-armed-danger': Boolean\(selected && workflows\.deleteArmedId === selected\.id\)/, 'UI-19: armed delete button must bind is-armed-danger')
  assert.ok(src.indexOf('new Intl.DateTimeFormat') < src.indexOf('function formatTime('), 'PERF-08: Intl.DateTimeFormat must be hoisted to module scope')
}

// 5. SettingsCenter & subpages contracts (UI-10, UI-11, UI-19, UX-11, UX-15)
{
  const src = readComponent('components/settings/SettingsCenter.vue')
  const searchSrc = readComponent('components/settings/SearchSettingsPage.vue')
  const ocrSrc = readComponent('components/settings/OCRSettingsPage.vue')
  assert.match(src, /v-else class="tool-error-state[\s\S]*settings\.loadError[\s\S]*settings\.load\(\)/, 'UI-10: SettingsCenter must render error & retry panel when settings fail to load')
  assert.match(src, /settings\.confirmReset\(\)/, 'UX-11: Settings reset must use two-step confirmReset()')
  assert.match(src, /'is-armed-danger': settings\.resetArmed/, 'UX-11: armed reset must bind is-armed-danger')
  assert.match(src, /'is-armed-danger': Boolean\(settings\.launcherDraft\.id && settings\.launcherDeleteArmedId === settings\.launcherDraft\.id\)/, 'UI-19: launcher delete armed must bind is-armed-danger')
  assert.match(searchSrc, /'is-armed-danger': settings\.searchUsageClearArmed/, 'UI-19: search usage clear armed must bind is-armed-danger')
  assert.match(ocrSrc, /'is-armed-danger': settings\.secretClearArmedKind === 'ai_api_key'/, 'UI-19: secret clear armed must bind is-armed-danger')
  assert.match(src, /function captureHotkey[\s\S]*if \(event\.key === 'Tab'\) return\s+event\.preventDefault\(\)/, 'UX-15: captureHotkey must call preventDefault() for non-Tab keys')
  assert.match(src, /event\.key === 'Escape'[\s\S]*hotkeySnapshots\.value\[key\]/, 'UX-15: Escape must restore previous hotkey snapshot')
  assert.match(src, /'is-danger': settings\.feedbackTone === 'danger'/, 'UI-11: SettingsCenter feedback must bind is-danger')
}

// 6. ApiTestingCenter & dialogs/panels contracts (UI-02, UI-22, UX-08, UX-09, UX-11)
{
  const center = readComponent('components/apitesting/ApiTestingCenter.vue')
  const env = readComponent('components/apitesting/ApiEnvironmentPanel.vue')
  const git = readComponent('components/apitesting/ApiGitPanel.vue')
  const actionDlg = readComponent('components/apitesting/ApiActionDialog.vue')
  const newReqDlg = readComponent('components/apitesting/ApiNewRequestDialog.vue')
  const editor = readComponent('components/apitesting/ApiRequestEditor.vue')
  const response = readComponent('components/apitesting/ApiResponsePanel.vue')

  assert.match(env, /<div class="api-env-list">/, 'UI-02: ApiEnvironmentPanel must wrap environment rows in .api-env-list')
  assert.match(env, /confirmRemoveSelectedEnvironment\(\)/, 'UX-11: ApiEnvironmentPanel must use two-step confirmRemoveSelectedEnvironment')
  assert.match(env, /'is-armed-danger': Boolean\(apiTesting\.selectedEnvironment && apiTesting\.envDeleteArmedId === apiTesting\.selectedEnvironment\.id\)/, 'UX-11: armed env delete must bind is-armed-danger')
  assert.match(center, /isEnvironmentPanelOpen[\s\S]*gitPanelOpen\.value[\s\S]*\[role="dialog"\]\[aria-modal="true"\]/, 'UX-09: Ctrl+S must be suppressed when any modal is open')
  for (const [name, dlgSrc] of [
    ['ApiEnvironmentPanel', env],
    ['ApiGitPanel', git],
    ['ApiActionDialog', actionDlg],
    ['ApiNewRequestDialog', newReqDlg],
  ]) {
    assert.match(dlgSrc, /event\.key === 'Escape'/, `UX-09: ${name} must close on Escape`)
    assert.match(dlgSrc, /previousFocus\?\.focus\(\)/, `UX-09: ${name} must restore focus on close`)
  }
  assert.match(actionDlg, /event\.isComposing \|\| event\.keyCode === 229/, 'UX-09: ApiActionDialog Enter must guard IME')
  assert.match(newReqDlg, /event\.isComposing \|\| event\.keyCode === 229/, 'UX-09: ApiNewRequestDialog Enter must guard IME')
  assert.equal(newReqDlg.includes('Request Name'), false, 'UI-22: Request Name must be localized')
  assert.equal(newReqDlg.includes('Request URL'), false, 'UI-22: Request URL must be localized')
  assert.equal(newReqDlg.includes('>Folder<'), false, 'UI-22: Folder must be localized')
  assert.equal(newReqDlg.includes('>Options<'), false, 'UI-22: Options must be localized')
  assert.match(newReqDlg, /从 cURL 导入/, 'UI-22: From cURL label must be localized')
  assert.match(newReqDlg, /高级选项/, 'UI-22: Options button must be localized')
  assert.match(editor, /isRequestRunning\(apiTesting\.selectedRequest\.id\)/, 'UX-08: ApiRequestEditor must use per-request running state')
  assert.match(editor, /isRequestStopping\(apiTesting\.selectedRequest\.id\)/, 'UX-08: ApiRequestEditor must use per-request stopping state')
  assert.match(editor, /apiTesting\.requestTimeoutSeconds/, 'UX-08: ApiRequestEditor must expose configurable timeout input')
  assert.match(response, /elapsedSeconds/, 'UX-08: ApiResponsePanel must show elapsed seconds timer while running')
}

// 7. ProcessNetworkCenter, AirPlayCenter, JsonCompareCenter contracts (UI-16, UI-22, UX-11, UX-14, PERF-10)
{
  const net = readComponent('components/network/ProcessNetworkCenter.vue')
  const airplay = readComponent('components/airplay/AirPlayCenter.vue')
  const jsonCmp = readComponent('components/jsoncompare/JsonCompareCenter.vue')

  assert.match(net, /正在连接进程网络采样\.\.\./, 'UI-16: ProcessNetworkCenter must display connecting message')
  assert.match(net, /该进程当前没有活动 TCP 连接/, 'UI-16: ProcessNetworkCenter must display empty TCP connection message')
  assert.equal(airplay.includes('>UxPlay<'), false, 'UI-22: UxPlay footer must be localized')
  assert.match(airplay, /AirPlay 接收/, 'UI-22: AirPlay footer must display AirPlay 接收')
  assert.match(airplay, /pendingFrameRate/, 'UX-14: AirPlayCenter must prompt before changing FPS while running')
  assert.match(airplay, /v-if="error \|\| status\?\.lastError"[\s\S]*v-else-if="feedback"/, 'UX-14: AirPlayCenter must not show contradictory dual error+feedback banners')

  for (const legacy of ['LEFT NORMALIZED', 'RIGHT NORMALIZED', 'SEMANTIC REPORT', 'UNIFIED DIFF']) {
    assert.equal(jsonCmp.includes(legacy), false, `UI-22: ${legacy} must be localized`)
  }
  assert.match(jsonCmp, /左侧规范化/, 'UI-22: localized left normalized header')
  assert.match(jsonCmp, /右侧规范化/, 'UI-22: localized right normalized header')
  assert.match(jsonCmp, /语义差异报告/, 'UI-22: localized semantic report header')
  assert.match(jsonCmp, /行差异/, 'UI-22: localized unified diff header')
  assert.match(jsonCmp, /jsonCompare\.confirmClearAll\(\)/, 'UX-11: JsonCompareCenter must use confirmClearAll()')
  assert.match(jsonCmp, /'is-armed-danger': jsonCompare\.clearArmed/, 'UX-11: JsonCompareCenter must bind is-armed-danger')
  assert.match(jsonCmp, /jsonCompare\.scheduleCompare\(\)/, 'PERF-10: JsonCompareCenter @input must call scheduleCompare()')
  assert.match(jsonCmp, /'is-danger': jsonCompare\.feedbackTone === 'danger'/, 'UI-11: JsonCompareCenter feedback must bind is-danger')
}

// 8. Behavioral check on useAirPlay composable (UX-14: feedback auto-clear + error suppression)
{
  const { ref, computed } = require('vue')
  const statusQueue = [
    { ready: true, running: true, phase: 'waiting', receiverName: 'Fancun', pin: '1234', frameRate: 30, lastError: '' },
    { ready: true, running: false, phase: 'idle', receiverName: 'Fancun', pin: '', frameRate: 30, lastError: '进程已退出' },
  ]
  const source = ts.transpileModule(readComponent('composables/useAirPlay.ts'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText

  let mountedCb = null
  let unmountedCb = null
  const mockVue = {
    ref,
    computed,
    onMounted: (fn) => { mountedCb = fn },
    onBeforeUnmount: (fn) => { unmountedCb = fn },
  }
  const exports = {}
  const customRequire = (name) => {
    if (name === 'vue') return mockVue
    if (name === '@wailsio/runtime') return { Clipboard: { SetText: async () => {} } }
    if (name === '../services/airplayApi') {
      return {
        getAirPlayStatus: async () => statusQueue.shift() ?? statusQueue[statusQueue.length - 1],
        executeAirPlay: async () => ({ ok: true, message: '接收服务已启动' }),
        getAirPlayDiagnostics: async () => 'diag',
      }
    }
    return require(name)
  }

  new Function('require', 'exports', source)(customRequire, exports)
  const airplay = exports.useAirPlay()
  await airplay.run('start')
  assert.equal(airplay.feedback.value?.message, '接收服务已启动', 'UX-14: start success feedback set when no lastError')
  await airplay.refresh()
  assert.equal(airplay.feedback.value, null, 'UX-14: green feedback must clear when status.lastError arrives')
  assert.equal(mountedCb !== null && unmountedCb !== null, true, 'lifecycle hooks registered')
  unmountedCb()
}

console.log('PASS tool-centers-contracts: all UI/UX/PERF component contracts verified')
