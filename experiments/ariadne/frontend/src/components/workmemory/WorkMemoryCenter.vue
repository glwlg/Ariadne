<script setup lang="ts">
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
  KeyRound,
  Pause,
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
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useSettingsStore } from '../../stores/settings'
import { useWorkMemoryStore } from '../../stores/workMemory'
import type { AgentTaskPackage, ExperienceInsight, WorkMemoryAppCaptureProfile, WorkMemoryAutonomousArtifact, WorkMemoryEntry, WorkMemoryFlowAskResponse } from '../../types/ariadne'

const appShell = useAppShellStore()
const settings = useSettingsStore()
const memory = useWorkMemoryStore()
type FlowPage = 'flow' | 'timeline' | 'insights' | 'drafts' | 'assets' | 'rules'
type TimelineSourceFilter = 'all' | 'screenshots' | 'clipboard' | 'notes' | 'ocr'
type FlowSettingsTab = 'capture' | 'model' | 'privacy'
type FlowChatRole = 'user' | 'assistant'

interface FlowChatMessage {
  id: string
  role: FlowChatRole
  text: string
  createdAt: number
  question?: string
  result?: WorkMemoryFlowAskResponse
  pending?: boolean
  error?: boolean
  system?: boolean
}

interface CaptureAppCandidate {
  id: string
  displayName: string
  processName: string
  count: number
}

interface TimelineAppOption {
  id: string
  label: string
  count: number
}

interface TimelineDayGroup {
  id: string
  label: string
  note: string
  entries: WorkMemoryEntry[]
}

const selected = computed(() => memory.selectedEntry)
const visibleEntries = computed(() => memory.filteredEntries)
const activeFlowPage = ref<FlowPage>('flow')
const activeAssetFocus = ref<'agent' | 'workflow' | 'checklist' | ''>('')
const flowQuestion = ref('')
const flowBusy = ref(false)
const flowChatThreadRef = ref<HTMLElement | null>(null)
const flowChatInputRef = ref<HTMLTextAreaElement | null>(null)
const flowChatMessages = ref<FlowChatMessage[]>([
  {
    id: 'flow-welcome',
    role: 'assistant',
    text: '我是心流。你可以直接问今天干了什么、谁找过你、哪些流程可以优化。当前对话只留在这次会话里，需要沉淀时选中消息再加入。',
    createdAt: Math.floor(Date.now() / 1000),
    system: true,
  },
])
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
const evidenceExpanded = ref(false)
const detailDrawerOpen = ref(false)
const timelineSourceFilter = ref<TimelineSourceFilter>('all')
const timelineAppFilter = ref('all')
const timelineAppPickerOpen = ref(false)
const timelineAppSearch = ref('')
const timelineAppSelectRef = ref<HTMLElement | null>(null)
const timelineAppSearchRef = ref<HTMLInputElement | null>(null)
const timelineSelectedIds = ref<string[]>([])
const timelineDeleteArmed = ref(false)
const timelineVisibleDayCount = ref(2)
const timelineLoadMoreRef = ref<HTMLElement | null>(null)
const assetFeedback = ref('')
let timelineLoadObserver: IntersectionObserver | null = null
const flowPages = [
  { id: 'flow' as const, label: '心流', detail: '对话', icon: Brain },
  { id: 'timeline' as const, label: '时间线', detail: '回放', icon: Clock3 },
  { id: 'insights' as const, label: '洞察', detail: '归纳', icon: Sparkles },
  { id: 'drafts' as const, label: '草稿', detail: '输出', icon: FileText },
  { id: 'assets' as const, label: '资产', detail: '能力', icon: Database },
  { id: 'rules' as const, label: '规则', detail: '边界', icon: Shield },
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
const todayEntries = computed(() => {
  const start = startOfToday()
  return memory.entries.filter((entry) => entry.createdAt >= start)
})
const askedEvidenceEntries = computed(() => {
  const evidence = memory.flowAskResult?.evidence ?? []
  if (!evidence.length) return []
  const byId = new Map(memory.entries.map((entry) => [entry.id, entry]))
  return evidence.map((item) => byId.get(item.id)).filter(Boolean) as WorkMemoryEntry[]
})
const recentEvidence = computed(() => {
  if (askedEvidenceEntries.value.length) {
    return askedEvidenceEntries.value
  }
  return (todayEntries.value.length ? todayEntries.value : memory.entries).slice(0, 8)
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
  const entries = todayEntries.value.length ? todayEntries.value : memory.entries
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
const selectableFlowChatMessages = computed(() => flowChatMessages.value.filter(isFlowMessageSelectable))
const selectedFlowChatMessages = computed(() => {
  const selectedIds = new Set(flowChatSelectedIds.value)
  return selectableFlowChatMessages.value.filter((message) => selectedIds.has(message.id))
})
const flowSelectionLabel = computed(() => {
  const count = selectedFlowChatMessages.value.length
  return count ? `已选 ${count} 条` : '右键或勾选消息后加入沉淀'
})
const flowChatStatusText = computed(() => {
  const count = todayEntries.value.length || memory.status.entryCount
  const evidenceCount = evidenceCounts.value.screenshots + evidenceCounts.value.clipboard + evidenceCounts.value.ocr + evidenceCounts.value.notes
  if (!count) return '等待上下文'
  return `${count} 条上下文 · ${evidenceCount} 条证据线索`
})
const evidenceCounts = computed(() => {
  const entries = todayEntries.value.length ? todayEntries.value : memory.entries
  return {
    screenshots: entries.filter((entry) => Boolean(entry.imagePath || entry.captureId)).length,
    clipboard: entries.filter((entry) => /clipboard/.test(entry.source)).length,
    notes: entries.filter((entry) => /note|manual_note/.test(entry.source)).length,
    ocr: entries.filter((entry) => Boolean(entry.ocrText || entry.ocrStatus)).length,
  }
})
const timelineFilterCounts = computed<Record<TimelineSourceFilter, number>>(() => {
  const entries = visibleEntries.value
  return {
    all: entries.length,
    screenshots: entries.filter(isScreenshotEntry).length,
    clipboard: entries.filter(isClipboardEntry).length,
    notes: entries.filter(isNoteEntry).length,
    ocr: entries.filter(isOcrEntry).length,
  }
})
const timelineSourceEntries = computed(() => {
  const entries = visibleEntries.value
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
const timelineAppOptions = computed<TimelineAppOption[]>(() => {
  const byApp = new Map<string, TimelineAppOption>()
  for (const entry of timelineSourceEntries.value) {
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
  const entries = timelineSourceEntries.value
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
  if (timelineAppFilter.value === 'all') return timelineSourceEntries.value.length
  return timelineAppOptions.value.find((option) => option.id === timelineAppFilter.value)?.count ?? timelineEntries.value.length
})
const timelineSelectionSummary = computed(() => {
  const count = timelineSelectedEntries.value.length
  if (!count) return '未选择轨迹'
  const sensitive = timelineSelectedEntries.value.filter((entry) => entry.sensitive).length
  return sensitive ? `已选 ${count} 条，含 ${sensitive} 条敏感` : `已选 ${count} 条`
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
  const selected = timelineSelectedEntries.value.filter(canRunTimelineOCR)
  if (selected.length) {
    return selected
  }
  return timelineEntries.value.filter(needsOCRSummary)
})
const timelineBatchOcrLabel = computed(() => {
  if (memory.isBatchRecognizingOCR) return 'OCR+总结中'
  const count = timelineBatchOcrEntries.value.length
  if (timelineSelectedEntries.value.length) return count ? `OCR+总结 ${count} 条` : 'OCR+总结'
  return count ? `补跑 OCR+总结 ${count}` : '批量 OCR+总结'
})
const insightProgressPercent = computed(() => Math.min(100, Math.max(0, memory.experienceDiscoveryProgress || 0)))
const timelineSelectedSummary = computed(() => {
  if (!memory.retrospectiveSelectionCount) return '还没有选择复盘证据。可以只勾几条关键轨迹，或者一键选择当前筛选结果。'
  return `已选择 ${memory.retrospectiveSelectionCount} 条证据，后续可生成复盘、日报或任务包。`
})

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

function canRunTimelineOCR(entry: WorkMemoryEntry) {
  return Boolean(entry.imagePath) && !entry.sensitive && entry.qualityStatus !== 'pending'
}

function needsOCRSummary(entry: WorkMemoryEntry) {
  if (!canRunTimelineOCR(entry)) {
    return false
  }
  if (!entry.ocrText || !String(entry.ocrStatus || '').startsWith('done')) {
    return true
  }
  return isGenericTimelineTitle(entry.title) || isGenericTimelineTitle(entry.summary)
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

function selectTimelineAppFilter(filter: string) {
  setTimelineAppFilter(filter)
  closeTimelineAppPicker()
}

function handleTimelineAppPointerDown(event: PointerEvent) {
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

function selectCurrentTimelineEntries() {
  timelineDeleteArmed.value = false
  timelineSelectedIds.value = timelineEntries.value.map((entry) => entry.id)
}

function clearTimelineSelection() {
  timelineDeleteArmed.value = false
  timelineSelectedIds.value = []
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
    return
  }
  if (!timelineDeleteArmed.value) {
    timelineDeleteArmed.value = true
    return
  }
  const deleted = await memory.deleteEntries(ids)
  if (deleted) {
    clearTimelineSelection()
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
    activeIntervalSeconds: Math.min(settings.settings?.workMemory.autoCaptureIntervalSeconds || 30, 30),
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
    '## 证据',
    ...(artifact.evidence.length ? artifact.evidence.map((id) => `- ${id}`) : ['- 无']),
  ]
  return lines.filter((line, index) => line || lines[index - 1]).join('\n')
}

async function copyAutonomousArtifact(artifact: WorkMemoryAutonomousArtifact) {
  await copyText(autonomousArtifactText(artifact))
  showAssetFeedback('自主产物已复制')
}

async function rejectAutonomousArtifact(artifact: WorkMemoryAutonomousArtifact) {
  const reason = window.prompt(`删除「${artifact.title}」的原因？心流会用这个原因避免再次生成同类产物。`, '')
  if (reason === null) {
    return
  }
  await memory.rejectAutonomousArtifact(artifact.id, reason.trim() || '我不需要这个自主产物')
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

function startOfToday() {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  return Math.floor(date.getTime() / 1000)
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

function flowMessageEvidenceLabel(message: FlowChatMessage) {
  const count = message.result?.evidence.length ?? 0
  if (!count) return ''
  return `${count} 条证据`
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
    '## 证据',
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

function focusAsset(kind: 'agent' | 'workflow' | 'checklist') {
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

async function askFlow(question = flowQuestion.value) {
  const normalized = question.trim()
  if (!normalized || flowBusy.value || memory.isAskingFlow) {
    return
  }
  closeFlowMessageMenu()
  flowQuestion.value = ''
  flowBusy.value = true
  const pendingId = `flow-assistant-pending-${Date.now()}`
  flowChatMessages.value = [
    ...flowChatMessages.value,
    createFlowChatMessage('user', normalized),
    createFlowChatMessage('assistant', '正在整理本地上下文...', { id: pendingId, question: normalized, pending: true }),
  ]
  clearFlowChatSelection()
  await scrollFlowChatToBottom()
  try {
    const result = await memory.askFlow(normalized, 8)
    const answer = result.answer || result.message || (result.ok ? '我没有整理出稳定结论，可以换个问法继续追问。' : '心流问答暂时不可用。')
    flowChatMessages.value = flowChatMessages.value.map((message) =>
      message.id === pendingId
        ? {
            ...message,
            text: answer,
            createdAt: result.createdAt || Math.floor(Date.now() / 1000),
            result,
            pending: false,
            error: !result.ok,
          }
        : message,
    )
  } finally {
    flowBusy.value = false
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

function openFlowPage(page: FlowPage) {
  activeFlowPage.value = page
  detailDrawerOpen.value = false
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

function evidenceLabel() {
  const count = recentEvidence.value.length
  return evidenceExpanded.value ? '收起证据' : `查看 ${count} 条证据`
}

onMounted(() => {
  void memory.load()
  void settings.load()
  setupTimelineLoadObserver()
  window.addEventListener('pointerdown', handleTimelineAppPointerDown, true)
})

watch(
  () => [activeFlowPage.value, visibleTimelineDayGroups.value.length, timelineHasMoreDays.value],
  () => setupTimelineLoadObserver(),
  { flush: 'post' },
)

onBeforeUnmount(() => {
  timelineLoadObserver?.disconnect()
  timelineLoadObserver = null
  window.removeEventListener('pointerdown', handleTimelineAppPointerDown, true)
})
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell memory-shell flow-shell" aria-label="Ariadne flow center">
        <div class="flow-app-shell">
          <aside class="flow-sidebar" aria-label="心流导航">
            <div class="flow-sidebar-brand">
              <div class="flow-logo-mark" aria-hidden="true">
                <img src="/favicon.svg" alt="" />
              </div>
              <div>
                <strong>心流</strong>
                <span>你的第二大脑</span>
              </div>
            </div>

            <nav class="flow-side-nav">
              <button
                v-for="page in flowPages"
                :key="page.id"
                type="button"
                class="flow-side-nav-item"
                :class="{ 'is-active': activeFlowPage === page.id }"
                @click="openFlowPage(page.id)"
              >
                <component :is="page.icon" :size="22" />
                <span>{{ page.label }}</span>
              </button>
            </nav>

            <div class="flow-sidebar-footer">
              <button type="button" class="flow-side-nav-item" @click="openFlowSettings()">
                <Settings :size="22" />
                <span>设置</span>
              </button>
              <button type="button" class="flow-side-nav-item" @click="appShell.openLauncher()">
                <ArrowLeft :size="22" />
                <span>收起</span>
              </button>
            </div>
          </aside>

          <section class="flow-stage">
            <header class="flow-stage-top">
              <div class="flow-center-pill">
                <span aria-hidden="true"></span>
                自动整理中 · 自建模型
              </div>
              <div class="flow-stage-actions">
                <button type="button" class="flow-soft-action" :disabled="memory.isRunningAutonomousFlow" @click="runAutonomousFlow()">
                  <RefreshCw :size="14" />
                  {{ memory.isRunningAutonomousFlow ? '整理中' : '自主整理' }}
                </button>
                <button
                  type="button"
                  class="flow-soft-action"
                  :disabled="memory.proactiveSourcesEnabled"
                  @click="memory.enableProactiveSinking()"
                >
                  <Sparkles :size="14" />
                  {{ memory.proactiveSourcesEnabled ? '主动沉淀已开' : '开启主动沉淀' }}
                </button>
                <button type="button" class="flow-soft-action" @click="memory.toggleTimeMachine()">
                  <component :is="memory.status.timeMachineEnabled ? Pause : Play" :size="14" />
                  {{ memory.status.timeMachineEnabled ? `采集${timeMachineLabel}` : '开启采集' }}
                </button>
                <button
                  type="button"
                  class="flow-soft-action"
                  :class="{ 'is-active': memory.status.privacyMode }"
                  @click="memory.togglePrivacyMode()"
                >
                  <Shield :size="14" />
                  {{ memory.status.privacyMode ? '隐私模式' : '本地优先' }}
                </button>
              </div>
            </header>

        <section v-if="activeFlowPage === 'flow'" class="flow-home flow-chat-home" aria-label="我与心流的对话" @click="closeFlowMessageMenu()">
          <section class="flow-chat-panel" data-no-drag>
            <header class="flow-chat-header">
              <div class="flow-chat-title">
                <span>
                  <Brain :size="16" />
                  我与心流的对话
                </span>
                <h1>直接问，把上下文交给心流整理</h1>
                <p>对话默认不进入沉淀，选中消息后再加入。</p>
              </div>
              <div class="flow-chat-status">
                <strong>{{ flowChatStatusText }}</strong>
                <small>{{ vectorProviderLabel }} · {{ vectorStoreLabel }}</small>
              </div>
            </header>

            <section v-if="memory.autonomousArtifacts.length" class="flow-auto-inbox" data-no-drag>
              <div class="flow-auto-inbox-head">
                <div>
                  <span>自主沉淀</span>
                  <strong>未删除即默认采纳</strong>
                </div>
                <button type="button" :disabled="memory.isRunningAutonomousFlow" @click.stop="runAutonomousFlow()">
                  <RefreshCw :size="14" />
                  {{ memory.isRunningAutonomousFlow ? '整理中' : '立即整理' }}
                </button>
              </div>
              <div class="flow-auto-artifacts">
                <article v-for="artifact in memory.autonomousArtifacts.slice(0, 3)" :key="artifact.id" class="flow-auto-artifact">
                  <div class="flow-auto-artifact-kicker">
                    <span>{{ autonomousKindLabel(artifact.kind) }}</span>
                    <small v-if="artifact.confidence">{{ confidenceLabel(artifact.confidence) }}</small>
                    <small v-if="artifact.agentExecutable">agent 可执行</small>
                  </div>
                  <h3>{{ artifact.title }}</h3>
                  <p>{{ artifact.summary }}</p>
                  <div class="flow-auto-artifact-foot">
                    <span>证据 {{ artifact.evidence.length }} 条</span>
                    <button type="button" @click.stop="copyAutonomousArtifact(artifact)">
                      <Copy :size="13" />
                      复制
                    </button>
                    <button type="button" @click.stop="rejectAutonomousArtifact(artifact)">
                      <Trash2 :size="13" />
                      删除
                    </button>
                  </div>
                </article>
              </div>
            </section>

            <div v-if="selectedFlowChatMessages.length" class="flow-selection-bar">
              <span>{{ flowSelectionLabel }}</span>
              <button type="button" @click.stop="copySelectedFlowMessages()">
                <Copy :size="14" />
                复制
              </button>
              <button type="button" @click.stop="rememberSelectedFlowMessages()">
                <Plus :size="14" />
                {{ flowRememberFeedback || '加入沉淀' }}
              </button>
              <button type="button" @click.stop="clearFlowChatSelection()">
                <X :size="14" />
                取消
              </button>
            </div>

            <div ref="flowChatThreadRef" class="flow-chat-thread" data-no-drag>
              <article
                v-for="message in flowChatMessages"
                :key="message.id"
                class="flow-message"
                :class="{
                  'is-user': message.role === 'user',
                  'is-assistant': message.role === 'assistant',
                  'is-selected': isFlowMessageSelected(message),
                  'is-pending': message.pending,
                  'is-error': message.error,
                }"
                @click="handleFlowMessageClick(message, $event)"
                @contextmenu="openFlowMessageMenu($event, message)"
              >
                <button
                  v-if="isFlowMessageSelectable(message)"
                  type="button"
                  class="flow-message-selector"
                  :aria-pressed="isFlowMessageSelected(message)"
                  :aria-label="isFlowMessageSelected(message) ? '取消选择消息' : '选择消息'"
                  @click.stop="toggleFlowMessageSelection(message)"
                >
                  <Check v-if="isFlowMessageSelected(message)" :size="13" />
                </button>
                <div class="flow-message-avatar" aria-hidden="true">
                  <span v-if="message.role === 'user'">我</span>
                  <Sparkles v-else :size="20" />
                </div>
                <div class="flow-message-bubble" data-no-drag>
                  <div class="flow-message-meta">
                    <strong>{{ flowMessageRoleLabel(message) }}</strong>
                    <small>{{ flowMessageTime(message) }}</small>
                  </div>
                  <p class="flow-message-text" data-no-drag>{{ message.text }}</p>
                  <div v-if="message.result?.evidence.length" class="flow-message-foot">
                    <button type="button" @click.stop="evidenceExpanded = true">
                      <Camera :size="14" />
                      {{ flowMessageEvidenceLabel(message) }}
                    </button>
                    <span>{{ message.result.usedAi ? 'AI 组织' : '本地归纳' }}</span>
                  </div>
                  <div v-if="isFlowMessageSelectable(message)" class="flow-message-actions">
                    <button type="button" @click.stop="copyFlowMessage(message)">
                      <Copy :size="13" />
                      复制
                    </button>
                    <button type="button" @click.stop="selectSingleFlowMessage(message); rememberSelectedFlowMessages()">
                      <Plus :size="13" />
                      沉淀
                    </button>
                  </div>
                </div>
              </article>
            </div>

            <footer class="flow-chat-composer" data-no-drag>
              <div class="flow-chat-question-chips">
                <button v-for="question in flowSuggestedQuestions" :key="question" type="button" @click="useFlowQuestion(question)">
                  {{ question }}
                </button>
              </div>
              <div class="flow-chat-input-row" @pointerdown="focusFlowChatInput">
                <textarea
                  ref="flowChatInputRef"
                  v-model="flowQuestion"
                  spellcheck="false"
                  placeholder="问心流，比如：今天哪些人找过我"
                  @keydown.enter.exact.prevent="askFlow()"
                  @keydown.ctrl.enter.prevent="askFlow()"
                />
                <button type="button" class="flow-send-button" :disabled="flowBusy || memory.isAskingFlow || !flowQuestion.trim()" aria-label="发送给心流" @click="askFlow()">
                  <ArrowRight :size="22" />
                </button>
              </div>
              <div class="flow-evidence-summary flow-chat-evidence-summary">
                <strong>证据</strong>
                <span>
                  <Camera :size="14" />
                  {{ evidenceCounts.screenshots }}
                </span>
                <span>
                  <FileText :size="14" />
                  {{ evidenceCounts.ocr }}
                </span>
                <span>
                  <Copy :size="14" />
                  {{ evidenceCounts.clipboard }}
                </span>
                <button type="button" @click.stop="evidenceExpanded = !evidenceExpanded">{{ evidenceLabel() }}</button>
                <small v-if="flowRememberFeedback && !selectedFlowChatMessages.length">{{ flowRememberFeedback }}</small>
              </div>
            </footer>
          </section>

          <div
            v-if="flowContextMenu.open"
            class="flow-chat-context-menu"
            :style="{ left: `${flowContextMenu.x}px`, top: `${flowContextMenu.y}px` }"
            @click.stop
          >
            <button type="button" @click="rememberSelectedFlowMessages()">
              <Plus :size="14" />
              加入沉淀
            </button>
            <button type="button" @click="copySelectedFlowMessages()">
              <Copy :size="14" />
              复制选中
            </button>
            <button type="button" @click="clearFlowChatSelection(); closeFlowMessageMenu()">
              <X :size="14" />
              清除选择
            </button>
          </div>

          <section class="flow-evidence-panel flow-chat-evidence-panel" :class="{ 'is-expanded': evidenceExpanded }">
            <div class="flow-panel-head">
              <div>
                <span>证据抽屉</span>
                <h2>{{ evidenceExpanded ? '最近证据' : '已收起' }}</h2>
              </div>
              <button type="button" class="flow-text-button" @click="evidenceExpanded = !evidenceExpanded">
                {{ evidenceExpanded ? '收起' : '展开' }}
              </button>
            </div>
            <div v-if="evidenceExpanded" class="flow-evidence-list">
              <button v-for="entry in recentEvidence" :key="entry.id" type="button" class="flow-evidence-row" @click="openEvidence(entry)">
                <span>{{ sourceLabel(entry) }}</span>
                <strong>{{ entry.title }}</strong>
                <small>{{ entry.appName || 'Unknown' }} · {{ formatTime(entry.createdAt) }}</small>
              </button>
              <div v-if="!recentEvidence.length" class="flow-empty-note">还没有可展示的证据。</div>
            </div>
          </section>
        </section>

        <section v-else-if="activeFlowPage === 'timeline'" class="flow-page-panel flow-timeline-page" aria-label="心流时间线">
          <div class="flow-page-header flow-timeline-hero">
            <div>
              <span>TIMELINE</span>
              <h1>时间线</h1>
              <p>只按时间回看关键轨迹。明细、OCR 和图片证据默认收起，需要时再打开。</p>
            </div>
            <div class="flow-page-actions">
              <AriButton size="sm" variant="secondary" :disabled="!timelineEntries.length" @click="selectCurrentTimelineEntries()">
                <Check :size="14" />
                选择当前结果
              </AriButton>
              <AriButton
                size="sm"
                variant="secondary"
                :disabled="!timelineBatchOcrEntries.length || memory.isBatchRecognizingOCR"
                @click="runTimelineBatchOCR()"
              >
                <RefreshCw :size="14" :class="{ 'is-spinning': memory.isBatchRecognizingOCR }" />
                {{ timelineBatchOcrLabel }}
              </AriButton>
              <AriButton
                size="sm"
                :variant="timelineDeleteArmed ? 'primary' : 'secondary'"
                class="flow-danger-action"
                :disabled="!timelineSelectedEntries.length || memory.isDeletingEntries || memory.isBatchRecognizingOCR"
                @click="deleteTimelineSelection()"
              >
                <Trash2 :size="14" />
                {{ timelineDeleteArmed ? `确认删除 ${timelineSelectedEntries.length} 条` : '删除选中' }}
              </AriButton>
              <AriButton size="sm" variant="ghost" :disabled="!timelineSelectedEntries.length" @click="clearTimelineSelection()">
                清空
              </AriButton>
            </div>
          </div>

          <div class="flow-timeline-stats">
            <div v-for="stat in timelineStats" :key="stat.label" class="flow-timeline-stat">
              <span>{{ stat.label }}</span>
              <strong>{{ stat.value }}</strong>
              <small>{{ stat.note }}</small>
            </div>
          </div>

          <div class="flow-filter-strip" aria-label="时间线筛选">
            <button
              v-for="filter in timelineFilters"
              :key="filter.id"
              type="button"
              :class="{ 'is-active': timelineSourceFilter === filter.id }"
              @click="setTimelineFilter(filter.id)"
            >
              <component :is="filter.icon" :size="15" />
              <span>{{ filter.label }}</span>
              <small>{{ timelineFilterCounts[filter.id] }}</small>
            </button>
          </div>

          <div v-if="timelineAppOptions.length" ref="timelineAppSelectRef" class="flow-app-select" aria-label="来源程序筛选" @keydown.esc.stop.prevent="closeTimelineAppPicker()">
            <button
              type="button"
              class="flow-app-select-trigger"
              :class="{ 'is-open': timelineAppPickerOpen, 'is-filtered': timelineAppFilter !== 'all' }"
              :aria-expanded="timelineAppPickerOpen"
              aria-haspopup="listbox"
              @click="toggleTimelineAppPicker()"
            >
              <span class="flow-app-select-label">
                <span>来源程序</span>
                <strong>{{ selectedTimelineAppLabel }}</strong>
              </span>
              <small>{{ selectedTimelineAppCount }}</small>
              <ChevronDown :size="16" />
            </button>
            <div v-if="timelineAppPickerOpen" class="flow-app-select-menu" role="listbox">
              <label class="flow-app-select-search">
                <Search :size="15" />
                <input ref="timelineAppSearchRef" v-model="timelineAppSearch" type="search" placeholder="搜索程序" />
              </label>
              <div class="flow-app-select-options">
                <button
                  type="button"
                  role="option"
                  :aria-selected="timelineAppFilter === 'all'"
                  :class="{ 'is-active': timelineAppFilter === 'all' }"
                  @click="selectTimelineAppFilter('all')"
                >
                  <span class="flow-app-avatar">A</span>
                  <span>全部程序</span>
                  <small>{{ timelineSourceEntries.length }}</small>
                </button>
                <button
                  v-for="option in filteredTimelineAppOptions"
                  :key="option.id"
                  type="button"
                  role="option"
                  :title="option.label"
                  :aria-selected="timelineAppFilter === option.id"
                  :class="{ 'is-active': timelineAppFilter === option.id }"
                  @click="selectTimelineAppFilter(option.id)"
                >
                  <span class="flow-app-avatar">{{ appAvatarText(option.label) }}</span>
                  <span>{{ option.label }}</span>
                  <small>{{ option.count }}</small>
                </button>
                <div v-if="!filteredTimelineAppOptions.length" class="flow-app-select-empty">没有匹配的程序</div>
              </div>
            </div>
          </div>

          <div v-if="memory.isDeletingEntries || memory.deleteProgressTotal" class="flow-progress-strip is-danger" role="status" aria-live="polite">
            <div>
              <strong>正在删除选中轨迹</strong>
              <small>{{ memory.deleteProgressDone }} / {{ memory.deleteProgressTotal }} 条</small>
            </div>
            <div class="flow-progress-track">
              <span :style="{ width: `${deleteProgressPercent}%` }" />
            </div>
          </div>

          <div v-if="memory.isBatchRecognizingOCR || memory.batchOcrProgressTotal" class="flow-progress-strip" role="status" aria-live="polite">
            <div>
              <strong>正在批量 OCR+总结</strong>
              <small>{{ memory.batchOcrProgressDone }} / {{ memory.batchOcrProgressTotal }} 条</small>
            </div>
            <p v-if="memory.batchOcrProgressStage" class="flow-progress-note">{{ memory.batchOcrProgressStage }}</p>
            <div class="flow-progress-track">
              <span :style="{ width: `${batchOcrProgressPercent}%` }" />
            </div>
          </div>

          <div class="flow-timeline-layout">
            <section class="flow-timeline-main" aria-label="轨迹列表">
              <div class="flow-panel-head">
                <div>
                  <span>最近轨迹</span>
                  <strong>{{ timelineEntries.length }} 条 · {{ selectedTimelineAppLabel }}</strong>
                </div>
                <small>{{ timelineSelectionSummary }}，点击条目打开证据抽屉</small>
              </div>

              <div class="flow-timeline-list">
                <section v-for="group in visibleTimelineDayGroups" :key="group.id" class="flow-timeline-day">
                  <div class="flow-timeline-day-marker">
                    <span>{{ group.label }}</span>
                    <small>{{ group.note }}</small>
                  </div>
                  <div class="flow-timeline-day-events">
                    <article
                      v-for="entry in group.entries"
                      :key="entry.id"
                      class="flow-timeline-row"
                      :class="{
                        'is-selected': entry.id === memory.selectedId,
                        'is-timeline-selected': isTimelineSelected(entry.id),
                        'is-retrospective-selected': memory.isRetrospectiveSelected(entry.id),
                        'is-sensitive': entry.sensitive,
                      }"
                    >
                      <button
                        type="button"
                        class="flow-timeline-check"
                        :aria-pressed="isTimelineSelected(entry.id)"
                        :aria-label="isTimelineSelected(entry.id) ? '从批量选择中移除' : '加入批量选择'"
                        @click.stop="toggleTimelineSelection(entry.id)"
                      >
                        <Check v-if="isTimelineSelected(entry.id)" :size="13" />
                      </button>
                      <button type="button" class="flow-timeline-open" @click="openEvidence(entry)">
                        <span class="flow-timeline-time">{{ formatTimelineClock(entry.createdAt) }}</span>
                        <span class="flow-timeline-copy">
                          <strong>{{ entryFocusTitle(entry) }}</strong>
                          <span>{{ entryFocusSummary(entry) }}</span>
                          <small>
                            <span>{{ entry.appName || 'Unknown' }}</span>
                            <span>{{ sourceLabel(entry) }}</span>
                            <span>{{ entry.windowTitle || '无窗口标题' }}</span>
                          </small>
                        </span>
                      </button>
                      <span class="flow-timeline-kind">
                        <Camera v-if="isScreenshotEntry(entry)" :size="14" />
                        <Copy v-else-if="isClipboardEntry(entry)" :size="14" />
                        <FileText v-else :size="14" />
                        {{ sourceLabel(entry) }}
                      </span>
                      <div v-if="entryEvidenceBadges(entry).length" class="flow-timeline-badges" aria-label="证据类型">
                        <span v-for="badge in entryEvidenceBadges(entry)" :key="badge">{{ badge }}</span>
                      </div>
                    </article>
                  </div>
                </section>

                <div v-if="timelineHasMoreDays" ref="timelineLoadMoreRef" class="flow-timeline-load-more">
                  <span>继续向下滚动加载更早一天</span>
                  <button type="button" @click="loadMoreTimelineDays">立即加载</button>
                </div>

                <div v-if="!timelineEntries.length" class="flow-empty-card">
                  <Clock3 :size="24" />
                  <strong>没有匹配的轨迹</strong>
                  <p>换一个来源筛选，或在主搜索里查找 OCR、剪贴板、窗口标题和笔记。</p>
                </div>
              </div>
            </section>

            <aside class="flow-timeline-sidebar" aria-label="时间线辅助信息">
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Flag :size="15" />
                  复盘选择
                </div>
                <strong>{{ memory.retrospectiveTargetLabel }}</strong>
                <p>{{ timelineSelectedSummary }}</p>
                <div class="memory-side-actions">
                  <AriButton size="sm" variant="secondary" :disabled="!timelineSelectedEntries.length" @click="addTimelineSelectionToRetrospective()">
                    选中入复盘
                  </AriButton>
                  <AriButton size="sm" variant="secondary" :disabled="!timelineEntries.length" @click="selectCurrentTimelineForRetrospective()">
                    当前入复盘
                  </AriButton>
                  <AriButton size="sm" variant="ghost" :disabled="!memory.retrospectiveSelectionCount" @click="memory.clearRetrospectiveSelection()">
                    清空复盘
                  </AriButton>
                  <AriButton size="sm" variant="primary" :disabled="!memory.retrospectiveSelectionCount" @click="memory.buildRetrospectiveDraft()">
                    生成复盘
                  </AriButton>
                </div>
              </section>

              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Clock3 :size="15" />
                  时间机器
                </div>
                <div class="flow-mini-playback" :class="{ 'has-image': Boolean(memory.playbackImageUrl) }">
                  <img v-if="memory.playbackImageUrl" :src="memory.playbackImageUrl" alt="时间机器回放帧" />
                  <span v-else>{{ memory.playbackEntries.length ? '可回放截图帧' : '暂无截图帧' }}</span>
                </div>
                <div class="memory-side-actions">
                  <AriButton size="sm" variant="ghost" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(-1)">
                    <ArrowLeft :size="14" />
                  </AriButton>
                  <AriButton size="sm" variant="secondary" :disabled="!memory.playbackEntries.length || memory.isLoadingPlaybackImage" @click="memory.startPlayback()">
                    <Play :size="14" />
                    {{ memory.playbackEntry ? '定位' : '开始' }}
                  </AriButton>
                  <AriButton size="sm" variant="ghost" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(1)">
                    <ArrowRight :size="14" />
                  </AriButton>
                </div>
                <small>{{ memory.playbackPosition }}</small>
              </section>

              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Database :size="15" />
                  今日来源
                </div>
                <div class="flow-source-summary">
                  <span><Camera :size="14" /> 截图 {{ evidenceCounts.screenshots }}</span>
                  <span><Copy :size="14" /> 剪贴板 {{ evidenceCounts.clipboard }}</span>
                  <span><FileText :size="14" /> OCR {{ evidenceCounts.ocr }}</span>
                </div>
                <div class="flow-app-list">
                  <span v-for="[app, count] in topApps" :key="app">
                    <strong>{{ app }}</strong>
                    <small>{{ count }} 条</small>
                  </span>
                </div>
              </section>
            </aside>
          </div>
        </section>

        <section v-else-if="activeFlowPage === 'insights'" class="flow-page-panel flow-insights-page" aria-label="心流洞察">
          <div class="flow-page-header">
            <div>
              <span>INSIGHTS</span>
              <h1>自动归纳的线索</h1>
              <p>这里不再要求你逐条接受或驳回。Ariadne 只展示可解释的发现，真正要转成任务包、工作流或清单时再确认。</p>
            </div>
            <div class="flow-page-actions">
              <AriButton size="sm" variant="secondary" @click="memory.discoverExperienceReport()">
                <Sparkles :size="14" />
                本地归纳
              </AriButton>
              <AriButton
                size="sm"
                :variant="memory.experienceDiscoveryArmed ? 'primary' : 'secondary'"
                :disabled="memory.isDiscoveringExperienceAI"
                @click="memory.discoverExperienceReportAI()"
              >
                <Shield :size="14" />
                {{ memory.experienceDiscoveryArmed ? '确认 AI 归纳' : 'AI 归纳' }}
              </AriButton>
            </div>
          </div>
          <div v-if="memory.experienceDiscoveryResult" class="flow-note-strip">
            {{ memory.experienceDiscoveryResult.message }}
            <template v-if="memory.experienceDiscoveryResult.provider || memory.experienceDiscoveryResult.model">
              · {{ memory.experienceDiscoveryResult.provider }} / {{ memory.experienceDiscoveryResult.model }}
            </template>
          </div>
          <div v-if="memory.isDiscoveringExperienceAI || memory.experienceDiscoveryProgress" class="flow-progress-strip" role="status" aria-live="polite">
            <div>
              <strong>{{ memory.experienceDiscoveryStage || 'AI 正在归纳' }}</strong>
              <small>{{ insightProgressPercent }}%</small>
            </div>
            <div class="flow-progress-track">
              <span :style="{ width: `${insightProgressPercent}%` }" />
            </div>
          </div>
          <div class="flow-insight-board">
            <article v-for="insight in memory.experienceReport?.insights || []" :key="insight.id" class="flow-insight-card">
              <div class="flow-insight-head">
                <div>
                  <span>{{ insight.kind }} · {{ insight.severity }}</span>
                  <h2>{{ insight.title }}</h2>
                </div>
                <strong>{{ confidenceLabel(insight.confidence) }}</strong>
              </div>
              <p>{{ insight.summary }}</p>
              <small>{{ insight.reason }}</small>
              <div class="experience-meta">
                <span>证据 {{ insight.evidence.length }}</span>
                <span>{{ decisionLabel(insight.decisionStatus) }}</span>
                <span>{{ formatTime(insight.createdAt) }}</span>
              </div>
              <div class="flow-insight-actions">
                <AriButton size="sm" variant="secondary" title="生成可复制的外部代理任务包，交给 Codex 或其他 agent 处理" @click="handoffInsightToAgent(insight)">
                  <Workflow :size="14" />
                  交给代理
                </AriButton>
                <AriButton size="sm" variant="secondary" title="生成可保存到 Ariadne 的自动化工作流草稿" @click="buildAutomationFromInsight(insight)">
                  <Workflow :size="14" />
                  生成自动化
                </AriButton>
                <AriButton size="sm" variant="secondary" title="生成可复用的检查清单草稿" @click="buildChecklistFromInsight(insight)">
                  <Check :size="14" />
                  生成检查清单
                </AriButton>
              </div>
            </article>
            <div v-if="!memory.experienceReport?.insights.length" class="flow-empty-card">
              <Sparkles :size="22" />
              <strong>还没有稳定洞察</strong>
              <p>点击“本地归纳”后，系统会从最近记录里找重复问题、知识沉淀和自动化机会。</p>
            </div>
          </div>
        </section>

        <section v-else-if="activeFlowPage === 'drafts'" class="flow-page-panel" aria-label="心流草稿">
          <div class="flow-page-header">
            <div>
              <span>DRAFTS</span>
              <h1>摘要、复盘和知识草稿</h1>
              <p>草稿页只保留输出物，不混入原始明细。需要证据时再从抽屉或时间线打开。</p>
            </div>
            <div class="flow-page-actions">
              <AriButton size="sm" variant="primary" @click="memory.buildDailyDraft()">
                <FileText :size="14" />
                生成日报
              </AriButton>
              <AriButton size="sm" variant="secondary" @click="memory.buildRetrospectiveDraft()">
                <Clock3 :size="14" />
                生成复盘
              </AriButton>
              <AriButton size="sm" variant="secondary" @click="memory.buildKnowledgeDraft()">
                <Brain :size="14" />
                生成知识
              </AriButton>
            </div>
          </div>
          <div class="flow-draft-grid">
            <article class="flow-draft-card">
              <span>日报</span>
              <h2>{{ memory.dailyDraft?.title || '未生成' }}</h2>
              <pre>{{ memory.dailyDraft?.body || '等待从今日上下文生成。' }}</pre>
              <small v-if="memory.dailyDraft">证据 {{ memory.dailyDraft.evidence.length }} 条</small>
              <AriButton v-if="memory.dailyDraft" size="sm" variant="secondary" :disabled="memory.isPolishingDailyDraft" @click="memory.polishDailyDraft()">
                <Sparkles :size="14" />
                {{ memory.dailyDraftPolishArmed ? '确认外发润色' : 'AI 润色' }}
              </AriButton>
            </article>
            <article class="flow-draft-card">
              <span>复盘</span>
              <h2>{{ memory.retrospectiveDraft?.title || '未生成' }}</h2>
              <pre>{{ memory.retrospectiveDraft?.body || '选择一组证据后生成问题复盘。' }}</pre>
              <small>范围 {{ memory.retrospectiveTargetLabel }}</small>
            </article>
            <article class="flow-draft-card">
              <span>知识</span>
              <h2>{{ memory.knowledgeDraft?.title || '未生成' }}</h2>
              <p>{{ memory.knowledgeDraft?.body || '把高价值记录整理成可保存的知识条目。' }}</p>
              <small v-if="memory.knowledgeDraft">证据 {{ memory.knowledgeDraft.evidence.join(', ') }}</small>
              <AriButton v-if="memory.knowledgeDraft" size="sm" variant="secondary" :disabled="memory.isSavingKnowledgeDraft" @click="memory.saveCurrentKnowledgeDraft()">
                <Check :size="14" />
                {{ memory.knowledgeDraftSaveArmed ? '确认保存' : '保存为 Skill' }}
              </AriButton>
            </article>
          </div>
        </section>

        <section v-else-if="activeFlowPage === 'assets'" class="flow-page-panel" aria-label="心流资产">
          <div class="flow-page-header">
            <div>
              <span>ASSETS</span>
              <h1>从记忆沉淀成可复用能力</h1>
              <p>这里集中处理低频落地动作：任务包、工作流、清单和 Skill。保存、安装、外部代理仍然需要明确确认。</p>
            </div>
            <div class="flow-page-actions">
              <small v-if="assetFeedback" class="flow-asset-feedback">{{ assetFeedback }}</small>
              <AriButton size="sm" variant="secondary" @click="buildCurrentMemoryTaskPackage()">
                <Workflow :size="14" />
                从当前记忆生成任务包
              </AriButton>
            </div>
          </div>
          <div class="flow-asset-grid">
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'agent' }" data-flow-asset="agent">
              <span>外部代理</span>
              <h2>{{ memory.agentTask?.goal || '未生成' }}</h2>
              <p>{{ memory.agentTask?.context || '选择一条记忆或一条洞察后生成可审阅任务包。' }}</p>
              <small v-if="memory.agentTask?.requiresReview">需要人工复核</small>
              <div v-if="memory.agentTask" class="flow-asset-next">
                <strong>下一步</strong>
                <p>复制任务包，贴给 Codex 或其他代理执行。执行前需要你确认范围、权限和验收标准。</p>
                <div class="flow-asset-mini-list">
                  <span>{{ memory.agentTask.evidence.length }} 条证据</span>
                  <span>{{ memory.agentTask.boundaries.length }} 条边界</span>
                  <span>{{ memory.agentTask.acceptance.length }} 条验收</span>
                </div>
                <AriButton size="sm" variant="secondary" @click="copyCurrentAgentTask()">
                  <Copy :size="14" />
                  复制任务包
                </AriButton>
              </div>
            </article>
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'workflow' }" data-flow-asset="workflow">
              <span>候选工作流</span>
              <h2>{{ memory.workflowDraft?.title || '未生成' }}</h2>
              <p>{{ memory.workflowDraft?.trigger || '从重复流程里生成可保存的启动器工作流草稿。' }}</p>
              <small v-if="memory.workflowDraft">下一步：检查步骤和命令，确认无误后保存到工作流。</small>
              <div v-if="memory.workflowDraft" class="draft-step-list">
                <div v-for="step in memory.workflowDraft.steps" :key="step.id" class="draft-step">
                  <span>{{ step.label }}</span>
                  <code>{{ step.command }}</code>
                </div>
              </div>
              <AriButton v-if="memory.workflowDraft" size="sm" variant="secondary" :disabled="memory.isSavingWorkflowDraft" @click="memory.saveCurrentWorkflowDraft()">
                <Check :size="14" />
                {{ memory.workflowDraftSaveArmed ? '确认保存' : '保存到工作流' }}
              </AriButton>
            </article>
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'checklist' }" data-flow-asset="checklist">
              <span>检查清单</span>
              <h2>{{ memory.checklistDraft?.title || '未生成' }}</h2>
              <p>{{ memory.checklistDraft?.context || '把重复排查经验整理成可审阅清单。' }}</p>
              <small v-if="memory.checklistDraft">下一步：检查条目是否完整，确认后保存为可复用清单。</small>
              <ol v-if="memory.checklistDraft" class="draft-checklist">
                <li v-for="item in memory.checklistDraft.items" :key="item">{{ item }}</li>
              </ol>
              <AriButton v-if="memory.checklistDraft" size="sm" variant="secondary" :disabled="memory.isSavingChecklistDraft" @click="memory.saveCurrentChecklistDraft()">
                <Check :size="14" />
                {{ memory.checklistDraftSaveArmed ? '确认保存' : '保存为清单' }}
              </AriButton>
            </article>
            <article class="flow-asset-card">
              <span>Skill</span>
              <h2>{{ memory.knowledgeDraftSaveResult?.ok ? memory.knowledgeDraftSaveResult.skill.id : '未保存' }}</h2>
              <p>{{ memory.knowledgeSkillInstallResult?.message || '知识草稿保存后，可以导出或安装到 Codex Skill。' }}</p>
              <div class="flow-page-actions">
                <AriButton v-if="memory.knowledgeDraftSaveResult?.ok" size="sm" variant="secondary" :disabled="memory.isExportingKnowledgeSkill" @click="memory.exportCurrentKnowledgeSkill()">
                  <Download :size="14" />
                  {{ memory.knowledgeSkillExportArmed ? '确认导出' : '导出' }}
                </AriButton>
                <AriButton v-if="memory.knowledgeDraftSaveResult?.ok" size="sm" variant="secondary" :disabled="memory.isInstallingKnowledgeSkill" @click="memory.installCurrentKnowledgeSkill()">
                  <KeyRound :size="14" />
                  {{ memory.knowledgeSkillInstallArmed ? '确认安装' : '安装' }}
                </AriButton>
              </div>
            </article>
          </div>
        </section>

        <section v-else-if="activeFlowPage === 'rules'" class="flow-page-panel flow-rules-page" aria-label="心流规则">
          <div class="flow-page-header">
            <div>
              <span>RULES</span>
              <h1>采集边界和索引</h1>
              <p>低频维护项集中在这里：手动补记、导入导出、排除规则和语义索引。</p>
            </div>
            <div class="flow-page-actions">
              <AriButton size="sm" variant="secondary" @click="memory.captureNow()">
                <Camera :size="14" />
                手动补记
              </AriButton>
              <AriButton size="sm" variant="secondary" @click="memory.exportData()">
                <Download :size="14" />
                导出
              </AriButton>
            </div>
          </div>
          <div class="flow-rules-grid">
            <div class="side-panel memory-note-panel">
              <div class="side-title">
                <Plus :size="15" />
                手动笔记
              </div>
              <input v-model="memory.noteDraft.title" class="memory-note-input" spellcheck="false" placeholder="标题" />
              <textarea v-model="memory.noteDraft.text" class="memory-note-textarea" spellcheck="false" placeholder="记录问题、结论、待办或证据..." />
              <input v-model="memory.noteDraft.tags" class="memory-note-input" spellcheck="false" placeholder="标签，用空格或逗号分隔" />
              <div class="memory-check-row">
                <label><input v-model="memory.noteDraft.favorite" type="checkbox" /> 收藏</label>
                <label><input v-model="memory.noteDraft.sensitive" type="checkbox" /> 敏感</label>
              </div>
              <AriButton size="sm" variant="primary" @click="memory.addNote()">
                <Plus :size="14" />
                加入心流
              </AriButton>
            </div>

            <div class="side-panel semantic-panel">
              <div class="side-title">
                <Database :size="15" />
                语义索引
              </div>
              <strong>{{ vectorStatusLabel }}</strong>
              <p>{{ memory.semanticStatus?.note || '本地关键词和 FTS 可用；外部 embedding 需要显式刷新。' }}</p>
              <div class="semantic-meta-grid">
                <span><small>Provider</small><strong>{{ vectorProviderLabel }}</strong></span>
                <span><small>Store</small><strong>{{ vectorStoreLabel }}</strong></span>
                <span><small>刷新</small><strong>{{ memory.semanticStatus?.lastEmbeddingAt ? formatTime(memory.semanticStatus.lastEmbeddingAt) : '未刷新' }}</strong></span>
                <span><small>Collection</small><strong>{{ memory.semanticStatus?.vectorCollection || 'ariadne_work_memory' }}</strong></span>
              </div>
              <div class="search-row semantic-search-row">
                <Search :size="15" class="text-[var(--muted)]" />
                <input v-model="memory.semanticDraft.query" class="search-input" spellcheck="false" placeholder="语义搜索非敏感心流记忆..." @keydown.enter="memory.runSemanticSearch()" />
              </div>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" :disabled="memory.isRefreshingEmbedding" @click="memory.refreshEmbedding()">
                  <RefreshCw :size="14" />
                  {{ memory.isRefreshingEmbedding ? '刷新中' : '刷新索引' }}
                </AriButton>
                <AriButton size="sm" variant="primary" :disabled="memory.isSemanticSearching" @click="memory.runSemanticSearch()">
                  <Search :size="14" />
                  {{ memory.isSemanticSearching ? '检索中' : '语义搜索' }}
                </AriButton>
              </div>
            </div>

            <div class="side-panel memory-data-panel">
              <div class="side-title">
                <Upload :size="15" />
                数据包
              </div>
              <textarea v-model="memory.importDraft.paths" class="memory-import-textarea" spellcheck="false" placeholder="粘贴文件路径，一行一个" />
              <input v-model="memory.importDraft.tags" class="memory-note-input" spellcheck="false" placeholder="导入标签" />
              <div class="memory-side-actions">
                <AriButton size="sm" variant="primary" :disabled="memory.isImportingMaterials" @click="memory.importMaterials()">
                  <Upload :size="14" />
                  {{ memory.isImportingMaterials ? '导入中' : '导入材料' }}
                </AriButton>
                <AriButton size="sm" variant="ghost" @click="memory.clearUnpinned()">
                  <Trash2 :size="14" />
                  {{ memory.clearUnpinnedArmed ? '确认清理' : '清理未收藏' }}
                </AriButton>
              </div>
              <small v-if="memory.importResult">
                导入 {{ memory.importResult.imported }} 条，跳过 {{ memory.importResult.skipped }} 条，失败 {{ memory.importResult.failed }} 条
              </small>
            </div>

            <div class="side-panel memory-rules-panel">
              <div class="side-title">
                <Shield :size="15" />
                排除规则
              </div>
              <p>优先于采集、OCR、导入、导出和经验发现。</p>
              <div class="memory-rule-summary">{{ memory.exclusionSummary }}</div>
              <div class="memory-rule-grid">
                <label class="memory-rule-field">
                  <span>应用进程</span>
                  <textarea v-model="memory.exclusionDraft.apps" class="memory-rule-textarea" spellcheck="false" placeholder="Code.exe&#10;chrome.exe" />
                </label>
                <label class="memory-rule-field">
                  <span>窗口关键词</span>
                  <textarea v-model="memory.exclusionDraft.windowKeywords" class="memory-rule-textarea" spellcheck="false" placeholder="密码&#10;隐私" />
                </label>
                <label class="memory-rule-field">
                  <span>路径片段</span>
                  <textarea v-model="memory.exclusionDraft.paths" class="memory-rule-textarea" spellcheck="false" placeholder="secrets&#10;.env" />
                </label>
                <label class="memory-rule-field">
                  <span>内容正则</span>
                  <textarea v-model="memory.exclusionDraft.contentPatterns" class="memory-rule-textarea" spellcheck="false" placeholder="token=&#10;classified" />
                </label>
              </div>
              <AriButton size="sm" variant="secondary" :disabled="memory.isSavingExclusions" @click="memory.saveExclusionRules()">
                <Shield :size="14" />
                {{ memory.isSavingExclusions ? '保存中' : '保存排除规则' }}
              </AriButton>
            </div>
          </div>
        </section>

        <div v-if="flowSettingsOpen" class="flow-settings-backdrop" @click.self="flowSettingsOpen = false">
          <aside class="flow-settings-drawer" data-no-drag aria-label="心流设置">
            <header class="flow-settings-header">
              <div>
                <span>FLOW SETTINGS</span>
                <h2>心流设置</h2>
                <p>采集、索引、模型和隐私边界只在这里维护，通用设置中心不再重复展示。</p>
              </div>
              <button type="button" class="flow-icon-button" aria-label="关闭心流设置" @click="flowSettingsOpen = false">
                <X :size="16" />
              </button>
            </header>

            <div v-if="settings.settings" class="flow-settings-body">
              <section class="flow-settings-overview">
                <div>
                  <span>当前状态</span>
                  <strong>{{ timeMachineLabel }} · {{ vectorStatusLabel }}</strong>
                  <small>{{ runtimeStatusText || '本地心流配置已就绪。' }}</small>
                </div>
                <div class="flow-settings-overview-grid">
                  <span>
                    <small>采集范围</small>
                    <strong>{{ captureScopeLabel }} / {{ multiMonitorLabel }}</strong>
                  </span>
                  <span>
                    <small>模型</small>
                    <strong>{{ vectorProviderLabel }}</strong>
                  </span>
                  <span>
                    <small>向量库</small>
                    <strong>{{ vectorStoreLabel }}</strong>
                  </span>
                </div>
              </section>

              <nav class="flow-settings-tabs" aria-label="心流设置分组">
                <button
                  v-for="tab in flowSettingsTabs"
                  :key="tab.id"
                  type="button"
                  :class="{ 'is-active': flowSettingsTab === tab.id }"
                  @click="setFlowSettingsTab(tab.id)"
                >
                  <strong>{{ tab.label }}</strong>
                  <small>{{ tab.detail }}</small>
                </button>
              </nav>

              <section v-show="flowSettingsTab === 'capture'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>采集与沉淀</span>
                  <small>{{ runtimeStatusText || '本地采集策略' }}</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.enabled" type="checkbox" />
                    <span />
                    <strong>心流总开关</strong>
                    <small>关闭后不采集新上下文，历史仍可搜索。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.timeMachineEnabled" type="checkbox" />
                    <span />
                    <strong>屏幕时间机器</strong>
                    <small>自动沉淀屏幕上下文，受排除规则约束。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.windowSwitchCaptureEnabled" type="checkbox" />
                    <span />
                    <strong>窗口切换触发</strong>
                    <small>前台窗口变化时补一帧证据。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.autoOcr" type="checkbox" />
                    <span />
                    <strong>自动 OCR</strong>
                    <small>本地识别截图文字，用于回答和检索。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.draftScheduleEnabled" type="checkbox" />
                    <span />
                    <strong>自动整理</strong>
                    <small>定时生成日报、复盘和经验候选。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.experienceScheduleEnabled" type="checkbox" />
                    <span />
                    <strong>经验发现</strong>
                    <small>后台归纳重复问题和可优化流程。</small>
                  </label>
                </div>

                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>截图间隔秒</span>
                    <input v-model.number="settings.settings.workMemory.autoCaptureIntervalSeconds" type="number" min="10" />
                  </label>
                  <label class="flow-setting-field">
                    <span>窗口冷却秒</span>
                    <input v-model.number="settings.settings.workMemory.windowSwitchCooldownSeconds" type="number" min="3" />
                  </label>
                  <label class="flow-setting-field">
                    <span>整理间隔分钟</span>
                    <input v-model.number="settings.settings.workMemory.draftScheduleIntervalMinutes" type="number" min="15" />
                  </label>
                  <label class="flow-setting-field">
                    <span>截图质量</span>
                    <input v-model.number="settings.settings.workMemory.screenshotQuality" type="number" min="1" max="100" />
                  </label>
                  <label class="flow-setting-field">
                    <span>采集范围</span>
                    <select v-model="settings.settings.workMemory.captureScope">
                      <option value="all_screens">全部屏幕</option>
                      <option value="active_window">前台窗口</option>
                      <option value="primary_screen">主屏幕</option>
                    </select>
                  </label>
                  <label class="flow-setting-field">
                    <span>多屏策略</span>
                    <select v-model="settings.settings.workMemory.multiMonitor">
                      <option value="combined">合并截图</option>
                      <option value="per_monitor">按屏幕分条</option>
                      <option value="primary_only">仅主屏</option>
                    </select>
                  </label>
                </div>

                <div class="flow-app-policy-panel">
                  <div class="flow-settings-section-head">
                    <span>应用采集策略</span>
                    <small>命中应用后接管全局截图间隔，用自己的切窗延迟和驻留节奏。</small>
                  </div>
                  <div class="flow-app-policy-layout">
                    <div class="flow-app-profile-list" aria-label="已配置应用采集策略">
                      <button
                        v-for="profile in appCaptureProfiles"
                        :key="profile.id"
                        type="button"
                        :class="{ 'is-active': selectedAppCaptureProfile?.id === profile.id }"
                        @click="selectAppCaptureProfile(profile.id)"
                      >
                        <span class="flow-app-avatar">{{ appAvatarText(profile.displayName || profile.processName) }}</span>
                        <span>
                          <strong>{{ profile.displayName || displayAppName(profile.processName) }}</strong>
                          <small>{{ profile.processName }} · {{ profile.enabled ? '已接管' : '已暂停' }}</small>
                        </span>
                      </button>
                      <p v-if="!appCaptureProfiles.length" class="flow-app-empty">
                        还没有应用策略。先从最近应用添加一个，比如 Weixin.exe。
                      </p>
                    </div>

                    <div v-if="selectedAppCaptureProfile" class="flow-app-profile-detail">
                      <div class="flow-app-profile-title">
                        <span class="flow-app-avatar is-large">{{ appAvatarText(selectedAppCaptureProfile.displayName || selectedAppCaptureProfile.processName) }}</span>
                        <span>
                          <strong>{{ selectedAppCaptureProfile.displayName || displayAppName(selectedAppCaptureProfile.processName) }}</strong>
                          <small>{{ selectedAppCaptureProfile.processName }}</small>
                        </span>
                        <button type="button" class="flow-icon-button" aria-label="移除应用策略" @click="removeAppCaptureProfile(selectedAppCaptureProfile.id)">
                          <Trash2 :size="15" />
                        </button>
                      </div>
                      <label class="flow-setting-switch is-compact">
                        <input v-model="selectedAppCaptureProfile.enabled" type="checkbox" />
                        <span />
                        <strong>启用应用策略</strong>
                        <small>关闭后恢复全局采集节奏。</small>
                      </label>
                      <div class="flow-settings-field-grid is-compact">
                        <label class="flow-setting-field">
                          <span>切换后延迟秒</span>
                          <input v-model.number="selectedAppCaptureProfile.windowSwitchDelaySeconds" type="number" min="0" max="3600" />
                        </label>
                        <label class="flow-setting-field">
                          <span>保持期间间隔秒</span>
                          <input v-model.number="selectedAppCaptureProfile.activeIntervalSeconds" type="number" min="10" max="86400" />
                        </label>
                      </div>
                    </div>
                  </div>

                  <div class="flow-app-candidates">
                    <button
                      v-for="candidate in appCaptureCandidates"
                      :key="candidate.id"
                      type="button"
                      @click="addAppCaptureProfile(candidate)"
                    >
                      <span class="flow-app-avatar">{{ appAvatarText(candidate.displayName) }}</span>
                      <span>
                        <strong>{{ candidate.displayName }}</strong>
                        <small>{{ candidate.processName }} · {{ candidate.count }} 条</small>
                      </span>
                      <Plus :size="15" />
                    </button>
                    <p v-if="!appCaptureCandidates.length" class="flow-app-empty">
                      最近应用都已配置，或还没有可用于添加的采集记录。
                    </p>
                  </div>
                </div>

                <div class="flow-settings-source-list">
                  <label v-for="source in settings.memorySources" :key="source.key" class="flow-source-pill">
                    <input
                      type="checkbox"
                      :checked="source.enabled"
                      @change="settings.setMemorySource(source.key, ($event.target as HTMLInputElement).checked)"
                    />
                    <span>{{ source.label }}</span>
                  </label>
                </div>
              </section>

              <section v-show="flowSettingsTab === 'model'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>模型与向量</span>
                  <small>{{ vectorProviderLabel }} · {{ vectorStoreLabel }}</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.enabled" type="checkbox" />
                    <span />
                    <strong>AI 回答与草稿</strong>
                    <small>使用你配置的兼容接口生成回答。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.embeddingEnabled" type="checkbox" />
                    <span />
                    <strong>语义索引</strong>
                    <small>用于“我今天干了什么”这类上下文问答。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.externalAgentEnabled" type="checkbox" />
                    <span />
                    <strong>外部代理任务包</strong>
                    <small>需要沉淀 Skill 或工作流时再显式确认。</small>
                  </label>
                </div>
                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>AI provider</span>
                    <input v-model="settings.settings.ai.provider" placeholder="openai-compatible" />
                  </label>
                  <label class="flow-setting-field">
                    <span>AI base URL</span>
                    <input v-model="settings.settings.ai.baseUrl" placeholder="http://127.0.0.1:4000/v1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>AI model</span>
                    <input v-model="settings.settings.ai.model" placeholder="glm-5.1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding provider</span>
                    <input v-model="settings.settings.ai.embeddingProvider" placeholder="openai-compatible" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding base URL</span>
                    <input v-model="settings.settings.ai.embeddingBaseUrl" placeholder="http://127.0.0.1:4000/v1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding model</span>
                    <input v-model="settings.settings.ai.embeddingModel" placeholder="/model/qwen_eb" />
                  </label>
                  <label class="flow-setting-field">
                    <span>向量存储</span>
                    <select v-model="settings.settings.ai.vectorStoreType">
                      <option value="embedded">内置缓存</option>
                      <option value="milvus">Milvus</option>
                      <option value="disabled">关闭</option>
                    </select>
                  </label>
                  <label class="flow-setting-field">
                    <span>向量 URI</span>
                    <input v-model="settings.settings.ai.vectorStoreUri" placeholder="milvus://192.168.1.100:19530" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Collection</span>
                    <input v-model="settings.settings.ai.vectorCollection" placeholder="ariadne_work_memory" />
                  </label>
                </div>
                <div class="secret-store-block flow-secret-block">
                  <div class="flow-settings-section-head">
                    <span>安全密钥存储</span>
                    <small>
                      {{ settings.secretStatus?.available ? 'Windows Credential Manager 可用' : '当前运行环境未暴露安全存储' }}
                      · {{ settings.secretStatus?.backend || '未检测' }}
                    </small>
                  </div>
                  <div class="secret-store-grid">
                    <div
                      v-for="record in settings.secretStatus?.records ?? []"
                      :key="record.kind"
                      class="secret-store-row"
                      :data-secret-kind="record.kind"
                      :data-secret-active-source="record.activeSource"
                      :data-secret-stored="record.stored ? 'true' : 'false'"
                    >
                      <div class="secret-store-meta">
                        <strong>{{ record.label }}</strong>
                        <small>
                          {{ record.stored ? '已保存' : '未保存' }}
                          · {{ secretSourceLabel(record.activeSource) }}
                        </small>
                        <small class="secret-store-target">{{ record.targetName }}</small>
                        <small v-if="record.envPresent">检测到环境变量：{{ record.envNames.join(' / ') }}</small>
                        <small v-if="record.lastError" class="is-danger">{{ record.lastError }}</small>
                      </div>
                      <input
                        v-model="settings.secretInputs[record.kind]"
                        class="settings-input"
                        type="password"
                        autocomplete="off"
                        placeholder="粘贴后保存，不写入 config.json"
                        :aria-label="`${record.label} 输入`"
                        :data-secret-input="record.kind"
                      />
                      <div class="secret-store-actions">
                        <AriButton
                          size="sm"
                          variant="secondary"
                          :disabled="!canSaveSecret(record.kind)"
                          :data-secret-save="record.kind"
                          @click="settings.saveSecret(record.kind)"
                        >
                          <Check :size="14" />
                          保存
                        </AriButton>
                        <AriButton
                          size="sm"
                          variant="ghost"
                          :disabled="!canClearSecret(record.stored)"
                          :data-secret-clear="record.kind"
                          @click="settings.clearSecret(record.kind)"
                        >
                          <Trash2 :size="14" />
                          {{ settings.secretClearArmedKind === record.kind ? '确认清除' : '清除' }}
                        </AriButton>
                      </div>
                    </div>
                  </div>
                  <p
                    v-if="settings.secretActionResult"
                    class="settings-note"
                    :class="{ 'is-danger': !settings.secretActionResult.ok && !settings.secretActionResult.requiresConfirmation }"
                    data-secret-action-result
                  >
                    {{ settings.secretActionResult.message }}
                  </p>
                </div>
              </section>

              <section v-show="flowSettingsTab === 'privacy'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>隐私边界与存储</span>
                  <small>排除规则已集中到“规则”页面维护。</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.privacyMode" type="checkbox" />
                    <span />
                    <strong>隐私模式</strong>
                    <small>暂停截图、OCR、embedding、AI 和导出。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.pauseOnIdle" type="checkbox" />
                    <span />
                    <strong>空闲暂停</strong>
                    <small>超过阈值时停止自动采集。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.pauseOnLock" type="checkbox" />
                    <span />
                    <strong>锁屏暂停</strong>
                    <small>锁屏或不可切换桌面时不采集。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.sensitiveRulesEnabled" type="checkbox" />
                    <span />
                    <strong>敏感内容规则</strong>
                    <small>识别 token、密码、cookie 等风险内容。</small>
                  </label>
                </div>
                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>空闲阈值秒</span>
                    <input v-model.number="settings.settings.workMemory.idlePauseSeconds" type="number" min="30" />
                  </label>
                  <label class="flow-setting-field">
                    <span>经验发现天数</span>
                    <input v-model.number="settings.settings.workMemory.experienceDiscoveryDays" type="number" min="1" max="365" />
                  </label>
                  <label class="flow-setting-field">
                    <span>记忆保留天数</span>
                    <input v-model.number="settings.settings.workMemory.retentionDays" type="number" min="1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>缩略图保留天数</span>
                    <input v-model.number="settings.settings.workMemory.thumbnailRetentionDays" type="number" min="1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>最大存储 MB</span>
                    <input v-model.number="settings.settings.workMemory.maxStorageMb" type="number" min="128" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Trace</span>
                    <select v-model="settings.settings.ai.traceMode">
                      <option value="off">关闭</option>
                      <option value="local">本地日志</option>
                      <option value="internal">内部观测</option>
                    </select>
                  </label>
                </div>
              </section>
            </div>
            <div v-else class="flow-empty-card">
              <strong>正在读取心流设置</strong>
              <p>配置会从 Ariadne 本地 JSON 与安全存储读取。</p>
            </div>

            <footer class="flow-settings-footer">
              <small>{{ settings.feedback || '心流设置保存在 Ariadne 本地配置中。' }}</small>
              <div class="flow-page-actions">
                <AriButton size="sm" variant="ghost" @click="flowSettingsOpen = false">关闭</AriButton>
                <AriButton size="sm" variant="primary" :disabled="settings.isSaving || !settings.settings" @click="saveFlowSettings()">
                  <Check :size="14" />
                  {{ settings.isSaving ? '保存中' : '保存心流设置' }}
                </AriButton>
              </div>
            </footer>
          </aside>
        </div>

        <div v-if="detailDrawerOpen && selected" class="flow-detail-backdrop" @click.self="detailDrawerOpen = false">
          <aside class="flow-detail-drawer" aria-label="心流证据明细">
            <div class="flow-detail-head">
              <div>
                <span>{{ sourceLabel(selected) }} · {{ formatTime(selected.createdAt) }}</span>
                <h2>{{ selected.title }}</h2>
                <p>{{ selected.summary }}</p>
              </div>
              <button type="button" class="flow-icon-button" aria-label="关闭明细" @click="detailDrawerOpen = false">
                <X :size="16" />
              </button>
            </div>

            <div class="memory-capture-frame flow-detail-capture" :class="{ 'has-image': Boolean(memory.selectedImageUrl) }">
              <OCRImageOverlay
                v-if="memory.selectedImageUrl"
                :src="memory.selectedImageUrl"
                :width="memory.ocrResult?.width || selected.width"
                :height="memory.ocrResult?.height || selected.height"
                :lines="memory.ocrLines"
                :is-line-selected="memory.isOCRLineSelected"
                :max-height="260"
                @toggle-line="memory.toggleOCRLine"
              />
              <template v-else>
                <Sparkles :size="24" />
                <span>{{ selected.windowTitle || 'Ariadne context' }}</span>
              </template>
            </div>

            <div class="flow-detail-actions">
              <AriButton size="sm" variant="secondary" :disabled="memory.isRecognizingOCR || !selected.imagePath" @click="memory.recognizeSelectedText()">
                <FileText :size="14" />
                {{ memory.isRecognizingOCR ? 'OCR 中' : '再次 OCR' }}
              </AriButton>
              <AriButton v-if="selected.ocrText" size="sm" variant="secondary" @click="memory.copyOCRText()">
                <Copy :size="14" />
                复制 OCR
              </AriButton>
              <AriButton size="sm" variant="secondary" @click="memory.buildKnowledgeDraft()">
                <Brain :size="14" />
                知识草稿
              </AriButton>
              <AriButton size="sm" variant="ghost" @click="memory.deleteSelected()">
                <Trash2 :size="14" />
                {{ memory.deleteArmedId === selected.id ? '确认删除' : '删除' }}
              </AriButton>
            </div>

            <div class="meta-grid flow-detail-meta">
              <div class="meta-item">
                <span>来源</span>
                <strong>{{ sourceLabel(selected) }}</strong>
              </div>
              <div class="meta-item">
                <span>应用</span>
                <strong>{{ selected.appName || '-' }}</strong>
              </div>
              <div class="meta-item">
                <span>窗口</span>
                <strong>{{ selected.windowTitle || '-' }}</strong>
              </div>
            </div>

            <pre v-if="selected.text" class="preview-text memory-text flow-detail-text">{{ selected.text }}</pre>
            <details v-if="selected.ocrText" class="flow-raw-ocr">
              <summary>原始 OCR 证据</summary>
              <pre class="preview-text memory-text flow-detail-text">{{ selected.ocrText }}</pre>
            </details>

            <div class="tag-row">
              <span v-for="tag in selected.tags" :key="tag">
                <Tags :size="12" />
                {{ tag }}
              </span>
              <span v-if="selected.favorite">
                <Flag :size="12" />
                收藏
              </span>
            </div>
          </aside>
        </div>

        <footer class="status-strip">
          <span>
            <Check :size="14" />
            心流本地保存
          </span>
          <span>
            <KeyRound :size="14" />
            高风险动作需确认
          </span>
          <span>
            <Shield :size="14" />
            敏感内容默认不外发
          </span>
          <span v-if="memory.feedback" class="inline-feedback">
            {{ memory.feedback }}
          </span>
        </footer>
          </section>
        </div>
      </section>
    </div>
  </main>
</template>
