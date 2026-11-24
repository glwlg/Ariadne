<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { countHostsLines } from '../../lib/hostsContentStats'

const props = defineProps<{
  title: string
  content: string
  readonly: boolean
  modified: boolean
}>()

const emit = defineEmits<{
  update: [content: string]
}>()

const GUTTER_OVERSCAN = 12
const DEFAULT_LINE_HEIGHT_PX = 20.8
const DEFAULT_VIEWPORT_HEIGHT_PX = 520

const lineNumbers = ref<HTMLElement | null>(null)
const editorTextarea = ref<HTMLTextAreaElement | null>(null)
const scrollTop = ref(0)
const viewportHeight = ref(DEFAULT_VIEWPORT_HEIGHT_PX)
const lineHeightPx = ref(DEFAULT_LINE_HEIGHT_PX)
let resizeObserver: ResizeObserver | null = null

const lines = computed(() => Math.max(1, countHostsLines(props.content)))

const visibleRange = computed(() => {
  const total = lines.value
  const lh = Math.max(12, lineHeightPx.value)
  const topOffset = Math.max(0, scrollTop.value - 14)
  const firstVisible = Math.floor(topOffset / lh) + 1
  const visibleCount = Math.max(1, Math.ceil(viewportHeight.value / lh))
  const start = Math.max(1, firstVisible - GUTTER_OVERSCAN)
  const end = Math.min(total, firstVisible + visibleCount + GUTTER_OVERSCAN)
  const result: number[] = []
  for (let line = start; line <= end; line += 1) {
    result.push(line)
  }
  return {
    start,
    end,
    lines: result,
    topSpacerPx: (start - 1) * lh,
    bottomSpacerPx: Math.max(0, total - end) * lh,
  }
})

function measureMetrics() {
  if (editorTextarea.value) {
    viewportHeight.value = editorTextarea.value.clientHeight || DEFAULT_VIEWPORT_HEIGHT_PX
  }
  if (lineNumbers.value && typeof window !== 'undefined' && typeof window.getComputedStyle === 'function') {
    const computedStyle = window.getComputedStyle(lineNumbers.value)
    const parsedLineHeight = Number.parseFloat(computedStyle.lineHeight)
    const parsedFontSize = Number.parseFloat(computedStyle.fontSize)
    if (Number.isFinite(parsedLineHeight) && parsedLineHeight > 0) {
      lineHeightPx.value = parsedLineHeight
    } else if (Number.isFinite(parsedFontSize) && parsedFontSize > 0) {
      lineHeightPx.value = parsedFontSize * 1.6
    }
  }
}

function syncScroll(event: Event) {
  const target = event.target as HTMLTextAreaElement
  scrollTop.value = target.scrollTop
  if (lineNumbers.value) {
    lineNumbers.value.scrollTop = target.scrollTop
  }
}

function updateContent(event: Event) {
  emit('update', (event.target as HTMLTextAreaElement).value)
  void nextTick(() => syncScroll(event))
}

watch(
  () => props.title,
  () => {
    void nextTick(() => {
      if (editorTextarea.value) {
        scrollTop.value = editorTextarea.value.scrollTop
        if (lineNumbers.value) {
          lineNumbers.value.scrollTop = editorTextarea.value.scrollTop
        }
      }
      measureMetrics()
    })
  },
)

onMounted(() => {
  measureMetrics()
  if (typeof ResizeObserver !== 'undefined' && editorTextarea.value) {
    resizeObserver = new ResizeObserver(() => measureMetrics())
    resizeObserver.observe(editorTextarea.value)
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
})
</script>

<template>
  <section class="hosts-editor-pane" aria-label="Hosts 编辑器">
    <header class="hosts-editor-pane-header">
      <strong>Hosts 编辑器 · {{ title }}</strong>
      <span class="system-pill">{{ readonly ? '只读' : '可编辑' }}</span>
    </header>
    <div class="hosts-editor-code-area">
      <div ref="lineNumbers" class="hosts-line-numbers" aria-hidden="true">
        <div
          v-if="visibleRange.topSpacerPx > 0"
          class="hosts-line-spacer"
          :style="{ height: `${visibleRange.topSpacerPx}px` }"
        />
        <span v-for="line in visibleRange.lines" :key="line">{{ line }}</span>
        <div
          v-if="visibleRange.bottomSpacerPx > 0"
          class="hosts-line-spacer"
          :style="{ height: `${visibleRange.bottomSpacerPx}px` }"
        />
      </div>
      <textarea
        ref="editorTextarea"
        class="hosts-code"
        spellcheck="false"
        :readonly="readonly"
        :value="content"
        @input="updateContent"
        @scroll="syncScroll"
      />
    </div>
    <footer class="hosts-editor-status">
      <span>共 {{ lines }} 行</span>
      <span class="hosts-editor-status-spacer" />
      <span>{{ modified ? '已修改' : '未修改' }}</span>
      <span>UTF-8</span>
    </footer>
  </section>
</template>
