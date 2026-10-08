package detect

import (
	"math"
	"sort"
	"time"

	"github.com/turman17/orbint/internal/feature"
)

type Kind string

const (
	KindDeltaV      Kind = "candidate_delta_v"
	KindPlaneChange Kind = "candidate_plane_change"
	KindDragAnomaly Kind = "candidate_drag_anomaly"
	KindEccChange   Kind = "candidate_ecc_change"
)

type Candidate struct {
	SatelliteID int
	Kind        Kind
	EpochStart  time.Time
	EpochEnd    time.Time
	ZScore      float64 // peak z-score that triggered it
	Value       float64 // the actual feature value that triggered
}

type Config struct {
	WindowSize  int     // sliding window for median/MAD (e.g. 20)
	Threshold   float64 // z-score cutoff (e.g. 3.5)
	Persistence int     // consecutive exceedances required (e.g. 2)
}

func DefaultConfig() Config {
	return Config{
		WindowSize:  20,
		Threshold:   3.5,
		Persistence: 2,
	}
}

// Detect runs the rule-based detector over a feature series.
// Returns candidates sorted by epoch.
func Detect(satelliteID int, features []feature.Feature, cfg Config) []Candidate {
	if len(features) == 0 {
		return nil
	}

	defaults := DefaultConfig()
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = defaults.WindowSize
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = defaults.Threshold
	}
	if cfg.Persistence <= 0 {
		cfg.Persistence = defaults.Persistence
	}

	// Detection operates on time order, even if the caller supplied features
	// in another order. Do not mutate the caller's slice.
	ordered := append([]feature.Feature(nil), features...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Epoch.Before(ordered[j].Epoch)
	})

	type signal struct {
		kind  Kind
		value func(feature.Feature) float64
	}

	signals := []signal{
		{kind: KindDeltaV, value: func(f feature.Feature) float64 {
			return f.MeanMotionDrift
		}},
		{kind: KindPlaneChange, value: func(f feature.Feature) float64 {
			return f.InclinationRate
		}},
		{kind: KindDragAnomaly, value: func(f feature.Feature) float64 {
			return f.BStarRate
		}},
		{kind: KindEccChange, value: func(f feature.Feature) float64 {
			return f.EccentricityRate
		}},
	}

	candidates := make([]Candidate, 0)
	for _, s := range signals {
		values := make([]float64, len(ordered))
		for i, f := range ordered {
			values[i] = s.value(f)
		}

		var run *Candidate
		runLength := 0
		finishRun := func() {
			if run != nil && runLength >= cfg.Persistence {
				candidates = append(candidates, *run)
			}
			run = nil
			runLength = 0
		}

		for i, value := range values {
			start := i - cfg.WindowSize
			if start < 0 {
				start = 0
			}
			if i == start {
				finishRun()
				continue
			}

			median, mad := medianMAD(values[start:i])
			z := robustZScore(value, median, mad)
			if math.IsNaN(z) || z < cfg.Threshold {
				finishRun()
				continue
			}

			runLength++
			if run == nil {
				run = &Candidate{
					SatelliteID: satelliteID,
					Kind:        s.kind,
					EpochStart:  ordered[i].Epoch,
					EpochEnd:    ordered[i].Epoch,
					ZScore:      z,
					Value:       value,
				}
			} else {
				run.EpochEnd = ordered[i].Epoch
				if z > run.ZScore {
					run.ZScore = z
					run.Value = value
				}
			}
		}
		finishRun()
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].EpochStart.Equal(candidates[j].EpochStart) {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].EpochStart.Before(candidates[j].EpochStart)
	})
	return candidates
}

func robustZScore(value, median, mad float64) float64 {
	if math.IsNaN(value) || math.IsNaN(median) || math.IsNaN(mad) {
		return math.NaN()
	}
	if mad == 0 {
		if value == median {
			return 0
		}
		return math.Inf(1)
	}
	return 0.6744897501960817 * math.Abs(value-median) / mad
}

func medianMAD(values []float64) (median, mad float64) {
	if len(values) == 0 {
		return math.NaN(), math.NaN()
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	median = middle(sorted)

	deviations := make([]float64, len(sorted))
	for i, value := range sorted {
		deviations[i] = math.Abs(value - median)
	}
	sort.Float64s(deviations)
	return median, middle(deviations)
}

func middle(values []float64) float64 {
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}
