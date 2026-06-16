<script setup lang="ts">
import AriadneLauncher from './components/launcher/AriadneLauncher.vue'
import CaptureOverlayWindow from './components/capture/CaptureOverlayWindow.vue'
import CaptureHistoryCenter from './components/capture/CaptureHistoryCenter.vue'
import ClipboardCenter from './components/clipboard/ClipboardCenter.vue'
import HostsCenter from './components/hosts/HostsCenter.vue'
import JsonCompareCenter from './components/jsoncompare/JsonCompareCenter.vue'
import NetworkMonitorCenter from './components/network/NetworkMonitorCenter.vue'
import NetworkMiniWindow from './components/network/NetworkMiniWindow.vue'
import PinnedImageWindow from './components/pinned/PinnedImageWindow.vue'
import SettingsCenter from './components/settings/SettingsCenter.vue'
import WorkMemoryCenter from './components/workmemory/WorkMemoryCenter.vue'
import WorkflowCenter from './components/workflows/WorkflowCenter.vue'
import { installSystemThemeListener, syncThemeFromSettings } from './lib/theme'
import { applyMainWindowPolicy, enableTaskbarToggle } from './services/toolWindowsApi'
import { useAppShellStore, type AppToolView } from './stores/appShell'
import { Window } from '@wailsio/runtime'
import { onMounted, onUnmounted } from 'vue'

const appShell = useAppShellStore()
const routeParams = new URLSearchParams(window.location.search)
const routeView = routeParams.get('view') ?? ''
const isLauncherWindow = routeView === 'launcher'
const isPinnedImageWindow = routeView === 'pinned-image'
const isCaptureOverlayWindow = routeView === 'capture-overlay'
const standaloneToolView = isStandaloneToolView(routeView) ? routeView : ''
const pinId = routeParams.get('pinId') ?? ''
const captureOverlaySessionId = routeParams.get('sessionId') ?? ''
const documentClass = isPinnedImageWindow
  ? 'pinned-image-document'
  : isCaptureOverlayWindow
    ? 'capture-overlay-document'
    : isLauncherWindow
      ? 'launcher-document'
      : standaloneToolView
      ? 'tool-window-document'
      : 'tool-window-document'
let uninstallShellEvents: (() => void) | null = null
let uninstallThemeEvents: (() => void) | null = null

onMounted(() => {
  document.documentElement.classList.add(documentClass)
  if (isPinnedImageWindow || isCaptureOverlayWindow) {
    void ensureUtilityWindow({
      alwaysOnTop: true,
      transparent: isPinnedImageWindow,
      frameless: true,
      resizable: false,
    })
    void syncThemeFromSettings()
    uninstallThemeEvents = installSystemThemeListener()
    return
  }
  if (standaloneToolView) {
    void ensureToolWindowMode(standaloneToolView)
    void syncThemeFromSettings()
    uninstallThemeEvents = installSystemThemeListener()
    return
  }
  if (isLauncherWindow) {
    void ensureLauncherWindow()
    void syncThemeFromSettings()
    uninstallThemeEvents = installSystemThemeListener()
    window.setTimeout(() => window.dispatchEvent(new CustomEvent('ariadne:focus-launcher', { detail: { reset: true } })), 0)
    return
  }
  void ensureMainWindow()
  void syncThemeFromSettings()
  uninstallThemeEvents = installSystemThemeListener()
  uninstallShellEvents = appShell.installShellEventListeners()
  appShell.activateMainView('work-memory')
})

onUnmounted(() => {
  uninstallShellEvents?.()
  uninstallThemeEvents?.()
  document.documentElement.classList.remove(documentClass)
})

async function ensureLauncherWindow() {
  try {
    await Window.SetFrameless(true)
    await Window.SetResizable(false)
    await Window.SetAlwaysOnTop(false)
    await Window.SetBackgroundColour(0, 0, 0, 0)
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

async function ensureMainWindow() {
  try {
    await Window.SetFrameless(false)
    await Window.SetResizable(true)
    await Window.SetMinSize(1040, 640)
    await Window.SetAlwaysOnTop(false)
    await Window.SetBackgroundColour(244, 244, 245, 255)
    void applyMainWindowPolicy()
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

async function ensureToolWindowMode(view: AppToolView) {
  const compactUtility = view === 'network-mini'
  await ensureUtilityWindow({
    alwaysOnTop: compactUtility,
    transparent: compactUtility,
    frameless: true,
    syncFrameless: compactUtility,
    resizable: !compactUtility,
    minWidth: view === 'work-memory' ? 1040 : 820,
    minHeight: view === 'work-memory' ? 640 : 560,
  })
  if (!compactUtility) {
    void enableTaskbarToggle(view)
  }
}

async function ensureUtilityWindow(options: {
  alwaysOnTop: boolean
  transparent: boolean
  frameless?: boolean
  syncFrameless?: boolean
  resizable?: boolean
  minWidth?: number
  minHeight?: number
}) {
  const {
    alwaysOnTop,
    transparent,
    frameless = true,
    syncFrameless = true,
    resizable = false,
    minWidth = 820,
    minHeight = 560,
  } = options
  try {
    if (syncFrameless) {
      await Window.SetFrameless(frameless)
    }
    await Window.SetResizable(resizable)
    if (resizable) {
      await Window.SetMinSize(minWidth, minHeight)
    }
    await Window.SetAlwaysOnTop(alwaysOnTop)
    if (transparent) {
      await Window.SetBackgroundColour(0, 0, 0, 0)
    } else {
      await Window.SetBackgroundColour(244, 244, 245, 255)
    }
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function isStandaloneToolView(view: string): view is AppToolView {
  return ['work-memory', 'clipboard', 'capture', 'hosts', 'workflow', 'json-compare', 'network-monitor', 'network-mini', 'settings'].includes(view)
}

function shouldRenderToolView(view: AppToolView) {
  return standaloneToolView ? standaloneToolView === view : appShell.activeView === view
}
</script>

<template>
  <PinnedImageWindow v-if="isPinnedImageWindow" :pin-id="pinId" />
  <CaptureOverlayWindow v-else-if="isCaptureOverlayWindow" :session-id="captureOverlaySessionId" />
  <AriadneLauncher v-else-if="isLauncherWindow || (!standaloneToolView && appShell.activeView === 'launcher')" />
  <WorkMemoryCenter v-else-if="shouldRenderToolView('work-memory')" />
  <ClipboardCenter v-else-if="shouldRenderToolView('clipboard')" />
  <CaptureHistoryCenter v-else-if="shouldRenderToolView('capture')" />
  <HostsCenter v-else-if="shouldRenderToolView('hosts')" />
  <WorkflowCenter v-else-if="shouldRenderToolView('workflow')" />
  <JsonCompareCenter v-else-if="shouldRenderToolView('json-compare')" />
  <NetworkMonitorCenter v-else-if="shouldRenderToolView('network-monitor')" />
  <NetworkMiniWindow v-else-if="shouldRenderToolView('network-mini')" />
  <SettingsCenter v-else-if="shouldRenderToolView('settings')" />
  <WorkMemoryCenter v-else />
</template>
