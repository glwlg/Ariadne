<script setup lang="ts">
import { Box, ChevronDown, ChevronRight, Folder, MoreHorizontal } from '@lucide/vue'
import { computed, ref } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { useAPITestingStore } from '../../stores/apiTesting'
import type { APICollection, APIRequest } from '../../types/ariadne'
import ApiActionDialog from './ApiActionDialog.vue'
import ApiContextMenu from './ApiContextMenu.vue'
import ApiNewRequestDialog from './ApiNewRequestDialog.vue'

type MenuTarget =
  | { kind: 'global' }
  | { kind: 'collection'; collectionId: string }
  | { kind: 'folder'; collectionId: string; name: string }
  | { kind: 'request'; collectionId: string; id: string }

type MenuState = {
  open: boolean
  x: number
  y: number
  target: MenuTarget
}

type MenuItem = {
  id: string
  label: string
  disabled?: boolean
  danger?: boolean
}

type DialogAction = '' | 'rename-collection' | 'delete-collection' | 'rename-folder' | 'rename-request' | 'move-request' | 'delete-request'

type ActionDialogState = {
  open: boolean
  mode: 'input' | 'confirm'
  title: string
  message: string
  label: string
  value: string
  placeholder: string
  confirmLabel: string
  danger: boolean
  allowEmpty: boolean
  action: DialogAction
  target: MenuTarget | null
}

const apiTesting = useAPITestingStore()
const collapsedCollections = ref<Set<string>>(new Set())
const collapsedGroups = ref<Set<string>>(new Set())
const newRequestDialog = ref({ open: false, folder: '', collectionId: '' })
const actionDialog = ref<ActionDialogState>(emptyActionDialog())
const menu = ref<MenuState>({
  open: false,
  x: 0,
  y: 0,
  target: { kind: 'global' },
})

const activeCollection = computed(() => apiTesting.treeCollections.find((collection) => collection.id === apiTesting.selectedCollectionId) ?? null)

const menuItems = computed<MenuItem[]>(() => {
  const target = menu.value.target
  if (target.kind === 'request') {
    return [
      { id: 'run-request', label: '发送请求' },
      { id: 'duplicate-request', label: '复制请求' },
      { id: 'rename-request', label: '重命名' },
      { id: 'move-request', label: '移动到分组' },
      { id: 'delete-request', label: '删除请求', danger: true, disabled: requestCount(target.collectionId) <= 1 },
    ]
  }
  if (target.kind === 'folder') {
    return [
      { id: 'new-request', label: '新请求' },
      { id: 'rename-folder', label: '重命名分组', disabled: target.name === '未分组' },
      { id: 'import-requests', label: '导入请求', disabled: apiTesting.isImporting },
    ]
  }
  if (target.kind === 'collection') {
    return [
      { id: 'new-request', label: '新请求' },
      { id: 'new-folder', label: '新分组' },
      { id: 'import-requests', label: '导入请求', disabled: apiTesting.isImporting },
      { id: 'rename-collection', label: '重命名集合' },
      { id: 'save-collection', label: apiTesting.isSaving ? '保存中' : '保存集合', disabled: apiTesting.isSaving },
      { id: 'delete-collection', label: '删除集合', danger: true, disabled: apiTesting.collections.length <= 1 },
    ]
  }
  return [
    { id: 'new-request', label: '新请求', disabled: !activeCollection.value },
    { id: 'new-folder', label: '新分组', disabled: !activeCollection.value },
    { id: 'import-requests', label: '导入请求', disabled: !activeCollection.value || apiTesting.isImporting },
    { id: 'new-collection', label: '新集合' },
  ]
})

function requestGroups(collection: APICollection) {
  const map = new Map<string, APICollection['requests']>()
  for (const request of collection.requests ?? []) {
    const group = request.folder?.trim() || '未分组'
    map.set(group, [...(map.get(group) ?? []), request])
  }
  return Array.from(map.entries()).map(([name, requests]) => ({ name, requests }))
}

function requestCount(collectionId: string) {
  return apiTesting.treeCollections.find((collection) => collection.id === collectionId)?.requests.length ?? 0
}

function folderKey(collectionId: string, name: string) {
  return `${collectionId}:${name}`
}

function isCollectionExpanded(collectionId: string) {
  return collectionId === apiTesting.selectedCollectionId && !collapsedCollections.value.has(collectionId)
}

function toggleCollection(collectionId: string) {
  if (collectionId !== apiTesting.selectedCollectionId) {
    apiTesting.selectCollection(collectionId)
    const next = new Set(collapsedCollections.value)
    next.delete(collectionId)
    collapsedCollections.value = next
    return
  }
  const next = new Set(collapsedCollections.value)
  if (next.has(collectionId)) {
    next.delete(collectionId)
  } else {
    next.add(collectionId)
  }
  collapsedCollections.value = next
}

function toggleGroup(collectionId: string, name: string) {
  const key = folderKey(collectionId, name)
  const next = new Set(collapsedGroups.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  collapsedGroups.value = next
}

function isGroupCollapsed(collectionId: string, name: string) {
  return collapsedGroups.value.has(folderKey(collectionId, name))
}

function createFolderName(collection: APICollection) {
  const base = '新分组'
  const existing = new Set(requestGroups(collection).map((group) => group.name))
  let name = base
  let index = 2
  while (existing.has(name)) {
    name = `${base} ${index}`
    index += 1
  }
  return name
}

function openMenu(event: MouseEvent, target: MenuTarget) {
  event.preventDefault()
  event.stopPropagation()
  menu.value = {
    open: true,
    x: Math.min(event.clientX, window.innerWidth - 210),
    y: Math.min(event.clientY, window.innerHeight - 260),
    target,
  }
}

function closeMenu() {
  menu.value.open = false
}

function emptyActionDialog(): ActionDialogState {
  return {
    open: false,
    mode: 'input',
    title: '',
    message: '',
    label: '',
    value: '',
    placeholder: '',
    confirmLabel: '确定',
    danger: false,
    allowEmpty: false,
    action: '',
    target: null,
  }
}

function closeActionDialog() {
  actionDialog.value = emptyActionDialog()
}

function openInputDialog(action: DialogAction, target: MenuTarget, options: Partial<ActionDialogState>) {
  actionDialog.value = {
    ...emptyActionDialog(),
    mode: 'input',
    confirmLabel: '保存',
    ...options,
    open: true,
    action,
    target,
  }
}

function openConfirmDialog(action: DialogAction, target: MenuTarget, options: Partial<ActionDialogState>) {
  actionDialog.value = {
    ...emptyActionDialog(),
    mode: 'confirm',
    confirmLabel: '确定',
    ...options,
    open: true,
    action,
    target,
  }
}

function ensureCollection(collectionId: string) {
  if (collectionId && collectionId !== apiTesting.selectedCollectionId) {
    apiTesting.selectCollection(collectionId)
  }
}

function selectRequest(collectionId: string, requestId: string) {
  ensureCollection(collectionId)
  apiTesting.selectRequest(requestId)
}

function targetCollectionId(target: MenuTarget) {
  if (target.kind === 'collection' || target.kind === 'folder' || target.kind === 'request') return target.collectionId
  return apiTesting.selectedCollectionId
}

function openNewRequest(target: MenuTarget) {
  const collectionId = targetCollectionId(target)
  if (!collectionId) return
  ensureCollection(collectionId)
  newRequestDialog.value = {
    open: true,
    collectionId,
    folder: target.kind === 'folder' ? target.name : '',
  }
}

function selectMenuAction(action: string) {
  const target = menu.value.target
  closeMenu()
  const collectionId = targetCollectionId(target)
  if (action === 'new-request') {
    openNewRequest(target)
    return
  }
  if (action === 'new-folder') {
    const collection = apiTesting.treeCollections.find((item) => item.id === collectionId)
    if (!collection) return
    ensureCollection(collection.id)
    newRequestDialog.value = { open: true, collectionId: collection.id, folder: createFolderName(collection) }
    return
  }
  if (action === 'import-requests') {
    ensureCollection(collectionId)
    void apiTesting.importRequests()
    return
  }
  if (action === 'new-collection') {
    void apiTesting.createCollection()
    return
  }
  if (action === 'save-collection') {
    ensureCollection(collectionId)
    void apiTesting.saveCollection()
    return
  }
  if (action === 'rename-collection') {
    ensureCollection(collectionId)
    const current = apiTesting.draftCollection?.name || ''
    openInputDialog('rename-collection', target, {
      title: '重命名集合',
      label: '集合名称',
      value: current,
      placeholder: '集合名称',
    })
    return
  }
  if (action === 'delete-collection') {
    ensureCollection(collectionId)
    openConfirmDialog('delete-collection', target, {
      title: '删除集合',
      message: '集合和其中的请求会被移除。',
      confirmLabel: '删除',
      danger: true,
    })
    return
  }
  if (target.kind === 'folder' && action === 'rename-folder') {
    ensureCollection(target.collectionId)
    openInputDialog('rename-folder', target, {
      title: '重命名分组',
      label: '分组名称',
      value: target.name,
      placeholder: '分组名称',
    })
    return
  }
  if (target.kind !== 'request') return
  ensureCollection(target.collectionId)
  if (action === 'run-request') {
    apiTesting.selectRequest(target.id)
    void apiTesting.runSelectedRequest()
  } else if (action === 'duplicate-request') {
    apiTesting.duplicateRequest(target.id)
  } else if (action === 'rename-request') {
    const request = apiTesting.draftCollection?.requests.find((item) => item.id === target.id)
    openInputDialog('rename-request', target, {
      title: '重命名请求',
      label: '请求名称',
      value: request?.name || '',
      placeholder: '请求名称',
    })
  } else if (action === 'move-request') {
    const request = apiTesting.draftCollection?.requests.find((item) => item.id === target.id)
    openInputDialog('move-request', target, {
      title: '移动请求',
      label: '分组名称',
      value: request?.folder || '',
      placeholder: '留空为未分组',
      allowEmpty: true,
    })
  } else if (action === 'delete-request') {
    openConfirmDialog('delete-request', target, {
      title: '删除请求',
      message: '这个请求会从当前集合中移除。',
      confirmLabel: '删除',
      danger: true,
    })
  }
}

function confirmActionDialog(value: string) {
  const dialog = actionDialog.value
  const target = dialog.target
  if (!target) return
  closeActionDialog()
  if (dialog.action === 'rename-collection') {
    const collectionId = targetCollectionId(target)
    ensureCollection(collectionId)
    apiTesting.updateCollectionName(value)
    return
  }
  if (dialog.action === 'delete-collection') {
    const collectionId = targetCollectionId(target)
    ensureCollection(collectionId)
    void apiTesting.removeCurrentCollection()
    return
  }
  if (dialog.action === 'rename-folder' && target.kind === 'folder') {
    ensureCollection(target.collectionId)
    apiTesting.renameFolder(target.name, value)
    return
  }
  if (target.kind !== 'request') return
  ensureCollection(target.collectionId)
  if (dialog.action === 'rename-request') {
    apiTesting.renameRequest(target.id, value)
  } else if (dialog.action === 'move-request') {
    apiTesting.moveRequestToFolder(target.id, value)
  } else if (dialog.action === 'delete-request') {
    apiTesting.removeRequest(target.id)
  }
}

function createRequest(payload: { folder: string } & Partial<Omit<APIRequest, 'id' | 'updatedAt'>>) {
  ensureCollection(newRequestDialog.value.collectionId)
  const { folder, ...fields } = payload
  apiTesting.createRequest(folder, fields)
  newRequestDialog.value.open = false
}
</script>

<template>
  <aside class="api-request-list" aria-label="集合树">
    <header class="api-sidebar-header">
      <span>Collections</span>
      <AriButton size="icon" variant="ghost" aria-label="集合菜单" @click="openMenu($event, { kind: 'global' })">
        <MoreHorizontal :size="15" />
      </AriButton>
    </header>

    <div class="api-tree" role="tree">
      <section v-for="collection in apiTesting.treeCollections" :key="collection.id" class="api-tree-collection">
        <div
          class="api-tree-root"
          :class="{ 'is-active': collection.id === apiTesting.selectedCollectionId }"
          @contextmenu="openMenu($event, { kind: 'collection', collectionId: collection.id })"
        >
          <button type="button" class="api-tree-root-toggle" @click="toggleCollection(collection.id)">
            <ChevronDown v-if="isCollectionExpanded(collection.id)" :size="14" />
            <ChevronRight v-else :size="14" />
            <Box :size="15" />
            <span>{{ collection.name }}</span>
          </button>
          <AriButton size="icon" variant="ghost" aria-label="集合菜单" @click="openMenu($event, { kind: 'collection', collectionId: collection.id })">
            <MoreHorizontal :size="14" />
          </AriButton>
        </div>

        <div v-if="isCollectionExpanded(collection.id)" class="api-tree-collection-body">
          <section v-for="group in requestGroups(collection)" :key="group.name" class="api-tree-group">
            <div class="api-tree-folder" @contextmenu="openMenu($event, { kind: 'folder', collectionId: collection.id, name: group.name })">
              <button type="button" class="api-tree-folder-toggle" @click="toggleGroup(collection.id, group.name)">
                <ChevronRight v-if="isGroupCollapsed(collection.id, group.name)" :size="14" />
                <ChevronDown v-else :size="14" />
                <Folder :size="14" />
                <span>{{ group.name }}</span>
                <small>{{ group.requests.length }}</small>
              </button>
              <AriButton size="icon" variant="ghost" aria-label="分组菜单" @click="openMenu($event, { kind: 'folder', collectionId: collection.id, name: group.name })">
                <MoreHorizontal :size="14" />
              </AriButton>
            </div>

            <div v-if="!isGroupCollapsed(collection.id, group.name)" class="api-tree-requests">
              <div
                v-for="request in group.requests"
                :key="request.id"
                class="api-request-row"
                :class="{ 'is-selected': request.id === apiTesting.selectedRequestId }"
                @contextmenu="openMenu($event, { kind: 'request', collectionId: collection.id, id: request.id })"
              >
                <button
                  type="button"
                  class="api-request-hit"
                  @click="selectRequest(collection.id, request.id)"
                >
                  <span class="api-method-text" :class="`is-${request.method.toLowerCase()}`">{{ request.method }}</span>
                  <strong>{{ request.name }}</strong>
                </button>
                <AriButton size="icon" variant="ghost" aria-label="请求菜单" @click="openMenu($event, { kind: 'request', collectionId: collection.id, id: request.id })">
                  <MoreHorizontal :size="14" />
                </AriButton>
              </div>
            </div>
          </section>
        </div>
      </section>
    </div>

    <ApiContextMenu :open="menu.open" :x="menu.x" :y="menu.y" :items="menuItems" @close="closeMenu" @select="selectMenuAction" />
    <ApiActionDialog
      :open="actionDialog.open"
      :mode="actionDialog.mode"
      :title="actionDialog.title"
      :message="actionDialog.message"
      :label="actionDialog.label"
      :value="actionDialog.value"
      :placeholder="actionDialog.placeholder"
      :confirm-label="actionDialog.confirmLabel"
      :danger="actionDialog.danger"
      :allow-empty="actionDialog.allowEmpty"
      @close="closeActionDialog"
      @confirm="confirmActionDialog"
    />
    <ApiNewRequestDialog
      :open="newRequestDialog.open"
      :folder="newRequestDialog.folder"
      @close="newRequestDialog.open = false"
      @create="createRequest"
    />
  </aside>
</template>
