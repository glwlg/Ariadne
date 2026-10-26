<script setup lang="ts">
import {
  AlertTriangle,
  Camera,
  Database,
  Download,
  HardDrive,
  KeyRound,
  Plus,
  Puzzle,
  RotateCcw,
  Rocket,
  Save,
  Search,
  Settings,
  Shield,
  Sparkles,
  Trash2,
  Upload,
} from '@lucide/vue'
import { computed, nextTick, onMounted, ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import OCRSettingsPage from './OCRSettingsPage.vue'
import HistorySettingsPage from './HistorySettingsPage.vue'
import SearchSettingsPage from './SearchSettingsPage.vue'
import { useSettingsStore } from '../../stores/settings'
import type { HotkeySettings, Launcher } from '../../types/ariadne'

const settings = useSettingsStore()
const settingsContentRef = ref<HTMLElement | null>(null)
const storageHealthy = computed(() => {
  const status = settings.storageStatus
  return Boolean(status?.exists && status.readBackOk && !status.lastSaveError && !status.readBackError)
})
const platformCapabilityCounts = computed(() => {
  const capabilities = settings.platformStatus?.capabilities ?? []
  return {
    enabled: capabilities.filter((capability) => capability.enabled).length,
    pending: capabilities.filter((capability) => !capability.enabled).length,
  }
})
const legacyRuntime = computed(() => settings.platformStatus?.legacyRuntime)
const searchPerformance = computed(() => settings.platformStatus?.searchPerformance)
const platformLogs = computed(() => settings.platformStatus?.logs)
const shellRuntime = computed(() => settings.platformStatus?.shell)
const legacyDataSources = computed(() => settings.legacyDataStatus?.sources ?? [])
const releaseDataRoots = computed(() => settings.releaseBackupStatus?.dataRoots ?? [])
const legacyConfigNeedsAttention = computed(() => Boolean(settings.legacyStatus?.exists && settings.legacyStatus.needsImport))
const legacyHistoryNeedsAttention = computed(() => Boolean(settings.legacyDataStatus?.needsImport || settings.legacyImportResult))
const legacyRuntimeNeedsAttention = computed(() => Boolean(legacyRuntime.value?.processRunning || legacyRuntime.value?.hotkeyConflictLikely || settings.legacyHandoffResult))
const showLegacyPanel = computed(() => legacyConfigNeedsAttention.value || legacyHistoryNeedsAttention.value || legacyRuntimeNeedsAttention.value)
const legacyHistorySummary = computed(() => {
  const status = settings.legacyDataStatus
  if (!status) return '未加载'
  if (status.totalCount > 0 && !status.needsImport) return '已迁移'
  if (status.totalCount > 0) return `${status.totalCount} 条 / ${formatBytes(status.totalBytes)}`
  return status.exists ? '未发现历史记录' : '未发现旧目录'
})
const releaseBackupSummary = computed(() => {
  const status = settings.releaseBackupStatus
  if (!status) return '未加载'
  if (status.backupCount > 0) return `${status.backupCount} 个 / ${formatBytes(status.backupBytes)}`
  return '暂无检查点'
})
const appUpdateSummary = computed(() => {
  const status = settings.appUpdateStatus
  if (!status) return '未加载'
  if (!status.enabled) return '开发构建未启用'
  if (status.installerLaunched) return '安装器已启动'
  if (status.updateAvailable) return `可更新至 ${status.availableVersion || '新版本'}`
  if (status.state === 'up-to-date') return '已是最新版本'
  return '可以检查更新'
})
const launcherKindOptions = [
  { value: 'app', label: '应用' },
  { value: 'file', label: '文件' },
  { value: 'folder', label: '文件夹' },
  { value: 'url', label: 'URL' },
  { value: 'command', label: '命令' },
]

function formatBytes(value?: number) {
  if (!value) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function formatMs(value?: number) {
  if (!value) return '0ms'
  return `${value}ms`
}

function launcherKindLabel(kind: string) {
  return launcherKindOptions.find((item) => item.value === kind)?.label ?? '启动项'
}

function legacySourceLabel(source: string) {
  const labels: Record<string, string> = {
    clipboard_history: '剪贴板历史',
    capture_history: '截图历史',
  }
  return labels[source] ?? source
}

function releaseRootLabel(kind: string) {
  const labels: Record<string, string> = {
    roaming: '标准数据目录',
    virtualized: '虚拟化数据目录',
    missing: '未发现目录',
  }
  return labels[kind] ?? kind
}

function pluginCapabilitiesLabel(capabilities: string[]) {
  return capabilities.length ? capabilities.join(' / ') : '本地命令'
}





type HotkeyKey = keyof HotkeySettings
type SettingsPageId = 'general' | 'hotkeys' | 'plugins' | 'launchers' | 'screenshot' | 'ocr' | 'search' | 'data' | 'advanced'

const activeSettingsPage = ref<SettingsPageId>('general')
const settingsPages: Array<{
  id: SettingsPageId
  label: string
  detail: string
  icon: typeof Settings
}> = [
  { id: 'general', label: '通用', detail: '主题、语言、启动', icon: Settings },
  { id: 'hotkeys', label: '快捷键', detail: '主窗口、截图、贴图', icon: KeyRound },
  { id: 'plugins', label: '插件', detail: '命令与能力开关', icon: Puzzle },
  { id: 'launchers', label: '启动项', detail: '应用、文件、URL', icon: Rocket },
  { id: 'screenshot', label: '截图', detail: '复制、贴图、保存', icon: Camera },
  { id: 'ocr', label: 'OCR 模型', detail: '截图、贴图、接口密钥', icon: Camera },
  { id: 'search', label: '搜索', detail: '索引、排除规则', icon: Search },
  { id: 'data', label: '数据与存储', detail: '配置、检查点', icon: Database },
  { id: 'advanced', label: '高级维护', detail: '诊断、导入、回滚', icon: HardDrive },
]

const activeSettingsPageInfo = computed(() => settingsPages.find((page) => page.id === activeSettingsPage.value) ?? settingsPages[0])

function setSettingsPage(id: SettingsPageId) {
  activeSettingsPage.value = id
  void nextTick(() => {
    settingsContentRef.value?.scrollTo({ top: 0, left: 0 })
  })
}

function selectLauncher(launcher: Launcher) {
  settings.editLauncher(launcher)
  settings.showFeedback(`已选择启动项：${launcher.name || '未命名'}`)
  void nextTick(() => {
    settingsContentRef.value?.scrollTo({ top: 0, left: 0 })
  })
}

function captureHotkey(key: HotkeyKey, event: KeyboardEvent) {
  if (event.key === 'Tab') return
  const value = formatHotkeyEvent(event)
  if (!value) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  settings.stageHotkey(key, value)
}

function selectHotkeyInput(event: FocusEvent | MouseEvent) {
  const target = event.target as HTMLInputElement | null
  target?.select()
}

function formatHotkeyEvent(event: KeyboardEvent) {
  if (event.isComposing || event.repeat) return ''
  const key = normalizeHotkeyKey(event.key)
  if (!key) return ''
  const hasModifier = event.ctrlKey || event.altKey || event.shiftKey || event.metaKey
  if (!hasModifier && !isBareFunctionHotkey(key)) return ''
  const parts: string[] = []
  if (event.ctrlKey) parts.push('ctrl')
  if (event.altKey) parts.push('alt')
  if (event.shiftKey) parts.push('shift')
  if (event.metaKey) parts.push('win')
  parts.push(key)
  return parts.join('+')
}

function normalizeHotkeyKey(key: string) {
  const normalized = key.trim().toLowerCase()
  if (!normalized) return ''
  if (['control', 'ctrl', 'alt', 'shift', 'meta', 'os', 'win', 'windows'].includes(normalized)) return ''
  if (normalized.length === 1) return normalized
  if (/^f([1-9]|1[0-9]|2[0-4])$/.test(normalized)) return normalized
  const labels: Record<string, string> = {
    ' ': 'space',
    spacebar: 'space',
    escape: 'escape',
    esc: 'escape',
    return: 'enter',
    enter: 'enter',
    tab: 'tab',
    backspace: 'backspace',
    delete: 'delete',
    del: 'delete',
  }
  return labels[normalized] ?? ''
}

function isBareFunctionHotkey(key: string) {
  return /^f([1-9]|1[0-9]|2[0-4])$/.test(key)
}

onMounted(() => {
  if (!settings.hasSettings) {
    void settings.load()
  }
})
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell settings-shell is-design-surface" aria-label="Ariadne settings center">
        <div v-if="settings.isLoading" class="empty-state">
          <Sparkles :size="22" />
          <span>正在读取设置</span>
        </div>

        <div v-else-if="settings.settings" class="settings-workspace">
          <aside class="settings-rail" aria-label="设置摘要">
            <section class="settings-summary">
              <div class="settings-summary-header">
                <h1>设置</h1>
              </div>

              <nav class="settings-nav" aria-label="设置分类">
                <button
                  v-for="page in settingsPages"
                  :key="page.id"
                  class="settings-nav-item"
                  :class="{ 'is-active': activeSettingsPage === page.id }"
                  @click="setSettingsPage(page.id)"
                >
                  <component :is="page.icon" :size="16" />
                  <strong>{{ page.label }}</strong>
                </button>
              </nav>
            </section>

            <details class="settings-panel settings-disclosure">
              <summary class="settings-panel-title">
                <Database :size="15" />
                配置存储
              </summary>
              <p class="settings-note">{{ settings.storageStatus?.path }}</p>
              <p class="settings-note">
                目录 {{ settings.storageStatus?.directoryExists ? '存在' : '不存在' }} ·
                {{ settings.storageStatus?.bytes ?? 0 }} bytes
              </p>
              <p class="settings-note">
                读回 {{ settings.storageStatus?.readBackOk ? '正常' : '失败' }} ·
                {{ settings.storageStatus?.readBackBytes ?? 0 }} bytes
                <span v-if="settings.storageStatus?.readBackVersion">
                  · v{{ settings.storageStatus.readBackVersion }}
                </span>
              </p>
              <p v-if="settings.storageStatus?.lastSaveError" class="settings-note is-danger">
                {{ settings.storageStatus.lastSaveError }}
              </p>
              <p v-if="settings.storageStatus?.readBackError" class="settings-note is-danger">
                {{ settings.storageStatus.readBackError }}
              </p>
              <p v-if="settings.storageStatus?.virtualizedExists" class="settings-note is-warning">
                MSIX 实际路径 {{ settings.storageStatus.virtualizedPath }}
                · {{ settings.storageStatus.virtualizedBytes }} bytes
              </p>
              <p class="settings-note">APPDATA {{ settings.storageStatus?.appDataEnv || '未设置' }}</p>
              <p class="settings-note">LOCALAPPDATA {{ settings.storageStatus?.localAppDataEnv || '未设置' }}</p>
              <p class="settings-note">运行目录 {{ settings.storageStatus?.workingDir || '-' }}</p>
            </details>

            <details class="settings-panel settings-disclosure">
              <summary class="settings-panel-title">
                <HardDrive :size="15" />
                平台诊断
              </summary>
              <p class="settings-note">
                {{ settings.platformStatus?.diagnostics.os }}/{{ settings.platformStatus?.diagnostics.arch }}
                · {{ settings.platformStatus?.diagnostics.goVersion || 'Go runtime' }}
              </p>
              <p class="settings-note">
                exe {{ formatBytes(settings.platformStatus?.diagnostics.executableBytes) }}
                · pid {{ settings.platformStatus?.diagnostics.processId || '-' }}
              </p>
              <p class="settings-note">
                能力 {{ platformCapabilityCounts.enabled }} 已接入 · {{ platformCapabilityCounts.pending }} 待接入
              </p>
              <p class="settings-note" :class="{ 'is-warning': searchPerformance && !searchPerformance.withinTarget }">
                搜索 p95 {{ formatMs(searchPerformance?.p95Ms) }}
                · 目标 {{ formatMs(searchPerformance?.targetP95Ms) }}
                · 样本 {{ searchPerformance?.sampleCount ?? 0 }}
              </p>
              <p v-if="searchPerformance?.lastQuery" class="settings-note">
                最近搜索 {{ searchPerformance.lastQuery }}
                · {{ formatMs(searchPerformance.lastElapsedMs) }}
                · {{ searchPerformance.lastResultCount }} 项
              </p>
              <p class="settings-note">
                Wails {{ settings.platformStatus?.diagnostics.wailsToolPath || '当前 PATH 未暴露' }}
              </p>
              <p
                class="settings-note"
                :class="{
                  'is-warning': Boolean(shellRuntime?.autostartEnabled && !shellRuntime?.autostartCommandValid),
                }"
              >
                开机启动
                {{
                  shellRuntime?.autostartEnabled
                    ? shellRuntime.autostartCommandValid
                      ? '已验证隐藏启动'
                      : '需检查'
                    : shellRuntime?.autostartSupported
                      ? '未启用'
                      : '不可用'
                }}
              </p>
              <p v-if="shellRuntime?.autostartPath" class="settings-note">
                {{ shellRuntime.autostartPath }}
              </p>
              <p v-if="shellRuntime?.autostartCommand" class="settings-note">
                {{ shellRuntime.autostartCommand }}
              </p>
              <p v-for="note in shellRuntime?.autostartNotes ?? []" :key="note" class="settings-note is-warning">
                {{ note }}
              </p>
              <p class="settings-note" :class="{ 'is-danger': Boolean(platformLogs?.lastError) }">
                日志 {{ platformLogs?.exists ? formatBytes(platformLogs.bytes) : platformLogs?.directoryExists ? '待写入' : '目录未创建' }}
              </p>
              <p class="settings-note">{{ platformLogs?.path || '日志路径未初始化' }}</p>
              <p v-if="platformLogs?.lastError" class="settings-note is-danger">
                {{ platformLogs.lastError }}
              </p>
              <AriButton size="sm" variant="secondary" :disabled="settings.isExportingDiagnostics" @click="settings.exportDiagnostics()">
                <HardDrive :size="14" />
                {{ settings.isExportingDiagnostics ? '导出中' : '导出诊断包' }}
              </AriButton>
              <p v-if="settings.diagnosticsExportResult?.path" class="settings-note">
                诊断包 {{ formatBytes(settings.diagnosticsExportResult.bytes) }} · {{ settings.diagnosticsExportResult.path }}
              </p>
            </details>

            <details v-if="showLegacyPanel" class="settings-panel settings-disclosure">
              <summary class="settings-panel-title">
                <Upload :size="15" />
                数据导入
              </summary>
              <div
                v-if="legacyRuntimeNeedsAttention && legacyRuntime"
                class="legacy-runtime-card"
                :class="{
                  'is-danger': legacyRuntime.hotkeyConflictLikely,
                  'is-warning': legacyRuntime.processRunning && !legacyRuntime.hotkeyConflictLikely,
                }"
              >
                <div>
                  <AlertTriangle :size="14" />
                  <strong>
                    {{
                      legacyRuntime.hotkeyConflictLikely
                        ? '检测到快捷键冲突'
                        : legacyRuntime.processRunning
                          ? '旧版正在运行'
                          : '未发现运行冲突'
                    }}
                  </strong>
                </div>
                <p v-if="legacyRuntime.processRunning">
                  {{ legacyRuntime.processName || 'x-tools.exe' }}
                  <span v-if="legacyRuntime.processId"> · pid {{ legacyRuntime.processId }}</span>
                </p>
                <p v-if="legacyRuntime.processPath">{{ legacyRuntime.processPath }}</p>
                <p v-if="legacyRuntime.configExists">旧配置 {{ legacyRuntime.configBytes || 0 }} bytes</p>
                <ul v-if="legacyRuntime.notes?.length">
                  <li v-for="note in legacyRuntime.notes" :key="note">{{ note }}</li>
                </ul>
                <div v-if="legacyRuntime.processRunning || legacyRuntime.hotkeyConflictLikely" class="legacy-handoff-actions">
                  <AriButton
                    size="sm"
                    variant="secondary"
                    :disabled="settings.isResolvingLegacyConflict"
                    @click="settings.resolveLegacyHandoff(false)"
                  >
                    <Shield :size="14" />
                    {{ settings.legacyHandoffMode === 'graceful' ? '确认交接' : '交接 Alt+Q' }}
                  </AriButton>
                  <AriButton
                    v-if="legacyRuntime.processRunning"
                    size="sm"
                    variant="ghost"
                    class="danger-action"
                    :disabled="settings.isResolvingLegacyConflict"
                    @click="settings.resolveLegacyHandoff(true)"
                  >
                    <AlertTriangle :size="14" />
                    {{ settings.legacyHandoffMode === 'force' ? '确认强制结束' : '强制结束旧版' }}
                  </AriButton>
                </div>
              </div>
              <div v-if="settings.legacyHandoffResult" class="legacy-handoff-result">
                <strong :class="{ 'is-danger': !settings.legacyHandoffResult.ok }">
                  {{ settings.legacyHandoffResult.message }}
                </strong>
                <span v-if="settings.legacyHandoffResult.hotkeyRetried">
                  Alt+Q {{ settings.legacyHandoffResult.shell.globalHotkeyRegistered ? '已接管' : '仍未接管' }}
                </span>
                <span v-for="action in settings.legacyHandoffResult.actions" :key="action">{{ action }}</span>
              </div>
              <template v-if="legacyConfigNeedsAttention">
                <p class="settings-note">{{ settings.legacyStatus?.path }}</p>
                <div class="tag-row">
                  <span v-for="key in settings.legacyStatus?.importedKeys" :key="key">{{ key }}</span>
                  <span v-if="!settings.legacyStatus?.importedKeys.length">无可导入键</span>
                </div>
                <ul v-if="settings.legacyStatus?.notes?.length" class="settings-note-list">
                  <li v-for="note in settings.legacyStatus.notes" :key="note">{{ note }}</li>
                </ul>
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="!settings.legacyStatus?.exists || settings.isSaving"
                  @click="settings.importLegacy()"
                >
                  <Upload :size="14" />
                  导入旧配置
                </AriButton>
              </template>

              <div v-if="legacyHistoryNeedsAttention" class="legacy-data-block">
                <div class="legacy-data-head">
                  <div>
                    <strong>旧历史数据</strong>
                    <p>{{ settings.legacyDataStatus?.root || '%APPDATA%/x-tools' }}</p>
                  </div>
                  <span>{{ legacyHistorySummary }}</span>
                </div>

                <div class="legacy-data-list">
                  <div
                    v-for="source in legacyDataSources"
                    :key="source.source"
                    class="legacy-data-row"
                    :class="{ 'is-empty': !source.exists || source.count === 0 }"
                  >
                    <div>
                      <strong>{{ legacySourceLabel(source.source) }}</strong>
                      <small>{{ source.path }}</small>
                    </div>
                    <span>{{ source.importedCount }}/{{ source.count }} 条 · {{ formatBytes(source.bytes + (source.imageBytes || 0)) }}</span>
                    <small v-if="source.imageCount">图片 {{ source.imageCount }} 个</small>
                    <small v-else-if="source.lastError" class="is-danger">{{ source.lastError }}</small>
                  </div>
                </div>

                <div v-if="settings.legacyImportResult?.sources.length" class="legacy-import-result">
                  <div
                    v-for="source in settings.legacyImportResult.sources"
                    :key="source.source"
                    :class="{ 'is-danger': source.failed > 0 || Boolean(source.error) }"
                  >
                    <span>{{ legacySourceLabel(source.source) }}</span>
                    <strong>
                      导入 {{ source.imported }} · 跳过 {{ source.skipped }} · 失败 {{ source.failed }}
                    </strong>
                  </div>
                </div>

                <p v-for="note in settings.legacyDataStatus?.notes" :key="note" class="settings-note">{{ note }}</p>

                <div class="legacy-actions">
                  <AriButton size="sm" variant="ghost" :disabled="settings.isMigrating" @click="settings.refreshLegacyDataStatus()">
                    <RotateCcw :size="14" />
                    刷新历史
                  </AriButton>
                  <AriButton
                    size="sm"
                    variant="secondary"
                    :disabled="!settings.legacyDataStatus?.totalCount || settings.isMigrating"
                    @click="settings.importLegacyHistoryData()"
                  >
                    <Database :size="14" />
                    {{ settings.isMigrating ? '迁移中' : '迁移旧历史' }}
                  </AriButton>
                </div>
              </div>

            </details>

            <details class="settings-panel settings-disclosure">
              <summary class="settings-panel-title">
                <Shield :size="15" />
                数据保护
              </summary>
              <div class="legacy-data-block">
                <div class="legacy-data-head">
                  <div>
                    <strong>回滚检查点</strong>
                    <p>{{ settings.releaseBackupStatus?.backupDir || '%APPDATA%/Ariadne/backups' }}</p>
                  </div>
                  <span>{{ releaseBackupSummary }}</span>
                </div>

                <div class="legacy-data-list">
                  <div
                    v-for="root in releaseDataRoots"
                    :key="`${root.kind}:${root.path}`"
                    class="legacy-data-row"
                    :class="{ 'is-empty': !root.exists || root.fileCount === 0 }"
                  >
                    <div>
                      <strong>{{ releaseRootLabel(root.kind) }}</strong>
                      <small>{{ root.path }}</small>
                    </div>
                    <span>{{ root.fileCount }} 文件 · {{ formatBytes(root.bytes) }}</span>
                  </div>
                </div>

                <p v-if="settings.releaseBackupStatus?.latestBackup" class="settings-note">
                  最近检查点 {{ settings.releaseBackupStatus.latestBackup }}
                </p>
                <p v-if="settings.releaseBackupResult?.path" class="settings-note">
                  {{ settings.releaseBackupResult.message }} · {{ settings.releaseBackupResult.path }}
                </p>
                <p v-if="settings.releaseRestoreResult?.message" class="settings-note" :class="{ 'is-danger': !settings.releaseRestoreResult.ok }">
                  {{ settings.releaseRestoreResult.message }}
                  <template v-if="settings.releaseRestoreResult.preRestoreBackupPath">
                    · 恢复前备份 {{ settings.releaseRestoreResult.preRestoreBackupPath }}
                  </template>
                </p>
                <div v-if="settings.releaseRestoreResult?.roots.length" class="legacy-import-result">
                  <div
                    v-for="root in settings.releaseRestoreResult.roots"
                    :key="`${root.archiveName}:${root.path}`"
                    :class="{ 'is-danger': Boolean(root.error) }"
                  >
                    <span>{{ releaseRootLabel(root.kind) }}</span>
                    <strong>
                      恢复 {{ root.restoredFiles }} 文件 · {{ formatBytes(root.restoredBytes) }}
                    </strong>
                  </div>
                </div>
                <p
                  v-for="note in settings.releaseBackupStatus?.notes"
                  :key="note"
                  class="settings-note"
                >
                  {{ note }}
                </p>

                <div class="legacy-actions">
                  <AriButton
                    size="sm"
                    variant="secondary"
                    :disabled="settings.isCreatingRollbackCheckpoint"
                    @click="settings.createRollbackCheckpoint()"
                  >
                    <Shield :size="14" />
                    {{ settings.isCreatingRollbackCheckpoint ? '创建中' : '创建检查点' }}
                  </AriButton>
                  <AriButton
                    size="sm"
                    variant="ghost"
                    class="danger-action"
                    :disabled="!settings.releaseBackupStatus?.latestBackup || settings.isRestoringRollbackCheckpoint"
                    @click="settings.restoreLatestRollbackCheckpoint()"
                  >
                    <RotateCcw :size="14" />
                    {{
                      settings.isRestoringRollbackCheckpoint
                        ? '恢复中'
                        : settings.rollbackRestoreArmed
                          ? '确认恢复'
                          : '恢复最近检查点'
                    }}
                  </AriButton>
                </div>
              </div>
            </details>
          </aside>

          <section ref="settingsContentRef" class="settings-content" aria-label="设置表单">
            <section class="settings-page-header">
              <div>
                <h2>{{ activeSettingsPageInfo.label }}</h2>
                <p>{{ activeSettingsPage === 'general' ? '配置 Ariadne 的基本使用选项。' : activeSettingsPageInfo.detail }}</p>
              </div>
              <span class="settings-page-status" v-if="activeSettingsPage === 'plugins'">
                {{ settings.enabledPluginCount }} / {{ settings.visiblePluginManifests.length }} 启用
              </span>
              <span class="settings-page-status" v-else-if="activeSettingsPage === 'data'">
                {{ storageHealthy ? '配置已读回' : '配置需检查' }}
              </span>
            </section>

            <section v-if="activeSettingsPage === 'general'" class="settings-panel settings-preference-panel">
              <div class="settings-preference-list">
                <label class="settings-preference-row">
                  <span class="settings-preference-copy">
                    <strong>主题</strong>
                    <small>选择 Ariadne 的界面主题</small>
                  </span>
                  <select v-model="settings.settings.general.theme" class="settings-select settings-preference-control">
                    <option value="professional-pink">专业粉（亮色）</option>
                    <option value="light-graphite">亮石墨（亮色）</option>
                    <option value="cloud-blue">云蓝（默认）</option>
                  </select>
                </label>

                <label class="settings-preference-row">
                  <span class="settings-preference-copy">
                    <strong>语言</strong>
                    <small>选择 Ariadne 的显示语言</small>
                  </span>
                  <select v-model="settings.settings.general.language" class="settings-select settings-preference-control">
                    <option value="zh-CN">简体中文</option>
                    <option value="en-US">English</option>
                  </select>
                </label>

                <label class="settings-preference-row">
                  <span class="settings-preference-copy">
                    <strong>开机启动</strong>
                    <small>系统启动时自动运行 Ariadne</small>
                  </span>
                  <span class="settings-preference-control">
                    <input v-model="settings.settings.general.runOnStartup" type="checkbox" class="settings-switch-input" />
                    <span class="settings-switch" />
                  </span>
                </label>
              </div>
            </section>

            <section v-if="activeSettingsPage === 'screenshot'" class="settings-panel settings-grid-panel">
              <div class="settings-panel-title">
                <Camera :size="15" />
                截图
              </div>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-toggle">
                <input v-model="settings.settings.screenshot.autoCopy" type="checkbox" />
                <span />
                <strong>截图后复制</strong>
                <small>区域截图完成后自动复制 PNG 到系统剪贴板，手动复制动作仍保留局部反馈。</small>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-toggle">
                <input v-model="settings.settings.screenshot.autoPin" type="checkbox" />
                <span />
                <strong>截图后贴图</strong>
                <small>区域截图完成后自动创建独立贴图窗口，位置会跟随当前选区。</small>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-toggle">
                <input v-model="settings.settings.screenshot.autoSave" type="checkbox" />
                <span />
                <strong>截图后自动保存</strong>
                <small>区域截图完成后按保存目录和文件名模板写入 PNG。</small>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-toggle">
                <input v-model="settings.settings.screenshot.autoRedact" type="checkbox" />
                <span />
                <strong>自动打码</strong>
                <small>复制、保存、贴图前遮盖命中的文字区域。</small>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-toggle">
                <input v-model="settings.settings.screenshot.redactPhones" type="checkbox" />
                <span />
                <strong>手机号</strong>
                <small>自动打码开启后，遮盖识别到的手机号码。</small>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-field">
                <span>打码关键字</span>
                <textarea
                  class="settings-textarea"
                  :value="settings.screenshotRedactKeywordsText()"
                  placeholder="每行一个关键字"
                  @input="settings.setScreenshotRedactKeywords(($event.target as HTMLTextAreaElement).value)"
                ></textarea>
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-field">
                <span>截图保存目录</span>
                <input v-model="settings.settings.screenshot.saveDir" class="settings-input" />
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-field">
                <span>文件名模板</span>
                <input v-model="settings.settings.screenshot.filenameTemplate" class="settings-input" />
              </label>

              <label v-if="activeSettingsPage === 'screenshot'" class="settings-field">
                <span>截图质量</span>
                <input v-model.number="settings.settings.screenshot.quality" class="settings-input" type="number" min="1" max="100" />
              </label>
            </section>


            <OCRSettingsPage v-if="activeSettingsPage === 'ocr'" />

            <section v-if="activeSettingsPage === 'hotkeys'" class="settings-panel">
              <div class="settings-panel-title">
                <KeyRound :size="15" />
                快捷键
              </div>
              <div class="settings-hotkey-grid settings-policy-grid">
                <label class="settings-field">
                  <span>主窗口</span>
                  <input
                    v-model="settings.settings.hotkeys.toggleWindow"
                    class="settings-input settings-hotkey-input"
                    autocomplete="off"
                    spellcheck="false"
                    placeholder="alt+q 或点击后按组合键"
                    @focus="selectHotkeyInput"
                    @click="selectHotkeyInput"
                    @keydown="captureHotkey('toggleWindow', $event)"
                    @blur="settings.normalizeHotkey('toggleWindow')"
                  />
                </label>
                <label class="settings-field">
                  <span>截图</span>
                  <input
                    v-model="settings.settings.hotkeys.screenshot"
                    class="settings-input settings-hotkey-input"
                    autocomplete="off"
                    spellcheck="false"
                    placeholder="alt+a 或点击后按组合键"
                    @focus="selectHotkeyInput"
                    @click="selectHotkeyInput"
                    @keydown="captureHotkey('screenshot', $event)"
                    @blur="settings.normalizeHotkey('screenshot')"
                  />
                </label>
                <label class="settings-field">
                  <span>贴图</span>
                  <input
                    v-model="settings.settings.hotkeys.pinClipboard"
                    class="settings-input settings-hotkey-input"
                    autocomplete="off"
                    spellcheck="false"
                    placeholder="alt+v 或点击后按组合键"
                    title="将当前剪贴板图片或文本固定到桌面。"
                    @focus="selectHotkeyInput"
                    @click="selectHotkeyInput"
                    @keydown="captureHotkey('pinClipboard', $event)"
                    @blur="settings.normalizeHotkey('pinClipboard')"
                  />
                  <small class="settings-help-text">触发后会把当前剪贴板图片或文本固定到桌面。</small>
                </label>
              </div>
              <div class="hotkey-status-row">
                <span class="hotkey-status-pill" :class="{ 'is-on': shellRuntime?.globalHotkeyRegistered }">
                  主窗口 {{ shellRuntime?.globalHotkeyRegistered ? '已注册' : '未注册' }} · {{ shellRuntime?.globalHotkey || settings.settings.hotkeys.toggleWindow }}
                </span>
                <span class="hotkey-status-pill" :class="{ 'is-on': shellRuntime?.screenshotHotkeyRegistered }">
                  截图 {{ shellRuntime?.screenshotHotkeyRegistered ? '已注册' : '未注册' }} · {{ shellRuntime?.screenshotHotkey || settings.settings.hotkeys.screenshot }}
                </span>
                <span class="hotkey-status-pill" :class="{ 'is-on': shellRuntime?.pinClipboardHotkeyRegistered }">
                  贴图 {{ shellRuntime?.pinClipboardHotkeyRegistered ? '已注册' : '未注册' }} · {{ shellRuntime?.pinClipboardHotkey || settings.settings.hotkeys.pinClipboard }}
                </span>
              </div>
              <p
                v-if="shellRuntime?.lastError && (!shellRuntime.globalHotkeyRegistered || !shellRuntime.screenshotHotkeyRegistered || !shellRuntime.pinClipboardHotkeyRegistered)"
                class="settings-help-text is-danger"
              >
                快捷键注册失败：{{ shellRuntime.lastError }}
              </p>
              <div class="hotkey-actions">
                <p class="settings-help-text">可以直接输入 alt+q 这类文本，也可以点击输入框后按组合键；应用后会重注册主窗口、截图和贴图热键。</p>
                <AriButton size="sm" variant="primary" :disabled="settings.isSaving" @click="settings.applyHotkeys()">
                  <KeyRound :size="14" />
                  {{ settings.isSaving ? '应用中' : '应用快捷键' }}
                </AriButton>
              </div>
            </section>

            <section v-if="activeSettingsPage === 'plugins'" class="settings-panel plugin-settings-panel">
              <div class="settings-panel-title">
                <Puzzle :size="15" />
                插件
                <span class="settings-title-count">
                  {{ settings.enabledPluginCount }} / {{ settings.visiblePluginManifests.length }} 启用
                </span>
              </div>
              <p class="settings-note">插件开关会影响启动器搜索、命令补全和插件命令执行；保存设置后生效。</p>

              <div v-if="settings.visiblePluginManifests.length" class="plugin-settings-grid">
                <label
                  v-for="plugin in settings.visiblePluginManifests"
                  :key="plugin.id"
                  class="plugin-toggle-card"
                  :class="{ 'is-disabled': !settings.pluginEnabled(plugin.id) }"
                >
                  <input
                    type="checkbox"
                    :checked="settings.pluginEnabled(plugin.id)"
                    @change="settings.setPluginEnabled(plugin.id, ($event.target as HTMLInputElement).checked)"
                  />
                  <span class="plugin-card-switch" aria-hidden="true" />
                  <span class="plugin-card-body">
                    <strong>{{ plugin.name }}</strong>
                    <small>{{ plugin.description }}</small>
                    <code>{{ plugin.commandSchema.usage || plugin.keywords[0] }}</code>
                    <span class="plugin-keywords">
                      <span v-for="keyword in plugin.keywords.slice(0, 5)" :key="keyword">{{ keyword }}</span>
                    </span>
                    <small>{{ pluginCapabilitiesLabel(plugin.requiredCapabilities) }}</small>
                  </span>
                </label>
              </div>
              <p v-else class="settings-note is-warning">插件清单暂未加载。</p>
            </section>

            <section v-if="activeSettingsPage === 'launchers'" class="settings-panel launcher-manager-panel">
              <div class="settings-panel-title">
                <Rocket :size="15" />
                自定义启动项
              </div>

              <div class="launcher-manager">
                <div class="launcher-list" aria-label="启动项列表">
                  <button
                    v-for="launcher in settings.launcherStatus?.items"
                    :key="launcher.id"
                    type="button"
                    data-no-drag
                    class="launcher-item"
                    :class="{ 'is-selected': launcher.id && launcher.id === settings.launcherDraft.id }"
                    @pointerdown.stop
                    @click="selectLauncher(launcher)"
                  >
                    <span class="launcher-item-title">{{ launcher.name }}</span>
                    <small>{{ launcherKindLabel(launcher.kind) }} · {{ launcher.enabled ? '启用' : '停用' }}</small>
                  </button>
                  <div v-if="!settings.launcherStatus?.items.length" class="launcher-empty">
                    还没有自定义启动项
                  </div>
                  <AriButton size="sm" variant="secondary" @click="settings.newLauncher()">
                    <Plus :size="14" />
                    新建启动项
                  </AriButton>
                </div>

                <div class="launcher-form" aria-label="启动项编辑">
                  <div class="settings-hotkey-grid">
                    <label class="settings-field">
                      <span>名称</span>
                      <input v-model="settings.launcherDraft.name" class="settings-input" placeholder="例如 Ariadne 配置目录" />
                    </label>
                    <label class="settings-field">
                      <span>类型</span>
                      <select v-model="settings.launcherDraft.kind" class="settings-select">
                        <option v-for="item in launcherKindOptions" :key="item.value" :value="item.value">
                          {{ item.label }}
                        </option>
                      </select>
                    </label>
                  </div>

                  <label class="settings-field">
                    <span>目标</span>
                    <input v-model="settings.launcherDraft.target" class="settings-input" placeholder="应用、文件、文件夹、URL 或命令" />
                  </label>

                  <div class="settings-hotkey-grid">
                    <label class="settings-field">
                      <span>参数</span>
                      <input v-model="settings.launcherDraft.arguments" class="settings-input" placeholder="命令参数，可留空" />
                    </label>
                    <label class="settings-field">
                      <span>工作目录</span>
                      <input v-model="settings.launcherDraft.workingDir" class="settings-input" placeholder="可留空" />
                    </label>
                  </div>

                  <div class="settings-text-grid">
                    <label class="settings-field">
                      <span>关键词</span>
                      <textarea
                        class="settings-textarea launcher-textarea"
                        :value="settings.launcherListText('keywords')"
                        @input="settings.setLauncherList('keywords', ($event.target as HTMLTextAreaElement).value)"
                      />
                    </label>
                    <label class="settings-field">
                      <span>标签</span>
                      <textarea
                        class="settings-textarea launcher-textarea"
                        :value="settings.launcherListText('tags')"
                        @input="settings.setLauncherList('tags', ($event.target as HTMLTextAreaElement).value)"
                      />
                    </label>
                  </div>

                  <label class="settings-toggle">
                    <input v-model="settings.launcherDraft.enabled" type="checkbox" />
                    <span />
                    <strong>启用启动项</strong>
                    <small>停用后不会出现在主搜索结果中。</small>
                  </label>

                  <p v-if="settings.launcherDraft.kind === 'command'" class="settings-note is-warning">
                    命令类启动项会作为高风险动作展示，执行前必须确认。
                  </p>
                  <p class="settings-note">
                    配置文件 {{ settings.launcherStatus?.path || '%APPDATA%/Ariadne/ariadne.sqlite' }}
                  </p>

                  <div class="launcher-actions">
                    <AriButton size="sm" variant="primary" :disabled="settings.isSaving" @click="settings.saveLauncher()">
                      <Save :size="14" />
                      保存启动项
                    </AriButton>
                    <AriButton size="sm" variant="secondary" :disabled="settings.isSaving" @click="settings.deleteLauncher()">
                      <Trash2 :size="14" />
                      {{ settings.launcherDeleteArmedId === settings.launcherDraft.id ? '确认删除' : '删除' }}
                    </AriButton>
                  </div>
                </div>
              </div>
            </section>




            <SearchSettingsPage v-if="activeSettingsPage === 'search'" />

            <HistorySettingsPage v-if="activeSettingsPage === 'data'" />
            <section v-if="activeSettingsPage === 'data'" class="settings-panel settings-grid-panel">
              <div class="settings-panel-title">
                <Database :size="15" />
                存储状态
              </div>
              <div class="settings-status-card">
                <span>配置文件</span>
                <strong>{{ storageHealthy ? '已读回' : settings.storageStatus?.exists ? '需检查' : '尚未写入' }}</strong>
                <small>{{ settings.storageStatus?.path || '配置路径未初始化' }}</small>
              </div>
              <div class="settings-status-card">
                <span>配置大小</span>
                <strong>{{ formatBytes(settings.storageStatus?.bytes) }}</strong>
                <small>读回 {{ settings.storageStatus?.readBackOk ? '正常' : '失败' }} · v{{ settings.storageStatus?.readBackVersion || '-' }}</small>
              </div>
              <div class="settings-status-card">
                <span>APPDATA</span>
                <strong>{{ settings.storageStatus?.appDataEnv ? '已定位' : '未设置' }}</strong>
                <small>{{ settings.storageStatus?.appDataEnv || '-' }}</small>
              </div>
              <p v-if="settings.storageStatus?.lastSaveError" class="settings-note is-danger">
                {{ settings.storageStatus.lastSaveError }}
              </p>
              <p v-if="settings.storageStatus?.readBackError" class="settings-note is-danger">
                {{ settings.storageStatus.readBackError }}
              </p>
            </section>

            <section v-if="activeSettingsPage === 'advanced'" class="settings-panel">
              <div class="settings-panel-title">
                <Download :size="15" />
                应用更新
              </div>
              <div class="settings-status-grid">
                <div class="settings-status-card">
                  <span>当前版本</span>
                  <strong>{{ settings.appUpdateStatus?.currentVersion || 'dev' }}</strong>
                  <small>{{ settings.appUpdateStatus?.artifactName || '通过 GitHub Releases 获取安装器' }}</small>
                </div>
                <div class="settings-status-card">
                  <span>更新状态</span>
                  <strong>{{ appUpdateSummary }}</strong>
                  <small>{{ settings.appUpdateStatus?.releaseName || settings.appUpdateStatus?.state || 'unconfigured' }}</small>
                </div>
              </div>
              <p v-if="settings.appUpdateStatus?.releaseNotes" class="settings-note">
                {{ settings.appUpdateStatus.releaseNotes }}
              </p>
              <p v-if="settings.appUpdateStatus?.message" class="settings-note" :class="{ 'is-danger': Boolean(settings.appUpdateStatus.lastError) }">
                {{ settings.appUpdateStatus.message }}
              </p>
              <div class="settings-inline-actions">
                <AriButton
                  size="sm"
                  variant="secondary"
                  :disabled="settings.isCheckingUpdate || settings.isInstallingUpdate || !settings.appUpdateStatus?.canCheck"
                  @click="settings.checkForUpdates()"
                >
                  <RotateCcw :size="14" />
                  {{ settings.isCheckingUpdate ? '检查中' : '检查更新' }}
                </AriButton>
                <AriButton
                  v-if="settings.appUpdateStatus?.updateAvailable || settings.appUpdateStatus?.installerLaunched"
                  size="sm"
                  variant="primary"
                  :disabled="settings.isInstallingUpdate || !settings.appUpdateStatus?.canInstall"
                  @click="settings.installAvailableUpdate()"
                >
                  <Download :size="14" />
                  {{ settings.isInstallingUpdate ? '下载校验中' : settings.appUpdateStatus?.installerLaunched ? '安装器已启动' : '下载并安装' }}
                </AriButton>
              </div>
            </section>

            <section v-if="activeSettingsPage === 'advanced'" class="settings-panel">
              <div class="settings-panel-title">
                <HardDrive :size="15" />
                平台诊断
              </div>
              <div class="settings-status-grid">
                <div class="settings-status-card">
                  <span>运行时</span>
                  <strong>{{ settings.platformStatus?.diagnostics.os }}/{{ settings.platformStatus?.diagnostics.arch }}</strong>
                  <small>pid {{ settings.platformStatus?.diagnostics.processId || '-' }}</small>
                </div>
                <div class="settings-status-card">
                  <span>日志</span>
                  <strong>{{ platformLogs?.exists ? formatBytes(platformLogs.bytes) : platformLogs?.directoryExists ? '待写入' : '目录未创建' }}</strong>
                  <small>{{ platformLogs?.path || '日志路径未初始化' }}</small>
                </div>
              </div>
              <p v-if="shellRuntime?.lastError" class="settings-note is-warning">
                快捷键/启动状态：{{ shellRuntime.lastError }}
              </p>
              <div class="settings-inline-actions">
                <AriButton size="sm" variant="secondary" @click="settings.refreshPlatformStatus()">
                  <RotateCcw :size="14" />
                  刷新状态
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="settings.isExportingDiagnostics" @click="settings.exportDiagnostics()">
                  <HardDrive :size="14" />
                  {{ settings.isExportingDiagnostics ? '导出中' : '导出诊断包' }}
                </AriButton>
              </div>
            </section>

            <section v-if="activeSettingsPage === 'advanced' && showLegacyPanel" class="settings-panel">
              <div class="settings-panel-title">
                <Upload :size="15" />
                数据导入
              </div>
              <div v-if="legacyRuntimeNeedsAttention && legacyRuntime" class="legacy-runtime-card" :class="{ 'is-danger': legacyRuntime.hotkeyConflictLikely, 'is-warning': legacyRuntime.processRunning && !legacyRuntime.hotkeyConflictLikely }">
                <div>
                  <AlertTriangle :size="14" />
                  <strong>{{ legacyRuntime.hotkeyConflictLikely ? '检测到快捷键冲突' : legacyRuntime.processRunning ? '旧版正在运行' : '运行状态需检查' }}</strong>
                </div>
                <p v-if="legacyRuntime.processRunning">{{ legacyRuntime.processName || 'x-tools.exe' }} <span v-if="legacyRuntime.processId">· pid {{ legacyRuntime.processId }}</span></p>
                <div v-if="legacyRuntime.processRunning || legacyRuntime.hotkeyConflictLikely" class="legacy-handoff-actions">
                  <AriButton size="sm" variant="secondary" :disabled="settings.isResolvingLegacyConflict" @click="settings.resolveLegacyHandoff(false)">
                    <Shield :size="14" />
                    {{ settings.legacyHandoffMode === 'graceful' ? '确认交接' : '交接 Alt+Q' }}
                  </AriButton>
                </div>
              </div>
              <div v-if="legacyConfigNeedsAttention" class="settings-status-card">
                <span>配置导入</span>
                <strong>{{ settings.legacyStatus?.importedKeys.length ?? 0 }} 项可导入</strong>
                <small>{{ settings.legacyStatus?.path }}</small>
                <AriButton size="sm" variant="secondary" :disabled="!settings.legacyStatus?.exists || settings.isSaving" @click="settings.importLegacy()">
                  <Upload :size="14" />
                  导入配置
                </AriButton>
              </div>
              <div v-if="legacyHistoryNeedsAttention" class="settings-status-card">
                <span>历史数据</span>
                <strong>{{ legacyHistorySummary }}</strong>
                <small>{{ settings.legacyDataStatus?.root || '%APPDATA%/x-tools' }}</small>
                <div class="settings-inline-actions">
                  <AriButton size="sm" variant="ghost" :disabled="settings.isMigrating" @click="settings.refreshLegacyDataStatus()">
                    <RotateCcw :size="14" />
                    刷新
                  </AriButton>
                  <AriButton size="sm" variant="secondary" :disabled="!settings.legacyDataStatus?.totalCount || settings.isMigrating" @click="settings.importLegacyHistoryData()">
                    <Database :size="14" />
                    {{ settings.isMigrating ? '迁移中' : '导入历史' }}
                  </AriButton>
                </div>
              </div>
            </section>

            <section v-if="activeSettingsPage === 'advanced'" class="settings-panel">
              <div class="settings-panel-title">
                <Shield :size="15" />
                回滚检查点
              </div>
              <div class="settings-status-grid">
                <div class="settings-status-card">
                  <span>检查点</span>
                  <strong>{{ releaseBackupSummary }}</strong>
                  <small>{{ settings.releaseBackupStatus?.backupDir || '%APPDATA%/Ariadne/backups' }}</small>
                </div>
                <div class="settings-status-card">
                  <span>数据根</span>
                  <strong>{{ releaseDataRoots.length }} 个</strong>
                  <small>{{ settings.releaseBackupStatus?.latestBackup || '暂无最近检查点' }}</small>
                </div>
              </div>
              <p v-if="settings.releaseBackupResult?.path" class="settings-note">
                {{ settings.releaseBackupResult.message }} · {{ settings.releaseBackupResult.path }}
              </p>
              <p v-if="settings.releaseRestoreResult?.message" class="settings-note" :class="{ 'is-danger': !settings.releaseRestoreResult.ok }">
                {{ settings.releaseRestoreResult.message }}
              </p>
              <div class="settings-inline-actions">
                <AriButton size="sm" variant="secondary" :disabled="settings.isCreatingRollbackCheckpoint" @click="settings.createRollbackCheckpoint()">
                  <Shield :size="14" />
                  {{ settings.isCreatingRollbackCheckpoint ? '创建中' : '创建检查点' }}
                </AriButton>
                <AriButton size="sm" variant="ghost" class="danger-action" :disabled="!settings.releaseBackupStatus?.latestBackup || settings.isRestoringRollbackCheckpoint" @click="settings.restoreLatestRollbackCheckpoint()">
                  <RotateCcw :size="14" />
                  {{ settings.isRestoringRollbackCheckpoint ? '恢复中' : settings.rollbackRestoreArmed ? '确认恢复' : '恢复最近检查点' }}
                </AriButton>
              </div>
            </section>
          </section>
        </div>

        <footer class="status-strip">
          <span class="settings-footer-spacer" />
          <span v-if="settings.feedback" class="inline-feedback">{{ settings.feedback }}</span>
          <span class="settings-footer-actions">
            <AriButton size="sm" variant="ghost" :disabled="settings.isSaving" @click="settings.reset()">
              <RotateCcw :size="14" />
              恢复默认
            </AriButton>
            <AriButton size="sm" variant="primary" :disabled="settings.isSaving" @click="settings.save()">
              <Save :size="14" />
              {{ settings.isSaving ? '保存中' : '保存设置' }}
            </AriButton>
          </span>
        </footer>
      </section>
    </div>
  </main>
</template>
