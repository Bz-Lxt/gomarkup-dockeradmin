<script setup lang="ts">
import { computed } from 'vue'
import { severityOf } from '../utils/format'

const props = defineProps<{
  label: string
  value: string
  sub?: string
  percent?: number
  spark?: number[]
}>()

const severity = computed(() => (props.percent !== undefined ? severityOf(props.percent) : 'ok'))
const barColor = computed(() => ({ ok: 'var(--ok)', warn: 'var(--warn)', danger: 'var(--danger)' })[severity.value])

const sparkPath = computed(() => {
  const pts = props.spark ?? []
  if (pts.length < 2) return ''
  const w = 100
  const h = 28
  const max = Math.max(...pts, 1)
  const step = w / (pts.length - 1)
  return pts.map((v, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(h - (v / max) * (h - 2) - 1).toFixed(1)}`).join(' ')
})
</script>

<template>
  <div class="card p-5 relative overflow-hidden animate-fade-up">
    <div class="absolute left-0 top-0 bottom-0 w-[3px]" :style="{ backgroundColor: barColor }" />
    <div class="text-[11px] uppercase tracking-[0.15em] text-text-lo font-display">{{ label }}</div>
    <div class="mt-2 flex items-end justify-between gap-3">
      <div class="num text-3xl md:text-4xl font-semibold glow-signal" :style="{ color: percent !== undefined ? barColor : 'var(--text-hi)' }">
        {{ value }}
      </div>
      <svg v-if="sparkPath" viewBox="0 0 100 28" class="w-24 h-7 shrink-0 opacity-80" preserveAspectRatio="none">
        <path :d="sparkPath" fill="none" :stroke="barColor" stroke-width="1.5" stroke-linejoin="round" />
      </svg>
    </div>
    <div v-if="sub" class="mt-1.5 text-xs text-text-lo font-mono truncate">{{ sub }}</div>
  </div>
</template>
