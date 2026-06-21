<script setup lang="ts">
import { computed, reactive, toRefs } from 'vue'
import AriInput from '../../../ui/AriInput.vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import { useWorkMemoryFlowContext } from '../context'
import type {
  WorkMemorySelfAssertion,
  WorkMemorySelfAssertionCategory,
  WorkMemorySelfAssertionPrivacy,
  WorkMemorySelfAssertionStatus,
} from '../../../../types/ariadne'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  Plus,
  Shield,
  Trash2,
  UserRound,
  formatTime,
  memory,
} = toRefs(ctx)

const draft = reactive({
  id: '',
  category: 'identity' as WorkMemorySelfAssertionCategory,
  key: 'name',
  label: '姓名',
  value: '',
  status: 'confirmed' as WorkMemorySelfAssertionStatus,
  privacy: 'always' as WorkMemorySelfAssertionPrivacy,
  scope: '',
})

const categoryOptions: Array<{ id: WorkMemorySelfAssertionCategory; label: string; note: string }> = [
  { id: 'identity', label: '身份', note: '姓名、称呼、账号名、角色、当前项目' },
  { id: 'preference', label: '偏好', note: '语言、语气、回答长度、工作方式' },
  { id: 'relationship', label: '关系', note: '联系人、群聊、项目成员' },
  { id: 'boundary', label: '边界', note: '自动整理、确认、外发限制' },
]

const presetFields = [
  { category: 'identity' as const, key: 'name', label: '姓名', privacy: 'always' as const },
  { category: 'identity' as const, key: 'nickname', label: '常用称呼', privacy: 'always' as const },
  { category: 'identity' as const, key: 'account', label: '账号显示名', privacy: 'always' as const },
  { category: 'identity' as const, key: 'role', label: '角色', privacy: 'always' as const },
  { category: 'identity' as const, key: 'project', label: '当前项目', privacy: 'always' as const },
  { category: 'preference' as const, key: 'language', label: '语言偏好', privacy: 'always' as const },
  { category: 'preference' as const, key: 'tone', label: '回答语气', privacy: 'always' as const },
  { category: 'relationship' as const, key: 'contact', label: '联系人', privacy: 'relevant' as const },
  { category: 'boundary' as const, key: 'external_reply', label: '外部回复边界', privacy: 'always' as const },
]

const assertions = computed<WorkMemorySelfAssertion[]>(() => memory.value.selfModel?.assertions ?? [])
const promptAssertions = computed<WorkMemorySelfAssertion[]>(() => memory.value.selfModel?.summary.included ?? [])
const groupedAssertions = computed(() => {
  return categoryOptions.map((category) => ({
    ...category,
    assertions: assertions.value.filter((assertion) => assertion.category === category.id),
  }))
})
const confirmedCount = computed(() => assertions.value.filter((assertion) => assertion.status === 'confirmed').length)
const observedCount = computed(() => assertions.value.filter((assertion) => assertion.status === 'observed').length)
const protectedCount = computed(() => assertions.value.filter((assertion) => assertion.privacy !== 'always').length)
const canSave = computed(() => Boolean(draft.label.trim() && draft.value.trim() && !memory.value.isSavingSelfAssertion))

function applyPreset(preset: (typeof presetFields)[number]) {
  draft.id = ''
  draft.category = preset.category
  draft.key = preset.key
  draft.label = preset.label
  draft.privacy = preset.privacy
  draft.status = 'confirmed'
  draft.scope = ''
}

function editAssertion(assertion: WorkMemorySelfAssertion) {
  draft.id = assertion.id
  draft.category = assertion.category
  draft.key = assertion.key
  draft.label = assertion.label
  draft.value = assertion.value
  draft.status = assertion.status
  draft.privacy = assertion.privacy
  draft.scope = assertion.scope ?? ''
}

function resetDraft() {
  draft.id = ''
  draft.category = 'identity'
  draft.key = 'name'
  draft.label = '姓名'
  draft.value = ''
  draft.status = 'confirmed'
  draft.privacy = 'always'
  draft.scope = ''
}

async function saveDraft() {
  if (!canSave.value) return
  const ok = await memory.value.saveSelfAssertion({
    id: draft.id,
    category: draft.category,
    key: draft.key.trim() || draft.label.trim(),
    label: draft.label,
    value: draft.value,
    status: draft.status,
    privacy: draft.privacy,
    scope: draft.scope,
    source: 'manual',
  })
  if (ok) {
    resetDraft()
  }
}

function categoryLabel(category: WorkMemorySelfAssertionCategory) {
  return categoryOptions.find((item) => item.id === category)?.label ?? '资料'
}

function statusLabel(status: WorkMemorySelfAssertionStatus) {
  const labels: Record<WorkMemorySelfAssertionStatus, string> = {
    confirmed: '已确认',
    observed: '观察到',
    rejected: '已否定',
    ephemeral: '本次有效',
  }
  return labels[status]
}

function privacyLabel(privacy: WorkMemorySelfAssertionPrivacy) {
  const labels: Record<WorkMemorySelfAssertionPrivacy, string> = {
    always: '可进入上下文',
    relevant: '相关时使用',
    never: '仅本地',
  }
  return labels[privacy]
}
</script>

<template>
  <section class="flow-page-panel flow-me-page" aria-label="我">
    <FlowPageHeader eyebrow="SELF MODEL" title="我" />

    <div class="flow-me-layout">
      <aside class="flow-me-editor side-panel">
        <div class="side-title">
          <UserRound :size="15" />
          资料条目
        </div>

        <div class="flow-me-presets" aria-label="常用资料">
          <button v-for="preset in presetFields" :key="`${preset.category}-${preset.key}`" type="button" @click="applyPreset(preset)">
            {{ preset.label }}
          </button>
        </div>

        <label class="flow-setting-field">
          <span>分组</span>
          <select v-model="draft.category">
            <option v-for="category in categoryOptions" :key="category.id" :value="category.id">{{ category.label }}</option>
          </select>
        </label>
        <label class="flow-setting-field">
          <span>名称</span>
          <input v-model="draft.label" spellcheck="false" placeholder="例如：姓名、当前项目、回答语气" />
        </label>
        <label class="flow-setting-field">
          <span>内容</span>
          <textarea v-model="draft.value" spellcheck="false" placeholder="填写心流理解你时可引用的内容"></textarea>
        </label>
        <div class="flow-settings-field-grid is-compact">
          <label class="flow-setting-field">
            <span>状态</span>
            <select v-model="draft.status">
              <option value="confirmed">已确认</option>
              <option value="observed">观察到</option>
              <option value="rejected">已否定</option>
              <option value="ephemeral">本次有效</option>
            </select>
          </label>
          <label class="flow-setting-field">
            <span>权限</span>
            <select v-model="draft.privacy">
              <option value="always">可进入上下文</option>
              <option value="relevant">相关时使用</option>
              <option value="never">仅本地</option>
            </select>
          </label>
        </div>
        <AriInput v-model="draft.scope" class="memory-note-input" spellcheck="false" placeholder="适用范围，例如：工作、DMS v2、群聊" />

        <div class="memory-side-actions">
          <AriButton size="sm" variant="primary" :disabled="!canSave" @click="saveDraft()">
            <Plus :size="14" />
            {{ memory.isSavingSelfAssertion ? '保存中' : draft.id ? '保存修改' : '保存资料' }}
          </AriButton>
          <AriButton size="sm" variant="ghost" @click="resetDraft()">清空</AriButton>
        </div>
      </aside>

      <main class="flow-me-main">
        <section class="flow-me-stats">
          <article>
            <span>已确认</span>
            <strong>{{ confirmedCount }}</strong>
            <small>长期可用资料</small>
          </article>
          <article>
            <span>观察到</span>
            <strong>{{ observedCount }}</strong>
            <small>等待确认</small>
          </article>
          <article>
            <span>受保护</span>
            <strong>{{ protectedCount }}</strong>
            <small>不会默认外发</small>
          </article>
          <article>
            <span>上下文</span>
            <strong>{{ promptAssertions.length }}</strong>
            <small>本次可用</small>
          </article>
        </section>

        <section class="flow-me-groups">
          <article v-for="group in groupedAssertions" :key="group.id" class="flow-me-group">
            <header>
              <div>
                <span>{{ group.label }}</span>
                <strong>{{ group.assertions.length }} 条</strong>
              </div>
              <small>{{ group.note }}</small>
            </header>
            <div class="flow-me-assertions" :class="{ 'is-empty': !group.assertions.length }">
              <article
                v-for="assertion in group.assertions"
                :key="assertion.id"
                class="flow-me-assertion"
                :class="{ 'is-prompt-ready': assertion.promptReady }"
                role="button"
                tabindex="0"
                @click="editAssertion(assertion)"
                @keydown.enter.prevent="editAssertion(assertion)"
              >
                <span>{{ assertion.label || assertion.key }}</span>
                <strong>{{ assertion.value }}</strong>
                <small>{{ statusLabel(assertion.status) }} · {{ privacyLabel(assertion.privacy) }} · {{ assertion.scope || categoryLabel(assertion.category) }}</small>
                <i>{{ assertion.updatedAt ? formatTime(assertion.updatedAt) : '刚刚' }}</i>
                <button type="button" class="flow-me-delete" aria-label="删除资料" @click.stop="memory.removeSelfAssertion(assertion)">
                  <Trash2 :size="13" />
                </button>
              </article>
              <p v-if="!group.assertions.length">暂无{{ group.label }}资料。</p>
            </div>
          </article>
        </section>
      </main>

      <aside class="flow-me-inspector flow-agent-inspector">
        <header class="flow-agent-inspector-head">
          <div>
            <span>Context</span>
            <strong>安全摘要</strong>
          </div>
          <small>本地筛选</small>
        </header>
        <section class="side-panel flow-me-summary">
          <div class="side-title">
            <Shield :size="15" />
            可进入模型上下文
          </div>
          <p>{{ memory.selfModel?.summary.prompt || '暂无可用资料' }}</p>
        </section>
        <section class="side-panel flow-me-summary">
          <div class="side-title">
            <Shield :size="15" />
            归因边界
          </div>
          <ul>
            <li>别人说的“你/我”保留原发言人视角。</li>
            <li>没有明确留痕时，待办和承诺保持不确定。</li>
            <li>已确认资料辅助判断，但不覆盖当前留痕。</li>
          </ul>
        </section>
      </aside>
    </div>
  </section>
</template>
