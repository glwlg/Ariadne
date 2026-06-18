<script setup lang="ts">
import { toRefs } from 'vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  Brain,
  Check,
  Copy,
  Download,
  Flag,
  KeyRound,
  Shield,
  Sparkles,
  Trash2,
  Workflow,
  X,
  activeAssetFocus,
  assetMissingEvidence,
  assetReadinessParts,
  assetReadinessScore,
  autonomousInboxSummary,
  autonomousKindLabel,
  autonomousRejectReason,
  beginRejectAutonomousArtifact,
  buildCurrentMemoryTaskPackage,
  cancelRejectAutonomousArtifact,
  confidenceLabel,
  confirmRejectAutonomousArtifact,
  copyAutonomousArtifact,
  copyCurrentAgentTask,
  focusAsset,
  memory,
  rejectingAutonomousArtifactId,
} = toRefs(ctx)
</script>

<template>
<section class="flow-page-panel flow-assets-page" aria-label="心流资产">
          <FlowPageHeader :eyebrow="`资产库 > 代理任务 > ${memory.agentTask?.goal || '待生成任务包'}`" title="从记忆沉淀成可复用能力" />

          <div class="flow-package-hub">
            <aside class="flow-package-library" aria-label="资产库">
              <button type="button" :class="{ 'is-active': activeAssetFocus === 'agent' }" @click="focusAsset('agent')">
                <Workflow :size="16" />
                <strong>代理任务</strong>
                <small>{{ memory.agentTask ? 'v1.2 · 已保存' : '15 · 待生成' }}</small>
              </button>
              <button type="button" :class="{ 'is-active': activeAssetFocus === 'workflow' }" @click="focusAsset('workflow')">
                <Workflow :size="16" />
                <strong>工作流</strong>
                <small>{{ memory.workflowDraft ? `${memory.workflowDraft.steps.length} 步 · 可保存` : '7 · 待生成' }}</small>
              </button>
              <button type="button" :class="{ 'is-active': activeAssetFocus === 'checklist' }" @click="focusAsset('checklist')">
                <Check :size="16" />
                <strong>检查清单</strong>
                <small>{{ memory.checklistDraft ? `${memory.checklistDraft.items.length} 项 · 可保存` : '8 · 待生成' }}</small>
              </button>
              <button type="button" :class="{ 'is-active': activeAssetFocus === 'skill' }" @click="focusAsset('skill')">
                <Brain :size="16" />
                <strong>Skill</strong>
                <small>{{ memory.knowledgeDraftSaveResult?.ok ? '6 · 可导出' : '6 · 待保存' }}</small>
              </button>
            </aside>

            <section class="flow-agent-package" data-flow-asset="agent" aria-label="代理任务包">
              <span>AGENT PACKAGE</span>
              <h2>{{ memory.agentTask?.goal || '代理任务包未生成' }}</h2>
              <div class="flow-agent-package-section flow-package-section-numbered">
                <strong><b>#1</b> 目标 Goal</strong>
                <p>{{ memory.agentTask?.goal || '从当前记忆、洞察或草稿生成可交给外部 agent 的任务。' }}</p>
              </div>
              <div class="flow-agent-package-section flow-package-section-numbered">
                <strong><b>#2</b> 背景 Context</strong>
                <p>{{ memory.agentTask?.context || '生成后会在这里展示上下文、留痕、边界和验收标准。' }}</p>
              </div>
              <div class="flow-agent-package-grid">
                <section>
                  <strong><b>#3</b> 留痕 Trace</strong>
                  <span v-for="item in memory.agentTask?.evidence || []" :key="item">{{ item }}</span>
                  <small v-if="!memory.agentTask?.evidence.length">等待绑定留痕</small>
                </section>
                <section>
                  <strong><b>#4</b> 边界 Boundaries</strong>
                  <span v-for="item in memory.agentTask?.boundaries || []" :key="item">{{ item }}</span>
                  <small v-if="!memory.agentTask?.boundaries.length">执行前需要确认权限和范围</small>
                </section>
                <section>
                  <strong><b>#5</b> 验收 Acceptance</strong>
                  <span v-for="item in memory.agentTask?.acceptance || []" :key="item">{{ item }}</span>
                  <small v-if="!memory.agentTask?.acceptance.length">需要可验证结果</small>
                </section>
              </div>
              <div class="flow-agent-package-actions">
                <AriButton size="sm" variant="secondary" @click="buildCurrentMemoryTaskPackage()">
                  <Workflow :size="14" />
                  生成任务包
                </AriButton>
                <AriButton size="sm" variant="primary" :disabled="!memory.agentTask" @click="copyCurrentAgentTask()">
                  <Copy :size="14" />
                  复制任务包
                </AriButton>
              </div>
            </section>

            <aside class="flow-package-inspector flow-agent-inspector" aria-label="任务包检查">
              <header class="flow-agent-inspector-head">
                <div>
                  <span>Agent Inspector</span>
                  <strong>就绪度评估</strong>
                </div>
                <small>{{ assetReadinessScore }}/100</small>
              </header>
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Shield :size="15" />
                  Readiness
                </div>
                <div class="flow-readiness-ring" :style="{ '--score': `${assetReadinessScore}%` }">
                  <strong>{{ assetReadinessScore }}</strong>
                  <small>/100</small>
                </div>
                <strong>{{ memory.agentTask ? '可交接，需人工确认' : '等待生成' }}</strong>
                <p>{{ memory.agentTask?.requiresReview ? '这个任务包包含边界约束，外发前需要你确认。' : '生成后会检查缺失留痕、权限边界和验收标准。' }}</p>
                <div class="flow-asset-mini-list">
                  <span v-for="part in assetReadinessParts" :key="part.label" :class="{ 'is-ok': part.ok }">{{ part.label }}</span>
                </div>
              </section>
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Flag :size="15" />
                  缺失留痕与风险边界
                </div>
                <span v-for="item in assetMissingEvidence" :key="item" class="flow-risk-chip">{{ item }}</span>
                <p>数据敏感度、本机文件访问和联系人触达都需要安装或外发前确认。</p>
              </section>
              <section class="flow-quiet-panel">
                <div class="side-title">
                  <Sparkles :size="15" />
                  自主产物
                </div>
                <strong>{{ autonomousInboxSummary }}</strong>
                <p>未删除即默认采纳；删除时说明原因，心流会减少同类产物。</p>
                <article v-for="artifact in memory.autonomousArtifacts.slice(0, 3)" :key="artifact.id" class="flow-auto-artifact flow-package-artifact">
                  <div class="flow-auto-artifact-kicker">
                    <span>{{ autonomousKindLabel(artifact.kind) }}</span>
                    <small v-if="artifact.confidence">{{ confidenceLabel(artifact.confidence) }}</small>
                    <small>留痕 {{ artifact.evidence.length }} 条</small>
                  </div>
                  <h3>{{ artifact.title }}</h3>
                  <p>{{ artifact.summary }}</p>
                  <div v-if="rejectingAutonomousArtifactId === artifact.id" class="flow-auto-reject">
                    <input
                      v-model="autonomousRejectReason"
                      type="text"
                      placeholder="删除原因"
                      @keydown.enter.prevent="confirmRejectAutonomousArtifact(artifact)"
                      @keydown.esc.prevent="cancelRejectAutonomousArtifact()"
                    />
                    <button type="button" @click="confirmRejectAutonomousArtifact(artifact)">
                      <Check :size="13" />
                      确认
                    </button>
                    <button type="button" @click="cancelRejectAutonomousArtifact()">
                      <X :size="13" />
                    </button>
                  </div>
                  <div v-else class="flow-auto-artifact-foot">
                    <button type="button" @click.stop="copyAutonomousArtifact(artifact)">
                      <Copy :size="13" />
                      复制
                    </button>
                    <button type="button" @click.stop="beginRejectAutonomousArtifact(artifact)">
                      <Trash2 :size="13" />
                      删除
                    </button>
                  </div>
                </article>
              </section>
            </aside>
          </div>

          <div class="flow-package-suggestions">
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'workflow' }" data-flow-asset="workflow">
              <span>候选工作流 · 预览工作流 →</span>
              <h2>{{ memory.workflowDraft?.title || '未生成' }}</h2>
              <p>{{ memory.workflowDraft?.trigger || '从重复流程里生成可保存的启动器工作流草稿。' }}</p>
              <div v-if="memory.workflowDraft" class="draft-step-list">
                <div v-for="step in memory.workflowDraft.steps" :key="step.id" class="draft-step">
                  <span>{{ step.label }}</span>
                  <code>{{ step.command }}</code>
                </div>
              </div>
              <AriButton v-if="memory.workflowDraft" size="sm" variant="secondary" :disabled="memory.isSavingWorkflowDraft" @click="memory.saveCurrentWorkflowDraft()">
                {{ memory.workflowDraftSaveArmed ? '确认保存' : '保存到工作流' }}
              </AriButton>
            </article>
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'checklist' }" data-flow-asset="checklist">
              <span>检查清单 · 预览清单 →</span>
              <h2>{{ memory.checklistDraft?.title || '未生成' }}</h2>
              <p>{{ memory.checklistDraft?.context || '把重复排查经验整理成可审阅清单。' }}</p>
              <ol v-if="memory.checklistDraft" class="draft-checklist">
                <li v-for="item in memory.checklistDraft.items" :key="item">{{ item }}</li>
              </ol>
              <AriButton v-if="memory.checklistDraft" size="sm" variant="secondary" :disabled="memory.isSavingChecklistDraft" @click="memory.saveCurrentChecklistDraft()">
                {{ memory.checklistDraftSaveArmed ? '确认保存' : '保存为清单' }}
              </AriButton>
            </article>
            <article class="flow-asset-card" :class="{ 'is-focus': activeAssetFocus === 'skill' }" data-flow-asset="skill">
              <span>Skill · 预览 Skill →</span>
              <h2>{{ memory.knowledgeDraftSaveResult?.ok ? memory.knowledgeDraftSaveResult.skill.id : '未保存' }}</h2>
              <p>{{ memory.knowledgeSkillInstallResult?.message || '知识草稿保存后，可以导出或安装到 Codex Skill。' }}</p>
              <div class="flow-page-actions">
                <AriButton v-if="memory.knowledgeDraftSaveResult?.ok" size="sm" variant="secondary" :disabled="memory.isExportingKnowledgeSkill" @click="memory.exportCurrentKnowledgeSkill()">
                  <Download :size="14" />
                  {{ memory.knowledgeSkillExportArmed ? '确认导出' : '导出' }}
                </AriButton>
                <AriButton v-if="memory.knowledgeDraftSaveResult?.ok" size="sm" variant="secondary" :disabled="memory.isInstallingKnowledgeSkill" @click="memory.installCurrentKnowledgeSkill()">
                  <KeyRound :size="14" />
                  {{ memory.knowledgeSkillInstallArmed ? '确认安装' : '安装' }}
                </AriButton>
              </div>
            </article>
          </div>
        </section>
</template>
