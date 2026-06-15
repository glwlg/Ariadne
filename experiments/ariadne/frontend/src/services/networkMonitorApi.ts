import type { NetworkAdapterTraffic, NetworkTrafficSnapshot } from '../types/ariadne'

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
