<script setup lang="ts">
import { CheckCircle2, Clock3, Copy, FileText, ListChecks, XCircle } from '@lucide/vue'
import { computed } from 'vue'
import AriButton from '../ui/AriButton.vue'
import ApiJsonViewer from './ApiJsonViewer.vue'
import { useAPITestingStore, type APIResponseTab } from '../../stores/apiTesting'

const apiTesting = useAPITestingStore()

const responseTabs: Array<{ id: APIResponseTab; label: string }> = [
  { id: 'body', label: 'Response' },
  { id: 'headers', label: 'Headers' },
  { id: 'assertions', label: 'Assert' },
]

const statusTone = computed(() => {
  const result = apiTesting.lastResult
  if (!result) return 'idle'
  if (!result.ok || result.statusCode >= 500 || result.failed > 0) return 'danger'
  if (result.statusCode >= 400) return 'warning'
  return 'ok'
})

type ResponseBodyView = { kind: 'json'; value: unknown } | { kind: 'text'; text: string }

const responseBody = computed<ResponseBodyView>(() => {
  const body = apiTesting.lastResult?.body || apiTesting.lastResult?.error || ''
  if (apiTesting.lastResult?.streaming) return { kind: 'text', text: body || 'SSE 连接已建立，暂未收到事件' }
  if (!body) return { kind: 'text', text: '没有响应体' }
  try {
    return { kind: 'json', value: JSON.parse(body) }
  } catch {
    return { kind: 'text', text: body }
  }
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
        </div>
      </header>

      <div class="api-response-url">{{ apiTesting.lastResult.requestUrl }}</div>

      <section v-if="apiTesting.responseTab === 'body'" class="api-response-section">
        <div class="api-section-title">
          <span>响应体</span>
          <AriButton size="sm" variant="secondary" @click="apiTesting.copyResponseBody()">
            <Copy :size="14" />
            复制
          </AriButton>
        </div>
        <ApiJsonViewer v-if="responseBody.kind === 'json'" class="api-json-viewer" :value="responseBody.value" root />
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

      <footer class="api-response-stats">
        <div>
          <Clock3 :size="15" />
          <span>{{ apiTesting.lastResult.durationMs }} ms</span>
        </div>
        <div>
          <FileText :size="15" />
          <span>{{ formatBytes(apiTesting.lastResult.bodySize) }}</span>
        </div>
        <div>
          <ListChecks :size="15" />
          <span>{{ apiTesting.lastResult.passed }}/{{ apiTesting.lastResult.assertionResults.length }}</span>
        </div>
      </footer>
    </template>

    <div v-else-if="apiTesting.isRunning" class="api-empty-panel">
      <Clock3 :size="22" />
      <span>{{ apiTesting.isStopping ? '正在停止请求' : '正在接收响应' }}</span>
    </div>

    <div v-else class="api-empty-panel">
      <FileText :size="22" />
      <span>发送请求后查看响应</span>
    </div>
  </aside>
</template>
