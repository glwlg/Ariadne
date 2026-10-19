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
}

export type AirPlayCommand = 'start' | 'stop' | 'firewall' | 'control' | 'fps 30' | 'fps 60'
