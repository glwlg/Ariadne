import type { SecretActionResult, SecretStatus } from '../types/ariadne'

const fallbackStatus: SecretStatus = {
  available: false,
  backend: 'fallback',
  records: [
    {
      kind: 'ai_api_key',
      label: 'AI API key',
      targetName: 'Ariadne/OpenAI/APIKey',
      stored: false,
      envNames: ['ARIADNE_AI_API_KEY', 'OPENAI__API_KEY', 'OPENAI_API_KEY'],
      envPresent: false,
      activeSource: 'missing',
    },
    {
      kind: 'embedding_api_key',
      label: 'Embedding API key',
      targetName: 'Ariadne/Embedding/APIKey',
      stored: false,
      envNames: ['ARIADNE_EMBED_API_KEY', 'EMBED__API_KEY', 'OPENAI__API_KEY', 'OPENAI_API_KEY'],
      envPresent: false,
      activeSource: 'missing',
    },
    {
      kind: 'milvus_token',
      label: 'Milvus token',
      targetName: 'Ariadne/Milvus/Token',
      stored: false,
      envNames: ['ARIADNE_MILVUS_TOKEN', 'MILVUS__TOKEN', 'MILVUS_TOKEN'],
      envPresent: false,
      activeSource: 'missing',
    },
  ],
}

async function trySecretsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/secrets/service.js')
  } catch {
    return null
  }
}

export async function getSecretStatus(): Promise<SecretStatus> {
  const binding = await trySecretsBinding()
  if (binding) {
    try {
      return await binding.Status()
    } catch {
      return structuredClone(fallbackStatus)
    }
  }
  return structuredClone(fallbackStatus)
}

export async function saveSecret(kind: string, value: string): Promise<SecretActionResult> {
  const binding = await trySecretsBinding()
  if (binding) {
    return await binding.SaveSecret({ kind, value })
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
  return {
    ok: false,
    message: confirm ? '开发态 fallback 不清除密钥' : '再次点击确认清除密钥',
    requiresConfirmation: !confirm,
    status: structuredClone(fallbackStatus),
  }
}
