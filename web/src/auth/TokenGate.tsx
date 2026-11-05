import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { ApiError, clearToken, getStatus, getToken, setToken, setUnauthorizedHandler } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const SignOutContext = createContext<() => void>(() => {})

// eslint-disable-next-line react-refresh/only-export-components -- the hook belongs with its provider
export function useSignOut(): () => void {
  return useContext(SignOutContext)
}

const rejected = 'The token was not accepted. Check the value and try again.'
const unreachable = 'The plugin API could not be reached. Check that Bifrost is running.'
const expired = 'The token is no longer accepted. Enter the token again.'

// TokenGate shows the sign-in form until a plugin API token is stored. A 401
// from any later request returns to the form once, with a sentence saying why.
export function TokenGate({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [signedIn, setSignedIn] = useState(() => getToken() !== null)
  const [notice, setNotice] = useState<string | null>(null)
  // While a new token is being checked, its 401 is a sign-in error, not an expiry.
  const checking = useRef(false)
  const signedInRef = useRef(signedIn)

  const signOut = useCallback(
    (message: string | null) => {
      clearToken()
      signedInRef.current = false
      setSignedIn(false)
      setNotice(message)
      queryClient.clear()
    },
    [queryClient],
  )

  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (checking.current || !signedInRef.current) return
      signOut(expired)
    })
    return () => setUnauthorizedHandler(null)
  }, [signOut])

  async function connect(token: string): Promise<string | null> {
    checking.current = true
    setToken(token)
    try {
      await getStatus()
      signedInRef.current = true
      setSignedIn(true)
      setNotice(null)
      return null
    } catch (err) {
      clearToken()
      return err instanceof ApiError && err.status === 401 ? rejected : unreachable
    } finally {
      checking.current = false
    }
  }

  if (!signedIn) return <SignInForm notice={notice} onConnect={connect} />
  return <SignOutContext.Provider value={() => signOut(null)}>{children}</SignOutContext.Provider>
}

function SignInForm({ notice, onConnect }: { notice: string | null; onConnect: (token: string) => Promise<string | null> }) {
  const [token, setTokenValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const errorId = useId()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(await onConnect(token.trim()))
    setBusy(false)
  }

  return (
    <main className="flex min-h-dvh items-center justify-center bg-bg p-4">
      <form
        onSubmit={submit}
        className="w-full max-w-md rounded-xl border border-border bg-surface-1 p-8 shadow-[var(--highlight-top)]"
      >
        <h1 className="text-2xl font-semibold">Connect to Freshness Guard</h1>
        <p className="mt-2 text-text-muted">Enter the plugin API token. It is the value of FG_API_TOKEN in deploy/.env.</p>
        {notice && <p className="mt-4 rounded-md border border-border bg-surface-2 p-3 text-text">{notice}</p>}
        <div className="mt-6 space-y-2">
          <Label htmlFor="api-token">API token</Label>
          <Input
            id="api-token"
            type="password"
            autoComplete="current-password"
            value={token}
            onChange={(e) => setTokenValue(e.target.value)}
            aria-invalid={error !== null}
            aria-describedby={error ? errorId : undefined}
            required
          />
          {error && (
            <p id={errorId} role="alert" className="text-sm text-status-outdated">
              {error}
            </p>
          )}
        </div>
        <Button type="submit" className="mt-6 h-10 w-full" disabled={busy || token.trim() === ''}>
          Connect
        </Button>
        <p className="mt-4 text-xs text-text-subtle">The token is stored in this browser until you sign out.</p>
      </form>
    </main>
  )
}
