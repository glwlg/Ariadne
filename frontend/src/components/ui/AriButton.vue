<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost'
    size?: 'sm' | 'md' | 'icon'
    active?: boolean
  }>(),
  {
    variant: 'secondary',
    size: 'md',
    active: false,
  },
)

const classes = computed(() =>
  cn(
    'inline-flex items-center justify-center gap-2 rounded-[var(--radius-sm)] border text-sm font-medium outline-none transition',
    'focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-0 disabled:pointer-events-none disabled:opacity-50',
    props.size === 'sm' && 'h-8 px-2.5',
    props.size === 'md' && 'h-9 px-3',
    props.size === 'icon' && 'size-9 p-0',
    props.variant === 'primary' &&
      'border-[var(--primary)] bg-[var(--primary)] text-[var(--primary-foreground)] hover:bg-[var(--primary-hover)]',
    props.variant === 'secondary' &&
      'border-[var(--border)] bg-[var(--surface-raised)] text-[var(--foreground)] hover:border-[var(--border-strong)] hover:bg-[var(--surface-hover)]',
    props.variant === 'ghost' &&
      'border-transparent bg-transparent text-[var(--muted)] hover:bg-[var(--surface-hover)] hover:text-[var(--foreground)]',
    props.active && 'border-[var(--primary)] text-[var(--primary)]',
  ),
)
</script>

<template>
  <button type="button" :class="classes" v-bind="$attrs">
    <slot />
  </button>
</template>
