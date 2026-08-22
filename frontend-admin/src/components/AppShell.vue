<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import type { Health } from '../api/types'

const route = useRoute()
const health = ref<Health | null>(null)
let timer: ReturnType<typeof setInterval> | undefined

async function refreshHealth() {
  try {
    health.value = await api.get<Health>('/api/health')
  } catch {
    health.value = null
  }
}

onMounted(() => {
  refreshHealth()
  timer = setInterval(refreshHealth, 15000)
})
onUnmounted(() => clearInterval(timer))

const nav = [
  { to: '/dashboard', label: '总览', icon: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z' },
  { to: '/containers', label: '容器', icon: 'M21 8l-9-5-9 5v8l9 5 9-5V8zM3 8l9 5 9-5M12 13v8' },
  { to: '/alerts', label: '告警', icon: 'M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9M10.3 21a2 2 0 003.4 0' },
]
</script>

<template>
  <div class="flex h-full">
    <!-- 桌面侧边栏 -->
    <aside class="hidden md:flex w-60 shrink-0 flex-col border-r border-line bg-ink-1/80 backdrop-blur">
      <div class="flex items-center gap-3 px-5 h-16 border-b border-line">
        <div class="w-8 h-8 rounded-md bg-signal-dim/30 border border-signal-dim flex items-center justify-center">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--signal)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 8l-9-5-9 5v8l9 5 9-5V8z" /><path d="M3 8l9 5 9-5" /><path d="M12 13v8" />
          </svg>
        </div>
        <div>
          <div class="font-display font-bold text-[15px] tracking-wide">DockerAdmin</div>
          <div class="text-[11px] text-text-lo font-mono">mission control</div>
        </div>
      </div>

      <nav class="flex-1 py-4 px-3 space-y-1">
        <router-link
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 px-3 py-2.5 rounded-md text-sm transition-colors"
          :class="route.path === item.to ? 'bg-signal-dim/20 text-signal border border-signal-dim/40' : 'text-text-lo hover:text-text-hi hover:bg-ink-2 border border-transparent'"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path :d="item.icon" />
          </svg>
          {{ item.label }}
        </router-link>
      </nav>

      <div class="px-5 py-4 border-t border-line">
        <div class="flex items-center gap-2 text-xs">
          <span
            class="w-2 h-2 rounded-full animate-pulse-dot"
            :class="health ? (health.docker === 'connected' ? 'bg-ok text-ok' : 'bg-warn text-warn') : 'bg-danger text-danger'"
          />
          <span class="text-text-lo font-mono">
            {{ health ? (health.docker === 'connected' ? 'docker · connected' : 'docker · degraded') : 'backend · offline' }}
          </span>
        </div>
        <div v-if="health" class="mt-1 text-[11px] text-text-lo/70 font-mono">v{{ health.version }} · {{ health.collect_interval }}</div>
      </div>
    </aside>

    <!-- 主区域 -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- 移动顶栏 -->
      <header class="md:hidden flex items-center justify-between px-4 h-14 border-b border-line bg-ink-1/80 backdrop-blur">
        <div class="font-display font-bold tracking-wide">DockerAdmin</div>
        <span
          class="w-2 h-2 rounded-full animate-pulse-dot"
          :class="health ? (health.docker === 'connected' ? 'bg-ok text-ok' : 'bg-warn text-warn') : 'bg-danger text-danger'"
        />
      </header>

      <main class="flex-1 overflow-y-auto p-4 md:p-6 pb-20 md:pb-6 w-full">
        <slot />
      </main>

      <!-- 移动底部 Tab -->
      <nav class="md:hidden fixed bottom-0 inset-x-0 z-20 flex border-t border-line bg-ink-1/95 backdrop-blur">
        <router-link
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="flex-1 flex flex-col items-center gap-1 py-2.5 text-[11px]"
          :class="route.path === item.to ? 'text-signal' : 'text-text-lo'"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path :d="item.icon" />
          </svg>
          {{ item.label }}
        </router-link>
      </nav>
    </div>
  </div>
</template>
