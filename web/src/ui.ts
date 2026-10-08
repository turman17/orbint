// DOM side of the HUD. No framework: the markup lives in index.html and this
// module fills it, wires events, and exposes a small imperative API to main.ts.

import type { Candidate, CandidateKind, Elements } from './api'
import {
  fmtAge,
  fmtDeg,
  fmtKm,
  fmtLat,
  fmtLon,
  fmtNum,
  fmtOffset,
  fmtPeriod,
  fmtSci,
  fmtUtc,
} from './format'
import { REGIME_LABEL, type Regime, type Tracked } from './orbit'
import { REGIME_COLORS } from './satellites'

export interface UIHandlers {
  onSelect(id: number): void
  onHover(id: number | null): void
  onClose(): void
  onPlayToggle(): void
  onSpeed(multiplier: number): void
  onNow(): void
  onFollowToggle(): void
  onGroundTrackToggle(): void
}

export type StatusState = 'loading' | 'ok' | 'error'

const REGIME_ORDER: Regime[] = ['LEO', 'MEO', 'GEO', 'HEO']

const KIND_LABEL: Record<CandidateKind, string> = {
  candidate_delta_v: 'Δv (mean motion)',
  candidate_plane_change: 'Plane change',
  candidate_drag_anomaly: 'Drag anomaly',
  candidate_ecc_change: 'Eccentricity change',
}

const KIND_SHORT: Record<CandidateKind, string> = {
  candidate_delta_v: 'Δv',
  candidate_plane_change: 'plane',
  candidate_drag_anomaly: 'drag',
  candidate_ecc_change: 'ecc',
}

/** One-line evidence summary for a candidate: "step −0.0073 rev/d" or "z 42". */
function evidence(c: Candidate): string {
  const primary = c.Signals.find((s) => s.Rule === 'jump' && s.Kind === c.Kind)
  if (primary) {
    const unit = c.Kind === 'candidate_plane_change' ? '°' : ' rev/d'
    return `step ${primary.Value >= 0 ? '+' : '−'}${fmtNum(Math.abs(primary.Value), c.Kind === 'candidate_plane_change' ? 3 : 4)}${unit}`
  }
  return Number.isFinite(c.ZScore) ? `z ${fmtNum(c.ZScore, 1)}` : 'z —'
}

interface SparkPoint {
  t: number
  v: number
}

function $<T extends HTMLElement = HTMLElement>(id: string): T {
  const el = document.getElementById(id)
  if (!el) throw new Error(`missing #${id}`)
  return el as T
}

function text(id: string, value: string): void {
  $(id).textContent = value
}

export function createUI(handlers: UIHandlers) {
  const listEl = $<HTMLUListElement>('list')
  const searchEl = $<HTMLInputElement>('search')
  const detailsEl = $('details')
  const tooltipEl = $('tooltip')
  const sparkEl = $('spark')
  const toastEl = $('toast')

  let all: Tracked[] = []
  let filter = ''
  let selectedId: number | null = null
  let hoveredId: number | null = null
  const rows = new Map<number, HTMLLIElement>()
  let toastTimer = 0
  // History and candidates arrive separately; the sparkline draws from both.
  let sparkPoints: SparkPoint[] = []
  let sparkCandidates: Candidate[] = []

  // ----- Legend -----
  const legend = $('legend')
  for (const r of REGIME_ORDER) {
    const item = document.createElement('span')
    item.className = 'legend-item'
    item.style.setProperty('--regime', REGIME_COLORS[r])
    item.title = REGIME_LABEL[r]
    const dot = document.createElement('i')
    const name = document.createElement('span')
    name.textContent = r
    const count = document.createElement('span')
    count.className = 'mono'
    count.dataset.regime = r
    count.textContent = '0'
    item.append(dot, name, count)
    legend.appendChild(item)
  }

  function renderLegendCounts(): void {
    const counts: Record<Regime, number> = { LEO: 0, MEO: 0, GEO: 0, HEO: 0 }
    for (const t of all) counts[t.summary.regime] += 1
    for (const r of REGIME_ORDER) {
      const el = legend.querySelector<HTMLElement>(`[data-regime="${r}"]`)
      if (el) el.textContent = String(counts[r])
    }
  }

  // ----- Catalog list -----
  function matches(t: Tracked): boolean {
    if (!filter) return true
    return t.name.toLowerCase().includes(filter) || String(t.id).includes(filter)
  }

  function renderList(): void {
    listEl.replaceChildren()
    rows.clear()
    const visible = all.filter(matches)
    if (visible.length === 0) {
      const empty = document.createElement('li')
      empty.className = 'list-empty'
      empty.textContent = all.length === 0 ? 'No objects loaded' : 'No matches'
      listEl.appendChild(empty)
      return
    }
    for (const t of visible) {
      const li = document.createElement('li')
      li.className = 'row'
      li.setAttribute('role', 'option')
      li.tabIndex = 0
      li.dataset.id = String(t.id)
      li.style.setProperty('--regime', REGIME_COLORS[t.summary.regime])
      li.classList.toggle('is-selected', t.id === selectedId)
      li.classList.toggle('is-hovered', t.id === hoveredId)

      const dot = document.createElement('span')
      dot.className = 'row-dot'
      const name = document.createElement('span')
      name.className = 'row-name'
      name.textContent = t.name
      const regime = document.createElement('span')
      regime.className = 'row-regime'
      regime.textContent = t.summary.regime
      const meta = document.createElement('span')
      meta.className = 'row-meta mono'
      meta.textContent = `${t.id} · ${fmtPeriod(t.summary.periodMin)} · ${fmtKm(t.summary.perigeeKm, 0)}`
      li.append(dot, name, regime, meta)
      rows.set(t.id, li)
      listEl.appendChild(li)
    }
  }

  listEl.addEventListener('click', (ev) => {
    const li = (ev.target as HTMLElement).closest<HTMLLIElement>('li.row')
    if (li?.dataset.id) handlers.onSelect(Number(li.dataset.id))
  })
  listEl.addEventListener('keydown', (ev) => {
    if (ev.key !== 'Enter' && ev.key !== ' ') return
    const li = (ev.target as HTMLElement).closest<HTMLLIElement>('li.row')
    if (li?.dataset.id) {
      ev.preventDefault()
      handlers.onSelect(Number(li.dataset.id))
    }
  })
  listEl.addEventListener('pointerover', (ev) => {
    const li = (ev.target as HTMLElement).closest<HTMLLIElement>('li.row')
    handlers.onHover(li?.dataset.id ? Number(li.dataset.id) : null)
  })
  listEl.addEventListener('pointerleave', () => handlers.onHover(null))
  searchEl.addEventListener('input', () => {
    filter = searchEl.value.trim().toLowerCase()
    renderList()
  })

  // ----- Details -----
  $('d-close').addEventListener('click', handlers.onClose)
  $('d-track').addEventListener('click', handlers.onFollowToggle)
  $('d-ground').addEventListener('click', handlers.onGroundTrackToggle)

  function showDetails(t: Tracked | null): void {
    if (!t) {
      detailsEl.classList.add('is-hidden')
      return
    }
    const color = REGIME_COLORS[t.summary.regime]
    const regimeChip = $('d-regime')
    regimeChip.textContent = t.summary.regime
    regimeChip.title = REGIME_LABEL[t.summary.regime]
    regimeChip.style.setProperty('--regime', color)
    text('d-norad', `NORAD ${t.id}`)
    text('d-name', t.name)
    text('d-intl', `${t.el.InternationalDesignator || '—'} · set ${t.el.ElementSetNumber} · rev ${t.el.RevolutionNumber}`)

    const s = t.summary
    text('d-period', fmtPeriod(s.periodMin))
    text('d-inc', fmtDeg(t.el.Inclination, 3))
    text('d-apogee', fmtKm(s.apogeeKm, 0))
    text('d-perigee', fmtKm(s.perigeeKm, 0))
    text('d-ecc', fmtNum(t.el.Eccentricity, 6))
    text('d-mm', `${fmtNum(t.el.MeanMotion, 5)} rev/d`)
    text('d-raan', fmtDeg(t.el.RAAN, 3))
    text('d-argp', fmtDeg(t.el.ArgumentOfPerigee, 3))
    text('d-bstar', fmtSci(t.el.BStar, 3))
    updateAge(t)
    detailsEl.classList.remove('is-hidden')
    updateLive(t)
  }

  function updateAge(t: Tracked): void {
    const age = Date.now() - t.epoch.getTime()
    const el = $('d-age')
    el.textContent = fmtAge(age)
    el.title = `Epoch ${fmtUtc(t.epoch)} UTC`
  }

  function updateLive(t: Tracked): void {
    if (!t.valid) {
      for (const id of ['d-lat', 'd-lon', 'd-alt', 'd-speed']) text(id, '—')
      return
    }
    text('d-lat', fmtLat(t.latDeg))
    text('d-lon', fmtLon(t.lonDeg))
    text('d-alt', fmtKm(t.altKm, 1))
    text('d-speed', `${fmtNum(t.speedKmS, 3)} km/s`)
  }

  // ----- History sparkline + candidate events -----
  function renderHistory(id: number, els: Elements[]): void {
    if (selectedId !== id) return
    sparkPoints = els
      .map((e) => ({ t: new Date(e.Epoch).getTime(), v: e.MeanMotion }))
      .filter((p) => Number.isFinite(p.t) && Number.isFinite(p.v))
      .sort((a, b) => a.t - b.t)
    drawSpark()
  }

  function renderAnomalies(id: number, cands: Candidate[] | null): void {
    if (selectedId !== id) return
    sparkCandidates = cands ?? []
    drawEvents(cands)
    drawSpark()
  }

  function clearAnalysis(): void {
    sparkPoints = []
    sparkCandidates = []
    sparkEl.replaceChildren()
    text('spark-readout', '—')
    text('h-count', '—')
    $('events').replaceChildren()
    text('ev-count', '—')
  }

  function drawEvents(cands: Candidate[] | null): void {
    const list = $('events')
    list.replaceChildren()
    if (cands === null) {
      text('ev-count', 'unavailable')
      const empty = document.createElement('li')
      empty.className = 'events-empty'
      empty.textContent = 'Detector unavailable'
      list.appendChild(empty)
      return
    }
    text('ev-count', `${cands.length} candidate${cands.length === 1 ? '' : 's'}`)
    if (cands.length === 0) {
      const empty = document.createElement('li')
      empty.className = 'events-empty'
      empty.textContent = sparkPoints.length < 3 ? 'Not enough history to run the detector' : 'Nothing flagged in the window'
      list.appendChild(empty)
      return
    }
    for (const c of cands) {
      const li = document.createElement('li')
      li.className = 'event'
      const dot = document.createElement('span')
      dot.className = 'event-dot'
      const kind = document.createElement('span')
      kind.className = 'event-kind'
      kind.textContent = KIND_LABEL[c.Kind] ?? c.Kind
      const z = document.createElement('span')
      z.className = 'event-z mono'
      z.textContent = evidence(c)
      z.title = `Primary value ${fmtSci(c.Value, 3)}`
      const sigs = document.createElement('span')
      sigs.className = 'event-signals'
      for (const s of c.Signals ?? []) {
        const chip = document.createElement('span')
        chip.className = 'event-sig'
        chip.dataset.rule = s.Rule
        chip.textContent = KIND_SHORT[s.Kind] ?? s.Kind
        chip.title = `${KIND_LABEL[s.Kind] ?? s.Kind} · ${s.Rule === 'jump' ? 'single step' : `z ${fmtNum(s.ZScore, 1)}`} · ${fmtSci(s.Value, 3)}`
        sigs.appendChild(chip)
      }
      const when = document.createElement('span')
      when.className = 'event-when mono'
      const a = new Date(c.EpochStart)
      const b = new Date(c.EpochEnd)
      when.textContent =
        a.getTime() === b.getTime()
          ? `${fmtUtc(a, false)} UTC`
          : `${fmtUtc(a, false)} → ${fmtUtc(b, false)} UTC`
      li.append(dot, kind, z, when, sigs)
      list.appendChild(li)
    }
  }

  function drawSpark(): void {
    const pts = sparkPoints
    text('h-count', `${pts.length} element set${pts.length === 1 ? '' : 's'}`)
    sparkEl.replaceChildren()
    const readout = $('spark-readout')

    if (pts.length < 2) {
      const empty = document.createElement('div')
      empty.className = 'spark-empty'
      empty.textContent = pts.length === 0 ? 'No history in the last 30 days' : 'One element set; need two for a trend'
      sparkEl.appendChild(empty)
      readout.textContent = pts.length === 1 ? `${fmtNum(pts[0].v, 6)} rev/d at ${fmtUtc(new Date(pts[0].t), false)}` : '—'
      return
    }

    const W = Math.max(sparkEl.clientWidth, 200)
    const H = 56
    const padX = 4
    const padY = 6
    const t0 = pts[0].t
    const t1 = pts[pts.length - 1].t
    let vMin = Infinity
    let vMax = -Infinity
    for (const p of pts) {
      vMin = Math.min(vMin, p.v)
      vMax = Math.max(vMax, p.v)
    }
    if (vMax - vMin < 1e-9) {
      vMin -= 1e-6
      vMax += 1e-6
    }
    const x = (t: number) => padX + ((t - t0) / Math.max(t1 - t0, 1)) * (W - padX * 2)
    const y = (v: number) => padY + (1 - (v - vMin) / (vMax - vMin)) * (H - padY * 2)
    const xy = pts.map((p) => [x(p.t), y(p.v)] as const)

    const svgNS = 'http://www.w3.org/2000/svg'
    const el = (tag: string, attrs: Record<string, string>) => {
      const node = document.createElementNS(svgNS, tag)
      for (const [k, v] of Object.entries(attrs)) node.setAttribute(k, v)
      return node
    }
    const svg = el('svg', { viewBox: `0 0 ${W} ${H}`, width: String(W), height: String(H) })

    // Candidate runs as shaded bands behind the line, clamped to the window.
    for (const c of sparkCandidates) {
      const a = Math.max(new Date(c.EpochStart).getTime(), t0)
      const b = Math.min(new Date(c.EpochEnd).getTime(), t1)
      if (!Number.isFinite(a) || !Number.isFinite(b) || b < a) continue
      const xa = x(a)
      const xb = Math.max(x(b), xa + 2)
      svg.appendChild(el('rect', { class: 'spark-band', x: xa.toFixed(1), y: '0', width: (xb - xa).toFixed(1), height: String(H) }))
      svg.appendChild(el('line', { class: 'spark-band-edge', x1: xa.toFixed(1), x2: xa.toFixed(1), y1: '0', y2: String(H) }))
    }

    const lineD = xy.map(([px, py], i) => `${i === 0 ? 'M' : 'L'}${px.toFixed(1)} ${py.toFixed(1)}`).join(' ')
    const area = el('path', { class: 'spark-area', d: `${lineD} L${xy[xy.length - 1][0].toFixed(1)} ${H} L${xy[0][0].toFixed(1)} ${H} Z` })
    const line = el('path', { class: 'spark-line', d: lineD })
    const cursor = el('line', { class: 'spark-cursor', y1: '0', y2: String(H) })
    cursor.style.display = 'none'
    const last = xy[xy.length - 1]
    const dot = el('circle', { class: 'spark-dot', r: '3.5', cx: last[0].toFixed(1), cy: last[1].toFixed(1) })
    const hit = el('rect', { class: 'spark-hit', width: String(W), height: String(H) })
    svg.append(area, line, cursor, dot, hit)
    sparkEl.appendChild(svg)

    const delta = pts[pts.length - 1].v - pts[0].v
    const summary = `Δ ${delta >= 0 ? '+' : '−'}${fmtNum(Math.abs(delta), 6)} rev/d over ${fmtAge(t1 - t0)} · latest ${fmtNum(pts[pts.length - 1].v, 5)}`
    readout.textContent = summary

    hit.addEventListener('pointermove', (ev) => {
      const rect = svg.getBoundingClientRect()
      const px = ((ev.clientX - rect.left) / rect.width) * W
      let best = 0
      let bestD = Infinity
      for (let i = 0; i < xy.length; i++) {
        const d = Math.abs(xy[i][0] - px)
        if (d < bestD) {
          bestD = d
          best = i
        }
      }
      cursor.style.display = ''
      cursor.setAttribute('x1', xy[best][0].toFixed(1))
      cursor.setAttribute('x2', xy[best][0].toFixed(1))
      dot.setAttribute('cx', xy[best][0].toFixed(1))
      dot.setAttribute('cy', xy[best][1].toFixed(1))
      readout.textContent = `${fmtUtc(new Date(pts[best].t), false)} UTC · ${fmtNum(pts[best].v, 6)} rev/d`
    })
    hit.addEventListener('pointerleave', () => {
      cursor.style.display = 'none'
      dot.setAttribute('cx', last[0].toFixed(1))
      dot.setAttribute('cy', last[1].toFixed(1))
      readout.textContent = summary
    })
  }

  // ----- Dock -----
  $('play').addEventListener('click', handlers.onPlayToggle)
  $('now').addEventListener('click', handlers.onNow)
  const speedButtons = Array.from($('speeds').querySelectorAll<HTMLButtonElement>('.speed'))
  for (const b of speedButtons) {
    b.addEventListener('click', () => handlers.onSpeed(Number(b.dataset.mult)))
  }

  function setClock(sim: Date, real: Date, multiplier: number, playing: boolean): void {
    const [date, time] = fmtUtc(sim).split(' ')
    text('sim-date', date)
    text('sim-clock', `${time} UTC`)
    const offset = sim.getTime() - real.getTime()
    const chip = $('sim-offset')
    if (Math.abs(offset) > 1500) {
      chip.textContent = fmtOffset(offset)
      chip.classList.remove('is-hidden')
    } else {
      chip.classList.add('is-hidden')
    }
    const play = $('play')
    play.setAttribute('aria-pressed', String(playing))
    play.title = playing ? 'Pause (Space)' : 'Play (Space)'
    play.setAttribute('aria-label', playing ? 'Pause' : 'Play')
    for (const b of speedButtons) {
      b.setAttribute('aria-pressed', String(Number(b.dataset.mult) === multiplier))
    }
  }

  // ----- Status / tooltip / toast / loading -----
  function setStatus(state: StatusState, label: string, count?: number): void {
    $('status-dot').dataset.state = state
    text('status-text', label)
    text('status-count', count === undefined ? '—' : `${count} object${count === 1 ? '' : 's'}`)
  }

  function setUtc(date: Date): void {
    text('status-utc', `${fmtUtc(date)} UTC`)
  }

  function tooltip(x: number, y: number, t: Tracked | null): void {
    if (!t) {
      tooltipEl.classList.add('is-hidden')
      return
    }
    tooltipEl.replaceChildren()
    const name = document.createElement('span')
    name.textContent = t.name
    const alt = document.createElement('span')
    alt.className = 'muted mono'
    alt.textContent = fmtKm(t.altKm, 0)
    tooltipEl.append(name, alt)
    tooltipEl.style.left = `${x}px`
    tooltipEl.style.top = `${y}px`
    tooltipEl.classList.remove('is-hidden')
  }

  /** Show a toast; `ms` 0 keeps it until `hideToast` is called. */
  function toast(message: string, kind: 'error' | 'info' = 'error', ms = 6000): void {
    toastEl.textContent = message
    toastEl.classList.toggle('is-info', kind === 'info')
    toastEl.classList.remove('is-hidden')
    window.clearTimeout(toastTimer)
    if (ms > 0) toastTimer = window.setTimeout(hideToast, ms)
  }

  function hideToast(): void {
    window.clearTimeout(toastTimer)
    toastEl.classList.add('is-hidden')
  }

  function loadingDone(): void {
    const el = $('loading')
    el.classList.add('is-done')
    window.setTimeout(() => el.remove(), 700)
  }

  function loadingText(message: string): void {
    text('loading-text', message)
  }

  return {
    setList(list: Tracked[]): void {
      all = list
      renderLegendCounts()
      renderList()
    },
    setSelected(id: number | null): void {
      if (selectedId !== null) rows.get(selectedId)?.classList.remove('is-selected')
      selectedId = id
      if (id !== null) {
        const li = rows.get(id)
        li?.classList.add('is-selected')
        li?.scrollIntoView({ block: 'nearest' })
      }
    },
    setHovered(id: number | null): void {
      if (hoveredId !== null) rows.get(hoveredId)?.classList.remove('is-hovered')
      hoveredId = id
      if (id !== null) rows.get(id)?.classList.add('is-hovered')
    },
    setFollow(on: boolean): void {
      $('d-track').setAttribute('aria-pressed', String(on))
    },
    setGroundTrack(on: boolean): void {
      $('d-ground').setAttribute('aria-pressed', String(on))
    },
    isTyping(): boolean {
      return document.activeElement === searchEl
    },
    showDetails,
    updateLive,
    updateAge,
    renderHistory,
    renderAnomalies,
    clearAnalysis,
    setClock,
    setStatus,
    setUtc,
    tooltip,
    toast,
    hideToast,
    loadingDone,
    loadingText,
  }
}

export type UI = ReturnType<typeof createUI>
