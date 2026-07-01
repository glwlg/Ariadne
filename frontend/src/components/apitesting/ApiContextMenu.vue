<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

export interface APIContextMenuItem {
  id: string
  label: string
  disabled?: boolean
  danger?: boolean
}

defineProps<{
  open: boolean
  x: number
  y: number
  items: APIContextMenuItem[]
}>()

const emit = defineEmits<{
  close: []
  select: [id: string]
}>()

function onWindowClick() {
  emit('close')
}

function onWindowKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  window.addEventListener('click', onWindowClick)
  window.addEventListener('keydown', onWindowKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('click', onWindowClick)
  window.removeEventListener('keydown', onWindowKeydown)
})
</script>

<template>
  <div
    v-if="open"
    class="api-context-menu"
    :style="{ left: `${x}px`, top: `${y}px` }"
    role="menu"
    @click.stop
    @contextmenu.prevent
  >
    <button
      v-for="item in items"
      :key="item.id"
      type="button"
      :disabled="item.disabled"
      :class="{ 'is-danger': item.danger }"
      role="menuitem"
      @click="emit('select', item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>
