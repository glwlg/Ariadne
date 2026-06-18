<script setup lang="ts">
import { toRefs } from 'vue'
import AriSearchBox from '../../../ui/AriSearchBox.vue'
import FlowDateSwitcher from './FlowDateSwitcher.vue'
import { useWorkMemoryFlowContext } from '../context'

const ctx = useWorkMemoryFlowContext()
const {
  FLOW_DAY_END_HOUR,
  FLOW_DAY_START_HOUR,
  adjustFlowTimeByKey,
  flowCurrentClock,
  flowDateLabel,
  flowTimeRulerNowPercent,
  flowTimeRulerTicks,
  globalFlowSearch,
  globalSearchPlaceholder,
  resetFlowDateToday,
  runGlobalFlowSearch,
  selectedFlowHour,
  setFlowTimeFromPointer,
  shiftFlowDate,
} = toRefs(ctx)
</script>

<template>
  <div class="flow-cognitive-topbar">
    <FlowDateSwitcher :label="flowDateLabel" @previous="shiftFlowDate(-1)" @next="shiftFlowDate(1)" @reset="resetFlowDateToday()" />
    <div
      class="flow-time-ruler"
      role="slider"
      tabindex="0"
      :aria-valuetext="flowCurrentClock"
      :aria-valuemin="FLOW_DAY_START_HOUR"
      :aria-valuemax="FLOW_DAY_END_HOUR"
      :aria-valuenow="Math.round(selectedFlowHour * 100) / 100"
      aria-label="选中时间"
      @pointerdown="setFlowTimeFromPointer"
      @keydown="adjustFlowTimeByKey"
    >
      <span v-for="tick in flowTimeRulerTicks" :key="tick.label" :style="{ left: `${tick.left}%` }">{{ tick.label }}</span>
      <i :style="{ left: `${flowTimeRulerNowPercent}%` }">
        <b>{{ flowCurrentClock }}</b>
      </i>
    </div>
    <AriSearchBox
      v-model="globalFlowSearch"
      class="flow-global-search"
      data-no-drag
      :placeholder="globalSearchPlaceholder"
      shortcut="Ctrl K"
      @keydown.enter.prevent="runGlobalFlowSearch()"
    />
  </div>
</template>
