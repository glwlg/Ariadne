<script setup lang="ts">
import { computed, ref } from 'vue'
import { X } from '@lucide/vue'
import { useAPITestingStore } from '../../stores/apiTesting'
import ApiContextMenu from './ApiContextMenu.vue'

const apiTesting = useAPITestingStore()

const menu = ref({
  open: false,
  x: 0,
  y: 0,
  requestId: '',
})

const menuItems = computed(() => [
  { id: 'close-tab', label: '关闭' },
  { id: 'close-others', label: '关闭其他', disabled: apiTesting.openRequestIds.length <= 1 },
  { id: 'close-right', label: '关闭右侧', disabled: apiTesting.openRequestIds.indexOf(menu.value.requestId) === apiTesting.openRequestIds.length - 1 },
  { id: 'close-all', label: '关闭全部', disabled: apiTesting.openRequestIds.length === 0 },
])

function openMenu(event: MouseEvent, requestId: string) {
  event.preventDefault()
  event.stopPropagation()
  menu.value = {
    open: true,
    x: Math.min(event.clientX, window.innerWidth - 210),
    y: Math.min(event.clientY, window.innerHeight - 180),
    requestId,
  }
}

function closeMenu() {
  menu.value.open = false
}

function selectMenuAction(action: string) {
  const requestId = menu.value.requestId
  closeMenu()
  if (action === 'close-tab') apiTesting.closeRequestTab(requestId)
  if (action === 'close-others') apiTesting.closeOtherRequestTabs(requestId)
  if (action === 'close-right') apiTesting.closeTabsToRight(requestId)
  if (action === 'close-all') apiTesting.closeAllRequestTabs()
}

function onAuxClick(event: MouseEvent, requestId: string) {
  if (event.button === 1) {
    event.preventDefault()
    apiTesting.closeRequestTab(requestId)
  }
}
</script>

<template>
  <nav class="api-request-tabs" aria-label="请求标签">
    <div
      v-for="request in apiTesting.openRequests"
      :key="request.id"
      class="api-request-tab"
      :class="{ 'is-active': request.id === apiTesting.selectedRequestId }"
      role="tab"
      @contextmenu="openMenu($event, request.id)"
      @auxclick="onAuxClick($event, request.id)"
    >
      <button type="button" class="api-request-tab-main" @click="apiTesting.selectRequest(request.id)">
        <span class="api-method-text" :class="`is-${request.method.toLowerCase()}`">{{ request.method }}</span>
        <span>{{ request.name }}</span>
      </button>
      <button type="button" class="api-request-tab-close" aria-label="关闭标签" @click.stop="apiTesting.closeRequestTab(request.id)">
        <X :size="13" />
      </button>
    </div>
    <div v-if="!apiTesting.openRequests.length" class="api-request-tabs-empty">未打开请求</div>
    <ApiContextMenu :open="menu.open" :x="menu.x" :y="menu.y" :items="menuItems" @close="closeMenu" @select="selectMenuAction" />
  </nav>
</template>
