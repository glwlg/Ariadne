<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { X } from '@lucide/vue'
import AriButton from '../ui/AriButton.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    mode: 'input' | 'confirm'
    title: string
    message?: string
    label?: string
    value?: string
    placeholder?: string
    confirmLabel?: string
    cancelLabel?: string
    danger?: boolean
    allowEmpty?: boolean
  }>(),
  {
    message: '',
    label: '',
    value: '',
    placeholder: '',
    confirmLabel: '确定',
    cancelLabel: '取消',
    danger: false,
    allowEmpty: false,
  },
)

const emit = defineEmits<{
  close: []
  confirm: [value: string]
}>()

const localValue = ref('')
const canConfirm = computed(() => props.mode === 'confirm' || props.allowEmpty || localValue.value.trim() !== '')

watch(
  () => props.open,
  (open) => {
    if (open) localValue.value = props.value
  },
)

function confirm() {
  if (!canConfirm.value) return
  emit('confirm', localValue.value)
}
</script>

<template>
  <div v-if="open" class="api-modal-layer" role="dialog" aria-modal="true" :aria-label="title">
    <div class="api-modal-backdrop" @click="emit('close')" />
    <section class="api-action-dialog">
      <header>
        <strong>{{ title }}</strong>
        <button type="button" aria-label="关闭" @click="emit('close')">
          <X :size="16" />
        </button>
      </header>

      <div class="api-action-dialog-body">
        <p v-if="message" class="api-dialog-message">{{ message }}</p>
        <label v-if="mode === 'input'" class="api-field">
          <span>{{ label }}</span>
          <input v-model="localValue" class="api-input" :placeholder="placeholder" autofocus @keydown.enter="confirm" />
        </label>
      </div>

      <footer>
        <AriButton size="sm" variant="ghost" @click="emit('close')">{{ cancelLabel }}</AriButton>
        <AriButton size="sm" :variant="danger ? 'secondary' : 'primary'" :class="{ 'api-dialog-danger-button': danger }" :disabled="!canConfirm" @click="confirm">
          {{ confirmLabel }}
        </AriButton>
      </footer>
    </section>
  </div>
</template>
