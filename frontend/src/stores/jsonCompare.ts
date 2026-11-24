import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import { compareJson, formatJson } from '../services/jsonCompareApi'
import type { JsonCompareResult } from '../types/fancun'

const sampleLeft = `{
  "name": "same",
  "meta": {
    "enabled": true,
    "drop": 1
  },
  "items": [1, 2]
}`

const sampleRight = `{
  "name": "same",
  "meta": {
    "enabled": true,
    "add": 2
  },
  "items": [1, 3, 4]
}`

const COMPARE_DEBOUNCE_MS = 150
const ARMED_TIMEOUT_MS = 5000

export const useJsonCompareStore = defineStore('json-compare', () => {
  const leftText = ref(sampleLeft)
  const rightText = ref(sampleRight)
  const sortKeys = ref(true)
  const result = ref<JsonCompareResult | null>(null)
  const feedback = ref('')
  const feedbackTone = ref<'success' | 'danger'>('success')
  const clearArmed = ref(false)
  const isComparing = ref(false)
  let compareSequence = 0
  let compareDebounceTimer: number | undefined
  let feedbackTimer: number | undefined
  let clearArmedTimer: number | undefined

  const differenceCount = computed(() => (result.value ? result.value.added + result.value.removed + result.value.changed : 0))
  const hasDifferences = computed(() => Boolean(result.value?.ok && differenceCount.value))
  const resultTone = computed(() => {
    if (!result.value) return 'idle'
    if (!result.value.ok) return 'error'
    return differenceCount.value ? 'changed' : 'same'
  })

  function clearDebounceTimer() {
    if (compareDebounceTimer !== undefined) {
      window.clearTimeout(compareDebounceTimer)
      compareDebounceTimer = undefined
    }
  }

  function scheduleCompare(delayMs = COMPARE_DEBOUNCE_MS) {
    clearDebounceTimer()
    compareDebounceTimer = window.setTimeout(() => {
      compareDebounceTimer = undefined
      void compare()
    }, delayMs)
  }

  async function compare() {
    clearDebounceTimer()
    const seq = ++compareSequence
    isComparing.value = true
    try {
      const nextResult = await compareJson({
        leftText: leftText.value,
        rightText: rightText.value,
        sortKeys: sortKeys.value,
        maxReportItems: 500,
      })
      if (seq !== compareSequence) return
      result.value = nextResult
      showFeedback(
        nextResult.ok ? nextResult.summary : nextResult.error || '解析失败',
        nextResult.ok ? 'success' : 'danger',
      )
    } catch (error) {
      if (seq !== compareSequence) return
      const message = error instanceof Error && error.message.trim() ? error.message.trim() : 'JSON 对比失败'
      showFeedback(message, 'danger')
    } finally {
      if (seq === compareSequence) {
        isComparing.value = false
      }
    }
  }

  async function formatSide(side: 'left' | 'right') {
    const current = side === 'left' ? leftText.value : rightText.value
    try {
      const formatted = await formatJson({
        text: current,
        sortKeys: sortKeys.value,
        label: side === 'left' ? '左侧 JSON' : '右侧 JSON',
      })
      if (formatted.ok) {
        if (side === 'left') {
          leftText.value = formatted.text
        } else {
          rightText.value = formatted.text
        }
        showFeedback(side === 'left' ? '左侧已格式化' : '右侧已格式化', 'success')
        await compare()
        return
      }
      showFeedback(formatted.error || '格式化失败', 'danger')
    } catch (error) {
      showFeedback(error instanceof Error && error.message.trim() ? error.message.trim() : '格式化失败', 'danger')
    }
  }

  async function formatBoth() {
    await formatSide('left')
    await formatSide('right')
  }

  async function pasteSide(side: 'left' | 'right') {
    try {
      const text = await Clipboard.Text()
      if (side === 'left') {
        leftText.value = text
      } else {
        rightText.value = text
      }
      showFeedback(side === 'left' ? '剪贴板已写入左侧' : '剪贴板已写入右侧', 'success')
      await compare()
    } catch (error) {
      showFeedback(error instanceof Error && error.message.trim() ? `读取剪贴板失败: ${error.message.trim()}` : '读取剪贴板失败', 'danger')
    }
  }

  async function loadFileSide(side: 'left' | 'right', file: File | null | undefined) {
    if (!file) {
      return
    }
    try {
      const text = await file.text()
      if (side === 'left') {
        leftText.value = text
      } else {
        rightText.value = text
      }
      showFeedback(`${file.name} 已写入${side === 'left' ? '左侧' : '右侧'}`, 'success')
      await compare()
    } catch (error) {
      showFeedback(error instanceof Error && error.message.trim() ? `读取 JSON 文件失败: ${error.message.trim()}` : '读取 JSON 文件失败', 'danger')
    }
  }

  async function loadTextSide(side: 'left' | 'right', text: string, source = '拖放文本') {
    if (!text.trim()) {
      showFeedback('没有可导入的 JSON 文本', 'danger')
      return
    }
    if (side === 'left') {
      leftText.value = text
    } else {
      rightText.value = text
    }
    showFeedback(`${source} 已写入${side === 'left' ? '左侧' : '右侧'}`, 'success')
    await compare()
  }

  async function copyReport() {
    const text = result.value?.report || result.value?.error || ''
    if (!text) {
      showFeedback('没有可复制的报告', 'danger')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('报告已复制', 'success')
    } catch (error) {
      showFeedback(error instanceof Error && error.message.trim() ? `复制报告失败: ${error.message.trim()}` : '复制报告失败', 'danger')
    }
  }

  function swapSides() {
    disarmClear()
    const nextLeft = rightText.value
    rightText.value = leftText.value
    leftText.value = nextLeft
    showFeedback('左右已交换', 'success')
    void compare()
  }

  function clearAll() {
    disarmClear()
    clearDebounceTimer()
    compareSequence++
    leftText.value = ''
    rightText.value = ''
    result.value = null
    showFeedback('已清空', 'success')
  }

  function confirmClearAll() {
    if (!clearArmed.value) {
      clearArmed.value = true
      if (clearArmedTimer !== undefined) window.clearTimeout(clearArmedTimer)
      clearArmedTimer = window.setTimeout(() => {
        clearArmedTimer = undefined
        clearArmed.value = false
      }, ARMED_TIMEOUT_MS)
      showFeedback('再次点击确认清空', 'danger')
      return
    }
    clearAll()
  }

  function disarmClear() {
    clearArmed.value = false
    if (clearArmedTimer !== undefined) {
      window.clearTimeout(clearArmedTimer)
      clearArmedTimer = undefined
    }
  }

  function loadSample() {
    disarmClear()
    leftText.value = sampleLeft
    rightText.value = sampleRight
    showFeedback('已载入示例', 'success')
    void compare()
  }

  function setSortKeys(value: boolean) {
    sortKeys.value = value
    void compare()
  }

  function showFeedback(message: string, tone: 'success' | 'danger' = 'success') {
    if (feedbackTimer !== undefined) {
      window.clearTimeout(feedbackTimer)
      feedbackTimer = undefined
    }
    feedback.value = message
    feedbackTone.value = tone
    const duration = tone === 'danger' ? 5000 : 2000
    feedbackTimer = window.setTimeout(() => {
      feedbackTimer = undefined
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, duration)
  }

  return {
    leftText,
    rightText,
    sortKeys,
    result,
    feedback,
    feedbackTone,
    clearArmed,
    isComparing,
    differenceCount,
    hasDifferences,
    resultTone,
    compare,
    scheduleCompare,
    formatSide,
    formatBoth,
    pasteSide,
    loadFileSide,
    loadTextSide,
    copyReport,
    swapSides,
    clearAll,
    confirmClearAll,
    loadSample,
    setSortKeys,
  }
})
