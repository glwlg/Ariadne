import type { HostsApplyPreview, HostsApplyResult, HostsProfile, HostsStatus } from '../types/ariadne'

let fallbackProfiles: HostsProfile[] = [
  {
    id: 'system-hosts',
    title: '系统 Hosts',
    content: '127.0.0.1 localhost\n',
    enabled: false,
    type: 'local',
    system: true,
  },
]

let fallbackStatus: HostsStatus = {
  configPath: '%APPDATA%/Ariadne/hosts_profiles.json',
  hostsPath: 'C:/Windows/System32/drivers/etc/hosts',
  legacyPath: '~/.x-tools/hosts_profiles.json',
  count: 1,
  enabledCount: 0,
  systemReadable: true,
  systemBytes: 20,
  legacyImported: false,
  profiles: fallbackProfiles,
  virtualizedExists: false,
  virtualizedBytes: 0,
}

async function tryHostsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/hosts/service.js')
  } catch {
    return null
  }
}

export async function getHostsStatus(): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    try {
      return normalizeStatus(await binding.Status())
    } catch {
      return normalizeStatus(fallbackStatus)
    }
  }
  return normalizeStatus(fallbackStatus)
}

export async function listHostsProfiles(): Promise<HostsProfile[]> {
  const binding = await tryHostsBinding()
  if (binding) {
    try {
      return normalizeProfiles(await binding.List())
    } catch {
      return normalizeProfiles(fallbackProfiles)
    }
  }
  return normalizeProfiles(fallbackProfiles)
}

export async function newHostsProfile(): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizeStatus(await binding.NewProfile())
  }
  const profile = normalizeProfile({
    id: crypto.randomUUID(),
    title: 'New Hosts Profile',
    content: '# Local Hosts\n127.0.0.1 example.local\n',
    enabled: false,
    type: 'local',
    system: false,
  })
  fallbackProfiles = [...fallbackProfiles, profile]
  fallbackStatus = normalizeStatus({ ...fallbackStatus, profiles: fallbackProfiles })
  return fallbackStatus
}

export async function upsertHostsProfile(profile: HostsProfile): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizeStatus(await binding.Upsert(normalizeProfile(profile)))
  }
  const normalized = normalizeProfile(profile)
  fallbackProfiles = fallbackProfiles.map((item) => (item.id === normalized.id ? normalized : item))
  if (!fallbackProfiles.some((item) => item.id === normalized.id)) {
    fallbackProfiles.push(normalized)
  }
  fallbackStatus = normalizeStatus({ ...fallbackStatus, profiles: fallbackProfiles })
  return fallbackStatus
}

export async function removeHostsProfile(id: string): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizeStatus(await binding.Remove(id))
  }
  fallbackProfiles = fallbackProfiles.filter((item) => item.id !== id || item.system)
  fallbackStatus = normalizeStatus({ ...fallbackStatus, profiles: fallbackProfiles })
  return fallbackStatus
}

export async function setHostsProfileEnabled(id: string, enabled: boolean): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizeStatus(await binding.SetEnabled(id, enabled))
  }
  fallbackProfiles = fallbackProfiles.map((item) => (item.id === id && !item.system ? { ...item, enabled } : item))
  fallbackStatus = normalizeStatus({ ...fallbackStatus, profiles: fallbackProfiles })
  return fallbackStatus
}

export async function fetchRemoteHosts(id: string): Promise<HostsStatus> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizeStatus(await binding.FetchRemote(id))
  }
  fallbackStatus = normalizeStatus({ ...fallbackStatus, lastRemoteError: '开发态 fallback 不执行远程拉取' })
  return fallbackStatus
}

export async function previewHostsApply(): Promise<HostsApplyPreview> {
  const binding = await tryHostsBinding()
  if (binding) {
    return normalizePreview(await binding.PreviewApply())
  }
  return normalizePreview(buildFallbackPreview())
}

export async function applyEnabledHostsProfiles(confirmed: boolean): Promise<HostsApplyResult> {
  const binding = await tryHostsBinding()
  if (binding) {
    const result = await binding.ApplyEnabledProfiles(confirmed)
    return { ...result, preview: normalizePreview(result.preview) }
  }
  return {
    ok: false,
    message: '开发态 fallback 不写入系统 Hosts',
    requiresConfirm: !confirmed,
    preview: buildFallbackPreview(),
  }
}

function normalizeStatus(status: HostsStatus): HostsStatus {
  const profiles = normalizeProfiles(status.profiles ?? fallbackProfiles)
  return {
    configPath: status.configPath || '%APPDATA%/Ariadne/hosts_profiles.json',
    hostsPath: status.hostsPath || 'C:/Windows/System32/drivers/etc/hosts',
    legacyPath: status.legacyPath || '~/.x-tools/hosts_profiles.json',
    count: status.count ?? profiles.length,
    enabledCount: status.enabledCount ?? profiles.filter((profile) => profile.enabled && !profile.system).length,
    systemReadable: Boolean(status.systemReadable),
    systemBytes: Number(status.systemBytes ?? 0),
    lastSaveError: status.lastSaveError ?? '',
    lastReadError: status.lastReadError ?? '',
    lastApplyError: status.lastApplyError ?? '',
    lastRemoteError: status.lastRemoteError ?? '',
    legacyImported: Boolean(status.legacyImported),
    profiles,
    virtualizedPath: status.virtualizedPath ?? '',
    virtualizedExists: Boolean(status.virtualizedExists),
    virtualizedBytes: Number(status.virtualizedBytes ?? 0),
  }
}

function normalizeProfiles(profiles: HostsProfile[]): HostsProfile[] {
  return [...(profiles ?? [])].map(normalizeProfile).sort((a, b) => {
    if (a.system !== b.system) return a.system ? -1 : 1
    return a.title.localeCompare(b.title, 'zh-CN')
  })
}

function normalizeProfile(profile: HostsProfile): HostsProfile {
  return {
    id: String(profile.id ?? '').trim(),
    title: String(profile.title ?? '').trim() || 'Hosts Profile',
    content: String(profile.content ?? '').replace(/\r\n/g, '\n').replace(/\r/g, '\n'),
    enabled: Boolean(profile.enabled),
    type: profile.type === 'remote' ? 'remote' : 'local',
    url: String(profile.url ?? '').trim(),
    system: Boolean(profile.system),
    updatedAt: Number(profile.updatedAt ?? 0),
  }
}

function normalizePreview(preview: HostsApplyPreview): HostsApplyPreview {
  return {
    hostsPath: preview.hostsPath || 'C:/Windows/System32/drivers/etc/hosts',
    lineCount: Number(preview.lineCount ?? 0),
    addedLines: Number(preview.addedLines ?? 0),
    removedLines: Number(preview.removedLines ?? 0),
    changed: Boolean(preview.changed),
    enabledProfiles: Array.isArray(preview.enabledProfiles) ? preview.enabledProfiles.map(String) : [],
    conflicts: Array.isArray(preview.conflicts)
      ? preview.conflicts.map((item) => ({ host: String(item.host), ips: Array.isArray(item.ips) ? item.ips.map(String) : [] }))
      : [],
    currentContent: String(preview.currentContent ?? ''),
    finalContent: String(preview.finalContent ?? ''),
    diffText: String(preview.diffText ?? ''),
    requiresConfirm: Boolean(preview.requiresConfirm),
    lastPreviewError: preview.lastPreviewError ?? '',
  }
}

function buildFallbackPreview(): HostsApplyPreview {
  const current = fallbackProfiles.find((profile) => profile.system)?.content ?? ''
  const enabled = fallbackProfiles.filter((profile) => profile.enabled && !profile.system)
  const finalContent = [current.trim(), ...enabled.map((profile) => `# --- Profile: ${profile.title} ---\n${profile.content.trim()}`)]
    .filter(Boolean)
    .join('\n\n')
  return normalizePreview({
    hostsPath: fallbackStatus.hostsPath,
    lineCount: finalContent.split('\n').filter(Boolean).length,
    addedLines: Math.max(0, finalContent.split('\n').length - current.split('\n').length),
    removedLines: 0,
    changed: current.trim() !== finalContent.trim(),
    enabledProfiles: enabled.map((profile) => profile.title),
    conflicts: [],
    currentContent: current,
    finalContent,
    diffText: finalContent,
    requiresConfirm: true,
  })
}
