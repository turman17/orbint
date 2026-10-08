package feature

import (
	"math"
	"sort"
	"time"

	"github.com/turman17/orbint/internal/orbit"
)

type Feature struct {
	Epoch   time.Time // epoch of the later element set
	GapDays float64   // time between the two epochs, in days

	// Rates (per day)
	MeanMotionDrift  float64 // rev/day per day — primary maneuver signal
	InclinationRate  float64 // deg/day
	EccentricityRate float64 // 1/day
	BStarRate        float64 // 1/(Earth radii) per day
	SemiMajorRate    float64 // km per day — derived from mean motion

	// Raw deltas (not per day — the absolute jump)
	MeanMotionJump  float64 // rev/day
	InclinationJump float64 // deg
}

func Compute(history []orbit.TLE) []Feature {
	if len(history) < 2 {
		return nil
	}

	const (
		// Element sets closer than this are the same observation published
		// twice (Space-Track keeps re-issues at the same epoch with different
		// precision). Dividing their last-digit differences by a sub-second
		// gap would produce absurd rates, so they are collapsed first.
		minGapDays    = 0.5 / 24
		maxGapDays    = 30.0
		secondsPerDay = 86400.0
		twoPi         = 2 * math.Pi
		mu            = 398600.4418 // Earth's gravitational parameter, km^3/s^2
	)

	// Mean motion is stored in revolutions per day. Convert it to radians per
	// second before applying a = cbrt(mu / n^2).
	semiMajorAxis := func(meanMotion float64) float64 {
		n := meanMotion * twoPi / secondsPerDay
		if n <= 0 {
			return 0
		}

		return math.Cbrt(mu / (n * n))
	}

	// Work on a sorted copy so callers can pass history in any order, then
	// collapse near-duplicate epochs keeping the later record.
	ordered := append([]orbit.TLE(nil), history...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Epoch.Before(ordered[j].Epoch)
	})
	kept := ordered[:0]
	for _, tle := range ordered {
		if n := len(kept); n > 0 && tle.Epoch.Sub(kept[n-1].Epoch).Hours()/24 < minGapDays {
			kept[n-1] = tle
			continue
		}
		kept = append(kept, tle)
	}
	if len(kept) < 2 {
		return nil
	}

	features := make([]Feature, 0, len(kept)-1)
	for i := 1; i < len(kept); i++ {
		earlier, later := kept[i-1], kept[i]

		gapDays := later.Epoch.Sub(earlier.Epoch).Hours() / 24
		if gapDays > maxGapDays {
			continue
		}

		meanMotionJump := later.MeanMotion - earlier.MeanMotion
		inclinationJump := later.Inclination - earlier.Inclination
		semiMajorJump := semiMajorAxis(later.MeanMotion) - semiMajorAxis(earlier.MeanMotion)

		features = append(features, Feature{
			Epoch:            later.Epoch,
			GapDays:          gapDays,
			MeanMotionDrift:  meanMotionJump / gapDays,
			InclinationRate:  inclinationJump / gapDays,
			EccentricityRate: (later.Eccentricity - earlier.Eccentricity) / gapDays,
			BStarRate:        (later.BStar - earlier.BStar) / gapDays,
			SemiMajorRate:    semiMajorJump / gapDays,
			MeanMotionJump:   meanMotionJump,
			InclinationJump:  inclinationJump,
		})
	}

	return features
}
