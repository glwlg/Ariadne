<script setup lang="ts">
import { Brain, Copy, FileText, Flag, ImageOff, Sparkles, Tags, Trash2, X } from '@lucide/vue'
import { toRefs } from 'vue'
import AriButton from '../../../ui/AriButton.vue'
import OCRImageOverlay from '../../../ocr/OCRImageOverlay.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  detailDrawerOpen,
  formatTime,
  memory,
  selected,
  sourceLabel,
} = toRefs(ctx)
</script>

<template>
  <div v-if="detailDrawerOpen && selected" class="flow-detail-backdrop" @click.self="detailDrawerOpen = false">
    <aside class="flow-detail-drawer" aria-label="心流证据明细">
      <div class="flow-detail-head">
        <div>
          <span>{{ sourceLabel(selected) }} · {{ formatTime(selected.createdAt) }}</span>
          <h2>{{ selected.title }}</h2>
          <p>{{ selected.summary }}</p>
        </div>
        <button type="button" class="flow-icon-button" aria-label="关闭明细" @click="detailDrawerOpen = false">
          <X :size="16" />
        </button>
      </div>

      <div class="memory-capture-frame flow-detail-capture" :class="{ 'has-image': Boolean(memory.selectedImageUrl) }">
        <OCRImageOverlay
          v-if="memory.selectedImageUrl"
          :src="memory.selectedImageUrl"
          :width="memory.ocrResult?.width || selected.width"
          :height="memory.ocrResult?.height || selected.height"
          :lines="memory.ocrLines"
          :is-line-selected="memory.isOCRLineSelected"
          :max-height="260"
          @toggle-line="memory.toggleOCRLine"
        />
        <template v-else>
          <ImageOff v-if="memory.selectedImageMissing" :size="24" />
          <Sparkles v-else :size="24" />
          <span>{{ memory.selectedImageMissing ? '原图已清理' : selected.windowTitle || 'Ariadne context' }}</span>
        </template>
      </div>

      <div class="flow-detail-actions">
        <AriButton size="sm" variant="secondary" :disabled="memory.isRecognizingOCR || !selected.imagePath || memory.selectedImageMissing" @click="memory.recognizeSelectedText()">
          <FileText :size="14" />
          {{ memory.isRecognizingOCR ? 'OCR 中' : '再次 OCR' }}
        </AriButton>
        <AriButton v-if="selected.ocrText" size="sm" variant="secondary" @click="memory.copyOCRText()">
          <Copy :size="14" />
          复制 OCR
        </AriButton>
        <AriButton size="sm" variant="secondary" @click="memory.buildKnowledgeDraft()">
          <Brain :size="14" />
          知识草稿
        </AriButton>
        <AriButton size="sm" variant="ghost" @click="memory.deleteSelected()">
          <Trash2 :size="14" />
          {{ memory.deleteArmedId === selected.id ? '确认删除' : '删除' }}
        </AriButton>
      </div>

      <div class="meta-grid flow-detail-meta">
        <div class="meta-item">
          <span>来源</span>
          <strong>{{ sourceLabel(selected) }}</strong>
        </div>
        <div class="meta-item">
          <span>应用</span>
          <strong>{{ selected.appName || '-' }}</strong>
        </div>
        <div class="meta-item">
          <span>窗口</span>
          <strong>{{ selected.windowTitle || '-' }}</strong>
        </div>
      </div>

      <pre v-if="selected.text" class="preview-text memory-text flow-detail-text">{{ selected.text }}</pre>
      <details v-if="selected.ocrText" class="flow-raw-ocr">
        <summary>原始 OCR 证据</summary>
        <pre class="preview-text memory-text flow-detail-text">{{ selected.ocrText }}</pre>
      </details>

      <div class="tag-row">
        <span v-for="tag in selected.tags" :key="tag">
          <Tags :size="12" />
          {{ tag }}
        </span>
        <span v-if="selected.favorite">
          <Flag :size="12" />
          收藏
        </span>
      </div>
    </aside>
  </div>
</template>
