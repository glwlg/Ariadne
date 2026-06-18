<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    tone?: 'neutral' | 'info' | 'warning' | 'danger'
  }>(),
  {
    tone: 'neutral',
  },
)

const classes = computed(() =>
  cn(
    'rounded-[var(--radius-sm)] border px-3 py-2 text-sm leading-6',
    props.tone === 'neutral' && 'border-[var(--border)] bg-[var(--surface-muted)] text-[var(--muted-foreground)]',
    props.tone === 'info' && 'border-cyan-200 bg-cyan-50 text-cyan-950',
    props.tone === 'warning' && 'border-amber-200 bg-amber-50 text-amber-950',
    props.tone === 'danger' && 'border-red-200 bg-red-50 text-red-950',
  ),
)
</script>

<template>
  <p :class="classes" v-bind="$attrs">
    <slot />
  </p>
</template>
