<script setup lang="ts">
import { CheckCircle2, Code2, FileJson2, Plus, Send, Square, Trash2 } from '@lucide/vue'
import AriButton from '../ui/AriButton.vue'
import { useAPITestingStore, type APIEditorTab } from '../../stores/apiTesting'

const apiTesting = useAPITestingStore()

const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const bodyTypes = [
  { value: 'none', label: '无请求体' },
  { value: 'json', label: 'JSON' },
  { value: 'text', label: '文本' },
  { value: 'form', label: 'Form' },
]
const tabs: Array<{ id: APIEditorTab; label: string }> = [
  { id: 'params', label: 'Params' },
  { id: 'body', label: 'Body' },
  { id: 'headers', label: 'Headers' },
  { id: 'assertions', label: 'Assert' },
]
const paramTypes = [
  { value: 'query', label: 'Query' },
  { value: 'path', label: 'Path' },
]
const assertionKinds = [
  { value: 'status', label: '状态码' },
  { value: 'header', label: '响应头' },
  { value: 'body', label: '响应体' },
  { value: 'json', label: 'JSON Path' },
  { value: 'response_time', label: '响应时间' },
]
const assertionOperators = [
  { value: 'equals', label: '等于' },
  { value: 'not_equals', label: '不等于' },
  { value: 'contains', label: '包含' },
  { value: 'exists', label: '存在' },
  { value: 'less_than', label: '小于' },
  { value: 'greater_than', label: '大于' },
]
</script>

<template>
  <section class="api-request-editor" aria-label="API 请求编辑">
    <template v-if="apiTesting.selectedRequest">
      <div class="api-url-row">
        <select class="api-method-select" :value="apiTesting.selectedRequest.method" @change="apiTesting.updateRequest({ method: ($event.target as HTMLSelectElement).value })">
          <option v-for="method in methods" :key="method" :value="method">{{ method }}</option>
        </select>
        <input
          class="api-input api-url-input"
          :value="apiTesting.selectedRequest.url"
          placeholder="https://api.example.com/v1/resource"
          @input="apiTesting.updateRequest({ url: ($event.target as HTMLInputElement).value })"
        />
        <AriButton v-if="apiTesting.isRunning" size="md" variant="secondary" :disabled="apiTesting.isStopping" @click="apiTesting.stopSelectedRequest()">
          <Square :size="15" />
          {{ apiTesting.isStopping ? '停止中' : '停止' }}
        </AriButton>
        <AriButton v-else size="md" variant="primary" @click="apiTesting.runSelectedRequest()">
          <Send :size="15" />
          发送
        </AriButton>
      </div>

      <nav class="api-editor-tabs" aria-label="请求编辑区域">
        <button v-for="tab in tabs" :key="tab.id" :class="{ 'is-active': apiTesting.editorTab === tab.id }" @click="apiTesting.editorTab = tab.id">
          {{ tab.label }}
        </button>
      </nav>

      <section v-if="apiTesting.editorTab === 'params'" class="api-editor-panel" aria-label="请求参数">
        <div class="api-panel-heading">
          <span>Query / Path</span>
          <AriButton size="sm" variant="secondary" @click="apiTesting.addParam()">
            <Plus :size="14" />
            添加参数
          </AriButton>
        </div>
        <div class="api-table">
          <div class="api-table-head is-param-grid">
            <span>启用</span>
            <span>类型</span>
            <span>名称</span>
            <span>值</span>
            <span></span>
          </div>
          <div v-for="(param, index) in apiTesting.selectedRequest.params" :key="param.id" class="api-table-row is-param-grid">
            <label class="api-checkbox">
              <input type="checkbox" :checked="param.enabled" @change="apiTesting.updateParam(index, { enabled: ($event.target as HTMLInputElement).checked })" />
            </label>
            <select class="api-input" :value="param.type" @change="apiTesting.updateParam(index, { type: ($event.target as HTMLSelectElement).value })">
              <option v-for="type in paramTypes" :key="type.value" :value="type.value">{{ type.label }}</option>
            </select>
            <input class="api-input" :value="param.name" placeholder="page / id" @input="apiTesting.updateParam(index, { name: ($event.target as HTMLInputElement).value })" />
            <input class="api-input" :value="param.value" placeholder="1 / {{userId}}" @input="apiTesting.updateParam(index, { value: ($event.target as HTMLInputElement).value })" />
            <AriButton size="icon" variant="ghost" aria-label="删除参数" @click="apiTesting.removeParam(index)">
              <Trash2 :size="14" />
            </AriButton>
          </div>
          <div v-if="!apiTesting.selectedRequest.params.length" class="api-empty-row">暂无参数</div>
        </div>
      </section>

      <section v-else-if="apiTesting.editorTab === 'headers'" class="api-editor-panel" aria-label="请求头">
        <div class="api-panel-heading">
          <span>Headers</span>
          <AriButton size="sm" variant="secondary" @click="apiTesting.addHeader()">
            <Plus :size="14" />
            添加请求头
          </AriButton>
        </div>
        <div class="api-table">
          <div class="api-table-head is-header-grid">
            <span>启用</span>
            <span>名称</span>
            <span>值</span>
            <span></span>
          </div>
          <div v-for="(header, index) in apiTesting.selectedRequest.headers" :key="header.id" class="api-table-row is-header-grid">
            <label class="api-checkbox">
              <input type="checkbox" :checked="header.enabled" @change="apiTesting.updateHeader(index, { enabled: ($event.target as HTMLInputElement).checked })" />
            </label>
            <input class="api-input" :value="header.name" placeholder="Authorization" @input="apiTesting.updateHeader(index, { name: ($event.target as HTMLInputElement).value })" />
            <input class="api-input" :value="header.value" placeholder="Bearer {{token}}" @input="apiTesting.updateHeader(index, { value: ($event.target as HTMLInputElement).value })" />
            <AriButton size="icon" variant="ghost" aria-label="删除请求头" @click="apiTesting.removeHeader(index)">
              <Trash2 :size="14" />
            </AriButton>
          </div>
          <div v-if="!apiTesting.selectedRequest.headers.length" class="api-empty-row">暂无请求头</div>
        </div>
      </section>

      <section v-else-if="apiTesting.editorTab === 'body'" class="api-editor-panel" aria-label="请求体">
        <div class="api-body-toolbar">
          <label class="api-field">
            <span>类型</span>
            <select class="api-input" :value="apiTesting.selectedRequest.bodyType" @change="apiTesting.updateRequest({ bodyType: ($event.target as HTMLSelectElement).value })">
              <option v-for="type in bodyTypes" :key="type.value" :value="type.value">{{ type.label }}</option>
            </select>
          </label>
          <span class="system-pill">
            <FileJson2 :size="13" />
            {{ apiTesting.selectedRequest.body.length }} 字符
          </span>
        </div>
        <textarea
          class="api-code-textarea"
          spellcheck="false"
          :disabled="apiTesting.selectedRequest.bodyType === 'none'"
          :value="apiTesting.selectedRequest.body"
          placeholder="{&#10;  &quot;name&quot;: &quot;Ariadne&quot;&#10;}"
          @input="apiTesting.updateRequest({ body: ($event.target as HTMLTextAreaElement).value })"
        />
      </section>

      <section v-else class="api-editor-panel" aria-label="断言">
        <div class="api-panel-heading">
          <span>Assertions</span>
          <AriButton size="sm" variant="secondary" @click="apiTesting.addAssertion()">
            <CheckCircle2 :size="14" />
            添加断言
          </AriButton>
        </div>
        <div class="api-table">
          <div class="api-table-head is-assertion-grid">
            <span>启用</span>
            <span>类型</span>
            <span>目标</span>
            <span>条件</span>
            <span>期望值</span>
            <span></span>
          </div>
          <div v-for="(assertion, index) in apiTesting.selectedRequest.assertions" :key="assertion.id" class="api-table-row is-assertion-grid">
            <label class="api-checkbox">
              <input type="checkbox" :checked="assertion.enabled" @change="apiTesting.updateAssertion(index, { enabled: ($event.target as HTMLInputElement).checked })" />
            </label>
            <select class="api-input" :value="assertion.kind" @change="apiTesting.updateAssertion(index, { kind: ($event.target as HTMLSelectElement).value })">
              <option v-for="kind in assertionKinds" :key="kind.value" :value="kind.value">{{ kind.label }}</option>
            </select>
            <input class="api-input" :value="assertion.target" placeholder="data.id / X-Request-Id" @input="apiTesting.updateAssertion(index, { target: ($event.target as HTMLInputElement).value })" />
            <select class="api-input" :value="assertion.operator" @change="apiTesting.updateAssertion(index, { operator: ($event.target as HTMLSelectElement).value })">
              <option v-for="operator in assertionOperators" :key="operator.value" :value="operator.value">{{ operator.label }}</option>
            </select>
            <input class="api-input" :value="assertion.expected" placeholder="200" @input="apiTesting.updateAssertion(index, { expected: ($event.target as HTMLInputElement).value })" />
            <AriButton size="icon" variant="ghost" aria-label="删除断言" @click="apiTesting.removeAssertion(index)">
              <Trash2 :size="14" />
            </AriButton>
          </div>
          <div v-if="!apiTesting.selectedRequest.assertions.length" class="api-empty-row">暂无断言</div>
        </div>
      </section>
    </template>

    <div v-else class="api-empty-panel">
      <Code2 :size="22" />
      <span>选择请求</span>
    </div>
  </section>
</template>
