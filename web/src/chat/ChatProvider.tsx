import { createContext, useContext, type ReactNode } from 'react'
import { useChatState, type ChatSession } from './useChat'

const ChatContext = createContext<ChatSession | null>(null)

export function ChatProvider({ children }: { children: ReactNode }) {
  return <ChatContext.Provider value={useChatState()}>{children}</ChatContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components -- the hook belongs with its provider
export function useChat(): ChatSession {
  const session = useContext(ChatContext)
  if (!session) throw new Error('useChat must be used inside ChatProvider.')
  return session
}
