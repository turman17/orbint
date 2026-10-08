// Thin client for the Go API (cmd/api). Field names mirror orbit.TLE in Go,
// which is encoded without json tags, hence the capitalised keys.

export interface Elements {
  Name: string
  ID: number
  /** Classification as a byte: 85 = 'U', 67 = 'C', 83 = 'S'. */
  Class: number
  InternationalDesignator: string
  ElementSetNumber: number
  /** RFC 3339, UTC. */
  Epoch: string
  Inclination: number
  RAAN: number
  Eccentricity: number
  ArgumentOfPerigee: number
  MeanAnomaly: number
  /** rev/day */
  MeanMotion: number
  /** rev/day², real first derivative (the parser already applied ×2). */
  MeanMotionDot: number
  /** rev/day³, real second derivative (the parser already applied ×6). */
  MeanMotionDotDot: number
  BStar: number
  EphemerisType: number
  RevolutionNumber: number
}

async function getJson<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    throw new Error(`API ${res.status} ${res.statusText} for ${url}`)
  }
  return (await res.json()) as T
}

/** Latest element set for every tracked object. */
export async function fetchSatellites(): Promise<Elements[]> {
  const list = await getJson<Elements[] | null>('/api/satellites')
  return list ?? []
}

/** detect.Kind in Go. */
export type CandidateKind =
  | 'candidate_delta_v'
  | 'candidate_plane_change'
  | 'candidate_drag_anomaly'
  | 'candidate_ecc_change'

/** detect.Rule in Go. */
export type CandidateRule = 'zscore' | 'jump'

/** detect.Signal in Go: one detector firing on one feature series. */
export interface Signal {
  Kind: CandidateKind
  Rule: CandidateRule
  EpochStart: string
  EpochEnd: string
  /** Peak robust z-score of the run; 0 for a jump. */
  ZScore: number
  /** Feature value at the peak (a rate), or the raw step for a jump. */
  Value: number
}

/**
 * detect.Candidate in Go: one event, i.e. every signal that fired within the
 * merge window, with the primary signal's kind, score and value on top.
 */
export interface Candidate {
  SatelliteID: number
  Kind: CandidateKind
  EpochStart: string
  EpochEnd: string
  ZScore: number
  Value: number
  Signals: Signal[]
}

/** Element-set history for one object over the trailing window (default 30 days). */
export async function fetchHistory(id: number, days = 30): Promise<Elements[]> {
  const to = new Date()
  const from = new Date(to.getTime() - days * 86_400_000)
  const query = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() })
  const list = await getJson<Elements[] | null>(`/api/satellites/${id}/history?${query}`)
  return list ?? []
}

/**
 * Candidate maneuvers/anomalies from the Go feature engine + detector over
 * the same trailing window as the history. Never proven events: the server
 * flags robust z-score excursions with persistence, nothing more.
 */
export async function fetchAnomalies(id: number, days = 30): Promise<Candidate[]> {
  const to = new Date()
  const from = new Date(to.getTime() - days * 86_400_000)
  const query = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() })
  const list = await getJson<Candidate[] | null>(`/api/satellites/${id}/anomalies?${query}`)
  return list ?? []
}
