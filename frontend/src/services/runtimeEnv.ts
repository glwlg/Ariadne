export function isDesktopRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false
  }
  const win = window as typeof window & {
    _wails?: {
      environment?: { OS?: string }
    }
    chrome?: {
      webview?: { postMessage?: unknown }
    }
    webkit?: {
      messageHandlers?: {
        external?: { postMessage?: unknown }
      }
    }
    wails?: {
      invoke?: unknown
    }
  }
  if (win._wails?.environment?.OS) {
    return true
  }
  if (typeof win.chrome?.webview?.postMessage === 'function') {
    return true
  }
  if (typeof win.webkit?.messageHandlers?.external?.postMessage === 'function') {
    return true
  }
  if (typeof win.wails?.invoke === 'function') {
    return true
  }
  const loc = win.location
  if (loc && (loc.hostname === 'wails.localhost' || loc.protocol === 'wails:')) {
    return true
  }
  return false
}

export function formatRpcError(error: unknown, fallback = '桌面服务调用失败'): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim()
  }
  if (typeof error === 'string' && error.trim()) {
    return error.trim()
  }
  if (error && typeof error === 'object' && 'message' in error) {
    const message = String((error as { message?: unknown }).message ?? '').trim()
    if (message) {
      return message
    }
  }
  return fallback
}
