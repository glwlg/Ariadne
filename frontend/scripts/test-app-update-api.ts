import assert from 'node:assert/strict'
import { normalizeAppUpdateStatus } from '../src/services/appUpdateApi.ts'

const status = normalizeAppUpdateStatus({
  currentVersion: '1.0.0',
  state: 'available',
  enabled: true,
  canCheck: true,
  canInstall: true,
  updateAvailable: true,
  availableVersion: '1.1.0',
  releaseName: 'Fancun 1.1.0',
  releaseNotes: 'Fixes',
  artifactName: 'FancunSetup-1.1.0-windows-x64.exe',
  installerLaunched: false,
})

assert.equal(status.currentVersion, '1.0.0')
assert.equal(status.availableVersion, '1.1.0')
assert.equal(status.canInstall, true)
assert.equal(status.installerLaunched, false)

const empty = normalizeAppUpdateStatus({} as never)
assert.equal(empty.currentVersion, 'dev')
assert.equal(empty.enabled, false)
assert.equal(empty.canInstall, false)

console.log('app update API normalization checks passed')
