<script setup lang="ts">
import { CheckCircle2, Clock3, Copy, FileText, XCircle } from '@lucide/vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import FancunButton from '../ui/FancunButton.vue'
import ApiJsonViewer from './ApiJsonViewer.vue'
import { useAPITestingStore, type APIResponseTab } from '../../stores/apiTesting'

const LARGE_JSON_TEXT_BYTES = 100 * 1024
const LARGE_JSON_NODE_LIMIT = 1500

const apiTesting = useAPITestingStore()
const forceJsonTree = ref(false)
const nowMs = ref(Date.now())
let elapsedTimer: number | undefined

const responseTabs: Array<{ id: APIResponseTab; label: string }> = [
  { id: 'body', label: '响应' },
  { id: 'headers', label: '响应头' },
  { id: 'assertions', label: '断言结果' },
]

const currentRunning = computed(() =>
  apiTesting.selectedRequest ? apiTesting.isRequestRunning(apiTesting.selectedRequest.id) : apiTesting.isRunning,
)
const currentStopping = computed(() =>
  apiTesting.selectedRequest ? apiTesting.isRequestStopping(apiTesting.selectedRequest.id) : apiTesting.isStopping,
)
const elapsedSeconds = computed(() => {
  const startedAt = apiTesting.currentRunStartedAt
  if (!currentRunning.value || !startedAt) return 0
  return Math.max(0, Math.floor((nowMs.value - startedAt) / 1000))
})

watch(
  currentRunning,
  (running) => {
    if (elapsedTimer !== undefined) {
      window.clearInterval(elapsedTimer)
      elapsedTimer = undefined
    }
    if (running) {
      nowMs.value = Date.now()
      elapsedTimer = window.setInterval(() => {
        nowMs.value = Date.now()
      }, 500)
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (elapsedTimer !== undefined) {
    window.clearInterval(elapsedTimer)
    elapsedTimer = undefined
  }
})

const statusTone = computed(() => {
  const result = apiTesting.lastResult
  if (!result) return 'idle'
  if (!result.ok || result.statusCode >= 500 || result.failed > 0) return 'danger'
  if (result.statusCode >= 400) return 'warning'
  return 'ok'
})

type ResponseBodyView = { kind: 'json'; value: unknown; rawLength: number } | { kind: 'text'; text: string }

const responseBody = computed<ResponseBodyView>(() => {
  const body = apiTesting.lastResult?.body || ''
  if (apiTesting.lastResult?.streaming) return { kind: 'text', text: body || 'SSE 连接已建立，暂未收到事件' }
  if (!body) return { kind: 'text', text: '没有响应体' }
  try {
    return { kind: 'json', value: JSON.parse(body), rawLength: body.length }
  } catch {
    return { kind: 'text', text: body }
  }
})

watch(
  () => apiTesting.lastResult,
  () => {
    forceJsonTree.value = false
  },
)

function exceedsJsonNodeLimit(value: unknown, limit: number): boolean {
  let count = 0
  const stack: unknown[] = [value]
  while (stack.length > 0) {
    const current = stack.pop()
    count += 1
    if (count > limit) return true
    if (Array.isArray(current)) {
      for (let i = 0; i < current.length; i += 1) {
        stack.push(current[i])
      }
    } else if (current !== null && typeof current === 'object') {
      for (const item of Object.values(current)) {
        stack.push(item)
      }
    }
  }
  return false
}

const isLargeJson = computed(() => {
  if (responseBody.value.kind !== 'json') return false
  if (responseBody.value.rawLength > LARGE_JSON_TEXT_BYTES) return true
  return exceedsJsonNodeLimit(responseBody.value.value, LARGE_JSON_NODE_LIMIT)
})

const formattedLargeJsonText = computed(() => {
  if (responseBody.value.kind !== 'json' || !isLargeJson.value || forceJsonTree.value) return ''
  try {
    return JSON.stringify(responseBody.value.value, null, 2)
  } catch {
    return apiTesting.lastResult?.body || ''
  }
})

type ResponseDiagnostic = { label: string; tone: 'neutral' | 'warning' | 'danger' }

const responseDiagnostics = computed<ResponseDiagnostic[]>(() => {
  const result = apiTesting.lastResult
  if (!result) return []

  const diagnostics: ResponseDiagnostic[] = []
  if (result.contentType?.trim()) diagnostics.push({ label: result.contentType.trim(), tone: 'neutral' })
  if (result.bodyTruncated) diagnostics.push({ label: '响应已截断', tone: 'warning' })
  if (result.missingVariables?.length) {
    diagnostics.push({ label: `缺失变量：${result.missingVariables.join('、')}`, tone: 'warning' })
  }
  if (result.error?.trim()) diagnostics.push({ label: `错误：${result.error.trim()}`, tone: 'danger' })
  if (result.message?.trim() && result.message.trim() !== result.error?.trim()) {
    diagnostics.push({ label: result.message.trim(), tone: result.ok ? 'neutral' : 'danger' })
  }
  return diagnostics
})

const showRequestUrl = computed(() => {
  const requestUrl = apiTesting.lastResult?.requestUrl?.trim()
  const sourceUrl = apiTesting.selectedRequest?.url?.trim()
  return Boolean(requestUrl && requestUrl !== sourceUrl)
})

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}
</script>

<template>
  <aside class="api-response-panel" aria-label="API 响应">
    <template v-if="apiTesting.lastResult">
      <header class="api-response-head">
        <nav class="api-response-tabs" aria-label="响应内容">
          <button v-for="tab in responseTabs" :key="tab.id" :class="{ 'is-active': apiTesting.responseTab === tab.id }" @click="apiTesting.responseTab = tab.id">
            {{ tab.label }}
          </button>
        </nav>
        <div class="api-response-meta" :class="`is-${statusTone}`">
          <strong>{{ apiTesting.lastResult.statusCode || '--' }} {{ apiTesting.lastResult.statusText.replace(String(apiTesting.lastResult.statusCode), '').trim() }}</strong>
          <span v-if="apiTesting.lastResult.streaming">SSE</span>
          <span>{{ apiTesting.lastResult.durationMs }} ms</span>
          <span>{{ formatBytes(apiTesting.lastResult.bodySize) }}</span>
          <span>{{ apiTesting.lastResult.passed }}/{{ apiTesting.lastResult.assertionResults.length }} 断言</span>
        </div>
      </header>

      <div v-if="showRequestUrl" class="api-response-url" :title="apiTesting.lastResult.requestUrl">{{ apiTesting.lastResult.requestUrl }}</div>

      <div v-if="responseDiagnostics.length" class="api-response-diagnostics" aria-label="响应诊断">
        <span v-for="diagnostic in responseDiagnostics" :key="`${diagnostic.tone}:${diagnostic.label}`" :class="`is-${diagnostic.tone}`">
          {{ diagnostic.label }}
        </span>
      </div>

      <section v-if="apiTesting.responseTab === 'body'" class="api-response-section">
        <div class="api-section-title">
          <span>响应体</span>
          <div class="flex items-center gap-2">
            <FancunButton
              v-if="responseBody.kind === 'json' && isLargeJson"
              size="sm"
              variant="ghost"
              @click="forceJsonTree = !forceJsonTree"
            >
              {{ forceJsonTree ? '切换快速文本预览' : '仍以树形展开 (节点较多)' }}
            </FancunButton>
            <FancunButton size="sm" variant="secondary" @click="apiTesting.copyResponseBody()">
              <Copy :size="14" />
              复制
            </FancunButton>
          </div>
        </div>
        <template v-if="responseBody.kind === 'json'">
          <pre v-if="isLargeJson && !forceJsonTree" class="api-response-body">{{ formattedLargeJsonText }}</pre>
          <ApiJsonViewer v-else class="api-json-viewer" :value="responseBody.value" root />
        </template>
        <pre v-else class="api-response-body">{{ responseBody.text }}</pre>
      </section>

      <section v-else-if="apiTesting.responseTab === 'headers'" class="api-response-section">
        <div class="api-section-title">
          <span>响应头</span>
          <small>{{ apiTesting.lastResult.headers.length }}</small>
        </div>
        <div class="api-response-headers">
          <div v-for="header in apiTesting.lastResult.headers" :key="header.id">
            <span>{{ header.name }}</span>
            <small>{{ header.value }}</small>
          </div>
          <div v-if="!apiTesting.lastResult.headers.length" class="api-empty-row">暂无响应头</div>
        </div>
      </section>

      <section v-else class="api-response-section">
        <div class="api-section-title">
          <span>断言结果</span>
          <small>{{ apiTesting.lastResult.passed }}/{{ apiTesting.lastResult.assertionResults.length }}</small>
        </div>
        <div class="api-assertion-results">
          <div v-for="assertion in apiTesting.lastResult.assertionResults" :key="assertion.id" class="api-assertion-result" :class="{ 'is-passed': assertion.passed }">
            <CheckCircle2 v-if="assertion.passed" :size="15" />
            <XCircle v-else :size="15" />
            <span>{{ assertion.kind }} {{ assertion.target || assertion.operator }}</span>
            <small>{{ assertion.actual || assertion.message }}</small>
          </div>
          <div v-if="!apiTesting.lastResult.assertionResults.length" class="api-empty-row">没有启用的断言</div>
        </div>
      </section>

    </template>

    <div v-else-if="currentRunning" class="api-empty-panel">
      <Clock3 :size="22" />
      <span>{{ currentStopping ? '正在停止请求' : '正在接收响应' }} · {{ elapsedSeconds }}s</span>
      <small v-if="apiTesting.requestTimeoutSeconds" class="text-xs text-[var(--muted)]">超时设置 {{ apiTesting.requestTimeoutSeconds }}s</small>
    </div>

    <div v-else class="api-empty-panel">
      <FileText :size="22" />
      <span>发送请求后查看响应</span>
    </div>
  </aside>
</template>
