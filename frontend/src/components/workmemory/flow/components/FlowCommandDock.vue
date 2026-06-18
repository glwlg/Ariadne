<script setup lang="ts">
import { Trash2 } from '@lucide/vue'
import { computed, toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  activeFlowPage,
  addTimelineSelectionToRetrospective,
  assetFeedback,
  askFlow,
  buildAutomationFromInsight,
  buildChecklistFromInsight,
  buildCurrentMemoryTaskPackage,
  copyCurrentAgentTask,
  copyTimelineSelectionReference,
  deleteTimelineSelection,
  exportTimelineSelection,
  flowBusy,
  flowPages,
  flowQuestion,
  handoffInsightToAgent,
  memory,
  recentEvidence,
  runAutonomousFlow,
  runGlobalFlowSearch,
  runTimelineBatchOCR,
  selectedInsight,
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
  <footer class="flow-command-dock" data-no-drag>
    <div class="flow-command-scope">
      <span>{{ activePageLabel }}</span>
      <strong>{{ todayEntries.length }} 条上下文 · {{ recentEvidence.length }} 条证据</strong>
    </div>
    <template v-if="activeFlowPage === 'flow'">
      <button type="button" class="is-primary" :disabled="flowBusy || memory.isAskingFlow || !flowQuestion.trim()" @click="askFlow()">Ask</button>
      <button type="button" @click="memory.buildDailyDraft()">Summarize</button>
      <button type="button" @click="runGlobalFlowSearch()">Search</button>
      <button type="button" @click="buildCurrentMemoryTaskPackage()">Handoff</button>
      <button type="button" :disabled="memory.isRunningAutonomousFlow" @click="runAutonomousFlow()">Optimize</button>
    </template>
    <template v-else-if="activeFlowPage === 'timeline'">
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
    </template>
    <template v-else-if="activeFlowPage === 'insights'">
      <button type="button" class="is-primary" @click="memory.discoverExperienceReportAI()">运行 AI 归纳</button>
      <button type="button" @click="memory.buildDailyDraft()">生成每日复盘</button>
      <button type="button" :disabled="!selectedInsight" @click="selectedInsight && handoffInsightToAgent(selectedInsight)">创建任务包</button>
      <button type="button" disabled>导出洞察报告</button>
      <button type="button" @click="memory.refreshEmbedding()">更新索引</button>
    </template>
    <template v-else-if="activeFlowPage === 'drafts'">
      <button type="button" class="is-primary" @click="memory.buildDailyDraft()">生成日报</button>
      <button type="button" @click="memory.buildRetrospectiveDraft()">生成复盘</button>
      <button type="button" @click="memory.buildKnowledgeDraft()">生成知识</button>
      <button type="button" :disabled="!memory.dailyDraft" @click="memory.polishDailyDraft()">AI 润色</button>
      <button type="button" :disabled="!memory.knowledgeDraft" @click="memory.saveCurrentKnowledgeDraft()">保存知识</button>
    </template>
    <template v-else-if="activeFlowPage === 'assets'">
      <button type="button" class="is-primary" :disabled="!memory.agentTask" @click="copyCurrentAgentTask()">复制任务包</button>
      <button type="button" @click="selectedInsight && buildAutomationFromInsight(selectedInsight)" :disabled="!selectedInsight">生成工作流</button>
      <button type="button" @click="selectedInsight && buildChecklistFromInsight(selectedInsight)" :disabled="!selectedInsight">生成检查清单</button>
      <button type="button" :disabled="!memory.knowledgeDraft" @click="memory.saveCurrentKnowledgeDraft()">保存为 Skill</button>
      <button type="button" :disabled="!memory.knowledgeDraftSaveResult?.ok" @click="memory.exportCurrentKnowledgeSkill()">导出</button>
    </template>
    <template v-else-if="activeFlowPage === 'rules'">
      <button type="button" @click="memory.captureNow()">手动补记</button>
      <button type="button" @click="memory.importMaterials()">导入材料</button>
      <button type="button" class="is-primary" :disabled="memory.isSavingExclusions" @click="memory.saveExclusionRules()">保存排除规则</button>
      <button type="button" :disabled="memory.isRefreshingEmbedding" @click="memory.refreshEmbedding()">刷新索引</button>
      <button type="button" @click="memory.exportData()">导出数据</button>
    </template>
    <span v-if="memory.feedback || timelineExclusionFeedback || assetFeedback" class="flow-command-feedback">{{ memory.feedback || timelineExclusionFeedback || assetFeedback }}</span>
  </footer>
</template>
