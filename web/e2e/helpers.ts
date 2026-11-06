import { expect, type Page } from '@playwright/test'

// The fake provider's fixed answer: the price claim is outdated (now $25 per
// month) and the version claim is current.
export const fakeAnswer = 'Example Cloud Pro costs $20 per month, and its current version is 4.2.'

export async function signIn(page: Page) {
  await page.goto('chat')
  await page.getByLabel('API token').fill(process.env.FG_TOKEN ?? 'e2e-token')
  await page.getByRole('button', { name: 'Open the chat' }).click()
  await expect(page.getByLabel('Message')).toBeVisible()
}

export async function pickModel(page: Page, id: string) {
  await page.getByRole('combobox', { name: 'Model' }).click()
  await page.getByRole('option', { name: id }).click()
}

export async function ask(page: Page, question = 'What does it cost?') {
  await page.getByLabel('Message').fill(question)
  await page.getByRole('button', { name: 'Send' }).click()
}

export function claim(page: Page, text: string) {
  return page.locator('[data-claim-status]').filter({ hasText: text })
}
