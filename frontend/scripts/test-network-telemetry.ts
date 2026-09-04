import assert from 'node:assert/strict'
import {
  connectNetworkTelemetry,
  NETWORK_TELEMETRY_STREAM,
  type NetworkTelemetrySocket,
} from '../src/services/networkTelemetry.ts'

function fakeSocket() {
  const sent: unknown[] = []
  let closed = false
  const socket: NetworkTelemetrySocket = {
    readyState: 0,
    onopen: null,
    onmessage: null,
    onerror: null,
    onclose: null,
    send(value: unknown) {
      sent.push(value)
    },
    close() {
      closed = true
    },
  }
  return { socket, sent, isClosed: () => closed }
}

const trafficSocket = fakeSocket()
let openedName = ''
let trafficFrame: unknown = null
const stopTraffic = connectNetworkTelemetry(
  'traffic',
  (frame) => {
    trafficFrame = frame
  },
  () => assert.fail('stream should not fail'),
  (name) => {
    openedName = name
    return trafficSocket.socket
  },
)
assert.equal(openedName, NETWORK_TELEMETRY_STREAM)
trafficSocket.socket.onopen?.({} as Event)
assert.deepEqual(trafficSocket.sent, [{ mode: 'traffic' }])
const expectedFrame = { traffic: { bytesSent: 42 } }
trafficSocket.socket.onmessage?.({ data: expectedFrame } as MessageEvent)
assert.deepEqual(trafficFrame, expectedFrame)
stopTraffic()
assert.equal(trafficSocket.isClosed(), true)

const processSocket = fakeSocket()
let failed = false
connectNetworkTelemetry(
  'processes',
  () => assert.fail('unexpected frame'),
  () => {
    failed = true
  },
  () => processSocket.socket,
)
processSocket.socket.onopen?.({} as Event)
assert.deepEqual(processSocket.sent, [{ mode: 'processes' }])
processSocket.socket.onerror?.({} as Event)
assert.equal(failed, true)

console.log('network telemetry stream client checks passed')
