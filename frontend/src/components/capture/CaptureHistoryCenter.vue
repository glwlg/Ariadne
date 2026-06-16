<script setup lang="ts">
import {
  ArrowLeft,
  Camera,
  Copy,
  Database,
  FileText,
  FolderOpen,
  Image as ImageIcon,
  Pin,
  QrCode,
  Search,
  Trash2,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRImageOverlay from '../ocr/OCRImageOverlay.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useCaptureHistoryStore } from '../../stores/captureHistory'
import { ocrConfidenceLabel, ocrRectLabel } from '../../lib/ocrDisplay'
import type { CaptureHistoryEntry } from '../../types/ariadne'

const appShell = useAppShellStore()
const capture = useCaptureHistoryStore()

const selected = computed(() => capture.selectedEntry)

onMounted(() => {
  void capture.load()
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

function dimensions(entry: CaptureHistoryEntry) {
  return entry.width > 0 && entry.height > 0 ? `${entry.width} x ${entry.height}` : '未知尺寸'
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell capture-shell" aria-label="截图历史中心">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Camera :size="18" />
          </div>
          <div class="brand-copy">
            <span>截图历史</span>
            <small>Local screen captures, visual recall, pinned evidence</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="capture.pinnedCount ? 'is-on' : ''">
              <Pin :size="13" />
              置顶 {{ capture.pinnedCount }}
            </span>
            <span class="system-pill">
              <Database :size="13" />
              {{ capture.status?.count ?? capture.entries.length }} 张
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
              :value="capture.query"
              spellcheck="false"
              placeholder="搜索尺寸、来源、路径或标签..."
              @input="capture.setQuery(($event.target as HTMLInputElement).value)"
            />
          </div>
          <AriButton size="sm" variant="primary" :disabled="capture.isCapturing" @click="capture.captureScreen()">
            <Camera :size="14" />
            {{ capture.isCapturing ? '捕获中' : '捕获当前屏幕' }}
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="capture.openOverlay()">
            <ImageIcon :size="14" />
            区域截图
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="capture.isScanningQR" @click="capture.scanCurrentScreenQRCode()">
            <QrCode :size="14" />
            {{ capture.isScanningQR ? '识别中' : '识别当前屏幕' }}
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="capture.isRecognizingOCR" @click="capture.recognizeCurrentScreenText()">
            <FileText :size="14" />
            {{ capture.isRecognizingOCR ? 'OCR 中' : 'OCR 当前屏幕' }}
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="capture.clearUnpinned()">
            <Trash2 :size="14" />
            {{ capture.clearArmed ? '确认清空' : '清空未置顶' }}
          </AriButton>
        </div>

        <div class="capture-workspace">
          <section class="capture-list" aria-label="截图条目">
            <button
              v-for="entry in capture.entries"
              :key="entry.id"
              class="capture-row"
              :class="{ 'is-selected': entry.id === capture.selectedId }"
              @click="capture.select(entry.id)"
            >
              <span class="capture-row-icon" :class="{ 'is-pinned': entry.pinned }">
                <Pin v-if="entry.pinned" :size="15" />
                <ImageIcon v-else :size="15" />
              </span>
              <span class="capture-row-main">
                <span class="capture-row-title">{{ dimensions(entry) }}</span>
                <span class="capture-row-meta">
                  {{ formatTime(entry.createdAt) }} · {{ entry.source }} · {{ formatBytes(entry.bytes) }}
                </span>
              </span>
            </button>

            <div v-if="!capture.entries.length" class="empty-state">
              <Camera :size="22" />
              <span>还没有匹配的截图记录</span>
            </div>
          </section>

          <section class="capture-detail" aria-label="截图详情">
            <template v-if="selected">
              <div class="capture-detail-header">
                <div>
                  <span class="preview-kicker">{{ selected.source }}</span>
                  <h1>{{ dimensions(selected) }}</h1>
                  <p>{{ formatTime(selected.createdAt) }} · {{ formatBytes(selected.bytes) }}</p>
                </div>
                <span class="system-pill" :class="selected.pinned ? 'is-on' : ''">
                  <Pin :size="13" />
                  {{ selected.pinned ? '已置顶' : '未置顶' }}
                </span>
              </div>

              <div class="capture-preview-frame">
                <OCRImageOverlay
                  v-if="capture.imageDataUrl"
                  :src="capture.imageDataUrl"
                  :width="capture.ocrResult?.width || selected.width"
                  :height="capture.ocrResult?.height || selected.height"
                  :lines="capture.ocrLines"
                  :is-line-selected="capture.isOCRLineSelected"
                  @toggle-line="capture.toggleOCRLine"
                />
                <div v-else class="capture-preview-empty">
                  <ImageIcon :size="28" />
                  <span>无法读取预览图</span>
                </div>
              </div>

              <div class="capture-actions">
                <AriButton size="sm" variant="primary" @click="capture.openImage(selected)">
                  <ImageIcon :size="14" />
                  打开
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="capture.openFolder(selected)">
                  <FolderOpen :size="14" />
                  打开所在文件夹
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="capture.copyPath(selected)">
                  <Copy :size="14" />
                  复制路径
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="capture.pinImage(selected)">
                  <Pin :size="14" />
                  创建贴图
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="capture.isScanningQR" @click="capture.scanQRCode(selected)">
                  <QrCode :size="14" />
                  {{ capture.isScanningQR ? '识别中' : '识别二维码' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="capture.isRecognizingOCR" @click="capture.recognizeText(selected)">
                  <FileText :size="14" />
                  {{ capture.isRecognizingOCR ? 'OCR 中' : '识别文字' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="capture.togglePin(selected)">
                  <Pin :size="14" />
                  {{ selected.pinned ? '取消置顶' : '置顶' }}
                </AriButton>
                <AriButton size="sm" variant="secondary" @click="capture.deleteEntry(selected)">
                  <Trash2 :size="14" />
                  {{ capture.deleteArmedId === selected.id ? '确认删除' : '删除' }}
                </AriButton>
              </div>

              <div v-if="capture.qrResult" class="qr-result-panel" :class="{ 'is-success': capture.qrResult.ok }">
                <div class="side-title">
                  <QrCode :size="15" />
                  二维码识别
                </div>
                <template v-if="capture.qrResult.ok">
                  <strong>{{ capture.qrResult.format || 'QR_CODE' }}</strong>
                  <p>{{ capture.qrResult.text }}</p>
                  <AriButton size="sm" variant="secondary" @click="capture.copyQRText()">
                    <Copy :size="14" />
                    复制内容
                  </AriButton>
                </template>
                <template v-else>
                  <strong>未识别到二维码</strong>
                  <p>{{ capture.qrResult.error || '当前图片中没有可识别的二维码。' }}</p>
                </template>
              </div>

              <div v-if="capture.ocrResult" class="qr-result-panel" :class="{ 'is-success': capture.ocrResult.ok }">
                <div class="side-title">
                  <FileText :size="15" />
                  OCR 文字识别
                </div>
                <template v-if="capture.ocrResult.ok">
                  <strong>
                    {{ capture.ocrResult.provider || 'RapidOCR' }} · {{ capture.ocrResult.elapsedMs || 0 }}ms
                    <template v-if="capture.ocrLines.length"> · {{ capture.ocrLines.length }} 行 · 已选 {{ capture.selectedOCRLineCount }}</template>
                  </strong>
                  <div v-if="capture.ocrLines.length" class="ocr-selection-panel">
                    <div class="ocr-selection-actions">
                      <AriButton size="sm" variant="secondary" @click="capture.selectAllOCRLines()">全选</AriButton>
                      <AriButton size="sm" variant="ghost" @click="capture.clearOCRLineSelection()">清空</AriButton>
                      <AriButton size="sm" variant="secondary" :disabled="!capture.selectedOCRLineCount" @click="capture.copySelectedOCRText()">
                        <Copy :size="14" />
                        复制选中
                      </AriButton>
                      <AriButton v-if="capture.ocrResult.text" size="sm" variant="secondary" @click="capture.copyOCRText()">
                        <Copy :size="14" />
                        复制全文
                      </AriButton>
                    </div>
                    <div class="ocr-line-list" aria-label="OCR 文本行">
                      <button
                        v-for="(line, index) in capture.ocrLines"
                        :key="`${index}-${line.text}`"
                        type="button"
                        class="ocr-line-row"
                        :class="{ 'is-selected': capture.isOCRLineSelected(index) }"
                        :aria-pressed="capture.isOCRLineSelected(index)"
                        @click="capture.toggleOCRLine(index)"
                      >
                        <span class="ocr-line-check">{{ capture.isOCRLineSelected(index) ? '已选' : '选择' }}</span>
                        <span class="ocr-line-body">
                          <span class="ocr-line-text">{{ line.text }}</span>
                          <span class="ocr-line-meta">{{ ocrConfidenceLabel(line.confidence) }} · {{ ocrRectLabel(line) }}</span>
                        </span>
                      </button>
                    </div>
                  </div>
                  <template v-else>
                    <p>{{ capture.ocrResult.text || '未识别到文字' }}</p>
                    <AriButton v-if="capture.ocrResult.text" size="sm" variant="secondary" @click="capture.copyOCRText()">
                      <Copy :size="14" />
                      复制全文
                    </AriButton>
                  </template>
                </template>
                <template v-else>
                  <strong>OCR 不可用</strong>
                  <p>{{ capture.ocrResult.error || '本地 OCR 组件不可用。' }}</p>
                </template>
              </div>

              <div class="meta-grid">
                <div class="meta-item">
                  <span>截图目录</span>
                  <strong>{{ capture.status?.imageDir || '%APPDATA%/Ariadne/capture_images' }}</strong>
                </div>
                <div v-if="capture.status?.virtualizedImageDir" class="meta-item">
                  <span>MSIX 实际目录</span>
                  <strong>{{ capture.status.virtualizedImageDir }}</strong>
                </div>
                <div class="meta-item">
                  <span>文件路径</span>
                  <strong>{{ selected.imagePath }}</strong>
                </div>
                <div class="meta-item">
                  <span>签名</span>
                  <strong>{{ selected.signature }}</strong>
                </div>
              </div>
            </template>

            <div v-else class="empty-state">
              <Camera :size="22" />
              <span>选择一张截图查看详情</span>
            </div>
          </section>
        </div>

        <footer class="status-strip">
          <span>
            <Camera :size="14" />
            截图历史已本地持久化
          </span>
          <span>
            <Pin :size="14" />
            置顶截图不会被清空未置顶删除
          </span>
          <span v-if="capture.status?.lastCaptureError" class="inline-feedback">
            {{ capture.status.lastCaptureError }}
          </span>
          <span v-else-if="capture.feedback" class="inline-feedback">{{ capture.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
