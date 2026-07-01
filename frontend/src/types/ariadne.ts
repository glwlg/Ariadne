export type SearchResultType =
  | 'file'
  | 'app'
  | 'plugin_trigger'
  | 'plugin_result'
  | 'workflow'
  | 'clipboard'
  | 'memory'
  | 'command'
  | 'capture'
  | 'settings'

export type PreviewActionKind =
  | 'open'
  | 'open_parent'
  | 'copy'
  | 'pin'
  | 'run'
  | 'plugin'
  | 'remember'
  | 'danger'

export type PreviewKind = 'text' | 'memory' | 'image' | 'settings' | 'workflow'

export interface PreviewAction {
  id: string
  label: string
  icon?: string
  shortcut?: string
  kind: PreviewActionKind
  payload?: Record<string, unknown>
  feedback?: {
    successLabel?: string
    durationMs?: number
  }
}

export interface PreviewDescriptor {
  kind: PreviewKind
  title: string
  subtitle?: string
  text?: string
  meta?: Array<{ label: string; value: string }>
  evidence?: Array<{ label: string; value: string }>
  imageHint?: string
}

export interface CommandParam {
  name: string
  label: string
  placeholder: string
  required: boolean
}

export interface CommandSchema {
  usage: string
  examples?: string[]
  params?: CommandParam[]
}

export interface SearchResult {
  id: string
  type: SearchResultType
  title: string
  subtitle?: string
  detail?: string
  icon: string
  score?: number
  tags?: string[]
  payload?: Record<string, unknown>
  preview: PreviewDescriptor
  actions: PreviewAction[]
}

export interface ActionResult {
  ok: boolean
  message: string
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface PinnedImageOpenResult {
  ok: boolean
  message: string
  pinId?: string
  title?: string
  width?: number
  height?: number
}

export interface PinnedImage {
  id: string
  source: 'capture' | 'clipboard' | 'qr' | string
  sourceId?: string
  title: string
  imagePath?: string
  text?: string
  dataUrl: string
  width: number
  height: number
  bytes: number
  createdAt: number
  windowWidth: number
  windowHeight: number
  windowX?: number
  windowY?: number
  positioned?: boolean
  canCopy: boolean
  copyAction?: string
  canOcr: boolean
}

export interface ScreenBounds {
  x: number
  y: number
  width: number
  height: number
}

export interface CaptureOverlayOpenResult {
  ok: boolean
  message: string
  sessionId?: string
  bounds?: ScreenBounds
  nativeBounds?: ScreenBounds
}

export interface CaptureOverlaySession {
  id: string
  bounds: ScreenBounds
  nativeBounds?: ScreenBounds
  imageUrl: string
  createdAt: number
}

export interface CaptureOverlaySelectionRequest {
  sessionId: string
  x: number
  y: number
  width: number
  height: number
  coordinateSpace?: 'visual' | 'session' | 'native'
  displayWidth?: number
  displayHeight?: number
  action: 'capture' | 'copy' | 'redact_copy' | 'pin' | 'qr' | 'save_as'
  savedPath?: string
  pinPositioned?: boolean
  pinX?: number
  pinY?: number
  operations?: CaptureOverlayAnnotationOperation[]
  renderedImage?: string
}

export interface CaptureOverlayAnnotationPoint {
  x: number
  y: number
}

export interface CaptureOverlayAnnotationOperation {
  kind: 'rect' | 'line' | 'arrow' | 'pen' | 'highlight' | 'mosaic' | 'text' | 'number' | 'eraser'
  x: number
  y: number
  width?: number
  height?: number
  endX?: number
  endY?: number
  color?: string
  strokeWidth?: number
  pixelSize?: number
  points?: CaptureOverlayAnnotationPoint[]
  text?: string
  fontSize?: number
  number?: number
}

export interface CaptureOverlayResult {
  ok: boolean
  message: string
  captureId?: string
  imagePath?: string
  savedPath?: string
  width?: number
  height?: number
  qr?: QRScanResult
  pin?: PinnedImageOpenResult
}

export interface ImageIndexRequest {
  sources?: Array<'capture_history' | 'clipboard_history' | string>
  limit?: number
  force?: boolean
}

export interface ImageIndexEntry {
  id: string
  source: string
  sourceId: string
  imagePath: string
  text?: string
  provider?: string
  indexedAt: number
  width?: number
  height?: number
  ok: boolean
  sensitive: boolean
  redacted: boolean
  error?: string
}

export interface ImageIndexBatchResult {
  ok: boolean
  startedAt: number
  finishedAt: number
  indexed: number
  skipped: number
  failed: number
  lastError?: string
  entries: ImageIndexEntry[]
}

export interface SearchResponse {
  query: string
  results: SearchResult[]
  elapsedMs: number
}

export interface SearchUsageRecord {
  resultId: string
  favorite: boolean
  useCount: number
  lastUsedAt: number
}

export interface SearchUsageStatus {
  path: string
  count: number
  records: SearchUsageRecord[]
}

export interface SearchUsageClearResult {
  ok: boolean
  message: string
  cleared: number
  status: SearchUsageStatus
}

export interface PlatformCapability {
  id: string
  enabled: boolean
  provider: string
  note?: string
}

export interface RuntimeDiagnostics {
  os: string
  arch: string
  goVersion: string
  processId: number
  workingDir: string
  executablePath: string
  executableBytes: number
  appDataEnv: string
  localAppDataEnv: string
  goToolPath?: string
  wailsToolPath?: string
}

export interface LegacyRuntimeStatus {
  processRunning: boolean
  processId?: number
  processName?: string
  processPath?: string
  configPath: string
  configExists: boolean
  configBytes?: number
  hotkeyConflictLikely: boolean
  notes?: string[]
}

export interface ShellRuntimeStatus {
  singleInstanceConfigured: boolean
  trayConfigured: boolean
  globalHotkeyRegistered: boolean
  globalHotkey: string
  screenshotHotkeyRegistered: boolean
  screenshotHotkey: string
  pinClipboardHotkeyRegistered: boolean
  pinClipboardHotkey: string
  autostartSupported: boolean
  autostartEnabled: boolean
  autostartPath: string
  autostartIdentifier?: string
  autostartValueName?: string
  autostartCommand?: string
  autostartCommandValid: boolean
  autostartHiddenArgPresent: boolean
  autostartNotes?: string[]
  lastError: string
}

export interface RuntimeMetric {
  id: string
  label: string
  value: number
  unit: string
}

export interface SearchPerformanceStatus {
  sampleCount: number
  targetP95Ms: number
  lastQuery?: string
  lastElapsedMs: number
  lastResultCount: number
  averageMs: number
  p95Ms: number
  maxMs: number
  withinTarget: boolean
  lastUpdatedAt?: number
}

export interface FileSearchStatus {
  dllPath?: string
  dllFound: boolean
  ready: boolean
  provider?: string
  serviceName?: string
  serviceInstalled: boolean
  serviceRunning: boolean
  serviceState?: string
  serviceError?: string
  indexing: boolean
  indexedCount: number
  volumeCount: number
  requiresAdmin: boolean
  elevated: boolean
  indexStartedAt?: number
  indexFinishedAt?: number
  lastError?: string
  lastQuery?: string
  lastElapsedMs: number
  lastResultCount: number
  lastUpdatedAt?: number
  coverageHint?: string
  policyErrors?: string[]
}

export interface LogStatus {
  path: string
  directory: string
  directoryExists: boolean
  exists: boolean
  bytes: number
  lastModifiedAt?: number
  lastError?: string
}

export interface PlatformStatus {
  appName: string
  legacyName: string
  capabilities: PlatformCapability[]
  diagnostics: RuntimeDiagnostics
  shell: ShellRuntimeStatus
  legacyRuntime: LegacyRuntimeStatus
  searchPerformance: SearchPerformanceStatus
  fileSearch: FileSearchStatus
  logs: LogStatus
  metrics: RuntimeMetric[]
}

export interface DiagnosticsExportResult {
  ok: boolean
  message: string
  path?: string
  bytes?: number
  createdAt?: number
  included?: string[]
  logIncluded: boolean
}

export interface LegacyHandoffRequest {
  confirm: boolean
  force?: boolean
  timeoutMs?: number
}

export interface LegacyHandoffResult {
  ok: boolean
  message: string
  before: LegacyRuntimeStatus
  after: LegacyRuntimeStatus
  shell: ShellRuntimeStatus
  actions: string[]
  requiresConfirmation: boolean
  forceUsed: boolean
  hotkeyRetried: boolean
  createdAt: number
}

export type LauncherKind = 'app' | 'file' | 'folder' | 'url' | 'command'

export interface Launcher {
  id: string
  name: string
  kind: LauncherKind
  target: string
  arguments?: string
  workingDir?: string
  keywords?: string[]
  tags?: string[]
  enabled: boolean
}

export interface LauncherStatus {
  path: string
  count: number
  items: Launcher[]
  lastSaveError?: string
}

export type ClipboardEntryType = 'text' | 'image'

export interface ClipboardHistoryEntry {
  id: string
  type: ClipboardEntryType
  text: string
  imagePath?: string
  thumbnailPath?: string
  thumbnailWidth?: number
  thumbnailHeight?: number
  thumbnailBytes?: number
  createdAt: number
  pinned: boolean
  signature: string
  contentType: string
  source: string
  summary: string
  width?: number
  height?: number
  bytes?: number
  tags?: string[]
}

export interface ClipboardHistoryStatus {
  path: string
  imageDir: string
  thumbnailDir?: string
  count: number
  pinnedCount: number
  imageCount: number
  thumbnailCount?: number
  thumbnailBytes?: number
  lastEntryAt?: number
  lastSaveError?: string
  watcherEnabled: boolean
  watcherRunning: boolean
  lastWatcherAt?: number
  lastWatcherError?: string
  entries?: ClipboardHistoryEntry[]
}

export interface CaptureHistoryEntry {
  id: string
  imagePath: string
  thumbnailPath?: string
  thumbnailWidth?: number
  thumbnailHeight?: number
  thumbnailBytes?: number
  savedPath?: string
  createdAt: number
  source: string
  actions?: string[]
  pinned: boolean
  width: number
  height: number
  bytes: number
  signature: string
  tags?: string[]
}

export interface CaptureHistoryStatus {
  path: string
  imageDir: string
  thumbnailDir?: string
  count: number
  pinnedCount: number
  thumbnailCount?: number
  thumbnailBytes?: number
  lastEntryAt?: number
  lastSaveError?: string
  lastCaptureError?: string
  virtualizedPath?: string
  virtualizedExists: boolean
  virtualizedBytes: number
  virtualizedImageDir?: string
  virtualizedImageCount: number
  virtualizedImageBytes: number
  entries?: CaptureHistoryEntry[]
}

export interface QRScanResult {
  ok: boolean
  text?: string
  format?: string
  source?: string
  captureId?: string
  imagePath?: string
  width?: number
  height?: number
  error?: string
  decodedAt?: number
}

export interface OCRRect {
  x: number
  y: number
  width: number
  height: number
}

export interface OCRLine {
  text: string
  confidence: number
  rect?: OCRRect
}

export interface OCRStatus {
  available: boolean
  provider: string
  mode: string
  pythonPath?: string
  bridgePath?: string
  lastError?: string
  lastRunAt?: number
}

export interface OCRResult {
  ok: boolean
  text?: string
  lines?: OCRLine[]
  source?: string
  captureId?: string
  clipboardId?: string
  memoryId?: string
  imagePath?: string
  width?: number
  height?: number
  provider?: string
  elapsedMs?: number
  sensitive: boolean
  error?: string
  recognizedAt?: number
  workMemory?: WorkMemoryEntry
}

export type HostsProfileType = 'local' | 'remote'

export interface HostsProfile {
  id: string
  title: string
  content: string
  enabled: boolean
  type: HostsProfileType
  url?: string
  system: boolean
  updatedAt?: number
}

export interface HostsConflict {
  host: string
  ips: string[]
}

export interface HostsApplyPreview {
  hostsPath: string
  lineCount: number
  addedLines: number
  removedLines: number
  changed: boolean
  enabledProfiles: string[]
  conflicts: HostsConflict[]
  currentContent: string
  finalContent: string
  diffText: string
  requiresConfirm: boolean
  lastPreviewError?: string
}

export interface HostsApplyResult {
  ok: boolean
  message: string
  requiresConfirm: boolean
  preview: HostsApplyPreview
  lastApplyError?: string
  confirmationCommand?: string
}

export interface HostsStatus {
  configPath: string
  hostsPath: string
  legacyPath: string
  count: number
  enabledCount: number
  systemReadable: boolean
  systemBytes: number
  lastSaveError?: string
  lastReadError?: string
  lastApplyError?: string
  lastRemoteError?: string
  legacyImported: boolean
  profiles: HostsProfile[]
  virtualizedPath?: string
  virtualizedExists: boolean
  virtualizedBytes: number
}

export interface WorkflowStep {
  command: string
  pick?: string
}

export interface WorkflowDefinition {
  id: string
  name: string
  description: string
  steps: WorkflowStep[]
  updatedAt?: number
}

export interface WorkflowStatus {
  path: string
  legacyPath: string
  count: number
  lastSaveError?: string
  legacyImported: boolean
  workflows: WorkflowDefinition[]
}

export interface WorkflowRunRequest {
  workflowId: string
  input?: string
  clipboardText?: string
  confirmed?: boolean
}

export interface WorkflowStepRun {
  index: number
  command: string
  renderedCommand: string
  pick?: string
  pickedTitle?: string
  output?: string
  ok: boolean
  message?: string
}

export interface WorkflowRunResult {
  ok: boolean
  message: string
  workflowId: string
  workflowName: string
  output?: string
  steps: WorkflowStepRun[]
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface WorkflowExportResult {
  ok: boolean
  message: string
  path?: string
  json?: string
  count: number
  bytes?: number
  exportedAt?: number
}

export interface WorkflowImportResult {
  ok: boolean
  message: string
  importedCount: number
  status: WorkflowStatus
}

export interface WorkflowDraftSaveRequest {
  draft: WorkflowDraft
  confirmed?: boolean
}

export interface WorkflowDraftSaveResult {
  ok: boolean
  message: string
  workflow: WorkflowDefinition
  status: WorkflowStatus
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface JsonDifference {
  kind: 'added' | 'removed' | 'changed' | string
  path: string
  left?: unknown
  right?: unknown
}

export interface JsonCompareRequest {
  leftText: string
  rightText: string
  sortKeys: boolean
  maxReportItems?: number
}

export interface JsonCompareResult {
  ok: boolean
  summary: string
  differences: JsonDifference[]
  report: string
  unifiedDiff: string
  leftFormatted: string
  rightFormatted: string
  diffTruncated?: boolean
  differencesTruncated?: boolean
  formattedTruncated?: boolean
  performanceNote?: string
  error?: string
  added: number
  removed: number
  changed: number
}

export interface JsonFormatRequest {
  text: string
  sortKeys: boolean
  label: string
}

export interface JsonFormatResult {
  ok: boolean
  text: string
  error?: string
}

export type APIMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS' | string

export interface APIHeader {
  id: string
  name: string
  value: string
  enabled: boolean
}

export interface APIParam {
  id: string
  name: string
  value: string
  type: 'query' | 'path' | string
  enabled: boolean
}

export interface APIVariable {
  id: string
  name: string
  value: string
  enabled: boolean
  secret?: boolean
}

export interface APIAssertion {
  id: string
  kind: 'status' | 'header' | 'body' | 'json' | 'response_time' | string
  target: string
  operator: 'equals' | 'not_equals' | 'contains' | 'exists' | 'less_than' | 'greater_than' | string
  expected: string
  enabled: boolean
}

export interface APIRequest {
  id: string
  name: string
  folder?: string
  method: APIMethod
  url: string
  bodyType: 'none' | 'json' | 'text' | 'form' | string
  body: string
  params: APIParam[]
  headers: APIHeader[]
  assertions: APIAssertion[]
  updatedAt?: number
}

export interface APIEnvironment {
  id: string
  name: string
  variables: APIVariable[]
  updatedAt?: number
}

export interface APIGitConfig {
  path?: string
  remote?: string
  branch?: string
}

export interface APICollection {
  id: string
  name: string
  variables: APIVariable[]
  environments: APIEnvironment[]
  requests: APIRequest[]
  git?: APIGitConfig
  activeEnvironmentId: string
  activeRequestId: string
  updatedAt?: number
}

export interface APITestingStatus {
  path: string
  databasePath: string
  collections: APICollection[]
  activeCollectionId: string
  collectionCount: number
  requestCount: number
  lastSaveError?: string
  lastLoadError?: string
}

export interface APIImportResult {
  ok: boolean
  message: string
  importedCount: number
  error?: string
  status: APITestingStatus
}

export interface APIGitStatus {
  ok: boolean
  message: string
  collectionId?: string
  path?: string
  remote?: string
  branch?: string
  dirty: boolean
  files?: string[]
  error?: string
}

export interface APIRunRequest {
  collectionId: string
  environmentId: string
  request: APIRequest
  timeoutSeconds?: number
  runId?: string
  stream?: boolean
}

export interface APIRunStopResult {
  ok: boolean
  message: string
}

export interface APIRunSnapshot {
  ok: boolean
  running: boolean
  message: string
  updatedAt?: number
  result: APIRunResult
}

export interface APIAssertionResult {
  id: string
  kind: string
  target: string
  operator: string
  expected: string
  actual: string
  passed: boolean
  message: string
}

export interface APIRunResult {
  ok: boolean
  message: string
  method: string
  requestUrl: string
  statusCode: number
  statusText: string
  durationMs: number
  headers: APIHeader[]
  body: string
  bodySize: number
  bodyTruncated: boolean
  contentType: string
  streaming?: boolean
  assertionResults: APIAssertionResult[]
  passed: number
  failed: number
  error?: string
  missingVariables?: string[]
}

export interface NetworkAdapterTraffic {
  name: string
  alias: string
  description: string
  interfaceIndex: number
  operational: boolean
  transmitLinkBitsPerSec: number
  receiveLinkBitsPerSec: number
  bytesSent: number
  bytesReceived: number
  uploadBytesPerSecond: number
  downloadBytesPerSecond: number
}

export interface NetworkTrafficSnapshot {
  timestampUnix: number
  adapterCount: number
  activeAdapterCount: number
  bytesSent: number
  bytesReceived: number
  uploadBytesPerSecond: number
  downloadBytesPerSecond: number
  adapters: NetworkAdapterTraffic[]
  lastError?: string
}

export interface WorkMemoryStatus {
  enabled: boolean
  timeMachineEnabled: boolean
  workerRunning?: boolean
  privacyMode: boolean
  pauseReason?: string
  autoOcrEnabled?: boolean
  captureScope?: string
  multiMonitor?: string
  pauseOnIdle?: boolean
  idlePauseSeconds?: number
  pauseOnLock?: boolean
  idleSeconds?: number
  lastActivityAt?: number
  sessionLocked?: boolean
  entryCount: number
  autoCaptureIntervalSeconds?: number
  windowSwitchCaptureEnabled?: boolean
  windowSwitchCooldownSeconds?: number
  appCaptureProfiles?: WorkMemoryAppCaptureProfile[]
  lastCaptureAt?: number
  lastCaptureId?: string
  lastCaptureError?: string
  lastSkippedAt?: number
  lastSkippedReason?: string
  lastAutoOcrAt?: number
  lastAutoOcrId?: string
  lastAutoOcrError?: string
  captureCount?: number
  storagePath?: string
}

export interface WorkMemoryHealthAppStat {
  appName: string
  count: number
  pending: number
  checked: number
  ocrDone: number
  qualityOcr?: number
  sensitive: number
  lastSeenAt?: number
}

export interface WorkMemoryHealthRecentEvent {
  id?: string
  kind: string
  title: string
  detail?: string
  appName?: string
  createdAt?: number
}

export interface WorkMemoryHealthSummary {
  ok: boolean
  message: string
  total: number
  today: number
  pending: number
  checked: number
  sensitive: number
  images: number
  multiFrame: number
  collapsedEntries: number
  removedFrames: number
  ocrDone: number
  ocrPending: number
  ocrFailed: number
  qualityOcrDone?: number
  qualityOcrPending?: number
  qualityOcrFailed?: number
  skippedSensitive: number
  skippedPending: number
  lastCaptureAt?: number
  lastQualityCheckAt?: number
  lastAutoOcrAt?: number
  lastSkippedReason?: string
  lastAutoOcrError?: string
  appStats?: WorkMemoryHealthAppStat[]
  recentEvents?: WorkMemoryHealthRecentEvent[]
  generatedAt: number
}

export interface WorkMemorySemanticStatus {
  enabled: boolean
  provider: string
  mode: string
  external: boolean
  ftsEnabled?: boolean
  ftsPath?: string
  indexedEntries: number
  lastIndexedAt?: number
  lastIndexError?: string
  externalEmbeddingReady?: boolean
  externalProvider?: string
  embeddingModel?: string
  embeddingIndexed?: number
  lastEmbeddingAt?: number
  lastEmbeddingError?: string
  vectorStoreType?: string
  vectorStoreUri?: string
  vectorCollection?: string
  note?: string
}

export interface WorkMemoryEmbeddingRefreshResult {
  ok: boolean
  message: string
  status: WorkMemorySemanticStatus
  indexed: number
  skipped: number
  failed: number
  provider?: string
  model?: string
  refreshedAt?: number
  requiresReview?: boolean
}

export interface WorkMemorySemanticSearchResult {
  ok: boolean
  message: string
  query: string
  results: SearchResult[]
  status: WorkMemorySemanticStatus
  provider?: string
  model?: string
}

export interface WorkMemoryFlowAskRequest {
  question: string
  limit?: number
  since?: number
}

export interface WorkMemoryFlowAskEvidence {
  id: string
  title: string
  summary: string
  source: string
  appName?: string
  windowTitle?: string
  createdAt: number
  score?: number
  hasImage: boolean
  sensitive: boolean
  tags: string[]
}

export interface WorkMemoryFlowAskResponse {
  ok: boolean
  question: string
  title: string
  answer: string
  intent: string
  mode: string
  evidence: WorkMemoryFlowAskEvidence[]
  suggestedQuestions?: string[]
  usedAi: boolean
  message?: string
  createdAt: number
}

export interface WorkMemoryFlowConversation {
  id: string
  title: string
  createdAt: number
  updatedAt: number
  messageCount: number
  lastMessage?: string
}

export type WorkMemoryFlowMessageRole = 'user' | 'assistant'

export interface WorkMemoryFlowMessage {
  id: string
  conversationId: string
  role: WorkMemoryFlowMessageRole
  text: string
  question?: string
  result?: WorkMemoryFlowAskResponse
  error?: boolean
  createdAt: number
}

export interface WorkMemoryFlowConversationAskRequest {
  conversationId?: string
  question: string
  limit?: number
  since?: number
}

export interface WorkMemoryFlowConversationAskResult {
  ok: boolean
  message?: string
  conversation: WorkMemoryFlowConversation
  messages: WorkMemoryFlowMessage[]
  response: WorkMemoryFlowAskResponse
}

export type WorkMemorySelfAssertionCategory = 'identity' | 'preference' | 'relationship' | 'boundary'
export type WorkMemorySelfAssertionStatus = 'confirmed' | 'observed' | 'rejected' | 'ephemeral'
export type WorkMemorySelfAssertionPrivacy = 'always' | 'relevant' | 'never'

export interface WorkMemorySelfAssertion {
  id: string
  category: WorkMemorySelfAssertionCategory
  key: string
  label: string
  value: string
  status: WorkMemorySelfAssertionStatus
  privacy: WorkMemorySelfAssertionPrivacy
  scope?: string
  source: string
  confidence: number
  evidence: string[]
  promptReady: boolean
  createdAt: number
  updatedAt: number
}

export interface WorkMemorySelfAssertionRequest {
  id?: string
  category: WorkMemorySelfAssertionCategory
  key: string
  label: string
  value: string
  status?: WorkMemorySelfAssertionStatus
  privacy?: WorkMemorySelfAssertionPrivacy
  scope?: string
  source?: string
  confidence?: number
  evidence?: string[]
}

export interface WorkMemorySelfModelSummary {
  prompt: string
  included: WorkMemorySelfAssertion[]
  excluded: number
  updatedAt: number
}

export interface WorkMemorySelfModel {
  assertions: WorkMemorySelfAssertion[]
  summary: WorkMemorySelfModelSummary
  updatedAt: number
}

export type WorkMemoryTodoStatus = 'open' | 'doing' | 'waiting' | 'done' | 'canceled'
export type WorkMemoryTodoPriority = 'low' | 'normal' | 'high' | 'urgent'

export interface WorkMemoryTodoItem {
  id: string
  title: string
  note?: string
  status: WorkMemoryTodoStatus
  priority: WorkMemoryTodoPriority
  scope?: string
  source: string
  evidence: string[]
  dueAt?: number
  remindAt?: number
  completedAt?: number
  createdAt: number
  updatedAt: number
}

export interface WorkMemoryTodoListRequest {
  status?: WorkMemoryTodoStatus | ''
  scope?: string
  query?: string
  includeDone?: boolean
  limit?: number
}

export interface WorkMemoryTodoRequest {
  id?: string
  title: string
  note?: string
  status?: WorkMemoryTodoStatus
  priority?: WorkMemoryTodoPriority
  scope?: string
  source?: string
  evidence?: string[]
  dueAt?: number
  remindAt?: number
}

export interface WorkMemoryTodoUpdateRequest {
  id: string
  title?: string
  note?: string
  status?: WorkMemoryTodoStatus | ''
  priority?: WorkMemoryTodoPriority | ''
  scope?: string
  source?: string
  evidence?: string[]
  dueAt?: number
  remindAt?: number
  clearDueAt?: boolean
  clearRemindAt?: boolean
}

export interface WorkMemoryTodoList {
  items: WorkMemoryTodoItem[]
  open: number
  doing: number
  waiting: number
  done: number
  canceled: number
  updatedAt: number
}

export interface WorkMemoryCaptureFrame {
  captureId?: string
  imagePath?: string
  imageSignature?: string
  imageFingerprint?: string
  width?: number
  height?: number
  bytes?: number
  windowTitle?: string
  appName?: string
  createdAt: number
}

export interface ScheduledDraftStatus {
  enabled: boolean
  running: boolean
  intervalMinutes: number
  dailyDraftEnabled: boolean
  retrospectiveEnabled: boolean
  experienceReportEnabled: boolean
  lastCheckedAt?: number
  lastRunAt?: number
  lastEntryCount: number
  lastEntryCreatedAt?: number
  lastError?: string
  lastAutonomousRunAt?: number
  autonomousGenerated: number
  autonomousMessage?: string
  dailyDraft?: WorkMemoryDraft
  retrospectiveDraft?: WorkMemoryDraft
  experienceReport?: ExperienceReport
}

export interface WorkMemoryAutonomousArtifact {
  id: string
  kind: 'daily' | 'retrospective' | 'knowledge' | 'skill' | string
  title: string
  summary: string
  body: string
  evidence: string[]
  sourceInsightId?: string
  dedupKey?: string
  status: string
  deleteReason?: string
  confidence?: number
  agentExecutable?: boolean
  createdAt: number
  updatedAt?: number
  deletedAt?: number
}

export interface WorkMemoryAutonomousRejectRequest {
  id: string
  reason: string
}

export interface WorkMemoryAutonomousRejectResult {
  ok: boolean
  message: string
  artifact?: WorkMemoryAutonomousArtifact
  status: ScheduledDraftStatus
}

export interface WorkMemoryAutonomousRunResult {
  ok: boolean
  message: string
  generated: number
  skipped: number
  artifacts: WorkMemoryAutonomousArtifact[]
  status: ScheduledDraftStatus
  createdAt: number
}

export type FlowCandidateActionStatus = 'pending' | 'accepted' | 'snoozed' | 'ignored' | 'expired' | 'dismissed_by_rule' | 'executed' | 'failed'
export type FlowCandidateActionType = 'prepare_reply' | 'follow_up_candidate' | 'fact_check_warning' | 'text_polish_hint' | string

export interface FlowAutonomyPolicy {
  enabled: boolean
  communicationAssistEnabled: boolean
  textQualityAssistEnabled: boolean
  candidateTtlHours: number
  candidateCooldownMinutes: number
  defaultSnoozeMinutes: number
  notifyLowRiskAutomaticAction: boolean
}

export interface FlowAutonomyExtensionManifest {
  id: string
  name: string
  description: string
  enabled: boolean
  eventSources: string[]
  readScopes: string[]
  actionTypes: string[]
  confirmationPolicy: string
  ttlSeconds: number
  cooldownSeconds: number
}

export interface FlowAutonomyStatus {
  enabled: boolean
  privacyMode: boolean
  lastRunAt?: number
  lastMessage?: string
  pending: number
  snoozed: number
  expired: number
  executed: number
  extensions: FlowAutonomyExtensionManifest[]
  notifyLowRiskAutomatic: boolean
  candidateTtlHours: number
  candidateCooldownMinutes: number
  defaultSnoozeMinutes: number
  updatedAt: number
}

export interface FlowNotificationAction {
  id: string
  label: string
  kind: string
}

export interface FlowCandidateAction {
  id: string
  extensionId: string
  actionType: FlowCandidateActionType
  title: string
  summary: string
  body: string
  target?: string
  status: FlowCandidateActionStatus
  priority: 'low' | 'normal' | 'high' | 'urgent' | string
  confirmationPolicy: string
  notificationActions: FlowNotificationAction[]
  payload?: Record<string, string>
  evidence: string[]
  dedupKey?: string
  source?: string
  decisionActionId?: string
  decisionReason?: string
  confidence?: number
  createdAt: number
  updatedAt?: number
  expiresAt?: number
  snoozedUntil?: number
  decidedAt?: number
  executedAt?: number
}

export interface FlowCandidateActionListRequest {
  status?: FlowCandidateActionStatus | ''
  includeExpired?: boolean
  limit?: number
}

export interface FlowCandidateActionList {
  items: FlowCandidateAction[]
  pending: number
  snoozed: number
  accepted: number
  ignored: number
  expired: number
  executed: number
  failed: number
  updatedAt: number
}

export interface FlowCandidateActionDecisionRequest {
  id: string
  actionId?: string
  decision?: FlowCandidateActionStatus | ''
  reason?: string
  snoozeMinutes?: number
}

export interface FlowCandidateActionDecisionResult {
  ok: boolean
  message: string
  action?: FlowCandidateAction
  list: FlowCandidateActionList
}

export interface FlowAutonomyRunResult {
  ok: boolean
  message: string
  generated: number
  skipped: number
  expired: number
  actions: FlowCandidateAction[]
  status: FlowAutonomyStatus
  createdAt: number
}

export interface WorkMemoryEntry {
  id: string
  source: string
  contentType: string
  title: string
  summary: string
  text: string
  ocrText?: string
  ocrStatus?: string
  qualityOcrText?: string
  qualityOcrStatus?: string
  windowTitle?: string
  appName?: string
  captureId?: string
  imagePath?: string
  imageSignature?: string
  imageFingerprint?: string
  frames?: WorkMemoryCaptureFrame[]
  frameCount?: number
  qualityStatus?: string
  qualityCheckedAt?: number
  qualityReason?: string
  width?: number
  height?: number
  bytes?: number
  tags: string[]
  favorite: boolean
  sensitive: boolean
  createdAt: number
}

export interface WorkMemoryNoteRequest {
  title?: string
  text: string
  tags?: string[]
  favorite?: boolean
  sensitive?: boolean
}

export interface WorkMemoryExportResult {
  ok: boolean
  message: string
  path?: string
  entryCount: number
  skippedSensitiveCount: number
  skippedExcludedCount?: number
  filteredOutCount?: number
  includesSensitive: boolean
  filter?: WorkMemoryExportFilter
  bytes?: number
  createdAt?: number
}

export interface WorkMemoryExportFilter {
  startAt?: number
  endAt?: number
  tags?: string[]
  entryIds?: string[]
}

export interface WorkMemoryExportRequest extends WorkMemoryExportFilter {
  includeSensitive?: boolean
}

export interface WorkMemoryImportMaterialRequest {
  paths: string[]
  tags?: string[]
  favorite?: boolean
  sensitive?: boolean
}

export interface WorkMemoryImportMaterialItemResult {
  path: string
  ok: boolean
  message: string
  entryId?: string
  source?: string
  contentType?: string
  bytes?: number
}

export interface WorkMemoryImportMaterialResult {
  ok: boolean
  message: string
  imported: number
  skipped: number
  failed: number
  entries: WorkMemoryEntry[]
  items: WorkMemoryImportMaterialItemResult[]
  createdAt: number
}

export interface WorkMemoryDraft {
  id: string
  title: string
  body: string
  evidence: string[]
  createdAt: number
}

export interface WorkMemoryDraftPolishRequest {
  draft: WorkMemoryDraft
  kind: string
  confirmed?: boolean
}

export interface WorkMemoryDraftPolishResult {
  ok: boolean
  message: string
  draft: WorkMemoryDraft
  polishedDraft?: WorkMemoryDraft
  requiresConfirmation?: boolean
  external?: boolean
  provider?: string
  model?: string
  riskReasons?: string[]
}

export interface SkillAsset {
  id: string
  title: string
  body: string
  evidence: string[]
  source?: string
  createdAt?: number
  updatedAt?: number
}

export interface SkillStatus {
  path: string
  count: number
  lastSaveError?: string
  skills: SkillAsset[]
}

export interface SkillDraftSaveRequest {
  draft: WorkMemoryDraft
  confirmed?: boolean
}

export interface SkillDraftSaveResult {
  ok: boolean
  message: string
  skill: SkillAsset
  status: SkillStatus
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface SkillExportRequest {
  skillId: string
  confirmed?: boolean
}

export interface SkillExportResult {
  ok: boolean
  message: string
  skill: SkillAsset
  directory?: string
  zipPath?: string
  bytes?: number
  exportedAt?: number
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface SkillInstallRequest {
  skillId: string
  targetRoot?: string
  confirmed?: boolean
  overwrite?: boolean
}

export interface SkillInstallResult {
  ok: boolean
  message: string
  skill: SkillAsset
  targetRoot?: string
  installedDir?: string
  files?: string[]
  installedAt?: number
  refreshRequested?: boolean
  refreshMarker?: string
  refreshManifest?: string
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface SkillInstallDiagnosticsRequest {
  skillId?: string
  targetRoot?: string
}

export interface CodexInstalledSkill {
  id: string
  title?: string
  directory: string
  skillPath: string
  readable: boolean
  bytes?: number
  updatedAt?: number
  ariadneManaged?: boolean
  error?: string
}

export interface SkillRefreshDiagnostics {
  manifestPath: string
  markerPath: string
  manifestExists: boolean
  markerExists: boolean
  valid: boolean
  source?: string
  action?: string
  skillId?: string
  skillTitle?: string
  installedDir?: string
  requestedAt?: number
  markerId?: string
  markerText?: string
  markerMatchesManifest: boolean
  skillMatchesRequest: boolean
  installedDirExists: boolean
  installedSkillFileFound: boolean
  error?: string
}

export interface SkillInstallDiagnosticsResult {
  ok: boolean
  message: string
  targetRoot: string
  targetRootExists: boolean
  targetRootReadable: boolean
  skillId?: string
  installedDir?: string
  installed: boolean
  skillPath?: string
  skillFileExists: boolean
  skillFileBytes?: number
  skillUpdatedAt?: number
  discoveredCount: number
  ariadneManagedCount: number
  skills: CodexInstalledSkill[]
  refresh: SkillRefreshDiagnostics
  lastError?: string
}

export interface AgentTaskPackage {
  id: string
  goal: string
  context: string
  evidence: string[]
  boundaries: string[]
  acceptance: string[]
  requiresReview: boolean
  createdAt: number
}

export interface WorkflowDraftStep {
  id: string
  label: string
  command: string
  requiresConfirm: boolean
}

export interface WorkflowDraft {
  id: string
  title: string
  trigger: string
  input: string
  steps: WorkflowDraftStep[]
  output: string
  riskLevel: string
  evidence: string[]
  requiresReview: boolean
  createdAt: number
}

export interface ChecklistDraft {
  id: string
  title: string
  context: string
  items: string[]
  evidence: string[]
  requiresReview: boolean
  createdAt: number
}

export interface ChecklistAsset {
  id: string
  title: string
  context: string
  items: string[]
  evidence: string[]
  source?: string
  createdAt?: number
  updatedAt?: number
}

export interface ChecklistStatus {
  path: string
  count: number
  lastSaveError?: string
  checklists: ChecklistAsset[]
}

export interface ChecklistDraftSaveRequest {
  draft: ChecklistDraft
  confirmed?: boolean
}

export interface ChecklistDraftSaveResult {
  ok: boolean
  message: string
  checklist: ChecklistAsset
  status: ChecklistStatus
  requiresConfirmation?: boolean
  riskReasons?: string[]
}

export interface ExperienceInsight {
  id: string
  kind: string
  title: string
  summary: string
  reason: string
  recommendation: string
  evidence: string[]
  confidence: number
  severity: string
  requiresReview: boolean
  createdAt: number
  decisionStatus?: string
  decisionNote?: string
  decisionUpdatedAt?: number
  taskPackageId?: string
}

export interface ExperienceReport {
  id: string
  title: string
  summary: string
  periodDays: number
  entryCount: number
  evidenceCount: number
  insights: ExperienceInsight[]
  generatedAt: number
}

export interface ExperienceDiscoveryRequest {
  periodDays?: number
  external?: boolean
  confirmed?: boolean
}

export interface ExperienceDiscoveryResult {
  ok: boolean
  message: string
  report: ExperienceReport
  requiresConfirmation?: boolean
  external?: boolean
  provider?: string
  model?: string
  riskReasons?: string[]
}

export interface ExperienceDecision {
  insightId: string
  status: string
  note?: string
  taskPackageId?: string
  updatedAt: number
}

export interface ExperienceDecisionResult {
  ok: boolean
  message: string
  decision?: ExperienceDecision
}

export interface GeneralSettings {
  theme: 'dark' | 'light' | 'professional-pink' | 'light-graphite' | 'cloud-blue'
  runOnStartup: boolean
  language: string
}

export interface HotkeySettings {
  toggleWindow: string
  screenshot: string
  pinClipboard: string
}

export interface ScreenshotSettings {
  autoCopy: boolean
  autoPin: boolean
  autoSave: boolean
  saveDir: string
  filenameTemplate: string
  quality: number
  autoRedact: boolean
  redactPhones: boolean
  redactKeywords: string[]
}

export interface AISettings {
  enabled: boolean
  provider: string
  baseUrl: string
  model: string
  ocrModelEnabled: boolean
  ocrProvider: string
  ocrBaseUrl: string
  ocrModel: string
  embeddingEnabled: boolean
  embeddingProvider: string
  embeddingBaseUrl: string
  embeddingModel: string
  vectorStoreType: string
  vectorStoreUri: string
  vectorCollection: string
  agentsSdkEnabled: boolean
  agentResponsesEnabled: boolean
  traceMode: 'off' | 'local' | 'internal'
  opscoreSyncEnabled: boolean
  externalAgentEnabled: boolean
  codexCollaborationEnabled: boolean
  externalAgentTaskDirectory: string
}

export interface WorkMemorySettings {
  enabled: boolean
  timeMachineEnabled: boolean
  autoCaptureIntervalSeconds: number
  windowSwitchCaptureEnabled: boolean
  windowSwitchCooldownSeconds: number
  appCaptureProfiles: WorkMemoryAppCaptureProfile[]
  captureScope: string
  screenshotQuality: number
  multiMonitor: string
  privacyMode: boolean
  pauseOnIdle: boolean
  idlePauseSeconds: number
  pauseOnLock: boolean
  sourceClipboard: boolean
  sourceCaptureHistory: boolean
  sourceManualNote: boolean
  sourceSearchFavorite: boolean
  sourceActions: boolean
  autoOcr: boolean
  draftScheduleEnabled: boolean
  draftScheduleIntervalMinutes: number
  dailyDraftScheduleEnabled: boolean
  retrospectiveDraftScheduleEnabled: boolean
  experienceScheduleEnabled: boolean
  experienceDiscoveryEnabled: boolean
  experienceDiscoveryDays: number
  skillSuggestionEnabled: boolean
  workflowSuggestionEnabled: boolean
  flowAutonomyEnabled: boolean
  flowCommunicationAssist: boolean
  flowTextQualityAssist: boolean
  flowCandidateTtlHours: number
  flowCandidateCooldownMinutes: number
  flowDefaultSnoozeMinutes: number
  flowNotifyLowRiskAutomatic: boolean
  retentionDays: number
  thumbnailRetentionDays: number
  maxStorageMb: number
  keepFavoritesForever: boolean
  excludeApps: string[]
  excludeWindowKeywords: string[]
  excludePaths: string[]
  excludeUrls: string[]
  excludeContentPatterns: string[]
  sensitiveRulesEnabled: boolean
  allowSensitiveExport: boolean
}

export interface WorkMemoryAppCaptureProfile {
  id: string
  displayName: string
  processName: string
  icon?: string
  enabled: boolean
  windowSwitchDelaySeconds: number
  activeIntervalSeconds: number
}

export interface PluginSettings {
  enabled: Record<string, boolean>
}

export interface SearchSettings {
  fileExcludeFolders: string[]
  fileExcludePatterns: string[]
}

export interface PluginManifest {
  id: string
  name: string
  description: string
  keywords: string[]
  supportedPlatforms: string[]
  requiredCapabilities: string[]
  commandSchema: CommandSchema
}

export interface AppSettings {
  version: number
  general: GeneralSettings
  hotkeys: HotkeySettings
  screenshot: ScreenshotSettings
  workMemory: WorkMemorySettings
  ai: AISettings
  plugins: PluginSettings
  search: SearchSettings
}

export interface SecretRecordStatus {
  kind: string
  label: string
  targetName: string
  stored: boolean
  envNames: string[]
  envPresent: boolean
  activeSource: 'environment' | 'credential_manager' | 'missing' | string
  lastError?: string
}

export interface SecretStatus {
  available: boolean
  backend: string
  records: SecretRecordStatus[]
  lastError?: string
}

export interface SecretActionResult {
  ok: boolean
  message: string
  requiresConfirmation?: boolean
  status: SecretStatus
}

export interface LegacyConfigStatus {
  path: string
  exists: boolean
  needsImport: boolean
  importedKeys: string[]
  notes: string[]
}

export interface LegacySourceStatus {
  source: string
  path: string
  exists: boolean
  count: number
  bytes: number
  imageDir?: string
  imageCount?: number
  imageBytes?: number
  importedCount: number
  needsImport: boolean
  lastError?: string
}

export interface LegacyDataStatus {
  root: string
  exists: boolean
  needsImport: boolean
  sources: LegacySourceStatus[]
  totalCount: number
  totalBytes: number
  notes: string[]
}

export interface LegacyImportRequest {
  sources?: string[]
  limit?: number
  dryRun?: boolean
}

export interface LegacyImportSourceResult {
  source: string
  path: string
  found: number
  imported: number
  skipped: number
  failed: number
  beforeCount: number
  afterCount: number
  error?: string
}

export interface LegacyImportResult {
  ok: boolean
  message: string
  startedAt: number
  finishedAt: number
  dryRun: boolean
  sources: LegacyImportSourceResult[]
}

export interface ReleaseDataRootStatus {
  kind: string
  archiveName: string
  path: string
  exists: boolean
  fileCount: number
  bytes: number
}

export interface ReleaseBackupStatus {
  dataRoots: ReleaseDataRootStatus[]
  backupDir: string
  backupCount: number
  backupBytes: number
  latestBackup?: string
  notes: string[]
}

export interface ReleaseBackupRequest {
  reason?: string
}

export interface ReleaseBackupResult {
  ok: boolean
  message: string
  path?: string
  bytes?: number
  fileCount: number
  roots: ReleaseDataRootStatus[]
  createdAt: number
}

export interface ReleaseRestoreRequest {
  path?: string
  confirm: boolean
  createPreRestoreBackup: boolean
}

export interface ReleaseRestoreRootResult {
  kind: string
  archiveName: string
  path: string
  restoredFiles: number
  restoredBytes: number
  skippedFiles: number
  error?: string
}

export interface ReleaseRestoreResult {
  ok: boolean
  message: string
  path?: string
  preRestoreBackupPath?: string
  fileCount: number
  bytes: number
  roots: ReleaseRestoreRootResult[]
  requiresConfirmation: boolean
  restoredAt: number
}

export interface SettingsStorageStatus {
  path: string
  directory: string
  directoryExists: boolean
  exists: boolean
  bytes: number
  readBackOk: boolean
  readBackBytes: number
  readBackVersion: number
  entries: string[]
  virtualizedPath?: string
  virtualizedExists: boolean
  virtualizedBytes: number
  appDataEnv: string
  localAppDataEnv: string
  userConfigDir: string
  workingDir: string
  executablePath: string
  lastSaveError?: string
  readBackError?: string
}
