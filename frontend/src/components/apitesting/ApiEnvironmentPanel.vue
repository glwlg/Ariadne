<script setup lang="ts">
import { Copy, Globe2, Plus, Save, Trash2, X } from '@lucide/vue'
import AriButton from '../ui/AriButton.vue'
import { useAPITestingStore } from '../../stores/apiTesting'

const apiTesting = useAPITestingStore()
</script>

<template>
  <div v-if="apiTesting.isEnvironmentPanelOpen" class="api-layer" role="dialog" aria-modal="true" aria-label="环境配置">
    <div class="api-layer-backdrop" @click="apiTesting.closeEnvironmentPanel()" />
    <section class="api-env-panel">
      <header class="api-env-panel-head">
        <div>
          <strong>环境配置</strong>
          <span v-if="apiTesting.saveFeedback" aria-live="polite">{{ apiTesting.saveFeedback }}</span>
          <span v-else-if="apiTesting.isDirty">未保存</span>
        </div>
        <div class="api-env-panel-actions">
          <AriButton size="sm" variant="secondary" :disabled="!apiTesting.draftCollection || apiTesting.isSaving" @click="apiTesting.saveCollection()">
            <Save :size="14" />
            保存
          </AriButton>
          <AriButton size="icon" variant="ghost" aria-label="关闭" @click="apiTesting.closeEnvironmentPanel()">
            <X :size="16" />
          </AriButton>
        </div>
      </header>

      <div class="api-env-panel-body">
        <aside class="api-env-rail" aria-label="环境列表">
          <div class="api-env-rail-title">
            <span>环境</span>
            <AriButton size="icon" variant="ghost" aria-label="新环境" @click="apiTesting.createEnvironment()">
              <Plus :size="15" />
            </AriButton>
          </div>
          <button
            v-for="environment in apiTesting.draftCollection?.environments ?? []"
            :key="environment.id"
            class="api-env-row"
            :class="{ 'is-active': environment.id === apiTesting.selectedEnvironmentId }"
            @click="apiTesting.selectEnvironment(environment.id)"
          >
            <Globe2 :size="15" />
            <span>{{ environment.name }}</span>
            <small>{{ environment.variables.filter((variable) => variable.enabled).length }}</small>
          </button>
          <div class="api-env-rail-actions">
            <AriButton size="sm" variant="secondary" :disabled="!apiTesting.selectedEnvironment" @click="apiTesting.duplicateEnvironment()">
              <Copy :size="14" />
              复制
            </AriButton>
            <AriButton
              size="sm"
              variant="ghost"
              :disabled="(apiTesting.draftCollection?.environments.length ?? 0) <= 1"
              @click="apiTesting.removeSelectedEnvironment()"
            >
              <Trash2 :size="14" />
              删除
            </AriButton>
          </div>
        </aside>

        <div class="api-env-detail">
          <section class="api-env-section" aria-label="当前环境">
            <div class="api-env-name-row">
              <label class="api-field">
                <span>环境名称</span>
                <input
                  class="api-input"
                  :value="apiTesting.selectedEnvironment?.name || ''"
                  placeholder="环境名称"
                  @input="apiTesting.updateEnvironment({ name: ($event.target as HTMLInputElement).value })"
                />
              </label>
              <AriButton size="sm" variant="secondary" @click="apiTesting.addEnvironmentVariable()">
                <Plus :size="14" />
                添加变量
              </AriButton>
            </div>
            <div class="api-table">
              <div class="api-table-head is-env-var-grid">
                <span>启用</span>
                <span>密钥</span>
                <span>名称</span>
                <span>值</span>
                <span></span>
              </div>
              <div v-for="(variable, index) in apiTesting.selectedEnvironment?.variables ?? []" :key="variable.id" class="api-table-row is-env-var-grid">
                <label class="api-checkbox">
                  <input type="checkbox" :checked="variable.enabled" @change="apiTesting.updateEnvironmentVariable(index, { enabled: ($event.target as HTMLInputElement).checked })" />
                </label>
                <label class="api-checkbox">
                  <input type="checkbox" :checked="Boolean(variable.secret)" @change="apiTesting.updateEnvironmentVariable(index, { secret: ($event.target as HTMLInputElement).checked })" />
                </label>
                <input class="api-input" :value="variable.name" placeholder="token" @input="apiTesting.updateEnvironmentVariable(index, { name: ($event.target as HTMLInputElement).value })" />
                <input
                  class="api-input"
                  :type="variable.secret ? 'password' : 'text'"
                  :value="variable.value"
                  placeholder="变量值"
                  @input="apiTesting.updateEnvironmentVariable(index, { value: ($event.target as HTMLInputElement).value })"
                />
                <AriButton size="icon" variant="ghost" aria-label="删除变量" @click="apiTesting.removeEnvironmentVariable(index)">
                  <Trash2 :size="14" />
                </AriButton>
              </div>
              <div v-if="!(apiTesting.selectedEnvironment?.variables.length ?? 0)" class="api-empty-row">暂无环境变量</div>
            </div>
          </section>

          <section class="api-env-section" aria-label="集合变量">
            <div class="api-env-name-row">
              <div class="api-section-title">
                <span>集合变量</span>
              </div>
              <AriButton size="sm" variant="secondary" @click="apiTesting.addCollectionVariable()">
                <Plus :size="14" />
                添加变量
              </AriButton>
            </div>
            <div class="api-table">
              <div class="api-table-head is-env-var-grid">
                <span>启用</span>
                <span>密钥</span>
                <span>名称</span>
                <span>值</span>
                <span></span>
              </div>
              <div v-for="(variable, index) in apiTesting.draftCollection?.variables ?? []" :key="variable.id" class="api-table-row is-env-var-grid">
                <label class="api-checkbox">
                  <input type="checkbox" :checked="variable.enabled" @change="apiTesting.updateCollectionVariable(index, { enabled: ($event.target as HTMLInputElement).checked })" />
                </label>
                <label class="api-checkbox">
                  <input type="checkbox" :checked="Boolean(variable.secret)" @change="apiTesting.updateCollectionVariable(index, { secret: ($event.target as HTMLInputElement).checked })" />
                </label>
                <input class="api-input" :value="variable.name" placeholder="baseUrl" @input="apiTesting.updateCollectionVariable(index, { name: ($event.target as HTMLInputElement).value })" />
                <input
                  class="api-input"
                  :type="variable.secret ? 'password' : 'text'"
                  :value="variable.value"
                  placeholder="https://api.example.com"
                  @input="apiTesting.updateCollectionVariable(index, { value: ($event.target as HTMLInputElement).value })"
                />
                <AriButton size="icon" variant="ghost" aria-label="删除变量" @click="apiTesting.removeCollectionVariable(index)">
                  <Trash2 :size="14" />
                </AriButton>
              </div>
              <div v-if="!(apiTesting.draftCollection?.variables.length ?? 0)" class="api-empty-row">暂无集合变量</div>
            </div>
          </section>
        </div>
      </div>
    </section>
  </div>
</template>
