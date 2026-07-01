<script setup lang="ts">
import {
  AppWindow,
  Brain,
  Camera,
  Clipboard,
  Clock3,
  Code2,
  CornerDownLeft,
  Copy,
  Database,
  ExternalLink,
  FileText,
  FolderOpen,
  ListChecks,
  MoreHorizontal,
  Pin,
  Play,
  Search,
  Settings,
  TerminalSquare,
  Workflow,
} from '@lucide/vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import { Window } from '@wailsio/runtime'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import AriButton from '../ui/AriButton.vue'
import { applyLauncherWindowGeometry } from '../../lib/launcherGeometry'
import { useLauncherStore } from '../../stores/launcher'
import type { CommandParam, CommandSchema, PreviewAction, SearchResult, SearchResultType } from '../../types/ariadne'

const launcher = useLauncherStore()
const searchInput = ref<HTMLInputElement | null>(null)
const commandParamDrafts = ref<Record<string, string>>({})

const iconMap = {
  app: AppWindow,
  capture: Camera,
  clipboard: Clipboard,
  command: TerminalSquare,
  file: FileText,
  folder: FolderOpen,
  memory: Brain,
  open: ExternalLink,
  pin: Pin,
  plugin: Code2,
  plugin_result: Code2,
  plugin_trigger: Code2,
  settings: Settings,
  workflow: Workflow,
}

const actionIconMap = {
  copy: Copy,
  folder: FolderOpen,
  open: ExternalLink,
  pin: Pin,
  plugin: Code2,
  remember: Brain,
  run: Play,
  settings: Settings,
  workflow: Workflow,
}

const selected = computed(() => launcher.selectedResult)
const preview = computed(() => selected.value?.preview ?? null)
const primaryActions = computed(() => selected.value?.actions.slice(0, 2) ?? [])
const extraActions = computed(() => selected.value?.actions.slice(2) ?? [])
const selectedTags = computed(() => selected.value?.tags?.slice(0, 4) ?? [])
const pluginCommand = computed(() => commandPayload(selected.value))
const pluginCommandExamples = computed(() => pluginCommand.value?.schema.examples?.slice(0, 3) ?? [])
const pluginCommandParams = computed(() => pluginCommand.value?.schema.params ?? [])
const pluginCommandDraft = computed(() => {
  const command = pluginCommand.value
  if (!command) return ''
  const parts = [command.keyword]
  for (const param of command.schema.params ?? []) {
    const value = commandParamDrafts.value[param.name]?.trim()
    if (value) {
      parts.push(value)
    }
  }
  return parts.join(' ')
})
const pluginCommandReady = computed(() => {
  const command = pluginCommand.value
  if (!command) return false
  return (command.schema.params ?? []).every((param) => !param.required || Boolean(commandParamDrafts.value[param.name]?.trim()))
})

async function resizePalette(expanded: boolean) {
  await applyLauncherWindowGeometry(expanded)
}

function resultIcon(type: SearchResultType, icon: string) {
  return iconMap[icon as keyof typeof iconMap] ?? iconMap[type] ?? FileText
}

function actionIcon(action: PreviewAction) {
  return actionIconMap[action.icon as keyof typeof actionIconMap] ?? MoreHorizontal
}

function commandPayload(result: SearchResult | null) {
  if (!result || result.type !== 'plugin_trigger' || !isRecord(result.payload)) {
    return null
  }
  const schema = result.payload.commandSchema
  if (!isCommandSchema(schema)) {
    return null
  }
  const keyword = typeof result.payload.keyword === 'string' && result.payload.keyword.trim()
    ? result.payload.keyword.trim()
    : firstUsageToken(schema.usage)
  if (!keyword) {
    return null
  }
  return { keyword, schema }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function isCommandSchema(value: unknown): value is CommandSchema {
  if (!isRecord(value) || typeof value.usage !== 'string') {
    return false
  }
  if (value.params !== undefined && !Array.isArray(value.params)) {
    return false
  }
  if (value.examples !== undefined && !Array.isArray(value.examples)) {
    return false
  }
  return true
}

function firstUsageToken(usage: string) {
  return usage.trim().split(/\s+/)[0]?.replace(/[<>\[\]]/g, '') ?? ''
}

function paramValue(param: CommandParam) {
  return commandParamDrafts.value[param.name] ?? ''
}

function updateParam(param: CommandParam, value: string) {
  commandParamDrafts.value = { ...commandParamDrafts.value, [param.name]: value }
}

async function applyCommandSuggestion(value: string) {
  await launcher.applyCommandSuggestion(value)
  await nextTick()
  searchInput.value?.focus()
  const end = searchInput.value?.value.length ?? 0
  searchInput.value?.setSelectionRange(end, end)
}

function applyCurrentCommand() {
  const command = pluginCommand.value
  if (!command) return
  const draft = pluginCommandDraft.value.trim()
  void applyCommandSuggestion(pluginCommandReady.value && draft ? draft : `${command.keyword} `)
}

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || target.isContentEditable
}

function sourceTone(type: SearchResult['type']) {
  if (type === 'memory') return 'text-[var(--primary)]'
  if (type === 'clipboard') return 'text-[var(--warning)]'
  if (type === 'workflow') return 'text-[var(--success)]'
  if (type === 'file') return 'text-[var(--info)]'
  return 'text-[var(--muted)]'
}

function resultTypeLabel(type: SearchResult['type']) {
  const labels: Record<SearchResult['type'], string> = {
    app: '应用',
    capture: '截图',
    clipboard: '剪贴板',
    command: '命令',
    file: '文件',
    memory: '记忆',
    plugin_result: '插件',
    plugin_trigger: '插件',
    settings: '设置',
    workflow: '工作流',
  }
  return labels[type] ?? '结果'
}

function triggerResultAction(result: SearchResult, action?: PreviewAction) {
  if (!action) return
  launcher.select(result.id)
  void launcher.triggerAction(action)
}

async function hideLauncher() {
  try {
    await Window.SetAlwaysOnTop(false)
    await Window.Hide()
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    void hideLauncher()
    return
  }
  if (event.target !== searchInput.value && isEditableTarget(event.target)) {
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    launcher.moveSelection(1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    launcher.moveSelection(-1)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    if (pluginCommand.value) {
      applyCurrentCommand()
    } else {
      launcher.runPrimaryAction()
    }
  }
}

function focusLauncher(event?: Event) {
  if (event instanceof CustomEvent && event.detail?.reset) {
    launcher.reset()
  }
  void resizePalette(launcher.isExpanded)
  searchInput.value?.focus()
  if (!event || !(event instanceof CustomEvent) || event.detail?.selectAll !== false) {
    searchInput.value?.select()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('ariadne:focus-launcher', focusLauncher)
  focusLauncher()
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('ariadne:focus-launcher', focusLauncher)
})

watch(
  () => launcher.isExpanded,
  (expanded) => {
    void resizePalette(expanded)
  },
  { immediate: true },
)

watch(
  () => selected.value?.id,
  () => {
    commandParamDrafts.value = {}
  },
)
</script>

<template>
  <main class="launcher-surface min-h-screen bg-[var(--background)] text-[var(--foreground)]">
    <div class="app-frame">
      <section
        class="launcher-shell palette-shell"
        :class="{ 'is-collapsed': !launcher.isExpanded }"
        aria-label="Ariadne launcher"
      >
        <div class="search-row">
          <Search :size="19" class="text-[var(--muted)]" />
          <input
            ref="searchInput"
            :value="launcher.query"
            class="search-input"
            spellcheck="false"
            placeholder="搜索，或输入命令：jsondiff / hosts / clip token / wf ..."
            @input="launcher.setQuery(($event.target as HTMLInputElement).value)"
          />
          <kbd>Alt Q</kbd>
        </div>

        <div v-if="launcher.isExpanded" class="palette-results" aria-label="搜索结果">
          <section class="results-pane">
            <div class="pane-title">
              <span>结果</span>
              <small>{{ launcher.results.length }} 项</small>
            </div>

            <div
              v-for="result in launcher.results"
              :key="result.id"
              class="result-row"
              :class="{ 'is-selected': result.id === launcher.selectedId }"
              role="button"
              tabindex="0"
              @click="launcher.select(result.id)"
              @dblclick="triggerResultAction(result, result.actions[0])"
              @keydown.enter.stop.prevent="triggerResultAction(result, result.actions[0])"
            >
              <span class="result-icon" :class="sourceTone(result.type)">
                <component :is="resultIcon(result.type, result.icon)" :size="18" />
              </span>
              <span class="result-main">
                <span class="result-title">{{ result.title }}</span>
                <span class="result-subtitle">{{ result.subtitle }}</span>
              </span>
              <button
                v-if="result.actions[0]"
                type="button"
                class="result-primary-action"
                @click.stop="triggerResultAction(result, result.actions[0])"
              >
                {{ result.actions[0].label }}
              </button>
              <span v-else class="result-primary-action">打开</span>
            </div>

            <div v-if="!launcher.results.length" class="empty-state">
              <Database :size="22" />
              <span>没有找到匹配线索</span>
            </div>
          </section>

          <aside v-if="selected && preview" class="preview-pane palette-preview" aria-label="结果预览">
            <header class="preview-header">
              <div>
                <span class="preview-kicker">{{ resultTypeLabel(selected.type) }}</span>
                <h1>{{ preview.title || selected.title }}</h1>
                <p>{{ preview.subtitle || selected.subtitle }}</p>
              </div>
              <span class="result-icon" :class="sourceTone(selected.type)">
                <component :is="resultIcon(selected.type, selected.icon)" :size="19" />
              </span>
            </header>

            <div v-if="preview.kind === 'image'" class="memory-thumb">
              <Database :size="24" />
              <span>{{ preview.imageHint || '图片预览' }}</span>
            </div>

            <pre v-if="preview.text" class="preview-text">{{ preview.text }}</pre>

            <div v-if="pluginCommand" class="command-builder" aria-label="插件命令参数">
              <div class="command-builder-head">
                <span>
                  <ListChecks :size="15" />
                  <code>{{ pluginCommand.schema.usage }}</code>
                </span>
                <AriButton size="sm" :variant="pluginCommandReady ? 'primary' : 'secondary'" @click="applyCurrentCommand">
                  <CornerDownLeft :size="14" />
                  {{ pluginCommandReady ? '填入命令' : '填入前缀' }}
                </AriButton>
              </div>

              <div v-if="pluginCommandParams.length" class="command-param-list">
                <label v-for="param in pluginCommandParams" :key="param.name" class="command-param">
                  <span>
                    {{ param.label || param.name }}
                    <em>{{ param.required ? '必填' : '可选' }}</em>
                  </span>
                  <input
                    :value="paramValue(param)"
                    :data-command-param="param.name"
                    :placeholder="param.placeholder"
                    spellcheck="false"
                    @input="updateParam(param, ($event.target as HTMLInputElement).value)"
                    @keydown.enter.prevent="applyCurrentCommand"
                  />
                </label>
              </div>

              <div class="command-preview" data-command-preview :title="pluginCommandDraft || pluginCommand.keyword">
                <span>命令</span>
                <code>{{ pluginCommandDraft || pluginCommand.keyword }}</code>
              </div>

              <div v-if="pluginCommandExamples.length" class="command-examples">
                <button
                  v-for="example in pluginCommandExamples"
                  :key="example"
                  type="button"
                  @click="applyCommandSuggestion(example)"
                >
                  <CornerDownLeft :size="13" />
                  <code>{{ example }}</code>
                </button>
              </div>
            </div>

            <div v-if="preview.meta?.length" class="meta-grid palette-meta">
              <div v-for="item in preview.meta.slice(0, 3)" :key="`${item.label}-${item.value}`" class="meta-item">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>

            <div v-if="preview.evidence?.length" class="evidence-list palette-evidence">
              <div v-for="item in preview.evidence.slice(0, 2)" :key="`${item.label}-${item.value}`" class="evidence-item">
                <FileText :size="14" />
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>

            <div v-if="selectedTags.length" class="result-tags palette-tags">
              <span v-for="tag in selectedTags" :key="tag">{{ tag }}</span>
            </div>
          </aside>
        </div>

        <footer v-if="launcher.isExpanded || launcher.lastAction" class="status-strip">
          <span>
            <Clock3 :size="14" />
            搜索 {{ launcher.elapsedMs }}ms
          </span>
          <span v-if="selected && !launcher.lastAction">
            Enter {{ selected.actions[0]?.label ?? '打开' }}
          </span>
          <span v-if="selected?.subtitle && !launcher.lastAction" class="status-detail">
            {{ selected.subtitle }}
          </span>
          <span v-if="launcher.lastAction" class="inline-feedback" :class="{ 'is-confirmation': launcher.lastAction.requiresConfirmation }">
            {{ launcher.lastAction.message }}
          </span>
          <div v-if="selected && (!launcher.lastAction || launcher.lastAction.requiresConfirmation)" class="palette-action-row">
            <AriButton
              v-for="action in primaryActions"
              :key="action.id"
              :variant="action.kind === 'copy' ? 'primary' : 'secondary'"
              size="sm"
              @click="launcher.triggerAction(action)"
            >
              <component :is="actionIcon(action)" :size="15" />
              {{ action.label }}
            </AriButton>

            <DropdownMenuRoot v-if="extraActions.length">
              <DropdownMenuTrigger as-child>
                <AriButton size="sm" variant="ghost">
                  <MoreHorizontal :size="15" />
                </AriButton>
              </DropdownMenuTrigger>
              <DropdownMenuPortal>
                <DropdownMenuContent class="action-menu" :side-offset="8" align="end">
                  <DropdownMenuItem
                    v-for="action in extraActions"
                    :key="action.id"
                    class="action-menu-item"
                    @select="launcher.triggerAction(action)"
                  >
                    <component :is="actionIcon(action)" :size="15" />
                    <span>{{ action.label }}</span>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenuPortal>
            </DropdownMenuRoot>
          </div>
        </footer>
      </section>
    </div>
  </main>
</template>
