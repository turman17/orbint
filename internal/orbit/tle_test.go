package orbit

import (
	"math"
	"testing"
	"time"
)

const (
	issLine1 = "1 25544U 98067A   26269.39984368  .00031849  00000+0  59036-3 0  9995"
	issLine2 = "2 25544  51.6292 159.1757 0007020 184.2024 175.8906 15.48664613587440"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestParseTle_ThreeLineISS(t *testing.T) {
	lines := []string{
		"ISS (ZARYA)",
		issLine1,
		issLine2,
	}

	tle, err := ParseTle(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tle.Name != "ISS (ZARYA)" {
		t.Errorf("Name = %q, want %q", tle.Name, "ISS (ZARYA)")
	}
	if tle.ID != 25544 {
		t.Errorf("ID = %d, want 25544", tle.ID)
	}
	if tle.Class != 'U' {
		t.Errorf("Class = %c, want U", tle.Class)
	}
	if tle.InternationalDesignator != "98067A" {
		t.Errorf("InternationalDesignator = %q, want %q", tle.InternationalDesignator, "98067A")
	}
	if tle.ElementSetNumber != 999 {
		t.Errorf("ElementSetNumber = %d, want 999", tle.ElementSetNumber)
	}

	wantEpoch := time.Date(2026, time.September, 26, 9, 35, 46, 0, time.UTC)
	if tle.Epoch.Sub(wantEpoch).Abs() > time.Second {
		t.Errorf("Epoch = %v, want ~%v", tle.Epoch, wantEpoch)
	}

	if !almostEqual(tle.Inclination, 51.6292, 1e-4) {
		t.Errorf("Inclination = %f, want 51.6292", tle.Inclination)
	}
	if !almostEqual(tle.RAAN, 159.1757, 1e-4) {
		t.Errorf("RAAN = %f, want 159.1757", tle.RAAN)
	}
	if !almostEqual(tle.Eccentricity, 0.0007020, 1e-7) {
		t.Errorf("Eccentricity = %f, want 0.0007020", tle.Eccentricity)
	}
	if !almostEqual(tle.ArgumentOfPerigee, 184.2024, 1e-4) {
		t.Errorf("ArgumentOfPerigee = %f, want 184.2024", tle.ArgumentOfPerigee)
	}
	if !almostEqual(tle.MeanAnomaly, 175.8906, 1e-4) {
		t.Errorf("MeanAnomaly = %f, want 175.8906", tle.MeanAnomaly)
	}
	if !almostEqual(tle.MeanMotion, 15.48664613, 1e-8) {
		t.Errorf("MeanMotion = %f, want 15.48664613", tle.MeanMotion)
	}
	if !almostEqual(tle.MeanMotionDot, 0.00063698, 1e-8) {
		t.Errorf("MeanMotionDot = %f, want 0.00063698 (stored/2 * 2)", tle.MeanMotionDot)
	}
	if !almostEqual(tle.MeanMotionDotDot, 0, 1e-10) {
		t.Errorf("MeanMotionDotDot = %f, want 0", tle.MeanMotionDotDot)
	}
	if !almostEqual(tle.BStar, 0.59036e-3, 1e-8) {
		t.Errorf("BStar = %e, want 0.59036e-3", tle.BStar)
	}
	if tle.EphemerisType != 0 {
		t.Errorf("EphemerisType = %d, want 0", tle.EphemerisType)
	}
	if tle.RevolutionNumber != 58744 {
		t.Errorf("RevolutionNumber = %d, want 58744", tle.RevolutionNumber)
	}
}

func TestParseTle_TwoLineNoName(t *testing.T) {
	lines := []string{issLine1, issLine2}

	tle, err := ParseTle(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tle.Name != "" {
		t.Errorf("Name = %q, want empty", tle.Name)
	}
	if tle.ID != 25544 {
		t.Errorf("ID = %d, want 25544", tle.ID)
	}
}

func TestParseTle_EpochYearRule(t *testing.T) {
	// year 57-99 → 1900s; build a TLE with epoch year 99
	line1 := "1 25544U 98067A   99269.39984368  .00031849  00000+0  59036-3 0  9992"
	line2 := "2 25544  51.6292 159.1757 0007020 184.2024 175.8906 15.48664613587440"

	// need to fix checksum for line1
	line1 = fixChecksum(line1)

	tle, err := ParseTle([]string{line1, line2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tle.Epoch.Year() != 1999 {
		t.Errorf("Epoch.Year() = %d, want 1999", tle.Epoch.Year())
	}
}

func TestParseTle_NegativeBStar(t *testing.T) {
	// Modify ISS line1 to have negative BStar: -59036-3
	line1 := "1 25544U 98067A   26269.39984368  .00031849  00000+0 -59036-3 0  9990"
	line1 = fixChecksum(line1)

	tle, err := ParseTle([]string{line1, issLine2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(tle.BStar, -0.59036e-3, 1e-8) {
		t.Errorf("BStar = %e, want -0.59036e-3", tle.BStar)
	}
}

func TestParseTle_InvalidCases(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{
			name:  "too few lines",
			lines: []string{issLine1},
		},
		{
			name:  "too many lines",
			lines: []string{"NAME", "EXTRA", issLine1, issLine2},
		},
		{
			name:  "wrong line1 length",
			lines: []string{issLine1[:68], issLine2},
		},
		{
			name:  "wrong line2 length",
			lines: []string{issLine1, issLine2[:68]},
		},
		{
			name: "bad checksum line1",
			lines: []string{
				issLine1[:68] + "0",
				issLine2,
			},
		},
		{
			name: "bad checksum line2",
			lines: []string{
				issLine1,
				issLine2[:68] + "1",
			},
		},
		{
			name: "mismatched IDs",
			lines: []string{
				issLine1,
				fixChecksum("2 99999  51.6292 159.1757 0007020 184.2024 175.8906 15.48664613587440"),
			},
		},
		{
			name: "invalid classification",
			lines: []string{
				fixChecksum("1 25544X 98067A   26269.39984368  .00031849  00000+0  59036-3 0  9990"),
				issLine2,
			},
		},
		{
			name: "line1 doesn't start with 1",
			lines: []string{
				"3" + issLine1[1:],
				issLine2,
			},
		},
		{
			name: "line2 doesn't start with 2",
			lines: []string{
				issLine1,
				"3" + issLine2[1:],
			},
		},
		{
			name:  "empty input",
			lines: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTle(tt.lines)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseTle_NameWhitespaceTrimmed(t *testing.T) {
	lines := []string{
		"  ISS (ZARYA)   ",
		issLine1,
		issLine2,
	}

	tle, err := ParseTle(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tle.Name != "ISS (ZARYA)" {
		t.Errorf("Name = %q, want %q", tle.Name, "ISS (ZARYA)")
	}
}

func TestParseTle_TrailingCRLF(t *testing.T) {
	lines := []string{
		issLine1 + "\r\n",
		issLine2 + "\r\n",
	}

	_, err := ParseTle(lines)
	if err != nil {
		t.Fatalf("unexpected error with trailing CRLF: %v", err)
	}
}

func TestParseTle_LeapYearEpoch(t *testing.T) {
	// 2024 is a leap year; day 366 should be valid
	line1 := fixChecksum("1 25544U 98067A   24366.50000000  .00031849  00000+0  59036-3 0  9990")

	tle, err := ParseTle([]string{line1, issLine2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tle.Epoch.Year() != 2024 {
		t.Errorf("Epoch.Year() = %d, want 2024", tle.Epoch.Year())
	}
	if tle.Epoch.Month() != time.December || tle.Epoch.Day() != 31 {
		t.Errorf("Epoch = %v, want Dec 31", tle.Epoch)
	}
}

func TestParseTle_NonLeapYearDay366Invalid(t *testing.T) {
	// 2025 is not a leap year; day 366 should be invalid
	line1 := fixChecksum("1 25544U 98067A   25366.50000000  .00031849  00000+0  59036-3 0  9990")

	_, err := ParseTle([]string{line1, issLine2})
	if err == nil {
		t.Error("expected error for day 366 in non-leap year, got nil")
	}
}

// fixChecksum recalculates and replaces the last character of a 69-char TLE line.
func fixChecksum(line string) string {
	if len(line) < 69 {
		for len(line) < 69 {
			line += " "
		}
	}
	sum := 0
	for i := 0; i < 68; i++ {
		switch {
		case line[i] >= '0' && line[i] <= '9':
			sum += int(line[i] - '0')
		case line[i] == '-':
			sum++
		}
	}
	return line[:68] + string(rune('0'+sum%10))
}
