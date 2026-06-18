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
    props.tone === 'success' && 'border-emerald-200 bg-emerald-50 text-emerald-900',
    props.tone === 'warning' && 'border-amber-200 bg-amber-50 text-amber-900',
    props.tone === 'danger' && 'border-red-200 bg-red-50 text-red-900',
  ),
)
</script>

<template>
  <span :class="classes" v-bind="$attrs">
    <slot />
  </span>
</template>
