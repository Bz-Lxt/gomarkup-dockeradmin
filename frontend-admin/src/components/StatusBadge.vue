<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ state: string }>()

const conf = computed(() => {
  const s = props.state.toLowerCase()
  if (s === 'running' || s === 'connected' || s === 'ok' || s === 'fired')
    return { color: 'var(--ok)', bg: 'rgba(52,211,153,.1)' }
  if (s === 'paused' || s === 'degraded' || s === 'restarting' || s === 'recovered')
    return { color: 'var(--warn)', bg: 'rgba(251,191,36,.1)' }
  return { color: 'var(--danger)', bg: 'rgba(248,113,113,.1)' }
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-mono"
    :style="{ color: conf.color, backgroundColor: conf.bg }"
  >
    <span class="w-1.5 h-1.5 rounded-full animate-pulse-dot" :style="{ backgroundColor: conf.color, color: conf.color }" />
    {{ state }}
  </span>
</template>
