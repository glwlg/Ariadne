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

type ConnectionState = 'stopped' | 'connecting' | 'connected' | 'disconnected'

export const useNetworkMonitorStore = defineStore('network-monitor', () => {
  const snapshot = ref<NetworkTrafficSnapshot | null>(null)
  const processSnapshot = ref<ProcessNetworkSnapshot | null>(null)
  const feedback = ref('')
  const isLoading = ref(false)
  const trafficConnection = ref<ConnectionState>('stopped')
  const processConnection = ref<ConnectionState>('stopped')
  const trafficError = ref('')
  const processError = ref('')
  const isPolling = computed(() => trafficConnection.value === 'connecting' || trafficConnection.value === 'connected')
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
    try {
      processSnapshot.value = await getProcessNetworkSnapshot()
    } catch {
      showFeedback('进程网络刷新失败')
    }
  }

  function startPolling() {
    if (stopTrafficStream !== null) return
    trafficConnection.value = 'connecting'
    trafficError.value = ''
    try {
      stopTrafficStream = connectNetworkTelemetry('traffic', (frame) => {
        if (!frame.traffic) return
        snapshot.value = normalizeSnapshot(frame.traffic)
        if (snapshot.value.lastError) showFeedback(snapshot.value.lastError)
      }, trafficDisconnected, undefined, () => { trafficConnection.value = 'connected' })
    } catch (error) {
      trafficDisconnected(error instanceof Error ? error.message : String(error))
    }
  }

  function trafficDisconnected(message: string) {
    stopTrafficStream = null
    trafficConnection.value = 'disconnected'
    trafficError.value = message
  }

  function stopPolling() {
    if (stopTrafficStream !== null) {
      stopTrafficStream()
      stopTrafficStream = null
    }
    trafficConnection.value = 'stopped'
    trafficError.value = ''
  }

  function startProcessPolling() {
    if (stopProcessStream !== null) return
    processConnection.value = 'connecting'
    processError.value = ''
    try {
      stopProcessStream = connectNetworkTelemetry('processes', (frame) => {
        if (!frame.processes) return
        processSnapshot.value = normalizeProcessSnapshot(frame.processes)
        if (processSnapshot.value.lastError) showFeedback(processSnapshot.value.lastError)
      }, processDisconnected, undefined, () => { processConnection.value = 'connected' })
    } catch (error) {
      processDisconnected(error instanceof Error ? error.message : String(error))
    }
  }

  function processDisconnected(message: string) {
    stopProcessStream = null
    processConnection.value = 'disconnected'
    processError.value = message
  }

  function stopProcessPolling() {
    stopProcessStream?.()
    stopProcessStream = null
    processConnection.value = 'stopped'
    processError.value = ''
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
    trafficConnection,
    processConnection,
    trafficError,
    processError,
    hasError,
    refresh,
    refreshProcesses,
    startPolling,
    stopPolling,
    startProcessPolling,
    stopProcessPolling,
  }
})
