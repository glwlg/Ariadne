import type { AppSettings, LegacyConfigStatus, SettingsStorageStatus } from '../types/ariadne'

const fallbackSettings: AppSettings = {
  version: 9,
  general: {
    theme: 'light',
    runOnStartup: false,
    language: 'zh-CN',
  },
  hotkeys: {
    toggleWindow: 'alt+q',
    screenshot: 'alt+a',
    pinClipboard: 'alt+v',
  },
  screenshot: {
    autoCopy: false,
    autoPin: false,
    autoSave: false,
    saveDir: '~/Pictures/Ariadne',
    filenameTemplate: 'ariadne_{date}_{time}',
    quality: 90,
  },
  workMemory: {
    enabled: true,
    timeMachineEnabled: false,
    autoCaptureIntervalSeconds: 300,
    windowSwitchCaptureEnabled: false,
    windowSwitchCooldownSeconds: 30,
    captureScope: 'all_screens',
    screenshotQuality: 90,
    multiMonitor: 'combined',
    privacyMode: false,
    pauseOnIdle: true,
    idlePauseSeconds: 600,
    pauseOnLock: true,
    sourceClipboard: true,
    sourceCaptureHistory: true,
    sourceManualNote: true,
    sourceSearchFavorite: true,
    sourceActions: true,
    autoOcr: false,
    draftScheduleEnabled: false,
    draftScheduleIntervalMinutes: 240,
    dailyDraftScheduleEnabled: true,
    retrospectiveDraftScheduleEnabled: true,
    experienceScheduleEnabled: true,
    experienceDiscoveryEnabled: true,
    experienceDiscoveryDays: 7,
    skillSuggestionEnabled: true,
    workflowSuggestionEnabled: true,
    retentionDays: 30,
    thumbnailRetentionDays: 90,
    maxStorageMb: 1024,
    keepFavoritesForever: true,
    excludeApps: [
      '1password.exe',
      'bitwarden.exe',
      'keepass.exe',
      'lastpass.exe',
      'credentialuibroker.exe',
      'lockapp.exe',
      'logonui.exe',
      'mstsc.exe',
      'remotehelp.exe',
    ],
    excludeWindowKeywords: ['password', 'token', 'secret', '验证码', '密码', '登录', '支付', '隐私', '无痕', '远程桌面', '堡垒机', 'vpn', 'sso'],
    excludePaths: [],
    excludeUrls: [],
    excludeContentPatterns: [],
    sensitiveRulesEnabled: true,
    allowSensitiveExport: false,
  },
  ai: {
    enabled: false,
    provider: 'disabled',
    baseUrl: '',
    model: '',
    embeddingEnabled: false,
    embeddingProvider: 'disabled',
    embeddingBaseUrl: '',
    embeddingModel: '',
    vectorStoreType: 'disabled',
    vectorStoreUri: '',
    vectorCollection: 'ariadne_work_memory',
    agentsSdkEnabled: false,
    traceMode: 'off',
    opscoreSyncEnabled: false,
    externalAgentEnabled: true,
    codexCollaborationEnabled: false,
    externalAgentTaskDirectory: '~/Documents/Ariadne/agent_tasks',
  },
  plugins: {
    enabled: {},
  },
}

let fallbackCurrent = structuredClone(fallbackSettings)

async function trySettingsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/settings/service.js')
  } catch {
    return null
  }
}

export async function getSettings(): Promise<AppSettings> {
  const binding = await trySettingsBinding()
  if (binding) {
    try {
      return await binding.GetSettings()
    } catch {
      return structuredClone(fallbackCurrent)
    }
  }
  return structuredClone(fallbackCurrent)
}

export async function updateSettings(settings: AppSettings): Promise<AppSettings> {
  const binding = await trySettingsBinding()
  if (binding) {
    return await binding.UpdateSettings(toPlainSettings(settings))
  }
  fallbackCurrent = normalizeFallback(settings)
  return structuredClone(fallbackCurrent)
}

export async function resetSettings(): Promise<AppSettings> {
  const binding = await trySettingsBinding()
  if (binding) {
    return await binding.ResetSettings()
  }
  fallbackCurrent = structuredClone(fallbackSettings)
  return structuredClone(fallbackCurrent)
}

export async function getLegacyConfigStatus(): Promise<LegacyConfigStatus> {
  const binding = await trySettingsBinding()
  if (binding) {
    try {
      return await binding.LegacyConfigStatus()
    } catch {
      return fallbackLegacyStatus()
    }
  }
  return fallbackLegacyStatus()
}

export async function getSettingsStorageStatus(): Promise<SettingsStorageStatus> {
  const binding = await trySettingsBinding()
  if (binding) {
    try {
      return await binding.StorageStatus()
    } catch {
      return fallbackStorageStatus()
    }
  }
  return fallbackStorageStatus()
}

export async function importLegacyConfig(): Promise<AppSettings> {
  const binding = await trySettingsBinding()
  if (binding) {
    return await binding.ImportLegacyConfig()
  }
  return structuredClone(fallbackCurrent)
}

function fallbackLegacyStatus(): LegacyConfigStatus {
  return {
    path: '%APPDATA%/x-tools/config.json',
    exists: false,
    needsImport: false,
    importedKeys: [],
    notes: ['开发态 fallback 未读取本机旧配置；桌面构建会调用 Go settings 服务。'],
  }
}

function fallbackStorageStatus(): SettingsStorageStatus {
  return {
    path: '%APPDATA%/Ariadne/config.json',
    directory: '%APPDATA%/Ariadne',
    directoryExists: false,
    exists: false,
    bytes: 0,
    readBackOk: false,
    readBackBytes: 0,
    readBackVersion: 0,
    entries: [],
    virtualizedPath: '',
    virtualizedExists: false,
    virtualizedBytes: 0,
    appDataEnv: '%APPDATA%',
    localAppDataEnv: '%LOCALAPPDATA%',
    userConfigDir: '%APPDATA%',
    workingDir: '',
    executablePath: '',
    lastSaveError: '开发态 fallback 未写入 Go 配置文件。',
  }
}

function normalizeFallback(settings: AppSettings): AppSettings {
  const next = structuredClone(settings)
  next.version = Math.max(next.version || 0, fallbackSettings.version)
  next.general.theme = next.general.theme === 'dark' ? 'dark' : 'light'
  next.screenshot.quality = clamp(next.screenshot.quality, 1, 100)
  next.workMemory.autoCaptureIntervalSeconds = clamp(next.workMemory.autoCaptureIntervalSeconds, 10, 86400)
  next.workMemory.screenshotQuality = clamp(next.workMemory.screenshotQuality, 1, 100)
  next.workMemory.idlePauseSeconds = clamp(next.workMemory.idlePauseSeconds, 30, 86400)
  next.workMemory.draftScheduleIntervalMinutes = clamp(next.workMemory.draftScheduleIntervalMinutes, 15, 1440)
  next.workMemory.experienceDiscoveryDays = clamp(next.workMemory.experienceDiscoveryDays, 1, 365)
  next.workMemory.retentionDays = clamp(next.workMemory.retentionDays, 1, 3650)
  next.workMemory.thumbnailRetentionDays = clamp(next.workMemory.thumbnailRetentionDays, 1, 3650)
  next.workMemory.maxStorageMb = clamp(next.workMemory.maxStorageMb, 128, 1024 * 1024)
  next.workMemory.excludeApps = cleanList(next.workMemory.excludeApps)
  next.workMemory.excludeWindowKeywords = cleanList(next.workMemory.excludeWindowKeywords)
  next.workMemory.excludePaths = cleanList(next.workMemory.excludePaths)
  next.workMemory.excludeUrls = cleanList(next.workMemory.excludeUrls)
  next.workMemory.excludeContentPatterns = cleanList(next.workMemory.excludeContentPatterns)
  next.ai.traceMode = ['off', 'local', 'internal'].includes(next.ai.traceMode) ? next.ai.traceMode : 'off'
  return next
}

function cleanList(items: string[] = []) {
  const seen = new Set<string>()
  return items
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false
      const key = item.toLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}

function clamp(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min
  return Math.max(min, Math.min(max, Math.round(value)))
}

function toPlainSettings(settings: AppSettings): AppSettings {
  return JSON.parse(JSON.stringify(settings)) as AppSettings
}
