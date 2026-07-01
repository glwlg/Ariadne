<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ChevronDown, ChevronRight } from '@lucide/vue'

defineOptions({
  name: 'ApiJsonViewer',
})

const props = withDefaults(
  defineProps<{
    value: unknown
    name?: string
    root?: boolean
    depth?: number
    comma?: boolean
  }>(),
  {
    name: '',
    root: false,
    depth: 0,
    comma: false,
  },
)

const expanded = ref(defaultExpanded())

const isArray = computed(() => Array.isArray(props.value))
const isObject = computed(() => isPlainObject(props.value))
const isContainer = computed(() => isArray.value || isObject.value)
const entries = computed(() => {
  if (Array.isArray(props.value)) {
    return props.value.map((value, index, source) => ({
      key: String(index),
      label: `[${index}]`,
      value,
      comma: index < source.length - 1,
    }))
  }
  if (isPlainObject(props.value)) {
    const source = Object.entries(props.value)
    return source.map(([key, value], index) => ({
      key,
      label: JSON.stringify(key),
      value,
      comma: index < source.length - 1,
    }))
  }
  return []
})
const openToken = computed(() => (isArray.value ? '[' : '{'))
const closeToken = computed(() => (isArray.value ? ']' : '}'))
const summary = computed(() => {
  const count = entries.value.length
  if (isArray.value) return count === 0 ? '空数组' : `${count} 项`
  return count === 0 ? '空对象' : `${count} 键`
})
const primitiveClass = computed(() => {
  if (props.value === null) return 'is-null'
  if (typeof props.value === 'string') return 'is-string'
  if (typeof props.value === 'number') return 'is-number'
  if (typeof props.value === 'boolean') return 'is-boolean'
  return 'is-unknown'
})
const primitiveText = computed(() => {
  if (typeof props.value === 'string') return JSON.stringify(props.value)
  if (props.value === undefined) return 'undefined'
  return String(props.value)
})

watch(
  () => props.value,
  () => {
    expanded.value = defaultExpanded()
  },
)

function defaultExpanded() {
  return props.depth < 2
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}
</script>

<template>
  <div class="api-json-node" :class="{ 'is-root': root }" :style="{ '--api-json-depth': depth }">
    <div class="api-json-line" :class="{ 'is-collapsible': isContainer }">
      <button v-if="isContainer" type="button" class="api-json-toggle" :aria-label="expanded ? '收起节点' : '展开节点'" @click="expanded = !expanded">
        <ChevronDown v-if="expanded" :size="14" />
        <ChevronRight v-else :size="14" />
      </button>
      <span v-else class="api-json-spacer" />

      <span v-if="!root && name" class="api-json-key">{{ name }}</span>
      <span v-if="!root && name" class="api-json-colon">:</span>

      <template v-if="isContainer">
        <span class="api-json-token">{{ openToken }}</span>
        <span v-if="!expanded" class="api-json-summary">{{ summary }}</span>
        <span v-if="!expanded" class="api-json-token">{{ closeToken }}</span>
        <span v-if="comma && !expanded" class="api-json-comma">,</span>
      </template>
      <template v-else>
        <span class="api-json-value" :class="primitiveClass">{{ primitiveText }}</span>
        <span v-if="comma" class="api-json-comma">,</span>
      </template>
    </div>

    <template v-if="isContainer && expanded">
      <ApiJsonViewer
        v-for="entry in entries"
        :key="entry.key"
        :name="entry.label"
        :value="entry.value"
        :depth="depth + 1"
        :comma="entry.comma"
      />
      <div v-if="entries.length === 0" class="api-json-line api-json-empty">
        <span class="api-json-spacer" />
        <span>{{ summary }}</span>
      </div>
      <div class="api-json-line api-json-closing">
        <span class="api-json-spacer" />
        <span class="api-json-token">{{ closeToken }}</span>
        <span v-if="comma" class="api-json-comma">,</span>
      </div>
    </template>
  </div>
</template>
