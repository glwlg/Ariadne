import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard, Events } from '@wailsio/runtime'
import { getCaptureImageDataURL } from '../services/captureApi'
import {
  addWorkMemoryNote,
  captureCurrentScreen,
  captureTimeMachineNow,
  clearUnpinnedWorkMemory,
  decideFlowCandidateAction,
  deleteWorkMemoryEntry,
  discoverExperiences,
  discoverExperiencesAI,
  exportWorkMemoryData,
  exportWorkMemoryDataWithOptions,
  generateAgentTaskPackage,
  generateChecklistDraft,
  generateDailyDraft,
  generateKnowledgeDraft,
  generateRetrospectiveDraft,
  generateWorkflowDraft,
  getAutonomousArtifacts,
  getFlowAutonomyStatus,
  getWorkMemoryFlowConversations,
  getWorkMemoryFlowMessages,
  getWorkMemorySelfModel,
  getScheduledDraftStatus,
  getSemanticStatus,
  getWorkMemoryHealth,
  getWorkMemoryStatus,
  getWorkMemoryTimeline,
  listWorkMemoryTodos,
  listFlowCandidateActions,
  importWorkMemoryMaterials,
  askWorkMemoryFlow,
  askWorkMemoryFlowConversation,
  createWorkMemoryFlowConversation,
  deleteWorkMemoryFlowConversation,
  polishWorkMemoryDraft,
  refreshEmbeddingIndex,
  rejectAutonomousArtifact as rejectAutonomousArtifactApi,
  runAutonomousFlowNow,
  runFlowAutonomyNow,
  runScheduledDraftsNow,
  searchWorkMemory,
  semanticSearchExternal,
  setExperienceInsightDecision,
  setTimeMachineEnabled,
  setWorkMemoryPrivacyMode,
  upsertWorkMemorySelfAssertion,
  deleteWorkMemorySelfAssertion,
  upsertWorkMemoryTodo,
  updateWorkMemoryTodo,
  deleteWorkMemoryTodo,
} from '../services/workMemoryApi'
import { recognizeWorkMemoryOCR } from '../services/ocrApi'
import { saveChecklistDraft } from '../services/checklistApi'
import { exportSkillPackage, getSkillInstallDiagnostics, installSkillPackage, saveSkillDraft } from '../services/skillApi'
import { saveWorkflowDraft } from '../services/workflowApi'
import { useSettingsStore } from './settings'
import { createOCRSelection } from '../lib/ocrSelection'
import type {
  AgentTaskPackage,
  ChecklistDraftSaveResult,
  AppSettings,
  ChecklistDraft,
  ExperienceInsight,
  ExperienceDiscoveryResult,
  ExperienceReport,
  FlowAutonomyRunResult,
  FlowAutonomyStatus,
  FlowCandidateAction,
  FlowCandidateActionDecisionResult,
  FlowCandidateActionList,
  FlowNotificationAction,
  OCRResult,
  SearchResult,
  ScheduledDraftStatus,
  SkillDraftSaveResult,
  SkillExportResult,
  SkillInstallDiagnosticsResult,
  SkillInstallResult,
  WorkflowDraft,
  WorkflowDraftSaveResult,
  WorkMemoryAutonomousArtifact,
  WorkMemoryAutonomousRunResult,
  WorkMemoryDraft,
  WorkMemoryDraftPolishResult,
  WorkMemoryEmbeddingRefreshResult,
  WorkMemoryEntry,
  WorkMemoryExportRequest,
  WorkMemoryExportResult,
  WorkMemoryFlowConversation,
  WorkMemoryFlowMessage,
  WorkMemoryFlowAskResponse,
  WorkMemoryHealthSummary,
  WorkMemoryImportMaterialResult,
  WorkMemoryNoteRequest,
  WorkMemorySemanticSearchResult,
  WorkMemorySemanticStatus,
  WorkMemorySelfAssertion,
  WorkMemorySelfAssertionRequest,
  WorkMemorySelfModel,
  WorkMemoryStatus,
  WorkMemoryTodoItem,
  WorkMemoryTodoList,
  WorkMemoryTodoRequest,
  WorkMemoryTodoUpdateRequest,
} from '../types/ariadne'

export const useWorkMemoryStore = defineStore('work-memory', () => {
  const workMemoryChangedEvent = 'ariadne:work-memory-changed'
  const status = ref<WorkMemoryStatus>({
    enabled: true,
    timeMachineEnabled: false,
    workerRunning: false,
    privacyMode: false,
    autoOcrEnabled: false,
    captureScope: 'active_window',
    multiMonitor: 'combined',
    pauseOnIdle: true,
    idlePauseSeconds: 600,
    pauseOnLock: true,
    sessionLocked: false,
    entryCount: 0,
    autoCaptureIntervalSeconds: 30,
    windowSwitchCaptureEnabled: true,
    windowSwitchCooldownSeconds: 3,
    captureCount: 0,
  })
  const entries = ref<WorkMemoryEntry[]>([])
  const selectedId = ref('')
  const retrospectiveSelectedIds = ref<string[]>([])
  const query = ref('')
  const searchResults = ref<SearchResult[]>([])
  const dailyDraft = ref<WorkMemoryDraft | null>(null)
  const retrospectiveDraft = ref<WorkMemoryDraft | null>(null)
  const knowledgeDraft = ref<WorkMemoryDraft | null>(null)
  const scheduledDraftStatus = ref<ScheduledDraftStatus | null>(null)
  const autonomousArtifacts = ref<WorkMemoryAutonomousArtifact[]>([])
  const autonomousRunResult = ref<WorkMemoryAutonomousRunResult | null>(null)
  const flowAutonomyStatus = ref<FlowAutonomyStatus | null>(null)
  const flowCandidateActions = ref<FlowCandidateActionList | null>(null)
  const flowAutonomyRunResult = ref<FlowAutonomyRunResult | null>(null)
  const flowCandidateDecisionResult = ref<FlowCandidateActionDecisionResult | null>(null)
  const health = ref<WorkMemoryHealthSummary | null>(null)
  const semanticStatus = ref<WorkMemorySemanticStatus | null>(null)
  const embeddingRefreshResult = ref<WorkMemoryEmbeddingRefreshResult | null>(null)
  const semanticSearchResult = ref<WorkMemorySemanticSearchResult | null>(null)
  const flowAskResult = ref<WorkMemoryFlowAskResponse | null>(null)
  const flowConversations = ref<WorkMemoryFlowConversation[]>([])
  const flowMessages = ref<WorkMemoryFlowMessage[]>([])
  const selfModel = ref<WorkMemorySelfModel | null>(null)
  const todoList = ref<WorkMemoryTodoList | null>(null)
  const activeFlowConversationId = ref('')
  const dailyDraftPolishResult = ref<WorkMemoryDraftPolishResult | null>(null)
  const knowledgeDraftSaveResult = ref<SkillDraftSaveResult | null>(null)
  const knowledgeSkillExportResult = ref<SkillExportResult | null>(null)
  const knowledgeSkillInstallResult = ref<SkillInstallResult | null>(null)
  const knowledgeSkillInstallDiagnostics = ref<SkillInstallDiagnosticsResult | null>(null)
  const agentTask = ref<AgentTaskPackage | null>(null)
  const workflowDraft = ref<WorkflowDraft | null>(null)
  const workflowDraftSaveResult = ref<WorkflowDraftSaveResult | null>(null)
  const checklistDraft = ref<ChecklistDraft | null>(null)
  const checklistDraftSaveResult = ref<ChecklistDraftSaveResult | null>(null)
  const experienceReport = ref<ExperienceReport | null>(null)
  const experienceDiscoveryResult = ref<ExperienceDiscoveryResult | null>(null)
  const exportResult = ref<WorkMemoryExportResult | null>(null)
  const importResult = ref<WorkMemoryImportMaterialResult | null>(null)
  const ocrResult = ref<OCRResult | null>(null)
  const noteDraft = ref({
    title: '',
    text: '',
    tags: '',
    favorite: false,
    sensitive: false,
  })
  const importDraft = ref({
    paths: '',
    tags: '',
    favorite: false,
    sensitive: false,
  })
  const exportDraft = ref({
    recentDays: '',
    tags: '',
    entryIds: '',
  })
  const semanticDraft = ref({
    query: '',
  })
  const exclusionDraft = ref({
    apps: '',
    windowKeywords: '',
    paths: '',
    urls: '',
    contentPatterns: '',
  })
  const deleteArmedId = ref('')
  const clearUnpinnedArmed = ref(false)
  const selectedImageUrl = ref('')
  const selectedImageMissing = ref(false)
  const playbackIndex = ref(-1)
  const playbackImageUrl = ref('')
  const playbackImageMissing = ref(false)
  const feedback = ref('')
  const isLoading = ref(false)
  const isLoadingPlaybackImage = ref(false)
  const isRecognizingOCR = ref(false)
  const isImportingMaterials = ref(false)
  const isSavingKnowledgeDraft = ref(false)
  const isExportingKnowledgeSkill = ref(false)
  const isInstallingKnowledgeSkill = ref(false)
  const isSavingWorkflowDraft = ref(false)
  const isSavingChecklistDraft = ref(false)
  const isSavingExclusions = ref(false)
  const isRunningScheduledDrafts = ref(false)
  const isRunningAutonomousFlow = ref(false)
  const isRunningFlowAutonomy = ref(false)
  const isDecidingFlowCandidate = ref(false)
  const isPolishingDailyDraft = ref(false)
  const isRefreshingEmbedding = ref(false)
  const isSemanticSearching = ref(false)
  const isAskingFlow = ref(false)
  const isLoadingFlowConversation = ref(false)
  const isDeletingFlowConversation = ref(false)
  const isSavingSelfAssertion = ref(false)
  const isSavingTodo = ref(false)
  const isDiscoveringExperienceAI = ref(false)
  const isDeletingEntries = ref(false)
  const isBatchRecognizingOCR = ref(false)
  const experienceDiscoveryProgress = ref(0)
  const experienceDiscoveryStage = ref('')
  const deleteProgressDone = ref(0)
  const deleteProgressTotal = ref(0)
  const batchOcrProgressDone = ref(0)
  const batchOcrProgressTotal = ref(0)
  const batchOcrProgressStage = ref('')
  let experienceProgressTimer: number | null = null
  let liveUpdateStop: (() => void) | null = null
  let liveUpdateTimer: number | null = null
  let liveRefreshInFlight = false
  let liveRefreshQueued = false
  const knowledgeDraftSaveArmed = ref(false)
  const knowledgeSkillExportArmed = ref(false)
  const knowledgeSkillInstallArmed = ref(false)
  const workflowDraftSaveArmed = ref(false)
  const checklistDraftSaveArmed = ref(false)
  const dailyDraftPolishArmed = ref(false)
  const experienceDiscoveryArmed = ref(false)
  const ocrSelection = createOCRSelection(ocrResult)
  const proactiveSourcesEnabled = computed(() => {
    const memory = useSettingsStore().settings?.workMemory
    return Boolean(memory?.enabled && !memory.privacyMode && memory.sourceClipboard && memory.sourceCaptureHistory)
  })

  const selectedEntry = computed(() => {
    return entries.value.find((entry) => entry.id === selectedId.value) ?? entries.value[0] ?? null
  })

  const activeFlowConversation = computed(() => {
    return flowConversations.value.find((conversation) => conversation.id === activeFlowConversationId.value) ?? flowConversations.value[0] ?? null
  })

  const retrospectiveSelectedIdSet = computed(() => new Set(retrospectiveSelectedIds.value))

  const retrospectiveSelectedEntries = computed(() => {
    const selected = retrospectiveSelectedIdSet.value
    return entries.value.filter((entry) => selected.has(entry.id) && !entry.sensitive)
  })

  const retrospectiveSelectionCount = computed(() => retrospectiveSelectedEntries.value.length)

  const retrospectiveDraftEntryIds = computed(() => {
    const selectedIds = retrospectiveSelectedEntries.value.map((entry) => entry.id)
    if (selectedIds.length) {
      return selectedIds
    }
    return selectedEntry.value?.id ? [selectedEntry.value.id] : []
  })

  const retrospectiveTargetLabel = computed(() => {
    if (retrospectiveSelectionCount.value) {
      return `复盘留痕 ${retrospectiveSelectionCount.value} 条`
    }
    return selectedEntry.value ? '当前详情记忆' : '未选择记忆'
  })

  const filteredEntries = computed(() => {
    const normalized = query.value.trim().toLowerCase()
    if (!normalized) {
      return entries.value
    }
    return entries.value.filter((entry) => {
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
  })

  const exclusionSummary = computed(() => {
    const counts = [
      ['应用', splitRuleLines(exclusionDraft.value.apps).length],
      ['窗口', splitRuleLines(exclusionDraft.value.windowKeywords).length],
      ['路径', splitRuleLines(exclusionDraft.value.paths).length],
      ['URL', splitRuleLines(exclusionDraft.value.urls).length],
      ['内容', splitRuleLines(exclusionDraft.value.contentPatterns).length],
    ]
    return counts.map(([label, count]) => `${label} ${count}`).join(' · ')
  })

  const playbackEntries = computed(() => {
    return entries.value
      .filter(isPlaybackEntry)
      .slice()
      .sort((left, right) => left.createdAt - right.createdAt)
  })

  const playbackEntry = computed(() => {
    return playbackIndex.value >= 0 ? playbackEntries.value[playbackIndex.value] ?? null : null
  })

  const playbackPosition = computed(() => {
    const total = playbackEntries.value.length
    return total && playbackEntry.value ? `${playbackIndex.value + 1} / ${total}` : `0 / ${total}`
  })

  async function load() {
    isLoading.value = true
    try {
      const [
        nextStatus,
        nextEntries,
        nextScheduledDrafts,
        nextSemanticStatus,
        nextAutonomousArtifacts,
        nextFlowAutonomyStatus,
        nextFlowCandidateActions,
        nextHealth,
        nextFlowConversations,
        nextSelfModel,
        nextTodoList,
      ] = await Promise.all([
        getWorkMemoryStatus(),
        getWorkMemoryTimeline(),
        getScheduledDraftStatus(),
        getSemanticStatus(),
        getAutonomousArtifacts(),
        getFlowAutonomyStatus(),
        listFlowCandidateActions({ limit: 20 }),
        getWorkMemoryHealth(),
        getWorkMemoryFlowConversations(),
        getWorkMemorySelfModel(),
        listWorkMemoryTodos({ includeDone: true, limit: 300 }),
      ])
      status.value = nextStatus
      entries.value = nextEntries
      scheduledDraftStatus.value = nextScheduledDrafts
      semanticStatus.value = nextSemanticStatus
      autonomousArtifacts.value = nextAutonomousArtifacts
      flowAutonomyStatus.value = nextFlowAutonomyStatus
      flowCandidateActions.value = nextFlowCandidateActions
      health.value = nextHealth
      flowConversations.value = nextFlowConversations
      selfModel.value = nextSelfModel
      todoList.value = nextTodoList
      if (!activeFlowConversationId.value && nextFlowConversations.length) {
        activeFlowConversationId.value = nextFlowConversations[0].id
        flowMessages.value = await getWorkMemoryFlowMessages(activeFlowConversationId.value)
      } else if (activeFlowConversationId.value) {
        flowMessages.value = await getWorkMemoryFlowMessages(activeFlowConversationId.value)
      }
      pruneRetrospectiveSelection()
      if (!selectedId.value || !entries.value.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      await loadSelectedImage()
      await loadExclusionRules()
    } finally {
      isLoading.value = false
    }
  }

  function installLiveUpdates() {
    if (liveUpdateStop) {
      return liveUpdateStop
    }

    const handleChanged = () => {
      queueLiveRefresh()
    }
    window.addEventListener(workMemoryChangedEvent, handleChanged)

    let uninstallWailsEvent = () => {}
    try {
      uninstallWailsEvent = Events.On(workMemoryChangedEvent, handleChanged)
    } catch {
      uninstallWailsEvent = () => {}
    }

    liveUpdateStop = () => {
      uninstallWailsEvent()
      window.removeEventListener(workMemoryChangedEvent, handleChanged)
      if (liveUpdateTimer !== null) {
        window.clearTimeout(liveUpdateTimer)
        liveUpdateTimer = null
      }
      liveUpdateStop = null
    }
    return liveUpdateStop
  }

  function queueLiveRefresh(delay = 180) {
    if (liveUpdateTimer !== null) {
      window.clearTimeout(liveUpdateTimer)
    }
    liveUpdateTimer = window.setTimeout(() => {
      liveUpdateTimer = null
      void refreshLiveData()
    }, delay)
  }

  async function refreshLiveData() {
    if (liveRefreshInFlight) {
      liveRefreshQueued = true
      return
    }
    liveRefreshInFlight = true
    try {
      const previousSelectedId = selectedId.value
      const [
        nextStatus,
        nextEntries,
        nextScheduledDrafts,
        nextSemanticStatus,
        nextAutonomousArtifacts,
        nextFlowAutonomyStatus,
        nextFlowCandidateActions,
        nextHealth,
        nextFlowConversations,
        nextSelfModel,
        nextTodoList,
      ] = await Promise.all([
        getWorkMemoryStatus(),
        getWorkMemoryTimeline(),
        getScheduledDraftStatus(),
        getSemanticStatus(),
        getAutonomousArtifacts(),
        getFlowAutonomyStatus(),
        listFlowCandidateActions({ limit: 20 }),
        getWorkMemoryHealth(),
        getWorkMemoryFlowConversations(),
        getWorkMemorySelfModel(),
        listWorkMemoryTodos({ includeDone: true, limit: 300 }),
      ])
      status.value = nextStatus
      entries.value = nextEntries
      scheduledDraftStatus.value = nextScheduledDrafts
      semanticStatus.value = nextSemanticStatus
      autonomousArtifacts.value = nextAutonomousArtifacts
      flowAutonomyStatus.value = nextFlowAutonomyStatus
      flowCandidateActions.value = nextFlowCandidateActions
      health.value = nextHealth
      flowConversations.value = nextFlowConversations
      selfModel.value = nextSelfModel
      todoList.value = nextTodoList
      if (activeFlowConversationId.value && nextFlowConversations.some((conversation) => conversation.id === activeFlowConversationId.value)) {
        flowMessages.value = await getWorkMemoryFlowMessages(activeFlowConversationId.value)
      } else if (nextFlowConversations.length) {
        activeFlowConversationId.value = nextFlowConversations[0].id
        flowMessages.value = await getWorkMemoryFlowMessages(activeFlowConversationId.value)
      } else {
        activeFlowConversationId.value = ''
        flowMessages.value = []
      }
      pruneRetrospectiveSelection()
      const selectedStillExists = Boolean(previousSelectedId && entries.value.some((entry) => entry.id === previousSelectedId))
      if (selectedStillExists) {
        selectedId.value = previousSelectedId
      } else if (!selectedId.value || !entries.value.some((entry) => entry.id === selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      await loadSelectedImage()
      await refreshSearch()
    } finally {
      liveRefreshInFlight = false
      if (liveRefreshQueued) {
        liveRefreshQueued = false
        queueLiveRefresh(80)
      }
    }
  }

  async function setQuery(value: string) {
    query.value = value
    searchResults.value = value.trim() ? await searchWorkMemory(value) : []
  }

  async function loadFlowConversations() {
    const conversations = await getWorkMemoryFlowConversations()
    flowConversations.value = conversations
    if (activeFlowConversationId.value && conversations.some((conversation) => conversation.id === activeFlowConversationId.value)) {
      flowMessages.value = await getWorkMemoryFlowMessages(activeFlowConversationId.value)
      return
    }
    activeFlowConversationId.value = conversations[0]?.id ?? ''
    flowMessages.value = activeFlowConversationId.value ? await getWorkMemoryFlowMessages(activeFlowConversationId.value) : []
  }

  async function selectFlowConversation(id: string) {
    const normalized = id.trim()
    if (!normalized || normalized === activeFlowConversationId.value) {
      return
    }
    isLoadingFlowConversation.value = true
    try {
      activeFlowConversationId.value = normalized
      flowMessages.value = await getWorkMemoryFlowMessages(normalized)
    } finally {
      isLoadingFlowConversation.value = false
    }
  }

  async function startFlowConversation(title = '') {
    const conversation = await createWorkMemoryFlowConversation(title)
    flowConversations.value = [conversation, ...flowConversations.value.filter((item) => item.id !== conversation.id)]
    activeFlowConversationId.value = conversation.id
    flowMessages.value = []
    flowAskResult.value = null
    return conversation
  }

  async function removeFlowConversation(id: string) {
    const normalized = id.trim()
    if (!normalized) return
    isDeletingFlowConversation.value = true
    try {
      const nextConversations = await deleteWorkMemoryFlowConversation(normalized)
      flowConversations.value = nextConversations
      if (activeFlowConversationId.value === normalized) {
        activeFlowConversationId.value = nextConversations[0]?.id ?? ''
        flowMessages.value = activeFlowConversationId.value ? await getWorkMemoryFlowMessages(activeFlowConversationId.value) : []
        flowAskResult.value = null
      }
      showFeedback('对话记录已删除')
    } catch {
      showFeedback('对话记录删除失败')
    } finally {
      isDeletingFlowConversation.value = false
    }
  }

  async function saveSelfAssertion(request: WorkMemorySelfAssertionRequest) {
    isSavingSelfAssertion.value = true
    try {
      selfModel.value = await upsertWorkMemorySelfAssertion(request)
      showFeedback('我模型已更新')
      return true
    } catch {
      showFeedback('我模型保存失败')
      return false
    } finally {
      isSavingSelfAssertion.value = false
    }
  }

  async function removeSelfAssertion(assertion: WorkMemorySelfAssertion) {
    if (!assertion.id) return
    isSavingSelfAssertion.value = true
    try {
      selfModel.value = await deleteWorkMemorySelfAssertion(assertion.id)
      showFeedback('我模型已更新')
    } catch {
      showFeedback('我模型更新失败')
    } finally {
      isSavingSelfAssertion.value = false
    }
  }

  async function saveTodo(request: WorkMemoryTodoRequest) {
    isSavingTodo.value = true
    try {
      todoList.value = await upsertWorkMemoryTodo(request)
      showFeedback('待办已保存')
      return true
    } catch {
      showFeedback('待办保存失败')
      return false
    } finally {
      isSavingTodo.value = false
    }
  }

  async function changeTodo(request: WorkMemoryTodoUpdateRequest) {
    if (!request.id) return false
    isSavingTodo.value = true
    try {
      todoList.value = await updateWorkMemoryTodo(request)
      showFeedback('待办已更新')
      return true
    } catch {
      showFeedback('待办更新失败')
      return false
    } finally {
      isSavingTodo.value = false
    }
  }

  async function removeTodo(todo: WorkMemoryTodoItem) {
    if (!todo.id) return
    isSavingTodo.value = true
    try {
      todoList.value = await deleteWorkMemoryTodo(todo.id)
      showFeedback('待办已删除')
    } catch {
      showFeedback('待办删除失败')
    } finally {
      isSavingTodo.value = false
    }
  }

  async function refreshTodos(options: { silent?: boolean } = {}) {
    try {
      todoList.value = await listWorkMemoryTodos({ includeDone: true, limit: 300 })
      if (!options.silent) {
        showFeedback('待办已刷新')
      }
      return true
    } catch {
      if (!options.silent) {
        showFeedback('待办刷新失败')
      }
      return false
    }
  }

  async function askFlow(question: string, limit = 8) {
    const normalized = question.trim()
    if (!normalized) {
      flowAskResult.value = await askWorkMemoryFlow({ question: '', limit })
      return flowAskResult.value
    }
    isAskingFlow.value = true
    try {
      const turn = await askWorkMemoryFlowConversation({ conversationId: activeFlowConversationId.value, question: normalized, limit })
      const result = turn.response
      flowAskResult.value = result
      activeFlowConversationId.value = turn.conversation.id
      flowConversations.value = [turn.conversation, ...flowConversations.value.filter((item) => item.id !== turn.conversation.id)]
      flowMessages.value = turn.messages
      if (result.evidence.length) {
        const evidenceIds = new Set(result.evidence.map((item) => item.id))
        searchResults.value = entries.value
          .filter((entry) => evidenceIds.has(entry.id))
          .map((entry) => ({
            id: entry.id,
            type: 'memory',
            title: entry.title,
            subtitle: `心流留痕 · ${entry.appName || entry.source}`,
            detail: entry.summary,
            icon: 'memory',
            tags: entry.tags,
            payload: { entryId: entry.id, source: entry.source },
            preview: {
              kind: 'memory',
              title: entry.title,
              subtitle: entry.windowTitle,
              text: [entry.text, entry.ocrText ? `OCR:\n${entry.ocrText}` : ''].filter(Boolean).join('\n\n'),
              evidence: [
                { label: '记忆 ID', value: entry.id },
                { label: '来源', value: entry.source },
              ],
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
      await refreshTodos({ silent: true })
      showFeedback(result.message || (result.ok ? '心流回答已更新' : '心流回答失败'))
      return result
    } catch {
      showFeedback('心流问答失败')
      const failed: WorkMemoryFlowAskResponse = {
        ok: false,
        question: normalized,
        title: normalized || '心流回答',
        answer: '心流问答暂时不可用。你可以稍后重试，或先切到时间线查看原始留痕。',
        intent: 'search',
        mode: 'error',
        evidence: [],
        suggestedQuestions: ['我今天干了些什么？', '今天有哪些人找过我？', '今天我的哪些工作流可以优化？'],
        usedAi: false,
        message: '心流问答失败',
        createdAt: Math.floor(Date.now() / 1000),
      }
      flowAskResult.value = failed
      return failed
    } finally {
      isAskingFlow.value = false
    }
  }

  function select(id: string) {
    selectedId.value = id
    deleteArmedId.value = ''
    ocrResult.value = null
    ocrSelection.clearOCRLineSelection()
    const nextPlaybackIndex = playbackEntries.value.findIndex((entry) => entry.id === id)
    playbackIndex.value = nextPlaybackIndex
    if (nextPlaybackIndex >= 0) {
      void loadPlaybackImage(playbackEntries.value[nextPlaybackIndex])
    } else {
      playbackImageUrl.value = ''
      playbackImageMissing.value = false
    }
  }

  function isRetrospectiveSelected(id: string) {
    return retrospectiveSelectedIdSet.value.has(id)
  }

  function toggleRetrospectiveSelection(id: string) {
    if (!id) {
      return
    }
    const entry = entries.value.find((item) => item.id === id)
    if (entry?.sensitive) {
      showFeedback('敏感记忆不会加入复盘留痕')
      return
    }
    const selected = retrospectiveSelectedIdSet.value
    if (selected.has(id)) {
      retrospectiveSelectedIds.value = retrospectiveSelectedIds.value.filter((entryId) => entryId !== id)
      return
    }
    if (retrospectiveSelectionCount.value >= 12) {
      showFeedback('一次复盘最多选择 12 条留痕')
      return
    }
    retrospectiveSelectedIds.value = [...retrospectiveSelectedIds.value, id]
  }

  function clearRetrospectiveSelection() {
    retrospectiveSelectedIds.value = []
  }

  function selectVisibleForRetrospective() {
    const selectable = filteredEntries.value.filter((entry) => !entry.sensitive)
    const ids = selectable.map((entry) => entry.id).slice(0, 12)
    if (!ids.length) {
      showFeedback('当前筛选没有非敏感可选记忆')
      return
    }
    retrospectiveSelectedIds.value = ids
    showFeedback(selectable.length > ids.length ? '已选择前 12 条非敏感记忆' : `已选择 ${ids.length} 条非敏感记忆`)
  }

  function selectEntriesForRetrospective(ids: string[]) {
    const requested = new Set(ids.filter(Boolean))
    const selectable = entries.value.filter((entry) => requested.has(entry.id) && !entry.sensitive)
    const nextIds = selectable.map((entry) => entry.id).slice(0, 12)
    if (!nextIds.length) {
      showFeedback('当前选择没有非敏感可复盘记忆')
      return
    }
    retrospectiveSelectedIds.value = nextIds
    showFeedback(selectable.length > nextIds.length ? '已选择前 12 条非敏感记忆' : `已选择 ${nextIds.length} 条非敏感记忆`)
  }

  async function addNote() {
    const text = noteDraft.value.text.trim()
    if (!text) {
      showFeedback('先输入笔记内容')
      return
    }
    try {
      const entry = await addWorkMemoryNote({
        title: noteDraft.value.title.trim(),
        text,
        tags: splitTags(noteDraft.value.tags),
        favorite: noteDraft.value.favorite,
        sensitive: noteDraft.value.sensitive,
      })
      if (!entry.id) {
        status.value = await getWorkMemoryStatus()
        showFeedback(status.value.pauseReason || '工作记忆当前已暂停')
        return
      }
      entries.value = [entry, ...entries.value.filter((item) => item.id !== entry.id)]
      selectedId.value = entry.id
      noteDraft.value = { title: '', text: '', tags: '', favorite: false, sensitive: false }
      status.value = await getWorkMemoryStatus()
      await refreshSearch()
      showFeedback('笔记已加入工作记忆')
    } catch {
      showFeedback('笔记保存失败')
    }
  }

  async function addManualNote(request: WorkMemoryNoteRequest) {
    const text = request.text.trim()
    if (!text) {
      showFeedback('先选择要沉淀的内容')
      return null
    }
    try {
      const entry = await addWorkMemoryNote({
        ...request,
        text,
        tags: request.tags ?? [],
        favorite: Boolean(request.favorite),
        sensitive: Boolean(request.sensitive),
      })
      if (!entry.id) {
        status.value = await getWorkMemoryStatus()
        showFeedback(status.value.pauseReason || '心流沉淀当前已暂停')
        return null
      }
      entries.value = [entry, ...entries.value.filter((item) => item.id !== entry.id)]
      selectedId.value = entry.id
      status.value = await getWorkMemoryStatus()
      await refreshSearch()
      showFeedback('已加入沉淀')
      return entry
    } catch {
      showFeedback('沉淀保存失败')
      return null
    }
  }

  async function toggleTimeMachine() {
    try {
      const nextEnabled = !status.value.timeMachineEnabled
      await persistWorkMemorySettings({ timeMachineEnabled: nextEnabled })
      status.value = await setTimeMachineEnabled(nextEnabled)
      if (status.value.timeMachineEnabled) {
        const entry = await captureTimeMachineNow()
        if (entry.id) {
          entries.value = [entry, ...entries.value.filter((item) => item.id !== entry.id)]
          selectedId.value = entry.id
          await loadSelectedImage()
          status.value = await getWorkMemoryStatus()
          await refreshSearch()
        }
      }
      showFeedback(status.value.timeMachineEnabled ? '时间机器已开启并记录当前屏幕' : '时间机器已暂停')
    } catch {
      showFeedback('时间机器状态更新失败')
    }
  }

  async function togglePrivacyMode() {
    try {
      const nextEnabled = !status.value.privacyMode
      await persistWorkMemorySettings({
        privacyMode: nextEnabled,
        ...(nextEnabled ? { timeMachineEnabled: false } : {}),
      })
      status.value = await setWorkMemoryPrivacyMode(nextEnabled)
      showFeedback(status.value.privacyMode ? '隐私模式已开启' : '隐私模式已关闭')
    } catch {
      showFeedback('隐私模式更新失败')
    }
  }

  async function enableProactiveSinking() {
    try {
      const settings = useSettingsStore()
      if (!settings.settings) {
        await settings.load()
      }
      await settings.updateWorkMemoryRuntime({
        enabled: true,
        privacyMode: false,
        sourceClipboard: true,
        sourceCaptureHistory: true,
        sourceSearchFavorite: true,
      })
      status.value = await getWorkMemoryStatus()
      showFeedback('主动沉淀已开启：剪贴板和截图历史会自动进入工作记忆')
    } catch {
      showFeedback('主动沉淀开启失败')
    }
  }

  async function captureNow() {
    try {
      const entry = await captureCurrentScreen()
      if (!entry.id) {
        status.value = await getWorkMemoryStatus()
        showFeedback(status.value.pauseReason || '补记已暂停')
        return
      }
      entries.value = [entry, ...entries.value.filter((item) => item.id !== entry.id)]
      selectedId.value = entry.id
      status.value = await getWorkMemoryStatus()
      await loadSelectedImage()
      await refreshSearch()
      showFeedback('已补记当前屏幕')
    } catch {
      showFeedback('补记失败')
    }
  }

  async function buildDailyDraft() {
    try {
      dailyDraft.value = await generateDailyDraft()
      dailyDraftPolishResult.value = null
      dailyDraftPolishArmed.value = false
      showFeedback('日报草稿已生成')
    } catch {
      showFeedback('日报草稿生成失败')
    }
  }

  async function polishDailyDraft() {
    const draft = dailyDraft.value
    if (!draft) {
      showFeedback('先生成日报草稿')
      return
    }
    if (!dailyDraftPolishArmed.value) {
      dailyDraftPolishArmed.value = true
      try {
        dailyDraftPolishResult.value = await polishWorkMemoryDraft({ draft, kind: 'daily', confirmed: false })
        showFeedback('AI 润色需要二次确认')
      } catch {
        dailyDraftPolishResult.value = null
        dailyDraftPolishArmed.value = false
        showFeedback('AI 润色预检失败')
      }
      return
    }
    isPolishingDailyDraft.value = true
    try {
      const result = await polishWorkMemoryDraft({ draft, kind: 'daily', confirmed: true })
      dailyDraftPolishResult.value = result
      dailyDraftPolishArmed.value = false
      if (result.ok && result.polishedDraft?.id) {
        dailyDraft.value = result.polishedDraft
      }
      showFeedback(result.message || (result.ok ? 'AI 润色草稿已生成' : 'AI 润色失败'))
    } catch {
      showFeedback('AI 润色失败')
    } finally {
      isPolishingDailyDraft.value = false
    }
  }

  async function runScheduledDrafts() {
    isRunningScheduledDrafts.value = true
    try {
      const result = await runScheduledDraftsNow()
      scheduledDraftStatus.value = result
      if (result.dailyDraft?.id) {
        dailyDraft.value = result.dailyDraft
      }
      if (result.retrospectiveDraft?.id) {
        retrospectiveDraft.value = result.retrospectiveDraft
      }
      if (result.experienceReport?.id) {
        experienceReport.value = result.experienceReport
      }
      showFeedback(result.lastError || `定期草稿已运行 · ${result.lastEntryCount} 条留痕`)
    } catch {
      showFeedback('定期草稿运行失败')
    } finally {
      isRunningScheduledDrafts.value = false
    }
  }

  async function runAutonomousFlow() {
    isRunningAutonomousFlow.value = true
    try {
      const result = await runAutonomousFlowNow()
      autonomousRunResult.value = result
      scheduledDraftStatus.value = result.status
      autonomousArtifacts.value = await getAutonomousArtifacts()
      health.value = await getWorkMemoryHealth()
      showFeedback(result.message || (result.generated ? `心流生成 ${result.generated} 个自主产物` : '心流暂未生成新产物'))
      return result
    } catch {
      showFeedback('自主整理运行失败')
      return null
    } finally {
      isRunningAutonomousFlow.value = false
    }
  }

  async function refreshFlowCandidateActions() {
    try {
      const [nextStatus, nextActions] = await Promise.all([getFlowAutonomyStatus(), listFlowCandidateActions({ limit: 20 })])
      flowAutonomyStatus.value = nextStatus
      flowCandidateActions.value = nextActions
      return nextActions
    } catch {
      showFeedback('主动动作刷新失败')
      return null
    }
  }

  async function runFlowAutonomy() {
    isRunningFlowAutonomy.value = true
    try {
      const result = await runFlowAutonomyNow()
      flowAutonomyRunResult.value = result
      flowAutonomyStatus.value = result.status
      flowCandidateActions.value = await listFlowCandidateActions({ limit: 20 })
      showFeedback(result.message || (result.generated ? `已生成 ${result.generated} 个待确认动作` : '暂无待确认动作'))
      return result
    } catch {
      showFeedback('主动动作扫描失败')
      return null
    } finally {
      isRunningFlowAutonomy.value = false
    }
  }

  async function handleFlowCandidateAction(action: FlowCandidateAction, notificationAction: FlowNotificationAction) {
    const actionId = notificationAction.id
    if (actionId === 'open' || actionId === 'view' || actionId === 'open_flow') {
      const entryId = action.evidence[0] || action.payload?.entryId || ''
      if (entryId) {
        selectedId.value = entryId
        await loadSelectedImage()
      }
      showFeedback(entryId ? '已打开关联留痕' : '已停留在心流')
      return null
    }
    if (actionId === 'copy' || actionId === 'copy_revision') {
      const text = action.payload?.copyText || action.body || action.summary || action.title
      if (text) {
        await Clipboard.SetText(text)
      }
    }
    isDecidingFlowCandidate.value = true
    try {
      const result = await decideFlowCandidateAction({
        id: action.id,
        actionId,
        snoozeMinutes: flowAutonomyStatus.value?.defaultSnoozeMinutes ?? 30,
      })
      flowCandidateDecisionResult.value = result
      flowCandidateActions.value = result.list
      flowAutonomyStatus.value = await getFlowAutonomyStatus()
      if (actionId === 'add' && result.ok) {
        todoList.value = await listWorkMemoryTodos({ includeDone: true, limit: 300 })
      }
      showFeedback(result.message || (result.ok ? '主动动作已处理' : '主动动作处理失败'))
      return result
    } catch {
      showFeedback('主动动作处理失败')
      return null
    } finally {
      isDecidingFlowCandidate.value = false
    }
  }

  async function rejectAutonomousArtifact(id: string, reason: string) {
    try {
      const result = await rejectAutonomousArtifactApi({ id, reason })
      scheduledDraftStatus.value = result.status
      if (result.ok) {
        autonomousArtifacts.value = autonomousArtifacts.value.filter((item) => item.id !== id)
      } else {
        autonomousArtifacts.value = await getAutonomousArtifacts()
      }
      health.value = await getWorkMemoryHealth()
      showFeedback(result.message || (result.ok ? '已删除自主产物' : '自主产物删除失败'))
      return result
    } catch {
      showFeedback('自主产物删除失败')
      return null
    }
  }

  async function buildRetrospectiveDraft() {
    const entryIds = retrospectiveDraftEntryIds.value
    if (!entryIds.length) {
      showFeedback('先选择要复盘的记忆')
      return
    }
    try {
      const draft = await generateRetrospectiveDraft(entryIds)
      retrospectiveDraft.value = draft
      showFeedback(`复盘草稿已生成 · ${draft.evidence.length} 条留痕`)
    } catch {
      showFeedback('复盘草稿生成失败')
    }
  }

  async function buildKnowledgeDraft() {
    const entry = selectedEntry.value
    if (!entry) {
      return
    }
    try {
      knowledgeDraft.value = await generateKnowledgeDraft([entry.id])
      knowledgeDraftSaveResult.value = null
      knowledgeSkillExportResult.value = null
      knowledgeSkillInstallResult.value = null
      knowledgeSkillInstallDiagnostics.value = null
      knowledgeDraftSaveArmed.value = false
      knowledgeSkillExportArmed.value = false
      knowledgeSkillInstallArmed.value = false
      showFeedback('知识草稿已生成')
    } catch {
      showFeedback('知识草稿生成失败')
    }
  }

  async function saveCurrentKnowledgeDraft() {
    const draft = knowledgeDraft.value
    if (!draft) {
      showFeedback('先生成知识草稿')
      return
    }
    if (!knowledgeDraftSaveArmed.value) {
      knowledgeDraftSaveArmed.value = true
      try {
        knowledgeDraftSaveResult.value = await saveSkillDraft({ draft, confirmed: false })
      } catch {
        knowledgeDraftSaveResult.value = null
      }
      showFeedback('再次点击保存为正式 Skill')
      return
    }
    isSavingKnowledgeDraft.value = true
    try {
      const result = await saveSkillDraft({ draft, confirmed: true })
      knowledgeDraftSaveResult.value = result
      knowledgeSkillExportResult.value = null
      knowledgeSkillInstallResult.value = null
      knowledgeSkillInstallDiagnostics.value = null
      knowledgeDraftSaveArmed.value = false
      knowledgeSkillExportArmed.value = false
      knowledgeSkillInstallArmed.value = false
      showFeedback(result.ok ? `已保存为 Skill: ${result.skill.title}` : result.message)
    } catch {
      showFeedback('Skill 保存失败')
    } finally {
      isSavingKnowledgeDraft.value = false
    }
  }

  async function exportCurrentKnowledgeSkill() {
    const skill = knowledgeDraftSaveResult.value?.skill
    if (!knowledgeDraftSaveResult.value?.ok || !skill?.id) {
      showFeedback('先保存为本地 Skill')
      return
    }
    if (!knowledgeSkillExportArmed.value) {
      knowledgeSkillExportArmed.value = true
      try {
        knowledgeSkillExportResult.value = await exportSkillPackage({ skillId: skill.id, confirmed: false })
      } catch {
        knowledgeSkillExportResult.value = null
      }
      showFeedback('再次点击导出 Codex Skill 包')
      return
    }
    isExportingKnowledgeSkill.value = true
    try {
      const result = await exportSkillPackage({ skillId: skill.id, confirmed: true })
      knowledgeSkillExportResult.value = result
      knowledgeSkillExportArmed.value = false
      showFeedback(result.ok ? 'Codex Skill 包已导出' : result.message)
    } catch {
      showFeedback('Skill 包导出失败')
    } finally {
      isExportingKnowledgeSkill.value = false
    }
  }

  async function installCurrentKnowledgeSkill() {
    const skill = knowledgeDraftSaveResult.value?.skill
    if (!knowledgeDraftSaveResult.value?.ok || !skill?.id) {
      showFeedback('先保存为本地 Skill')
      return
    }
    if (!knowledgeSkillInstallArmed.value) {
      knowledgeSkillInstallArmed.value = true
      knowledgeSkillInstallDiagnostics.value = null
      try {
        knowledgeSkillInstallResult.value = await installSkillPackage({
          skillId: skill.id,
          confirmed: false,
          overwrite: true,
        })
      } catch {
        knowledgeSkillInstallResult.value = null
      }
      showFeedback('再次点击安装到 Codex skills')
      return
    }
    isInstallingKnowledgeSkill.value = true
    try {
      const result = await installSkillPackage({ skillId: skill.id, confirmed: true, overwrite: true })
      knowledgeSkillInstallResult.value = result
      knowledgeSkillInstallArmed.value = false
      if (result.ok) {
        try {
          knowledgeSkillInstallDiagnostics.value = await getSkillInstallDiagnostics({
            skillId: skill.id,
            targetRoot: result.targetRoot,
          })
        } catch {
          knowledgeSkillInstallDiagnostics.value = null
        }
      } else {
        knowledgeSkillInstallDiagnostics.value = null
      }
      showFeedback(
        result.ok
          ? knowledgeSkillInstallDiagnostics.value?.ok
            ? '已安装到 Codex skills，并通过本地发现核验'
            : '已安装到 Codex skills，等待刷新握手核验'
          : result.message,
      )
    } catch {
      showFeedback('Skill 安装失败')
    } finally {
      isInstallingKnowledgeSkill.value = false
    }
  }

  async function buildAgentTask() {
    const entry = selectedEntry.value
    if (!entry) {
      return false
    }
    try {
      agentTask.value = await generateAgentTaskPackage(`沉淀 ${entry.title} 的可复用能力`, [entry.id])
      showFeedback('外部代理任务包已生成')
      return true
    } catch {
      showFeedback('任务包生成失败')
      return false
    }
  }

  async function discoverExperienceReport() {
    try {
      const periodDays = experiencePeriodDays()
      experienceReport.value = await discoverExperiences(periodDays)
      experienceDiscoveryResult.value = null
      experienceDiscoveryArmed.value = false
      const count = experienceReport.value.insights.length
      showFeedback(count ? `发现 ${count} 条经验线索` : '暂未发现稳定经验线索')
    } catch {
      showFeedback('经验发现失败')
    }
  }

  async function discoverExperienceReportAI() {
    const periodDays = experiencePeriodDays()
    if (!experienceDiscoveryArmed.value) {
      experienceDiscoveryArmed.value = true
      try {
        experienceDiscoveryResult.value = await discoverExperiencesAI({ periodDays, external: true, confirmed: false })
        showFeedback(experienceDiscoveryResult.value.message || 'AI 经验发现需要二次确认')
      } catch {
        experienceDiscoveryResult.value = null
        experienceDiscoveryArmed.value = false
        showFeedback('AI 经验发现预检失败')
      }
      return
    }
    isDiscoveringExperienceAI.value = true
    startExperienceProgress('准备非敏感留痕')
    try {
      experienceDiscoveryStage.value = 'AI 正在归纳线索'
      const result = await discoverExperiencesAI({ periodDays, external: true, confirmed: true })
      completeExperienceProgress('整理归纳结果')
      experienceDiscoveryResult.value = result
      experienceDiscoveryArmed.value = false
      if (result.report?.id) {
        experienceReport.value = result.report
      }
      const count = result.report?.insights.length ?? 0
      showFeedback(result.message || (result.ok ? `AI 发现 ${count} 条经验线索` : 'AI 经验发现失败'))
    } catch {
      showFeedback('AI 经验发现失败')
    } finally {
      isDiscoveringExperienceAI.value = false
      stopExperienceProgress()
    }
  }

  async function markExperienceInsight(insight: ExperienceInsight, status: 'accepted' | 'rejected' | 'later') {
    try {
      const result = await setExperienceInsightDecision(insight.id, status)
      if (!result.ok || !result.decision) {
        showFeedback(result.message || '经验线索状态保存失败')
        return
      }
      applyExperienceDecision(result.decision.insightId, result.decision.status, result.decision.updatedAt, result.decision.taskPackageId)
      const labels: Record<string, string> = {
        accepted: '已接受经验线索',
        rejected: '已驳回经验线索',
        later: '已标记稍后处理',
      }
      showFeedback(labels[result.decision.status] || '经验线索状态已保存')
    } catch {
      showFeedback('经验线索状态保存失败')
    }
  }

  async function buildAgentTaskFromInsight(insight: ExperienceInsight) {
    try {
      const task = await generateAgentTaskPackage(insight.recommendation || insight.title, insight.evidence)
      agentTask.value = task
      const result = await setExperienceInsightDecision(insight.id, 'task_package', '', task.id)
      if (result.ok && result.decision) {
        applyExperienceDecision(result.decision.insightId, result.decision.status, result.decision.updatedAt, result.decision.taskPackageId)
        showFeedback('已生成外部代理任务包')
      } else {
        showFeedback('任务包已生成，状态保存失败')
      }
      return true
    } catch {
      showFeedback('任务包生成失败')
      return false
    }
  }

  async function buildWorkflowDraftFromInsight(insight: ExperienceInsight) {
    try {
      workflowDraft.value = await generateWorkflowDraft(insight.recommendation || insight.title, insight.evidence)
      workflowDraftSaveResult.value = null
      workflowDraftSaveArmed.value = false
      const result = await setExperienceInsightDecision(insight.id, 'workflow_draft', '', workflowDraft.value.id)
      if (result.ok && result.decision) {
        applyExperienceDecision(result.decision.insightId, result.decision.status, result.decision.updatedAt, result.decision.taskPackageId)
      }
      showFeedback('候选工作流草稿已生成')
      return true
    } catch {
      showFeedback('候选工作流生成失败')
      return false
    }
  }

  async function saveCurrentWorkflowDraft() {
    const draft = workflowDraft.value
    if (!draft) {
      showFeedback('先生成候选工作流草稿')
      return
    }
    if (!workflowDraftSaveArmed.value) {
      workflowDraftSaveArmed.value = true
      try {
        workflowDraftSaveResult.value = await saveWorkflowDraft({ draft, confirmed: false })
      } catch {
        workflowDraftSaveResult.value = null
      }
      showFeedback('再次点击保存为正式工作流')
      return
    }
    isSavingWorkflowDraft.value = true
    try {
      const result = await saveWorkflowDraft({ draft, confirmed: true })
      workflowDraftSaveResult.value = result
      workflowDraftSaveArmed.value = false
      showFeedback(result.ok ? `已保存为工作流: ${result.workflow.name}` : result.message)
    } catch {
      showFeedback('候选工作流保存失败')
    } finally {
      isSavingWorkflowDraft.value = false
    }
  }

  async function buildChecklistDraftFromInsight(insight: ExperienceInsight) {
    try {
      checklistDraft.value = await generateChecklistDraft(insight.recommendation || insight.title, insight.evidence)
      checklistDraftSaveResult.value = null
      checklistDraftSaveArmed.value = false
      const result = await setExperienceInsightDecision(insight.id, 'checklist_draft', '', checklistDraft.value.id)
      if (result.ok && result.decision) {
        applyExperienceDecision(result.decision.insightId, result.decision.status, result.decision.updatedAt, result.decision.taskPackageId)
      }
      showFeedback('检查清单草稿已生成')
      return true
    } catch {
      showFeedback('检查清单生成失败')
      return false
    }
  }

  async function saveCurrentChecklistDraft() {
    const draft = checklistDraft.value
    if (!draft) {
      showFeedback('先生成检查清单草稿')
      return
    }
    if (!checklistDraftSaveArmed.value) {
      checklistDraftSaveArmed.value = true
      try {
        checklistDraftSaveResult.value = await saveChecklistDraft({ draft, confirmed: false })
      } catch {
        checklistDraftSaveResult.value = null
      }
      showFeedback('再次点击保存为正式清单')
      return
    }
    isSavingChecklistDraft.value = true
    try {
      const result = await saveChecklistDraft({ draft, confirmed: true })
      checklistDraftSaveResult.value = result
      checklistDraftSaveArmed.value = false
      showFeedback(result.ok ? `已保存为清单: ${result.checklist.title}` : result.message)
    } catch {
      showFeedback('检查清单保存失败')
    } finally {
      isSavingChecklistDraft.value = false
    }
  }

  function applyExperienceDecision(insightId: string, status: string, updatedAt: number, taskPackageId = '') {
    if (!experienceReport.value) {
      return
    }
    experienceReport.value = {
      ...experienceReport.value,
      insights: experienceReport.value.insights.map((item) =>
        item.id === insightId
          ? {
              ...item,
              decisionStatus: status,
              decisionUpdatedAt: updatedAt,
              taskPackageId,
            }
          : item,
      ),
    }
  }

  async function recognizeSelectedText() {
    const entry = selectedEntry.value
    if (!entry) {
      return
    }
    if (!entry.imagePath) {
      showFeedback('当前记忆没有图片留痕')
      return
    }
    isRecognizingOCR.value = true
    try {
      const result = await recognizeWorkMemoryOCR(entry.id)
      ocrResult.value = result
      ocrSelection.clearOCRLineSelection()
      if (result.workMemory?.id) {
        entries.value = entries.value.map((item) => (item.id === result.workMemory?.id ? result.workMemory : item))
      }
      showFeedback(result.ok ? (result.text ? 'OCR 已写回工作记忆' : '未识别到文字') : result.error || 'OCR 不可用')
      await refreshSearch()
    } catch {
      showFeedback('OCR 识别失败')
    } finally {
      isRecognizingOCR.value = false
    }
  }

  async function recognizeEntriesOCR(ids: string[]) {
    const uniqueIds = [...new Set(ids.filter(Boolean))]
    if (!uniqueIds.length) {
      showFeedback('先选择要 OCR+总结的截图记忆')
      return { processed: 0, failed: 0, skipped: 0, total: 0 }
    }
    const byId = new Map(entries.value.map((entry) => [entry.id, entry]))
    const candidates = uniqueIds
      .map((id) => byId.get(id))
      .filter((entry): entry is WorkMemoryEntry => Boolean(entry))
    const targets = candidates.filter((entry) => Boolean(entry.imagePath) && !entry.sensitive && entry.qualityStatus !== 'pending')
    const skipped = uniqueIds.length - targets.length
    if (!targets.length) {
      showFeedback(skipped ? `没有可处理的图片记忆，已跳过 ${skipped} 条` : '没有可处理的图片记忆')
      return { processed: 0, failed: 0, skipped, total: 0 }
    }
    isBatchRecognizingOCR.value = true
    batchOcrProgressDone.value = 0
    batchOcrProgressTotal.value = targets.length
    batchOcrProgressStage.value = '准备 OCR+总结'
    let processed = 0
    let failed = 0
    try {
      for (const entry of targets) {
        batchOcrProgressStage.value = entry.title || entry.windowTitle || entry.appName || entry.id
        try {
          const result = await recognizeWorkMemoryOCR(entry.id)
          ocrResult.value = result
          if (result.workMemory?.id) {
            entries.value = entries.value.map((item) => (item.id === result.workMemory?.id ? result.workMemory : item))
          }
          if (result.ok) {
            processed += 1
          } else {
            failed += 1
          }
        } catch {
          failed += 1
        } finally {
          batchOcrProgressDone.value = processed + failed
        }
      }
      await refreshSearch()
      const parts = [`已完成 OCR+总结 ${processed} 条`]
      if (failed) parts.push(`失败 ${failed} 条`)
      if (skipped) parts.push(`跳过 ${skipped} 条`)
      showFeedback(parts.join('，'))
      return { processed, failed, skipped, total: targets.length }
    } finally {
      isBatchRecognizingOCR.value = false
      batchOcrProgressStage.value = '完成'
      window.setTimeout(() => {
        if (!isBatchRecognizingOCR.value) {
          batchOcrProgressDone.value = 0
          batchOcrProgressTotal.value = 0
          batchOcrProgressStage.value = ''
        }
      }, 1400)
    }
  }

  async function exportData() {
    try {
      const settings = useSettingsStore()
      if (!settings.settings) {
        await settings.load()
      }
      const includeSensitive = Boolean(settings.settings?.workMemory.allowSensitiveExport)
      const request = buildExportRequest(includeSensitive)
      const hasFilter = Boolean(request.startAt || request.endAt || request.tags?.length || request.entryIds?.length)
      const result = hasFilter ? await exportWorkMemoryDataWithOptions(request) : await exportWorkMemoryData(includeSensitive)
      exportResult.value = result
      showFeedback(result.ok ? `已导出 ${result.entryCount} 条记忆` : result.message)
    } catch {
      showFeedback('导出失败')
    }
  }

  async function importMaterials() {
    const paths = splitImportPaths(importDraft.value.paths)
    if (!paths.length) {
      showFeedback('先粘贴要导入的文件路径')
      return
    }
    isImportingMaterials.value = true
    try {
      const result = await importWorkMemoryMaterials({
        paths,
        tags: splitTags(importDraft.value.tags),
        favorite: importDraft.value.favorite,
        sensitive: importDraft.value.sensitive,
      })
      importResult.value = result
      if (result.entries?.length) {
        entries.value = [...result.entries, ...entries.value.filter((entry) => !result.entries.some((imported) => imported.id === entry.id))]
        pruneRetrospectiveSelection()
        selectedId.value = result.entries[0]?.id ?? selectedId.value
        await loadSelectedImage()
      }
      status.value = await getWorkMemoryStatus()
      await refreshSearch()
      showFeedback(result.ok ? `已导入 ${result.imported} 条材料` : result.message)
    } catch {
      showFeedback('导入失败')
    } finally {
      isImportingMaterials.value = false
    }
  }

  async function loadExclusionRules() {
    try {
      const settings = useSettingsStore()
      if (!settings.settings) {
        await settings.load()
      }
      const memorySettings = settings.settings?.workMemory
      if (!memorySettings) {
        return
      }
      exclusionDraft.value = {
        apps: listToText(memorySettings.excludeApps),
        windowKeywords: listToText(memorySettings.excludeWindowKeywords),
        paths: listToText(memorySettings.excludePaths),
        urls: listToText(memorySettings.excludeUrls),
        contentPatterns: listToText(memorySettings.excludeContentPatterns),
      }
    } catch {
      showFeedback('排除规则加载失败')
    }
  }

  async function saveExclusionRules() {
    isSavingExclusions.value = true
    try {
      const settings = useSettingsStore()
      const updated = await settings.updateWorkMemoryRuntime({
        excludeApps: splitRuleLines(exclusionDraft.value.apps),
        excludeWindowKeywords: splitRuleLines(exclusionDraft.value.windowKeywords),
        excludePaths: splitRuleLines(exclusionDraft.value.paths),
        excludeUrls: splitRuleLines(exclusionDraft.value.urls),
        excludeContentPatterns: splitRuleLines(exclusionDraft.value.contentPatterns),
      })
      if (updated) {
        exclusionDraft.value = {
          apps: listToText(updated.excludeApps),
          windowKeywords: listToText(updated.excludeWindowKeywords),
          paths: listToText(updated.excludePaths),
          urls: listToText(updated.excludeUrls),
          contentPatterns: listToText(updated.excludeContentPatterns),
        }
      }
      status.value = await getWorkMemoryStatus()
      showFeedback('排除规则已保存并生效')
    } catch {
      showFeedback('排除规则保存失败')
    } finally {
      isSavingExclusions.value = false
    }
  }

  async function copyOCRText() {
    const text = ocrResult.value?.text || selectedEntry.value?.ocrText
    if (!text) {
      showFeedback('没有可复制的 OCR 文本')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('OCR 文本已复制')
    } catch {
      showFeedback('复制失败')
    }
  }

  async function copySelectedOCRText() {
    const text = ocrSelection.selectedOCRText.value
    if (!text) {
      showFeedback('先选择 OCR 行')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('已复制选中文字')
    } catch {
      showFeedback('复制失败')
    }
  }

  async function deleteSelected() {
    const entry = selectedEntry.value
    if (!entry) {
      return
    }
    if (deleteArmedId.value !== entry.id) {
      deleteArmedId.value = entry.id
      showFeedback('再次点击删除当前记忆')
      return
    }
    try {
      status.value = await deleteWorkMemoryEntry(entry.id)
      entries.value = entries.value.filter((item) => item.id !== entry.id)
      pruneRetrospectiveSelection()
      selectedId.value = entries.value[0]?.id ?? ''
      deleteArmedId.value = ''
      await loadSelectedImage()
      await refreshSearch()
      showFeedback('记忆已删除')
    } catch {
      showFeedback('删除失败')
    }
  }

  async function deleteEntries(ids: string[]) {
    const uniqueIds = [...new Set(ids.filter(Boolean))]
    if (!uniqueIds.length) {
      showFeedback('先选择要删除的记忆')
      return 0
    }
    const removedIds: string[] = []
    isDeletingEntries.value = true
    deleteProgressDone.value = 0
    deleteProgressTotal.value = uniqueIds.length
    try {
      for (const id of uniqueIds) {
        status.value = await deleteWorkMemoryEntry(id)
        removedIds.push(id)
        deleteProgressDone.value = removedIds.length
      }
      const removed = new Set(removedIds)
      entries.value = entries.value.filter((entry) => !removed.has(entry.id))
      pruneRetrospectiveSelection()
      if (!selectedId.value || removed.has(selectedId.value)) {
        selectedId.value = entries.value[0]?.id ?? ''
      }
      deleteArmedId.value = ''
      await loadSelectedImage()
      await refreshSearch()
      showFeedback(`已删除 ${removedIds.length} 条记忆`)
      return removedIds.length
    } catch {
      if (removedIds.length) {
        const removed = new Set(removedIds)
        entries.value = entries.value.filter((entry) => !removed.has(entry.id))
        deleteProgressDone.value = removedIds.length
        pruneRetrospectiveSelection()
        if (!selectedId.value || removed.has(selectedId.value)) {
          selectedId.value = entries.value[0]?.id ?? ''
        }
        await loadSelectedImage()
        await refreshSearch()
        showFeedback(`已删除 ${removedIds.length} 条，后续删除失败`)
        return removedIds.length
      }
      showFeedback('批量删除失败')
      return 0
    } finally {
      isDeletingEntries.value = false
      window.setTimeout(() => {
        if (!isDeletingEntries.value) {
          deleteProgressDone.value = 0
          deleteProgressTotal.value = 0
        }
      }, 1200)
    }
  }

  async function clearUnpinned() {
    if (!clearUnpinnedArmed.value) {
      clearUnpinnedArmed.value = true
      showFeedback('再次点击清理未收藏记忆')
      return
    }
    try {
      status.value = await clearUnpinnedWorkMemory()
      entries.value = entries.value.filter((entry) => entry.favorite)
      pruneRetrospectiveSelection()
      selectedId.value = entries.value[0]?.id ?? ''
      clearUnpinnedArmed.value = false
      await loadSelectedImage()
      await refreshSearch()
      showFeedback('未收藏记忆已清理')
    } catch {
      showFeedback('清理失败')
    }
  }

  async function startPlayback() {
    if (!playbackEntries.value.length) {
      playbackIndex.value = -1
      playbackImageUrl.value = ''
      playbackImageMissing.value = false
      showFeedback('还没有可回放的截图记忆')
      return
    }
    const selectedPlaybackIndex = playbackEntries.value.findIndex((entry) => entry.id === selectedId.value)
    await selectPlayback(selectedPlaybackIndex >= 0 ? selectedPlaybackIndex : playbackEntries.value.length - 1)
    showFeedback('时间机器回放已定位')
  }

  async function stepPlayback(delta: number) {
    if (!playbackEntries.value.length) {
      showFeedback('还没有可回放的截图记忆')
      return
    }
    const current = playbackIndex.value >= 0 ? playbackIndex.value : playbackEntries.value.length - 1
    const next = Math.min(Math.max(current + delta, 0), playbackEntries.value.length - 1)
    await selectPlayback(next)
  }

  async function selectPlayback(index: number) {
    const entry = playbackEntries.value[index]
    if (!entry) {
      playbackIndex.value = -1
      playbackImageUrl.value = ''
      playbackImageMissing.value = false
      return
    }
    playbackIndex.value = index
    selectedId.value = entry.id
    deleteArmedId.value = ''
    ocrResult.value = null
    ocrSelection.clearOCRLineSelection()
    await Promise.all([loadSelectedImage(), loadPlaybackImage(entry)])
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 1600)
  }

  function startExperienceProgress(stage: string) {
    stopExperienceProgress()
    experienceDiscoveryStage.value = stage
    experienceDiscoveryProgress.value = 8
    experienceProgressTimer = window.setInterval(() => {
      if (!isDiscoveringExperienceAI.value) return
      const current = experienceDiscoveryProgress.value
      if (current < 38) {
        experienceDiscoveryProgress.value = current + 6
      } else if (current < 72) {
        experienceDiscoveryProgress.value = current + 3
      } else if (current < 88) {
        experienceDiscoveryProgress.value = current + 1
      }
    }, 700)
  }

  function completeExperienceProgress(stage: string) {
    experienceDiscoveryStage.value = stage
    experienceDiscoveryProgress.value = 100
  }

  function stopExperienceProgress() {
    if (experienceProgressTimer !== null) {
      window.clearInterval(experienceProgressTimer)
      experienceProgressTimer = null
    }
    if (experienceDiscoveryProgress.value >= 100) {
      window.setTimeout(() => {
        if (!isDiscoveringExperienceAI.value) {
          experienceDiscoveryProgress.value = 0
          experienceDiscoveryStage.value = ''
        }
      }, 1000)
    } else if (!isDiscoveringExperienceAI.value) {
      experienceDiscoveryProgress.value = 0
      experienceDiscoveryStage.value = ''
    }
  }

  async function loadSelectedImage() {
    const captureId = selectedEntry.value?.captureId
    if (!captureId) {
      selectedImageUrl.value = ''
      selectedImageMissing.value = false
      return
    }
    const selectedAtRequest = selectedEntry.value?.id ?? ''
    const url = await getCaptureImageDataURL(captureId)
    if (selectedEntry.value?.id !== selectedAtRequest) {
      return
    }
    selectedImageUrl.value = url
    selectedImageMissing.value = !url
  }

  async function loadPlaybackImage(entry = playbackEntry.value) {
    if (!entry?.captureId) {
      playbackImageUrl.value = ''
      playbackImageMissing.value = false
      return
    }
    isLoadingPlaybackImage.value = true
    try {
      const url = await getCaptureImageDataURL(entry.captureId)
      if (playbackEntry.value?.id !== entry.id) {
        return
      }
      playbackImageUrl.value = url
      playbackImageMissing.value = !url
    } finally {
      isLoadingPlaybackImage.value = false
    }
  }

  async function refreshSearch() {
    if (query.value.trim()) {
      searchResults.value = await searchWorkMemory(query.value)
    }
  }

  async function refreshEmbedding() {
    isRefreshingEmbedding.value = true
    try {
      const result = await refreshEmbeddingIndex()
      embeddingRefreshResult.value = result
      semanticStatus.value = result.status
      showFeedback(result.message || (result.ok ? 'embedding 索引已刷新' : 'embedding 刷新失败'))
    } catch {
      showFeedback('embedding 刷新失败')
    } finally {
      isRefreshingEmbedding.value = false
    }
  }

  async function runSemanticSearch() {
    const nextQuery = semanticDraft.value.query.trim() || query.value.trim()
    if (!nextQuery) {
      showFeedback('请输入语义搜索关键词')
      return
    }
    semanticDraft.value.query = nextQuery
    isSemanticSearching.value = true
    try {
      const result = await semanticSearchExternal(nextQuery)
      semanticSearchResult.value = result
      semanticStatus.value = result.status
      if (result.ok) {
        searchResults.value = result.results
      }
      showFeedback(result.message || (result.ok ? '语义搜索完成' : '语义搜索失败'))
    } catch {
      showFeedback('语义搜索失败')
    } finally {
      isSemanticSearching.value = false
    }
  }

  function splitTags(value: string) {
    return value
      .split(/[,\s，、]+/)
      .map((tag) => tag.trim())
      .filter(Boolean)
  }

  function splitImportPaths(value: string) {
    return value
      .split(/\r?\n|;/)
      .map((path) => path.trim().replace(/^["']|["']$/g, ''))
      .filter(Boolean)
  }

  function splitEntryIds(value: string) {
    return value
      .split(/[,\s，、;；]+/)
      .map((id) => id.trim())
      .filter(Boolean)
  }

  function splitRuleLines(value: string) {
    const seen = new Set<string>()
    return value
      .split(/\r?\n|,/)
      .map((rule) => rule.trim())
      .filter((rule) => {
        if (!rule) return false
        const key = rule.toLowerCase()
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
  }

  function listToText(items?: string[]) {
    return (items ?? []).join('\n')
  }

  function isPlaybackEntry(entry: WorkMemoryEntry) {
    return Boolean(entry.captureId && (entry.source === 'time_machine' || entry.source === 'manual_capture' || entry.source === 'screenshot'))
  }

  function experiencePeriodDays() {
    return useSettingsStore().settings?.workMemory.experienceDiscoveryDays || 7
  }

  function buildExportRequest(includeSensitive: boolean): WorkMemoryExportRequest {
    const recentDays = Number(exportDraft.value.recentDays)
    const startAt = Number.isFinite(recentDays) && recentDays > 0 ? Math.floor(Date.now() / 1000) - Math.floor(recentDays * 86400) : 0
    return {
      includeSensitive,
      startAt,
      tags: splitTags(exportDraft.value.tags),
      entryIds: splitEntryIds(exportDraft.value.entryIds),
    }
  }

  function pruneRetrospectiveSelection() {
    if (!retrospectiveSelectedIds.value.length) {
      return
    }
    const valid = new Set(entries.value.map((entry) => entry.id))
    retrospectiveSelectedIds.value = retrospectiveSelectedIds.value.filter((id) => valid.has(id))
  }

  async function persistWorkMemorySettings(patch: Partial<AppSettings['workMemory']>) {
    const settings = useSettingsStore()
    const cleaned: Partial<AppSettings['workMemory']> = {}
    if (patch.timeMachineEnabled !== undefined) cleaned.timeMachineEnabled = patch.timeMachineEnabled
    if (patch.privacyMode !== undefined) cleaned.privacyMode = patch.privacyMode
    await settings.updateWorkMemoryRuntime(cleaned)
  }

  watch(
    selectedEntry,
    () => {
      void loadSelectedImage()
    },
    { flush: 'post' },
  )

  return {
    status,
    entries,
    selectedId,
    retrospectiveSelectedIds,
    retrospectiveSelectedEntries,
    retrospectiveSelectionCount,
    retrospectiveDraftEntryIds,
    retrospectiveTargetLabel,
    proactiveSourcesEnabled,
    selectedEntry,
    filteredEntries,
    playbackEntries,
    playbackEntry,
    playbackPosition,
    query,
    searchResults,
    dailyDraft,
    dailyDraftPolishResult,
    retrospectiveDraft,
    knowledgeDraft,
    scheduledDraftStatus,
    autonomousArtifacts,
    autonomousRunResult,
    flowAutonomyStatus,
    flowCandidateActions,
    flowAutonomyRunResult,
    flowCandidateDecisionResult,
    health,
    semanticStatus,
    embeddingRefreshResult,
    semanticSearchResult,
    flowAskResult,
    flowConversations,
    flowMessages,
    selfModel,
    todoList,
    activeFlowConversationId,
    activeFlowConversation,
    knowledgeDraftSaveResult,
    knowledgeSkillExportResult,
    knowledgeSkillInstallResult,
    knowledgeSkillInstallDiagnostics,
    agentTask,
    workflowDraft,
    workflowDraftSaveResult,
    checklistDraft,
    checklistDraftSaveResult,
    experienceReport,
    experienceDiscoveryResult,
    exportResult,
    importResult,
    ocrResult,
    ocrLines: ocrSelection.ocrLines,
    selectedOCRLineCount: ocrSelection.selectedOCRLineCount,
    noteDraft,
    importDraft,
    exportDraft,
    semanticDraft,
    exclusionDraft,
    exclusionSummary,
    deleteArmedId,
    clearUnpinnedArmed,
    selectedImageUrl,
    selectedImageMissing,
    playbackIndex,
    playbackImageUrl,
    playbackImageMissing,
    feedback,
    isLoading,
    isLoadingPlaybackImage,
    isRecognizingOCR,
    isImportingMaterials,
    isSavingKnowledgeDraft,
    isExportingKnowledgeSkill,
    isInstallingKnowledgeSkill,
    isSavingWorkflowDraft,
    isSavingChecklistDraft,
    isSavingExclusions,
    isRunningScheduledDrafts,
    isRunningAutonomousFlow,
    isRunningFlowAutonomy,
    isDecidingFlowCandidate,
    isPolishingDailyDraft,
    isRefreshingEmbedding,
    isSemanticSearching,
    isAskingFlow,
    isLoadingFlowConversation,
    isDeletingFlowConversation,
    isSavingSelfAssertion,
    isSavingTodo,
    isDiscoveringExperienceAI,
    isDeletingEntries,
    isBatchRecognizingOCR,
    experienceDiscoveryProgress,
    experienceDiscoveryStage,
    deleteProgressDone,
    deleteProgressTotal,
    batchOcrProgressDone,
    batchOcrProgressTotal,
    batchOcrProgressStage,
    knowledgeDraftSaveArmed,
    dailyDraftPolishArmed,
    experienceDiscoveryArmed,
    knowledgeSkillExportArmed,
    knowledgeSkillInstallArmed,
    workflowDraftSaveArmed,
    checklistDraftSaveArmed,
    load,
    installLiveUpdates,
    setQuery,
    loadFlowConversations,
    selectFlowConversation,
    startFlowConversation,
    removeFlowConversation,
    askFlow,
    refreshTodos,
    saveSelfAssertion,
    removeSelfAssertion,
    saveTodo,
    changeTodo,
    removeTodo,
    select,
    isRetrospectiveSelected,
    toggleRetrospectiveSelection,
    clearRetrospectiveSelection,
    selectVisibleForRetrospective,
    selectEntriesForRetrospective,
    addNote,
    addManualNote,
    toggleTimeMachine,
    togglePrivacyMode,
    enableProactiveSinking,
    captureNow,
    buildDailyDraft,
    polishDailyDraft,
    runScheduledDrafts,
    runAutonomousFlow,
    refreshFlowCandidateActions,
    runFlowAutonomy,
    handleFlowCandidateAction,
    rejectAutonomousArtifact,
    refreshEmbedding,
    runSemanticSearch,
    buildRetrospectiveDraft,
    buildKnowledgeDraft,
    saveCurrentKnowledgeDraft,
    exportCurrentKnowledgeSkill,
    installCurrentKnowledgeSkill,
    buildAgentTask,
    discoverExperienceReport,
    discoverExperienceReportAI,
    markExperienceInsight,
    buildAgentTaskFromInsight,
    buildWorkflowDraftFromInsight,
    saveCurrentWorkflowDraft,
    buildChecklistDraftFromInsight,
    saveCurrentChecklistDraft,
    recognizeSelectedText,
    recognizeEntriesOCR,
    copyOCRText,
    copySelectedOCRText,
    isOCRLineSelected: ocrSelection.isOCRLineSelected,
    toggleOCRLine: ocrSelection.toggleOCRLine,
    selectAllOCRLines: ocrSelection.selectAllOCRLines,
    clearOCRLineSelection: ocrSelection.clearOCRLineSelection,
    exportData,
    importMaterials,
    loadExclusionRules,
    saveExclusionRules,
    startPlayback,
    stepPlayback,
    selectPlayback,
    deleteSelected,
    deleteEntries,
    clearUnpinned,
  }
})
