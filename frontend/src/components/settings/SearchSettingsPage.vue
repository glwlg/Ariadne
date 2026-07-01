<script setup lang="ts">
import { AlertTriangle, Database, RotateCcw, Save, Search, ShieldCheck, Trash2 } from '@lucide/vue'
import { computed } from 'vue'
import { useSettingsStore } from '../../stores/settings'
import AriButton from '../ui/AriButton.vue'

const settings = useSettingsStore()
const fileSearch = computed(() => settings.platformStatus?.fileSearch)
const searchPerformance = computed(() => settings.platformStatus?.searchPerformance)

const excludedFolderText = computed({
  get: () => settings.searchExcludeFoldersText(),
  set: (value: string) => settings.setSearchExcludeFolders(value),
})

const excludedPatternText = computed({
  get: () => settings.searchExcludePatternsText(),
  set: (value: string) => settings.setSearchExcludePatterns(value),
})

const searchServiceMissing = computed(() => {
  const status = fileSearch.value
  return Boolean(status && !status.serviceInstalled && !status.serviceRunning)
})

const fileSearchState = computed(() => {
  const status = fileSearch.value
  if (!status) return '未加载'
  if (searchServiceMissing.value) return '等待搜索服务'
  if (status.serviceInstalled && !status.serviceRunning) return '服务已停止'
  if (status.indexing) return '建立中'
  if (status.ready) return '可用'
  return '未就绪'
})

const fileSearchServiceState = computed(() => {
  const status = fileSearch.value
  if (!status) return '未加载'
  if (status.serviceRunning) return '运行中'
  if (status.serviceInstalled) return '已停止'
  return '未安装'
})

const fileSearchSummary = computed(() => {
  const status = fileSearch.value
  if (!status) return '索引状态未加载'
  if (searchServiceMissing.value) return '安装搜索服务后开始建立索引'
  if (status.serviceInstalled && !status.serviceRunning) return '启动搜索服务后继续维护索引'
  if (status.lastQuery) return `${status.lastQuery} ${formatMs(status.lastElapsedMs)} / ${status.lastResultCount} 项`
  return `已索引 ${status.indexedCount} 项`
})

const fileSearchServiceSummary = computed(() => {
  const status = fileSearch.value
  if (!status) return '服务状态未加载'
  if (status.serviceRunning) return status.serviceName || 'AriadneFileSearch'
  if (status.serviceInstalled) return status.serviceState || 'stopped'
  return '安装后自动维护本机文件索引'
})

const fileSearchLastError = computed(() => {
  const error = fileSearch.value?.lastError?.trim() ?? ''
  if (!error) return ''
  if (isPrivilegeMessage(error)) return ''
  return error
})

const fileSearchNotice = computed(() => {
  if (searchServiceMissing.value) return '搜索服务未安装。安装后会自动维护本机文件索引。'
  const status = fileSearch.value
  if (status?.serviceInstalled && !status.serviceRunning) return '搜索服务未运行。'
  const hint = status?.coverageHint?.trim() ?? ''
  if (!hint) return ''
  if (status?.serviceRunning && isSearchServiceStateMessage(hint)) return ''
  return isPrivilegeMessage(hint) ? '搜索服务未运行。' : hint
})

const excludedFolderCount = computed(() => settings.settings?.search?.fileExcludeFolders?.length ?? 0)
const excludedPatternCount = computed(() => settings.settings?.search?.fileExcludePatterns?.length ?? 0)

function formatMs(value?: number) {
  if (!value) return '0ms'
  return `${value}ms`
}

function isPrivilegeMessage(value: string) {
  const lower = value.toLowerCase()
  return value.includes('管理员权限') || lower.includes('access is denied')
}

function isSearchServiceStateMessage(value: string) {
  return value.includes('搜索服务未运行') || value.includes('搜索服务未安装')
}
</script>

<template>
  <section class="settings-panel settings-grid-panel">
    <div class="settings-panel-title">
      <Search :size="15" />
      搜索状态
    </div>
    <div class="settings-status-card">
      <span>搜索性能</span>
      <strong>{{ formatMs(searchPerformance?.p95Ms) }}</strong>
      <small>目标 {{ formatMs(searchPerformance?.targetP95Ms) }} · 样本 {{ searchPerformance?.sampleCount ?? 0 }}</small>
    </div>
    <div class="settings-status-card">
      <span>最近搜索</span>
      <strong>{{ searchPerformance?.lastQuery || '暂无记录' }}</strong>
      <small>{{ formatMs(searchPerformance?.lastElapsedMs) }} · {{ searchPerformance?.lastResultCount ?? 0 }} 项</small>
    </div>
    <div class="settings-status-card">
      <span>搜索收藏/最近使用</span>
      <strong>{{ settings.searchUsageStatus?.count ?? 0 }} 条</strong>
      <small>{{ settings.searchUsageStatus?.path || '搜索数据未初始化' }}</small>
    </div>
    <div class="settings-status-card">
      <span>文件索引</span>
      <strong>{{ fileSearchState }}</strong>
      <small>{{ fileSearchSummary }}</small>
    </div>
    <div class="settings-status-card">
      <span>文件索引服务</span>
      <strong>{{ fileSearchServiceState }}</strong>
      <small>{{ fileSearchServiceSummary }}</small>
    </div>
    <p v-if="fileSearchLastError" class="settings-note is-danger">
      {{ fileSearchLastError }}
    </p>
    <p v-if="fileSearchNotice" class="settings-note is-warning">
      {{ fileSearchNotice }}
    </p>
    <p
      v-if="settings.fileSearchServiceActionResult"
      class="settings-note"
      :class="{ 'is-danger': !settings.fileSearchServiceActionResult.ok }"
    >
      {{ settings.fileSearchServiceActionResult.message }}
    </p>
    <p v-for="error in fileSearch?.policyErrors ?? []" :key="error" class="settings-note is-danger">
      排除正则无效：{{ error }}
    </p>
    <div class="settings-inline-actions">
      <AriButton size="sm" variant="secondary" @click="settings.refreshPlatformStatus()">
        <RotateCcw :size="14" />
        刷新状态
      </AriButton>
      <AriButton
        v-if="searchServiceMissing"
        size="sm"
        variant="primary"
        :disabled="settings.isInstallingSearchService"
        @click="settings.installSearchService()"
      >
        <ShieldCheck :size="14" />
        {{ settings.isInstallingSearchService ? '正在安装搜索服务' : '安装搜索服务' }}
      </AriButton>
      <AriButton
        size="sm"
        variant="secondary"
        class="danger-action"
        :disabled="settings.isSaving || !(settings.searchUsageStatus?.count ?? 0)"
        @click="settings.clearSearchUsageState()"
      >
        <Trash2 :size="14" />
        {{ settings.searchUsageClearArmed ? '确认清理搜索数据' : '清理搜索数据' }}
      </AriButton>
    </div>
  </section>

  <section class="settings-panel">
    <div class="settings-panel-title">
      <Database :size="15" />
      文件索引排除
    </div>
    <div class="settings-status-grid">
      <div class="settings-status-card">
        <span>排除文件夹</span>
        <strong>{{ excludedFolderCount }} 个</strong>
        <small>命中路径不会出现在文件搜索结果中</small>
      </div>
      <div class="settings-status-card">
        <span>排除正则</span>
        <strong>{{ excludedPatternCount }} 条</strong>
        <small>按完整路径匹配</small>
      </div>
    </div>
    <label class="settings-field">
      <span>排除文件夹</span>
      <textarea
        v-model="excludedFolderText"
        class="settings-textarea"
        rows="5"
        spellcheck="false"
        placeholder="每行一个文件夹路径"
      ></textarea>
    </label>
    <label class="settings-field">
      <span>排除正则</span>
      <textarea
        v-model="excludedPatternText"
        class="settings-textarea"
        rows="5"
        spellcheck="false"
        placeholder="每行一个 Go 正则表达式"
      ></textarea>
    </label>
    <p class="settings-note">
      默认排除 Windows 最近使用目录。保存后立即生效；后台刷新完成后会重写索引文件。
    </p>
    <p v-if="fileSearch?.policyErrors?.length" class="settings-note is-danger">
      <AlertTriangle :size="14" />
      正则错误会被跳过，请修改后保存。
    </p>
    <div class="settings-inline-actions">
      <AriButton size="sm" variant="primary" :disabled="settings.isSaving" @click="settings.saveSearchSettings()">
        <Save :size="14" />
        {{ settings.isSaving ? '保存中' : '保存排除规则' }}
      </AriButton>
    </div>
  </section>
</template>
