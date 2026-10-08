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

/** Element-set history for one object over the trailing window (default 30 days). */
export async function fetchHistory(id: number, days = 30): Promise<Elements[]> {
  const to = new Date()
  const from = new Date(to.getTime() - days * 86_400_000)
  const query = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() })
  const list = await getJson<Elements[] | null>(`/api/satellites/${id}/history?${query}`)
  return list ?? []
}
