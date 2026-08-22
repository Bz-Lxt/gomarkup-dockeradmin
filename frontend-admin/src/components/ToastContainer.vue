<script setup lang="ts">
import { toasts, dismiss } from '../composables/useToast'

const styles: Record<string, { border: string; icon: string }> = {
  success: { border: 'var(--ok)', icon: 'M20 6L9 17l-5-5' },
  error: { border: 'var(--danger)', icon: 'M18 6L6 18M6 6l12 12' },
  info: { border: 'var(--signal)', icon: 'M12 8h.01M12 12v4M12 22a10 10 0 100-20 10 10 0 000 20z' },
}
</script>

<template>
  <div class="fixed top-4 right-4 z-[60] space-y-2 w-80 max-w-[calc(100vw-2rem)]">
    <div
      v-for="t in toasts"
      :key="t.id"
      class="card p-3.5 flex items-start gap-3 animate-fade-up shadow-xl"
      :style="{ borderLeftWidth: '3px', borderLeftColor: styles[t.kind].border }"
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" :stroke="styles[t.kind].border" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 mt-0.5">
        <path :d="styles[t.kind].icon" />
      </svg>
      <div class="flex-1 text-sm leading-snug break-words">{{ t.text }}</div>
      <button class="text-text-lo hover:text-text-hi shrink-0" aria-label="关闭提示" @click="dismiss(t.id)">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>
