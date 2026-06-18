<script setup lang="ts">
defineProps<{
  label: string
  hint?: string
}>()
</script>

<template>
  <label class="ari-field">
    <span>{{ label }}</span>
    <slot />
    <small v-if="hint">{{ hint }}</small>
  </label>
</template>

<style scoped>
.ari-field {
  display: grid;
  min-width: 0;
  gap: 7px;
  color: var(--muted-foreground);
  font-size: 12px;
}

.ari-field > span {
  color: var(--muted-foreground);
  font-weight: 700;
}

.ari-field > small {
  color: var(--muted);
  line-height: 1.5;
}
</style>
