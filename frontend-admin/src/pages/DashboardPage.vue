<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import MetricCard from '../components/MetricCard.vue'
import LineChart from '../components/LineChart.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, openStream } from '../api/client'
import type { MetricSnapshot } from '../api/types'
import { fmtBytes, fmtPercent, fmtRate } from '../utils/format'
import { toast } from '../composables/useToast'

const MAX_POINTS = 1800 // 1h @ 2s
const history = ref<MetricSnapshot[]>([])
const loading = ref(true)
const loadError = ref('')
let closeStream: (() => void) | undefined

const current = computed<MetricSnapshot | null>(() => history.value[history.value.length - 1] ?? null)

function pushSnapshot(s: MetricSnapshot) {
  history.value.push(s)
  if (history.value.length > MAX_POINTS) history.value.splice(0, history.value.length - MAX_POINTS)
}

const cpuSpark = computed(() => history.value.slice(-30).map((s) => s.cpu.percent))
const memSpark = computed(() => history.value.slice(-30).map((s) => s.mem.percent))

const diskWorst = computed(() => {
  const disks = current.value?.disk ?? []
  if (disks.length === 0) return null
  return disks.reduce((a, b) => (a.percent >= b.percent ? a : b))
})

const netTotal = computed(() => {
  const nets = (current.value?.net ?? []).filter((n) => n.iface !== 'lo')
  return { rx: nets.reduce((a, n) => a + n.rx_bps, 0), tx: nets.reduce((a, n) => a + n.tx_bps, 0) }
})

const ts = (s: MetricSnapshot) => new Date(s.ts).getTime()
const cpuSeries = computed(() => [{ name: 'CPU', data: history.value.map((s) => [ts(s), s.cpu.percent] as [number, number]) }])
const memSeries = computed(() => [{ name: 'MEM', data: history.value.map((s) => [ts(s), s.mem.percent] as [number, number]), color: '#34d399' }])
const netSeries = computed(() => [
  { name: 'RX', data: history.value.map((s) => [ts(s), s.net.filter((n) => n.iface !== 'lo').reduce((a, n) => a + n.rx_bps, 0) / 1024] as [number, number]) },
  { name: 'TX', data: history.value.map((s) => [ts(s), s.net.filter((n) => n.iface !== 'lo').reduce((a, n) => a + n.tx_bps, 0) / 1024] as [number, number]), color: '#fbbf24' },
])

onMounted(async () => {
  try {
    const h = await api.get<MetricSnapshot[]>('/api/metrics/history?minutes=30')
    history.value = h ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
  closeStream = openStream(
    '/api/stream/metrics',
    (data) => pushSnapshot(data as MetricSnapshot),
    () => toast.error('实时流断开，正在自动重连…'),
  )
})

onUnmounted(() => closeStream?.())
</script>

<template>
  <div class="w-full space-y-4">
    <div class="flex items-baseline justify-between">
      <h1 class="font-display text-xl font-bold tracking-wide">系统总览</h1>
      <span v-if="current" class="text-xs text-text-lo font-mono">
        load {{ current.load.map((v) => v.toFixed(2)).join(' / ') }} · procs {{ current.procs }}
      </span>
    </div>

    <!-- 加载骨架 -->
    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div v-for="i in 4" :key="i" class="card p-5 h-28 animate-pulse">
        <div class="h-3 w-16 bg-line rounded" />
        <div class="mt-4 h-8 w-24 bg-line rounded" />
      </div>
    </div>

    <EmptyState v-else-if="loadError" icon="[!]" text="指标加载失败" :hint="loadError" />

    <template v-else>
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
        <MetricCard
          label="CPU"
          :value="fmtPercent(current?.cpu.percent ?? NaN)"
          :percent="current?.cpu.percent"
          :spark="cpuSpark"
          :sub="`${current?.cpu.per_core.length ?? 0} cores`"
        />
        <MetricCard
          label="内存"
          :value="fmtPercent(current?.mem.percent ?? NaN)"
          :percent="current?.mem.percent"
          :spark="memSpark"
          :sub="`${fmtBytes(current?.mem.used ?? 0)} / ${fmtBytes(current?.mem.total ?? 0)}`"
        />
        <MetricCard
          label="磁盘（最忙挂载点）"
          :value="diskWorst ? fmtPercent(diskWorst.percent) : '-'"
          :percent="diskWorst?.percent"
          :sub="diskWorst ? `${diskWorst.mount} · ${fmtBytes(diskWorst.used)} / ${fmtBytes(diskWorst.total)}` : '无数据'"
        />
        <MetricCard
          label="网络吞吐"
          :value="fmtRate(netTotal.rx + netTotal.tx)"
          :sub="`↓ ${fmtRate(netTotal.rx)} · ↑ ${fmtRate(netTotal.tx)}`"
        />
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <LineChart title="CPU 使用率" :series="cpuSeries" unit="%" />
        <LineChart title="内存使用率" :series="memSeries" unit="%" />
      </div>
      <LineChart title="网络速率" :series="netSeries" unit="KB/s" height="200px" />
    </template>
  </div>
</template>
