<script setup lang="ts">
import {
  AlertTriangle,
  Download,
  FileCode2,
  Plus,
  RefreshCw,
  Save,
  ShieldAlert,
  Trash2,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import HostsEditorPane from './HostsEditorPane.vue'
import { calculateHostsContentStats } from '../../lib/hostsContentStats'
import { useHostsStore } from '../../stores/hosts'

const hosts = useHostsStore()

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
const draftStats = computed(() => calculateHostsContentStats(hosts.draft?.content ?? ''))
const draftModified = computed(() => {
  if (!hosts.draft || !selected.value) return false
  return hosts.draft.title !== selected.value.title
    || hosts.draft.content !== selected.value.content
    || hosts.draft.type !== selected.value.type
    || hosts.draft.url !== selected.value.url
})
const draftBytes = computed(() => new TextEncoder().encode(hosts.draft?.content ?? '').byteLength)
const draftLocation = computed(() => {
  if (hosts.draft?.system) return hosts.status?.hostsPath || '—'
  if (hosts.draft?.type === 'remote') return hosts.draft.url || '尚未设置远程地址'
  return 'Ariadne 本地配置'
})

function formatDate(timestamp?: number) {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}

onMounted(() => {
  void hosts.load()
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
            <AriButton size="sm" variant="secondary" @click="hosts.createProfile()">
              <Plus :size="14" />新建方案
            </AriButton>
            <AriButton size="sm" variant="secondary" :disabled="!hosts.draft || hosts.draft.system || hosts.isSaving" @click="hosts.saveDraft()">
              <Save :size="14" />保存方案
            </AriButton>
            <AriButton size="sm" variant="secondary" :disabled="!selected || selected.system" @click="hosts.deleteProfile(selected)">
              <Trash2 :size="14" />{{ selected && hosts.deleteArmedId === selected.id ? '确认删除' : '删除方案' }}
            </AriButton>
            <AriButton size="sm" variant="secondary" @click="hosts.buildPreview()">
              <FileCode2 :size="14" />生成预览
            </AriButton>
            <AriButton size="sm" variant="primary" @click="hosts.applyHosts()">
              <ShieldAlert :size="14" />{{ hosts.applyArmed ? '确认写入系统 Hosts' : '应用到系统' }}
            </AriButton>
          </div>
        </header>

        <div class="hosts-workspace">
          <section class="hosts-list hosts-workbench-panel" aria-label="Hosts 方案列表">
            <div class="hosts-column-header"><strong>方案列表</strong></div>
            <div class="hosts-list-body">
              <button
                v-for="profile in hosts.profiles"
                :key="profile.id"
                class="hosts-row"
                :class="{ 'is-selected': profile.id === hosts.selectedId }"
                @click="hosts.select(profile.id)"
              >
                <span class="hosts-row-main"><span class="hosts-row-title">{{ profile.title }}</span></span>
                <span
                  v-if="!profile.system"
                  class="hosts-switch"
                  :class="{ 'is-on': profile.enabled }"
                  @click.stop="hosts.toggleEnabled(profile)"
                ><span /></span>
              </button>
            </div>
            <footer class="hosts-list-footer">
              <span>已启用 {{ hosts.enabledCount }} / {{ Math.max(0, hosts.profiles.filter((profile) => !profile.system).length) }}</span>
              <AriButton size="sm" variant="ghost" :disabled="hosts.isLoading" @click="hosts.load()">
                <RefreshCw :size="14" />刷新
              </AriButton>
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
              <AriButton
                v-if="hosts.draft && !hosts.draft.system && hosts.draft.type === 'remote'"
                class="hosts-remote-fetch"
                size="sm"
                variant="secondary"
                :disabled="hosts.isSaving || !(hosts.draft.url ?? '').trim()"
                @click="hosts.fetchRemote(hosts.draft)"
              >
                <Download :size="14" />拉取远程
              </AriButton>
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
              <div v-if="hosts.preview?.conflicts.length" class="hosts-conflicts">
                <span class="side-title"><AlertTriangle :size="14" />冲突域名</span>
                <div v-for="conflict in hosts.preview.conflicts.slice(0, 8)" :key="conflict.host">
                  <strong>{{ conflict.host }}</strong><small>{{ conflict.ips.join(', ') }}</small>
                </div>
              </div>
              <AriButton size="sm" variant="secondary" @click="hosts.buildPreview()">
                <ShieldAlert :size="14" />查看冲突详情
              </AriButton>
            </section>

            <p class="hosts-preview-note">写入系统 Hosts 需要再次确认，并由 Windows UAC 授权。</p>
          </aside>
        </div>

        <footer class="status-strip hosts-status-strip">
          <span>方案保存在 Ariadne 本地配置</span>
          <span v-if="hosts.feedback" class="inline-feedback">{{ hosts.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
