import { seedResults } from '../data/seed'
import type { ActionResult, PreviewAction, SearchResponse } from '../types/ariadne'
import type { SearchResult } from '../types/ariadne'

export class SearchCancelledError extends Error {
  constructor(message = 'Search request was cancelled') {
    super(message)
    this.name = 'SearchCancelledError'
  }
}

export interface AriadneSearchRequest {
  promise: Promise<SearchResponse>
  cancel: (cause?: unknown) => void
}

type CancellableSearchPromise = Promise<SearchResponse> & {
  cancel?: (cause?: unknown) => Promise<void> | void
}

function normalize(value: string) {
  return value.trim().toLowerCase()
}

function fallbackSearch(query: string): SearchResponse {
  const started = performance.now()
  const q = normalize(query)
  if (!q) {
    return {
      query,
      results: [],
      elapsedMs: Math.round(performance.now() - started),
    }
  }

  const commandResults = fallbackCommandResults(query)
  const results = commandResults.length ? commandResults : seedResults.filter((result) => {
    const haystack = [
      result.type,
      result.title,
      result.subtitle,
      result.detail,
      ...(result.tags ?? []),
      result.preview.title,
      result.preview.subtitle,
      result.preview.text,
      ...(result.preview.meta?.flatMap((item) => [item.label, item.value]) ?? []),
      ...(result.preview.evidence?.flatMap((item) => [item.label, item.value]) ?? []),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()

    return haystack.includes(q)
  })

  return {
    query,
    results,
    elapsedMs: Math.round(performance.now() - started),
  }
}

function fallbackCommandResults(query: string): SearchResult[] {
  const trimmed = query.trim()
  const [keyword = '', ...rest] = trimmed.split(/\s+/)
  const value = rest.join(' ')
  const lowerKeyword = keyword.toLowerCase()

  if (['uuid', 'guid'].includes(lowerKeyword)) {
    const count = clampCount(Number.parseInt(value, 10) || 5)
    return Array.from({ length: count }, (_, index) => {
      const uuid = crypto.randomUUID()
      return copyResult(`uuid-${index + 1}`, uuid, 'UUID v4', uuid, '随机 UUID', ['UUID'])
    })
  }

  if (['base64', 'b64', 'b'].includes(lowerKeyword) && value) {
    const encoded = btoa(unescape(encodeURIComponent(value)))
    return [copyResult('base64-encode', `编码结果: ${encoded}`, 'Base64', encoded, value, ['Base64'])]
  }

  if (['url', 'u'].includes(lowerKeyword) && value) {
    const encoded = encodeURIComponent(value)
    return [copyResult('url-encode', `编码结果: ${encoded}`, 'URL', encoded, value, ['URL'])]
  }

  if (['json', 'j'].includes(lowerKeyword) && value) {
    try {
      const parsed = JSON.parse(value)
      const formatted = JSON.stringify(parsed, null, 2)
      const minified = JSON.stringify(parsed)
      return [
        copyResult('json-format', `格式化结果: ${clip(formatted)}`, 'JSON', formatted, '格式化 JSON', ['JSON']),
        copyResult('json-minify', `压缩结果: ${clip(minified)}`, 'JSON', minified, '压缩 JSON', ['JSON']),
      ]
    } catch {
      return [messageResult('json-error', 'JSON 解析错误', 'JSON', '请检查 JSON 文本。')]
    }
  }

  if (['calc', 'calculate', 'c'].includes(lowerKeyword) && value) {
    const normalizedExpression = value.replaceAll('x', '*').replaceAll('^', '**')
    if (!/^[\d\s+\-*/().%*]+$/.test(normalizedExpression)) {
      return [messageResult('calc-error', '无效的表达式', '计算器', value)]
    }
    try {
      const result = Function(`"use strict"; return (${normalizedExpression})`)()
      return [copyResult('calc-result', `= ${String(result)}`, '计算器', String(result), value, ['计算器'])]
    } catch {
      return [messageResult('calc-error', '计算错误', '计算器', value)]
    }
  }

  if (['qr', 'qrcode'].includes(lowerKeyword) && value) {
    return [
      {
        id: 'qr-generate',
        type: 'plugin_result',
        title: '生成二维码',
        subtitle: '二维码生成',
        detail: value,
        icon: 'plugin',
        tags: ['二维码', '贴图'],
        payload: { qrText: value },
        preview: {
          kind: 'image',
          title: '生成二维码',
          subtitle: '内容会在前端预览，并可贴到屏幕',
          text: value,
          imageHint: 'QR 预览',
        },
        actions: [
          { id: 'pin_qr', label: '贴到屏幕', icon: 'pin', kind: 'pin', payload: { text: value } },
          copyAction('copy_qr_text', '复制内容', value),
        ],
      },
    ]
  }

  return []
}

function copyResult(id: string, title: string, subtitle: string, text: string, detail: string, tags: string[]): SearchResult {
  return {
    id,
    type: 'plugin_result',
    title,
    subtitle,
    detail,
    icon: 'plugin',
    tags,
    preview: {
      kind: 'text',
      title,
      subtitle,
      text,
      meta: [{ label: '动作来源', value: '开发态 fallback preview action' }],
    },
    actions: [copyAction('copy_value', '复制结果', text), {
      id: 'remember',
      label: '加入记忆',
      icon: 'remember',
      kind: 'remember',
      payload: { targetId: id },
      feedback: { successLabel: '已加入' },
    }],
  }
}

function copyAction(id: string, label: string, text: string): PreviewAction {
  return {
    id,
    label,
    icon: 'copy',
    kind: 'copy',
    payload: { text },
    feedback: { successLabel: '已复制', durationMs: 1400 },
  }
}

function messageResult(id: string, title: string, subtitle: string, text: string): SearchResult {
  return {
    id,
    type: 'plugin_result',
    title,
    subtitle,
    detail: text,
    icon: 'plugin',
    tags: ['插件'],
    preview: { kind: 'text', title, subtitle, text },
    actions: [copyAction('copy_message', '复制说明', text)],
  }
}

function clampCount(count: number) {
  return Math.max(1, Math.min(50, count))
}

function clip(value: string) {
  return value.length > 96 ? `${value.slice(0, 96)}...` : value
}

async function tryWailsAction(action: PreviewAction): Promise<ActionResult | null> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/platform/service.js')
    return await binding.ExecuteAction(action)
  } catch {
    return null
  }
}

async function tryWailsRecordUse(resultId: string): Promise<boolean> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/search/service.js')
    await binding.RecordUse(resultId)
    return true
  } catch {
    return false
  }
}

async function tryWailsSetFavorite(resultId: string, favorite: boolean): Promise<boolean> {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    const binding = await import('../../bindings/ariadne/internal/search/service.js')
    await binding.SetFavorite(resultId, favorite)
    return true
  } catch {
    return false
  }
}

export function createAriadneSearchRequest(query: string): AriadneSearchRequest {
  let cancelled = false
  let activePromise: CancellableSearchPromise | null = null

  const promise = (async () => {
    try {
      // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
      const binding = await import('../../bindings/ariadne/internal/search/service.js')
      if (cancelled) {
        throw new SearchCancelledError()
      }
      const request = binding.Search(query) as CancellableSearchPromise
      activePromise = request
      if (cancelled) {
        cancelQuietly(request, 'superseded')
        throw new SearchCancelledError()
      }
      return await request
    } catch (error) {
      if (cancelled || isSearchCancelled(error)) {
        throw new SearchCancelledError()
      }
      return fallbackSearch(query)
    }
  })()

  return {
    promise,
    cancel(cause?: unknown) {
      cancelled = true
      if (activePromise) {
        cancelQuietly(activePromise, cause ?? 'superseded')
      }
    },
  }
}

function cancelQuietly(request: CancellableSearchPromise, cause: unknown) {
  const cancellation = request.cancel?.(cause)
  if (cancellation && typeof cancellation === 'object' && 'then' in cancellation) {
    void Promise.resolve(cancellation).catch(() => {})
  }
}

export function isSearchCancelled(error: unknown) {
  if (error instanceof SearchCancelledError) {
    return true
  }
  if (!error || typeof error !== 'object') {
    return false
  }
  const name = 'name' in error ? String((error as { name?: unknown }).name ?? '') : ''
  return name === 'CancelError' || name === 'SearchCancelledError'
}

export async function searchAriadne(query: string): Promise<SearchResponse> {
  return await createAriadneSearchRequest(query).promise
}

export async function executeAriadneAction(action: PreviewAction): Promise<ActionResult> {
  const response = await tryWailsAction(action)
  if (response) {
    return response
  }
  if ((action.kind === 'danger' || action.payload?.requiresConfirmation) && !action.payload?.confirmed && !action.payload?.confirm) {
    return {
      ok: false,
      message: `再次点击确认运行：${String(action.payload?.command ?? action.label)}`,
      requiresConfirmation: true,
      riskReasons: ['命令类启动项会启动本机进程'],
    }
  }

  return {
    ok: true,
    message: action.feedback?.successLabel ?? `${action.label} 已发送`,
  }
}

export async function recordResultUse(resultId: string): Promise<boolean> {
  if (!resultId) {
    return false
  }
  return await tryWailsRecordUse(resultId)
}

export async function setResultFavorite(resultId: string, favorite: boolean): Promise<boolean> {
  if (!resultId) {
    return false
  }
  return await tryWailsSetFavorite(resultId, favorite)
}
