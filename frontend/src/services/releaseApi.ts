import type { ReleaseBackupRequest, ReleaseBackupResult, ReleaseBackupStatus, ReleaseRestoreRequest, ReleaseRestoreResult } from '../types/ariadne'

async function tryReleaseBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/release/service.js')
  } catch {
    return null
  }
}

export async function getReleaseBackupStatus(): Promise<ReleaseBackupStatus> {
  const binding = await tryReleaseBinding()
  if (binding) {
    try {
      return await binding.Status()
    } catch {
      return fallbackReleaseBackupStatus()
    }
  }
  return fallbackReleaseBackupStatus()
}

export async function createRollbackCheckpoint(request: ReleaseBackupRequest = {}): Promise<ReleaseBackupResult> {
  const binding = await tryReleaseBinding()
  if (binding) {
    try {
      return await binding.CreateRollbackCheckpoint(request)
    } catch (error) {
      return fallbackReleaseBackupResult(error instanceof Error ? error.message : '回滚检查点服务调用失败')
    }
  }
  return fallbackReleaseBackupResult('开发态 fallback 未连接 Go release 服务，未创建检查点。')
}

export async function restoreRollbackCheckpoint(request: ReleaseRestoreRequest): Promise<ReleaseRestoreResult> {
  const binding = await tryReleaseBinding()
  if (binding) {
    try {
      return await binding.RestoreRollbackCheckpoint(request)
    } catch (error) {
      return fallbackReleaseRestoreResult(error instanceof Error ? error.message : '回滚恢复服务调用失败')
    }
  }
  return fallbackReleaseRestoreResult('开发态 fallback 未连接 Go release 服务，未恢复检查点。')
}

function fallbackReleaseBackupStatus(): ReleaseBackupStatus {
  return {
    dataRoots: [
      { kind: 'roaming', archiveName: 'roaming', path: '%APPDATA%/Ariadne', exists: false, fileCount: 0, bytes: 0 },
    ],
    backupDir: '%APPDATA%/Ariadne/backups',
    backupCount: 0,
    backupBytes: 0,
    latestBackup: '',
    notes: ['开发态 fallback 未读取 Ariadne 本地数据；桌面构建会调用 Go release 服务。'],
  }
}

function fallbackReleaseBackupResult(message: string): ReleaseBackupResult {
  return {
    ok: false,
    message,
    fileCount: 0,
    roots: [],
    createdAt: Math.floor(Date.now() / 1000),
  }
}

function fallbackReleaseRestoreResult(message: string): ReleaseRestoreResult {
  return {
    ok: false,
    message,
    fileCount: 0,
    bytes: 0,
    roots: [],
    requiresConfirmation: false,
    restoredAt: Math.floor(Date.now() / 1000),
  }
}
