<script setup lang="ts">
import { ArrowLeft, Database, GitBranch, Globe2, Save, Send, TestTube2 } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAPITestingStore } from '../../stores/apiTesting'
import { useAppShellStore } from '../../stores/appShell'
import ApiEnvironmentPanel from './ApiEnvironmentPanel.vue'
import ApiGitPanel from './ApiGitPanel.vue'
import ApiRequestEditor from './ApiRequestEditor.vue'
import ApiRequestList from './ApiRequestList.vue'
import ApiRequestTabs from './ApiRequestTabs.vue'
import ApiResponsePanel from './ApiResponsePanel.vue'

const appShell = useAppShellStore()
const apiTesting = useAPITestingStore()
const gitPanelOpen = ref(false)

const resultLabel = computed(() => {
  if (apiTesting.isRunning) return '请求中'
  if (!apiTesting.lastResult) return '待执行'
  if (!apiTesting.lastResult.ok) return '请求失败'
  return apiTesting.lastResult.failed > 0 ? '断言未通过' : '断言通过'
})

onMounted(() => {
  if (!apiTesting.status) {
    void apiTesting.load()
  }
  window.addEventListener('keydown', onWindowKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKeydown)
})

function onWindowKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    void apiTesting.saveCollection()
  }
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell api-testing-shell" aria-label="API 测试">
        <header class="api-workbench-header">
          <div class="api-brand-lockup">
            <div class="brand-mark" aria-hidden="true">
              <Send :size="18" />
            </div>
            <div class="brand-copy">
              <span>API 测试</span>
              <small>请求 · 环境 · 断言</small>
            </div>
          </div>

          <div class="api-header-controls">
            <span class="system-pill" :class="{ 'is-on': apiTesting.lastResult?.ok, 'is-danger': apiTesting.lastResult && apiTesting.lastResult.failed > 0 }">
              <TestTube2 :size="13" />
              {{ resultLabel }}
            </span>
            <span class="system-pill">
              <Database :size="13" />
              {{ apiTesting.requestCount }} 个请求
            </span>
            <label class="api-env-selector">
              <Globe2 :size="14" />
              <select :value="apiTesting.selectedEnvironmentId" @change="apiTesting.selectEnvironment(($event.target as HTMLSelectElement).value)">
                <option v-for="environment in apiTesting.draftCollection?.environments ?? []" :key="environment.id" :value="environment.id">
                  {{ environment.name }}
                </option>
              </select>
            </label>
            <AriButton size="sm" variant="secondary" @click="apiTesting.openEnvironmentPanel()">
              <Globe2 :size="14" />
              环境
            </AriButton>
            <AriButton size="sm" variant="secondary" :active="Boolean(apiTesting.draftCollection?.git?.path)" @click="gitPanelOpen = true">
              <GitBranch :size="14" />
              Git
            </AriButton>
            <AriButton size="sm" variant="secondary" :disabled="!apiTesting.draftCollection || apiTesting.isSaving" @click="apiTesting.saveCollection()">
              <Save :size="14" />
              {{ apiTesting.isSaving ? '保存中' : '保存' }}
            </AriButton>
            <AriButton size="sm" variant="ghost" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="api-testing-workspace">
          <ApiRequestList />
          <section class="api-main-workbench" aria-label="请求工作台">
            <ApiRequestTabs />
            <div class="api-editor-response-grid">
              <ApiRequestEditor />
              <ApiResponsePanel />
            </div>
          </section>
        </div>

        <footer class="status-strip">
          <span>
            <TestTube2 :size="14" />
            {{ apiTesting.enabledAssertionCount }} 条断言
          </span>
          <span v-if="apiTesting.feedback">{{ apiTesting.feedback }}</span>
          <span v-else-if="apiTesting.isDirty">有未保存修改</span>
          <span v-if="apiTesting.status?.path">{{ apiTesting.status.path }}</span>
        </footer>
        <ApiEnvironmentPanel />
        <ApiGitPanel :open="gitPanelOpen" @close="gitPanelOpen = false" />
      </section>
    </div>
  </main>
</template>
