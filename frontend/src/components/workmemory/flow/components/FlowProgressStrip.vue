<script setup lang="ts">
import AriProgress from '../../../ui/AriProgress.vue'

withDefaults(
  defineProps<{
    label: string
    detail: string
    percent: number
    note?: string
    danger?: boolean
  }>(),
  {
    note: '',
    danger: false,
  },
)
</script>

<template>
  <div class="flow-progress-strip" :class="{ 'is-danger': danger }" role="status" aria-live="polite">
    <AriProgress :label="label" :detail="detail" :value="percent" :tone="danger ? 'danger' : 'primary'">
      <p v-if="note" class="flow-progress-note">{{ note }}</p>
    </AriProgress>
  </div>
</template>
