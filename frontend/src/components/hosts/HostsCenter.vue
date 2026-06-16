<script setup lang="ts">
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  Download,
  FileCode2,
  Globe2,
  Plus,
  Save,
  Server,
  ShieldAlert,
  Trash2,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useHostsStore } from '../../stores/hosts'

const appShell = useAppShellStore()
const hosts = useHostsStore()

const selected = computed(() => hosts.selectedProfile)

onMounted(() => {
  void hosts.load()
})

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(1)} KB`
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell hosts-shell" aria-label="Hosts 管理中心">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Server :size="18" />
          </div>
          <div class="brand-copy">
            <span>Hosts 管理</span>
            <small>Profiles, conflict preview, guarded system write</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="hosts.enabledCount ? 'is-on' : ''">
              <CheckCircle2 :size="13" />
              启用 {{ hosts.enabledCount }}
            </span>
            <span class="system-pill" :class="hosts.preview?.conflicts.length ? 'is-danger' : ''">
              <AlertTriangle :size="13" />
              冲突 {{ hosts.preview?.conflicts.length ?? 0 }}
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="tool-toolbar hosts-toolbar">
          <AriButton size="sm" variant="primary" @click="hosts.createProfile()">
            <Plus :size="14" />
            新建方案
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="!hosts.draft || hosts.draft.system || hosts.isSaving" @click="hosts.saveDraft()">
            <Save :size="14" />
            保存方案
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="!selected || selected.system" @click="hosts.deleteProfile(selected)">
            <Trash2 :size="14" />
            {{ selected && hosts.deleteArmedId === selected.id ? '确认删除' : '删除方案' }}
          </AriButton>
          <div class="hosts-toolbar-spacer" />
          <AriButton size="sm" variant="secondary" @click="hosts.buildPreview()">
            <FileCode2 :size="14" />
            生成预览
          </AriButton>
          <AriButton size="sm" variant="primary" @click="hosts.applyHosts()">
            <ShieldAlert :size="14" />
            {{ hosts.applyArmed ? '确认写入系统 Hosts' : '应用到系统' }}
          </AriButton>
        </div>

        <div class="hosts-workspace">
          <section class="hosts-list" aria-label="Hosts 方案列表">
            <button
              v-for="profile in hosts.profiles"
              :key="profile.id"
              class="hosts-row"
              :class="{ 'is-selected': profile.id === hosts.selectedId }"
              @click="hosts.select(profile.id)"
            >
              <span class="hosts-row-icon" :class="{ 'is-on': profile.enabled, 'is-system': profile.system }">
                <Server v-if="profile.system" :size="15" />
                <Globe2 v-else-if="profile.type === 'remote'" :size="15" />
                <FileCode2 v-else :size="15" />
              </span>
              <span class="hosts-row-main">
                <span class="hosts-row-title">{{ profile.title }}</span>
                <span class="hosts-row-meta">
                  {{ profile.system ? '系统原始 Hosts' : profile.type === 'remote' ? '远程拉取' : '本地模式' }}
                  · {{ profile.enabled ? '启用' : '停用' }}
                </span>
              </span>
              <span v-if="!profile.system" class="hosts-switch" :class="{ 'is-on': profile.enabled }" @click.stop="hosts.toggleEnabled(profile)">
                <span />
              </span>
            </button>
          </section>

          <section class="hosts-editor" aria-label="Hosts 方案编辑">
            <template v-if="hosts.draft">
              <div class="hosts-editor-header">
                <div>
                  <span class="preview-kicker">{{ hosts.draft.system ? 'SYSTEM' : hosts.draft.type }}</span>
                  <input
                    class="hosts-title-input"
                    :readonly="hosts.draft.system"
                    :value="hosts.draft.title"
                    @input="hosts.updateDraft({ title: ($event.target as HTMLInputElement).value })"
                  />
                </div>
                <span class="system-pill" :class="hosts.draft.enabled ? 'is-on' : ''">
                  {{ hosts.draft.system ? '只读' : hosts.draft.enabled ? '启用' : '停用' }}
                </span>
              </div>

              <div v-if="!hosts.draft.system" class="hosts-form-row">
                <label class="settings-field">
                  <span>方案类型</span>
                  <select
                    class="settings-select"
                    :value="hosts.draft.type"
                    @change="hosts.updateDraft({ type: ($event.target as HTMLSelectElement).value === 'remote' ? 'remote' : 'local' })"
                  >
                    <option value="local">本地模式</option>
                    <option value="remote">远程拉取</option>
                  </select>
                </label>
                <label class="settings-field">
                  <span>远程 URL</span>
                  <input
                    class="settings-input"
                    :disabled="hosts.draft.type !== 'remote'"
                    :value="hosts.draft.url"
                    placeholder="https://example.com/hosts.txt"
                    @input="hosts.updateDraft({ url: ($event.target as HTMLInputElement).value })"
                  />
                </label>
                <AriButton size="sm" variant="secondary" :disabled="hosts.draft.type !== 'remote'" @click="hosts.fetchRemote(selected)">
                  <Download :size="14" />
                  拉取远程
                </AriButton>
              </div>

              <textarea
                class="hosts-code"
                spellcheck="false"
                :readonly="hosts.draft.system || hosts.draft.type === 'remote'"
                :value="hosts.draft.content"
                @input="hosts.updateDraft({ content: ($event.target as HTMLTextAreaElement).value })"
              />
            </template>
          </section>

          <aside class="hosts-preview" aria-label="Hosts 应用预览">
            <div class="side-panel">
              <span class="side-title">
                <ShieldAlert :size="14" />
                写入边界
              </span>
              <p>写入系统 Hosts 需要再次确认，并由 Windows UAC 授权；预览不会改动系统文件。</p>
              <small>{{ hosts.status?.hostsPath }}</small>
            </div>

            <div v-if="hosts.status?.virtualizedExists" class="side-panel">
              <span class="side-title">MSIX 实际路径</span>
              <strong>{{ hosts.status.virtualizedPath }}</strong>
              <small>{{ hosts.status.virtualizedBytes }} bytes</small>
            </div>

            <div class="hosts-preview-summary">
              <div>
                <span>系统 Hosts</span>
                <strong>{{ hosts.status?.systemReadable ? formatBytes(hosts.status.systemBytes) : '不可读' }}</strong>
              </div>
              <div>
                <span>最终行数</span>
                <strong>{{ hosts.preview?.lineCount ?? 0 }}</strong>
              </div>
              <div>
                <span>新增/移除</span>
                <strong>+{{ hosts.preview?.addedLines ?? 0 }} / -{{ hosts.preview?.removedLines ?? 0 }}</strong>
              </div>
            </div>

            <div v-if="hosts.preview?.conflicts.length" class="hosts-conflicts">
              <span class="side-title">
                <AlertTriangle :size="14" />
                冲突域名
              </span>
              <div v-for="conflict in hosts.preview.conflicts.slice(0, 8)" :key="conflict.host">
                <strong>{{ conflict.host }}</strong>
                <small>{{ conflict.ips.join(', ') }}</small>
              </div>
            </div>

            <pre class="hosts-diff">{{ hosts.preview?.diffText || '生成预览后显示差异、冲突和最终写入内容。' }}</pre>
          </aside>
        </div>

        <footer class="status-strip">
          <span>
            <Server :size="14" />
            方案保存在 Ariadne 本地配置
          </span>
          <span>
            <ShieldAlert :size="14" />
            系统写入必须二次确认
          </span>
          <span v-if="hosts.feedback" class="inline-feedback">{{ hosts.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
