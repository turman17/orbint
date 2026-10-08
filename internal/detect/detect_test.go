package detect

import (
	"math"
	"testing"
	"time"

	"github.com/turman17/orbint/internal/feature"
)

var epoch0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func day(n int) time.Time {
	return epoch0.Add(time.Duration(n) * 24 * time.Hour)
}

func steadyFeatures(n int, drift float64) []feature.Feature {
	fs := make([]feature.Feature, n)
	for i := range fs {
		fs[i] = feature.Feature{
			Epoch:           day(i),
			GapDays:         1.0,
			MeanMotionDrift: drift,
		}
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
	fs := steadyFeatures(30, 0.0001)
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
		fs[i].InclinationRate = 0.0001
		fs[i].EccentricityRate = 0.00001
		fs[i].BStarDelta = 0.00001
	}

	fs[20].MeanMotionDrift = 5.0
	fs[21].MeanMotionDrift = 5.0

	fs[20].InclinationRate = 3.0
	fs[21].InclinationRate = 3.0

	cfg := Config{WindowSize: 10, Threshold: 3.5, Persistence: 2}
	candidates := Detect(25544, fs, cfg)

	kinds := make(map[Kind]bool)
	for _, c := range candidates {
		kinds[c.Kind] = true
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
		fs[i].InclinationRate = 0.0001
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
	if z != 0 {
		t.Errorf("z = %f, want 0 when value == median and MAD == 0", z)
	}

	z = robustZScore(6.0, 5.0, 0.0)
	if !math.IsInf(z, 1) {
		t.Errorf("z = %f, want +Inf when value != median and MAD == 0", z)
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
