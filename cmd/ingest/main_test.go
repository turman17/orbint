package main

import (
	"reflect"
	"testing"
)

func TestGroups(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want []string
	}{
		{"default when unset", "", []string{"stations", "visual", "weather", "noaa", "iridium-NEXT", "gnss", "geo", "science", "last-30-days"}},
		{"default when blank", "   ", []string{"stations", "visual", "weather", "noaa", "iridium-NEXT", "gnss", "geo", "science", "last-30-days"}},
		{"single", "stations", []string{"stations"}},
		{"trims and drops empties", " stations, geo ,,active, ", []string{"stations", "geo", "active"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CELESTRAK_GROUPS", c.env)
			if got := groups(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("groups() = %v, want %v", got, c.want)
			}
		})
	}
}
