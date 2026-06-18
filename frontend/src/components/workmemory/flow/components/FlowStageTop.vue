<script setup lang="ts">
import { Clock3 } from '@lucide/vue'
import { toRefs } from 'vue'
import AriSearchBox from '../../../ui/AriSearchBox.vue'
import FlowStatusPill from './FlowStatusPill.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  globalFlowSearch,
  globalSearchPlaceholder,
  memory,
  runGlobalFlowSearch,
} = toRefs(ctx)
</script>

<template>
  <header class="flow-stage-top">
    <div class="flow-stage-top-left">
      <FlowStatusPill>自动整理中 · 自建模型</FlowStatusPill>
      <AriSearchBox
        v-model="globalFlowSearch"
        class="flow-global-search"
        data-no-drag
        :placeholder="globalSearchPlaceholder"
        shortcut="Ctrl K"
        @keydown.enter.prevent="runGlobalFlowSearch()"
        @keydown.ctrl.k.prevent
      />
      <button type="button" class="flow-top-action" @click="memory.toggleTimeMachine()">
        <Clock3 :size="14" />
        时间机器
      </button>
    </div>
  </header>
</template>
