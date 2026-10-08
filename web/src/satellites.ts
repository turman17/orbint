// Renders tracked objects with Cesium primitives (not entities) so the layer
// scales to thousands of points: one PointPrimitiveCollection for markers,
// one LabelCollection for names, one PolylineCollection for the selected
// object's orbit ring and ground track.

import {
  BoundingRectangle,
  Cartesian2,
  Cartesian3,
  Color,
  Ellipsoid,
  EllipsoidalOccluder,
  HorizontalOrigin,
  Intersect,
  LabelCollection,
  LabelStyle,
  Material,
  NearFarScalar,
  PointPrimitiveCollection,
  PolylineCollection,
  SceneTransforms,
  VerticalOrigin,
  type Label,
  type PointPrimitive,
  type Polyline,
  type Scene,
} from 'cesium'
import {
  eciRingToFixed,
  gstime,
  sampleGroundTrack,
  sampleOrbitEci,
  updateLive,
  type Regime,
  type Tracked,
} from './orbit'

/** Validated categorical palette (dark surface #05080f). Mirrors style.css. */
export const REGIME_COLORS: Record<Regime, string> = {
  LEO: '#2c8fd4',
  MEO: '#c28217',
  GEO: '#c3558c',
  HEO: '#25a25f',
}

const LABEL_FONT = '500 12px "JetBrains Mono", ui-monospace, Menlo, monospace'
/**
 * Above this many objects, names render only on hover and selection. Below
 * it, the declutter pass decides which names fit, so a thousand objects
 * still read as a sparse set of labels rather than a wall of text.
 */
const LABEL_ALL_LIMIT = 1500
/** Sim-time staleness (ms) before the orbit ring / ground track are resampled. */
const RING_REFRESH_MS = 120_000
const TRACK_REFRESH_MS = 45_000
/** Wall-clock cadence (ms) of the label collision pass. */
const DECLUTTER_MS = 200
/**
 * Wall-clock cadence (ms) for re-uploading the orbit ring. Each upload rebuilds
 * a vertex buffer; the ring only rotates with Earth (0.004°/s), so once a
 * second is visually identical to every frame and far cheaper.
 */
const RING_UPLOAD_MS = 1000
/** Approximate glyph advance and line height of LABEL_FONT, in CSS px. */
const LABEL_CHAR_W = 7.2
const LABEL_H = 16

interface Entry {
  t: Tracked
  point: PointPrimitive
  label: Label
  color: Color
}

const POINT_COLORS = Object.fromEntries(
  (Object.keys(REGIME_COLORS) as Regime[]).map((r) => {
    const base = Color.fromCssColorString(REGIME_COLORS[r])
    return [r, base.brighten(0.22, new Color())]
  }),
) as Record<Regime, Color>

export class SatelliteLayer {
  private readonly points: PointPrimitiveCollection
  private readonly labels: LabelCollection
  private readonly lines: PolylineCollection
  private readonly entries = new Map<number, Entry>()
  private readonly halo: PointPrimitive

  private ring: Polyline
  private track: Polyline
  private ringEci: Float64Array | null = null
  private ringSampledAt = 0
  private ringUploadedAt = 0
  private trackSampledAt = 0

  private selectedId: number | null = null
  private hoveredId: number | null = null
  private lastDeclutter = 0
  private readonly occluder = new EllipsoidalOccluder(Ellipsoid.WGS84, Cartesian3.ZERO)
  private readonly scratchWindow = new Cartesian2()
  showGroundTrack = true

  private readonly scene: Scene

  constructor(scene: Scene) {
    this.scene = scene
    this.points = scene.primitives.add(new PointPrimitiveCollection())
    this.labels = scene.primitives.add(new LabelCollection())
    this.lines = scene.primitives.add(new PolylineCollection())

    this.halo = this.points.add({
      show: false,
      pixelSize: 22,
      color: Color.WHITE.withAlpha(0.18),
      outlineWidth: 0,
      id: -1,
    })

    this.ring = this.lines.add({
      show: false,
      width: 2.5,
      positions: [],
      material: Material.fromType(Material.PolylineGlowType, {
        color: Color.WHITE,
        glowPower: 0.18,
        taperPower: 1.0,
      }),
    })
    this.track = this.lines.add({
      show: false,
      width: 1.5,
      positions: [],
      material: Material.fromType(Material.PolylineDashType, {
        color: Color.WHITE.withAlpha(0.55),
        gapColor: Color.TRANSPARENT,
        dashLength: 14,
      }),
    })
  }

  get count(): number {
    return this.entries.size
  }

  get selected(): Tracked | null {
    return this.selectedId === null ? null : (this.entries.get(this.selectedId)?.t ?? null)
  }

  /** Replace the tracked set; existing objects keep their primitives. */
  setSatellites(list: Tracked[]): void {
    const seen = new Set<number>()
    for (const t of list) {
      seen.add(t.id)
      const existing = this.entries.get(t.id)
      if (existing) {
        existing.t = t
        existing.label.text = t.name
        continue
      }
      const color = POINT_COLORS[t.summary.regime]
      const point = this.points.add({
        position: Cartesian3.ZERO,
        show: false,
        pixelSize: 7,
        color,
        outlineColor: Color.BLACK.withAlpha(0.65),
        outlineWidth: 1.5,
        scaleByDistance: new NearFarScalar(2.0e6, 1.35, 9.0e7, 0.8),
        id: t.id,
      })
      const label = this.labels.add({
        position: Cartesian3.ZERO,
        show: false,
        text: t.name,
        font: LABEL_FONT,
        style: LabelStyle.FILL_AND_OUTLINE,
        fillColor: Color.fromCssColorString('#dfe7f2'),
        outlineColor: Color.fromCssColorString('#05080f').withAlpha(0.9),
        outlineWidth: 3,
        horizontalOrigin: HorizontalOrigin.LEFT,
        verticalOrigin: VerticalOrigin.CENTER,
        pixelOffset: new Cartesian2(11, 0),
        translucencyByDistance: new NearFarScalar(1.5e7, 1.0, 1.0e8, 0.35),
        id: t.id,
      })
      this.entries.set(t.id, { t, point, label, color })
    }
    for (const [id, e] of this.entries) {
      if (seen.has(id)) continue
      this.points.remove(e.point)
      this.labels.remove(e.label)
      this.entries.delete(id)
      if (this.selectedId === id) this.setSelected(null)
      if (this.hoveredId === id) this.hoveredId = null
    }
    this.ringEci = null
    this.restyleAll()
  }

  /** Propagate everything to `date` and move the primitives. */
  update(date: Date): void {
    const gmst = gstime(date)
    for (const e of this.entries.values()) {
      const ok = updateLive(e.t, date, gmst)
      e.point.show = ok
      e.point.position = e.t.position
      e.label.position = e.t.position
      if (!ok) e.label.show = false
    }
    this.updateSelection(date, gmst)

    const now = performance.now()
    if (now - this.lastDeclutter >= DECLUTTER_MS) {
      this.lastDeclutter = now
      this.declutterLabels()
    }
  }

  /**
   * Greedy label placement: selected and hovered objects always win, then
   * objects in catalog order; any label whose box would overlap one already
   * placed is hidden. Objects behind the globe are skipped. Co-located
   * objects (a station and its visiting vehicles) collapse to one name.
   */
  private declutterLabels(): void {
    const candidates: Entry[] = []
    for (const e of this.entries.values()) {
      if (e.t.valid && this.labelVisible(e.t.id)) candidates.push(e)
      else e.label.show = false
    }
    if (candidates.length === 0) return

    candidates.sort((a, b) => this.labelPriority(a) - this.labelPriority(b))
    this.occluder.cameraPosition = this.scene.camera.positionWC
    const placed: BoundingRectangle[] = []

    for (const e of candidates) {
      if (!this.occluder.isPointVisible(e.t.position)) {
        e.label.show = false
        continue
      }
      const win = SceneTransforms.worldToWindowCoordinates(this.scene, e.t.position, this.scratchWindow)
      if (!win) {
        e.label.show = false
        continue
      }
      const w = e.t.name.length * LABEL_CHAR_W + 14
      const box = new BoundingRectangle(win.x, win.y - LABEL_H / 2, w, LABEL_H)
      const collides = placed.some((p) => BoundingRectangle.intersect(p, box) !== Intersect.OUTSIDE)
      e.label.show = !collides
      if (!collides) placed.push(box)
    }
  }

  private labelPriority(e: Entry): number {
    if (e.t.id === this.selectedId) return 0
    if (e.t.id === this.hoveredId) return 1
    return 2
  }

  /** Object under a window position, or null. */
  pick(windowPosition: Cartesian2): Tracked | null {
    const picked = this.scene.pick(windowPosition) as { id?: unknown; collection?: unknown } | undefined
    if (!picked || picked.collection !== this.points || typeof picked.id !== 'number') return null
    return this.entries.get(picked.id)?.t ?? null
  }

  setHovered(id: number | null): void {
    if (id === this.hoveredId) return
    const prev = this.hoveredId
    this.hoveredId = id
    if (prev !== null) this.restyle(prev)
    if (id !== null) this.restyle(id)
  }

  setSelected(id: number | null): void {
    if (id === this.selectedId) return
    const prev = this.selectedId
    this.selectedId = id
    this.ringEci = null
    this.ringUploadedAt = 0
    this.trackSampledAt = 0
    if (prev !== null) this.restyle(prev)
    if (id !== null) {
      this.restyle(id)
      const e = this.entries.get(id)
      if (e) {
        ;(this.ring.material.uniforms as { color: Color }).color = e.color
        ;(this.track.material.uniforms as { color: Color }).color = e.color.withAlpha(0.6)
        this.halo.color = e.color.withAlpha(0.2)
      }
    }
    this.halo.show = id !== null
    this.ring.show = id !== null
    this.track.show = id !== null && this.showGroundTrack
  }

  private updateSelection(date: Date, gmst: number): void {
    const e = this.selectedId === null ? undefined : this.entries.get(this.selectedId)
    if (!e || !e.t.valid) {
      this.halo.show = false
      this.ring.show = false
      this.track.show = false
      return
    }
    this.halo.show = true
    this.halo.position = e.t.position
    // Gentle pulse, in wall-clock time so it reads the same at any sim speed.
    this.halo.pixelSize = 20 + 4 * Math.sin(performance.now() / 420)

    const simMs = date.getTime()
    const wall = performance.now()
    let resampled = false
    if (!this.ringEci || Math.abs(simMs - this.ringSampledAt) > RING_REFRESH_MS) {
      this.ringEci = sampleOrbitEci(e.t, date)
      this.ringSampledAt = simMs
      resampled = true
    }
    if (this.ringEci) {
      if (resampled || wall - this.ringUploadedAt > RING_UPLOAD_MS) {
        this.ring.positions = eciRingToFixed(this.ringEci, gmst)
        this.ringUploadedAt = wall
      }
      this.ring.show = true
    } else {
      this.ring.show = false
    }

    if (this.showGroundTrack) {
      if (Math.abs(simMs - this.trackSampledAt) > TRACK_REFRESH_MS) {
        const positions = sampleGroundTrack(e.t, date)
        this.trackSampledAt = simMs
        if (positions) this.track.positions = positions
        this.track.show = positions !== null
      }
    } else {
      this.track.show = false
    }
  }

  private labelVisible(id: number): boolean {
    if (id === this.selectedId || id === this.hoveredId) return true
    return this.entries.size <= LABEL_ALL_LIMIT
  }

  private restyleAll(): void {
    for (const id of this.entries.keys()) this.restyle(id)
  }

  private restyle(id: number): void {
    const e = this.entries.get(id)
    if (!e) return
    const selected = id === this.selectedId
    const hovered = id === this.hoveredId
    e.point.pixelSize = selected ? 10 : hovered ? 9 : 7
    e.point.color = selected || hovered ? Color.WHITE : e.color
    e.point.outlineColor = selected || hovered ? e.color : Color.BLACK.withAlpha(0.65)
    e.point.outlineWidth = selected ? 3 : hovered ? 2 : 1.5
    e.label.fillColor = selected ? Color.WHITE : Color.fromCssColorString('#dfe7f2')
    e.label.show = e.t.valid && this.labelVisible(id)
  }
}
