import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Clipboard } from '@wailsio/runtime'
import { executeAirPlay, getAirPlayStatus } from '../services/airplayApi'
import type { ActionResult } from '../types/ariadne'
import type { AirPlayCommand, AirPlayStatus } from '../types/airplay'

export function useAirPlay() {
  const status = ref<AirPlayStatus | null>(null)
  const error = ref('')
  const pending = ref<AirPlayCommand | ''>('')
  const refreshing = ref(false)
  const feedback = ref<ActionResult | null>(null)
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined

  const stateLabel = computed(() => {
    if (error.value) return '状态读取失败'
    const current = status.value
    if (!current) return '正在读取状态'
    if (!current.ready) return '投屏不可用'
    if (!current.running) return '未启动'
    if (current.phase === 'mirroring') return '正在投屏'
    if (current.phase === 'disconnected') return '连接已断开'
    return '等待 iPhone 连接'
  })

  async function refresh() {
    if (refreshing.value || disposed) return
    refreshing.value = true
    try {
      status.value = await getAirPlayStatus()
      error.value = ''
    } catch (cause) {
      error.value = `读取投屏状态失败：${String(cause)}`
    } finally {
      refreshing.value = false
    }
  }

  async function run(command: AirPlayCommand) {
    if (pending.value) return
    pending.value = command
    feedback.value = null
    try {
      feedback.value = await executeAirPlay(command)
    } catch (cause) {
      feedback.value = { ok: false, message: `操作失败：${String(cause)}` }
    } finally {
      pending.value = ''
      await refresh()
    }
  }

  async function copy(text: string, label: string) {
    try {
      await Clipboard.SetText(text)
      feedback.value = { ok: true, message: `已复制${label}` }
    } catch (cause) {
      feedback.value = { ok: false, message: `复制失败：${String(cause)}` }
    }
  }

  async function poll() {
    if (disposed) return
    if (!document.hidden && !pending.value) await refresh()
    if (!disposed) timer = setTimeout(poll, 1500)
  }

  onMounted(() => { void poll() })
  onBeforeUnmount(() => {
    disposed = true
    clearTimeout(timer)
  })

  return { status, error, pending, refreshing, feedback, stateLabel, refresh, run, copy }
}
