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

function installMockWindow(extra = {}) {
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
    setInterval(callback, delay) {
      const id = nextTimerId++
      const handle = setInterval(callback, delay)
      handle.unref?.()
      scheduledTimers.set(id, handle)
      return id
    },
    clearInterval(id) {
      const handle = scheduledTimers.get(id)
      if (handle) {
        clearInterval(handle)
        scheduledTimers.delete(id)
      }
    },
    ...extra,
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

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}

try {
  installMockWindow()

  // 1. [UX-02], [UX-03], [UX-06], [UX-11] Hosts store: save error preserves draft, new profile selection, toggle disarms apply, rollback
  {
    let profiles = [
      { id: 'system-hosts', title: '系统 Hosts', content: '127.0.0.1 localhost', enabled: false, type: 'local', system: true },
      { id: 'a-first', title: 'Alpha', content: '127.0.0.1 a.test', enabled: true, type: 'local', system: false, updatedAt: 10 },
    ]
    let canRollback = false
    let rollbackCalls = 0
    let failUpsert = false
    const { useHostsStore } = loadTs('../src/stores/hosts.ts', {
      '../services/hostsApi': {
        getHostsStatus: async () => ({ profiles, enabledCount: profiles.filter((p) => p.enabled && !p.system).length, canRollback, lastBackupAt: canRollback ? '2026-11-24T10:00:00Z' : '' }),
        listHostsProfiles: async () => profiles,
        newHostsProfile: async () => {
          const created = { id: 'z-new', title: 'New Hosts Profile', content: '# new', enabled: false, type: 'local', system: false, updatedAt: 20 }
          profiles = [profiles[0], profiles[1], created]
          return { profiles, canRollback }
        },
        upsertHostsProfile: async (next) => {
          if (failUpsert) {
            return { profiles, lastSaveError: 'Hosts 方案标题不能为空', canRollback }
          }
          profiles = profiles.map((p) => (p.id === next.id ? { ...next } : p))
          return { profiles, lastSaveError: '', canRollback }
        },
        setHostsProfileEnabled: async (id, enabled) => {
          profiles = profiles.map((p) => (p.id === id ? { ...p, enabled } : p))
          return { profiles, lastSaveError: '', canRollback }
        },
        removeHostsProfile: async () => ({ profiles, canRollback }),
        fetchRemoteHosts: async () => ({ profiles, canRollback }),
        previewHostsApply: async () => ({
          hostsPath: 'hosts',
          lineCount: 2,
          addedLines: 1,
          removedLines: 0,
          changed: true,
          enabledProfiles: ['Alpha'],
          conflicts: [],
          currentContent: '127.0.0.1 localhost',
          finalContent: '127.0.0.1 localhost\n127.0.0.1 a.test',
          diffText: '+ 127.0.0.1 a.test',
          requiresConfirm: true,
        }),
        applyEnabledHostsProfiles: async () => {
          canRollback = true
          return { ok: true, message: '已写入系统 Hosts', requiresConfirm: false, preview: { conflicts: [] } }
        },
        rollbackHosts: async () => {
          rollbackCalls += 1
          canRollback = false
          return { ok: true, message: '已回滚系统 Hosts', requiresConfirm: false, preview: { conflicts: [] } }
        },
      },
    })

    const hosts = useHostsStore()
    await hosts.load()
    await hosts.createProfile()
    assert.equal(hosts.selectedId, 'z-new', 'createProfile must select newly created profile instead of first non-system profile')

    hosts.updateDraft({ title: '', content: 'unsaved hosts content' })
    failUpsert = true
    const ok = await hosts.saveDraft()
    assert.equal(ok, false)
    assert.equal(hosts.draft?.content, 'unsaved hosts content', 'save failure must preserve unsaved draft edits')
    assert.equal(hosts.feedbackTone, 'danger')

    failUpsert = false
    hosts.updateDraft({ title: 'Zeta' })
    await hosts.applyHosts()
    assert.equal(hosts.applyArmed, true)
    await hosts.toggleEnabled(profiles[1])
    assert.equal(hosts.applyArmed, false, 'toggleEnabled must disarm applyArmed')

    await hosts.applyHosts()
    await hosts.applyHosts()
    assert.equal(hosts.canRollback, true)
    assert.equal(hosts.lastBackupAt, '2026-11-24T10:00:00Z')

    await hosts.rollbackHosts()
    assert.equal(hosts.rollbackArmed, true)
    assert.equal(rollbackCalls, 0)
    await hosts.rollbackHosts()
    assert.equal(rollbackCalls, 1)
    assert.equal(hosts.canRollback, false)
    console.log('PASS UX-02/UX-03/UX-06/UX-11 hosts draft preservation, profile selection, disarm, and rollback')
  }

  // 2. [UX-06] Workflows store: save error preserves draft & createWorkflow selects newly created ID
  {
    let workflowsList = [
      { id: 'alpha', name: 'Alpha', description: '', steps: [{ command: 'hash {input}', pick: '' }], updatedAt: 10 },
      { id: 'zzz', name: 'ZZZ', description: '', steps: [{ command: 'url {input}', pick: '' }], updatedAt: 10 },
    ]
    let failSave = false
    const { useWorkflowsStore } = loadTs('../src/stores/workflows.ts', {
      '@wailsio/runtime': { Clipboard: { Text: async () => '', SetText: async () => {} } },
      '../services/workflowApi': {
        getWorkflowStatus: async () => ({ workflows: workflowsList, count: workflowsList.length }),
        listWorkflows: async () => workflowsList,
        newWorkflow: async () => {
          workflowsList = [
            workflowsList[0],
            { id: 'new-workflow', name: '新工作流', description: '', steps: [{ command: 'hash {input}', pick: '' }], updatedAt: 20 },
            workflowsList[1],
          ]
          return { workflows: workflowsList, count: workflowsList.length }
        },
        upsertWorkflow: async (wf) => {
          if (failSave) return { workflows: workflowsList, count: workflowsList.length, lastSaveError: '步骤不能为空' }
          return { workflows: workflowsList, count: workflowsList.length }
        },
        removeWorkflow: async () => ({ workflows: workflowsList, count: workflowsList.length }),
        exportWorkflows: async () => ({ ok: true, count: 0 }),
        importWorkflows: async () => ({ ok: true, importedCount: 0, status: { workflows: workflowsList } }),
        runWorkflow: async () => {
          throw new Error('执行器崩溃')
        },
      },
    })

    const wfStore = useWorkflowsStore()
    await wfStore.load()
    await wfStore.createWorkflow()
    assert.equal(wfStore.selectedId, 'new-workflow', 'createWorkflow must select new workflow by ID even when sorted in the middle')

    wfStore.updateDraft({ name: 'Unsaved Edit', id: 'edited-id' })
    failSave = true
    const saved = await wfStore.saveDraft()
    assert.equal(saved, false)
    assert.equal(wfStore.draft?.name, 'Unsaved Edit', 'workflow save error must preserve draft')
    assert.equal(wfStore.selectedId, 'new-workflow')

    failSave = false
    await wfStore.runSelected()
    assert.equal(wfStore.isRunning, false)
    assert.equal(wfStore.feedbackTone, 'danger')
    console.log('PASS UX-06 workflows draft preservation, ID selection, and run catch handling')
  }

  // 3. [UX-06], [UX-08], [PERF-07] API Testing store: per-request running isolation, stop error recovery, save error draft preservation, snapshot dedup
  {
    const runA = deferred()
    const runB = deferred()
    let snapshotCalls = 0
    let stopShouldFail = true
    let failCollectionSave = false
    const collection = {
      id: 'col-1',
      name: 'Collection 1',
      variables: [],
      environments: [{ id: 'env-1', name: 'Default', variables: [] }],
      requests: [
        { id: 'req-a', name: 'Slow SSE', method: 'GET', url: 'https://example.com/sse', bodyType: 'none', body: '', params: [], headers: [], assertions: [] },
        { id: 'req-b', name: 'Fast GET', method: 'GET', url: 'https://example.com/fast', bodyType: 'none', body: '', params: [], headers: [], assertions: [] },
      ],
      activeEnvironmentId: 'env-1',
      activeRequestId: 'req-a',
    }

    const { useAPITestingStore } = loadTs('../src/stores/apiTesting.ts', {
      '@wailsio/runtime': { Clipboard: { SetText: async () => {} }, Dialogs: { OpenFile: async () => '' } },
      '../services/apiTestingApi': {
        getAPITestingStatus: async () => ({ activeCollectionId: 'col-1', collections: [collection] }),
        upsertAPICollection: async () => ({
          activeCollectionId: 'col-1',
          collections: [collection],
          lastSaveError: failCollectionSave ? 'SQLite 只读' : '',
        }),
        createAPICollection: async () => ({ activeCollectionId: 'col-1', collections: [collection] }),
        deleteAPICollection: async () => ({ activeCollectionId: 'col-1', collections: [collection] }),
        setActiveAPICollection: async () => ({ activeCollectionId: 'col-1', collections: [collection] }),
        runAPIRequest: (req) => (req.request.id === 'req-a' ? runA.promise : runB.promise),
        stopAPIRequest: async () => {
          if (stopShouldFail) throw new Error('stop rpc failed')
          return { ok: true, message: 'stopped' }
        },
        getAPIRunSnapshot: async () => {
          snapshotCalls += 1
          return {
            ok: true,
            running: true,
            message: 'streaming',
            updatedAt: 1,
            result: { ok: true, requestUrl: 'https://example.com/sse', statusCode: 200, statusText: '200 OK', durationMs: snapshotCalls * 10, headers: [], body: 'chunk1', bodySize: 6, assertionResults: [], passed: 0, failed: 0 },
          }
        },
        importAPIRequests: async () => ({ ok: false, message: '', importedCount: 0, status: { collections: [collection] } }),
        configureAPIGit: async () => ({ ok: true, message: '', dirty: false }),
        getAPIGitStatus: async () => ({ ok: true, message: '', dirty: false }),
        pullAPIGit: async () => ({ ok: true, message: '', dirty: false }),
        commitPushAPIGit: async () => ({ ok: true, message: '', dirty: false }),
        newAPIRequest: () => ({ id: 'req-new', name: 'New', method: 'GET', url: '', bodyType: 'none', body: '', params: [], headers: [], assertions: [] }),
        newAPIHeader: () => ({ id: 'h', name: '', value: '', enabled: true }),
        newAPIParam: () => ({ id: 'p', name: '', value: '', type: 'query', enabled: true }),
        newAPIVariable: () => ({ id: 'v', name: '', value: '', enabled: true }),
        newAPIEnvironment: () => ({ id: 'e', name: 'Env', variables: [] }),
        newAPIAssertion: () => ({ id: 'a', kind: 'status', target: '', operator: 'equals', expected: '200', enabled: true }),
      },
    })

    const api = useAPITestingStore()
    await api.load()

    const execA = api.runSelectedRequest()
    assert.equal(api.isRunning, true)
    assert.equal(api.isRequestRunning('req-a'), true)

    // Switch to req-b while req-a is still running
    api.selectRequest('req-b')
    assert.equal(api.isRunning, false, 'req-b must not appear running while req-a is running')
    const execB = api.runSelectedRequest()
    assert.equal(api.isRequestRunning('req-b'), true)
    assert.equal(api.isRequestRunning('req-a'), true)

    runB.resolve({ ok: true, message: 'B done', method: 'GET', requestUrl: 'https://example.com/fast', statusCode: 200, statusText: '200 OK', durationMs: 12, headers: [], body: 'ok', bodySize: 2, assertionResults: [], passed: 0, failed: 0 })
    await execB
    assert.equal(api.isRequestRunning('req-b'), false)
    assert.equal(api.isRequestRunning('req-a'), true)

    // Switch back to req-a, verify stop error recovery
    api.selectRequest('req-a')
    await api.stopSelectedRequest()
    assert.equal(api.isStopping, false, 'failed stopAPIRequest must reset isStopping')
    assert.equal(api.feedbackTone, 'danger')

    // PERF-07: snapshot polling should not replace lastResult reference when bodySize/status/running are unchanged
    await new Promise((resolve) => setTimeout(resolve, 680))
    const refAfterFirstPoll = api.lastResult
    await new Promise((resolve) => setTimeout(resolve, 350))
    assert.equal(api.lastResult, refAfterFirstPoll, 'unchanged snapshot poll must skip redundant state replacement')

    runA.resolve({ ok: true, message: 'A done', method: 'GET', requestUrl: 'https://example.com/sse', statusCode: 200, statusText: '200 OK', durationMs: 90, headers: [], body: 'done', bodySize: 4, assertionResults: [], passed: 0, failed: 0 })
    await execA

    // UX-06: saveCollection error preserves draftCollection & isDirty
    api.updateRequest({ url: 'https://edited.example/v2' })
    assert.equal(api.isDirty, true)
    failCollectionSave = true
    const saved = await api.saveCollection()
    assert.equal(saved, false)
    assert.equal(api.isDirty, true)
    assert.equal(api.selectedRequest?.url, 'https://edited.example/v2')
    console.log('PASS UX-06/UX-08/PERF-07 API testing per-request isolation, stop recovery, snapshot dedup, and save draft preservation')
  }

  // 4. [UX-10] CaptureHistory & ClipboardHistory async OCR/QR stale-selection guards + [UX-13] filter/events
  {
    const ocrA = deferred()
    const qrA = deferred()
    const { useCaptureHistoryStore } = loadTs('../src/stores/captureHistory.ts', {
      '@wailsio/runtime': { Clipboard: {} },
      '../services/captureApi': {
        getCaptureStatus: async () => ({ count: 2, pinnedCount: 0 }),
        listCaptureEntries: async () => [{ id: 'cap-a', imagePath: 'a.png' }, { id: 'cap-b', imagePath: 'b.png' }],
        getCaptureThumbnailDataURL: async (id) => `thumb:${id}`,
      },
      '../services/captureOverlayApi': {},
      '../services/fancunApi': {},
      '../services/pinnedImageApi': {},
      '../services/qrScanApi': { decodeCaptureQRCode: () => qrA.promise },
      '../services/ocrApi': { recognizeCaptureOCR: () => ocrA.promise },
    })

    const cap = useCaptureHistoryStore()
    await cap.load()
    const pendingOCR = cap.recognizeText({ id: 'cap-a' })
    const pendingQR = cap.scanQRCode({ id: 'cap-a' })
    cap.select('cap-b')
    assert.equal(cap.isRecognizingOCR, false)
    assert.equal(cap.isScanningQR, false)

    ocrA.resolve({ ok: true, text: 'stale-a-ocr', sensitive: false })
    qrA.resolve({ ok: true, text: 'stale-a-qr' })
    await pendingOCR
    await pendingQR
    assert.equal(cap.ocrResult, null, 'stale OCR result for cap-a must not overwrite cap-b')
    assert.equal(cap.qrResult, null, 'stale QR result for cap-a must not overwrite cap-b')

    // ClipboardHistory OCR/QR guard + contentTypeFilter + clipboard-updated subscription
    const clipOcrA = deferred()
    let updatedListener = null
    let loadCount = 0
    const { useClipboardHistoryStore } = loadTs('../src/stores/clipboardHistory.ts', {
      '@wailsio/runtime': { Clipboard: {} },
      '../services/clipboardApi': {
        getClipboardStatus: async () => ({ count: 2, pinnedCount: 0 }),
        listClipboardEntries: async () => {
          loadCount += 1
          return [
            { id: 'clip-img', type: 'image', contentType: 'image', text: '', imagePath: 'img.png' },
            { id: 'clip-json', type: 'text', contentType: 'json', text: '{"a":1}' },
          ]
        },
        getClipboardThumbnailDataURL: async (id) => `clip-thumb:${id}`,
        onClipboardUpdated: (listener) => {
          updatedListener = listener
          return () => {
            updatedListener = null
          }
        },
      },
      '../services/pinnedImageApi': {},
      '../services/ocrApi': { recognizeClipboardImageOCR: () => clipOcrA.promise },
    })

    const clip = useClipboardHistoryStore()
    const unsubscribe = clip.subscribeUpdates()
    await clip.load()
    const pendingClipOcr = clip.recognizeText({ id: 'clip-img', type: 'image' })
    clip.select('clip-json')
    clipOcrA.resolve({ ok: true, text: 'stale-clip-ocr', sensitive: false })
    await pendingClipOcr
    assert.equal(clip.ocrResult, null, 'stale clipboard OCR must not overwrite newly selected entry')

    clip.setContentTypeFilter('json')
    assert.deepEqual(clip.filteredEntries.map((e) => e.id), ['clip-json'])
    const beforeEventLoadCount = loadCount
    updatedListener?.()
    await new Promise((resolve) => setImmediate(resolve))
    assert.equal(loadCount, beforeEventLoadCount + 1, 'fancun:clipboard-updated event should refresh clipboard store')
    unsubscribe()
    console.log('PASS UX-10/UX-13 capture & clipboard stale OCR/QR guards, contentTypeFilter, and event refresh')
  }

  // 5. [PERF-10] JSON Compare debounce & stale guard + [UI-10] Settings loadError
  {
    const req1 = deferred()
    const req2 = deferred()
    let callNum = 0
    const { useJsonCompareStore } = loadTs('../src/stores/jsonCompare.ts', {
      '@wailsio/runtime': { Clipboard: {} },
      '../services/jsonCompareApi': {
        compareJson: async () => {
          callNum += 1
          return callNum === 1 ? req1.promise : req2.promise
        },
        formatJson: async () => ({ ok: true, text: '{}' }),
      },
    })
    const jc = useJsonCompareStore()
    const p1 = jc.compare()
    const p2 = jc.compare()
    req2.resolve({ ok: true, summary: 'newer', differences: [], report: '', unifiedDiff: '', leftFormatted: '', rightFormatted: '', added: 0, removed: 0, changed: 0 })
    await p2
    req1.resolve({ ok: true, summary: 'stale', differences: [], report: '', unifiedDiff: '', leftFormatted: '', rightFormatted: '', added: 1, removed: 0, changed: 0 })
    await p1
    assert.equal(jc.result?.summary, 'newer', 'stale json compare response must not overwrite newer result')

    const { useSettingsStore } = loadTs('../src/stores/settings.ts', {
      '../services/appUpdateApi': { getAppUpdateStatus: async () => ({}) },
      '../services/platformApi': { getPlatformStatus: async () => ({}) },
      '../services/launchersApi': {
        createLauncherDraft: () => ({ id: '', name: '', kind: 'app', target: '', enabled: true }),
        getLauncherStatus: async () => ({ items: [] }),
      },
      '../services/migrationApi': { getLegacyDataStatus: async () => ({}) },
      '../lib/theme': { applyTheme: () => {}, publishTheme: () => {} },
      '../services/pluginsApi': { listPlugins: async () => [] },
      '../services/releaseApi': { getReleaseBackupStatus: async () => ({}) },
      '../services/searchUsageApi': { getSearchUsageStatus: async () => ({}) },
      '../services/secretsApi': { getSecretStatus: async () => ({}) },
      '../services/settingsApi': {
        getSettings: async () => {
          throw new Error('SQLite 损坏')
        },
        getLegacyConfigStatus: async () => ({}),
        getSettingsStorageStatus: async () => ({}),
      },
    })
    const settingsStore = useSettingsStore()
    await settingsStore.load()
    assert.equal(settingsStore.settings, null)
    assert.equal(settingsStore.loadError, 'SQLite 损坏')
    assert.equal(settingsStore.feedbackTone, 'danger')
    console.log('PASS PERF-10/UI-10 jsonCompare stale sequence guard and settings loadError propagation')
  }

  // 6. [P2-4] Desktop RPC errors must NOT be masked by browser mock fallbacks
  {
    installMockWindow({ _wails: { environment: { OS: 'windows' } } })

    const hostsApi = loadTs('../src/services/hostsApi.ts')
    await assert.rejects(() => hostsApi.getHostsStatus(), /不可用|SQLITE|绑定/, 'desktop mode must not fall back to mock hosts status')

    const apiTestingApi = loadTs('../src/services/apiTestingApi.ts')
    await assert.rejects(() => apiTestingApi.getAPITestingStatus(), /不可用|绑定/, 'desktop mode must not fall back to httpbin mock collection')

    const clipboardApi = loadTs('../src/services/clipboardApi.ts')
    await assert.rejects(() => clipboardApi.getClipboardStatus(), /不可用|绑定/)

    const captureApi = loadTs('../src/services/captureApi.ts')
    await assert.rejects(() => captureApi.getCaptureStatus(), /不可用|绑定/)

    const settingsApi = loadTs('../src/services/settingsApi.ts')
    await assert.rejects(() => settingsApi.getSettings(), /不可用|绑定/)

    // Also test when binding exists in desktop mode and throws an SQLite error
    const failingHostsApi = loadTs('../src/services/hostsApi.ts', {
      '../../bindings/fancun/internal/hosts/service.js': {
        Status: async () => {
          throw new Error('SQLITE_BUSY: database is locked')
        },
      },
    })
    await assert.rejects(() => failingHostsApi.getHostsStatus(), /SQLITE_BUSY/)

    // Without desktop runtime markers, browser fallback continues to work
    installMockWindow()
    const browserHostsApi = loadTs('../src/services/hostsApi.ts')
    const browserStatus = await browserHostsApi.getHostsStatus()
    assert.equal(browserStatus.profiles[0].id, 'system-hosts')
    console.log('PASS P2-4 desktop RPC errors are never masked by browser mock fallbacks')
  }
} finally {
  globalThis.window = originalWindow
}
