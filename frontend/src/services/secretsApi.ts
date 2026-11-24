import type { SecretActionResult, SecretStatus } from '../types/fancun'
import { formatRpcError, isDesktopRuntime } from './runtimeEnv'

const fallbackStatus: SecretStatus = {
  available: false,
  backend: 'fallback',
  records: [
    {
      kind: 'ai_api_key',
      label: 'OCR API key',
      targetName: 'Fancun/OpenAI/APIKey',
      stored: false,
      envNames: ['FANCUN_AI_API_KEY', 'OPENAI__API_KEY', 'OPENAI_API_KEY'],
      envPresent: false,
      activeSource: 'missing',
    },
  ],
}

async function trySecretsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/fancun/internal/secrets/service.js')
  } catch {
    return null
  }
}

export async function getSecretStatus(): Promise<SecretStatus> {
  const binding = await trySecretsBinding()
  if (binding) {
    try {
      return await binding.Status()
    } catch (error) {
      if (isDesktopRuntime()) {
        throw new Error(formatRpcError(error, '读取密钥状态失败'))
      }
      return structuredClone(fallbackStatus)
    }
  }
  if (isDesktopRuntime()) {
    throw new Error('密钥服务绑定不可用')
  }
  return structuredClone(fallbackStatus)
}

export async function saveSecret(kind: string, value: string): Promise<SecretActionResult> {
  const binding = await trySecretsBinding()
  if (binding) {
    return await binding.SaveSecret({ kind, value })
  }
  if (isDesktopRuntime()) {
    throw new Error('密钥服务绑定不可用')
  }
  return {
    ok: false,
    message: '开发态 fallback 不保存密钥',
    status: structuredClone(fallbackStatus),
  }
}

export async function clearSecret(kind: string, confirm: boolean): Promise<SecretActionResult> {
  const binding = await trySecretsBinding()
  if (binding) {
    return await binding.ClearSecret({ kind, confirm })
  }
  if (isDesktopRuntime()) {
    throw new Error('密钥服务绑定不可用')
  }
  return {
    ok: false,
    message: confirm ? '开发态 fallback 不清除密钥' : '再次点击确认清除密钥',
    requiresConfirmation: !confirm,
    status: structuredClone(fallbackStatus),
  }
}
