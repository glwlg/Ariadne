import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { Clipboard } from '@wailsio/runtime'
import {
  exportWorkflows,
  getWorkflowStatus,
  importWorkflows,
  listWorkflows,
  newWorkflow,
  removeWorkflow,
  runWorkflow,
  upsertWorkflow,
} from '../services/workflowApi'
import type { WorkflowDefinition, WorkflowExportResult, WorkflowImportResult, WorkflowRunResult, WorkflowStatus, WorkflowStep } from '../types/fancun'

const ARMED_TIMEOUT_MS = 5000

export const useWorkflowsStore = defineStore('workflows', () => {
  const workflows = ref<WorkflowDefinition[]>([])
  const status = ref<WorkflowStatus | null>(null)
  const selectedId = ref('')
  const draft = ref<WorkflowDefinition | null>(null)
  const runInput = ref('')
  const importText = ref('')
  const lastRun = ref<WorkflowRunResult | null>(null)
  const exportResult = ref<WorkflowExportResult | null>(null)
  const importResult = ref<WorkflowImportResult | null>(null)
  const feedback = ref('')
  const feedbackTone = ref<'success' | 'danger'>('success')
  const loadError = ref('')
  const deleteArmedId = ref('')
  const riskArmedId = ref('')
  const isLoading = ref(false)
  const isSaving = ref(false)
  const isRunning = ref(false)
  let feedbackTimer: number | undefined
  let deleteArmedTimer: number | undefined
  let riskArmedTimer: number | undefined

  const selectedWorkflow = computed(() => workflows.value.find((workflow) => workflow.id === selectedId.value) ?? workflows.value[0] ?? null)
  const draftModified = computed(() => {
    const selected = selectedWorkflow.value
    if (!draft.value || !selected) return false
    return JSON.stringify(normalizeDraft(draft.value)) !== JSON.stringify(normalizeDraft(selected))
  })

  async function load() {
    isLoading.value = true
    loadError.value = ''
    try {
      const [nextStatus, nextWorkflows] = await Promise.all([getWorkflowStatus(), listWorkflows()])
      status.value = nextStatus
      workflows.value = nextWorkflows
      if (!workflows.value.some((workflow) => workflow.id === selectedId.value)) {
        selectedId.value = workflows.value[0]?.id ?? ''
      }
      syncDraft()
    } catch (error) {
      loadError.value = formatError(error, '工作流加载失败')
      showFeedback(`工作流加载失败: ${shortError(loadError.value)}`, 'danger')
    } finally {
      isLoading.value = false
    }
  }

  function select(id: string) {
    selectedId.value = id
    disarmDelete()
    disarmRisk()
    syncDraft()
  }

  async function createWorkflow() {
    disarmDelete()
    disarmRisk()
    const beforeIds = new Set(workflows.value.map((workflow) => workflow.id))
    isSaving.value = true
    try {
      const nextStatus = await newWorkflow()
      status.value = nextStatus
      workflows.value = nextStatus.workflows
      if (nextStatus.lastSaveError) {
        showFeedback(`新建失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return
      }
      const created =
        workflows.value.find((workflow) => !beforeIds.has(workflow.id)) ??
        [...workflows.value]
          .filter((workflow) => workflow.id.startsWith('new-workflow'))
          .sort((a, b) => (b.updatedAt ?? 0) - (a.updatedAt ?? 0))[0] ??
        workflows.value[0]
      selectedId.value = created?.id ?? ''
      syncDraft()
      showFeedback('已新建工作流', 'success')
    } catch (error) {
      showFeedback(`新建失败: ${shortError(formatError(error, '新建工作流失败'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  async function saveDraft(): Promise<boolean> {
    if (!draft.value) return false
    isSaving.value = true
    try {
      const nextStatus = await upsertWorkflow(draft.value)
      status.value = nextStatus
      workflows.value = nextStatus.workflows
      if (nextStatus.lastSaveError) {
        showFeedback(`保存失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return false
      }
      selectedId.value = draft.value.id
      syncDraft()
      showFeedback('已保存工作流', 'success')
      return true
    } catch (error) {
      showFeedback(`保存失败: ${shortError(formatError(error, '保存工作流失败'))}`, 'danger')
      return false
    } finally {
      isSaving.value = false
    }
  }

  async function deleteWorkflow(workflow = selectedWorkflow.value) {
    if (!workflow) return
    disarmRisk()
    if (deleteArmedId.value !== workflow.id) {
      armDelete(workflow.id)
      showFeedback('再次点击确认删除', 'danger')
      return
    }
    disarmDelete()
    isSaving.value = true
    try {
      const nextStatus = await removeWorkflow(workflow.id)
      status.value = nextStatus
      workflows.value = nextStatus.workflows
      if (nextStatus.lastSaveError) {
        showFeedback(`删除失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return
      }
      selectedId.value = workflows.value[0]?.id ?? ''
      syncDraft()
      showFeedback('已删除工作流', 'success')
    } catch (error) {
      showFeedback(`删除失败: ${shortError(formatError(error, '删除工作流失败'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  async function runSelected(workflow = selectedWorkflow.value) {
    if (!workflow) return
    if (draftModified.value && draft.value && workflow.id === selectedWorkflow.value?.id) {
      const saved = await saveDraft()
      if (!saved) return
      workflow = selectedWorkflow.value ?? workflow
    }
    isRunning.value = true
    try {
      let clipboardText = ''
      try {
        clipboardText = await Clipboard.Text()
      } catch {
        clipboardText = ''
      }
      const result = await runWorkflow({
        workflowId: workflow.id,
        input: runInput.value,
        clipboardText,
        confirmed: riskArmedId.value === workflow.id,
      })
      lastRun.value = result
      if (result.requiresConfirmation) {
        armRisk(workflow.id)
        showFeedback('再次点击运行以确认高风险工作流', 'danger')
        return
      }
      disarmRisk()
      if (result.ok && result.output) {
        try {
          await Clipboard.SetText(result.output)
          showFeedback(`${result.message}，结果已复制`, 'success')
        } catch (error) {
          showFeedback(`${result.message}，复制失败: ${shortError(formatError(error, '剪贴板不可用'))}`, 'danger')
        }
      } else {
        showFeedback(result.message, result.ok ? 'success' : 'danger')
      }
    } catch (error) {
      showFeedback(`工作流运行失败: ${shortError(formatError(error, '运行异常'))}`, 'danger')
    } finally {
      isRunning.value = false
    }
  }

  async function exportData() {
    try {
      const result = await exportWorkflows()
      exportResult.value = result
      if (result.ok && result.json) {
        try {
          await Clipboard.SetText(result.json)
          showFeedback(`已导出 ${result.count} 个工作流，JSON 已复制`, 'success')
        } catch {
          showFeedback(`已导出 ${result.count} 个工作流，复制 JSON 失败`, 'danger')
        }
      } else {
        showFeedback(result.message, result.ok ? 'success' : 'danger')
      }
    } catch (error) {
      showFeedback(`工作流导出失败: ${shortError(formatError(error, '导出异常'))}`, 'danger')
    }
  }

  async function importData() {
    const raw = importText.value.trim()
    if (!raw) {
      showFeedback('先粘贴 Fancun 工作流 JSON', 'danger')
      return
    }
    isSaving.value = true
    try {
      const result = await importWorkflows(raw)
      importResult.value = result
      status.value = result.status
      workflows.value = result.status.workflows
      if (result.ok) {
        selectedId.value = workflows.value[0]?.id ?? ''
        importText.value = ''
        syncDraft()
      }
      showFeedback(result.message, result.ok ? 'success' : 'danger')
    } catch (error) {
      showFeedback(`工作流导入失败: ${shortError(formatError(error, '导入异常'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  function updateDraft(patch: Partial<WorkflowDefinition>) {
    if (!draft.value) return
    draft.value = normalizeDraft({ ...draft.value, ...patch })
  }

  function updateStep(index: number, patch: Partial<WorkflowStep>) {
    if (!draft.value) return
    const steps = draft.value.steps.map((step, stepIndex) => stepIndex === index ? { ...step, ...patch } : step)
    draft.value = normalizeDraft({ ...draft.value, steps })
  }

  function addStep() {
    if (!draft.value) return
    draft.value = normalizeDraft({
      ...draft.value,
      steps: [...draft.value.steps, { command: 'url {prev}', pick: '编码结果' }],
    })
  }

  function removeStep(index: number) {
    if (!draft.value) return
    const steps = draft.value.steps.filter((_, stepIndex) => stepIndex !== index)
    draft.value = normalizeDraft({ ...draft.value, steps: steps.length ? steps : [{ command: '', pick: '' }] })
  }

  function syncDraft() {
    const source = selectedWorkflow.value
    draft.value = source ? normalizeDraft(JSON.parse(JSON.stringify(source)) as WorkflowDefinition) : null
    lastRun.value = null
    disarmRisk()
  }

  function armDelete(id: string) {
    disarmDelete()
    deleteArmedId.value = id
    deleteArmedTimer = window.setTimeout(() => {
      deleteArmedTimer = undefined
      if (deleteArmedId.value === id) deleteArmedId.value = ''
    }, ARMED_TIMEOUT_MS)
  }

  function disarmDelete() {
    deleteArmedId.value = ''
    if (deleteArmedTimer !== undefined) {
      window.clearTimeout(deleteArmedTimer)
      deleteArmedTimer = undefined
    }
  }

  function armRisk(id: string) {
    disarmRisk()
    riskArmedId.value = id
    riskArmedTimer = window.setTimeout(() => {
      riskArmedTimer = undefined
      if (riskArmedId.value === id) riskArmedId.value = ''
    }, ARMED_TIMEOUT_MS)
  }

  function disarmRisk() {
    riskArmedId.value = ''
    if (riskArmedTimer !== undefined) {
      window.clearTimeout(riskArmedTimer)
      riskArmedTimer = undefined
    }
  }

  function showFeedback(message: string, tone: 'success' | 'danger' = 'success') {
    if (feedbackTimer !== undefined) {
      window.clearTimeout(feedbackTimer)
      feedbackTimer = undefined
    }
    feedback.value = message
    feedbackTone.value = tone
    const duration = tone === 'danger' ? 5000 : 2000
    feedbackTimer = window.setTimeout(() => {
      feedbackTimer = undefined
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, duration)
  }

  return {
    workflows,
    status,
    selectedId,
    selectedWorkflow,
    draft,
    draftModified,
    runInput,
    importText,
    lastRun,
    exportResult,
    importResult,
    feedback,
    feedbackTone,
    loadError,
    deleteArmedId,
    riskArmedId,
    isLoading,
    isSaving,
    isRunning,
    load,
    select,
    createWorkflow,
    saveDraft,
    deleteWorkflow,
    runSelected,
    exportData,
    importData,
    updateDraft,
    updateStep,
    addStep,
    removeStep,
  }
})

function formatError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message.trim()
  if (typeof error === 'string' && error.trim()) return error.trim()
  return fallback
}

function normalizeDraft(workflow: WorkflowDefinition): WorkflowDefinition {
  return {
    id: String(workflow.id ?? '').trim().toLowerCase(),
    name: String(workflow.name ?? ''),
    description: String(workflow.description ?? ''),
    steps: (workflow.steps ?? []).map((step) => ({
      command: String(step.command ?? ''),
      pick: String(step.pick ?? ''),
    })),
    updatedAt: Number(workflow.updatedAt ?? 0),
  }
}

function shortError(message: string) {
  const text = message.trim()
  return text.length > 72 ? `${text.slice(0, 69)}...` : text
}
