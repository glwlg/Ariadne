<script setup lang="ts">
import { toRefs } from 'vue'
import FlowCommandDock from './components/FlowCommandDock.vue'
import FlowDetailDrawer from './components/FlowDetailDrawer.vue'
import FlowSettingsDrawer from './components/FlowSettingsDrawer.vue'
import FlowSidebar from './components/FlowSidebar.vue'
import FlowRouteView from './FlowRouteView.vue'
import { useWorkMemoryFlowContext } from './context'

const ctx = useWorkMemoryFlowContext()
const { activeFlowPage, flowSidebarCollapsed } = toRefs(ctx)
</script>

<template>
  <div class="flow-app-shell" :class="{ 'is-sidebar-collapsed': flowSidebarCollapsed }">
    <FlowSidebar />

    <section class="flow-stage" :class="{ 'is-chat-home': activeFlowPage === 'flow' }">
      <FlowRouteView />
      <FlowSettingsDrawer />
      <FlowDetailDrawer />
      <FlowCommandDock />
    </section>
  </div>
</template>
