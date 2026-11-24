/** Theme runtime. Visual constitution: docs/plans/2026-07-22-frontend-aesthetic-audit.md */
import { Events, Window } from '@wailsio/runtime'
import { getSettings } from '../services/settingsApi'

export type ThemePreference = 'professional-pink' | 'light-graphite' | 'cloud-blue'

const THEME_STORAGE_KEY = 'fancun:theme-preference'
const THEME_EVENT = 'fancun:theme-changed'

let currentTheme: ThemePreference = 'cloud-blue'
let lastAppliedWindowBg = ''

export function applyTheme(theme: string | undefined) {
  currentTheme = normalizeTheme(theme)
  document.documentElement.classList.remove('dark')
  document.documentElement.dataset.theme = currentTheme
  void syncWindowBackground(currentTheme)
}

export function publishTheme(theme: string | undefined) {
  const normalized = normalizeTheme(theme)
  applyTheme(normalized)
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify({ theme: normalized, at: Date.now() }))
  } catch {
    // Some embedded windows may not expose localStorage; the current window still updates above.
  }
  window.dispatchEvent(new CustomEvent(THEME_EVENT, { detail: { theme: normalized } }))
  try {
    void Events.Emit(THEME_EVENT, { theme: normalized })
  } catch {
    // Ignore in browser-only mode.
  }
}

export async function syncThemeFromSettings() {
  try {
    const settings = await getSettings()
    applyTheme(settings.general.theme)
  } catch {
    applyTheme('cloud-blue')
  }
}

export function installSystemThemeListener() {
  const syncPublishedTheme = (theme: string | undefined) => {
    applyTheme(theme)
  }
  const handleStorage = (event: StorageEvent) => {
    if (event.key !== THEME_STORAGE_KEY || !event.newValue) return
    try {
      const payload = JSON.parse(event.newValue) as { theme?: string }
      syncPublishedTheme(payload.theme)
    } catch {
      syncPublishedTheme(event.newValue)
    }
  }
  const handleThemeEvent = (event: Event) => {
    const detail = event instanceof CustomEvent ? event.detail as { theme?: string } : null
    syncPublishedTheme(detail?.theme)
  }

  window.addEventListener('storage', handleStorage)
  window.addEventListener(THEME_EVENT, handleThemeEvent)
  let unsubscribeWailsTheme = () => {}
  try {
    if (typeof Events?.On === 'function') {
      unsubscribeWailsTheme = Events.On(THEME_EVENT, (event) => {
        const data = event?.data as { theme?: string } | undefined
        syncPublishedTheme(data?.theme)
      })
    }
  } catch {
    unsubscribeWailsTheme = () => {}
  }

  return () => {
    window.removeEventListener('storage', handleStorage)
    window.removeEventListener(THEME_EVENT, handleThemeEvent)
    unsubscribeWailsTheme()
  }
}

function normalizeTheme(theme: string | undefined): ThemePreference {
  if (theme === 'professional-pink' || theme === 'light-graphite' || theme === 'cloud-blue') {
    return theme
  }
  return 'cloud-blue'
}

async function syncWindowBackground(theme: ThemePreference) {
  try {
    if (
      document.documentElement.classList.contains('launcher-document') ||
      document.documentElement.classList.contains('pinned-image-document') ||
      document.documentElement.classList.contains('capture-overlay-document')
    ) {
      return
    }
    if (lastAppliedWindowBg === theme) {
      return
    }
    lastAppliedWindowBg = theme
    if (theme === 'professional-pink') {
      await Window.SetBackgroundColour(251, 247, 249, 255)
    } else if (theme === 'light-graphite') {
      await Window.SetBackgroundColour(246, 247, 249, 255)
    } else {
      await Window.SetBackgroundColour(246, 250, 255, 255)
    }
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}
