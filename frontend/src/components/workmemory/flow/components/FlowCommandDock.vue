<script setup lang="ts">
import { Trash2 } from '@lucide/vue'
import { computed, toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  activeFlowPage,
  addTimelineSelectionToRetrospective,
  assetFeedback,
  copyTimelineSelectionReference,
  deleteTimelineSelection,
  exportTimelineSelection,
  flowPages,
  memory,
  recentEvidence,
  runTimelineBatchOCR,
  timelineBatchOcrEntries,
  timelineDeleteLabel,
  timelineEntries,
  timelineExclusionFeedback,
  timelineSelectAllLabel,
  timelineSelectedEntries,
  todayEntries,
  toggleCurrentTimelineSelection,
} = toRefs(ctx)

const activePageLabel = computed(() => {
  const pages = flowPages.value as Array<{ id: string; label: string }>
  return pages.find((page) => page.id === activeFlowPage.value)?.label || ''
})
</script>

<template>
  <footer v-if="activeFlowPage === 'timeline'" class="flow-command-dock" data-no-drag>
    <div class="flow-command-scope">
      <span>{{ activePageLabel }}</span>
      <strong>{{ todayEntries.length }} 条上下文 · {{ recentEvidence.length }} 条留痕</strong>
    </div>
    <button type="button" :disabled="!timelineEntries.length" @click="toggleCurrentTimelineSelection()">{{ timelineSelectAllLabel }}</button>
    <button type="button" class="is-primary" :disabled="!timelineBatchOcrEntries.length" @click="runTimelineBatchOCR()">补跑 OCR+质检</button>
    <button type="button" :disabled="!timelineSelectedEntries.length" @click="addTimelineSelectionToRetrospective()">加入复盘</button>
    <button type="button" :disabled="!timelineSelectedEntries.length" @click="exportTimelineSelection()">导出所选</button>
    <button type="button" @click="copyTimelineSelectionReference()">复制链接</button>
    <button type="button" class="is-danger" :disabled="!timelineSelectedEntries.length || memory.isDeletingEntries" @click="deleteTimelineSelection()">
      <Trash2 :size="13" />
      {{ timelineDeleteLabel }}
    </button>
    <button type="button" disabled>标记敏感</button>
    <span v-if="memory.feedback || timelineExclusionFeedback || assetFeedback" class="flow-command-feedback">{{ memory.feedback || timelineExclusionFeedback || assetFeedback }}</span>
  </footer>
</template>
