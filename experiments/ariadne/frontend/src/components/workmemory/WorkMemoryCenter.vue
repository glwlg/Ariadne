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
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useWorkMemoryStore } from '../../stores/workMemory'
import { ocrConfidenceLabel, ocrRectLabel } from '../../lib/ocrDisplay'
import type { WorkMemoryEntry } from '../../types/ariadne'

const appShell = useAppShellStore()
const memory = useWorkMemoryStore()

const selected = computed(() => memory.selectedEntry)
const visibleEntries = computed(() => memory.filteredEntries)
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

onMounted(() => {
  void memory.load()
})
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell memory-shell" aria-label="Ariadne work memory center">
        <header class="launcher-header">
          <div class="brand-mark" aria-hidden="true">
            <Brain :size="18" />
          </div>
          <div class="brand-copy">
            <span>工作记忆中心</span>
            <small>Timeline, evidence, drafts</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="memory.status.timeMachineEnabled ? 'is-on' : ''">
              <Clock3 :size="13" />
              时间机器 {{ timeMachineLabel }}
            </span>
            <span class="system-pill" :class="memory.status.privacyMode ? 'is-danger' : ''">
              <Shield :size="13" />
              隐私 {{ memory.status.privacyMode ? '开启' : '关闭' }}
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
            <AriButton size="sm" variant="secondary" @click="appShell.openSettings()">
              <Settings :size="14" />
              设置
            </AriButton>
          </div>
        </header>

        <div class="memory-toolbar">
          <div class="search-row memory-search">
            <Search :size="18" class="text-[var(--muted)]" />
            <input
              :value="memory.query"
              class="search-input"
              spellcheck="false"
              placeholder="搜索 OCR、剪贴板、截图、窗口标题、标签或证据..."
              @input="memory.setQuery(($event.target as HTMLInputElement).value)"
            />
          </div>
          <div class="memory-actions">
            <AriButton size="sm" :variant="memory.proactiveSourcesEnabled ? 'secondary' : 'primary'" :disabled="memory.proactiveSourcesEnabled" @click="memory.enableProactiveSinking()">
              <Sparkles :size="14" />
              {{ memory.proactiveSourcesEnabled ? '主动沉淀已开' : '开启主动沉淀' }}
            </AriButton>
            <AriButton size="sm" :variant="memory.status.timeMachineEnabled ? 'secondary' : 'primary'" @click="memory.toggleTimeMachine()">
              <component :is="memory.status.timeMachineEnabled ? Pause : Play" :size="14" />
              {{ memory.status.timeMachineEnabled ? '暂停采集' : '开启时间机器' }}
            </AriButton>
            <AriButton size="sm" variant="secondary" @click="memory.captureNow()">
              <Camera :size="14" />
              手动补记
            </AriButton>
            <AriButton size="sm" :variant="memory.status.privacyMode ? 'primary' : 'secondary'" @click="memory.togglePrivacyMode()">
              <Shield :size="14" />
              {{ memory.status.privacyMode ? '关闭隐私' : '隐私模式' }}
            </AriButton>
            <AriButton size="sm" variant="secondary" @click="memory.exportData()">
              <Download :size="14" />
              导出
            </AriButton>
          </div>
        </div>

        <div class="memory-stats">
          <div>
            <span>条目</span>
            <strong>{{ memory.status.entryCount }}</strong>
          </div>
          <div>
            <span>筛选</span>
            <strong>{{ visibleEntries.length }}</strong>
          </div>
          <div>
            <span>命中</span>
            <strong>{{ memory.searchResults.length }}</strong>
          </div>
          <div>
            <span>向量</span>
            <strong>{{ vectorStatusLabel }}</strong>
          </div>
          <div>
            <span>采集</span>
            <strong>{{ memory.status.captureCount ?? 0 }}</strong>
          </div>
          <div>
            <span>范围</span>
            <strong>{{ captureScopeLabel }} · {{ multiMonitorLabel }}</strong>
          </div>
          <div>
            <span>保护</span>
            <strong>{{ activityLabel }}</strong>
          </div>
          <div>
            <span>OCR</span>
            <strong>{{ autoOCRLabel }}</strong>
          </div>
          <div>
            <span>跳过</span>
            <strong>{{ memory.status.lastSkippedReason ? '有记录' : '无' }}</strong>
          </div>
        </div>

        <div v-if="runtimeStatusText" class="memory-inline-status">
          {{ runtimeStatusText }}
        </div>

        <div class="memory-workspace">
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

        <footer class="status-strip">
          <span>
            <Check :size="14" />
            证据引用保留
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
  </main>
</template>
