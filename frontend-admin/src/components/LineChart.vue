<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

const props = defineProps<{
  title: string
  series: { name: string; data: [number, number][]; color?: string }[]
  unit?: string
  height?: string
}>()

const el = ref<HTMLDivElement>()
let chart: echarts.ECharts | undefined
let ro: ResizeObserver | undefined

function render() {
  if (!chart) return
  chart.setOption({
    backgroundColor: 'transparent',
    grid: { left: 44, right: 12, top: 12, bottom: 24 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#0d141d',
      borderColor: '#1e2a3a',
      textStyle: { color: '#e8eef5', fontSize: 12, fontFamily: 'IBM Plex Mono' },
      valueFormatter: (v: unknown) => `${Number(v).toFixed(1)}${props.unit ?? ''}`,
    },
    legend: { show: props.series.length > 1, textStyle: { color: '#7c8da6', fontSize: 11 }, top: 0, right: 0 },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: '#1e2a3a' } },
      axisLabel: { color: '#7c8da6', fontSize: 10, fontFamily: 'IBM Plex Mono', hideOverlap: true },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: '#7c8da6', fontSize: 10, fontFamily: 'IBM Plex Mono', formatter: `{value}${props.unit ?? ''}` },
      splitLine: { lineStyle: { color: '#1e2a3a', type: 'dashed', opacity: 0.5 } },
    },
    series: props.series.map((s, i) => {
      const color = s.color ?? ['#22d3ee', '#34d399', '#fbbf24', '#f87171'][i % 4]
      return {
        name: s.name,
        type: 'line',
        showSymbol: false,
        smooth: 0.25,
        data: s.data,
        lineStyle: { color, width: 1.5 },
        itemStyle: { color },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: color + '33' },
            { offset: 1, color: color + '00' },
          ]),
        },
      }
    }),
  })
}

onMounted(() => {
  if (!el.value) return
  chart = echarts.init(el.value)
  render()
  ro = new ResizeObserver(() => chart?.resize())
  ro.observe(el.value)
})

watch(() => props.series, render, { deep: true })

onUnmounted(() => {
  ro?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div class="card p-4 animate-fade-up">
    <div class="text-[11px] uppercase tracking-[0.15em] text-text-lo font-display mb-2">{{ title }}</div>
    <div ref="el" :style="{ height: height ?? '220px', width: '100%' }" />
  </div>
</template>
