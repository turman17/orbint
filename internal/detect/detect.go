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

// Rule says which test produced a signal.
type Rule string

const (
	// RuleZScore: a run of feature values far outside the window's robust
	// spread, lasting at least Persistence consecutive element sets.
	RuleZScore Rule = "zscore"
	// RuleJump: a single step in an element too large for natural drift,
	// accepted on its own with no persistence requirement. This is how a
	// clean impulsive maneuver shows up: one jump between two element sets.
	RuleJump Rule = "jump"
)

// Signal is one detector firing on one feature series.
type Signal struct {
	Kind       Kind
	Rule       Rule
	EpochStart time.Time
	EpochEnd   time.Time
	ZScore     float64 // peak robust z-score of the run; 0 for RuleJump
	Value      float64 // feature value at the peak (a rate), or the raw jump for RuleJump
}

// Candidate is one event: every signal that fired within MergeWindow of
// each other, collapsed into a single row. Kind, ZScore and Value describe
// the primary signal (a jump if any fired, else the highest z-score).
type Candidate struct {
	SatelliteID int
	Kind        Kind
	EpochStart  time.Time
	EpochEnd    time.Time
	ZScore      float64
	Value       float64
	Signals     []Signal
}

type Config struct {
	WindowSize  int     // sliding window for median/MAD (e.g. 20)
	Threshold   float64 // z-score cutoff for RuleZScore (e.g. 7)
	Persistence int     // consecutive exceedances required for RuleZScore (e.g. 2)

	// Minimum absolute rate a value must reach before it can trigger,
	// regardless of z-score. These are quantisation guards, not physical
	// thresholds: TLE fields carry a fixed number of decimals, so a change in
	// the last digit divided by a short gap looks like a large rate against a
	// baseline of exact zeros. Zero means "use the default".
	MinMeanMotionDrift  float64 // rev/day per day
	MinInclinationRate  float64 // deg/day
	MinEccentricityRate float64 // 1/day
	MinBStarRate        float64 // 1/(Earth radii) per day

	// RuleJump thresholds on the raw step between consecutive element sets.
	// Sized for LEO: 0.001 rev/day is a few hundred metres of altitude, far
	// beyond what drag moves in one gap for most objects. GEO station
	// keeping is smaller and relies on RuleZScore.
	MinMeanMotionJump  float64 // rev/day
	MinInclinationJump float64 // deg

	// Drag only ever raises mean motion, so a drop past MinMeanMotionJump is
	// a maneuver on its own. A rise must additionally exceed this multiple
	// of the object's own median |step| over the window, otherwise objects
	// in the last months of decay (perigee below ~350 km) fire on every
	// element set.
	PositiveJumpRatio float64

	// Signals whose intervals come within MergeWindow of each other are one
	// event. A reboost moves mean motion, eccentricity and the B* fit on the
	// same day; this keeps it one row.
	MergeWindow time.Duration
}

// Defaults for the quantisation guards, roughly ten times the smallest
// representable change over a typical six-hour gap between element sets:
// mean motion has 8 decimals, inclination 4, eccentricity 7, B* about 5
// significant digits.
const (
	defaultMinMeanMotionDrift  = 2e-7
	defaultMinInclinationRate  = 5e-3
	defaultMinEccentricityRate = 2e-6
	defaultMinBStarRate        = 1e-6

	defaultMinMeanMotionJump  = 1e-3
	defaultPositiveJumpRatio  = 5
	defaultMinInclinationJump = 1e-2
	defaultMergeWindow        = 24 * time.Hour
)

func DefaultConfig() Config {
	return Config{
		WindowSize:  20,
		Threshold:   7,
		Persistence: 2,

		MinMeanMotionDrift:  defaultMinMeanMotionDrift,
		MinInclinationRate:  defaultMinInclinationRate,
		MinEccentricityRate: defaultMinEccentricityRate,
		MinBStarRate:        defaultMinBStarRate,

		MinMeanMotionJump:  defaultMinMeanMotionJump,
		MinInclinationJump: defaultMinInclinationJump,
		PositiveJumpRatio:  defaultPositiveJumpRatio,
		MergeWindow:        defaultMergeWindow,
	}
}

func (cfg Config) withDefaults() Config {
	d := DefaultConfig()
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = d.WindowSize
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = d.Threshold
	}
	if cfg.Persistence <= 0 {
		cfg.Persistence = d.Persistence
	}
	if cfg.MinMeanMotionDrift <= 0 {
		cfg.MinMeanMotionDrift = d.MinMeanMotionDrift
	}
	if cfg.MinInclinationRate <= 0 {
		cfg.MinInclinationRate = d.MinInclinationRate
	}
	if cfg.MinEccentricityRate <= 0 {
		cfg.MinEccentricityRate = d.MinEccentricityRate
	}
	if cfg.MinBStarRate <= 0 {
		cfg.MinBStarRate = d.MinBStarRate
	}
	if cfg.MinMeanMotionJump <= 0 {
		cfg.MinMeanMotionJump = d.MinMeanMotionJump
	}
	if cfg.MinInclinationJump <= 0 {
		cfg.MinInclinationJump = d.MinInclinationJump
	}
	if cfg.PositiveJumpRatio <= 0 {
		cfg.PositiveJumpRatio = d.PositiveJumpRatio
	}
	if cfg.MergeWindow <= 0 {
		cfg.MergeWindow = d.MergeWindow
	}
	return cfg
}

// Detect runs the rule-based detector over a feature series and merges the
// signals into candidate events, sorted by epoch.
func Detect(satelliteID int, features []feature.Feature, cfg Config) []Candidate {
	if len(features) == 0 {
		return nil
	}
	cfg = cfg.withDefaults()

	// Detection operates on time order, even if the caller supplied features
	// in another order. Do not mutate the caller's slice.
	ordered := append([]feature.Feature(nil), features...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Epoch.Before(ordered[j].Epoch)
	})

	signals := zScoreSignals(ordered, cfg)
	signals = append(signals, jumpSignals(ordered, cfg)...)
	return mergeSignals(satelliteID, signals, cfg.MergeWindow)
}

// zScoreSignals applies RuleZScore to each rate series.
func zScoreSignals(ordered []feature.Feature, cfg Config) []Signal {
	type series struct {
		kind   Kind
		minAbs float64
		value  func(feature.Feature) float64
	}
	all := []series{
		{KindDeltaV, cfg.MinMeanMotionDrift, func(f feature.Feature) float64 { return f.MeanMotionDrift }},
		{KindPlaneChange, cfg.MinInclinationRate, func(f feature.Feature) float64 { return f.InclinationRate }},
		{KindDragAnomaly, cfg.MinBStarRate, func(f feature.Feature) float64 { return f.BStarRate }},
		{KindEccChange, cfg.MinEccentricityRate, func(f feature.Feature) float64 { return f.EccentricityRate }},
	}

	var signals []Signal
	for _, s := range all {
		values := make([]float64, len(ordered))
		for i, f := range ordered {
			values[i] = s.value(f)
		}

		var run *Signal
		runLength := 0
		finishRun := func() {
			if run != nil && runLength >= cfg.Persistence {
				signals = append(signals, *run)
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
			// Below the quantisation guard nothing can trigger, however the
			// window is distributed.
			if math.Abs(value) < s.minAbs {
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
				run = &Signal{
					Kind:       s.kind,
					Rule:       RuleZScore,
					EpochStart: ordered[i].Epoch,
					EpochEnd:   ordered[i].Epoch,
					ZScore:     z,
					Value:      value,
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
	return signals
}

// jumpSignals applies RuleJump: one element set to the next moved by more
// than natural forces can, so a single feature is enough.
func jumpSignals(ordered []feature.Feature, cfg Config) []Signal {
	var signals []Signal
	absSteps := make([]float64, len(ordered))
	for i, f := range ordered {
		absSteps[i] = math.Abs(f.MeanMotionJump)
	}
	for i, f := range ordered {
		if f.MeanMotionJump <= -cfg.MinMeanMotionJump || positiveJump(f.MeanMotionJump, absSteps, i, cfg) {
			signals = append(signals, Signal{
				Kind: KindDeltaV, Rule: RuleJump,
				EpochStart: f.Epoch, EpochEnd: f.Epoch, Value: f.MeanMotionJump,
			})
		}
		if math.Abs(f.InclinationJump) >= cfg.MinInclinationJump {
			signals = append(signals, Signal{
				Kind: KindPlaneChange, Rule: RuleJump,
				EpochStart: f.Epoch, EpochEnd: f.Epoch, Value: f.InclinationJump,
			})
		}
	}
	return signals
}

// positiveJump decides whether a rise in mean motion at index i is a
// maneuver rather than drag: it must pass the absolute threshold and be at
// least PositiveJumpRatio times the median |step| of the preceding window.
// With no preceding window the absolute threshold alone decides.
func positiveJump(step float64, absSteps []float64, i int, cfg Config) bool {
	if step < cfg.MinMeanMotionJump {
		return false
	}
	start := i - cfg.WindowSize
	if start < 0 {
		start = 0
	}
	if i == start {
		return true
	}
	median, _ := medianMAD(absSteps[start:i])
	return step >= cfg.PositiveJumpRatio*median
}

// mergeSignals groups signals whose intervals come within window of each
// other into one Candidate, choosing the primary signal per Candidate.
func mergeSignals(satelliteID int, signals []Signal, window time.Duration) []Candidate {
	if len(signals) == 0 {
		return nil
	}
	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].EpochStart.Equal(signals[j].EpochStart) {
			return signals[i].Kind < signals[j].Kind
		}
		return signals[i].EpochStart.Before(signals[j].EpochStart)
	})

	var events []Candidate
	for _, s := range signals {
		n := len(events)
		if n > 0 && !s.EpochStart.After(events[n-1].EpochEnd.Add(window)) {
			ev := &events[n-1]
			ev.Signals = append(ev.Signals, s)
			if s.EpochEnd.After(ev.EpochEnd) {
				ev.EpochEnd = s.EpochEnd
			}
			continue
		}
		events = append(events, Candidate{
			SatelliteID: satelliteID,
			EpochStart:  s.EpochStart,
			EpochEnd:    s.EpochEnd,
			Signals:     []Signal{s},
		})
	}

	for i := range events {
		ev := &events[i]
		primary := ev.Signals[0]
		for _, s := range ev.Signals[1:] {
			if stronger(s, primary) {
				primary = s
			}
		}
		ev.Kind = primary.Kind
		ev.Value = primary.Value
		for _, s := range ev.Signals {
			if s.ZScore > ev.ZScore {
				ev.ZScore = s.ZScore
			}
		}
	}
	return events
}

// stronger ranks signals for the primary slot: a jump beats any z-score
// run (it is absolute evidence), larger jumps beat smaller ones, and among
// z-score runs the higher score wins.
func stronger(a, b Signal) bool {
	if a.Rule != b.Rule {
		return a.Rule == RuleJump
	}
	if a.Rule == RuleJump {
		return math.Abs(a.Value) > math.Abs(b.Value)
	}
	return a.ZScore > b.ZScore
}

// robustZScore scales |value - median| by the MAD (0.6745/MAD matches the
// standard deviation for normal data). A zero MAD means the window has no
// spread to judge against, typically a run of identical quantised values,
// so it is reported as NaN ("no evidence") and the caller skips it rather
// than treating any change as infinitely significant.
func robustZScore(value, median, mad float64) float64 {
	if math.IsNaN(value) || math.IsNaN(median) || math.IsNaN(mad) || mad == 0 {
		return math.NaN()
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
