<script setup lang="ts">
import {
  ArrowLeft,
  Braces,
  Database,
  Download,
  Play,
  Plus,
  Save,
  Search,
  ShieldAlert,
  Trash2,
  Upload,
  Workflow,
  X,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAppShellStore } from '../../stores/appShell'
import { useWorkflowsStore } from '../../stores/workflows'

const appShell = useAppShellStore()
const workflows = useWorkflowsStore()

const selected = computed(() => workflows.selectedWorkflow)

onMounted(() => {
  void workflows.load()
})

function formatTime(seconds?: number) {
  if (!seconds) return '未保存'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(seconds * 1000))
}
</script>

<template>
  <main class="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section class="launcher-shell workflow-shell" aria-label="工作流宏中心">
        <header class="tool-header">
          <div class="brand-mark" aria-hidden="true">
            <Workflow :size="18" />
          </div>
          <div class="brand-copy">
            <span>工作流宏</span>
            <small>Command chains, variables, explicit execution feedback</small>
          </div>
          <div class="header-tools">
            <span class="system-pill" :class="workflows.status?.legacyImported ? 'is-on' : ''">
              <Database :size="13" />
              {{ workflows.status?.legacyImported ? '已导入旧配置' : '本地配置' }}
            </span>
            <span class="system-pill">
              <Workflow :size="13" />
              {{ workflows.status?.count ?? workflows.workflows.length }} 个
            </span>
            <AriButton size="sm" variant="secondary" @click="appShell.openLauncher()">
              <ArrowLeft :size="14" />
              启动器
            </AriButton>
          </div>
        </header>

        <div class="tool-toolbar workflow-toolbar">
          <AriButton size="sm" variant="primary" @click="workflows.createWorkflow()">
            <Plus :size="14" />
            新建工作流
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="!workflows.draft || workflows.isSaving" @click="workflows.saveDraft()">
            <Save :size="14" />
            保存工作流
          </AriButton>
          <AriButton size="sm" variant="secondary" :disabled="!selected" @click="workflows.deleteWorkflow(selected)">
            <Trash2 :size="14" />
            {{ selected && workflows.deleteArmedId === selected.id ? '确认删除' : '删除工作流' }}
          </AriButton>
          <AriButton size="sm" variant="secondary" @click="workflows.exportData()">
            <Download :size="14" />
            导出
          </AriButton>
          <div class="hosts-toolbar-spacer" />
          <div class="tool-search workflow-run-input">
            <Search :size="17" />
            <input
              v-model="workflows.runInput"
              spellcheck="false"
              placeholder="可选输入，步骤中用 {input} 引用..."
            />
          </div>
          <AriButton size="sm" variant="primary" :disabled="!selected || workflows.isRunning" @click="workflows.runSelected(selected)">
            <Play :size="14" />
            {{ selected && workflows.riskArmedId === selected.id ? '确认运行' : '运行' }}
          </AriButton>
        </div>

        <div class="workflow-workspace">
          <section class="workflow-list" aria-label="工作流列表">
            <button
              v-for="workflow in workflows.workflows"
              :key="workflow.id"
              class="workflow-row"
              :class="{ 'is-selected': workflow.id === workflows.selectedId }"
              @click="workflows.select(workflow.id)"
            >
              <span class="workflow-row-icon">
                <Workflow :size="15" />
              </span>
              <span class="workflow-row-main">
                <span class="workflow-row-title">{{ workflow.name }}</span>
                <span class="workflow-row-meta">{{ workflow.id }} · {{ workflow.steps.length }} 步 · {{ formatTime(workflow.updatedAt) }}</span>
              </span>
            </button>

            <div v-if="!workflows.workflows.length" class="empty-state">
              <Workflow :size="22" />
              <span>还没有工作流</span>
            </div>
          </section>

          <section class="workflow-editor" aria-label="工作流编辑">
            <template v-if="workflows.draft">
              <div class="workflow-editor-grid">
                <label>
                  <span>ID</span>
                  <input
                    :value="workflows.draft.id"
                    class="settings-input"
                    spellcheck="false"
                    placeholder="clip-md5"
                    @input="workflows.updateDraft({ id: ($event.target as HTMLInputElement).value })"
                  />
                </label>
                <label>
                  <span>名称</span>
                  <input
                    :value="workflows.draft.name"
                    class="settings-input"
                    spellcheck="false"
                    placeholder="剪贴板文本 -> MD5"
                    @input="workflows.updateDraft({ name: ($event.target as HTMLInputElement).value })"
                  />
                </label>
              </div>

              <label class="workflow-description">
                <span>说明</span>
                <input
                  :value="workflows.draft.description"
                  class="settings-input"
                  spellcheck="false"
                  placeholder="描述这个命令链的用途"
                  @input="workflows.updateDraft({ description: ($event.target as HTMLInputElement).value })"
                />
              </label>

              <div class="workflow-step-header">
                <div>
                  <span class="preview-kicker">STEPS</span>
                  <h2>命令链步骤</h2>
                </div>
                <AriButton size="sm" variant="secondary" @click="workflows.addStep()">
                  <Plus :size="14" />
                  添加步骤
                </AriButton>
              </div>

              <div class="workflow-steps">
                <div v-for="(step, index) in workflows.draft.steps" :key="index" class="workflow-step">
                  <span class="workflow-step-index">{{ index + 1 }}</span>
                  <label>
                    <span>命令</span>
                    <input
                      :value="step.command"
                      class="settings-input"
                      spellcheck="false"
                      placeholder="hash {clipboard}"
                      @input="workflows.updateStep(index, { command: ($event.target as HTMLInputElement).value })"
                    />
                  </label>
                  <label>
                    <span>选择结果</span>
                    <input
                      :value="step.pick"
                      class="settings-input"
                      spellcheck="false"
                      placeholder="MD5 / 编码结果"
                      @input="workflows.updateStep(index, { pick: ($event.target as HTMLInputElement).value })"
                    />
                  </label>
                  <button class="icon-button" type="button" @click="workflows.removeStep(index)">
                    <X :size="15" />
                  </button>
                </div>
              </div>
            </template>

            <div v-else class="empty-state">
              <Workflow :size="22" />
              <span>选择一个工作流进行编辑</span>
            </div>
          </section>

          <aside class="workflow-run-panel" aria-label="工作流运行结果">
            <div class="side-panel">
              <Braces :size="16" />
              <strong>变量</strong>
              <small>{clipboard} 当前剪贴板 · {input} 本次输入 · {prev} 上一步输出</small>
            </div>

            <div class="side-panel">
              <Database :size="16" />
              <strong>{{ workflows.status?.path || '%APPDATA%/Ariadne/workflows.json' }}</strong>
              <small>旧配置：{{ workflows.status?.legacyPath || '%APPDATA%/x-tools/config.json' }}</small>
            </div>

            <div class="side-panel workflow-data-panel">
              <div class="side-title">
                <Download :size="15" />
                导入导出
              </div>
              <p>导出会写入 Ariadne exports 目录，并把 JSON 复制到剪贴板；导入只接受 Ariadne 工作流 JSON。</p>
              <small v-if="workflows.exportResult?.path">{{ workflows.exportResult.path }}</small>
              <textarea
                v-model="workflows.importText"
                class="workflow-import-textarea"
                spellcheck="false"
                placeholder="粘贴导出的 workflows JSON..."
              />
              <div class="workflow-side-actions">
                <AriButton size="sm" variant="secondary" @click="workflows.exportData()">
                  <Download :size="14" />
                  导出
                </AriButton>
                <AriButton size="sm" variant="secondary" :disabled="workflows.isSaving" @click="workflows.importData()">
                  <Upload :size="14" />
                  导入
                </AriButton>
              </div>
              <small v-if="workflows.importResult">{{ workflows.importResult.message }}</small>
            </div>

            <template v-if="workflows.lastRun">
              <div class="workflow-run-summary" :class="{ 'is-ok': workflows.lastRun.ok }">
                <span>{{ workflows.lastRun.requiresConfirmation ? '待确认' : workflows.lastRun.ok ? '完成' : '失败' }}</span>
                <strong>{{ workflows.lastRun.message }}</strong>
              </div>
              <div v-if="workflows.lastRun.requiresConfirmation" class="workflow-risk-panel">
                <div class="side-title">
                  <ShieldAlert :size="15" />
                  高风险确认
                </div>
                <small v-for="reason in workflows.lastRun.riskReasons" :key="reason">{{ reason }}</small>
              </div>
              <div class="workflow-run-steps">
                <div v-for="step in workflows.lastRun.steps" :key="step.index" class="workflow-run-step" :class="{ 'is-ok': step.ok }">
                  <span>{{ step.index }}</span>
                  <div>
                    <strong>{{ step.renderedCommand || step.command }}</strong>
                    <small>{{ step.ok ? step.pickedTitle : step.message }}</small>
                    <code v-if="step.output">{{ step.output }}</code>
                  </div>
                </div>
              </div>
            </template>

            <pre v-else class="hosts-diff">运行后会显示每一步渲染后的命令、命中结果和最终复制内容。</pre>
          </aside>
        </div>

        <footer class="status-strip">
          <span>
            <Workflow :size="14" />
            工作流动作由结果显式声明
          </span>
          <span>
            <Braces :size="14" />
            不允许递归调用工作流
          </span>
          <span v-if="workflows.feedback" class="inline-feedback">{{ workflows.feedback }}</span>
        </footer>
      </section>
    </div>
  </main>
</template>
