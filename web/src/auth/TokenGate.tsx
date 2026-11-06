import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { ApiError, clearToken, getStatus, getToken, setToken, setUnauthorizedHandler } from '@/api/client'
import { HomePage } from '@/home/HomePage'

const SignOutContext = createContext<() => void>(() => {})

// eslint-disable-next-line react-refresh/only-export-components -- the hook belongs with its provider
export function useSignOut(): () => void {
  return useContext(SignOutContext)
}

const rejected = 'The token was not accepted. Check the value and try again.'
const unreachable = 'The plugin API could not be reached. Check that Bifrost is running.'
const expired = 'The token is no longer accepted. Enter the token again.'

// TokenGate shows the home page, which holds the sign-in form, until a plugin API token is stored. A 401
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

  if (!signedIn) return <HomePage notice={notice} onConnect={connect} />
  return <SignOutContext.Provider value={() => signOut(null)}>{children}</SignOutContext.Provider>
}
