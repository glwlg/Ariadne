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
  setHostsProfileEnabled,
  upsertHostsProfile,
} from '../services/hostsApi'
import type { HostsApplyPreview, HostsProfile, HostsStatus } from '../types/ariadne'

export const useHostsStore = defineStore('hosts', () => {
  const profiles = ref<HostsProfile[]>([])
  const status = ref<HostsStatus | null>(null)
  const selectedId = ref('')
  const draft = ref<HostsProfile | null>(null)
  const preview = ref<HostsApplyPreview | null>(null)
  const feedback = ref('')
  const deleteArmedId = ref('')
  const applyArmed = ref(false)
  const isLoading = ref(false)
  const isSaving = ref(false)

  const selectedProfile = computed(() => profiles.value.find((profile) => profile.id === selectedId.value) ?? profiles.value[0] ?? null)
  const enabledCount = computed(() => status.value?.enabledCount ?? profiles.value.filter((profile) => profile.enabled && !profile.system).length)

  async function load() {
    isLoading.value = true
    try {
      const [nextStatus, nextProfiles] = await Promise.all([getHostsStatus(), listHostsProfiles()])
      status.value = nextStatus
      profiles.value = nextProfiles
      if (!profiles.value.some((profile) => profile.id === selectedId.value)) {
        selectedId.value = profiles.value[0]?.id ?? ''
      }
      syncDraft()
    } catch {
      showFeedback('Hosts 加载失败')
    } finally {
      isLoading.value = false
    }
  }

  function select(id: string) {
    selectedId.value = id
    deleteArmedId.value = ''
    syncDraft()
  }

  function syncDraft() {
    const selected = selectedProfile.value
    draft.value = selected ? { ...selected } : null
  }

  async function createProfile() {
    status.value = await newHostsProfile()
    profiles.value = status.value.profiles
    selectedId.value = profiles.value.find((profile) => !profile.system)?.id ?? profiles.value[0]?.id ?? ''
    syncDraft()
    showFeedback('已新建方案')
  }

  async function saveDraft() {
    if (!draft.value || draft.value.system) return
    isSaving.value = true
    try {
      status.value = await upsertHostsProfile(draft.value)
      profiles.value = status.value.profiles
      showFeedback(status.value.lastSaveError ? `保存失败: ${shortError(status.value.lastSaveError)}` : '已保存方案')
      syncDraft()
    } finally {
      isSaving.value = false
    }
  }

  async function toggleEnabled(profile: HostsProfile) {
    if (profile.system) return
    status.value = await setHostsProfileEnabled(profile.id, !profile.enabled)
    profiles.value = status.value.profiles
    showFeedback(status.value.lastSaveError ? `保存失败: ${shortError(status.value.lastSaveError)}` : profile.enabled ? '已停用方案' : '已启用方案')
    syncDraft()
  }

  async function deleteProfile(profile = selectedProfile.value) {
    if (!profile || profile.system) return
    if (deleteArmedId.value !== profile.id) {
      deleteArmedId.value = profile.id
      showFeedback('再次点击确认删除方案')
      return
    }
    status.value = await removeHostsProfile(profile.id)
    profiles.value = status.value.profiles
    selectedId.value = profiles.value[0]?.id ?? ''
    deleteArmedId.value = ''
    syncDraft()
    showFeedback(status.value.lastSaveError ? `删除失败: ${shortError(status.value.lastSaveError)}` : '已删除方案')
  }

  async function fetchRemote(profile = selectedProfile.value) {
    if (!profile || profile.system || profile.type !== 'remote') return
    status.value = await fetchRemoteHosts(profile.id)
    profiles.value = status.value.profiles
    syncDraft()
    showFeedback(status.value.lastRemoteError ? `拉取失败: ${shortError(status.value.lastRemoteError)}` : '已拉取远程 Hosts')
  }

  async function buildPreview() {
    preview.value = await previewHostsApply()
    applyArmed.value = false
    showFeedback(preview.value.conflicts.length ? `发现 ${preview.value.conflicts.length} 个冲突域名` : '已生成应用预览')
  }

  async function applyHosts() {
    if (!applyArmed.value) {
      preview.value = await previewHostsApply()
      applyArmed.value = true
      showFeedback('检查预览后再次点击确认写入')
      return
    }
    const result = await applyEnabledHostsProfiles(true)
    preview.value = result.preview
    applyArmed.value = false
    showFeedback(result.ok ? result.message : shortError(result.lastApplyError || result.message))
    await load()
  }

  function updateDraft(patch: Partial<HostsProfile>) {
    if (!draft.value) return
    draft.value = { ...draft.value, ...patch }
    applyArmed.value = false
  }

  function showFeedback(message: string) {
    feedback.value = message
    window.setTimeout(() => {
      if (feedback.value === message) {
        feedback.value = ''
      }
    }, 1800)
  }

  return {
    profiles,
    status,
    selectedId,
    selectedProfile,
    draft,
    preview,
    feedback,
    deleteArmedId,
    applyArmed,
    isLoading,
    isSaving,
    enabledCount,
    load,
    select,
    createProfile,
    saveDraft,
    toggleEnabled,
    deleteProfile,
    fetchRemote,
    buildPreview,
    applyHosts,
    updateDraft,
  }
})

function shortError(message: string) {
  const text = message.trim()
  return text.length > 90 ? `${text.slice(0, 87)}...` : text
}
