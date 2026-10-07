package orbit

import (
	"encoding/json"
	"fmt"
	"time"
)

type ommRecord struct {
	ObjectName         string  `json:"OBJECT_NAME"`
	ObjectID           string  `json:"OBJECT_ID"`
	Epoch              string  `json:"EPOCH"`
	MeanMotion         float64 `json:"MEAN_MOTION"`
	Eccentricity       float64 `json:"ECCENTRICITY"`
	Inclination        float64 `json:"INCLINATION"`
	RAOfAscNode        float64 `json:"RA_OF_ASC_NODE"`
	ArgOfPericenter    float64 `json:"ARG_OF_PERICENTER"`
	MeanAnomaly        float64 `json:"MEAN_ANOMALY"`
	EphemerisType      int     `json:"EPHEMERIS_TYPE"`
	ClassificationType string  `json:"CLASSIFICATION_TYPE"`
	NoradCatID         int     `json:"NORAD_CAT_ID"`
	ElementSetNo       int     `json:"ELEMENT_SET_NO"`
	RevAtEpoch         int     `json:"REV_AT_EPOCH"`
	BStar              float64 `json:"BSTAR"`
	MeanMotionDot      float64 `json:"MEAN_MOTION_DOT"`
	MeanMotionDDot     float64 `json:"MEAN_MOTION_DDOT"`
}

func ParseOMMJSON(data []byte) ([]TLE, error) {
	var records []ommRecord

	err := json.Unmarshal(data, &records)
	if err != nil {
		return nil, err
	}

	tles := make([]TLE, 0, len(records))

	for _, rec := range records {
		epoch, err := time.Parse("2006-01-02T15:04:05.999999", rec.Epoch)
		if err != nil {
			return nil, fmt.Errorf("invalid epoch %q: %w", rec.Epoch, err)
		}
		var class byte
		if len(rec.ClassificationType) > 0 {
			class = rec.ClassificationType[0]
		}
		tle := TLE{
			Name:                    rec.ObjectName,
			ID:                      rec.NoradCatID,
			Class:                   class,
			InternationalDesignator: rec.ObjectID,
			ElementSetNumber:        rec.ElementSetNo,
			Epoch:                   epoch.UTC(),
			Inclination:             rec.Inclination,
			RAAN:                    rec.RAOfAscNode,
			Eccentricity:            rec.Eccentricity,
			ArgumentOfPerigee:       rec.ArgOfPericenter,
			MeanAnomaly:             rec.MeanAnomaly,
			MeanMotion:              rec.MeanMotion,
			MeanMotionDot:           rec.MeanMotionDot,
			MeanMotionDotDot:        rec.MeanMotionDDot,
			BStar:                   rec.BStar,
			EphemerisType:           rec.EphemerisType,
			RevolutionNumber:        rec.RevAtEpoch,
		}
		tles = append(tles, tle)
	}
	return tles, nil
}
