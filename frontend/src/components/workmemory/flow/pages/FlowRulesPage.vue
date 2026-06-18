<script setup lang="ts">
import { toRefs } from 'vue'
import AriField from '../../../ui/AriField.vue'
import AriInput from '../../../ui/AriInput.vue'
import AriSearchBox from '../../../ui/AriSearchBox.vue'
import AriToolbar from '../../../ui/AriToolbar.vue'
import FlowPageHeader from '../components/FlowPageHeader.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  AriButton,
  Camera,
  Database,
  KeyRound,
  Plus,
  RefreshCw,
  Search,
  Shield,
  Trash2,
  Upload,
  captureSourceCards,
  displayAppName,
  exclusionRuleRows,
  exclusionRuleTabs,
  flowDateLabel,
  formatTime,
  globalFlowSearch,
  memory,
  rulesImpactStats,
  rulesPipelineStatus,
  runGlobalFlowSearch,
  topApps,
  vectorProviderLabel,
  vectorStatusLabel,
  vectorStoreLabel,
} = toRefs(ctx)
</script>

<template>
<section class="flow-page-panel flow-rules-page" aria-label="心流规则">
          <FlowPageHeader eyebrow="RULES" title="采集边界和索引 · 本地模式" />
          <AriToolbar class="flow-page-toolbar">
            <span>日期 {{ flowDateLabel }}</span>
            <span>范围 今日</span>
            <AriSearchBox v-model="globalFlowSearch" class="flow-global-search is-compact" compact placeholder="搜索规则、进程、窗口关键词..." @keydown.enter.prevent="runGlobalFlowSearch()" />
            <button type="button">
              <Shield :size="14" />
              通知
            </button>
          </AriToolbar>
          <div class="flow-pipeline-room">
            <aside class="flow-pipeline-side">
              <section class="side-panel flow-capture-source-panel">
                <div class="side-title">
                  <Camera :size="15" />
                  捕获来源
                </div>
                <button v-for="source in captureSourceCards" :key="source.label" type="button" class="flow-capture-source-row">
                  <span :class="{ 'is-on': source.state !== '暂停' }"></span>
                  <strong>{{ source.label }}</strong>
                  <small>今日 {{ source.count }} · {{ source.state }}</small>
                </button>
              </section>
              <section class="side-panel memory-note-panel">
                <div class="side-title">
                  <Plus :size="15" />
                  手动补记
                </div>
                <AriInput v-model="memory.noteDraft.title" class="memory-note-input" spellcheck="false" placeholder="标题" />
                <AriInput v-model="memory.noteDraft.text" class="memory-note-textarea" multiline spellcheck="false" placeholder="记录问题、结论、待办或留痕..." />
                <AriInput v-model="memory.noteDraft.tags" class="memory-note-input" spellcheck="false" placeholder="标签，用空格或逗号分隔" />
                <div class="memory-check-row">
                  <label><input v-model="memory.noteDraft.favorite" type="checkbox" /> 收藏</label>
                  <label><input v-model="memory.noteDraft.sensitive" type="checkbox" /> 敏感</label>
                </div>
                <AriButton size="sm" variant="primary" @click="memory.addNote()">
                  <Plus :size="14" />
                  加入心流
                </AriButton>
              </section>

              <section class="side-panel memory-data-panel">
                <div class="side-title">
                  <Upload :size="15" />
                  导入材料
                </div>
                <AriInput v-model="memory.importDraft.paths" class="memory-import-textarea" multiline spellcheck="false" placeholder="粘贴文件路径，一行一个" />
                <AriInput v-model="memory.importDraft.tags" class="memory-note-input" spellcheck="false" placeholder="导入标签" />
                <div class="memory-side-actions">
                  <AriButton size="sm" variant="primary" :disabled="memory.isImportingMaterials" @click="memory.importMaterials()">
                    <Upload :size="14" />
                    {{ memory.isImportingMaterials ? '导入中' : '导入材料' }}
                  </AriButton>
                  <AriButton size="sm" variant="ghost" @click="memory.clearUnpinned()">
                    <Trash2 :size="14" />
                    {{ memory.clearUnpinnedArmed ? '确认清理' : '清理未收藏' }}
                  </AriButton>
                </div>
                <small v-if="memory.importResult">
                  导入 {{ memory.importResult.imported }} 条，跳过 {{ memory.importResult.skipped }} 条，失败 {{ memory.importResult.failed }} 条
                </small>
              </section>
            </aside>

            <div class="flow-pipeline-main">
              <section class="flow-pipeline-board flow-pipeline-visual" aria-label="采集流水线">
                <article v-for="stage in rulesPipelineStatus" :key="stage.key" class="flow-pipeline-stage" :class="`is-${stage.status}`">
                  <span>{{ stage.label }}</span>
                  <strong>{{ stage.state }}</strong>
                  <p>{{ stage.note }}</p>
                </article>
              </section>
              <section class="flow-rules-table-card">
                <div class="side-title">
                  <Shield :size="15" />
                  排除规则表
                </div>
                <div class="flow-rules-tabs" aria-label="排除规则分类">
                  <button v-for="tab in exclusionRuleTabs" :key="tab.key" type="button">
                    <span>{{ tab.label }}</span>
                    <strong>{{ tab.count }}</strong>
                  </button>
                </div>
                <table class="flow-rules-table">
                  <thead>
                    <tr>
                      <th>规则名</th>
                      <th>匹配条件</th>
                      <th>命中</th>
                      <th>动作</th>
                      <th>优先级</th>
                      <th>状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in exclusionRuleRows" :key="`${row.group}-${row.value}`">
                      <td>{{ row.group }}</td>
                      <td>{{ row.value }}</td>
                      <td>{{ row.hits }}</td>
                      <td>{{ row.action }}</td>
                      <td>{{ row.priority }}</td>
                      <td><span class="flow-table-switch">启用</span></td>
                    </tr>
                  </tbody>
                </table>
              </section>
            </div>

            <aside class="flow-pipeline-inspector flow-agent-inspector">
              <header class="flow-agent-inspector-head">
                <div>
                  <span>Rule Inspector</span>
                  <strong>影响预览</strong>
                </div>
                <small>本地生效</small>
              </header>
              <section class="side-panel flow-impact-preview">
                <div v-for="stat in rulesImpactStats" :key="stat.label">
                  <strong>{{ stat.value }}</strong>
                  <span>{{ stat.label }}</span>
                  <small>{{ stat.note }}</small>
                </div>
              </section>
              <section class="side-panel semantic-panel">
                <div class="side-title">
                  <Database :size="15" />
                  语义索引
                </div>
                <strong>{{ vectorStatusLabel }}</strong>
                <p>{{ memory.semanticStatus?.note || '本地关键词和 FTS 可用；外部 embedding 需要显式刷新。' }}</p>
                <div class="semantic-meta-grid">
                  <span><small>Provider</small><strong>{{ vectorProviderLabel }}</strong></span>
                  <span><small>Store</small><strong>{{ vectorStoreLabel }}</strong></span>
                  <span><small>刷新</small><strong>{{ memory.semanticStatus?.lastEmbeddingAt ? formatTime(memory.semanticStatus.lastEmbeddingAt) : '未刷新' }}</strong></span>
                  <span><small>Collection</small><strong>{{ memory.semanticStatus?.vectorCollection || 'ariadne_work_memory' }}</strong></span>
                </div>
                <div class="search-row semantic-search-row">
                  <Search :size="15" class="text-[var(--muted)]" />
                  <input v-model="memory.semanticDraft.query" class="search-input" spellcheck="false" placeholder="语义搜索非敏感心流记忆..." @keydown.enter="memory.runSemanticSearch()" />
                </div>
                <div class="memory-side-actions">
                  <AriButton size="sm" variant="secondary" :disabled="memory.isRefreshingEmbedding" @click="memory.refreshEmbedding()">
                    <RefreshCw :size="14" />
                    {{ memory.isRefreshingEmbedding ? '刷新中' : '刷新索引' }}
                  </AriButton>
                  <AriButton size="sm" variant="primary" :disabled="memory.isSemanticSearching" @click="memory.runSemanticSearch()">
                    <Search :size="14" />
                    {{ memory.isSemanticSearching ? '检索中' : '语义搜索' }}
                  </AriButton>
                </div>
              </section>

              <section class="side-panel memory-rules-panel">
                <div class="side-title">
                  <Shield :size="15" />
                  规则 gates
                </div>
                <p>优先于采集、OCR、导入、导出和经验发现。</p>
                <div class="memory-rule-summary">{{ memory.exclusionSummary }}</div>
                <div class="memory-rule-grid">
                  <AriField class="memory-rule-field" label="应用进程">
                    <AriInput v-model="memory.exclusionDraft.apps" class="memory-rule-textarea" multiline spellcheck="false" placeholder="Code.exe&#10;chrome.exe" />
                  </AriField>
                  <AriField class="memory-rule-field" label="窗口关键词">
                    <AriInput v-model="memory.exclusionDraft.windowKeywords" class="memory-rule-textarea" multiline spellcheck="false" placeholder="密码&#10;隐私" />
                  </AriField>
                  <AriField class="memory-rule-field" label="路径片段">
                    <AriInput v-model="memory.exclusionDraft.paths" class="memory-rule-textarea" multiline spellcheck="false" placeholder="secrets&#10;.env" />
                  </AriField>
                  <AriField class="memory-rule-field" label="内容正则">
                    <AriInput v-model="memory.exclusionDraft.contentPatterns" class="memory-rule-textarea" multiline spellcheck="false" placeholder="token=&#10;classified" />
                  </AriField>
                </div>
                <AriButton size="sm" variant="secondary" :disabled="memory.isSavingExclusions" @click="memory.saveExclusionRules()">
                  <Shield :size="14" />
                  {{ memory.isSavingExclusions ? '保存中' : '保存排除规则' }}
                </AriButton>
              </section>
              <section class="side-panel flow-local-boundary-panel">
                <div class="side-title">
                  <KeyRound :size="15" />
                  敏感凭据模式
                </div>
                <strong>已启用 · 高敏感</strong>
                <p>所有规则均在本地生效，不上传云端；外部 AI 只接收确认后的非敏感摘要。</p>
                <div class="flow-app-list">
                  <span v-for="[app, count] in topApps.slice(0, 5)" :key="app">
                    <strong>{{ displayAppName(app) }}</strong>
                    <small>{{ count }} 条</small>
                  </span>
                </div>
              </section>
            </aside>
          </div>
        </section>
</template>
