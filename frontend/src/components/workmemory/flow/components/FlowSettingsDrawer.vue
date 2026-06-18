<script setup lang="ts">
import { Check, Plus, Trash2, X } from '@lucide/vue'
import { toRefs } from 'vue'
import AriButton from '../../../ui/AriButton.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  addAppCaptureProfile,
  appAvatarText,
  appCaptureCandidates,
  appCaptureProfiles,
  canClearSecret,
  canSaveSecret,
  captureScopeLabel,
  displayAppName,
  flowSettingsOpen,
  flowSettingsTab,
  flowSettingsTabs,
  multiMonitorLabel,
  removeAppCaptureProfile,
  runtimeStatusText,
  saveFlowSettings,
  secretSourceLabel,
  selectAppCaptureProfile,
  selectedAppCaptureProfile,
  setFlowSettingsTab,
  settings,
  timeMachineLabel,
  vectorProviderLabel,
  vectorStatusLabel,
  vectorStoreLabel,
} = toRefs(ctx)
</script>

<template>
        <div v-if="flowSettingsOpen" class="flow-settings-backdrop" @click.self="flowSettingsOpen = false">
          <aside class="flow-settings-drawer" data-no-drag aria-label="心流设置">
            <header class="flow-settings-header">
              <div>
                <span>FLOW SETTINGS</span>
                <h2>心流设置</h2>
                <p>采集、索引、模型和隐私边界只在这里维护，通用设置中心不再重复展示。</p>
              </div>
              <button type="button" class="flow-icon-button" aria-label="关闭心流设置" @click="flowSettingsOpen = false">
                <X :size="16" />
              </button>
            </header>

            <div v-if="settings.settings" class="flow-settings-body">
              <section class="flow-settings-overview">
                <div>
                  <span>当前状态</span>
                  <strong>{{ timeMachineLabel }} · {{ vectorStatusLabel }}</strong>
                  <small>{{ runtimeStatusText || '本地心流配置已就绪。' }}</small>
                </div>
                <div class="flow-settings-overview-grid">
                  <span>
                    <small>采集范围</small>
                    <strong>{{ captureScopeLabel }} / {{ multiMonitorLabel }}</strong>
                  </span>
                  <span>
                    <small>模型</small>
                    <strong>{{ vectorProviderLabel }}</strong>
                  </span>
                  <span>
                    <small>向量库</small>
                    <strong>{{ vectorStoreLabel }}</strong>
                  </span>
                </div>
              </section>

              <nav class="flow-settings-tabs" aria-label="心流设置分组">
                <button
                  v-for="tab in flowSettingsTabs"
                  :key="tab.id"
                  type="button"
                  :class="{ 'is-active': flowSettingsTab === tab.id }"
                  @click="setFlowSettingsTab(tab.id)"
                >
                  <strong>{{ tab.label }}</strong>
                  <small>{{ tab.detail }}</small>
                </button>
              </nav>

              <section v-show="flowSettingsTab === 'capture'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>采集与沉淀</span>
                  <small>{{ runtimeStatusText || '本地采集策略' }}</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.enabled" type="checkbox" />
                    <span />
                    <strong>心流总开关</strong>
                    <small>关闭后不采集新上下文，历史仍可搜索。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.timeMachineEnabled" type="checkbox" />
                    <span />
                    <strong>屏幕时间机器</strong>
                    <small>自动沉淀屏幕上下文，受排除规则约束。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.windowSwitchCaptureEnabled" type="checkbox" />
                    <span />
                    <strong>窗口切换触发</strong>
                    <small>前台窗口变化时补一帧留痕。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.autoOcr" type="checkbox" />
                    <span />
                    <strong>自动 OCR</strong>
                    <small>自动识别截图文字；优先使用 GPU OCR，失败后回退本地 RapidOCR。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.draftScheduleEnabled" type="checkbox" />
                    <span />
                    <strong>自动整理</strong>
                    <small>定时生成日报、复盘和经验候选。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.experienceScheduleEnabled" type="checkbox" />
                    <span />
                    <strong>经验发现</strong>
                    <small>后台归纳重复问题和可优化流程。</small>
                  </label>
                </div>

                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>同窗探测秒</span>
                    <input v-model.number="settings.settings.workMemory.autoCaptureIntervalSeconds" type="number" min="10" />
                  </label>
                  <label class="flow-setting-field">
                    <span>窗口稳定秒</span>
                    <input v-model.number="settings.settings.workMemory.windowSwitchCooldownSeconds" type="number" min="3" />
                  </label>
                  <label class="flow-setting-field">
                    <span>整理间隔分钟</span>
                    <input v-model.number="settings.settings.workMemory.draftScheduleIntervalMinutes" type="number" min="15" />
                  </label>
                  <label class="flow-setting-field">
                    <span>截图质量</span>
                    <input v-model.number="settings.settings.workMemory.screenshotQuality" type="number" min="1" max="100" />
                  </label>
                  <label class="flow-setting-field">
                    <span>采集范围</span>
                    <select v-model="settings.settings.workMemory.captureScope">
                      <option value="all_screens">全部屏幕</option>
                      <option value="active_window">前台窗口</option>
                      <option value="primary_screen">主屏幕</option>
                    </select>
                  </label>
                  <label class="flow-setting-field">
                    <span>多屏策略</span>
                    <select v-model="settings.settings.workMemory.multiMonitor">
                      <option value="combined">合并截图</option>
                      <option value="per_monitor">按屏幕分条</option>
                      <option value="primary_only">仅主屏</option>
                    </select>
                  </label>
                </div>

                <div class="flow-app-policy-panel">
                  <div class="flow-settings-section-head">
                    <span>应用采集策略</span>
                    <small>为高频应用设置独立的稳定时间和探测节奏。</small>
                  </div>
                  <div class="flow-app-policy-layout">
                    <div class="flow-app-profile-list" aria-label="已配置应用采集策略">
                      <button
                        v-for="profile in appCaptureProfiles"
                        :key="profile.id"
                        type="button"
                        :class="{ 'is-active': selectedAppCaptureProfile?.id === profile.id }"
                        @click="selectAppCaptureProfile(profile.id)"
                      >
                        <span class="flow-app-avatar">{{ appAvatarText(profile.displayName || profile.processName) }}</span>
                        <span>
                          <strong>{{ profile.displayName || displayAppName(profile.processName) }}</strong>
                          <small>{{ profile.processName }} · {{ profile.enabled ? '独立节奏' : '已暂停' }}</small>
                        </span>
                      </button>
                      <p v-if="!appCaptureProfiles.length" class="flow-app-empty">
                        可从最近应用添加独立采集节奏。
                      </p>
                    </div>

                    <div v-if="selectedAppCaptureProfile" class="flow-app-profile-detail">
                      <div class="flow-app-profile-title">
                        <span class="flow-app-avatar is-large">{{ appAvatarText(selectedAppCaptureProfile.displayName || selectedAppCaptureProfile.processName) }}</span>
                        <span>
                          <strong>{{ selectedAppCaptureProfile.displayName || displayAppName(selectedAppCaptureProfile.processName) }}</strong>
                          <small>{{ selectedAppCaptureProfile.processName }}</small>
                        </span>
                        <button type="button" class="flow-icon-button" aria-label="移除应用策略" @click="removeAppCaptureProfile(selectedAppCaptureProfile.id)">
                          <Trash2 :size="15" />
                        </button>
                      </div>
                      <label class="flow-setting-switch is-compact">
                        <input v-model="selectedAppCaptureProfile.enabled" type="checkbox" />
                        <span />
                        <strong>启用应用策略</strong>
                        <small>关闭后恢复全局采集节奏。</small>
                      </label>
                      <div class="flow-settings-field-grid is-compact">
                        <label class="flow-setting-field">
                          <span>稳定等待秒</span>
                          <input v-model.number="selectedAppCaptureProfile.windowSwitchDelaySeconds" type="number" min="0" max="3600" />
                        </label>
                        <label class="flow-setting-field">
                          <span>驻留探测秒</span>
                          <input v-model.number="selectedAppCaptureProfile.activeIntervalSeconds" type="number" min="10" max="86400" />
                        </label>
                      </div>
                    </div>
                  </div>

                  <div class="flow-app-candidates">
                    <button
                      v-for="candidate in appCaptureCandidates"
                      :key="candidate.id"
                      type="button"
                      @click="addAppCaptureProfile(candidate)"
                    >
                      <span class="flow-app-avatar">{{ appAvatarText(candidate.displayName) }}</span>
                      <span>
                        <strong>{{ candidate.displayName }}</strong>
                        <small>{{ candidate.processName }} · {{ candidate.count }} 条</small>
                      </span>
                      <Plus :size="15" />
                    </button>
                    <p v-if="!appCaptureCandidates.length" class="flow-app-empty">
                      最近应用都已配置，或还没有可用于添加的采集记录。
                    </p>
                  </div>
                </div>

                <div class="flow-settings-source-list">
                  <label v-for="source in settings.memorySources" :key="source.key" class="flow-source-pill">
                    <input
                      type="checkbox"
                      :checked="source.enabled"
                      @change="settings.setMemorySource(source.key, ($event.target as HTMLInputElement).checked)"
                    />
                    <span>{{ source.label }}</span>
                  </label>
                </div>
              </section>

              <section v-show="flowSettingsTab === 'model'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>模型与向量</span>
                  <small>{{ vectorProviderLabel }} · {{ vectorStoreLabel }}</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.enabled" type="checkbox" />
                    <span />
                    <strong>OpenAI Agents SDK 问答</strong>
                    <small>先检索本地留痕，再交给 Ariadne agent sidecar 生成动态回答。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.embeddingEnabled" type="checkbox" />
                    <span />
                    <strong>语义索引</strong>
                    <small>用于“我今天干了什么”这类上下文问答。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.ocrModelEnabled" type="checkbox" />
                    <span />
                    <strong>大模型 OCR</strong>
                    <small>支持 OpenAI-compatible GPU OCR 或 Ollama /api/generate；失败自动回退本地 RapidOCR。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.agentResponsesEnabled" type="checkbox" />
                    <span />
                    <strong>Responses 原生 Skill</strong>
                    <small>兼容接口支持 /responses 时优先用原生 ShellTool；失败回退工具降级。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.externalAgentEnabled" type="checkbox" />
                    <span />
                    <strong>外部代理任务包</strong>
                    <small>需要沉淀 Skill 或工作流时再显式确认。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.ai.codexCollaborationEnabled" type="checkbox" />
                    <span />
                    <strong>Codex 协作</strong>
                    <small>可选扩展；开启后心流问答才交给 Codex runner。</small>
                  </label>
                </div>
                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>AI provider</span>
                    <input v-model="settings.settings.ai.provider" placeholder="openai-compatible" />
                  </label>
                  <label class="flow-setting-field">
                    <span>AI base URL</span>
                    <input v-model="settings.settings.ai.baseUrl" placeholder="http://127.0.0.1:4000/v1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>AI model</span>
                    <input v-model="settings.settings.ai.model" placeholder="glm-5.1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>OCR provider</span>
                    <input v-model="settings.settings.ai.ocrProvider" placeholder="openai-compatible / ollama-generate" />
                  </label>
                  <label class="flow-setting-field">
                    <span>OCR base URL</span>
                    <input v-model="settings.settings.ai.ocrBaseUrl" placeholder="http://192.168.1.11:11434/api/generate" />
                  </label>
                  <label class="flow-setting-field">
                    <span>OCR vision model</span>
                    <input v-model="settings.settings.ai.ocrModel" placeholder="glm-ocr:latest" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding provider</span>
                    <input v-model="settings.settings.ai.embeddingProvider" placeholder="openai-compatible" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding base URL</span>
                    <input v-model="settings.settings.ai.embeddingBaseUrl" placeholder="http://127.0.0.1:4000/v1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Embedding model</span>
                    <input v-model="settings.settings.ai.embeddingModel" placeholder="/model/qwen_eb" />
                  </label>
                  <label class="flow-setting-field">
                    <span>向量存储</span>
                    <select v-model="settings.settings.ai.vectorStoreType">
                      <option value="embedded">内置缓存</option>
                      <option value="milvus">Milvus</option>
                      <option value="disabled">关闭</option>
                    </select>
                  </label>
                  <label class="flow-setting-field">
                    <span>向量 URI</span>
                    <input v-model="settings.settings.ai.vectorStoreUri" placeholder="milvus://192.168.1.100:19530" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Collection</span>
                    <input v-model="settings.settings.ai.vectorCollection" placeholder="ariadne_work_memory" />
                  </label>
                </div>
                <div class="secret-store-block flow-secret-block">
                  <div class="flow-settings-section-head">
                    <span>安全密钥存储</span>
                    <small>
                      {{ settings.secretStatus?.available ? 'Windows Credential Manager 可用' : '当前运行环境未暴露安全存储' }}
                      · {{ settings.secretStatus?.backend || '未检测' }}
                    </small>
                  </div>
                  <div class="secret-store-grid">
                    <div
                      v-for="record in settings.secretStatus?.records ?? []"
                      :key="record.kind"
                      class="secret-store-row"
                      :data-secret-kind="record.kind"
                      :data-secret-active-source="record.activeSource"
                      :data-secret-stored="record.stored ? 'true' : 'false'"
                    >
                      <div class="secret-store-meta">
                        <strong>{{ record.label }}</strong>
                        <small>
                          {{ record.stored ? '已保存' : '未保存' }}
                          · {{ secretSourceLabel(record.activeSource) }}
                        </small>
                        <small class="secret-store-target">{{ record.targetName }}</small>
                        <small v-if="record.envPresent">检测到环境变量：{{ record.envNames.join(' / ') }}</small>
                        <small v-if="record.lastError" class="is-danger">{{ record.lastError }}</small>
                      </div>
                      <input
                        v-model="settings.secretInputs[record.kind]"
                        class="settings-input"
                        type="password"
                        autocomplete="off"
                        placeholder="粘贴后保存到安全存储"
                        :aria-label="`${record.label} 输入`"
                        :data-secret-input="record.kind"
                      />
                      <div class="secret-store-actions">
                        <AriButton
                          size="sm"
                          variant="secondary"
                          :disabled="!canSaveSecret(record.kind)"
                          :data-secret-save="record.kind"
                          @click="settings.saveSecret(record.kind)"
                        >
                          <Check :size="14" />
                          保存
                        </AriButton>
                        <AriButton
                          size="sm"
                          variant="ghost"
                          :disabled="!canClearSecret(record.stored)"
                          :data-secret-clear="record.kind"
                          @click="settings.clearSecret(record.kind)"
                        >
                          <Trash2 :size="14" />
                          {{ settings.secretClearArmedKind === record.kind ? '确认清除' : '清除' }}
                        </AriButton>
                      </div>
                    </div>
                  </div>
                  <p
                    v-if="settings.secretActionResult"
                    class="settings-note"
                    :class="{ 'is-danger': !settings.secretActionResult.ok && !settings.secretActionResult.requiresConfirmation }"
                    data-secret-action-result
                  >
                    {{ settings.secretActionResult.message }}
                  </p>
                </div>
              </section>

              <section v-show="flowSettingsTab === 'privacy'" class="flow-settings-section">
                <div class="flow-settings-section-head">
                  <span>隐私边界与存储</span>
                  <small>排除规则已集中到“规则”页面维护。</small>
                </div>
                <div class="flow-settings-toggle-grid">
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.privacyMode" type="checkbox" />
                    <span />
                    <strong>隐私模式</strong>
                    <small>暂停截图、OCR、embedding、AI 和导出。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.pauseOnIdle" type="checkbox" />
                    <span />
                    <strong>空闲暂停</strong>
                    <small>超过阈值时停止自动采集。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.pauseOnLock" type="checkbox" />
                    <span />
                    <strong>锁屏暂停</strong>
                    <small>锁屏或不可切换桌面时不采集。</small>
                  </label>
                  <label class="flow-setting-switch">
                    <input v-model="settings.settings.workMemory.sensitiveRulesEnabled" type="checkbox" />
                    <span />
                    <strong>敏感内容规则</strong>
                    <small>识别 token、密码、cookie 等风险内容。</small>
                  </label>
                </div>
                <div class="flow-settings-field-grid">
                  <label class="flow-setting-field">
                    <span>空闲阈值秒</span>
                    <input v-model.number="settings.settings.workMemory.idlePauseSeconds" type="number" min="30" />
                  </label>
                  <label class="flow-setting-field">
                    <span>经验发现天数</span>
                    <input v-model.number="settings.settings.workMemory.experienceDiscoveryDays" type="number" min="1" max="365" />
                  </label>
                  <label class="flow-setting-field">
                    <span>记忆保留天数</span>
                    <input v-model.number="settings.settings.workMemory.retentionDays" type="number" min="1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>缩略图保留天数</span>
                    <input v-model.number="settings.settings.workMemory.thumbnailRetentionDays" type="number" min="1" />
                  </label>
                  <label class="flow-setting-field">
                    <span>最大存储 MB</span>
                    <input v-model.number="settings.settings.workMemory.maxStorageMb" type="number" min="128" />
                  </label>
                  <label class="flow-setting-field">
                    <span>Trace</span>
                    <select v-model="settings.settings.ai.traceMode">
                      <option value="off">关闭</option>
                      <option value="local">本地日志</option>
                      <option value="internal">内部观测</option>
                    </select>
                  </label>
                </div>
              </section>
            </div>
            <div v-else class="flow-empty-card">
              <strong>正在读取心流设置</strong>
              <p>配置会从 Ariadne 本地存储与安全存储读取。</p>
            </div>

            <footer class="flow-settings-footer">
              <small>{{ settings.feedback || '心流设置保存在 Ariadne 本地配置中。' }}</small>
              <div class="flow-page-actions">
                <AriButton size="sm" variant="ghost" @click="flowSettingsOpen = false">关闭</AriButton>
                <AriButton size="sm" variant="primary" :disabled="settings.isSaving || !settings.settings" @click="saveFlowSettings()">
                  <Check :size="14" />
                  {{ settings.isSaving ? '保存中' : '保存心流设置' }}
                </AriButton>
              </div>
            </footer>
          </aside>
        </div>
</template>
