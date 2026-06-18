import {
  ArrowLeft,
  ArrowRight,
  Brain,
  Camera,
  Check,
  ChevronDown,
  Clock3,
  Copy,
  Database,
  Download,
  FileText,
  Flag,
  ImageOff,
  KeyRound,
  Play,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Shield,
  Sparkles,
  Tags,
  Trash2,
  Upload,
  Workflow,
  X,
} from '@lucide/vue'
import { Clipboard as WailsClipboard } from '@wailsio/runtime'
import { computed, markRaw, nextTick, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from 'vue'
import AriButton from '../../ui/AriButton.vue'
import { workMemoryFlowContextKey } from './context'
import { useAppShellStore } from '../../../stores/appShell'
import { useSettingsStore } from '../../../stores/settings'
import { useWorkMemoryStore } from '../../../stores/workMemory'
import { getCaptureThumbnailDataURL } from '../../../services/captureApi'
import type {
  AgentTaskPackage,
  ExperienceInsight,
  WorkMemoryAppCaptureProfile,
  WorkMemoryAutonomousArtifact,
  WorkMemoryEntry,
  WorkMemoryFlowConversation,
  WorkMemoryFlowMessage,
} from '../../../types/ariadne'
import type {
  CaptureAppCandidate,
  DraftKind,
  DraftKindItem,
  FlowCanvasEntry,
  FlowChatMessage,
  FlowPage,
  FlowChatRole,
  FlowSettingsTab,
  InsightMapNode,
  TimelineAppOption,
  TimelineAxisTick,
  TimelineDayGroup,
  TimelineLane,
  TimelineSourceFilter,
} from './types'

export function useWorkMemoryFlow() {
  const appShell = useAppShellStore()
  const settings = useSettingsStore()
  const memory = useWorkMemoryStore()
  
  const selected = computed(() => memory.selectedEntry)
  const visibleEntries = computed(() => memory.filteredEntries)
  const activeFlowPage = ref<FlowPage>('flow')
  const flowSidebarCollapsed = ref(false)
  const activeAssetFocus = ref<'agent' | 'workflow' | 'checklist' | 'skill' | ''>('')
  const flowQuestion = ref('')
  const globalFlowSearch = ref('')
  const flowBusy = ref(false)
  const flowChatThreadRef = ref<HTMLElement | null>(null)
  const flowChatInputRef = ref<HTMLTextAreaElement | null>(null)
  const flowPendingMessages = ref<FlowChatMessage[]>([])
  const flowChatSelectedIds = ref<string[]>([])
  const flowContextMenu = ref({
    open: false,
    x: 0,
    y: 0,
  })
  const flowRememberFeedback = ref('')
  const flowSettingsOpen = ref(false)
  const flowSettingsTab = ref<FlowSettingsTab>('capture')
  const selectedAppCaptureProfileId = ref('')
  const detailDrawerOpen = ref(false)
  const flowCanvasActiveId = ref('')
  const activeDraftKind = ref<DraftKind>('daily')
  const activeInsightId = ref('')
  const timelineSourceFilter = ref<TimelineSourceFilter>('all')
  const timelineAppFilter = ref('all')
  const timelineAppPickerOpen = ref(false)
  const timelineAppSearch = ref('')
  const timelineAppSelectRef = ref<HTMLElement | null>(null)
  const timelineAppSearchRef = ref<HTMLInputElement | null>(null)
  const timelineLaneMenuRef = ref<HTMLElement | null>(null)
  const timelineLaneMenu = ref({
    open: false,
    x: 0,
    y: 0,
    appName: '',
    label: '',
    count: 0,
  })
  const timelineExclusionFeedback = ref('')
  const timelineSelectedIds = ref<string[]>([])
  const timelineDeleteArmed = ref(false)
  const timelineThumbnailUrls = ref<Record<string, string>>({})
  const timelineThumbnailMissing = ref<Record<string, boolean>>({})
  const timelineVisibleDayCount = ref(2)
  const timelineLoadMoreRef = ref<HTMLElement | null>(null)
  const DAY_SECONDS = 86400
  const FLOW_DAY_START_HOUR = 6
  const FLOW_DAY_END_HOUR = 22
  const MIN_TIMELINE_ZOOM_HOURS = 0.25
  const selectedFlowDayStart = ref(startOfLocalDaySeconds(Math.floor(Date.now() / 1000)))
  const selectedFlowHour = ref(currentFlowHour())
  const timelineZoomStartHour = ref(FLOW_DAY_START_HOUR)
  const timelineZoomEndHour = ref(FLOW_DAY_END_HOUR)
  const assetFeedback = ref('')
  const rejectingAutonomousArtifactId = ref('')
  const autonomousRejectReason = ref('')
  let timelineLoadObserver: IntersectionObserver | null = null
  let uninstallMemoryLiveUpdates: (() => void) | null = null
  const draftKinds: DraftKindItem[] = [
    {
      kind: 'daily',
      label: '日报',
      icon: '日报',
      title: '日报',
      emptyHint: '等待从今日上下文生成日报，或手动选择留痕后生成。',
    },
    {
      kind: 'retrospective',
      label: '复盘',
      icon: '复盘',
      title: '复盘',
      emptyHint: '选择一组留痕后生成复盘，可回放关键决策路径。',
    },
    {
      kind: 'knowledge',
      label: '知识',
      icon: '知识',
      title: '知识',
      emptyHint: '从高价值记录里提炼可复用条目。',
    },
  ]
  const flowPages = [
    { id: 'flow' as const, label: '心流', detail: '对话', icon: markRaw(Brain) },
    { id: 'timeline' as const, label: '时间线', detail: '回放', icon: markRaw(Clock3) },
    { id: 'insights' as const, label: '洞察', detail: '归纳', icon: markRaw(Sparkles) },
    { id: 'drafts' as const, label: '草稿', detail: '输出', icon: markRaw(FileText) },
    { id: 'assets' as const, label: '资产', detail: '能力', icon: markRaw(Database) },
    { id: 'rules' as const, label: '规则', detail: '边界', icon: markRaw(Shield) },
  ]
  const flowQuestions = [
    '我今天干了些什么？',
    '今天有哪些人找过我？',
    '今天我的哪些工作流可以优化？',
    '刚才那个报错我后来怎么处理的？',
    '最近我重复做了哪些事？',
  ]
  const timelineFilters = [
    { id: 'all' as const, label: '全部', icon: Clock3 },
    { id: 'screenshots' as const, label: '截图', icon: Camera },
    { id: 'clipboard' as const, label: '剪贴板', icon: Copy },
    { id: 'notes' as const, label: '笔记', icon: FileText },
    { id: 'ocr' as const, label: 'OCR', icon: Search },
  ]
  const flowSettingsTabs = [
    { id: 'capture' as const, label: '采集', detail: '时间机器与沉淀' },
    { id: 'model' as const, label: '模型', detail: 'AI 与向量库' },
    { id: 'privacy' as const, label: '边界', detail: '排除与存储' },
  ]
  const timeMachineLabel = computed(() => {
    if (!memory.status.timeMachineEnabled) return '暂停'
    return memory.status.workerRunning ? '运行中' : '待启动'
  })
  const captureScopeLabel = computed(() => {
    const labels: Record<string, string> = {
      all_screens: '全部屏幕',
      active_window: '前台窗口',
      primary_screen: '主屏幕',
    }
    return labels[memory.status.captureScope ?? 'all_screens'] ?? '全部屏幕'
  })
  const multiMonitorLabel = computed(() => {
    const labels: Record<string, string> = {
      combined: '合并',
      per_monitor: '分屏',
      primary_only: '主屏',
    }
    return labels[memory.status.multiMonitor ?? 'combined'] ?? '合并'
  })
  const runtimeStatusText = computed(() => {
    const parts = [
      memory.status.pauseReason || memory.status.lastSkippedReason,
      memory.status.lastAutoOcrError ? `OCR ${memory.status.lastAutoOcrError}` : '',
      `策略 ${captureScopeLabel.value} / ${multiMonitorLabel.value}`,
      memory.status.autoOcrEnabled ? '自动 OCR' : 'OCR 手动',
      memory.status.windowSwitchCaptureEnabled ? `窗口切换触发 ${memory.status.windowSwitchCooldownSeconds || 30}s` : '窗口切换不触发',
      memory.status.pauseOnIdle ? `空闲阈值 ${formatDuration(memory.status.idlePauseSeconds ?? 0)}` : '不按空闲暂停',
      memory.status.pauseOnLock ? '锁屏暂停' : '锁屏不暂停',
    ].filter(Boolean)
    return parts.join(' · ')
  })
  const vectorStatusLabel = computed(() => {
    const status = memory.semanticStatus
    if (!status) return '未加载'
    if (status.lastEmbeddingError) return '异常'
    if (status.embeddingIndexed) return `${status.embeddingIndexed} 条`
    if (status.externalProvider) return status.externalEmbeddingReady ? '待刷新' : '未就绪'
    return status.ftsEnabled ? '本地 FTS' : '本地'
  })
  const vectorProviderLabel = computed(() => {
    const status = memory.semanticStatus
    if (!status) return '本地'
    const provider = status.externalProvider || status.provider || 'local'
    const model = status.embeddingModel ? ` / ${status.embeddingModel}` : ''
    return `${provider}${model}`
  })
  const vectorStoreLabel = computed(() => {
    const status = memory.semanticStatus
    if (!status?.vectorStoreType) return 'embedded'
    if (status.vectorStoreType === 'milvus' && status.vectorStoreUri) {
      return `Milvus · ${status.vectorStoreUri}`
    }
    return status.vectorStoreType
  })
  const selectedFlowDayEnd = computed(() => selectedFlowDayStart.value + DAY_SECONDS - 1)
  const todayEntries = computed(() =>
    memory.entries
      .filter((entry) => isEntryInSelectedFlowDay(entry))
      .sort((left, right) => right.createdAt - left.createdAt),
  )
  const visibleDayEntries = computed(() =>
    visibleEntries.value
      .filter((entry) => isEntryInSelectedFlowDay(entry))
      .sort((left, right) => right.createdAt - left.createdAt),
  )
  const selectedFlowTimeWindowEntries = computed(() => {
    const entries = todayEntries.value
    if (!entries.length) return []
    const center = selectedFlowDayStart.value + Math.round(selectedFlowHour.value * 3600)
    const windowStart = center - 30 * 60
    const windowEnd = center + 30 * 60
    const matches = entries.filter((entry) => entry.createdAt >= windowStart && entry.createdAt <= windowEnd)
    return matches.length ? matches : entries
  })
  const askedEvidenceEntries = computed(() => {
    const evidence = memory.flowAskResult?.evidence ?? []
    if (!evidence.length) return []
    const byId = new Map(memory.entries.map((entry) => [entry.id, entry]))
    return evidence.map((item) => byId.get(item.id)).filter(Boolean) as WorkMemoryEntry[]
  })
  const recentEvidence = computed(() => {
    return todayEntries.value
  })
  const topApps = computed(() => {
    const counts = new Map<string, number>()
    for (const entry of todayEntries.value) {
      const app = entry.appName || 'Unknown'
      counts.set(app, (counts.get(app) ?? 0) + 1)
    }
    return [...counts.entries()].sort((left, right) => right[1] - left[1]).slice(0, 4)
  })
  const appCaptureProfiles = computed(() => settings.settings?.workMemory.appCaptureProfiles ?? [])
  const selectedAppCaptureProfile = computed(() => {
    const profiles = appCaptureProfiles.value
    if (!profiles.length) return null
    return profiles.find((profile) => profile.id === selectedAppCaptureProfileId.value) ?? profiles[0]
  })
  const appCaptureCandidates = computed<CaptureAppCandidate[]>(() => {
    const existing = new Set(appCaptureProfiles.value.map((profile) => appProfileId(profile.processName || profile.displayName || profile.id)))
    const counts = new Map<string, CaptureAppCandidate>()
    const entries = todayEntries.value
    for (const entry of entries) {
      const processName = (entry.appName || '').trim()
      if (!processName) continue
      const id = appProfileId(processName)
      if (!id || existing.has(id)) continue
      const current = counts.get(id)
      if (current) {
        current.count += 1
        continue
      }
      counts.set(id, {
        id,
        displayName: displayAppName(processName),
        processName,
        count: 1,
      })
    }
    return [...counts.values()].sort((left, right) => right.count - left.count).slice(0, 8)
  })
  const flowSuggestedQuestions = computed(() => {
    const suggested = memory.flowAskResult?.suggestedQuestions?.filter(Boolean) ?? []
    return (suggested.length ? suggested : flowQuestions).slice(0, 3)
  })
  const flowConversations = computed(() => memory.flowConversations)
  const activeFlowConversation = computed(() => memory.activeFlowConversation)
  const activeFlowConversationId = computed(() => memory.activeFlowConversationId)
  const flowChatMessages = computed<FlowChatMessage[]>(() => [
    ...memory.flowMessages.map(flowChatMessageFromStored),
    ...flowPendingMessages.value,
  ])
  const flowCanvasEntryLookup = computed(() => {
    const byId = new Map<string, WorkMemoryEntry>()
    for (const entry of memory.entries) {
      byId.set(entry.id, entry)
    }
    return byId
  })
  const flowCanvasMessages = computed<FlowCanvasEntry[]>(() => {
    return flowChatMessages.value
      .filter((message) => !message.system)
      .filter((message) => message.role === 'assistant')
      .filter((message) => message.text.trim())
      .map((message) => {
        const evidenceEntries = (message.result?.evidence ?? [])
          .map((item) => flowCanvasEntryLookup.value.get(item.id))
          .filter(Boolean) as WorkMemoryEntry[]
        const uncertainty: string[] = []
        if (message.pending) {
          uncertainty.push('处理中')
        }
        if (message.error) {
          uncertainty.push('本次归纳出现异常')
        }
        if (!message.result?.usedAi && !message.result?.mode) {
          uncertainty.push('仅基于本地检索生成')
        }
        if (!evidenceEntries.length) {
          uncertainty.push('当前结论缺少可展开留痕')
        }
        const recommendedActions = message.result?.evidence?.length
          ? ['打开留痕', '复制该结论', ...(message.result.usedAi ? ['转为 AI 润色提示'] : [])]
          : ['补充留痕']
        return {
          message,
          conclusion: message.text.trim().slice(0, 220),
          evidenceEntries,
          uncertainty,
          recommendedActions,
        }
      })
  })
  const flowCanvasPrimaryEntry = computed(() => {
    const all = flowCanvasMessages.value
    const fallback = all[all.length - 1] ?? null
    const matched = flowCanvasMessages.value.find((entry) => entry.message.id === flowCanvasActiveId.value)
    return matched ?? fallback
  })
  const selectableFlowChatMessages = computed(() => flowChatMessages.value.filter(isFlowMessageSelectable))
  const draftItems = computed(() => {
    return draftKinds.map((item) => {
      const draft = {
        daily: memory.dailyDraft,
        retrospective: memory.retrospectiveDraft,
        knowledge: memory.knowledgeDraft,
      }[item.kind]
      if (!draft) {
        return {
          ...item,
          draft: null,
          evidence: [],
          createdAtLabel: '',
        }
      }
      return {
        ...item,
        draft,
        evidence: evidenceEntriesFromIds(draft.evidence),
        createdAtLabel: formatTime(draft.createdAt),
      }
    })
  })
  const activeDraft = computed(() => draftItems.value.find((item) => item.kind === activeDraftKind.value) ?? draftItems.value[0])
  const draftTimelineEntries = computed(() => activeDraft.value?.evidence ?? [])
  const draftEvidenceTimeline = computed(() =>
    [...draftTimelineEntries.value]
      .filter((entry) => entry.createdAt)
      .sort((left, right) => left.createdAt - right.createdAt),
  )
  const activeDraftSourceSummary = computed(() => {
    const count = draftTimelineEntries.value.length
    if (!count) return '尚未绑定留痕'
    return `已绑定 ${count} 条留痕`
  })
  const selectedFlowChatMessages = computed(() => {
    const selectedIds = new Set(flowChatSelectedIds.value)
    return selectableFlowChatMessages.value.filter((message) => selectedIds.has(message.id))
  })
  const flowChatIsEmpty = computed(() => !flowChatMessages.value.length && !memory.isLoadingFlowConversation)
  const flowSelectionLabel = computed(() => {
    const count = selectedFlowChatMessages.value.length
    return count ? `已选 ${count} 条` : '右键或勾选消息后加入沉淀'
  })
  const autonomousInboxSummary = computed(() => {
    const count = memory.autonomousArtifacts.length
    if (!count) return '暂无待确认产物'
    const skillCount = memory.autonomousArtifacts.filter((artifact) => artifact.kind === 'skill').length
    return `${count} 个自主产物${skillCount ? ` · ${skillCount} 个 Skill` : ''}`
  })
  const evidenceCounts = computed(() => {
    const entries = todayEntries.value
    return {
      screenshots: entries.filter((entry) => Boolean(entry.imagePath || entry.captureId)).length,
      clipboard: entries.filter((entry) => /clipboard/.test(entry.source)).length,
      notes: entries.filter((entry) => /note|manual_note/.test(entry.source)).length,
      ocr: entries.filter((entry) => Boolean(entry.ocrText || entry.ocrStatus)).length,
    }
  })
  const timelineFilterCounts = computed<Record<TimelineSourceFilter, number>>(() => {
    const entries = visibleDayEntries.value
    return {
      all: entries.length,
      screenshots: entries.filter(isScreenshotEntry).length,
      clipboard: entries.filter(isClipboardEntry).length,
      notes: entries.filter(isNoteEntry).length,
      ocr: entries.filter(isOcrEntry).length,
    }
  })
  const timelineSourceEntries = computed(() => {
    const entries = visibleDayEntries.value
    switch (timelineSourceFilter.value) {
      case 'screenshots':
        return entries.filter(isScreenshotEntry)
      case 'clipboard':
        return entries.filter(isClipboardEntry)
      case 'notes':
        return entries.filter(isNoteEntry)
      case 'ocr':
        return entries.filter(isOcrEntry)
      default:
        return entries
    }
  })
  const timelineRangeSourceEntries = computed(() => {
    const { min, max } = timelineRangeWindow.value
    return timelineSourceEntries.value.filter((entry) => entry.createdAt >= min && entry.createdAt <= max)
  })
  const timelineAppOptions = computed<TimelineAppOption[]>(() => {
    const byApp = new Map<string, TimelineAppOption>()
    for (const entry of timelineRangeSourceEntries.value) {
      const id = timelineAppKey(entry)
      const label = entry.appName ? displayAppName(entry.appName) : 'Unknown'
      const current = byApp.get(id)
      if (current) {
        current.count += 1
        continue
      }
      byApp.set(id, { id, label, count: 1 })
    }
    return [...byApp.values()].sort((left, right) => right.count - left.count || left.label.localeCompare(right.label))
  })
  const filteredTimelineAppOptions = computed(() => {
    const query = timelineAppSearch.value.trim().toLowerCase()
    if (!query) {
      return timelineAppOptions.value
    }
    return timelineAppOptions.value.filter((option) => {
      return option.label.toLowerCase().includes(query) || option.id.toLowerCase().includes(query)
    })
  })
  const timelineEntries = computed(() => {
    const entries = timelineRangeSourceEntries.value
    if (timelineAppFilter.value === 'all') {
      return entries
    }
    return entries.filter((entry) => timelineAppKey(entry) === timelineAppFilter.value)
  })
  const timelineSelectedIdSet = computed(() => new Set(timelineSelectedIds.value))
  const timelineSelectedEntries = computed(() => {
    const selected = timelineSelectedIdSet.value
    return memory.entries.filter((entry) => selected.has(entry.id))
  })
  const timelineCurrentSelectionCount = computed(() => timelineEntries.value.filter((entry) => timelineSelectedIdSet.value.has(entry.id)).length)
  const timelineAllCurrentSelected = computed(() => Boolean(timelineEntries.value.length) && timelineCurrentSelectionCount.value === timelineEntries.value.length)
  const timelineSelectAllLabel = computed(() => (timelineAllCurrentSelected.value ? '取消全选' : `全选当前 ${timelineEntries.value.length}`))
  const timelineDeleteLabel = computed(() => (timelineDeleteArmed.value ? `确认删除 ${timelineSelectedEntries.value.length}` : '删除所选'))
  const timelineDayGroups = computed<TimelineDayGroup[]>(() => {
    const groups = new Map<string, TimelineDayGroup>()
    const sorted = timelineEntries.value.slice().sort((left, right) => right.createdAt - left.createdAt)
    for (const entry of sorted) {
      const id = timelineDayKey(entry.createdAt)
      const current = groups.get(id)
      if (current) {
        current.entries.push(entry)
        current.note = timelineDayNote(current.entries)
        continue
      }
      groups.set(id, {
        id,
        label: timelineDayLabel(entry.createdAt),
        note: timelineDayNote([entry]),
        entries: [entry],
      })
    }
    return [...groups.values()]
  })
  const visibleTimelineDayGroups = computed(() => timelineDayGroups.value.slice(0, timelineVisibleDayCount.value))
  const timelineHasMoreDays = computed(() => timelineVisibleDayCount.value < timelineDayGroups.value.length)
  const timelineStats = computed(() => {
    const entries = timelineEntries.value
    const sensitive = entries.filter((entry) => entry.sensitive).length
    return [
      { label: '轨迹', value: String(entries.length), note: memory.isLoading ? '同步中' : '当前筛选' },
      { label: '程序', value: String(timelineAppOptions.value.length), note: timelineAppFilter.value === 'all' ? '来源应用' : selectedTimelineAppLabel.value },
      { label: '天数', value: String(timelineDayGroups.value.length), note: timelineHasMoreDays.value ? '滚动加载' : '已加载' },
      { label: '可复盘', value: String(Math.max(entries.length - sensitive, 0)), note: sensitive ? `已排除 ${sensitive} 条敏感` : '非敏感记录' },
      { label: '已选', value: String(timelineSelectedEntries.value.length), note: timelineSelectionSummary.value },
    ]
  })
  const selectedTimelineAppLabel = computed(() => {
    if (timelineAppFilter.value === 'all') return '全部程序'
    return timelineAppOptions.value.find((option) => option.id === timelineAppFilter.value)?.label ?? '当前程序'
  })
  const selectedTimelineAppCount = computed(() => {
    if (timelineAppFilter.value === 'all') return timelineRangeSourceEntries.value.length
    return timelineAppOptions.value.find((option) => option.id === timelineAppFilter.value)?.count ?? timelineEntries.value.length
  })
  const timelineSelectionSummary = computed(() => {
    const count = timelineSelectedEntries.value.length
    if (!count) return '未选择轨迹'
    const sensitive = timelineSelectedEntries.value.filter((entry) => entry.sensitive).length
    return sensitive ? `已选 ${count} 条，含 ${sensitive} 条敏感` : `已选 ${count} 条`
  })
  const timelineRangeWindow = computed(() => {
    const min = selectedFlowDayStart.value + Math.round(timelineZoomStartHour.value * 3600)
    const max = selectedFlowDayStart.value + Math.round(timelineZoomEndHour.value * 3600)
    return {
      min,
      max,
      span: Math.max(1, max - min),
    }
  })
  const timelineAxisTicks = computed<TimelineAxisTick[]>(() => {
    return flowTimeRulerTicks.value
  })
  const timelineLanes = computed<TimelineLane[]>(() => {
    const byLane = new Map<string, TimelineLane>()
    const sorted = timelineEntries.value.slice().sort((left, right) => left.createdAt - right.createdAt)
    for (const entry of sorted) {
      const appName = entry.appName?.trim() ?? ''
      const key = appName || sourceLabel(entry)
      const lane = byLane.get(key) ?? { key, label: key, appName, entries: [] }
      if (appName && !lane.appName) {
        lane.appName = appName
      }
      lane.entries.push(entry)
      byLane.set(key, lane)
    }
    return [...byLane.values()].sort((left, right) => right.entries.length - left.entries.length)
  })
  const timelineScrubPercent = computed(() => {
    if (!memory.playbackEntries.length) return 0
    if (memory.playbackIndex < 0) return 0
    return ((memory.playbackIndex + 1) / memory.playbackEntries.length) * 100
  })
  const timelinePlayStateLabel = computed(() => {
    if (!memory.playbackEntries.length) return '暂无回放帧'
    if (memory.playbackEntry) return `回放中 ${memory.playbackPosition}`
    return `可用 ${memory.playbackEntries.length} 帧`
  })
  const deleteProgressPercent = computed(() => {
    if (!memory.deleteProgressTotal) return 0
    return Math.min(100, Math.round((memory.deleteProgressDone / memory.deleteProgressTotal) * 100))
  })
  const batchOcrProgressPercent = computed(() => {
    if (!memory.batchOcrProgressTotal) return 0
    return Math.min(100, Math.round((memory.batchOcrProgressDone / memory.batchOcrProgressTotal) * 100))
  })
  const timelineBatchOcrEntries = computed(() => {
    return timelineSelectedEntries.value.filter(canRunTimelineOCR)
  })
  const insightProgressPercent = computed(() => Math.min(100, Math.max(0, memory.experienceDiscoveryProgress || 0)))
  const insightNodes = computed<InsightMapNode[]>(() => {
    const insights = memory.experienceReport?.insights ?? []
    if (!insights.length) return []
    const baseRadius = insights.length > 8 ? 188 : insights.length > 5 ? 208 : 224
    return insights.map((insight, index) => {
      const angle = (360 / insights.length) * index
      return {
        insight,
        angle,
        radius: baseRadius + (index % 2 === 0 ? 0 : 34),
      }
    })
  })
  const selectedInsight = computed(() => {
    const current = insightNodes.value.find((node) => node.insight.id === activeInsightId.value)
    return current?.insight ?? insightNodes.value[0]?.insight ?? null
  })
  const insightEvidencePreview = computed(() => {
    if (!selectedInsight.value) return []
    return evidenceEntriesFromIds(selectedInsight.value.evidence)
  })
  const rulesPipelineStatus = computed(() => {
    const now = memory.status
    return [
      {
        key: 'capture',
        label: '采集',
        state: now.timeMachineEnabled ? '运行中' : '待启动',
        status: now.timeMachineEnabled ? 'ok' : 'warn',
        note: now.timeMachineEnabled ? '持续接入时间机器输出' : '关闭会暂停新留痕采集',
      },
      {
        key: 'ocr',
        label: 'OCR',
        state: now.autoOcrEnabled ? '就绪' : '未启用',
        status: now.autoOcrEnabled ? 'ok' : 'warn',
        note: now.autoOcrEnabled ? '可复用截图内容' : '截图需手动触发',
      },
      {
        key: 'quality',
        label: '质检',
        state: memory.semanticStatus ? '已接入' : '未连接',
        status: memory.semanticStatus?.embeddingIndexed || memory.semanticStatus?.indexedEntries ? 'ok' : 'warn',
        note:
          memory.semanticStatus?.embeddingIndexed || memory.semanticStatus?.indexedEntries
            ? '质检结果可回看'
            : '需确认向量与模型状态',
      },
      {
        key: 'index',
        label: '索引',
        state: memory.semanticStatus ? vectorStoreLabel.value : '未初始化',
        status: memory.semanticStatus?.embeddingIndexed ? 'ok' : 'warn',
        note: `总计 ${memory.semanticStatus?.embeddingIndexed ?? 0} 条已索引`,
      },
      {
        key: 'export',
        label: '导出/打包',
        state: memory.scheduledDraftStatus ? '可同步' : '待触发',
        status: 'ok',
        note: '导出与导入为规则链保留入口',
      },
    ]
  })
  const timelineSelectedSummary = computed(() => {
    if (!memory.retrospectiveSelectionCount) return '未选择复盘留痕。可勾选关键轨迹或选择当前筛选结果。'
    return `已选择 ${memory.retrospectiveSelectionCount} 条留痕，可生成复盘、日报或任务包。`
  })
  const flowDateLabel = computed(() => new Date(selectedFlowDayStart.value * 1000).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', weekday: 'short' }))
  const flowDateButtonLabel = computed(() => {
    const date = new Date(selectedFlowDayStart.value * 1000)
    const weekday = date.toLocaleDateString('zh-CN', { weekday: 'short' })
    const relative = relativeDayLabel(selectedFlowDayStart.value)
    return `${timelineDayKey(selectedFlowDayStart.value)} ${relative ? `${relative} · ` : ''}${weekday}`
  })
  const flowCurrentClock = computed(() => flowHourLabel(selectedFlowHour.value))
  const timelineZoomActive = computed(() => timelineZoomStartHour.value > FLOW_DAY_START_HOUR || timelineZoomEndHour.value < FLOW_DAY_END_HOUR)
  const timelineZoomLabel = computed(() => `${flowHourLabel(timelineZoomStartHour.value)}-${flowHourLabel(timelineZoomEndHour.value)}`)
  const flowTimeRangeLabel = computed(() => timelineZoomLabel.value)
  const flowWorkHoursLabel = computed(() => '09:00-18:30')
  const flowTimeRulerTicks = computed<TimelineAxisTick[]>(() => {
    const start = timelineZoomStartHour.value
    const end = timelineZoomEndHour.value
    const span = Math.max(MIN_TIMELINE_ZOOM_HOURS, end - start)
    const step = timelineTickStepHours(span)
    const ticks: TimelineAxisTick[] = []
    const seen = new Set<string>()
    const addTick = (hour: number) => {
      const normalized = Math.round(hour * 100) / 100
      if (normalized < start - 0.001 || normalized > end + 0.001) return
      const key = normalized.toFixed(2)
      if (seen.has(key)) return
      seen.add(key)
      ticks.push({
        label: flowHourLabel(normalized),
        left: timelineHourToPercent(normalized),
      })
    }
    addTick(start)
    for (let hour = Math.ceil(start / step) * step; hour <= end + 0.001; hour += step) {
      addTick(hour)
    }
    addTick(end)
    return ticks.sort((left, right) => left.left - right.left)
  })
  const flowTimeRulerNowPercent = computed(() => {
    return timelineHourToPercent(selectedFlowHour.value)
  })
  const globalSearchPlaceholder = computed(() => {
    const labels: Record<FlowPage, string> = {
      flow: '搜索留痕、窗口、OCR 或对话...',
      timeline: '搜索时间线留痕...',
      insights: '搜索洞察和留痕链...',
      drafts: '搜索草稿来源...',
      assets: '搜索任务包和资产...',
      rules: '搜索规则和索引...',
    }
    return labels[activeFlowPage.value]
  })
  const flowWindowPanelItems = computed(() => {
    const byApp = new Map<string, { app: string; count: number; latest?: WorkMemoryEntry }>()
    for (const entry of selectedFlowTimeWindowEntries.value) {
      const app = entry.appName || 'Unknown'
      const current = byApp.get(app) ?? { app, count: 0, latest: entry }
      current.count += 1
      if (!current.latest || entry.createdAt > current.latest.createdAt) {
        current.latest = entry
      }
      byApp.set(app, current)
    }
    return [...byApp.values()].sort((left, right) => right.count - left.count).slice(0, 5)
  })
  const timelineDensityBars = computed(() => {
    const { min, span } = timelineRangeWindow.value
    const bucketSeconds = timelineDensityBucketSeconds(span)
    const bucketCount = Math.max(1, Math.ceil(span / bucketSeconds))
    const buckets = Array.from({ length: bucketCount }, (_, index) => ({
      key: `bucket-${index}`,
      start: min + index * bucketSeconds,
      end: Math.min(min + (index + 1) * bucketSeconds, min + span),
      count: 0,
    }))
    for (const entry of timelineEntries.value) {
      const index = Math.max(0, Math.min(bucketCount - 1, Math.floor((entry.createdAt - min) / bucketSeconds)))
      buckets[index].count += 1
    }
    const max = Math.max(...buckets.map((bucket) => bucket.count), 1)
    return buckets.map((bucket) => ({
      key: bucket.key,
      count: bucket.count,
      left: ((bucket.start - min) / span) * 100,
      width: Math.max(0.35, ((bucket.end - bucket.start) / span) * 100),
      height: bucket.count ? Math.max(16, Math.round((bucket.count / max) * 100)) : 6,
      label: `${formatTimelineClock(bucket.start)}-${formatTimelineClock(Math.max(bucket.start, bucket.end - 1))} · ${bucket.count} 条`,
    }))
  })
  const insightLinks = computed(() => {
    const nodes = insightNodes.value
    return nodes.slice(0, -1).map((node, index) => {
      const next = nodes[index + 1]
      return {
        id: `${node.insight.id}-${next.insight.id}`,
        x1: 50 + Math.cos((node.angle * Math.PI) / 180) * (node.radius / 5.4),
        y1: 50 + Math.sin((node.angle * Math.PI) / 180) * (node.radius / 5.4),
        x2: 50 + Math.cos((next.angle * Math.PI) / 180) * (next.radius / 5.4),
        y2: 50 + Math.sin((next.angle * Math.PI) / 180) * (next.radius / 5.4),
        strength: index % 3,
      }
    })
  })
  const assetReadinessScore = computed(() => {
    const task = memory.agentTask
    if (!task) return 0
    const checks = [Boolean(task.goal), Boolean(task.context), task.evidence.length > 0, task.boundaries.length > 0, task.acceptance.length > 0]
    return Math.round((checks.filter(Boolean).length / checks.length) * 100)
  })
  const assetReadinessParts = computed(() => [
    { label: 'Goal', ok: Boolean(memory.agentTask?.goal) },
    { label: 'Context', ok: Boolean(memory.agentTask?.context) },
    { label: 'Trace', ok: Boolean(memory.agentTask?.evidence.length) },
    { label: 'Boundaries', ok: Boolean(memory.agentTask?.boundaries.length) },
    { label: 'Acceptance', ok: Boolean(memory.agentTask?.acceptance.length) },
  ])
  const assetMissingEvidence = computed(() => {
    if (!memory.agentTask) return ['生成任务包后检查关键留痕', '补充验收标准', '确认外部访问边界']
    const missing: string[] = []
    if (!memory.agentTask.evidence.length) missing.push('缺少可追溯留痕')
    if (!memory.agentTask.boundaries.length) missing.push('缺少权限和隐私边界')
    if (!memory.agentTask.acceptance.length) missing.push('缺少验收标准')
    return missing.length ? missing : ['任务包字段完整，外发前人工确认']
  })
  const captureSourceCards = computed(() => [
    { label: '屏幕截图', count: evidenceCounts.value.screenshots, state: memory.status.timeMachineEnabled ? '运行中' : '暂停' },
    { label: 'OCR 文本', count: evidenceCounts.value.ocr, state: memory.status.autoOcrEnabled ? '自动' : '手动' },
    { label: '剪贴板', count: evidenceCounts.value.clipboard, state: '本地' },
    { label: '应用上下文', count: topApps.value.length, state: '记录中' },
    { label: '窗口标题', count: todayEntries.value.filter((entry) => entry.windowTitle).length, state: '索引中' },
  ])
  const exclusionRuleTabs = computed(() => {
    const draft = memory.exclusionDraft
    const countLines = (value?: string) =>
      String(value || '')
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean).length
    return [
      { key: 'apps', label: '应用进程', count: countLines(draft.apps) },
      { key: 'window', label: '窗口关键词', count: countLines(draft.windowKeywords) },
      { key: 'paths', label: '路径片段', count: countLines(draft.paths) },
      { key: 'content', label: '内容正则', count: countLines(draft.contentPatterns) },
      { key: 'sensitive', label: '敏感凭据', count: rulesImpactStats.value[0]?.value ?? '0' },
    ]
  })
  const exclusionRuleRows = computed(() => {
    const draft = memory.exclusionDraft
    const rows: Array<{ group: string; value: string; action: string; hits: number; priority: string }> = []
    const addRows = (group: string, value: string | undefined, action: string, priority: string) => {
      String(value || '')
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter(Boolean)
        .slice(0, 6)
        .forEach((line, index) => rows.push({ group, value: line, action, hits: Math.max(0, todayEntries.value.length - index * 2), priority }))
    }
    addRows('应用进程', draft.apps, '阻止采集', 'P0')
    addRows('窗口关键词', draft.windowKeywords, '标记敏感', 'P1')
    addRows('路径片段', draft.paths, '阻止索引', 'P1')
    addRows('内容正则', draft.contentPatterns, '质检隔离', 'P2')
    return rows.length ? rows : [{ group: '默认', value: '暂无排除规则', action: '允许采集', hits: 0, priority: 'P3' }]
  })
  const rulesImpactStats = computed(() => {
    const sensitive = todayEntries.value.filter((entry) => entry.sensitive).length
    const blocked = sensitive
    const reduced = Math.round((blocked / Math.max(todayEntries.value.length, 1)) * 100)
    return [
      { label: '阻止采集', value: String(blocked), note: '今日敏感/排除' },
      { label: '减少体积', value: `${reduced}%`, note: '估算节省' },
      { label: '节省索引', value: String(blocked), note: '未入库记录' },
    ]
  })
  
  async function runGlobalFlowSearch() {
    const query = globalFlowSearch.value.trim()
    if (!query) return
    if (activeFlowPage.value === 'flow') {
      await askFlow(query)
      return
    }
    memory.semanticDraft.query = query
    await memory.runSemanticSearch()
  }
  
  async function copyTimelineSelectionReference() {
    const entries = timelineSelectedEntries.value.length ? timelineSelectedEntries.value : timelineEntries.value.slice(0, 8)
    if (!entries.length) return
    await copyText(entries.map((entry) => `${formatTime(entry.createdAt)} · ${entryFocusTitle(entry)} · ${entry.id}`).join('\n'))
  }
  
  function evidenceEntriesFromIds(ids: string[]) {
    return ids
      .map((id) => flowCanvasEntryLookup.value.get(id))
      .filter(Boolean)
      .slice(0, 16) as WorkMemoryEntry[]
  }
  
  function setActiveDraftKind(kind: DraftKind) {
    activeDraftKind.value = kind
  }
  
  function setActiveInsight(insight: ExperienceInsight) {
    activeInsightId.value = insight.id
  }
  
  function insightNodeStyle(node: InsightMapNode) {
    return {
      transform: `rotate(${node.angle}deg) translate(${node.radius}px) rotate(-${node.angle}deg)`,
    }
  }
  
  function openTimelinePlaybackTick(event: Event) {
    const target = event.target as HTMLInputElement | null
    if (!target) return
    const value = Number(target.value)
    if (!Number.isFinite(value)) return
    const index = Math.max(0, Math.min(memory.playbackEntries.length - 1, value - 1))
    void memory.selectPlayback(index)
  }

  async function openTimelinePlaybackDetail() {
    if (!memory.playbackEntries.length) return
    if (!memory.playbackEntry) {
      const index = memory.playbackIndex >= 0 ? memory.playbackIndex : memory.playbackEntries.length - 1
      await memory.selectPlayback(index)
    }
    if (memory.playbackEntry) {
      openEvidence(memory.playbackEntry)
    }
  }
  
  function timelineEventLeft(entry: WorkMemoryEntry) {
    const { min, span } = timelineRangeWindow.value
    const pos = Math.max(0, Math.min(100, ((entry.createdAt - min) / span) * 100))
    return pos
  }
  
  function timelineEventStyle(entry: WorkMemoryEntry, index = 0) {
    return {
      left: `${timelineEventLeft(entry)}%`,
      top: `${8 + (index % 3) * 48}px`,
    }
  }
  
  function sourceLabel(entry: WorkMemoryEntry) {
    const labels: Record<string, string> = {
      clipboard: '剪贴板',
      time_machine: '屏幕时间机器',
      manual_capture: '手动补记',
      manual_note: '手动笔记',
      screenshot: '截图',
      file: '文件',
      note: '笔记',
    }
    return labels[entry.source] ?? entry.source
  }
  
  function isScreenshotEntry(entry: WorkMemoryEntry) {
    return Boolean(entry.imagePath || entry.captureId) || /screenshot|capture|time_machine/.test(entry.source)
  }
  
  function isClipboardEntry(entry: WorkMemoryEntry) {
    return /clipboard/.test(entry.source)
  }
  
  function isNoteEntry(entry: WorkMemoryEntry) {
    return /note|manual_note/.test(entry.source)
  }
  
  function isOcrEntry(entry: WorkMemoryEntry) {
    return Boolean(entry.ocrText || entry.ocrStatus)
  }
  
  function timelineThumbnailUrl(entry: WorkMemoryEntry) {
    const captureId = entry.captureId?.trim()
    if (!captureId) return ''
    return timelineThumbnailUrls.value[captureId] ?? ''
  }
  
  function timelineThumbnailIsMissing(entry: WorkMemoryEntry) {
    const captureId = entry.captureId?.trim()
    if (!captureId) return false
    return Boolean(timelineThumbnailMissing.value[captureId])
  }
  
  async function loadTimelineThumbnail(captureId: string) {
    if (!captureId || timelineThumbnailUrls.value[captureId] !== undefined) {
      return
    }
    timelineThumbnailUrls.value = {
      ...timelineThumbnailUrls.value,
      [captureId]: '',
    }
    timelineThumbnailMissing.value = {
      ...timelineThumbnailMissing.value,
      [captureId]: false,
    }
    try {
      const url = await getCaptureThumbnailDataURL(captureId)
      if (url) {
        timelineThumbnailUrls.value = {
          ...timelineThumbnailUrls.value,
          [captureId]: url,
        }
      } else {
        timelineThumbnailMissing.value = {
          ...timelineThumbnailMissing.value,
          [captureId]: true,
        }
      }
    } catch {
      timelineThumbnailUrls.value = {
        ...timelineThumbnailUrls.value,
        [captureId]: '',
      }
      timelineThumbnailMissing.value = {
        ...timelineThumbnailMissing.value,
        [captureId]: true,
      }
    }
  }
  
  function primeTimelineThumbnails(entries: WorkMemoryEntry[]) {
    if (activeFlowPage.value !== 'timeline') return
    const captureIds = [
      ...new Set(
        entries
          .map((entry) => entry.captureId?.trim() ?? '')
          .filter(Boolean),
      ),
    ].slice(0, 160)
    for (const captureId of captureIds) {
      void loadTimelineThumbnail(captureId)
    }
  }
  
  function canRunTimelineOCR(entry: WorkMemoryEntry) {
    return Boolean(entry.imagePath) && !entry.sensitive && entry.qualityStatus !== 'pending'
  }
  
  function timelineAppKey(entry: WorkMemoryEntry) {
    return appProfileId(entry.appName || 'Unknown') || 'unknown'
  }
  
  function timelineDayKey(timestamp: number) {
    const date = new Date(timestamp * 1000)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }
  
  function timelineDayLabel(timestamp: number) {
    const date = new Date(timestamp * 1000)
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    const target = new Date(date)
    target.setHours(0, 0, 0, 0)
    const diffDays = Math.round((today.getTime() - target.getTime()) / 86400000)
    const weekday = date.toLocaleDateString('zh-CN', { weekday: 'short' })
    if (diffDays === 0) return `今天 · ${weekday}`
    if (diffDays === 1) return `昨天 · ${weekday}`
    return `${date.getMonth() + 1}月${date.getDate()}日 · ${weekday}`
  }
  
  function relativeDayLabel(timestamp: number) {
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    const target = new Date(timestamp * 1000)
    target.setHours(0, 0, 0, 0)
    const diffDays = Math.round((today.getTime() - target.getTime()) / 86400000)
    if (diffDays === 0) return '今天'
    if (diffDays === 1) return '昨天'
    if (diffDays === -1) return '明天'
    return ''
  }
  
  function timelineDayNote(entries: WorkMemoryEntry[]) {
    const apps = new Set(entries.map((entry) => entry.appName || 'Unknown'))
    const screenshots = entries.filter(isScreenshotEntry).length
    return `${entries.length} 条 · ${apps.size} 个程序${screenshots ? ` · ${screenshots} 张画面` : ''}`
  }
  
  function formatTimelineClock(timestamp: number) {
    if (!timestamp) return '--:--'
    return new Date(timestamp * 1000).toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    })
  }
  
  function cleanTimelineText(value?: string) {
    return String(value || '')
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line && !/^(截图路径|尺寸|来源|采集范围|多屏策略|OCR:?)[:：]/i.test(line))
      .join(' ')
      .replace(/\s+/g, ' ')
      .trim()
  }
  
  function isGenericTimelineTitle(value?: string) {
    return /^(截图历史|屏幕时间机器|剪贴板|手动补记|自动记录|自动沉淀|剪贴板图片|当前屏幕|工作记忆)/i.test(String(value || '').trim())
  }
  
  function trimTimelineText(value: string, maxLength: number) {
    const chars = [...value]
    return chars.length > maxLength ? `${chars.slice(0, maxLength).join('')}...` : value
  }
  
  function entryFocusTitle(entry: WorkMemoryEntry) {
    const candidates = [entry.title, entry.summary, entry.text, entry.windowTitle, entry.ocrText]
    for (const candidate of candidates) {
      const text = cleanTimelineText(candidate)
      if (text && !isGenericTimelineTitle(text)) {
        return trimTimelineText(text, 54)
      }
    }
    return trimTimelineText(cleanTimelineText(entry.title) || sourceLabel(entry), 54)
  }
  
  function entryFocusSummary(entry: WorkMemoryEntry) {
    const title = entryFocusTitle(entry)
    const candidates = [entry.summary, entry.text, entry.windowTitle, entry.title, entry.ocrText]
    for (const candidate of candidates) {
      const text = cleanTimelineText(candidate)
      if (text && text !== title && !isGenericTimelineTitle(text)) {
        return trimTimelineText(text, 150)
      }
    }
    return entry.windowTitle || sourceLabel(entry)
  }
  
  function entryEvidenceBadges(entry: WorkMemoryEntry) {
    return [
      isScreenshotEntry(entry) ? '画面' : '',
      isClipboardEntry(entry) ? '剪贴板' : '',
      isOcrEntry(entry) ? 'OCR' : '',
      entry.sensitive ? '敏感' : '',
      entry.qualityStatus === 'pending' ? '待质检' : '',
    ].filter(Boolean)
  }
  
  function setTimelineFilter(filter: TimelineSourceFilter) {
    timelineSourceFilter.value = filter
    timelineAppFilter.value = 'all'
    closeTimelineAppPicker()
    resetTimelinePaging()
    clearTimelineSelection()
  }
  
  function setTimelineAppFilter(filter: string) {
    timelineAppFilter.value = filter || 'all'
    resetTimelinePaging()
    clearTimelineSelection()
  }
  
  function toggleTimelineAppPicker() {
    timelineAppPickerOpen.value = !timelineAppPickerOpen.value
    if (timelineAppPickerOpen.value) {
      void nextTick(() => timelineAppSearchRef.value?.focus())
    }
  }
  
  function closeTimelineAppPicker() {
    timelineAppPickerOpen.value = false
    timelineAppSearch.value = ''
  }
  
  function timelineExclusionLines() {
    return memory.exclusionDraft.apps
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
  }
  
  function isTimelineAppExcluded(appName?: string) {
    const normalized = String(appName || '').trim().toLowerCase()
    if (!normalized) return false
    return timelineExclusionLines().some((line) => line.toLowerCase() === normalized)
  }
  
  function closeTimelineLaneMenu() {
    timelineLaneMenu.value = {
      open: false,
      x: 0,
      y: 0,
      appName: '',
      label: '',
      count: 0,
    }
  }
  
  function openTimelineLaneMenu(event: MouseEvent, lane: TimelineLane) {
    const appName = lane.appName.trim()
    if (!appName) {
      closeTimelineLaneMenu()
      return
    }
    timelineAppPickerOpen.value = false
    const menuWidth = 252
    const menuHeight = 176
    timelineLaneMenu.value = {
      open: true,
      x: Math.max(12, Math.min(event.clientX, window.innerWidth - menuWidth - 12)),
      y: Math.max(12, Math.min(event.clientY, window.innerHeight - menuHeight - 12)),
      appName,
      label: lane.label,
      count: lane.entries.length,
    }
  }
  
  function showTimelineFeedback(message: string) {
    timelineExclusionFeedback.value = message
    window.setTimeout(() => {
      if (timelineExclusionFeedback.value === message) {
        timelineExclusionFeedback.value = ''
      }
    }, 2400)
  }
  
  function showTimelineExclusionFeedback(message: string) {
    showTimelineFeedback(message)
  }
  
  async function addTimelineLaneAppToExclusions(appName = timelineLaneMenu.value.appName) {
    const normalized = appName.trim()
    if (!normalized) {
      closeTimelineLaneMenu()
      return
    }
    const lines = timelineExclusionLines()
    if (lines.some((line) => line.toLowerCase() === normalized.toLowerCase())) {
      showTimelineExclusionFeedback(`${normalized} 已在排除名单`)
      closeTimelineLaneMenu()
      return
    }
    memory.exclusionDraft.apps = [...lines, normalized].join('\n')
    showTimelineExclusionFeedback(`正在保存排除应用：${normalized}`)
    closeTimelineLaneMenu()
    await memory.saveExclusionRules()
  }
  
  function selectTimelineAppFilter(filter: string) {
    setTimelineAppFilter(filter)
    closeTimelineAppPicker()
  }
  
  function handleTimelineAppPointerDown(event: PointerEvent) {
    if (timelineLaneMenu.value.open) {
      const target = event.target
      if (!(target instanceof Node && timelineLaneMenuRef.value?.contains(target))) {
        closeTimelineLaneMenu()
      }
    }
    if (!timelineAppPickerOpen.value) {
      return
    }
    const target = event.target
    if (target instanceof Node && timelineAppSelectRef.value?.contains(target)) {
      return
    }
    closeTimelineAppPicker()
  }
  
  function resetTimelinePaging() {
    timelineVisibleDayCount.value = 2
    setupTimelineLoadObserver()
  }
  
  function loadMoreTimelineDays() {
    if (!timelineHasMoreDays.value) {
      return
    }
    timelineVisibleDayCount.value += 1
  }
  
  function setupTimelineLoadObserver() {
    timelineLoadObserver?.disconnect()
    timelineLoadObserver = null
    if (typeof IntersectionObserver === 'undefined') {
      return
    }
    void nextTick(() => {
      const target = timelineLoadMoreRef.value
      if (!target) return
      timelineLoadObserver = new IntersectionObserver(
        (entries) => {
          if (activeFlowPage.value === 'timeline' && entries.some((entry) => entry.isIntersecting)) {
            loadMoreTimelineDays()
          }
        },
        { root: null, rootMargin: '260px 0px', threshold: 0 },
      )
      timelineLoadObserver.observe(target)
    })
  }
  
  function isTimelineSelected(id: string) {
    return timelineSelectedIdSet.value.has(id)
  }
  
  function toggleTimelineSelection(id: string) {
    if (!id) return
    timelineDeleteArmed.value = false
    if (timelineSelectedIdSet.value.has(id)) {
      timelineSelectedIds.value = timelineSelectedIds.value.filter((entryId) => entryId !== id)
      return
    }
    timelineSelectedIds.value = [...timelineSelectedIds.value, id]
  }
  
  function clearTimelineSelection() {
    timelineDeleteArmed.value = false
    timelineSelectedIds.value = []
  }
  
  function toggleCurrentTimelineSelection() {
    const currentIds = timelineEntries.value.map((entry) => entry.id).filter(Boolean)
    timelineDeleteArmed.value = false
    if (!currentIds.length) {
      showTimelineFeedback('当前没有可选择的轨迹')
      return
    }
    if (timelineAllCurrentSelected.value) {
      const current = new Set(currentIds)
      timelineSelectedIds.value = timelineSelectedIds.value.filter((id) => !current.has(id))
      showTimelineFeedback(`已取消当前 ${currentIds.length} 条轨迹`)
      return
    }
    timelineSelectedIds.value = [...new Set([...timelineSelectedIds.value, ...currentIds])]
    showTimelineFeedback(`已全选当前 ${currentIds.length} 条轨迹`)
  }
  
  function selectCurrentTimelineForRetrospective() {
    memory.selectEntriesForRetrospective(timelineEntries.value.map((entry) => entry.id))
  }
  
  function addTimelineSelectionToRetrospective() {
    memory.selectEntriesForRetrospective(timelineSelectedEntries.value.map((entry) => entry.id))
  }
  
  async function deleteTimelineSelection() {
    const ids = timelineSelectedEntries.value.map((entry) => entry.id)
    if (!ids.length) {
      showTimelineFeedback('先勾选要删除的轨迹')
      return
    }
    if (!timelineDeleteArmed.value) {
      timelineDeleteArmed.value = true
      showTimelineFeedback(`再次点击删除 ${ids.length} 条轨迹`)
      return
    }
    timelineDeleteArmed.value = false
    const removed = await memory.deleteEntries(ids)
    if (removed) {
      clearTimelineSelection()
    }
  }
  
  async function exportTimelineSelection() {
    const ids = timelineSelectedEntries.value.map((entry) => entry.id)
    if (!ids.length) {
      showTimelineFeedback('先勾选要导出的轨迹')
      return
    }
    const previous = memory.exportDraft.entryIds
    memory.exportDraft.entryIds = ids.join('\n')
    try {
      await memory.exportData()
    } finally {
      memory.exportDraft.entryIds = previous
    }
  }
  
  async function runTimelineBatchOCR() {
    const ids = timelineBatchOcrEntries.value.map((entry) => entry.id)
    if (!ids.length) {
      return
    }
    await memory.recognizeEntriesOCR(ids)
  }
  
  function setFlowSettingsTab(tab: FlowSettingsTab) {
    flowSettingsTab.value = tab
  }
  
  function appProfileId(value: string) {
    const normalized = String(value || '').trim().replace(/\\/g, '/')
    const parts = normalized.split('/')
    return (parts[parts.length - 1] || normalized).toLowerCase()
  }
  
  function displayAppName(value: string) {
    const normalized = String(value || '').trim().replace(/\\/g, '/')
    const base = normalized.split('/').filter(Boolean).pop() || normalized || '应用'
    return base.replace(/\.exe$/i, '')
  }
  
  function appAvatarText(value: string) {
    const display = displayAppName(value)
    if (!display) return 'A'
    const ascii = display.match(/[a-z0-9]/i)
    return (ascii?.[0] || display[0] || 'A').toUpperCase()
  }
  
  function ensureAppCaptureProfiles() {
    if (!settings.settings) return []
    if (!Array.isArray(settings.settings.workMemory.appCaptureProfiles)) {
      settings.settings.workMemory.appCaptureProfiles = []
    }
    return settings.settings.workMemory.appCaptureProfiles
  }
  
  function selectAppCaptureProfile(id: string) {
    selectedAppCaptureProfileId.value = id
  }
  
  function addAppCaptureProfile(candidate: CaptureAppCandidate) {
    const profiles = ensureAppCaptureProfiles()
    const id = appProfileId(candidate.processName || candidate.displayName || candidate.id)
    if (!id) return
    const existing = profiles.find((profile) => appProfileId(profile.processName || profile.displayName || profile.id) === id)
    if (existing) {
      selectedAppCaptureProfileId.value = existing.id
      return
    }
    const profile: WorkMemoryAppCaptureProfile = {
      id,
      displayName: candidate.displayName || displayAppName(candidate.processName),
      processName: candidate.processName,
      enabled: true,
      windowSwitchDelaySeconds: settings.settings?.workMemory.windowSwitchCooldownSeconds || 3,
      activeIntervalSeconds: settings.settings?.workMemory.autoCaptureIntervalSeconds || 60,
    }
    profiles.push(profile)
    selectedAppCaptureProfileId.value = id
  }
  
  function removeAppCaptureProfile(id: string) {
    if (!settings.settings) return
    const profiles = ensureAppCaptureProfiles()
    const index = profiles.findIndex((profile) => profile.id === id)
    if (index < 0) return
    profiles.splice(index, 1)
    selectedAppCaptureProfileId.value = profiles[0]?.id || ''
  }
  
  function formatTime(timestamp: number) {
    if (!timestamp) {
      return '-'
    }
    return new Date(timestamp * 1000).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }
  
  function formatDuration(seconds: number) {
    if (!seconds) return '0s'
    if (seconds < 60) return `${seconds}s`
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}m`
    const hours = Math.floor(minutes / 60)
    return `${hours}h ${minutes % 60}m`
  }
  
  function confidenceLabel(value: number) {
    return `${Math.round((value || 0) * 100)}%`
  }
  
  function autonomousKindLabel(kind: string) {
    const labels: Record<string, string> = {
      daily: '日报',
      retrospective: '复盘',
      knowledge: '知识',
      skill: 'Skill',
    }
    return labels[kind] ?? '产物'
  }
  
  function autonomousArtifactText(artifact: WorkMemoryAutonomousArtifact) {
    const lines = [
      `# ${artifact.title}`,
      '',
      artifact.summary,
      '',
      artifact.body,
      '',
      '## 留痕',
      ...(artifact.evidence.length ? artifact.evidence.map((id) => `- ${id}`) : ['- 无']),
    ]
    return lines.filter((line, index) => line || lines[index - 1]).join('\n')
  }
  
  async function copyAutonomousArtifact(artifact: WorkMemoryAutonomousArtifact) {
    await copyText(autonomousArtifactText(artifact))
    showAssetFeedback('自主产物已复制')
  }
  
  function beginRejectAutonomousArtifact(artifact: WorkMemoryAutonomousArtifact) {
    rejectingAutonomousArtifactId.value = artifact.id
    autonomousRejectReason.value = ''
  }
  
  function cancelRejectAutonomousArtifact() {
    rejectingAutonomousArtifactId.value = ''
    autonomousRejectReason.value = ''
  }
  
  async function confirmRejectAutonomousArtifact(artifact: WorkMemoryAutonomousArtifact) {
    const reason = autonomousRejectReason.value.trim() || '不符合我的工作方式'
    await memory.rejectAutonomousArtifact(artifact.id, reason)
    rejectingAutonomousArtifactId.value = ''
    autonomousRejectReason.value = ''
    showAssetFeedback('已删除，并记录了避免重复生成的原因')
  }
  
  async function runAutonomousFlow() {
    await memory.runAutonomousFlow()
  }
  
  function decisionLabel(status?: string) {
    const labels: Record<string, string> = {
      accepted: '已接受',
      rejected: '已驳回',
      later: '稍后处理',
      task_package: '已转任务包',
      workflow_draft: '已生成自动化草稿',
      checklist_draft: '已生成检查清单',
    }
    return status ? labels[status] || status : '待处理'
  }
  
  function secretSourceLabel(source: string) {
    const labels: Record<string, string> = {
      environment: '环境变量优先',
      credential_manager: '安全存储',
      missing: '未配置',
    }
    return labels[source] ?? source
  }
  
  function secretInputValue(kind: string) {
    return settings.secretInputs[kind]?.trim() ?? ''
  }
  
  function canSaveSecret(kind: string) {
    return Boolean(settings.secretStatus?.available && secretInputValue(kind) && !settings.isSaving)
  }
  
  function canClearSecret(stored: boolean) {
    return Boolean(settings.secretStatus?.available && stored && !settings.isSaving)
  }
  
  function startOfLocalDaySeconds(timestamp: number) {
    const date = new Date(timestamp * 1000)
    date.setHours(0, 0, 0, 0)
    return Math.floor(date.getTime() / 1000)
  }
  
  function isEntryInSelectedFlowDay(entry: WorkMemoryEntry) {
    return entry.createdAt >= selectedFlowDayStart.value && entry.createdAt <= selectedFlowDayEnd.value
  }
  
  function currentFlowHour() {
    const now = new Date()
    return clampFlowHour(now.getHours() + now.getMinutes() / 60)
  }
  
  function clampFlowHour(hour: number) {
    if (!Number.isFinite(hour)) return FLOW_DAY_START_HOUR
    return Math.max(FLOW_DAY_START_HOUR, Math.min(FLOW_DAY_END_HOUR, hour))
  }
  
  function normalizeTimelineZoomRange(startHour: number, endHour: number) {
    let start = clampFlowHour(Math.min(startHour, endHour))
    let end = clampFlowHour(Math.max(startHour, endHour))
    if (end - start >= MIN_TIMELINE_ZOOM_HOURS) {
      return { start, end }
    }
    const midpoint = clampFlowHour((start + end) / 2)
    start = midpoint - MIN_TIMELINE_ZOOM_HOURS / 2
    end = midpoint + MIN_TIMELINE_ZOOM_HOURS / 2
    if (start < FLOW_DAY_START_HOUR) {
      start = FLOW_DAY_START_HOUR
      end = FLOW_DAY_START_HOUR + MIN_TIMELINE_ZOOM_HOURS
    }
    if (end > FLOW_DAY_END_HOUR) {
      end = FLOW_DAY_END_HOUR
      start = FLOW_DAY_END_HOUR - MIN_TIMELINE_ZOOM_HOURS
    }
    return { start, end }
  }

  function timelineTickStepHours(spanHours: number) {
    if (spanHours <= 2) return 0.25
    if (spanHours <= 4) return 0.5
    if (spanHours <= 8) return 1
    if (spanHours <= 12) return 2
    return 3
  }

  function timelineDensityBucketSeconds(spanSeconds: number) {
    const target = spanSeconds / 48
    const candidates = [5 * 60, 10 * 60, 15 * 60, 30 * 60, 60 * 60]
    return candidates.find((seconds) => seconds >= target) ?? candidates[candidates.length - 1]
  }
  
  function timelineHourToPercent(hour: number) {
    const start = timelineZoomStartHour.value
    const end = timelineZoomEndHour.value
    const span = Math.max(MIN_TIMELINE_ZOOM_HOURS, end - start)
    return ((clampFlowHour(hour) - start) / span) * 100
  }
  
  function flowHourLabel(hour: number) {
    const clamped = clampFlowHour(hour)
    let wholeHour = Math.floor(clamped)
    let minute = Math.round((clamped - wholeHour) * 60)
    if (minute >= 60) {
      wholeHour += 1
      minute = 0
    }
    return `${String(wholeHour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`
  }
  
  function setFlowSelectedHour(hour: number) {
    selectedFlowHour.value = clampFlowHour(hour)
  }

  function setTimelineZoomRange(startHour: number, endHour: number) {
    const range = normalizeTimelineZoomRange(startHour, endHour)
    timelineZoomStartHour.value = range.start
    timelineZoomEndHour.value = range.end
    if (selectedFlowHour.value < range.start || selectedFlowHour.value > range.end) {
      selectedFlowHour.value = (range.start + range.end) / 2
    }
    resetTimelinePaging()
    clearTimelineSelection()
    closeTimelineAppPicker()
    closeTimelineLaneMenu()
  }

  function resetTimelineZoomRange() {
    timelineZoomStartHour.value = FLOW_DAY_START_HOUR
    timelineZoomEndHour.value = FLOW_DAY_END_HOUR
    resetTimelinePaging()
    closeTimelineAppPicker()
    closeTimelineLaneMenu()
  }
  
  function shiftFlowDate(days: number) {
    selectedFlowDayStart.value += days * DAY_SECONDS
    resetTimelineZoomRange()
    resetTimelinePaging()
    clearTimelineSelection()
    closeTimelineAppPicker()
  }
  
  function resetFlowDateToday() {
    selectedFlowDayStart.value = startOfLocalDaySeconds(Math.floor(Date.now() / 1000))
    selectedFlowHour.value = currentFlowHour()
    resetTimelineZoomRange()
    resetTimelinePaging()
    clearTimelineSelection()
    closeTimelineAppPicker()
  }
  
  function setFlowTimeFromPointer(event: PointerEvent) {
    const target = event.currentTarget
    if (!(target instanceof HTMLElement)) return
    const rect = target.getBoundingClientRect()
    if (!rect.width) return
    const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
    setFlowSelectedHour(timelineZoomStartHour.value + ratio * (timelineZoomEndHour.value - timelineZoomStartHour.value))
  }
  
  function adjustFlowTimeByKey(event: KeyboardEvent) {
    const step = event.shiftKey ? 1 : 0.25
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
      event.preventDefault()
      setFlowSelectedHour(selectedFlowHour.value - step)
    } else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
      event.preventDefault()
      setFlowSelectedHour(selectedFlowHour.value + step)
    } else if (event.key === 'Home') {
      event.preventDefault()
      setFlowSelectedHour(timelineZoomStartHour.value)
    } else if (event.key === 'End') {
      event.preventDefault()
      setFlowSelectedHour(timelineZoomEndHour.value)
    }
  }
  
  function createFlowChatMessage(role: FlowChatRole, text: string, patch: Partial<FlowChatMessage> = {}): FlowChatMessage {
    return {
      id: `flow-${role}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      role,
      text,
      createdAt: Math.floor(Date.now() / 1000),
      ...patch,
    }
  }

  function flowChatMessageFromStored(message: WorkMemoryFlowMessage): FlowChatMessage {
    return {
      id: message.id,
      role: message.role,
      text: message.text,
      createdAt: message.createdAt,
      question: message.question,
      result: message.result,
      error: Boolean(message.error),
    }
  }
  
  function isFlowMessageSelectable(message: FlowChatMessage) {
    return Boolean(!message.pending && !message.system && message.text.trim())
  }
  
  function isFlowMessageSelected(message: FlowChatMessage) {
    return flowChatSelectedIds.value.includes(message.id)
  }
  
  function toggleFlowMessageSelection(message: FlowChatMessage) {
    if (!isFlowMessageSelectable(message)) {
      return
    }
    if (isFlowMessageSelected(message)) {
      flowChatSelectedIds.value = flowChatSelectedIds.value.filter((id) => id !== message.id)
      return
    }
    flowChatSelectedIds.value = [...flowChatSelectedIds.value, message.id]
  }
  
  function selectSingleFlowMessage(message: FlowChatMessage) {
    if (!isFlowMessageSelectable(message)) {
      return
    }
    flowChatSelectedIds.value = [message.id]
  }
  
  function clearFlowChatSelection() {
    flowChatSelectedIds.value = []
  }
  
  function handleFlowMessageClick(message: FlowChatMessage, event: MouseEvent) {
    closeFlowMessageMenu()
    if (event.ctrlKey || event.metaKey || flowChatSelectedIds.value.length) {
      toggleFlowMessageSelection(message)
    }
  }
  
  function openFlowMessageMenu(event: MouseEvent, message: FlowChatMessage) {
    event.preventDefault()
    if (!isFlowMessageSelectable(message)) {
      return
    }
    if (!isFlowMessageSelected(message)) {
      selectSingleFlowMessage(message)
    }
    flowContextMenu.value = {
      open: true,
      x: event.clientX,
      y: event.clientY,
    }
  }
  
  function closeFlowMessageMenu() {
    if (flowContextMenu.value.open) {
      flowContextMenu.value = { ...flowContextMenu.value, open: false }
    }
  }
  
  function flowMessageRoleLabel(message: FlowChatMessage) {
    return message.role === 'user' ? '我' : '心流'
  }
  
  function flowMessageTime(message: FlowChatMessage) {
    return new Date(message.createdAt * 1000).toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  function flowConversationTime(conversation: WorkMemoryFlowConversation) {
    const time = conversation.updatedAt || conversation.createdAt
    if (!time) return ''
    return new Date(time * 1000).toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  function flowConversationPreview(conversation: WorkMemoryFlowConversation) {
    const last = conversation.lastMessage?.trim()
    if (last) return last
    return conversation.messageCount ? `${conversation.messageCount} 条消息` : '新对话'
  }
  
  function flowMessageEvidenceLabel(message: FlowChatMessage) {
    const count = message.result?.evidence.length ?? 0
    if (!count) return ''
    return `${count} 条留痕`
  }
  
  function flowMessageModeLabel(message: FlowChatMessage) {
    const result = message.result
    if (!result) return ''
    if (result.mode === 'agent_error') return 'Agent 调用失败'
    if (result.mode === 'agent:openai-agents-sdk-shell-skill') return 'OpenAI SDK Skill'
    if (result.mode === 'agent:openai-agents-sdk-function-tool-fallback') return 'OpenAI SDK 工具降级'
    if (result.mode === 'agent:openai-agents-sdk') return 'OpenAI Agents SDK'
    if (result.mode === 'agent:openai-compatible-direct') return 'OpenAI 兼容直连'
    if (result.mode === 'agent:codex') return 'Codex Agent'
    if (result.usedAi || result.mode.startsWith('agent:')) return 'Agent 生成'
    if (result.mode === 'local_search') return '本地检索'
    if (result.mode === 'local_insights') return '本地洞察'
    return '本地归纳'
  }
  
  function flowMessageModeClass(message: FlowChatMessage) {
    const mode = message.result?.mode ?? ''
    return {
      'is-agent': Boolean(message.result?.usedAi || mode.startsWith('agent:')),
      'is-local': Boolean(message.result && !message.result.usedAi && !mode.startsWith('agent:')),
      'is-error': mode === 'agent_error',
    }
  }
  
  function flowMessageHtml(message: FlowChatMessage) {
    return renderFlowMarkdown(message.text)
  }
  
  function renderFlowMarkdown(source: string) {
    const lines = String(source || '').replace(/\r\n?/g, '\n').trim().split('\n')
    const blocks: string[] = []
    let paragraph: string[] = []
    let listType: 'ul' | 'ol' | '' = ''
    let listItems: string[] = []
    let quote: string[] = []
    let inCode = false
    let codeLines: string[] = []
  
    const flushParagraph = () => {
      if (!paragraph.length) return
      blocks.push(`<p>${paragraph.map(renderInlineMarkdown).join('<br>')}</p>`)
      paragraph = []
    }
    const flushList = () => {
      if (!listType || !listItems.length) return
      blocks.push(`<${listType}>${listItems.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join('')}</${listType}>`)
      listType = ''
      listItems = []
    }
    const flushQuote = () => {
      if (!quote.length) return
      blocks.push(`<blockquote>${quote.map(renderInlineMarkdown).join('<br>')}</blockquote>`)
      quote = []
    }
    const flushCode = () => {
      blocks.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`)
      codeLines = []
    }
    const flushLoose = () => {
      flushParagraph()
      flushList()
      flushQuote()
    }
  
    for (const line of lines) {
      const trimmed = line.trim()
      if (/^```/.test(trimmed)) {
        if (inCode) {
          flushCode()
          inCode = false
        } else {
          flushLoose()
          inCode = true
        }
        continue
      }
      if (inCode) {
        codeLines.push(line)
        continue
      }
      if (!trimmed) {
        flushLoose()
        continue
      }
      const heading = /^(#{1,4})\s+(.+)$/.exec(trimmed)
      if (heading) {
        flushLoose()
        const level = Math.min(5, heading[1].length + 2)
        blocks.push(`<h${level}>${renderInlineMarkdown(heading[2])}</h${level}>`)
        continue
      }
      const unordered = /^[-*]\s+(.+)$/.exec(trimmed)
      if (unordered) {
        flushParagraph()
        flushQuote()
        if (listType && listType !== 'ul') flushList()
        listType = 'ul'
        listItems.push(unordered[1])
        continue
      }
      const ordered = /^\d+[.)]\s+(.+)$/.exec(trimmed)
      if (ordered) {
        flushParagraph()
        flushQuote()
        if (listType && listType !== 'ol') flushList()
        listType = 'ol'
        listItems.push(ordered[1])
        continue
      }
      const quoted = /^>\s?(.+)$/.exec(trimmed)
      if (quoted) {
        flushParagraph()
        flushList()
        quote.push(quoted[1])
        continue
      }
      flushList()
      flushQuote()
      paragraph.push(line)
    }
    if (inCode) flushCode()
    flushLoose()
    return blocks.join('')
  }
  
  function renderInlineMarkdown(source: string) {
    let html = escapeHtml(source)
    html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
    html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    return html
  }
  
  function escapeHtml(source: string) {
    return String(source)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;')
  }
  
  function flowTranscript(messages: FlowChatMessage[]) {
    return messages
      .map((message) => `【${flowMessageRoleLabel(message)} · ${flowMessageTime(message)}】\n${message.text.trim()}`)
      .join('\n\n')
  }
  
  async function copyText(text: string) {
    if (!text.trim()) return
    try {
      await WailsClipboard.SetText(text)
    } catch {
      await navigator.clipboard?.writeText?.(text)
    }
  }
  
  function agentTaskPackageText(task: AgentTaskPackage) {
    return [
      `# ${task.goal}`,
      '',
      '## 上下文',
      task.context,
      '',
      '## 留痕',
      ...(task.evidence.length ? task.evidence.map((item) => `- ${item}`) : ['- 无']),
      '',
      '## 边界',
      ...(task.boundaries.length ? task.boundaries.map((item) => `- ${item}`) : ['- 执行前需要用户确认范围和权限']),
      '',
      '## 验收标准',
      ...(task.acceptance.length ? task.acceptance.map((item) => `- ${item}`) : ['- 返回可验证的处理结果']),
    ].join('\n')
  }
  
  function showAssetFeedback(message: string) {
    assetFeedback.value = message
    window.setTimeout(() => {
      if (assetFeedback.value === message) {
        assetFeedback.value = ''
      }
    }, 2400)
  }
  
  function focusAsset(kind: 'agent' | 'workflow' | 'checklist' | 'skill') {
    activeAssetFocus.value = kind
    activeFlowPage.value = 'assets'
    detailDrawerOpen.value = false
    void nextTick(() => {
      document.querySelector(`[data-flow-asset="${kind}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    })
  }
  
  async function buildCurrentMemoryTaskPackage() {
    const ok = await memory.buildAgentTask()
    if (ok) {
      focusAsset('agent')
      showAssetFeedback('任务包已生成，复制后可交给 Codex 或其他代理处理')
    }
  }
  
  async function handoffInsightToAgent(insight: ExperienceInsight) {
    const ok = await memory.buildAgentTaskFromInsight(insight)
    if (ok) {
      focusAsset('agent')
      showAssetFeedback('任务包已生成，复制后可交给 Codex 或其他代理处理')
    }
  }
  
  async function buildAutomationFromInsight(insight: ExperienceInsight) {
    const ok = await memory.buildWorkflowDraftFromInsight(insight)
    if (ok) {
      focusAsset('workflow')
      showAssetFeedback('自动化草稿已生成，确认后可保存到工作流')
    }
  }
  
  async function buildChecklistFromInsight(insight: ExperienceInsight) {
    const ok = await memory.buildChecklistDraftFromInsight(insight)
    if (ok) {
      focusAsset('checklist')
      showAssetFeedback('检查清单已生成，确认后可保存为清单')
    }
  }
  
  async function copyCurrentAgentTask() {
    if (!memory.agentTask) {
      showAssetFeedback('还没有任务包')
      return
    }
    await copyText(agentTaskPackageText(memory.agentTask))
    showAssetFeedback('任务包已复制')
  }
  
  async function copyFlowMessage(message: FlowChatMessage) {
    await copyText(message.text)
    showFlowRememberFeedback('已复制')
  }
  
  async function copySelectedFlowMessages() {
    const messages = selectedFlowChatMessages.value
    if (!messages.length) {
      showFlowRememberFeedback('先选择消息')
      return
    }
    await copyText(flowTranscript(messages))
    showFlowRememberFeedback('已复制选中对话')
  }
  
  async function rememberSelectedFlowMessages() {
    closeFlowMessageMenu()
    const messages = selectedFlowChatMessages.value
    if (!messages.length) {
      showFlowRememberFeedback('先选择消息')
      return
    }
    const firstText = messages[0]?.text.replace(/\s+/g, ' ').trim() || '心流对话'
    const entry = await memory.addManualNote({
      title: messages.length === 1 ? `心流对话：${firstText.slice(0, 28)}` : `心流对话：${messages.length} 条消息`,
      text: flowTranscript(messages),
      tags: ['心流', '对话沉淀'],
      favorite: false,
      sensitive: false,
    })
    if (entry?.id) {
      clearFlowChatSelection()
      showFlowRememberFeedback('已加入沉淀')
    }
  }
  
  function showFlowRememberFeedback(message: string) {
    flowRememberFeedback.value = message
    window.setTimeout(() => {
      if (flowRememberFeedback.value === message) {
        flowRememberFeedback.value = ''
      }
    }, 1400)
  }
  
  async function scrollFlowChatToBottom() {
    await nextTick()
    const thread = flowChatThreadRef.value
    if (thread) {
      thread.scrollTop = thread.scrollHeight
    }
  }

  async function startFlowConversation() {
    closeFlowMessageMenu()
    clearFlowChatSelection()
    flowPendingMessages.value = []
    await memory.startFlowConversation('新对话')
    await scrollFlowChatToBottom()
  }

  async function selectFlowConversation(id: string) {
    closeFlowMessageMenu()
    clearFlowChatSelection()
    flowPendingMessages.value = []
    await memory.selectFlowConversation(id)
    flowCanvasActiveId.value = ''
    await scrollFlowChatToBottom()
  }
  
  async function askFlow(question = flowQuestion.value) {
    const normalized = question.trim()
    if (!normalized || flowBusy.value || memory.isAskingFlow) {
      return
    }
    closeFlowMessageMenu()
    flowQuestion.value = ''
    flowBusy.value = true
    const pendingId = `flow-assistant-pending-${Date.now()}`
    flowPendingMessages.value = [
      createFlowChatMessage('user', normalized),
      createFlowChatMessage('assistant', '正在整理本地上下文...', { id: pendingId, question: normalized, pending: true }),
    ]
    clearFlowChatSelection()
    await scrollFlowChatToBottom()
    try {
      const result = await memory.askFlow(normalized, 8)
      const answer = result.answer || result.message || (result.ok ? '我没有整理出稳定结论，可以换个问法继续追问。' : '心流问答暂时不可用。')
      if (!memory.flowMessages.length) {
        flowPendingMessages.value = [
          createFlowChatMessage('user', normalized),
          createFlowChatMessage('assistant', answer, {
            id: pendingId,
            question: normalized,
            createdAt: result.createdAt || Math.floor(Date.now() / 1000),
            result,
            error: !result.ok,
          }),
        ]
      } else {
        flowPendingMessages.value = []
      }
    } finally {
      flowBusy.value = false
      if (memory.flowMessages.length) {
        flowPendingMessages.value = []
      }
      await scrollFlowChatToBottom()
    }
  }
  
  function useFlowQuestion(question: string) {
    void askFlow(question)
  }
  
  function focusFlowChatInput(event?: PointerEvent) {
    const target = event?.target as HTMLElement | null
    if (target?.closest('button')) {
      return
    }
    flowChatInputRef.value?.focus()
  }

  function toggleFlowSidebar() {
    flowSidebarCollapsed.value = !flowSidebarCollapsed.value
  }
  
  function openFlowPage(page: FlowPage) {
    activeFlowPage.value = page
    detailDrawerOpen.value = false
    closeTimelineLaneMenu()
  }
  
  function openEvidence(entry: WorkMemoryEntry) {
    memory.select(entry.id)
    detailDrawerOpen.value = true
  }
  
  function openFlowSettings() {
    flowSettingsTab.value = 'capture'
    flowSettingsOpen.value = true
    if (!settings.settings) {
      void settings.load()
    }
  }
  
  async function saveFlowSettings() {
    await settings.save()
    await memory.load()
  }
  
  const flowPageContext = reactive({
    AriButton: markRaw(AriButton),
    ArrowLeft: markRaw(ArrowLeft),
    ArrowRight: markRaw(ArrowRight),
    Brain: markRaw(Brain),
    Camera: markRaw(Camera),
    Check: markRaw(Check),
    ChevronDown: markRaw(ChevronDown),
    Clock3: markRaw(Clock3),
    Copy: markRaw(Copy),
    Database: markRaw(Database),
    Download: markRaw(Download),
    FileText: markRaw(FileText),
    Flag: markRaw(Flag),
    ImageOff: markRaw(ImageOff),
    KeyRound: markRaw(KeyRound),
    Play: markRaw(Play),
    Plus: markRaw(Plus),
    RefreshCw: markRaw(RefreshCw),
    Search: markRaw(Search),
    Settings: markRaw(Settings),
    Shield: markRaw(Shield),
    Sparkles: markRaw(Sparkles),
    Tags: markRaw(Tags),
    Trash2: markRaw(Trash2),
    Upload: markRaw(Upload),
    Workflow: markRaw(Workflow),
    X: markRaw(X),
    DAY_SECONDS,
    FLOW_DAY_END_HOUR,
    FLOW_DAY_START_HOUR,
    activeAssetFocus,
    activeDraft,
    activeDraftKind,
    activeDraftSourceSummary,
    activeFlowConversation,
    activeFlowConversationId,
    activeFlowPage,
    activeInsightId,
    addAppCaptureProfile,
    addTimelineLaneAppToExclusions,
    addTimelineSelectionToRetrospective,
    adjustFlowTimeByKey,
    appAvatarText,
    appCaptureCandidates,
    appCaptureProfiles,
    appShell,
    askFlow,
    askedEvidenceEntries,
    assetFeedback,
    assetMissingEvidence,
    assetReadinessParts,
    assetReadinessScore,
    autonomousInboxSummary,
    autonomousKindLabel,
    autonomousRejectReason,
    batchOcrProgressPercent,
    beginRejectAutonomousArtifact,
    buildAutomationFromInsight,
    buildChecklistFromInsight,
    buildCurrentMemoryTaskPackage,
    canClearSecret,
    canRunTimelineOCR,
    canSaveSecret,
    cancelRejectAutonomousArtifact,
    captureScopeLabel,
    captureSourceCards,
    clearFlowChatSelection,
    clearTimelineSelection,
    closeFlowMessageMenu,
    closeTimelineAppPicker,
    closeTimelineLaneMenu,
    confidenceLabel,
    confirmRejectAutonomousArtifact,
    copyAutonomousArtifact,
    copyCurrentAgentTask,
    copyFlowMessage,
    copySelectedFlowMessages,
    copyTimelineSelectionReference,
    decisionLabel,
    deleteProgressPercent,
    deleteTimelineSelection,
    detailDrawerOpen,
    displayAppName,
    draftEvidenceTimeline,
    draftItems,
    draftTimelineEntries,
    entryEvidenceBadges,
    entryFocusSummary,
    entryFocusTitle,
    evidenceCounts,
    evidenceEntriesFromIds,
    exclusionRuleRows,
    exclusionRuleTabs,
    exportTimelineSelection,
    filteredTimelineAppOptions,
    flowBusy,
    flowCanvasActiveId,
    flowCanvasPrimaryEntry,
    flowChatInputRef,
    flowChatIsEmpty,
    flowChatMessages,
    flowChatSelectedIds,
    flowChatThreadRef,
    flowContextMenu,
    flowConversationPreview,
    flowConversationTime,
    flowConversations,
    flowCurrentClock,
    flowDateButtonLabel,
    flowDateLabel,
    flowMessageEvidenceLabel,
    flowMessageHtml,
    flowMessageModeClass,
    flowMessageModeLabel,
    flowMessageRoleLabel,
    flowMessageTime,
    flowPages,
    flowQuestion,
    flowRememberFeedback,
    flowSidebarCollapsed,
    flowSelectionLabel,
    flowSettingsOpen,
    flowSettingsTab,
    flowSettingsTabs,
    flowSuggestedQuestions,
    flowTimeRangeLabel,
    flowTimeRulerNowPercent,
    flowTimeRulerTicks,
    flowWindowPanelItems,
    flowWorkHoursLabel,
    focusAsset,
    focusFlowChatInput,
    formatDuration,
    formatTime,
    formatTimelineClock,
    globalFlowSearch,
    globalSearchPlaceholder,
    handleFlowMessageClick,
    handoffInsightToAgent,
    insightEvidencePreview,
    insightLinks,
    insightNodeStyle,
    insightNodes,
    insightProgressPercent,
    isClipboardEntry,
    isFlowMessageSelectable,
    isFlowMessageSelected,
    isNoteEntry,
    isOcrEntry,
    isScreenshotEntry,
    isTimelineAppExcluded,
    isTimelineSelected,
    loadMoreTimelineDays,
    memory,
    multiMonitorLabel,
    openEvidence,
    openFlowPage,
    openFlowSettings,
    openFlowMessageMenu,
    openTimelineLaneMenu,
    openTimelinePlaybackDetail,
    openTimelinePlaybackTick,
    recentEvidence,
    rejectingAutonomousArtifactId,
    rememberSelectedFlowMessages,
    removeAppCaptureProfile,
    resetFlowDateToday,
    resetTimelineZoomRange,
    rulesImpactStats,
    rulesPipelineStatus,
    runAutonomousFlow,
    runGlobalFlowSearch,
    runTimelineBatchOCR,
    runtimeStatusText,
    saveFlowSettings,
    secretInputValue,
    secretSourceLabel,
    selectAppCaptureProfile,
    selectCurrentTimelineForRetrospective,
    selectFlowConversation,
    selectSingleFlowMessage,
    selectTimelineAppFilter,
    selected,
    selectedAppCaptureProfile,
    selectedAppCaptureProfileId,
    selectedFlowChatMessages,
    selectedFlowHour,
    selectedInsight,
    selectedTimelineAppLabel,
    selectedTimelineAppCount,
    setActiveDraftKind,
    setActiveInsight,
    setFlowSettingsTab,
    setFlowTimeFromPointer,
    setTimelineFilter,
    setTimelineZoomRange,
    settings,
    shiftFlowDate,
    sourceLabel,
    startFlowConversation,
    timeMachineLabel,
    timelineAllCurrentSelected,
    timelineAppFilter,
    timelineAppOptions,
    timelineAppPickerOpen,
    timelineAppSearch,
    timelineAppSearchRef,
    timelineAppSelectRef,
    timelineAxisTicks,
    timelineBatchOcrEntries,
    timelineDeleteLabel,
    timelineDensityBars,
    timelineEntries,
    timelineEventStyle,
    timelineExclusionFeedback,
    timelineFilterCounts,
    timelineFilters,
    timelineHasMoreDays,
    timelineLaneMenu,
    timelineLaneMenuRef,
    timelineLanes,
    timelineLoadMoreRef,
    timelinePlayStateLabel,
    timelineScrubPercent,
    timelineSelectAllLabel,
    timelineSelectedEntries,
    timelineSelectedSummary,
    timelineRangeSourceEntries,
    timelineSourceEntries,
    timelineSourceFilter,
    timelineStats,
    timelineThumbnailIsMissing,
    timelineThumbnailUrl,
    timelineZoomActive,
    timelineZoomEndHour,
    timelineZoomLabel,
    timelineZoomStartHour,
    todayEntries,
    toggleCurrentTimelineSelection,
    toggleFlowSidebar,
    toggleFlowMessageSelection,
    toggleTimelineAppPicker,
    toggleTimelineSelection,
    topApps,
    useFlowQuestion,
    vectorProviderLabel,
    vectorStatusLabel,
    vectorStoreLabel,
  })
  
  provide(workMemoryFlowContextKey, flowPageContext)
  
  onMounted(() => {
    void memory.load()
    uninstallMemoryLiveUpdates = memory.installLiveUpdates()
    void settings.load()
    setupTimelineLoadObserver()
    window.addEventListener('pointerdown', handleTimelineAppPointerDown, true)
  })
  
  watch(
    () => [activeFlowPage.value, visibleTimelineDayGroups.value.length, timelineHasMoreDays.value],
    () => setupTimelineLoadObserver(),
    { flush: 'post' },
  )
  
  watch(
    () => [activeFlowPage.value, timelineEntries.value.map((entry) => `${entry.id}:${entry.captureId ?? ''}`).join('|')],
    () => primeTimelineThumbnails(timelineEntries.value),
    { immediate: true, flush: 'post' },
  )
  
  onBeforeUnmount(() => {
    uninstallMemoryLiveUpdates?.()
    uninstallMemoryLiveUpdates = null
    timelineLoadObserver?.disconnect()
    timelineLoadObserver = null
    window.removeEventListener('pointerdown', handleTimelineAppPointerDown, true)
  })

  return {
    activeFlowPage,
    flowPageContext,
  }
}

