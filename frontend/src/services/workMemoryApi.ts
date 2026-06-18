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
  WorkMemoryAutonomousArtifact,
  WorkMemoryAutonomousRejectRequest,
  WorkMemoryAutonomousRejectResult,
  WorkMemoryAutonomousRunResult,
  WorkMemoryDraftPolishRequest,
  WorkMemoryDraftPolishResult,
  WorkMemoryEmbeddingRefreshResult,
  WorkMemoryExportResult,
  WorkMemoryExportRequest,
  WorkMemoryDraft,
  WorkMemoryEntry,
  WorkMemoryFlowConversation,
  WorkMemoryFlowConversationAskRequest,
  WorkMemoryFlowConversationAskResult,
  WorkMemoryFlowMessage,
  WorkMemoryImportMaterialRequest,
  WorkMemoryImportMaterialResult,
  WorkMemoryFlowAskRequest,
  WorkMemoryFlowAskResponse,
  WorkMemoryHealthSummary,
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
  tags: ['网络', '留痕'],
  favorite: true,
  sensitive: false,
  createdAt: 1770000000,
}

let fallbackStatus: WorkMemoryStatus = {
  enabled: true,
  timeMachineEnabled: false,
  privacyMode: false,
  autoOcrEnabled: false,
  captureScope: 'active_window',
  multiMonitor: 'combined',
  pauseOnIdle: true,
  idlePauseSeconds: 600,
  pauseOnLock: true,
  sessionLocked: false,
  entryCount: 1,
  autoCaptureIntervalSeconds: 30,
  windowSwitchCaptureEnabled: true,
  windowSwitchCooldownSeconds: 3,
  appCaptureProfiles: [],
  captureCount: 0,
}

let fallbackHealth: WorkMemoryHealthSummary = {
  ok: true,
  message: '开发态采集健康，后台会继续质检、OCR 和清理',
  total: 1,
  today: 1,
  pending: 0,
  checked: 1,
  sensitive: 0,
  images: 0,
  multiFrame: 0,
  collapsedEntries: 0,
  removedFrames: 0,
  ocrDone: 0,
  ocrPending: 0,
  ocrFailed: 0,
  skippedSensitive: 0,
  skippedPending: 0,
  appStats: [
    {
      appName: fallbackEntry.appName || 'Terminal',
      count: 1,
      pending: 0,
      checked: 1,
      ocrDone: 0,
      sensitive: 0,
      lastSeenAt: fallbackEntry.createdAt,
    },
  ],
  recentEvents: [],
  generatedAt: Math.floor(Date.now() / 1000),
}

let fallbackTimeline: WorkMemoryEntry[] = [fallbackEntry]

let fallbackFlowConversations: WorkMemoryFlowConversation[] = []
let fallbackFlowMessages: Record<string, WorkMemoryFlowMessage[]> = {}

let fallbackScheduledDraftStatus: ScheduledDraftStatus = {
  enabled: true,
  running: false,
  intervalMinutes: 240,
  dailyDraftEnabled: true,
  retrospectiveEnabled: true,
  experienceReportEnabled: true,
  lastEntryCount: 0,
  autonomousGenerated: 0,
}

let fallbackAutonomousArtifacts: WorkMemoryAutonomousArtifact[] = [
  {
    id: 'auto-fallback-skill',
    kind: 'skill',
    title: '剪贴板上下文整理 Skill',
    summary: '开发态 fallback 模拟：当留痕足够、流程清晰且低风险时，心流会自动生成可执行 Skill 草稿。',
    body: '# 剪贴板上下文整理 Skill\n\n## When To Use\n当剪贴板文本反复需要格式化、提取或归档时使用。\n\n## Steps\n1. 读取当前剪贴板文本\n2. 判断文本类型并生成结构化摘要\n3. 输出可复制结果\n\n## Autonomous Boundary\n删除该产物并填写原因后，心流会避免再次生成同类 Skill。',
    evidence: [fallbackEntry.id],
    dedupKey: 'skill:fallback-clipboard-context',
    status: 'active',
    confidence: 0.86,
    agentExecutable: true,
    createdAt: fallbackEntry.createdAt,
    updatedAt: fallbackEntry.createdAt,
  },
]

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

export async function getWorkMemoryHealth(): Promise<WorkMemoryHealthSummary> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeHealthSummary(await binding.HealthSummary())
    } catch {
      return normalizeHealthSummary(fallbackHealth)
    }
  }
  return normalizeHealthSummary(fallbackHealth)
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

export async function getAutonomousArtifacts(): Promise<WorkMemoryAutonomousArtifact[]> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeAutonomousArtifacts(await binding.AutonomousArtifacts())
    } catch {
      return fallbackAutonomousArtifacts
    }
  }
  return fallbackAutonomousArtifacts
}

export async function runAutonomousFlowNow(): Promise<WorkMemoryAutonomousRunResult> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeAutonomousRunResult(await binding.RunAutonomousFlowNow())
    } catch {
      return fallbackRunAutonomousFlow()
    }
  }
  return fallbackRunAutonomousFlow()
}

export async function rejectAutonomousArtifact(request: WorkMemoryAutonomousRejectRequest): Promise<WorkMemoryAutonomousRejectResult> {
  const normalized = {
    id: request.id.trim(),
    reason: request.reason.trim() || '不需要这个自主产物',
  }
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeAutonomousRejectResult(await binding.RejectAutonomousArtifact(normalized))
    } catch {
      return fallbackRejectAutonomousArtifact(normalized)
    }
  }
  return fallbackRejectAutonomousArtifact(normalized)
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

export async function askWorkMemoryFlow(request: WorkMemoryFlowAskRequest): Promise<WorkMemoryFlowAskResponse> {
  const normalized = {
    question: request.question.trim(),
    limit: request.limit ?? 8,
    since: request.since ?? 0,
  }
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeFlowAskResponse(await binding.AskFlow(normalized), normalized.question)
    } catch {
      return fallbackAskFlow(normalized)
    }
  }
  return fallbackAskFlow(normalized)
}

export async function getWorkMemoryFlowConversations(): Promise<WorkMemoryFlowConversation[]> {
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeFlowConversations(await binding.FlowConversations())
    } catch {
      return normalizeFlowConversations(fallbackFlowConversations)
    }
  }
  return normalizeFlowConversations(fallbackFlowConversations)
}

export async function getWorkMemoryFlowMessages(conversationId: string): Promise<WorkMemoryFlowMessage[]> {
  const normalizedId = conversationId.trim()
  if (!normalizedId) return []
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeFlowMessages(await binding.FlowMessages(normalizedId))
    } catch {
      return normalizeFlowMessages(fallbackFlowMessages[normalizedId] ?? [])
    }
  }
  return normalizeFlowMessages(fallbackFlowMessages[normalizedId] ?? [])
}

export async function createWorkMemoryFlowConversation(title = ''): Promise<WorkMemoryFlowConversation> {
  const normalizedTitle = title.trim() || '新对话'
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeFlowConversation(await binding.CreateFlowConversation(normalizedTitle))
    } catch {
      return fallbackCreateFlowConversation(normalizedTitle)
    }
  }
  return fallbackCreateFlowConversation(normalizedTitle)
}

export async function askWorkMemoryFlowConversation(request: WorkMemoryFlowConversationAskRequest): Promise<WorkMemoryFlowConversationAskResult> {
  const normalized = {
    conversationId: request.conversationId?.trim() || '',
    question: request.question.trim(),
    limit: request.limit ?? 8,
    since: request.since ?? 0,
  }
  const binding = await tryWorkMemoryBinding()
  if (binding) {
    try {
      return normalizeFlowConversationAskResult(await binding.AskFlowConversation(normalized), normalized.question)
    } catch {
      return fallbackAskFlowConversation(normalized)
    }
  }
  return fallbackAskFlowConversation(normalized)
}

function normalizeFlowAskResponse(result: WorkMemoryFlowAskResponse, question: string): WorkMemoryFlowAskResponse {
  return {
    ok: Boolean(result?.ok),
    question: result?.question || question,
    title: result?.title || question || '心流回答',
    answer: result?.answer || result?.message || '',
    intent: result?.intent || 'search',
    mode: result?.mode || 'local',
    evidence: Array.isArray(result?.evidence)
      ? result.evidence
          .filter((item) => item?.id && !item.sensitive)
          .map((item) => ({
            id: item.id,
            title: item.title || item.id,
            summary: item.summary || '',
            source: item.source || '',
            appName: item.appName || '',
            windowTitle: item.windowTitle || '',
            createdAt: Number(item.createdAt ?? 0),
            score: Number(item.score ?? 0),
            hasImage: Boolean(item.hasImage),
            sensitive: Boolean(item.sensitive),
            tags: Array.isArray(item.tags) ? item.tags.map(String).filter(Boolean) : [],
          }))
      : [],
    suggestedQuestions: Array.isArray(result?.suggestedQuestions) ? result.suggestedQuestions.map(String).filter(Boolean) : [],
    usedAi: Boolean(result?.usedAi),
    message: result?.message || '',
    createdAt: Number(result?.createdAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeFlowConversation(item: WorkMemoryFlowConversation): WorkMemoryFlowConversation {
  const now = Math.floor(Date.now() / 1000)
  return {
    id: item?.id || fallbackFlowId('conversation'),
    title: item?.title || '新对话',
    createdAt: Number(item?.createdAt ?? now),
    updatedAt: Number(item?.updatedAt ?? item?.createdAt ?? now),
    messageCount: Number(item?.messageCount ?? 0),
    lastMessage: item?.lastMessage || '',
  }
}

function normalizeFlowConversations(items: WorkMemoryFlowConversation[]): WorkMemoryFlowConversation[] {
  return Array.isArray(items)
    ? items
        .filter((item) => item?.id)
        .map(normalizeFlowConversation)
        .sort((left, right) => right.updatedAt - left.updatedAt)
    : []
}

function normalizeFlowMessage(item: WorkMemoryFlowMessage): WorkMemoryFlowMessage {
  const role = item?.role === 'user' ? 'user' : 'assistant'
  const result = item?.result ? normalizeFlowAskResponse(item.result, item.question || item.text || '') : undefined
  return {
    id: item?.id || fallbackFlowId(`message-${role}`),
    conversationId: item?.conversationId || '',
    role,
    text: item?.text || '',
    question: item?.question || result?.question || '',
    result,
    error: Boolean(item?.error),
    createdAt: Number(item?.createdAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeFlowMessages(items: WorkMemoryFlowMessage[]): WorkMemoryFlowMessage[] {
  return Array.isArray(items) ? items.filter((item) => item?.id).map(normalizeFlowMessage).sort((left, right) => left.createdAt - right.createdAt) : []
}

function normalizeFlowConversationAskResult(result: WorkMemoryFlowConversationAskResult, question: string): WorkMemoryFlowConversationAskResult {
  const response = normalizeFlowAskResponse(result?.response, question)
  const conversation = normalizeFlowConversation(result?.conversation)
  const messages = normalizeFlowMessages(result?.messages ?? [])
  return {
    ok: Boolean(result?.ok),
    message: result?.message || response.message || '',
    conversation,
    messages,
    response,
  }
}

function fallbackAskFlow(request: WorkMemoryFlowAskRequest): WorkMemoryFlowAskResponse {
  const question = request.question.trim()
  const now = Math.floor(Date.now() / 1000)
  const limit = request.limit && request.limit > 0 ? request.limit : 8
  const todayStart = new Date()
  todayStart.setHours(0, 0, 0, 0)
  const since = request.since || Math.floor(todayStart.getTime() / 1000)
  const haystack = question.toLowerCase()
  const intent = /优化|工作流|自动化|重复|流程|效率/.test(question)
    ? 'optimization'
    : /谁|人|找过|联系|消息|沟通|微信|weixin|wechat|会议|meeting|mail|邮件/i.test(question)
      ? 'contacts'
      : /今天|今日|干了|做了|总结|发生|上下文|心流/.test(question)
        ? 'today'
        : 'search'
  const visible = fallbackTimeline.filter((entry) => !entry.sensitive && (!request.since || entry.createdAt >= request.since))
  const selected =
    intent === 'contacts'
      ? visible.filter((entry) =>
          /微信|weixin|wechat|钉钉|dingtalk|qq|teams|outlook|邮件|mail|meeting|会议|消息|聊天/i.test(
            [entry.title, entry.summary, entry.text, entry.windowTitle, entry.appName, entry.source].filter(Boolean).join(' '),
          ),
        )
      : intent === 'search' && haystack
        ? visible.filter((entry) =>
            [entry.title, entry.summary, entry.text, entry.ocrText, entry.windowTitle, entry.appName, entry.source, ...entry.tags]
              .filter(Boolean)
              .join(' ')
              .toLowerCase()
              .includes(haystack),
          )
        : visible.filter((entry) => entry.createdAt >= since)
  const evidence = selected.slice(0, limit).map((entry) => ({
    id: entry.id,
    title: entry.title,
    summary: entry.summary,
    source: entry.source,
    appName: entry.appName,
    windowTitle: entry.windowTitle,
    createdAt: entry.createdAt,
    score: 0,
    hasImage: Boolean(entry.captureId || entry.imagePath),
    sensitive: false,
    tags: entry.tags,
  }))
  const answer = evidence.length
    ? `我找到 ${evidence.length} 条相关留痕，已按当前问题整理。`
    : '当前没有找到足够的本地留痕。可以换个关键词，或先让时间机器继续积累截图、OCR 和剪贴板上下文。'
  return {
    ok: true,
    question,
    title: question || '心流回答',
    answer,
    intent,
    mode: 'fallback',
    evidence,
    suggestedQuestions: ['我今天干了些什么？', '今天有哪些人找过我？', '今天我的哪些工作流可以优化？'],
    usedAi: false,
    message: '',
    createdAt: now,
  }
}

function fallbackCreateFlowConversation(title = '新对话'): WorkMemoryFlowConversation {
  const now = Math.floor(Date.now() / 1000)
  const conversation: WorkMemoryFlowConversation = {
    id: fallbackFlowId('conversation'),
    title: title.trim() || '新对话',
    createdAt: now,
    updatedAt: now,
    messageCount: 0,
    lastMessage: '',
  }
  fallbackFlowConversations = [conversation, ...fallbackFlowConversations]
  fallbackFlowMessages[conversation.id] = []
  return conversation
}

function fallbackAskFlowConversation(request: WorkMemoryFlowConversationAskRequest): WorkMemoryFlowConversationAskResult {
  let conversation = request.conversationId ? fallbackFlowConversations.find((item) => item.id === request.conversationId) : undefined
  if (!conversation) {
    conversation = fallbackCreateFlowConversation(shortFlowTitle(request.question))
  }
  const response = fallbackAskFlow(request)
  const now = Math.floor(Date.now() / 1000)
  const userMessage: WorkMemoryFlowMessage = {
    id: fallbackFlowId('message-user'),
    conversationId: conversation.id,
    role: 'user',
    text: request.question,
    createdAt: now,
  }
  const assistantMessage: WorkMemoryFlowMessage = {
    id: fallbackFlowId('message-assistant'),
    conversationId: conversation.id,
    role: 'assistant',
    text: response.answer || response.message || '心流问答暂时不可用。',
    question: request.question,
    result: response,
    error: !response.ok,
    createdAt: response.createdAt || now,
  }
  fallbackFlowMessages[conversation.id] = [...(fallbackFlowMessages[conversation.id] ?? []), userMessage, assistantMessage]
  conversation = {
    ...conversation,
    title: conversation.title || shortFlowTitle(request.question),
    updatedAt: assistantMessage.createdAt,
    messageCount: fallbackFlowMessages[conversation.id].length,
    lastMessage: assistantMessage.text,
  }
  fallbackFlowConversations = [conversation, ...fallbackFlowConversations.filter((item) => item.id !== conversation.id)]
  return {
    ok: response.ok,
    message: response.message,
    conversation,
    messages: fallbackFlowMessages[conversation.id],
    response,
  }
}

function fallbackFlowId(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function shortFlowTitle(value: string) {
  const normalized = value.trim().replace(/\s+/g, ' ')
  if (!normalized) return '新对话'
  return normalized.length > 32 ? `${normalized.slice(0, 32)}...` : normalized
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

function normalizeHealthSummary(summary: WorkMemoryHealthSummary): WorkMemoryHealthSummary {
  const generatedAt = Math.floor(Date.now() / 1000)
  return {
    ok: summary?.ok !== false,
    message: summary?.message || '采集健康，后台会继续质检、OCR 和清理',
    total: Number(summary?.total || 0),
    today: Number(summary?.today || 0),
    pending: Number(summary?.pending || 0),
    checked: Number(summary?.checked || 0),
    sensitive: Number(summary?.sensitive || 0),
    images: Number(summary?.images || 0),
    multiFrame: Number(summary?.multiFrame || 0),
    collapsedEntries: Number(summary?.collapsedEntries || 0),
    removedFrames: Number(summary?.removedFrames || 0),
    ocrDone: Number(summary?.ocrDone || 0),
    ocrPending: Number(summary?.ocrPending || 0),
    ocrFailed: Number(summary?.ocrFailed || 0),
    qualityOcrDone: Number(summary?.qualityOcrDone || 0),
    qualityOcrPending: Number(summary?.qualityOcrPending || 0),
    qualityOcrFailed: Number(summary?.qualityOcrFailed || 0),
    skippedSensitive: Number(summary?.skippedSensitive || 0),
    skippedPending: Number(summary?.skippedPending || 0),
    lastCaptureAt: summary?.lastCaptureAt,
    lastQualityCheckAt: summary?.lastQualityCheckAt,
    lastAutoOcrAt: summary?.lastAutoOcrAt,
    lastSkippedReason: summary?.lastSkippedReason || '',
    lastAutoOcrError: summary?.lastAutoOcrError || '',
    appStats: Array.isArray(summary?.appStats)
      ? summary.appStats.map((item) => ({
          appName: item.appName || 'Unknown',
          count: Number(item.count || 0),
          pending: Number(item.pending || 0),
          checked: Number(item.checked || 0),
          ocrDone: Number(item.ocrDone || 0),
          qualityOcr: Number(item.qualityOcr || 0),
          sensitive: Number(item.sensitive || 0),
          lastSeenAt: item.lastSeenAt,
        }))
      : [],
    recentEvents: Array.isArray(summary?.recentEvents)
      ? summary.recentEvents.map((item) => ({
          id: item.id || '',
          kind: item.kind || 'event',
          title: item.title || '后台事件',
          detail: item.detail || '',
          appName: item.appName || '',
          createdAt: item.createdAt,
        }))
      : [],
    generatedAt: summary?.generatedAt || generatedAt,
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
    lastAutonomousRunAt: Math.floor(Date.now() / 1000),
    autonomousGenerated: fallbackAutonomousArtifacts.length,
    autonomousMessage: fallbackAutonomousArtifacts.length ? `开发态 fallback 自主产物 ${fallbackAutonomousArtifacts.length} 个` : '暂无自主产物',
  }
  return fallbackScheduledDraftStatus
}

function fallbackRunAutonomousFlow(): WorkMemoryAutonomousRunResult {
  const createdAt = Math.floor(Date.now() / 1000)
  fallbackScheduledDraftStatus = {
    ...fallbackScheduledDraftStatus,
    lastAutonomousRunAt: createdAt,
    autonomousGenerated: fallbackAutonomousArtifacts.length,
    autonomousMessage: fallbackAutonomousArtifacts.length ? `开发态 fallback 自主产物 ${fallbackAutonomousArtifacts.length} 个` : '暂无自主产物',
  }
  return {
    ok: true,
    message: fallbackScheduledDraftStatus.autonomousMessage || '',
    generated: fallbackAutonomousArtifacts.length,
    skipped: 0,
    artifacts: fallbackAutonomousArtifacts,
    status: fallbackScheduledDraftStatus,
    createdAt,
  }
}

function fallbackRejectAutonomousArtifact(request: WorkMemoryAutonomousRejectRequest): WorkMemoryAutonomousRejectResult {
  const artifact = fallbackAutonomousArtifacts.find((item) => item.id === request.id)
  if (!artifact) {
    return {
      ok: false,
      message: '未找到自主产物',
      status: fallbackScheduledDraftStatus,
    }
  }
  const rejected = {
    ...artifact,
    status: 'rejected',
    deleteReason: request.reason,
    deletedAt: Math.floor(Date.now() / 1000),
    updatedAt: Math.floor(Date.now() / 1000),
  }
  fallbackAutonomousArtifacts = fallbackAutonomousArtifacts.filter((item) => item.id !== request.id)
  fallbackScheduledDraftStatus = {
    ...fallbackScheduledDraftStatus,
    autonomousGenerated: fallbackAutonomousArtifacts.length,
    autonomousMessage: '已删除该自主产物，并记录拒绝原因',
  }
  return {
    ok: true,
    message: '已删除该自主产物，并记录拒绝原因',
    artifact: rejected,
    status: fallbackScheduledDraftStatus,
  }
}

function normalizeAutonomousArtifacts(items: WorkMemoryAutonomousArtifact[] | undefined): WorkMemoryAutonomousArtifact[] {
  if (!Array.isArray(items)) {
    return []
  }
  return items
    .map((item) => ({
      id: item?.id || '',
      kind: item?.kind || 'knowledge',
      title: item?.title || '自主产物',
      summary: item?.summary || '',
      body: item?.body || '',
      evidence: Array.isArray(item?.evidence) ? item.evidence.map(String).filter(Boolean) : [],
      sourceInsightId: item?.sourceInsightId || '',
      dedupKey: item?.dedupKey || '',
      status: item?.status || 'active',
      deleteReason: item?.deleteReason || '',
      confidence: Number(item?.confidence ?? 0),
      agentExecutable: Boolean(item?.agentExecutable),
      createdAt: Number(item?.createdAt ?? 0),
      updatedAt: Number(item?.updatedAt ?? 0),
      deletedAt: Number(item?.deletedAt ?? 0),
    }))
    .filter((item) => item.id)
}

function normalizeAutonomousRunResult(result: WorkMemoryAutonomousRunResult): WorkMemoryAutonomousRunResult {
  return {
    ok: Boolean(result?.ok),
    message: result?.message || '',
    generated: Number(result?.generated ?? 0),
    skipped: Number(result?.skipped ?? 0),
    artifacts: normalizeAutonomousArtifacts(result?.artifacts),
    status: {
      ...(result?.status ?? fallbackScheduledDraftStatus),
      autonomousGenerated: Number(result?.status?.autonomousGenerated ?? result?.generated ?? 0),
    },
    createdAt: Number(result?.createdAt ?? Math.floor(Date.now() / 1000)),
  }
}

function normalizeAutonomousRejectResult(result: WorkMemoryAutonomousRejectResult): WorkMemoryAutonomousRejectResult {
  return {
    ok: Boolean(result?.ok),
    message: result?.message || '',
    artifact: result?.artifact ? normalizeAutonomousArtifacts([result.artifact])[0] : undefined,
    status: {
      ...(result?.status ?? fallbackScheduledDraftStatus),
      autonomousGenerated: Number(result?.status?.autonomousGenerated ?? 0),
    },
  }
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
    ...(evidence.length ? evidence.map((entry) => `- ${new Date(entry.createdAt * 1000).toLocaleString()} · ${entry.title}`) : ['- 暂无可用留痕。']),
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
    summary: evidence.length ? '开发态 fallback 发现 1 条可整理线索，正式环境由本地规则和工作记忆留痕生成。' : '暂无足够工作记忆生成经验发现。',
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
            recommendation: '生成知识草稿并保留留痕，外发或同步前由用户确认。',
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
    acceptance: ['任务包包含目标、上下文、留痕、边界和验收标准'],
    requiresReview: true,
    createdAt: Math.floor(Date.now() / 1000),
  }
}

function fallbackWorkflowDraft(title: string, evidence: string[]): WorkflowDraft {
  return {
    id: `workflow-draft-${Date.now()}`,
    title: title.trim() || '候选工作流草稿',
    trigger: '用户在启动器中搜索相似任务并确认执行',
    input: '选中的文本、截图 OCR 或工作记忆留痕',
    steps: [
      {
        id: 'collect-evidence',
        label: '收集当前输入和历史留痕',
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
