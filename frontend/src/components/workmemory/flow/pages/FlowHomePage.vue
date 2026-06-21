<script setup lang="ts">
import { toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  ArrowRight,
  Check,
  Copy,
  Plus,
  RefreshCw,
  Sparkles,
  Trash2,
  X,
  activeFlowConversation,
  activeFlowConversationId,
  askFlow,
  clearFlowChatSelection,
  closeFlowMessageMenu,
  copyFlowContextMessage,
  copySelectedFlowMessages,
  flowBusy,
  flowCanvasActiveId,
  flowCanvasPrimaryEntry,
  flowCandidateActionLabel,
  flowCandidateInboxSummary,
  flowCandidateTimeLabel,
  flowChatInputRef,
  flowChatIsEmpty,
  flowChatMessages,
  flowChatSelectionMode,
  flowChatThreadRef,
  flowConversationDeleteArmedId,
  flowConversationPreview,
  flowConversationTime,
  flowConversations,
  flowContextMenu,
  flowContextMenuMessage,
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
  handleFlowCandidateAction,
  handleFlowMessageClick,
  isFlowMessageSelectable,
  isFlowMessageSelected,
  memory,
  openFlowMessageMenu,
  rememberFlowContextMessage,
  rememberSelectedFlowMessages,
  removeFlowConversation,
  runFlowAutonomy,
  selectFlowConversation,
  selectedFlowChatMessages,
  startFlowConversation,
  startFlowMessageMultiSelect,
  toggleFlowMessageSelection,
  useFlowQuestion,
} = toRefs(ctx)

void flowChatInputRef
void flowChatThreadRef
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
              <section class="flow-action-inbox" aria-label="主动动作">
                <header>
                  <div>
                    <span>主动动作</span>
                    <strong>{{ flowCandidateInboxSummary() }}</strong>
                  </div>
                  <button type="button" :disabled="memory.isRunningFlowAutonomy" title="检查主动动作" @click.stop="runFlowAutonomy()">
                    <RefreshCw :size="14" />
                  </button>
                </header>
                <div v-if="memory.flowCandidateActions?.items.length" class="flow-action-list">
                  <article v-for="action in memory.flowCandidateActions.items.slice(0, 3)" :key="action.id" class="flow-action-item">
                    <div class="flow-action-kicker">
                      <span>{{ flowCandidateActionLabel(action.actionType) }}</span>
                      <small>{{ flowCandidateTimeLabel(action) }}</small>
                    </div>
                    <strong>{{ action.title }}</strong>
                    <p>{{ action.summary }}</p>
                    <div class="flow-action-buttons">
                      <button
                        v-for="notificationAction in action.notificationActions"
                        :key="notificationAction.id"
                        type="button"
                        :class="`is-${notificationAction.kind || 'secondary'}`"
                        :disabled="memory.isDecidingFlowCandidate"
                        @click.stop="handleFlowCandidateAction(action, notificationAction)"
                      >
                        {{ notificationAction.label }}
                      </button>
                    </div>
                  </article>
                </div>
                <p v-else class="flow-action-empty">暂无待确认动作。</p>
              </section>
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
                  <button
                    type="button"
                    class="flow-canvas-history-delete"
                    :class="{ 'is-delete-armed': flowConversationDeleteArmedId === conversation.id }"
                    :disabled="memory.isDeletingFlowConversation"
                    :aria-label="flowConversationDeleteArmedId === conversation.id ? '确认删除对话' : '删除对话'"
                    :title="flowConversationDeleteArmedId === conversation.id ? '确认删除对话' : '删除对话'"
                    @click.stop="removeFlowConversation(conversation.id)"
                  >
                    <Trash2 :size="13" />
                  </button>
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
              <section ref="flowChatThreadRef" class="flow-chat-dialog" :class="{ 'is-empty': flowChatIsEmpty, 'is-selection-mode': flowChatSelectionMode }" data-no-drag>
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
                  @click="handleFlowMessageClick(message, $event); if (!flowChatSelectionMode && message.role === 'assistant') flowCanvasActiveId = message.id"
                  @contextmenu="openFlowMessageMenu($event, message)"
                >
                  <button
                    v-if="flowChatSelectionMode && isFlowMessageSelectable(message)"
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

          </section>

          <div
            v-if="flowContextMenu.open"
            class="flow-chat-context-menu"
            :style="{ left: `${flowContextMenu.x}px`, top: `${flowContextMenu.y}px` }"
            @click.stop
          >
            <button type="button" @click="startFlowMessageMultiSelect(flowContextMenuMessage)">
              <Check :size="14" />
              多选
            </button>
            <button type="button" @click="selectedFlowChatMessages.length ? rememberSelectedFlowMessages() : rememberFlowContextMessage()">
              <Plus :size="14" />
              加入沉淀
            </button>
            <button v-if="selectedFlowChatMessages.length" type="button" @click="copySelectedFlowMessages(); closeFlowMessageMenu()">
              <Copy :size="14" />
              复制选中
            </button>
            <button v-else type="button" @click="copyFlowContextMessage()">
              <Copy :size="14" />
              复制此消息
            </button>
            <button v-if="flowChatSelectionMode || selectedFlowChatMessages.length" type="button" @click="clearFlowChatSelection(); closeFlowMessageMenu()">
              <X :size="14" />
              清除选择
            </button>
          </div>

        </section>
</template>
