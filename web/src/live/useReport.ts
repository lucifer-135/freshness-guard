import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { mergeReport, reportKey } from './reportCache'
import { ApiError, getReport } from '@/api/client'
import type { Report } from '@/api/types'

const POLL_MS = 3000
const NOT_FOUND_RETRIES = 4
const NOT_FOUND_RETRY_MS = 2000

// useReport reads one report from the cache that the event stream fills. It
// polls while the report is pending, in case events are missed. A report that
// is not found yet is retried, because a streamed answer's report is stored
// only after the stream ends.
export function useReport(id: string | null, options: { enabled?: boolean } = {}): UseQueryResult<Report, ApiError> {
  const queryClient = useQueryClient()
  return useQuery<Report, ApiError>({
    queryKey: reportKey(id ?? ''),
    queryFn: async () => {
      const fetched = await getReport(id ?? '')
      return mergeReport(queryClient.getQueryData<Report>(reportKey(id ?? '')), fetched)
    },
    enabled: id !== null && (options.enabled ?? true),
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? POLL_MS : false),
    retry: (failures, error) => error instanceof ApiError && error.status === 404 && failures < NOT_FOUND_RETRIES,
    retryDelay: NOT_FOUND_RETRY_MS,
  })
}
