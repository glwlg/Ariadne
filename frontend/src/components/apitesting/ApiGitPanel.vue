<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Download, GitBranch, RefreshCw, Upload, X } from '@lucide/vue'
import AriButton from '../ui/AriButton.vue'
import { useAPITestingStore } from '../../stores/apiTesting'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const apiTesting = useAPITestingStore()
const repoPath = ref('')
const remote = ref('')
const message = ref('')

const statusText = computed(() => {
  const status = apiTesting.gitStatus
  if (!status) return apiTesting.draftCollection?.git?.path ? '未读取状态' : '未绑定'
  return status.error || status.message
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    repoPath.value = apiTesting.draftCollection?.git?.path || ''
    remote.value = apiTesting.draftCollection?.git?.remote || ''
    message.value = `Update ${apiTesting.draftCollection?.name || 'API collection'}`
    if (repoPath.value) void apiTesting.refreshGitStatus()
  },
)

function configure() {
  void apiTesting.configureGit(repoPath.value, remote.value)
}

function refresh() {
  void apiTesting.refreshGitStatus()
}

function pull() {
  void apiTesting.pullGit()
}

function commitPush() {
  void apiTesting.commitPushGit(message.value)
}
</script>

<template>
  <div v-if="open" class="api-modal-layer" role="dialog" aria-modal="true" aria-label="Git 同步">
    <div class="api-modal-backdrop" @click="emit('close')" />
    <section class="api-git-panel">
      <header>
        <div>
          <GitBranch :size="16" />
          <strong>Git 同步</strong>
        </div>
        <button type="button" aria-label="关闭" @click="emit('close')">
          <X :size="16" />
        </button>
      </header>

      <div class="api-git-body">
        <label class="api-field">
          <span>Git 目录</span>
          <input v-model="repoPath" class="api-input" placeholder="D:\api\opscore" />
        </label>
        <label class="api-field">
          <span>远端地址</span>
          <input v-model="remote" class="api-input" placeholder="https://example.com/team/opscore-api.git" />
        </label>
        <div class="api-git-actions">
          <AriButton size="sm" variant="primary" :disabled="apiTesting.isGitSyncing || !repoPath.trim()" @click="configure">
            <GitBranch :size="14" />
            绑定
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="apiTesting.isGitSyncing || !apiTesting.draftCollection?.git?.path" @click="refresh">
            <RefreshCw :size="14" />
            状态
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="apiTesting.isGitSyncing || !apiTesting.draftCollection?.git?.path" @click="pull">
            <Download :size="14" />
            拉取
          </AriButton>
        </div>

        <section class="api-git-status" :class="{ 'is-danger': apiTesting.gitStatus && !apiTesting.gitStatus.ok }">
          <div>
            <span>{{ statusText }}</span>
            <small v-if="apiTesting.gitStatus?.branch">{{ apiTesting.gitStatus.branch }}</small>
          </div>
          <div v-if="apiTesting.gitStatus?.path" class="api-git-path">{{ apiTesting.gitStatus.path }}</div>
          <div v-if="apiTesting.gitStatus?.files?.length" class="api-git-files">
            <span v-for="file in apiTesting.gitStatus.files.slice(0, 6)" :key="file">{{ file }}</span>
          </div>
        </section>

        <label class="api-field">
          <span>提交信息</span>
          <input v-model="message" class="api-input" placeholder="Update API collection" />
        </label>
      </div>

      <footer>
        <AriButton size="sm" variant="ghost" @click="emit('close')">关闭</AriButton>
        <AriButton size="sm" variant="primary" :disabled="apiTesting.isGitSyncing || !apiTesting.draftCollection?.git?.path" @click="commitPush">
          <Upload :size="14" />
          提交并推送
        </AriButton>
      </footer>
    </section>
  </div>
</template>
