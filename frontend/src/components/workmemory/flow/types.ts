import type { ExperienceInsight, WorkMemoryEntry, WorkMemoryFlowAskResponse } from '../../../types/ariadne'

export type FlowPage = 'flow' | 'timeline' | 'insights' | 'drafts' | 'assets' | 'rules'
export type TimelineSourceFilter = 'all' | 'screenshots' | 'clipboard' | 'notes' | 'ocr'
export type FlowSettingsTab = 'capture' | 'model' | 'privacy'
export type FlowChatRole = 'user' | 'assistant'
export type DraftKind = 'daily' | 'retrospective' | 'knowledge'

export interface FlowChatMessage {
  id: string
  role: FlowChatRole
  text: string
  createdAt: number
  question?: string
  result?: WorkMemoryFlowAskResponse
  pending?: boolean
  error?: boolean
  system?: boolean
}

export interface CaptureAppCandidate {
  id: string
  displayName: string
  processName: string
  count: number
}

export interface TimelineAppOption {
  id: string
  label: string
  count: number
}

export interface TimelineDayGroup {
  id: string
  label: string
  note: string
  entries: WorkMemoryEntry[]
}

export interface FlowCanvasEntry {
  message: FlowChatMessage
  conclusion: string
  evidenceEntries: WorkMemoryEntry[]
  uncertainty: string[]
  recommendedActions: string[]
}

export interface TimelineLane {
  key: string
  label: string
  appName: string
  entries: WorkMemoryEntry[]
}

export interface TimelineAxisTick {
  left: number
  label: string
}

export interface InsightMapNode {
  insight: ExperienceInsight
  angle: number
  radius: number
}

export interface DraftKindItem {
  kind: DraftKind
  label: string
  title: string
  icon: string
  emptyHint: string
}
