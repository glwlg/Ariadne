import type { QRScanResult } from '../types/fancun'
import { formatRpcError, isDesktopRuntime } from './runtimeEnv'

async function tryQRScanBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/fancun/internal/qrscan/service.js')
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
  if (isDesktopRuntime()) {
    throw new Error('二维码识别服务绑定不可用')
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
  if (isDesktopRuntime()) {
    throw new Error('二维码识别服务绑定不可用')
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
    error: formatRpcError(error, '二维码识别失败'),
    decodedAt: Math.floor(Date.now() / 1000),
  }
}
