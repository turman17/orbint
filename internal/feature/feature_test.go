package feature

import (
	"math"
	"testing"
	"time"

	"github.com/turman17/orbint/internal/orbit"
)

func makeTLE(epoch time.Time, meanMotion, inclination, eccentricity, bstar float64) orbit.TLE {
	return orbit.TLE{
		ID:           25544,
		Name:         "ISS",
		Epoch:        epoch,
		MeanMotion:   meanMotion,
		Inclination:  inclination,
		Eccentricity: eccentricity,
		BStar:        bstar,
	}
}

var epoch0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestComputeNil(t *testing.T) {
	if got := Compute(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %d features", len(got))
	}
	if got := Compute([]orbit.TLE{}); got != nil {
		t.Fatalf("expected nil for empty input, got %d features", len(got))
	}
}

func TestComputeSingleElement(t *testing.T) {
	if got := Compute([]orbit.TLE{makeTLE(epoch0, 15.5, 51.6, 0.0007, 0.0001)}); got != nil {
		t.Fatalf("expected nil for single element, got %d features", len(got))
	}
}

func TestComputeBasicPair(t *testing.T) {
	t1 := makeTLE(epoch0, 15.500, 51.60, 0.00070, 0.000100)
	t2 := makeTLE(epoch0.Add(24*time.Hour), 15.502, 51.61, 0.00072, 0.000105)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	f := features[0]
	if !f.Epoch.Equal(t2.Epoch) {
		t.Errorf("epoch = %v, want %v", f.Epoch, t2.Epoch)
	}
	if math.Abs(f.GapDays-1.0) > 1e-9 {
		t.Errorf("GapDays = %f, want 1.0", f.GapDays)
	}
	if math.Abs(f.MeanMotionJump-0.002) > 1e-9 {
		t.Errorf("MeanMotionJump = %e, want 0.002", f.MeanMotionJump)
	}
	if math.Abs(f.MeanMotionDrift-0.002) > 1e-9 {
		t.Errorf("MeanMotionDrift = %e, want 0.002", f.MeanMotionDrift)
	}
	if math.Abs(f.InclinationJump-0.01) > 1e-9 {
		t.Errorf("InclinationJump = %e, want 0.01", f.InclinationJump)
	}
	if math.Abs(f.InclinationRate-0.01) > 1e-9 {
		t.Errorf("InclinationRate = %e, want 0.01", f.InclinationRate)
	}
	if math.Abs(f.EccentricityRate-0.00002) > 1e-9 {
		t.Errorf("EccentricityRate = %e, want 0.00002", f.EccentricityRate)
	}
	if math.Abs(f.BStarRate-0.000005) > 1e-9 {
		t.Errorf("BStarRate = %e, want 0.000005", f.BStarRate)
	}
}

func TestComputeRatesScaleWithGap(t *testing.T) {
	t1 := makeTLE(epoch0, 15.500, 51.60, 0.00070, 0.0001)
	t2 := makeTLE(epoch0.Add(48*time.Hour), 15.502, 51.62, 0.00074, 0.0002)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	f := features[0]
	if math.Abs(f.GapDays-2.0) > 1e-9 {
		t.Errorf("GapDays = %f, want 2.0", f.GapDays)
	}
	if math.Abs(f.MeanMotionJump-0.002) > 1e-9 {
		t.Errorf("MeanMotionJump = %e, want 0.002 (raw jump, not rate)", f.MeanMotionJump)
	}
	if math.Abs(f.MeanMotionDrift-0.001) > 1e-9 {
		t.Errorf("MeanMotionDrift = %e, want 0.001 (0.002 / 2 days)", f.MeanMotionDrift)
	}
}

func TestComputeMultipleElements(t *testing.T) {
	history := []orbit.TLE{
		makeTLE(epoch0, 15.500, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(24*time.Hour), 15.501, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(48*time.Hour), 15.502, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(72*time.Hour), 15.503, 51.60, 0.0007, 0.0001),
	}

	features := Compute(history)
	if len(features) != 3 {
		t.Fatalf("expected 3 features from 4 elements, got %d", len(features))
	}
	for i, f := range features {
		if math.Abs(f.MeanMotionDrift-0.001) > 1e-9 {
			t.Errorf("feature[%d]: MeanMotionDrift = %e, want 0.001", i, f.MeanMotionDrift)
		}
	}
}

func TestComputeSkipsLargeGap(t *testing.T) {
	t1 := makeTLE(epoch0, 15.500, 51.60, 0.0007, 0.0001)
	t2 := makeTLE(epoch0.Add(31*24*time.Hour), 15.502, 51.60, 0.0007, 0.0001)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 0 {
		t.Fatalf("expected 0 features for 31-day gap, got %d", len(features))
	}
}

func TestComputeSkipsZeroGap(t *testing.T) {
	t1 := makeTLE(epoch0, 15.500, 51.60, 0.0007, 0.0001)
	t2 := makeTLE(epoch0, 15.502, 51.60, 0.0007, 0.0001)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 0 {
		t.Fatalf("expected 0 features for zero gap, got %d", len(features))
	}
}

func TestComputeHandlesReversedEpochs(t *testing.T) {
	t1 := makeTLE(epoch0.Add(24*time.Hour), 15.502, 51.61, 0.00072, 0.0001)
	t2 := makeTLE(epoch0, 15.500, 51.60, 0.00070, 0.0001)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature for reversed pair, got %d", len(features))
	}
	f := features[0]
	if !f.Epoch.Equal(t1.Epoch) {
		t.Errorf("epoch should be the later one: got %v, want %v", f.Epoch, t1.Epoch)
	}
	if f.MeanMotionJump < 0 {
		t.Errorf("MeanMotionJump should be positive after swap, got %e", f.MeanMotionJump)
	}
}

func TestComputeGapInMiddle(t *testing.T) {
	history := []orbit.TLE{
		makeTLE(epoch0, 15.500, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(24*time.Hour), 15.501, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(60*24*time.Hour), 15.510, 51.60, 0.0007, 0.0001),
		makeTLE(epoch0.Add(61*24*time.Hour), 15.511, 51.60, 0.0007, 0.0001),
	}

	features := Compute(history)
	if len(features) != 2 {
		t.Fatalf("expected 2 features (middle pair skipped), got %d", len(features))
	}
	if !features[0].Epoch.Equal(history[1].Epoch) {
		t.Errorf("first feature epoch wrong: got %v, want %v", features[0].Epoch, history[1].Epoch)
	}
	if !features[1].Epoch.Equal(history[3].Epoch) {
		t.Errorf("second feature epoch wrong: got %v, want %v", features[1].Epoch, history[3].Epoch)
	}
}

func TestComputeSemiMajorAxis(t *testing.T) {
	// ISS mean motion ~15.5 rev/day → semi-major axis ~6780 km.
	// A small mean motion increase lowers the orbit (smaller semi-major axis).
	t1 := makeTLE(epoch0, 15.500, 51.60, 0.0007, 0.0001)
	t2 := makeTLE(epoch0.Add(24*time.Hour), 15.510, 51.60, 0.0007, 0.0001)

	features := Compute([]orbit.TLE{t1, t2})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	if features[0].SemiMajorRate >= 0 {
		t.Errorf("SemiMajorRate should be negative when mean motion increases, got %f", features[0].SemiMajorRate)
	}

	const mu = 398600.4418
	n1 := 15.500 * 2 * math.Pi / 86400
	n2 := 15.510 * 2 * math.Pi / 86400
	expectedDelta := math.Cbrt(mu/(n2*n2)) - math.Cbrt(mu/(n1*n1))
	if math.Abs(features[0].SemiMajorRate-expectedDelta) > 1e-6 {
		t.Errorf("SemiMajorRate = %f, expected %f", features[0].SemiMajorRate, expectedDelta)
	}
}

func TestComputeManeuverSignature(t *testing.T) {
	// Simulate an ISS reboost: steady mean motion, then a sudden jump.
	history := make([]orbit.TLE, 10)
	for i := range history {
		mm := 15.500
		if i >= 7 {
			mm = 15.520
		}
		history[i] = makeTLE(epoch0.Add(time.Duration(i)*24*time.Hour), mm, 51.60, 0.0007, 0.0001)
	}

	features := Compute(history)
	if len(features) != 9 {
		t.Fatalf("expected 9 features, got %d", len(features))
	}

	maxDrift := 0.0
	maxIdx := 0
	for i, f := range features {
		if math.Abs(f.MeanMotionDrift) > maxDrift {
			maxDrift = math.Abs(f.MeanMotionDrift)
			maxIdx = i
		}
	}
	if maxIdx != 6 {
		t.Errorf("peak drift at index %d, expected 6 (the reboost boundary)", maxIdx)
	}
	if maxDrift < 0.01 {
		t.Errorf("peak drift = %e, expected a clear maneuver signal (>0.01)", maxDrift)
	}
}
