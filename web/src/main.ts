import './style.css'
import 'cesium/Build/Cesium/Widgets/widgets.css'
import {
  Cartesian3,
  ClockStep,
  EasingFunction,
  HeadingPitchRange,
  JulianDate,
  Math as CesiumMath,
  Matrix4,
  ScreenSpaceEventHandler,
  ScreenSpaceEventType,
  Transforms,
  type Cartesian2,
} from 'cesium'
import { fetchAnomalies, fetchHistory, fetchSatellites } from './api'
import { track, type Tracked } from './orbit'
import { SatelliteLayer } from './satellites'
import { QUALITY_ORDER, applyQuality, createViewer, playIntro, type Quality } from './scene'
import { createUI, type UIHandlers } from './ui'

/** Re-read the catalog this often; ingestion runs every two hours. */
const CATALOG_REFRESH_MS = 10 * 60_000
/** Retry cadence while the API is unreachable. */
const RETRY_MS = 15_000
/** Seconds without input before the globe starts to turn on its own. */
const IDLE_ROTATE_AFTER_S = 8
/** Idle rotation rate, radians per second (one turn in ~10 minutes). */
const IDLE_ROTATE_RATE = 0.0105
/** DOM refresh cadence for live numbers. */
const UI_TICK_MS = 250
/** Adaptive quality: evaluate frame times over windows of this length. */
const QUALITY_WINDOW_MS = 3000
/** Step quality down below this average fps, up above the other one. */
const FPS_STEP_DOWN = 30
const FPS_STEP_UP = 56

function main(): void {
  const globeEl = document.getElementById('globe')!
  const creditsEl = document.getElementById('credits')!

  const state = {
    tracked: [] as Tracked[],
    byId: new Map<number, Tracked>(),
    selectedId: null as number | null,
    follow: false,
    followInitialised: false,
    lastInteraction: performance.now(),
    lastFrame: performance.now(),
    lastUi: 0,
  }

  const handlers: UIHandlers = {
    onSelect: (id) => select(id, { fly: true }),
    onHover: (id) => hover(id),
    onClose: () => select(null),
    onPlayToggle: () => {
      viewer.clock.shouldAnimate = !viewer.clock.shouldAnimate
    },
    onSpeed: (m) => {
      viewer.clock.multiplier = m
      if (!viewer.clock.shouldAnimate) viewer.clock.shouldAnimate = true
    },
    onNow: () => {
      viewer.clock.currentTime = JulianDate.now()
      viewer.clock.multiplier = 1
      viewer.clock.shouldAnimate = true
      // Lock to wall-clock time; Cesium drops back to multiplier mode on its
      // own as soon as the speed changes or the clock is paused.
      viewer.clock.clockStep = ClockStep.SYSTEM_CLOCK
    },
    onFollowToggle: () => setFollow(!state.follow),
    onGroundTrackToggle: () => {
      layer.showGroundTrack = !layer.showGroundTrack
      ui.setGroundTrack(layer.showGroundTrack)
      // Re-apply so the polyline shows/hides immediately.
      const id = state.selectedId
      layer.setSelected(null)
      layer.setSelected(id)
    },
  }

  const ui = createUI(handlers)
  const viewer = createViewer(globeEl, creditsEl)
  const layer = new SatelliteLayer(viewer.scene)
  const { camera } = viewer
  viewer.clock.clockStep = ClockStep.SYSTEM_CLOCK

  // ----- Selection & camera -----

  function select(id: number | null, opts: { fly?: boolean } = {}): void {
    if (id !== null && !state.byId.has(id)) return
    const changed = id !== state.selectedId
    state.selectedId = id
    layer.setSelected(id)
    ui.setSelected(id)
    const t = id === null ? null : state.byId.get(id)!
    ui.showDetails(t)

    if (!t) {
      setFollow(false)
      return
    }
    if (changed) {
      ui.clearAnalysis()
      // History draws the sparkline; the detector's candidates are listed
      // and shaded onto it. Either may fail independently.
      void fetchHistory(t.id)
        .then((els) => ui.renderHistory(t.id, els))
        .catch(() => ui.renderHistory(t.id, []))
      void fetchAnomalies(t.id)
        .then((cands) => ui.renderAnomalies(t.id, cands))
        .catch((err) => {
          console.warn('[orbint] anomalies unavailable', err)
          ui.renderAnomalies(t.id, null)
        })
    }
    if (state.follow) {
      state.followInitialised = false
    } else if (opts.fly && t.valid) {
      flyTo(t)
    }
  }

  function hover(id: number | null): void {
    layer.setHovered(id)
    ui.setHovered(id)
  }

  function flyTo(t: Tracked): void {
    const height = CesiumMath.clamp(t.altKm * 1000 * 4, 9.0e6, 4.0e7)
    camera.flyTo({
      destination: Cartesian3.fromDegrees(t.lonDeg, t.latDeg, height),
      duration: 1.6,
      easingFunction: EasingFunction.QUADRATIC_IN_OUT,
    })
  }

  function setFollow(on: boolean): void {
    state.follow = on && state.selectedId !== null
    state.followInitialised = false
    ui.setFollow(state.follow)
    if (!state.follow) camera.lookAtTransform(Matrix4.IDENTITY)
  }

  function followSelected(t: Tracked): void {
    const transform = Transforms.eastNorthUpToFixedFrame(t.position)
    if (!state.followInitialised) {
      const range = CesiumMath.clamp(t.altKm * 1000 * 2.4, 1.2e6, 2.5e7)
      camera.lookAtTransform(
        transform,
        new HeadingPitchRange(CesiumMath.toRadians(25), CesiumMath.toRadians(-32), range),
      )
      state.followInitialised = true
      return
    }
    // camera.position is expressed in the previous transform's frame, so
    // reusing it keeps whatever orbit/zoom the user has applied.
    camera.lookAtTransform(transform, Cartesian3.clone(camera.position))
  }

  function idleRotate(dtSeconds: number): void {
    const idle = (performance.now() - state.lastInteraction) / 1000
    if (idle < IDLE_ROTATE_AFTER_S || state.selectedId !== null) return
    const rampUp = Math.min(1, (idle - IDLE_ROTATE_AFTER_S) / 3)
    camera.rotate(Cartesian3.UNIT_Z, -IDLE_ROTATE_RATE * rampUp * dtSeconds)
  }

  // ----- Input -----

  const noteInteraction = () => {
    state.lastInteraction = performance.now()
  }
  for (const ev of ['pointerdown', 'wheel', 'touchstart'] as const) {
    viewer.canvas.addEventListener(ev, noteInteraction, { passive: true })
  }
  window.addEventListener('keydown', (ev) => {
    noteInteraction()
    if (ui.isTyping()) return
    if (ev.key === ' ') {
      ev.preventDefault()
      handlers.onPlayToggle()
    } else if (ev.key === 'Escape') {
      select(null)
    } else if (ev.key.toLowerCase() === 'f' && state.selectedId !== null) {
      setFollow(!state.follow)
    }
  })

  const input = new ScreenSpaceEventHandler(viewer.canvas)
  input.setInputAction((movement: { endPosition: Cartesian2 }) => {
    const t = layer.pick(movement.endPosition)
    hover(t?.id ?? null)
    viewer.canvas.style.cursor = t ? 'pointer' : ''
    ui.tooltip(movement.endPosition.x, movement.endPosition.y, t)
  }, ScreenSpaceEventType.MOUSE_MOVE)
  input.setInputAction((click: { position: Cartesian2 }) => {
    const t = layer.pick(click.position)
    select(t ? t.id : null)
  }, ScreenSpaceEventType.LEFT_CLICK)

  // ----- Clock -----

  viewer.clock.onTick.addEventListener((clock) => {
    const now = performance.now()
    const dt = Math.min((now - state.lastFrame) / 1000, 0.1)
    state.lastFrame = now

    const sim = JulianDate.toDate(clock.currentTime)
    layer.update(sim)

    const selected = state.selectedId === null ? null : state.byId.get(state.selectedId) ?? null
    if (state.follow && selected?.valid) followSelected(selected)
    else idleRotate(dt)

    if (now - state.lastUi >= UI_TICK_MS) {
      state.lastUi = now
      ui.setClock(sim, new Date(), clock.multiplier, clock.shouldAnimate)
      ui.setUtc(new Date())
      if (selected) {
        ui.updateLive(selected)
        ui.updateAge(selected)
      }
    }
  })

  // ----- Adaptive quality -----
  // Start at 'medium'; climb to 'high' (Retina resolution + bloom) only when
  // the GPU proves it can hold a smooth frame rate, drop to 'low' if it can't.
  const quality = { tier: 'medium' as Quality, frames: 0, windowStart: performance.now(), locked: false }
  viewer.scene.postRender.addEventListener(() => {
    quality.frames += 1
    const elapsed = performance.now() - quality.windowStart
    if (elapsed < QUALITY_WINDOW_MS) return
    const fps = (quality.frames * 1000) / elapsed
    quality.frames = 0
    quality.windowStart = performance.now()
    if (quality.locked || document.hidden) return
    const i = QUALITY_ORDER.indexOf(quality.tier)
    if (fps < FPS_STEP_DOWN && i > 0) {
      quality.tier = QUALITY_ORDER[i - 1]
      quality.locked = true // never bounce back up after a downgrade
      applyQuality(viewer, quality.tier)
      console.info(`[orbint] quality → ${quality.tier} (${fps.toFixed(0)} fps)`)
    } else if (fps > FPS_STEP_UP && i < QUALITY_ORDER.length - 1) {
      quality.tier = QUALITY_ORDER[i + 1]
      applyQuality(viewer, quality.tier)
      console.info(`[orbint] quality → ${quality.tier} (${fps.toFixed(0)} fps)`)
    }
  })

  // ----- Data -----

  async function loadCatalog(): Promise<boolean> {
    try {
      const elements = await fetchSatellites()
      const tracked = elements.map(track)
      state.tracked = tracked
      state.byId = new Map(tracked.map((t) => [t.id, t]))
      layer.setSatellites(tracked)
      ui.setList(tracked)
      ui.setStatus('ok', 'CelesTrak · live', tracked.length)
      if (state.selectedId !== null) {
        const t = state.byId.get(state.selectedId)
        if (t) ui.showDetails(t)
        else select(null)
      }
      return true
    } catch (err) {
      console.error('[orbint] catalog load failed', err)
      ui.setStatus('error', 'API unreachable')
      return false
    }
  }

  async function boot(): Promise<void> {
    ui.loadingText('Loading typefaces')
    const fonts = Promise.race([
      Promise.all([
        document.fonts.load('500 12px "JetBrains Mono"'),
        document.fonts.load('500 13px "Inter"'),
      ]),
      new Promise((r) => setTimeout(r, 1500)),
    ])
    ui.loadingText('Fetching element sets')
    ui.setList([])
    const [ok] = await Promise.all([loadCatalog(), fonts])
    layer.setSatellites(state.tracked) // re-apply so labels pick up the loaded font

    playIntro(viewer, new Date())
    ui.loadingDone()

    if (!ok) {
      ui.toast('Could not reach the API at /api/satellites. Is cmd/api running on :8090? Retrying…', 'error', 0)
      const retry = window.setInterval(async () => {
        if (await loadCatalog()) {
          window.clearInterval(retry)
          ui.toast('Connected to API', 'info', 2500)
        }
      }, RETRY_MS)
    }
    window.setInterval(() => void loadCatalog(), CATALOG_REFRESH_MS)
  }

  void boot()
}

main()
