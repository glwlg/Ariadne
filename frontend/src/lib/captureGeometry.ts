export interface VisualRect {
  left: number
  top: number
  width: number
  height: number
}

export interface Size {
  width: number
  height: number
}

export interface BoundsRect extends Size {
  left: number
  top: number
}

export interface ScreenBounds extends Size {
  x: number
  y: number
}

export interface SourcePixelRect {
  x: number
  y: number
  width: number
  height: number
}

export interface Point {
  x: number
  y: number
}

export function mapVisualSelectionToSourcePixels(
  selection: VisualRect,
  sourceSize: Size,
  imageRect: BoundsRect,
  surfaceRect: BoundsRect,
): SourcePixelRect {
  const width = Math.max(1, Math.round(sourceSize.width))
  const height = Math.max(1, Math.round(sourceSize.height))
  const imageWidth = Math.max(1, imageRect.width)
  const imageHeight = Math.max(1, imageRect.height)
  const imageLeftInSurface = imageRect.left - surfaceRect.left
  const imageTopInSurface = imageRect.top - surfaceRect.top
  const localLeft = selection.left - imageLeftInSurface
  const localTop = selection.top - imageTopInSurface
  const scaleX = width / imageWidth
  const scaleY = height / imageHeight

  const left = clampInt(Math.floor(localLeft * scaleX), 0, width - 1)
  const top = clampInt(Math.floor(localTop * scaleY), 0, height - 1)
  const right = clampInt(Math.ceil((localLeft + selection.width) * scaleX), left + 1, width)
  const bottom = clampInt(Math.ceil((localTop + selection.height) * scaleY), top + 1, height)

  return {
    x: left,
    y: top,
    width: right - left,
    height: bottom - top,
  }
}

export function mapVisualSelectionToPinPosition(
  selection: Pick<VisualRect, 'left' | 'top'>,
  displayBounds: Partial<ScreenBounds> | null | undefined,
  imageRect: BoundsRect,
  surfaceRect: BoundsRect,
  offset = 15,
): Point {
  const imageWidth = Math.max(1, imageRect.width)
  const imageHeight = Math.max(1, imageRect.height)
  const imageLeftInSurface = imageRect.left - surfaceRect.left
  const imageTopInSurface = imageRect.top - surfaceRect.top
  const localLeft = clampFloat(selection.left - imageLeftInSurface, 0, imageWidth)
  const localTop = clampFloat(selection.top - imageTopInSurface, 0, imageHeight)
  const scaleX = Math.max(1, displayBounds?.width ?? imageWidth) / imageWidth
  const scaleY = Math.max(1, displayBounds?.height ?? imageHeight) / imageHeight
  return {
    x: Math.round((displayBounds?.x ?? 0) + localLeft * scaleX - offset),
    y: Math.round((displayBounds?.y ?? 0) + localTop * scaleY - offset),
  }
}

function clampInt(value: number, min: number, max: number) {
  const lower = Math.min(min, max)
  const upper = Math.max(min, max)
  return Math.max(lower, Math.min(upper, value))
}

function clampFloat(value: number, min: number, max: number) {
  const lower = Math.min(min, max)
  const upper = Math.max(min, max)
  return Math.max(lower, Math.min(upper, value))
}
