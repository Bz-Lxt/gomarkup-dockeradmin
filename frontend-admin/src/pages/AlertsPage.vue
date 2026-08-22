<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import Modal from '../components/Modal.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import EmptyState from '../components/EmptyState.vue'
import { api } from '../api/client'
import type { AlertEvent, AlertRule, MetricKind, WebhookReceipt } from '../api/types'
import { fmtDateTime } from '../utils/format'
import { toast } from '../composables/useToast'

const MOCK_WEBHOOK_URL = 'http://localhost:8080/api/mock/webhook'

const rules = ref<AlertRule[]>([])
const events = ref<AlertEvent[]>([])
const receipts = ref<WebhookReceipt[]>([])
const loading = ref(true)

const metricOptions: { value: MetricKind; label: string; unit: string; min: number; max: number }[] = [
  { value: 'cpu_percent', label: '系统 CPU %', unit: '%', min: 0, max: 100 },
  { value: 'mem_percent', label: '系统内存 %', unit: '%', min: 0, max: 100 },
  { value: 'disk_percent', label: '磁盘（任一挂载点）%', unit: '%', min: 0, max: 100 },
  { value: 'net_rx_bps', label: '网络接收速率 B/s', unit: 'B/s', min: 0, max: Number.MAX_SAFE_INTEGER },
  { value: 'net_tx_bps', label: '网络发送速率 B/s', unit: 'B/s', min: 0, max: Number.MAX_SAFE_INTEGER },
  { value: 'container_cpu_percent', label: '容器 CPU %', unit: '%', min: 0, max: Number.MAX_SAFE_INTEGER },
  { value: 'container_mem_percent', label: '容器内存 %', unit: '%', min: 0, max: 100 },
]

const metricLabel = (m: string) => metricOptions.find((o) => o.value === m)?.label ?? m

interface RuleForm {
  name: string
  metric: MetricKind
  target: string
  op: AlertRule['op']
  threshold: number | null
  duration_sec: number | null
  cooldown_sec: number | null
  webhook_url: string
  notify_recovery: boolean
  enabled: boolean
}

const blankForm = (): RuleForm => ({
  name: '',
  metric: 'cpu_percent',
  target: '',
  op: '>',
  threshold: 80,
  duration_sec: 30,
  cooldown_sec: 300,
  webhook_url: MOCK_WEBHOOK_URL,
  notify_recovery: true,
  enabled: true,
})

const form = reactive<RuleForm>(blankForm())
const formErrors = reactive<Record<string, string>>({})
const editingId = ref<string | null>(null)
const formOpen = ref(false)
const saving = ref(false)
const deleting = ref<{ open: boolean; target: AlertRule | null; busy: boolean }>({ open: false, target: null, busy: false })

function isContainerMetric(m: MetricKind) {
  return m === 'container_cpu_percent' || m === 'container_mem_percent'
}

// schema 驱动校验（client.md：min/max 读 schema，不硬编码）
function validate(): boolean {
  Object.keys(formErrors).forEach((k) => delete formErrors[k])
  if (!form.name.trim()) formErrors.name = '规则名称不能为空'
  const schema = metricOptions.find((o) => o.value === form.metric)!
  if (form.threshold === null || Number.isNaN(form.threshold)) {
    formErrors.threshold = '阈值不能为空'
  } else if (form.threshold < schema.min || form.threshold > schema.max) {
    formErrors.threshold = `阈值范围为 ${schema.min} ~ ${schema.max}${schema.unit}`
  }
  if (form.duration_sec === null || form.duration_sec < 0) formErrors.duration_sec = '持续时间须 ≥ 0 秒'
  if (form.cooldown_sec === null || form.cooldown_sec < 0) formErrors.cooldown_sec = '冷却期须 ≥ 0 秒'
  if (!form.webhook_url.trim()) {
    formErrors.webhook_url = 'Webhook URL 不能为空'
  } else if (!/^https?:\/\/.+/.test(form.webhook_url.trim())) {
    formErrors.webhook_url = '仅支持 http/https 协议的 URL'
  }
  if (isContainerMetric(form.metric) && !form.target.trim()) {
    formErrors.target = '容器类指标必须填写目标容器名称'
  }
  return Object.keys(formErrors).length === 0
}

function openCreate() {
  Object.assign(form, blankForm())
  Object.keys(formErrors).forEach((k) => delete formErrors[k])
  editingId.value = null
  formOpen.value = true
}

function openEdit(r: AlertRule) {
  Object.assign(form, {
    name: r.name, metric: r.metric, target: r.target, op: r.op, threshold: r.threshold,
    duration_sec: r.duration_sec, cooldown_sec: r.cooldown_sec, webhook_url: r.webhook_url,
    notify_recovery: r.notify_recovery, enabled: r.enabled,
  })
  Object.keys(formErrors).forEach((k) => delete formErrors[k])
  editingId.value = r.id
  formOpen.value = true
}

async function save() {
  if (!validate()) {
    toast.error('表单校验未通过，请检查标红字段')
    return
  }
  saving.value = true
  try {
    const payload = { ...form, name: form.name.trim(), target: form.target.trim(), webhook_url: form.webhook_url.trim() }
    if (editingId.value) {
      await api.put(`/api/alert-rules/${editingId.value}`, payload)
      toast.success('规则已更新')
    } else {
      await api.post('/api/alert-rules', payload)
      toast.success('规则已创建')
    }
    formOpen.value = false
    await refresh()
  } catch (e) {
    toast.error(`保存失败：${e instanceof Error ? e.message : '未知错误'}`)
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(r: AlertRule) {
  try {
    await api.put(`/api/alert-rules/${r.id}`, { ...r, enabled: !r.enabled })
    toast.info(`规则「${r.name}」已${r.enabled ? '停用' : '启用'}`)
    await refresh()
  } catch (e) {
    toast.error(`切换失败：${e instanceof Error ? e.message : '未知错误'}`)
  }
}

async function confirmDelete() {
  if (!deleting.value.target) return
  deleting.value.busy = true
  try {
    await api.del(`/api/alert-rules/${deleting.value.target.id}`)
    toast.success('规则已删除')
    deleting.value = { open: false, target: null, busy: false }
    await refresh()
  } catch (e) {
    toast.error(`删除失败：${e instanceof Error ? e.message : '未知错误'}`)
    deleting.value.busy = false
  }
}

async function refresh() {
  const [r, e, rc] = await Promise.all([
    api.get<AlertRule[]>('/api/alert-rules'),
    api.get<AlertEvent[]>('/api/alert-events?limit=50'),
    api.get<WebhookReceipt[]>('/api/mock/webhook/receipts'),
  ])
  rules.value = r ?? []
  events.value = e ?? []
  receipts.value = rc ?? []
}

onMounted(async () => {
  try {
    await refresh()
  } catch (e) {
    toast.error(`加载失败：${e instanceof Error ? e.message : '未知错误'}`)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="w-full space-y-4">
    <div class="flex items-center justify-between">
      <h1 class="font-display text-xl font-bold tracking-wide">告警规则</h1>
      <button class="btn btn-primary" @click="openCreate">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
        新建规则
      </button>
    </div>

    <div v-if="loading" class="card p-4 space-y-3 animate-pulse">
      <div v-for="i in 3" :key="i" class="h-10 bg-line/50 rounded" />
    </div>

    <EmptyState v-else-if="rules.length === 0" icon="[ ]" text="暂无告警规则" hint="点击右上角「新建规则」，例如：CPU > 80% 持续 30 秒触发 Webhook" />

    <div v-else class="card overflow-hidden animate-fade-up">
      <div class="overflow-x-auto">
        <table class="table-base">
          <thead>
            <tr><th>名称</th><th>条件</th><th>持续/冷却</th><th>Webhook</th><th>启用</th><th class="text-right">操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="r in rules" :key="r.id">
              <td class="font-medium">{{ r.name }}</td>
              <td class="font-mono text-xs">
                {{ metricLabel(r.metric) }} {{ r.op }} {{ r.threshold }}
                <span v-if="r.target" class="text-signal">@{{ r.target }}</span>
              </td>
              <td class="num text-xs text-text-lo">{{ r.duration_sec }}s / {{ r.cooldown_sec }}s</td>
              <td class="font-mono text-xs text-text-lo max-w-[220px] truncate" :title="r.webhook_url">{{ r.webhook_url }}</td>
              <td>
                <button
                  class="relative w-10 h-5.5 h-6 rounded-full transition-colors"
                  :class="r.enabled ? 'bg-signal-dim' : 'bg-line'"
                  :aria-label="r.enabled ? '停用规则' : '启用规则'"
                  @click="toggleEnabled(r)"
                >
                  <span class="absolute top-0.5 w-5 h-5 rounded-full bg-white transition-all" :class="r.enabled ? 'left-[18px]' : 'left-0.5'" />
                </button>
              </td>
              <td class="text-right whitespace-nowrap">
                <button class="btn btn-ghost !py-1 !px-2.5 text-xs" @click="openEdit(r)">编辑</button>
                <button class="btn btn-danger !py-1 !px-2.5 text-xs ml-2" @click="deleting = { open: true, target: r, busy: false }">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- 触发记录 -->
      <div class="card p-4 animate-fade-up">
        <h2 class="text-[11px] uppercase tracking-[0.15em] text-text-lo font-display mb-3">触发记录（最近 50 条）</h2>
        <EmptyState v-if="events.length === 0" icon="[ ]" text="暂无触发记录" />
        <div v-else class="space-y-2 max-h-96 overflow-y-auto">
          <div v-for="e in events" :key="e.id" class="flex items-start gap-3 text-xs border-b border-line/40 pb-2">
            <span class="w-1.5 h-1.5 rounded-full mt-1.5 shrink-0" :class="e.kind === 'fired' ? 'bg-danger' : 'bg-ok'" />
            <div class="flex-1 min-w-0">
              <div>
                <span class="font-medium">{{ e.rule_name }}</span>
                <span class="text-text-lo"> · {{ e.kind === 'fired' ? '触发' : '恢复' }} · 值 </span>
                <span class="num text-signal">{{ e.value.toFixed(1) }}</span>
                <span class="text-text-lo">（阈值 {{ e.op }} {{ e.threshold }}）</span>
              </div>
              <div class="text-text-lo/70 font-mono mt-0.5">
                {{ fmtDateTime(e.fired_at) }} · webhook {{ e.webhook_status || e.webhook_error || 'ok' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Mock Webhook 接收记录 -->
      <div class="card p-4 animate-fade-up">
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-[11px] uppercase tracking-[0.15em] text-text-lo font-display">Mock Webhook 接收记录</h2>
          <button class="btn btn-ghost !py-1 !px-2.5 text-xs" @click="refresh">刷新</button>
        </div>
        <EmptyState v-if="receipts.length === 0" icon="[ ]" text="暂无接收记录" :hint="`将规则的 Webhook 填为 ${MOCK_WEBHOOK_URL} 即可在此观察推送`" />
        <div v-else class="space-y-2 max-h-96 overflow-y-auto">
          <div v-for="r in receipts" :key="r.id" class="text-xs border-b border-line/40 pb-2">
            <div class="font-mono text-text-lo/70 mb-1">#{{ r.id }} · {{ fmtDateTime(r.received_at) }}</div>
            <pre class="font-mono text-[11px] text-text-lo whitespace-pre-wrap break-all">{{ r.payload }}</pre>
          </div>
        </div>
      </div>
    </div>

    <!-- 规则表单 -->
    <Modal :title="editingId ? '编辑规则' : '新建规则'" :open="formOpen" @close="formOpen = false">
      <form class="space-y-4" novalidate @submit.prevent="save">
        <div>
          <label class="block text-xs text-text-lo mb-1.5">规则名称 <span class="text-danger">*</span></label>
          <input v-model="form.name" class="input" :class="{ 'input-error': formErrors.name }" placeholder="例如：CPU 过高告警" />
          <p v-if="formErrors.name" class="mt-1 text-xs text-danger">{{ formErrors.name }}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label class="block text-xs text-text-lo mb-1.5">监控指标 <span class="text-danger">*</span></label>
            <select v-model="form.metric" class="input">
              <option v-for="o in metricOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-text-lo mb-1.5">目标容器<span v-if="isContainerMetric(form.metric)" class="text-danger"> *</span></label>
            <input v-model="form.target" class="input" :class="{ 'input-error': formErrors.target }" placeholder="容器名称（系统指标留空）" :disabled="!isContainerMetric(form.metric)" />
            <p v-if="formErrors.target" class="mt-1 text-xs text-danger">{{ formErrors.target }}</p>
          </div>
        </div>

        <div class="grid grid-cols-3 gap-4">
          <div>
            <label class="block text-xs text-text-lo mb-1.5">运算符</label>
            <select v-model="form.op" class="input">
              <option value=">">&gt;</option><option value=">=">&ge;</option><option value="<">&lt;</option><option value="<=">&le;</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-text-lo mb-1.5">阈值 <span class="text-danger">*</span></label>
            <input v-model.number="form.threshold" type="number" class="input" :class="{ 'input-error': formErrors.threshold }" />
            <p v-if="formErrors.threshold" class="mt-1 text-xs text-danger">{{ formErrors.threshold }}</p>
          </div>
          <div>
            <label class="block text-xs text-text-lo mb-1.5">持续（秒）</label>
            <input v-model.number="form.duration_sec" type="number" min="0" class="input" :class="{ 'input-error': formErrors.duration_sec }" />
            <p v-if="formErrors.duration_sec" class="mt-1 text-xs text-danger">{{ formErrors.duration_sec }}</p>
          </div>
        </div>

        <div>
          <label class="block text-xs text-text-lo mb-1.5">冷却期（秒）</label>
          <input v-model.number="form.cooldown_sec" type="number" min="0" class="input" :class="{ 'input-error': formErrors.cooldown_sec }" />
          <p v-if="formErrors.cooldown_sec" class="mt-1 text-xs text-danger">{{ formErrors.cooldown_sec }}</p>
          <p class="mt-1 text-[11px] text-text-lo/60">触发后冷却期内不再重复告警，防止告警风暴</p>
        </div>

        <div>
          <label class="block text-xs text-text-lo mb-1.5">Webhook URL <span class="text-danger">*</span></label>
          <div class="flex gap-2">
            <input v-model="form.webhook_url" class="input flex-1" :class="{ 'input-error': formErrors.webhook_url }" placeholder="https://example.com/hook" />
            <button type="button" class="btn btn-ghost shrink-0 text-xs" @click="form.webhook_url = MOCK_WEBHOOK_URL">填入 Mock 地址</button>
          </div>
          <p v-if="formErrors.webhook_url" class="mt-1 text-xs text-danger">{{ formErrors.webhook_url }}</p>
        </div>

        <label class="flex items-center gap-2 text-sm cursor-pointer select-none">
          <input v-model="form.notify_recovery" type="checkbox" class="accent-[#22d3ee] w-4 h-4" />
          恢复时也发送通知
        </label>

        <div class="flex justify-end gap-3 pt-2">
          <button type="button" class="btn btn-ghost" :disabled="saving" @click="formOpen = false">取消</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存规则' }}</button>
        </div>
      </form>
    </Modal>

    <ConfirmModal
      :open="deleting.open"
      title="删除规则"
      :message="`确认删除告警规则「${deleting.target?.name}」？删除后将不再对该指标进行监控告警。`"
      confirm-text="确认删除"
      :busy="deleting.busy"
      @confirm="confirmDelete"
      @cancel="deleting.open = false"
    />
  </div>
</template>
