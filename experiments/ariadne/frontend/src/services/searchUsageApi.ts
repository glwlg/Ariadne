import type { SearchUsageClearResult, SearchUsageRecord, SearchUsageStatus } from '../types/ariadne'

let fallbackStatus: SearchUsageStatus = {
  path: 'dev:fallback-search-state',
  count: 0,
  records: [],
}

async function trySearchBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/search/service.js')
  } catch {
    return null
  }
}

export async function getSearchUsageStatus(): Promise<SearchUsageStatus> {
  const binding = await trySearchBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.UsageStatus())
    } catch {
      return structuredClone(fallbackStatus)
    }
  }
  return structuredClone(fallbackStatus)
}

export async function clearSearchUsage(): Promise<SearchUsageClearResult> {
  const binding = await trySearchBinding()
  if (binding) {
    return normalizeClearResult(await binding.ClearUsage())
  }
  const cleared = fallbackStatus.count
  fallbackStatus = { ...fallbackStatus, count: 0, records: [] }
  return {
    ok: true,
    message: cleared ? '开发态 fallback 已清理搜索收藏和最近使用' : '没有可清理的搜索收藏或最近使用记录',
    cleared,
    status: structuredClone(fallbackStatus),
  }
}

function normalizeStatus(status: SearchUsageStatus): SearchUsageStatus {
  const records = (status.records ?? []).map(normalizeRecord)
  return {
    path: status.path || '',
    count: Number(status.count ?? records.length),
    records,
  }
}

function normalizeRecord(record: SearchUsageRecord): SearchUsageRecord {
  return {
    resultId: String(record.resultId ?? ''),
    favorite: Boolean(record.favorite),
    useCount: Number(record.useCount ?? 0),
    lastUsedAt: Number(record.lastUsedAt ?? 0),
  }
}

function normalizeClearResult(result: SearchUsageClearResult): SearchUsageClearResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || (result.ok ? '已清理搜索收藏和最近使用记录' : '搜索状态清理失败'),
    cleared: Number(result.cleared ?? 0),
    status: normalizeStatus(result.status ?? { path: '', count: 0, records: [] }),
  }
}
