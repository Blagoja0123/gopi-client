// In-app replacements for prompt/confirm (native ones are unreliable in
// webviews).

export type DialogField = { label: string; value: string; options?: { value: string; label: string }[] }

type DialogState = {
  title: string
  message?: string
  fields: DialogField[]
  confirmLabel: string
  danger?: boolean
  resolve: (v: string[] | null) => void
}

export const dialog = $state<{ current: DialogState | null }>({ current: null })

function open(d: Omit<DialogState, 'resolve'>): Promise<string[] | null> {
  return new Promise((resolve) => {
    dialog.current = { ...d, resolve }
  })
}

export function closeDialog(values: string[] | null) {
  const d = dialog.current
  dialog.current = null
  d?.resolve(values)
}

export async function promptText(title: string, label: string, value = '', confirmLabel = 'OK'): Promise<string | null> {
  const r = await open({ title, fields: [{ label, value }], confirmLabel })
  return r ? r[0] : null
}

export async function promptFields(title: string, fields: DialogField[], confirmLabel = 'OK'): Promise<string[] | null> {
  return open({ title, fields, confirmLabel })
}

export async function confirmAction(title: string, message: string, confirmLabel = 'Delete'): Promise<boolean> {
  return (await open({ title, message, fields: [], confirmLabel, danger: true })) !== null
}

export const toasts = $state<{ items: { id: number; text: string; kind: 'error' | 'info' }[] }>({ items: [] })
let toastSeq = 0

export function toast(text: string, kind: 'error' | 'info' = 'info') {
  const id = ++toastSeq
  toasts.items.push({ id, text, kind })
  setTimeout(() => {
    toasts.items = toasts.items.filter((t) => t.id !== id)
  }, kind === 'error' ? 6000 : 2500)
}
