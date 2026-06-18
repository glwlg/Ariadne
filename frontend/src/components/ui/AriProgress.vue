<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '../../lib/utils'

const props = withDefaults(
  defineProps<{
    value: number
    label?: string
    detail?: string
    tone?: 'primary' | 'danger'
  }>(),
  {
    label: '',
    detail: '',
    tone: 'primary',
  },
)

const normalizedValue = computed(() => Math.min(100, Math.max(0, Number.isFinite(props.value) ? props.value : 0)))
const trackClasses = computed(() =>
  cn('flow-progress-track', props.tone === 'danger' && 'is-danger'),
)
</script>

<template>
  <div class="ari-progress" role="status" aria-live="polite">
    <div v-if="label || detail" class="ari-progress-head">
      <strong v-if="label">{{ label }}</strong>
      <small v-if="detail">{{ detail }}</small>
    </div>
    <slot />
    <div :class="trackClasses">
      <span :style="{ width: `${normalizedValue}%` }" />
    </div>
  </div>
</template>

<style scoped>
.ari-progress {
  display: grid;
  min-width: 0;
  gap: 7px;
}

.ari-progress-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}

.ari-progress-head strong {
  min-width: 0;
  color: var(--foreground);
  font-size: 13px;
}

.ari-progress-head small {
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.flow-progress-track.is-danger span {
  background: linear-gradient(90deg, #ef4444, #f97316);
}
</style>
