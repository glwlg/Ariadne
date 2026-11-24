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
import type { APIAssertion, APICollection, APIEnvironment, APIGitStatus, APIHeader, APIParam, APIRequest, APIRunResult, APITestingStatus, APIVariable } from '../types/fancun'

export type APIEditorTab = 'params' | 'headers' | 'body' | 'variables' | 'assertions'
export type APIResponseTab = 'body' | 'headers' | 'assertions'

const ARMED_TIMEOUT_MS = 5000

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
  const feedbackTone = ref<'success' | 'danger'>('success')
  const saveFeedback = ref('')
  const saveFeedbackTone = ref<'success' | 'danger'>('success')
  const loadError = ref('')
  const isLoading = ref(false)
  const isSaving = ref(false)
  const runningRequestIds = ref<Record<string, boolean>>({})
  const stoppingRequestIds = ref<Record<string, boolean>>({})
  const runIdByRequestId = ref<Record<string, string>>({})
  const runStartedAtByRequestId = ref<Record<string, number>>({})
  const requestTimeoutSeconds = ref(30)
  const isImporting = ref(false)
  const isGitSyncing = ref(false)
  const isDirty = ref(false)
  const isEnvironmentPanelOpen = ref(false)
  const envDeleteArmedId = ref('')
  const gitStatus = ref<APIGitStatus | null>(null)
  const snapshotTimers = new Map<string, number>()
  const lastSnapshotMetaByRunId = new Map<string, { bodySize: number; statusCode: number; running: boolean }>()
  let feedbackTimer: number | undefined
  let saveFeedbackTimer: number | undefined
  let envDeleteArmedTimer: number | undefined

  const isRunning = computed(() => Boolean(selectedRequestId.value && runningRequestIds.value[selectedRequestId.value]))
  const isStopping = computed(() => Boolean(selectedRequestId.value && stoppingRequestIds.value[selectedRequestId.value]))
  const currentRunId = computed(() => (selectedRequestId.value ? runIdByRequestId.value[selectedRequestId.value] ?? '' : ''))
  const currentRunStartedAt = computed(() => (selectedRequestId.value ? runStartedAtByRequestId.value[selectedRequestId.value] ?? 0 : 0))

  function isRequestRunning(requestId: string) {
    return Boolean(requestId && runningRequestIds.value[requestId])
  }

  function isRequestStopping(requestId: string) {
    return Boolean(requestId && stoppingRequestIds.value[requestId])
  }

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
    loadError.value = ''
    try {
      status.value = await getAPITestingStatus()
      selectCollection(status.value.activeCollectionId || status.value.collections[0]?.id || '')
      if (status.value.lastLoadError) {
        loadError.value = status.value.lastLoadError
        showFeedback(status.value.lastLoadError, 'danger')
      }
    } catch (error) {
      loadError.value = formatError(error, 'API 测试加载失败')
      showFeedback(loadError.value, 'danger')
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
    try {
      const nextStatus = await createAPICollection()
      status.value = nextStatus
      if (nextStatus.lastSaveError) {
        showFeedback(`创建失败：${nextStatus.lastSaveError}`, 'danger')
        return
      }
      selectCollection(nextStatus.activeCollectionId)
      showFeedback('集合已创建', 'success')
    } catch (error) {
      showFeedback(`创建失败：${formatError(error, '创建集合失败')}`, 'danger')
    }
  }

  async function removeCurrentCollection() {
    if (!draftCollection.value || collections.value.length <= 1) {
      showFeedback('至少保留一个集合', 'danger')
      return
    }
    try {
      const nextStatus = await deleteAPICollection(draftCollection.value.id)
      status.value = nextStatus
      if (nextStatus.lastSaveError) {
        showFeedback(`删除失败：${nextStatus.lastSaveError}`, 'danger')
        return
      }
      selectCollection(nextStatus.activeCollectionId)
      showFeedback('集合已删除', 'success')
    } catch (error) {
      showFeedback(`删除失败：${formatError(error, '删除集合失败')}`, 'danger')
    }
  }

  async function saveCollection(): Promise<boolean> {
    if (!draftCollection.value) return false
    const id = draftCollection.value.id
    const previousOpenIds = [...openRequestIds.value]
    const previousRequestId = selectedRequestId.value
    const previousEnvironmentId = selectedEnvironmentId.value
    isSaving.value = true
    try {
      const nextStatus = await upsertAPICollection(clone(draftCollection.value))
      status.value = nextStatus
      if (nextStatus.lastSaveError) {
        const message = `保存失败：${nextStatus.lastSaveError}`
        showSaveFeedback(message, 'danger')
        showFeedback(message, 'danger')
        return false
      }
      restoreDraftAfterSave(id, previousRequestId, previousOpenIds, previousEnvironmentId)
      isDirty.value = false
      showSaveFeedback('保存成功', 'success')
      showFeedback('保存成功', 'success')
      return true
    } catch (error) {
      const message = `保存失败：${formatError(error, '保存集合失败')}`
      showSaveFeedback(message, 'danger')
      showFeedback(message, 'danger')
      return false
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
      showFeedback(result.ok ? result.message : result.error || result.message, result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(formatError(error, '配置 Git 同步失败'), 'danger')
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
    } catch (error) {
      showFeedback(formatError(error, '读取 Git 状态失败'), 'danger')
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
      showFeedback(result.ok ? result.message : result.error || result.message, result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(formatError(error, 'Git 拉取失败'), 'danger')
    } finally {
      isGitSyncing.value = false
    }
  }

  async function commitPushGit(message: string) {
    if (!draftCollection.value?.git?.path) return
    isGitSyncing.value = true
    const collectionId = draftCollection.value.id
    try {
      const saved = await saveCollection()
      if (!saved) return
      const result = await commitPushAPIGit(collectionId, message)
      gitStatus.value = result
      showFeedback(result.ok ? result.message : result.error || result.message, result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(formatError(error, 'Git 提交推送失败'), 'danger')
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
      showFeedback('至少保留一个环境', 'danger')
      return
    }
    disarmEnvDelete()
    draftCollection.value.environments = draftCollection.value.environments.filter((environment) => environment.id !== selectedEnvironmentId.value)
    selectedEnvironmentId.value = draftCollection.value.environments[0]?.id ?? ''
    draftCollection.value.activeEnvironmentId = selectedEnvironmentId.value
    touchDraft()
  }

  function confirmRemoveSelectedEnvironment() {
    if (!draftCollection.value || !selectedEnvironment.value) return
    if (draftCollection.value.environments.length <= 1) {
      showFeedback('至少保留一个环境', 'danger')
      return
    }
    if (envDeleteArmedId.value !== selectedEnvironment.value.id) {
      armEnvDelete(selectedEnvironment.value.id)
      showFeedback('再次点击确认删除环境', 'danger')
      return
    }
    removeSelectedEnvironment()
  }

  function armEnvDelete(id: string) {
    disarmEnvDelete()
    envDeleteArmedId.value = id
    envDeleteArmedTimer = window.setTimeout(() => {
      envDeleteArmedTimer = undefined
      if (envDeleteArmedId.value === id) envDeleteArmedId.value = ''
    }, ARMED_TIMEOUT_MS)
  }

  function disarmEnvDelete() {
    envDeleteArmedId.value = ''
    if (envDeleteArmedTimer !== undefined) {
      window.clearTimeout(envDeleteArmedTimer)
      envDeleteArmedTimer = undefined
    }
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
    disarmEnvDelete()
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

  async function runSelectedRequest(timeoutOverride?: number) {
    if (!draftCollection.value || !selectedRequest.value) return
    const request = clone(selectedRequest.value)
    const requestId = request.id
    if (isRequestRunning(requestId)) return
    const runId = newRunId()
    const stream = shouldKeepConnectionOpen(request)
    const timeoutSeconds = stream ? 0 : Math.max(1, Math.round(timeoutOverride ?? requestTimeoutSeconds.value ?? 30))

    runningRequestIds.value = { ...runningRequestIds.value, [requestId]: true }
    stoppingRequestIds.value = { ...stoppingRequestIds.value, [requestId]: false }
    runIdByRequestId.value = { ...runIdByRequestId.value, [requestId]: runId }
    runStartedAtByRequestId.value = { ...runStartedAtByRequestId.value, [requestId]: Date.now() }

    clearResponseForRequest(requestId)
    if (selectedRequestId.value === requestId) {
      lastResult.value = null
    }
    startRunSnapshotPolling(runId, requestId)
    try {
      const result = await runAPIRequest({
        collectionId: draftCollection.value.id,
        environmentId: selectedEnvironmentId.value,
        request,
        timeoutSeconds,
        runId,
        stream,
      })
      setResponseForRequest(requestId, result)
      showFeedback(result.message || (result.ok ? '请求完成' : '请求失败'), result.ok ? 'success' : 'danger')
    } catch (error) {
      const message = formatError(error, '请求发送失败')
      setResponseForRequest(requestId, {
        ok: false,
        message,
        method: request.method,
        requestUrl: request.url,
        statusCode: 0,
        statusText: '',
        durationMs: Math.max(0, Date.now() - (runStartedAtByRequestId.value[requestId] ?? Date.now())),
        headers: [],
        body: '',
        bodySize: 0,
        bodyTruncated: false,
        contentType: '',
        assertionResults: [],
        passed: 0,
        failed: 0,
        error: message,
      })
      showFeedback(message, 'danger')
    } finally {
      stopRunSnapshotPolling(runId)
      const nextRunning = { ...runningRequestIds.value }
      delete nextRunning[requestId]
      runningRequestIds.value = nextRunning

      const nextStopping = { ...stoppingRequestIds.value }
      delete nextStopping[requestId]
      stoppingRequestIds.value = nextStopping

      const nextRunIds = { ...runIdByRequestId.value }
      if (nextRunIds[requestId] === runId) {
        delete nextRunIds[requestId]
      }
      runIdByRequestId.value = nextRunIds

      const nextStartedAt = { ...runStartedAtByRequestId.value }
      delete nextStartedAt[requestId]
      runStartedAtByRequestId.value = nextStartedAt
    }
  }

  async function stopSelectedRequest() {
    const requestId = selectedRequestId.value
    const runId = requestId ? runIdByRequestId.value[requestId] : ''
    if (!requestId || !runId || isRequestStopping(requestId)) return
    stoppingRequestIds.value = { ...stoppingRequestIds.value, [requestId]: true }
    try {
      const result = await stopAPIRequest(runId)
      showFeedback(result.message, result.ok ? 'success' : 'danger')
      if (!result.ok) {
        stoppingRequestIds.value = { ...stoppingRequestIds.value, [requestId]: false }
      }
    } catch (error) {
      stoppingRequestIds.value = { ...stoppingRequestIds.value, [requestId]: false }
      showFeedback(formatError(error, '停止请求失败'), 'danger')
    }
  }

  function startRunSnapshotPolling(runId: string, requestId: string) {
    stopRunSnapshotPolling(runId)
    const poll = async () => {
      if (runIdByRequestId.value[requestId] !== runId) return
      try {
        const snapshot = await getAPIRunSnapshot(runId)
        if (runIdByRequestId.value[requestId] !== runId) return
        if (snapshot.ok && snapshot.running && snapshot.result.requestUrl) {
          const prevMeta = lastSnapshotMetaByRunId.get(runId)
          if (
            !prevMeta ||
            prevMeta.bodySize !== snapshot.result.bodySize ||
            prevMeta.statusCode !== snapshot.result.statusCode ||
            prevMeta.running !== snapshot.running
          ) {
            lastSnapshotMetaByRunId.set(runId, {
              bodySize: snapshot.result.bodySize,
              statusCode: snapshot.result.statusCode,
              running: snapshot.running,
            })
            setResponseForRequest(requestId, snapshot.result)
          }
        }
      } catch {
        // Snapshot polling failures are non-fatal while main Run() is active.
      }
    }
    void poll()
    const timer = window.setInterval(() => {
      void poll()
    }, 300)
    snapshotTimers.set(runId, timer)
  }

  function stopRunSnapshotPolling(runId?: string) {
    if (runId) {
      const timer = snapshotTimers.get(runId)
      if (timer !== undefined) {
        window.clearInterval(timer)
        snapshotTimers.delete(runId)
      }
      lastSnapshotMetaByRunId.delete(runId)
      return
    }
    for (const timer of snapshotTimers.values()) {
      window.clearInterval(timer)
    }
    snapshotTimers.clear()
    lastSnapshotMetaByRunId.clear()
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
      showFeedback('没有可复制的响应体', 'danger')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('响应体已复制', 'success')
    } catch {
      try {
        await navigator.clipboard?.writeText(text)
        showFeedback('响应体已复制', 'success')
      } catch (error) {
        showFeedback(formatError(error, '复制响应体失败'), 'danger')
      }
    }
  }

  function touchDraft() {
    if (draftCollection.value) {
      draftCollection.value.updatedAt = now()
    }
    isDirty.value = true
  }

  function showFeedback(message: string, tone: 'success' | 'danger' = 'success') {
    if (!message) return
    if (feedbackTimer !== undefined) {
      window.clearTimeout(feedbackTimer)
      feedbackTimer = undefined
    }
    feedback.value = message
    feedbackTone.value = tone
    const duration = tone === 'danger' ? 5000 : 2000
    feedbackTimer = window.setTimeout(() => {
      feedbackTimer = undefined
      if (feedback.value === message) feedback.value = ''
    }, duration)
  }

  function showSaveFeedback(message: string, tone: 'success' | 'danger' = 'success') {
    if (!message) return
    if (saveFeedbackTimer !== undefined) {
      window.clearTimeout(saveFeedbackTimer)
      saveFeedbackTimer = undefined
    }
    saveFeedback.value = message
    saveFeedbackTone.value = tone
    const duration = tone === 'danger' ? 5000 : 2200
    saveFeedbackTimer = window.setTimeout(() => {
      saveFeedbackTimer = undefined
      if (saveFeedback.value === message) saveFeedback.value = ''
    }, duration)
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
    feedbackTone,
    saveFeedback,
    saveFeedbackTone,
    loadError,
    isLoading,
    isSaving,
    isRunning,
    isStopping,
    runningRequestIds,
    stoppingRequestIds,
    runIdByRequestId,
    runStartedAtByRequestId,
    currentRunId,
    currentRunStartedAt,
    requestTimeoutSeconds,
    isRequestRunning,
    isRequestStopping,
    isImporting,
    isGitSyncing,
    isDirty,
    isEnvironmentPanelOpen,
    envDeleteArmedId,
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
    confirmRemoveSelectedEnvironment,
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

function formatError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message.trim()
  if (typeof error === 'string' && error.trim()) return error.trim()
  return fallback
}

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
