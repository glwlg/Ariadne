import assert from 'node:assert/strict'
import { sortProcessTraffic } from '../src/lib/processNetworkSort.ts'
import type { ProcessNetworkTraffic } from '../src/types/ariadne.ts'

const process = (pid: number, name: string, download: number, upload: number): ProcessNetworkTraffic => ({
  pid,
  name,
  downloadBytesPerSecond: download,
  uploadBytesPerSecond: upload,
  bytesSent: 0,
  bytesReceived: 0,
  connections: [],
})

const input = [process(1, 'Alpha', 20, 2), process(2, 'Beta', 10, 30), process(3, 'Gamma', 20, 1)]

assert.deepEqual(sortProcessTraffic(input, 'download', 'descending').map(({ pid }) => pid), [1, 3, 2])
assert.deepEqual(sortProcessTraffic(input, 'download', 'ascending').map(({ pid }) => pid), [2, 1, 3])
assert.deepEqual(sortProcessTraffic(input, 'upload', 'descending').map(({ pid }) => pid), [2, 1, 3])
assert.deepEqual(input.map(({ pid }) => pid), [1, 2, 3])

console.log('process network sorting checks passed')
