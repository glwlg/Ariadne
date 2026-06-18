<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRefs, watch } from 'vue'
import AriEmptyState from '../../../ui/AriEmptyState.vue'
import AriToolbar from '../../../ui/AriToolbar.vue'
import FlowDateSwitcher from '../components/FlowDateSwitcher.vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import FlowProgressStrip from '../components/FlowProgressStrip.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  ArrowLeft,
  ArrowRight,
  Camera,
  Check,
  ChevronDown,
  Clock3,
  Copy,
  Database,
  FileText,
  Flag,
  ImageOff,
  Search,
  Settings,
  Shield,
  addTimelineLaneAppToExclusions,
  addTimelineSelectionToRetrospective,
  appAvatarText,
  batchOcrProgressPercent,
  closeTimelineAppPicker,
  closeTimelineLaneMenu,
  deleteProgressPercent,
  entryEvidenceBadges,
  entryFocusSummary,
  entryFocusTitle,
  evidenceCounts,
  filteredTimelineAppOptions,
  flowDateButtonLabel,
  flowDateLabel,
  flowTimeRangeLabel,
  flowWorkHoursLabel,
  formatTimelineClock,
  isClipboardEntry,
  isOcrEntry,
  isScreenshotEntry,
  isTimelineAppExcluded,
  isTimelineSelected,
  loadMoreTimelineDays,
  memory,
  openEvidence,
  openFlowPage,
  openTimelineLaneMenu,
  openTimelinePlaybackDetail,
  openTimelinePlaybackTick,
  resetFlowDateToday,
  resetTimelineZoomRange,
  runTimelineBatchOCR,
  selectCurrentTimelineForRetrospective,
  selectTimelineAppFilter,
  selectedTimelineAppCount,
  selectedTimelineAppLabel,
  setTimelineFilter,
  setTimelineZoomRange,
  shiftFlowDate,
  timelineAppFilter,
  timelineAppOptions,
  timelineAppPickerOpen,
  timelineAppSearch,
  timelineAppSearchRef,
  timelineAppSelectRef,
  timelineAxisTicks,
  timelineBatchOcrEntries,
  timelineDensityBars,
  timelineEntries,
  timelineEventStyle,
  timelineFilterCounts,
  timelineFilters,
  timelineHasMoreDays,
  timelineLaneMenu,
  timelineLaneMenuRef,
  timelineLanes,
  timelineLoadMoreRef,
  timelinePlayStateLabel,
  timelineScrubPercent,
  timelineSelectedEntries,
  timelineSelectedSummary,
  timelineRangeSourceEntries,
  timelineSourceFilter,
  timelineStats,
  timelineThumbnailIsMissing,
  timelineThumbnailUrl,
  timelineZoomActive,
  timelineZoomEndHour,
  timelineZoomStartHour,
  toggleTimelineAppPicker,
  toggleTimelineSelection,
  topApps,
} = toRefs(ctx)

const timelineBoardRef = ref<HTMLElement | null>(null)
const timelineAxisScrollRef = ref<HTMLElement | null>(null)
const timelineRangeSelecting = ref(false)
const timelineRangeSelection = ref<{ start: number; end: number } | null>(null)
const timelineRangeSelectionStyle = computed(() => {
  if (!timelineRangeSelection.value) return {}
  const start = Math.min(timelineRangeSelection.value.start, timelineRangeSelection.value.end)
  const end = Math.max(timelineRangeSelection.value.start, timelineRangeSelection.value.end)
  return {
    left: `${start * 100}%`,
    width: `${Math.max((end - start) * 100, 0.35)}%`,
  }
})
const timelineTrackWidth = computed(() => {
  const visibleHours = Math.max(0.25, timelineZoomEndHour.value - timelineZoomStartHour.value)
  const hourWidth = visibleHours <= 1 ? 640 : visibleHours <= 2 ? 420 : visibleHours <= 4 ? 300 : visibleHours <= 8 ? 210 : 132
  const minimumWidth = 1440
  const entryWidth = timelineEntries.value.length * 48
  return `${Math.max(minimumWidth, entryWidth, visibleHours * hourWidth)}px`
})
let timelineScrollSyncing = false
let timelineRangePointerId = -1
let timelineRangeStartRatio = 0

function timelineScrollTargets() {
  const targets: HTMLElement[] = []
  if (timelineAxisScrollRef.value) {
    targets.push(timelineAxisScrollRef.value)
  }
  timelineBoardRef.value
    ?.querySelectorAll<HTMLElement>('.flow-forensic-track-scroll')
    .forEach((target) => targets.push(target))
  return targets
}

function setTimelineScrollLeft(scrollLeft: number, source?: HTMLElement | null) {
  timelineScrollSyncing = true
  for (const target of timelineScrollTargets()) {
    if (target !== source) {
      target.scrollLeft = scrollLeft
    }
  }
  requestAnimationFrame(() => {
    timelineScrollSyncing = false
  })
}

function syncTimelineScroll(event: Event) {
  if (timelineScrollSyncing) return
  const source = event.currentTarget as HTMLElement | null
  if (!source) return
  setTimelineScrollLeft(source.scrollLeft, source)
}

function timelinePointerRatio(event: PointerEvent) {
  const ruler = timelineAxisScrollRef.value
  if (!ruler) return 0
  const rect = ruler.getBoundingClientRect()
  const width = Math.max(1, ruler.scrollWidth)
  const x = event.clientX - rect.left + ruler.scrollLeft
  return Math.max(0, Math.min(1, x / width))
}

function timelineHourFromRatio(ratio: number) {
  return timelineZoomStartHour.value + ratio * (timelineZoomEndHour.value - timelineZoomStartHour.value)
}

function beginTimelineRulerSelection(event: PointerEvent) {
  if (event.button !== 0 || !timelineAxisScrollRef.value) return
  timelineRangeSelecting.value = true
  timelineRangePointerId = event.pointerId
  timelineRangeStartRatio = timelinePointerRatio(event)
  timelineRangeSelection.value = { start: timelineRangeStartRatio, end: timelineRangeStartRatio }
  timelineAxisScrollRef.value.setPointerCapture(event.pointerId)
  event.preventDefault()
}

function updateTimelineRulerSelection(event: PointerEvent) {
  if (!timelineRangeSelecting.value || event.pointerId !== timelineRangePointerId) return
  timelineRangeSelection.value = { start: timelineRangeStartRatio, end: timelinePointerRatio(event) }
  event.preventDefault()
}

function endTimelineRulerSelection(event?: PointerEvent) {
  if (!timelineRangeSelecting.value) return
  if (event && event.pointerId !== timelineRangePointerId) return
  const selection = timelineRangeSelection.value
  timelineRangeSelecting.value = false
  if (event && timelineAxisScrollRef.value?.hasPointerCapture(event.pointerId)) {
    timelineAxisScrollRef.value.releasePointerCapture(event.pointerId)
  }
  timelineRangePointerId = -1
  timelineRangeSelection.value = null
  if (!selection || Math.abs(selection.end - selection.start) < 0.012) return
  const startHour = timelineHourFromRatio(Math.min(selection.start, selection.end))
  const endHour = timelineHourFromRatio(Math.max(selection.start, selection.end))
  setTimelineZoomRange.value(startHour, endHour)
  void nextTick(() => setTimelineScrollLeft(0, timelineAxisScrollRef.value))
}

function resetTimelineRulerZoom() {
  resetTimelineZoomRange.value()
  void nextTick(() => setTimelineScrollLeft(0, timelineAxisScrollRef.value))
}

watch(
  () => [timelineLanes.value.length, timelineEntries.value.length, timelineTrackWidth.value, timelineZoomStartHour.value, timelineZoomEndHour.value],
  async () => {
    await nextTick()
    setTimelineScrollLeft(timelineAxisScrollRef.value?.scrollLeft ?? 0, timelineAxisScrollRef.value)
  },
  { flush: 'post' },
)

onMounted(async () => {
  await nextTick()
  setTimelineScrollLeft(0, timelineAxisScrollRef.value)
})

onBeforeUnmount(() => {
  endTimelineRulerSelection()
})

void timelineAppSearchRef
void timelineAppSelectRef
void timelineLaneMenuRef
void timelineLoadMoreRef
</script>

<template>
<section class="flow-page-panel flow-timeline-page" aria-label="心流时间线">
          <FlowPageHeader class="flow-timeline-hero" eyebrow="TIMELINE" :title="`时间线 · ${flowDateLabel}`" />
          <AriToolbar class="flow-page-toolbar flow-timeline-toolbar">
            <FlowDateSwitcher :label="flowDateButtonLabel" @previous="shiftFlowDate(-1)" @next="shiftFlowDate(1)" @reset="resetFlowDateToday()" />
            <span>时间范围 {{ flowTimeRangeLabel }}</span>
            <span>工作时间 {{ flowWorkHoursLabel }}</span>
            <button
              type="button"
              :title="memory.status.timeMachineEnabled ? '暂停屏幕时间机器' : '开启屏幕时间机器'"
              @click="memory.toggleTimeMachine()"
            >
              <Clock3 :size="14" />
              {{ memory.status.timeMachineEnabled ? '暂停采集' : '开启采集' }}
            </button>
            <button v-if="timelineZoomActive" type="button" title="恢复 06:00-22:00" @click="resetTimelineRulerZoom">
              <Clock3 :size="14" />
              恢复全时段
            </button>
          </AriToolbar>

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
                  <small>{{ timelineRangeSourceEntries.length }}</small>
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

          <div
            v-if="memory.isDeletingEntries || memory.deleteProgressTotal || memory.isBatchRecognizingOCR || memory.batchOcrProgressTotal"
            class="flow-timeline-progress-stack"
            aria-label="时间线批量任务进度"
          >
            <FlowProgressStrip
              v-if="memory.isDeletingEntries || memory.deleteProgressTotal"
              label="正在删除选中轨迹"
              :detail="`${memory.deleteProgressDone} / ${memory.deleteProgressTotal} 条`"
              :percent="deleteProgressPercent"
              danger
            />

            <FlowProgressStrip
              v-if="memory.isBatchRecognizingOCR || memory.batchOcrProgressTotal"
              label="正在补跑/重跑勾选项"
              :detail="`${memory.batchOcrProgressDone} / ${memory.batchOcrProgressTotal} 条`"
              :note="memory.batchOcrProgressStage"
              :percent="batchOcrProgressPercent"
            />
          </div>

          <div class="flow-forensic-layout">
            <section ref="timelineBoardRef" class="flow-forensic-board" :style="{ '--timeline-track-width': timelineTrackWidth }" aria-label="多轨时间留痕板">
              <div class="flow-forensic-axis">
                <div class="flow-forensic-axis-spacer" aria-hidden="true"></div>
                <div
                  ref="timelineAxisScrollRef"
                  class="flow-forensic-axis-scroll"
                  :class="{ 'is-dragging': timelineRangeSelecting }"
                  aria-label="时间尺"
                  @scroll="syncTimelineScroll"
                  @dblclick="resetTimelineRulerZoom"
                  @pointerdown="beginTimelineRulerSelection"
                  @pointermove="updateTimelineRulerSelection"
                  @pointerup="endTimelineRulerSelection"
                  @pointercancel="endTimelineRulerSelection"
                  @lostpointercapture="endTimelineRulerSelection"
                >
                  <div class="flow-forensic-time-content">
                    <div class="flow-forensic-density" aria-hidden="true">
                      <span
                        v-for="bar in timelineDensityBars"
                        :key="bar.key"
                        :style="{ left: `${bar.left}%`, width: `${bar.width}%`, height: `${bar.height}%` }"
                        :title="bar.label"
                      ></span>
                    </div>
                    <div v-if="timelineRangeSelection" class="flow-forensic-range-selection" :style="timelineRangeSelectionStyle"></div>
                    <span v-for="tick in timelineAxisTicks" :key="tick.label" class="flow-forensic-axis-tick" :style="{ left: `${tick.left}%` }">{{ tick.label }}</span>
                  </div>
                </div>
              </div>
              <div class="flow-forensic-lanes">
                <section v-for="lane in timelineLanes" :key="lane.key" class="flow-forensic-lane">
                  <header
                    :class="{
                      'is-excludable': Boolean(lane.appName),
                      'is-excluded': isTimelineAppExcluded(lane.appName),
                    }"
                    :title="lane.appName ? `右键将 ${lane.appName} 加入排除名单` : '该轨道不是应用程序轨道'"
                    @contextmenu.prevent.stop="openTimelineLaneMenu($event, lane)"
                  >
                    <strong>{{ lane.label }}</strong>
                    <small>{{ lane.entries.length }} 条</small>
                    <span v-if="lane.appName" class="flow-forensic-lane-hint">
                      {{ isTimelineAppExcluded(lane.appName) ? '已排除' : '右键排除' }}
                    </span>
                  </header>
                  <div class="flow-forensic-track-scroll" @scroll="syncTimelineScroll">
                    <div class="flow-forensic-track">
                      <article
                        v-for="(entry, entryIndex) in lane.entries"
                        :key="entry.id"
                        class="flow-forensic-event"
                        :class="{
                          'is-selected': entry.id === memory.selectedId,
                          'is-checked': isTimelineSelected(entry.id),
                          'is-sensitive': entry.sensitive,
                        }"
                        :style="timelineEventStyle(entry, entryIndex)"
                        :title="entryFocusTitle(entry)"
                      >
                        <button
                          type="button"
                          class="flow-forensic-select"
                          :aria-pressed="isTimelineSelected(entry.id)"
                          :aria-label="isTimelineSelected(entry.id) ? '取消勾选轨迹' : '勾选轨迹'"
                          @click.stop="toggleTimelineSelection(entry.id)"
                        >
                          <Check v-if="isTimelineSelected(entry.id)" :size="12" />
                        </button>
                        <button type="button" class="flow-forensic-open" @click="openEvidence(entry)">
                          <span
                            class="flow-forensic-thumb"
                            :class="{
                              'has-image': Boolean(timelineThumbnailUrl(entry)),
                              'is-image-missing': timelineThumbnailIsMissing(entry),
                            }"
                          >
                            <img v-if="timelineThumbnailUrl(entry)" :src="timelineThumbnailUrl(entry)" :alt="entryFocusTitle(entry)" loading="lazy" />
                            <template v-else-if="timelineThumbnailIsMissing(entry)">
                              <ImageOff :size="14" />
                              <span>已清理</span>
                            </template>
                            <Camera v-else-if="isScreenshotEntry(entry)" :size="14" />
                            <Copy v-else-if="isClipboardEntry(entry)" :size="14" />
                            <FileText v-else :size="14" />
                          </span>
                          <Camera v-if="isScreenshotEntry(entry)" :size="13" />
                          <Copy v-else-if="isClipboardEntry(entry)" :size="13" />
                          <FileText v-else :size="13" />
                          <span>{{ formatTimelineClock(entry.createdAt) }}</span>
                          <small>{{ entryFocusSummary(entry) }}</small>
                          <i v-if="entryEvidenceBadges(entry).length">{{ entryEvidenceBadges(entry)[0] }}</i>
                          <em v-if="isOcrEntry(entry)">OCR {{ entry.qualityOcrStatus === 'ok' ? '92%' : '待校验' }}</em>
                        </button>
                      </article>
                    </div>
                  </div>
                </section>
                <AriEmptyState
                  v-if="!timelineLanes.length"
                  class="flow-empty-card"
                  title="没有匹配的轨迹"
                  description="换一个来源筛选，或在主搜索里查找 OCR、剪贴板、窗口标题和笔记。"
                >
                  <template #icon>
                    <Clock3 :size="24" />
                  </template>
                </AriEmptyState>
              </div>
              <div v-if="timelineHasMoreDays" ref="timelineLoadMoreRef" class="flow-timeline-load-more">
                <span>继续加载更早轨迹</span>
                <button type="button" @click="loadMoreTimelineDays">立即加载</button>
              </div>
              <div
                v-if="timelineLaneMenu.open"
                ref="timelineLaneMenuRef"
                class="flow-timeline-lane-menu"
                :style="{ left: `${timelineLaneMenu.x}px`, top: `${timelineLaneMenu.y}px` }"
                role="menu"
                @click.stop
                @contextmenu.prevent.stop
              >
                <header>
                  <span>应用程序</span>
                  <strong>{{ timelineLaneMenu.appName }}</strong>
                  <small>{{ timelineLaneMenu.count }} 条轨迹</small>
                </header>
                <button
                  type="button"
                  role="menuitem"
                  :disabled="memory.isSavingExclusions || isTimelineAppExcluded(timelineLaneMenu.appName)"
                  @click="addTimelineLaneAppToExclusions()"
                >
                  <Shield :size="14" />
                  {{ isTimelineAppExcluded(timelineLaneMenu.appName) ? '已在排除名单' : '加入排除名单' }}
                </button>
                <button type="button" role="menuitem" @click="selectTimelineAppFilter(timelineLaneMenu.appName); closeTimelineLaneMenu()">
                  <Search :size="14" />
                  只看这个程序
                </button>
                <button type="button" role="menuitem" @click="openFlowPage('rules')">
                  <Settings :size="14" />
                  打开规则页
                </button>
              </div>
            </section>

            <div class="flow-timeline-lower-deck flow-timeline-side-deck" aria-label="时间线操作区">
              <section class="flow-time-machine-panel flow-quiet-panel" aria-label="屏幕时间机器">
                <header class="flow-time-machine-head">
                  <div class="side-title">
                    <Clock3 :size="15" />
                    屏幕时间机器
                  </div>
                  <small>{{ memory.playbackPosition }}</small>
                </header>
                <div class="flow-time-machine-body">
                  <div class="flow-time-machine-preview" :class="{ 'has-image': Boolean(memory.playbackImageUrl) }">
                    <img v-if="memory.playbackImageUrl" :src="memory.playbackImageUrl" alt="时间机器回放帧" />
                    <span v-else-if="memory.playbackImageMissing">原图已清理</span>
                    <span v-else>{{ memory.playbackEntries.length ? '选择轨迹开始回放' : '暂无截图帧' }}</span>
                  </div>
                  <div class="flow-time-machine-controls">
                    <strong>{{ timelinePlayStateLabel }}</strong>
                    <input
                      class="flow-playback-scrubber"
                      type="range"
                      min="1"
                      :max="Math.max(1, memory.playbackEntries.length)"
                      :value="Math.max(1, memory.playbackIndex + 1)"
                      :disabled="!memory.playbackEntries.length"
                      :style="{ '--progress': `${timelineScrubPercent}%` }"
                      @input="openTimelinePlaybackTick"
                    />
                    <div class="memory-side-actions flow-time-machine-actions">
                      <AriButton size="sm" variant="ghost" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(-1)">
                        <ArrowLeft :size="14" />
                      </AriButton>
                      <AriButton size="sm" variant="secondary" :disabled="!memory.playbackEntries.length || memory.isLoadingPlaybackImage" @click="openTimelinePlaybackDetail()">
                        <FileText :size="14" />
                        详情
                      </AriButton>
                      <AriButton size="sm" variant="ghost" :disabled="!memory.playbackEntries.length" @click="memory.stepPlayback(1)">
                        <ArrowRight :size="14" />
                      </AriButton>
                    </div>
                    <small>{{ memory.playbackEntries.length }} 帧</small>
                  </div>
                </div>
              </section>

              <section class="flow-timeline-support-card flow-quiet-panel">
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
                  <AriButton size="sm" variant="secondary" :disabled="!timelineBatchOcrEntries.length" @click="runTimelineBatchOCR()">
                    补跑 OCR
                  </AriButton>
                  <AriButton size="sm" variant="primary" :disabled="!memory.retrospectiveSelectionCount" @click="memory.buildRetrospectiveDraft()">
                    生成复盘
                  </AriButton>
                </div>
              </section>

              <section class="flow-timeline-support-card flow-quiet-panel">
                <div class="side-title">
                  <Database :size="15" />
                  留痕摘要
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
            </div>
          </div>
        </section>
</template>
