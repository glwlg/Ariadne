import type { CaptureHistoryEntry, CaptureHistoryStatus } from '../types/ariadne'

let fallbackEntries: CaptureHistoryEntry[] = []

let fallbackStatus: CaptureHistoryStatus = {
  path: '%APPDATA%/Ariadne/capture_history.json',
  imageDir: '%APPDATA%/Ariadne/capture_images',
  thumbnailDir: '%APPDATA%/Ariadne/capture_thumbnails',
  count: 0,
  pinnedCount: 0,
  thumbnailCount: 0,
  thumbnailBytes: 0,
  entries: [],
  lastSaveError: '',
  lastCaptureError: '',
  virtualizedExists: false,
  virtualizedBytes: 0,
  virtualizedImageCount: 0,
  virtualizedImageBytes: 0,
}

async function tryCaptureBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/capturehistory/service.js')
  } catch {
    return null
  }
}

export async function getCaptureStatus(): Promise<CaptureHistoryStatus> {
  const binding = await tryCaptureBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.Status())
    } catch {
      return normalizeStatus(fallbackStatus)
    }
  }
  return normalizeStatus(fallbackStatus)
}

export async function listCaptureEntries(query = '', limit = 300): Promise<CaptureHistoryEntry[]> {
  const binding = await tryCaptureBinding()
  if (binding) {
    try {
      return normalizeEntries(await binding.List(query, limit))
    } catch {
      return fallbackList(query, limit)
    }
  }
  return fallbackList(query, limit)
}

export async function captureCurrentScreen(source = 'manual'): Promise<CaptureHistoryStatus> {
  const binding = await tryCaptureBinding()
  if (binding) {
    return normalizeStatus(await binding.CaptureScreen(source))
  }
  const entry = normalizeEntry({
    id: crypto.randomUUID(),
    imagePath: '%APPDATA%/Ariadne/capture_images/dev-placeholder.png',
    thumbnailPath: '',
    createdAt: Math.floor(Date.now() / 1000),
    source,
    pinned: false,
    width: 1440,
    height: 900,
    bytes: 0,
    signature: 'dev-placeholder',
    tags: ['截图', '捕获历史', '1440x900'],
  })
  fallbackEntries = [entry, ...fallbackEntries]
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function toggleCapturePin(id: string): Promise<CaptureHistoryStatus> {
  const binding = await tryCaptureBinding()
  if (binding) {
    return normalizeStatus(await binding.TogglePin(id))
  }
  fallbackEntries = fallbackEntries.map((item) => (item.id === id ? { ...item, pinned: !item.pinned } : item))
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function deleteCaptureEntry(id: string): Promise<CaptureHistoryStatus> {
  const binding = await tryCaptureBinding()
  if (binding) {
    return normalizeStatus(await binding.Delete(id))
  }
  fallbackEntries = fallbackEntries.filter((item) => item.id !== id)
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function clearUnpinnedCaptureEntries(): Promise<CaptureHistoryStatus> {
  const binding = await tryCaptureBinding()
  if (binding) {
    return normalizeStatus(await binding.ClearUnpinned())
  }
  fallbackEntries = fallbackEntries.filter((item) => item.pinned)
  fallbackStatus = normalizeStatus({ ...fallbackStatus, entries: fallbackEntries })
  return fallbackStatus
}

export async function getCaptureImageDataURL(id: string): Promise<string> {
  const binding = await tryCaptureBinding()
  if (binding) {
    try {
      return String(await binding.ImageDataURL(id))
    } catch {
      return ''
    }
  }
  return ''
}

export async function getCaptureThumbnailDataURL(id: string): Promise<string> {
  const binding = await tryCaptureBinding()
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

function fallbackList(query: string, limit: number) {
  const normalized = query.trim().toLowerCase()
  const items = fallbackEntries.filter((item) => {
    if (!normalized) return true
    return [
      item.imagePath,
      item.savedPath,
      item.source,
      `${item.width}x${item.height}`,
      ...(item.actions ?? []),
      ...(item.tags ?? []),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(normalized)
  })
  return sortEntries(items).slice(0, limit)
}

function normalizeStatus(status: CaptureHistoryStatus): CaptureHistoryStatus {
  const entries = normalizeEntries(status.entries ?? fallbackEntries)
  return {
    path: status.path || '%APPDATA%/Ariadne/capture_history.json',
    imageDir: status.imageDir || '%APPDATA%/Ariadne/capture_images',
    thumbnailDir: status.thumbnailDir ?? '%APPDATA%/Ariadne/capture_thumbnails',
    count: status.count ?? entries.length,
    pinnedCount: status.pinnedCount ?? entries.filter((item) => item.pinned).length,
    thumbnailCount: Number(status.thumbnailCount ?? 0),
    thumbnailBytes: Number(status.thumbnailBytes ?? 0),
    lastEntryAt: status.lastEntryAt,
    lastSaveError: status.lastSaveError ?? '',
    lastCaptureError: status.lastCaptureError ?? '',
    virtualizedPath: status.virtualizedPath ?? '',
    virtualizedExists: Boolean(status.virtualizedExists),
    virtualizedBytes: Number(status.virtualizedBytes ?? 0),
    virtualizedImageDir: status.virtualizedImageDir ?? '',
    virtualizedImageCount: Number(status.virtualizedImageCount ?? 0),
    virtualizedImageBytes: Number(status.virtualizedImageBytes ?? 0),
    entries,
  }
}

function normalizeEntries(entries: CaptureHistoryEntry[]): CaptureHistoryEntry[] {
  return sortEntries((entries ?? []).map(normalizeEntry).filter((item) => item.id && item.imagePath))
}

function normalizeEntry(entry: CaptureHistoryEntry): CaptureHistoryEntry {
  const width = Number(entry.width ?? 0)
  const height = Number(entry.height ?? 0)
  return {
    id: String(entry.id ?? '').trim(),
    imagePath: String(entry.imagePath ?? '').trim(),
    thumbnailPath: String(entry.thumbnailPath ?? '').trim(),
    thumbnailWidth: Number(entry.thumbnailWidth ?? 0),
    thumbnailHeight: Number(entry.thumbnailHeight ?? 0),
    thumbnailBytes: Number(entry.thumbnailBytes ?? 0),
    savedPath: String(entry.savedPath ?? '').trim(),
    createdAt: Number(entry.createdAt ?? 0),
    source: String(entry.source ?? 'manual'),
    actions: Array.isArray(entry.actions) ? entry.actions.map((item) => String(item).trim()).filter(Boolean) : [],
    pinned: Boolean(entry.pinned),
    width,
    height,
    bytes: Number(entry.bytes ?? 0),
    signature: String(entry.signature ?? ''),
    tags: Array.isArray(entry.tags) ? entry.tags.map((item) => String(item).trim()).filter(Boolean) : [`${width}x${height}`],
  }
}

function sortEntries(entries: CaptureHistoryEntry[]) {
  return [...entries].sort((a, b) => {
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
    return b.createdAt - a.createdAt
  })
}
