export interface HostsContentStats {
  totalLines: number
  validRules: number
  commentLines: number
  emptyLines: number
  duplicateRules: number
}

export function countHostsLines(content: string): number {
  if (!content) return 0
  let lines = 1
  for (let i = 0; i < content.length; i += 1) {
    const code = content.charCodeAt(i)
    if (code === 10) {
      lines += 1
    } else if (code === 13) {
      lines += 1
      if (content.charCodeAt(i + 1) === 10) i += 1
    }
  }
  return lines
}

export function calculateHostsContentStats(content: string): HostsContentStats {
  if (!content) {
    return { totalLines: 0, validRules: 0, commentLines: 0, emptyLines: 0, duplicateRules: 0 }
  }

  const seenHosts = new Set<string>()
  const stats: HostsContentStats = {
    totalLines: 0,
    validRules: 0,
    commentLines: 0,
    emptyLines: 0,
    duplicateRules: 0,
  }

  const processLine = (rawLine: string) => {
    stats.totalLines += 1
    const trimmed = rawLine.trim()
    if (!trimmed) {
      stats.emptyLines += 1
      return
    }
    if (trimmed.startsWith('#')) {
      stats.commentLines += 1
      return
    }
    const commentIndex = trimmed.indexOf('#')
    const rule = (commentIndex >= 0 ? trimmed.slice(0, commentIndex) : trimmed).trim()
    const fields = rule.split(/\s+/).filter(Boolean)
    if (fields.length < 2 || !isIpAddress(fields[0] ?? '')) return

    stats.validRules += 1
    for (let i = 1; i < fields.length; i += 1) {
      const normalized = fields[i]!.toLowerCase()
      if (seenHosts.has(normalized)) stats.duplicateRules += 1
      else seenHosts.add(normalized)
    }
  }

  let lineStart = 0
  for (let i = 0; i < content.length; i += 1) {
    const code = content.charCodeAt(i)
    if (code === 10 || code === 13) {
      processLine(content.slice(lineStart, i))
      if (code === 13 && content.charCodeAt(i + 1) === 10) {
        i += 1
      }
      lineStart = i + 1
    }
  }
  processLine(content.slice(lineStart))

  return stats
}

function isIpAddress(value: string) {
  if (value.includes(':')) {
    return /^[0-9a-f:.]+$/i.test(value) && value.includes(':')
  }
  const parts = value.split('.')
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255)
}
