import type { AppUpdateResult, AppUpdateStatus } from '../types/fancun.ts'
import { formatRpcError, isDesktopRuntime } from './runtimeEnv.ts'

async function tryAppUpdateBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/fancun/internal/appupdate/service.js')
  } catch {
    return null
  }
}

export async function getAppUpdateStatus(): Promise<AppUpdateStatus> {
  const binding = await tryAppUpdateBinding()
  if (!binding?.Status) {
    if (isDesktopRuntime()) {
      throw new Error('应用更新服务绑定不可用')
    }
    return fallbackAppUpdateStatus()
  }
  try {
    return normalizeAppUpdateStatus(await binding.Status())
  } catch (error) {
    if (isDesktopRuntime()) {
      throw new Error(formatRpcError(error, '读取更新状态失败'))
    }
    return fallbackAppUpdateStatus('读取更新状态失败')
  }
}

export async function checkForAppUpdates(): Promise<AppUpdateResult> {
  const binding = await tryAppUpdateBinding()
  if (!binding?.CheckForUpdates) {
    if (isDesktopRuntime()) {
      throw new Error('应用更新服务绑定不可用')
    }
    return fallbackAppUpdateResult('开发态未连接应用更新服务')
  }
  try {
    return normalizeAppUpdateResult(await binding.CheckForUpdates())
  } catch (error) {
    return fallbackAppUpdateResult(formatRpcError(error, '检查应用更新失败'))
  }
}

export async function downloadAndLaunchAppUpdate(): Promise<AppUpdateResult> {
  const binding = await tryAppUpdateBinding()
  if (!binding?.DownloadAndLaunchInstaller) {
    if (isDesktopRuntime()) {
      throw new Error('应用更新服务绑定不可用')
    }
    return fallbackAppUpdateResult('开发态未连接应用更新服务')
  }
  try {
    return normalizeAppUpdateResult(await binding.DownloadAndLaunchInstaller())
  } catch (error) {
    return fallbackAppUpdateResult(formatRpcError(error, '下载或启动更新安装器失败'))
  }
}

export function normalizeAppUpdateStatus(source: Partial<AppUpdateStatus>): AppUpdateStatus {
  return {
    currentVersion: String(source.currentVersion || 'dev'),
    state: String(source.state || 'unconfigured'),
    enabled: Boolean(source.enabled),
    canCheck: Boolean(source.canCheck),
    canInstall: Boolean(source.canInstall),
    updateAvailable: Boolean(source.updateAvailable),
    availableVersion: source.availableVersion ? String(source.availableVersion) : undefined,
    releaseName: source.releaseName ? String(source.releaseName) : undefined,
    releaseNotes: source.releaseNotes ? String(source.releaseNotes) : undefined,
    artifactName: source.artifactName ? String(source.artifactName) : undefined,
    downloadedPath: source.downloadedPath ? String(source.downloadedPath) : undefined,
    installerLaunched: Boolean(source.installerLaunched),
    message: source.message ? String(source.message) : undefined,
    lastError: source.lastError ? String(source.lastError) : undefined,
  }
}

function normalizeAppUpdateResult(source: AppUpdateResult): AppUpdateResult {
  return {
    ok: Boolean(source?.ok),
    message: String(source?.message || ''),
    status: normalizeAppUpdateStatus(source?.status || {}),
  }
}

function fallbackAppUpdateStatus(message = '开发构建未启用应用更新'): AppUpdateStatus {
  return normalizeAppUpdateStatus({ message })
}

function fallbackAppUpdateResult(message: string): AppUpdateResult {
  return {
    ok: false,
    message,
    status: fallbackAppUpdateStatus(message),
  }
}
