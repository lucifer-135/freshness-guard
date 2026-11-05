// Query keys of the dashboard's data. The event stream invalidates them, so the
// numbers follow what the gateway does.
export const statsKey = ['stats'] as const

// The live feed's list of recent reports.
export const feedKey = ['reports', 'feed'] as const
