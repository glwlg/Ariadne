import { ref } from 'vue'
import { defineStore } from 'pinia'
import { Events, Window } from '@wailsio/runtime'
import { launcherGeometry } from '../lib/launcherGeometry'
import { enableTaskbarToggle, openToolWindow, showLauncherWindow } from '../services/toolWindowsApi'
import { isToolView, toolViews, type AppToolView } from '../lib/toolWindows'
export type { AppToolView } from '../lib/toolWindows'
export type AppView = 'launcher' | AppToolView

const viewSizes = {
  launcher: { width: launcherGeometry.width, height: launcherGeometry.collapsedHeight, minWidth: 0, minHeight: 0 },
  ...toolViews,
}

function viewTitle(view: AppView) {
  return view === 'launcher' ? 'Ariadne' : toolViews[view].title
}

export const useAppShellStore = defineStore('app-shell', () => {
  const activeView = ref<AppView>('launcher')

  async function setWindowSize(width: number, height: number, resizable = true, frameless = true, minWidth = 900, minHeight = 620) {
    try {
      await Window.Restore()
      await Window.SetFrameless(frameless)
      await Window.SetResizable(resizable)
      if (resizable) {
        await Window.SetMinSize(minWidth, minHeight)
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
      document.title = viewTitle('launcher')
      window.setTimeout(() => window.dispatchEvent(new CustomEvent('ariadne:focus-launcher', { detail: { selectAll: true } })), 0)
      return
    }
    void showLauncherWindow()
  }

  function activateMainView(view: AppView) {
    activeView.value = view
    document.title = viewTitle(activeView.value)
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

  function openAirPlay() {
    return openView('airplay')
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
    document.title = viewTitle(view)
    const size = viewSizes[view]
    void setWindowSize(size.width, size.height, view !== 'network-mini', view === 'launcher' || view === 'network-mini', size.minWidth, size.minHeight).then(() => {
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
    return view === 'launcher' || isToolView(view)
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
    openClipboard,
    openCaptureHistory,
    openHosts,
    openWorkflow,
    openJsonCompare,
    openAPITesting,
    openAirPlay,
    openNetworkMonitor,
    openNetworkMini,
    openSettings,
    closeCurrentWindow,
    installShellEventListeners,
  }
})
