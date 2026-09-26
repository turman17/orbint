package main

import (
	"fmt"
	"github.com/turman17/orbint/internal/orbit"
)

func main() {
	tleLines := []string{
		"ISS (ZARYA)",
		"1 25544U 98067A   26269.39984368  .00031849  00000+0  59036-3 0  9995",
		"2 25544  51.6292 159.1757 0007020 184.2024 175.8906 15.48664613587440",
	}

	tle, err := orbit.ParseTle(tleLines)
	if err != nil {
		print("\n---------\n")
		fmt.Printf("Error parsing TLE: %s\n", err.Error())
		return
	}
	print("\n---------\n")
	fmt.Printf("Parsed TLE: %+v\n", tle)
}