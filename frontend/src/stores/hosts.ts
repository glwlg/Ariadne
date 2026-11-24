import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  applyEnabledHostsProfiles,
  fetchRemoteHosts,
  getHostsStatus,
  listHostsProfiles,
  newHostsProfile,
  previewHostsApply,
  removeHostsProfile,
  rollbackHosts as rollbackHostsApi,
  setHostsProfileEnabled,
  upsertHostsProfile,
} from '../services/hostsApi'
import type { HostsApplyPreview, HostsProfile, HostsStatus } from '../types/fancun'

const ARMED_TIMEOUT_MS = 5000

export const useHostsStore = defineStore('hosts', () => {
  const profiles = ref<HostsProfile[]>([])
  const status = ref<HostsStatus | null>(null)
  const selectedId = ref('')
  const draft = ref<HostsProfile | null>(null)
  const preview = ref<HostsApplyPreview | null>(null)
  const feedback = ref('')
  const feedbackTone = ref<'success' | 'danger'>('success')
  const loadError = ref('')
  const deleteArmedId = ref('')
  const applyArmed = ref(false)
  const rollbackArmed = ref(false)
  const isLoading = ref(false)
  const isSaving = ref(false)
  const isFetching = ref(false)
  const isApplying = ref(false)
  const isRollingBack = ref(false)
  let feedbackTimer: number | undefined
  let deleteArmedTimer: number | undefined
  let applyArmedTimer: number | undefined
  let rollbackArmedTimer: number | undefined

  const selectedProfile = computed(() => profiles.value.find((profile) => profile.id === selectedId.value) ?? profiles.value[0] ?? null)
  const enabledCount = computed(() => status.value?.enabledCount ?? profiles.value.filter((profile) => profile.enabled && !profile.system).length)
  const canRollback = computed(() => Boolean(status.value?.canRollback))
  const lastBackupAt = computed(() => status.value?.lastBackupAt ?? '')
  const draftModified = computed(() => {
    const selected = selectedProfile.value
    if (!draft.value || !selected) return false
    return (
      draft.value.title !== selected.title ||
      draft.value.content !== selected.content ||
      draft.value.type !== selected.type ||
      (draft.value.url ?? '') !== (selected.url ?? '')
    )
  })

  async function load() {
    isLoading.value = true
    loadError.value = ''
    try {
      const [nextStatus, nextProfiles] = await Promise.all([getHostsStatus(), listHostsProfiles()])
      status.value = nextStatus
      profiles.value = nextProfiles
      if (!profiles.value.some((profile) => profile.id === selectedId.value)) {
        selectedId.value = profiles.value[0]?.id ?? ''
      }
      syncDraft()
    } catch (error) {
      loadError.value = formatError(error, 'Hosts 加载失败')
      showFeedback(`Hosts 加载失败: ${shortError(loadError.value)}`, 'danger')
    } finally {
      isLoading.value = false
    }
  }

  async function select(id: string) {
    clearArmedFlags()
    if (id === selectedId.value) {
      return
    }
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) {
        return
      }
    }
    selectedId.value = id
    syncDraft()
  }

  function syncDraft() {
    const selected = selectedProfile.value
    draft.value = selected ? { ...selected } : null
  }

  function discardDraft() {
    clearArmedFlags()
    syncDraft()
  }

  async function createProfile() {
    clearArmedFlags()
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) return
    }
    const beforeIds = new Set(profiles.value.map((profile) => profile.id))
    isSaving.value = true
    try {
      const nextStatus = await newHostsProfile()
      status.value = nextStatus
      profiles.value = nextStatus.profiles
      if (nextStatus.lastSaveError) {
        showFeedback(`新建失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return
      }
      const created =
        profiles.value.find((profile) => !profile.system && !beforeIds.has(profile.id)) ??
        [...profiles.value]
          .filter((profile) => !profile.system)
          .sort((a, b) => (b.updatedAt ?? 0) - (a.updatedAt ?? 0))[0] ??
        profiles.value[0]
      selectedId.value = created?.id ?? ''
      syncDraft()
      showFeedback('已新建方案', 'success')
    } catch (error) {
      showFeedback(`新建失败: ${shortError(formatError(error, '新建方案失败'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  async function saveDraft(): Promise<boolean> {
    if (!draft.value || draft.value.system) return false
    isSaving.value = true
    try {
      status.value = await upsertHostsProfile(draft.value)
      profiles.value = status.value.profiles
      if (status.value.lastSaveError) {
        showFeedback(`保存失败: ${shortError(status.value.lastSaveError)}`, 'danger')
        return false
      }
      if (!profiles.value.some((profile) => profile.id === selectedId.value)) {
        selectedId.value = draft.value.id || profiles.value[0]?.id || ''
      }
      syncDraft()
      showFeedback('已保存方案', 'success')
      return true
    } catch (error) {
      showFeedback(`保存失败: ${shortError(formatError(error, '保存方案失败'))}`, 'danger')
      return false
    } finally {
      isSaving.value = false
    }
  }

  async function toggleEnabled(profile: HostsProfile) {
    if (profile.system) return
    disarmApply()
    disarmRollback()
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) return
    }
    isSaving.value = true
    try {
      const nextStatus = await setHostsProfileEnabled(profile.id, !profile.enabled)
      status.value = nextStatus
      profiles.value = nextStatus.profiles
      if (nextStatus.lastSaveError) {
        showFeedback(`保存失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return
      }
      syncDraft()
      showFeedback(profile.enabled ? '已停用方案' : '已启用方案', 'success')
    } catch (error) {
      showFeedback(`切换失败: ${shortError(formatError(error, '切换启用状态失败'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  async function deleteProfile(profile = selectedProfile.value) {
    if (!profile || profile.system) return
    disarmApply()
    disarmRollback()
    if (deleteArmedId.value !== profile.id) {
      armDelete(profile.id)
      showFeedback('再次点击确认删除方案', 'danger')
      return
    }
    disarmDelete()
    isSaving.value = true
    try {
      const nextStatus = await removeHostsProfile(profile.id)
      status.value = nextStatus
      profiles.value = nextStatus.profiles
      if (nextStatus.lastSaveError) {
        showFeedback(`删除失败: ${shortError(nextStatus.lastSaveError)}`, 'danger')
        return
      }
      selectedId.value = profiles.value[0]?.id ?? ''
      syncDraft()
      showFeedback('已删除方案', 'success')
    } catch (error) {
      showFeedback(`删除失败: ${shortError(formatError(error, '删除方案失败'))}`, 'danger')
    } finally {
      isSaving.value = false
    }
  }

  async function fetchRemote(profile = selectedProfile.value) {
    if (!profile || profile.system || profile.type !== 'remote') return
    disarmApply()
    disarmRollback()
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) return
    }
    isFetching.value = true
    try {
      status.value = await fetchRemoteHosts(profile.id)
      profiles.value = status.value.profiles
      if (status.value.lastRemoteError) {
        showFeedback(`拉取失败: ${shortError(status.value.lastRemoteError)}`, 'danger')
        return
      }
      syncDraft()
      showFeedback('已拉取远程 Hosts', 'success')
    } catch (error) {
      showFeedback(`拉取失败: ${shortError(formatError(error, '远程拉取失败'))}`, 'danger')
    } finally {
      isFetching.value = false
    }
  }

  async function buildPreview() {
    disarmApply()
    disarmRollback()
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) return
    }
    try {
      preview.value = await previewHostsApply()
      if (preview.value.lastPreviewError) {
        showFeedback(`预览失败: ${shortError(preview.value.lastPreviewError)}`, 'danger')
        return
      }
      showFeedback(
        preview.value.conflicts.length ? `发现 ${preview.value.conflicts.length} 个冲突域名` : '已生成应用预览',
        preview.value.conflicts.length ? 'danger' : 'success',
      )
    } catch (error) {
      showFeedback(`预览失败: ${shortError(formatError(error, '生成预览失败'))}`, 'danger')
    }
  }

  async function applyHosts() {
    disarmRollback()
    if (draftModified.value && draft.value && !draft.value.system) {
      const saved = await saveDraft()
      if (!saved) return
    }
    if (!applyArmed.value) {
      try {
        preview.value = await previewHostsApply()
        armApply()
        showFeedback('检查预览后再次点击确认写入', 'success')
      } catch (error) {
        showFeedback(`预览失败: ${shortError(formatError(error, '生成预览失败'))}`, 'danger')
      }
      return
    }
    disarmApply()
    isApplying.value = true
    try {
      const result = await applyEnabledHostsProfiles(true)
      preview.value = result.preview
      showFeedback(
        result.ok ? result.message : shortError(result.lastApplyError || result.message),
        result.ok ? 'success' : 'danger',
      )
      await load()
    } catch (error) {
      showFeedback(`写入失败: ${shortError(formatError(error, '写入系统 Hosts 失败'))}`, 'danger')
    } finally {
      isApplying.value = false
    }
  }

  async function rollbackHosts() {
    disarmApply()
    if (!rollbackArmed.value) {
      armRollback()
      showFeedback('再次点击确认回滚系统 Hosts', 'danger')
      return
    }
    disarmRollback()
    isRollingBack.value = true
    try {
      const result = await rollbackHostsApi()
      preview.value = result.preview
      showFeedback(
        result.ok ? result.message : shortError(result.lastApplyError || result.message),
        result.ok ? 'success' : 'danger',
      )
      await load()
    } catch (error) {
      showFeedback(`回滚失败: ${shortError(formatError(error, '回滚系统 Hosts 失败'))}`, 'danger')
    } finally {
      isRollingBack.value = false
    }
  }

  function updateDraft(patch: Partial<HostsProfile>) {
    if (!draft.value) return
    draft.value = { ...draft.value, ...patch }
    disarmApply()
    disarmRollback()
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

  function armApply() {
    disarmApply()
    applyArmed.value = true
    applyArmedTimer = window.setTimeout(() => {
      applyArmedTimer = undefined
      applyArmed.value = false
    }, ARMED_TIMEOUT_MS)
  }

  function disarmApply() {
    applyArmed.value = false
    if (applyArmedTimer !== undefined) {
      window.clearTimeout(applyArmedTimer)
      applyArmedTimer = undefined
    }
  }

  function armRollback() {
    disarmRollback()
    rollbackArmed.value = true
    rollbackArmedTimer = window.setTimeout(() => {
      rollbackArmedTimer = undefined
      rollbackArmed.value = false
    }, ARMED_TIMEOUT_MS)
  }

  function disarmRollback() {
    rollbackArmed.value = false
    if (rollbackArmedTimer !== undefined) {
      window.clearTimeout(rollbackArmedTimer)
      rollbackArmedTimer = undefined
    }
  }

  function clearArmedFlags() {
    disarmDelete()
    disarmApply()
    disarmRollback()
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
    profiles,
    status,
    selectedId,
    selectedProfile,
    draft,
    draftModified,
    preview,
    feedback,
    feedbackTone,
    loadError,
    deleteArmedId,
    applyArmed,
    rollbackArmed,
    canRollback,
    lastBackupAt,
    isLoading,
    isSaving,
    isFetching,
    isApplying,
    isRollingBack,
    enabledCount,
    load,
    select,
    createProfile,
    saveDraft,
    discardDraft,
    toggleEnabled,
    deleteProfile,
    fetchRemote,
    buildPreview,
    applyHosts,
    rollbackHosts,
    updateDraft,
  }
})

function formatError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message.trim()
  if (typeof error === 'string' && error.trim()) return error.trim()
  return fallback
}

function shortError(message: string) {
  const text = message.trim()
  return text.length > 90 ? `${text.slice(0, 87)}...` : text
}
