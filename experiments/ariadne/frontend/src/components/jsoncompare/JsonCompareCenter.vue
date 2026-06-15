<script setup lang="ts">
import {
  ArrowLeft,
  Braces,
  Clipboard,
  Copy,
  Database,
  FileText,
  Play,
  RefreshCw,
  Shuffle,
  Trash2,
} from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useJsonCompareStore } from '../../stores/jsonCompare'

const appShell = useAppShellStore()
const jsonCompare = useJsonCompareStore()
const leftFileInput = ref<HTMLInputElement | null>(null)
const rightFileInput = ref<HTMLInputElement | null>(null)
const dragTarget = ref<'left' | 'right' | ''>('')

const resultLabel = computed(() => {
  if (!jsonCompare.result) return '待对比'
  if (!jsonCompare.result.ok) return '解析失败'
  return jsonCompare.differenceCount ? '存在差异' : '语义一致'
})

onMounted(() => {
  if (!jsonCompare.result) {
    void jsonCompare.compare()
  }
})

function pickJsonFile(side: 'left' | 'right') {
  const input = side === 'left' ? leftFileInput.value : rightFileInput.value
  input?.click()
}

async function handleJsonFile(side: 'left' | 'right', event: Event) {
  const input = event.target as HTMLInputElement
  await jsonCompare.loadFileSide(side, input.files?.[0])
  input.value = ''
}

function handleJsonDrag(side: 'left' | 'right', event: DragEvent) {
  const transfer = event.dataTransfer
  if (transfer) {
    transfer.dropEffect = 'copy'
  }
  dragTarget.value = side
}

function clearJsonDrag(side?: 'left' | 'right') {
  if (!side || dragTarget.value === side) {
    dragTarget.value = ''
  }
}

async function handleJsonDrop(side: 'left' | 'right', event: DragEvent) {
  clearJsonDrag(side)
  const transfer = event.dataTransfer
  if (!transfer) {
    return
  }
  const file = transfer.files?.[0]
  if (file) {
    await jsonCompare.loadFileSide(side, file)
    return
  }
  const text = transfer.getData('application/json') || transfer.getData('text/plain')
  await jsonCompare.loadTextSide(side, text, '拖放内容')
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell json-compare-shell" aria-label="JSON 对比中心">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Braces :size="18" />
          </div>
          <div class="brand-copy">
            <span>JSON 对比</span>
            <small>Semantic diff, normalized lines, explicit local feedback</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="{ 'is-on': jsonCompare.result?.ok, 'is-danger': jsonCompare.result && !jsonCompare.result.ok }">
              <Database :size="13" />
              {{ resultLabel }}
            </span>
            <span v-if="jsonCompare.result?.ok" class="system-pill">
              <Braces :size="13" />
              {{ jsonCompare.differenceCount }} 处差异
            </span>
            <span v-if="jsonCompare.result?.performanceNote" class="system-pill is-warning">
              <RefreshCw :size="13" />
              性能预算
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="tool-toolbar json-compare-toolbar">
          <AriButton size="sm" variant="primary" :disabled="jsonCompare.isComparing" @click="jsonCompare.compare()">
            <Play :size="14" />
            对比
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.formatBoth()">
            <RefreshCw :size="14" />
            格式化两侧
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.pasteSide('left')">
            <Clipboard :size="14" />
            剪贴板到左侧
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.pasteSide('right')">
            <Clipboard :size="14" />
            剪贴板到右侧
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="pickJsonFile('left')">
            <FileText :size="14" />
            文件到左侧
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="pickJsonFile('right')">
            <FileText :size="14" />
            文件到右侧
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.swapSides()">
            <Shuffle :size="14" />
            交换
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.copyReport()">
            <Copy :size="14" />
            复制报告
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.loadSample()">
            <FileText :size="14" />
            示例
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="jsonCompare.clearAll()">
            <Trash2 :size="14" />
            清空
          </AriButton>
          <label class="json-sort-toggle">
            <input
              type="checkbox"
              :checked="jsonCompare.sortKeys"
              @change="jsonCompare.setSortKeys(($event.target as HTMLInputElement).checked)"
            />
            <span>规范化 key 顺序</span>
          </label>
          <input
            ref="leftFileInput"
            class="json-file-input"
            type="file"
            accept=".json,application/json,text/json,text/plain"
            @change="handleJsonFile('left', $event)"
          />
          <input
            ref="rightFileInput"
            class="json-file-input"
            type="file"
            accept=".json,application/json,text/json,text/plain"
            @change="handleJsonFile('right', $event)"
          />
        </div>

        <div class="json-compare-workspace">
          <section class="json-editor-zone" aria-label="JSON 输入">
            <div class="json-editors">
              <label
                class="json-editor-panel"
                :class="{ 'is-drop-target': dragTarget === 'left' }"
                @dragenter.prevent="handleJsonDrag('left', $event)"
                @dragover.prevent="handleJsonDrag('left', $event)"
                @dragleave="clearJsonDrag('left')"
                @drop.prevent="handleJsonDrop('left', $event)"
              >
                <span>左侧 JSON</span>
                <textarea
                  v-model="jsonCompare.leftText"
                  spellcheck="false"
                  @input="jsonCompare.compare()"
                />
              </label>
              <label
                class="json-editor-panel"
                :class="{ 'is-drop-target': dragTarget === 'right' }"
                @dragenter.prevent="handleJsonDrag('right', $event)"
                @dragover.prevent="handleJsonDrag('right', $event)"
                @dragleave="clearJsonDrag('right')"
                @drop.prevent="handleJsonDrop('right', $event)"
              >
                <span>右侧 JSON</span>
                <textarea
                  v-model="jsonCompare.rightText"
                  spellcheck="false"
                  @input="jsonCompare.compare()"
                />
              </label>
            </div>

            <div class="json-formatted-grid">
              <section>
                <div class="json-section-title">
                  <span>LEFT NORMALIZED</span>
                  <AriButton size="sm" variant="secondary" @click="jsonCompare.formatSide('left')">
                    <RefreshCw :size="13" />
                    格式化左侧
                  </AriButton>
                </div>
                <pre class="json-code-preview">{{ jsonCompare.result?.leftFormatted || '等待有效 JSON' }}</pre>
              </section>
              <section>
                <div class="json-section-title">
                  <span>RIGHT NORMALIZED</span>
                  <AriButton size="sm" variant="secondary" @click="jsonCompare.formatSide('right')">
                    <RefreshCw :size="13" />
                    格式化右侧
                  </AriButton>
                </div>
                <pre class="json-code-preview">{{ jsonCompare.result?.rightFormatted || '等待有效 JSON' }}</pre>
              </section>
            </div>
          </section>

          <aside class="json-result-panel" aria-label="JSON 对比结果">
            <div class="json-result-summary" :class="`is-${jsonCompare.resultTone}`">
              <span>{{ resultLabel }}</span>
              <strong>{{ jsonCompare.result?.summary || '输入两侧 JSON 后开始对比' }}</strong>
              <small v-if="jsonCompare.result?.error">{{ jsonCompare.result.error }}</small>
              <small v-else-if="jsonCompare.result?.performanceNote">{{ jsonCompare.result.performanceNote }}</small>
            </div>

            <div class="json-stat-grid">
              <div>
                <span>新增</span>
                <strong>{{ jsonCompare.result?.added ?? 0 }}</strong>
              </div>
              <div>
                <span>删除</span>
                <strong>{{ jsonCompare.result?.removed ?? 0 }}</strong>
              </div>
              <div>
                <span>变更</span>
                <strong>{{ jsonCompare.result?.changed ?? 0 }}</strong>
              </div>
            </div>

            <section class="json-report-block">
              <div class="json-section-title">
                <span>SEMANTIC REPORT</span>
              </div>
              <pre class="json-code-preview is-report">{{ jsonCompare.result?.report || '暂无报告' }}</pre>
            </section>

            <section class="json-report-block">
              <div class="json-section-title">
                <span>UNIFIED DIFF</span>
              </div>
              <pre class="json-code-preview is-diff">{{ jsonCompare.result?.unifiedDiff || '暂无行差异' }}</pre>
            </section>
          </aside>
        </div>

        <footer class="status-strip">
          <span>
            <Braces :size="14" />
            对象字段顺序默认不算语义差异
          </span>
          <span>
            <Copy :size="14" />
            复制反馈留在本页
          </span>
          <span v-if="jsonCompare.result?.diffTruncated || jsonCompare.result?.differencesTruncated || jsonCompare.result?.formattedTruncated">
            <RefreshCw :size="14" />
            {{ jsonCompare.result.performanceNote || '大输入已按性能预算处理' }}
          </span>
          <span v-if="jsonCompare.feedback" class="inline-feedback">{{ jsonCompare.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
