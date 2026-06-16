import type { AppToolView } from '../stores/appShell'

export interface ToolWindowOpenResult {
  ok: boolean
  message: string
  view?: string
}

export interface NetworkMiniStatus {
  anchor: string
  screenMode: string
  screenId?: string
  activeScreenId?: string
  screenName?: string
  screenLabel?: string
  screenCount: number
  screens?: NetworkMiniScreenStatus[]
  autoHideFullscreen: boolean
  fullscreenActive: boolean
  autoHidden: boolean
  visible: boolean
  locked: boolean
  configPath?: string
  lastError?: string
}

export interface NetworkMiniScreenStatus {
  id: string
  name: string
  primary: boolean
  x: number
  y: number
  width: number
  height: number
  workX: number
  workY: number
  workWidth: number
  workHeight: number
}

async function tryToolWindowBinding() {
  try {
    // @ts-expect-error Wails generated bindings are JavaScript-only, but this import must stay static for Vite packaging.
    return await import('../../bindings/ariadne/internal/toolwindows/service.js')
  } catch {
    return null
  }
}

export async function openToolWindow(view: AppToolView): Promise<ToolWindowOpenResult> {
  const binding = await tryToolWindowBinding()
  if (!binding) {
    return { ok: false, message: '工具窗口服务仅在桌面运行时可用', view }
  }
  return await binding.Open(view)
}

export async function showLauncherWindow(): Promise<ToolWindowOpenResult> {
  const binding = await tryToolWindowBinding()
  if (!binding) {
    return { ok: false, message: '启动器窗口服务仅在桌面运行时可用', view: 'launcher' }
  }
  return await binding.ShowLauncher()
}

export async function enableTaskbarToggle(view: AppToolView): Promise<ToolWindowOpenResult | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.EnableTaskbarToggle) return null
  return await binding.EnableTaskbarToggle(view)
}

export async function applyMainWindowPolicy(): Promise<ToolWindowOpenResult | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.ApplyMainWindowPolicy) return null
  return await binding.ApplyMainWindowPolicy()
}

export async function getNetworkMiniStatus(): Promise<NetworkMiniStatus | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.NetworkMiniStatus) return null
  return await binding.NetworkMiniStatus()
}

export async function setNetworkMiniAnchor(anchor: string): Promise<NetworkMiniStatus | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.SetNetworkMiniAnchor) return null
  return await binding.SetNetworkMiniAnchor(anchor)
}

export async function setNetworkMiniScreenMode(mode: string, screenId = ''): Promise<NetworkMiniStatus | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.SetNetworkMiniScreenMode) return null
  return await binding.SetNetworkMiniScreenMode(mode, screenId)
}

export async function setNetworkMiniAutoHideFullscreen(enabled: boolean): Promise<NetworkMiniStatus | null> {
  const binding = await tryToolWindowBinding()
  if (!binding?.SetNetworkMiniAutoHideFullscreen) return null
  return await binding.SetNetworkMiniAutoHideFullscreen(enabled)
}
