import type { NetworkAdapterTraffic, NetworkTrafficSnapshot, ProcessNetworkSnapshot, ProcessNetworkTraffic } from '../types/ariadne'

async function tryNetworkMonitorBinding() {
  try {
    // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
    return await import('../../bindings/ariadne/internal/networkmonitor/service.js')
  } catch {
    return null
  }
}

export async function getNetworkTrafficSnapshot(): Promise<NetworkTrafficSnapshot> {
  const binding = await tryNetworkMonitorBinding()
  if (binding) {
    try {
      return normalizeSnapshot(await binding.Snapshot())
    } catch (error) {
      return { ...fallbackSnapshot(), lastError: error instanceof Error ? error.message : String(error) }
    }
  }
  return fallbackSnapshot()
}

export async function getProcessNetworkSnapshot(): Promise<ProcessNetworkSnapshot> {
  const binding = await tryNetworkMonitorBinding()
  if (!binding?.ProcessSnapshot) {
    return emptyProcessSnapshot('进程网络统计仅在 Ariadne 桌面端可用')
  }
  try {
    return normalizeProcessSnapshot(await binding.ProcessSnapshot())
  } catch (error) {
    return emptyProcessSnapshot(error instanceof Error ? error.message : String(error))
  }
}

function normalizeProcessSnapshot(snapshot: ProcessNetworkSnapshot): ProcessNetworkSnapshot {
  const processes = Array.isArray(snapshot.processes) ? snapshot.processes.map(normalizeProcessTraffic) : []
  return {
    timestampUnix: Number(snapshot.timestampUnix ?? Math.floor(Date.now() / 1000)),
    processCount: Number(snapshot.processCount ?? processes.length),
    connectionCount: Number(snapshot.connectionCount ?? processes.reduce((total, item) => total + item.connections.length, 0)),
    uploadBytesPerSecond: Number(snapshot.uploadBytesPerSecond ?? 0),
    downloadBytesPerSecond: Number(snapshot.downloadBytesPerSecond ?? 0),
    processes,
    lastError: snapshot.lastError || '',
  }
}

function normalizeProcessTraffic(process: ProcessNetworkTraffic): ProcessNetworkTraffic {
  return {
    pid: Number(process.pid ?? 0),
    name: String(process.name ?? ''),
    path: String(process.path ?? ''),
    iconUrl: String(process.iconUrl ?? ''),
    uploadBytesPerSecond: Number(process.uploadBytesPerSecond ?? 0),
    downloadBytesPerSecond: Number(process.downloadBytesPerSecond ?? 0),
    bytesSent: Number(process.bytesSent ?? 0),
    bytesReceived: Number(process.bytesReceived ?? 0),
    connections: Array.isArray(process.connections)
      ? process.connections.map((connection) => ({
          localAddress: String(connection.localAddress ?? ''),
          remoteAddress: String(connection.remoteAddress ?? ''),
          uploadBytesPerSecond: Number(connection.uploadBytesPerSecond ?? 0),
          downloadBytesPerSecond: Number(connection.downloadBytesPerSecond ?? 0),
          bytesSent: Number(connection.bytesSent ?? 0),
          bytesReceived: Number(connection.bytesReceived ?? 0),
        }))
      : [],
  }
}

function emptyProcessSnapshot(message: string): ProcessNetworkSnapshot {
  return {
    timestampUnix: Math.floor(Date.now() / 1000),
    processCount: 0,
    connectionCount: 0,
    uploadBytesPerSecond: 0,
    downloadBytesPerSecond: 0,
    processes: [],
    lastError: message,
  }
}

function normalizeSnapshot(snapshot: NetworkTrafficSnapshot): NetworkTrafficSnapshot {
  const adapters = Array.isArray(snapshot.adapters) ? snapshot.adapters.map(normalizeAdapter) : []
  return {
    timestampUnix: Number(snapshot.timestampUnix ?? Math.floor(Date.now() / 1000)),
    adapterCount: Number(snapshot.adapterCount ?? adapters.length),
    activeAdapterCount: Number(snapshot.activeAdapterCount ?? adapters.filter((item) => item.operational).length),
    bytesSent: Number(snapshot.bytesSent ?? 0),
    bytesReceived: Number(snapshot.bytesReceived ?? 0),
    uploadBytesPerSecond: Number(snapshot.uploadBytesPerSecond ?? 0),
    downloadBytesPerSecond: Number(snapshot.downloadBytesPerSecond ?? 0),
    adapters,
    lastError: snapshot.lastError || '',
  }
}

function normalizeAdapter(adapter: NetworkAdapterTraffic): NetworkAdapterTraffic {
  return {
    name: String(adapter.name ?? ''),
    alias: String(adapter.alias ?? ''),
    description: String(adapter.description ?? ''),
    interfaceIndex: Number(adapter.interfaceIndex ?? 0),
    operational: Boolean(adapter.operational),
    transmitLinkBitsPerSec: Number(adapter.transmitLinkBitsPerSec ?? 0),
    receiveLinkBitsPerSec: Number(adapter.receiveLinkBitsPerSec ?? 0),
    bytesSent: Number(adapter.bytesSent ?? 0),
    bytesReceived: Number(adapter.bytesReceived ?? 0),
    uploadBytesPerSecond: Number(adapter.uploadBytesPerSecond ?? 0),
    downloadBytesPerSecond: Number(adapter.downloadBytesPerSecond ?? 0),
  }
}

function fallbackSnapshot(): NetworkTrafficSnapshot {
  return {
    timestampUnix: Math.floor(Date.now() / 1000),
    adapterCount: 1,
    activeAdapterCount: 1,
    bytesSent: 128 * 1024 * 1024,
    bytesReceived: 512 * 1024 * 1024,
    uploadBytesPerSecond: 24 * 1024,
    downloadBytesPerSecond: 164 * 1024,
    adapters: [
      {
        name: 'Development adapter',
        alias: 'Development adapter',
        description: 'Frontend fallback sample',
        interfaceIndex: 1,
        operational: true,
        transmitLinkBitsPerSec: 1_000_000_000,
        receiveLinkBitsPerSec: 1_000_000_000,
        bytesSent: 128 * 1024 * 1024,
        bytesReceived: 512 * 1024 * 1024,
        uploadBytesPerSecond: 24 * 1024,
        downloadBytesPerSecond: 164 * 1024,
      },
    ],
  }
}
