<script setup lang="ts">
import { toRefs } from 'vue'
import AriEmptyState from '../../../ui/AriEmptyState.vue'
import AriSearchBox from '../../../ui/AriSearchBox.vue'
import AriToolbar from '../../../ui/AriToolbar.vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import FlowProgressStrip from '../components/FlowProgressStrip.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  Camera,
  Check,
  Sparkles,
  Workflow,
  buildAutomationFromInsight,
  buildChecklistFromInsight,
  confidenceLabel,
  decisionLabel,
  entryFocusTitle,
  flowDateLabel,
  formatTime,
  globalFlowSearch,
  handoffInsightToAgent,
  insightEvidencePreview,
  insightLinks,
  insightNodeStyle,
  insightNodes,
  insightProgressPercent,
  memory,
  openEvidence,
  runGlobalFlowSearch,
  selectedInsight,
  setActiveInsight,
  sourceLabel,
} = toRefs(ctx)
</script>

<template>
<section class="flow-page-panel flow-insights-page" aria-label="心流洞察">
          <FlowPageHeader eyebrow="INSIGHTS" :title="`自动归纳的线索 · ${flowDateLabel}`" />
          <AriToolbar class="flow-page-toolbar">
            <span>时间范围 最近 7 天</span>
            <span>进度 {{ insightProgressPercent }}%</span>
            <button type="button" @click="memory.discoverExperienceReport()">本地归纳</button>
            <button type="button" @click="memory.discoverExperienceReportAI()">AI 归纳</button>
            <AriSearchBox v-model="globalFlowSearch" class="flow-global-search is-compact" compact placeholder="搜索洞察、留痕、建议动作..." @keydown.enter.prevent="runGlobalFlowSearch()" />
          </AriToolbar>
          <div v-if="memory.experienceDiscoveryResult" class="flow-note-strip">
            {{ memory.experienceDiscoveryResult.message }}
            <template v-if="memory.experienceDiscoveryResult.provider || memory.experienceDiscoveryResult.model">
              · {{ memory.experienceDiscoveryResult.provider }} / {{ memory.experienceDiscoveryResult.model }}
            </template>
          </div>
          <FlowProgressStrip
            v-if="memory.isDiscoveringExperienceAI || memory.experienceDiscoveryProgress"
            :label="memory.experienceDiscoveryStage || 'AI 正在归纳'"
            :detail="`${insightProgressPercent}%`"
            :percent="insightProgressPercent"
          />
          <div class="flow-radar-layout">
            <section class="flow-radar-map" aria-label="模式雷达">
              <svg class="flow-radar-links" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                <line
                  v-for="link in insightLinks"
                  :key="link.id"
                  class="flow-radar-link"
                  :class="`is-strength-${link.strength}`"
                  :x1="link.x1"
                  :y1="link.y1"
                  :x2="link.x2"
                  :y2="link.y2"
                />
              </svg>
              <div class="flow-radar-core">
                <Sparkles :size="22" />
                <strong>模式雷达</strong>
                <small>{{ memory.experienceReport?.insights.length || 0 }} 个发现</small>
              </div>
              <button
                v-for="node in insightNodes"
                :key="node.insight.id"
                type="button"
                class="flow-radar-node"
                :class="{ 'is-active': selectedInsight?.id === node.insight.id }"
                :data-kind="node.insight.kind"
                :style="insightNodeStyle(node)"
                @click="setActiveInsight(node.insight)"
              >
                <span>{{ node.insight.kind }}</span>
                <strong>{{ node.insight.title }}</strong>
                <small>{{ confidenceLabel(node.insight.confidence) }}</small>
              </button>
              <AriEmptyState
                v-if="!insightNodes.length"
                class="flow-empty-card"
                title="还没有稳定洞察"
                description="点击“本地归纳”后，系统会从最近记录里找重复问题、知识沉淀和自动化机会。"
              >
                <template #icon>
                  <Sparkles :size="22" />
                </template>
              </AriEmptyState>
              <div class="flow-radar-legend">
                <span>实线 强关联</span>
                <span>虚线 弱关联</span>
                <span>节点大小 留痕链数量</span>
              </div>
            </section>
            <aside class="flow-radar-inspector flow-agent-inspector" aria-label="洞察解释">
              <header class="flow-agent-inspector-head">
                <div>
                  <span>Insight Inspector</span>
                  <strong>洞察详情</strong>
                </div>
                <small>{{ selectedInsight ? confidenceLabel(selectedInsight.confidence) : '无' }}</small>
              </header>
              <section v-if="selectedInsight" class="flow-quiet-panel">
                <div class="flow-insight-head">
                  <div>
                    <span>{{ selectedInsight.kind }} · {{ selectedInsight.severity }}</span>
                    <h2>{{ selectedInsight.title }}</h2>
                  </div>
                  <strong>{{ confidenceLabel(selectedInsight.confidence) }}</strong>
                </div>
                <p>{{ selectedInsight.summary }}</p>
                <small>{{ selectedInsight.reason }}</small>
                <div class="experience-meta">
                  <span>留痕 {{ selectedInsight.evidence.length }}</span>
                  <span>{{ decisionLabel(selectedInsight.decisionStatus) }}</span>
                  <span>{{ formatTime(selectedInsight.createdAt) }}</span>
                </div>
                <div class="flow-insight-actions">
                  <AriButton size="sm" variant="secondary" title="生成可复制的外部代理任务包，交给 Codex 或其他 agent 处理" @click="handoffInsightToAgent(selectedInsight)">
                    <Workflow :size="14" />
                    交给代理
                  </AriButton>
                  <AriButton size="sm" variant="secondary" title="生成可保存到 Ariadne 的自动化工作流草稿" @click="buildAutomationFromInsight(selectedInsight)">
                    <Workflow :size="14" />
                    生成自动化
                  </AriButton>
                  <AriButton size="sm" variant="secondary" title="生成可复用的检查清单草稿" @click="buildChecklistFromInsight(selectedInsight)">
                    <Check :size="14" />
                    生成检查清单
                  </AriButton>
                </div>
              </section>
              <section v-if="selectedInsight" class="flow-quiet-panel">
                <div class="side-title">
                  <Sparkles :size="15" />
                  建议动作
                </div>
                <div class="flow-action-card-grid">
                  <button type="button" @click="handoffInsightToAgent(selectedInsight)">交给代理</button>
                  <button type="button" @click="buildAutomationFromInsight(selectedInsight)">生成自动化</button>
                  <button type="button" @click="buildChecklistFromInsight(selectedInsight)">生成清单</button>
                  <button type="button" @click="memory.buildDailyDraft()">加入复盘</button>
                </div>
                <small>隐私边界：只引用本地留痕，外发任务包前需要人工确认。</small>
              </section>
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Camera :size="15" />
                  留痕链
                </div>
                <button v-for="entry in insightEvidencePreview" :key="entry.id" type="button" class="flow-answer-evidence" @click="openEvidence(entry)">
                  <strong>{{ entryFocusTitle(entry) }}</strong>
                  <small>{{ sourceLabel(entry) }} · {{ formatTime(entry.createdAt) }}</small>
                </button>
                <p v-if="!insightEvidencePreview.length">选择一个洞察后查看它引用的留痕。</p>
              </section>
            </aside>
          </div>
        </section>
</template>
