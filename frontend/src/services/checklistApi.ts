import type {
  ChecklistAsset,
  ChecklistDraft,
  ChecklistDraftSaveRequest,
  ChecklistDraftSaveResult,
  ChecklistStatus,
} from '../types/ariadne'

let fallbackChecklists: ChecklistAsset[] = []

let fallbackStatus: ChecklistStatus = {
  path: '%APPDATA%/Ariadne/ariadne.sqlite',
  count: 0,
  checklists: fallbackChecklists,
}

async function tryChecklistBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/checklists/service.js')
  } catch {
    return null
  }
}

export async function getChecklistStatus(): Promise<ChecklistStatus> {
  const binding = await tryChecklistBinding()
  if (binding) {
    return normalizeStatus(await binding.Status())
  }
  return fallbackStatus
}

export async function listChecklists(): Promise<ChecklistAsset[]> {
  const binding = await tryChecklistBinding()
  if (binding) {
    return normalizeChecklists(await binding.List())
  }
  return fallbackChecklists
}

export async function saveChecklistDraft(request: ChecklistDraftSaveRequest): Promise<ChecklistDraftSaveResult> {
  const binding = await tryChecklistBinding()
  if (binding) {
    return normalizeDraftSaveResult(await binding.SaveChecklistDraft(request))
  }
  const checklist = checklistFromDraft(request.draft)
  const risks = draftRiskReasons(request.draft, checklist)
  if (!request.confirmed) {
    return {
      ok: false,
      message: '保存检查清单需要确认',
      checklist,
      status: fallbackStatus,
      requiresConfirmation: true,
      riskReasons: risks.length ? risks : ['检查清单来自工作记忆经验发现，保存为正式资产前需要用户确认'],
    }
  }
  fallbackChecklists = [
    ...fallbackChecklists.filter((item) => item.id !== checklist.id),
    checklist,
  ].sort((a, b) => Number(b.updatedAt ?? 0) - Number(a.updatedAt ?? 0) || a.id.localeCompare(b.id))
  fallbackStatus = { ...fallbackStatus, count: fallbackChecklists.length, checklists: fallbackChecklists }
  return {
    ok: true,
    message: '检查清单已保存为正式资产',
    checklist,
    status: fallbackStatus,
    riskReasons: risks,
  }
}

function normalizeDraftSaveResult(result: ChecklistDraftSaveResult): ChecklistDraftSaveResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || '',
    checklist: normalizeChecklist(result.checklist),
    status: normalizeStatus(result.status),
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: Array.isArray(result.riskReasons) ? result.riskReasons : [],
  }
}

function normalizeStatus(status: ChecklistStatus): ChecklistStatus {
  const checklists = normalizeChecklists(status.checklists ?? [])
  return {
    path: status.path || '%APPDATA%/Ariadne/ariadne.sqlite',
    count: Number(status.count ?? checklists.length),
    lastSaveError: status.lastSaveError,
    checklists,
  }
}

function normalizeChecklists(checklists: ChecklistAsset[]): ChecklistAsset[] {
  return (checklists ?? [])
    .map(normalizeChecklist)
    .filter((item) => item.id && item.title && item.items.length)
    .sort((a, b) => Number(b.updatedAt ?? 0) - Number(a.updatedAt ?? 0) || a.id.localeCompare(b.id))
}

function normalizeChecklist(checklist: ChecklistAsset): ChecklistAsset {
  return {
    id: normalizeId(checklist?.id || 'memory-checklist'),
    title: (checklist?.title || '工作记忆检查清单').trim(),
    context: (checklist?.context || '').trim(),
    items: cleanStrings(checklist?.items ?? []),
    evidence: cleanStrings(checklist?.evidence ?? []),
    source: checklist?.source || 'work_memory',
    createdAt: Number(checklist?.createdAt ?? 0),
    updatedAt: Number(checklist?.updatedAt ?? Date.now() / 1000),
  }
}

function checklistFromDraft(draft: ChecklistDraft): ChecklistAsset {
  const now = Math.floor(Date.now() / 1000)
  return normalizeChecklist({
    id: checklistIdFromDraft(draft),
    title: draft.title || '工作记忆检查清单',
    context: draft.context || '由工作记忆经验发现生成，保存前需要用户审阅。',
    items: draft.items ?? [],
    evidence: draft.evidence ?? [],
    source: 'work_memory',
    createdAt: draft.createdAt || now,
    updatedAt: now,
  })
}

function checklistIdFromDraft(draft: ChecklistDraft): string {
  const raw = (draft.id || `${draft.title}-${draft.createdAt || ''}`).toLowerCase().replace(/^checklist-draft-/, '')
  const slug = normalizeId(raw)
  return `memory-checklist-${slug || 'draft'}`
}

function draftRiskReasons(draft: ChecklistDraft, checklist: ChecklistAsset): string[] {
  const reasons: string[] = []
  if (draft.requiresReview) {
    reasons.push('草稿来自工作记忆经验发现，需要确认后才写入正式检查清单')
  }
  if (!checklist.evidence.length) {
    reasons.push('缺少证据引用，后续复盘时需要人工补证据')
  }
  if (checklist.items.length > 8) {
    reasons.push('清单条目较多，建议保存前快速审阅')
  }
  return Array.from(new Set(reasons))
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
