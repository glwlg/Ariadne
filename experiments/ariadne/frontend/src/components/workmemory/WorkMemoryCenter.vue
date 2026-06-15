<script setup lang="ts">
import {
  ArrowLeft,
  ArrowRight,
  Brain,
  Camera,
  Check,
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
import { computed, onMounted, ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useWorkMemoryStore } from '../../stores/workMemory'
import { ocrConfidenceLabel, ocrRectLabel } from '../../lib/ocrDisplay'
import type { WorkMemoryEntry } from '../../types/ariadne'

const appShell = useAppShellStore()
const memory = useWorkMemoryStore()
type FlowPage = 'flow' | 'timeline' | 'insights' | 'drafts' | 'assets' | 'rules'

const selected = computed(() => memory.selectedEntry)
const visibleEntries = computed(() => memory.filteredEntries)
const activeFlowPage = ref<FlowPage>('flow')
const flowQuestion = ref('')
const flowAnswer = ref('')
const flowBusy = ref(false)
const evidenceExpanded = ref(false)
const detailDrawerOpen = ref(false)
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
const flowOptimizationFallback = [
  {
    title: '截图贴图步骤偏多',
    tag: '效率提升',
    summary: '截图后如果多次进入贴图和复制动作，适合沉淀成快捷流程。',
  },
  {
    title: '重复问题处理',
    tag: '知识沉淀',
    summary: '高频排查和修复线索会优先整理成模板、清单或 Skill。',
  },
  {
    title: '资料查找耗时',
    tag: '信息管理',
    summary: '多次切换目录或搜索同类资料时，可建立统一索引。',
  },
]
const todayLabel = computed(() => new Date().toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' }))
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
const activityLabel = computed(() => {
  if (memory.status.sessionLocked) return '锁屏'
  const idle = memory.status.idleSeconds ?? 0
  if (memory.status.pauseOnIdle && idle >= (memory.status.idlePauseSeconds ?? 0)) {
    return `空闲 ${formatDuration(idle)}`
  }
  return memory.status.pauseOnLock || memory.status.pauseOnIdle ? '受保护' : '常开'
})
const autoOCRLabel = computed(() => {
  if (!memory.status.autoOcrEnabled) return '关闭'
  return memory.status.lastAutoOcrError ? '异常' : '自动'
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
const recentEvidence = computed(() => (todayEntries.value.length ? todayEntries.value : memory.entries).slice(0, 8))
const visibleInsights = computed(() => memory.experienceReport?.insights.slice(0, 5) ?? [])
const contactEntries = computed(() => {
  return todayEntries.value.filter((entry) => /微信|weixin|wechat|钉钉|dingtalk|企业微信|qq|teams|outlook|邮件|mail|会议|meeting|腾讯会议/i.test([
    entry.title,
    entry.summary,
    entry.text,
    entry.windowTitle,
    entry.appName,
  ].filter(Boolean).join(' ')))
})
const topApps = computed(() => {
  const counts = new Map<string, number>()
  for (const entry of todayEntries.value) {
    const app = entry.appName || 'Unknown'
    counts.set(app, (counts.get(app) ?? 0) + 1)
  }
  return [...counts.entries()].sort((left, right) => right[1] - left[1]).slice(0, 4)
})
const flowAnswerTitle = computed(() => {
  if (flowQuestion.value.trim()) return flowQuestion.value.trim()
  return '今天的心流'
})
const flowAnswerBody = computed(() => {
  if (flowAnswer.value) return flowAnswer.value
  if (memory.dailyDraft?.body) return compactDraftBody(memory.dailyDraft.body)
  const count = todayEntries.value.length || memory.status.entryCount
  if (!count) return '心流已经准备好。开启主动沉淀或时间机器后，我会在后台整理截图、剪贴板、OCR 和工作流线索。'
  const appText = topApps.value.length ? `高频应用包括 ${topApps.value.map(([app, count]) => `${app} ${count} 条`).join('、')}。` : ''
  return `今天已经沉淀 ${count} 条上下文，采集到 ${memory.status.captureCount ?? 0} 次屏幕证据。${appText} 你可以直接问我今天做了什么、谁找过你，或哪些流程值得优化。`
})
const dailySections = computed(() => {
  const count = todayEntries.value.length
  const captureCount = todayEntries.value.filter((entry) => Boolean(entry.imagePath || entry.captureId)).length
  const workflowCount = visibleInsights.value.filter((insight) => /automation|workflow|opportunity|流程|工作流/i.test(`${insight.kind} ${insight.title} ${insight.summary}`)).length
  return [
    {
      label: '主线',
      value: count ? `已整理 ${count} 条今日上下文，核心线索会自动归并到摘要里。` : '今日还没有形成稳定主线。',
      meta: topApps.value.length ? topApps.value.map(([app]) => app).join(' / ') : '等待采集',
    },
    {
      label: '证据',
      value: captureCount ? `保留 ${captureCount} 条截图或视觉证据，明细默认收起。` : '暂时没有截图型证据。',
      meta: `${memory.status.captureScope ?? 'all_screens'} · ${memory.status.multiMonitor ?? 'combined'}`,
    },
    {
      label: '可优化',
      value: workflowCount || visibleInsights.value.length ? `发现 ${workflowCount || visibleInsights.value.length} 条可能可优化线索，默认不打扰。` : '暂无需要你处理的优化项。',
      meta: memory.experienceReport ? '已自动归纳' : '可按需发现',
    },
  ]
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

function decisionLabel(status?: string) {
  const labels: Record<string, string> = {
    accepted: '已接受',
    rejected: '已驳回',
    later: '稍后处理',
    task_package: '已转任务包',
  }
  return status ? labels[status] || status : '待处理'
}

function startOfToday() {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  return Math.floor(date.getTime() / 1000)
}

function compactDraftBody(value: string) {
  return value
    .replace(/^#+\s*/gm, '')
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean)
    .slice(0, 8)
    .join('\n')
}

async function askFlow(question = flowQuestion.value) {
  const normalized = question.trim()
  if (!normalized) {
    flowAnswer.value = '你可以直接问：我今天干了些什么、有哪些人找过我，或哪些流程可以优化。'
    return
  }
  flowQuestion.value = normalized
  flowBusy.value = true
  flowAnswer.value = ''
  try {
    if (/优化|工作流|重复|自动化|流程/.test(normalized)) {
      await memory.discoverExperienceReport()
      const count = memory.experienceReport?.insights.length ?? 0
      flowAnswer.value = count
        ? `我从最近记录里归纳出 ${count} 条可优化线索，已经按主题放到“洞察”里。默认不会让你逐条决策，只有转成 Skill、工作流、清单或任务包时才需要确认。`
        : '暂时没有发现稳定的流程优化线索。我会继续在后台观察重复动作和高频上下文。'
      return
    }
    if (/谁|人|找过|联系|消息|沟通/.test(normalized)) {
      const count = contactEntries.value.length
      const apps = [...new Set(contactEntries.value.map((entry) => entry.appName || sourceLabel(entry)).filter(Boolean))].slice(0, 5)
      flowAnswer.value = count
        ? `今天我看到 ${count} 条可能与沟通有关的记录，主要来自 ${apps.join('、')}。当前还没有联系人实体抽取，所以我先按应用和窗口线索归并。`
        : '今天还没有稳定识别到沟通类记录。后续可以把联系人抽取放进自动整理链路。'
      return
    }
    if (/今天|干了|做了|总结|发生/.test(normalized)) {
      await memory.buildDailyDraft()
      flowAnswer.value = ''
      return
    }
    memory.semanticDraft.query = normalized
    await memory.runSemanticSearch()
    const resultCount = memory.semanticSearchResult?.results.length ?? memory.searchResults.length
    flowAnswer.value = resultCount
      ? `我找到了 ${resultCount} 条相关记忆，已把最相关的证据放在下方。你可以展开证据或切到时间线继续看。`
      : '没有找到足够相关的记忆。我会继续使用 OCR、剪贴板和向量索引补全上下文。'
  } finally {
    flowBusy.value = false
  }
}

function useFlowQuestion(question: string) {
  flowQuestion.value = question
  void askFlow(question)
}

function openFlowPage(page: FlowPage) {
  activeFlowPage.value = page
  detailDrawerOpen.value = false
}

function openEvidence(entry: WorkMemoryEntry) {
  memory.select(entry.id)
  detailDrawerOpen.value = true
}

function evidenceLabel() {
  const count = recentEvidence.value.length
  return evidenceExpanded.value ? '收起证据' : `查看 ${count} 条证据`
}

onMounted(() => {
  void memory.load()
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
                <Sparkles :size="28" />
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
              <button type="button" class="flow-side-nav-item" @click="appShell.openSettings()">
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

        <section v-if="activeFlowPage === 'flow'" class="flow-home" aria-label="心流首页">
          <div class="flow-main-column">
            <section class="flow-ask-panel">
              <div class="flow-question-box">
                <div class="flow-question-copy">
                  <strong>今天想了解什么？</strong>
                  <span>向心流提问，获取你的专属记忆与洞察</span>
                </div>
                <textarea
                  v-model="flowQuestion"
                  spellcheck="false"
                  placeholder="输入你的问题"
                  @keydown.ctrl.enter.prevent="askFlow()"
                />
                <button type="button" class="flow-send-button" :disabled="flowBusy" aria-label="询问心流" @click="askFlow()">
                  <ArrowRight :size="22" />
                </button>
                <div class="flow-question-chips">
                  <button v-for="question in flowQuestions.slice(0, 3)" :key="question" type="button" @click="useFlowQuestion(question)">
                    {{ question }}
                  </button>
                </div>
              </div>
            </section>

            <section class="flow-answer-panel">
              <div class="flow-panel-head">
                <div>
                  <span>心流回答</span>
                  <h2>{{ flowAnswerTitle }}</h2>
                </div>
                <small>{{ new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }}</small>
              </div>
              <div class="flow-answer-card-body">
                <div class="flow-answer-icon" aria-hidden="true">
                  <Sparkles :size="24" />
                </div>
                <div class="flow-answer-content">
                  <p>{{ flowAnswerBody }}</p>
                  <ul>
                    <li v-for="section in dailySections" :key="section.label">
                      <Check :size="15" />
                      <span>{{ section.value }}</span>
                    </li>
                  </ul>
                </div>
              </div>
              <div class="flow-evidence-summary">
                <strong>证据与来源</strong>
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
                <button type="button" @click="evidenceExpanded = !evidenceExpanded">{{ evidenceLabel() }}</button>
              </div>
            </section>

            <section class="flow-day-panel">
              <div class="flow-panel-head">
                <div>
                  <span>你还可以问</span>
                  <h2>继续追问上下文</h2>
                </div>
              </div>
              <div class="flow-followup-chips">
                <button v-for="question in flowQuestions.slice(2)" :key="question" type="button" @click="useFlowQuestion(question)">
                  {{ question }}
                </button>
              </div>
            </section>

            <section class="flow-evidence-panel" :class="{ 'is-expanded': evidenceExpanded }">
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
          </div>

          <aside class="flow-right-rail">
            <section class="flow-brief-panel">
              <div class="flow-panel-head">
                <div>
                  <span>今日自动摘要</span>
                  <h2>{{ todayLabel }}</h2>
                </div>
              </div>
              <div class="flow-summary-list">
                <div class="flow-summary-row">
                  <Check :size="18" />
                  <span>
                    <strong>核心产出</strong>
                    <small>完成 {{ todayEntries.length || memory.status.entryCount }} 条上下文沉淀，保留 {{ evidenceCounts.screenshots }} 条视觉证据</small>
                  </span>
                </div>
                <div class="flow-summary-row">
                  <Clock3 :size="18" />
                  <span>
                    <strong>专注时长</strong>
                    <small>{{ activityLabel }} · {{ captureScopeLabel }} / {{ multiMonitorLabel }}</small>
                  </span>
                </div>
                <div class="flow-summary-row">
                  <Brain :size="18" />
                  <span>
                    <strong>互动情况</strong>
                    <small>OCR {{ autoOCRLabel }} {{ evidenceCounts.ocr }} · 剪贴板 {{ evidenceCounts.clipboard }} · 笔记 {{ evidenceCounts.notes }}</small>
                  </span>
                </div>
              </div>
            </section>

            <section class="flow-insight-panel">
              <div class="flow-panel-head">
                <div>
                  <span>{{ visibleInsights.length || flowOptimizationFallback.length }} 个可优化流程</span>
                </div>
                <button type="button" class="flow-link-button" @click="openFlowPage('insights')">查看全部</button>
              </div>
              <div class="flow-insight-list">
                <article
                  v-for="insight in visibleInsights"
                  :key="insight.id"
                  class="flow-insight-row"
                  @click="openFlowPage('insights')"
                >
                  <strong>{{ insight.title }}</strong>
                  <p>{{ insight.summary }}</p>
                  <small>{{ insight.kind }} · 证据 {{ insight.evidence.length }} · {{ confidenceLabel(insight.confidence) }}</small>
                  <ArrowRight :size="16" />
                </article>
                <article
                  v-for="item in visibleInsights.length ? [] : flowOptimizationFallback"
                  :key="item.title"
                  class="flow-insight-row"
                  @click="openFlowPage('insights')"
                >
                  <strong>{{ item.title }}</strong>
                  <p>{{ item.summary }}</p>
                  <small>{{ item.tag }}</small>
                  <ArrowRight :size="16" />
                </article>
              </div>
            </section>

            <section class="flow-model-panel">
              <div class="flow-panel-head">
                <div>
                  <span>连接与模型</span>
                </div>
                <small class="flow-running-dot">运行正常</small>
              </div>
              <div class="flow-model-grid">
                <span>模型</span>
                <strong>自建模型</strong>
                <span>向量库</span>
                <strong>{{ vectorStoreLabel }}</strong>
              </div>
              <div class="flow-privacy-note">
                <Shield :size="16" />
                <span>记录仅保存在本地与内网环境，隐私由你掌控。</span>
              </div>
              <small v-if="runtimeStatusText" class="flow-runtime-line">{{ runtimeStatusText }}</small>
            </section>
          </aside>
        </section>

        <div v-else-if="activeFlowPage === 'timeline'" class="memory-workspace">
          <section class="memory-timeline" aria-label="工作记忆时间线">
            <div class="pane-title">
              <span>时间线</span>
              <small>{{ memory.isLoading ? '加载中' : `${visibleEntries.length} 条` }}</small>
            </div>
            <div class="memory-selection-bar">
              <span>{{ memory.retrospectiveTargetLabel }}</span>
              <div class="memory-selection-actions">
                <button type="button" @click="memory.selectVisibleForRetrospective()">选择筛选</button>
                <button type="button" :disabled="!memory.retrospectiveSelectionCount" @click="memory.clearRetrospectiveSelection()">清空</button>
              </div>
            </div>

            <div
              v-for="entry in visibleEntries"
              :key="entry.id"
              class="memory-row"
              :class="{
                'is-selected': entry.id === memory.selectedId,
                'is-retrospective-selected': memory.isRetrospectiveSelected(entry.id),
              }"
            >
              <button
                type="button"
                class="memory-select-toggle"
                :class="{ 'is-selected': memory.isRetrospectiveSelected(entry.id), 'is-sensitive': entry.sensitive }"
                :disabled="entry.sensitive"
                :aria-pressed="memory.isRetrospectiveSelected(entry.id)"
                :aria-label="entry.sensitive ? '敏感记忆不会加入复盘证据' : memory.isRetrospectiveSelected(entry.id) ? '从复盘证据移除' : '加入复盘证据'"
                @click="memory.toggleRetrospectiveSelection(entry.id)"
              >
                <Check v-if="memory.isRetrospectiveSelected(entry.id)" :size="12" />
              </button>
              <button type="button" class="memory-row-main memory-row-open" @click="memory.select(entry.id)">
                <span class="memory-row-title">{{ entry.title }}</span>
                <span class="memory-row-summary">{{ entry.summary }}</span>
                <span class="memory-row-meta">
                  {{ sourceLabel(entry) }} · {{ entry.appName || 'Unknown' }} · {{ formatTime(entry.createdAt) }}
                </span>
              </button>
              <span v-if="entry.favorite" class="memory-favorite">
                <Flag :size="13" />
              </span>
            </div>

            <div v-if="!visibleEntries.length" class="empty-state">
              <Brain :size="22" />
              <span>没有匹配的工作记忆</span>
            </div>
          </section>

          <section class="memory-detail" aria-label="工作记忆详情">
            <template v-if="selected">
              <div class="memory-detail-header">
                <span class="preview-kicker">{{ selected.contentType }}</span>
                <h1>{{ selected.title }}</h1>
                <p>{{ selected.summary }}</p>
              </div>

              <div class="action-row memory-action-row">
                <AriButton size="sm" variant="primary" @click="memory.buildDailyDraft()">
                  <FileText :size="14" />
                  日报草稿
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="memory.buildRetrospectiveDraft()">
                  <Clock3 :size="14" />
                  复盘草稿{{ memory.retrospectiveSelectionCount ? `(${memory.retrospectiveSelectionCount})` : '' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="memory.buildKnowledgeDraft()">
                  <Brain :size="14" />
                  知识草稿
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="memory.buildAgentTask()">
                  <Workflow :size="14" />
                  任务包
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="memory.isRecognizingOCR || !selected.imagePath" @click="memory.recognizeSelectedText()">
                  <FileText :size="14" />
                  {{ memory.isRecognizingOCR ? 'OCR 中' : '再次 OCR' }}
                </AriButton>
                <AriButton size="sm" variant="ghost" @click="memory.deleteSelected()">
                  <Trash2 :size="14" />
                  {{ memory.deleteArmedId === selected.id ? '确认删除' : '删除' }}
                </AriButton>
              </div>

              <div class="memory-capture-frame" :class="{ 'has-image': Boolean(memory.selectedImageUrl) }">
                <OCRImageOverlay
                  v-if="memory.selectedImageUrl"
                  :src="memory.selectedImageUrl"
                  :width="memory.ocrResult?.width || selected.width"
                  :height="memory.ocrResult?.height || selected.height"
                  :lines="memory.ocrLines"
                  :is-line-selected="memory.isOCRLineSelected"
                  :max-height="220"
                  @toggle-line="memory.toggleOCRLine"
                />
                <template v-else>
                  <Sparkles :size="24" />
                  <span>{{ selected.windowTitle || 'Ariadne context' }}</span>
                </template>
              </div>

              <pre class="preview-text memory-text">{{ selected.text }}</pre>
              <div v-if="memory.ocrResult?.ok && memory.ocrLines.length" class="qr-result-panel is-success">
                <div class="side-title">
                  <FileText :size="15" />
                  OCR 文本选择
                </div>
                <strong>
                  {{ memory.ocrResult.provider || 'RapidOCR' }} · {{ memory.ocrResult.elapsedMs || 0 }}ms
                  · {{ memory.ocrLines.length }} 行 · 已选 {{ memory.selectedOCRLineCount }}
                </strong>
                <div class="ocr-selection-panel">
                  <div class="ocr-selection-actions">
                    <AriButton size="sm" variant="secondary" @click="memory.selectAllOCRLines()">全选</AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.clearOCRLineSelection()">清空</AriButton>
                    <AriButton size="sm" variant="secondary" :disabled="!memory.selectedOCRLineCount" @click="memory.copySelectedOCRText()">
                      <Copy :size="14" />
                      复制选中
                    </AriButton>
                    <AriButton v-if="memory.ocrResult.text" size="sm" variant="secondary" @click="memory.copyOCRText()">
                      <Copy :size="14" />
                      复制全文
                    </AriButton>
                  </div>
                  <div class="ocr-line-list" aria-label="OCR 文本行">
                    <button
                      v-for="(line, index) in memory.ocrLines"
                      :key="`${index}-${line.text}`"
                      type="button"
                      class="ocr-line-row"
                      :class="{ 'is-selected': memory.isOCRLineSelected(index) }"
                      :aria-pressed="memory.isOCRLineSelected(index)"
                      @click="memory.toggleOCRLine(index)"
                    >
                      <span class="ocr-line-check">{{ memory.isOCRLineSelected(index) ? '已选' : '选择' }}</span>
                      <span class="ocr-line-body">
                        <span class="ocr-line-text">{{ line.text }}</span>
                        <span class="ocr-line-meta">{{ ocrConfidenceLabel(line.confidence) }} · {{ ocrRectLabel(line) }}</span>
                      </span>
                    </button>
                  </div>
                </div>
              </div>
              <pre v-if="selected.ocrText" class="preview-text memory-text">OCR:
{{ selected.ocrText }}</pre>
              <div v-if="selected.ocrText" class="ocr-selection-actions">
                <AriButton size="sm" variant="secondary" @click="memory.copyOCRText()">
                  <Copy :size="14" />
                  复制 OCR
                </AriButton>
              </div>
              <div v-else-if="selected.ocrStatus" class="qr-result-panel">
                <div class="side-title">
                  <FileText :size="15" />
                  OCR 状态
                </div>
                <p>{{ selected.ocrStatus }}</p>
              </div>

              <div class="meta-grid">
                <div class="meta-item">
                  <span>来源</span>
                  <strong>{{ sourceLabel(selected) }}</strong>
                </div>
                <div class="meta-item">
                  <span>应用</span>
                  <strong>{{ selected.appName || '-' }}</strong>
                </div>
                <div class="meta-item">
                  <span>时间</span>
                  <strong>{{ formatTime(selected.createdAt) }}</strong>
                </div>
              </div>

              <div class="tag-row">
                <span v-for="tag in selected.tags" :key="tag">
                  <Tags :size="12" />
                  {{ tag }}
                </span>
              </div>
            </template>
          </section>

          <aside class="memory-side" aria-label="草稿与代理建议">
            <div class="side-panel memory-note-panel">
              <div class="side-title">
                <Plus :size="15" />
                手动笔记
              </div>
              <input v-model="memory.noteDraft.title" class="memory-note-input" spellcheck="false" placeholder="标题" />
              <textarea
                v-model="memory.noteDraft.text"
                class="memory-note-textarea"
                spellcheck="false"
                placeholder="记录问题、结论、待办或证据..."
              />
              <input v-model="memory.noteDraft.tags" class="memory-note-input" spellcheck="false" placeholder="标签，用空格或逗号分隔" />
              <div class="memory-check-row">
                <label>
                  <input v-model="memory.noteDraft.favorite" type="checkbox" />
                  收藏
                </label>
                <label>
                  <input v-model="memory.noteDraft.sensitive" type="checkbox" />
                  敏感
                </label>
              </div>
              <AriButton size="sm" variant="primary" @click="memory.addNote()">
                <Plus :size="14" />
                加入记忆
              </AriButton>
            </div>

            <div class="side-panel memory-playback-panel">
              <div class="side-title">
                <Clock3 :size="15" />
                时间机器回放
              </div>
              <p>按时间顺序回看截图型工作记忆，当前帧会同步到详情区。</p>
              <div class="memory-playback-frame" :class="{ 'has-image': Boolean(memory.playbackImageUrl) }">
                <img v-if="memory.playbackImageUrl" :src="memory.playbackImageUrl" alt="时间机器回放帧" />
                <template v-else>
                  <Clock3 :size="22" />
                  <span>{{ memory.playbackEntries.length ? '选择一帧开始回放' : '暂无截图帧' }}</span>
                </template>
              </div>
              <div class="memory-playback-meta">
                <strong>{{ memory.playbackEntry?.title || '未定位' }}</strong>
                <small>
                  {{ memory.playbackPosition }}
                  <template v-if="memory.playbackEntry">
                    · {{ sourceLabel(memory.playbackEntry) }} · {{ formatTime(memory.playbackEntry.createdAt) }}
                  </template>
                </small>
              </div>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(-1)">
                  <ArrowLeft :size="14" />
                  上一帧
                </AriButton>
                <AriButton size="sm" variant="primary" :disabled="!memory.playbackEntries.length || memory.isLoadingPlaybackImage" @click="memory.startPlayback()">
                  <Play :size="14" />
                  {{ memory.playbackEntry ? '定位最近' : '开始回放' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(1)">
                  <ArrowRight :size="14" />
                  下一帧
                </AriButton>
              </div>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Clock3 :size="15" />
                定期草稿
              </div>
              <strong>{{ memory.scheduledDraftStatus?.enabled ? (memory.scheduledDraftStatus.running ? '调度运行中' : '调度待命') : '未启用' }}</strong>
              <p>
                间隔 {{ memory.scheduledDraftStatus?.intervalMinutes || 240 }} 分钟 ·
                最近 {{ memory.scheduledDraftStatus?.lastRunAt ? formatTime(memory.scheduledDraftStatus.lastRunAt) : '未运行' }}
              </p>
              <small v-if="memory.scheduledDraftStatus?.lastError">{{ memory.scheduledDraftStatus.lastError }}</small>
              <small v-else-if="memory.scheduledDraftStatus?.lastRunAt">
                {{ memory.scheduledDraftStatus.lastEntryCount }} 条非敏感证据 ·
                日报 {{ memory.scheduledDraftStatus.dailyDraft?.id ? '已生成' : '未生成' }} ·
                复盘 {{ memory.scheduledDraftStatus.retrospectiveDraft?.id ? '已生成' : '无问题线索' }} ·
                经验 {{ memory.scheduledDraftStatus.experienceReport?.id ? '已生成' : '未生成' }}
              </small>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" :disabled="memory.isRunningScheduledDrafts" @click="memory.runScheduledDrafts()">
                  <Clock3 :size="14" />
                  {{ memory.isRunningScheduledDrafts ? '运行中' : '立即运行' }}
                </AriButton>
              </div>
            </div>

            <div class="side-panel semantic-panel">
              <div class="side-title">
                <Database :size="15" />
                语义索引
              </div>
              <strong>{{ vectorStatusLabel }}</strong>
              <p>{{ memory.semanticStatus?.note || '本地关键词和 FTS 可用；外部 embedding 需要显式刷新。' }}</p>
              <div class="semantic-meta-grid">
                <span>
                  <small>Provider</small>
                  <strong>{{ vectorProviderLabel }}</strong>
                </span>
                <span>
                  <small>Store</small>
                  <strong>{{ vectorStoreLabel }}</strong>
                </span>
                <span>
                  <small>刷新</small>
                  <strong>{{ memory.semanticStatus?.lastEmbeddingAt ? formatTime(memory.semanticStatus.lastEmbeddingAt) : '未刷新' }}</strong>
                </span>
                <span>
                  <small>Collection</small>
                  <strong>{{ memory.semanticStatus?.vectorCollection || 'ariadne_work_memory' }}</strong>
                </span>
              </div>
              <div class="search-row semantic-search-row">
                <Search :size="15" class="text-[var(--muted)]" />
                <input
                  v-model="memory.semanticDraft.query"
                  class="search-input"
                  spellcheck="false"
                  placeholder="语义搜索非敏感工作记忆..."
                  @keydown.enter="memory.runSemanticSearch()"
                />
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
              <small v-if="memory.embeddingRefreshResult">
                {{ memory.embeddingRefreshResult.message }} · 跳过 {{ memory.embeddingRefreshResult.skipped }}
              </small>
              <small v-if="memory.semanticStatus?.lastEmbeddingError">
                {{ memory.semanticStatus.lastEmbeddingError }}
              </small>
              <small v-if="memory.semanticSearchResult">
                {{ memory.semanticSearchResult.message }}
              </small>
              <div v-if="memory.semanticSearchResult?.results.length" class="semantic-result-list">
                <button
                  v-for="result in memory.semanticSearchResult.results"
                  :key="result.id"
                  type="button"
                  class="semantic-result-row"
                  @click="memory.select(result.id)"
                >
                  <strong>{{ result.title }}</strong>
                  <small>{{ result.detail || result.subtitle || result.preview?.text }}</small>
                </button>
              </div>
            </div>

            <div class="side-panel memory-data-panel">
              <div class="side-title">
                <Download :size="15" />
                数据包
              </div>
              <textarea
                v-model="memory.importDraft.paths"
                class="memory-import-textarea"
                spellcheck="false"
                placeholder="粘贴文件路径，一行一个"
              />
              <input v-model="memory.importDraft.tags" class="memory-note-input" spellcheck="false" placeholder="导入标签" />
              <div class="memory-check-row">
                <label>
                  <input v-model="memory.importDraft.favorite" type="checkbox" />
                  收藏
                </label>
                <label>
                  <input v-model="memory.importDraft.sensitive" type="checkbox" />
                  敏感
                </label>
              </div>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="primary" :disabled="memory.isImportingMaterials" @click="memory.importMaterials()">
                  <Upload :size="14" />
                  {{ memory.isImportingMaterials ? '导入中' : '导入材料' }}
                </AriButton>
              </div>
              <small v-if="memory.importResult">
                导入 {{ memory.importResult.imported }} 条，跳过 {{ memory.importResult.skipped }} 条，失败 {{ memory.importResult.failed }} 条
              </small>
              <div class="memory-export-filter">
                <input v-model="memory.exportDraft.recentDays" class="memory-note-input" inputmode="numeric" spellcheck="false" placeholder="最近天数，空为全部" />
                <input v-model="memory.exportDraft.tags" class="memory-note-input" spellcheck="false" placeholder="导出标签，空为全部" />
                <input v-model="memory.exportDraft.entryIds" class="memory-note-input" spellcheck="false" placeholder="条目 ID，可逗号分隔" />
              </div>
              <small v-if="memory.exportResult?.path">{{ memory.exportResult.path }}</small>
              <small v-if="memory.exportResult">
                {{ memory.exportResult.entryCount }} 条，跳过 {{ memory.exportResult.skippedSensitiveCount }} 条敏感记忆、{{ memory.exportResult.skippedExcludedCount || 0 }} 条排除规则、{{ memory.exportResult.filteredOutCount || 0 }} 条筛选外
              </small>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" @click="memory.exportData()">
                  <Download :size="14" />
                  导出
                </AriButton>
                <AriButton size="sm" variant="ghost" @click="memory.clearUnpinned()">
                  <Trash2 :size="14" />
                  {{ memory.clearUnpinnedArmed ? '确认清理' : '清理未收藏' }}
                </AriButton>
              </div>
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
                  <textarea
                    v-model="memory.exclusionDraft.apps"
                    class="memory-rule-textarea"
                    spellcheck="false"
                    placeholder="Code.exe&#10;chrome.exe"
                  />
                </label>
                <label class="memory-rule-field">
                  <span>窗口关键词</span>
                  <textarea
                    v-model="memory.exclusionDraft.windowKeywords"
                    class="memory-rule-textarea"
                    spellcheck="false"
                    placeholder="密码&#10;隐私"
                  />
                </label>
                <label class="memory-rule-field">
                  <span>路径片段</span>
                  <textarea
                    v-model="memory.exclusionDraft.paths"
                    class="memory-rule-textarea"
                    spellcheck="false"
                    placeholder="secrets&#10;.env"
                  />
                </label>
                <label class="memory-rule-field">
                  <span>URL 域名/路径</span>
                  <textarea
                    v-model="memory.exclusionDraft.urls"
                    class="memory-rule-textarea"
                    spellcheck="false"
                    placeholder="example.com/private&#10;*.corp.local"
                  />
                </label>
                <label class="memory-rule-field">
                  <span>内容正则</span>
                  <textarea
                    v-model="memory.exclusionDraft.contentPatterns"
                    class="memory-rule-textarea"
                    spellcheck="false"
                    placeholder="token=&#10;classified"
                  />
                </label>
              </div>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" :disabled="memory.isSavingExclusions" @click="memory.saveExclusionRules()">
                  <Shield :size="14" />
                  {{ memory.isSavingExclusions ? '保存中' : '保存排除规则' }}
                </AriButton>
              </div>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <FileText :size="15" />
                日报
              </div>
              <strong>{{ memory.dailyDraft?.title || '未生成' }}</strong>
              <pre class="daily-draft-body">{{ memory.dailyDraft?.body || '等待从时间线生成草稿。' }}</pre>
              <small v-if="memory.dailyDraft">证据 {{ memory.dailyDraft.evidence.length }} 条</small>
              <div v-if="memory.dailyDraft" class="memory-side-actions">
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="memory.isPolishingDailyDraft"
                  @click="memory.polishDailyDraft()"
                >
                  <Sparkles :size="14" />
                  {{ memory.dailyDraftPolishArmed ? '确认外发润色' : 'AI 润色' }}
                </AriButton>
              </div>
              <small v-if="memory.dailyDraftPolishResult">
                {{
                  memory.dailyDraftPolishResult.ok
                    ? `${memory.dailyDraftPolishResult.provider || 'AI'} 已润色 · ${memory.dailyDraftPolishResult.model || 'model'}`
                    : memory.dailyDraftPolishResult.message
                }}
              </small>
              <small v-if="memory.dailyDraftPolishResult?.requiresConfirmation">
                {{ memory.dailyDraftPolishResult.riskReasons?.join(' · ') }}
              </small>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Clock3 :size="15" />
                复盘
              </div>
              <strong>{{ memory.retrospectiveDraft?.title || '未生成' }}</strong>
              <small>范围 {{ memory.retrospectiveTargetLabel }}</small>
              <pre class="daily-draft-body">{{ memory.retrospectiveDraft?.body || '选择一条或一组记忆后生成问题复盘草稿。' }}</pre>
              <small v-if="memory.retrospectiveDraft">证据 {{ memory.retrospectiveDraft.evidence.join(', ') }}</small>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Brain :size="15" />
                知识
              </div>
              <strong>{{ memory.knowledgeDraft?.title || '未生成' }}</strong>
              <p>{{ memory.knowledgeDraft?.body || '选择一条记忆后生成知识草稿。' }}</p>
              <small v-if="memory.knowledgeDraft">证据 {{ memory.knowledgeDraft.evidence.join(', ') }}</small>
              <div v-if="memory.knowledgeDraft" class="memory-side-actions">
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="memory.isSavingKnowledgeDraft"
                  @click="memory.saveCurrentKnowledgeDraft()"
                >
                  <Check :size="14" />
                  {{ memory.knowledgeDraftSaveArmed ? '确认保存' : '保存为 Skill' }}
                </AriButton>
              </div>
              <small v-if="memory.knowledgeDraftSaveResult">
                {{ memory.knowledgeDraftSaveResult.ok ? `已保存: ${memory.knowledgeDraftSaveResult.skill.id}` : memory.knowledgeDraftSaveResult.message }}
              </small>
              <div v-if="memory.knowledgeDraftSaveResult?.ok" class="memory-side-actions">
                <AriButton
                  size="sm"
                  variant="ghost"
                  :disabled="memory.isExportingKnowledgeSkill"
                  @click="memory.exportCurrentKnowledgeSkill()"
                >
                  <Download :size="14" />
                  {{ memory.knowledgeSkillExportArmed ? '确认导出' : '导出 Skill 包' }}
                </AriButton>
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="memory.isInstallingKnowledgeSkill"
                  @click="memory.installCurrentKnowledgeSkill()"
                >
                  <KeyRound :size="14" />
                  {{ memory.knowledgeSkillInstallArmed ? '确认安装' : '安装到 Codex' }}
                </AriButton>
              </div>
              <small v-if="memory.knowledgeSkillExportResult">
                {{
                  memory.knowledgeSkillExportResult.ok
                    ? `已导出: ${memory.knowledgeSkillExportResult.zipPath || memory.knowledgeSkillExportResult.directory}`
                    : memory.knowledgeSkillExportResult.message
                }}
              </small>
              <small v-if="memory.knowledgeSkillInstallResult">
                {{
                  memory.knowledgeSkillInstallResult.ok
                    ? `已安装: ${memory.knowledgeSkillInstallResult.installedDir || memory.knowledgeSkillInstallResult.targetRoot}`
                    : memory.knowledgeSkillInstallResult.message
                }}
              </small>
              <small v-if="memory.knowledgeSkillInstallResult?.ok && memory.knowledgeSkillInstallResult.refreshMarker">
                刷新握手 {{ memory.knowledgeSkillInstallResult.refreshMarker }}
              </small>
              <small v-if="memory.knowledgeSkillInstallDiagnostics">
                {{
                  memory.knowledgeSkillInstallDiagnostics.ok
                    ? `Codex 已发现 ${memory.knowledgeSkillInstallDiagnostics.discoveredCount} 个 Skill，目标可读，握手有效`
                    : memory.knowledgeSkillInstallDiagnostics.message
                }}
              </small>
              <small v-if="memory.knowledgeSkillInstallDiagnostics?.refresh && !memory.knowledgeSkillInstallDiagnostics.refresh.valid">
                marker {{ memory.knowledgeSkillInstallDiagnostics.refresh.markerPath || '未写入' }}
              </small>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Workflow :size="15" />
                外部代理
              </div>
              <strong>{{ memory.agentTask?.goal || '未生成' }}</strong>
              <p>{{ memory.agentTask?.context || '任务包生成后仍需用户确认。' }}</p>
              <small v-if="memory.agentTask && memory.agentTask.requiresReview">Requires review</small>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Workflow :size="15" />
                候选工作流
              </div>
              <strong>{{ memory.workflowDraft?.title || '未生成' }}</strong>
              <p>{{ memory.workflowDraft?.trigger || '从经验线索生成可审阅的启动器工作流草稿。' }}</p>
              <div v-if="memory.workflowDraft" class="draft-step-list">
                <div v-for="step in memory.workflowDraft.steps" :key="step.id" class="draft-step">
                  <span>{{ step.label }}</span>
                  <code>{{ step.command }}</code>
                  <small v-if="step.requiresConfirm">需确认</small>
                </div>
              </div>
              <small v-if="memory.workflowDraft">
                {{ memory.workflowDraft.riskLevel }} · 证据 {{ memory.workflowDraft.evidence.length }} 条
              </small>
              <div v-if="memory.workflowDraft" class="memory-side-actions">
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="memory.isSavingWorkflowDraft"
                  @click="memory.saveCurrentWorkflowDraft()"
                >
                  <Check :size="14" />
                  {{ memory.workflowDraftSaveArmed ? '确认保存' : '保存到工作流' }}
                </AriButton>
              </div>
              <small v-if="memory.workflowDraftSaveResult">
                {{ memory.workflowDraftSaveResult.ok ? `已保存: ${memory.workflowDraftSaveResult.workflow.id}` : memory.workflowDraftSaveResult.message }}
              </small>
            </div>

            <div class="side-panel">
              <div class="side-title">
                <Check :size="15" />
                检查清单
              </div>
              <strong>{{ memory.checklistDraft?.title || '未生成' }}</strong>
              <p>{{ memory.checklistDraft?.context || '把重复排查经验整理成保存前可审阅的清单。' }}</p>
              <ol v-if="memory.checklistDraft" class="draft-checklist">
                <li v-for="item in memory.checklistDraft.items" :key="item">{{ item }}</li>
              </ol>
              <small v-if="memory.checklistDraft">证据 {{ memory.checklistDraft.evidence.length }} 条</small>
              <div v-if="memory.checklistDraft" class="memory-side-actions">
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="memory.isSavingChecklistDraft"
                  @click="memory.saveCurrentChecklistDraft()"
                >
                  <Check :size="14" />
                  {{ memory.checklistDraftSaveArmed ? '确认保存' : '保存为清单' }}
                </AriButton>
              </div>
              <small v-if="memory.checklistDraftSaveResult">
                {{ memory.checklistDraftSaveResult.ok ? `已保存: ${memory.checklistDraftSaveResult.checklist.id}` : memory.checklistDraftSaveResult.message }}
              </small>
            </div>

            <div class="side-panel experience-panel">
              <div class="side-title">
                <Sparkles :size="15" />
                经验发现
              </div>
              <p>{{ memory.experienceReport?.summary || '从最近工作记忆中发现重复问题、流程经验和自动化机会。' }}</p>
              <div class="memory-side-actions">
                <AriButton size="sm" variant="secondary" @click="memory.discoverExperienceReport()">
                  <Sparkles :size="14" />
                  发现经验
                </AriButton>
                <AriButton
                  size="sm"
                  :variant="memory.experienceDiscoveryArmed ? 'primary' : 'secondary'"
                  :disabled="memory.isDiscoveringExperienceAI"
                  @click="memory.discoverExperienceReportAI()"
                >
                  <Shield :size="14" />
                  {{ memory.experienceDiscoveryArmed ? '确认外发发现' : 'AI 发现' }}
                </AriButton>
              </div>
              <div v-if="memory.experienceDiscoveryResult" class="experience-ai-note">
                <small>
                  {{ memory.experienceDiscoveryResult.message }}
                  <template v-if="memory.experienceDiscoveryResult.provider || memory.experienceDiscoveryResult.model">
                    · {{ memory.experienceDiscoveryResult.provider }} / {{ memory.experienceDiscoveryResult.model }}
                  </template>
                </small>
                <ul v-if="memory.experienceDiscoveryResult.requiresConfirmation && memory.experienceDiscoveryResult.riskReasons?.length">
                  <li v-for="reason in memory.experienceDiscoveryResult.riskReasons" :key="reason">{{ reason }}</li>
                </ul>
              </div>
              <div v-if="memory.experienceReport?.insights.length" class="experience-list">
                <article v-for="insight in memory.experienceReport.insights" :key="insight.id" class="experience-item">
                  <div class="experience-item-head">
                    <strong>{{ insight.title }}</strong>
                    <span>{{ confidenceLabel(insight.confidence) }}</span>
                  </div>
                  <p>{{ insight.summary }}</p>
                  <small>{{ insight.reason }}</small>
                  <div class="experience-meta">
                    <span>{{ insight.kind }}</span>
                    <span>{{ insight.severity }}</span>
                    <span>证据 {{ insight.evidence.length }}</span>
                    <span>{{ decisionLabel(insight.decisionStatus) }}</span>
                  </div>
                  <div class="experience-actions">
                    <AriButton size="sm" variant="ghost" @click="memory.markExperienceInsight(insight, 'accepted')">
                      <Check :size="14" />
                      接受
                    </AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.markExperienceInsight(insight, 'later')">
                      <Clock3 :size="14" />
                      稍后
                    </AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.markExperienceInsight(insight, 'rejected')">
                      <Trash2 :size="14" />
                      驳回
                    </AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.buildAgentTaskFromInsight(insight)">
                      <Workflow :size="14" />
                      转任务包
                    </AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.buildWorkflowDraftFromInsight(insight)">
                      <Workflow :size="14" />
                      转工作流
                    </AriButton>
                    <AriButton size="sm" variant="ghost" @click="memory.buildChecklistDraftFromInsight(insight)">
                      <Check :size="14" />
                      转清单
                    </AriButton>
                  </div>
                </article>
              </div>
            </div>
          </aside>
        </div>

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
                <AriButton size="sm" variant="secondary" @click="memory.buildAgentTaskFromInsight(insight)">
                  <Workflow :size="14" />
                  转任务包
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="memory.buildWorkflowDraftFromInsight(insight)">
                  <Workflow :size="14" />
                  转工作流
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="memory.buildChecklistDraftFromInsight(insight)">
                  <Check :size="14" />
                  转清单
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
            <AriButton size="sm" variant="secondary" @click="memory.buildAgentTask()">
              <Workflow :size="14" />
              从当前记忆生成任务包
            </AriButton>
          </div>
          <div class="flow-asset-grid">
            <article class="flow-asset-card">
              <span>外部代理</span>
              <h2>{{ memory.agentTask?.goal || '未生成' }}</h2>
              <p>{{ memory.agentTask?.context || '选择一条记忆或一条洞察后生成可审阅任务包。' }}</p>
              <small v-if="memory.agentTask?.requiresReview">需要人工复核</small>
            </article>
            <article class="flow-asset-card">
              <span>候选工作流</span>
              <h2>{{ memory.workflowDraft?.title || '未生成' }}</h2>
              <p>{{ memory.workflowDraft?.trigger || '从重复流程里生成可保存的启动器工作流草稿。' }}</p>
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
            <article class="flow-asset-card">
              <span>检查清单</span>
              <h2>{{ memory.checklistDraft?.title || '未生成' }}</h2>
              <p>{{ memory.checklistDraft?.context || '把重复排查经验整理成可审阅清单。' }}</p>
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

            <pre class="preview-text memory-text flow-detail-text">{{ selected.text }}</pre>
            <pre v-if="selected.ocrText" class="preview-text memory-text flow-detail-text">OCR:
{{ selected.ocrText }}</pre>

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
