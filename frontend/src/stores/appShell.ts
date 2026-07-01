import { ref } from 'vue'
import { defineStore } from 'pinia'
import { Events, Window } from '@wailsio/runtime'
import { launcherGeometry } from '../lib/launcherGeometry'
import { enableTaskbarToggle, openToolWindow, showLauncherWindow } from '../services/toolWindowsApi'

export type AppToolView =
  | 'work-memory'
  | 'clipboard'
  | 'capture'
  | 'hosts'
  | 'workflow'
  | 'json-compare'
  | 'api-testing'
  | 'network-monitor'
  | 'network-mini'
  | 'settings'
export type AppView = 'launcher' | AppToolView

const viewSizes: Record<AppView, { width: number; height: number }> = {
  launcher: { width: launcherGeometry.width, height: launcherGeometry.collapsedHeight },
  'work-memory': { width: 1120, height: 720 },
  clipboard: { width: 1120, height: 720 },
  capture: { width: 1120, height: 720 },
  hosts: { width: 1120, height: 720 },
  workflow: { width: 1120, height: 720 },
  'json-compare': { width: 1180, height: 760 },
  'api-testing': { width: 1240, height: 780 },
  'network-monitor': { width: 980, height: 640 },
  'network-mini': { width: 318, height: 168 },
  settings: { width: 1120, height: 720 },
}

const viewTitles: Record<AppView, string> = {
  launcher: 'Ariadne',
  'work-memory': 'Ariadne - 心流',
  clipboard: 'Ariadne - 剪贴板历史',
  capture: 'Ariadne - 捕获历史',
  hosts: 'Ariadne - Hosts',
  workflow: 'Ariadne - 工作流',
  'json-compare': 'Ariadne - JSON 对比',
  'api-testing': 'Ariadne - API 测试',
  'network-monitor': 'Ariadne - 网络监控',
  'network-mini': 'Ariadne - 网速小窗',
  settings: 'Ariadne - 设置',
}

export const useAppShellStore = defineStore('app-shell', () => {
  const activeView = ref<AppView>('work-memory')

  async function setWindowSize(width: number, height: number, resizable = true, frameless = true) {
    try {
      await Window.Restore()
      await Window.SetFrameless(frameless)
      await Window.SetResizable(resizable)
      if (resizable) {
        await Window.SetMinSize(Math.min(width, 900), Math.min(height, 620))
      }
      await Window.SetSize(width, height)
      await Window.Center()
    } catch {
      // Runtime calls are unavailable in browser-only dev mode.
    }
  }

  function openLauncher() {
    if (isLauncherWindow()) {
      activeView.value = 'launcher'
      document.title = viewTitles.launcher
      window.setTimeout(() => window.dispatchEvent(new CustomEvent('ariadne:focus-launcher', { detail: { selectAll: true } })), 0)
      return
    }
    void showLauncherWindow()
  }

  function activateMainView(view: AppView) {
    activeView.value = view
    document.title = viewTitles[activeView.value]
  }

  function openWorkMemory() {
    return openView('work-memory')
  }

  function openClipboard() {
    return openView('clipboard')
  }

  function openCaptureHistory() {
    return openView('capture')
  }

  function openHosts() {
    return openView('hosts')
  }

  function openWorkflow() {
    return openView('workflow')
  }

  function openJsonCompare() {
    return openView('json-compare')
  }

  function openAPITesting() {
    return openView('api-testing')
  }

  function openNetworkMonitor() {
    return openView('network-monitor')
  }

  function openNetworkMini() {
    return openView('network-mini')
  }

  function openSettings() {
    return openView('settings')
  }

  async function closeCurrentWindow() {
    if (isStandaloneToolWindow()) {
      try {
        await Window.Close()
        return
      } catch {
        return
      }
    }
    openLauncher()
  }

  function openView(view: AppView) {
    if (view === 'launcher') {
      openLauncher()
      return Promise.resolve(true)
    }
    return openToolView(view)
  }

  async function openToolView(view: AppToolView) {
    const result = await openToolWindow(view)
    if (result.ok) {
      if (isLauncherWindow()) {
        void Window.Hide()
      }
      return true
    }
    if (import.meta.env.DEV) {
      openViewFallback(view)
      return true
    }
    console.warn(`Ariadne tool window failed: ${result.message}`)
    return false
  }

  function openViewFallback(view: AppView) {
    activeView.value = view
    document.title = viewTitles[view]
    const size = viewSizes[view]
    void setWindowSize(size.width, size.height, view !== 'network-mini', view === 'launcher' || view === 'network-mini').then(() => {
      if (view !== 'launcher' && view !== 'network-mini') {
        void enableTaskbarToggle(view)
      }
    })
  }

  function installShellEventListeners() {
    const handleNavigate = (view: string) => {
      if (!isAppView(view)) return
      if (view === 'launcher') {
        openLauncher()
      } else if (view === 'work-memory' && !isStandaloneToolWindow()) {
        activateMainView('work-memory')
      } else {
        void openToolView(view)
      }
    }
    const handleDomNavigate = (event: Event) => {
      handleNavigate(String((event as CustomEvent).detail ?? ''))
    }

    window.addEventListener('ariadne:navigate', handleDomNavigate)
    let uninstallWailsEvent = () => {}
    try {
      uninstallWailsEvent = Events.On('ariadne:navigate', (event) => {
        handleNavigate(String(event.data ?? ''))
      })
    } catch {
      uninstallWailsEvent = () => {}
    }

    return () => {
      uninstallWailsEvent()
      window.removeEventListener('ariadne:navigate', handleDomNavigate)
    }
  }

  function isAppView(view: string): view is AppView {
    return view in viewSizes
  }

  function isStandaloneToolWindow() {
    const params = new URLSearchParams(window.location.search)
    const view = params.get('view') ?? ''
    return view !== '' && view !== 'pinned-image' && view !== 'capture-overlay'
  }

  function isLauncherWindow() {
    const params = new URLSearchParams(window.location.search)
    return params.get('view') === 'launcher'
  }

  return {
    activeView,
    activateMainView,
    openView,
    openLauncher,
    openWorkMemory,
    openClipboard,
    openCaptureHistory,
    openHosts,
    openWorkflow,
    openJsonCompare,
    openAPITesting,
    openNetworkMonitor,
    openNetworkMini,
    openSettings,
    closeCurrentWindow,
    installShellEventListeners,
  }
})
