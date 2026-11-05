import type { Model } from '@/api/types'

// Bifrost model IDs are "provider/model", and the model part may contain slashes.
export function splitModel(id: string): { provider: string; name: string } {
  const slash = id.indexOf('/')
  return slash === -1 ? { provider: '', name: id } : { provider: id.slice(0, slash), name: id.slice(slash + 1) }
}

export function groupModels(models: Model[]): { provider: string; models: Model[] }[] {
  const groups = new Map<string, Model[]>()
  for (const m of models) groups.set(m.provider, [...(groups.get(m.provider) ?? []), m])
  return [...groups].map(([provider, list]) => ({ provider, models: list }))
}
