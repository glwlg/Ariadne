import { Window } from '@wailsio/runtime'

export const launcherGeometry = {
  width: 860,
  collapsedHeight: 96,
  expandedHeight: 468,
} as const

type LauncherScreen = {
  WorkArea?: {
    Width?: number
    Height?: number
  }
  Bounds?: {
    Width?: number
    Height?: number
  }
  Size?: {
    Width?: number
    Height?: number
  }
}

export function launcherWindowSize(expanded: boolean) {
  return {
    width: launcherGeometry.width,
    height: expanded ? launcherGeometry.expandedHeight : launcherGeometry.collapsedHeight,
  }
}

export function reservedLauncherPosition(screen: LauncherScreen | null | undefined) {
  const width = validDimension(screen?.WorkArea?.Width) ?? validDimension(screen?.Bounds?.Width) ?? validDimension(screen?.Size?.Width)
  const height = validDimension(screen?.WorkArea?.Height) ?? validDimension(screen?.Bounds?.Height) ?? validDimension(screen?.Size?.Height)
  if (width === null || height === null) {
    return null
  }
  return {
    x: Math.max(0, Math.floor((width - launcherGeometry.width) / 2)),
    y: Math.max(0, Math.floor((height - launcherGeometry.expandedHeight) / 2)),
  }
}

export async function applyLauncherWindowGeometry(
  expanded: boolean,
  options: { reservePosition?: boolean; restore?: boolean } = {},
) {
  const size = launcherWindowSize(expanded)
  try {
    if (options.restore) {
      await Window.Restore()
    }
    await Window.SetFrameless(true)
    await Window.SetAlwaysOnTop(false)
    await Window.SetBackgroundColour(0, 0, 0, 0)
    await Window.SetSize(size.width, size.height)
    if (options.reservePosition) {
      const position = reservedLauncherPosition(await Window.GetScreen())
      if (position) {
        await Window.SetRelativePosition(position.x, position.y)
      }
    }
  } catch {
    // Runtime calls are unavailable in browser-only dev mode.
  }
}

function validDimension(value: number | undefined) {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null
}
