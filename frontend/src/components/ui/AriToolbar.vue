<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    dense?: boolean
  }>(),
  {
    dense: false,
  },
)

const classes = computed(() =>
  cn(
    'flex min-w-0 flex-wrap items-center gap-2 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)]',
    props.dense ? 'p-2' : 'p-3',
  ),
)
</script>

<template>
  <div :class="classes" v-bind="$attrs">
    <slot />
  </div>
</template>
