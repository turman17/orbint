package orbit

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidTLE = errors.New("invalid TLE")

// TLE holds the decoded data of one two-line element set.
// Values are stored in TLE-native units (degrees, rev/day) and decoded
// from TLE notation: implied decimals and exponents are already applied.
type TLE struct {
	// Identity
	Name                    string // optional name line; "" if absent
	ID                      int    // NORAD catalog number; Alpha-5 IDs not supported
	Class                   byte   // 'U', 'C' or 'S'
	InternationalDesignator string // e.g. "98067A": launch year, launch number, piece
	ElementSetNumber        int    // incremented on updates; wraps, don't rely on it

	// Time
	Epoch time.Time // UTC moment the elements are valid for

	// Orbit shape and orientation
	Inclination       float64 // degrees
	RAAN              float64 // degrees
	Eccentricity      float64 // dimensionless, 0 <= e < 1
	ArgumentOfPerigee float64 // degrees
	MeanAnomaly       float64 // degrees
	MeanMotion        float64 // rev/day

	// Drag and decay
	MeanMotionDot    float64 // rev/day², real first derivative (TLE stores it /2; parser multiplies by 2)
	MeanMotionDotDot float64 // rev/day³, real second derivative (TLE stores it /6; parser multiplies by 6)
	BStar            float64 // 1/Earth radii, decoded from "59036-3" form

	// Other
	EphemerisType    int // always 0 in public TLEs
	RevolutionNumber int // wraps at 99999, unreliable for long-lived objects
}

func parseTleLines(line1, line2 string) (TLE, error) {
	line1 = strings.TrimRight(line1, " \r\n")
	line2 = strings.TrimRight(line2, " \r\n")

	if len(line1) != 69 || len(line2) != 69 || line1[0] != '1' || line2[0] != '2' {
		return TLE{}, ErrInvalidTLE
	}

	validChecksum := func(line string) bool {
		if line[68] < '0' || line[68] > '9' {
			return false
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
		return sum%10 == int(line[68]-'0')
	}
	if !validChecksum(line1) || !validChecksum(line2) {
		return TLE{}, ErrInvalidTLE
	}


	trim := func(s string) string {
		start, end := 0, len(s)
		for start < end && s[start] == ' ' {
			start++
		}
		for end > start && s[end-1] == ' ' {
			end--
		}
		return s[start:end]
	}

	parseInt := func(s string) (int, bool) {
		s = trim(s)
		if s == "" {
			return 0, false
		}
		value := 0
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return 0, false
			}
			value = value*10 + int(s[i]-'0')
		}
		return value, true
	}

	parseFloat := func(s string) (float64, bool) {
		var value float64
		n, err := fmt.Sscanf(s, "%f", &value)
		return value, err == nil && n == 1
	}

	pow10 := func(exponent int) float64 {
		value := 1.0
		if exponent >= 0 {
			for i := 0; i < exponent; i++ {
				value *= 10
			}
		} else {
			for i := 0; i > exponent; i-- {
				value /= 10
			}
		}
		return value
	}

	// TLE exponent fields have an implied decimal point. For example,
	// " 39617-4" means 0.39617e-4.
	parseExponent := func(s string) (float64, bool) {
		if len(s) != 8 || (s[0] != ' ' && s[0] != '+' && s[0] != '-') ||
			(s[6] != '+' && s[6] != '-') {
			return 0, false
		}
		mantissa, ok := parseInt(s[1:6])
		if !ok || s[7] < '0' || s[7] > '9' {
			return 0, false
		}
		exponent := int(s[7] - '0')
		if s[6] == '-' {
			exponent = -exponent
		}
		value := float64(mantissa) / 100000 * pow10(exponent)
		if s[0] == '-' {
			value = -value
		}
		return value, true
	}

	var tle TLE
	var ok bool

	if tle.ID, ok = parseInt(line1[2:7]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	line2ID, ok := parseInt(line2[2:7])
	if !ok || line2ID != tle.ID {
		return TLE{}, ErrInvalidTLE
	}

	tle.Class = line1[7]
	if tle.Class != 'U' && tle.Class != 'C' && tle.Class != 'S' {
		return TLE{}, ErrInvalidTLE
	}
	tle.InternationalDesignator = trim(line1[9:17])

	epochYear, ok := parseInt(line1[18:20])
	if !ok {
		return TLE{}, ErrInvalidTLE
	}
	epochDay, ok := parseFloat(line1[20:32])
	if !ok {
		return TLE{}, ErrInvalidTLE
	}
	year := 2000 + epochYear
	if epochYear >= 57 {
		year = 1900 + epochYear
	}
	maxDay := 365
	if year%400 == 0 || (year%4 == 0 && year%100 != 0) {
		maxDay = 366
	}
	if epochDay < 1 || epochDay >= float64(maxDay+1) {
		return TLE{}, ErrInvalidTLE
	}
	tle.Epoch = time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).
		Add(time.Duration((epochDay - 1) * float64(24*time.Hour)))

	storedDot, ok := parseFloat(line1[33:43])
	if !ok {
		return TLE{}, ErrInvalidTLE
	}
	tle.MeanMotionDot = storedDot * 2
	storedDotDot, ok := parseExponent(line1[44:52])
	if !ok {
		return TLE{}, ErrInvalidTLE
	}
	tle.MeanMotionDotDot = storedDotDot * 6
	if tle.BStar, ok = parseExponent(line1[53:61]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.EphemerisType, ok = parseInt(line1[62:63]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.ElementSetNumber, ok = parseInt(line1[64:68]); !ok {
		return TLE{}, ErrInvalidTLE
	}

	if tle.Inclination, ok = parseFloat(line2[8:16]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.RAAN, ok = parseFloat(line2[17:25]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	eccentricity, ok := parseInt(line2[26:33])
	if !ok {
		return TLE{}, ErrInvalidTLE
	}
	tle.Eccentricity = float64(eccentricity) / 10000000
	if tle.ArgumentOfPerigee, ok = parseFloat(line2[34:42]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.MeanAnomaly, ok = parseFloat(line2[43:51]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.MeanMotion, ok = parseFloat(line2[52:63]); !ok {
		return TLE{}, ErrInvalidTLE
	}
	if tle.RevolutionNumber, ok = parseInt(line2[63:68]); !ok {
		return TLE{}, ErrInvalidTLE
	}

	if tle.Inclination < 0 || tle.Inclination > 180 ||
		tle.RAAN < 0 || tle.RAAN >= 360 ||
		tle.Eccentricity < 0 || tle.Eccentricity >= 1 ||
		tle.ArgumentOfPerigee < 0 || tle.ArgumentOfPerigee >= 360 ||
		tle.MeanAnomaly < 0 || tle.MeanAnomaly >= 360 || tle.MeanMotion <= 0 {
		return TLE{}, ErrInvalidTLE
	}

	return tle, nil
}

func ParseTle(lines []string) (TLE, error) {
	if len(lines) < 2 || len(lines) > 3 {
		return TLE{}, ErrInvalidTLE
	}

	name := ""
	if len(lines) == 3 {
		name = strings.TrimSpace(lines[0])
		lines = lines[1:]
	}

	tle, err := parseTleLines(lines[0], lines[1])
	if err != nil {
		return TLE{}, err
	}
	tle.Name = name
	return tle, nil
}
