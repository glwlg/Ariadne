import type { ActionResult, CaptureHistoryStatus, ClipboardHistoryEntry, ClipboardHistoryStatus, QRScanResult } from '../types/ariadne'

let fallbackEntries: ClipboardHistoryEntry[] = []

let fallbackStatus: ClipboardHistoryStatus = {
  path: '%APPDATA%/Ariadne/clipboard_history.json',
  imageDir: '%APPDATA%/Ariadne/clipboard_images',
  thumbnailDir: '%APPDATA%/Ariadne/clipboard_thumbnails',
  count: 0,
  pinnedCount: 0,
  imageCount: 0,
  thumbnailCount: 0,
  thumbnailBytes: 0,
  entries: [],
  lastSaveError: '',
  watcherEnabled: false,
  watcherRunning: false,
  lastWatcherError: '',
}

async function tryClipboardBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/clipboardhistory/service.js')
  } catch {
    return null
  }
}

export async function getClipboardStatus(): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.Status())
    } catch {
      return normalizeStatus(fallbackStatus)
    }
  }
  return normalizeStatus(fallbackStatus)
}

export async function listClipboardEntries(query = '', limit = 200): Promise<ClipboardHistoryEntry[]> {
  const binding = await tryClipboardBinding()
  if (binding) {
    try {
      return normalizeEntries(await binding.List(query, limit))
    } catch {
      return fallbackList(query, limit)
    }
  }
  return fallbackList(query, limit)
}

export async function addClipboardText(text: string, source = 'manual'): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeStatus(await binding.AddText(text, source))
  }
  const entry = normalizeEntry({
    id: crypto.randomUUID(),
    type: 'text',
    text,
    createdAt: Math.floor(Date.now() / 1000),
    pinned: false,
    signature: `dev:${text}`,
    contentType: classifyText(text),
    source,
    summary: summarize(text, 120),
    tags: ['剪贴板', '文本'],
  })
  fallbackEntries = [entry, ...fallbackEntries.filter((item) => item.signature !== entry.signature)]
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function collectCurrentClipboard(source = 'manual'): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeStatus(await binding.CollectCurrent(source))
  }
  try {
    const text = await navigator.clipboard?.readText?.()
    if (text?.trim()) {
      return addClipboardText(text, source)
    }
  } catch {
    // Fall through to explicit local feedback.
  }
  return normalizeStatus({ ...fallbackStatus, lastSaveError: '开发态没有可收集的剪贴板内容' })
}

export async function copyClipboardImage(id: string): Promise<ActionResult> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeActionResult(await binding.CopyImage(id))
  }
  return { ok: false, message: '开发态不支持复制图片' }
}

export async function getClipboardImageDataURL(id: string): Promise<string> {
  const binding = await tryClipboardBinding()
  if (binding) {
    try {
      return String(await binding.ImageDataURL(id))
    } catch {
      return ''
    }
  }
  return ''
}

export async function getClipboardThumbnailDataURL(id: string): Promise<string> {
  const binding = await tryClipboardBinding()
  if (binding) {
    try {
      if (typeof binding.ThumbnailDataURL === 'function') {
        return String(await binding.ThumbnailDataURL(id))
      }
      return String(await binding.ImageDataURL(id))
    } catch {
      return ''
    }
  }
  return ''
}

export async function addClipboardImageToCapture(id: string): Promise<CaptureHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeCaptureStatus(await binding.AddImageToCapture(id))
  }
  return {
    path: '%APPDATA%/Ariadne/capture_history.json',
    imageDir: '%APPDATA%/Ariadne/capture_images',
    count: 0,
    pinnedCount: 0,
    lastCaptureError: '开发态未连接截图历史服务',
    virtualizedExists: false,
    virtualizedBytes: 0,
    virtualizedImageCount: 0,
    virtualizedImageBytes: 0,
    entries: [],
  }
}

export async function decodeClipboardImageQRCode(id: string): Promise<QRScanResult> {
  const binding = await tryClipboardBinding()
  if (binding) {
    try {
      return normalizeQRResult(await binding.DecodeImageQRCode(id))
    } catch (error) {
      return {
        ok: false,
        error: error instanceof Error ? error.message : '二维码识别失败',
        decodedAt: Math.floor(Date.now() / 1000),
      }
    }
  }
  return {
    ok: false,
    source: 'clipboard_history',
    error: '开发态未连接二维码识别服务',
    decodedAt: Math.floor(Date.now() / 1000),
  }
}

export async function toggleClipboardPin(id: string): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeStatus(await binding.TogglePin(id))
  }
  fallbackEntries = fallbackEntries.map((item) => (item.id === id ? { ...item, pinned: !item.pinned } : item))
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function deleteClipboardEntry(id: string): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeStatus(await binding.Delete(id))
  }
  fallbackEntries = fallbackEntries.filter((item) => item.id !== id)
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function clearUnpinnedClipboardEntries(): Promise<ClipboardHistoryStatus> {
  const binding = await tryClipboardBinding()
  if (binding) {
    return normalizeStatus(await binding.ClearUnpinned())
  }
  fallbackEntries = fallbackEntries.filter((item) => item.pinned)
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

function fallbackList(query: string, limit: number) {
  const normalized = query.trim().toLowerCase()
  const items = fallbackEntries.filter((item) => {
    if (!normalized) return true
    return [item.text, item.summary, item.contentType, item.source, ...(item.tags ?? [])].join(' ').toLowerCase().includes(normalized)
  })
  return sortEntries(items).slice(0, limit)
}

function normalizeStatus(status: ClipboardHistoryStatus): ClipboardHistoryStatus {
  const entries = normalizeEntries(status.entries ?? fallbackEntries)
  return {
    path: status.path || '%APPDATA%/Ariadne/clipboard_history.json',
    imageDir: status.imageDir || '%APPDATA%/Ariadne/clipboard_images',
    thumbnailDir: status.thumbnailDir ?? '%APPDATA%/Ariadne/clipboard_thumbnails',
    count: status.count ?? entries.length,
    pinnedCount: status.pinnedCount ?? entries.filter((item) => item.pinned).length,
    imageCount: status.imageCount ?? entries.filter((item) => item.type === 'image').length,
    thumbnailCount: Number(status.thumbnailCount ?? 0),
    thumbnailBytes: Number(status.thumbnailBytes ?? 0),
    lastEntryAt: status.lastEntryAt,
    lastSaveError: status.lastSaveError ?? '',
    watcherEnabled: Boolean(status.watcherEnabled),
    watcherRunning: Boolean(status.watcherRunning),
    lastWatcherAt: status.lastWatcherAt,
    lastWatcherError: status.lastWatcherError ?? '',
    entries,
  }
}

function normalizeEntries(entries: ClipboardHistoryEntry[]): ClipboardHistoryEntry[] {
  return sortEntries((entries ?? []).map(normalizeEntry).filter((item) => item.id && (item.text || item.imagePath)))
}

function normalizeEntry(entry: ClipboardHistoryEntry): ClipboardHistoryEntry {
  const text = String(entry.text ?? '').trim()
  const type = entry.type === 'image' ? 'image' : 'text'
  return {
    id: String(entry.id ?? '').trim(),
    type,
    text,
    imagePath: String(entry.imagePath ?? '').trim(),
    thumbnailPath: String(entry.thumbnailPath ?? '').trim(),
    thumbnailWidth: Number(entry.thumbnailWidth ?? 0),
    thumbnailHeight: Number(entry.thumbnailHeight ?? 0),
    thumbnailBytes: Number(entry.thumbnailBytes ?? 0),
    createdAt: Number(entry.createdAt ?? 0),
    pinned: Boolean(entry.pinned),
    signature: String(entry.signature ?? ''),
    contentType: String(entry.contentType ?? (type === 'image' ? 'image' : classifyText(text))),
    source: String(entry.source ?? 'manual'),
    summary: String(entry.summary ?? (type === 'image' ? `剪贴板图片 ${entry.width ?? 0}x${entry.height ?? 0}` : summarize(text, 120))),
    width: Number(entry.width ?? 0),
    height: Number(entry.height ?? 0),
    bytes: Number(entry.bytes ?? 0),
    tags: Array.isArray(entry.tags) ? entry.tags.map((item) => String(item).trim()).filter(Boolean) : [],
  }
}

function normalizeActionResult(result: ActionResult): ActionResult {
  return {
    ok: Boolean(result?.ok),
    message: String(result?.message ?? ''),
  }
}

function normalizeCaptureStatus(status: CaptureHistoryStatus): CaptureHistoryStatus {
  return {
    path: String(status.path ?? ''),
    imageDir: String(status.imageDir ?? ''),
    count: Number(status.count ?? 0),
    pinnedCount: Number(status.pinnedCount ?? 0),
    lastEntryAt: status.lastEntryAt,
    lastSaveError: status.lastSaveError ?? '',
    lastCaptureError: status.lastCaptureError ?? '',
    virtualizedPath: status.virtualizedPath ?? '',
    virtualizedExists: Boolean(status.virtualizedExists),
    virtualizedBytes: Number(status.virtualizedBytes ?? 0),
    virtualizedImageDir: status.virtualizedImageDir ?? '',
    virtualizedImageCount: Number(status.virtualizedImageCount ?? 0),
    virtualizedImageBytes: Number(status.virtualizedImageBytes ?? 0),
    entries: status.entries ?? [],
  }
}

function normalizeQRResult(result: QRScanResult): QRScanResult {
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

function sortEntries(entries: ClipboardHistoryEntry[]) {
  return [...entries].sort((a, b) => {
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
    return b.createdAt - a.createdAt
  })
}

function summarize(value: string, limit: number) {
  const text = value.trim().replace(/\s+/g, ' ')
  return text.length > limit ? `${text.slice(0, limit - 3)}...` : text
}

function classifyText(value: string) {
  const text = value.trim()
  if (!text) return 'text'
  try {
    JSON.parse(text)
    return 'json'
  } catch {
    // Not JSON.
  }
  if (/^https?:\/\//i.test(text)) return 'url'
  if (/^[a-z]:\\/i.test(text) || text.startsWith('\\\\')) return 'path'
  if (/^(git|go|pnpm|npm|wails3)\s+/i.test(text)) return 'command'
  return 'text'
}
