<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'

const props = defineProps<{
  title: string
  content: string
  readonly: boolean
  modified: boolean
}>()

const emit = defineEmits<{
  update: [content: string]
}>()

const lineNumbers = ref<HTMLElement | null>(null)
const lines = computed(() => Math.max(1, props.content.replace(/\r\n?/g, '\n').split('\n').length))

function syncScroll(event: Event) {
  if (!lineNumbers.value) return
  lineNumbers.value.scrollTop = (event.target as HTMLTextAreaElement).scrollTop
}

function updateContent(event: Event) {
  emit('update', (event.target as HTMLTextAreaElement).value)
  void nextTick(() => syncScroll(event))
}
</script>

<template>
  <section class="hosts-editor-pane" aria-label="Hosts 编辑器">
    <header class="hosts-editor-pane-header">
      <strong>Hosts 编辑器 · {{ title }}</strong>
      <span class="system-pill">{{ readonly ? '只读' : '可编辑' }}</span>
    </header>
    <div class="hosts-editor-code-area">
      <div ref="lineNumbers" class="hosts-line-numbers" aria-hidden="true">
        <span v-for="line in lines" :key="line">{{ line }}</span>
      </div>
      <textarea
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
