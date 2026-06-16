<script setup lang="ts">
import { Activity, ArrowDown, ArrowLeft, ArrowUp, Gauge, Minimize2, RefreshCw, Router, Wifi } from '@lucide/vue'
import { computed, onMounted, onUnmounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useNetworkMonitorStore } from '../../stores/networkMonitor'

const appShell = useAppShellStore()
const network = useNetworkMonitorStore()

const snapshotTime = computed(() => {
  const timestamp = network.snapshot?.timestampUnix
  if (!timestamp) return '--:--:--'
  return new Date(timestamp * 1000).toLocaleTimeString('zh-CN', { hour12: false })
})

const peakRate = computed(() => {
  const upload = network.snapshot?.uploadBytesPerSecond ?? 0
  const download = network.snapshot?.downloadBytesPerSecond ?? 0
  return Math.max(upload, download, 1024)
})

onMounted(() => {
  network.startPolling()
})

onUnmounted(() => {
  network.stopPolling()
})

function rateWidth(value: number) {
  return `${Math.max(3, Math.min(100, (value / peakRate.value) * 100))}%`
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

function formatLinkSpeed(value: number) {
  if (!value) return '未知'
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(value >= 10_000_000_000 ? 0 : 1)} Gbps`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(0)} Mbps`
  return `${(value / 1000).toFixed(0)} Kbps`
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell network-monitor-shell" aria-label="网络监控">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Wifi :size="18" />
          </div>
          <div class="brand-copy">
            <span>网络监控</span>
            <small>实时速率 · 网卡流量 · 本机计数</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="{ 'is-on': network.isPolling, 'is-danger': network.hasError }">
              <Activity :size="13" />
              {{ network.hasError ? '读取异常' : network.isPolling ? '刷新中' : '已暂停' }}
            </span>
            <span class="system-pill">
              <Router :size="13" />
              {{ network.snapshot?.activeAdapterCount ?? 0 }} / {{ network.snapshot?.adapterCount ?? 0 }} 网卡
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openNetworkMini()">
              <Minimize2 :size="14" />
              小窗
            </AriButton>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="tool-toolbar network-toolbar">
          <div class="network-toolbar-meta">
            <span>最后刷新 {{ snapshotTime }}</span>
            <strong>{{ network.primaryAdapter?.alias || network.primaryAdapter?.name || '未检测到活动网卡' }}</strong>
          </div>
          <AriButton size="sm" variant="secondary" :disabled="network.isLoading" @click="network.refresh()">
            <RefreshCw :size="14" />
            刷新
          </AriButton>
        </div>

        <div class="network-workspace">
          <section class="network-speed-board" aria-label="实时速率">
            <div class="network-speed-card is-download">
              <span class="network-speed-icon">
                <ArrowDown :size="18" />
              </span>
              <div>
                <span>下载</span>
                <strong>{{ formatRate(network.snapshot?.downloadBytesPerSecond ?? 0) }}</strong>
              </div>
              <div class="network-meter">
                <span :style="{ width: rateWidth(network.snapshot?.downloadBytesPerSecond ?? 0) }" />
              </div>
            </div>

            <div class="network-speed-card is-upload">
              <span class="network-speed-icon">
                <ArrowUp :size="18" />
              </span>
              <div>
                <span>上传</span>
                <strong>{{ formatRate(network.snapshot?.uploadBytesPerSecond ?? 0) }}</strong>
              </div>
              <div class="network-meter">
                <span :style="{ width: rateWidth(network.snapshot?.uploadBytesPerSecond ?? 0) }" />
              </div>
            </div>

            <div class="network-total-grid">
              <div>
                <span>累计下载</span>
                <strong>{{ formatBytes(network.snapshot?.bytesReceived ?? 0) }}</strong>
              </div>
              <div>
                <span>累计上传</span>
                <strong>{{ formatBytes(network.snapshot?.bytesSent ?? 0) }}</strong>
              </div>
              <div>
                <span>接收链路</span>
                <strong>{{ formatLinkSpeed(network.primaryAdapter?.receiveLinkBitsPerSec ?? 0) }}</strong>
              </div>
              <div>
                <span>发送链路</span>
                <strong>{{ formatLinkSpeed(network.primaryAdapter?.transmitLinkBitsPerSec ?? 0) }}</strong>
              </div>
            </div>
          </section>

          <section class="network-adapter-panel" aria-label="网卡列表">
            <div class="network-section-title">
              <span>
                <Gauge :size="14" />
                网卡
              </span>
              <small>{{ network.feedback || network.snapshot?.lastError || '' }}</small>
            </div>

            <div class="network-adapter-list">
              <div
                v-for="adapter in network.adapters"
                :key="adapter.interfaceIndex"
                class="network-adapter-row"
                :class="{ 'is-on': adapter.operational }"
              >
                <span class="network-adapter-icon">
                  <Router :size="15" />
                </span>
                <div class="network-adapter-main">
                  <span class="network-adapter-name">{{ adapter.alias || adapter.name }}</span>
                  <small>{{ adapter.description || adapter.name }}</small>
                </div>
                <div class="network-adapter-rate">
                  <span>
                    <ArrowDown :size="12" />
                    {{ formatRate(adapter.downloadBytesPerSecond) }}
                  </span>
                  <span>
                    <ArrowUp :size="12" />
                    {{ formatRate(adapter.uploadBytesPerSecond) }}
                  </span>
                </div>
              </div>
              <div v-if="!network.adapters.length" class="empty-state">
                <span>未检测到活动网卡</span>
              </div>
            </div>
          </section>
        </div>

        <footer class="status-strip">
          <span>
            <Wifi :size="14" />
            本机计数
          </span>
          <span>
            <Activity :size="14" />
            1s 刷新
          </span>
          <span v-if="network.feedback" class="inline-feedback">{{ network.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
