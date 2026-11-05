import type { Report, Verdict } from '@/api/types'

function claims(n: number): string {
  return `${n} claim${n === 1 ? '' : 's'}`
}

function list(parts: string[]): string {
  if (parts.length <= 1) return parts.join('')
  return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`
}

function withPeriod(text: string): string {
  return /[.!?]$/.test(text) ? text : `${text}.`
}

// progressSentence describes a report's state in one or two complete
// sentences, for the status line and the live region. revealed is the number
// of checked claims whose verdict is shown so far.
export function progressSentence(report: Report, revealed: number): string {
  if (report.status === 'error') {
    return report.reason ? `The freshness check could not finish. ${withPeriod(report.reason)}` : 'The freshness check could not finish.'
  }
  const checked = report.claims.filter((c) => c.checked).length
  if (report.status === 'pending') {
    if (report.claims.length === 0) return 'Finding the claims in this answer.'
    return `Checking claims against live sources. ${revealed} of ${claims(checked)} checked.`
  }

  if (report.claims.length === 0) return 'No time-sensitive claims were found in this answer.'
  if (checked === 0) return 'No claims were checked because none of them is likely to change over time.'

  const count = (v: Verdict) => report.claims.filter((c) => c.verdict === v).length
  const parts: string[] = []
  const outdated = count('STALE')
  const incorrect = count('WRONG')
  const unverified = count('UNVERIFIABLE')
  if (outdated) parts.push(`${claims(outdated)} ${outdated === 1 ? 'is' : 'are'} outdated`)
  if (incorrect) parts.push(`${claims(incorrect)} ${incorrect === 1 ? 'is' : 'are'} incorrect`)
  if (unverified) parts.push(`${claims(unverified)} could not be verified`)

  let sentence = `${claims(checked)} checked. ${parts.length ? `${list(parts)}.` : 'All of them are current.'}`
  const notChecked = count('NOT_CHECKED')
  if (notChecked) {
    sentence += ` ${claims(notChecked)} ${notChecked === 1 ? 'was' : 'were'} not checked because ${notChecked === 1 ? 'it is' : 'they are'} unlikely to change.`
  }
  return sentence
}
