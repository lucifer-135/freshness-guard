const dateFormat = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' })

// formatDate writes a date as "1 October 2026"; it returns "" for an invalid date.
export function formatDate(value: string | Date): string {
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? '' : dateFormat.format(date)
}

export function percent(share: number): string {
  return `${Math.round(share * 100)}%`
}
