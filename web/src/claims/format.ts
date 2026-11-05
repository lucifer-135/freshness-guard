const dateFormat = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' })

// formatDate writes a date as "1 October 2026"; it returns "" for an invalid date.
export function formatDate(value: string | Date): string {
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? '' : dateFormat.format(date)
}

export function percent(share: number): string {
  return `${Math.round(share * 100)}%`
}

const localDate = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'long', year: 'numeric' })
const localTime = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' })

// formatDateTime writes a time in the reader's time zone as "1 November 2026, 18:42".
export function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : `${localDate.format(date)}, ${localTime.format(date)}`
}
