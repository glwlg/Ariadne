import { Window } from '@wailsio/runtime'
import { getSettings } from '../services/settingsApi'

export type ThemePreference = 'light' | 'dark'

let currentTheme: ThemePreference = 'light'

export function applyTheme(theme: string | undefined) {
  currentTheme = normalizeTheme(theme)
  const useDark = currentTheme === 'dark'
  document.documentElement.classList.toggle('dark', useDark)
  document.documentElement.dataset.theme = currentTheme
  void syncWindowBackground(useDark)
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
  return () => {}
}

function normalizeTheme(theme: string | undefined): ThemePreference {
  return theme === 'dark' ? 'dark' : 'light'
}

async function syncWindowBackground(useDark: boolean) {
  try {
    if (
      document.documentElement.classList.contains('launcher-document') ||
      document.documentElement.classList.contains('pinned-image-document')
    ) {
      await Window.SetBackgroundColour(0, 0, 0, 0)
      return
    }
    if (useDark) {
      await Window.SetBackgroundColour(9, 9, 11, 255)
    } else {
      await Window.SetBackgroundColour(244, 244, 245, 255)
    }
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}
