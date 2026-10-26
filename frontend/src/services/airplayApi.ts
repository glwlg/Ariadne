import type { ActionResult } from '../types/ariadne'
import type { AirPlayCommand, AirPlayStatus } from '../types/airplay'

async function binding() {
  // @ts-expect-error Wails generates JavaScript bindings without TypeScript declarations.
  return await import('../../bindings/ariadne/internal/airplay/service.js')
}

export async function getAirPlayStatus(): Promise<AirPlayStatus> {
  return await (await binding()).Status()
}

export async function executeAirPlay(command: AirPlayCommand): Promise<ActionResult> {
  return await (await binding()).Execute(command)
}

export async function getAirPlayLogs(): Promise<string> {
  return await (await binding()).Logs()
}

export async function getAirPlayDiagnostics(): Promise<string> {
  return await (await binding()).Diagnostics()
}
