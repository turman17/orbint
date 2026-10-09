# orbint

Orbital Intelligence Engine. Ingests public satellite orbital elements,
propagates orbits, derives time-series features from the element history,
and flags candidate maneuvers on a 3D globe.

![orbint demo: satellites over a lit Earth with the catalog and clock controls](docs/demo.gif)

Detected events are always **candidates**, never proven facts. The detector
sees a step in the published elements; it cannot know why it happened.

## What it does

- Pulls orbital elements (OMM/TLE) from CelesTrak and keeps every element set
  in PostgreSQL, so each object has a history rather than a single snapshot.
- Propagates orbits with SGP4 and renders them live on a CesiumJS globe with a
  day/night terminator, orbit ring and ground track for the selected object.
- Turns each object's element history into per-day rates (mean motion drift,
  inclination rate, eccentricity rate, B* rate) and raw steps between
  consecutive element sets.
- Flags candidate maneuvers with two rules: a robust z-score (median/MAD over
  a sliding window, with persistence) for gradual anomalies, and a single-step
  rule for impulsive burns. Overlapping signals merge into one event.

Example: on 2026-08-27 NASA reported an ISS reboost by a Cygnus vehicle.
From the public element history alone, orbint flags a step of −0.0073 rev/day
in mean motion on the next element set, about 2 km of altitude gained.

## Stack

| Part | Technology |
|---|---|
| Ingestion, orbital core, feature engine, detector, API | Go |
| Storage | PostgreSQL |
| Frontend | TypeScript, Vite, CesiumJS, satellite.js |
| Planned | Python for EDA and ML on top of the feature engine |

## Layout

```
cmd/api        HTTP API (satellites, history, anomalies)
cmd/ingest     periodic CelesTrak ingestion into Postgres
cmd/orbint     thin CLI
internal/orbit      TLE/OMM parsing, SGP4 wrapper
internal/celestrak  CelesTrak client
internal/store      Postgres access
internal/feature    element history -> feature time series
internal/detect     rule-based candidate detector
migrations/         database schema
web/                CesiumJS frontend
```

## Running it

Requirements: Go 1.25, Node 22, Docker.

```sh
# 1. Database (and the ingestion loop) in containers
docker compose up -d

# 2. Configuration
cp .env_example .env
#    DATABASE_URL=postgres://orbint:orbint@localhost:5432/orbint?sslmode=disable

# 3. API on :8090
go run ./cmd/api

# 4. Frontend on :3000 (proxies /api to :8090)
cd web && npm install && npm run dev
```

The ingestion loop pulls a set of CelesTrak groups every two hours.
Override the set with `CELESTRAK_GROUPS` in `.env`, for example
`CELESTRAK_GROUPS="stations,visual,geo"`. Group names are listed at
https://celestrak.org/NORAD/elements/.

The detector needs history to work with: roughly twenty element sets per
object before it can score anything. Fresh installs show "not enough
history" in the panel until the loop has run for a while.

## API

| Endpoint | Returns |
|---|---|
| `GET /api/satellites` | latest element set for every tracked object |
| `GET /api/satellites/{id}/history?from=&to=` | element sets in the window (default: last 30 days) |
| `GET /api/satellites/{id}/anomalies?from=&to=` | candidate events in the window |

A candidate event:

```json
{
  "SatelliteID": 25544,
  "Kind": "candidate_delta_v",
  "EpochStart": "2026-08-28T15:03:39Z",
  "EpochEnd": "2026-08-28T15:03:39Z",
  "ZScore": 0,
  "Value": -0.007342,
  "Signals": [
    { "Kind": "candidate_delta_v", "Rule": "jump", "Value": -0.007342, "ZScore": 0,
      "EpochStart": "2026-08-28T15:03:39Z", "EpochEnd": "2026-08-28T15:03:39Z" }
  ]
}
```

`Kind` is the primary signal. `Rule` is `jump` for a single step too large
for natural forces, or `zscore` for a persistent statistical excursion.

## Detection, briefly

1. Consecutive element sets become one feature: the change in each element
   divided by the gap in days, plus the raw step. Element sets closer than
   30 minutes are treated as re-issues and collapsed.
2. For each rate series, a sliding window of 20 features gives a median and
   MAD; a value scores as a robust z. Scores above 7 on at least 2
   consecutive features form a signal. A window with zero spread is treated
   as no evidence, and each series has a floor below which quantisation of
   the published fields cannot trigger anything.
3. A step in mean motion of 0.001 rev/day or in inclination of 0.01° is a
   signal on its own. Drag only raises mean motion, so drops always count,
   while rises must also exceed five times the object's own typical step.
4. Signals within 24 hours of each other merge into one event.

Tunables live in `internal/detect.Config`.

## Data and terms

Orbital elements come from [CelesTrak](https://celestrak.org). Keep the
request rate low; one fetch per group every two hours is what they ask for.
Downloaded data is never committed to this repository.

## Tests

```sh
go test ./...
cd web && npm run build
```

## License

MIT. See [LICENSE](LICENSE).
