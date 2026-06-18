<script setup lang="ts">
import { toRefs } from 'vue'
import AriToolbar from '../../../ui/AriToolbar.vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  FileText,
  Sparkles,
  activeDraft,
  activeDraftKind,
  activeDraftSourceSummary,
  draftEvidenceTimeline,
  draftItems,
  draftTimelineEntries,
  entryFocusTitle,
  flowDateLabel,
  formatTime,
  formatTimelineClock,
  memory,
  openEvidence,
  setActiveDraftKind,
  sourceLabel,
  vectorProviderLabel,
} = toRefs(ctx)
</script>

<template>
<section class="flow-page-panel flow-drafts-page" aria-label="心流草稿">
          <FlowPageHeader eyebrow="DRAFTS" :title="`摘要、复盘和知识草稿 · ${flowDateLabel}`" />
          <AriToolbar class="flow-page-toolbar">
            <span>时间范围 今日工作时段</span>
            <span>自动整理中 · 本地模型</span>
            <button type="button">列表</button>
            <button type="button">网格</button>
          </AriToolbar>
          <div class="flow-draft-studio">
            <aside class="flow-draft-tabs" aria-label="草稿类型">
              <button
                v-for="item in draftItems"
                :key="item.kind"
                type="button"
                :class="{ 'is-active': activeDraftKind === item.kind }"
                @click="setActiveDraftKind(item.kind)"
              >
                <span>{{ item.icon }}</span>
                <strong>{{ item.label }}</strong>
                <small>{{ item.draft ? `${item.createdAtLabel} · 证据 ${item.evidence.length}` : '未生成' }}</small>
              </button>
            </aside>

            <section class="flow-draft-document" aria-label="草稿预览">
              <div class="flow-draft-evidence-line">
                <span>证据时间线</span>
                <button v-for="(entry, index) in draftEvidenceTimeline.slice(0, 8)" :key="entry.id" type="button" class="flow-draft-thumb" @click="openEvidence(entry)">
                  <i>#{{ Number(index) + 1 }}</i>
                  <strong>{{ formatTimelineClock(entry.createdAt) }}</strong>
                  <small>{{ sourceLabel(entry) }}</small>
                </button>
                <p v-if="!draftEvidenceTimeline.length">{{ activeDraftSourceSummary }}</p>
              </div>
              <article class="flow-draft-paper">
                <span>{{ activeDraft?.label || '草稿' }} · v2 · AI 润色版</span>
                <h2>{{ activeDraft?.draft?.title || activeDraft?.title || '未生成' }}</h2>
                <div class="flow-draft-meta-line">
                  <small>{{ activeDraft?.draft?.body?.length || 0 }} 字</small>
                  <small>{{ draftTimelineEntries.length }} 条证据</small>
                  <small>时间同步</small>
                </div>
                <pre>{{ activeDraft?.draft?.body || activeDraft?.emptyHint || '等待生成草稿。' }}</pre>
                <div class="flow-draft-paragraph-map">
                  <button v-for="(entry, index) in draftTimelineEntries.slice(0, 6)" :key="entry.id" type="button" class="flow-draft-paragraph-badge" @click="openEvidence(entry)">
                    #{{ Number(index) + 1 }} {{ sourceLabel(entry) }}
                  </button>
                </div>
              </article>
            </section>

            <aside class="flow-draft-inspector" aria-label="来源段落">
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <FileText :size="15" />
                  来源段落
                </div>
                <strong>{{ activeDraftSourceSummary }}</strong>
                <button v-for="entry in draftTimelineEntries.slice(0, 6)" :key="entry.id" type="button" class="flow-answer-evidence" @click="openEvidence(entry)">
                  <strong>{{ entryFocusTitle(entry) }}</strong>
                  <small>{{ entry.appName || 'Unknown' }} · {{ formatTime(entry.createdAt) }}</small>
                </button>
                <p v-if="!draftTimelineEntries.length">生成草稿后，这里会显示段落引用的 OCR、截图或剪贴板证据。</p>
              </section>
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Sparkles :size="15" />
                  输出动作
                </div>
                <div class="memory-side-actions">
                  <AriButton v-if="activeDraftKind === 'daily' && memory.dailyDraft" size="sm" variant="secondary" :disabled="memory.isPolishingDailyDraft" @click="memory.polishDailyDraft()">
                    {{ memory.dailyDraftPolishArmed ? '确认外发润色' : 'AI 润色' }}
                  </AriButton>
                  <AriButton v-if="activeDraftKind === 'knowledge' && memory.knowledgeDraft" size="sm" variant="secondary" :disabled="memory.isSavingKnowledgeDraft" @click="memory.saveCurrentKnowledgeDraft()">
                    {{ memory.knowledgeDraftSaveArmed ? '确认保存' : '保存为 Skill' }}
                  </AriButton>
                  <AriButton size="sm" variant="secondary" @click="memory.buildDailyDraft()">生成日报</AriButton>
                  <AriButton size="sm" variant="secondary" @click="memory.buildRetrospectiveDraft()">生成复盘</AriButton>
                  <AriButton size="sm" variant="secondary" @click="memory.buildKnowledgeDraft()">生成知识</AriButton>
                </div>
              </section>
              <section class="flow-quiet-panel flow-polish-status">
                <div class="side-title">
                  <Sparkles :size="15" />
                  AI 润色状态
                </div>
                <span>模型：{{ vectorProviderLabel }}</span>
                <span>风格：工作复盘</span>
                <span>内容优化 ✓</span>
                <span>结构优化 ✓</span>
                <span>术语统一 ✓</span>
                <small>外发前会移除敏感字段，并保留本地证据链。</small>
              </section>
            </aside>
          </div>
        </section>
</template>
