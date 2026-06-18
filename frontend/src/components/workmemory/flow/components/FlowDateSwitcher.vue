<script setup lang="ts">
import { ArrowLeft, ArrowRight } from '@lucide/vue'
import AriButton from '../../../ui/AriButton.vue'

defineProps<{
  label: string
}>()

const emit = defineEmits<{
  previous: []
  next: []
  reset: []
}>()
</script>

<template>
  <div class="flow-date-switcher">
    <AriButton size="icon" variant="ghost" aria-label="前一天" @click="emit('previous')">
      <ArrowLeft :size="14" />
    </AriButton>
    <strong title="回到今天" @dblclick="emit('reset')">{{ label }}</strong>
    <AriButton size="icon" variant="ghost" aria-label="后一天" @click="emit('next')">
      <ArrowRight :size="14" />
    </AriButton>
  </div>
</template>
