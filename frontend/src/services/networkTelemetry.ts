import { JSONStream } from '@wailsio/runtime'
import type { NetworkTrafficSnapshot, ProcessNetworkSnapshot } from '../types/ariadne'

export const NETWORK_TELEMETRY_STREAM = 'network-telemetry'

export type NetworkTelemetryMode = 'traffic' | 'processes'

export interface NetworkTelemetryFrame {
  traffic?: NetworkTrafficSnapshot
  processes?: ProcessNetworkSnapshot
}

export interface NetworkTelemetrySocket {
  readyState: number
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent) => void) | null
  onerror: ((event: Event) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  send(value: unknown): void
  close(code?: number, reason?: string): void
}

type StreamFactory = (name: string) => NetworkTelemetrySocket

const defaultStreamFactory: StreamFactory = (name) => JSONStream(name) as NetworkTelemetrySocket

export function connectNetworkTelemetry(
  mode: NetworkTelemetryMode,
  onFrame: (frame: NetworkTelemetryFrame) => void,
  onError: (message: string) => void,
  createStream: StreamFactory = defaultStreamFactory,
): () => void {
  let stopped = false
  let socket: NetworkTelemetrySocket
  try {
    socket = createStream(NETWORK_TELEMETRY_STREAM)
  } catch (error) {
    onError(error instanceof Error ? error.message : String(error))
    return () => {}
  }

  socket.onopen = () => {
    if (!stopped) {
      socket.send({ mode })
    }
  }
  socket.onmessage = (event) => {
    if (!stopped && event.data && typeof event.data === 'object') {
      onFrame(event.data as NetworkTelemetryFrame)
    }
  }
  socket.onerror = () => {
    if (!stopped) {
      onError('网络实时数据连接失败')
    }
  }
  socket.onclose = () => {
    if (!stopped) {
      onError('网络实时数据连接已断开')
    }
  }

  return () => {
    if (stopped) return
    stopped = true
    socket.close(1000, 'view closed')
  }
}
