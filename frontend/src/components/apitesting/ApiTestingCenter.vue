<script setup lang="ts">
import { Globe2, MoreHorizontal, Send, TestTube2 } from '@lucide/vue'
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
import ApiContextMenu from './ApiContextMenu.vue'

const appShell = useAppShellStore()
const apiTesting = useAPITestingStore()
const gitPanelOpen = ref(false)
const commandMenu = ref({ open: false, x: 0, y: 0 })
const commandItems = computed(() => [
  { id: 'environment', label: '环境管理' },
  { id: 'git', label: 'Git 同步' },
  { id: 'save', label: apiTesting.isSaving ? '保存中' : '保存集合', disabled: !apiTesting.draftCollection || apiTesting.isSaving },
  { id: 'launcher', label: '返回启动器' },
])

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

function openCommandMenu(event: MouseEvent) {
  commandMenu.value = {
    open: true,
    x: Math.min(event.clientX, window.innerWidth - 210),
    y: Math.min(event.clientY, window.innerHeight - 180),
  }
}

function selectCommand(action: string) {
  commandMenu.value.open = false
  if (action === 'environment') apiTesting.openEnvironmentPanel()
  if (action === 'git') gitPanelOpen.value = true
  if (action === 'save') void apiTesting.saveCollection()
  if (action === 'launcher') appShell.openLauncher()
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell api-testing-shell" aria-label="API 测试">
        <header class="api-workbench-header">
          <div class="api-brand-lockup">
            <div class="brand-mark" aria-hidden="true"><Send :size="18" /></div>
            <div class="brand-copy">
              <span>API 测试</span>
              <small>{{ apiTesting.draftCollection?.name || '请求工作台' }}</small>
            </div>
          </div>

          <div class="api-header-controls">
            <label class="api-env-selector">
              <Globe2 :size="14" />
              <select :value="apiTesting.selectedEnvironmentId" @change="apiTesting.selectEnvironment(($event.target as HTMLSelectElement).value)">
                <option v-for="environment in apiTesting.draftCollection?.environments ?? []" :key="environment.id" :value="environment.id">
                  {{ environment.name }}
                </option>
              </select>
            </label>
            <AriButton size="sm" variant="ghost" aria-label="更多命令" @click="openCommandMenu">
              <MoreHorizontal :size="16" />
              更多
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
          <span v-else-if="apiTesting.isSaving">正在保存</span>
          <span v-else-if="apiTesting.isDirty">有未保存修改</span>
          <span v-else>已保存</span>
          <span v-if="apiTesting.status?.path" class="api-status-file" :title="apiTesting.status.path">{{ apiTesting.status.path.split(/[\\/]/).pop() }}</span>
        </footer>
        <ApiContextMenu
          :open="commandMenu.open"
          :x="commandMenu.x"
          :y="commandMenu.y"
          :items="commandItems"
          @close="commandMenu.open = false"
          @select="selectCommand"
        />
        <ApiEnvironmentPanel />
        <ApiGitPanel :open="gitPanelOpen" @close="gitPanelOpen = false" />
      </section>
    </div>
  </main>
</template>
