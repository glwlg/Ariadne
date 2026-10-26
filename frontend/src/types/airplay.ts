export interface AirPlayStatus {
  ready: boolean
  controlReady: boolean
  running: boolean
  receiverName: string
  pin: string
  processId: number
  logPath: string
  lastError: string
  phase: string
  frameRate: number
  width: number
  height: number
  contentTitle: string
  clientName: string
  control: AirPlayControlStatus
}

export interface AirPlayControlStatus {
  phase: string
  running: boolean
  processId: number
  advertising: boolean
  connectedDevices: number
  targetName: string
  readiness: number
  lastError: string
  diagnostics: string
}

export type AirPlayCommand = 'start' | 'stop' | 'firewall' | 'control' | 'fps 30' | 'fps 60'
