// Number and time formatting. Everything numeric renders in the monospace
// face with tabular digits, so these helpers only decide precision and units.

const pad = (n: number, width = 2) => String(n).padStart(width, '0')

export function fmtUtc(date: Date, withSeconds = true): string {
  const d = `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`
  const t = `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}`
  return withSeconds ? `${d} ${t}:${pad(date.getUTCSeconds())}` : `${d} ${t}`
}

export function fmtNum(n: number, digits = 2): string {
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

export function fmtDeg(n: number, digits = 2): string {
  return `${fmtNum(n, digits)}°`
}

export function fmtLat(lat: number): string {
  return `${fmtNum(Math.abs(lat), 2)}° ${lat >= 0 ? 'N' : 'S'}`
}

export function fmtLon(lon: number): string {
  return `${fmtNum(Math.abs(lon), 2)}° ${lon >= 0 ? 'E' : 'W'}`
}

export function fmtKm(km: number, digits = 1): string {
  return `${fmtNum(km, digits)} km`
}

/** Minutes → "92.9 min" or "23 h 56 min". */
export function fmtPeriod(minutes: number): string {
  if (!Number.isFinite(minutes)) return '—'
  if (minutes < 180) return `${fmtNum(minutes, 1)} min`
  const h = Math.floor(minutes / 60)
  const m = Math.round(minutes - h * 60)
  return `${h} h ${pad(m)} min`
}

/** Milliseconds → "42 s", "18 min", "3.2 h", "4.1 d". */
export function fmtAge(ms: number): string {
  const s = Math.abs(ms) / 1000
  if (s < 60) return `${Math.round(s)} s`
  if (s < 3600) return `${Math.round(s / 60)} min`
  if (s < 86_400) return `${fmtNum(s / 3600, 1)} h`
  return `${fmtNum(s / 86_400, 1)} d`
}

/** Signed offset of simulated time from wall-clock time, e.g. "+3 h 12 min". */
export function fmtOffset(ms: number): string {
  const sign = ms >= 0 ? '+' : '−'
  const s = Math.abs(ms) / 1000
  if (s < 60) return `${sign}${Math.round(s)} s`
  if (s < 3600) return `${sign}${Math.floor(s / 60)} min ${pad(Math.round(s % 60))} s`
  if (s < 86_400) return `${sign}${Math.floor(s / 3600)} h ${pad(Math.floor((s % 3600) / 60))} min`
  return `${sign}${fmtNum(s / 86_400, 1)} d`
}

export function fmtSci(n: number, digits = 3): string {
  if (!Number.isFinite(n)) return '—'
  if (n === 0) return '0'
  return n.toExponential(digits).replace('e', ' e')
}

export function fmtMultiplier(m: number): string {
  return m >= 1 ? `${m}×` : `${fmtNum(m, 2)}×`
}
