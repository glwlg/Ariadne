import { computed, ref, type Ref } from 'vue'
import type { OCRLine, OCRResult } from '../types/ariadne'

export function createOCRSelection(result: Ref<OCRResult | null>) {
  const selectedOCRLineKeys = ref<string[]>([])

  const ocrLines = computed(() => {
    if (!result.value?.ok) return []
    return (result.value.lines ?? []).filter((line) => line.text.trim())
  })

  const selectedOCRLineCount = computed(() => selectedOCRLineKeys.value.length)

  const selectedOCRText = computed(() => {
    const lines = ocrLines.value
    return selectedOCRLineKeys.value
      .map((key) => lines[Number(key)])
      .filter((line): line is OCRLine => Boolean(line?.text?.trim()))
      .map((line) => line.text.trim())
      .join('\n')
  })

  function isOCRLineSelected(index: number) {
    return selectedOCRLineKeys.value.includes(ocrLineKey(index))
  }

  function toggleOCRLine(index: number) {
    const key = ocrLineKey(index)
    if (selectedOCRLineKeys.value.includes(key)) {
      selectedOCRLineKeys.value = selectedOCRLineKeys.value.filter((item) => item !== key)
      return
    }
    selectedOCRLineKeys.value = [...selectedOCRLineKeys.value, key].sort((left, right) => Number(left) - Number(right))
  }

  function selectAllOCRLines() {
    selectedOCRLineKeys.value = ocrLines.value.map((_, index) => ocrLineKey(index))
  }

  function clearOCRLineSelection() {
    selectedOCRLineKeys.value = []
  }

  return {
    ocrLines,
    selectedOCRLineKeys,
    selectedOCRLineCount,
    selectedOCRText,
    isOCRLineSelected,
    toggleOCRLine,
    selectAllOCRLines,
    clearOCRLineSelection,
  }
}

function ocrLineKey(index: number) {
  return String(index)
}
