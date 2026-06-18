<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'muted' | 'panel'
    padded?: boolean
  }>(),
  {
    variant: 'default',
    padded: true,
  },
)

const classes = computed(() =>
  cn(
    'min-w-0 rounded-[var(--radius-md)] border text-[var(--foreground)]',
    props.padded && 'p-4',
    props.variant === 'default' && 'border-[var(--border)] bg-[var(--surface)] shadow-sm',
    props.variant === 'muted' && 'border-[var(--border)] bg-[var(--surface-muted)]',
    props.variant === 'panel' && 'border-[var(--border)] bg-[var(--surface-raised)] shadow-[var(--shadow-soft)]',
  ),
)
</script>

<template>
  <section :class="classes" v-bind="$attrs">
    <slot />
  </section>
</template>
