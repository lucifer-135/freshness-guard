import path from 'node:path'
import { loadConfigFromFile } from 'vite'
import { expect, test } from 'vitest'

test('the production build writes the third-party licences next to the app', async () => {
  const configFile = path.resolve(import.meta.dirname, '../../vite.config.ts')
  const loaded = await loadConfigFromFile({ command: 'build', mode: 'production' }, configFile)

  // The name has no leading dot because go:embed skips files that start with a dot.
  expect(loaded?.config.build?.license).toEqual({ fileName: 'third-party-licenses.md' })
})
