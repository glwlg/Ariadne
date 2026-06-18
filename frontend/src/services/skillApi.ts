import type {
  SkillAsset,
  SkillDraftSaveRequest,
  SkillDraftSaveResult,
  SkillExportRequest,
  SkillExportResult,
  SkillInstallDiagnosticsRequest,
  SkillInstallDiagnosticsResult,
  SkillInstallRequest,
  SkillInstallResult,
  SkillStatus,
  WorkMemoryDraft,
} from '../types/ariadne'

let fallbackSkills: SkillAsset[] = []

let fallbackStatus: SkillStatus = {
  path: '%APPDATA%/Ariadne/ariadne.sqlite',
  count: 0,
  skills: fallbackSkills,
}

async function trySkillBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/skills/service.js')
  } catch {
    return null
  }
}

export async function getSkillStatus(): Promise<SkillStatus> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeStatus(await binding.Status())
  }
  return fallbackStatus
}

export async function listSkills(): Promise<SkillAsset[]> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeSkills(await binding.List())
  }
  return fallbackSkills
}

export async function saveSkillDraft(request: SkillDraftSaveRequest): Promise<SkillDraftSaveResult> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeDraftSaveResult(await binding.SaveSkillDraft(request))
  }
  const skill = skillFromDraft(request.draft)
  const risks = draftRiskReasons(request.draft, skill)
  if (!request.confirmed) {
    return {
      ok: false,
      message: '保存 Skill 需要确认',
      skill,
      status: fallbackStatus,
      requiresConfirmation: true,
      riskReasons: risks.length ? risks : ['Skill 草稿来自工作记忆，保存为正式资产前需要用户确认'],
    }
  }
  fallbackSkills = [
    ...fallbackSkills.filter((item) => item.id !== skill.id),
    skill,
  ].sort((a, b) => Number(b.updatedAt ?? 0) - Number(a.updatedAt ?? 0) || a.id.localeCompare(b.id))
  fallbackStatus = { ...fallbackStatus, count: fallbackSkills.length, skills: fallbackSkills }
  return {
    ok: true,
    message: 'Skill 已保存为正式资产',
    skill,
    status: fallbackStatus,
    riskReasons: risks,
  }
}

export async function exportSkillPackage(request: SkillExportRequest): Promise<SkillExportResult> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeExportResult(await binding.ExportSkillPackage(request))
  }
  const skill = fallbackSkills.find((item) => item.id === request.skillId)
  if (!skill) {
    return {
      ok: false,
      message: '未找到对应 Skill',
      skill: normalizeSkill({ id: request.skillId || 'missing', title: '未知 Skill', body: '', evidence: [] }),
    }
  }
  const risks = exportRiskReasons(skill)
  if (!request.confirmed) {
    return {
      ok: false,
      message: '导出 Codex Skill 包需要确认',
      skill,
      requiresConfirmation: true,
      riskReasons: risks,
    }
  }
  return {
    ok: true,
    message: '开发态 fallback 已生成 Codex Skill 包',
    skill,
    directory: `%APPDATA%/Ariadne/skill_exports/${skill.id}`,
    zipPath: `%APPDATA%/Ariadne/skill_exports/${skill.id}.zip`,
    bytes: Math.max(1, renderSkillMarkdown(skill).length),
    exportedAt: Math.floor(Date.now() / 1000),
    riskReasons: risks,
  }
}

export async function installSkillPackage(request: SkillInstallRequest): Promise<SkillInstallResult> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeInstallResult(await binding.InstallSkillPackage(request))
  }
  const skill = fallbackSkills.find((item) => item.id === request.skillId)
  if (!skill) {
    return {
      ok: false,
      message: '未找到对应 Skill',
      skill: normalizeSkill({ id: request.skillId || 'missing', title: '未知 Skill', body: '', evidence: [] }),
    }
  }
  const targetRoot = request.targetRoot || '%USERPROFILE%/.codex/skills'
  const risks = installRiskReasons(skill, targetRoot, Boolean(request.overwrite))
  if (!request.confirmed) {
    return {
      ok: false,
      message: '安装到 Codex skills 目录需要确认',
      skill,
      targetRoot,
      requiresConfirmation: true,
      riskReasons: risks,
    }
  }
  return {
    ok: true,
    message: '开发态 fallback 已模拟安装到 Codex skills 目录并写入刷新握手',
    skill,
    targetRoot,
    installedDir: `${targetRoot}/${skill.id}`,
    files: ['SKILL.md', '.ariadne-refresh.json', '.ariadne-refresh.touch'],
    installedAt: Math.floor(Date.now() / 1000),
    refreshRequested: true,
    refreshMarker: `${targetRoot}/.ariadne-refresh.touch`,
    refreshManifest: `${targetRoot}/.ariadne-refresh.json`,
    riskReasons: risks,
  }
}

export async function getSkillInstallDiagnostics(request: SkillInstallDiagnosticsRequest = {}): Promise<SkillInstallDiagnosticsResult> {
  const binding = await trySkillBinding()
  if (binding) {
    return normalizeInstallDiagnosticsResult(await binding.InstallDiagnostics(request))
  }
  const targetRoot = request.targetRoot || '%USERPROFILE%/.codex/skills'
  const skillId = normalizeId(request.skillId || '')
  const skill = skillId ? fallbackSkills.find((item) => item.id === skillId) : null
  const installed = Boolean(skill)
  return normalizeInstallDiagnosticsResult({
    ok: !skillId || installed,
    message: skillId
      ? installed
        ? '开发态 fallback 已核验模拟 Skill 安装'
        : '开发态 fallback 未发现目标 Skill'
      : '开发态 fallback 已核验 Codex skills 目录',
    targetRoot,
    targetRootExists: true,
    targetRootReadable: true,
    skillId,
    installedDir: skill ? `${targetRoot}/${skill.id}` : '',
    installed,
    skillPath: skill ? `${targetRoot}/${skill.id}/SKILL.md` : '',
    skillFileExists: installed,
    skillFileBytes: skill ? renderSkillMarkdown(skill).length : 0,
    skillUpdatedAt: skill?.updatedAt || 0,
    discoveredCount: fallbackSkills.length,
    ariadneManagedCount: fallbackSkills.length,
    skills: fallbackSkills.map((item) => ({
      id: item.id,
      title: item.title,
      directory: `${targetRoot}/${item.id}`,
      skillPath: `${targetRoot}/${item.id}/SKILL.md`,
      readable: true,
      bytes: renderSkillMarkdown(item).length,
      updatedAt: item.updatedAt || 0,
      ariadneManaged: true,
    })),
    refresh: {
      manifestPath: `${targetRoot}/.ariadne-refresh.json`,
      markerPath: `${targetRoot}/.ariadne-refresh.touch`,
      manifestExists: installed,
      markerExists: installed,
      valid: installed,
      source: 'ariadne',
      action: 'skills.refresh',
      skillId,
      skillTitle: skill?.title || '',
      installedDir: skill ? `${targetRoot}/${skill.id}` : '',
      requestedAt: Math.floor(Date.now() / 1000),
      markerId: skill ? `${skill.id}-fallback` : '',
      markerText: skill ? `${skill.id}-fallback` : '',
      markerMatchesManifest: installed,
      skillMatchesRequest: true,
      installedDirExists: installed,
      installedSkillFileFound: installed,
    },
  })
}

function normalizeDraftSaveResult(result: SkillDraftSaveResult): SkillDraftSaveResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    skill: normalizeSkill(result.skill),
    status: normalizeStatus(result.status),
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: Array.isArray(result.riskReasons) ? result.riskReasons : [],
  }
}

function normalizeExportResult(result: SkillExportResult): SkillExportResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    skill: normalizeSkill(result.skill),
    directory: result.directory || '',
    zipPath: result.zipPath || '',
    bytes: Number(result.bytes ?? 0),
    exportedAt: Number(result.exportedAt ?? 0),
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: Array.isArray(result.riskReasons) ? result.riskReasons : [],
  }
}

function normalizeInstallResult(result: SkillInstallResult): SkillInstallResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    skill: normalizeSkill(result.skill),
    targetRoot: result.targetRoot || '',
    installedDir: result.installedDir || '',
    files: Array.isArray(result.files) ? result.files.map(String).filter(Boolean) : [],
    installedAt: Number(result.installedAt ?? 0),
    refreshRequested: Boolean(result.refreshRequested),
    refreshMarker: result.refreshMarker || '',
    refreshManifest: result.refreshManifest || '',
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: Array.isArray(result.riskReasons) ? result.riskReasons : [],
  }
}

function normalizeInstallDiagnosticsResult(result: SkillInstallDiagnosticsResult): SkillInstallDiagnosticsResult {
  const refresh = result.refresh ?? {
    manifestPath: '',
    markerPath: '',
    manifestExists: false,
    markerExists: false,
    valid: false,
    markerMatchesManifest: false,
    skillMatchesRequest: false,
    installedDirExists: false,
    installedSkillFileFound: false,
  }
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    targetRoot: result.targetRoot || '',
    targetRootExists: Boolean(result.targetRootExists),
    targetRootReadable: Boolean(result.targetRootReadable),
    skillId: result.skillId || '',
    installedDir: result.installedDir || '',
    installed: Boolean(result.installed),
    skillPath: result.skillPath || '',
    skillFileExists: Boolean(result.skillFileExists),
    skillFileBytes: Number(result.skillFileBytes ?? 0),
    skillUpdatedAt: Number(result.skillUpdatedAt ?? 0),
    discoveredCount: Number(result.discoveredCount ?? 0),
    ariadneManagedCount: Number(result.ariadneManagedCount ?? 0),
    skills: Array.isArray(result.skills)
      ? result.skills.map((item) => ({
          id: item.id || '',
          title: item.title || '',
          directory: item.directory || '',
          skillPath: item.skillPath || '',
          readable: Boolean(item.readable),
          bytes: Number(item.bytes ?? 0),
          updatedAt: Number(item.updatedAt ?? 0),
          ariadneManaged: Boolean(item.ariadneManaged),
          error: item.error || '',
        }))
      : [],
    refresh: {
      manifestPath: refresh.manifestPath || '',
      markerPath: refresh.markerPath || '',
      manifestExists: Boolean(refresh.manifestExists),
      markerExists: Boolean(refresh.markerExists),
      valid: Boolean(refresh.valid),
      source: refresh.source || '',
      action: refresh.action || '',
      skillId: refresh.skillId || '',
      skillTitle: refresh.skillTitle || '',
      installedDir: refresh.installedDir || '',
      requestedAt: Number(refresh.requestedAt ?? 0),
      markerId: refresh.markerId || '',
      markerText: refresh.markerText || '',
      markerMatchesManifest: Boolean(refresh.markerMatchesManifest),
      skillMatchesRequest: Boolean(refresh.skillMatchesRequest),
      installedDirExists: Boolean(refresh.installedDirExists),
      installedSkillFileFound: Boolean(refresh.installedSkillFileFound),
      error: refresh.error || '',
    },
    lastError: result.lastError || '',
  }
}

function normalizeStatus(status: SkillStatus): SkillStatus {
  const skills = normalizeSkills(status.skills ?? [])
  return {
    path: status.path || '%APPDATA%/Ariadne/ariadne.sqlite',
    count: Number(status.count ?? skills.length),
    lastSaveError: status.lastSaveError,
    skills,
  }
}

function normalizeSkills(skills: SkillAsset[]): SkillAsset[] {
  return (skills ?? [])
    .map(normalizeSkill)
    .filter((item) => item.id && item.title && item.body)
    .sort((a, b) => Number(b.updatedAt ?? 0) - Number(a.updatedAt ?? 0) || a.id.localeCompare(b.id))
}

function normalizeSkill(skill: SkillAsset): SkillAsset {
  return {
    id: normalizeId(skill?.id || 'memory-skill'),
    title: (skill?.title || '工作记忆 Skill').trim(),
    body: (skill?.body || '').trim(),
    evidence: cleanStrings(skill?.evidence ?? []),
    source: skill?.source || 'work_memory',
    createdAt: Number(skill?.createdAt ?? 0),
    updatedAt: Number(skill?.updatedAt ?? Date.now() / 1000),
  }
}

function skillFromDraft(draft: WorkMemoryDraft): SkillAsset {
  const now = Math.floor(Date.now() / 1000)
  return normalizeSkill({
    id: skillIdFromDraft(draft),
    title: draft.title || '工作记忆 Skill',
    body: draft.body || '',
    evidence: draft.evidence ?? [],
    source: 'work_memory',
    createdAt: draft.createdAt || now,
    updatedAt: now,
  })
}

function skillIdFromDraft(draft: WorkMemoryDraft): string {
  const raw = (draft.id || `${draft.title}-${draft.createdAt || ''}`)
    .toLowerCase()
    .replace(/^knowledge-/, '')
    .replace(/^skill-draft-/, '')
  const slug = normalizeId(raw)
  return `memory-skill-${slug || 'draft'}`
}

function draftRiskReasons(draft: WorkMemoryDraft, skill: SkillAsset): string[] {
  const reasons = ['草稿来自工作记忆知识沉淀，需要确认后才写入正式 Skill']
  if (!skill.evidence.length) {
    reasons.push('缺少证据引用，保存后需要人工补充来源')
  }
  if ((draft.body || '').trim().length < 20) {
    reasons.push('正文较短，建议保存前补充复用步骤或适用边界')
  }
  return Array.from(new Set(reasons))
}

function exportRiskReasons(skill: SkillAsset): string[] {
  const reasons = ['导出包会生成可安装的 Codex skill 文件，安装前需要确认内容不包含敏感信息']
  if (skill.evidence.length) {
    reasons.push('导出的 SKILL.md 会保留 evidence ID 作为来源线索')
  }
  const body = skill.body.toLowerCase()
  if (body.includes('password') || body.includes('token') || skill.body.includes('密码')) {
    reasons.push('正文疑似包含敏感词，安装或分享前必须人工复核')
  }
  return Array.from(new Set(reasons))
}

function installRiskReasons(skill: SkillAsset, targetRoot: string, overwrite: boolean): string[] {
  const reasons = [
    '安装会写入 Codex skills 发现目录，Codex 重启或刷新后可能加载该 Skill',
    '安装成功后会写入 Ariadne refresh marker，供 Codex runtime 或后续工具检测 newly installed skill',
    'Skill 内容来自工作记忆，安装前需要确认不包含敏感信息',
    `目标目录: ${targetRoot}`,
  ]
  if (overwrite) {
    reasons.push('若同名 Skill 已存在，将覆盖旧的 SKILL.md')
  }
  if (skill.evidence.length) {
    reasons.push('安装的 SKILL.md 会保留 evidence ID 作为来源线索')
  }
  const body = skill.body.toLowerCase()
  if (body.includes('password') || body.includes('token') || skill.body.includes('密码')) {
    reasons.push('正文疑似包含敏感词，安装前必须人工复核')
  }
  return Array.from(new Set(reasons))
}

function renderSkillMarkdown(skill: SkillAsset): string {
  return [
    '---',
    `name: ${skill.id}`,
    `description: "Use when Codex needs to apply the Ariadne work-memory guidance captured as ${skill.title}."`,
    '---',
    '',
    `# ${skill.title}`,
    '',
    skill.body,
    '',
    ...skill.evidence.map((evidence) => `- ${evidence}`),
  ].join('\n')
}

function normalizeId(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64)
}

function cleanStrings(values: string[]): string[] {
  return Array.from(new Set((values ?? []).map((value) => String(value).trim()).filter(Boolean)))
}
