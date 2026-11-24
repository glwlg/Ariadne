<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    tone?: 'neutral' | 'primary' | 'success' | 'warning' | 'danger'
    size?: 'sm' | 'md'
  }>(),
  {
    tone: 'neutral',
    size: 'sm',
  },
)

const classes = computed(() =>
  cn(
    'inline-flex min-w-0 items-center justify-center gap-1 rounded-full border font-medium leading-none',
    props.size === 'sm' && 'h-6 px-2 text-xs',
    props.size === 'md' && 'h-7 px-2.5 text-sm',
    props.tone === 'neutral' && 'border-[var(--border)] bg-[var(--surface-muted)] text-[var(--muted-foreground)]',
    props.tone === 'primary' && 'border-[var(--primary)] bg-[var(--primary-soft)] text-[var(--primary)]',
    props.tone === 'success' && 'border-[var(--success-border)] bg-[var(--success-soft)] text-[var(--success-foreground)]',
    props.tone === 'warning' && 'border-[var(--warning-border)] bg-[var(--warning-soft)] text-[var(--warning-foreground)]',
    props.tone === 'danger' && 'border-[var(--danger-border)] bg-[var(--danger-soft)] text-[var(--danger-foreground)]',
  ),
)
</script>

<template>
  <span :class="classes" v-bind="$attrs">
    <slot />
  </span>
</template>
