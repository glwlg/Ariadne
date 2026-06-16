<script setup lang="ts">
import { computed } from 'vue'
import type { OCRLine } from '../../types/ariadne'

const props = withDefaults(
  defineProps<{
    src?: string
    width?: number
    height?: number
    lines?: OCRLine[]
    isLineSelected?: (index: number) => boolean
    maxHeight?: number
  }>(),
  {
    src: '',
    width: 0,
    height: 0,
    lines: () => [],
    isLineSelected: () => false,
    maxHeight: 360,
  },
)

const emit = defineEmits<{
  'toggle-line': [index: number]
}>()

const validSize = computed(() => props.width > 0 && props.height > 0)

const visibleLines = computed(() => {
  if (!validSize.value) return []
  return props.lines
    .map((line, index) => ({ line, index }))
    .filter(({ line }) => Boolean(line.text.trim()) && Boolean(line.rect) && line.rect!.width > 0 && line.rect!.height > 0)
})

const stageStyle = computed(() => {
  const width = Math.max(1, props.width)
  const height = Math.max(1, props.height)
  const maxWidth = Math.round((props.maxHeight * width) / height)
  return {
    '--ocr-stage-max-height': `${props.maxHeight}px`,
    '--ocr-stage-max-width': `${maxWidth}px`,
    '--ocr-stage-ratio': `${width} / ${height}`,
  }
})

function boxStyle(line: OCRLine) {
  const rect = line.rect
  if (!rect || !validSize.value) return {}
  return {
    left: `${(rect.x / props.width) * 100}%`,
    top: `${(rect.y / props.height) * 100}%`,
    width: `${(rect.width / props.width) * 100}%`,
    height: `${(rect.height / props.height) * 100}%`,
  }
}

function toggleLine(index: number) {
  emit('toggle-line', index)
}
</script>

<template>
  <div class="ocr-image-preview">
    <div v-if="src" class="ocr-image-stage" :style="stageStyle">
      <img :src="src" alt="" />
      <button
        v-for="{ line, index } in visibleLines"
        :key="`${index}-${line.text}`"
        type="button"
        class="ocr-overlay-box"
        :class="{ 'is-selected': isLineSelected(index) }"
        :style="boxStyle(line)"
        :aria-label="`选择 OCR 第 ${index + 1} 行：${line.text}`"
        :aria-pressed="isLineSelected(index)"
        @pointerdown.stop.prevent="toggleLine(index)"
        @keydown.enter.stop.prevent="toggleLine(index)"
        @keydown.space.stop.prevent="toggleLine(index)"
      />
    </div>
    <slot v-else />
  </div>
</template>
