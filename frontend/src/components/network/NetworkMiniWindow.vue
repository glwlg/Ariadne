<script setup lang="ts">
import { ArrowDown, ArrowUp } from '@lucide/vue'
import { computed, onMounted, onUnmounted } from 'vue'
import { useAppShellStore } from '../../stores/appShell'
import { useNetworkMonitorStore } from '../../stores/networkMonitor'

const appShell = useAppShellStore()
const network = useNetworkMonitorStore()

const primaryName = computed(() => network.primaryAdapter?.alias || network.primaryAdapter?.name || '未检测到网卡')
const downloadRate = computed(() => network.trafficConnection === 'connected' ? formatRate(network.snapshot?.downloadBytesPerSecond ?? 0) : '—')
const uploadRate = computed(() => network.trafficConnection === 'connected' ? formatRate(network.snapshot?.uploadBytesPerSecond ?? 0) : '—')
const windowTitle = computed(() => network.trafficError || `${primaryName.value} 上传 ${uploadRate.value} / 下载 ${downloadRate.value}`)

onMounted(() => {
  network.startPolling()
})

onUnmounted(() => {
  network.stopPolling()
})

function openCenter() {
  appShell.openNetworkMonitor()
}

function formatRate(value: number) {
  return `${formatBytes(value)}/s`
}

function formatBytes(value: number) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let next = Math.max(0, Number(value) || 0)
  let index = 0
  while (next >= 1024 && index < units.length - 1) {
    next /= 1024
    index++
  }
  if (index === 0) return `${Math.round(next)} ${units[index]}`
  return `${next.toFixed(next >= 100 ? 0 : 1)} ${units[index]}`
}
</script>

<template>
  <main class="network-mini-root" aria-label="任务栏网速监控" :title="windowTitle" @dblclick="openCenter">
    <section class="network-mini-shell">
      <button v-if="network.trafficConnection === 'disconnected'" type="button" class="network-mini-reconnect" @click.stop="network.startPolling()" @dblclick.stop>
        重新连接
      </button>
      <div v-else class="network-mini-row is-upload">
        <ArrowUp :size="11" />
        <strong>{{ uploadRate }}</strong>
      </div>
      <div class="network-mini-row is-download">
        <ArrowDown :size="11" />
        <strong>{{ downloadRate }}</strong>
      </div>
    </section>
  </main>
</template>

<style scoped>
.network-mini-reconnect { border: 0; padding: 0; background: transparent; color: inherit; font: inherit; font-size: 10px; cursor: pointer; }
.network-mini-reconnect:focus-visible { outline: 1px solid var(--ring); }
</style>
