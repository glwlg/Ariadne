<script setup lang="ts">
import { useSettingsStore } from '../../stores/settings'
const settings = useSettingsStore()
</script>

<template>
  <section v-if="settings.settings" class="settings-panel">
    <div class="settings-panel-title">历史记录</div>
    <div class="settings-toggle-grid">
      <label class="settings-toggle">
        <input v-model="settings.settings.history.clipboardEnabled" type="checkbox" />
        <span />
        <strong>记录剪贴板历史</strong>
      </label>
      <label class="settings-toggle">
        <input v-model="settings.settings.history.privacyMode" type="checkbox" />
        <span />
        <strong>暂停剪贴板记录</strong>
      </label>
      <label class="settings-toggle">
        <input v-model="settings.settings.history.keepFavoritesForever" type="checkbox" />
        <span />
        <strong>保留收藏记录</strong>
      </label>
    </div>
    <div class="settings-hotkey-grid">
      <label class="settings-field">
        <span>历史保留天数</span>
        <input v-model.number="settings.settings.history.retentionDays" class="settings-input" type="number" min="1" max="3650" />
      </label>
      <label class="settings-field">
        <span>截图存储上限（MB）</span>
        <input v-model.number="settings.settings.history.maxStorageMb" class="settings-input" type="number" min="128" />
      </label>
    </div>
  </section>
</template>
