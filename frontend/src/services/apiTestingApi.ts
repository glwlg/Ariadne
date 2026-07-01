import type {
  APIAssertion,
  APIAssertionResult,
  APICollection,
  APIEnvironment,
  APIHeader,
  APIGitStatus,
  APIImportResult,
  APIParam,
  APIRequest,
  APIRunRequest,
  APIRunResult,
  APIRunSnapshot,
  APIRunStopResult,
  APITestingStatus,
  APIVariable,
} from '../types/ariadne'

async function tryAPITestingBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/apitesting/service.js')
  } catch {
    return null
  }
}

const fallbackCollectionId = 'col-fallback'
const fallbackEnvironmentId = 'env-fallback'
const fallbackRequestId = 'req-fallback'
const fallbackRunControllers = new Map<string, AbortController>()
const fallbackRunSnapshots = new Map<string, APIRunSnapshot>()

let fallbackStatus: APITestingStatus = normalizeStatus({
  path: 'browser-memory',
  databasePath: '',
  activeCollectionId: fallbackCollectionId,
  collectionCount: 1,
  requestCount: 1,
  collections: [
    {
      id: fallbackCollectionId,
      name: '默认集合',
      variables: [{ id: id('var'), name: 'baseUrl', value: 'https://httpbin.org', enabled: true }],
      environments: [
        {
          id: fallbackEnvironmentId,
          name: '默认环境',
          variables: [
            { id: id('var'), name: 'traceId', value: 'ariadne-{{$timestamp}}', enabled: false },
            { id: id('var'), name: 'apiKey', value: '', enabled: false, secret: true },
          ],
        },
      ],
      requests: [
        {
          id: fallbackRequestId,
          name: 'GET 示例',
          folder: '示例',
          method: 'GET',
          url: '{{baseUrl}}/get',
          bodyType: 'none',
          body: '',
          params: [{ id: id('param'), name: 'show_env', value: '1', type: 'query', enabled: true }],
          headers: [{ id: id('hdr'), name: 'Accept', value: 'application/json', enabled: true }],
          assertions: [
            { id: id('ast'), kind: 'status', target: '', operator: 'equals', expected: '200', enabled: true },
            { id: id('ast'), kind: 'json', target: 'url', operator: 'contains', expected: '/get', enabled: true },
          ],
          updatedAt: Math.floor(Date.now() / 1000),
        },
      ],
      activeEnvironmentId: fallbackEnvironmentId,
      activeRequestId: fallbackRequestId,
      updatedAt: Math.floor(Date.now() / 1000),
    },
  ],
})

export async function getAPITestingStatus(): Promise<APITestingStatus> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      const status = normalizeStatus(await binding.Status())
      if (status.collections.length) {
        return status
      }
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return clone(fallbackStatus)
}

export async function upsertAPICollection(collection: APICollection): Promise<APITestingStatus> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.UpsertCollection(collection))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  const next = normalizeCollection(collection)
  const index = fallbackStatus.collections.findIndex((item) => item.id === next.id)
  if (index >= 0) {
    fallbackStatus.collections[index] = next
  } else {
    fallbackStatus.collections.push(next)
  }
  fallbackStatus.activeCollectionId ||= next.id
  refreshFallbackCounts()
  return clone(fallbackStatus)
}

export async function createAPICollection(): Promise<APITestingStatus> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.NewCollection())
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  const collection = normalizeCollection({
    id: id('col'),
    name: '新 API 集合',
    variables: [{ id: id('var'), name: 'baseUrl', value: 'https://example.com', enabled: true }],
    environments: [{ id: id('env'), name: '默认环境', variables: [] }],
    requests: [newAPIRequest()],
    activeEnvironmentId: '',
    activeRequestId: '',
    updatedAt: Math.floor(Date.now() / 1000),
  })
  fallbackStatus.collections.push(collection)
  fallbackStatus.activeCollectionId = collection.id
  refreshFallbackCounts()
  return clone(fallbackStatus)
}

export async function deleteAPICollection(collectionId: string): Promise<APITestingStatus> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.RemoveCollection(collectionId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  fallbackStatus.collections = fallbackStatus.collections.filter((collection) => collection.id !== collectionId)
  if (!fallbackStatus.collections.length) {
    fallbackStatus.collections = [fallbackDefaultCollection()]
  }
  if (!fallbackStatus.collections.some((collection) => collection.id === fallbackStatus.activeCollectionId)) {
    fallbackStatus.activeCollectionId = fallbackStatus.collections[0]?.id ?? ''
  }
  refreshFallbackCounts()
  return clone(fallbackStatus)
}

export async function setActiveAPICollection(collectionId: string): Promise<APITestingStatus> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.SetActiveCollection(collectionId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  if (fallbackStatus.collections.some((collection) => collection.id === collectionId)) {
    fallbackStatus.activeCollectionId = collectionId
  }
  return clone(fallbackStatus)
}

export async function runAPIRequest(request: APIRunRequest): Promise<APIRunResult> {
  const binding = await tryAPITestingBinding()
  if (binding) {
    try {
      return normalizeRunResult(await binding.Run(request))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return fallbackRun(request)
}

export async function stopAPIRequest(runId: string): Promise<APIRunStopResult> {
  const binding = await tryAPITestingBinding()
  if (binding?.StopRun) {
    try {
      return normalizeRunStopResult(await binding.StopRun(runId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  const controller = fallbackRunControllers.get(runId)
  if (!controller) {
    return { ok: false, message: '请求已结束' }
  }
  controller.abort()
  fallbackRunControllers.delete(runId)
  return { ok: true, message: '正在停止请求' }
}

export async function getAPIRunSnapshot(runId: string): Promise<APIRunSnapshot> {
  const binding = await tryAPITestingBinding()
  if (binding?.RunSnapshot) {
    try {
      return normalizeRunSnapshot(await binding.RunSnapshot(runId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return fallbackRunSnapshots.get(runId) ?? normalizeRunSnapshot({ ok: false, running: false, message: '请求已结束' })
}

export async function importAPIRequests(path: string, collectionId: string): Promise<APIImportResult> {
  const binding = await tryAPITestingBinding()
  if (binding?.ImportRequests) {
    try {
      return normalizeImportResult(await binding.ImportRequests(path, collectionId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return {
    ok: false,
    message: '当前环境不支持文件导入',
    importedCount: 0,
    status: clone(fallbackStatus),
  }
}

export async function configureAPIGit(collectionId: string, path: string, remote: string): Promise<APIGitStatus> {
  const binding = await tryAPITestingBinding()
  if (binding?.ConfigureGit) {
    try {
      return normalizeGitStatus(await binding.ConfigureGit(collectionId, path, remote))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return { ok: false, message: '当前环境不支持 Git 同步', dirty: false }
}

export async function getAPIGitStatus(collectionId: string): Promise<APIGitStatus> {
  const binding = await tryAPITestingBinding()
  if (binding?.GitStatus) {
    try {
      return normalizeGitStatus(await binding.GitStatus(collectionId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return { ok: false, message: '当前环境不支持 Git 同步', dirty: false }
}

export async function pullAPIGit(collectionId: string): Promise<APIGitStatus> {
  const binding = await tryAPITestingBinding()
  if (binding?.GitPull) {
    try {
      return normalizeGitStatus(await binding.GitPull(collectionId))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return { ok: false, message: '当前环境不支持 Git 同步', dirty: false }
}

export async function commitPushAPIGit(collectionId: string, message: string): Promise<APIGitStatus> {
  const binding = await tryAPITestingBinding()
  if (binding?.GitCommitPush) {
    try {
      return normalizeGitStatus(await binding.GitCommitPush(collectionId, message))
    } catch {
      // Desktop bindings are unavailable in browser-only dev mode.
    }
  }
  return { ok: false, message: '当前环境不支持 Git 同步', dirty: false }
}

export function newAPIRequest(): APIRequest {
  return {
    id: id('req'),
    name: '新请求',
    folder: '',
    method: 'GET',
    url: '{{baseUrl}}/resource',
    bodyType: 'none',
    body: '',
    params: [],
    headers: [{ id: id('hdr'), name: 'Accept', value: 'application/json', enabled: true }],
    assertions: [{ id: id('ast'), kind: 'status', target: '', operator: 'equals', expected: '200', enabled: true }],
    updatedAt: Math.floor(Date.now() / 1000),
  }
}

export function parseCurlRequest(source: string): Partial<Pick<APIRequest, 'name' | 'method' | 'url' | 'bodyType' | 'body' | 'params' | 'headers'>> {
  const tokens = tokenizeCurl(source)
  if (!tokens.length) throw new Error('请输入 cURL')
  let index = tokens[0]?.toLowerCase() === 'curl' ? 1 : 0
  let method = ''
  let rawUrl = ''
  let useDataAsQuery = false
  const headers: APIHeader[] = []
  const bodyParts: string[] = []
  const queryParts: string[] = []

  const nextValue = (token: string) => {
    const equalIndex = token.indexOf('=')
    if (equalIndex > 0) return token.slice(equalIndex + 1)
    index += 1
    return tokens[index] ?? ''
  }

  for (; index < tokens.length; index += 1) {
    const token = tokens[index]
    if (!token || token === '--') continue
    if (token === '-X' || token === '--request' || token.startsWith('--request=')) {
      method = nextValue(token).toUpperCase()
      continue
    }
    if (token.startsWith('-X') && token.length > 2) {
      method = token.slice(2).toUpperCase()
      continue
    }
    if (token === '--url' || token.startsWith('--url=')) {
      rawUrl = nextValue(token)
      continue
    }
    if (token === '-H' || token === '--header' || token.startsWith('--header=')) {
      const header = parseCurlHeader(nextValue(token))
      if (header) headers.push(header)
      continue
    }
    if (token.startsWith('-H') && token.length > 2) {
      const header = parseCurlHeader(token.slice(2))
      if (header) headers.push(header)
      continue
    }
    if (token === '-A' || token === '--user-agent' || token.startsWith('--user-agent=')) {
      const value = nextValue(token)
      if (value) headers.push({ id: id('hdr'), name: 'User-Agent', value, enabled: true })
      continue
    }
    if (token === '-e' || token === '--referer' || token.startsWith('--referer=')) {
      const value = nextValue(token)
      if (value) headers.push({ id: id('hdr'), name: 'Referer', value, enabled: true })
      continue
    }
    if (token === '-b' || token === '--cookie' || token.startsWith('--cookie=')) {
      const value = nextValue(token)
      if (value.includes('=')) headers.push({ id: id('hdr'), name: 'Cookie', value, enabled: true })
      continue
    }
    if (token === '-u' || token === '--user' || token.startsWith('--user=')) {
      const value = nextValue(token)
      if (value) headers.push({ id: id('hdr'), name: 'Authorization', value: `Basic ${btoa(value)}`, enabled: true })
      continue
    }
    if (curlOptionHasValue(token)) {
      nextValue(token)
      continue
    }
    if (token === '-G' || token === '--get') {
      useDataAsQuery = true
      continue
    }
    if (
      token === '-d' ||
      token === '--data' ||
      token === '--data-raw' ||
      token === '--data-binary' ||
      token === '--data-ascii' ||
      token === '--data-urlencode' ||
      token.startsWith('--data=') ||
      token.startsWith('--data-raw=') ||
      token.startsWith('--data-binary=') ||
      token.startsWith('--data-ascii=') ||
      token.startsWith('--data-urlencode=')
    ) {
      const value = nextValue(token)
      if (useDataAsQuery) {
        queryParts.push(value)
      } else {
        bodyParts.push(value)
      }
      continue
    }
    if (token.startsWith('-d') && token.length > 2) {
      const value = token.slice(2)
      if (useDataAsQuery) {
        queryParts.push(value)
      } else {
        bodyParts.push(value)
      }
      continue
    }
    if (token === '-F' || token === '--form' || token === '--form-string' || token.startsWith('--form=') || token.startsWith('--form-string=')) {
      bodyParts.push(nextValue(token))
      continue
    }
    if (token === '-I' || token === '--head') {
      method = 'HEAD'
      continue
    }
    if (!token.startsWith('-') && !rawUrl) {
      rawUrl = token
    }
  }

  rawUrl = rawUrl.trim()
  if (!rawUrl) throw new Error('未找到请求 URL')
  if (!method) method = bodyParts.length ? 'POST' : 'GET'

  const splitUrl = splitCurlUrl(rawUrl)
  const dataQueryParts = useDataAsQuery ? [...queryParts, ...bodyParts] : queryParts
  const params = [...splitUrl.params, ...dataQueryParts.flatMap(parseCurlParams)].filter((param) => param.name)
  const body = useDataAsQuery ? '' : bodyParts.join('&')
  const bodyType = body ? inferCurlBodyType(headers, body) : 'none'
  return {
    name: requestNameFromUrl(method, splitUrl.url),
    method,
    url: splitUrl.url,
    headers,
    params,
    bodyType,
    body,
  }
}

export function newAPIHeader(): APIHeader {
  return { id: id('hdr'), name: '', value: '', enabled: true }
}

export function newAPIParam(): APIParam {
  return { id: id('param'), name: '', value: '', type: 'query', enabled: true }
}

export function newAPIVariable(): APIVariable {
  return { id: id('var'), name: '', value: '', enabled: true }
}

export function newAPIEnvironment(): APIEnvironment {
  return { id: id('env'), name: '新环境', variables: [], updatedAt: Math.floor(Date.now() / 1000) }
}

export function newAPIAssertion(): APIAssertion {
  return { id: id('ast'), kind: 'status', target: '', operator: 'equals', expected: '200', enabled: true }
}

function normalizeStatus(status: APITestingStatus): APITestingStatus {
  const collections = (status.collections ?? []).map(normalizeCollection)
  const activeCollectionId = status.activeCollectionId || collections[0]?.id || ''
  return {
    path: status.path || '',
    databasePath: status.databasePath || '',
    collections,
    activeCollectionId,
    collectionCount: Number(status.collectionCount ?? collections.length),
    requestCount: Number(status.requestCount ?? collections.reduce((sum, collection) => sum + collection.requests.length, 0)),
    lastSaveError: status.lastSaveError || '',
    lastLoadError: status.lastLoadError || '',
  }
}

function normalizeCollection(collection: APICollection): APICollection {
  const requests = (collection.requests ?? []).map(normalizeRequest)
  const environments = (collection.environments ?? []).map(normalizeEnvironment)
  const normalizedRequests = requests.length ? requests : [newAPIRequest()]
  const normalizedEnvironments = environments.length ? environments : [newAPIEnvironment()]
  const variables = (collection.variables ?? []).map(normalizeVariable)
  return {
    id: collection.id || id('col'),
    name: collection.name || 'API 集合',
    variables,
    environments: normalizedEnvironments,
    requests: normalizedRequests,
    git: normalizeGitConfig(collection.git),
    activeEnvironmentId: collection.activeEnvironmentId || normalizedEnvironments[0]?.id || '',
    activeRequestId: collection.activeRequestId || normalizedRequests[0]?.id || '',
    updatedAt: Number(collection.updatedAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeGitConfig(git: APICollection['git']): APICollection['git'] {
  if (!git) return undefined
  return {
    path: git.path || '',
    remote: git.remote || '',
    branch: git.branch || '',
  }
}

function fallbackDefaultCollection(): APICollection {
  return normalizeCollection({
    id: fallbackCollectionId,
    name: '默认集合',
    variables: [{ id: id('var'), name: 'baseUrl', value: 'https://httpbin.org', enabled: true }],
    environments: [{ id: fallbackEnvironmentId, name: '默认环境', variables: [] }],
    requests: [newAPIRequest()],
    activeEnvironmentId: fallbackEnvironmentId,
    activeRequestId: '',
    updatedAt: Math.floor(Date.now() / 1000),
  })
}

function normalizeRequest(request: APIRequest): APIRequest {
  return {
    id: request.id || id('req'),
    name: request.name || '新请求',
    folder: request.folder || '',
    method: String(request.method || 'GET').toUpperCase(),
    url: request.url || '',
    bodyType: request.bodyType || 'none',
    body: request.body || '',
    params: (request.params ?? []).map(normalizeParam),
    headers: (request.headers ?? []).map(normalizeHeader),
    assertions: (request.assertions ?? []).map(normalizeAssertion),
    updatedAt: Number(request.updatedAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeEnvironment(environment: APIEnvironment): APIEnvironment {
  return {
    id: environment.id || id('env'),
    name: environment.name || '环境',
    variables: (environment.variables ?? []).map(normalizeVariable),
    updatedAt: Number(environment.updatedAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeHeader(header: APIHeader): APIHeader {
  return {
    id: header.id || id('hdr'),
    name: header.name || '',
    value: header.value || '',
    enabled: Boolean(header.enabled),
  }
}

function normalizeParam(param: APIParam): APIParam {
  return {
    id: param.id || id('param'),
    name: param.name || '',
    value: param.value || '',
    type: param.type === 'path' ? 'path' : 'query',
    enabled: Boolean(param.enabled),
  }
}

function normalizeVariable(variable: APIVariable): APIVariable {
  return {
    id: variable.id || id('var'),
    name: variable.name || '',
    value: variable.value || '',
    enabled: Boolean(variable.enabled),
    secret: Boolean(variable.secret),
  }
}

function normalizeAssertion(assertion: APIAssertion): APIAssertion {
  return {
    id: assertion.id || id('ast'),
    kind: assertion.kind || 'status',
    target: assertion.target || '',
    operator: assertion.operator || 'equals',
    expected: assertion.expected || '',
    enabled: Boolean(assertion.enabled),
  }
}

function normalizeRunResult(result: Partial<APIRunResult>): APIRunResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    method: result.method || '',
    requestUrl: result.requestUrl || '',
    statusCode: Number(result.statusCode ?? 0),
    statusText: result.statusText || '',
    durationMs: Number(result.durationMs ?? 0),
    headers: (result.headers ?? []).map(normalizeHeader),
    body: result.body || '',
    bodySize: Number(result.bodySize ?? 0),
    bodyTruncated: Boolean(result.bodyTruncated),
    contentType: result.contentType || '',
    streaming: Boolean(result.streaming),
    assertionResults: (result.assertionResults ?? []).map(normalizeAssertionResult),
    passed: Number(result.passed ?? 0),
    failed: Number(result.failed ?? 0),
    error: result.error || '',
    missingVariables: result.missingVariables ?? [],
  }
}

function normalizeRunSnapshot(result: Partial<APIRunSnapshot> & { result?: Partial<APIRunResult> }): APIRunSnapshot {
  return {
    ok: Boolean(result.ok),
    running: Boolean(result.running),
    message: result.message || '',
    updatedAt: Number(result.updatedAt ?? 0),
    result: normalizeRunResult(result.result ?? {}),
  }
}

function normalizeGitStatus(result: Partial<APIGitStatus>): APIGitStatus {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    collectionId: result.collectionId || '',
    path: result.path || '',
    remote: result.remote || '',
    branch: result.branch || '',
    dirty: Boolean(result.dirty),
    files: result.files ?? [],
    error: result.error || '',
  }
}

function normalizeImportResult(result: APIImportResult): APIImportResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    importedCount: Number(result.importedCount ?? 0),
    error: result.error || '',
    status: normalizeStatus(result.status),
  }
}

function normalizeRunStopResult(result: APIRunStopResult): APIRunStopResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
  }
}

function normalizeAssertionResult(result: APIAssertionResult): APIAssertionResult {
  return {
    id: result.id || id('ast'),
    kind: result.kind || '',
    target: result.target || '',
    operator: result.operator || '',
    expected: result.expected || '',
    actual: result.actual || '',
    passed: Boolean(result.passed),
    message: result.message || '',
  }
}

async function fallbackRun(payload: APIRunRequest): Promise<APIRunResult> {
  const started = performance.now()
  const collection = fallbackStatus.collections.find((item) => item.id === payload.collectionId)
  const environment = collection?.environments.find((item) => item.id === payload.environmentId)
  const variables = variableMap(collection, environment)
  const request = normalizeRequest(payload.request)
  const requestUrl = resolveRequestUrl(request.url, request.params, variables)
  const body = applyVariables(request.body, variables)
  const controller = new AbortController()
  if (payload.runId) {
    fallbackRunControllers.set(payload.runId, controller)
  }
  try {
    const response = await fetch(requestUrl, {
      method: request.method,
      headers: Object.fromEntries(
        request.headers
          .filter((header) => header.enabled && header.name.trim())
          .map((header) => [applyVariables(header.name, variables), applyVariables(header.value, variables)]),
      ),
      body: request.method === 'GET' || request.method === 'HEAD' || !body ? undefined : body,
      signal: controller.signal,
    })
    const baseResult = normalizeRunResult({
      ok: true,
      message: '正在接收响应',
      method: request.method,
      requestUrl,
      statusCode: response.status,
      statusText: `${response.status} ${response.statusText}`,
      durationMs: Math.round(performance.now() - started),
      headers: Array.from(response.headers.entries()).map(([name, value]) => ({ id: id('hdr'), name, value, enabled: true })),
      body: '',
      bodySize: 0,
      bodyTruncated: false,
      contentType: response.headers.get('content-type') || '',
      streaming: (response.headers.get('content-type') || '').toLowerCase().includes('event-stream'),
      assertionResults: [],
      passed: 0,
      failed: 0,
    })
    updateFallbackRunSnapshot(payload.runId, baseResult)
    const bodyResult = await readFetchResponseBody(response, controller.signal, (text, truncated) => {
      updateFallbackRunSnapshot(payload.runId, {
        ...baseResult,
        durationMs: Math.round(performance.now() - started),
        body: text,
        bodySize: text.length,
        bodyTruncated: truncated,
        message: '正在接收响应',
      })
    })
    const result = normalizeRunResult({
      ...baseResult,
      message: bodyResult.streaming && bodyResult.stopped ? 'SSE 已停止' : '请求完成',
      durationMs: Math.round(performance.now() - started),
      body: bodyResult.text,
      bodySize: bodyResult.text.length,
      bodyTruncated: bodyResult.truncated,
    })
    result.assertionResults = evaluateAssertions(request.assertions, result)
    result.passed = result.assertionResults.filter((item) => item.passed).length
    result.failed = result.assertionResults.length - result.passed
    if (bodyResult.streaming) {
      result.message = bodyResult.stopped ? 'SSE 已停止' : 'SSE 已结束'
    } else {
      result.message = result.assertionResults.length ? `请求完成，断言 ${result.passed}/${result.assertionResults.length} 通过` : '请求完成'
    }
    return result
  } catch (error) {
    const stopped = controller.signal.aborted
    return normalizeRunResult({
      ok: false,
      message: stopped ? '请求已停止' : '请求失败',
      method: request.method,
      requestUrl,
      statusCode: 0,
      statusText: '',
      durationMs: Math.round(performance.now() - started),
      headers: [],
      body: '',
      bodySize: 0,
      bodyTruncated: false,
      contentType: '',
      assertionResults: [],
      passed: 0,
      failed: 0,
      error: stopped ? '请求已停止' : error instanceof Error ? error.message : String(error),
    })
  } finally {
    if (payload.runId && fallbackRunControllers.get(payload.runId) === controller) {
      fallbackRunControllers.delete(payload.runId)
    }
    if (payload.runId) {
      fallbackRunSnapshots.delete(payload.runId)
    }
  }
}

async function readFetchResponseBody(response: Response, signal: AbortSignal, onUpdate?: (text: string, truncated: boolean) => void) {
  const contentType = response.headers.get('content-type') || ''
  const streaming = contentType.toLowerCase().includes('event-stream')
  if (!streaming) {
    return { text: await response.text(), streaming: false, truncated: false, stopped: signal.aborted }
  }
  const reader = response.body?.getReader()
  if (!reader) {
    return { text: 'SSE 连接已建立，暂未收到事件', streaming: true, truncated: false, stopped: signal.aborted }
  }
  const decoder = new TextDecoder()
  let text = ''
  let truncated = false
  try {
    while (true) {
      const chunk = await reader.read()
      if (chunk.done) break
      const value = decoder.decode(chunk.value, { stream: true })
      if (text.length < 64 * 1024) {
        const remaining = 64 * 1024 - text.length
        text += value.slice(0, remaining)
        if (value.length > remaining) truncated = true
      } else {
        truncated = true
      }
      onUpdate?.(text || 'SSE 连接已建立，暂未收到事件', truncated)
    }
    const tail = decoder.decode()
    if (tail) {
      if (text.length < 64 * 1024) {
        const remaining = 64 * 1024 - text.length
        text += tail.slice(0, remaining)
        if (tail.length > remaining) truncated = true
      } else {
        truncated = true
      }
    }
    if (text.length > 64 * 1024) {
      text = text.slice(0, 64 * 1024)
      truncated = true
    }
  } catch (error) {
    if (!signal.aborted) throw error
  } finally {
    await reader.cancel().catch(() => undefined)
  }
  return { text: text || 'SSE 连接已建立，暂未收到事件', streaming: true, truncated, stopped: signal.aborted }
}

function updateFallbackRunSnapshot(runId: string | undefined, result: APIRunResult) {
  if (!runId) return
  fallbackRunSnapshots.set(
    runId,
    normalizeRunSnapshot({
      ok: true,
      running: true,
      message: result.message,
      updatedAt: Date.now(),
      result,
    }),
  )
}

function evaluateAssertions(assertions: APIAssertion[], result: APIRunResult): APIAssertionResult[] {
  return assertions
    .filter((assertion) => assertion.enabled)
    .map((assertion) => {
      const actual = actualForAssertion(assertion, result)
      const passed = compareAssertion(assertion, actual)
      return {
        id: assertion.id,
        kind: assertion.kind,
        target: assertion.target,
        operator: assertion.operator,
        expected: assertion.expected,
        actual: actual.value,
        passed,
        message: passed ? '通过' : '未通过',
      }
    })
}

function actualForAssertion(assertion: APIAssertion, result: APIRunResult) {
  if (assertion.kind === 'status') return { value: String(result.statusCode), exists: true }
  if (assertion.kind === 'response_time') return { value: String(result.durationMs), exists: true }
  if (assertion.kind === 'body') return { value: result.body, exists: result.body.length > 0 }
  if (assertion.kind === 'header') {
    const header = result.headers.find((item) => item.name.toLowerCase() === assertion.target.toLowerCase())
    return { value: header?.value || '', exists: Boolean(header) }
  }
  if (assertion.kind === 'json') {
    try {
      const parsed = JSON.parse(result.body)
      const value = jsonPathValue(parsed, assertion.target)
      return { value: value === undefined ? '' : typeof value === 'string' ? value : JSON.stringify(value), exists: value !== undefined }
    } catch {
      return { value: '', exists: false }
    }
  }
  return { value: '', exists: false }
}

function compareAssertion(assertion: APIAssertion, actual: { value: string; exists: boolean }) {
  if (assertion.operator === 'exists') return actual.exists
  if (assertion.operator === 'contains') return actual.value.includes(assertion.expected)
  if (assertion.operator === 'not_equals') return actual.value !== assertion.expected
  if (assertion.operator === 'less_than') return Number(actual.value) < Number(assertion.expected)
  if (assertion.operator === 'greater_than') return Number(actual.value) > Number(assertion.expected)
  return actual.value === assertion.expected
}

function jsonPathValue(value: unknown, path: string): unknown {
  const segments = path.replace(/^\$\.?/, '').split('.').filter(Boolean)
  let current: unknown = value
  for (const segment of segments) {
    const match = segment.match(/^([^\[]+)(?:\[(\d+)\])?$/)
    if (!match || current === null || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[match[1]]
    if (match[2] !== undefined) {
      if (!Array.isArray(current)) return undefined
      current = current[Number(match[2])]
    }
  }
  return current
}

function variableMap(collection?: APICollection, environment?: APIEnvironment) {
  const variables: Record<string, string> = {}
  for (const variable of collection?.variables ?? []) {
    if (variable.enabled && variable.name.trim()) variables[variable.name.trim()] = variable.value
  }
  for (const variable of environment?.variables ?? []) {
    if (variable.enabled && variable.name.trim()) variables[variable.name.trim()] = variable.value
  }
  return variables
}

function applyVariables(text: string, variables: Record<string, string>) {
  return text.replace(/\{\{\s*([A-Za-z0-9_$.-]+)\s*\}\}/g, (match, name) => {
    if (name === '$timestamp') return String(Math.floor(Date.now() / 1000))
    return variables[name] ?? match
  })
}

function resolveRequestUrl(rawUrl: string, params: APIParam[], variables: Record<string, string>) {
  let resolved = applyVariables(rawUrl, variables)
  for (const param of params) {
    if (!param.enabled || param.type !== 'path' || !param.name.trim()) continue
    const name = applyVariables(param.name, variables)
    const value = encodeURIComponent(applyVariables(param.value, variables))
    resolved = resolved.replaceAll(`:${name}`, value).replaceAll(`{${name}}`, value)
  }
  try {
    const url = new URL(resolved)
    for (const param of params) {
      if (!param.enabled || param.type !== 'query' || !param.name.trim()) continue
      url.searchParams.append(applyVariables(param.name, variables), applyVariables(param.value, variables))
    }
    return url.toString()
  } catch {
    return resolved
  }
}

function refreshFallbackCounts() {
  fallbackStatus.collectionCount = fallbackStatus.collections.length
  fallbackStatus.requestCount = fallbackStatus.collections.reduce((sum, collection) => sum + collection.requests.length, 0)
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function tokenizeCurl(source: string) {
  const normalized = source.trim().replace(/\\\r?\n/g, ' ').replace(/\^\r?\n/g, ' ').replace(/`\r?\n/g, ' ')
  const tokens: string[] = []
  let current = ''
  let quote = ''
  for (let index = 0; index < normalized.length; index += 1) {
    const char = normalized[index]
    if (quote) {
      if (char === quote) {
        quote = ''
      } else if (quote === '"' && char === '\\' && index + 1 < normalized.length) {
        index += 1
        current += normalized[index]
      } else {
        current += char
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (/\s/.test(char)) {
      if (current) {
        tokens.push(current)
        current = ''
      }
      continue
    }
    if (char === '\\' && index + 1 < normalized.length) {
      index += 1
      current += normalized[index]
      continue
    }
    current += char
  }
  if (current) tokens.push(current)
  return tokens
}

function parseCurlHeader(raw: string): APIHeader | null {
  const index = raw.indexOf(':')
  if (index <= 0) return null
  return {
    id: id('hdr'),
    name: raw.slice(0, index).trim(),
    value: raw.slice(index + 1).trim(),
    enabled: true,
  }
}

function curlOptionHasValue(token: string) {
  return [
    '-o',
    '--output',
    '-x',
    '--proxy',
    '--connect-timeout',
    '--max-time',
    '--retry',
    '--cacert',
    '--cert',
    '--key',
    '--request-target',
    '--resolve',
  ].some((option) => token === option || token.startsWith(`${option}=`))
}

function splitCurlUrl(rawUrl: string) {
  const hashIndex = rawUrl.indexOf('#')
  const hash = hashIndex >= 0 ? rawUrl.slice(hashIndex) : ''
  const withoutHash = hashIndex >= 0 ? rawUrl.slice(0, hashIndex) : rawUrl
  const queryIndex = withoutHash.indexOf('?')
  if (queryIndex < 0) return { url: rawUrl, params: [] as APIParam[] }
  return {
    url: `${withoutHash.slice(0, queryIndex)}${hash}`,
    params: parseCurlParams(withoutHash.slice(queryIndex + 1)),
  }
}

function parseCurlParams(raw: string): APIParam[] {
  return raw
    .split('&')
    .filter(Boolean)
    .map((part) => {
      const equalIndex = part.indexOf('=')
      const name = equalIndex >= 0 ? part.slice(0, equalIndex) : part
      const value = equalIndex >= 0 ? part.slice(equalIndex + 1) : ''
      return {
        id: id('param'),
        name: safeDecodeCurlValue(name),
        value: safeDecodeCurlValue(value),
        type: 'query',
        enabled: true,
      }
    })
}

function safeDecodeCurlValue(value: string) {
  try {
    return decodeURIComponent(value.replace(/\+/g, ' '))
  } catch {
    return value
  }
}

function inferCurlBodyType(headers: APIHeader[], body: string) {
  const contentType = headers.find((header) => header.name.toLowerCase() === 'content-type')?.value.toLowerCase() ?? ''
  if (contentType.includes('application/json')) return 'json'
  if (contentType.includes('application/x-www-form-urlencoded') || /^[^=&\s]+=[\s\S]*(&[^=&\s]+=[\s\S]*)*$/.test(body)) return 'form'
  if (/^\s*[\[{]/.test(body)) return 'json'
  return 'text'
}

function requestNameFromUrl(method: string, rawUrl: string) {
  const withoutQuery = rawUrl.split('?')[0] || rawUrl
  const path = withoutQuery.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]+/i, '').replace(/^\{\{[^}]+\}\}/, '')
  const segments = path.split('/').filter(Boolean)
  const label = segments.length ? segments.at(-1) : withoutQuery.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '').split('/')[0]
  return `${method} ${label || '请求'}`
}

function id(prefix: string) {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}
