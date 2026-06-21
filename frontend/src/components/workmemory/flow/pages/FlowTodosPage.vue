<script setup lang="ts">
import {
  Bell,
  CalendarDays,
  Check,
  ChevronDown,
  Clock3,
  Flag,
  List,
  MapPin,
  MoreHorizontal,
  Play,
  Plus,
  Search,
  Sparkles,
  Tag,
} from '@lucide/vue'
import { computed, reactive, ref, toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'
import type { WorkMemoryTodoItem, WorkMemoryTodoPriority, WorkMemoryTodoStatus } from '../../../../types/ariadne'

const ctx = useWorkMemoryFlowContext()
const {
  askFlow,
  formatTime,
  memory,
} = toRefs(ctx)

const statusOptions: Array<{ id: WorkMemoryTodoStatus; label: string }> = [
  { id: 'open', label: '待处理' },
  { id: 'doing', label: '进行中' },
  { id: 'waiting', label: '稍后' },
  { id: 'done', label: '已完成' },
  { id: 'canceled', label: '已取消' },
]

const priorityOptions: Array<{ id: WorkMemoryTodoPriority; label: string }> = [
  { id: 'urgent', label: '紧急' },
  { id: 'high', label: '高' },
  { id: 'normal', label: '普通' },
  { id: 'low', label: '低' },
]

const draft = reactive({
  id: '',
  title: '',
  note: '',
  status: 'open' as WorkMemoryTodoStatus,
  priority: 'normal' as WorkMemoryTodoPriority,
  scope: '',
  trace: '',
  dueDate: '',
  remindDate: '',
})

const query = ref('')
const quickAddOpen = ref(false)
const selectedTodoId = ref('')
const completedOpen = ref(false)

const todoList = computed(() => memory.value.todoList)
const todos = computed<WorkMemoryTodoItem[]>(() => todoList.value?.items ?? [])
const activeTodos = computed(() => todos.value.filter((item) => item.status !== 'done' && item.status !== 'canceled'))
const completedTodos = computed(() => todos.value.filter((item) => item.status === 'done'))
const visibleActiveTodos = computed(() => {
  const needle = query.value.trim().toLowerCase()
  if (!needle) return activeTodos.value
  return activeTodos.value.filter((item) => {
    const haystack = [
      item.title,
      normalizedText(item.note),
      item.scope,
      statusLabel(item.status),
      priorityLabel(item.priority),
      scheduleText(item),
    ]
      .join(' ')
      .toLowerCase()
    return haystack.includes(needle)
  })
})
const focusTodo = computed(() => {
  const selected = activeTodos.value.find((item) => item.id === selectedTodoId.value)
  if (selected) return selected
  return activeTodos.value.find((item) => item.status === 'doing') ?? activeTodos.value.find((item) => item.status === 'open') ?? activeTodos.value[0] ?? null
})
const canSave = computed(() => Boolean(draft.title.trim() && !memory.value.isSavingTodo))

function openNewTodo() {
  resetDraft()
  quickAddOpen.value = true
}

function resetDraft() {
  draft.id = ''
  draft.title = ''
  draft.note = ''
  draft.status = 'open'
  draft.priority = 'normal'
  draft.scope = ''
  draft.trace = ''
  draft.dueDate = ''
  draft.remindDate = ''
}

function selectTodo(todo: WorkMemoryTodoItem) {
  selectedTodoId.value = todo.id
}

function editTodo(todo: WorkMemoryTodoItem) {
  selectTodo(todo)
  draft.id = todo.id
  draft.title = todo.title
  draft.note = todo.note ?? ''
  draft.status = todo.status
  draft.priority = todo.priority
  draft.scope = todo.scope ?? ''
  draft.trace = todo.evidence.join(', ')
  draft.dueDate = unixToDate(todo.dueAt)
  draft.remindDate = unixToDate(todo.remindAt)
  quickAddOpen.value = true
}

async function saveDraft() {
  if (!canSave.value) return
  const ok = await memory.value.saveTodo({
    id: draft.id,
    title: draft.title,
    note: draft.note,
    status: draft.status,
    priority: draft.priority,
    scope: draft.scope,
    source: 'manual',
    evidence: splitTrace(draft.trace),
    dueAt: dateToUnix(draft.dueDate),
    remindAt: dateToUnix(draft.remindDate),
  })
  if (ok) {
    quickAddOpen.value = false
    resetDraft()
  }
}

async function setStatus(todo: WorkMemoryTodoItem, status: WorkMemoryTodoStatus) {
  selectedTodoId.value = todo.id
  await memory.value.changeTodo({ id: todo.id, status })
}

function runFlowReview() {
  void askFlow.value?.('今天还有什么事没办的吗？')
}

function statusLabel(status: WorkMemoryTodoStatus) {
  return statusOptions.find((item) => item.id === status)?.label ?? '待处理'
}

function priorityLabel(priority: WorkMemoryTodoPriority) {
  return priorityOptions.find((item) => item.id === priority)?.label ?? '普通'
}

function priorityClass(priority: WorkMemoryTodoPriority) {
  return `is-${priority}`
}

function normalizedText(value?: string) {
  return (value ?? '')
    .replace(/\\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/\n{2,}/g, '\n')
    .trim()
}

function oneLine(value?: string) {
  return normalizedText(value).replace(/\s+/g, ' ').trim()
}

function truncate(value: string, max = 46) {
  if (value.length <= max) return value
  return `${value.slice(0, max - 1)}…`
}

function extractField(todo: WorkMemoryTodoItem, label: string) {
  const source = normalizedText(`${todo.title}\n${todo.note ?? ''}`)
  const match = source.match(new RegExp(`${label}\\s*[：:]\\s*([^\\n|]+)`))
  return match?.[1]?.trim().replace(/[，,。；;]$/, '') ?? ''
}

function scheduleText(todo: WorkMemoryTodoItem) {
  const parsedDate = extractField(todo, '日期')
  const parsedTime = extractField(todo, '时间')
  if (parsedDate || parsedTime) {
    return [parsedDate, parsedTime].filter(Boolean).join(' ')
  }
  if (todo.dueAt) {
    return formatTime.value?.(todo.dueAt) ?? ''
  }
  return '时间未定'
}

function dueText(todo: WorkMemoryTodoItem) {
  const parsedDate = extractField(todo, '日期')
  const parsedTime = extractField(todo, '时间')
  if (parsedDate || parsedTime) {
    const endTime = parsedTime.match(/(\d{1,2}:\d{2})\s*[–—-]\s*(\d{1,2}:\d{2})/)?.[2]
    return [parsedDate, endTime || parsedTime].filter(Boolean).join(' ')
  }
  return todo.dueAt ? formatTime.value?.(todo.dueAt) ?? '未设置' : '未设置'
}

function locationText(todo: WorkMemoryTodoItem) {
  return extractField(todo, '地点') || '未设置'
}

function nextStepText(todo: WorkMemoryTodoItem) {
  const explicit = extractField(todo, '下一步')
  if (explicit) return truncate(explicit, 68)
  const note = oneLine(todo.note)
  if (note && note.length <= 84) return note
  return todo.status === 'doing' ? '继续推进当前事项。' : '确认下一步并开始处理。'
}

function todoSubtitle(todo: WorkMemoryTodoItem) {
  const note = oneLine(todo.note)
  if (!note) return statusLabel(todo.status)
  return truncate(note, 92)
}

function scopeText(todo: WorkMemoryTodoItem) {
  return todo.scope?.trim() || extractField(todo, '项目') || '待办'
}

function reminderText(todo: WorkMemoryTodoItem) {
  if (todo.remindAt) return formatTime.value?.(todo.remindAt) ?? '已设置'
  return '未设置'
}

function splitTrace(value: string) {
  return value
    .split(/[,\s，、;；]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function dateToUnix(value: string) {
  if (!value) return 0
  const parsed = new Date(`${value}T23:59:59`)
  const timestamp = Math.floor(parsed.getTime() / 1000)
  return Number.isFinite(timestamp) ? timestamp : 0
}

function unixToDate(value?: number) {
  if (!value) return ''
  const date = new Date(value * 1000)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 10)
}
</script>

<template>
  <section class="flow-page-panel flow-todos-page" aria-label="待办">
    <div class="flow-todos-canvas">
      <header class="flow-todos-topbar">
        <h1>待办</h1>
        <div class="flow-todos-top-actions">
          <label class="flow-todos-search-control">
            <Search :size="18" />
            <input v-model="query" type="search" spellcheck="false" placeholder="搜索待办" />
          </label>
          <button type="button" class="flow-todos-button is-primary" @click="openNewTodo()">
            <Plus :size="18" />
            新建
          </button>
          <button type="button" class="flow-todos-button" :disabled="memory.isAskingFlow" @click="runFlowReview()">
            <Sparkles :size="17" />
            从心流整理
          </button>
        </div>
      </header>

      <div class="flow-todos-board">
        <main class="flow-todos-main">
          <section v-if="focusTodo" class="flow-todos-focus-card">
            <header class="flow-todos-card-title">
              <span>
                <span class="flow-todos-title-icon"><List :size="17" /></span>
                下一件事
              </span>
              <button type="button" title="编辑" @click="editTodo(focusTodo)">
                <MoreHorizontal :size="20" />
              </button>
            </header>

            <div class="flow-todos-focus-body">
              <div class="flow-todos-focus-symbol" aria-hidden="true">
                <CalendarDays :size="38" />
                <span><Check :size="13" /></span>
              </div>

              <div class="flow-todos-focus-content">
                <h2>{{ focusTodo.title }}</h2>
                <div class="flow-todos-focus-meta">
                  <span>{{ scheduleText(focusTodo) }}</span>
                  <span class="flow-meta-divider" aria-hidden="true"></span>
                  <span class="flow-flag-meta" :class="priorityClass(focusTodo.priority)">
                    <Flag :size="15" />
                    {{ priorityLabel(focusTodo.priority) }}
                  </span>
                  <span class="flow-scope-pill">{{ scopeText(focusTodo) }}</span>
                </div>
                <p>下一步：{{ nextStepText(focusTodo) }}</p>
              </div>
            </div>

            <div class="flow-todos-focus-actions">
              <button v-if="focusTodo.status !== 'doing'" type="button" class="flow-action-button is-primary" @click="setStatus(focusTodo, 'doing')">
                <Play :size="16" />
                开始处理
              </button>
              <button type="button" class="flow-action-button is-success" @click="setStatus(focusTodo, 'done')">
                <Check :size="17" />
                完成
              </button>
              <button v-if="focusTodo.status !== 'waiting'" type="button" class="flow-action-button" @click="setStatus(focusTodo, 'waiting')">
                <Clock3 :size="17" />
                稍后
              </button>
            </div>
          </section>

          <section v-else class="flow-todos-focus-card is-empty">
            <header class="flow-todos-card-title">
              <span>
                <span class="flow-todos-title-icon"><List :size="17" /></span>
                下一件事
              </span>
            </header>
            <div class="flow-todos-empty-focus">
              <strong>现在没有要跟进的事</strong>
              <p>有新事项时，可以手动添加，也可以让心流从最近工作里整理。</p>
              <div>
                <button type="button" class="flow-action-button is-primary" @click="openNewTodo()">新建待办</button>
                <button type="button" class="flow-action-button" :disabled="memory.isAskingFlow" @click="runFlowReview()">从心流整理</button>
              </div>
            </div>
          </section>

          <section v-if="quickAddOpen" class="flow-todo-quick-add">
            <header>
              <strong>{{ draft.id ? '编辑待办' : '新建待办' }}</strong>
              <button type="button" @click="quickAddOpen = false">收起</button>
            </header>
            <div class="flow-todo-quick-fields">
              <label>
                <span>事项</span>
                <input v-model="draft.title" spellcheck="false" placeholder="例如：确认端午值班安排" />
              </label>
              <label class="is-wide">
                <span>下一步</span>
                <input v-model="draft.note" spellcheck="false" placeholder="写清下一步即可" />
              </label>
              <label>
                <span>状态</span>
                <select v-model="draft.status">
                  <option v-for="status in statusOptions" :key="status.id" :value="status.id">{{ status.label }}</option>
                </select>
              </label>
              <label>
                <span>优先级</span>
                <select v-model="draft.priority">
                  <option v-for="priority in priorityOptions" :key="priority.id" :value="priority.id">{{ priority.label }}</option>
                </select>
              </label>
              <label>
                <span>范围</span>
                <input v-model="draft.scope" spellcheck="false" placeholder="工作、值班、项目" />
              </label>
              <label>
                <span>截止</span>
                <input v-model="draft.dueDate" type="date" />
              </label>
              <label>
                <span>提醒</span>
                <input v-model="draft.remindDate" type="date" />
              </label>
              <label class="is-wide">
                <span>留痕</span>
                <input v-model="draft.trace" spellcheck="false" placeholder="可选，多个用逗号分隔" />
              </label>
            </div>
            <div class="flow-todo-quick-actions">
              <button type="button" class="flow-todos-button is-primary" :disabled="!canSave" @click="saveDraft()">
                <Plus :size="16" />
                {{ memory.isSavingTodo ? '保存中' : '保存' }}
              </button>
              <button type="button" class="flow-todos-button" @click="resetDraft()">清空</button>
            </div>
          </section>

          <section class="flow-todos-list-card">
            <header class="flow-todos-list-title">
              <div>
                <strong>接下来</strong>
                <span>{{ visibleActiveTodos.length }}</span>
              </div>
            </header>

            <div class="flow-todos-table" :class="{ 'is-empty': !visibleActiveTodos.length }">
              <article
                v-for="todo in visibleActiveTodos"
                :key="todo.id"
                class="flow-todo-row"
                :class="{ 'is-selected': todo.id === focusTodo?.id }"
                role="button"
                tabindex="0"
                @click="selectTodo(todo)"
                @keydown.enter.prevent="selectTodo(todo)"
              >
                <button type="button" class="flow-todo-check" title="完成" @click.stop="setStatus(todo, 'done')"></button>
                <span :class="['flow-todo-flag', priorityClass(todo.priority)]">
                  <Flag :size="18" />
                </span>
                <div class="flow-todo-title">
                  <strong>{{ todo.title }}</strong>
                  <small>{{ todoSubtitle(todo) }}</small>
                </div>
                <span class="flow-scope-pill">{{ scopeText(todo) }}</span>
                <span class="flow-todo-time">
                  <CalendarDays :size="17" />
                  {{ scheduleText(todo) }}
                </span>
                <span class="flow-todo-trace">
                  <List :size="16" />
                  留痕 · {{ todo.evidence.length }}
                </span>
                <button type="button" class="flow-todo-more" title="编辑" @click.stop="editTodo(todo)">
                  <MoreHorizontal :size="19" />
                </button>
              </article>
              <p v-if="!visibleActiveTodos.length">没有匹配的待办。</p>
            </div>
          </section>

          <section class="flow-todos-completed">
            <button type="button" @click="completedOpen = !completedOpen">
              <span>已完成</span>
              <strong>{{ completedTodos.length }}</strong>
              <ChevronDown :size="18" :class="{ 'is-open': completedOpen }" />
            </button>
            <div v-if="completedOpen" class="flow-todos-completed-list">
              <article v-for="todo in completedTodos.slice(0, 8)" :key="todo.id" @click="selectTodo(todo)">
                <span>{{ todo.title }}</span>
                <small>{{ todo.completedAt ? formatTime(todo.completedAt) : '已完成' }}</small>
              </article>
              <p v-if="!completedTodos.length">还没有完成项。</p>
            </div>
          </section>
        </main>

        <aside class="flow-todos-reminder">
          <header>
            <Bell :size="22" />
            <strong>提醒</strong>
          </header>

          <template v-if="focusTodo">
            <div class="flow-reminder-section">
              <span><CalendarDays :size="19" />截止时间</span>
              <strong>{{ dueText(focusTodo) }}</strong>
            </div>
            <div class="flow-reminder-section">
              <span><Tag :size="19" />范围</span>
              <strong class="flow-scope-pill">{{ scopeText(focusTodo) }}</strong>
            </div>
            <div class="flow-reminder-section">
              <span><MapPin :size="19" />地点</span>
              <strong>{{ locationText(focusTodo) }}</strong>
            </div>
            <div class="flow-reminder-section">
              <span><Bell :size="19" />提醒设置</span>
              <strong>{{ reminderText(focusTodo) }}</strong>
            </div>
            <button type="button" class="flow-reminder-trace" @click="selectTodo(focusTodo)">
              <span><List :size="18" />来自 {{ focusTodo.evidence.length }} 条留痕</span>
              <ChevronDown :size="18" />
            </button>
          </template>

          <p v-else>新的事项会显示在这里。</p>
        </aside>
      </div>

    </div>
  </section>
</template>
