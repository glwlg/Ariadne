import type { LegacyDataStatus, LegacyImportRequest, LegacyImportResult } from '../types/ariadne'

async function tryMigrationBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/migration/service.js')
  } catch {
    return null
  }
}

export async function getLegacyDataStatus(): Promise<LegacyDataStatus> {
  const binding = await tryMigrationBinding()
  if (binding) {
    try {
      return await binding.Status()
    } catch {
      return fallbackLegacyDataStatus()
    }
  }
  return fallbackLegacyDataStatus()
}

export async function importLegacyData(request: LegacyImportRequest = {}): Promise<LegacyImportResult> {
  const binding = await tryMigrationBinding()
  if (binding) {
    try {
      return await binding.ImportLegacyData(request)
    } catch (error) {
      return fallbackLegacyImportResult(error instanceof Error ? error.message : '迁移服务调用失败')
    }
  }
  return fallbackLegacyImportResult('开发态 fallback 未连接 Go migration 服务。')
}

function fallbackLegacyDataStatus(): LegacyDataStatus {
  return {
    root: '%APPDATA%/x-tools',
    exists: false,
    needsImport: false,
    sources: [
      { source: 'clipboard_history', path: '%APPDATA%/x-tools/clipboard_history.json', exists: false, count: 0, bytes: 0, importedCount: 0, needsImport: false },
      { source: 'capture_history', path: '%APPDATA%/x-tools/capture_history.json', exists: false, count: 0, bytes: 0, importedCount: 0, needsImport: false },
      { source: 'work_memory', path: '%APPDATA%/x-tools/work_memory/entries.json', exists: false, count: 0, bytes: 0, importedCount: 0, needsImport: false },
    ],
    totalCount: 0,
    totalBytes: 0,
    notes: ['开发态 fallback 未读取旧版历史；桌面构建会调用 Go migration 服务。'],
  }
}

function fallbackLegacyImportResult(message: string): LegacyImportResult {
  const now = Math.floor(Date.now() / 1000)
  return {
    ok: false,
    message,
    startedAt: now,
    finishedAt: now,
    dryRun: false,
    sources: [],
  }
}
