<script setup lang="ts">
import { defineAsyncComponent, computed, toRefs } from 'vue'
import { useWorkMemoryFlowContext } from './context'

const FlowHomePage = defineAsyncComponent(() => import('./pages/FlowHomePage.vue'))
const FlowTimelinePage = defineAsyncComponent(() => import('./pages/FlowTimelinePage.vue'))
const FlowInsightsPage = defineAsyncComponent(() => import('./pages/FlowInsightsPage.vue'))
const FlowDraftsPage = defineAsyncComponent(() => import('./pages/FlowDraftsPage.vue'))
const FlowAssetsPage = defineAsyncComponent(() => import('./pages/FlowAssetsPage.vue'))
const FlowTodosPage = defineAsyncComponent(() => import('./pages/FlowTodosPage.vue'))
const FlowMePage = defineAsyncComponent(() => import('./pages/FlowMePage.vue'))
const FlowRulesPage = defineAsyncComponent(() => import('./pages/FlowRulesPage.vue'))

const routeComponents = {
  flow: FlowHomePage,
  timeline: FlowTimelinePage,
  insights: FlowInsightsPage,
  drafts: FlowDraftsPage,
  assets: FlowAssetsPage,
  todos: FlowTodosPage,
  me: FlowMePage,
  rules: FlowRulesPage,
}

const ctx = useWorkMemoryFlowContext()
const { activeFlowPage } = toRefs(ctx)
const currentRouteComponent = computed(() => routeComponents[activeFlowPage.value as keyof typeof routeComponents] ?? FlowHomePage)
</script>

<template>
  <component :is="currentRouteComponent" />
</template>
