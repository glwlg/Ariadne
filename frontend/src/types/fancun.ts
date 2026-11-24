export type SearchResultType =
  | 'file'
  | 'app'
  | 'plugin_trigger'
  | 'plugin_result'
  | 'workflow'
  | 'clipboard'
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
  | 'danger'

export type PreviewKind = 'text' | 'image' | 'settings' | 'workflow'

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
  iconAsset?: {
    url: string
    cacheKey: string
    kind: 'windows-shell'
  }
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
  totalResults?: number
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
  imagePath?: string
  width?: number
  height?: number
  provider?: string
  elapsedMs?: number
  sensitive: boolean
  error?: string
  recognizedAt?: number
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

export interface ProcessNetworkConnection {
  localAddress: string
  remoteAddress: string
  uploadBytesPerSecond: number
  downloadBytesPerSecond: number
  bytesSent: number
  bytesReceived: number
}

export interface ProcessNetworkTraffic {
  pid: number
  name: string
  path?: string
  iconUrl?: string
  uploadBytesPerSecond: number
  downloadBytesPerSecond: number
  bytesSent: number
  bytesReceived: number
  connections: ProcessNetworkConnection[]
}

export interface ProcessNetworkSnapshot {
  timestampUnix: number
  processCount: number
  connectionCount: number
  uploadBytesPerSecond: number
  downloadBytesPerSecond: number
  processes: ProcessNetworkTraffic[]
  lastError?: string
}

export interface GeneralSettings {
  theme: 'professional-pink' | 'light-graphite' | 'cloud-blue'
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
  ocrModelEnabled: boolean
  ocrProvider: string
  ocrBaseUrl: string
  ocrModel: string
}

export interface HistorySettings {
  clipboardEnabled: boolean
  privacyMode: boolean
  retentionDays: number
  maxStorageMb: number
  keepFavoritesForever: boolean
}

export interface PluginSettings {
  enabled: Record<string, boolean>
}

export interface SearchSettings {
  fileExcludeFolders: string[]
  fileExcludePatterns: string[]
  fileIncludeExtensions: string[]
  fileExcludeExtensions: string[]
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
  history: HistorySettings
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

export interface AppUpdateStatus {
  currentVersion: string
  state: string
  enabled: boolean
  canCheck: boolean
  canInstall: boolean
  updateAvailable: boolean
  availableVersion?: string
  releaseName?: string
  releaseNotes?: string
  artifactName?: string
  downloadedPath?: string
  installerLaunched: boolean
  message?: string
  lastError?: string
}

export interface AppUpdateResult {
  ok: boolean
  message: string
  status: AppUpdateStatus
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
