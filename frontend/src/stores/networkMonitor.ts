import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getNetworkTrafficSnapshot, getProcessNetworkSnapshot } from '../services/networkMonitorApi'
import type { NetworkTrafficSnapshot, ProcessNetworkSnapshot } from '../types/ariadne'

export const useNetworkMonitorStore = defineStore('network-monitor', () => {
  const snapshot = ref<NetworkTrafficSnapshot | null>(null)
  const processSnapshot = ref<ProcessNetworkSnapshot | null>(null)
  const feedback = ref('')
  const isLoading = ref(false)
  const isPolling = ref(false)
  let timer: number | null = null
  let processTimer: number | null = null

  const adapters = computed(() => snapshot.value?.adapters ?? [])
  const primaryAdapter = computed(() => adapters.value.find((item) => item.operational) ?? adapters.value[0] ?? null)
  const hasError = computed(() => Boolean(snapshot.value?.lastError))

  async function refresh() {
    isLoading.value = true
    try {
      snapshot.value = await getNetworkTrafficSnapshot()
      if (snapshot.value.lastError) {
        showFeedback(snapshot.value.lastError)
      }
    } catch {
      showFeedback('网络监控刷新失败')
    } finally {
      isLoading.value = false
    }
  }

  async function refreshProcesses() {
    processSnapshot.value = await getProcessNetworkSnapshot()
  }

  function startPolling() {
    if (timer !== null) return
    isPolling.value = true
    void refresh()
    timer = window.setInterval(() => {
      void refresh()
    }, 1000)
  }

  function stopPolling() {
    if (timer !== null) {
      window.clearInterval(timer)
      timer = null
    }
    isPolling.value = false
  }

  function startProcessPolling() {
    if (processTimer !== null) return
    void refreshProcesses()
    processTimer = window.setInterval(() => void refreshProcesses(), 1000)
  }

  function stopProcessPolling() {
    if (processTimer === null) return
    window.clearInterval(processTimer)
    processTimer = null
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 1800)
  }

  return {
    snapshot,
    processSnapshot,
    adapters,
    primaryAdapter,
    feedback,
    isLoading,
    isPolling,
    hasError,
    refresh,
    refreshProcesses,
    startPolling,
    stopPolling,
    startProcessPolling,
    stopProcessPolling,
  }
})
