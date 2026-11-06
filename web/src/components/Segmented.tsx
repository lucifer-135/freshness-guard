import RubberSegment from './reactbits/RubberSegment'

interface SegmentedProps<T extends string> {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
  size?: 'sm' | 'md'
}

// Segmented is the project's one segmented control. It fixes the colours to the
// design tokens, so pages never pass raw colours to the React Bits component.
export function Segmented<T extends string>({ label, value, options, onChange, size = 'sm' }: SegmentedProps<T>) {
  return (
    <RubberSegment
      aria-label={label}
      items={options}
      value={value}
      onChange={(next) => onChange(next as T)}
      trackColor="var(--surface-1)"
      thumbColor="var(--surface-3)"
      textColor="var(--text)"
      activeTextColor="var(--text)"
      radius={8}
      size={size}
    />
  )
}
