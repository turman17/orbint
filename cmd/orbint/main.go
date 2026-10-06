package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"github.com/turman17/orbint/internal/orbit"
)

func main() {
	data, err := os.ReadFile("internal/orbit/testdata/iss.tle")
	check(err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	tle, err := orbit.ParseTle(lines)
	check(err)
	fmt.Printf("%+v\n", tle)

	p , err := orbit.NewPropagator(strings.Join(lines, "\n"))
	check(err)

	current_time := time.Now()
	lat, lat, alt , err := p.Position(current_time)
	check(err)
	fmt.Printf("alt: %f, lat: %f ,alt: %f \n", lat , lon , alt)
}
