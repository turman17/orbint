package orbit

import (
	"os"
	"testing"
	"time"
)

func TestParseOMMJSON_ISSFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/iss.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tles) != 1 {
		t.Fatalf("got %d TLEs, want 1", len(tles))
	}

	tle := tles[0]

	if tle.Name != "ISS (ZARYA)" {
		t.Errorf("Name = %q, want %q", tle.Name, "ISS (ZARYA)")
	}
	if tle.ID != 25544 {
		t.Errorf("ID = %d, want 25544", tle.ID)
	}
	if tle.Class != 'U' {
		t.Errorf("Class = %c, want U", tle.Class)
	}
	if tle.InternationalDesignator != "1998-067A" {
		t.Errorf("InternationalDesignator = %q, want %q", tle.InternationalDesignator, "1998-067A")
	}
	if tle.ElementSetNumber != 999 {
		t.Errorf("ElementSetNumber = %d, want 999", tle.ElementSetNumber)
	}

	wantEpoch := time.Date(2026, time.October, 6, 12, 44, 7, 877472000, time.UTC)
	if tle.Epoch.Sub(wantEpoch).Abs() > time.Millisecond {
		t.Errorf("Epoch = %v, want %v", tle.Epoch, wantEpoch)
	}

	if !almostEqual(tle.Inclination, 51.6312, 1e-4) {
		t.Errorf("Inclination = %f, want 51.6312", tle.Inclination)
	}
	if !almostEqual(tle.RAAN, 109.0734, 1e-4) {
		t.Errorf("RAAN = %f, want 109.0734", tle.RAAN)
	}
	if !almostEqual(tle.Eccentricity, 0.00068694, 1e-8) {
		t.Errorf("Eccentricity = %f, want 0.00068694", tle.Eccentricity)
	}
	if !almostEqual(tle.ArgumentOfPerigee, 229.798, 1e-3) {
		t.Errorf("ArgumentOfPerigee = %f, want 229.798", tle.ArgumentOfPerigee)
	}
	if !almostEqual(tle.MeanAnomaly, 130.2407, 1e-4) {
		t.Errorf("MeanAnomaly = %f, want 130.2407", tle.MeanAnomaly)
	}
	if !almostEqual(tle.MeanMotion, 15.48752789, 1e-8) {
		t.Errorf("MeanMotion = %f, want 15.48752789", tle.MeanMotion)
	}
	// Fixture MEAN_MOTION_DOT is 4.741e-5 in TLE convention (ndot/2).
	if !almostEqual(tle.MeanMotionDot, 9.482e-5, 1e-9) {
		t.Errorf("MeanMotionDot = %e, want 9.482e-5 (4.741e-5 × 2)", tle.MeanMotionDot)
	}
	if !almostEqual(tle.MeanMotionDotDot, 0, 1e-10) {
		t.Errorf("MeanMotionDotDot = %f, want 0", tle.MeanMotionDotDot)
	}
	if !almostEqual(tle.BStar, 9.4937468e-5, 1e-10) {
		t.Errorf("BStar = %e, want 9.4937468e-5", tle.BStar)
	}
	if tle.EphemerisType != 0 {
		t.Errorf("EphemerisType = %d, want 0", tle.EphemerisType)
	}
	if tle.RevolutionNumber != 58901 {
		t.Errorf("RevolutionNumber = %d, want 58901", tle.RevolutionNumber)
	}
}

func TestParseOMMJSON_MultipleRecords(t *testing.T) {
	data := []byte(`[
		{"OBJECT_NAME":"SAT-A","OBJECT_ID":"2020-001A","EPOCH":"2026-01-15T10:30:00.000000",
		 "MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		 "RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		 "EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":50001,
		 "ELEMENT_SET_NO":100,"REV_AT_EPOCH":1000,"BSTAR":0.0001,
		 "MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0},
		{"OBJECT_NAME":"SAT-B","OBJECT_ID":"2020-001B","EPOCH":"2026-01-15T10:30:00.000000",
		 "MEAN_MOTION":14.5,"ECCENTRICITY":0.002,"INCLINATION":52.0,
		 "RA_OF_ASC_NODE":101.0,"ARG_OF_PERICENTER":201.0,"MEAN_ANOMALY":301.0,
		 "EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":50002,
		 "ELEMENT_SET_NO":101,"REV_AT_EPOCH":1001,"BSTAR":0.0002,
		 "MEAN_MOTION_DOT":0.00002,"MEAN_MOTION_DDOT":0}
	]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tles) != 2 {
		t.Fatalf("got %d TLEs, want 2", len(tles))
	}
	if tles[0].Name != "SAT-A" {
		t.Errorf("tles[0].Name = %q, want %q", tles[0].Name, "SAT-A")
	}
	if tles[0].ID != 50001 {
		t.Errorf("tles[0].ID = %d, want 50001", tles[0].ID)
	}
	if tles[1].Name != "SAT-B" {
		t.Errorf("tles[1].Name = %q, want %q", tles[1].Name, "SAT-B")
	}
	if tles[1].ID != 50002 {
		t.Errorf("tles[1].ID = %d, want 50002", tles[1].ID)
	}
}

func TestParseOMMJSON_EmptyArray(t *testing.T) {
	tles, err := ParseOMMJSON([]byte(`[]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tles) != 0 {
		t.Errorf("got %d TLEs, want 0", len(tles))
	}
}

func TestParseOMMJSON_InvalidJSON(t *testing.T) {
	_, err := ParseOMMJSON([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseOMMJSON_InvalidEpoch(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"TEST","OBJECT_ID":"2020-001A","EPOCH":"not-a-date",
		"MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		"RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":99999,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":1,"BSTAR":0.0001,
		"MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0
	}]`)

	_, err := ParseOMMJSON(data)
	if err == nil {
		t.Error("expected error for invalid epoch, got nil")
	}
}

func TestParseOMMJSON_EmptyClassification(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"TEST","OBJECT_ID":"2020-001A","EPOCH":"2026-01-15T10:30:00.000000",
		"MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		"RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"","NORAD_CAT_ID":99999,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":1,"BSTAR":0.0001,
		"MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0
	}]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tles[0].Class != 0 {
		t.Errorf("Class = %d, want 0 for empty classification", tles[0].Class)
	}
}

func TestParseOMMJSON_AppliesDerivativeMultipliers(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"TEST","OBJECT_ID":"2020-001A","EPOCH":"2026-01-15T10:30:00.000000",
		"MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		"RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":99999,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":1,"BSTAR":0.0001,
		"MEAN_MOTION_DOT":0.00005,"MEAN_MOTION_DDOT":0.000001
	}]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// OMM uses the same ndot/2, nddot/6 convention as TLE line 1, so the
	// parser must apply ×2 and ×6 exactly like parseTleLines.
	if !almostEqual(tles[0].MeanMotionDot, 0.0001, 1e-10) {
		t.Errorf("MeanMotionDot = %e, want 1e-4 (5e-5 × 2)", tles[0].MeanMotionDot)
	}
	if !almostEqual(tles[0].MeanMotionDotDot, 0.000006, 1e-10) {
		t.Errorf("MeanMotionDotDot = %e, want 6e-6 (1e-6 × 6)", tles[0].MeanMotionDotDot)
	}
}

func TestParseOMMJSON_SixDigitCatalogNumber(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"FENCE OBJ","OBJECT_ID":"2026-100A","EPOCH":"2026-08-01T00:00:00.000000",
		"MEAN_MOTION":14.0,"ECCENTRICITY":0.005,"INCLINATION":45.0,
		"RA_OF_ASC_NODE":90.0,"ARG_OF_PERICENTER":180.0,"MEAN_ANOMALY":0.5,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":100953,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":10,"BSTAR":0.0001,
		"MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0
	}]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tles[0].ID != 100953 {
		t.Errorf("ID = %d, want 100953 (6-digit catalog number)", tles[0].ID)
	}
}

func TestParseOMMJSON_EpochWithoutMicroseconds(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"TEST","OBJECT_ID":"2020-001A","EPOCH":"2026-01-15T10:30:00",
		"MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		"RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":99999,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":1,"BSTAR":0.0001,
		"MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0
	}]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error parsing epoch without microseconds: %v", err)
	}

	want := time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC)
	if !tles[0].Epoch.Equal(want) {
		t.Errorf("Epoch = %v, want %v", tles[0].Epoch, want)
	}
}

func TestParseOMMJSON_NegativeBStar(t *testing.T) {
	data := []byte(`[{
		"OBJECT_NAME":"TEST","OBJECT_ID":"2020-001A","EPOCH":"2026-01-15T10:30:00.000000",
		"MEAN_MOTION":15.0,"ECCENTRICITY":0.001,"INCLINATION":51.0,
		"RA_OF_ASC_NODE":100.0,"ARG_OF_PERICENTER":200.0,"MEAN_ANOMALY":300.0,
		"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":99999,
		"ELEMENT_SET_NO":1,"REV_AT_EPOCH":1,"BSTAR":-0.0005,
		"MEAN_MOTION_DOT":0.00001,"MEAN_MOTION_DDOT":0
	}]`)

	tles, err := ParseOMMJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(tles[0].BStar, -0.0005, 1e-10) {
		t.Errorf("BStar = %e, want -5e-4", tles[0].BStar)
	}
}

