import type {
  CaptureOverlayOpenResult,
  CaptureOverlayResult,
  CaptureOverlaySelectionRequest,
  CaptureOverlaySession,
  ScreenBounds,
} from '../types/ariadne'

async function tryCaptureOverlayBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/captureoverlay/service.js')
  } catch {
    return null
  }
}

export async function openCaptureOverlay(): Promise<CaptureOverlayOpenResult> {
  const binding = await tryCaptureOverlayBinding()
  if (binding) {
    return normalizeOpenResult(await binding.Open())
  }
  return { ok: false, message: '开发态未连接截图覆盖层服务' }
}

export async function getCaptureOverlaySession(sessionId: string): Promise<CaptureOverlaySession | null> {
  const binding = await tryCaptureOverlayBinding()
  if (!binding) return null
  try {
    return normalizeSession(await binding.GetSession(sessionId))
  } catch {
    return null
  }
}

export async function captureOverlaySelection(request: CaptureOverlaySelectionRequest): Promise<CaptureOverlayResult> {
  const binding = await tryCaptureOverlayBinding()
  if (binding) {
    return normalizeCaptureResult(await binding.CaptureSelection(request))
  }
  return { ok: false, message: '开发态未连接截图覆盖层服务' }
}

export async function cancelCaptureOverlay(sessionId: string): Promise<CaptureOverlayResult> {
  const binding = await tryCaptureOverlayBinding()
  if (binding) {
    return normalizeCaptureResult(await binding.Cancel(sessionId))
  }
  return { ok: true, message: '已取消截图' }
}

function normalizeOpenResult(result: CaptureOverlayOpenResult): CaptureOverlayOpenResult {
  return {
    ok: Boolean(result?.ok),
    message: String(result?.message ?? ''),
    sessionId: String(result?.sessionId ?? ''),
    bounds: normalizeBounds(result?.bounds),
    nativeBounds: normalizeOptionalBounds(result?.nativeBounds),
  }
}

function normalizeSession(session: CaptureOverlaySession): CaptureOverlaySession | null {
  const id = String(session?.id ?? '').trim()
  const imageUrl = String(session?.imageUrl ?? '').trim()
  if (!id || !imageUrl) return null
  return {
    id,
    bounds: normalizeBounds(session.bounds),
    nativeBounds: normalizeOptionalBounds(session.nativeBounds),
    imageUrl,
    createdAt: Number(session.createdAt ?? 0),
  }
}

function normalizeCaptureResult(result: CaptureOverlayResult): CaptureOverlayResult {
  return {
    ok: Boolean(result?.ok),
    message: String(result?.message ?? ''),
    captureId: String(result?.captureId ?? ''),
    imagePath: String(result?.imagePath ?? ''),
    savedPath: String(result?.savedPath ?? ''),
    width: Number(result?.width ?? 0),
    height: Number(result?.height ?? 0),
    qr: result?.qr,
    pin: result?.pin,
  }
}

function normalizeBounds(bounds?: ScreenBounds): ScreenBounds {
  return {
    x: Number(bounds?.x ?? 0),
    y: Number(bounds?.y ?? 0),
    width: Number(bounds?.width ?? 0),
    height: Number(bounds?.height ?? 0),
  }
}

function normalizeOptionalBounds(bounds?: ScreenBounds): ScreenBounds | undefined {
  const normalized = normalizeBounds(bounds)
  return normalized.width > 0 && normalized.height > 0 ? normalized : undefined
}
