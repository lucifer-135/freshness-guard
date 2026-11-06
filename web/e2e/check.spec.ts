import { expect, test } from '@playwright/test'
import { claim, fakeAnswer, signIn } from './helpers'

test('any pasted answer can be checked', async ({ page }) => {
  await signIn(page)
  await page.getByRole('radio', { name: 'Check any answer' }).click()
  await page.getByLabel('Answer to check').fill(fakeAnswer)
  await page.getByRole('button', { name: 'Check this answer' }).click()
  await expect(claim(page, '$20 per month')).toHaveAttribute('data-claim-status', 'outdated', { timeout: 30_000 })
})
