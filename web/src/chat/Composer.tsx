import { useQuery } from '@tanstack/react-query'
import { Send } from 'lucide-react'
import { useId, useState, type FormEvent, type KeyboardEvent } from 'react'
import { ModelPicker } from './ModelPicker'
import { getModels } from '@/api/client'
import type { Mode, Policy } from '@/api/types'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { MagneticButton } from '@/motion/MagneticButton'
import { useStatus } from '@/shell/TopBar'

const MODEL_KEY = 'fg.model'

const modes: { value: Mode; label: string }[] = [
  { value: 'async', label: 'In the background (async)' },
  { value: 'sync', label: 'Before the answer is returned (sync)' },
  { value: 'off', label: 'Off' },
]

const policies: { value: Policy; label: string }[] = [
  { value: 'annotate', label: 'Show warnings only' },
  { value: 'heal', label: 'Correct the answer' },
  { value: 'block', label: 'Block the answer' },
]

function storedModel(): string | null {
  try {
    return localStorage.getItem(MODEL_KEY)
  } catch {
    return null
  }
}

function Choice<T extends string>({
  legend,
  value,
  options,
  onChange,
}: {
  legend: string
  value: T
  options: { value: T; label: string }[]
  onChange: (v: T) => void
}) {
  const id = useId()
  return (
    <fieldset className="space-y-2">
      <legend className="eyebrow">{legend}</legend>
      <RadioGroup value={value} onValueChange={(v) => onChange(v as T)} className="flex flex-wrap gap-x-4 gap-y-2">
        {options.map((o) => (
          <div key={o.value} className="flex items-center gap-2">
            <RadioGroupItem value={o.value} id={`${id}-${o.value}`} />
            <Label htmlFor={`${id}-${o.value}`} className="font-normal">
              {o.label}
            </Label>
          </div>
        ))}
      </RadioGroup>
    </fieldset>
  )
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
    <form onSubmit={submit} className="space-y-4 rounded-xl border border-border bg-surface-1 p-4">
      <div className="flex flex-wrap gap-6">
        <div className="space-y-2">
          <Label htmlFor={`${messageId}-model`} className="eyebrow">
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
        </div>
        <Choice legend="Verification mode" value={mode} options={modes} onChange={setMode} />
        {mode === 'sync' && <Choice legend="When a claim is outdated" value={policy} options={policies} onChange={setPolicy} />}
      </div>
      <div className="space-y-2">
        <Label htmlFor={messageId}>Message</Label>
        <Textarea id={messageId} value={text} onChange={(e) => setText(e.target.value)} onKeyDown={onKeyDown} rows={3} />
      </div>
      <MagneticButton type="submit" className="h-10" disabled={!canSend}>
        <Send aria-hidden="true" />
        Send
      </MagneticButton>
    </form>
  )
}
