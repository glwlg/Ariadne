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
import FancunButton from '../ui/FancunButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { isDesktopRuntime } from '../../services/runtimeEnv'
import { useAppShellStore } from '../../stores/appShell'
import { useClipboardHistoryStore } from '../../stores/clipboardHistory'
import { ocrConfidenceLabel, ocrRectLabel } from '../../lib/ocrDisplay'
import type { ClipboardHistoryEntry } from '../../types/fancun'

const timeFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

const contentTypeFilters = [
  { id: 'all', label: '全部' },
  { id: 'text', label: '文本' },
  { id: 'image', label: '图片' },
  { id: 'json', label: 'JSON' },
  { id: 'url', label: 'URL' },
  { id: 'code', label: '代码' },
]

const appShell = useAppShellStore()
const clipboard = useClipboardHistoryStore()

const selected = computed(() => clipboard.selectedEntry)
let refreshTimer = 0
let disposed = false
let unsubscribeUpdates: (() => void) | null = null

async function refresh() {
  if (disposed) return
  if (!document.hidden && !clipboard.isLoading) await clipboard.load()
  if (!disposed) refreshTimer = window.setTimeout(refresh, 2500)
}

function onVisibilityChange() {
  if (!document.hidden && !clipboard.isLoading) void clipboard.load()
}

onMounted(() => {
  if (isDesktopRuntime()) {
    void clipboard.load()
    unsubscribeUpdates = clipboard.subscribeUpdates()
  } else {
    void refresh()
  }
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  disposed = true
  window.clearTimeout(refreshTimer)
  unsubscribeUpdates?.()
  unsubscribeUpdates = null
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

function formatTime(seconds: number) {
  if (!seconds) return '未知时间'
  return timeFormatter.format(new Date(seconds * 1000))
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
            <small>本地文本时间线 · 置顶复用</small>
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
            <FancunButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </FancunButton>
          </div>
        </header>

        <div class="tool-toolbar is-wrapping">
          <div class="tool-search">
            <Search :size="17" />
            <input
              :value="clipboard.query"
              spellcheck="false"
              placeholder="搜索剪贴板文本、JSON、URL、命令或路径..."
              @input="clipboard.setQuery(($event.target as HTMLInputElement).value)"
            />
          </div>
          <FancunButton size="sm" variant="primary" @click="clipboard.collectCurrentText()">
            <Plus :size="14" />
            收集当前剪贴板
          </FancunButton>
          <FancunButton
            size="sm"
            variant="secondary"
            :class="{ 'is-armed-danger': clipboard.clearArmed }"
            @click="clipboard.clearUnpinned()"
          >
            <Trash2 :size="14" />
            {{ clipboard.clearArmed ? '确认清空' : '清空未置顶' }}
          </FancunButton>
          <div class="flex flex-wrap items-center gap-1.5 w-full" role="group" aria-label="内容类型筛选">
            <button
              v-for="filter in contentTypeFilters"
              :key="filter.id"
              type="button"
              class="system-pill cursor-pointer"
              :class="{ 'is-on': clipboard.contentTypeFilter === filter.id }"
              :aria-pressed="clipboard.contentTypeFilter === filter.id"
              @click="clipboard.setContentTypeFilter(filter.id)"
            >
              {{ filter.label }}
            </button>
          </div>
        </div>

        <div class="clipboard-workspace">
          <section class="clipboard-list" aria-label="剪贴板条目">
            <div v-if="clipboard.isLoading && !clipboard.filteredEntries.length" class="tool-loading-state">
              <Clipboard :size="22" />
              <strong>正在加载剪贴板历史...</strong>
              <small>读取本地剪贴板记录</small>
            </div>

            <div v-else-if="clipboard.loadError && !clipboard.filteredEntries.length" class="tool-error-state">
              <Clipboard :size="22" />
              <strong>剪贴板历史加载失败</strong>
              <small>{{ clipboard.loadError }}</small>
              <FancunButton size="sm" variant="secondary" @click="clipboard.load()">重试加载</FancunButton>
            </div>

            <template v-else-if="clipboard.filteredEntries.length">
              <button
                v-for="entry in clipboard.filteredEntries"
                :key="entry.id"
                class="clipboard-row"
                :class="{ 'is-selected': entry.id === clipboard.selectedId }"
                @click="clipboard.select(entry.id)"
                @dblclick="clipboard.copyEntry(entry)"
                @keydown.enter.prevent="clipboard.copyEntry(entry)"
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
            </template>

            <div v-else class="tool-empty-state">
              <Clipboard :size="22" />
              <strong>还没有匹配的剪贴板记录</strong>
              <small>尝试切换筛选条件或复制新的文本/图片</small>
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
                <FancunButton size="sm" variant="primary" @click="clipboard.copyEntry(selected)">
                  <Copy :size="14" />
                  {{ selected.type === 'image' ? '复制图片' : '复制内容' }}
                </FancunButton>
                <FancunButton v-if="selected.type === 'image'" size="sm" variant="secondary" :disabled="clipboard.isScanningQR" @click="clipboard.scanQRCode(selected)">
                  <QrCode :size="14" />
                  {{ clipboard.isScanningQR ? '识别中' : '识别二维码' }}
                </FancunButton>
                <FancunButton v-if="selected.type === 'image'" size="sm" variant="secondary" :disabled="clipboard.isRecognizingOCR" @click="clipboard.recognizeText(selected)">
                  <FileText :size="14" />
                  {{ clipboard.isRecognizingOCR ? 'OCR 中' : '识别文字' }}
                </FancunButton>
                <FancunButton v-if="selected.type === 'image'" size="sm" variant="secondary" @click="clipboard.addImageToCapture(selected)">
                  <Image :size="14" />
                  加入截图历史
                </FancunButton>
                <FancunButton v-if="selected.type === 'image'" size="sm" variant="secondary" @click="clipboard.pinImage(selected)">
                  <Pin :size="14" />
                  贴到屏幕
                </FancunButton>
                <FancunButton size="sm" variant="secondary" @click="clipboard.togglePin(selected)">
                  <Pin :size="14" />
                  {{ selected.pinned ? '取消置顶' : '置顶' }}
                </FancunButton>
                <FancunButton
                  size="sm"
                  variant="secondary"
                  :class="{ 'is-armed-danger': clipboard.deleteArmedId === selected.id }"
                  @click="clipboard.deleteEntry(selected)"
                >
                  <Trash2 :size="14" />
                  {{ clipboard.deleteArmedId === selected.id ? '确认删除' : '删除' }}
                </FancunButton>
              </div>

              <figure v-if="selected.type === 'image'" class="clipboard-image-preview">
                <div v-if="clipboard.isLoadingImage" class="tool-loading-state">
                  <Image :size="22" />
                  <strong>正在加载图片预览...</strong>
                </div>
                <OCRImageOverlay
                  v-else-if="clipboard.imageDataUrl"
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

              <div v-if="clipboard.isScanningQR" class="qr-result-panel" aria-live="polite">
                <div class="side-title">
                  <QrCode :size="15" />
                  正在识别二维码...
                </div>
                <p>正在分析图片内容，请稍候。</p>
              </div>

              <div v-if="clipboard.isRecognizingOCR" class="qr-result-panel" aria-live="polite">
                <div class="side-title">
                  <FileText :size="15" />
                  正在执行 OCR 文字识别...
                </div>
                <p>正在提取图片文字与坐标，请稍候。</p>
              </div>

              <div v-if="clipboard.qrResult" class="qr-result-panel" :class="{ 'is-success': clipboard.qrResult.ok }">
                <div class="side-title">
                  <QrCode :size="15" />
                  二维码识别
                </div>
                <template v-if="clipboard.qrResult.ok">
                  <strong>{{ clipboard.qrResult.format || 'QR_CODE' }}</strong>
                  <p>{{ clipboard.qrResult.text }}</p>
                  <FancunButton size="sm" variant="secondary" @click="clipboard.copyQRText()">
                    <Copy :size="14" />
                    复制内容
                  </FancunButton>
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
                      <FancunButton size="sm" variant="secondary" @click="clipboard.selectAllOCRLines()">全选</FancunButton>
                      <FancunButton size="sm" variant="ghost" @click="clipboard.clearOCRLineSelection()">清空</FancunButton>
                      <FancunButton size="sm" variant="secondary" :disabled="!clipboard.selectedOCRLineCount" @click="clipboard.copySelectedOCRText()">
                        <Copy :size="14" />
                        复制选中
                      </FancunButton>
                      <FancunButton v-if="clipboard.ocrResult.text" size="sm" variant="secondary" @click="clipboard.copyOCRText()">
                        <Copy :size="14" />
                        复制全文
                      </FancunButton>
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
                    <FancunButton v-if="clipboard.ocrResult.text" size="sm" variant="secondary" @click="clipboard.copyOCRText()">
                      <Copy :size="14" />
                      复制全文
                    </FancunButton>
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
                  <strong>{{ selected.type === 'image' ? selected.imagePath : clipboard.status?.path || '%APPDATA%/Fancun/fancun.sqlite' }}</strong>
                </div>
                <div class="meta-item">
                  <span>签名</span>
                  <strong>{{ selected.signature }}</strong>
                </div>
              </div>
            </template>

            <div v-else class="tool-empty-state">
              <Clipboard :size="22" />
              <strong>选择一条记录查看详情</strong>
            </div>
          </section>
        </div>

        <footer class="status-strip">
          <span>
            <Clipboard :size="14" />
            文本剪贴板历史已本地持久化{{ clipboard.status?.watcherRunning ? '，自动监听中' : '' }}
          </span>
          <span v-if="clipboard.status?.lastWatcherError" class="inline-feedback is-danger">
            {{ clipboard.status.lastWatcherError }}
          </span>
          <span>
            <Pin :size="14" />
            置顶记录不会被清空未置顶删除
          </span>
          <span
            v-if="clipboard.feedback"
            class="inline-feedback"
            :class="{ 'is-danger': clipboard.feedbackTone === 'danger' }"
          >{{ clipboard.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
