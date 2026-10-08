package feature

import (
	"math"
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
	BStarDelta       float64 // 1/(Earth radii) per day
	SemiMajorDelta   float64 // km per day — derived from mean motion

	// Raw deltas (not per day — the absolute jump)
	MeanMotionJump  float64 // rev/day
	InclinationJump float64 // deg
}

func Compute(history []orbit.TLE) []Feature {
	if len(history) < 2 {
		return nil
	}

	const (
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

	features := make([]Feature, 0, len(history)-1)
	for i := 1; i < len(history); i++ {
		earlier, later := history[i-1], history[i]
		if later.Epoch.Before(earlier.Epoch) {
			earlier, later = later, earlier
		}

		gapDays := later.Epoch.Sub(earlier.Epoch).Hours() / 24
		if gapDays <= 0 || gapDays > maxGapDays {
			continue
		}

		meanMotionJump := later.MeanMotion - earlier.MeanMotion
		inclinationJump := later.Inclination - earlier.Inclination
		semiMajorJump := semiMajorAxis(later.MeanMotion) - semiMajorAxis(earlier.MeanMotion)

		features = append(features, Feature{
			Epoch:             later.Epoch,
			GapDays:           gapDays,
			MeanMotionDrift:   meanMotionJump / gapDays,
			InclinationRate:   inclinationJump / gapDays,
			EccentricityRate:  (later.Eccentricity - earlier.Eccentricity) / gapDays,
			BStarDelta:        (later.BStar - earlier.BStar) / gapDays,
			SemiMajorDelta:    semiMajorJump / gapDays,
			MeanMotionJump:    meanMotionJump,
			InclinationJump:   inclinationJump,
		})
	}

	return features
}

