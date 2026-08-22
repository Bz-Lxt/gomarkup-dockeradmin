// API 契约类型（与 docs/Roadmap.md §1 冻结契约一致）

export interface Health {
  status: string
  version: string
  docker: 'connected' | 'degraded'
  uptime_sec: number
  collect_interval: string
}

export interface CpuMetric {
  percent: number
  per_core: number[]
}

export interface MemMetric {
  total: number
  used: number
  percent: number
}

export interface DiskMetric {
  mount: string
  total: number
  used: number
  percent: number
}

export interface NetMetric {
  iface: string
  rx_bps: number
  tx_bps: number
}

export interface MetricSnapshot {
  ts: string
  cpu: CpuMetric
  mem: MemMetric
  disk: DiskMetric[]
  net: NetMetric[]
  load: [number, number, number]
  procs: number
}

export interface ContainerInfo {
  id: string
  name: string
  image: string
  state: string
  status: string
  cpu_percent: number
  mem_used: number
  mem_limit: number
  mem_percent: number
  net_rx: number
  net_tx: number
  created_at: string
  uptime_sec: number
}

export interface ContainerDetail extends ContainerInfo {
  ports: string[]
  mounts: string[]
  env_preview: string[]
}

export type MetricKind =
  | 'cpu_percent'
  | 'mem_percent'
  | 'disk_percent'
  | 'net_rx_bps'
  | 'net_tx_bps'
  | 'container_cpu_percent'
  | 'container_mem_percent'

export type AlertOp = '>' | '>=' | '<' | '<='

export interface AlertRule {
  id: string
  name: string
  metric: MetricKind
  target: string
  op: AlertOp
  threshold: number
  duration_sec: number
  cooldown_sec: number
  enabled: boolean
  webhook_url: string
  notify_recovery: boolean
  created_at: string
  updated_at: string
}

export interface AlertEvent {
  id: string
  rule_id: string
  rule_name: string
  metric: string
  target: string
  value: number
  threshold: number
  op: string
  kind: 'fired' | 'recovered'
  webhook_status: number
  webhook_error: string
  fired_at: string
}

export interface WebhookReceipt {
  id: number
  received_at: string
  payload: string
}

export interface ApiError {
  code: string
  message: string
  details?: { field: string; message: string }[]
}
