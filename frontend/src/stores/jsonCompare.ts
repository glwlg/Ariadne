import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import { compareJson, formatJson } from '../services/jsonCompareApi'
import type { JsonCompareResult } from '../types/ariadne'

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

export const useJsonCompareStore = defineStore('json-compare', () => {
  const leftText = ref(sampleLeft)
  const rightText = ref(sampleRight)
  const sortKeys = ref(true)
  const result = ref<JsonCompareResult | null>(null)
  const feedback = ref('')
  const isComparing = ref(false)

  const differenceCount = computed(() => (result.value ? result.value.added + result.value.removed + result.value.changed : 0))
  const hasDifferences = computed(() => Boolean(result.value?.ok && differenceCount.value))
  const resultTone = computed(() => {
    if (!result.value) return 'idle'
    if (!result.value.ok) return 'error'
    return differenceCount.value ? 'changed' : 'same'
  })

  async function compare() {
    isComparing.value = true
    try {
      result.value = await compareJson({
        leftText: leftText.value,
        rightText: rightText.value,
        sortKeys: sortKeys.value,
        maxReportItems: 500,
      })
      showFeedback(result.value.ok ? result.value.summary : result.value.error || '解析失败')
    } finally {
      isComparing.value = false
    }
  }

  async function formatSide(side: 'left' | 'right') {
    const current = side === 'left' ? leftText.value : rightText.value
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
      showFeedback(side === 'left' ? '左侧已格式化' : '右侧已格式化')
      await compare()
      return
    }
    showFeedback(formatted.error || '格式化失败')
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
      showFeedback(side === 'left' ? '剪贴板已写入左侧' : '剪贴板已写入右侧')
      await compare()
    } catch {
      showFeedback('读取剪贴板失败')
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
      showFeedback(`${file.name} 已写入${side === 'left' ? '左侧' : '右侧'}`)
      await compare()
    } catch {
      showFeedback('读取 JSON 文件失败')
    }
  }

  async function loadTextSide(side: 'left' | 'right', text: string, source = '拖放文本') {
    if (!text.trim()) {
      showFeedback('没有可导入的 JSON 文本')
      return
    }
    if (side === 'left') {
      leftText.value = text
    } else {
      rightText.value = text
    }
    showFeedback(`${source} 已写入${side === 'left' ? '左侧' : '右侧'}`)
    await compare()
  }

  async function copyReport() {
    const text = result.value?.report || result.value?.error || ''
    if (!text) {
      showFeedback('没有可复制的报告')
      return
    }
    try {
      await Clipboard.SetText(text)
      showFeedback('报告已复制')
    } catch {
      showFeedback('复制报告失败')
    }
  }

  function swapSides() {
    const nextLeft = rightText.value
    rightText.value = leftText.value
    leftText.value = nextLeft
    showFeedback('左右已交换')
    void compare()
  }

  function clearAll() {
    leftText.value = ''
    rightText.value = ''
    result.value = null
    showFeedback('已清空')
  }

  function loadSample() {
    leftText.value = sampleLeft
    rightText.value = sampleRight
    showFeedback('已载入示例')
    void compare()
  }

  function setSortKeys(value: boolean) {
    sortKeys.value = value
    void compare()
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
    leftText,
    rightText,
    sortKeys,
    result,
    feedback,
    isComparing,
    differenceCount,
    hasDifferences,
    resultTone,
    compare,
    formatSide,
    formatBoth,
    pasteSide,
    loadFileSide,
    loadTextSide,
    copyReport,
    swapSides,
    clearAll,
    loadSample,
    setSortKeys,
  }
})
