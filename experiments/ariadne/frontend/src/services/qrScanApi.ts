import type { QRScanResult } from '../types/ariadne'

async function tryQRScanBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/qrscan/service.js')
  } catch {
    return null
  }
}

export async function decodeCaptureQRCode(captureId: string): Promise<QRScanResult> {
  const binding = await tryQRScanBinding()
  if (binding) {
    try {
      return normalizeResult(await binding.DecodeCapture(captureId))
    } catch (error) {
      return failure(error)
    }
  }
  return {
    ok: false,
    captureId,
    error: '开发态 fallback 未连接 Go 二维码识别服务。',
    decodedAt: Math.floor(Date.now() / 1000),
  }
}

export async function decodeCurrentScreenQRCode(): Promise<QRScanResult> {
  const binding = await tryQRScanBinding()
  if (binding) {
    try {
      return normalizeResult(await binding.DecodeCurrentScreen())
    } catch (error) {
      return failure(error)
    }
  }
  return {
    ok: false,
    source: 'current_screen',
    error: '开发态 fallback 未连接 Go 二维码识别服务。',
    decodedAt: Math.floor(Date.now() / 1000),
  }
}

function normalizeResult(result: QRScanResult): QRScanResult {
  return {
    ok: Boolean(result.ok),
    text: result.text ?? '',
    format: result.format ?? '',
    source: result.source ?? '',
    captureId: result.captureId ?? '',
    imagePath: result.imagePath ?? '',
    width: result.width ?? 0,
    height: result.height ?? 0,
    error: result.error ?? '',
    decodedAt: result.decodedAt ?? 0,
  }
}

function failure(error: unknown): QRScanResult {
  return {
    ok: false,
    error: error instanceof Error ? error.message : '二维码识别失败',
    decodedAt: Math.floor(Date.now() / 1000),
  }
}
