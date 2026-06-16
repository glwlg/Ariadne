import type { OCRLine } from '../types/ariadne'

export function ocrConfidenceLabel(value: number) {
  if (!value || Number.isNaN(value)) return '置信度 -'
  const percent = value <= 1 ? value * 100 : value
  return `置信度 ${Math.round(percent)}%`
}

export function ocrRectLabel(line: OCRLine) {
  const rect = line.rect
  if (!rect || rect.width <= 0 || rect.height <= 0) return '未提供位置'
  return `${rect.x},${rect.y} · ${rect.width}x${rect.height}`
}
