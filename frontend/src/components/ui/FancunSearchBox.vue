<script setup lang="ts">
import { Search } from '@lucide/vue'
import { computed } from 'vue'
import { useAttrs } from 'vue'
import { cn } from '../../lib/utils'

defineOptions({
  inheritAttrs: false,
})

const model = defineModel<string>({ default: '' })
const attrs = useAttrs()
const props = withDefaults(
  defineProps<{
    compact?: boolean
    shortcut?: string
  }>(),
  {
    compact: false,
    shortcut: '',
  },
)

const classes = computed(() =>
  cn(
    'inline-flex min-w-0 items-center gap-2 rounded-full border border-[var(--border)] bg-[var(--surface-raised)] text-[var(--foreground)] shadow-sm',
    props.compact ? 'h-9 px-3 text-xs' : 'h-11 px-4 text-sm',
  ),
)

const inputAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})
</script>

<template>
  <label :class="[classes, attrs.class]">
    <Search :size="compact ? 14 : 16" class="shrink-0 text-[var(--muted)]" />
    <input
      v-model="model"
      class="min-w-0 flex-1 border-0 bg-transparent outline-none placeholder:text-[var(--muted)]"
      spellcheck="false"
      v-bind="inputAttrs"
    />
    <kbd v-if="shortcut" class="rounded-md border border-[var(--border)] bg-[var(--surface-muted)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--muted)]">
      {{ shortcut }}
    </kbd>
  </label>
</template>
