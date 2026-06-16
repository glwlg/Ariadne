import type { PinnedImage, PinnedImageOpenResult } from '../types/ariadne'

async function tryPinnedImageBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/pinnedimage/service.js')
  } catch {
    return null
  }
}

export async function openPinnedCapture(captureId: string): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding) {
    return normalizeOpenResult(await binding.OpenCapture(captureId))
  }
  return { ok: false, message: '开发态未连接贴图窗口服务' }
}

export async function openPinnedClipboardImage(clipboardId: string): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding) {
    return normalizeOpenResult(await binding.OpenClipboardImage(clipboardId))
  }
  return { ok: false, message: '开发态未连接贴图窗口服务' }
}

export async function openCurrentClipboardPin(): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding?.OpenCurrentClipboard) {
    return normalizeOpenResult(await binding.OpenCurrentClipboard())
  }
  return { ok: false, message: '开发态未连接贴图窗口服务' }
}

export async function openPinnedQRText(text: string): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding) {
    return normalizeOpenResult(await binding.OpenQRText(text))
  }
  return { ok: false, message: '开发态未连接贴图窗口服务' }
}

export async function getPinnedImage(pinId: string): Promise<PinnedImage | null> {
  const binding = await tryPinnedImageBinding()
  if (!binding) return null
  try {
    return normalizePinnedImage(await binding.GetPinned(pinId))
  } catch {
    return null
  }
}

export async function closePinnedImage(pinId: string): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding) {
    return normalizeOpenResult(await binding.ClosePinned(pinId))
  }
  return { ok: true, message: '贴图已关闭', pinId }
}

export async function movePinnedImage(pinId: string, deltaX: number, deltaY: number): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding) {
    return normalizeOpenResult(await binding.MovePinned(pinId, Math.round(deltaX), Math.round(deltaY)))
  }
  return { ok: false, message: '开发态未连接贴图窗口服务', pinId }
}

export async function setPinnedImagePosition(pinId: string, x: number, y: number): Promise<PinnedImageOpenResult> {
  const binding = await tryPinnedImageBinding()
  if (binding?.SetPinnedPosition) {
    return normalizeOpenResult(await binding.SetPinnedPosition(pinId, Math.round(x), Math.round(y)))
  }
  return { ok: true, message: '贴图位置已同步', pinId }
}

function normalizeOpenResult(result: PinnedImageOpenResult): PinnedImageOpenResult {
  return {
    ok: Boolean(result?.ok),
    message: String(result?.message ?? ''),
    pinId: String(result?.pinId ?? ''),
    title: String(result?.title ?? ''),
    width: Number(result?.width ?? 0),
    height: Number(result?.height ?? 0),
  }
}

function normalizePinnedImage(image: PinnedImage): PinnedImage | null {
  const id = String(image?.id ?? '').trim()
  const dataUrl = String(image?.dataUrl ?? '').trim()
  if (!id || !dataUrl) return null
  return {
    id,
    source: String(image.source ?? ''),
    sourceId: String(image.sourceId ?? ''),
    title: String(image.title ?? '贴图'),
    imagePath: String(image.imagePath ?? ''),
    text: String(image.text ?? ''),
    dataUrl,
    width: Number(image.width ?? 0),
    height: Number(image.height ?? 0),
    bytes: Number(image.bytes ?? 0),
    createdAt: Number(image.createdAt ?? 0),
    windowWidth: Number(image.windowWidth ?? 0),
    windowHeight: Number(image.windowHeight ?? 0),
    windowX: Number(image.windowX ?? 0),
    windowY: Number(image.windowY ?? 0),
    positioned: Boolean(image.positioned),
    canCopy: Boolean(image.canCopy),
    copyAction: String(image.copyAction ?? ''),
    canOcr: Boolean(image.canOcr),
  }
}
