import { reactive } from 'vue'

export interface Toast {
  id: number
  kind: 'success' | 'error' | 'info'
  text: string
}

let seq = 0
export const toasts = reactive<Toast[]>([])

function push(kind: Toast['kind'], text: string) {
  const id = ++seq
  toasts.push({ id, kind, text })
  // 5s 自动消失（client.md 规则）
  setTimeout(() => dismiss(id), 5000)
}

export function dismiss(id: number) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}

export const toast = {
  success: (text: string) => push('success', text),
  error: (text: string) => push('error', text),
  info: (text: string) => push('info', text),
}
