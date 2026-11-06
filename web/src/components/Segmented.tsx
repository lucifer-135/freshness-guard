import { useId } from 'react'

interface SegmentedProps<T extends string> {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
}

// Segmented is the project's one segmented control. It is a group of native
// radio inputs, so arrow keys, Tab and screen readers behave as the platform
// does; the selected option sits on a raised surface inside a quiet track.
export function Segmented<T extends string>({ label, value, options, onChange }: SegmentedProps<T>) {
  const name = useId()
  return (
    <div role="radiogroup" aria-label={label} className="inline-flex rounded-md border border-border bg-surface-1 p-0.5">
      {options.map((option) => (
        <label key={option.value} className="relative">
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={option.value === value}
            onChange={() => onChange(option.value)}
            className="peer absolute inset-0 cursor-pointer appearance-none rounded-[5px]"
          />
          <span className="pointer-events-none block rounded-[5px] px-3 py-1 text-sm whitespace-nowrap text-text-muted transition-colors duration-[var(--dur-hover)] peer-hover:text-text peer-checked:bg-surface-3 peer-checked:text-text peer-checked:shadow-[var(--highlight-top)]">
            {option.label}
          </span>
        </label>
      ))}
    </div>
  )
}
