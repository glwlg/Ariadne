<script setup lang="ts">
import { computed, toRefs } from 'vue'
import FlowCognitiveTopbar from '../components/FlowCognitiveTopbar.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  ArrowRight,
  Camera,
  Check,
  Copy,
  FileText,
  Plus,
  Sparkles,
  X,
  appAvatarText,
  askFlow,
  buildCurrentMemoryTaskPackage,
  clearFlowChatSelection,
  closeFlowMessageMenu,
  copyFlowMessage,
  copySelectedFlowMessages,
  displayAppName,
  entryFocusTitle,
  evidenceCounts,
  evidenceExpanded,
  evidenceLabel,
  flowBusy,
  flowCanvasActiveId,
  flowCanvasEvidenceClusters,
  flowCanvasPrimaryEntry,
  flowChatInputRef,
  flowChatIsEmpty,
  flowChatMessages,
  flowChatThreadRef,
  flowContextMenu,
  flowEvidenceThumbGroups,
  flowMessageEvidenceLabel,
  flowMessageHtml,
  flowMessageModeClass,
  flowMessageModeLabel,
  flowMessageRoleLabel,
  flowMessageTime,
  flowQuestion,
  flowRememberFeedback,
  flowSelectionLabel,
  flowSuggestedQuestions,
  flowWindowPanelItems,
  focusFlowChatInput,
  formatTime,
  handleFlowMessageClick,
  isFlowMessageSelectable,
  isFlowMessageSelected,
  isScreenshotEntry,
  memory,
  openEvidence,
  openFlowMessageMenu,
  recentEvidence,
  rememberSelectedFlowMessages,
  selectSingleFlowMessage,
  selectedFlowChatMessages,
  sourceLabel,
  toggleFlowMessageSelection,
  useFlowQuestion,
} = toRefs(ctx)

void flowChatInputRef
void flowChatThreadRef

const completedQualityCount = computed(() => {
  const entries = recentEvidence.value as Array<{ qualityStatus?: string }>
  return entries.filter((entry) => entry.qualityStatus && entry.qualityStatus !== 'pending').length
})
</script>

<template>
<section class="flow-home flow-chat-home flow-cognitive-home" aria-label="我与心流的对话" @click="closeFlowMessageMenu()">
          <FlowCognitiveTopbar />
          <section class="flow-cognitive-shell" data-no-drag>
            <aside class="flow-cognitive-rail" aria-label="会话轨道">
              <div class="flow-cognitive-kicker">
                <span>COGNITIVE CANVAS</span>
                <strong>心流</strong>
                <small>{{ flowChatMessages.length }} 条消息 · {{ recentEvidence.length }} 条最近证据</small>
              </div>
              <div v-if="selectedFlowChatMessages.length" class="flow-selection-bar flow-cognitive-selection">
                <span>{{ flowSelectionLabel }}</span>
                <button type="button" @click.stop="copySelectedFlowMessages()">
                  <Copy :size="14" />
                  复制
                </button>
                <button type="button" @click.stop="rememberSelectedFlowMessages()">
                  <Plus :size="14" />
                  {{ flowRememberFeedback || '沉淀' }}
                </button>
                <button type="button" @click.stop="clearFlowChatSelection()">
                  <X :size="14" />
                </button>
              </div>
              <div ref="flowChatThreadRef" class="flow-canvas-history" :class="{ 'is-empty': flowChatIsEmpty }" data-no-drag>
                <article
                  v-for="message in flowChatMessages"
                  :key="message.id"
                  class="flow-canvas-history-item"
                  :class="{
                    'is-user': message.role === 'user',
                    'is-system': message.system,
                    'is-active': flowCanvasPrimaryEntry?.message.id === message.id,
                    'is-selected': isFlowMessageSelected(message),
                    'is-pending': message.pending,
                    'is-error': message.error,
                  }"
                  :data-mode="flowMessageModeClass(message)"
                  @click="handleFlowMessageClick(message, $event); if (message.role === 'assistant') flowCanvasActiveId = message.id"
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
                  <span class="flow-canvas-history-dot">{{ message.role === 'user' ? '我' : 'AI' }}</span>
                  <div>
                    <strong>{{ flowMessageRoleLabel(message) }}</strong>
                    <p>{{ message.text || '处理中...' }}</p>
                    <small>{{ flowMessageTime(message) }}</small>
                  </div>
                </article>
              </div>
              <div class="flow-evidence-thumb-stack" aria-label="证据缩略分组">
                <button v-for="group in flowEvidenceThumbGroups" :key="group.key" type="button" class="flow-evidence-thumb" @click.stop="evidenceExpanded = true">
                  <span>{{ group.icon }}</span>
                  <strong>{{ group.label }}</strong>
                  <small>{{ group.count }} 条</small>
                  <i v-if="group.entries.length">+{{ group.entries.length }}</i>
                </button>
              </div>
            </aside>

            <main class="flow-cognitive-canvas" aria-label="认知画布">
              <div v-if="flowCanvasPrimaryEntry" class="flow-cognitive-question">
                <span>当前问题</span>
                <strong>{{ flowCanvasPrimaryEntry.message.question || flowCanvasPrimaryEntry.message.text.slice(0, 64) }}</strong>
              </div>
              <article v-if="flowCanvasPrimaryEntry" class="flow-answer-surface">
                <header>
                  <span>结论</span>
                  <div>
                    <strong>置信度 {{ flowCanvasPrimaryEntry.evidenceEntries.length ? '86%' : '待证据' }}</strong>
                    <strong>{{ flowMessageModeLabel(flowCanvasPrimaryEntry.message) || '本地推理' }}</strong>
                    <small>{{ flowMessageTime(flowCanvasPrimaryEntry.message) }}</small>
                    <button type="button" @click.stop="buildCurrentMemoryTaskPackage()">交给代理</button>
                  </div>
                </header>
                <section class="flow-answer-markdown flow-message-markdown" v-html="flowMessageHtml(flowCanvasPrimaryEntry.message)"></section>
                <div v-if="flowCanvasPrimaryEntry.message.result?.evidence.length" class="flow-message-foot">
                  <button type="button" @click.stop="evidenceExpanded = true">
                    <Camera :size="14" />
                    {{ flowMessageEvidenceLabel(flowCanvasPrimaryEntry.message) }}
                  </button>
                  <span>{{ flowCanvasPrimaryEntry.message.result.usedAi ? 'AI 组织' : '本地归纳' }}</span>
                </div>
                <div class="flow-answer-grid">
                  <section>
                    <span>证据链</span>
                    <button
                      v-for="entry in flowCanvasPrimaryEntry.evidenceEntries.slice(0, 4)"
                      :key="entry.id"
                      type="button"
                      class="flow-answer-evidence"
                      @click.stop="openEvidence(entry)"
                    >
                      <Camera v-if="isScreenshotEntry(entry)" :size="14" />
                      <FileText v-else :size="14" />
                      <strong>{{ entryFocusTitle(entry) }}</strong>
                      <small>{{ sourceLabel(entry) }} · {{ formatTime(entry.createdAt) }}</small>
                    </button>
                    <p v-if="!flowCanvasPrimaryEntry.evidenceEntries.length">这次回答还没有可展开证据，适合先补充时间范围或关键词。</p>
                  </section>
                  <section>
                    <span>不确定项</span>
                    <p v-for="item in flowCanvasPrimaryEntry.uncertainty" :key="item">{{ item }}</p>
                  </section>
                  <section>
                    <span>建议动作</span>
                    <div class="flow-answer-actions">
                      <button type="button" :disabled="!flowCanvasPrimaryEntry.evidenceEntries.length" @click.stop="openEvidence(flowCanvasPrimaryEntry.evidenceEntries[0])">
                        打开证据
                      </button>
                      <button type="button" @click.stop="copyFlowMessage(flowCanvasPrimaryEntry.message)">复制结论</button>
                      <button type="button" @click.stop="selectSingleFlowMessage(flowCanvasPrimaryEntry.message); rememberSelectedFlowMessages()">加入沉淀</button>
                    </div>
                  </section>
                </div>
              </article>
              <article v-else class="flow-answer-surface is-empty">
                <Sparkles :size="28" />
                <h2>把当前上下文交给心流整理</h2>
                <p>提问后，这里会以结论、证据链、不确定项和建议动作呈现，而不是堆成普通聊天气泡。</p>
              </article>
              <aside class="flow-window-panel" aria-label="当前窗口">
                <div>
                  <span>窗口</span>
                  <strong>当前上下文</strong>
                </div>
                <button v-for="item in flowWindowPanelItems" :key="item.app" type="button" @click.stop="item.latest && openEvidence(item.latest)">
                  <span>{{ appAvatarText(item.app) }}</span>
                  <strong>{{ displayAppName(item.app) }}</strong>
                  <small>{{ item.count }} 条</small>
                </button>
                <p v-if="!flowWindowPanelItems.length">等待采集窗口上下文。</p>
              </aside>

              <footer class="flow-chat-composer flow-cognitive-composer" data-no-drag>
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
              </footer>
            </main>

            <aside class="flow-cognitive-inspector flow-agent-inspector" aria-label="Agent Inspector">
              <header class="flow-agent-inspector-head">
                <div>
                  <span>Agent Inspector</span>
                  <strong>证据检查器</strong>
                </div>
                <small>本地</small>
              </header>
              <section>
                <span>当前回答基于</span>
                <button v-for="cluster in flowCanvasEvidenceClusters" :key="cluster.source" type="button" @click.stop="evidenceExpanded = true">
                  <strong>{{ cluster.source }}</strong>
                  <small>{{ cluster.size }} 次引用</small>
                </button>
                <p v-if="!flowCanvasEvidenceClusters.length">回答生成后会在这里聚合截图、OCR、剪贴板和笔记证据。</p>
              </section>
              <section>
                <span>最近证据</span>
                <button v-for="entry in recentEvidence.slice(0, 5)" :key="entry.id" type="button" @click.stop="openEvidence(entry)">
                  <strong>{{ entryFocusTitle(entry) }}</strong>
                  <small>{{ sourceLabel(entry) }} · {{ entry.appName || 'Unknown' }}</small>
                </button>
              </section>
              <section>
                <span>OCR / 质检</span>
                <div class="flow-agent-meter"><strong>OCR</strong><small>{{ evidenceCounts.ocr }} 条可检索</small></div>
                <div class="flow-agent-meter"><strong>质检</strong><small>{{ completedQualityCount }} 条完成</small></div>
                <div class="flow-agent-meter"><strong>影响</strong><small>引用 {{ flowCanvasPrimaryEntry?.evidenceEntries.length || 0 }} 条证据</small></div>
              </section>
              <section>
                <span>隐私与边界</span>
                <p>{{ memory.status.privacyMode ? '隐私模式开启，敏感内容默认不会外发。' : '本地优先，外部 AI 动作需要显式确认。' }}</p>
              </section>
              <div class="flow-evidence-summary flow-chat-evidence-summary">
                <strong>证据</strong>
                <span><Camera :size="14" />{{ evidenceCounts.screenshots }}</span>
                <span><FileText :size="14" />{{ evidenceCounts.ocr }}</span>
                <span><Copy :size="14" />{{ evidenceCounts.clipboard }}</span>
                <button type="button" @click.stop="evidenceExpanded = !evidenceExpanded">{{ evidenceLabel() }}</button>
                <small v-if="flowRememberFeedback && !selectedFlowChatMessages.length">{{ flowRememberFeedback }}</small>
              </div>
            </aside>
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

          <section class="flow-evidence-panel flow-chat-evidence-panel flow-cognitive-evidence-panel" :class="{ 'is-expanded': evidenceExpanded }">
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
</template>
