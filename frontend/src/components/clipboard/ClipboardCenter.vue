<script setup lang="ts">
import {
  ArrowLeft,
  Clipboard,
  Copy,
  Database,
  FileText,
  Image,
  Pin,
  Plus,
  QrCode,
  Search,
  Trash2,
} from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useClipboardHistoryStore } from '../../stores/clipboardHistory'
import { ocrConfidenceLabel, ocrRectLabel } from '../../lib/ocrDisplay'
import type { ClipboardHistoryEntry } from '../../types/ariadne'

const appShell = useAppShellStore()
const clipboard = useClipboardHistoryStore()

const selected = computed(() => clipboard.selectedEntry)
let refreshTimer = 0

onMounted(() => {
  void clipboard.load()
  refreshTimer = window.setInterval(() => {
    void clipboard.load()
  }, 2500)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})

function formatTime(seconds: number) {
  if (!seconds) return '未知时间'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(seconds * 1000))
}

function contentLabel(entry: ClipboardHistoryEntry) {
  const labels: Record<string, string> = {
    command: '命令',
    code: '代码',
    json: 'JSON',
    path: '路径',
    sql: 'SQL',
    text: '文本',
    url: 'URL',
    image: '图片',
  }
  return labels[entry.contentType] ?? entry.contentType
}

</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell clipboard-shell" aria-label="剪贴板历史中心">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Clipboard :size="18" />
          </div>
          <div class="brand-copy">
            <span>剪贴板历史</span>
            <small>Local text timeline, pinned reuse, memory-ready evidence</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="clipboard.pinnedCount ? 'is-on' : ''">
              <Pin :size="13" />
              置顶 {{ clipboard.pinnedCount }}
            </span>
            <span class="system-pill" :class="clipboard.status?.watcherRunning ? 'is-on' : ''">
              <Clipboard :size="13" />
              {{ clipboard.status?.watcherRunning ? '监听中' : '监听暂停' }}
            </span>
            <span class="system-pill">
              <Database :size="13" />
              {{ clipboard.status?.count ?? clipboard.entries.length }} 条
            </span>
            <span class="system-pill" :class="clipboard.status?.imageCount ? 'is-on' : ''">
              <Image :size="13" />
              图片 {{ clipboard.status?.imageCount ?? 0 }}
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="tool-toolbar">
          <div class="tool-search">
            <Search :size="17" />
            <input
              :value="clipboard.query"
              spellcheck="false"
              placeholder="搜索剪贴板文本、JSON、URL、命令或路径..."
              @input="clipboard.setQuery(($event.target as HTMLInputElement).value)"
            />
          </div>
          <AriButton size="sm" variant="primary" @click="clipboard.collectCurrentText()">
            <Plus :size="14" />
            收集当前剪贴板
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="clipboard.clearUnpinned()">
            <Trash2 :size="14" />
            {{ clipboard.clearArmed ? '确认清空' : '清空未置顶' }}
          </AriButton>
        </div>

        <div class="clipboard-workspace">
          <section class="clipboard-list" aria-label="剪贴板条目">
            <button
              v-for="entry in clipboard.entries"
              :key="entry.id"
              class="clipboard-row"
              :class="{ 'is-selected': entry.id === clipboard.selectedId }"
              @click="clipboard.select(entry.id)"
            >
              <span class="clipboard-row-icon" :class="{ 'is-pinned': entry.pinned }">
                <Pin v-if="entry.pinned" :size="15" />
                <Image v-else-if="entry.type === 'image'" :size="15" />
                <Clipboard v-else :size="15" />
              </span>
              <span class="clipboard-row-main">
                <span class="clipboard-row-title">{{ entry.summary }}</span>
                <span class="clipboard-row-meta">
                  {{ contentLabel(entry) }}
                  <template v-if="entry.type === 'image'"> · {{ entry.width }}x{{ entry.height }}</template>
                  · {{ formatTime(entry.createdAt) }} · {{ entry.source }}
                </span>
              </span>
            </button>

            <div v-if="!clipboard.entries.length" class="empty-state">
              <Clipboard :size="22" />
              <span>还没有匹配的剪贴板记录</span>
            </div>
          </section>

          <section class="clipboard-detail" aria-label="剪贴板详情">
            <template v-if="selected">
              <div class="clipboard-detail-header">
                <div>
                  <span class="preview-kicker">{{ contentLabel(selected) }}</span>
                  <h1>{{ selected.summary }}</h1>
                  <p>{{ formatTime(selected.createdAt) }} · {{ selected.source }}</p>
                </div>
                <span class="system-pill" :class="selected.pinned ? 'is-on' : ''">
                  <Pin :size="13" />
                  {{ selected.pinned ? '已置顶' : '未置顶' }}
                </span>
              </div>

              <div class="clipboard-actions">
                <AriButton size="sm" variant="primary" @click="clipboard.copyEntry(selected)">
                  <Copy :size="14" />
                  {{ selected.type === 'image' ? '复制图片' : '复制内容' }}
                </AriButton>
                <AriButton v-if="selected.type === 'image'" size="sm" variant="secondary" @click="clipboard.scanQRCode(selected)">
                  <QrCode :size="14" />
                  识别二维码
                </AriButton>
                <AriButton v-if="selected.type === 'image'" size="sm" variant="secondary" :disabled="clipboard.isRecognizingOCR" @click="clipboard.recognizeText(selected)">
                  <FileText :size="14" />
                  {{ clipboard.isRecognizingOCR ? 'OCR 中' : '识别文字' }}
                </AriButton>
                <AriButton v-if="selected.type === 'image'" size="sm" variant="secondary" @click="clipboard.addImageToCapture(selected)">
                  <Image :size="14" />
                  加入截图历史
                </AriButton>
                <AriButton v-if="selected.type === 'image'" size="sm" variant="secondary" @click="clipboard.pinImage(selected)">
                  <Pin :size="14" />
                  贴到屏幕
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="clipboard.togglePin(selected)">
                  <Pin :size="14" />
                  {{ selected.pinned ? '取消置顶' : '置顶' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="clipboard.deleteEntry(selected)">
                  <Trash2 :size="14" />
                  {{ clipboard.deleteArmedId === selected.id ? '确认删除' : '删除' }}
                </AriButton>
              </div>

              <figure v-if="selected.type === 'image'" class="clipboard-image-preview">
                <OCRImageOverlay
                  v-if="clipboard.imageDataUrl"
                  :src="clipboard.imageDataUrl"
                  :width="clipboard.ocrResult?.width || selected.width"
                  :height="clipboard.ocrResult?.height || selected.height"
                  :lines="clipboard.ocrLines"
                  :is-line-selected="clipboard.isOCRLineSelected"
                  :max-height="300"
                  @toggle-line="clipboard.toggleOCRLine"
                />
                <div v-else class="capture-preview-empty">图片预览不可用</div>
                <figcaption>{{ selected.width }}x{{ selected.height }} · {{ selected.bytes }} bytes</figcaption>
              </figure>
              <pre v-else class="clipboard-text">{{ selected.text }}</pre>

              <div v-if="clipboard.qrResult" class="qr-result-panel" :class="{ 'is-success': clipboard.qrResult.ok }">
                <div class="side-title">
                  <QrCode :size="15" />
                  二维码识别
                </div>
                <template v-if="clipboard.qrResult.ok">
                  <strong>{{ clipboard.qrResult.format || 'QR_CODE' }}</strong>
                  <p>{{ clipboard.qrResult.text }}</p>
                  <AriButton size="sm" variant="secondary" @click="clipboard.copyQRText()">
                    <Copy :size="14" />
                    复制内容
                  </AriButton>
                </template>
                <template v-else>
                  <strong>未识别到二维码</strong>
                  <p>{{ clipboard.qrResult.error || '当前图片中没有可识别的二维码。' }}</p>
                </template>
              </div>

              <div v-if="clipboard.ocrResult" class="qr-result-panel" :class="{ 'is-success': clipboard.ocrResult.ok }">
                <div class="side-title">
                  <FileText :size="15" />
                  OCR 文字识别
                </div>
                <template v-if="clipboard.ocrResult.ok">
                  <strong>
                    {{ clipboard.ocrResult.provider || 'RapidOCR' }} · {{ clipboard.ocrResult.elapsedMs || 0 }}ms
                    <template v-if="clipboard.ocrLines.length"> · {{ clipboard.ocrLines.length }} 行 · 已选 {{ clipboard.selectedOCRLineCount }}</template>
                  </strong>
                  <div v-if="clipboard.ocrLines.length" class="ocr-selection-panel">
                    <div class="ocr-selection-actions">
                      <AriButton size="sm" variant="secondary" @click="clipboard.selectAllOCRLines()">全选</AriButton>
                      <AriButton size="sm" variant="ghost" @click="clipboard.clearOCRLineSelection()">清空</AriButton>
                      <AriButton size="sm" variant="secondary" :disabled="!clipboard.selectedOCRLineCount" @click="clipboard.copySelectedOCRText()">
                        <Copy :size="14" />
                        复制选中
                      </AriButton>
                      <AriButton v-if="clipboard.ocrResult.text" size="sm" variant="secondary" @click="clipboard.copyOCRText()">
                        <Copy :size="14" />
                        复制全文
                      </AriButton>
                    </div>
                    <div class="ocr-line-list" aria-label="OCR 文本行">
                      <button
                        v-for="(line, index) in clipboard.ocrLines"
                        :key="`${index}-${line.text}`"
                        type="button"
                        class="ocr-line-row"
                        :class="{ 'is-selected': clipboard.isOCRLineSelected(index) }"
                        :aria-pressed="clipboard.isOCRLineSelected(index)"
                        @click="clipboard.toggleOCRLine(index)"
                      >
                        <span class="ocr-line-check">{{ clipboard.isOCRLineSelected(index) ? '已选' : '选择' }}</span>
                        <span class="ocr-line-body">
                          <span class="ocr-line-text">{{ line.text }}</span>
                          <span class="ocr-line-meta">{{ ocrConfidenceLabel(line.confidence) }} · {{ ocrRectLabel(line) }}</span>
                        </span>
                      </button>
                    </div>
                  </div>
                  <template v-else>
                    <p>{{ clipboard.ocrResult.text || '未识别到文字' }}</p>
                    <AriButton v-if="clipboard.ocrResult.text" size="sm" variant="secondary" @click="clipboard.copyOCRText()">
                      <Copy :size="14" />
                      复制全文
                    </AriButton>
                  </template>
                </template>
                <template v-else>
                  <strong>OCR 不可用</strong>
                  <p>{{ clipboard.ocrResult.error || '本地 OCR 组件不可用。' }}</p>
                </template>
              </div>

              <div class="meta-grid">
                <div class="meta-item">
                  <span>{{ selected.type === 'image' ? '图片路径' : '配置文件' }}</span>
                  <strong>{{ selected.type === 'image' ? selected.imagePath : clipboard.status?.path || '%APPDATA%/Ariadne/ariadne.sqlite' }}</strong>
                </div>
                <div class="meta-item">
                  <span>签名</span>
                  <strong>{{ selected.signature }}</strong>
                </div>
              </div>
            </template>

            <div v-else class="empty-state">
              <Clipboard :size="22" />
              <span>选择一条记录查看详情</span>
            </div>
          </section>
        </div>

        <footer class="status-strip">
          <span>
            <Clipboard :size="14" />
            文本剪贴板历史已本地持久化{{ clipboard.status?.watcherRunning ? '，自动监听中' : '' }}
          </span>
          <span v-if="clipboard.status?.lastWatcherError" class="inline-feedback">
            {{ clipboard.status.lastWatcherError }}
          </span>
          <span>
            <Pin :size="14" />
            置顶记录不会被清空未置顶删除
          </span>
          <span v-if="clipboard.feedback" class="inline-feedback">{{ clipboard.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
