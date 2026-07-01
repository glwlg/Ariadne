<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { parseCurlRequest } from '../../services/apiTestingApi'
import type { APIRequest } from '../../types/ariadne'

const props = defineProps<{
  open: boolean
  folder: string
}>()

const emit = defineEmits<{
  close: []
  create: [request: { folder: string } & Partial<Omit<APIRequest, 'id' | 'updatedAt'>>]
}>()

const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const requestTypes = [
  { value: 'HTTP', label: 'HTTP', disabled: false },
  { value: 'gRPC', label: 'gRPC', disabled: true },
  { value: 'From cURL', label: 'From cURL', disabled: false },
  { value: 'GraphQL', label: 'GraphQL', disabled: true },
  { value: 'WebSocket', label: 'WebSocket', disabled: true },
]
const draft = reactive({
  type: 'HTTP',
  name: '',
  method: 'GET',
  url: '',
  curl: '',
  error: '',
  folder: '',
  showOptions: false,
})

const isCurlMode = computed(() => draft.type === 'From cURL')
const canCreate = computed(() => {
  if (isCurlMode.value) return draft.curl.trim() !== ''
  return draft.type === 'HTTP' && draft.name.trim() !== '' && draft.url.trim() !== ''
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    draft.type = 'HTTP'
    draft.name = ''
    draft.method = 'GET'
    draft.url = ''
    draft.curl = ''
    draft.error = ''
    draft.folder = props.folder
    draft.showOptions = false
  },
)

function createRequest() {
  if (!canCreate.value) return
  draft.error = ''
  if (isCurlMode.value) {
    try {
      const parsed = parseCurlRequest(draft.curl)
      emit('create', {
        folder: draft.folder.trim(),
        ...parsed,
        name: draft.name.trim() || parsed.name || '新请求',
      })
    } catch (error) {
      draft.error = error instanceof Error ? error.message : 'cURL 解析失败'
    }
    return
  }
  emit('create', {
    folder: draft.folder.trim(),
    name: draft.name.trim(),
    method: draft.method,
    url: draft.url.trim(),
  })
}
</script>

<template>
  <div v-if="open" class="api-modal-layer" role="dialog" aria-modal="true" aria-label="新请求">
    <div class="api-modal-backdrop" @click="emit('close')" />
    <section class="api-new-request-modal">
      <header>
        <strong>New Request</strong>
        <button type="button" aria-label="关闭" @click="emit('close')">×</button>
      </header>

      <div class="api-new-request-body">
        <div class="api-new-request-types" aria-label="请求类型">
          <label v-for="type in requestTypes" :key="type.value" :class="{ 'is-disabled': type.disabled }">
            <input v-model="draft.type" type="radio" :value="type.value" :disabled="type.disabled" />
            <span>{{ type.label }}</span>
          </label>
        </div>

        <label class="api-field">
          <span>Request Name</span>
          <input v-model="draft.name" class="api-input" placeholder="Request Name" @keydown.enter="createRequest" />
        </label>

        <label v-if="!isCurlMode" class="api-field">
          <span>URL</span>
          <div class="api-new-request-url">
            <select v-model="draft.method" class="api-method-select">
              <option v-for="method in methods" :key="method" :value="method">{{ method }}</option>
            </select>
            <input v-model="draft.url" class="api-input" placeholder="Request URL" @keydown.enter="createRequest" />
          </div>
        </label>

        <label v-else class="api-field">
          <span>cURL</span>
          <textarea v-model="draft.curl" class="api-curl-textarea" spellcheck="false" placeholder="curl 'https://api.example.com/users' -H 'Authorization: Bearer {{token}}'" />
        </label>

        <div v-if="draft.error" class="api-dialog-error" role="alert">{{ draft.error }}</div>

        <label v-if="draft.showOptions" class="api-field">
          <span>Folder</span>
          <input v-model="draft.folder" class="api-input" placeholder="未分组" />
        </label>
      </div>

      <footer>
        <button type="button" class="api-new-request-options" :class="{ 'is-active': draft.showOptions }" @click="draft.showOptions = !draft.showOptions">Options</button>
        <div>
          <AriButton size="sm" variant="ghost" @click="emit('close')">Cancel</AriButton>
          <AriButton size="sm" variant="primary" :disabled="!canCreate" @click="createRequest">Create</AriButton>
        </div>
      </footer>
    </section>
  </div>
</template>
