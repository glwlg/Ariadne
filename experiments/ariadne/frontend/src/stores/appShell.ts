import { ref } from 'vue'
import { defineStore } from 'pinia'
import { Events, Window } from '@wailsio/runtime'
import { applyLauncherWindowGeometry, launcherGeometry } from '../lib/launcherGeometry'
import { openToolWindow, showLauncherWindow } from '../services/toolWindowsApi'

export type AppToolView =
  | 'work-memory'
  | 'clipboard'
  | 'capture'
  | 'hosts'
  | 'workflow'
  | 'json-compare'
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
  'network-monitor': { width: 980, height: 640 },
  'network-mini': { width: 318, height: 168 },
  settings: { width: 1120, height: 720 },
}

const viewTitles: Record<AppView, string> = {
  launcher: 'Ariadne',
  'work-memory': 'Ariadne - 工作记忆',
  clipboard: 'Ariadne - 剪贴板历史',
  capture: 'Ariadne - 捕获历史',
  hosts: 'Ariadne - Hosts',
  workflow: 'Ariadne - 工作流',
  'json-compare': 'Ariadne - JSON 对比',
  'network-monitor': 'Ariadne - 网络监控',
  'network-mini': 'Ariadne - 网速小窗',
  settings: 'Ariadne - 设置',
}

export const useAppShellStore = defineStore('app-shell', () => {
  const activeView = ref<AppView>('launcher')

  async function setWindowSize(width: number, height: number) {
    try {
      await Window.Restore()
      await Window.SetFrameless(true)
      await Window.SetSize(width, height)
      await Window.Center()
    } catch {
      // Runtime calls are unavailable in browser-only dev mode.
    }
  }

  function openLauncher() {
    if (isStandaloneToolWindow()) {
      void showLauncherWindow().finally(() => {
        void Window.Close()
      })
      return
    }

    activeView.value = 'launcher'
    document.title = viewTitles.launcher
    void applyLauncherWindowGeometry(false, { restore: true, reservePosition: true })
    window.setTimeout(() => window.dispatchEvent(new CustomEvent('ariadne:focus-launcher', { detail: { reset: true } })), 0)
  }

  function openWorkMemory() {
    openView('work-memory')
  }

  function openClipboard() {
    openView('clipboard')
  }

  function openCaptureHistory() {
    openView('capture')
  }

  function openHosts() {
    openView('hosts')
  }

  function openWorkflow() {
    openView('workflow')
  }

  function openJsonCompare() {
    openView('json-compare')
  }

  function openNetworkMonitor() {
    openView('network-monitor')
  }

  function openNetworkMini() {
    openView('network-mini')
  }

  function openSettings() {
    openView('settings')
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
      return
    }
    openToolView(view)
  }

  function openToolView(view: AppToolView) {
    void openToolWindow(view).then((result) => {
      if (result.ok) {
        if (!isStandaloneToolWindow()) {
          void Window.Hide()
        }
        return
      }
      openViewFallback(view)
    })
  }

  function openViewFallback(view: AppView) {
    activeView.value = view
    document.title = viewTitles[view]
    const size = viewSizes[view]
    void setWindowSize(size.width, size.height)
  }

  function installShellEventListeners() {
    const handleNavigate = (view: string) => {
      if (!isAppView(view)) return
      if (view === 'launcher') {
        openLauncher()
      } else {
        openToolView(view)
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

  return {
    activeView,
    openView,
    openLauncher,
    openWorkMemory,
    openClipboard,
    openCaptureHistory,
    openHosts,
    openWorkflow,
    openJsonCompare,
    openNetworkMonitor,
    openNetworkMini,
    openSettings,
    closeCurrentWindow,
    installShellEventListeners,
  }
})
