import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { ask, claim, pickModel, signIn } from './helpers'

test('the dashboard shows the checked answer and has no serious accessibility violations', async ({ page }) => {
  await signIn(page)
  await pickModel(page, 'openai/fake-model')
  await ask(page)
  await expect(claim(page, '$20 per month')).toHaveAttribute('data-claim-status', 'outdated', { timeout: 30_000 })

  await page.getByRole('link', { name: 'Dashboard' }).click()
  await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible()
  await expect(page.getByText('Answers checked in the last 24 hours')).toBeVisible()
  await expect(page.getByText('Cached answers removed in the last 24 hours')).toBeVisible()

  const heatmap = page.getByRole('table', { name: 'Claim rates by model and category' })
  await expect(heatmap).toBeVisible()
  await expect(heatmap.getByRole('rowheader', { name: 'openai/fake-model' })).toBeVisible()
  await expect(page.getByRole('list', { name: 'Recent reports' }).getByRole('link').first()).toContainText('openai/fake-model')

  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)).toEqual([])
})
