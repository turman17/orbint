package detect

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/turman17/orbint/internal/feature"
)

var epoch0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func day(n int) time.Time {
	return epoch0.Add(time.Duration(n) * 24 * time.Hour)
}

// steadyFeatures builds a quiet baseline with a small deterministic ripple
// (±5e-5 around drift) so the window has a non-zero MAD to score against.
// A perfectly constant series is "no evidence" by design; see
// TestDetectZeroMADIsNoEvidence.
func steadyFeatures(n int, drift float64) []feature.Feature {
	fs := make([]feature.Feature, n)
	for i := range fs {
		fs[i] = feature.Feature{
			Epoch:           day(i),
			GapDays:         1.0,
			MeanMotionDrift: drift + float64(i%3-1)*5e-5,
		}
	}
	return fs
}

// constantFeatures builds a series with no spread at all.
func constantFeatures(n int, drift float64) []feature.Feature {
	fs := make([]feature.Feature, n)
	for i := range fs {
		fs[i] = feature.Feature{Epoch: day(i), GapDays: 1.0, MeanMotionDrift: drift}
	}
	return fs
}

func TestDetectNil(t *testing.T) {
	if got := Detect(25544, nil, DefaultConfig()); got != nil {
		t.Fatalf("expected nil for nil input, got %d candidates", len(got))
	}
	if got := Detect(25544, []feature.Feature{}, DefaultConfig()); got != nil {
		t.Fatalf("expected nil for empty input, got %d candidates", len(got))
	}
}

func TestDetectConstantSeries(t *testing.T) {
	fs := constantFeatures(30, 0.0001)
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for constant series, got %d", len(candidates))
	}
}

func TestDetectSingleOutlierSuppressed(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0

	candidates := Detect(25544, fs, Config{
		WindowSize:  10,
		Threshold:   3.5,
		Persistence: 2,
	})

	for _, c := range candidates {
		if c.Kind == KindDeltaV {
			t.Fatalf("single outlier should be suppressed by persistence=2, got %+v", c)
		}
	}
}

func TestDetectPersistentAnomaly(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	candidates := Detect(25544, fs, Config{
		WindowSize:  10,
		Threshold:   3.5,
		Persistence: 2,
	})

	var found *Candidate
	for i := range candidates {
		if candidates[i].Kind == KindDeltaV {
			found = &candidates[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected a KindDeltaV candidate for 2 consecutive outliers")
	}
	if found.SatelliteID != 25544 {
		t.Errorf("SatelliteID = %d, want 25544", found.SatelliteID)
	}
	if !found.EpochStart.Equal(day(20)) {
		t.Errorf("EpochStart = %v, want %v", found.EpochStart, day(20))
	}
	if !found.EpochEnd.Equal(day(21)) {
		t.Errorf("EpochEnd = %v, want %v", found.EpochEnd, day(21))
	}
	if found.ZScore <= 3.5 {
		t.Errorf("ZScore = %f, expected > 3.5", found.ZScore)
	}
}

func TestDetectPersistenceThree(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[15].MeanMotionDrift = 5.0
	fs[16].MeanMotionDrift = 5.0

	candidates := Detect(25544, fs, Config{
		WindowSize:  10,
		Threshold:   3.5,
		Persistence: 3,
	})

	for _, c := range candidates {
		if c.Kind == KindDeltaV {
			t.Fatalf("2 outliers should be suppressed by persistence=3, got %+v", c)
		}
	}

	fs[17].MeanMotionDrift = 5.0
	candidates = Detect(25544, fs, Config{
		WindowSize:  10,
		Threshold:   3.5,
		Persistence: 3,
	})

	var found bool
	for _, c := range candidates {
		if c.Kind == KindDeltaV {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("3 consecutive outliers should trigger with persistence=3")
	}
}

func TestDetectMultipleKinds(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	for i := range fs {
		// Rippled baselines: a constant series has zero MAD and is skipped.
		fs[i].InclinationRate = 0.0001 + float64(i%3)*0.00005
		fs[i].EccentricityRate = 0.00001 + float64(i%3)*0.000005
		fs[i].BStarRate = 0.00001 + float64(i%3)*0.000005
	}

	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	fs[20].InclinationRate = 3.0
	fs[21].InclinationRate = 3.0

	cfg := Config{WindowSize: 10, Threshold: 3.5, Persistence: 2}
	candidates := Detect(25544, fs, cfg)

	// Both signals fire on the same epochs, so they merge into one event
	// that carries both kinds.
	if len(candidates) != 1 {
		t.Fatalf("expected 1 merged event, got %d", len(candidates))
	}
	kinds := make(map[Kind]bool)
	for _, sig := range candidates[0].Signals {
		kinds[sig.Kind] = true
	}
	if !kinds[KindDeltaV] {
		t.Error("expected KindDeltaV candidate")
	}
	if !kinds[KindPlaneChange] {
		t.Error("expected KindPlaneChange candidate")
	}
}

func TestDetectDoesNotMutateInput(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	original := make([]feature.Feature, len(fs))
	copy(original, fs)

	Detect(25544, fs, DefaultConfig())

	for i := range fs {
		if fs[i].Epoch != original[i].Epoch || fs[i].MeanMotionDrift != original[i].MeanMotionDrift {
			t.Fatalf("Detect mutated input at index %d", i)
		}
	}
}

func TestDetectReversedInputOrder(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	reversed := make([]feature.Feature, len(fs))
	for i, f := range fs {
		reversed[len(fs)-1-i] = f
	}

	cfg := Config{WindowSize: 10, Threshold: 3.5, Persistence: 2}
	normal := Detect(25544, fs, cfg)
	rev := Detect(25544, reversed, cfg)

	if len(normal) != len(rev) {
		t.Fatalf("reversed input produced %d candidates, normal produced %d", len(rev), len(normal))
	}
	for i := range normal {
		if normal[i].Kind != rev[i].Kind || !normal[i].EpochStart.Equal(rev[i].EpochStart) {
			t.Errorf("candidate %d differs: normal=%+v, reversed=%+v", i, normal[i], rev[i])
		}
	}
}

func TestDetectPeakZScoreTracked(t *testing.T) {
	// Use a noisy baseline so MAD > 0 and z-scores are finite and distinct.
	fs := make([]feature.Feature, 30)
	for i := range fs {
		fs[i] = feature.Feature{
			Epoch:           day(i),
			GapDays:         1.0,
			MeanMotionDrift: 0.0001 + float64(i%3)*0.00005,
		}
	}
	fs[20].MeanMotionDrift = 0.5
	fs[21].MeanMotionDrift = 2.0
	fs[22].MeanMotionDrift = 1.0

	cfg := Config{WindowSize: 15, Threshold: 3.5, Persistence: 2}
	candidates := Detect(25544, fs, cfg)

	var found *Candidate
	for i := range candidates {
		if candidates[i].Kind == KindDeltaV {
			found = &candidates[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected a KindDeltaV candidate")
	}
	if found.Value != 2.0 {
		t.Errorf("Value = %f, want 2.0 (the peak outlier)", found.Value)
	}
}

func TestDetectSortedByEpoch(t *testing.T) {
	fs := steadyFeatures(40, 0.0001)
	for i := range fs {
		fs[i].InclinationRate = 0.0001 + float64(i%3)*0.00005 // rippled, non-zero MAD
	}

	fs[10].MeanMotionDrift = 5.0
	fs[11].MeanMotionDrift = 5.0

	fs[30].InclinationRate = 3.0
	fs[31].InclinationRate = 3.0

	cfg := Config{WindowSize: 8, Threshold: 3.5, Persistence: 2}
	candidates := Detect(25544, fs, cfg)

	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(candidates))
	}
	for i := 1; i < len(candidates); i++ {
		if candidates[i].EpochStart.Before(candidates[i-1].EpochStart) {
			t.Errorf("candidates not sorted: [%d] %v after [%d] %v",
				i, candidates[i].EpochStart, i-1, candidates[i-1].EpochStart)
		}
	}
}

func TestDetectDefaultConfigFallback(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	candidates := Detect(25544, fs, Config{})
	var found bool
	for _, c := range candidates {
		if c.Kind == KindDeltaV {
			found = true
		}
	}
	if !found {
		t.Fatal("zero-value Config should use defaults and still detect the anomaly")
	}
}

func TestDetectTwoSeparateEvents(t *testing.T) {
	fs := steadyFeatures(40, 0.0001)

	fs[10].MeanMotionDrift = 5.0
	fs[11].MeanMotionDrift = 5.0

	fs[30].MeanMotionDrift = 5.0
	fs[31].MeanMotionDrift = 5.0

	cfg := Config{WindowSize: 8, Threshold: 3.5, Persistence: 2}
	candidates := Detect(25544, fs, cfg)

	dvCount := 0
	for _, c := range candidates {
		if c.Kind == KindDeltaV {
			dvCount++
		}
	}
	if dvCount != 2 {
		t.Fatalf("expected 2 separate KindDeltaV candidates, got %d", dvCount)
	}
}

func TestMedianMADEmpty(t *testing.T) {
	median, mad := medianMAD(nil)
	if !math.IsNaN(median) || !math.IsNaN(mad) {
		t.Errorf("expected NaN for empty, got median=%f mad=%f", median, mad)
	}
}

func TestMedianMADSingleValue(t *testing.T) {
	median, mad := medianMAD([]float64{42.0})
	if median != 42.0 {
		t.Errorf("median = %f, want 42.0", median)
	}
	if mad != 0.0 {
		t.Errorf("mad = %f, want 0.0", mad)
	}
}

func TestMedianMADOdd(t *testing.T) {
	median, mad := medianMAD([]float64{1, 2, 3, 4, 5})
	if median != 3.0 {
		t.Errorf("median = %f, want 3.0", median)
	}
	if mad != 1.0 {
		t.Errorf("mad = %f, want 1.0", mad)
	}
}

func TestMedianMADEven(t *testing.T) {
	median, mad := medianMAD([]float64{1, 2, 3, 4})
	if median != 2.5 {
		t.Errorf("median = %f, want 2.5", median)
	}
	// deviations: |1-2.5|=1.5, |2-2.5|=0.5, |3-2.5|=0.5, |4-2.5|=1.5
	// sorted: [0.5, 0.5, 1.5, 1.5], median = (0.5+1.5)/2 = 1.0
	if mad != 1.0 {
		t.Errorf("mad = %f, want 1.0", mad)
	}
}

func TestMedianMADDoesNotMutate(t *testing.T) {
	values := []float64{5, 3, 1, 4, 2}
	original := make([]float64, len(values))
	copy(original, values)

	medianMAD(values)

	for i := range values {
		if values[i] != original[i] {
			t.Fatalf("medianMAD mutated input at index %d: got %f, want %f", i, values[i], original[i])
		}
	}
}

func TestRobustZScoreBasic(t *testing.T) {
	z := robustZScore(10.0, 5.0, 1.0)
	expected := 0.6744897501960817 * 5.0 / 1.0
	if math.Abs(z-expected) > 1e-10 {
		t.Errorf("z = %f, want %f", z, expected)
	}
}

func TestRobustZScoreZeroMAD(t *testing.T) {
	z := robustZScore(5.0, 5.0, 0.0)
	if !math.IsNaN(z) {
		t.Errorf("z = %f, want NaN (no evidence) when MAD == 0, even at the median", z)
	}

	z = robustZScore(6.0, 5.0, 0.0)
	if !math.IsNaN(z) {
		t.Errorf("z = %f, want NaN (no evidence) when MAD == 0", z)
	}
}

func TestRobustZScoreNaN(t *testing.T) {
	if z := robustZScore(math.NaN(), 5.0, 1.0); !math.IsNaN(z) {
		t.Errorf("expected NaN for NaN value, got %f", z)
	}
	if z := robustZScore(5.0, math.NaN(), 1.0); !math.IsNaN(z) {
		t.Errorf("expected NaN for NaN median, got %f", z)
	}
	if z := robustZScore(5.0, 5.0, math.NaN()); !math.IsNaN(z) {
		t.Errorf("expected NaN for NaN mad, got %f", z)
	}
}

func TestMiddle(t *testing.T) {
	if v := middle([]float64{7}); v != 7 {
		t.Errorf("middle([7]) = %f, want 7", v)
	}
	if v := middle([]float64{3, 7}); v != 5 {
		t.Errorf("middle([3,7]) = %f, want 5", v)
	}
	if v := middle([]float64{1, 3, 5}); v != 3 {
		t.Errorf("middle([1,3,5]) = %f, want 3", v)
	}
	if v := middle([]float64{1, 2, 3, 4}); v != 2.5 {
		t.Errorf("middle([1,2,3,4]) = %f, want 2.5", v)
	}
}

func TestCandidatesEncodeAsJSON(t *testing.T) {
	// A perfectly steady baseline gives MAD == 0, the case that used to
	// produce an infinite z-score and an encoding failure in the API.
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if _, err := json.Marshal(candidates); err != nil {
		t.Fatalf("candidates must be JSON-encodable: %v", err)
	}
}

func TestDetectZeroMADIsNoEvidence(t *testing.T) {
	// A perfectly flat baseline followed by a jump used to score as infinite
	// (then capped). With no spread to judge against it must not trigger.
	fs := constantFeatures(30, 0.0001)
	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0
	for _, c := range Detect(25544, fs, DefaultConfig()) {
		if c.Kind == KindDeltaV {
			t.Fatalf("zero-MAD window must be treated as no evidence, got %+v", c)
		}
	}
}

func TestDetectMinAbsSuppressesQuantisation(t *testing.T) {
	// Inclination published to 4 decimals: most pairs change by 0, a few by
	// one last-digit step over a six-hour gap (0.0001° / 0.25 d = 0.0004°/d).
	// Against a baseline of exact zeros that is a huge z-score but it is
	// below the guard, so it must never become a plane-change candidate.
	fs := make([]feature.Feature, 40)
	for i := range fs {
		rate := 0.0
		if i%2 == 1 {
			rate = 0.0004
		}
		fs[i] = feature.Feature{Epoch: day(i), GapDays: 0.25, InclinationRate: rate}
	}
	fs[30].InclinationRate = 0.0004
	fs[31].InclinationRate = 0.0004
	for _, c := range Detect(25544, fs, DefaultConfig()) {
		if c.Kind == KindPlaneChange {
			t.Fatalf("last-digit steps must be suppressed by MinInclinationRate, got %+v", c)
		}
	}

	// A real plane change well above the guard on a baseline with spread
	// still triggers.
	for i := range fs {
		fs[i].InclinationRate = 0.0004 * float64(i%3)
	}
	fs[30].InclinationRate = 0.05
	fs[31].InclinationRate = 0.05
	var found bool
	for _, c := range Detect(25544, fs, DefaultConfig()) {
		if c.Kind == KindPlaneChange {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a plane-change candidate above the guard")
	}
}

func TestDefaultConfigSetsGuards(t *testing.T) {
	d := DefaultConfig()
	if d.MinMeanMotionDrift <= 0 || d.MinInclinationRate <= 0 || d.MinEccentricityRate <= 0 || d.MinBStarRate <= 0 {
		t.Fatalf("defaults must set every guard: %+v", d)
	}
}

func TestJumpRuleFiresOnSingleStep(t *testing.T) {
	// A clean impulsive reboost: one element set to the next, mean motion
	// drops by 0.007 rev/day, and the following rates are back to normal.
	// The z-score path (persistence 2) misses this; the jump rule must not.
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionJump = -0.007
	fs[20].MeanMotionDrift = -0.028 // the same step over a quarter-day gap
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != KindDeltaV || c.Value != -0.007 {
		t.Errorf("primary = %s %g, want %s -0.007", c.Kind, c.Value, KindDeltaV)
	}
	if len(c.Signals) != 1 || c.Signals[0].Rule != RuleJump {
		t.Errorf("expected a single RuleJump signal, got %+v", c.Signals)
	}
	if !c.EpochStart.Equal(day(20)) || !c.EpochEnd.Equal(day(20)) {
		t.Errorf("event span = %v..%v, want day 20", c.EpochStart, c.EpochEnd)
	}
}

func TestJumpRuleDecayingObjectDoesNotFireOnDrag(t *testing.T) {
	// An object in the last months of decay (perigee ~200 km): drag raises
	// mean motion by ~0.0008 rev/day between element sets with spikes past
	// the absolute threshold. None of those rises is a maneuver.
	fs := steadyFeatures(40, 0.0001)
	for i := range fs {
		fs[i].MeanMotionJump = 0.0008 + float64(i%3)*0.0003 // 0.0008 .. 0.0014
	}
	fs[25].MeanMotionJump = 0.0031 // the largest drag spike seen in real data
	for _, c := range Detect(25544, fs, DefaultConfig()) {
		for _, sig := range c.Signals {
			if sig.Rule == RuleJump {
				t.Fatalf("drag rises must not trigger the jump rule, got %+v", sig)
			}
		}
	}

	// The same object raising its orbit: mean motion drops. Drag cannot do
	// that, so it is a maneuver regardless of the object's noisy baseline.
	fs[25].MeanMotionJump = -0.0031
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 1 || candidates[0].Signals[0].Rule != RuleJump || candidates[0].Value != -0.0031 {
		t.Fatalf("expected one jump candidate for the drop, got %+v", candidates)
	}
}

func TestJumpRulePositiveStepOnQuietObject(t *testing.T) {
	// A lowering burn on a quiet object (ISS-like steps of ~3e-5) raises
	// mean motion far above both the absolute threshold and 5x its median.
	fs := steadyFeatures(40, 0.0001)
	for i := range fs {
		fs[i].MeanMotionJump = 0.00003 * float64(i%2)
	}
	fs[25].MeanMotionJump = 0.002
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 1 || candidates[0].Signals[0].Rule != RuleJump {
		t.Fatalf("expected one jump candidate for the rise, got %+v", candidates)
	}
}

func TestJumpRuleIgnoresSmallSteps(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[20].MeanMotionJump = 0.0004 // drag-sized, below the 0.001 default
	for _, c := range Detect(25544, fs, DefaultConfig()) {
		for _, sig := range c.Signals {
			if sig.Rule == RuleJump {
				t.Fatalf("drag-sized step must not trigger the jump rule: %+v", sig)
			}
		}
	}
}

func TestJumpRuleInclination(t *testing.T) {
	fs := steadyFeatures(30, 0.0001)
	fs[12].InclinationJump = 0.05
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 1 || candidates[0].Kind != KindPlaneChange || candidates[0].Signals[0].Rule != RuleJump {
		t.Fatalf("expected one plane-change jump candidate, got %+v", candidates)
	}
}

func TestMergeOverlappingSignalsIntoOneEvent(t *testing.T) {
	// Reboost signature: a mean-motion jump plus z-score runs on the
	// eccentricity and B* series on the same and following day.
	fs := steadyFeatures(40, 0.0001)
	for i := range fs {
		fs[i].EccentricityRate = 1e-5 + float64(i%3)*1e-6
		fs[i].BStarRate = 1e-5 + float64(i%3)*1e-6
	}
	fs[20].MeanMotionJump = -0.007
	fs[20].EccentricityRate = 5e-4
	fs[21].EccentricityRate = 5e-4
	fs[21].BStarRate = 1e-3
	fs[22].BStarRate = 1e-3

	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 1 {
		t.Fatalf("expected the three signals to merge into 1 event, got %d: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.Kind != KindDeltaV {
		t.Errorf("primary kind = %s, want the jump (%s)", c.Kind, KindDeltaV)
	}
	if len(c.Signals) != 3 {
		t.Errorf("expected 3 signals, got %d: %+v", len(c.Signals), c.Signals)
	}
	if !c.EpochStart.Equal(day(20)) || !c.EpochEnd.Equal(day(22)) {
		t.Errorf("event span = %v..%v, want day 20..22", c.EpochStart, c.EpochEnd)
	}
	if c.ZScore <= 0 {
		t.Errorf("event ZScore should carry the peak of the z-score signals, got %g", c.ZScore)
	}
}

func TestMergeKeepsDistantSignalsApart(t *testing.T) {
	fs := steadyFeatures(40, 0.0001)
	fs[10].MeanMotionJump = -0.007
	fs[30].MeanMotionJump = -0.007
	candidates := Detect(25544, fs, DefaultConfig())
	if len(candidates) != 2 {
		t.Fatalf("expected 2 events 20 days apart, got %d", len(candidates))
	}
}

func TestDefaultThresholdIsConservative(t *testing.T) {
	if d := DefaultConfig(); d.Threshold < 6 || d.Threshold > 8 {
		t.Fatalf("default Threshold = %g, want 6..8 for the persistence path", d.Threshold)
	}
}
