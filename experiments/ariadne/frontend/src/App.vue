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
import WindowControls from './components/ui/WindowControls.vue'
import WorkMemoryCenter from './components/workmemory/WorkMemoryCenter.vue'
import WorkflowCenter from './components/workflows/WorkflowCenter.vue'
import { installSystemThemeListener, syncThemeFromSettings } from './lib/theme'
import { useAppShellStore, type AppToolView } from './stores/appShell'
import { Window } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, watch } from 'vue'

const appShell = useAppShellStore()
const routeParams = new URLSearchParams(window.location.search)
const routeView = routeParams.get('view') ?? ''
const isPinnedImageWindow = routeView === 'pinned-image'
const isCaptureOverlayWindow = routeView === 'capture-overlay'
const standaloneToolView = isStandaloneToolView(routeView) ? routeView : ''
const pinId = routeParams.get('pinId') ?? ''
const captureOverlaySessionId = routeParams.get('sessionId') ?? ''
const currentToolView = computed<AppToolView | ''>(() => {
  if (standaloneToolView) return standaloneToolView
  return appShell.activeView === 'launcher' ? '' : appShell.activeView
})
const showWindowControls = computed(() => Boolean(currentToolView.value && currentToolView.value !== 'network-mini'))
const documentClass = isPinnedImageWindow
  ? 'pinned-image-document'
  : isCaptureOverlayWindow
    ? 'capture-overlay-document'
    : standaloneToolView
      ? 'tool-window-document'
      : 'launcher-document'
let uninstallShellEvents: (() => void) | null = null
let uninstallThemeEvents: (() => void) | null = null

onMounted(() => {
  document.documentElement.classList.add(documentClass)
  watch(
    showWindowControls,
    (visible) => {
      document.documentElement.classList.toggle('window-controls-visible', visible)
    },
    { immediate: true },
  )
  if (isPinnedImageWindow || isCaptureOverlayWindow) {
    void ensureUtilityWindow(true, isPinnedImageWindow)
    void syncThemeFromSettings()
    uninstallThemeEvents = installSystemThemeListener()
    return
  }
  if (standaloneToolView) {
    const compactUtility = standaloneToolView === 'network-mini'
    void ensureUtilityWindow(compactUtility, compactUtility, compactUtility)
    void syncThemeFromSettings()
    uninstallThemeEvents = installSystemThemeListener()
    return
  }
  void ensureLauncherWindow()
  void syncThemeFromSettings()
  uninstallThemeEvents = installSystemThemeListener()
  uninstallShellEvents = appShell.installShellEventListeners()
  appShell.openLauncher()
})

onUnmounted(() => {
  uninstallShellEvents?.()
  uninstallThemeEvents?.()
  document.documentElement.classList.remove(documentClass)
  document.documentElement.classList.remove('window-controls-visible')
})

async function ensureLauncherWindow() {
  try {
    await Window.SetFrameless(true)
    await Window.SetAlwaysOnTop(false)
    await Window.SetBackgroundColour(0, 0, 0, 0)
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

async function ensureUtilityWindow(alwaysOnTop: boolean, transparent: boolean, frameless = true) {
  try {
    await Window.SetFrameless(frameless)
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
</script>

<template>
  <PinnedImageWindow v-if="isPinnedImageWindow" :pin-id="pinId" />
  <CaptureOverlayWindow v-else-if="isCaptureOverlayWindow" :session-id="captureOverlaySessionId" />
  <WorkMemoryCenter v-else-if="standaloneToolView === 'work-memory' || appShell.activeView === 'work-memory'" />
  <ClipboardCenter v-else-if="standaloneToolView === 'clipboard' || appShell.activeView === 'clipboard'" />
  <CaptureHistoryCenter v-else-if="standaloneToolView === 'capture' || appShell.activeView === 'capture'" />
  <HostsCenter v-else-if="standaloneToolView === 'hosts' || appShell.activeView === 'hosts'" />
  <WorkflowCenter v-else-if="standaloneToolView === 'workflow' || appShell.activeView === 'workflow'" />
  <JsonCompareCenter v-else-if="standaloneToolView === 'json-compare' || appShell.activeView === 'json-compare'" />
  <NetworkMonitorCenter v-else-if="standaloneToolView === 'network-monitor' || appShell.activeView === 'network-monitor'" />
  <NetworkMiniWindow v-else-if="standaloneToolView === 'network-mini' || appShell.activeView === 'network-mini'" />
  <SettingsCenter v-else-if="standaloneToolView === 'settings' || appShell.activeView === 'settings'" />
  <AriadneLauncher v-else />
  <WindowControls v-if="showWindowControls" />
</template>
