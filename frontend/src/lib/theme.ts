import { Window } from '@wailsio/runtime'
import { getSettings } from '../services/settingsApi'

export type ThemePreference = 'light' | 'professional-pink' | 'light-graphite' | 'cloud-blue' | 'dark'

const THEME_STORAGE_KEY = 'ariadne:theme-preference'
const THEME_EVENT = 'ariadne:theme-changed'

let currentTheme: ThemePreference = 'light'

export function applyTheme(theme: string | undefined) {
  currentTheme = normalizeTheme(theme)
  const useDark = currentTheme === 'dark'
  document.documentElement.classList.toggle('dark', useDark)
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
}

export async function syncThemeFromSettings() {
  try {
    const settings = await getSettings()
    applyTheme(settings.general.theme)
  } catch {
    applyTheme('light')
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
  const handleFocus = () => {
    void syncThemeFromSettings()
  }

  window.addEventListener('storage', handleStorage)
  window.addEventListener(THEME_EVENT, handleThemeEvent)
  window.addEventListener('focus', handleFocus)

  return () => {
    window.removeEventListener('storage', handleStorage)
    window.removeEventListener(THEME_EVENT, handleThemeEvent)
    window.removeEventListener('focus', handleFocus)
  }
}

function normalizeTheme(theme: string | undefined): ThemePreference {
  if (theme === 'professional-pink' || theme === 'light-graphite' || theme === 'cloud-blue' || theme === 'dark') {
    return theme
  }
  return 'light'
}

async function syncWindowBackground(theme: ThemePreference) {
  try {
    if (
      document.documentElement.classList.contains('launcher-document') ||
      document.documentElement.classList.contains('pinned-image-document')
    ) {
      await Window.SetBackgroundColour(0, 0, 0, 0)
      return
    }
    if (theme === 'dark') {
      await Window.SetBackgroundColour(9, 9, 11, 255)
    } else if (theme === 'professional-pink') {
      await Window.SetBackgroundColour(251, 247, 249, 255)
    } else if (theme === 'light-graphite') {
      await Window.SetBackgroundColour(246, 247, 249, 255)
    } else if (theme === 'cloud-blue') {
      await Window.SetBackgroundColour(246, 250, 255, 255)
    } else {
      await Window.SetBackgroundColour(244, 244, 245, 255)
    }
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}
