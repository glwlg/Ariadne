<script setup lang="ts">
import { Activity, AppWindow, ArrowDown, ArrowUp, ClipboardCopy, RefreshCw, Search, Wifi } from '@lucide/vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useAppShellStore } from '../../stores/appShell'
import { useNetworkMonitorStore } from '../../stores/networkMonitor'
import { sortProcessTraffic, type ProcessNetworkSortKey, type SortDirection } from '../../lib/processNetworkSort'
import type { ProcessNetworkTraffic } from '../../types/ariadne'
import AriButton from '../ui/AriButton.vue'

const appShell = useAppShellStore()
const network = useNetworkMonitorStore()
const query = ref('')
const selectedPID = ref<number | null>(null)
const feedback = ref('')
const sortKey = ref<ProcessNetworkSortKey>('download')
const sortDirection = ref<SortDirection>('descending')

const processes = computed(() => network.processSnapshot?.processes ?? [])
const filteredProcesses = computed(() => {
  const needle = query.value.trim().toLowerCase()
  const matches = needle
    ? processes.value.filter((item) => item.name.toLowerCase().includes(needle) || String(item.pid).includes(needle))
    : processes.value
  return sortProcessTraffic(matches, sortKey.value, sortDirection.value)
})
const selectedProcess = computed(() => processes.value.find((item) => item.pid === selectedPID.value) ?? filteredProcesses.value[0] ?? null)
const totalRate = computed(() => Math.max(1, (network.processSnapshot?.downloadBytesPerSecond ?? 0) + (network.processSnapshot?.uploadBytesPerSecond ?? 0)))

watch(processes, (items) => {
  if (!items.length) {
    selectedPID.value = null
  } else if (!items.some((item) => item.pid === selectedPID.value)) {
    selectedPID.value = items[0].pid
  }
}, { immediate: true })

onMounted(() => network.startProcessPolling())
onUnmounted(() => network.stopProcessPolling())

function selectProcess(process: ProcessNetworkTraffic) {
  selectedPID.value = process.pid
}

function toggleSort(key: ProcessNetworkSortKey) {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'descending' ? 'ascending' : 'descending'
  } else {
    sortKey.value = key
    sortDirection.value = 'descending'
  }
}

function ariaSort(key: ProcessNetworkSortKey) {
  return sortKey.value === key ? sortDirection.value : 'none'
}

function rateShare(process: ProcessNetworkTraffic) {
  return Math.round(((process.downloadBytesPerSecond + process.uploadBytesPerSecond) / totalRate.value) * 100)
}

async function copySummary() {
  const lines = processes.value.map((item) => `${item.name} (PID ${item.pid})  ↓ ${formatRate(item.downloadBytesPerSecond)}  ↑ ${formatRate(item.uploadBytesPerSecond)}`)
  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    showFeedback('进程网络摘要已复制')
  } catch {
    showFeedback('复制失败')
  }
}

async function returnToMini() {
  await appShell.openNetworkMini()
  await appShell.closeCurrentWindow()
}

function showFeedback(message: string) {
  feedback.value = message
  window.setTimeout(() => {
    if (feedback.value === message) feedback.value = ''
  }, 1800)
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
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell process-network-shell" aria-label="进程网络">
        <header class="tool-header process-network-header">
          <div class="brand-mark" aria-hidden="true"><Wifi :size="18" /></div>
          <div class="brand-copy">
            <span>进程网络</span>
            <small>各进程实时 TCP / UDP 上下行 · 点击表头排序</small>
          </div>
        </header>

        <div class="tool-toolbar process-network-toolbar">
          <div class="process-network-metrics">
            <span class="system-pill is-on"><Activity :size="13" />刷新中</span>
            <span class="system-pill is-download"><ArrowDown :size="13" />下载 {{ formatRate(network.processSnapshot?.downloadBytesPerSecond ?? 0) }}</span>
            <span class="system-pill is-upload"><ArrowUp :size="13" />上传 {{ formatRate(network.processSnapshot?.uploadBytesPerSecond ?? 0) }}</span>
          </div>
          <label class="process-network-search">
            <Search :size="15" aria-hidden="true" />
            <input v-model="query" type="search" placeholder="筛选进程 / PID" aria-label="筛选进程或 PID" />
          </label>
          <div class="process-network-actions">
            <AriButton class="process-network-action" size="sm" variant="secondary" @click="network.refreshProcesses()"><RefreshCw :size="14" />刷新</AriButton>
            <AriButton class="process-network-action" size="sm" variant="secondary" :disabled="!processes.length" @click="copySummary"><ClipboardCopy :size="14" />复制摘要</AriButton>
            <AriButton class="process-network-action" size="sm" variant="secondary" @click="returnToMini">返回小窗</AriButton>
          </div>
        </div>

        <div v-if="network.processSnapshot?.lastError" class="process-network-warning" role="status">
          {{ network.processSnapshot.lastError }}
        </div>

        <div class="process-network-workspace">
          <section class="process-network-list" aria-label="进程流量列表">
            <div class="process-network-table-head" role="row">
              <span role="columnheader">进程</span><span role="columnheader">PID</span>
              <span role="columnheader" :aria-sort="ariaSort('download')">
                <button type="button" class="process-network-sort" :class="{ 'is-active': sortKey === 'download' }" @click="toggleSort('download')">
                  下载 <ArrowDown v-if="sortKey === 'download' && sortDirection === 'descending'" :size="13" aria-hidden="true" />
                  <ArrowUp v-else-if="sortKey === 'download'" :size="13" aria-hidden="true" />
                </button>
              </span>
              <span role="columnheader" :aria-sort="ariaSort('upload')">
                <button type="button" class="process-network-sort" :class="{ 'is-active': sortKey === 'upload' }" @click="toggleSort('upload')">
                  上传 <ArrowDown v-if="sortKey === 'upload' && sortDirection === 'descending'" :size="13" aria-hidden="true" />
                  <ArrowUp v-else-if="sortKey === 'upload'" :size="13" aria-hidden="true" />
                </button>
              </span>
              <span role="columnheader">占比</span>
            </div>
            <div class="process-network-table-body">
              <button
                v-for="process in filteredProcesses"
                :key="process.pid"
                type="button"
                class="process-network-row"
                :class="{ 'is-selected': selectedProcess?.pid === process.pid }"
                @click="selectProcess(process)"
              >
                <span class="process-network-name"><span class="process-network-icon"><img v-if="process.iconUrl" :src="process.iconUrl" alt="" /><AppWindow v-else :size="16" /></span><strong>{{ process.name }}</strong></span>
                <span class="process-network-pid">{{ process.pid }}</span>
                <span class="process-network-rate is-download">{{ formatRate(process.downloadBytesPerSecond) }}</span>
                <span class="process-network-rate is-upload">{{ formatRate(process.uploadBytesPerSecond) }}</span>
                <span class="process-network-share"><i><b :style="{ width: `${rateShare(process)}%` }" /></i>{{ rateShare(process) }}%</span>
              </button>
              <div v-if="!filteredProcesses.length" class="empty-state process-network-empty">
                <span>{{ query ? '没有匹配的进程' : '当前没有网络活动' }}</span>
              </div>
            </div>
          </section>

          <aside class="process-network-detail" aria-label="进程详情">
            <template v-if="selectedProcess">
              <div class="process-network-detail-head">
                <span class="process-network-detail-icon"><img v-if="selectedProcess.iconUrl" :src="selectedProcess.iconUrl" alt="" /><AppWindow v-else :size="20" /></span>
                <div><strong>{{ selectedProcess.name }}</strong><small>PID {{ selectedProcess.pid }}</small></div>
              </div>
              <div class="process-network-stat-grid">
                <div><span><ArrowDown :size="14" />下载</span><strong>{{ formatRate(selectedProcess.downloadBytesPerSecond) }}</strong></div>
                <div><span><ArrowUp :size="14" />上传</span><strong>{{ formatRate(selectedProcess.uploadBytesPerSecond) }}</strong></div>
                <div><span>本次接收</span><strong>{{ formatBytes(selectedProcess.bytesReceived) }}</strong></div>
                <div><span>本次发送</span><strong>{{ formatBytes(selectedProcess.bytesSent) }}</strong></div>
              </div>
              <div class="process-network-connections">
                <div class="network-section-title"><span>当前 TCP 连接</span><small>{{ selectedProcess.connections.length }} 条</small></div>
                <div class="process-network-connection-list">
                  <div v-for="connection in selectedProcess.connections" :key="`${connection.localAddress}-${connection.remoteAddress}`">
                    <span><strong>{{ connection.remoteAddress }}</strong><small>{{ connection.localAddress }}</small></span>
                    <span><b>{{ formatRate(connection.downloadBytesPerSecond) }}</b><small>↓ / ↑ {{ formatRate(connection.uploadBytesPerSecond) }}</small></span>
                  </div>
                </div>
              </div>
            </template>
            <div v-else class="empty-state"><span>选择一个进程查看连接明细</span></div>
          </aside>
        </div>

        <footer class="status-strip">
          <span>1s 刷新</span><span>TCP / UDP · IPv4 / IPv6</span><span>{{ network.processSnapshot?.processCount ?? 0 }} 个进程</span><span>{{ network.processSnapshot?.connectionCount ?? 0 }} 条连接</span>
          <span v-if="feedback" class="inline-feedback">{{ feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
