import type { ProcessNetworkTraffic } from '../types/ariadne'

export type ProcessNetworkSortKey = 'download' | 'upload'
export type SortDirection = 'ascending' | 'descending'

export function sortProcessTraffic(
  processes: readonly ProcessNetworkTraffic[],
  key: ProcessNetworkSortKey,
  direction: SortDirection,
) {
  const field = key === 'download' ? 'downloadBytesPerSecond' : 'uploadBytesPerSecond'
  const multiplier = direction === 'descending' ? -1 : 1
  return [...processes].sort((left, right) =>
    (left[field] - right[field]) * multiplier
    || left.name.localeCompare(right.name)
    || left.pid - right.pid,
  )
}
