// 展示层格式化工具：日期统一 yyyy-MM-dd HH:mm:ss（client.md 规则）

export function fmtBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`
}

export function fmtRate(bps: number): string {
  return `${fmtBytes(bps)}/s`
}

export function fmtPercent(p: number): string {
  if (!Number.isFinite(p)) return '-'
  return `${p.toFixed(1)}%`
}

export function fmtDateTime(input: string | number | Date): string {
  const d = new Date(input)
  if (Number.isNaN(d.getTime())) return '-'
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function fmtUptime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m`
  return `${Math.floor(sec)}s`
}

export function severityOf(percent: number): 'ok' | 'warn' | 'danger' {
  if (percent >= 90) return 'danger'
  if (percent >= 70) return 'warn'
  return 'ok'
}
