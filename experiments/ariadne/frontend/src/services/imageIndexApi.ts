import type { ImageIndexBatchResult, ImageIndexEntry, ImageIndexRequest } from '../types/ariadne'

async function tryImageIndexBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/imageindex/service.js')
  } catch {
    return null
  }
}

export async function indexRecentImages(request: ImageIndexRequest = {}): Promise<ImageIndexBatchResult> {
  const binding = await tryImageIndexBinding()
  if (!binding) {
    return {
      ok: false,
      startedAt: 0,
      finishedAt: 0,
      indexed: 0,
      skipped: 0,
      failed: 1,
      lastError: '图片索引服务不可用',
      entries: [],
    }
  }
  const result = await binding.IndexRecent({
    sources: request.sources ?? [],
    limit: request.limit ?? 30,
    force: Boolean(request.force),
  })
  return normalizeBatchResult(result)
}

function normalizeBatchResult(result: Partial<ImageIndexBatchResult> | null | undefined): ImageIndexBatchResult {
  return {
    ok: Boolean(result?.ok),
    startedAt: Number(result?.startedAt ?? 0),
    finishedAt: Number(result?.finishedAt ?? 0),
    indexed: Number(result?.indexed ?? 0),
    skipped: Number(result?.skipped ?? 0),
    failed: Number(result?.failed ?? 0),
    lastError: String(result?.lastError ?? ''),
    entries: (result?.entries ?? []).map(normalizeEntry),
  }
}

function normalizeEntry(entry: Partial<ImageIndexEntry>): ImageIndexEntry {
  return {
    id: String(entry.id ?? ''),
    source: String(entry.source ?? ''),
    sourceId: String(entry.sourceId ?? ''),
    imagePath: String(entry.imagePath ?? ''),
    text: String(entry.text ?? ''),
    provider: String(entry.provider ?? ''),
    indexedAt: Number(entry.indexedAt ?? 0),
    width: Number(entry.width ?? 0),
    height: Number(entry.height ?? 0),
    ok: Boolean(entry.ok),
    sensitive: Boolean(entry.sensitive),
    redacted: Boolean(entry.redacted),
    error: String(entry.error ?? ''),
  }
}
