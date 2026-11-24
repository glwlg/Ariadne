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
    props.tone === 'info' && 'border-[var(--info-border)] bg-[var(--info-soft)] text-[var(--info-foreground)]',
    props.tone === 'warning' && 'border-[var(--warning-border)] bg-[var(--warning-soft)] text-[var(--warning-foreground)]',
    props.tone === 'danger' && 'border-[var(--danger-border)] bg-[var(--danger-soft)] text-[var(--danger-foreground)]',
  ),
)
</script>

<template>
  <p :class="classes" v-bind="$attrs">
    <slot />
  </p>
</template>
