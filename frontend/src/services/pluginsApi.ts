import type { PluginManifest } from '../types/ariadne'

async function tryPluginsBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/plugins/service.js')
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
