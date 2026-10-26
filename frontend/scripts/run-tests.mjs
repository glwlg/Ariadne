import { spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'

const { scripts } = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'))
for (const [name, command] of Object.entries(scripts)) {
  if (!name.startsWith('test:')) continue
  // Test commands stay as direct Node scripts so failures and exit codes are preserved on Windows.
  const match = /^node (scripts\/[\w.-]+\.(?:mjs|ts))$/.exec(command)
  if (!match) throw new Error(`Unsupported test command: ${name}: ${command}`)
  console.log(`\n${name}`)
  const result = spawnSync(process.execPath, [match[1]], {
    cwd: new URL('../', import.meta.url), stdio: 'inherit',
  })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
