<script setup lang="ts">
import AriCard from '../../../ui/AriCard.vue'

defineProps<{
  eyebrow: string
  title: string
}>()
</script>

<template>
  <AriCard class="flow-page-header" variant="default">
    <div>
      <span>{{ eyebrow }}</span>
      <h1>{{ title }}</h1>
    </div>
    <slot />
  </AriCard>
</template>
