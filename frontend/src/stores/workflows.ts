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
import type { WorkflowDefinition, WorkflowExportResult, WorkflowImportResult, WorkflowRunResult, WorkflowStatus, WorkflowStep } from '../types/ariadne'

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
  const deleteArmedId = ref('')
  const riskArmedId = ref('')
  const isLoading = ref(false)
  const isSaving = ref(false)
  const isRunning = ref(false)

  const selectedWorkflow = computed(() => workflows.value.find((workflow) => workflow.id === selectedId.value) ?? workflows.value[0] ?? null)

  async function load() {
    isLoading.value = true
    try {
      const [nextStatus, nextWorkflows] = await Promise.all([getWorkflowStatus(), listWorkflows()])
      status.value = nextStatus
      workflows.value = nextWorkflows
      if (!workflows.value.some((workflow) => workflow.id === selectedId.value)) {
        selectedId.value = workflows.value[0]?.id ?? ''
      }
      syncDraft()
    } catch {
      showFeedback('工作流加载失败')
    } finally {
      isLoading.value = false
    }
  }

  function select(id: string) {
    selectedId.value = id
    deleteArmedId.value = ''
    riskArmedId.value = ''
    syncDraft()
  }

  async function createWorkflow() {
    status.value = await newWorkflow()
    workflows.value = status.value.workflows
    selectedId.value = workflows.value[workflows.value.length - 1]?.id ?? ''
    syncDraft()
    showFeedback('已新建工作流')
  }

  async function saveDraft() {
    if (!draft.value) return
    isSaving.value = true
    try {
      status.value = await upsertWorkflow(draft.value)
      workflows.value = status.value.workflows
      selectedId.value = draft.value.id
      syncDraft()
      showFeedback(status.value.lastSaveError ? `保存失败: ${shortError(status.value.lastSaveError)}` : '已保存工作流')
    } finally {
      isSaving.value = false
    }
  }

  async function deleteWorkflow(workflow = selectedWorkflow.value) {
    if (!workflow) return
    if (deleteArmedId.value !== workflow.id) {
      deleteArmedId.value = workflow.id
      showFeedback('再次点击确认删除')
      return
    }
    status.value = await removeWorkflow(workflow.id)
    workflows.value = status.value.workflows
    selectedId.value = workflows.value[0]?.id ?? ''
    deleteArmedId.value = ''
    syncDraft()
    showFeedback(status.value.lastSaveError ? `删除失败: ${shortError(status.value.lastSaveError)}` : '已删除工作流')
  }

  async function runSelected(workflow = selectedWorkflow.value) {
    if (!workflow) return
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
        riskArmedId.value = workflow.id
        showFeedback('再次点击运行以确认高风险工作流')
        return
      }
      riskArmedId.value = ''
      if (result.ok && result.output) {
        try {
          await Clipboard.SetText(result.output)
          showFeedback(`${result.message}，结果已复制`)
        } catch {
          showFeedback(`${result.message}，复制失败`)
        }
      } else {
        showFeedback(result.message)
      }
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
          showFeedback(`已导出 ${result.count} 个工作流，JSON 已复制`)
        } catch {
          showFeedback(`已导出 ${result.count} 个工作流，复制 JSON 失败`)
        }
      } else {
        showFeedback(result.message)
      }
    } catch {
      showFeedback('工作流导出失败')
    }
  }

  async function importData() {
    const raw = importText.value.trim()
    if (!raw) {
      showFeedback('先粘贴 Ariadne 工作流 JSON')
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
      showFeedback(result.message)
    } catch {
      showFeedback('工作流导入失败')
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
    riskArmedId.value = ''
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 2000)
  }

  return {
    workflows,
    status,
    selectedId,
    selectedWorkflow,
    draft,
    runInput,
    importText,
    lastRun,
    exportResult,
    importResult,
    feedback,
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
