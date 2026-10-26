import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const require = createRequire(import.meta.url)
const { createPinia, setActivePinia } = require('pinia')
const originalWindow = globalThis.window
globalThis.window = { setTimeout: (callback, delay) => setTimeout(callback, delay).unref() }

// Run the real stores with controllable API promises and sockets.
function loadStore(path, mocks) {
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
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

try {
  const lists = new Map()
  const images = new Map()
  const imageCalls = []
  let list = (query) => {
    const request = deferred()
    lists.set(query, request)
    return request.promise
  }
  const { useClipboardHistoryStore } = loadStore('../src/stores/clipboardHistory.ts', {
    '@wailsio/runtime': { Clipboard: {} },
    '../services/clipboardApi': {
      getClipboardStatus: async () => ({ count: 0 }),
      listClipboardEntries: (query) => list(query),
      getClipboardThumbnailDataURL: (id) => {
        imageCalls.push(id)
        return images.get(id).promise
      },
    },
    '../services/pinnedImageApi': {},
    '../services/ocrApi': {},
  })
  const clipboard = useClipboardHistoryStore()
  const old = clipboard.load('old')
  const current = clipboard.load('current')
  lists.get('current').resolve([{ id: 'new', type: 'text' }])
  await current
  lists.get('old').resolve([{ id: 'old', type: 'text' }])
  await old
  assert.equal(clipboard.entries[0].id, 'new', 'old results must not replace a newer query')
  assert.equal(clipboard.selectedId, 'new')

  const failing = clipboard.load('failing')
  const waiting = clipboard.load('waiting')
  lists.get('failing').reject(new Error('old request failed'))
  await failing
  assert.equal(clipboard.isLoading, true, 'a stale completion must not clear the current loading state')
  assert.equal(clipboard.feedback, '', 'a stale failure must not report an error on the current query')
  lists.get('waiting').resolve([])
  await waiting
  assert.equal(clipboard.isLoading, false)

  images.set('a', deferred())
  images.set('b', deferred())
  list = async () => [{ id: 'a', type: 'image' }, { id: 'b', type: 'image' }, { id: 'text', type: 'text' }]
  const firstImage = clipboard.load()
  // Let the API list resolve, leaving the first thumbnail pending.
  await new Promise((resolve) => setImmediate(resolve))
  clipboard.select('b')
  images.get('b').resolve('image-b')
  await new Promise((resolve) => setImmediate(resolve))
  images.get('a').resolve('image-a')
  await firstImage
  assert.equal(clipboard.imageDataUrl, 'image-b', 'an old thumbnail must not replace the selected image')
  await clipboard.load()
  assert.deepEqual(imageCalls, ['a', 'b'], 'refreshing unchanged selection must reuse its thumbnail')

  images.set('a', deferred())
  clipboard.select('a')
  const duplicateImage = clipboard.load()
  await new Promise((resolve) => setImmediate(resolve))
  assert.deepEqual(imageCalls, ['a', 'b', 'a'], 'overlapping refresh must share the selected thumbnail request')
  clipboard.select('text')
  images.get('a').reject(new Error('late thumbnail failure'))
  await duplicateImage
  assert.equal(clipboard.imageDataUrl, '')
  assert.equal(clipboard.feedback, '', 'a stale thumbnail error must not affect another selection')
  console.log('PASS clipboard query order, loading state, thumbnail order and request reuse')

  const sockets = []
  let failCreation = false
  const { useNetworkMonitorStore } = loadStore('../src/stores/networkMonitor.ts', {
    '@wailsio/runtime': {
      JSONStream: () => {
        if (failCreation) throw new Error('cannot open stream')
        const socket = { closeCount: 0, send() {}, close() { this.closeCount++ } }
        sockets.push(socket)
        return socket
      },
    },
    '../services/networkMonitorApi': {
      normalizeSnapshot: (value) => value,
      normalizeProcessSnapshot: (value) => value,
    },
  })
  const network = useNetworkMonitorStore()
  network.startPolling()
  assert.equal(network.trafficConnection, 'connecting')
  sockets[0].onopen({})
  assert.equal(network.trafficConnection, 'connected')
  sockets[0].onmessage({ data: { traffic: { bytesSent: 1 } } })
  sockets[0].onerror({})
  sockets[0].onclose({})
  assert.equal(network.trafficConnection, 'disconnected')
  assert.equal(network.isPolling, false)
  assert.equal(sockets[0].closeCount, 1)
  network.startPolling()
  assert.equal(sockets.length, 2, 'disconnect must release the old subscription so the user can reconnect')
  sockets[1].onopen({})
  sockets[1].onmessage({ data: { traffic: { bytesSent: 2 } } })
  sockets[0].onmessage({ data: { traffic: { bytesSent: 99 } } })
  sockets[0].onclose({})
  assert.equal(network.snapshot.bytesSent, 2)
  assert.equal(network.trafficConnection, 'connected', 'late events from an old stream must not disconnect the new one')
  assert.equal(network.trafficError, '')

  network.startProcessPolling()
  sockets[2].onopen({})
  sockets[2].onclose({})
  assert.equal(network.processConnection, 'disconnected')
  assert.equal(network.trafficConnection, 'connected', 'traffic and process subscriptions must remain independent')
  network.startProcessPolling()
  sockets[3].onopen({})
  assert.equal(network.processConnection, 'connected')
  network.stopProcessPolling()
  sockets[3].onclose({})
  assert.equal(network.processConnection, 'stopped')

  network.stopPolling()
  failCreation = true
  network.startPolling()
  assert.equal(network.trafficConnection, 'disconnected')
  assert.match(network.trafficError, /cannot open stream/)
  failCreation = false
  network.startPolling()
  sockets[4].onopen({})
  assert.equal(network.trafficConnection, 'connected', 'failed stream creation must not block the next attempt')
  network.stopPolling()
  console.log('PASS telemetry disconnect, manual reconnect, late events, independent streams and creation failure')

  const requests = []
  const mocks = {
    '@wailsio/runtime': { Clipboard: {} },
    '../lib/toolWindows': { toolViewForCommand: () => undefined },
    './appShell': {},
    '../services/ariadneApi': {
      createAriadneSearchRequest: (query, progress) => {
        const request = { ...deferred(), query, progress, cancelCount: 0, cancel() { this.cancelCount++ } }
        requests.push(request)
        return request
      },
      isSearchCancelled: () => false,
    },
  }
  for (const name of ['captureApi', 'captureOverlayApi', 'clipboardApi', 'qrScanApi', 'imageIndexApi', 'pinnedImageApi', 'workflowApi']) {
    mocks[`../services/${name}`] = {}
  }
  const { useLauncherStore } = loadStore('../src/stores/launcher.ts', mocks)
  const launcher = useLauncherStore()
  const response = (...ids) => ({ query: 'same', results: ids.map((id) => ({ id })), elapsedMs: 10, totalResults: ids.length })
  const firstSearch = launcher.refreshResults('same')
  requests[0].progress(response('first'))
  assert.equal(launcher.selectedId, 'first', 'partial results must be usable before completion')
  requests[0].progress(response('ranked-first', 'first'))
  assert.equal(launcher.selectedId, 'ranked-first', 'default selection must follow ranking until the user selects a result')
  const secondSearch = launcher.refreshResults('same')
  assert.equal(requests[0].cancelCount, 1)
  requests[1].progress(response('a', 'b'))
  launcher.select('b')
  requests[0].progress(response('stale'))
  assert.equal(launcher.selectedId, 'b', 'an older request for identical text must be ignored')
  requests[1].progress(response('higher-ranked', 'a', 'b'))
  assert.equal(launcher.selectedId, 'b', 'incoming results must preserve keyboard selection')
  requests[1].resolve(response('higher-ranked', 'a', 'b', 'final'))
  await secondSearch
  requests[0].resolve(response('stale-final'))
  await firstSearch
  assert.equal(launcher.selectedId, 'b')
  assert.equal(launcher.results.length, 4)
  const resetSearch = launcher.refreshResults('same')
  launcher.reset()
  requests[2].progress(response('late'))
  requests[2].resolve(response('late-final'))
  await resetSearch
  assert.equal(launcher.results.length, 0)
  console.log('PASS progressive search, repeated query isolation, keyboard selection and reset')

  const listeners = new Set()
  const backendRequests = []
  const { createAriadneSearchRequest } = loadStore('../src/services/ariadneApi.ts', {
    '../data/seed': { seedResults: [] },
    '@wailsio/runtime': { Events: {
      On: (_name, listener) => { listeners.add(listener); return () => listeners.delete(listener) },
    } },
    '../../bindings/ariadne/internal/search/service.js': {
      SearchProgressively: (_query, requestId) => {
        const request = deferred()
        request.promise.cancel = () => request.reject({ name: 'CancelError' })
        backendRequests.push({ ...request, requestId })
        return request.promise
      },
    },
  })
  const progressValues = []
  const apiFirst = createAriadneSearchRequest('same', (value) => progressValues.push(value))
  const apiSecond = createAriadneSearchRequest('same', (value) => progressValues.push(value))
  await new Promise(setImmediate)
  assert.equal(listeners.size, 2)
  for (const listener of listeners) listener({ data: { requestId: backendRequests[1].requestId, response: response('current') } })
  assert.equal(progressValues.length, 1, 'events must match the request token, even for identical queries')
  const rejected = assert.rejects(apiFirst.promise, { name: 'SearchCancelledError' })
  apiFirst.cancel()
  await rejected
  assert.equal(listeners.size, 1, 'cancellation must unsubscribe progress events')
  backendRequests[1].resolve(response('done'))
  await apiSecond.promise
  assert.equal(listeners.size, 0, 'completion must unsubscribe progress events')
  console.log('PASS progress event request tokens and subscription cleanup')
} finally {
  globalThis.window = originalWindow
}
