import type {
  AgentTaskPackage,
  ChecklistDraft,
  ExperienceDecisionResult,
  ExperienceDiscoveryRequest,
  ExperienceDiscoveryResult,
  ExperienceReport,
  SearchResult,
  ScheduledDraftStatus,
  WorkflowDraft,
  WorkMemoryDraftPolishRequest,
  WorkMemoryDraftPolishResult,
  WorkMemoryEmbeddingRefreshResult,
  WorkMemoryExportResult,
  WorkMemoryExportRequest,
  WorkMemoryDraft,
  WorkMemoryEntry,
  WorkMemoryImportMaterialRequest,
  WorkMemoryImportMaterialResult,
  WorkMemoryNoteRequest,
  WorkMemorySemanticSearchResult,
  WorkMemorySemanticStatus,
  WorkMemoryStatus,
} from '../types/ariadne'

const fallbackEntry: WorkMemoryEntry = {
  id: 'memory-gateway',
  source: 'clipboard',
  contentType: 'issue_note',
  title: '网关代理异常排查记录',
  summary: 'WiFi 代理失败优先确认默认网关是否指向 OpenWrt 192.168.1.10。',
  text: 'Cloudflare Tunnel 入口正常，OpenWrt 网关疑似仍指向 192.168.1.1。',
  windowTitle: 'Windows Terminal',
  appName: 'Terminal',
  tags: ['网络', '证据'],
  favorite: true,
  sensitive: false,
  createdAt: 1770000000,
}

let fallbackStatus: WorkMemoryStatus = {
  enabled: true,
  timeMachineEnabled: false,
  privacyMode: false,
  autoOcrEnabled: false,
  captureScope: 'all_screens',
  multiMonitor: 'combined',
  pauseOnIdle: true,
  idlePauseSeconds: 600,
  pauseOnLock: true,
  sessionLocked: false,
  entryCount: 1,
  autoCaptureIntervalSeconds: 300,
  windowSwitchCaptureEnabled: false,
  windowSwitchCooldownSeconds: 30,
  captureCount: 0,
}

let fallbackTimeline: WorkMemoryEntry[] = [fallbackEntry]

let fallbackScheduledDraftStatus: ScheduledDraftStatus = {
  enabled: false,
  running: false,
  intervalMinutes: 240,
  dailyDraftEnabled: true,
  retrospectiveEnabled: true,
  experienceReportEnabled: true,
  lastEntryCount: 0,
}

let fallbackSemanticStatus: WorkMemorySemanticStatus = {
  enabled: true,
  provider: 'sqlite_fts5+local_term_vector',
  mode: 'local',
  external: false,
  ftsEnabled: true,
  indexedEntries: fallbackTimeline.length,
  externalEmbeddingReady: false,
  embeddingIndexed: 0,
  vectorStoreType: 'embedded',
  vectorCollection: 'ariadne_work_memory',
  note: '开发态 fallback 使用本地语义检索；桌面运行时可刷新外部 embedding 缓存。',
}

async function tryWorkMemoryBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/workmemory/service.js')
  } catch {
    return null
  }
}

export async function getWorkMemoryStatus(): Promise<WorkMemoryStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.Status()
    } catch {
      return fallbackStatus
    }
  }
  return fallbackStatus
}

export async function getWorkMemoryTimeline(): Promise<WorkMemoryEntry[]> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.Timeline()
    } catch {
      return fallbackTimeline
    }
  }
  return fallbackTimeline
}

export async function getScheduledDraftStatus(): Promise<ScheduledDraftStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.ScheduledDraftStatus()
    } catch {
      return fallbackScheduledDraftStatus
    }
  }
  return fallbackScheduledDraftStatus
}

export async function getSemanticStatus(): Promise<WorkMemorySemanticStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeSemanticStatus(await binding.SemanticStatus())
    } catch {
      return fallbackSemanticStatus
    }
  }
  return fallbackSemanticStatus
}

export async function refreshEmbeddingIndex(): Promise<WorkMemoryEmbeddingRefreshResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeEmbeddingRefreshResult(await binding.RefreshEmbeddingIndex())
    } catch {
      return fallbackRefreshEmbeddingIndex()
    }
  }
  return fallbackRefreshEmbeddingIndex()
}

export async function semanticSearchExternal(query: string): Promise<WorkMemorySemanticSearchResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeSemanticSearchResult(await binding.SemanticSearchExternal(query))
    } catch {
      return fallbackSemanticSearch(query)
    }
  }
  return fallbackSemanticSearch(query)
}

export async function runScheduledDraftsNow(): Promise<ScheduledDraftStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.RunScheduledDraftsNow()
    } catch {
      return fallbackRunScheduledDrafts()
    }
  }
  return fallbackRunScheduledDrafts()
}

export async function searchWorkMemory(query: string): Promise<SearchResult[]> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.Search(query)
    } catch {
      return fallbackSearch(query)
    }
  }
  return fallbackSearch(query)
}

function fallbackSearch(query: string): SearchResult[] {
  const normalized = query.trim().toLowerCase()
  if (!normalized) {
    return []
  }
  return fallbackTimeline
    .filter((entry) => {
      const haystack = [
        entry.title,
        entry.summary,
        entry.text,
        entry.ocrText,
        entry.ocrStatus,
        entry.windowTitle,
        entry.appName,
        entry.source,
        entry.contentType,
        ...entry.tags,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return haystack.includes(normalized)
    })
    .map((entry) => ({
      id: entry.id,
      type: 'memory',
      title: entry.title,
      subtitle: `工作记忆 · ${entry.appName ?? entry.source}`,
      detail: entry.summary,
      icon: 'memory',
      tags: entry.tags,
      preview: {
        kind: 'memory',
        title: entry.title,
        subtitle: entry.windowTitle,
        text: entry.text,
        evidence: [{ label: '记忆 ID', value: entry.id }],
      },
      actions: [
        {
          id: 'copy_summary',
          label: '复制摘要',
          icon: 'copy',
          kind: 'copy',
          payload: { text: entry.summary },
          feedback: { successLabel: '已复制' },
        },
      ],
    }))
}

function fallbackRefreshEmbeddingIndex(): WorkMemoryEmbeddingRefreshResult {
  const indexed = fallbackTimeline.filter((entry) => !entry.sensitive).length
  fallbackSemanticStatus = {
    ...fallbackSemanticStatus,
    external: true,
    mode: 'hybrid',
    provider: 'sqlite_fts5+local_term_vector+external_embedding',
    externalEmbeddingReady: true,
    externalProvider: 'openai-compatible',
    embeddingModel: 'configured-embedding-model',
    embeddingIndexed: indexed,
    lastEmbeddingAt: Math.floor(Date.now() / 1000),
    lastEmbeddingError: '',
    note: '开发态 fallback 已模拟刷新外部 embedding 到内置向量缓存。',
  }
  return {
    ok: true,
    message: `开发态 fallback 已刷新 embedding · ${indexed} 条`,
    status: fallbackSemanticStatus,
    indexed,
    skipped: fallbackTimeline.length - indexed,
    failed: 0,
    provider: fallbackSemanticStatus.externalProvider,
    model: fallbackSemanticStatus.embeddingModel,
    refreshedAt: fallbackSemanticStatus.lastEmbeddingAt,
  }
}

function fallbackSemanticSearch(query: string): WorkMemorySemanticSearchResult {
  const results = fallbackSearch(query)
  return {
    ok: true,
    message: `开发态 fallback 语义命中 ${results.length} 条`,
    query,
    results,
    status: fallbackSemanticStatus,
    provider: fallbackSemanticStatus.externalProvider,
    model: fallbackSemanticStatus.embeddingModel,
  }
}

function normalizeSemanticStatus(status: WorkMemorySemanticStatus): WorkMemorySemanticStatus {
  return {
    enabled: Boolean(status.enabled),
    provider: status.provider || 'local_term_vector',
    mode: status.mode || 'local',
    external: Boolean(status.external),
    ftsEnabled: Boolean(status.ftsEnabled),
    ftsPath: status.ftsPath || '',
    indexedEntries: Number(status.indexedEntries ?? 0),
    lastIndexedAt: Number(status.lastIndexedAt ?? 0),
    lastIndexError: status.lastIndexError || '',
    externalEmbeddingReady: Boolean(status.externalEmbeddingReady),
    externalProvider: status.externalProvider || '',
    embeddingModel: status.embeddingModel || '',
    embeddingIndexed: Number(status.embeddingIndexed ?? 0),
    lastEmbeddingAt: Number(status.lastEmbeddingAt ?? 0),
    lastEmbeddingError: status.lastEmbeddingError || '',
    vectorStoreType: status.vectorStoreType || '',
    vectorStoreUri: status.vectorStoreUri || '',
    vectorCollection: status.vectorCollection || '',
    note: status.note || '',
  }
}

function normalizeEmbeddingRefreshResult(result: WorkMemoryEmbeddingRefreshResult): WorkMemoryEmbeddingRefreshResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    status: normalizeSemanticStatus(result.status),
    indexed: Number(result.indexed ?? 0),
    skipped: Number(result.skipped ?? 0),
    failed: Number(result.failed ?? 0),
    provider: result.provider || '',
    model: result.model || '',
    refreshedAt: Number(result.refreshedAt ?? 0),
    requiresReview: Boolean(result.requiresReview),
  }
}

function normalizeSemanticSearchResult(result: WorkMemorySemanticSearchResult): WorkMemorySemanticSearchResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    query: result.query || '',
    results: Array.isArray(result.results) ? result.results : [],
    status: normalizeSemanticStatus(result.status),
    provider: result.provider || '',
    model: result.model || '',
  }
}

export async function setWorkMemoryPrivacyMode(enabled: boolean): Promise<WorkMemoryStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.SetPrivacyMode(enabled)
    } catch {
      return fallbackSetPrivacyMode(enabled)
    }
  }
  return fallbackSetPrivacyMode(enabled)
}

function fallbackSetPrivacyMode(enabled: boolean): WorkMemoryStatus {
  fallbackStatus = {
    ...fallbackStatus,
    privacyMode: enabled,
    timeMachineEnabled: enabled ? false : fallbackStatus.timeMachineEnabled,
    pauseReason: enabled ? '隐私模式已开启' : undefined,
  }
  return fallbackStatus
}

export async function setTimeMachineEnabled(enabled: boolean): Promise<WorkMemoryStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.SetTimeMachineEnabled(enabled)
    } catch {
      return fallbackSetTimeMachine(enabled)
    }
  }
  return fallbackSetTimeMachine(enabled)
}

function fallbackSetTimeMachine(enabled: boolean): WorkMemoryStatus {
  if (fallbackStatus.privacyMode && enabled) {
    fallbackStatus = { ...fallbackStatus, timeMachineEnabled: false, pauseReason: '隐私模式已开启' }
  } else {
    fallbackStatus = { ...fallbackStatus, timeMachineEnabled: enabled, pauseReason: undefined }
  }
  return fallbackStatus
}

export async function captureCurrentScreen(): Promise<WorkMemoryEntry> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.CaptureCurrentScreen()
    } catch {
      return fallbackCaptureCurrentScreen()
    }
  }
  return fallbackCaptureCurrentScreen()
}

export async function captureTimeMachineNow(): Promise<WorkMemoryEntry> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.CaptureTimeMachineNow()
    } catch {
      return fallbackCaptureCurrentScreen('time_machine')
    }
  }
  return fallbackCaptureCurrentScreen('time_machine')
}

export async function addWorkMemoryNote(request: WorkMemoryNoteRequest): Promise<WorkMemoryEntry> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.AddNote(request)
    } catch {
      return fallbackAddNote(request)
    }
  }
  return fallbackAddNote(request)
}

function fallbackAddNote(request: WorkMemoryNoteRequest): WorkMemoryEntry {
  const text = request.text.trim()
  if (!text || fallbackStatus.privacyMode) {
    return {
      id: '',
      source: '',
      contentType: '',
      title: '',
      summary: '',
      text: '',
      tags: [],
      favorite: false,
      sensitive: false,
      createdAt: 0,
    }
  }
  const lower = `${request.title ?? ''} ${text}`.toLowerCase()
  const sensitive = Boolean(request.sensitive) || ['password', 'token', 'secret', 'authorization:', '密码', '密钥'].some((item) => lower.includes(item))
  const entry: WorkMemoryEntry = {
    id: `memory-note-${Date.now()}`,
    source: 'manual_note',
    contentType: 'note',
    title: request.title?.trim() || text.split('\n').find(Boolean)?.slice(0, 32) || '手动笔记',
    summary: text.replace(/\s+/g, ' ').slice(0, 96),
    text,
    windowTitle: 'Ariadne',
    appName: 'Ariadne',
    tags: ['手动笔记', ...(request.tags ?? []), ...(sensitive ? ['敏感'] : [])],
    favorite: Boolean(request.favorite),
    sensitive,
    createdAt: Math.floor(Date.now() / 1000),
  }
  fallbackTimeline = [entry, ...fallbackTimeline]
  fallbackStatus = { ...fallbackStatus, entryCount: fallbackTimeline.length }
  return entry
}

function fallbackCaptureCurrentScreen(source = 'manual_capture'): WorkMemoryEntry {
  if (fallbackStatus.privacyMode) {
    return {
      id: '',
      source: '',
      contentType: '',
      title: '',
      summary: '',
      text: '',
      tags: [],
      favorite: false,
      sensitive: false,
      createdAt: 0,
    }
  }
  const title = source === 'time_machine' ? '屏幕时间机器自动记录' : '手动补记当前屏幕'
  const summary = source === 'time_machine' ? '后台时间机器按策略记录当前屏幕。' : '用户主动把当前屏幕纳入工作记忆。'
  const entry: WorkMemoryEntry = {
    id: `${source}-${Date.now()}`,
    source,
    contentType: 'screenshot',
    title,
    summary,
    text: '开发态 fallback 已记录一条屏幕补记。',
    windowTitle: 'Ariadne',
    appName: 'Ariadne',
    tags: [source === 'time_machine' ? '屏幕时间机器' : '补记', '截图'],
    favorite: false,
    sensitive: false,
    createdAt: Math.floor(Date.now() / 1000),
  }
  fallbackTimeline = [entry, ...fallbackTimeline]
  fallbackStatus = {
    ...fallbackStatus,
    entryCount: fallbackTimeline.length,
    lastCaptureAt: entry.createdAt,
    lastCaptureId: entry.id,
    captureCount: (fallbackStatus.captureCount ?? 0) + 1,
  }
  return entry
}

export async function deleteWorkMemoryEntry(id: string): Promise<WorkMemoryStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.Delete(id)
    } catch {
      return fallbackDeleteEntry(id)
    }
  }
  return fallbackDeleteEntry(id)
}

function fallbackDeleteEntry(id: string): WorkMemoryStatus {
  fallbackTimeline = fallbackTimeline.filter((entry) => entry.id !== id)
  fallbackStatus = { ...fallbackStatus, entryCount: fallbackTimeline.length }
  return fallbackStatus
}

export async function clearUnpinnedWorkMemory(): Promise<WorkMemoryStatus> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.ClearUnpinned()
    } catch {
      return fallbackClearUnpinned()
    }
  }
  return fallbackClearUnpinned()
}

function fallbackClearUnpinned(): WorkMemoryStatus {
  fallbackTimeline = fallbackTimeline.filter((entry) => entry.favorite)
  fallbackStatus = { ...fallbackStatus, entryCount: fallbackTimeline.length }
  return fallbackStatus
}

export async function exportWorkMemoryData(includeSensitive: boolean): Promise<WorkMemoryExportResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.ExportData(includeSensitive)
    } catch {
      return fallbackExportData(includeSensitive)
    }
  }
  return fallbackExportData(includeSensitive)
}

export async function exportWorkMemoryDataWithOptions(request: WorkMemoryExportRequest): Promise<WorkMemoryExportResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.ExportDataWithOptions(request)
    } catch {
      return fallbackExportData(Boolean(request.includeSensitive), request)
    }
  }
  return fallbackExportData(Boolean(request.includeSensitive), request)
}

function fallbackExportData(includeSensitive: boolean, request: WorkMemoryExportRequest = {}): WorkMemoryExportResult {
  const startAt = request.startAt ?? 0
  const endAt = request.endAt ?? 0
  const tags = request.tags?.filter(Boolean) ?? []
  const entryIds = request.entryIds?.filter(Boolean) ?? []
  const filteredTimeline = fallbackTimeline.filter((entry) => {
    if (startAt && entry.createdAt < startAt) return false
    if (endAt && entry.createdAt > endAt) return false
    if (entryIds.length && !entryIds.includes(entry.id)) return false
    if (tags.length && !entry.tags.some((tag) => tags.includes(tag))) return false
    return true
  })
  const filteredOut = fallbackTimeline.length - filteredTimeline.length
  const filteredSensitive = includeSensitive ? 0 : filteredTimeline.filter((entry) => entry.sensitive).length
  return {
    ok: true,
    message: '开发态 fallback 已生成导出摘要',
    path: '',
    entryCount: includeSensitive ? filteredTimeline.length : filteredTimeline.length - filteredSensitive,
    skippedSensitiveCount: filteredSensitive,
    skippedExcludedCount: 0,
    filteredOutCount: filteredOut,
    includesSensitive: includeSensitive,
    filter: { startAt, endAt, tags, entryIds },
    bytes: 0,
    createdAt: Math.floor(Date.now() / 1000),
  }
}

export async function importWorkMemoryMaterials(request: WorkMemoryImportMaterialRequest): Promise<WorkMemoryImportMaterialResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.ImportMaterials(request)
    } catch {
      return fallbackImportMaterials(request)
    }
  }
  return fallbackImportMaterials(request)
}

function fallbackImportMaterials(request: WorkMemoryImportMaterialRequest): WorkMemoryImportMaterialResult {
  const now = Math.floor(Date.now() / 1000)
  const entries: WorkMemoryEntry[] = request.paths.map((path, index) => {
    const title = path.split(/[\\/]/).filter(Boolean).pop() || `导入材料 ${index + 1}`
    return {
      id: `memory-import-fallback-${now}-${index}`,
      source: 'import',
      contentType: title.toLowerCase().endsWith('.png') ? 'image' : 'text',
      title,
      summary: `开发态 fallback 导入 ${title}`,
      text: `导入材料: ${path}`,
      tags: ['导入', ...(request.tags ?? [])],
      favorite: Boolean(request.favorite),
      sensitive: Boolean(request.sensitive),
      createdAt: now,
    }
  })
  fallbackTimeline = [...entries, ...fallbackTimeline]
  fallbackStatus = { ...fallbackStatus, entryCount: fallbackTimeline.length, lastCaptureAt: now, captureCount: (fallbackStatus.captureCount ?? 0) + entries.length }
  return {
    ok: entries.length > 0,
    message: entries.length > 0 ? `开发态 fallback 已导入 ${entries.length} 条材料` : '没有可导入的路径',
    imported: entries.length,
    skipped: 0,
    failed: 0,
    entries,
    items: request.paths.map((path, index) => ({
      path,
      ok: true,
      message: '开发态 fallback 已导入',
      entryId: entries[index]?.id,
      source: 'import',
      contentType: entries[index]?.contentType,
    })),
    createdAt: now,
  }
}

export async function generateDailyDraft(): Promise<WorkMemoryDraft> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateDailyDraft()
    } catch {
      return fallbackDailyDraft()
    }
  }
  return fallbackDailyDraft()
}

export async function polishWorkMemoryDraft(request: WorkMemoryDraftPolishRequest): Promise<WorkMemoryDraftPolishResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeDraftPolishResult(await binding.PolishDraft(request))
    } catch {
      return fallbackPolishDraft(request)
    }
  }
  return fallbackPolishDraft(request)
}

function fallbackDailyDraft(): WorkMemoryDraft {
  const evidence = fallbackTimeline.filter((entry) => !entry.sensitive).slice(0, 6)
  const lines = [
    '## 今日概览',
    `- 开发态 fallback 基于 ${evidence.length} 条非敏感工作记忆生成。`,
    '- 正式环境由 Go 本地规则按时间、来源、待跟进和复盘线索整理。',
    '',
    '## 主要工作',
    ...(evidence.length ? evidence.map((entry) => `- ${entry.title}：${entry.summary}`) : ['- 暂无可用工作记忆。']),
    '',
    '## 隐私与边界',
    '- 外发 AI、知识库同步或代理执行前仍需要用户确认。',
  ]
  return {
    id: `daily-${new Date().toISOString().slice(0, 10)}`,
    title: '今日工作日报草稿',
    body: lines.join('\n'),
    evidence: evidence.map((entry) => entry.id),
    createdAt: Math.floor(Date.now() / 1000),
  }
}

function fallbackPolishDraft(request: WorkMemoryDraftPolishRequest): WorkMemoryDraftPolishResult {
  const draft = normalizeDraft(request.draft)
  const risks = [
    'AI 润色会外发当前草稿正文，必须由用户二次确认',
    `草稿包含 ${draft.evidence.length} 条 evidence 引用`,
  ]
  if (!request.confirmed) {
    return {
      ok: false,
      message: 'AI 润色需要确认外发',
      draft,
      requiresConfirmation: true,
      external: true,
      provider: 'openai-compatible',
      model: 'configured-model',
      riskReasons: risks,
    }
  }
  return {
    ok: true,
    message: '开发态 fallback 已模拟 AI 润色',
    draft,
    polishedDraft: {
      ...draft,
      id: `${draft.id || 'draft'}-ai-polished`,
      title: `AI 润色：${draft.title || '工作记忆草稿'}`,
      body: `${draft.body}\n\n## AI 润色提示\n- 开发态 fallback 未调用外部接口。\n- 真实桌面运行时会按设置和环境变量调用 OpenAI-compatible endpoint。`,
      createdAt: Math.floor(Date.now() / 1000),
    },
    external: true,
    provider: 'openai-compatible',
    model: 'configured-model',
    riskReasons: risks,
  }
}

function normalizeDraftPolishResult(result: WorkMemoryDraftPolishResult): WorkMemoryDraftPolishResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    draft: normalizeDraft(result.draft),
    polishedDraft: result.polishedDraft ? normalizeDraft(result.polishedDraft) : undefined,
    requiresConfirmation: Boolean(result.requiresConfirmation),
    external: Boolean(result.external),
    provider: result.provider || '',
    model: result.model || '',
    riskReasons: Array.isArray(result.riskReasons) ? result.riskReasons.map(String).filter(Boolean) : [],
  }
}

function normalizeDraft(draft: WorkMemoryDraft): WorkMemoryDraft {
  return {
    id: draft?.id || '',
    title: draft?.title || '',
    body: draft?.body || '',
    evidence: Array.isArray(draft?.evidence) ? draft.evidence.map(String).filter(Boolean) : [],
    createdAt: Number(draft?.createdAt ?? 0),
  }
}

function fallbackRunScheduledDrafts(): ScheduledDraftStatus {
  const dailyDraft = fallbackDailyDraft()
  const retrospectiveDraft = fallbackRetrospectiveDraft(fallbackTimeline.filter((entry) => !entry.sensitive).map((entry) => entry.id))
  const experienceReport = fallbackExperienceReport(7)
  fallbackScheduledDraftStatus = {
    ...fallbackScheduledDraftStatus,
    lastCheckedAt: Math.floor(Date.now() / 1000),
    lastRunAt: Math.floor(Date.now() / 1000),
    lastEntryCount: fallbackTimeline.filter((entry) => !entry.sensitive).length,
    lastEntryCreatedAt: Math.max(...fallbackTimeline.map((entry) => entry.createdAt)),
    lastError: undefined,
    dailyDraft,
    retrospectiveDraft,
    experienceReport,
  }
  return fallbackScheduledDraftStatus
}

export async function generateRetrospectiveDraft(entryIDs: string[]): Promise<WorkMemoryDraft> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateRetrospectiveDraft(entryIDs)
    } catch {
      return fallbackRetrospectiveDraft(entryIDs)
    }
  }
  return fallbackRetrospectiveDraft(entryIDs)
}

function fallbackRetrospectiveDraft(entryIDs: string[]): WorkMemoryDraft {
  const evidence = fallbackTimeline.filter((entry) => entryIDs.includes(entry.id) && !entry.sensitive)
  const lines = [
    '## 复盘概览',
    `- 开发态 fallback 基于 ${evidence.length} 条选中工作记忆生成。`,
    '',
    '## 问题背景',
    ...(evidence.length ? evidence.map((entry) => `- ${entry.title}：${entry.summary}`) : ['- 先选择一组工作记忆再生成问题复盘。']),
    '',
    '## 时间线',
    ...(evidence.length ? evidence.map((entry) => `- ${new Date(entry.createdAt * 1000).toLocaleString()} · ${entry.title}`) : ['- 暂无可用证据。']),
    '',
    '## 初步原因',
    '- 正式环境由 Go 本地规则根据错误、原因、验证和待办线索整理。',
    '',
    '## 遗留风险与后续动作',
    '- 保存为知识、工作流或交给代理前需要用户确认。',
  ]
  return {
    id: `retrospective-${Date.now()}`,
    title: '问题复盘草稿',
    body: lines.join('\n'),
    evidence: evidence.map((entry) => entry.id),
    createdAt: Math.floor(Date.now() / 1000),
  }
}

export async function generateKnowledgeDraft(entryIDs: string[]): Promise<WorkMemoryDraft> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateKnowledgeDraft(entryIDs)
    } catch {
      return fallbackKnowledgeDraft(entryIDs)
    }
  }
  return fallbackKnowledgeDraft(entryIDs)
}

function fallbackKnowledgeDraft(entryIDs: string[]): WorkMemoryDraft {
  return {
    id: `knowledge-${Date.now()}`,
    title: '知识条目草稿',
    body: '从选中工作记忆整理问题背景、处理步骤、注意事项和敏感内容提示。',
    evidence: entryIDs,
    createdAt: Math.floor(Date.now() / 1000),
  }
}

export async function generateAgentTaskPackage(goal: string, evidence: string[]): Promise<AgentTaskPackage> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateAgentTaskPackage(goal, evidence)
    } catch {
      return fallbackAgentTaskPackage(goal, evidence)
    }
  }
  return fallbackAgentTaskPackage(goal, evidence)
}

export async function generateWorkflowDraft(title: string, evidence: string[]): Promise<WorkflowDraft> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateWorkflowDraft(title, evidence)
    } catch {
      return fallbackWorkflowDraft(title, evidence)
    }
  }
  return fallbackWorkflowDraft(title, evidence)
}

export async function generateChecklistDraft(title: string, evidence: string[]): Promise<ChecklistDraft> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.GenerateChecklistDraft(title, evidence)
    } catch {
      return fallbackChecklistDraft(title, evidence)
    }
  }
  return fallbackChecklistDraft(title, evidence)
}

export async function discoverExperiences(periodDays = 7): Promise<ExperienceReport> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.DiscoverExperiences(periodDays)
    } catch {
      return fallbackExperienceReport(periodDays)
    }
  }
  return fallbackExperienceReport(periodDays)
}

export async function discoverExperiencesAI(request: ExperienceDiscoveryRequest = {}): Promise<ExperienceDiscoveryResult> {
  const binding = await tryWorkMemoryBinding()
  const normalized = {
    periodDays: request.periodDays || 7,
    external: request.external ?? true,
    confirmed: request.confirmed ?? false,
  }
  if (binding) {
    try {
      return await binding.DiscoverExperiencesAI(normalized)
    } catch {
      return fallbackExperienceDiscoveryResult(normalized)
    }
  }
  return fallbackExperienceDiscoveryResult(normalized)
}

export async function setExperienceInsightDecision(
  insightId: string,
  status: string,
  note = '',
  taskPackageId = '',
): Promise<ExperienceDecisionResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return await binding.SetExperienceInsightDecision(insightId, status, note, taskPackageId)
    } catch {
      return fallbackExperienceDecision(insightId, status, note, taskPackageId)
    }
  }
  return fallbackExperienceDecision(insightId, status, note, taskPackageId)
}

function fallbackExperienceReport(periodDays: number): ExperienceReport {
  const evidence = fallbackTimeline.slice(0, 3).map((entry) => entry.id)
  return {
    id: `experience-${Date.now()}`,
    title: '经验发现报告',
    summary: evidence.length ? '开发态 fallback 发现 1 条可整理线索，正式环境由本地规则和工作记忆证据生成。' : '暂无足够工作记忆生成经验发现。',
    periodDays,
    entryCount: fallbackTimeline.length,
    evidenceCount: evidence.length,
    generatedAt: Math.floor(Date.now() / 1000),
    insights: evidence.length
      ? [
          {
            id: 'insight-fallback-knowledge',
            kind: 'knowledge_gap',
            title: '知识沉淀机会',
            summary: '当前 fallback 时间线中存在可整理的排障记录。',
            reason: '有收藏或高信息密度记录，适合生成知识草稿。',
            recommendation: '生成知识草稿并保留证据，外发或同步前由用户确认。',
            evidence,
            confidence: 0.52,
            severity: 'medium',
            requiresReview: true,
            createdAt: Math.floor(Date.now() / 1000),
          },
        ]
      : [],
  }
}

function fallbackExperienceDiscoveryResult(request: ExperienceDiscoveryRequest): ExperienceDiscoveryResult {
  const report = fallbackExperienceReport(request.periodDays || 7)
  if (request.external && !request.confirmed) {
    return {
      ok: false,
      message: 'AI 经验发现需要二次确认',
      report,
      requiresConfirmation: true,
      external: true,
      provider: 'fallback',
      model: 'fallback',
      riskReasons: [
        '将发送非敏感工作记忆摘要和 evidence ID 到外部 AI provider。',
        '不会发送已标记敏感的记忆。',
        '返回线索仍需人工审核后才能转成工作流、清单或 Skill。',
      ],
    }
  }
  return {
    ok: !request.external,
    message: request.external ? '开发态 fallback 无法调用外部 AI，已保留本地规则报告' : '本地经验发现完成',
    report,
    external: Boolean(request.external),
    provider: request.external ? 'fallback' : undefined,
    model: request.external ? 'fallback' : undefined,
  }
}

function fallbackExperienceDecision(insightId: string, status: string, note: string, taskPackageId: string): ExperienceDecisionResult {
  const ok = Boolean(insightId && status)
  return {
    ok,
    message: ok ? '经验线索处理状态已保存' : '缺少经验线索或处理状态',
    decision: {
      insightId,
      status,
      note,
      taskPackageId,
      updatedAt: Math.floor(Date.now() / 1000),
    },
  }
}

function fallbackAgentTaskPackage(goal: string, evidence: string[]): AgentTaskPackage {
  return {
    id: `agent-task-${Date.now()}`,
    goal,
    context: '由 Ariadne 工作记忆中心生成，交给外部代理前必须由用户确认。',
    evidence,
    boundaries: ['不得绕过用户授权修改文件或运行高风险命令', '不得默认外发敏感记忆'],
    acceptance: ['任务包包含目标、上下文、证据、边界和验收标准'],
    requiresReview: true,
    createdAt: Math.floor(Date.now() / 1000),
  }
}

function fallbackWorkflowDraft(title: string, evidence: string[]): WorkflowDraft {
  return {
    id: `workflow-draft-${Date.now()}`,
    title: title.trim() || '候选工作流草稿',
    trigger: '用户在启动器中搜索相似任务并确认执行',
    input: '选中的文本、截图 OCR 或工作记忆证据',
    steps: [
      {
        id: 'collect-evidence',
        label: '收集当前输入和历史证据',
        command: 'work_memory.search(input)',
        requiresConfirm: false,
      },
      {
        id: 'preview-plan',
        label: '生成处理计划和预览结果',
        command: 'workflow.preview(evidence)',
        requiresConfirm: false,
      },
      {
        id: 'confirm-run',
        label: '用户确认后执行动作',
        command: 'workflow.run_after_confirm()',
        requiresConfirm: true,
      },
    ],
    output: '生成可复制结果或待审核记录，不自动外发敏感内容',
    riskLevel: 'medium',
    evidence,
    requiresReview: true,
    createdAt: Math.floor(Date.now() / 1000),
  }
}

function fallbackChecklistDraft(title: string, evidence: string[]): ChecklistDraft {
  return {
    id: `checklist-draft-${Date.now()}`,
    title: title.trim() || '检查清单草稿',
    context: '由本地工作记忆整理，保存到正式知识库前需要人工确认。',
    items: ['确认问题背景和触发条件', '核对关键命令、路径和环境差异', '标记敏感信息并决定是否脱敏', '补充验收方式和回滚条件'],
    evidence,
    requiresReview: true,
    createdAt: Math.floor(Date.now() / 1000),
  }
}
