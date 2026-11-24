import type { PluginManifest } from '../types/fancun'
import { formatRpcError, isDesktopRuntime } from './runtimeEnv'

async function tryPluginsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/fancun/internal/plugins/service.js')
  } catch {
    return null
  }
}

export async function listPlugins(): Promise<PluginManifest[]> {
  const binding = await tryPluginsBinding()
  if (binding) {
    try {
      return await binding.List()
    } catch (error) {
      if (isDesktopRuntime()) {
        throw new Error(formatRpcError(error, '读取插件列表失败'))
      }
      return []
    }
  }
  if (isDesktopRuntime()) {
    throw new Error('插件服务绑定不可用')
  }
  return []
}
