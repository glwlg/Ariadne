import type { AppSettings, LegacyConfigStatus, SettingsStorageStatus } from '../types/ariadne'

const fallbackSettings: AppSettings = {
  version: 23,
  general: {
    theme: 'cloud-blue',
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
    autoRedact: false,
    redactPhones: true,
    redactKeywords: [],
  },
  history: {
    clipboardEnabled: true,
    privacyMode: false,
    retentionDays: 30,
    maxStorageMb: 1024,
    keepFavoritesForever: true,
  },
  ai: {
    ocrModelEnabled: false,
    ocrProvider: 'openai-compatible',
    ocrBaseUrl: '',
    ocrModel: '',
  },
  plugins: {
    enabled: {},
  },
  search: {
    fileExcludeFolders: [
      '%APPDATA%\\Microsoft\\Windows\\Recent',
      '%APPDATA%',
      '%TEMP%',
      '%TMP%',
      '%LOCALAPPDATA%',
      '%USERPROFILE%\\AppData\\LocalLow',
      '%LOCALAPPDATA%\\Temp',
      '%WINDIR%\\Temp',
      '%WINDIR%',
      '%SystemRoot%',
      '%ProgramFiles%',
      '%ProgramFiles(x86)%',
      '%ProgramW6432%',
      '%ProgramData%',
    ],
    fileExcludePatterns: [
      String.raw`(?i)\.(tmp|temp|part|crdownload|download)$`,
      String.raw`(?i)(^|[\\/])~\$[^\\/]*$`,
      String.raw`(?i)(^|[\\/])\$recycle\.bin([\\/]|$)`,
      String.raw`(?i)(^|[\\/])system volume information([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(tmp|temp|temp-index|tmp-index)([\\/]|$)`,
      String.raw`(?i)[\\/]ebwebview([\\/]|$)`,
      String.raw`(?i)(^|[\\/])\..*\.tmp[-.][^\\/]*$`,
      String.raw`(?i)[\\/]users[\\/][^\\/]+[\\/]appdata[\\/](local|locallow|roaming)([\\/]|$)`,
      String.raw`(?i)^[a-z]:[\\/](app|appdata|program files|program files \(x86\)|programdata|devapp)([\\/]|$)`,
      String.raw`(?i)^[a-z]:[\\/]workspace[\\/]env([\\/]|$)`,
      String.raw`(?i)^[a-z]:[\\/]users[\\/][^\\/]+[\\/]\.(cache|config|local|m2|gradle|nuget|npm|pnpm|yarn|cargo|rustup|wox|gemini|confirmo|switchhosts|antigravity|antigravity_cockpit|marscode)([\\/]|$)`,
      String.raw`(?i)[\\/]appdata[\\/](local|locallow|roaming)[\\/].*[\\/](cache|cache2|cachestorage|code cache|gpucache|shadercache|crashpad|dawncache|blob_storage|inetcache|webcache|startupcache|browsermetrics|media cache|service worker|thumbnails)([\\/]|$)`,
      String.raw`(?i)^[a-z]:[\\/](recovery|\$winreagent|config\.msi|windows\.old|msocache)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(d3dscache|dxcache|gpucache|grshadercache|shadercache|dawncache)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(\.m2|\.npm|\.pnpm-store|pnpm-store|npm-cache|pnpm-cache|go-build|pip-cache|ms-playwright|ms-playwright-go|package cache)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(nuget[\\/]packages|\.cargo|\.rustup)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(\.venv|venv|envs|virtualenv|__pypackages__)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(site-packages|dist-packages)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(jetbrains|trae cn|trae|antigravity|zed|qoder)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])(logs?|log)([\\/]|$)`,
      String.raw`(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`,
      String.raw`(?i)\.(log|journal|db-journal|sqlite-wal|sqlite-shm|db-wal|db-shm|odlgz|statistic|dxcache-shm|dxcache-wal)$`,
      String.raw`(?i)(^|[\\/])(\$mft|\$logfile|\$bitmap|\$boot|\$badclus|\$secure|\$upcase|\$volume|\$attrdef|pagefile\.sys|hiberfil\.sys|swapfile\.sys|dumpstack\.log\.tmp|thumbs\.db|desktop\.ini)$`,
      String.raw`(?i)(^|[\\/])(\.git|\.hg|\.svn|node_modules|\.pnpm-store|__pycache__|\.pytest_cache|\.ruff_cache|\.mypy_cache|\.gradle|\.idea|\.vscode|\.cache|\.codex|\.codex-audit|coverage|dist|build|target|out|bin|obj|\.next|\.nuxt|\.vite|\.turbo|\.parcel-cache|\.svelte-kit|\.angular|\.vercel)([\\/]|$)`,
    ],
    fileIncludeExtensions: [],
    fileExcludeExtensions: [
      '.tmp',
      '.temp',
      '.part',
      '.crdownload',
      '.download',
      '.log',
      '.journal',
      '.db-journal',
      '.sqlite-wal',
      '.sqlite-shm',
      '.db-wal',
      '.db-shm',
      '.odlgz',
      '.statistic',
      '.dxcache-shm',
      '.dxcache-wal',
    ],
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
    path: '%APPDATA%/Ariadne/ariadne.sqlite',
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
  const previousVersion = next.version || 0
  next.version = Math.max(next.version || 0, fallbackSettings.version)
  next.general.theme = normalizeTheme(next.general.theme)
  next.screenshot.quality = clamp(next.screenshot.quality, 1, 100)
  next.screenshot.autoRedact = Boolean(next.screenshot.autoRedact)
  next.screenshot.redactPhones = next.screenshot.redactPhones !== false
  next.screenshot.redactKeywords = cleanList(next.screenshot.redactKeywords)
  if (!next.search) {
    next.search = structuredClone(fallbackSettings.search)
  }
  if (previousVersion < 20) {
    next.search.fileExcludeFolders = [...fallbackSettings.search.fileExcludeFolders, ...(next.search.fileExcludeFolders || [])]
    next.search.fileExcludePatterns = [...fallbackSettings.search.fileExcludePatterns, ...(next.search.fileExcludePatterns || [])]
  }
  if (previousVersion < 21) {
    next.search.fileExcludeExtensions = [...fallbackSettings.search.fileExcludeExtensions, ...(next.search.fileExcludeExtensions || [])]
  }
  next.search.fileExcludeFolders = cleanList(next.search.fileExcludeFolders)
  next.search.fileExcludePatterns = cleanList(next.search.fileExcludePatterns)
  next.search.fileIncludeExtensions = cleanExtensionList(next.search.fileIncludeExtensions)
  next.search.fileExcludeExtensions = cleanExtensionList(next.search.fileExcludeExtensions)
  return next
}

function normalizeTheme(theme: string): AppSettings['general']['theme'] {
  if (theme === 'professional-pink' || theme === 'light-graphite' || theme === 'cloud-blue') {
    return theme
  }
  return 'cloud-blue'
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

function cleanExtensionList(items: string[] = []) {
  const seen = new Set<string>()
  return items
    .map((item) => normalizeExtension(item))
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function normalizeExtension(value: string) {
  let item = String(value || '').trim().toLowerCase()
  if (item.startsWith('*')) item = item.slice(1).trim()
  if (!item || item === '.') return ''
  if (!item.startsWith('.')) item = `.${item}`
  if (item.includes('/') || item.includes('\\') || item.includes(' ')) return ''
  return item
}



function clamp(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min
  return Math.max(min, Math.min(max, Math.round(value)))
}

function toPlainSettings(settings: AppSettings): AppSettings {
  return JSON.parse(JSON.stringify(settings)) as AppSettings
}
