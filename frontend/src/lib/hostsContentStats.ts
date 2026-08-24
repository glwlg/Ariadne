export interface HostsContentStats {
  totalLines: number
  validRules: number
  commentLines: number
  emptyLines: number
  duplicateRules: number
}

export function calculateHostsContentStats(content: string): HostsContentStats {
  if (!content) {
    return { totalLines: 0, validRules: 0, commentLines: 0, emptyLines: 0, duplicateRules: 0 }
  }

  const lines = content.replace(/\r\n?/g, '\n').split('\n')
  const seenHosts = new Set<string>()
  const stats: HostsContentStats = {
    totalLines: lines.length,
    validRules: 0,
    commentLines: 0,
    emptyLines: 0,
    duplicateRules: 0,
  }

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) {
      stats.emptyLines += 1
      continue
    }
    if (trimmed.startsWith('#')) {
      stats.commentLines += 1
      continue
    }
    const rule = trimmed.split('#', 1)[0]?.trim() ?? ''
    const fields = rule.split(/\s+/).filter(Boolean)
    if (fields.length < 2 || !isIpAddress(fields[0] ?? '')) continue

    stats.validRules += 1
    for (const host of fields.slice(1)) {
      const normalized = host.toLowerCase()
      if (seenHosts.has(normalized)) stats.duplicateRules += 1
      else seenHosts.add(normalized)
    }
  }
  return stats
}

function isIpAddress(value: string) {
  if (value.includes(':')) {
    return /^[0-9a-f:.]+$/i.test(value) && value.includes(':')
  }
  const parts = value.split('.')
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255)
}
