import type { Launcher, LauncherStatus } from '../types/ariadne'

const fallbackLaunchers: Launcher[] = [
  {
    id: 'ariadne-config-dir',
    name: 'Ariadne 配置目录',
    kind: 'folder',
    target: '%APPDATA%/Ariadne',
    keywords: ['ariadne', 'config', '配置'],
    tags: ['配置'],
    enabled: true,
  },
]

let fallbackStatus: LauncherStatus = {
  path: '%APPDATA%/Ariadne/ariadne.sqlite',
  count: fallbackLaunchers.length,
  items: structuredClone(fallbackLaunchers),
  lastSaveError: '',
}

async function tryLaunchersBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/launchers/service.js')
  } catch {
    return null
  }
}

export async function getLauncherStatus(): Promise<LauncherStatus> {
  const binding = await tryLaunchersBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.Status())
    } catch {
      return structuredClone(fallbackStatus)
    }
  }
  return structuredClone(fallbackStatus)
}

export async function upsertLauncher(launcher: Launcher): Promise<LauncherStatus> {
  const binding = await tryLaunchersBinding()
  const next = normalizeLauncher(launcher)
  if (binding) {
    return normalizeStatus(await binding.Upsert(next))
  }
  const index = fallbackStatus.items.findIndex((item) => item.id === next.id)
  if (index >= 0) {
    fallbackStatus.items[index] = next
  } else {
    fallbackStatus.items.push(next)
  }
  fallbackStatus = normalizeStatus(fallbackStatus)
  return structuredClone(fallbackStatus)
}

export async function removeLauncher(id: string): Promise<LauncherStatus> {
  const binding = await tryLaunchersBinding()
  if (binding) {
    return normalizeStatus(await binding.Remove(id))
  }
  fallbackStatus.items = fallbackStatus.items.filter((item) => item.id !== id)
  fallbackStatus = normalizeStatus(fallbackStatus)
  return structuredClone(fallbackStatus)
}

export function createLauncherDraft(): Launcher {
  return {
    id: '',
    name: '',
    kind: 'app',
    target: '',
    arguments: '',
    workingDir: '',
    keywords: [],
    tags: [],
    enabled: true,
  }
}

function normalizeStatus(status: LauncherStatus): LauncherStatus {
  const items = (status.items ?? [])
    .map(normalizeLauncher)
    .filter((item) => item.id || item.name || item.target)
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-Hans-CN'))
  return {
    path: status.path || '%APPDATA%/Ariadne/ariadne.sqlite',
    count: items.length,
    items,
    lastSaveError: status.lastSaveError ?? '',
  }
}

function normalizeLauncher(launcher: Launcher): Launcher {
  const next = JSON.parse(JSON.stringify(launcher)) as Launcher
  next.id = next.id?.trim() ?? ''
  next.name = next.name?.trim() ?? ''
  next.kind = normalizeKind(next.kind)
  next.target = next.target?.trim() ?? ''
  next.arguments = next.arguments?.trim() ?? ''
  next.workingDir = next.workingDir?.trim() ?? ''
  next.keywords = cleanList(next.keywords ?? [])
  next.tags = cleanList(next.tags ?? [])
  next.enabled = Boolean(next.enabled)
  return next
}

function normalizeKind(kind: string): Launcher['kind'] {
  if (['app', 'file', 'folder', 'url', 'command'].includes(kind)) {
    return kind as Launcher['kind']
  }
  return 'app'
}

function cleanList(items: string[]) {
  const seen = new Set<string>()
  return items
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false
      const key = item.toLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}
