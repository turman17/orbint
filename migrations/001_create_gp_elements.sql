CREATE TABLE gp_elements (
    norad_cat_id        INTEGER     NOT NULL,
    epoch               TIMESTAMPTZ NOT NULL,
    object_name         TEXT        NOT NULL,
    object_id           TEXT        NOT NULL,
    classification      CHAR(1)     NOT NULL DEFAULT 'U',
    element_set_number  INTEGER     NOT NULL,
    inclination         DOUBLE PRECISION NOT NULL,
    raan                DOUBLE PRECISION NOT NULL,
    eccentricity        DOUBLE PRECISION NOT NULL,
    arg_of_perigee      DOUBLE PRECISION NOT NULL,
    mean_anomaly        DOUBLE PRECISION NOT NULL,
    mean_motion         DOUBLE PRECISION NOT NULL,
    mean_motion_dot     DOUBLE PRECISION NOT NULL,
    mean_motion_ddot    DOUBLE PRECISION NOT NULL,
    bstar               DOUBLE PRECISION NOT NULL,
    ephemeris_type      SMALLINT    NOT NULL DEFAULT 0,
    rev_number          INTEGER     NOT NULL,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (norad_cat_id, epoch)
);
