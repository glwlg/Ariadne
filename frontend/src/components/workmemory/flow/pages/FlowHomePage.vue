<script setup lang="ts">
import { computed, toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  ArrowRight,
  Check,
  Copy,
  Plus,
  Sparkles,
  X,
  activeFlowConversation,
  activeFlowConversationId,
  askFlow,
  buildCurrentMemoryTaskPackage,
  clearFlowChatSelection,
  closeFlowMessageMenu,
  copyFlowMessage,
  copySelectedFlowMessages,
  entryFocusTitle,
  evidenceCounts,
  flowBusy,
  flowCanvasActiveId,
  flowCanvasPrimaryEntry,
  flowChatInputRef,
  flowChatIsEmpty,
  flowChatMessages,
  flowChatThreadRef,
  flowConversationPreview,
  flowConversationTime,
  flowConversations,
  flowContextMenu,
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
  focusFlowChatInput,
  handleFlowMessageClick,
  isFlowMessageSelectable,
  isFlowMessageSelected,
  memory,
  openEvidence,
  openFlowMessageMenu,
  recentEvidence,
  rememberSelectedFlowMessages,
  selectFlowConversation,
  selectSingleFlowMessage,
  selectedFlowChatMessages,
  sourceLabel,
  startFlowConversation,
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
          <section class="flow-cognitive-shell" data-no-drag>
            <aside class="flow-cognitive-rail" aria-label="会话轨道">
              <div class="flow-cognitive-kicker">
                <span>CHAT HISTORY</span>
                <strong>对话记录</strong>
                <small>{{ flowConversations.length }} 个会话</small>
              </div>
              <button type="button" class="flow-new-conversation" @click.stop="startFlowConversation()">
                <Plus :size="15" />
                新对话
              </button>
              <div class="flow-canvas-history" :class="{ 'is-empty': !flowConversations.length }" data-no-drag>
                <article
                  v-for="conversation in flowConversations"
                  :key="conversation.id"
                  class="flow-canvas-history-item"
                  :class="{
                    'is-active': conversation.id === activeFlowConversationId,
                  }"
                  @click="selectFlowConversation(conversation.id)"
                >
                  <span class="flow-canvas-history-dot">AI</span>
                  <div>
                    <strong>{{ conversation.title }}</strong>
                    <p>{{ flowConversationPreview(conversation) }}</p>
                    <small>{{ flowConversationTime(conversation) }} · {{ conversation.messageCount }} 条消息</small>
                  </div>
                </article>
                <article v-if="!flowConversations.length" class="flow-canvas-history-item is-empty-state">
                  <span class="flow-canvas-history-dot">AI</span>
                  <div>
                    <strong>暂无会话</strong>
                    <p>发送第一条问题后会自动创建。</p>
                  </div>
                </article>
              </div>
            </aside>

            <main class="flow-cognitive-canvas" aria-label="心流对话">
              <section ref="flowChatThreadRef" class="flow-chat-dialog" :class="{ 'is-empty': flowChatIsEmpty }" data-no-drag>
                <article v-if="flowChatIsEmpty" class="flow-chat-empty">
                  <Sparkles :size="28" />
                  <h2>{{ activeFlowConversation?.title || '开始新的心流对话' }}</h2>
                  <p>直接提问，心流会结合今天的本地留痕回答。</p>
                </article>
                <article
                  v-for="message in flowChatMessages"
                  :key="message.id"
                  class="flow-message flow-message-row"
                  :class="{
                    'is-user': message.role === 'user',
                    'is-assistant': message.role === 'assistant',
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
                  <span class="flow-message-avatar">{{ message.role === 'user' ? '我' : 'AI' }}</span>
                  <div class="flow-message-bubble">
                    <header class="flow-message-meta">
                      <strong>{{ flowMessageRoleLabel(message) }}</strong>
                      <span>{{ flowMessageTime(message) }}</span>
                      <small v-if="flowMessageModeLabel(message)">{{ flowMessageModeLabel(message) }}</small>
                    </header>
                    <p v-if="message.role === 'user'" class="flow-message-text">{{ message.text }}</p>
                    <section v-else class="flow-message-markdown" v-html="flowMessageHtml(message)"></section>
                    <div v-if="message.result?.evidence.length" class="flow-message-foot">
                      <span>{{ flowMessageEvidenceLabel(message) }}</span>
                      <button type="button" @click.stop="flowCanvasActiveId = message.id">查看详情</button>
                    </div>
                  </div>
                </article>
              </section>

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
                  <strong>回答面板</strong>
                </div>
                <small>本地</small>
              </header>
              <section v-if="flowCanvasPrimaryEntry" class="flow-answer-side-panel">
                <span>当前问题</span>
                <strong>{{ flowCanvasPrimaryEntry.message.question || flowCanvasPrimaryEntry.message.text.slice(0, 64) }}</strong>
                <div class="flow-answer-side-meta">
                  <small>置信度 {{ flowCanvasPrimaryEntry.evidenceEntries.length ? '86%' : '待留痕' }}</small>
                  <small>{{ flowMessageModeLabel(flowCanvasPrimaryEntry.message) || '本地推理' }}</small>
                  <small>{{ flowMessageTime(flowCanvasPrimaryEntry.message) }}</small>
                </div>
                <div class="flow-answer-actions">
                  <button type="button" @click.stop="buildCurrentMemoryTaskPackage()">交给代理</button>
                  <button type="button" :disabled="!flowCanvasPrimaryEntry.evidenceEntries.length" @click.stop="openEvidence(flowCanvasPrimaryEntry.evidenceEntries[0])">
                    打开留痕
                  </button>
                  <button type="button" @click.stop="copyFlowMessage(flowCanvasPrimaryEntry.message)">复制结论</button>
                  <button type="button" @click.stop="selectSingleFlowMessage(flowCanvasPrimaryEntry.message); rememberSelectedFlowMessages()">加入沉淀</button>
                </div>
              </section>
              <section>
                <span>最近留痕</span>
                <button v-for="entry in recentEvidence.slice(0, 5)" :key="entry.id" type="button" @click.stop="openEvidence(entry)">
                  <strong>{{ entryFocusTitle(entry) }}</strong>
                  <small>{{ sourceLabel(entry) }} · {{ entry.appName || 'Unknown' }}</small>
                </button>
              </section>
              <section>
                <span>OCR / 质检</span>
                <div class="flow-agent-meter"><strong>OCR</strong><small>{{ evidenceCounts.ocr }} 条可检索</small></div>
                <div class="flow-agent-meter"><strong>质检</strong><small>{{ completedQualityCount }} 条完成</small></div>
                <div class="flow-agent-meter"><strong>影响</strong><small>引用 {{ flowCanvasPrimaryEntry?.evidenceEntries.length || 0 }} 条留痕</small></div>
              </section>
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

        </section>
</template>
