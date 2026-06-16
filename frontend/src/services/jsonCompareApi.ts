import type {
  JsonCompareRequest,
  JsonCompareResult,
  JsonDifference,
  JsonFormatRequest,
  JsonFormatResult,
} from '../types/ariadne'

async function tryJsonCompareBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/jsoncompare/service.js')
  } catch {
    return null
  }
}

export async function compareJson(request: JsonCompareRequest): Promise<JsonCompareResult> {
  const binding = await tryJsonCompareBinding()
  if (binding) {
    return normalizeCompareResult(await binding.Compare(request))
  }
  return fallbackCompare(request)
}

export async function formatJson(request: JsonFormatRequest): Promise<JsonFormatResult> {
  const binding = await tryJsonCompareBinding()
  if (binding) {
    return normalizeFormatResult(await binding.Format(request))
  }
  try {
    const parsed = JSON.parse(request.text)
    return {
      ok: true,
      text: JSON.stringify(parsed, null, 2),
    }
  } catch (error) {
    return {
      ok: false,
      text: '',
      error: `${request.label || 'JSON'} 解析失败: ${error instanceof Error ? error.message : String(error)}`,
    }
  }
}

function normalizeCompareResult(result: JsonCompareResult): JsonCompareResult {
  const differences = (result.differences ?? []).map(normalizeDifference)
  return {
    ok: Boolean(result.ok),
    summary: result.summary || (result.ok ? '两个 JSON 语义一致' : '解析失败'),
    differences,
    report: result.report || '',
    unifiedDiff: result.unifiedDiff || '',
    leftFormatted: result.leftFormatted || '',
    rightFormatted: result.rightFormatted || '',
    diffTruncated: Boolean(result.diffTruncated),
    differencesTruncated: Boolean(result.differencesTruncated),
    formattedTruncated: Boolean(result.formattedTruncated),
    performanceNote: result.performanceNote || '',
    error: result.error || '',
    added: Number(result.added ?? differences.filter((item) => item.kind === 'added').length),
    removed: Number(result.removed ?? differences.filter((item) => item.kind === 'removed').length),
    changed: Number(result.changed ?? differences.filter((item) => item.kind === 'changed').length),
  }
}

function normalizeDifference(difference: JsonDifference): JsonDifference {
  return {
    kind: String(difference.kind ?? ''),
    path: String(difference.path ?? ''),
    left: difference.left,
    right: difference.right,
  }
}

function normalizeFormatResult(result: JsonFormatResult): JsonFormatResult {
  return {
    ok: Boolean(result.ok),
    text: result.text || '',
    error: result.error || '',
  }
}

function fallbackCompare(request: JsonCompareRequest): JsonCompareResult {
  try {
    const left = JSON.parse(request.leftText)
    const right = JSON.parse(request.rightText)
    const leftFormatted = JSON.stringify(left, null, 2)
    const rightFormatted = JSON.stringify(right, null, 2)
    const same = leftFormatted === rightFormatted
    return {
      ok: true,
      summary: same ? '两个 JSON 语义一致' : '开发态 fallback：格式化文本存在差异',
      differences: [],
      report: same ? '两个 JSON 语义一致。对象字段顺序不会被判定为差异。' : '开发态 fallback 仅执行格式化文本对比；桌面版使用 Go 语义差异服务。',
      unifiedDiff: same ? '(规范化格式后没有行差异)' : `--- left.json\n+++ right.json\n@@\n-${leftFormatted}\n+${rightFormatted}`,
      leftFormatted,
      rightFormatted,
      diffTruncated: false,
      differencesTruncated: false,
      formattedTruncated: false,
      performanceNote: '',
      added: 0,
      removed: 0,
      changed: same ? 0 : 1,
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return {
      ok: false,
      summary: '解析失败',
      differences: [],
      report: message,
      unifiedDiff: '',
      leftFormatted: '',
      rightFormatted: '',
      diffTruncated: false,
      differencesTruncated: false,
      formattedTruncated: false,
      performanceNote: '',
      error: message,
      added: 0,
      removed: 0,
      changed: 0,
    }
  }
}
