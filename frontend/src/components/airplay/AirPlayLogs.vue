<script setup lang="ts">
import { RefreshCw } from '@lucide/vue'
import { ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { getAirPlayLogs } from '../../services/airplayApi'

const logs = ref('')
const loading = ref(false)
const error = ref('')

async function refresh() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    logs.value = await getAirPlayLogs()
  } catch (cause) {
    error.value = `读取日志失败：${String(cause)}`
  } finally {
    loading.value = false
  }
}

function onToggle(event: Event) {
  if ((event.target as HTMLDetailsElement).open) void refresh()
}
</script>

<template>
  <details class="airplay-logs" @toggle="onToggle">
    <summary>接收日志</summary>
    <div class="airplay-log-toolbar">
      <span>最近 4 KB</span>
      <AriButton size="sm" variant="ghost" :disabled="loading" @click="refresh">
        <RefreshCw :size="14" />{{ loading ? '读取中' : '刷新日志' }}
      </AriButton>
    </div>
    <p v-if="error" class="airplay-log-error" role="alert">{{ error }}</p>
    <pre v-else>{{ loading && !logs ? '正在读取日志…' : logs || '暂无日志，启动投屏后可查看。' }}</pre>
  </details>
</template>

<style scoped>
.airplay-logs { border-top: 1px solid var(--border); padding-top: 18px; }
summary { font-size: 13px; font-weight: 600; cursor: pointer; width: fit-content; }
.airplay-log-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 0 6px; color: var(--muted-foreground); font-size: 12px; }
pre { margin: 0; padding: 16px; max-height: 260px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; font-family: var(--font-mono); font-size: 12px; line-height: 1.7; background: var(--surface-muted); border: 1px solid var(--border); border-radius: var(--radius-sm); user-select: text; }
.airplay-log-error { font-size: 13px; color: var(--danger); }
</style>
