import { expect, test, type Page } from '@playwright/test'
import { ask, pickModel, signIn } from './helpers'

// Takes the README screenshots against the real stack (real models, real
// search). It runs only in the "screenshots" project: npx playwright test
// --project=screenshots. The images are written to web/docs-images/.
// Both prices changed in 2026 (bench/curated.json), so a model with an older
// knowledge cutoff is likely to give at least one outdated value.
const question =
  'How much does Apple TV cost per month in the United States, and how much does Xbox Game Pass Ultimate cost per month? Answer in two plain sentences, without any formatting.'

// Lets entrance animations finish, so the image does not show a half-faded card.
async function settle(page: Page) {
  await page.waitForTimeout(1200)
}

test('README screenshots', async ({ page }) => {
  test.setTimeout(180_000)
  await signIn(page)
  await pickModel(page, 'groq/openai/gpt-oss-20b')
  await ask(page, question)

  // The verdicts arrive one by one; wait for the first outdated or incorrect claim.
  const outdated = page.locator('[data-claim-status="outdated"], [data-claim-status="incorrect"]').first()
  await expect(outdated).toBeVisible({ timeout: 90_000 })
  await expect(page.getByText('Checking claims against live sources').first()).toBeHidden({ timeout: 90_000 })

  await outdated.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('complementary', { name: 'Claims in this answer' })).toContainText('Why this claim was checked')
  await settle(page)
  await page.screenshot({ path: 'docs-images/chat.png' })

  await page.getByRole('radio', { name: 'Corrected' }).click()
  await expect(page.locator('ins').first()).toBeVisible()
  await settle(page)
  await page.screenshot({ path: 'docs-images/corrected.png' })

  await page.getByRole('link', { name: 'Dashboard' }).click()
  await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible()
  await expect(page.getByRole('table', { name: 'Claim rates by model and category' })).toBeVisible()
  await settle(page)
  await page.screenshot({ path: 'docs-images/dashboard.png' })
})
