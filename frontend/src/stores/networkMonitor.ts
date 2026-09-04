import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  getNetworkTrafficSnapshot,
  getProcessNetworkSnapshot,
  normalizeProcessSnapshot,
  normalizeSnapshot,
} from '../services/networkMonitorApi'
import { connectNetworkTelemetry } from '../services/networkTelemetry'
import type { NetworkTrafficSnapshot, ProcessNetworkSnapshot } from '../types/ariadne'

export const useNetworkMonitorStore = defineStore('network-monitor', () => {
  const snapshot = ref<NetworkTrafficSnapshot | null>(null)
  const processSnapshot = ref<ProcessNetworkSnapshot | null>(null)
  const feedback = ref('')
  const isLoading = ref(false)
  const isPolling = ref(false)
  let stopTrafficStream: (() => void) | null = null
  let stopProcessStream: (() => void) | null = null

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
    if (stopTrafficStream !== null) return
    isPolling.value = true
    stopTrafficStream = connectNetworkTelemetry('traffic', (frame) => {
      if (!frame.traffic) return
      snapshot.value = normalizeSnapshot(frame.traffic)
      if (snapshot.value.lastError) {
        showFeedback(snapshot.value.lastError)
      }
    }, (message) => {
      showFeedback(message)
      if (snapshot.value === null) {
        void refresh()
      }
    })
  }

  function stopPolling() {
    if (stopTrafficStream !== null) {
      stopTrafficStream()
      stopTrafficStream = null
    }
    isPolling.value = false
  }

  function startProcessPolling() {
    if (stopProcessStream !== null) return
    stopProcessStream = connectNetworkTelemetry('processes', (frame) => {
      if (!frame.processes) return
      processSnapshot.value = normalizeProcessSnapshot(frame.processes)
      if (processSnapshot.value.lastError) {
        showFeedback(processSnapshot.value.lastError)
      }
    }, (message) => {
      showFeedback(message)
      if (processSnapshot.value === null) {
        void refreshProcesses()
      }
    })
  }

  function stopProcessPolling() {
    if (stopProcessStream === null) return
    stopProcessStream()
    stopProcessStream = null
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
