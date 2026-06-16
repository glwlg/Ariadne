import type { OCRResult, OCRStatus } from '../types/ariadne'

async function tryOCRBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/ocr/service.js')
  } catch {
    return null
  }
}

export async function getOCRStatus(): Promise<OCRStatus> {
  const binding = await tryOCRBinding()
  if (!binding) {
    return {
      available: false,
      provider: 'rapidocr_onnxruntime',
      mode: 'unavailable',
      lastError: 'OCR 服务不可用',
    }
  }
  try {
    return normalizeStatus(await binding.Status())
  } catch {
    return {
      available: false,
      provider: 'rapidocr_onnxruntime',
      mode: 'unavailable',
      lastError: 'OCR 服务不可用',
    }
  }
}

export async function recognizeCaptureOCR(captureId: string): Promise<OCRResult> {
  const binding = await tryOCRBinding()
  if (!binding) return unavailableResult('capture_history')
  return normalizeResult(await binding.RecognizeCapture(captureId))
}

export async function recognizeCurrentScreenOCR(): Promise<OCRResult> {
  const binding = await tryOCRBinding()
  if (!binding) return unavailableResult('current_screen')
  return normalizeResult(await binding.RecognizeCurrentScreen())
}

export async function recognizeClipboardImageOCR(clipboardId: string): Promise<OCRResult> {
  const binding = await tryOCRBinding()
  if (!binding) return unavailableResult('clipboard_history')
  return normalizeResult(await binding.RecognizeClipboardImage(clipboardId))
}

export async function recognizeWorkMemoryOCR(memoryId: string): Promise<OCRResult> {
  const binding = await tryOCRBinding()
  if (!binding) return unavailableResult('work_memory')
  return normalizeResult(await binding.RecognizeWorkMemory(memoryId))
}

function normalizeStatus(value: Partial<OCRStatus> | null | undefined): OCRStatus {
  return {
    available: Boolean(value?.available),
    provider: value?.provider || 'rapidocr_onnxruntime',
    mode: value?.mode || 'python',
    pythonPath: value?.pythonPath || '',
    bridgePath: value?.bridgePath || '',
    lastError: value?.lastError || '',
    lastRunAt: Number(value?.lastRunAt || 0),
  }
}

function normalizeResult(value: Partial<OCRResult> | null | undefined): OCRResult {
  return {
    ok: Boolean(value?.ok),
    text: value?.text || '',
    lines: value?.lines ?? [],
    source: value?.source || '',
    captureId: value?.captureId || '',
    clipboardId: value?.clipboardId || '',
    memoryId: value?.memoryId || '',
    imagePath: value?.imagePath || '',
    width: Number(value?.width || 0),
    height: Number(value?.height || 0),
    provider: value?.provider || 'rapidocr_onnxruntime',
    elapsedMs: Number(value?.elapsedMs || 0),
    sensitive: Boolean(value?.sensitive),
    error: value?.error || '',
    recognizedAt: Number(value?.recognizedAt || 0),
    workMemory: value?.workMemory,
  }
}

function unavailableResult(source: string): OCRResult {
  return {
    ok: false,
    source,
    provider: 'rapidocr_onnxruntime',
    sensitive: false,
    error: '开发态未连接 OCR 服务',
    recognizedAt: Math.floor(Date.now() / 1000),
  }
}
