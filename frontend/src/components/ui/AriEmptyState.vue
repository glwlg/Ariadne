<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<{
    title: string
    description?: string
    compact?: boolean
  }>(),
  {
    description: '',
    compact: false,
  },
)

const classes = computed(() =>
  cn(
    'flex min-w-0 flex-col items-center justify-center rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] text-center text-[var(--muted-foreground)]',
    props.compact ? 'gap-1 p-4' : 'min-h-56 gap-2 p-8',
  ),
)
</script>

<template>
  <div :class="classes" v-bind="$attrs">
    <div class="text-[var(--muted)]">
      <slot name="icon" />
    </div>
    <strong class="text-base text-[var(--foreground)]">{{ title }}</strong>
    <p v-if="description" class="max-w-md text-sm leading-6">{{ description }}</p>
    <slot />
  </div>
</template>
