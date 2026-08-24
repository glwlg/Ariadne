import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const app = readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
const settings = readFileSync(new URL('../src/components/settings/SettingsCenter.vue', import.meta.url), 'utf8')
const network = readFileSync(new URL('../src/components/network/ProcessNetworkCenter.vue', import.meta.url), 'utf8')
const themeRuntime = readFileSync(new URL('../src/lib/theme.ts', import.meta.url), 'utf8')
const captureOverlay = readFileSync(new URL('../src/components/capture/CaptureOverlayWindow.vue', import.meta.url), 'utf8')
const tokens = readFileSync(new URL('../src/styles/tokens.css', import.meta.url), 'utf8')
const themes = readFileSync(new URL('../src/styles/themes.css', import.meta.url), 'utf8')
const surfaces = [
  readFileSync(new URL('../src/styles/surfaces-launcher-tools.css', import.meta.url), 'utf8'),
  readFileSync(new URL('../src/styles/surfaces-settings-api-network-overlays.css', import.meta.url), 'utf8'),
  readFileSync(new URL('../src/styles/surfaces-flow.css', import.meta.url), 'utf8'),
].join('\n')

assert.doesNotMatch(app, /process-network/, 'network details must reuse the existing network-monitor window')
assert.match(network, /进程网络/, 'the existing network-monitor window must render process traffic')
assert.doesNotMatch(settings, /Graphite Teal|value="(?:light|dark)"/, 'green themes must not remain selectable')
assert.doesNotMatch(themeRuntime, /ThemePreference[^\n]*(?:'light'|'dark')|classList\.toggle\('dark'/, 'removed themes must not remain in the runtime')
assert.doesNotMatch(captureOverlay, /#22c55e|#14b8a6/i, 'green annotation colours must not remain')
assert.doesNotMatch(`${tokens}\n${themes}\n${surfaces}`, /\b(?:teal|emerald|green)\b|15,\s*118,\s*110|20,\s*184,\s*166|21,\s*128,\s*61|52,\s*211,\s*153|#0f766e|#14b8a6|#5eead4/i, 'green palette values must not remain')

const forbiddenColours = []
for (const file of sourceFiles(fileURLToPath(new URL('../src/', import.meta.url)))) {
  const source = readFileSync(file, 'utf8')
  for (const match of source.matchAll(/#([\da-f]{6})\b|rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/gi)) {
    const rgb = match[1]
      ? [0, 2, 4].map((offset) => Number.parseInt(match[1].slice(offset, offset + 2), 16))
      : [Number(match[2]), Number(match[3]), Number(match[4])]
    if (isGreenHue(rgb)) forbiddenColours.push(`${file}: ${match[0]}`)
  }
}
assert.deepEqual(forbiddenColours, [], `green colour literals remain:\n${forbiddenColours.join('\n')}`)

function sourceFiles(directory) {
  return readdirSync(directory).flatMap((name) => {
    const path = join(directory, name)
    if (statSync(path).isDirectory()) return sourceFiles(path)
    return /\.(?:css|ts|vue)$/.test(name) ? [path] : []
  })
}

function isGreenHue([red, green, blue]) {
  const [r, g, b] = [red, green, blue].map((value) => value / 255)
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const delta = max - min
  if (delta === 0) return false
  let hue = max === r ? ((g - b) / delta) % 6 : max === g ? (b - r) / delta + 2 : (r - g) / delta + 4
  hue = (hue * 60 + 360) % 360
  const lightness = (max + min) / 2
  const saturation = delta / (1 - Math.abs(2 * lightness - 1))
  return hue >= 75 && hue <= 180 && saturation >= 0.25
}
