// Client-side SGP4 via satellite.js. The API returns parsed elements (the Go
// orbit.TLE struct), which satellite.js accepts directly as an OMM object, so
// every satellite is propagated in the browser: no per-object network call,
// and the scene can animate at any clock speed.

import { Cartesian3 } from 'cesium'
import {
  json2satrec,
  propagate,
  gstime,
  eciToEcf,
  eciToGeodetic,
  radiansToDegrees,
  type SatRec,
  type OMMJsonObject,
} from 'satellite.js'
import type { Elements } from './api'

export const EARTH_RADIUS_KM = 6378.137
/** Earth gravitational parameter, km³/s². */
const MU = 398_600.4418
/** Sidereal day in minutes: the period of a geosynchronous orbit. */
const SIDEREAL_DAY_MIN = 1436.07

export type Regime = 'LEO' | 'MEO' | 'GEO' | 'HEO'

export const REGIME_LABEL: Record<Regime, string> = {
  LEO: 'Low Earth orbit',
  MEO: 'Medium Earth orbit',
  GEO: 'Geosynchronous',
  HEO: 'Highly elliptical',
}

export interface OrbitSummary {
  periodMin: number
  semiMajorKm: number
  apogeeKm: number
  perigeeKm: number
  regime: Regime
}

/** One tracked object: its elements, its SGP4 record and its live state. */
export interface Tracked {
  id: number
  name: string
  el: Elements
  satrec: SatRec
  epoch: Date
  summary: OrbitSummary
  // Live state, written by updateLive on every clock tick.
  position: Cartesian3
  latDeg: number
  lonDeg: number
  altKm: number
  speedKmS: number
  /** false when SGP4 reports an error (decayed object, bad elements). */
  valid: boolean
}

export function summarize(el: Elements): OrbitSummary {
  const n = (el.MeanMotion * 2 * Math.PI) / 86_400 // rad/s
  const a = Math.cbrt(MU / (n * n))
  const e = el.Eccentricity
  const periodMin = 1440 / el.MeanMotion
  const apogeeKm = a * (1 + e) - EARTH_RADIUS_KM
  const perigeeKm = a * (1 - e) - EARTH_RADIUS_KM

  let regime: Regime
  if (e >= 0.25) regime = 'HEO'
  else if (Math.abs(periodMin - SIDEREAL_DAY_MIN) < 30) regime = 'GEO'
  else if (apogeeKm < 2000) regime = 'LEO'
  else regime = 'MEO'

  return { periodMin, semiMajorKm: a, apogeeKm, perigeeKm, regime }
}

function classification(code: number): 'U' | 'C' {
  return String.fromCharCode(code) === 'C' ? 'C' : 'U'
}

export function toSatrec(el: Elements): SatRec {
  const omm: OMMJsonObject = {
    OBJECT_NAME: el.Name,
    OBJECT_ID: el.InternationalDesignator,
    EPOCH: el.Epoch,
    MEAN_MOTION: el.MeanMotion,
    ECCENTRICITY: el.Eccentricity,
    INCLINATION: el.Inclination,
    RA_OF_ASC_NODE: el.RAAN,
    ARG_OF_PERICENTER: el.ArgumentOfPerigee,
    MEAN_ANOMALY: el.MeanAnomaly,
    EPHEMERIS_TYPE: 0,
    CLASSIFICATION_TYPE: classification(el.Class),
    NORAD_CAT_ID: el.ID,
    ELEMENT_SET_NO: el.ElementSetNumber,
    REV_AT_EPOCH: el.RevolutionNumber,
    BSTAR: el.BStar,
    // The Go parser stores the real derivatives; OMM/TLE carry ndot/2 and nddot/6.
    MEAN_MOTION_DOT: el.MeanMotionDot / 2,
    MEAN_MOTION_DDOT: el.MeanMotionDotDot / 6,
  }
  return json2satrec(omm)
}

export function track(el: Elements): Tracked {
  return {
    id: el.ID,
    name: el.Name || `NORAD ${el.ID}`,
    el,
    satrec: toSatrec(el),
    epoch: new Date(el.Epoch),
    summary: summarize(el),
    position: new Cartesian3(),
    latDeg: 0,
    lonDeg: 0,
    altKm: 0,
    speedKmS: 0,
    valid: false,
  }
}

/**
 * Propagate to `date` and write the Earth-fixed position (metres) plus
 * geodetic state into `t`. `gmst` is shared across all objects for one tick.
 */
export function updateLive(t: Tracked, date: Date, gmst: number): boolean {
  const pv = propagate(t.satrec, date)
  if (!pv) {
    t.valid = false
    return false
  }
  const ecf = eciToEcf(pv.position, gmst)
  Cartesian3.fromElements(ecf.x * 1000, ecf.y * 1000, ecf.z * 1000, t.position)
  const geo = eciToGeodetic(pv.position, gmst)
  t.latDeg = radiansToDegrees(geo.latitude)
  t.lonDeg = radiansToDegrees(geo.longitude)
  t.altKm = geo.height
  const v = pv.velocity
  t.speedKmS = Math.hypot(v.x, v.y, v.z)
  t.valid = true
  return true
}

/**
 * One full revolution sampled in the inertial frame, starting at `date`.
 * Returns a flat [x, y, z, ...] array in kilometres; rotate it into the fixed
 * frame with `eciRingToFixed` for the current sidereal time.
 */
export function sampleOrbitEci(t: Tracked, date: Date, samples = 256): Float64Array | null {
  const periodMs = t.summary.periodMin * 60_000
  const out = new Float64Array((samples + 1) * 3)
  for (let i = 0; i <= samples; i++) {
    const when = new Date(date.getTime() + (i / samples) * periodMs)
    const pv = propagate(t.satrec, when)
    if (!pv) return null
    out[i * 3] = pv.position.x
    out[i * 3 + 1] = pv.position.y
    out[i * 3 + 2] = pv.position.z
  }
  return out
}

/** Rotate inertial samples by `gmst` into Earth-fixed Cartesian3 metres. */
export function eciRingToFixed(ring: Float64Array, gmst: number): Cartesian3[] {
  const c = Math.cos(gmst)
  const s = Math.sin(gmst)
  const n = ring.length / 3
  const out = new Array<Cartesian3>(n)
  for (let i = 0; i < n; i++) {
    const x = ring[i * 3]
    const y = ring[i * 3 + 1]
    const z = ring[i * 3 + 2]
    out[i] = new Cartesian3((x * c + y * s) * 1000, (-x * s + y * c) * 1000, z * 1000)
  }
  return out
}

/**
 * Sub-satellite ground track over one revolution centred on `date`, as
 * positions a few kilometres above the ellipsoid so the line never fights
 * the globe surface for depth.
 */
export function sampleGroundTrack(t: Tracked, date: Date, samples = 256): Cartesian3[] | null {
  const periodMs = t.summary.periodMin * 60_000
  const heightM = 8_000
  const out: Cartesian3[] = []
  for (let i = 0; i <= samples; i++) {
    const when = new Date(date.getTime() + (i / samples - 0.5) * periodMs)
    const pv = propagate(t.satrec, when)
    if (!pv) return null
    const geo = eciToGeodetic(pv.position, gstime(when))
    out.push(Cartesian3.fromRadians(geo.longitude, geo.latitude, heightM))
  }
  return out
}

export { gstime }
