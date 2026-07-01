import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard, Dialogs } from '@wailsio/runtime'
import {
  commitPushAPIGit,
  configureAPIGit,
  createAPICollection,
  deleteAPICollection,
  getAPIGitStatus,
  getAPIRunSnapshot,
  getAPITestingStatus,
  importAPIRequests,
  newAPIAssertion,
  newAPIEnvironment,
  newAPIHeader,
  newAPIParam,
  newAPIRequest,
  newAPIVariable,
  pullAPIGit,
  runAPIRequest,
  setActiveAPICollection,
  stopAPIRequest,
  upsertAPICollection,
} from '../services/apiTestingApi'
import type { APIAssertion, APICollection, APIEnvironment, APIGitStatus, APIHeader, APIParam, APIRequest, APIRunResult, APITestingStatus, APIVariable } from '../types/ariadne'

export type APIEditorTab = 'params' | 'headers' | 'body' | 'variables' | 'assertions'
export type APIResponseTab = 'body' | 'headers' | 'assertions'

export const useAPITestingStore = defineStore('api-testing', () => {
  const status = ref<APITestingStatus | null>(null)
  const draftCollection = ref<APICollection | null>(null)
  const selectedCollectionId = ref('')
  const selectedRequestId = ref('')
  const selectedEnvironmentId = ref('')
  const editorTab = ref<APIEditorTab>('params')
  const responseTab = ref<APIResponseTab>('body')
  const lastResult = ref<APIRunResult | null>(null)
  const responseByRequestId = ref<Record<string, APIRunResult>>({})
  const openRequestIds = ref<string[]>([])
  const feedback = ref('')
  const saveFeedback = ref('')
  const isLoading = ref(false)
  const isSaving = ref(false)
  const isRunning = ref(false)
  const isStopping = ref(false)
  const isImporting = ref(false)
  const isGitSyncing = ref(false)
  const isDirty = ref(false)
  const isEnvironmentPanelOpen = ref(false)
  const currentRunId = ref('')
  const gitStatus = ref<APIGitStatus | null>(null)
  let runSnapshotTimer: number | undefined

  const collections = computed(() => status.value?.collections ?? [])
  const treeCollections = computed(() => {
    if (!draftCollection.value) return collections.value
    const replaced = collections.value.map((collection) => (collection.id === draftCollection.value?.id ? draftCollection.value : collection))
    if (replaced.some((collection) => collection.id === draftCollection.value?.id)) return replaced
    return [...replaced, draftCollection.value]
  })
  const requestCount = computed(() => treeCollections.value.reduce((total, collection) => total + collection.requests.length, 0))
  const selectedRequest = computed(() => draftCollection.value?.requests.find((request) => request.id === selectedRequestId.value) ?? null)
  const openRequests = computed(() => {
    const requests = draftCollection.value?.requests ?? []
    const openIds = new Set(openRequestIds.value)
    return requests.filter((request) => openIds.has(request.id))
  })
  const selectedEnvironment = computed(
    () => draftCollection.value?.environments.find((environment) => environment.id === selectedEnvironmentId.value) ?? null,
  )
  const enabledAssertionCount = computed(() => selectedRequest.value?.assertions.filter((assertion) => assertion.enabled).length ?? 0)
  const resultTone = computed(() => {
    if (!lastResult.value) return 'idle'
    if (!lastResult.value.ok) return 'error'
    return lastResult.value.failed > 0 ? 'failed' : 'passed'
  })

  async function load() {
    isLoading.value = true
    try {
      status.value = await getAPITestingStatus()
      selectCollection(status.value.activeCollectionId || status.value.collections[0]?.id || '')
      showFeedback(status.value.lastLoadError || '')
    } finally {
      isLoading.value = false
    }
  }

  function selectCollection(id: string) {
    const collection = status.value?.collections.find((item) => item.id === id) ?? status.value?.collections[0]
    if (!collection) return
    selectedCollectionId.value = collection.id
    draftCollection.value = clone(collection)
    selectedRequestId.value = draftCollection.value.activeRequestId || draftCollection.value.requests[0]?.id || ''
    openRequestIds.value = selectedRequestId.value ? [selectedRequestId.value] : []
    selectedEnvironmentId.value = draftCollection.value.activeEnvironmentId || draftCollection.value.environments[0]?.id || ''
    isDirty.value = false
    lastResult.value = selectedRequestId.value ? responseByRequestId.value[selectedRequestId.value] ?? null : null
    gitStatus.value = null
    void setActiveAPICollection(collection.id)
    if (draftCollection.value.git?.path) void refreshGitStatus()
  }

  function selectRequest(id: string) {
    if (!draftCollection.value?.requests.some((request) => request.id === id)) return
    openRequestTab(id)
    selectedRequestId.value = id
    draftCollection.value.activeRequestId = id
    lastResult.value = responseByRequestId.value[id] ?? null
  }

  function openRequestTab(id: string) {
    if (!draftCollection.value?.requests.some((request) => request.id === id)) return
    if (!openRequestIds.value.includes(id)) {
      openRequestIds.value = [...openRequestIds.value, id]
    }
  }

  function closeRequestTab(id: string) {
    const index = openRequestIds.value.indexOf(id)
    if (index < 0) return
    const next = openRequestIds.value.filter((item) => item !== id)
    openRequestIds.value = next
    if (selectedRequestId.value !== id) return
    const fallback = next[index] ?? next[index - 1] ?? ''
    selectedRequestId.value = fallback
    if (draftCollection.value) draftCollection.value.activeRequestId = fallback
    lastResult.value = fallback ? responseByRequestId.value[fallback] ?? null : null
  }

  function closeOtherRequestTabs(id: string) {
    if (!draftCollection.value?.requests.some((request) => request.id === id)) return
    openRequestIds.value = [id]
    selectedRequestId.value = id
    draftCollection.value.activeRequestId = id
    lastResult.value = responseByRequestId.value[id] ?? null
  }

  function closeAllRequestTabs() {
    openRequestIds.value = []
    selectedRequestId.value = ''
    if (draftCollection.value) draftCollection.value.activeRequestId = ''
    lastResult.value = null
  }

  function closeTabsToRight(id: string) {
    const index = openRequestIds.value.indexOf(id)
    if (index < 0) return
    const closing = new Set(openRequestIds.value.slice(index + 1))
    openRequestIds.value = openRequestIds.value.slice(0, index + 1)
    if (closing.has(selectedRequestId.value)) {
      selectedRequestId.value = id
      if (draftCollection.value) draftCollection.value.activeRequestId = id
      lastResult.value = responseByRequestId.value[id] ?? null
    }
  }

  function selectEnvironment(id: string) {
    if (!draftCollection.value?.environments.some((environment) => environment.id === id)) return
    selectedEnvironmentId.value = id
    draftCollection.value.activeEnvironmentId = id
    touchDraft()
  }

  async function createCollection() {
    status.value = await createAPICollection()
    selectCollection(status.value.activeCollectionId)
    showFeedback('集合已创建')
  }

  async function removeCurrentCollection() {
    if (!draftCollection.value || collections.value.length <= 1) {
      showFeedback('至少保留一个集合')
      return
    }
    status.value = await deleteAPICollection(draftCollection.value.id)
    selectCollection(status.value.activeCollectionId)
    showFeedback('集合已删除')
  }

  async function saveCollection() {
    if (!draftCollection.value) return
    const id = draftCollection.value.id
    const previousOpenIds = [...openRequestIds.value]
    const previousRequestId = selectedRequestId.value
    const previousEnvironmentId = selectedEnvironmentId.value
    isSaving.value = true
    try {
      status.value = await upsertAPICollection(clone(draftCollection.value))
      restoreDraftAfterSave(id, previousRequestId, previousOpenIds, previousEnvironmentId)
      isDirty.value = false
      const message = status.value.lastSaveError ? `保存失败：${status.value.lastSaveError}` : '保存成功'
      showSaveFeedback(message)
      showFeedback(message)
    } finally {
      isSaving.value = false
    }
  }

  function restoreDraftAfterSave(collectionId: string, requestId: string, openIds: string[], environmentId: string) {
    const saved = status.value?.collections.find((collection) => collection.id === collectionId)
    if (!saved) {
      selectCollection(collectionId)
      return
    }
    selectedCollectionId.value = saved.id
    draftCollection.value = clone(saved)
    const requestIds = new Set(draftCollection.value.requests.map((request) => request.id))
    openRequestIds.value = openIds.filter((id) => requestIds.has(id))
    if (!openRequestIds.value.length && requestId && requestIds.has(requestId)) {
      openRequestIds.value = [requestId]
    }
    selectedRequestId.value = requestId && requestIds.has(requestId) ? requestId : openRequestIds.value[0] ?? draftCollection.value.activeRequestId ?? ''
    if (selectedRequestId.value) openRequestTab(selectedRequestId.value)
    draftCollection.value.activeRequestId = selectedRequestId.value
    selectedEnvironmentId.value = draftCollection.value.environments.some((environment) => environment.id === environmentId)
      ? environmentId
      : draftCollection.value.activeEnvironmentId || draftCollection.value.environments[0]?.id || ''
    draftCollection.value.activeEnvironmentId = selectedEnvironmentId.value
    lastResult.value = selectedRequestId.value ? responseByRequestId.value[selectedRequestId.value] ?? null : null
    gitStatus.value = null
    if (draftCollection.value.git?.path) void refreshGitStatus()
  }

  async function configureGit(path: string, remote: string) {
    if (!draftCollection.value) return
    isGitSyncing.value = true
    const collectionId = draftCollection.value.id
    const previousOpenIds = [...openRequestIds.value]
    const previousRequestId = selectedRequestId.value
    const previousEnvironmentId = selectedEnvironmentId.value
    try {
      const result = await configureAPIGit(collectionId, path, remote)
      gitStatus.value = result
      status.value = await getAPITestingStatus()
      restoreDraftAfterSave(collectionId, previousRequestId, previousOpenIds, previousEnvironmentId)
      showFeedback(result.ok ? result.message : result.error || result.message)
    } finally {
      isGitSyncing.value = false
    }
  }

  async function refreshGitStatus() {
    if (!draftCollection.value?.git?.path) {
      gitStatus.value = null
      return
    }
    isGitSyncing.value = true
    try {
      gitStatus.value = await getAPIGitStatus(draftCollection.value.id)
    } finally {
      isGitSyncing.value = false
    }
  }

  async function pullGit() {
    if (!draftCollection.value?.git?.path) return
    isGitSyncing.value = true
    const collectionId = draftCollection.value.id
    const previousOpenIds = [...openRequestIds.value]
    const previousRequestId = selectedRequestId.value
    const previousEnvironmentId = selectedEnvironmentId.value
    try {
      const result = await pullAPIGit(collectionId)
      gitStatus.value = result
      status.value = await getAPITestingStatus()
      restoreDraftAfterSave(collectionId, previousRequestId, previousOpenIds, previousEnvironmentId)
      showFeedback(result.ok ? result.message : result.error || result.message)
    } finally {
      isGitSyncing.value = false
    }
  }

  async function commitPushGit(message: string) {
    if (!draftCollection.value?.git?.path) return
    isGitSyncing.value = true
    const collectionId = draftCollection.value.id
    try {
      await saveCollection()
      const result = await commitPushAPIGit(collectionId, message)
      gitStatus.value = result
      showFeedback(result.ok ? result.message : result.error || result.message)
    } finally {
      isGitSyncing.value = false
    }
  }

  function updateCollectionName(name: string) {
    if (!draftCollection.value) return
    draftCollection.value.name = name
    touchDraft()
  }

  function createRequest(folder = '', fields: Partial<Omit<APIRequest, 'id' | 'updatedAt'>> = {}) {
    if (!draftCollection.value) return
    const request = newAPIRequest()
    request.folder = folder
    Object.assign(request, fields, { updatedAt: now() })
    draftCollection.value.requests.push(request)
    selectedRequestId.value = request.id
    openRequestTab(request.id)
    draftCollection.value.activeRequestId = request.id
    lastResult.value = null
    touchDraft()
  }

  function duplicateRequest(requestId = selectedRequestId.value) {
    if (!draftCollection.value) return
    const source = draftCollection.value.requests.find((request) => request.id === requestId)
    if (!source) return
    const copy = clone(source)
    copy.id = newAPIRequest().id
    copy.name = `${copy.name} 副本`
    copy.updatedAt = now()
    draftCollection.value.requests.push(copy)
    selectedRequestId.value = copy.id
    openRequestTab(copy.id)
    draftCollection.value.activeRequestId = copy.id
    lastResult.value = null
    touchDraft()
  }

  function removeRequest(requestId = selectedRequestId.value) {
    if (!draftCollection.value) return
    if (!draftCollection.value.requests.some((request) => request.id === requestId)) return
    if (draftCollection.value.requests.length <= 1) {
      showFeedback('至少保留一个请求')
      return
    }
    draftCollection.value.requests = draftCollection.value.requests.filter((request) => request.id !== requestId)
    const nextOpen = openRequestIds.value.filter((id) => id !== requestId && draftCollection.value?.requests.some((request) => request.id === id))
    openRequestIds.value = nextOpen
    selectedRequestId.value = nextOpen[0] ?? draftCollection.value.requests[0]?.id ?? ''
    if (selectedRequestId.value) openRequestTab(selectedRequestId.value)
    draftCollection.value.activeRequestId = selectedRequestId.value
    clearResponseForRequest(requestId)
    lastResult.value = selectedRequestId.value ? responseByRequestId.value[selectedRequestId.value] ?? null : null
    touchDraft()
  }

  function removeSelectedRequest() {
    removeRequest()
  }

  function renameRequest(requestId: string, name: string) {
    if (!draftCollection.value) return
    const request = draftCollection.value.requests.find((item) => item.id === requestId)
    const next = name.trim()
    if (!request || !next) return
    request.name = next
    request.updatedAt = now()
    touchDraft()
  }

  function moveRequestToFolder(requestId: string, folder: string) {
    if (!draftCollection.value) return
    const request = draftCollection.value.requests.find((item) => item.id === requestId)
    if (!request) return
    request.folder = folder.trim()
    request.updatedAt = now()
    touchDraft()
  }

  function renameFolder(currentName: string, nextName: string) {
    if (!draftCollection.value) return
    const current = currentName.trim()
    const next = nextName.trim()
    if (!current || !next || current === next) return
    for (const request of draftCollection.value.requests) {
      if ((request.folder || '未分组') === current) {
        request.folder = next === '未分组' ? '' : next
        request.updatedAt = now()
      }
    }
    touchDraft()
  }

  function updateRequest(fields: Partial<Pick<APIRequest, 'name' | 'folder' | 'method' | 'url' | 'bodyType' | 'body'>>) {
    if (!selectedRequest.value) return
    Object.assign(selectedRequest.value, fields, { updatedAt: now() })
    clearResponseForRequest(selectedRequest.value.id)
    lastResult.value = null
    touchDraft()
  }

  function addHeader() {
    selectedRequest.value?.headers.push(newAPIHeader())
    touchDraft()
  }

  function updateHeader(index: number, fields: Partial<APIHeader>) {
    const header = selectedRequest.value?.headers[index]
    if (!header) return
    Object.assign(header, fields)
    touchDraft()
  }

  function removeHeader(index: number) {
    selectedRequest.value?.headers.splice(index, 1)
    touchDraft()
  }

  function addParam() {
    selectedRequest.value?.params.push(newAPIParam())
    touchDraft()
  }

  function updateParam(index: number, fields: Partial<APIParam>) {
    const param = selectedRequest.value?.params[index]
    if (!param) return
    Object.assign(param, fields)
    touchDraft()
  }

  function removeParam(index: number) {
    selectedRequest.value?.params.splice(index, 1)
    touchDraft()
  }

  function addCollectionVariable() {
    draftCollection.value?.variables.push(newAPIVariable())
    touchDraft()
  }

  function updateCollectionVariable(index: number, fields: Partial<APIVariable>) {
    const variable = draftCollection.value?.variables[index]
    if (!variable) return
    Object.assign(variable, fields)
    touchDraft()
  }

  function removeCollectionVariable(index: number) {
    draftCollection.value?.variables.splice(index, 1)
    touchDraft()
  }

  function createEnvironment() {
    if (!draftCollection.value) return
    const environment = newAPIEnvironment()
    draftCollection.value.environments.push(environment)
    selectedEnvironmentId.value = environment.id
    draftCollection.value.activeEnvironmentId = environment.id
    isEnvironmentPanelOpen.value = true
    touchDraft()
  }

  function duplicateEnvironment() {
    if (!draftCollection.value || !selectedEnvironment.value) return
    const copy = clone(selectedEnvironment.value)
    copy.id = newAPIEnvironment().id
    copy.name = `${copy.name} 副本`
    copy.updatedAt = now()
    copy.variables = copy.variables.map((variable) => ({ ...variable, id: newAPIVariable().id }))
    draftCollection.value.environments.push(copy)
    selectedEnvironmentId.value = copy.id
    draftCollection.value.activeEnvironmentId = copy.id
    isEnvironmentPanelOpen.value = true
    touchDraft()
  }

  function updateEnvironment(fields: Partial<Pick<APIEnvironment, 'name'>>) {
    if (!selectedEnvironment.value) return
    Object.assign(selectedEnvironment.value, fields, { updatedAt: now() })
    touchDraft()
  }

  function removeSelectedEnvironment() {
    if (!draftCollection.value || !selectedEnvironment.value) return
    if (draftCollection.value.environments.length <= 1) {
      showFeedback('至少保留一个环境')
      return
    }
    draftCollection.value.environments = draftCollection.value.environments.filter((environment) => environment.id !== selectedEnvironmentId.value)
    selectedEnvironmentId.value = draftCollection.value.environments[0]?.id ?? ''
    draftCollection.value.activeEnvironmentId = selectedEnvironmentId.value
    touchDraft()
  }

  function addEnvironmentVariable() {
    selectedEnvironment.value?.variables.push(newAPIVariable())
    touchDraft()
  }

  function updateEnvironmentVariable(index: number, fields: Partial<APIVariable>) {
    const variable = selectedEnvironment.value?.variables[index]
    if (!variable) return
    Object.assign(variable, fields)
    touchDraft()
  }

  function removeEnvironmentVariable(index: number) {
    selectedEnvironment.value?.variables.splice(index, 1)
    touchDraft()
  }

  function openEnvironmentPanel() {
    isEnvironmentPanelOpen.value = true
  }

  function closeEnvironmentPanel() {
    isEnvironmentPanelOpen.value = false
  }

  function addAssertion() {
    selectedRequest.value?.assertions.push(newAPIAssertion())
    touchDraft()
  }

  function updateAssertion(index: number, fields: Partial<APIAssertion>) {
    const assertion = selectedRequest.value?.assertions[index]
    if (!assertion) return
    Object.assign(assertion, fields)
    touchDraft()
  }

  function removeAssertion(index: number) {
    selectedRequest.value?.assertions.splice(index, 1)
    touchDraft()
  }

  async function runSelectedRequest() {
    if (!draftCollection.value || !selectedRequest.value || isRunning.value) return
    const request = clone(selectedRequest.value)
    const requestId = request.id
    const runId = newRunId()
    const stream = shouldKeepConnectionOpen(request)
    isRunning.value = true
    isStopping.value = false
    currentRunId.value = runId
    clearResponseForRequest(requestId)
    lastResult.value = null
    startRunSnapshotPolling(runId, requestId)
    try {
      const result = await runAPIRequest({
        collectionId: draftCollection.value.id,
        environmentId: selectedEnvironmentId.value,
        request,
        timeoutSeconds: stream ? 0 : 30,
        runId,
        stream,
      })
      setResponseForRequest(requestId, result)
      showFeedback(result.message || (result.ok ? '请求完成' : '请求失败'))
    } finally {
      stopRunSnapshotPolling()
      isRunning.value = false
      isStopping.value = false
      currentRunId.value = ''
    }
  }

  async function stopSelectedRequest() {
    if (!currentRunId.value || isStopping.value) return
    isStopping.value = true
    const result = await stopAPIRequest(currentRunId.value)
    showFeedback(result.message)
    if (!result.ok) {
      isStopping.value = false
    }
  }

  function startRunSnapshotPolling(runId: string, requestId: string) {
    stopRunSnapshotPolling()
    const poll = async () => {
      if (currentRunId.value !== runId) return
      const snapshot = await getAPIRunSnapshot(runId)
      if (currentRunId.value !== runId) return
      if (snapshot.ok && snapshot.running && snapshot.result.requestUrl) {
        setResponseForRequest(requestId, snapshot.result)
      }
    }
    void poll()
    runSnapshotTimer = window.setInterval(() => {
      void poll()
    }, 300)
  }

  function stopRunSnapshotPolling() {
    if (runSnapshotTimer === undefined) return
    window.clearInterval(runSnapshotTimer)
    runSnapshotTimer = undefined
  }

  function setResponseForRequest(requestId: string, result: APIRunResult) {
    responseByRequestId.value = { ...responseByRequestId.value, [requestId]: result }
    if (selectedRequestId.value === requestId) {
      lastResult.value = result
    }
  }

  function clearResponseForRequest(requestId: string) {
    if (!responseByRequestId.value[requestId]) return
    const next = { ...responseByRequestId.value }
    delete next[requestId]
    responseByRequestId.value = next
  }

  async function importRequests() {
    if (!draftCollection.value) return
    isImporting.value = true
    try {
      const selected = await Dialogs.OpenFile({
        Title: '导入请求',
        ButtonText: '导入',
        Filters: [
          { DisplayName: 'API 集合', Pattern: '*.json;*.postman_collection.json' },
          { DisplayName: '所有文件', Pattern: '*.*' },
        ],
      })
      const path = Array.isArray(selected) ? selected[0] : selected
      if (!path) {
        showFeedback('已取消导入')
        return
      }
      const result = await importAPIRequests(path, draftCollection.value.id)
      status.value = result.status
      selectCollection(result.status.activeCollectionId || draftCollection.value.id)
      showFeedback(result.ok ? result.message : result.error || result.message)
    } catch (error) {
      showFeedback(error instanceof Error ? error.message : '导入失败')
    } finally {
      isImporting.value = false
    }
  }

  async function copyResponseBody() {
    const text = lastResult.value?.body ?? ''
    if (!text) {
      showFeedback('没有可复制的响应体')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('响应体已复制')
    } catch {
      await navigator.clipboard?.writeText(text)
      showFeedback('响应体已复制')
    }
  }

  function touchDraft() {
    if (draftCollection.value) {
      draftCollection.value.updatedAt = now()
    }
    isDirty.value = true
  }

  function showFeedback(message: string) {
    if (!message) return
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) feedback.value = ''
    }, 1800)
  }

  function showSaveFeedback(message: string) {
    if (!message) return
    saveFeedback.value = message
    window.setTimeout(() => {
      if (saveFeedback.value === message) saveFeedback.value = ''
    }, 2200)
  }

  return {
    status,
    draftCollection,
    selectedCollectionId,
    selectedRequestId,
    selectedEnvironmentId,
    editorTab,
    responseTab,
    lastResult,
    openRequestIds,
    feedback,
    saveFeedback,
    isLoading,
    isSaving,
    isRunning,
    isStopping,
    isImporting,
    isGitSyncing,
    isDirty,
    isEnvironmentPanelOpen,
    gitStatus,
    collections,
    treeCollections,
    requestCount,
    selectedRequest,
    openRequests,
    selectedEnvironment,
    enabledAssertionCount,
    resultTone,
    load,
    selectCollection,
    selectRequest,
    closeRequestTab,
    closeOtherRequestTabs,
    closeAllRequestTabs,
    closeTabsToRight,
    selectEnvironment,
    createCollection,
    removeCurrentCollection,
    saveCollection,
    configureGit,
    refreshGitStatus,
    pullGit,
    commitPushGit,
    updateCollectionName,
    createRequest,
    duplicateRequest,
    removeRequest,
    removeSelectedRequest,
    renameRequest,
    moveRequestToFolder,
    renameFolder,
    updateRequest,
    addHeader,
    updateHeader,
    removeHeader,
    addParam,
    updateParam,
    removeParam,
    addCollectionVariable,
    updateCollectionVariable,
    removeCollectionVariable,
    createEnvironment,
    duplicateEnvironment,
    updateEnvironment,
    removeSelectedEnvironment,
    addEnvironmentVariable,
    updateEnvironmentVariable,
    removeEnvironmentVariable,
    openEnvironmentPanel,
    closeEnvironmentPanel,
    addAssertion,
    updateAssertion,
    removeAssertion,
    runSelectedRequest,
    stopSelectedRequest,
    importRequests,
    copyResponseBody,
  }
})

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function now() {
  return Math.floor(Date.now() / 1000)
}

function newRunId() {
  return `run-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function shouldKeepConnectionOpen(request: APIRequest) {
  const url = request.url.toLowerCase()
  if (/(^|[/?&#._-])(sse|stream)([/?&#._-]|$)|event-stream/i.test(url)) return true
  return request.headers.some((header) => {
    const name = header.name.trim().toLowerCase()
    const value = header.value.trim().toLowerCase()
    return header.enabled && name === 'accept' && value.includes('event-stream')
  })
}
