import type { AdminGroup } from '@/types'

export interface ApiKeyGroupFilterOption {
  value: number | null
  label: string
  kind?: 'group'
  disabled?: boolean
}

export interface ApiKeyGroupFilterLabels {
  all: string
  exclusive: string
  public: string
  disabled: string
}

const HEADER_EXCLUSIVE = -1
const HEADER_PUBLIC = -2
const HEADER_DISABLED = -3

export function buildApiKeyGroupFilterOptions(
  groups: AdminGroup[],
  labels: ApiKeyGroupFilterLabels
): ApiKeyGroupFilterOption[] {
  const exclusive: ApiKeyGroupFilterOption[] = []
  const publicGroups: ApiKeyGroupFilterOption[] = []
  const disabledGroups: ApiKeyGroupFilterOption[] = []

  for (const grp of groups) {
    const item: ApiKeyGroupFilterOption = { value: grp.id, label: grp.name }
    if (grp.status !== 'active') {
      disabledGroups.push(item)
    } else if (grp.is_exclusive) {
      exclusive.push(item)
    } else {
      publicGroups.push(item)
    }
  }

  const options: ApiKeyGroupFilterOption[] = [{ value: null, label: labels.all }]
  const sections: Array<[string, number, ApiKeyGroupFilterOption[]]> = [
    [labels.exclusive, HEADER_EXCLUSIVE, exclusive],
    [labels.public, HEADER_PUBLIC, publicGroups],
    [labels.disabled, HEADER_DISABLED, disabledGroups],
  ]

  for (const [label, headerValue, items] of sections) {
    if (items.length === 0) continue
    options.push({ value: headerValue, label, kind: 'group', disabled: true })
    options.push(...items)
  }
  return options
}
