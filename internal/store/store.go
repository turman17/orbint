package store

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
	"github.com/turman17/orbint/internal/orbit"
)

type Store struct {
	db *sql.DB
}

func NewStore(connStr string) (*Store, error) {
	db, err := sql.Open(

		"postgres",
		connStr,
	)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) InsertTLEs(tles []orbit.TLE) error {
	for _, tle := range tles {
		_, err := s.db.Exec(
			`INSERT INTO gp_elements (
				norad_cat_id,
				epoch,
				object_name,
				object_id,
				classification,
				element_set_number,
				inclination,
				raan,
				eccentricity,
				arg_of_perigee,
				mean_anomaly,
				mean_motion,
				mean_motion_dot,
				mean_motion_ddot,
				bstar,
				ephemeris_type,
				rev_number
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14, $15, $16, $17
			)
			ON CONFLICT (norad_cat_id, epoch) DO NOTHING`,

			tle.ID,                      // $1
			tle.Epoch,                   // $2
			tle.Name,                    // $3
			tle.InternationalDesignator, // $4
			string(tle.Class),           // $5
			tle.ElementSetNumber,        // $6
			tle.Inclination,             // $7
			tle.RAAN,                    // $8
			tle.Eccentricity,            // $9
			tle.ArgumentOfPerigee,       // $10
			tle.MeanAnomaly,             // $11
			tle.MeanMotion,              // $12
			tle.MeanMotionDot,           // $13
			tle.MeanMotionDotDot,        // $14
			tle.BStar,                   // $15
			tle.EphemerisType,           // $16
			tle.RevolutionNumber,        // $17
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) GetHistory(noradCatID int, from time.Time, to time.Time) ([]orbit.TLE, error) {
	rows, err := s.db.Query(
		`SELECT norad_cat_id, epoch, object_name, object_id, classification,
        element_set_number, inclination, raan, eccentricity, arg_of_perigee,
        mean_anomaly, mean_motion, mean_motion_dot, mean_motion_ddot,
        bstar, ephemeris_type, rev_number
    FROM gp_elements
    WHERE norad_cat_id = $1 AND epoch BETWEEN $2 AND $3
    ORDER BY epoch`,
		noradCatID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tles []orbit.TLE
	for rows.Next() {
		var tle orbit.TLE
		var class string
		err := rows.Scan(
			&tle.ID,
			&tle.Epoch,
			&tle.Name,
			&tle.InternationalDesignator,
			&class,
			&tle.ElementSetNumber,
			&tle.Inclination,
			&tle.RAAN,
			&tle.Eccentricity,
			&tle.ArgumentOfPerigee,
			&tle.MeanAnomaly,
			&tle.MeanMotion,
			&tle.MeanMotionDot,
			&tle.MeanMotionDotDot,
			&tle.BStar,
			&tle.EphemerisType,
			&tle.RevolutionNumber,
		)
		if err != nil {
			return nil, err
		}
		tle.Class = class[0]
		tles = append(tles, tle)
	}
	return tles, rows.Err()
}

func (s *Store) ListSatellites() ([]orbit.TLE, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT ON (norad_cat_id)
    norad_cat_id, epoch, object_name, object_id, classification,
    element_set_number, inclination, raan, eccentricity, arg_of_perigee,
    mean_anomaly, mean_motion, mean_motion_dot, mean_motion_ddot,
    bstar, ephemeris_type, rev_number 
    FROM gp_elements 
    ORDER BY norad_cat_id, epoch DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tles []orbit.TLE
	for rows.Next() {
		var tle orbit.TLE
		var class string
		err := rows.Scan(
			&tle.ID,
			&tle.Epoch,
			&tle.Name,
			&tle.InternationalDesignator,
			&class,
			&tle.ElementSetNumber,
			&tle.Inclination,
			&tle.RAAN,
			&tle.Eccentricity,
			&tle.ArgumentOfPerigee,
			&tle.MeanAnomaly,
			&tle.MeanMotion,
			&tle.MeanMotionDot,
			&tle.MeanMotionDotDot,
			&tle.BStar,
			&tle.EphemerisType,
			&tle.RevolutionNumber,
		)

		if err != nil {
			return nil, err
		}
		tle.Class = class[0]
		tles = append(tles, tle)
	}
	return tles, rows.Err()
}

func (s *Store) GetLatest(id int) (*orbit.TLE, error) {
	var tle orbit.TLE
	var class string

	err := s.db.QueryRow(`
		SELECT
			norad_cat_id,
			epoch,
			object_name,
			object_id,
			classification,
			element_set_number,
			inclination,
			raan,
			eccentricity,
			arg_of_perigee,
			mean_anomaly,
			mean_motion,
			mean_motion_dot,
			mean_motion_ddot,
			bstar,
			ephemeris_type,
			rev_number
		FROM gp_elements
		WHERE norad_cat_id = $1
		ORDER BY epoch DESC
		LIMIT 1
	`, id).Scan(
		&tle.ID,
		&tle.Epoch,
		&tle.Name,
		&tle.InternationalDesignator,
		&class,
		&tle.ElementSetNumber,
		&tle.Inclination,
		&tle.RAAN,
		&tle.Eccentricity,
		&tle.ArgumentOfPerigee,
		&tle.MeanAnomaly,
		&tle.MeanMotion,
		&tle.MeanMotionDot,
		&tle.MeanMotionDotDot,
		&tle.BStar,
		&tle.EphemerisType,
		&tle.RevolutionNumber,
	)

	if err != nil {
		return nil, err
	}

	if len(class) > 0 {
		tle.Class = class[0]
	}

	return &tle, nil
}
