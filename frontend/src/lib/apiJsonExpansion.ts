export function defaultJsonNodeExpanded(value: unknown): boolean {
  return value !== null && typeof value === 'object'
}
