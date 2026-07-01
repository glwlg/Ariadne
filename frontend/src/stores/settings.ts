import { computed, ref, toRaw } from 'vue'
import { defineStore } from 'pinia'
import { exportDiagnosticsBundle, getPlatformStatus, installFileSearchService, resolveLegacyConflict } from '../services/platformApi'
import { createLauncherDraft, getLauncherStatus, removeLauncher, upsertLauncher } from '../services/launchersApi'
import { getLegacyDataStatus, importLegacyData } from '../services/migrationApi'
import { applyTheme, publishTheme } from '../lib/theme'
import { listPlugins } from '../services/pluginsApi'
import { createRollbackCheckpoint as createReleaseRollbackCheckpoint, getReleaseBackupStatus, restoreRollbackCheckpoint as restoreReleaseRollbackCheckpoint } from '../services/releaseApi'
import { clearSearchUsage, getSearchUsageStatus } from '../services/searchUsageApi'
import { clearSecret as clearStoredSecret, getSecretStatus, saveSecret as saveStoredSecret } from '../services/secretsApi'
import {
  getLegacyConfigStatus,
  getSettingsStorageStatus,
  getSettings,
  importLegacyConfig,
  resetSettings,
  updateSettings,
} from '../services/settingsApi'
import type {
  AppSettings,
  ActionResult,
  DiagnosticsExportResult,
  HotkeySettings,
  Launcher,
  LauncherStatus,
  LegacyConfigStatus,
  LegacyDataStatus,
  LegacyHandoffResult,
  LegacyImportResult,
  PlatformStatus,
  PluginManifest,
  ReleaseBackupResult,
  ReleaseBackupStatus,
  ReleaseRestoreResult,
  SearchUsageClearResult,
  SearchUsageStatus,
  SecretActionResult,
  SecretStatus,
  SettingsStorageStatus,
} from '../types/ariadne'

type WorkMemoryListPath = 'excludeApps' | 'excludeWindowKeywords' | 'excludePaths' | 'excludeUrls' | 'excludeContentPatterns'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<AppSettings | null>(null)
  const legacyStatus = ref<LegacyConfigStatus | null>(null)
  const legacyDataStatus = ref<LegacyDataStatus | null>(null)
  const legacyImportResult = ref<LegacyImportResult | null>(null)
  const releaseBackupStatus = ref<ReleaseBackupStatus | null>(null)
  const releaseBackupResult = ref<ReleaseBackupResult | null>(null)
  const releaseRestoreResult = ref<ReleaseRestoreResult | null>(null)
  const diagnosticsExportResult = ref<DiagnosticsExportResult | null>(null)
  const legacyHandoffResult = ref<LegacyHandoffResult | null>(null)
  const storageStatus = ref<SettingsStorageStatus | null>(null)
  const platformStatus = ref<PlatformStatus | null>(null)
  const fileSearchServiceActionResult = ref<ActionResult | null>(null)
  const searchUsageStatus = ref<SearchUsageStatus | null>(null)
  const searchUsageClearResult = ref<SearchUsageClearResult | null>(null)
  const secretStatus = ref<SecretStatus | null>(null)
  const secretActionResult = ref<SecretActionResult | null>(null)
  const pluginManifests = ref<PluginManifest[]>([])
  const secretInputs = ref<Record<string, string>>({
    ai_api_key: '',
    embedding_api_key: '',
    milvus_token: '',
  })
  const launcherStatus = ref<LauncherStatus | null>(null)
  const launcherDraft = ref<Launcher>(createLauncherDraft())
  const launcherDeleteArmedId = ref('')
  const searchUsageClearArmed = ref(false)
  const secretClearArmedKind = ref('')
  const workMemoryListDrafts = ref<Record<WorkMemoryListPath, string>>({
    excludeApps: '',
    excludeWindowKeywords: '',
    excludePaths: '',
    excludeUrls: '',
    excludeContentPatterns: '',
  })
  const searchExcludeFoldersDraft = ref('')
  const searchExcludePatternsDraft = ref('')
  const screenshotRedactKeywordsDraft = ref('')
  const feedback = ref('')
  const isLoading = ref(false)
  const isSaving = ref(false)
  const isMigrating = ref(false)
  const isCreatingRollbackCheckpoint = ref(false)
  const isRestoringRollbackCheckpoint = ref(false)
  const isExportingDiagnostics = ref(false)
  const isResolvingLegacyConflict = ref(false)
  const isInstallingSearchService = ref(false)
  const legacyHandoffMode = ref<'graceful' | 'force' | ''>('')
  const rollbackRestoreArmed = ref(false)

  const hasSettings = computed(() => Boolean(settings.value))
  const visiblePluginManifests = computed(() => pluginManifests.value)
  const enabledPluginCount = computed(() => visiblePluginManifests.value.filter((plugin) => pluginEnabled(plugin.id)).length)

  const memorySources = computed(() => {
    const memory = settings.value?.workMemory
    if (!memory) return []
    return [
      { key: 'sourceClipboard', label: '剪贴板', enabled: memory.sourceClipboard },
      { key: 'sourceCaptureHistory', label: '截图历史', enabled: memory.sourceCaptureHistory },
      { key: 'sourceManualNote', label: '手动笔记', enabled: memory.sourceManualNote },
      { key: 'sourceSearchFavorite', label: '搜索收藏', enabled: memory.sourceSearchFavorite },
      { key: 'sourceActions', label: '动作轨迹', enabled: memory.sourceActions },
    ]
  })

  async function load() {
    isLoading.value = true
    try {
      const [nextSettings, nextLegacy, nextStorage, nextPlatform, nextLaunchers, nextSearchUsage, nextLegacyData, nextReleaseBackup, nextSecrets, nextPlugins] = await Promise.all([
        getSettings(),
        getLegacyConfigStatus(),
        getSettingsStorageStatus(),
        getPlatformStatus(),
        getLauncherStatus(),
        getSearchUsageStatus(),
        getLegacyDataStatus(),
        getReleaseBackupStatus(),
        getSecretStatus(),
        listPlugins(),
      ])
      settings.value = nextSettings
      applyTheme(nextSettings.general.theme)
      legacyStatus.value = nextLegacy
      storageStatus.value = nextStorage
      platformStatus.value = nextPlatform
      launcherStatus.value = nextLaunchers
      searchUsageStatus.value = nextSearchUsage
      legacyDataStatus.value = nextLegacyData
      releaseBackupStatus.value = nextReleaseBackup
      secretStatus.value = nextSecrets
      pluginManifests.value = nextPlugins
      syncTextDraftsFromSettings()
      if (!launcherDraft.value.name && nextLaunchers.items[0]) {
        editLauncher(nextLaunchers.items[0])
      }
    } catch {
      showFeedback('设置加载失败')
    } finally {
      isLoading.value = false
    }
  }

  async function save() {
    if (!settings.value) return
    const hotkeyValidation = validateHotkeys(settings.value.hotkeys)
    if (hotkeyValidation) {
      showFeedback(hotkeyValidation)
      return
    }
    syncSearchDraftsToSettings()
    isSaving.value = true
    try {
      settings.value = await updateSettings(settings.value)
      syncTextDraftsFromSettings()
      publishTheme(settings.value.general.theme)
      storageStatus.value = await getSettingsStorageStatus()
      await refreshPlatformStatus()
      showFeedback(isStorageHealthy(storageStatus.value) ? '设置已保存' : '设置保存失败')
    } catch {
      showFeedback('设置保存失败')
    } finally {
      isSaving.value = false
    }
  }

  async function saveSearchSettings() {
    if (!settings.value) return
    syncSearchDraftsToSettings()
    isSaving.value = true
    try {
      settings.value = await updateSettings(settings.value)
      syncTextDraftsFromSettings()
      storageStatus.value = await getSettingsStorageStatus()
      await refreshPlatformStatus()
      showFeedback(isStorageHealthy(storageStatus.value) ? '搜索排除规则已保存' : '搜索排除规则保存失败')
    } catch {
      showFeedback('搜索排除规则保存失败')
    } finally {
      isSaving.value = false
    }
  }

  async function applyHotkeys() {
    if (!settings.value) return
    const validation = validateHotkeys(settings.value.hotkeys)
    if (validation) {
      showFeedback(validation)
      return
    }
    normalizeAllHotkeys(settings.value.hotkeys)

    isSaving.value = true
    try {
      settings.value = await updateSettings(settings.value)
      syncTextDraftsFromSettings()
      publishTheme(settings.value.general.theme)
      storageStatus.value = await getSettingsStorageStatus()
      await refreshPlatformStatus()
      const shell = platformStatus.value?.shell
      if (shell?.lastError) {
        showFeedback(`快捷键已保存，注册失败: ${shortError(shell.lastError)}`)
        return
      }
      showFeedback(isStorageHealthy(storageStatus.value) ? '快捷键已应用' : '快捷键保存失败')
    } catch {
      showFeedback('快捷键保存失败')
    } finally {
      isSaving.value = false
    }
  }

  async function reset() {
    isSaving.value = true
    try {
      settings.value = await resetSettings()
      syncTextDraftsFromSettings()
      publishTheme(settings.value.general.theme)
      storageStatus.value = await getSettingsStorageStatus()
      await refreshPlatformStatus()
      showFeedback(isStorageHealthy(storageStatus.value) ? '已恢复默认设置' : '恢复默认失败')
    } catch {
      showFeedback('恢复默认失败')
    } finally {
      isSaving.value = false
    }
  }

  async function importLegacy() {
    isSaving.value = true
    try {
      settings.value = await importLegacyConfig()
      syncTextDraftsFromSettings()
      publishTheme(settings.value.general.theme)
      legacyStatus.value = await getLegacyConfigStatus()
      storageStatus.value = await getSettingsStorageStatus()
      await refreshPlatformStatus()
      showFeedback(isStorageHealthy(storageStatus.value) ? '旧版配置已导入' : '旧版配置导入失败')
    } catch {
      showFeedback('旧版配置导入失败')
    } finally {
      isSaving.value = false
    }
  }

  async function refreshLegacyDataStatus() {
    try {
      legacyDataStatus.value = await getLegacyDataStatus()
      showFeedback(legacyDataStatus.value.exists ? '旧历史状态已刷新' : '未发现旧历史数据')
    } catch {
      showFeedback('旧历史状态刷新失败')
    }
  }

  async function importLegacyHistoryData() {
    isMigrating.value = true
    try {
      legacyImportResult.value = await importLegacyData({ limit: 5000 })
      legacyDataStatus.value = await getLegacyDataStatus()
      const imported = legacyImportResult.value.sources.reduce((total, source) => total + source.imported, 0)
      if (legacyImportResult.value.ok) {
        showFeedback(imported > 0 ? `旧历史已迁移 ${imported} 条` : '旧历史已检查，无新增记录')
      } else {
        showFeedback('旧历史迁移完成，但有失败项')
      }
    } catch {
      showFeedback('旧历史迁移失败')
    } finally {
      isMigrating.value = false
    }
  }

  async function createRollbackCheckpoint() {
    isCreatingRollbackCheckpoint.value = true
    try {
      releaseBackupResult.value = await createReleaseRollbackCheckpoint({ reason: 'manual_settings_checkpoint' })
      releaseBackupStatus.value = await getReleaseBackupStatus()
      showFeedback(releaseBackupResult.value.ok ? releaseBackupResult.value.message : `检查点失败: ${shortError(releaseBackupResult.value.message)}`)
    } catch {
      showFeedback('回滚检查点创建失败')
    } finally {
      isCreatingRollbackCheckpoint.value = false
    }
  }

  async function restoreLatestRollbackCheckpoint() {
    if (!rollbackRestoreArmed.value) {
      rollbackRestoreArmed.value = true
      showFeedback('再次点击确认恢复最近检查点')
      return
    }
    isRestoringRollbackCheckpoint.value = true
    try {
      releaseRestoreResult.value = await restoreReleaseRollbackCheckpoint({
        path: releaseBackupStatus.value?.latestBackup,
        confirm: true,
        createPreRestoreBackup: true,
      })
      releaseBackupStatus.value = await getReleaseBackupStatus()
      showFeedback(releaseRestoreResult.value.message)
    } catch {
      showFeedback('回滚恢复失败')
    } finally {
      rollbackRestoreArmed.value = false
      isRestoringRollbackCheckpoint.value = false
    }
  }

  async function exportDiagnostics() {
    isExportingDiagnostics.value = true
    try {
      diagnosticsExportResult.value = await exportDiagnosticsBundle()
      platformStatus.value = await getPlatformStatus()
      showFeedback(diagnosticsExportResult.value.ok ? diagnosticsExportResult.value.message : `诊断导出失败: ${shortError(diagnosticsExportResult.value.message)}`)
    } catch {
      showFeedback('诊断导出失败')
    } finally {
      isExportingDiagnostics.value = false
    }
  }

  async function resolveLegacyHandoff(force = false) {
    const mode = force ? 'force' : 'graceful'
    if (legacyHandoffMode.value !== mode) {
      legacyHandoffMode.value = mode
      showFeedback(force ? '再次点击确认强制结束旧版 x-tools' : '再次点击确认关闭旧版 x-tools 并重试 Alt+Q')
      return
    }
    isResolvingLegacyConflict.value = true
    try {
      legacyHandoffResult.value = await resolveLegacyConflict({ confirm: true, force, timeoutMs: force ? 1500 : 3000 })
      platformStatus.value = await getPlatformStatus()
      showFeedback(legacyHandoffResult.value.message)
    } catch {
      showFeedback('旧版交接失败')
    } finally {
      legacyHandoffMode.value = ''
      isResolvingLegacyConflict.value = false
    }
  }

  async function updateWorkMemoryRuntime(patch: Partial<AppSettings['workMemory']>) {
    if (!settings.value) {
      await load()
    }
    if (!settings.value) return null
    const next = structuredClone(toRaw(settings.value))
    next.workMemory = {
      ...next.workMemory,
      ...patch,
    }
    settings.value = await updateSettings(next)
    syncTextDraftsFromSettings()
    applyTheme(settings.value.general.theme)
    storageStatus.value = await getSettingsStorageStatus()
    return settings.value.workMemory
  }

  function setList(path: WorkMemoryListPath, value: string) {
    if (!settings.value) return
    workMemoryListDrafts.value[path] = value
    settings.value.workMemory[path] = linesToList(value)
  }

  function listText(path: WorkMemoryListPath) {
    return workMemoryListDrafts.value[path]
  }

  function setScreenshotRedactKeywords(value: string) {
    if (!settings.value) return
    screenshotRedactKeywordsDraft.value = value
    settings.value.screenshot.redactKeywords = linesToList(value)
  }

  function screenshotRedactKeywordsText() {
    return screenshotRedactKeywordsDraft.value
  }

  function setSearchExcludeFolders(value: string) {
    if (!settings.value) return
    searchExcludeFoldersDraft.value = value
    ensureSearchSettings()
    settings.value.search.fileExcludeFolders = multilineToList(value)
  }

  function searchExcludeFoldersText() {
    return searchExcludeFoldersDraft.value
  }

  function setSearchExcludePatterns(value: string) {
    if (!settings.value) return
    searchExcludePatternsDraft.value = value
    ensureSearchSettings()
    settings.value.search.fileExcludePatterns = multilineToList(value)
  }

  function searchExcludePatternsText() {
    return searchExcludePatternsDraft.value
  }

  function syncTextDraftsFromSettings() {
    const memory = settings.value?.workMemory
    workMemoryListDrafts.value = {
      excludeApps: memory?.excludeApps.join('\n') ?? '',
      excludeWindowKeywords: memory?.excludeWindowKeywords.join('\n') ?? '',
      excludePaths: memory?.excludePaths.join('\n') ?? '',
      excludeUrls: memory?.excludeUrls.join('\n') ?? '',
      excludeContentPatterns: memory?.excludeContentPatterns.join('\n') ?? '',
    }
    screenshotRedactKeywordsDraft.value = settings.value?.screenshot.redactKeywords.join('\n') ?? ''
    ensureSearchSettings()
    searchExcludeFoldersDraft.value = settings.value?.search.fileExcludeFolders.join('\n') ?? ''
    searchExcludePatternsDraft.value = settings.value?.search.fileExcludePatterns.join('\n') ?? ''
  }

  function syncSearchDraftsToSettings() {
    if (!settings.value) return
    ensureSearchSettings()
    settings.value.search.fileExcludeFolders = multilineToList(searchExcludeFoldersDraft.value)
    settings.value.search.fileExcludePatterns = multilineToList(searchExcludePatternsDraft.value)
  }

  function ensureSearchSettings() {
    if (!settings.value) return
    if (!settings.value.search) {
      settings.value.search = {
        fileExcludeFolders: [],
        fileExcludePatterns: [],
      }
    }
    settings.value.search.fileExcludeFolders ??= []
    settings.value.search.fileExcludePatterns ??= []
  }

  function setMemorySource(key: string, enabled: boolean) {
    if (!settings.value) return
    const memory = settings.value.workMemory as unknown as Record<string, boolean>
    memory[key] = enabled
  }

  function pluginEnabled(id: string) {
    return settings.value?.plugins.enabled[id] !== false
  }

  function setPluginEnabled(id: string, enabled: boolean) {
    if (!settings.value) return
    if (!settings.value.plugins.enabled) {
      settings.value.plugins.enabled = {}
    }
    settings.value.plugins.enabled[id] = enabled
  }

  function setHotkey(key: keyof HotkeySettings, value: string) {
    if (!settings.value) return
    settings.value.hotkeys[key] = value
  }

  function stageHotkey(key: keyof HotkeySettings, value: string) {
    if (!settings.value) return
    const normalized = normalizeHotkeyValue(value)
    settings.value.hotkeys[key] = normalized.value || value.trim().toLowerCase()
    showFeedback('快捷键已暂存，点击应用快捷键生效')
  }

  function normalizeHotkey(key: keyof HotkeySettings) {
    if (!settings.value) return
    const current = settings.value.hotkeys[key]
    const normalized = normalizeHotkeyValue(current)
    settings.value.hotkeys[key] = normalized.value || current.trim().toLowerCase()
  }

  function editLauncher(launcher: Launcher) {
    launcherDraft.value = cloneLauncher(launcher)
    launcherDeleteArmedId.value = ''
  }

  function newLauncher() {
    launcherDraft.value = createLauncherDraft()
    launcherDeleteArmedId.value = ''
  }

  async function saveLauncher() {
    const draft = cloneLauncher(launcherDraft.value)
    if (!draft.name || !draft.target) {
      showFeedback('启动项需要名称和目标')
      return
    }
    isSaving.value = true
    try {
      launcherStatus.value = await upsertLauncher(draft)
      if (launcherStatus.value.lastSaveError) {
        showFeedback(`启动项保存失败: ${shortError(launcherStatus.value.lastSaveError)}`)
        return
      }
      const saved = launcherStatus.value.items.find((item) => item.id === draft.id) ?? launcherStatus.value.items.find((item) => item.name === draft.name && item.target === draft.target)
      if (saved) {
        editLauncher(saved)
      }
      showFeedback('启动项已保存')
    } catch {
      showFeedback('启动项保存失败')
    } finally {
      isSaving.value = false
    }
  }

  async function deleteLauncher() {
    if (!launcherDraft.value.id) {
      newLauncher()
      return
    }
    if (launcherDeleteArmedId.value !== launcherDraft.value.id) {
      launcherDeleteArmedId.value = launcherDraft.value.id
      showFeedback('再次点击确认删除启动项')
      return
    }
    isSaving.value = true
    try {
      launcherStatus.value = await removeLauncher(launcherDraft.value.id)
      if (launcherStatus.value.lastSaveError) {
        showFeedback(`启动项删除失败: ${shortError(launcherStatus.value.lastSaveError)}`)
        return
      }
      if (launcherStatus.value.items[0]) {
        editLauncher(launcherStatus.value.items[0])
      } else {
        newLauncher()
      }
      launcherDeleteArmedId.value = ''
      showFeedback('启动项已删除')
    } catch {
      showFeedback('启动项删除失败')
    } finally {
      isSaving.value = false
    }
  }

  async function clearSearchUsageState() {
    if (!searchUsageClearArmed.value) {
      searchUsageClearArmed.value = true
      showFeedback('再次点击确认清理搜索收藏和最近使用')
      return
    }
    isSaving.value = true
    try {
      searchUsageClearResult.value = await clearSearchUsage()
      searchUsageStatus.value = searchUsageClearResult.value.status
      showFeedback(searchUsageClearResult.value.message)
    } catch {
      showFeedback('搜索收藏和最近使用清理失败')
    } finally {
      searchUsageClearArmed.value = false
      isSaving.value = false
    }
  }

  async function installSearchService() {
    isInstallingSearchService.value = true
    try {
      fileSearchServiceActionResult.value = await installFileSearchService()
      await refreshPlatformStatus()
      showFeedback(fileSearchServiceActionResult.value.message)
    } catch {
      fileSearchServiceActionResult.value = {
        ok: false,
        message: '搜索服务安装失败',
      }
      showFeedback(fileSearchServiceActionResult.value.message)
    } finally {
      isInstallingSearchService.value = false
    }
  }

  async function saveSecret(kind: string) {
    const value = secretInputs.value[kind]?.trim() ?? ''
    if (!value) {
      showFeedback('请输入密钥后再保存')
      return
    }
    secretClearArmedKind.value = ''
    isSaving.value = true
    try {
      secretActionResult.value = await saveStoredSecret(kind, value)
      secretStatus.value = secretActionResult.value.status
      if (secretActionResult.value.ok) {
        secretInputs.value[kind] = ''
      }
      showFeedback(secretActionResult.value.message)
    } catch {
      showFeedback('密钥保存失败')
    } finally {
      isSaving.value = false
    }
  }

  async function clearSecret(kind: string) {
    const confirm = secretClearArmedKind.value === kind
    isSaving.value = true
    try {
      secretActionResult.value = await clearStoredSecret(kind, confirm)
      secretStatus.value = secretActionResult.value.status
      if (secretActionResult.value.requiresConfirmation) {
        secretClearArmedKind.value = kind
      } else {
        secretClearArmedKind.value = ''
      }
      showFeedback(secretActionResult.value.message)
    } catch {
      showFeedback('密钥清除失败')
    } finally {
      isSaving.value = false
    }
  }

  function launcherListText(path: 'keywords' | 'tags') {
    return launcherDraft.value[path]?.join('\n') ?? ''
  }

  function setLauncherList(path: 'keywords' | 'tags', value: string) {
    launcherDraft.value[path] = linesToList(value)
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 1800)
  }

  async function refreshPlatformStatus() {
    try {
      platformStatus.value = await getPlatformStatus()
    } catch {
      platformStatus.value = null
    }
  }

  return {
    settings,
    legacyStatus,
    legacyDataStatus,
    legacyImportResult,
    releaseBackupStatus,
    releaseBackupResult,
    releaseRestoreResult,
    diagnosticsExportResult,
    legacyHandoffResult,
    storageStatus,
    platformStatus,
    fileSearchServiceActionResult,
    searchUsageStatus,
    searchUsageClearResult,
    secretStatus,
    secretActionResult,
    pluginManifests,
    secretInputs,
    searchExcludeFoldersDraft,
    searchExcludePatternsDraft,
    launcherStatus,
    launcherDraft,
    launcherDeleteArmedId,
    searchUsageClearArmed,
    secretClearArmedKind,
    feedback,
    isLoading,
    isSaving,
    isMigrating,
    isCreatingRollbackCheckpoint,
    isRestoringRollbackCheckpoint,
    isExportingDiagnostics,
    isResolvingLegacyConflict,
    isInstallingSearchService,
    legacyHandoffMode,
    rollbackRestoreArmed,
    hasSettings,
    memorySources,
    visiblePluginManifests,
    enabledPluginCount,
    load,
    save,
    saveSearchSettings,
    reset,
    applyHotkeys,
    importLegacy,
    refreshLegacyDataStatus,
    importLegacyHistoryData,
    refreshPlatformStatus,
    createRollbackCheckpoint,
    restoreLatestRollbackCheckpoint,
    exportDiagnostics,
    resolveLegacyHandoff,
    updateWorkMemoryRuntime,
    setList,
    listText,
    setScreenshotRedactKeywords,
    screenshotRedactKeywordsText,
    setSearchExcludeFolders,
    searchExcludeFoldersText,
    setSearchExcludePatterns,
    searchExcludePatternsText,
    setMemorySource,
    pluginEnabled,
    setPluginEnabled,
    setHotkey,
    stageHotkey,
    normalizeHotkey,
    editLauncher,
    newLauncher,
    saveLauncher,
    deleteLauncher,
    clearSearchUsageState,
    installSearchService,
    saveSecret,
    clearSecret,
    launcherListText,
    setLauncherList,
    showFeedback,
  }
})

const hotkeyLabels: Record<keyof HotkeySettings, string> = {
  toggleWindow: '主窗口',
  screenshot: '截图',
  pinClipboard: '贴图',
}

const hotkeyModifierAliases: Record<string, 'ctrl' | 'alt' | 'shift' | 'win'> = {
  ctrl: 'ctrl',
  control: 'ctrl',
  alt: 'alt',
  option: 'alt',
  shift: 'shift',
  win: 'win',
  windows: 'win',
  meta: 'win',
  cmd: 'win',
  command: 'win',
}

const hotkeyKeyAliases: Record<string, string> = {
  spacebar: 'space',
  return: 'enter',
  esc: 'escape',
  del: 'delete',
}

function validateHotkeys(hotkeys: HotkeySettings) {
  for (const key of Object.keys(hotkeyLabels) as Array<keyof HotkeySettings>) {
    const normalized = normalizeHotkeyValue(hotkeys[key])
    if (!normalized.value) {
      return `${hotkeyLabels[key]}快捷键无效: ${normalized.error}`
    }
    hotkeys[key] = normalized.value
  }

  const seen = new Map<string, string>()
  for (const key of Object.keys(hotkeyLabels) as Array<keyof HotkeySettings>) {
    const existing = seen.get(hotkeys[key])
    if (existing) {
      return `${existing}和${hotkeyLabels[key]}快捷键不能相同`
    }
    seen.set(hotkeys[key], hotkeyLabels[key])
  }
  return ''
}

function normalizeAllHotkeys(hotkeys: HotkeySettings) {
  for (const key of Object.keys(hotkeyLabels) as Array<keyof HotkeySettings>) {
    const normalized = normalizeHotkeyValue(hotkeys[key])
    if (normalized.value) {
      hotkeys[key] = normalized.value
    }
  }
}

function normalizeHotkeyValue(value: string) {
  const raw = value.trim().toLowerCase()
  if (!raw) return { value: '', error: '不能为空' }

  const tokens = raw.split(/[+\s]+/).map((token) => token.trim()).filter(Boolean)
  const modifiers = new Set<'ctrl' | 'alt' | 'shift' | 'win'>()
  let keyName = ''

  for (const token of tokens) {
    const modifier = hotkeyModifierAliases[token]
    if (modifier) {
      modifiers.add(modifier)
      continue
    }
    if (keyName) {
      return { value: '', error: '只能包含一个主键' }
    }
    const normalizedKey = normalizeHotkeyMainKey(token)
    if (!normalizedKey) {
      return { value: '', error: `不支持 ${token}` }
    }
    keyName = normalizedKey
  }

  if (!keyName) return { value: '', error: '缺少主键' }
  if (!modifiers.size && !isBareFunctionHotkey(keyName)) return { value: '', error: '裸快捷键只支持 F1-F24，其他按键需要 ctrl / alt / shift / win 修饰键' }

  const parts: string[] = []
  for (const modifier of ['ctrl', 'alt', 'shift', 'win'] as const) {
    if (modifiers.has(modifier)) parts.push(modifier)
  }
  parts.push(keyName)
  return { value: parts.join('+'), error: '' }
}

function normalizeHotkeyMainKey(token: string) {
  const aliased = hotkeyKeyAliases[token] ?? token
  if (/^[a-z0-9]$/.test(aliased)) return aliased
  if (/^f([1-9]|1[0-9]|2[0-4])$/.test(aliased)) return aliased
  if (['space', 'tab', 'enter', 'escape', 'backspace', 'delete'].includes(aliased)) return aliased
  return ''
}

function isBareFunctionHotkey(key: string) {
  return /^f([1-9]|1[0-9]|2[0-4])$/.test(key)
}

function cloneLauncher(launcher: Launcher): Launcher {
  return {
    ...structuredClone(toRaw(launcher)),
    keywords: [...(launcher.keywords ?? [])],
    tags: [...(launcher.tags ?? [])],
  }
}

function linesToList(value: string) {
  const seen = new Set<string>()
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false
      const key = item.toLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}

function multilineToList(value: string) {
  const seen = new Set<string>()
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false
      const key = item.toLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}

function isStorageHealthy(status: SettingsStorageStatus) {
  return status.exists && status.readBackOk && !status.lastSaveError && !status.readBackError
}

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
