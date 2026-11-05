import { Check, ChevronsUpDown } from 'lucide-react'
import { useState } from 'react'
import { groupModels } from './models'
import type { Model } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

export interface ModelPickerProps {
  id: string
  models: Model[]
  value: string
  onChange: (id: string) => void
  placeholder: string
  disabled?: boolean
}

// ModelPicker is a combobox with a search field. Bifrost lists hundreds of
// models, which a plain select cannot be searched through.
export function ModelPicker({ id, models, value, onChange, placeholder, disabled }: ModelPickerProps) {
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={disabled}
          className="w-72 max-w-full justify-between font-normal"
        >
          <span className="truncate">{value || placeholder}</span>
          <ChevronsUpDown aria-hidden="true" className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-(--radix-popover-trigger-width) min-w-72 p-0">
        <Command label="Search models">
          <CommandInput placeholder="Search models" />
          <CommandList>
            <CommandEmpty>No models match this search.</CommandEmpty>
            {groupModels(models).map((group) => (
              <CommandGroup key={group.provider} heading={group.provider || 'Other'}>
                {group.models.map((m) => (
                  <CommandItem
                    key={m.id}
                    value={m.id}
                    onSelect={() => {
                      onChange(m.id)
                      setOpen(false)
                    }}
                  >
                    <span className="truncate">{m.id}</span>
                    {m.id === value && <Check aria-hidden="true" className="ml-auto" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
