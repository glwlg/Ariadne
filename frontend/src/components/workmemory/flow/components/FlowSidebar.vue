<script setup lang="ts">
import { ArrowLeft, Settings } from '@lucide/vue'
import { toRefs } from 'vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  activeFlowPage,
  appShell,
  flowPages,
  openFlowPage,
  openFlowSettings,
} = toRefs(ctx)
</script>

<template>
  <aside class="flow-sidebar" aria-label="心流导航">
    <div class="flow-sidebar-brand">
      <div class="flow-logo-mark" aria-hidden="true">
        <img src="/favicon.svg" alt="" />
      </div>
      <div>
        <small>Ariadne</small>
        <strong>心流</strong>
        <span>本地优先 · 自动整理</span>
      </div>
    </div>

    <nav class="flow-side-nav">
      <button
        v-for="page in flowPages"
        :key="page.id"
        type="button"
        class="flow-side-nav-item"
        :class="{ 'is-active': activeFlowPage === page.id }"
        @click="openFlowPage(page.id)"
      >
        <component :is="page.icon" :size="22" />
        <span>{{ page.label }}</span>
      </button>
    </nav>

    <div class="flow-sidebar-footer">
      <div class="flow-user-badge">
        <span>LW</span>
        <div>
          <strong>luwei</strong>
          <small>本地模式</small>
        </div>
      </div>
      <button type="button" class="flow-side-nav-item" @click="openFlowSettings()">
        <Settings :size="22" />
        <span>设置</span>
      </button>
      <button type="button" class="flow-side-nav-item" @click="appShell.openLauncher()">
        <ArrowLeft :size="22" />
        <span>收起</span>
      </button>
    </div>
  </aside>
</template>
