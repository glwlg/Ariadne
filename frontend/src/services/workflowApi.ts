import type {
  WorkflowDefinition,
  WorkflowDraft,
  WorkflowDraftSaveRequest,
  WorkflowDraftSaveResult,
  WorkflowExportResult,
  WorkflowImportResult,
  WorkflowRunRequest,
  WorkflowRunResult,
  WorkflowStatus,
} from '../types/ariadne'

let fallbackWorkflows: WorkflowDefinition[] = [
  {
    id: 'clip-md5',
    name: '剪贴板文本 -> MD5',
    description: '读取剪贴板文本并复制其 MD5 值',
    steps: [{ command: 'hash {clipboard}', pick: 'MD5' }],
  },
  {
    id: 'clip-url-encode',
    name: '剪贴板文本 -> URL 编码',
    description: '读取剪贴板文本并复制 URL 编码结果',
    steps: [{ command: 'url {clipboard}', pick: '编码结果' }],
  },
]

let fallbackStatus: WorkflowStatus = {
  path: '%APPDATA%/Ariadne/workflows.json',
  legacyPath: '%APPDATA%/x-tools/config.json',
  count: fallbackWorkflows.length,
  legacyImported: false,
  workflows: fallbackWorkflows,
}

async function tryWorkflowBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/workflows/service.js')
  } catch {
    return null
  }
}

export async function getWorkflowStatus(): Promise<WorkflowStatus> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeStatus(await binding.Status())
  }
  return fallbackStatus
}

export async function listWorkflows(): Promise<WorkflowDefinition[]> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeWorkflows(await binding.List())
  }
  return fallbackWorkflows
}

export async function newWorkflow(): Promise<WorkflowStatus> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeStatus(await binding.NewWorkflow())
  }
  const id = uniqueFallbackId()
  fallbackWorkflows = [...fallbackWorkflows, {
    id,
    name: '新工作流',
    description: '描述这个命令链的用途',
    steps: [{ command: 'hash {input}', pick: 'MD5' }],
  }]
  fallbackStatus = { ...fallbackStatus, count: fallbackWorkflows.length, workflows: fallbackWorkflows }
  return fallbackStatus
}

export async function upsertWorkflow(workflow: WorkflowDefinition): Promise<WorkflowStatus> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeStatus(await binding.Upsert(workflow))
  }
  const normalized = normalizeWorkflow(workflow)
  fallbackWorkflows = [
    ...fallbackWorkflows.filter((item) => item.id !== normalized.id),
    normalized,
  ].sort((a, b) => a.id.localeCompare(b.id))
  fallbackStatus = { ...fallbackStatus, count: fallbackWorkflows.length, workflows: fallbackWorkflows }
  return fallbackStatus
}

export async function removeWorkflow(id: string): Promise<WorkflowStatus> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeStatus(await binding.Remove(id))
  }
  fallbackWorkflows = fallbackWorkflows.filter((item) => item.id !== id)
  fallbackStatus = { ...fallbackStatus, count: fallbackWorkflows.length, workflows: fallbackWorkflows }
  return fallbackStatus
}

export async function exportWorkflows(): Promise<WorkflowExportResult> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeExportResult(await binding.ExportData())
  }
  const json = JSON.stringify({ version: 1, exportedBy: 'Ariadne fallback', exportedAt: Math.floor(Date.now() / 1000), workflows: fallbackWorkflows }, null, 2)
  return {
    ok: true,
    message: '开发态 fallback 已导出工作流',
    json,
    count: fallbackWorkflows.length,
    bytes: json.length,
    exportedAt: Math.floor(Date.now() / 1000),
  }
}

export async function importWorkflows(raw: string): Promise<WorkflowImportResult> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeImportResult(await binding.ImportData(raw))
  }
  try {
    const parsed = JSON.parse(raw) as { workflows?: WorkflowDefinition[] } | WorkflowDefinition[]
    const imported = Array.isArray(parsed) ? parsed : parsed.workflows ?? []
    const normalized = normalizeWorkflows(imported)
    if (!normalized.length) {
      throw new Error('导入内容没有合法工作流')
    }
    fallbackWorkflows = [
      ...fallbackWorkflows.filter((item) => !normalized.some((next) => next.id === item.id)),
      ...normalized,
    ].sort((a, b) => a.id.localeCompare(b.id))
    fallbackStatus = { ...fallbackStatus, count: fallbackWorkflows.length, workflows: fallbackWorkflows }
    return { ok: true, message: `已导入 ${normalized.length} 个工作流`, importedCount: normalized.length, status: fallbackStatus }
  } catch {
    return { ok: false, message: '导入内容不是 Ariadne 工作流 JSON', importedCount: 0, status: fallbackStatus }
  }
}

export async function saveWorkflowDraft(request: WorkflowDraftSaveRequest): Promise<WorkflowDraftSaveResult> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeDraftSaveResult(await binding.SaveWorkflowDraft(request))
  }
  const workflow = workflowFromDraft(request.draft)
  const risks = draftRiskReasons(request.draft, workflow)
  if (!request.confirmed) {
    return {
      ok: false,
      message: '保存候选工作流需要确认',
      workflow,
      status: fallbackStatus,
      requiresConfirmation: true,
      riskReasons: risks.length ? risks : ['候选工作流来自工作记忆经验发现，保存为正式工作流前需要用户确认'],
    }
  }
  fallbackWorkflows = [
    ...fallbackWorkflows.filter((item) => item.id !== workflow.id),
    workflow,
  ].sort((a, b) => a.id.localeCompare(b.id))
  fallbackStatus = { ...fallbackStatus, count: fallbackWorkflows.length, workflows: fallbackWorkflows }
  return {
    ok: true,
    message: '候选工作流已保存为正式工作流',
    workflow,
    status: fallbackStatus,
    riskReasons: risks,
  }
}

export async function runWorkflow(request: WorkflowRunRequest): Promise<WorkflowRunResult> {
  const binding = await tryWorkflowBinding()
  if (binding) {
    return normalizeRunResult(await binding.Run(request))
  }
  const workflow = fallbackWorkflows.find((item) => item.id === request.workflowId)
  if (!workflow) {
    return { ok: false, message: '未找到对应工作流', workflowId: request.workflowId, workflowName: '', steps: [] }
  }
  const riskReasons = fallbackRiskReasons(workflow)
  if (riskReasons.length && !request.confirmed) {
    return {
      ok: false,
      message: '工作流包含高风险步骤，需要再次确认',
      workflowId: workflow.id,
      workflowName: workflow.name,
      steps: [],
      requiresConfirmation: true,
      riskReasons,
    }
  }
  return {
    ok: true,
    message: `工作流完成：${workflow.name}（开发态 fallback）`,
    workflowId: workflow.id,
    workflowName: workflow.name,
    output: request.input || request.clipboardText || workflow.id,
    steps: workflow.steps.map((step, index) => ({
      index: index + 1,
      command: step.command,
      renderedCommand: renderFallback(step.command, request),
      pick: step.pick,
      pickedTitle: step.pick || step.command,
      output: request.input || request.clipboardText || workflow.id,
      ok: true,
    })),
  }
}

function normalizeStatus(status: WorkflowStatus): WorkflowStatus {
  const workflows = normalizeWorkflows(status.workflows ?? [])
  return {
    path: status.path || '%APPDATA%/Ariadne/workflows.json',
    legacyPath: status.legacyPath || '%APPDATA%/x-tools/config.json',
    count: Number(status.count ?? workflows.length),
    lastSaveError: status.lastSaveError,
    legacyImported: Boolean(status.legacyImported),
    workflows,
  }
}

function normalizeWorkflows(workflows: WorkflowDefinition[]): WorkflowDefinition[] {
  return (workflows ?? []).map(normalizeWorkflow).filter((workflow) => workflow.id && workflow.name && workflow.steps.length)
}

function normalizeWorkflow(workflow: WorkflowDefinition): WorkflowDefinition {
  return {
    id: String(workflow.id ?? '').trim().toLowerCase(),
    name: String(workflow.name ?? '').trim() || '未命名工作流',
    description: String(workflow.description ?? '').trim(),
    steps: (workflow.steps ?? [])
      .map((step) => ({
        command: String(step.command ?? '').trim(),
        pick: String(step.pick ?? '').trim(),
      }))
      .filter((step) => step.command),
    updatedAt: Number(workflow.updatedAt ?? 0),
  }
}

function normalizeRunResult(result: WorkflowRunResult): WorkflowRunResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || (result.ok ? '工作流完成' : '工作流失败'),
    workflowId: result.workflowId || '',
    workflowName: result.workflowName || '',
    output: result.output || '',
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: result.riskReasons ?? [],
    steps: (result.steps ?? []).map((step) => ({
      index: Number(step.index ?? 0),
      command: step.command || '',
      renderedCommand: step.renderedCommand || '',
      pick: step.pick || '',
      pickedTitle: step.pickedTitle || '',
      output: step.output || '',
      ok: Boolean(step.ok),
      message: step.message || '',
    })),
  }
}

function normalizeExportResult(result: WorkflowExportResult): WorkflowExportResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || (result.ok ? '工作流已导出' : '导出失败'),
    path: result.path || '',
    json: result.json || '',
    count: Number(result.count ?? 0),
    bytes: Number(result.bytes ?? 0),
    exportedAt: Number(result.exportedAt ?? 0),
  }
}

function normalizeImportResult(result: WorkflowImportResult): WorkflowImportResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || (result.ok ? '工作流已导入' : '导入失败'),
    importedCount: Number(result.importedCount ?? 0),
    status: normalizeStatus(result.status),
  }
}

function normalizeDraftSaveResult(result: WorkflowDraftSaveResult): WorkflowDraftSaveResult {
  return {
    ok: Boolean(result.ok),
    message: result.message || (result.ok ? '候选工作流已保存' : '候选工作流保存失败'),
    workflow: normalizeWorkflow(result.workflow ?? { id: '', name: '', description: '', steps: [] }),
    status: normalizeStatus(result.status ?? fallbackStatus),
    requiresConfirmation: Boolean(result.requiresConfirmation),
    riskReasons: result.riskReasons ?? [],
  }
}

function workflowFromDraft(draft: WorkflowDraft): WorkflowDefinition {
  const id = draftWorkflowId(draft)
  const description = [
    '由工作记忆经验发现生成，保存前需要用户审阅。',
    draft.trigger ? `触发: ${draft.trigger}` : '',
    draft.input ? `输入: ${draft.input}` : '',
    draft.output ? `输出: ${draft.output}` : '',
    draft.riskLevel ? `风险: ${draft.riskLevel}` : '',
    draft.evidence?.length ? `证据: ${draft.evidence.join(', ')}` : '',
  ].filter(Boolean).join('\n')
  return normalizeWorkflow({
    id,
    name: draft.title || '工作记忆候选工作流',
    description,
    steps: (draft.steps ?? []).map((step) => ({ command: step.command })),
    updatedAt: Math.floor(Date.now() / 1000),
  })
}

function draftWorkflowId(draft: WorkflowDraft) {
  const raw = String(draft.id || '').trim().toLowerCase().replace(/^workflow-draft-/, '')
  const slug = raw.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  return slug ? `memory-${slug}` : `memory-${Math.max(1, Number(draft.createdAt || 0))}`
}

function draftRiskReasons(draft: WorkflowDraft, workflow: WorkflowDefinition) {
  const reasons = new Set<string>()
  const risk = String(draft.riskLevel || '').trim().toLowerCase()
  if (risk && risk !== 'low') {
    reasons.add(`草稿风险等级: ${risk}`)
  }
  for (const step of draft.steps ?? []) {
    if (step.requiresConfirm) {
      reasons.add(`步骤需要确认: ${step.label || step.command}`)
    }
  }
  fallbackRiskReasons(workflow).forEach((reason) => reasons.add(reason))
  return [...reasons]
}

function uniqueFallbackId() {
  for (let index = 1; index < 1000; index += 1) {
    const id = index === 1 ? 'new-workflow' : `new-workflow-${index}`
    if (!fallbackWorkflows.some((workflow) => workflow.id === id)) {
      return id
    }
  }
  return `new-workflow-${Date.now()}`
}

function renderFallback(template: string, request: WorkflowRunRequest) {
  return template
    .replaceAll('{clipboard}', request.clipboardText ?? '')
    .replaceAll('{input}', request.input ?? '')
    .replaceAll('{prev}', '')
}

function fallbackRiskReasons(workflow: WorkflowDefinition) {
  const reasons: string[] = []
  workflow.steps.forEach((step, index) => {
    const [keyword, ...rest] = step.command.trim().toLowerCase().split(/\s+/)
    const args = rest.join(' ')
    if (['sys', 'system'].includes(keyword)) {
      reasons.push(`第 ${index + 1} 步: 系统命令需要确认`)
    }
    if (['clip', 'clipboard', 'cap', 'capture', 'shot'].includes(keyword) && ['clear', '清空'].includes(args)) {
      reasons.push(`第 ${index + 1} 步: 清理历史需要确认`)
    }
  })
  return reasons
}
