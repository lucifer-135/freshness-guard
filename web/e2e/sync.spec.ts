import { expect, test } from '@playwright/test'
import { ask, pickModel, signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
  await pickModel(page, 'openai/fake-model')
  await page.getByRole('radio', { name: 'Before reply' }).click()
})

test('a sync answer arrives already corrected', async ({ page }) => {
  await page.getByRole('radio', { name: 'Correct' }).click()
  await ask(page)
  await expect(page.locator('ins').filter({ hasText: '$25' }).first()).toBeVisible({ timeout: 30_000 })
  await expect(page.getByRole('radio', { name: 'Corrected' })).toHaveAttribute('aria-checked', 'true')
})

test('a blocked answer links to its report', async ({ page }) => {
  await page.getByRole('radio', { name: 'Block' }).click()
  await ask(page)
  await expect(page.getByText('The answer was blocked because it contains outdated or incorrect claims.')).toBeVisible({
    timeout: 30_000,
  })
  await page.getByRole('link', { name: 'Open the full report' }).click()
  await expect(page.getByText('This answer was blocked before it reached the user.')).toBeVisible()
})
