<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, openStream } from '../api/client'
import type { ContainerDetail, ContainerInfo, Health } from '../api/types'
import { fmtBytes, fmtPercent, fmtUptime } from '../utils/format'
import { toast } from '../composables/useToast'

const containers = ref<ContainerInfo[]>([])
const loading = ref(true)
const degraded = ref(false)
const loadError = ref('')

const detail = ref<ContainerDetail | null>(null)
const detailLogs = ref<string>('')
const detailOpen = ref(false)
const detailLoading = ref(false)

const confirm = ref<{ open: boolean; action: 'stop' | 'restart'; target: ContainerInfo | null; busy: boolean }>({
  open: false,
  action: 'stop',
  target: null,
  busy: false,
})

let closeStream: (() => void) | undefined

async function refresh() {
  try {
    containers.value = (await api.get<ContainerInfo[]>('/api/containers')) ?? []
    loadError.value = ''
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function checkHealth() {
  try {
    const h = await api.get<Health>('/api/health')
    degraded.value = h.docker !== 'connected'
  } catch {
    degraded.value = false
  }
}

async function doAction(c: ContainerInfo, action: 'start' | 'stop' | 'restart') {
  if (action === 'start') {
    await execAction(c, action)
    return
  }
  confirm.value = { open: true, action, target: c, busy: false }
}

async function execAction(c: ContainerInfo, action: 'start' | 'stop' | 'restart') {
  try {
    await api.post(`/api/containers/${c.id}/${action}`)
    toast.success(`容器 ${c.name} ${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}成功`)
    await refresh()
  } catch (e) {
    toast.error(`操作失败：${e instanceof Error ? e.message : '未知错误'}`)
  }
}

async function confirmAction() {
  const { target, action } = confirm.value
  if (!target) return
  confirm.value.busy = true
  await execAction(target, action)
  confirm.value = { open: false, action: 'stop', target: null, busy: false }
}

async function openDetail(c: ContainerInfo) {
  detailOpen.value = true
  detailLoading.value = true
  detail.value = null
  detailLogs.value = ''
  try {
    const [d, logs] = await Promise.all([
      api.get<ContainerDetail>(`/api/containers/${c.id}`),
      api.get<{ lines: string }>(`/api/containers/${c.id}/logs?tail=100`),
    ])
    detail.value = d
    detailLogs.value = logs.lines || '(无日志输出)'
  } catch (e) {
    toast.error(`加载详情失败：${e instanceof Error ? e.message : '未知错误'}`)
    detailOpen.value = false
  } finally {
    detailLoading.value = false
  }
}

onMounted(async () => {
  await checkHealth()
  await refresh()
  closeStream = openStream('/api/stream/containers', (data) => {
    containers.value = (data as ContainerInfo[]) ?? []
  })
})

onUnmounted(() => closeStream?.())
</script>

<template>
  <div class="w-full space-y-4">
    <h1 class="font-display text-xl font-bold tracking-wide">容器管理</h1>

    <div v-if="degraded" class="card p-4 border-warn/40 flex items-center gap-3 text-sm">
      <span class="w-2 h-2 rounded-full bg-warn animate-pulse-dot text-warn" />
      <span class="text-warn">Docker 降级模式：未检测到 docker.sock，容器功能不可用，系统指标监控不受影响。</span>
    </div>

    <div v-if="loading" class="card p-4 space-y-3 animate-pulse">
      <div v-for="i in 4" :key="i" class="h-10 bg-line/50 rounded" />
    </div>

    <EmptyState v-else-if="loadError && !degraded" icon="[!]" text="容器列表加载失败" :hint="loadError" />
    <EmptyState v-else-if="containers.length === 0 && !degraded" icon="[ ]" text="暂无容器" hint="Docker 守护进程中没有正在运行或已停止的容器" />

    <div v-else-if="!degraded" class="card overflow-hidden animate-fade-up">
      <div class="overflow-x-auto">
        <table class="table-base">
          <thead>
            <tr>
              <th>名称</th><th>镜像</th><th>状态</th><th>CPU</th><th>内存</th><th>网络 ↓/↑</th><th>运行时长</th><th class="text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id" class="cursor-pointer" @click="openDetail(c)">
              <td class="font-mono text-signal">{{ c.name }}</td>
              <td class="font-mono text-text-lo text-xs max-w-[200px] truncate">{{ c.image }}</td>
              <td><StatusBadge :state="c.state" /></td>
              <td class="num">{{ fmtPercent(c.cpu_percent) }}</td>
              <td class="num text-xs">{{ fmtBytes(c.mem_used) }}<span class="text-text-lo"> / {{ fmtBytes(c.mem_limit) }}</span></td>
              <td class="num text-xs">{{ fmtBytes(c.net_rx) }} / {{ fmtBytes(c.net_tx) }}</td>
              <td class="num text-xs text-text-lo">{{ fmtUptime(c.uptime_sec) }}</td>
              <td class="text-right whitespace-nowrap" @click.stop>
                <button v-if="c.state !== 'running'" class="btn btn-ghost !py-1 !px-2.5 text-xs" @click="doAction(c, 'start')">启动</button>
                <button v-if="c.state === 'running'" class="btn btn-ghost !py-1 !px-2.5 text-xs" @click="doAction(c, 'restart')">重启</button>
                <button v-if="c.state === 'running'" class="btn btn-danger !py-1 !px-2.5 text-xs ml-2" @click="doAction(c, 'stop')">停止</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 详情抽屉 -->
    <Teleport to="body">
      <div v-if="detailOpen" class="fixed inset-0 z-40">
        <div class="absolute inset-0 bg-ink-0/70 backdrop-blur-sm" @click="detailOpen = false" />
        <aside class="absolute right-0 top-0 bottom-0 w-full md:w-[520px] bg-ink-1 border-l border-line overflow-y-auto p-6 animate-fade-up">
          <div class="flex items-center justify-between mb-5">
            <h3 class="font-display font-semibold text-lg">容器详情</h3>
            <button class="text-text-lo hover:text-text-hi" aria-label="关闭详情" @click="detailOpen = false">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12" /></svg>
            </button>
          </div>

          <div v-if="detailLoading" class="space-y-3 animate-pulse">
            <div v-for="i in 5" :key="i" class="h-8 bg-line/50 rounded" />
          </div>

          <template v-else-if="detail">
            <div class="space-y-3 text-sm">
              <div class="flex justify-between"><span class="text-text-lo">名称</span><span class="font-mono text-signal">{{ detail.name }}</span></div>
              <div class="flex justify-between"><span class="text-text-lo">ID</span><span class="font-mono text-xs">{{ detail.id.slice(0, 12) }}</span></div>
              <div class="flex justify-between"><span class="text-text-lo">镜像</span><span class="font-mono text-xs break-all text-right">{{ detail.image }}</span></div>
              <div class="flex justify-between items-center"><span class="text-text-lo">状态</span><StatusBadge :state="detail.state" /></div>
              <div class="flex justify-between"><span class="text-text-lo">CPU</span><span class="num">{{ fmtPercent(detail.cpu_percent) }}</span></div>
              <div class="flex justify-between">
                <span class="text-text-lo">内存</span>
                <span class="num">{{ fmtBytes(detail.mem_used) }} / {{ fmtBytes(detail.mem_limit) }}（{{ fmtPercent(detail.mem_percent) }}）</span>
              </div>
              <div v-if="detail.ports.length" class="flex justify-between gap-4"><span class="text-text-lo shrink-0">端口</span><span class="font-mono text-xs text-right break-all">{{ detail.ports.join(', ') }}</span></div>
              <div v-if="detail.mounts.length" class="flex justify-between gap-4"><span class="text-text-lo shrink-0">挂载</span><span class="font-mono text-xs text-right break-all">{{ detail.mounts.join(', ') }}</span></div>
            </div>

            <h4 class="mt-6 mb-2 text-[11px] uppercase tracking-[0.15em] text-text-lo font-display">最近日志（100 行）</h4>
            <div class="card !bg-ink-0 p-4 max-h-80 overflow-y-auto log-view text-text-lo">{{ detailLogs }}</div>
          </template>
        </aside>
      </div>
    </Teleport>

    <ConfirmModal
      :open="confirm.open"
      :title="confirm.action === 'stop' ? '停止容器' : '重启容器'"
      :message="`确认${confirm.action === 'stop' ? '停止' : '重启'}容器「${confirm.target?.name}」？该操作会中断容器内正在运行的服务。`"
      :confirm-text="confirm.action === 'stop' ? '确认停止' : '确认重启'"
      :busy="confirm.busy"
      @confirm="confirmAction"
      @cancel="confirm.open = false"
    />
  </div>
</template>
