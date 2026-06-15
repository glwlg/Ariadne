<script setup lang="ts">
import { ArrowDown, ArrowUp } from '@lucide/vue'
import { computed, onMounted, onUnmounted } from 'vue'
import { useAppShellStore } from '../../stores/appShell'
import { useNetworkMonitorStore } from '../../stores/networkMonitor'

const appShell = useAppShellStore()
const network = useNetworkMonitorStore()

const primaryName = computed(() => network.primaryAdapter?.alias || network.primaryAdapter?.name || '未检测到网卡')
const downloadRate = computed(() => formatRate(network.snapshot?.downloadBytesPerSecond ?? 0))
const uploadRate = computed(() => formatRate(network.snapshot?.uploadBytesPerSecond ?? 0))
const windowTitle = computed(() => `${primaryName.value} 上传 ${uploadRate.value} / 下载 ${downloadRate.value}`)

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
      <div class="network-mini-row is-upload">
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
