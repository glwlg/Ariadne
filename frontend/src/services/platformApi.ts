import type { ActionResult, DiagnosticsExportResult, LegacyHandoffRequest, LegacyHandoffResult, PlatformStatus } from '../types/ariadne'

export async function getPlatformStatus(): Promise<PlatformStatus> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/platform/service.js')
    return await binding.Status()
  } catch {
    return fallbackPlatformStatus()
  }
}

export async function exportDiagnosticsBundle(): Promise<DiagnosticsExportResult> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/platform/service.js')
    return await binding.ExportDiagnostics()
  } catch {
    return {
      ok: false,
      message: '开发态 fallback 未接入诊断包导出',
      logIncluded: false,
    }
  }
}

export async function installFileSearchService(): Promise<ActionResult> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/platform/service.js')
    return await binding.InstallFileSearchService()
  } catch {
    return {
      ok: false,
      message: '开发态 fallback 未接入搜索服务安装',
    }
  }
}

export async function resolveLegacyConflict(request: LegacyHandoffRequest): Promise<LegacyHandoffResult> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/platform/service.js')
    return await binding.ResolveLegacyConflict(request)
  } catch {
    return {
      ok: false,
      message: '开发态 fallback 未接入旧版交接',
      before: fallbackPlatformStatus().legacyRuntime,
      after: fallbackPlatformStatus().legacyRuntime,
      shell: {
        singleInstanceConfigured: false,
        trayConfigured: false,
        globalHotkeyRegistered: false,
        globalHotkey: 'Alt+Q',
        screenshotHotkeyRegistered: false,
        screenshotHotkey: 'Alt+A',
        pinClipboardHotkeyRegistered: false,
        pinClipboardHotkey: 'Alt+V',
        autostartSupported: false,
        autostartEnabled: false,
        autostartPath: '',
        autostartIdentifier: 'com.glwlg.ariadne',
        autostartValueName: '',
        autostartCommand: '',
        autostartCommandValid: false,
        autostartHiddenArgPresent: false,
        autostartNotes: ['frontend fallback'],
        lastError: 'frontend fallback',
      },
      actions: ['开发态 fallback 不会关闭任何进程'],
      requiresConfirmation: false,
      forceUsed: false,
      hotkeyRetried: false,
      createdAt: Math.floor(Date.now() / 1000),
    }
  }
}

function fallbackPlatformStatus(): PlatformStatus {
  return {
    appName: 'Ariadne',
    legacyName: 'x-tools',
    capabilities: [
      {
        id: 'preview_actions',
        enabled: true,
        provider: 'frontend fallback',
        note: '开发态 fallback 只验证前端协议形态。',
      },
      {
        id: 'settings',
        enabled: false,
        provider: 'frontend fallback',
        note: '桌面构建会调用 Go settings/platform 服务。',
      },
      {
        id: 'file_search',
        enabled: false,
        provider: 'Ariadne USN/MFT',
        note: '开发态 fallback 未接入文件索引。',
      },
      {
        id: 'json_compare',
        enabled: true,
        provider: 'frontend fallback',
        note: '开发态 fallback 可打开 JSON 对比页；桌面构建使用 Go 语义 diff 服务。',
      },
      {
        id: 'legacy_coexistence',
        enabled: true,
        provider: 'frontend fallback',
        note: '桌面构建会检查旧版 x-tools 进程与热键冲突。',
      },
    ],
    diagnostics: {
      os: 'browser',
      arch: 'unknown',
      goVersion: '',
      processId: 0,
      workingDir: '',
      executablePath: '',
      executableBytes: 0,
      appDataEnv: '',
      localAppDataEnv: '',
      goToolPath: '',
      wailsToolPath: '',
    },
    shell: {
      singleInstanceConfigured: false,
      trayConfigured: false,
      globalHotkeyRegistered: false,
      globalHotkey: 'Alt+Q',
      screenshotHotkeyRegistered: false,
      screenshotHotkey: 'Alt+A',
      pinClipboardHotkeyRegistered: false,
      pinClipboardHotkey: 'Alt+V',
      autostartSupported: false,
      autostartEnabled: false,
      autostartPath: '',
      autostartIdentifier: 'com.glwlg.ariadne',
      autostartValueName: '',
      autostartCommand: '',
      autostartCommandValid: false,
      autostartHiddenArgPresent: false,
      autostartNotes: ['frontend fallback'],
      lastError: 'frontend fallback',
    },
    legacyRuntime: {
      processRunning: false,
      configPath: '%APPDATA%/x-tools/config.json',
      configExists: false,
      hotkeyConflictLikely: false,
      notes: ['开发态 fallback 未检查旧版运行时。'],
    },
    searchPerformance: {
      sampleCount: 0,
      targetP95Ms: 100,
      lastElapsedMs: 0,
      lastResultCount: 0,
      averageMs: 0,
      p95Ms: 0,
      maxMs: 0,
      withinTarget: true,
    },
    fileSearch: {
      dllPath: '',
      dllFound: false,
      ready: false,
      provider: 'Ariadne USN/MFT',
      serviceName: 'AriadneFileSearch',
      serviceInstalled: false,
      serviceRunning: false,
      serviceState: '',
      indexing: false,
      indexedCount: 0,
      volumeCount: 0,
      requiresAdmin: false,
      elevated: false,
      lastElapsedMs: 0,
      lastResultCount: 0,
      lastError: '开发态 fallback 未接入文件索引。',
      coverageHint: '开发态 fallback 不执行文件索引查询；桌面构建会显示索引覆盖提示。',
      policyErrors: [],
    },
    logs: {
      path: '',
      directory: '',
      directoryExists: false,
      exists: false,
      bytes: 0,
      lastError: '开发态 fallback 未接入本地日志。',
    },
    metrics: [],
  }
}
