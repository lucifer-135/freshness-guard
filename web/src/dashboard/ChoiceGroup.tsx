import { useId } from 'react'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

// ChoiceGroup is a labelled group of radio buttons.
export function ChoiceGroup<T extends string>({
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
