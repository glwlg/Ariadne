<script setup lang="ts">
import { computed } from 'vue'
import { useSettingsStore } from '../../stores/settings'
import AriButton from '../ui/AriButton.vue'

const settings = useSettingsStore()
const credential = computed(() => settings.secretStatus?.records.find((record) => record.kind === 'ai_api_key'))
</script>

<template>
  <template v-if="settings.settings">
    <section class="settings-panel">
      <div class="settings-panel-title">OCR 文字识别</div>
      <p class="settings-note">截图、贴图和剪贴板图片共用此配置。</p>
      <label class="settings-toggle">
        <input v-model="settings.settings.ai.ocrModelEnabled" type="checkbox" />
        <span />
        <strong>使用模型识别文字</strong>
        <small>关闭后使用本地 OCR。自动打码使用本地识别坐标。</small>
      </label>
      <div class="settings-hotkey-grid">
        <label class="settings-field">
          <span>接口类型</span>
          <input v-model="settings.settings.ai.ocrProvider" class="settings-input" placeholder="openai-compatible / ollama-generate" />
        </label>
        <label class="settings-field">
          <span>接口地址</span>
          <input v-model="settings.settings.ai.ocrBaseUrl" class="settings-input" />
        </label>
        <label class="settings-field">
          <span>模型名称</span>
          <input v-model="settings.settings.ai.ocrModel" class="settings-input" />
        </label>
      </div>
    </section>

    <section class="settings-panel">
      <div class="settings-panel-title">OCR 接口密钥</div>
      <p class="settings-note">{{ credential?.stored ? '已保存密钥' : credential?.envPresent ? '使用环境变量中的密钥' : '尚未配置密钥' }}</p>
      <label class="settings-field">
        <span>API Key</span>
        <input v-model="settings.secretInputs.ai_api_key" class="settings-input" type="password" autocomplete="off" />
      </label>
      <div class="launcher-actions">
        <AriButton variant="primary" size="sm" :disabled="!settings.secretStatus?.available || !settings.secretInputs.ai_api_key?.trim() || settings.isSaving" @click="settings.saveSecret('ai_api_key')">保存密钥</AriButton>
        <AriButton variant="secondary" size="sm" :disabled="!credential?.stored || settings.isSaving" @click="settings.clearSecret('ai_api_key')">清除密钥</AriButton>
      </div>
    </section>
  </template>
</template>
