import { useQuery } from '@tanstack/react-query'
import { ArrowUp } from 'lucide-react'
import { useId, useState, type FormEvent, type KeyboardEvent } from 'react'
import { modeHint, modeOptions, policyOptions } from './modes'
import { ModelPicker } from './ModelPicker'
import { getModels } from '@/api/client'
import type { Mode, Policy } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Segmented } from '@/components/Segmented'
import { Textarea } from '@/components/ui/textarea'
import { useStatus } from '@/shell/TopBar'

const MODEL_KEY = 'fg.model'

function storedModel(): string | null {
  try {
    return localStorage.getItem(MODEL_KEY)
  } catch {
    return null
  }
}

export interface ComposerProps {
  busy: boolean
  onSend: (text: string, options: { model: string; mode: Mode; policy: Policy | null }) => void
}

export function Composer({ busy, onSend }: ComposerProps) {
  const settings = useStatus().data?.settings
  const models = useQuery({ queryKey: ['models'], queryFn: getModels })
  const [chosenModel, setChosenModel] = useState<string | null>(storedModel)
  const [chosenMode, setMode] = useState<Mode | null>(null)
  const [chosenPolicy, setPolicy] = useState<Policy | null>(null)
  const [text, setText] = useState('')
  const messageId = useId()

  const list = models.data ?? []
  const model = list.some((m) => m.id === chosenModel) ? (chosenModel ?? '') : (list[0]?.id ?? '')
  const mode = chosenMode ?? settings?.default_mode ?? 'async'
  const policy = chosenPolicy ?? settings?.default_policy ?? 'heal'
  const canSend = !busy && model !== '' && text.trim() !== ''

  function chooseModel(id: string) {
    setChosenModel(id)
    try {
      localStorage.setItem(MODEL_KEY, id)
    } catch {
      // Storage is blocked; the choice lasts for this page.
    }
  }

  function submit(e?: FormEvent) {
    e?.preventDefault()
    if (!canSend) return
    onSend(text.trim(), { model, mode, policy: mode === 'sync' ? policy : null })
    setText('')
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  return (
    <form onSubmit={submit} className="space-y-2 rounded-xl border border-border-strong bg-surface-2 p-3">
      <Label htmlFor={messageId} className="sr-only">
        Message
      </Label>
      <Textarea
        id={messageId}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={onKeyDown}
        rows={2}
        placeholder="Ask about something that changes over time"
      />
      <div className="flex flex-wrap items-center gap-2">
        <Label htmlFor={`${messageId}-model`} className="sr-only">
          Model
        </Label>
        <ModelPicker
          id={`${messageId}-model`}
          models={list}
          value={model}
          onChange={chooseModel}
          disabled={list.length === 0}
          placeholder={models.isError ? 'The models could not be loaded.' : 'Loading models'}
        />
        <Segmented label="Verification mode" value={mode} options={modeOptions} onChange={setMode} />
        {mode === 'sync' && <Segmented label="When a claim is outdated" value={policy} options={policyOptions} onChange={setPolicy} />}
        <Button type="submit" className="ml-auto h-9" disabled={!canSend}>
          <ArrowUp aria-hidden="true" />
          Send
        </Button>
      </div>
      <p className="text-xs text-text-muted">{modeHint(mode, settings?.sync_timeout_ms ?? 8000)}</p>
    </form>
  )
}
