import type { PluginManifest } from '../types/fancun'

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
    } catch {
      return []
    }
  }
  return []
}
