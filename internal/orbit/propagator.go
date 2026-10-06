package orbit

import (
	"time"

	"github.com/akhenakh/sgp4"
)

type Propagator struct {
	tle *sgp4.TLE
}

func NewPropagator(tle string) (*Propagator, error) {
	p, err := sgp4.ParseTLE(tle)
	if err != nil{
		return nil,  err
	}
	return &Propagator{p}, nil
}

func (p *Propagator)Position(t time.Time) (lat, lon, alt float64, err error) {
	pos, err := p.tle.FindPositionAtTime(t)
	if err != nil{
		return 0, 0, 0, err
	}
	lat, lon, alt = pos.ToGeodetic()
	return lat , lon , alt , nil
}


