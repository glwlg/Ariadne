<script setup lang="ts">
import {
  AlertTriangle,
  Download,
  FileCode2,
  Plus,
  RefreshCw,
  RotateCcw,
  Save,
  ShieldAlert,
  Trash2,
} from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import FancunButton from '../ui/FancunButton.vue'
import HostsEditorPane from './HostsEditorPane.vue'
import { calculateHostsContentStats, type HostsContentStats } from '../../lib/hostsContentStats'
import { useHostsStore } from '../../stores/hosts'

const hosts = useHostsStore()

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

const showFinalPreview = ref(false)
const draftStats = ref<HostsContentStats>(calculateHostsContentStats(''))
const draftBytes = ref(0)
let statsTimer: number | undefined
let lastProfileIdForStats = ''

function refreshDraftMetrics(content: string) {
  draftStats.value = calculateHostsContentStats(content)
  draftBytes.value = new TextEncoder().encode(content).byteLength
}

watch(
  () => [hosts.draft?.id ?? '', hosts.draft?.content ?? ''] as const,
  ([profileId, content]) => {
    if (statsTimer !== undefined) {
      window.clearTimeout(statsTimer)
      statsTimer = undefined
    }
    if (profileId !== lastProfileIdForStats) {
      lastProfileIdForStats = profileId
      refreshDraftMetrics(content)
      return
    }
    statsTimer = window.setTimeout(() => {
      statsTimer = undefined
      refreshDraftMetrics(content)
    }, 180)
  },
  { immediate: true },
)

const selected = computed(() => hosts.selectedProfile)
const hostsStatusLabel = computed(() => {
  const conflicts = hosts.preview?.conflicts.length ?? 0
  return conflicts > 0
    ? `${hosts.enabledCount} 个方案启用 · ${conflicts} 处冲突`
    : `${hosts.enabledCount} 个方案启用 · 无冲突`
})
const draftKindLabel = computed(() => {
  if (!hosts.draft) return '—'
  if (hosts.draft.system) return '系统'
  if (hosts.draft.type === 'remote') return '远程'
  return '本地'
})
const draftModified = computed(() => hosts.draftModified)
const draftLocation = computed(() => {
  if (hosts.draft?.system) return hosts.status?.hostsPath || '—'
  if (hosts.draft?.type === 'remote') return hosts.draft.url || '尚未设置远程地址'
  return 'Fancun 本地配置'
})

function formatDate(timestamp?: number) {
  if (!timestamp) return '—'
  return dateFormatter.format(new Date(timestamp * 1000))
}

function formatBackupTime(iso?: string) {
  if (!iso) return ''
  const parsed = new Date(iso)
  if (Number.isNaN(parsed.getTime())) return iso
  return dateFormatter.format(parsed)
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}

onMounted(() => {
  void hosts.load()
})

onBeforeUnmount(() => {
  if (statsTimer !== undefined) {
    window.clearTimeout(statsTimer)
    statsTimer = undefined
  }
})
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell hosts-shell is-design-surface" aria-label="Hosts 管理中心">
        <header class="hosts-page-header">
          <div class="hosts-page-title">
            <h1>Hosts 管理</h1>
            <p>{{ hostsStatusLabel }}</p>
          </div>
          <div class="hosts-page-actions">
            <FancunButton size="sm" variant="secondary" @click="hosts.createProfile()">
              <Plus :size="14" />新建方案
            </FancunButton>
            <FancunButton size="sm" variant="secondary" :disabled="!hosts.draft || hosts.draft.system || hosts.isSaving" @click="hosts.saveDraft()">
              <Save :size="14" />{{ hosts.isSaving ? '保存中...' : '保存方案' }}
            </FancunButton>
            <FancunButton
              v-if="hosts.draftModified && hosts.draft && !hosts.draft.system"
              size="sm"
              variant="ghost"
              :disabled="hosts.isSaving"
              @click="hosts.discardDraft()"
            >
              放弃修改
            </FancunButton>
            <FancunButton
              size="sm"
              variant="secondary"
              :class="{ 'is-armed-danger': Boolean(selected && hosts.deleteArmedId === selected.id) }"
              :disabled="!selected || selected.system || hosts.isSaving"
              @click="hosts.deleteProfile(selected)"
            >
              <Trash2 :size="14" />{{ selected && hosts.deleteArmedId === selected.id ? '确认删除' : '删除方案' }}
            </FancunButton>
            <FancunButton size="sm" variant="secondary" :disabled="hosts.isSaving || hosts.isApplying" @click="hosts.buildPreview()">
              <FileCode2 :size="14" />生成预览
            </FancunButton>
            <FancunButton
              size="sm"
              variant="secondary"
              :class="{ 'is-armed-danger': hosts.rollbackArmed }"
              :disabled="!hosts.canRollback || hosts.isRollingBack || hosts.isApplying"
              :title="hosts.lastBackupAt ? `最近备份：${formatBackupTime(hosts.lastBackupAt)}` : '暂无可回滚的写入备份'"
              @click="hosts.rollbackHosts()"
            >
              <RotateCcw :size="14" />{{ hosts.isRollingBack ? '回滚中...' : hosts.rollbackArmed ? '确认回滚系统 Hosts' : '回滚上次写入' }}
            </FancunButton>
            <FancunButton
              size="sm"
              variant="primary"
              :class="{ 'is-armed-danger': hosts.applyArmed }"
              :disabled="hosts.isApplying || hosts.isRollingBack"
              @click="hosts.applyHosts()"
            >
              <ShieldAlert :size="14" />{{ hosts.isApplying ? '写入中...' : hosts.applyArmed ? '确认写入系统 Hosts' : '应用到系统' }}
            </FancunButton>
          </div>
        </header>

        <div class="hosts-workspace">
          <section class="hosts-list hosts-workbench-panel" aria-label="Hosts 方案列表">
            <div class="hosts-column-header"><strong>方案列表</strong></div>
            <div class="hosts-list-body">
              <div v-if="hosts.isLoading && !hosts.profiles.length" class="tool-loading-state">
                <strong>正在加载 Hosts 方案...</strong>
                <small>读取本地规则配置与系统 Hosts 文件</small>
              </div>
              <div v-else-if="hosts.loadError && !hosts.profiles.length" class="tool-error-state">
                <strong>加载 Hosts 失败</strong>
                <small>{{ hosts.loadError }}</small>
                <FancunButton size="sm" variant="secondary" @click="hosts.load()">重试加载</FancunButton>
              </div>
              <div v-else-if="!hosts.profiles.length" class="tool-empty-state">
                <strong>暂无 Hosts 方案</strong>
                <small>点击右上角“新建方案”创建本地或远程规则</small>
              </div>
              <div
                v-for="profile in hosts.profiles"
                v-else
                :key="profile.id"
                class="hosts-row"
                :class="{ 'is-selected': profile.id === hosts.selectedId }"
                @click="hosts.select(profile.id)"
              >
                <button
                  type="button"
                  class="hosts-row-main border-0 bg-transparent p-0 text-left cursor-pointer"
                  @click.stop="hosts.select(profile.id)"
                >
                  <span class="hosts-row-title">{{ profile.title }}</span>
                </button>
                <button
                  v-if="!profile.system"
                  type="button"
                  role="switch"
                  :aria-checked="profile.enabled"
                  :aria-label="`${profile.enabled ? '停用' : '启用'} ${profile.title}`"
                  class="hosts-switch-btn"
                  :class="{ 'is-on': profile.enabled }"
                  @click.stop="hosts.toggleEnabled(profile)"
                >
                  <span />
                </button>
              </div>
            </div>
            <footer class="hosts-list-footer">
              <span>已启用 {{ hosts.enabledCount }} / {{ Math.max(0, hosts.profiles.filter((profile) => !profile.system).length) }}</span>
              <FancunButton size="sm" variant="ghost" :disabled="hosts.isLoading" @click="hosts.load()">
                <RefreshCw :size="14" />刷新
              </FancunButton>
            </footer>
          </section>

          <section class="hosts-editor-column hosts-workbench-panel" aria-label="Hosts 方案编辑">
            <template v-if="hosts.draft">
              <HostsEditorPane
                :title="hosts.draft.title"
                :content="hosts.draft.content"
                :readonly="hosts.draft.system || hosts.draft.type === 'remote'"
                :modified="draftModified"
                @update="hosts.updateDraft({ content: $event })"
              />
            </template>
            <div v-else-if="hosts.isLoading" class="tool-loading-state">
              <strong>正在载入编辑器...</strong>
              <small>加载当前方案内容</small>
            </div>
            <div v-else-if="hosts.loadError" class="tool-error-state">
              <strong>无法载入方案内容</strong>
              <small>{{ hosts.loadError }}</small>
              <FancunButton size="sm" variant="secondary" @click="hosts.load()">重试加载</FancunButton>
            </div>
            <div v-else class="tool-empty-state">
              <strong>未选择 Hosts 方案</strong>
              <small>从左侧选择一个方案进行查看或编辑</small>
            </div>
          </section>

          <aside class="hosts-preview hosts-workbench-panel" aria-label="Hosts 方案信息">
            <section class="hosts-inspector-section">
              <div class="hosts-column-header"><strong>方案信息</strong></div>
              <div class="hosts-property-list">
                <div class="hosts-property-row hosts-property-control-row">
                  <span>方案名称</span>
                  <input
                    v-if="hosts.draft && !hosts.draft.system"
                    class="hosts-property-input"
                    aria-label="方案名称"
                    :value="hosts.draft.title"
                    @input="hosts.updateDraft({ title: ($event.target as HTMLInputElement).value })"
                  />
                  <strong v-else>{{ hosts.draft?.title || '—' }}</strong>
                </div>
                <div class="hosts-property-row"><span>启用状态</span><strong>{{ hosts.draft?.system ? '只读' : hosts.draft?.enabled ? '已启用' : '已停用' }}</strong></div>
                <div class="hosts-property-row hosts-property-control-row">
                  <span>类型</span>
                  <select
                    v-if="hosts.draft && !hosts.draft.system"
                    class="hosts-property-select"
                    aria-label="方案类型"
                    :value="hosts.draft.type"
                    @change="hosts.updateDraft({ type: ($event.target as HTMLSelectElement).value === 'remote' ? 'remote' : 'local' })"
                  >
                    <option value="local">本地</option>
                    <option value="remote">远程</option>
                  </select>
                  <strong v-else>{{ draftKindLabel }}</strong>
                </div>
                <label v-if="hosts.draft && !hosts.draft.system && hosts.draft.type === 'remote'" class="hosts-property-row hosts-property-control-row hosts-property-url-row">
                  <span>远程 URL</span>
                  <input
                    class="hosts-property-input"
                    aria-label="远程 URL"
                    :value="hosts.draft.url"
                    placeholder="https://example.com/hosts.txt"
                    @input="hosts.updateDraft({ url: ($event.target as HTMLInputElement).value })"
                  />
                </label>
                <div class="hosts-property-row"><span>更新时间</span><strong>{{ formatDate(hosts.draft?.updatedAt) }}</strong></div>
                <div class="hosts-property-row"><span>位置</span><strong :title="draftLocation">{{ draftLocation }}</strong></div>
                <div class="hosts-property-row"><span>内容大小</span><strong>{{ formatBytes(draftBytes) }}</strong></div>
              </div>
              <FancunButton
                v-if="hosts.draft && !hosts.draft.system && hosts.draft.type === 'remote'"
                class="hosts-remote-fetch"
                size="sm"
                variant="secondary"
                :disabled="hosts.isSaving || hosts.isFetching || !(hosts.draft.url ?? '').trim()"
                @click="hosts.fetchRemote(hosts.draft)"
              >
                <Download :size="14" />{{ hosts.isFetching ? '拉取中...' : '拉取远程' }}
              </FancunButton>
            </section>

            <section class="hosts-inspector-section hosts-statistics-section">
              <div class="hosts-column-header"><strong>统计信息</strong></div>
              <div class="hosts-property-list">
                <div class="hosts-property-row"><span>总行数</span><strong>{{ draftStats.totalLines }}</strong></div>
                <div class="hosts-property-row"><span>有效规则</span><strong>{{ draftStats.validRules }}</strong></div>
                <div class="hosts-property-row"><span>注释行</span><strong>{{ draftStats.commentLines }}</strong></div>
                <div class="hosts-property-row"><span>空行</span><strong>{{ draftStats.emptyLines }}</strong></div>
                <div class="hosts-property-row"><span>重复规则</span><strong>{{ draftStats.duplicateRules }}</strong></div>
              </div>
              <FancunButton size="sm" variant="secondary" @click="hosts.buildPreview()">
                <ShieldAlert :size="14" />{{ hosts.preview ? '刷新应用预览' : '查看冲突详情' }}
              </FancunButton>
            </section>

            <section v-if="hosts.preview" class="hosts-inspector-section p-4 grid gap-3" aria-label="写入预览与差异">
              <div class="flex items-center justify-between gap-2">
                <strong class="text-[13px] font-semibold">写入预览</strong>
                <FancunButton size="sm" variant="ghost" @click="showFinalPreview = !showFinalPreview">
                  {{ showFinalPreview ? '查看差异 Diff' : '查看合并后全文' }}
                </FancunButton>
              </div>
              <div class="hosts-preview-summary">
                <div><span>新增行</span><strong>+{{ hosts.preview.addedLines }}</strong></div>
                <div><span>移除行</span><strong>-{{ hosts.preview.removedLines }}</strong></div>
                <div><span>总行数</span><strong>{{ hosts.preview.lineCount }}</strong></div>
              </div>
              <div class="text-xs text-[var(--muted)] leading-relaxed">
                已纳入方案：{{ hosts.preview.enabledProfiles.length ? hosts.preview.enabledProfiles.join('、') : '仅系统基础内容' }}
              </div>
              <div v-if="hosts.preview.conflicts.length" class="hosts-conflicts" style="margin: 0">
                <span class="side-title"><AlertTriangle :size="14" />冲突域名（{{ hosts.preview.conflicts.length }}）</span>
                <div v-for="conflict in hosts.preview.conflicts" :key="conflict.host">
                  <strong>{{ conflict.host }}</strong><small>{{ conflict.ips.join(', ') }}</small>
                </div>
              </div>
              <pre class="hosts-diff">{{ showFinalPreview ? (hosts.preview.finalContent || '（空文件）') : (hosts.preview.diffText || (hosts.preview.changed ? '（无文本差异）' : '与当前系统 Hosts 一致，无变更')) }}</pre>
            </section>

            <p class="hosts-preview-note">
              写入系统 Hosts 需要再次确认，并由 Windows UAC 授权。
              <template v-if="hosts.lastBackupAt"> 最近备份：{{ formatBackupTime(hosts.lastBackupAt) }}。</template>
            </p>
          </aside>
        </div>

        <footer class="status-strip hosts-status-strip">
          <span>方案保存在 Fancun 本地配置</span>
          <span
            v-if="hosts.feedback"
            class="inline-feedback"
            :class="{ 'is-danger': hosts.feedbackTone === 'danger' }"
          >{{ hosts.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
