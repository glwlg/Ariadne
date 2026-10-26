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
  onOpen: () => void = () => {},
): () => void {
  let stopped = false
  const socket = createStream(NETWORK_TELEMETRY_STREAM)

  function stop() {
    if (stopped) return
    stopped = true
    socket.close(1000, 'view closed')
  }

  function fail(message: string) {
    if (stopped) return
    stop()
    onError(message)
  }

  socket.onopen = () => {
    if (!stopped) {
      try {
        socket.send({ mode })
        onOpen()
      } catch (error) {
        fail(error instanceof Error ? error.message : String(error))
      }
    }
  }
  socket.onmessage = (event) => {
    if (!stopped && event.data && typeof event.data === 'object') {
      onFrame(event.data as NetworkTelemetryFrame)
    }
  }
  socket.onerror = () => fail('网络实时数据连接失败')
  socket.onclose = () => fail('网络实时数据连接已断开')
  return stop
}
