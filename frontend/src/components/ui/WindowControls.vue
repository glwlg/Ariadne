<script setup lang="ts">
import { Minus, Square, X } from '@lucide/vue'
import { Window } from '@wailsio/runtime'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useAppShellStore } from '../../stores/appShell'

const appShell = useAppShellStore()
const isWorkAreaMaximised = ref(false)
const maximiseTitle = computed(() => (isWorkAreaMaximised.value ? '还原' : '最大化'))
let restoreFrame: WindowFrame | null = null
let syncTimer = 0

async function minimiseWindow() {
  try {
    await Window.Minimise()
  } catch {
    // Browser-only dev mode has no Wails runtime.
  }
}

async function toggleWindowMaximise() {
  try {
    const nativeMaximised = await Window.IsMaximised()
    if (nativeMaximised) {
      await Window.Restore()
      isWorkAreaMaximised.value = false
      return
    }
    if (isWorkAreaMaximised.value || (await windowMatchesWorkArea())) {
      await restoreWindow()
      return
    }
    await maximiseToWorkArea()
  } catch {
    // Browser-only dev mode has no Wails runtime.
  }
}

function closeWindow() {
  void appShell.closeCurrentWindow()
}

onMounted(() => {
  window.addEventListener('resize', queueMaximiseStateSync)
  void syncMaximiseState()
})

onUnmounted(() => {
  window.removeEventListener('resize', queueMaximiseStateSync)
  window.clearTimeout(syncTimer)
})

async function maximiseToWorkArea() {
  const frame = await currentWindowFrame()
  const workArea = workAreaFromScreen(await Window.GetScreen())
  if (!workArea) {
    await Window.Maximise()
    isWorkAreaMaximised.value = true
    return
  }
  if (frame && !framesMatch(frame, workArea)) {
    restoreFrame = frame
  }
  await Window.Restore()
  await Window.SetPosition(workArea.x, workArea.y)
  await Window.SetSize(workArea.width, workArea.height)
  isWorkAreaMaximised.value = true
}

async function restoreWindow() {
  await Window.Restore()
  if (restoreFrame) {
    await Window.SetSize(restoreFrame.width, restoreFrame.height)
    await Window.SetPosition(restoreFrame.x, restoreFrame.y)
  } else {
    await restoreToDefaultFrame()
  }
  isWorkAreaMaximised.value = false
}

async function restoreToDefaultFrame() {
  const workArea = workAreaFromScreen(await Window.GetScreen())
  if (!workArea) {
    await Window.Center()
    return
  }
  const width = clamp(1120, 900, Math.max(900, workArea.width - 96))
  const height = clamp(720, 620, Math.max(620, workArea.height - 96))
  await Window.SetSize(width, height)
  await Window.SetPosition(Math.round(workArea.x + (workArea.width - width) / 2), Math.round(workArea.y + (workArea.height - height) / 2))
}

function queueMaximiseStateSync() {
  window.clearTimeout(syncTimer)
  syncTimer = window.setTimeout(() => {
    void syncMaximiseState()
  }, 120)
}

async function syncMaximiseState() {
  try {
    isWorkAreaMaximised.value = (await Window.IsMaximised()) || (await windowMatchesWorkArea())
  } catch {
    isWorkAreaMaximised.value = false
  }
}

async function windowMatchesWorkArea() {
  const frame = await currentWindowFrame()
  const workArea = workAreaFromScreen(await Window.GetScreen())
  return Boolean(frame && workArea && framesMatch(frame, workArea))
}

async function currentWindowFrame(): Promise<WindowFrame | null> {
  const [position, size] = await Promise.all([Window.Position(), Window.Size()])
  if (!validNumber(position.x) || !validNumber(position.y) || !validNumber(size.width) || !validNumber(size.height)) {
    return null
  }
  return {
    x: Math.round(position.x),
    y: Math.round(position.y),
    width: Math.round(size.width),
    height: Math.round(size.height),
  }
}

function workAreaFromScreen(screen: RuntimeScreen | null | undefined): WindowFrame | null {
  const rect = normalizedRect(screen?.WorkArea) ?? normalizedRect(screen?.Bounds)
  if (!rect) {
    return null
  }
  return rect
}

function normalizedRect(rect: RuntimeRect | null | undefined): WindowFrame | null {
  const x = rect?.X ?? 0
  const y = rect?.Y ?? 0
  const width = rect?.Width
  const height = rect?.Height
  if (!validNumber(x) || !validNumber(y) || !validNumber(width) || !validNumber(height) || width <= 0 || height <= 0) {
    return null
  }
  return {
    x: Math.round(x),
    y: Math.round(y),
    width: Math.round(width),
    height: Math.round(height),
  }
}

function framesMatch(frame: WindowFrame, target: WindowFrame) {
  return (
    Math.abs(frame.x - target.x) <= 2 &&
    Math.abs(frame.y - target.y) <= 2 &&
    Math.abs(frame.width - target.width) <= 2 &&
    Math.abs(frame.height - target.height) <= 2
  )
}

function validNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

type WindowFrame = {
  x: number
  y: number
  width: number
  height: number
}

type RuntimeRect = {
  X?: number
  Y?: number
  Width?: number
  Height?: number
}

type RuntimeScreen = {
  WorkArea?: RuntimeRect
  Bounds?: RuntimeRect
}
</script>

<template>
  <div class="window-control-strip" data-no-drag aria-label="窗口控制">
    <button type="button" class="window-control-button" title="最小化" aria-label="最小化" @click="minimiseWindow">
      <Minus :size="14" />
    </button>
    <button type="button" class="window-control-button" :title="maximiseTitle" :aria-label="maximiseTitle" @click="toggleWindowMaximise">
      <span v-if="isWorkAreaMaximised" class="window-restore-icon" aria-hidden="true"></span>
      <Square v-else :size="12" aria-hidden="true" />
    </button>
    <button type="button" class="window-control-button is-close" title="关闭" aria-label="关闭" @click="closeWindow">
      <X :size="14" />
    </button>
  </div>
</template>
