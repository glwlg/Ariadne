<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const model = defineModel<string | number>()
const props = withDefaults(
  defineProps<{
    multiline?: boolean
    size?: 'sm' | 'md'
  }>(),
  {
    multiline: false,
    size: 'md',
  },
)

const classes = computed(() =>
  cn(
    'w-full min-w-0 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface-raised)] text-[var(--foreground)] outline-none transition',
    'placeholder:text-[var(--muted)] focus:border-[var(--border-strong)] focus:ring-2 focus:ring-[var(--ring)] disabled:cursor-not-allowed disabled:opacity-50',
    props.size === 'sm' && 'px-2.5 py-1.5 text-xs',
    props.size === 'md' && 'px-3 py-2 text-sm',
    props.multiline && 'min-h-24 resize-vertical leading-6',
  ),
)
</script>

<template>
  <textarea v-if="multiline" v-model="model" :class="classes" v-bind="$attrs" />
  <input v-else v-model="model" :class="classes" v-bind="$attrs" />
</template>
