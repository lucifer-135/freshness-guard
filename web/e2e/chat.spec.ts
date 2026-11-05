import { expect, test } from '@playwright/test'
import { ask, claim, pickModel, signIn } from './helpers'

test('an async answer is underlined as verdicts arrive and explains itself', async ({ page }) => {
  await signIn(page)
  await pickModel(page, 'openai/fake-model')
  await ask(page)

  const price = claim(page, '$20 per month')
  await expect(price).toHaveAttribute('data-claim-status', 'outdated', { timeout: 30_000 })
  await expect(page.locator('article').getByText('2 claims checked. 1 claim is outdated.')).toBeVisible()

  await price.focus()
  await page.keyboard.press('Enter')
  const card = page.getByRole('dialog')
  await expect(card).toContainText('Current value: $25 per month')
  await expect(card.getByRole('heading', { name: 'Evidence from live sources' })).toBeVisible()
  await expect(card.getByRole('heading', { name: 'Inference by the judge model' })).toBeVisible()
  await page.keyboard.press('Escape')

  await page.getByRole('switch', { name: 'Show corrected answer' }).click()
  await expect(page.locator('ins').filter({ hasText: '$25 per month' }).first()).toBeVisible()
})
