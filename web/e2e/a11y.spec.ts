import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { ask, claim, pickModel, signIn } from './helpers'

test('the chat with a checked answer has no serious accessibility violations', async ({ page }) => {
  await signIn(page)
  await pickModel(page, 'openai/fake-model')
  await ask(page)
  await expect(claim(page, '$20 per month')).toHaveAttribute('data-claim-status', 'outdated', { timeout: 30_000 })
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)).toEqual([])
})

test('reduced motion turns the cursor effects off', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await signIn(page)
  await expect(page.locator('[data-aura]')).toHaveCount(0)
  await expect(page.locator('canvas')).toHaveCount(0)
})
